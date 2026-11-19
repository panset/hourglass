// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/collector"
	"github.com/panset/hourglass/internal/events"
	"github.com/panset/hourglass/internal/holds"
	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

const testAdminPassword = "correct horse battery staple"

// syncBuf is a bytes.Buffer safe for concurrent use (a logger writes to it).
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// backend is a full Hourglass backend: real store, ledger, holds, collector,
// events and router; fakes only for the extensions and the clock. The three
// test environments (router, user routes, admin routes) are built on it.
type backend struct {
	t        *testing.T
	st       *store.Store
	clk      *hourglasstest.FakeClock
	auth     *auth.Service
	led      *ledger.Service
	col      *collector.Collector
	ev       *events.Dispatcher
	prov     *hourglasstest.FakeProvider
	pressure *hourglasstest.FakePressureSource
	policy   *hourglasstest.FakePolicy
	hook     *hourglasstest.FakeHook
	h        http.Handler
	logs     *syncBuf
	admin    store.User // the local admin
}

type backendOpts struct {
	start      time.Time      // fake clock start; zero means now
	loc        *time.Location // configured time zone; nil means UTC
	oidcIssuer string
	origins    []string
	fallback   http.Handler
	runtime    Runtime
	// startEvents starts the event dispatcher's delivery worker.
	startEvents bool
	// keepMustChange leaves the local admin's forced password change in place.
	keepMustChange bool
}

func newBackend(t *testing.T, o backendOpts) *backend {
	t.Helper()
	ctx := context.Background()
	if o.start.IsZero() {
		o.start = time.Now()
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	logs := &syncBuf{}
	log := slog.New(slog.NewJSONHandler(logs, nil))
	cfg := auth.Config{SessionKey: strings.Repeat("k", 40), AllowedOrigins: o.origins}
	if o.oidcIssuer != "" {
		cfg.OIDC = auth.OIDCConfig{IssuerURL: o.oidcIssuer, ClientID: "cid", ClientSecret: "csecret", RedirectURL: "http://app.test/auth/callback"}
	}
	svc, err := auth.New(cfg, st, nil, log)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := auth.EnsureLocalAdmin(ctx, st, testAdminPassword, log)
	if err != nil {
		t.Fatal(err)
	}
	if !o.keepMustChange {
		if err := st.SetPassword(ctx, admin.ID, admin.PasswordHash, false); err != nil {
			t.Fatal(err)
		}
	}
	b := &backend{t: t, st: st, auth: svc, logs: logs, admin: admin, clk: hourglasstest.NewFakeClock(o.start),
		prov: hourglasstest.NewFakeProvider(), pressure: hourglasstest.NewFakePressureSource(),
		policy: hourglasstest.NewFakePolicy(), hook: hourglasstest.NewFakeHook()}
	b.pressure.SetValue(0.8) // 2x with the default bands
	b.ev = events.New([]hourglass.Hook{b.hook}, st, events.WithTimeout(time.Second))
	if o.startEvents {
		b.ev.Start(ctx)
	}
	t.Cleanup(func() {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = b.ev.Close(cctx)
	})
	loc := o.loc
	if loc == nil {
		loc = time.UTC
	}
	b.led = ledger.NewService(st, b.ev, b.clk, loc)
	b.col = collector.New(collector.Deps{Store: st, Provider: b.prov, Pressure: b.pressure, Policy: b.policy,
		Events: b.ev, Ledger: b.led, Clock: b.clk, Logger: log})
	hs := holds.NewService(holds.Deps{Store: st, Events: b.ev, ProviderConfigured: true, Clock: b.clk, Logger: log,
		Poll: func() holds.PollView { return b.col.LastPoll().ForHolds() }})
	b.h = New(Deps{Store: st, Auth: svc, Holds: hs, Ledger: b.led, Events: b.ev, Collector: b.col, Clock: b.clk,
		Location: loc, Logger: log, Runtime: o.runtime, Fallback: o.fallback})
	return b
}

// sess is a signed-in client: the user, their cookie and CSRF token.
type sess struct {
	user         store.User
	cookie, csrf string
}

// Every test environment speaks the same session type.
type (
	session = sess
	usess   = sess
	adSess  = sess
)

// session stores a session row for the user and returns its cookie and the
// CSRF token the API reports for it.
func (b *backend) session(u store.User) sess {
	b.t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		b.t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	id := hex.EncodeToString(sum[:])
	now := time.Now()
	if err := b.st.CreateSession(context.Background(), id, u.ID, now, now.Add(time.Hour)); err != nil {
		b.t.Fatal(err)
	}
	return sess{user: u, cookie: base64.RawURLEncoding.EncodeToString(raw), csrf: b.auth.CSRFToken(id)}
}

// newUser creates an OIDC user. Status "pending" leaves the new-user default;
// any other status is set. Roles are stored with the first allowance, as an
// approval would.
func (b *backend) newUser(name, status string, roles ...string) sess {
	b.t.Helper()
	ctx := context.Background()
	u, err := b.st.CreateOIDCUser(ctx, name, name+"@example.test", "User "+name, b.clk.Now())
	if err != nil {
		b.t.Fatal(err)
	}
	if status != store.StatusPending {
		if err := b.st.SetUserStatus(ctx, u.ID, status); err != nil {
			b.t.Fatal(err)
		}
	}
	if len(roles) > 0 {
		if err := b.st.SetUserRoles(ctx, u.ID, roles); err != nil {
			b.t.Fatal(err)
		}
		if err := b.st.Tx(ctx, func(tx *store.Tx) error { return b.led.EnsureFirstAllowanceTx(tx, u.ID, b.clk.Now()) }); err != nil {
			b.t.Fatal(err)
		}
	}
	u, _ = b.st.GetUser(ctx, u.ID)
	return b.session(u)
}

// serve sends one request through the router. s may be nil (no session); csrf
// says whether the session's CSRF header is sent. hdr is name, value pairs.
func (b *backend) serve(method, path string, s *sess, csrf bool, body string, hdr ...string) *httptest.ResponseRecorder {
	b.t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if s != nil {
		req.Header.Set("Cookie", auth.SessionCookie+"="+s.cookie)
		if csrf {
			req.Header.Set(auth.CSRFHeader, s.csrf)
		}
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	b.h.ServeHTTP(rec, req)
	return rec
}

// env is the router test environment: the local admin still has to change
// the password, as after a first start.
type env struct{ *backend }

type envOpts struct {
	oidcIssuer string
	origins    []string
	fallback   http.Handler
}

func newEnv(t *testing.T, o envOpts) *env {
	t.Helper()
	return &env{newBackend(t, backendOpts{oidcIssuer: o.oidcIssuer, origins: o.origins, fallback: o.fallback, keepMustChange: true})}
}

// user creates an OIDC user with the given status and roles and a session.
func (e *env) user(sub, status string, roles ...string) session {
	return e.newUser(sub, status, roles...)
}

// adminSession is a session of the local admin after the password change
// requirement was cleared.
func (e *env) adminSession(clearMustChange bool) session {
	e.t.Helper()
	if clearMustChange {
		if err := e.st.SetPassword(context.Background(), e.admin.ID, e.admin.PasswordHash, false); err != nil {
			e.t.Fatal(err)
		}
	}
	return e.session(e.admin)
}

type reply struct {
	*httptest.ResponseRecorder
}

func (r reply) errCode() string {
	var b struct {
		Error struct{ Code, Message, Field string }
	}
	_ = json.Unmarshal(r.Body.Bytes(), &b)
	return b.Error.Code
}

func (e *env) do(method, path string, s *session, csrf bool, body string, hdr ...string) reply {
	e.t.Helper()
	return reply{e.serve(method, path, s, csrf, body, hdr...)}
}

func newLogger(w io.Writer) *slog.Logger { return slog.New(slog.NewJSONHandler(w, nil)) }

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
