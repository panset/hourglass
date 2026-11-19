// SPDX-License-Identifier: MIT

package auth

import (
	"bytes"
	"context"
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

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

var t0 = time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)

const testKey = "0123456789abcdef0123456789abcdef-test-only"

type fakeEmitter struct {
	mu     sync.Mutex
	events []hourglass.Event
}

func (e *fakeEmitter) Emit(ev hourglass.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, ev)
}

func (e *fakeEmitter) list() []hourglass.Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]hourglass.Event(nil), e.events...)
}

// syncBuffer is a goroutine-safe log sink.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

type env struct {
	t     *testing.T
	st    *store.Store
	svc   *Service
	clock *hourglasstest.FakeClock
	em    *fakeEmitter
	logs  *syncBuffer
	h     http.Handler
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "hourglass.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// newEnv builds a Service with a fake clock and a mux carrying the auth
// routes plus stand-in data routes behind RoleRoute and AdminRoute.
func newEnv(t *testing.T, cfg Config, opts ...Option) *env {
	t.Helper()
	e := &env{t: t, st: newStore(t), clock: hourglasstest.NewFakeClock(t0), em: &fakeEmitter{}, logs: &syncBuffer{}}
	if cfg.SessionKey == "" {
		cfg.SessionKey = testKey
	}
	log := slog.New(slog.NewTextHandler(e.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	svc, err := New(cfg, e.st, e.em, log, append([]Option{WithClock(e.clock.Now)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	e.svc = svc
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":"secret-data"}`)
	})
	mux := http.NewServeMux()
	svc.Register(mux.Handle)
	mux.Handle("GET /api/v1/me", svc.RoleRoute(ok))
	mux.Handle("POST /api/v1/setups/{id}/acquire", svc.RoleRoute(ok))
	mux.Handle("GET /api/v1/admin/users", svc.AdminRoute(ok))
	mux.Handle("PUT /api/v1/admin/settings", svc.AdminRoute(ok))
	e.h = svc.CORS(svc.Authenticate(mux))
	return e
}

func (e *env) ctx() context.Context { return context.Background() }

// user creates an OIDC user with the given status and roles.
func (e *env) user(sub, status string, roles ...string) store.User {
	e.t.Helper()
	u, err := e.st.CreateOIDCUser(e.ctx(), sub, sub+"@example.test", "User "+sub, e.clock.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.st.SetUserStatus(e.ctx(), u.ID, status); err != nil {
		e.t.Fatal(err)
	}
	if err := e.st.SetUserRoles(e.ctx(), u.ID, roles); err != nil {
		e.t.Fatal(err)
	}
	return u
}

// login inserts a session row for userID and returns the cookie value and CSRF token.
func (e *env) login(userID int64) (cookie, csrf string) {
	e.t.Helper()
	c, id, err := newSessionToken()
	if err != nil {
		e.t.Fatal(err)
	}
	now := e.clock.Now()
	if err := e.st.CreateSession(e.ctx(), id, userID, now, now.Add(IdleTimeout)); err != nil {
		e.t.Fatal(err)
	}
	return c, e.svc.CSRFToken(id)
}

type req struct {
	method, path string
	body         any
	cookie, csrf string
	header       map[string]string
}

func (e *env) do(r req) *httptest.ResponseRecorder {
	e.t.Helper()
	var body io.Reader
	if r.body != nil {
		switch b := r.body.(type) {
		case string:
			body = strings.NewReader(b)
		default:
			raw, err := json.Marshal(b)
			if err != nil {
				e.t.Fatal(err)
			}
			body = bytes.NewReader(raw)
		}
	}
	hr := httptest.NewRequest(r.method, "https://hourglass.example"+r.path, body)
	if r.body != nil {
		hr.Header.Set("Content-Type", "application/json")
	}
	if r.cookie != "" {
		hr.AddCookie(&http.Cookie{Name: SessionCookie, Value: r.cookie})
	}
	if r.csrf != "" {
		hr.Header.Set(CSRFHeader, r.csrf)
	}
	for k, v := range r.header {
		hr.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, hr)
	return rec
}

// errCode returns the error code of an error response, failing if the body
// holds anything but the error object.
func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	if len(raw) != 1 || raw["error"] == nil {
		t.Fatalf("error body carries more than the error: %s", rec.Body.String())
	}
	var b apierr.Body
	_ = json.Unmarshal(rec.Body.Bytes(), &b)
	return b.Error.Code
}

func wantErr(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status %d, want %d; body %s", rec.Code, status, rec.Body.String())
	}
	if got := errCode(t, rec); got != code {
		t.Fatalf("code %q, want %q", got, code)
	}
	if strings.Contains(rec.Body.String(), "secret-data") {
		t.Fatal("error response leaked data")
	}
}

func cookieNamed(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func decodeView(t *testing.T, rec *httptest.ResponseRecorder) SessionView {
	t.Helper()
	var v SessionView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return v
}

func slogTo(b *syncBuffer) *slog.Logger { return slog.New(slog.NewTextHandler(b, nil)) }
