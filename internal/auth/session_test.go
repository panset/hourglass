// SPDX-License-Identifier: MIT

package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/store"
)

func TestAUTH3_SessionCookieAttributes(t *testing.T) {
	e := newEnv(t, Config{})
	e.localAdmin()
	rec := e.do(req{method: "POST", path: "/auth/local", body: map[string]string{"username": "admin", "password": adminPW}})
	c := cookieNamed(rec, SessionCookie)
	if c == nil {
		t.Fatal("no cookie")
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Name != "hourglass_session" {
		t.Fatalf("cookie attributes %+v", c)
	}
	raw := rec.Header().Get("Set-Cookie")
	for _, want := range []string{"HttpOnly", "Secure", "SameSite=Lax", "Path=/"} {
		if !strings.Contains(raw, want) {
			t.Fatalf("Set-Cookie %q lacks %s", raw, want)
		}
	}
	// The stored id is the SHA-256 hex of the cookie bytes, not the value itself.
	id, ok := sessionIDFromCookie(c.Value)
	if !ok || len(id) != 64 || id == c.Value {
		t.Fatalf("id %q", id)
	}
	if _, err := e.st.GetSession(e.ctx(), id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.st.GetSession(e.ctx(), c.Value); err == nil {
		t.Fatal("raw cookie value stored as id")
	}
}

func TestAUTH3_IdleTimeout(t *testing.T) {
	e := newEnv(t, Config{})
	u := e.user("sub-1", store.StatusActive, store.RoleDeveloper)
	c, _ := e.login(u.ID)
	id, _ := sessionIDFromCookie(c)
	get := func() int { return e.do(req{method: "GET", path: "/api/v1/me", cookie: c}).Code }

	// Activity within 8h keeps it alive; expires_at is refreshed at most once a minute.
	e.clock.Advance(30 * time.Second)
	if get() != http.StatusOK {
		t.Fatal("fresh session refused")
	}
	if s, _ := e.st.GetSession(e.ctx(), id); !s.LastSeenAt.Equal(t0) {
		t.Fatalf("touched within a minute: %v", s.LastSeenAt)
	}
	e.clock.Advance(7*time.Hour + 59*time.Minute)
	if get() != http.StatusOK {
		t.Fatal("session expired before 8h idle")
	}
	s, _ := e.st.GetSession(e.ctx(), id)
	if !s.LastSeenAt.Equal(e.clock.Now()) || !s.ExpiresAt.Equal(e.clock.Now().Add(IdleTimeout)) {
		t.Fatalf("not refreshed: %+v", s)
	}
	// 8 hours idle: gone.
	e.clock.Advance(IdleTimeout)
	if code := get(); code != http.StatusUnauthorized {
		t.Fatalf("after idle timeout: %d", code)
	}
	if _, err := e.st.GetSession(e.ctx(), id); err == nil {
		t.Fatal("expired session row kept")
	}
}

func TestAUTH3_CSRFRequiredOnPostNotGet(t *testing.T) {
	e := newEnv(t, Config{})
	u := e.user("sub-1", store.StatusActive, store.RoleDeveloper)
	c, csrf := e.login(u.ID)
	other := e.user("sub-2", store.StatusActive, store.RoleDeveloper)
	_, otherCSRF := e.login(other.ID)

	if r := e.do(req{method: "GET", path: "/api/v1/me", cookie: c}); r.Code != http.StatusOK {
		t.Fatalf("GET without CSRF: %d", r.Code)
	}
	wantErr(t, e.do(req{method: "POST", path: "/api/v1/setups/1/acquire", cookie: c}), http.StatusForbidden, apierr.CSRF)
	wantErr(t, e.do(req{method: "POST", path: "/api/v1/setups/1/acquire", cookie: c, csrf: "00" + csrf[2:]}), http.StatusForbidden, apierr.CSRF)
	wantErr(t, e.do(req{method: "POST", path: "/api/v1/setups/1/acquire", cookie: c, csrf: otherCSRF}), http.StatusForbidden, apierr.CSRF)
	if r := e.do(req{method: "POST", path: "/api/v1/setups/1/acquire", cookie: c, csrf: csrf}); r.Code != http.StatusOK {
		t.Fatalf("POST with CSRF: %d", r.Code)
	}
	// The token served by the session endpoint is the one checked.
	v := decodeView(t, e.do(req{method: "GET", path: "/api/v1/session", cookie: c}))
	if v.CSRFToken != csrf || len(csrf) != 64 {
		t.Fatalf("session csrf %q", v.CSRFToken)
	}
	wantErr(t, e.do(req{method: "POST", path: "/auth/logout", cookie: c}), http.StatusForbidden, apierr.CSRF)
}

func TestAUTH3_Logout(t *testing.T) {
	e := newEnv(t, Config{})
	u := e.user("sub-1", store.StatusPending) // any session, even without a role
	c, csrf := e.login(u.ID)
	rec := e.do(req{method: "POST", path: "/auth/logout", cookie: c, csrf: csrf})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout %d", rec.Code)
	}
	if ck := cookieNamed(rec, SessionCookie); ck == nil || ck.MaxAge >= 0 || ck.Value != "" {
		t.Fatalf("cookie not cleared: %+v", ck)
	}
	wantErr(t, e.do(req{method: "GET", path: "/api/v1/session", cookie: c}), http.StatusUnauthorized, apierr.Unauthenticated)
	wantErr(t, e.do(req{method: "POST", path: "/auth/logout"}), http.StatusUnauthorized, apierr.Unauthenticated)
}

func TestAUTH3_BadCookieIsAnonymous(t *testing.T) {
	e := newEnv(t, Config{})
	for _, c := range []string{"garbage", strings.Repeat("A", 43), "x.y"} {
		wantErr(t, e.do(req{method: "GET", path: "/api/v1/session", cookie: c}), http.StatusUnauthorized, apierr.Unauthenticated)
	}
}

func TestAUTH5_SessionOfUnassignedUser(t *testing.T) {
	e := newEnv(t, Config{})
	u := e.user("sub-1", store.StatusPending)
	c, csrf := e.login(u.ID)
	rec := e.do(req{method: "GET", path: "/api/v1/session", cookie: c})
	if rec.Code != http.StatusOK {
		t.Fatalf("%d", rec.Code)
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	want := []string{"user_id", "username", "email", "name", "auth_source", "status", "roles", "csrf_token", "must_change_password", "is_local_admin"}
	if len(raw) != len(want) {
		t.Fatalf("fields %v", raw)
	}
	for _, k := range want {
		if _, ok := raw[k]; !ok {
			t.Fatalf("missing %s", k)
		}
	}
	if string(raw["roles"]) != "[]" || decodeView(t, rec).CSRFToken != csrf {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestAUTH4_RoleMiddlewareMatrix(t *testing.T) {
	e := newEnv(t, Config{})
	type who struct {
		name   string
		cookie string
		csrf   string
	}
	mk := func(name, status string, roles ...string) who {
		u := e.user(name, status, roles...)
		c, csrf := e.login(u.ID)
		return who{name, c, csrf}
	}
	anon := who{name: "anonymous"}
	pending := mk("pending", store.StatusPending)
	rejected := mk("rejected", store.StatusRejected)
	developer := mk("developer", store.StatusActive, store.RoleDeveloper)
	admin := mk("admin", store.StatusActive, store.RoleAdmin)
	both := mk("both", store.StatusActive, store.RoleAdmin, store.RoleDeveloper)
	activeNoRole := mk("active-no-role", store.StatusActive)

	type cell struct {
		status int
		code   string
	}
	ok := cell{status: http.StatusOK}
	unauth := cell{http.StatusUnauthorized, apierr.Unauthenticated}
	noRole := cell{http.StatusForbidden, apierr.NoRole}
	forbidden := cell{http.StatusForbidden, apierr.Forbidden}
	routes := []struct{ method, path string }{
		{"GET", "/api/v1/session"},           // session route
		{"GET", "/api/v1/me"},                // role route
		{"POST", "/api/v1/setups/7/acquire"}, // role route, state-changing
		{"GET", "/api/v1/admin/users"},       // admin route
		{"PUT", "/api/v1/admin/settings"},    // admin route, state-changing
		{"GET", "/api/v1/auth/methods"},      // public
	}
	matrix := map[string][]cell{
		"anonymous":      {unauth, unauth, unauth, unauth, unauth, ok},
		"pending":        {ok, noRole, noRole, noRole, noRole, ok},
		"rejected":       {ok, noRole, noRole, noRole, noRole, ok},
		"active-no-role": {ok, noRole, noRole, noRole, noRole, ok},
		"developer":      {ok, ok, ok, forbidden, forbidden, ok},
		"admin":          {ok, ok, ok, ok, ok, ok},
		"both":           {ok, ok, ok, ok, ok, ok},
	}
	for _, w := range []who{anon, pending, rejected, activeNoRole, developer, admin, both} {
		for i, rt := range routes {
			want := matrix[w.name][i]
			rec := e.do(req{method: rt.method, path: rt.path, cookie: w.cookie, csrf: w.csrf})
			if want.code == "" {
				if rec.Code != want.status {
					t.Errorf("%s %s %s: %d, want %d", w.name, rt.method, rt.path, rec.Code, want.status)
				}
				continue
			}
			if rec.Code != want.status || errCode(t, rec) != want.code || strings.Contains(rec.Body.String(), "secret-data") {
				t.Errorf("%s %s %s: %d %s, want %d %s", w.name, rt.method, rt.path, rec.Code, rec.Body.String(), want.status, want.code)
			}
		}
	}
	// A pending user later given a role but not yet active still sees nothing.
	pu := e.user("pending-with-role", store.StatusPending, store.RoleDeveloper)
	c, _ := e.login(pu.ID)
	wantErr(t, e.do(req{method: "GET", path: "/api/v1/me", cookie: c}), http.StatusForbidden, apierr.NoRole)
}

func TestAUTH_MethodsLocalOnlyWithoutOIDC(t *testing.T) {
	e := newEnv(t, Config{})
	rec := e.do(req{method: "GET", path: "/api/v1/auth/methods"})
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"oidc":false,"local":true}` {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, e.do(req{method: "GET", path: "/auth/oidc/start"}), http.StatusNotFound, apierr.NotFound)
	wantErr(t, e.do(req{method: "GET", path: "/auth/callback?code=x&state=y"}), http.StatusNotFound, apierr.NotFound)
}

func TestUI4_CORS(t *testing.T) {
	t.Run("off", func(t *testing.T) {
		e := newEnv(t, Config{})
		for _, m := range []string{"GET", "OPTIONS"} {
			rec := e.do(req{method: m, path: "/api/v1/auth/methods", header: map[string]string{
				"Origin": "http://localhost:5173", "Access-Control-Request-Method": "POST"}})
			for k := range rec.Header() {
				if strings.HasPrefix(k, "Access-Control-") || k == "Vary" {
					t.Fatalf("%s: CORS header %s sent with an empty list", m, k)
				}
			}
		}
	})
	t.Run("on", func(t *testing.T) {
		e := newEnv(t, Config{AllowedOrigins: []string{"http://localhost:5173"}})
		rec := e.do(req{method: "GET", path: "/api/v1/auth/methods", header: map[string]string{"Origin": "http://localhost:5173"}})
		if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" || rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Fatalf("headers %v", rec.Header())
		}
		pre := e.do(req{method: "OPTIONS", path: "/api/v1/setups/1/acquire", header: map[string]string{
			"Origin": "http://localhost:5173", "Access-Control-Request-Method": "POST", "Access-Control-Request-Headers": "x-csrf-token"}})
		if pre.Code != http.StatusNoContent || !strings.Contains(pre.Header().Get("Access-Control-Allow-Headers"), CSRFHeader) ||
			pre.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
			t.Fatalf("preflight %d %v", pre.Code, pre.Header())
		}
		bad := e.do(req{method: "GET", path: "/api/v1/auth/methods", header: map[string]string{"Origin": "https://evil.example"}})
		if bad.Header().Get("Access-Control-Allow-Origin") != "" || bad.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Fatalf("unlisted origin allowed: %v", bad.Header())
		}
	})
}

func TestAUTH3_SessionKeyRules(t *testing.T) {
	st := newStore(t)
	if _, err := New(Config{SessionKey: "too-short"}, st, nil, nil); err == nil || strings.Contains(err.Error(), "too-short") {
		t.Fatalf("short key: %v", err)
	}
	logs := &syncBuffer{}
	s, err := New(Config{}, st, nil, slogTo(logs))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "sessions will not survive a restart") {
		t.Fatalf("no warning: %s", logs.String())
	}
	s2, _ := New(Config{}, st, nil, nil)
	if s.CSRFToken("id") == s2.CSRFToken("id") {
		t.Fatal("generated keys are not random")
	}
	if _, err := New(Config{OIDC: OIDCConfig{IssuerURL: "https://idp.example"}}, st, nil, nil); err == nil {
		t.Fatal("incomplete OIDC config accepted")
	}
}
