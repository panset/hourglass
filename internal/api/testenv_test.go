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
	"github.com/panset/hourglass/internal/store"
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

type env struct {
	t       *testing.T
	st      *store.Store
	auth    *auth.Service
	h       http.Handler
	logs    *syncBuf
	admin   store.User // the local admin (must_change_password true until changed)
	handler func(Deps) Deps
}

type envOpts struct {
	oidcIssuer string
	origins    []string
	fallback   http.Handler
}

func newEnv(t *testing.T, o envOpts) *env {
	t.Helper()
	ctx := context.Background()
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
	e := &env{t: t, st: st, auth: svc, logs: logs, admin: admin}
	e.h = New(Deps{Store: st, Auth: svc, Logger: log, Fallback: o.fallback})
	return e
}

// session is a signed-in client: its cookie and CSRF token.
type session struct {
	cookie, csrf string
}

// sessionFor stores a session row for the user and returns its cookie and
// the CSRF token the API reports for it.
func (e *env) sessionFor(userID int64) session {
	e.t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		e.t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	id := hex.EncodeToString(sum[:])
	now := time.Now()
	if err := e.st.CreateSession(context.Background(), id, userID, now, now.Add(time.Hour)); err != nil {
		e.t.Fatal(err)
	}
	return session{cookie: base64.RawURLEncoding.EncodeToString(raw), csrf: e.auth.CSRFToken(id)}
}

// user creates an OIDC user with the given status and roles and a session.
func (e *env) user(sub, status string, roles ...string) session {
	e.t.Helper()
	ctx := context.Background()
	u, err := e.st.CreateOIDCUser(ctx, sub, sub+"@example.test", sub, time.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	if status != store.StatusPending {
		if err := e.st.SetUserStatus(ctx, u.ID, status); err != nil {
			e.t.Fatal(err)
		}
	}
	if len(roles) > 0 {
		if err := e.st.SetUserRoles(ctx, u.ID, roles); err != nil {
			e.t.Fatal(err)
		}
	}
	return e.sessionFor(u.ID)
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
	return e.sessionFor(e.admin.ID)
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
	e.h.ServeHTTP(rec, req)
	return reply{rec}
}

func newLogger(w io.Writer) *slog.Logger { return slog.New(slog.NewJSONHandler(w, nil)) }

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
