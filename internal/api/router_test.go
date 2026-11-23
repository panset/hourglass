// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/store"
)

func concretePath(p string) string {
	return strings.NewReplacer("{id}", "1", "{tokenId}", "1").Replace(p)
}

// seed creates setup 1, hold 1 (held by a developer), service account 1 and
// its token 1, so that every id of concretePath names something that exists.
func (e *env) seed() {
	e.t.Helper()
	ctx := context.Background()
	if id, err := e.st.CreateSetup(ctx, store.Setup{Name: "lab-01", Enabled: true, Labels: map[string]string{}, DetailsJSON: `{}`}); err != nil || id != 1 {
		e.t.Fatalf("seed setup: %d %v", id, err)
	}
	holder := e.user("holder", store.StatusActive, store.RoleDeveloper)
	if h, err := e.st.CreateHold(ctx, 1, holder.user.ID, e.clk.Now()); err != nil || h.ID != 1 {
		e.t.Fatalf("seed hold: %d %v", h.ID, err)
	}
	if a, err := e.st.CreateServiceAccount(ctx, store.ServiceAccount{Name: "svc-01", Scopes: []string{}, Enabled: true}, e.clk.Now()); err != nil || a.ID != 1 {
		e.t.Fatalf("seed service account: %d %v", a.ID, err)
	}
	if tok, err := e.st.InsertServiceToken(ctx, 1, "seed-hash", nil, e.clk.Now()); err != nil || tok.ID != 1 {
		e.t.Fatalf("seed service token: %d %v", tok.ID, err)
	}
}

func TestAPI_RouteTableIsConsistent(t *testing.T) {
	seen := map[string]bool{}
	ops := map[string]bool{}
	for _, r := range Routes() {
		k := r.Method + " " + r.Pattern
		if seen[k] || ops[r.OperationID] {
			t.Errorf("duplicate route %s / %s", k, r.OperationID)
		}
		seen[k], ops[r.OperationID] = true, true
	}
	if len(Routes()) != 40 {
		t.Errorf("routes = %d, want the 40 operations of api/openapi.yaml", len(Routes()))
	}
	// Every non-auth operation has a handler, and the handler map has nothing extra.
	hs := (&API{}).handlers()
	for _, r := range Routes() {
		if _, ok := hs[r.OperationID]; !ok && !authServed[r.OperationID] {
			t.Errorf("operation %s has no handler", r.OperationID)
		}
	}
	for op := range hs {
		if !ops[op] {
			t.Errorf("handler %s is not in the route table", op)
		}
	}
	// Mutating the returned slice must not change the table.
	rs := Routes()
	rs[0].Pattern = "/changed"
	if Routes()[0].Pattern == "/changed" {
		t.Error("Routes returns the table itself")
	}
}

func TestAPI_EveryRouteAnswersSomethingOtherThan404(t *testing.T) {
	// An issuer that is not there: /auth/oidc/start fails, but is a route.
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	e := newEnv(t, envOpts{oidcIssuer: dead.URL})
	e.seed()
	admin := e.adminSession(true)
	for _, r := range Routes() {
		path := concretePath(r.Pattern)
		for _, s := range []*session{nil, &admin} {
			rec := e.do(r.Method, path, s, true, "")
			if rec.Code == http.StatusNotFound {
				t.Errorf("%s %s (session=%v) answered 404: %s", r.Method, path, s != nil, rec.Body)
			}
		}
	}
}

func TestAUTH4_AdminSessionReachesEveryRealHandler(t *testing.T) {
	// An issuer that is not there, so the OIDC routes exist.
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	e := newEnv(t, envOpts{oidcIssuer: dead.URL})
	e.seed()
	e.adminSession(true) // clears the forced password change
	for _, r := range Routes() {
		admin := e.session(e.admin) // a fresh session each time: POST /auth/logout ends one
		rec := e.do(r.Method, concretePath(r.Pattern), &admin, true, "")
		switch {
		case rec.Code == http.StatusNotImplemented, rec.Code == http.StatusNotFound:
			t.Errorf("%s %s = %d %s, want a real handler's answer", r.Method, r.Pattern, rec.Code, rec.Body)
		case r.Access == AccessRole || r.Access == AccessAdmin:
			// The admin passes the middleware: never 401/403 for a role or
			// admin route, and no server error from an empty request.
			if rec.Code == http.StatusUnauthorized || rec.Code >= 500 {
				t.Errorf("%s %s = %d %s", r.Method, r.Pattern, rec.Code, rec.Body)
			}
			if rec.Code == http.StatusForbidden && rec.errCode() != apierr.Forbidden {
				t.Errorf("%s %s = 403 %s: the admin was stopped by the middleware", r.Method, r.Pattern, rec.Body)
			}
		}
	}
}

func TestAUTH4_UnauthenticatedGets401OnEveryGuardedRoute(t *testing.T) {
	e := newEnv(t, envOpts{})
	for _, r := range Routes() {
		if r.Access == AccessPublic {
			continue
		}
		rec := e.do(r.Method, concretePath(r.Pattern), nil, true, "")
		if rec.Code != http.StatusUnauthorized || rec.errCode() != apierr.Unauthenticated {
			t.Errorf("%s %s = %d %s, want 401 unauthenticated", r.Method, r.Pattern, rec.Code, rec.Body)
		}
	}
}

func TestAUTH4_UnknownAPIPathIsJSONNotFound(t *testing.T) {
	e := newEnv(t, envOpts{})
	dev := e.user("dev", store.StatusActive, store.RoleDeveloper)
	paths := []string{"/api/v1/x", "/api/", "/api/v2/me", "/api/v1/me/extra", "/auth/nothing", "/auth/", "/api/v1/admin/nothing"}
	for _, p := range paths {
		for _, m := range []string{"GET", "POST", "DELETE"} {
			for _, s := range []*session{nil, &dev} {
				rec := e.do(m, p, s, true, "")
				ct := rec.Header().Get("Content-Type")
				if rec.Code != http.StatusNotFound || rec.errCode() != apierr.NotFound || !strings.HasPrefix(ct, "application/json") {
					t.Errorf("%s %s = %d %q %s, want JSON not_found", m, p, rec.Code, ct, rec.Body)
				}
			}
		}
	}
	// A known path with a method the table does not list is also not_found JSON.
	if rec := e.do("DELETE", "/api/v1/me", &dev, true, ""); rec.errCode() != apierr.NotFound {
		t.Errorf("DELETE /api/v1/me = %d %s", rec.Code, rec.Body)
	}
}

func TestUI3_OtherPathsGoToFallbackOrPlain404(t *testing.T) {
	e := newEnv(t, envOpts{})
	rec := e.do("GET", "/some/page", nil, false, "")
	if rec.Code != 404 || strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Errorf("no fallback: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	fb := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>ui " + r.URL.Path)) })
	e = newEnv(t, envOpts{fallback: fb})
	if rec := e.do("GET", "/some/page", nil, false, ""); rec.Code != 200 || rec.Body.String() != "<html>ui /some/page" {
		t.Errorf("fallback: %d %s", rec.Code, rec.Body)
	}
	if rec := e.do("GET", "/", nil, false, ""); rec.Body.String() != "<html>ui /" {
		t.Errorf("fallback root: %s", rec.Body)
	}
	// The fallback never sees an API path.
	if rec := e.do("GET", "/api/v1/zzz", nil, false, ""); rec.errCode() != apierr.NotFound {
		t.Errorf("api path reached the fallback: %s", rec.Body)
	}
	if rec := e.do("GET", "/healthz", nil, false, ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"database"`) {
		t.Errorf("healthz shadowed by fallback: %d %s", rec.Code, rec.Body)
	}
}

func TestAUTH4_MiddlewareOrder401Then403ThenPasswordThenCSRF(t *testing.T) {
	e := newEnv(t, envOpts{})
	e.seed()
	dev := e.user("dev", store.StatusActive, store.RoleDeveloper)
	none := e.user("none", store.StatusActive)
	pending := e.user("pend", store.StatusPending)
	mustChange := e.adminSession(false)
	const adminPost = "/api/v1/admin/setups"
	const rolePost = "/api/v1/setups/1/acquire"

	check := func(name string, rec reply, status int, code string) {
		t.Helper()
		if rec.Code != status || (code != "" && rec.errCode() != code) {
			t.Errorf("%s: %d %s, want %d %s", name, rec.Code, rec.Body, status, code)
		}
	}
	// No session: 401 even without a CSRF header (401 before 403).
	check("no session, no csrf", e.do("POST", adminPost, nil, false, ""), 401, apierr.Unauthenticated)
	// No role: no_role, before CSRF is looked at.
	check("no role, no csrf", e.do("POST", rolePost, &none, false, ""), 403, apierr.NoRole)
	check("pending, no csrf", e.do("POST", rolePost, &pending, false, ""), 403, apierr.NoRole)
	check("no role on admin route", e.do("POST", adminPost, &none, false, ""), 403, apierr.NoRole)
	// Developer on an admin route: forbidden, again before CSRF.
	check("developer admin route, no csrf", e.do("POST", adminPost, &dev, false, ""), 403, apierr.Forbidden)
	check("developer admin route, csrf", e.do("POST", adminPost, &dev, true, ""), 403, apierr.Forbidden)
	// Forced password change wins over CSRF.
	check("must change, no csrf", e.do("POST", adminPost, &mustChange, false, ""), 403, apierr.PasswordChangeRequired)
	check("must change GET", e.do("GET", "/api/v1/admin/audit", &mustChange, true, ""), 403, apierr.PasswordChangeRequired)
	// CSRF is checked on every state-changing method, not on GET.
	check("developer POST, no csrf", e.do("POST", rolePost, &dev, false, ""), 403, apierr.CSRF)
	wrong := dev
	wrong.csrf = strings.Repeat("0", 64)
	check("developer POST, wrong csrf", e.do("POST", rolePost, &wrong, true, ""), 403, apierr.CSRF)
	// With CSRF the request reaches the real handler: setup 1 is held by someone else.
	check("developer POST, csrf", e.do("POST", rolePost, &dev, true, ""), 409, apierr.SetupHeld)
	check("developer GET, no csrf needed", e.do("GET", "/api/v1/me", &dev, false, ""), 200, "")
	admin := e.adminSession(true)
	check("admin POST no csrf", e.do("POST", adminPost, &admin, false, ""), 403, apierr.CSRF)
	for _, m := range []string{"PUT", "DELETE"} {
		check("admin "+m+" no csrf", e.do(m, "/api/v1/admin/setups/1", &admin, false, ""), 403, apierr.CSRF)
	}
	check("admin PUT csrf", e.do("PUT", "/api/v1/admin/setups/1", &admin, true, ""), 400, apierr.Validation)
}

func TestAUTH5_UnassignedUserSessionHasEmptyRoleList(t *testing.T) {
	e := newEnv(t, envOpts{})
	pending := e.user("newcomer", store.StatusPending)
	rec := e.do("GET", "/api/v1/session", &pending, false, "")
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if string(raw["roles"]) != "[]" {
		t.Errorf("roles = %s, want []", raw["roles"])
	}
	// Exactly the Session schema's properties, nothing else (no data leaks).
	var keys []string
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := "auth_source,csrf_token,display_name,email,is_local_admin,must_change_password,name,roles,status,user_id,username"
	if got := strings.Join(keys, ","); got != want {
		t.Errorf("session keys = %s, want %s", got, want)
	}
	var s struct {
		Status string
		Email  string
		CSRF   string `json:"csrf_token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &s)
	if s.Status != "pending" || s.Email != "newcomer@example.test" || s.CSRF == "" {
		t.Errorf("session %s", rec.Body)
	}
	if rec := e.do("GET", "/api/v1/session", nil, false, ""); rec.Code != 401 {
		t.Errorf("no session: %d", rec.Code)
	}
}

func TestAUTH4_UnassignedUserGetsNoDataFromAnyRoute(t *testing.T) {
	e := newEnv(t, envOpts{})
	pending := e.user("p", store.StatusPending)
	none := e.user("n", store.StatusActive)
	for _, s := range []session{pending, none} {
		for _, r := range Routes() {
			if r.Access != AccessRole && r.Access != AccessAdmin {
				continue
			}
			s := s
			rec := e.do(r.Method, concretePath(r.Pattern), &s, true, `{}`)
			want := `{"error":{"code":"no_role","message":"you have no role"}}` + "\n"
			if rec.Code != 403 || rec.Body.String() != want {
				t.Errorf("%s %s = %d %q", r.Method, r.Pattern, rec.Code, rec.Body)
			}
		}
	}
}

func TestAUTH6_AuthMethodsShape(t *testing.T) {
	e := newEnv(t, envOpts{})
	rec := e.do("GET", "/api/v1/auth/methods", nil, false, "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"oidc":false,"local":true}` {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
	e = newEnv(t, envOpts{oidcIssuer: "http://idp.invalid"})
	rec = e.do("GET", "/api/v1/auth/methods", nil, false, "")
	if strings.TrimSpace(rec.Body.String()) != `{"oidc":true,"local":true}` {
		t.Errorf("%s", rec.Body)
	}
}

func TestNFR_NoStoreOnEveryAPIResponse(t *testing.T) {
	e := newEnv(t, envOpts{})
	dev := e.user("dev", store.StatusActive, store.RoleDeveloper)
	for _, c := range []struct {
		m, p string
		s    *session
	}{
		{"GET", "/healthz", nil},
		{"GET", "/api/v1/auth/methods", nil},
		{"GET", "/api/v1/session", &dev},
		{"GET", "/api/v1/session", nil},
		{"GET", "/api/v1/me", &dev},
		{"GET", "/api/v1/me", nil},
		{"POST", "/api/v1/setups/1/acquire", &dev},
		{"GET", "/api/v1/nope", nil},
		{"GET", "/auth/nope", nil},
		{"POST", "/auth/local", nil},
	} {
		rec := e.do(c.m, c.p, c.s, false, "")
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
			t.Errorf("%s %s (%d): Cache-Control = %q", c.m, c.p, rec.Code, cc)
		}
	}
}

func TestUI4_CORSOnlyForListedOrigins(t *testing.T) {
	e := newEnv(t, envOpts{origins: []string{"http://localhost:5173"}})
	pre := func(origin string) reply {
		return e.do("OPTIONS", "/api/v1/admin/setups", nil, false, "",
			"Origin", origin, "Access-Control-Request-Method", "POST")
	}
	rec := pre("http://localhost:5173")
	if rec.Code != 204 || rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" ||
		rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("listed origin: %d %v", rec.Code, rec.Header())
	}
	if rec := pre("http://evil.test"); rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("unlisted origin got CORS headers: %v", rec.Header())
	}
	rec = e.do("GET", "/healthz", nil, false, "", "Origin", "http://localhost:5173")
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("simple request: %v", rec.Header())
	}
	e = newEnv(t, envOpts{})
	if rec := e.do("GET", "/healthz", nil, false, "", "Origin", "http://localhost:5173"); rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("default config sends CORS headers: %v", rec.Header())
	}
}

func TestNFR_RecovererReturnsStandardInternalError(t *testing.T) {
	e := newEnv(t, envOpts{})
	a := &API{d: Deps{Logger: newLogger(e.logs)}}
	h := a.recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom secret-value-123")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/x", nil))
	if rec.Code != 500 || strings.TrimSpace(rec.Body.String()) != `{"error":{"code":"internal","message":"internal error"}}` {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "secret-value-123") || strings.Contains(e.logs.String(), "secret-value-123") {
		t.Error("panic value leaked into the response or the log")
	}
	if !strings.Contains(e.logs.String(), "handler panic") {
		t.Errorf("panic not logged: %s", e.logs)
	}
	// A panic after the header was written does not append a second body.
	h = a.recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		panic("late")
	}))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 201 || rec.Body.Len() != 0 {
		t.Errorf("late panic: %d %q", rec.Code, rec.Body)
	}
}

func TestNFR_RequestLogHasNoBodyCookieTokenOrQuery(t *testing.T) {
	e := newEnv(t, envOpts{})
	e.seed()
	dev := e.user("dev", store.StatusActive, store.RoleDeveloper)
	rec := e.do("POST", "/api/v1/setups/1/acquire?code=QUERYSECRET", &dev, true, `{"password":"BODYSECRET"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("acquire of a held setup = %d %s", rec.Code, rec.Body)
	}
	out := e.logs.String()
	if !strings.Contains(out, `"path":"/api/v1/setups/1/acquire"`) || !strings.Contains(out, `"status":409`) || !strings.Contains(out, `"method":"POST"`) {
		t.Fatalf("request not logged: %s", out)
	}
	for _, s := range []string{"QUERYSECRET", "BODYSECRET", dev.cookie, dev.csrf, "hourglass_session"} {
		if strings.Contains(out, s) {
			t.Errorf("log contains %q", s)
		}
	}
}

func TestAUTH6_LocalSignInRecordsLastLogin(t *testing.T) {
	e := newEnv(t, envOpts{})
	ctx := context.Background()
	body := func(pw string) string { return `{"username":"admin","password":"` + pw + `"}` }
	if rec := e.do("POST", "/auth/local", nil, false, body("wrong wrong wrong")); rec.Code != 401 {
		t.Fatalf("wrong password: %d %s", rec.Code, rec.Body)
	}
	if u, _ := e.st.GetUser(ctx, e.admin.ID); u.LastLoginAt != nil {
		t.Fatalf("failed sign-in set last_login_at: %v", u.LastLoginAt)
	}
	before := e.clk.Now().Add(-time.Second) // the router stamps with its (fake) clock
	rec := e.do("POST", "/auth/local", nil, false, body(testAdminPassword))
	if rec.Code != 200 {
		t.Fatalf("sign-in: %d %s", rec.Code, rec.Body)
	}
	u, _ := e.st.GetUser(ctx, e.admin.ID)
	if u.LastLoginAt == nil || u.LastLoginAt.Before(before) {
		t.Fatalf("last_login_at = %v", u.LastLoginAt)
	}
}

func TestHealthz_ReportsDatabaseAndPollAge(t *testing.T) {
	e := newEnv(t, envOpts{})
	ctx := context.Background()
	type health struct {
		Status   string
		Database string
		Age      *int64 `json:"last_poll_age_seconds"`
	}
	get := func() (int, health, string) {
		rec := e.do("GET", "/healthz", nil, false, "")
		var h health
		_ = json.Unmarshal(rec.Body.Bytes(), &h)
		return rec.Code, h, rec.Body.String()
	}
	code, h, raw := get()
	if code != 200 || h.Status != "ok" || h.Database != "ok" || h.Age != nil || !strings.Contains(raw, `"last_poll_age_seconds":null`) {
		t.Fatalf("before first poll: %d %s", code, raw)
	}
	// A failed run does not count.
	id, _ := e.st.StartCollectorRun(ctx, e.clk.Now().Add(-time.Minute))
	_ = e.st.FinishCollectorRun(ctx, id, e.clk.Now(), false, "boom")
	if _, h, _ := get(); h.Age != nil {
		t.Errorf("failed poll counted: %v", *h.Age)
	}
	id, _ = e.st.StartCollectorRun(ctx, e.clk.Now().Add(-90*time.Second))
	_ = e.st.FinishCollectorRun(ctx, id, e.clk.Now().Add(-89*time.Second), true, "")
	code, h, raw = get()
	if code != 200 || h.Age == nil || *h.Age < 89 || *h.Age > 95 {
		t.Fatalf("after poll: %d %s", code, raw)
	}
	if strings.Contains(raw, "user") || strings.Contains(raw, "setup") {
		t.Errorf("health body reveals data: %s", raw)
	}
	e.st.Close()
	code, h, raw = get()
	if code != 503 || h.Status != "degraded" || h.Database != "error" {
		t.Fatalf("database down: %d %s", code, raw)
	}
}
