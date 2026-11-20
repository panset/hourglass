// SPDX-License-Identifier: MIT

package auth

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/store"
)

// svcEnv is an auth env plus a mux with one integration route behind
// ServiceRoute(state:read) and one session route behind RoleRoute.
type svcEnv struct {
	*env
	h http.Handler
}

func newSvcEnv(t *testing.T) *svcEnv {
	t.Helper()
	e := newEnv(t, Config{})
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := ServiceFrom(r.Context())
		_, _ = io.WriteString(w, "ok "+p.AccountName)
	})
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/integration/state", e.svc.ServiceRoute(ScopeStateRead, ok))
	mux.Handle("GET /api/v1/me", e.svc.RoleRoute(ok))
	return &svcEnv{env: e, h: e.svc.CORS(e.svc.Authenticate(mux))}
}

func (e *svcEnv) account(name string, scopes ...string) store.ServiceAccount {
	e.t.Helper()
	a, err := e.st.CreateServiceAccount(e.ctx(), store.ServiceAccount{Name: name, Scopes: scopes, Enabled: true}, e.clock.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	return a
}

func (e *svcEnv) token(accountID int64, expires *time.Time) (string, store.ServiceToken) {
	e.t.Helper()
	secret, hash, err := NewServiceSecret()
	if err != nil {
		e.t.Fatal(err)
	}
	row, err := e.st.InsertServiceToken(e.ctx(), accountID, hash, expires, e.clock.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	return FormatServiceToken(row.ID, secret), row
}

func (e *svcEnv) get(path, token, addr string) *httptest.ResponseRecorder {
	e.t.Helper()
	r := httptest.NewRequest("GET", "https://hourglass.example"+path, nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if addr != "" {
		r.RemoteAddr = addr
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, r)
	return rec
}

var tokenShape = regexp.MustCompile(`^hgsa_([0-9]+)_([0-9a-f]{64})$`)

// SVC-4: the token is hgsa_<id>_<secret> with 256 bits of secret, and only
// the SHA-256 of the secret is stored.
func TestSVC4_TokenFormatAndStoredHash(t *testing.T) {
	e := newSvcEnv(t)
	a := e.account("svc", ScopeStateRead)
	tok, row := e.token(a.ID, nil)
	m := tokenShape.FindStringSubmatch(tok)
	if m == nil || m[1] != "1" {
		t.Fatalf("token %q does not have the documented shape", tok)
	}
	if row.TokenHash != HashServiceSecret(m[2]) || strings.Contains(row.TokenHash, m[2]) || len(row.TokenHash) != 64 {
		t.Errorf("stored hash %q is not hex SHA-256 of the secret", row.TokenHash)
	}
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		s, _, err := NewServiceSecret()
		if err != nil || len(s) != 64 || seen[s] {
			t.Fatalf("secret %q (err %v) is not 32 fresh random bytes", s, err)
		}
		seen[s] = true
	}
	if rec := e.get("/api/v1/integration/state", tok, ""); rec.Code != 200 || rec.Body.String() != "ok svc" {
		t.Fatalf("valid token: %d %s", rec.Code, rec.Body)
	}
}

// SVC-5: expiry is checked on every request, to the instant; a malformed,
// unknown or wrong token is the same 401.
func TestSVC5_ExpiryAndBadTokensAre401(t *testing.T) {
	e := newSvcEnv(t)
	a := e.account("svc", ScopeStateRead)
	exp := e.clock.Now().Add(time.Hour)
	tok, _ := e.token(a.ID, &exp)
	if rec := e.get("/api/v1/integration/state", tok, ""); rec.Code != 200 {
		t.Fatalf("before expiry: %d %s", rec.Code, rec.Body)
	}
	e.clock.Advance(time.Hour - time.Millisecond)
	if rec := e.get("/api/v1/integration/state", tok, ""); rec.Code != 200 {
		t.Fatalf("just before expiry: %d %s", rec.Code, rec.Body)
	}
	e.clock.Advance(time.Millisecond)
	wantErr(t, e.get("/api/v1/integration/state", tok, ""), 401, apierr.Unauthenticated)

	good, _ := e.token(a.ID, nil)
	id, secret, _ := parseServiceToken(good)
	flipped := []byte(secret)
	flipped[0] ^= 1
	var first string
	for i, bad := range []string{
		"", "hgsa_", "hgsa_1", "nope", strings.TrimPrefix(good, "hgsa_"),
		FormatServiceToken(id, string(flipped)), FormatServiceToken(999, secret), good + "0",
	} {
		e.svc.bearerLimiter = addrLimiter{} // this test is about the refusal, not the limit
		r := httptest.NewRequest("GET", "/api/v1/integration/state", nil)
		r.Header.Set("Authorization", "Bearer "+bad)
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, r)
		wantErr(t, rec, 401, apierr.Unauthenticated)
		if i == 0 {
			first = rec.Body.String()
		} else if rec.Body.String() != first {
			t.Errorf("refusal of %q differs from the others: %s", bad, rec.Body)
		}
	}
	wantErr(t, e.get("/api/v1/integration/state", "", ""), 401, apierr.Unauthenticated)
}

// last_used_at is written on use, at most once a minute (D44).
func TestSVC_LastUsedAtAtMostOnceAMinute(t *testing.T) {
	e := newSvcEnv(t)
	a := e.account("svc", ScopeStateRead)
	tok, row := e.token(a.ID, nil)
	lastUsed := func() *time.Time {
		t.Helper()
		got, _, err := e.st.GetServiceTokenWithAccount(e.ctx(), row.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got.LastUsedAt
	}
	if lastUsed() != nil {
		t.Fatal("a new token has last_used_at")
	}
	t1 := e.clock.Now()
	e.get("/api/v1/integration/state", tok, "")
	if lu := lastUsed(); lu == nil || !lu.Equal(t1) {
		t.Fatalf("after first use: %v", lu)
	}
	e.clock.Advance(30 * time.Second)
	e.get("/api/v1/integration/state", tok, "")
	if lu := lastUsed(); !lu.Equal(t1) {
		t.Fatalf("written again within a minute: %v", lu)
	}
	e.clock.Advance(30 * time.Second)
	e.get("/api/v1/integration/state", tok, "")
	if lu := lastUsed(); !lu.Equal(t1.Add(time.Minute)) {
		t.Fatalf("not written after a minute: %v", lu)
	}
}

// SVC-1 and SVC-2 at the middleware: the bearer token means nothing on a
// session route, and the cookie means nothing on an integration route.
func TestSVC1_SVC2_BoundaryInTheMiddleware(t *testing.T) {
	e := newSvcEnv(t)
	a := e.account("svc", Scopes()...)
	tok, _ := e.token(a.ID, nil)
	wantErr(t, e.get("/api/v1/me", tok, ""), 401, apierr.Unauthenticated)

	admin := e.user("boss", store.StatusActive, store.RoleAdmin)
	cookie, _ := e.login(admin.ID)
	r := httptest.NewRequest("GET", "/api/v1/integration/state", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookie, Value: cookie})
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, r)
	wantErr(t, rec, 401, apierr.Unauthenticated)
}

// SVC-7: a valid token always authenticates, from any address, and is not a
// failure. After five failed attempts from one address the next failed attempt
// is 429 instead of 401; another address and the window are unaffected.
func TestSVC7_LimiterPerAddress(t *testing.T) {
	e := newSvcEnv(t)
	a := e.account("svc", ScopeStateRead)
	tok, row := e.token(a.ID, nil)
	const path = "/api/v1/integration/state"
	bad := FormatServiceToken(row.ID, strings.Repeat("a", 64))
	unknown := FormatServiceToken(row.ID+500, strings.Repeat("a", 64))
	for i := 0; i < maxFailures; i++ {
		wantErr(t, e.get(path, bad, "198.51.100.7:4000"), 401, apierr.Unauthenticated)
		// Valid requests in between are not failures and are never refused.
		if rec := e.get(path, tok, "198.51.100.7:4000"); rec.Code != 200 {
			t.Fatalf("valid token while counting: %d %s", rec.Code, rec.Body)
		}
	}
	// Over the limit: a valid token still works, and does not extend the lockout.
	for i := 0; i < 20; i++ {
		if rec := e.get(path, tok, "198.51.100.7:4001"); rec.Code != 200 {
			t.Fatalf("valid token over the limit: %d %s", rec.Code, rec.Body)
		}
	}
	// A failed attempt is 429, and a known and an unknown token id look the same.
	var bodies []string
	for _, b := range []string{bad, unknown, "garbage"} {
		rec := e.get(path, b, "198.51.100.7:4002")
		wantErr(t, rec, 429, apierr.RateLimited)
		if s := rec.Header().Get("Retry-After"); s == "" || s == "0" {
			t.Errorf("Retry-After = %q", s)
		}
		bodies = append(bodies, rec.Body.String())
	}
	if bodies[0] != bodies[1] || bodies[1] != bodies[2] {
		t.Errorf("over-limit bodies differ: %q", bodies)
	}
	wantErr(t, e.get(path, bad, "198.51.100.8:4000"), 401, apierr.Unauthenticated)
	e.clock.Advance(failureWindow)
	wantErr(t, e.get(path, bad, "198.51.100.7:4003"), 401, apierr.Unauthenticated)
}

// The limiter reserves an attempt under one lock: a burst of concurrent
// failures from one address lets exactly maxFailures through. Run with -race.
func TestSVC7_ConcurrentFailuresCannotExceedTheLimit(t *testing.T) {
	var l addrLimiter
	now := time.Unix(1_700_000_000, 0)
	var wg sync.WaitGroup
	var passed atomic.Int64
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limited, _ := l.attempt("192.0.2.1", now); !limited {
				passed.Add(1)
			}
		}()
	}
	wg.Wait()
	if passed.Load() != maxFailures {
		t.Fatalf("%d attempts passed, want %d", passed.Load(), maxFailures)
	}
}

// The limiter sweeps stale addresses at most once a minute and its map is
// bounded.
func TestSVC7_LimiterSweepAndCap(t *testing.T) {
	var l addrLimiter
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < maxTrackedAddrs; i++ {
		if limited, _ := l.attempt(fmt.Sprintf("a%d", i), now); limited {
			t.Fatalf("address %d limited", i)
		}
	}
	// Full of live addresses: a new address is refused, and the map does not grow.
	if limited, _ := l.attempt("new", now.Add(time.Second)); !limited {
		t.Error("new address accepted while the map is full")
	}
	if len(l.by) != maxTrackedAddrs {
		t.Errorf("map holds %d addresses, cap is %d", len(l.by), maxTrackedAddrs)
	}
	// After the window the stale addresses are swept and a new one is accepted.
	later := now.Add(failureWindow + sweepEvery)
	if limited, _ := l.attempt("new", later); limited {
		t.Error("new address refused after the sweep")
	}
	if len(l.by) != 1 {
		t.Errorf("map holds %d addresses after the sweep, want 1", len(l.by))
	}
	// No second sweep inside sweepEvery.
	next := l.nextSweep
	l.attempt("other", later.Add(time.Second))
	if !l.nextSweep.Equal(next) {
		t.Error("swept again within sweepEvery")
	}
}

// A token has exactly one spelling: the id is canonical decimal. Anything else
// is an ordinary failed attempt (401) and counts as a failure.
func TestSVC_TokenIDMustBeCanonicalDecimal(t *testing.T) {
	e := newSvcEnv(t)
	a := e.account("svc", ScopeStateRead)
	tok, row := e.token(a.ID, nil)
	if row.ID != 1 {
		t.Fatalf("token id %d, test expects 1", row.ID)
	}
	_, secret, _ := parseServiceToken(tok)
	const path = "/api/v1/integration/state"
	for i, spelling := range []string{"+1", "01", "001", "1 ", "-1", "0", "1.0", "0x1", "\u0661", ""} {
		bad := ServiceTokenPrefix + spelling + "_" + secret
		if _, _, ok := parseServiceToken(bad); ok {
			t.Errorf("%q parsed", bad)
		}
		addr := fmt.Sprintf("203.0.113.%d:1", i+1)
		wantErr(t, e.get(path, bad, addr), 401, apierr.Unauthenticated)
		rec := e.get(path, bad, addr)
		if rec.Code != 401 {
			t.Errorf("%q: %d", bad, rec.Code)
		}
	}
	if rec := e.get(path, tok, ""); rec.Code != 200 {
		t.Fatalf("canonical token: %d %s", rec.Code, rec.Body)
	}
}

// SVC-2 defence in depth: on an integration path Authenticate leaves no user
// in the context even when the request carries a valid session cookie, so a
// handler that read UserFrom there would not honour it.
func TestSVC2_AuthenticateIgnoresTheCookieOnIntegrationPaths(t *testing.T) {
	e := newSvcEnv(t)
	admin := e.user("boss", store.StatusActive, store.RoleAdmin)
	cookie, _ := e.login(admin.ID)
	mux := http.NewServeMux()
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFrom(r.Context()); ok {
			_, _ = io.WriteString(w, "user")
			return
		}
		_, _ = io.WriteString(w, "anonymous")
	})
	mux.Handle("GET /api/v1/integration/probe", probe)
	mux.Handle("GET /api/v1/other/probe", probe)
	h := e.svc.Authenticate(mux)
	get := func(path string) string {
		r := httptest.NewRequest("GET", path, nil)
		r.AddCookie(&http.Cookie{Name: SessionCookie, Value: cookie})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Body.String()
	}
	if got := get("/api/v1/other/probe"); got != "user" {
		t.Fatalf("control: the cookie was not honoured outside the prefix: %q", got)
	}
	if got := get("/api/v1/integration/probe"); got != "anonymous" {
		t.Errorf("the cookie was honoured on an integration path: %q", got)
	}
}

func TestSVC_ServiceRouteRejectsUnknownScope(t *testing.T) {
	e := newSvcEnv(t)
	defer func() {
		if recover() == nil {
			t.Error("ServiceRoute accepted an unknown scope")
		}
	}()
	e.svc.ServiceRoute("admin", http.NotFoundHandler())
}
