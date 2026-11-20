// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/store"
)

// Tests of the service-account requirements SVC-1 to SVC-8
// (docs/design/integration-api.md), through the real router and store.

func serviceRoutes() []Route {
	var out []Route
	for _, r := range Routes() {
		if r.Access == AccessService {
			out = append(out, r)
		}
	}
	return out
}

// newRoutes are the routes the integration API added: the six admin routes
// and the four integration routes.
func newRoutes() []Route {
	var out []Route
	for _, r := range Routes() {
		if r.Access == AccessService || strings.HasPrefix(r.Pattern, "/api/v1/admin/service-accounts") {
			out = append(out, r)
		}
	}
	return out
}

func decodeMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %q: %v", rec.Body, err)
	}
	return m
}

func lastUsed(t *testing.T, st *store.Store, tokenID int64) *time.Time {
	t.Helper()
	tok, _, err := st.GetServiceTokenWithAccount(context.Background(), tokenID)
	if err != nil {
		t.Fatal(err)
	}
	return tok.LastUsedAt
}

// SVC-1: outside /api/v1/integration/ a service token is ignored. On every
// other route a token with all four scopes gets exactly what an anonymous
// caller gets, the token is not even looked up (last_used_at stays empty),
// and a wrong token there does not count as a failed attempt.
func TestSVC1_ServiceTokenIsAnonymousOutsideIntegrationRoutes(t *testing.T) {
	f := newPermFix(t)
	fresh := f.mintServiceAccount("svc1-all", allScopes()...)
	anon := f.actor(actAnonymous)
	n := 0
	for _, r := range Routes() {
		if r.Access == AccessService {
			continue
		}
		p := f.probe(r.OperationID, anon, modeDefault)
		// The same request twice: once anonymous, once with the token. Probes
		// create their own targets, so build the second from the same probe.
		want := f.serve(p.method, p.path, nil, false, p.body)
		got := f.serve(p.method, p.path, nil, false, p.body, bearer(fresh.token)...)
		n++
		if got.Code != want.Code {
			t.Errorf("%s %s: with the token %d, anonymous %d: %s", r.Method, r.Pattern, got.Code, want.Code, got.Body)
			continue
		}
		if r.OperationID == "oidcStart" || r.OperationID == "oidcCallback" {
			continue // redirects carry a fresh random state each time
		}
		if got.Body.String() != want.Body.String() {
			t.Errorf("%s %s: body with the token %s, anonymous %s", r.Method, r.Pattern, got.Body, want.Body)
		}
		if got.Header().Get("Set-Cookie") != want.Header().Get("Set-Cookie") {
			t.Errorf("%s %s: Set-Cookie differs with the token", r.Method, r.Pattern)
		}
	}
	if n == 0 {
		t.Fatal("no pre-existing route was exercised")
	}
	if lu := lastUsed(t, f.st, fresh.tokID); lu != nil {
		t.Errorf("the token was looked up on a non-integration route: last_used_at = %v", lu)
	}
	// Wrong tokens outside the integration routes are not failed attempts.
	for i := 0; i < 10; i++ {
		f.serve("GET", "/api/v1/me", nil, false, "", bearer("hgsa_1_"+strings.Repeat("0", 64))...)
	}
	if rec := f.serveService("GET", "/api/v1/integration/state", fresh.token, ""); rec.Code != http.StatusNotImplemented {
		t.Errorf("valid token on an integration route after ignored attempts: %d %s", rec.Code, rec.Body)
	}
}

// SVC-2: a session cookie, an admin's included, does not authenticate on the
// integration routes, with or without the CSRF token; a cookie next to a
// token adds nothing; and the routes need no CSRF token.
func TestSVC2_SessionCookieDoesNotAuthenticateOnIntegrationRoutes(t *testing.T) {
	f := newPermFix(t)
	weak := f.mintServiceAccount("svc2-none")
	for _, r := range serviceRoutes() {
		for _, name := range []string{actAdmin, actLocal, actDeveloper} {
			a := f.actor(name)
			f.setMustChange(false)
			p := f.probe(r.OperationID, a, modeDefault)
			for _, csrf := range []bool{true, false} {
				rec := f.serve(p.method, p.path, a.s, csrf, p.body)
				permWantDenied(t, rec, permDenial{http.StatusUnauthorized, apierr.Unauthenticated})
			}
			// The admin's cookie does not lift a token without the scope.
			rec := f.serve(p.method, p.path, a.s, true, p.body, bearer(weak.token)...)
			permWantDenied(t, rec, permDenial{http.StatusForbidden, apierr.Forbidden})
		}
		// No CSRF token needed: the token alone reaches the handler.
		p := f.probe(r.OperationID, f.actor(actSvcScoped), modeDefault)
		rec := f.serveService(p.method, p.path, f.svcAll.token, p.body)
		if rec.Code != http.StatusNotImplemented {
			t.Errorf("%s %s with the token and no CSRF: %d %s", r.Method, r.Pattern, rec.Code, rec.Body)
		}
		if r.StateChanging() {
			t.Errorf("%s %s is marked state-changing (CSRF) but reads no cookie", r.Method, r.Pattern)
		}
	}
}

// SVC-3: each integration route requires its own scope. A token holding every
// other scope gets 403 forbidden, with one fixed body for every route that
// names no scope and carries no data; the token with the scope gets in.
func TestSVC3_EachIntegrationRouteRequiresItsScope(t *testing.T) {
	f := newPermFix(t)
	var body string
	for _, r := range serviceRoutes() {
		if !auth.ValidScope(r.Scope) {
			t.Fatalf("%s %s has scope %q", r.Method, r.Pattern, r.Scope)
		}
		p := f.probe(r.OperationID, f.actor(actSvcNoScope), modeDefault)
		rec := f.serveService(p.method, p.path, f.svcWithout[r.Scope].token, p.body)
		permWantDenied(t, rec, permDenial{http.StatusForbidden, apierr.Forbidden})
		if body == "" {
			body = rec.Body.String()
		} else if rec.Body.String() != body {
			t.Errorf("%s %s: refusal %s differs from %s", r.Method, r.Pattern, rec.Body, body)
		}
		low := strings.ToLower(rec.Body.String())
		for _, sc := range allScopes() {
			if strings.Contains(low, sc) || strings.Contains(low, strings.Split(sc, ":")[0]) {
				t.Errorf("%s %s: the refusal names a scope: %s", r.Method, r.Pattern, rec.Body)
			}
		}
		if m := permContainsMarker(rec); m != "" {
			t.Errorf("%s %s: the refusal shows %q", r.Method, r.Pattern, m)
		}
		// Only the route's own scope opens it.
		only := f.mintServiceAccount("svc3-only-"+r.OperationID, r.Scope)
		if rec := f.serveService(p.method, p.path, only.token, p.body); rec.Code != http.StatusNotImplemented {
			t.Errorf("%s %s with only %s: %d %s", r.Method, r.Pattern, r.Scope, rec.Code, rec.Body)
		}
	}
	// The four routes use the four scopes, one each.
	seen := map[string]bool{}
	for _, r := range serviceRoutes() {
		seen[r.Scope] = true
	}
	if len(seen) != len(allScopes()) || len(serviceRoutes()) != len(allScopes()) {
		t.Errorf("scopes in use %v over %d routes, want each of %v once", seen, len(serviceRoutes()), allScopes())
	}
}

var tokenRE = regexp.MustCompile(`^hgsa_([0-9]+)_([0-9a-f]{64})$`)

// SVC-4: the token is in the issue response only. The store holds the
// SHA-256 of the secret; the listing, later responses, error bodies, the
// server log and the audit rows hold neither the token nor its secret (nor
// the hash).
func TestSVC4_TokenIsShownOnceAndNeverLoggedOrAudited(t *testing.T) {
	b := newBackend(t, backendOpts{start: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)})
	adm := b.newUser("boss", store.StatusActive, store.RoleAdmin)
	ctx := context.Background()

	rec := b.serve("POST", "/api/v1/admin/service-accounts", &adm, true, `{"name":"deployer","scopes":["state:read"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	aid := int64(decodeMap(t, rec)["id"].(float64))
	path := fmt.Sprintf("/api/v1/admin/service-accounts/%d", aid)
	rec = b.serve("POST", path+"/tokens", &adm, true, `{"expires_at":"2027-06-01T00:00:00Z"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("issue: %d %s", rec.Code, rec.Body)
	}
	issued := decodeMap(t, rec)
	token, _ := issued["token"].(string)
	m := tokenRE.FindStringSubmatch(token)
	if m == nil || m[1] != fmt.Sprint(int64(issued["id"].(float64))) {
		t.Fatalf("issued token %q does not have the form hgsa_<token id>_<secret>", token)
	}
	secret := m[2]
	tokID := int64(issued["id"].(float64))

	row, _, err := b.st.GetServiceTokenWithAccount(ctx, tokID)
	if err != nil {
		t.Fatal(err)
	}
	if row.TokenHash != auth.HashServiceSecret(secret) {
		t.Errorf("stored %q, want the SHA-256 of the secret", row.TokenHash)
	}
	hash := row.TokenHash
	secrets := map[string]string{"token": token, "secret": secret, "hash": hash}
	noSecret := func(where, text string) {
		t.Helper()
		for what, v := range secrets {
			if strings.Contains(text, v) {
				t.Errorf("%s contains the %s", where, what)
			}
		}
	}

	// Use: accepted, refused with a wrong secret, ignored outside the
	// integration routes, refused after revocation.
	if rec := b.serveService("GET", "/api/v1/integration/state", token, ""); rec.Code != http.StatusNotImplemented {
		t.Fatalf("use: %d %s", rec.Code, rec.Body)
	}
	wrong := token[:len(token)-1] + map[bool]string{true: "1", false: "0"}[strings.HasSuffix(token, "0")]
	noSecret("a refusal body", b.serveService("GET", "/api/v1/integration/state", wrong, "").Body.String())
	noSecret("a non-integration response", b.serve("GET", "/api/v1/me", nil, false, "", bearer(token)...).Body.String())
	noSecret("a validation error", b.serveService("PUT", "/api/v1/integration/pressure", token, `{"value":`).Body.String())

	// Later responses: the listing and the account update.
	rec = b.serve("GET", "/api/v1/admin/service-accounts", &adm, false, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"tokens":[{"id":`+fmt.Sprint(tokID)) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	noSecret("the listing", rec.Body.String())
	rec = b.serve("PUT", path, &adm, true, `{"description":"x","scopes":["state:read"],"enabled":true}`)
	noSecret("the update response", rec.Body.String())
	rec = b.serve("DELETE", fmt.Sprintf("%s/tokens/%d", path, tokID), &adm, true, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body)
	}
	noSecret("a refusal after revocation", b.serveService("GET", "/api/v1/integration/state", token, "").Body.String())
	b.serve("DELETE", path, &adm, true, "")

	// The server log and the audit rows.
	logs := b.logs.String()
	if !strings.Contains(logs, "/api/v1/integration/state") {
		t.Fatal("the request log did not capture the integration requests; the check below would prove nothing")
	}
	noSecret("the server log", logs)
	entries, err := b.st.ListAudit(ctx, store.AuditFilter{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	actions := map[string]bool{}
	for _, e := range entries {
		actions[e.Action] = true
		noSecret("audit entry "+e.Action, e.Target+" "+e.BeforeJSON+" "+e.AfterJSON+" "+e.ActorService)
	}
	for _, a := range []string{auditServiceCreate, auditServiceTokenIssue, auditServiceUpdate, auditServiceTokenRevoke, auditServiceDelete} {
		if !actions[a] {
			t.Errorf("no audit entry %s; the check above did not see it", a)
		}
	}
	rec = b.serve("GET", "/api/v1/admin/audit", &adm, false, "")
	noSecret("the audit API", rec.Body.String())
}

// SVC-5: a revoked or expired token, or a token of a disabled or deleted
// account, is refused with 401 on the next request; the change needs no
// restart, and re-enabling an account restores its tokens.
func TestSVC5_RevokedExpiredDisabledOrDeletedTokenIsRefused(t *testing.T) {
	b := newBackend(t, backendOpts{start: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)})
	adm := b.newUser("boss", store.StatusActive, store.RoleAdmin)
	works := func(token string) bool {
		t.Helper()
		rec := b.serveService("GET", "/api/v1/integration/state", token, "")
		switch rec.Code {
		case http.StatusNotImplemented:
			return true
		case http.StatusUnauthorized:
			if c := permExactError(t, rec); c != apierr.Unauthenticated {
				t.Errorf("401 with code %q", c)
			}
			return false
		}
		t.Fatalf("unexpected %d %s", rec.Code, rec.Body)
		return false
	}
	acctPath := func(a svcAccount) string { return fmt.Sprintf("/api/v1/admin/service-accounts/%d", a.acct.ID) }
	mustStatus := func(rec *httptest.ResponseRecorder, want int) {
		t.Helper()
		if rec.Code != want {
			t.Fatalf("%d %s, want %d", rec.Code, rec.Body, want)
		}
	}

	// Revoked: the other token of the same account keeps working (rotation).
	rev := b.mintServiceAccount("revoked", auth.ScopeStateRead)
	other, _ := b.mintServiceToken(rev.acct.ID, nil)
	if !works(rev.token) || !works(other) {
		t.Fatal("fresh tokens refused")
	}
	mustStatus(b.serve("DELETE", fmt.Sprintf("%s/tokens/%d", acctPath(rev), rev.tokID), &adm, true, ""), 204)
	if works(rev.token) {
		t.Error("a revoked token is accepted")
	}
	if !works(other) {
		t.Error("revoking one token refused the account's other token")
	}

	// Expired: expiry is checked against the server's clock on each request.
	exp := b.mintServiceAccount("expiring", auth.ScopeStateRead)
	past := time.Now().Add(-time.Millisecond)
	future := time.Now().Add(time.Hour)
	expired, _ := b.mintServiceToken(exp.acct.ID, &past)
	live, _ := b.mintServiceToken(exp.acct.ID, &future)
	if works(expired) {
		t.Error("an expired token is accepted")
	}
	if !works(live) {
		t.Error("a token that expires later is refused")
	}

	// Disabled, then enabled again.
	dis := b.mintServiceAccount("disabled", auth.ScopeStateRead)
	if !works(dis.token) {
		t.Fatal("fresh token refused")
	}
	mustStatus(b.serve("PUT", acctPath(dis), &adm, true, `{"description":"","scopes":["state:read"],"enabled":false}`), 200)
	if works(dis.token) {
		t.Error("a disabled account's token is accepted")
	}
	mustStatus(b.serve("PUT", acctPath(dis), &adm, true, `{"description":"","scopes":["state:read"],"enabled":true}`), 200)
	if !works(dis.token) {
		t.Error("re-enabling the account did not restore its token")
	}

	// Deleted.
	del := b.mintServiceAccount("deleted", auth.ScopeStateRead)
	if !works(del.token) {
		t.Fatal("fresh token refused")
	}
	mustStatus(b.serve("DELETE", acctPath(del), &adm, true, ""), 204)
	if works(del.token) {
		t.Error("a deleted account's token is accepted")
	}

	// A scope removed by an admin is gone on the next request too (403).
	sc := b.mintServiceAccount("rescoped", auth.ScopeStateRead)
	mustStatus(b.serve("PUT", acctPath(sc), &adm, true, `{"description":"","scopes":["holds:release"],"enabled":true}`), 200)
	permWantDenied(t, b.serveService("GET", "/api/v1/integration/state", sc.token, ""), permDenial{http.StatusForbidden, apierr.Forbidden})
}

// SVC-6: the six admin routes are admin-only (every other principal is
// refused and changes nothing) and each success writes one audit entry with
// the admin as actor, in the same request.
func TestSVC6_AccountAndTokenChangesAreAdminOnlyAndAudited(t *testing.T) {
	f := newPermFix(t)
	ctx := context.Background()
	countAudit := func() int {
		t.Helper()
		es, err := f.st.ListAudit(ctx, store.AuditFilter{Limit: 100000})
		if err != nil {
			t.Fatal(err)
		}
		return len(es)
	}
	snapshot := func() string {
		t.Helper()
		l, err := f.st.ListServiceAccounts(ctx)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(l)
		return string(raw)
	}
	wantAction := map[string]string{
		"createServiceAccount": auditServiceCreate, "updateServiceAccount": auditServiceUpdate,
		"deleteServiceAccount": auditServiceDelete, "issueServiceToken": auditServiceTokenIssue,
		"revokeServiceToken": auditServiceTokenRevoke,
	}
	for _, r := range newRoutes() {
		if r.Access == AccessService {
			continue
		}
		if r.Access != AccessAdmin {
			t.Errorf("%s %s has access %q, want admin", r.Method, r.Pattern, r.Access)
		}
		if r.Method != "GET" && !r.StateChanging() {
			t.Errorf("%s %s does not require CSRF", r.Method, r.Pattern)
		}
		for _, a := range f.actors {
			if a.isAdmin() && !a.mustChange {
				continue
			}
			p := f.probe(r.OperationID, a, modeDefault)
			before, n := snapshot(), countAudit()
			rec := f.send(a, p)
			if !permAuthRefusal(rec) {
				t.Errorf("%s %s as %s: %d %s, want a refusal", r.Method, r.Pattern, a.name, rec.Code, rec.Body)
			}
			if snapshot() != before || countAudit() != n {
				t.Errorf("%s %s as %s changed data or the audit log", r.Method, r.Pattern, a.name)
			}
		}
		for _, name := range []string{actAdmin, actLocal} {
			a := f.actor(name)
			f.setMustChange(false)
			p := f.probe(r.OperationID, a, modeDefault)
			n := countAudit()
			rec := f.send(a, p)
			if rec.Code >= 300 {
				t.Fatalf("%s %s as %s: %d %s", r.Method, r.Pattern, name, rec.Code, rec.Body)
			}
			want := wantAction[r.OperationID]
			if want == "" {
				if countAudit() != n {
					t.Errorf("%s %s (a read) wrote an audit entry", r.Method, r.Pattern)
				}
				continue
			}
			es, err := f.st.ListAudit(ctx, store.AuditFilter{Action: want, Limit: 1})
			if err != nil || len(es) == 0 {
				t.Fatalf("%s: no %s entry (%v)", r.OperationID, want, err)
			}
			e := es[0]
			if countAudit() != n+1 || e.ActorID == nil || *e.ActorID != a.s.user.ID || !strings.HasPrefix(e.Target, "service_account:") {
				t.Errorf("%s as %s: audit %+v (entries %d -> %d)", r.OperationID, name, e, n, countAudit())
			}
		}
	}
}

// SVC-7: failed bearer attempts are limited per client address. A valid token
// always authenticates, even from an address that is over the limit; only a
// further failed attempt from it gets 429 rate_limited (never a hint whether
// the token id exists), and another address is unaffected.
func TestSVC7_FailedBearerAttemptsAreRateLimitedPerAddress(t *testing.T) {
	b := newBackend(t, backendOpts{})
	good := b.mintServiceAccount("svc7", auth.ScopeStateRead)
	from := func(addr, token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/v1/integration/state", nil)
		req.RemoteAddr = addr
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		b.h.ServeHTTP(rec, req)
		return rec
	}
	bad := fmt.Sprintf("hgsa_%d_%s", good.tokID, strings.Repeat("f", 64))
	unknown := fmt.Sprintf("hgsa_%d_%s", good.tokID+1000, strings.Repeat("f", 64))
	for i := 0; i < 5; i++ {
		permWantDenied(t, from("203.0.113.9:5000", bad), permDenial{http.StatusUnauthorized, apierr.Unauthenticated})
	}
	// Over the limit: the valid token still reaches the handler.
	for i := 0; i < 3; i++ {
		if rec := from("203.0.113.9:5001", good.token); rec.Code != http.StatusNotImplemented {
			t.Errorf("valid token from an address over the limit: %d %s", rec.Code, rec.Body)
		}
	}
	// A bad token from it gets 429, whether or not its id exists.
	var bodies []string
	for _, tok := range []string{bad, unknown, "garbage"} {
		rec := from("203.0.113.9:5002", tok)
		permWantDenied(t, rec, permDenial{http.StatusTooManyRequests, apierr.RateLimited})
		if s := rec.Header().Get("Retry-After"); s == "" || s == "0" {
			t.Errorf("Retry-After = %q", s)
		}
		bodies = append(bodies, rec.Body.String())
	}
	if bodies[0] != bodies[1] || bodies[1] != bodies[2] {
		t.Errorf("429 bodies tell the tokens apart: %q", bodies)
	}
	if rec := from("203.0.113.10:5000", good.token); rec.Code != http.StatusNotImplemented {
		t.Errorf("another address: %d %s", rec.Code, rec.Body)
	}
	permWantDenied(t, from("203.0.113.10:5000", bad), permDenial{http.StatusUnauthorized, apierr.Unauthenticated})
	// Session users are not affected by the limit.
	adm := b.newUser("boss", store.StatusActive, store.RoleAdmin)
	if rec := b.serve("GET", "/api/v1/admin/service-accounts", &adm, false, ""); rec.Code != 200 {
		t.Errorf("admin while the address is limited: %d", rec.Code)
	}
}

// SVC-8: the permissions test covers every new route for every principal:
// anonymous, unassigned, developer, admin, local admin, and a service account
// with and without the scope. It runs the exhaustive route x actor check on
// the ten new routes and fails if a principal or a route is missing.
func TestSVC8_PermissionsCoverEveryNewRouteForEveryPrincipal(t *testing.T) {
	f := newPermFix(t)
	have := map[string]bool{}
	for _, a := range f.actors {
		have[a.name] = true
	}
	for _, need := range []string{actAnonymous, actUnassigned, actDeveloper, actAdmin, actLocal, actSvcScoped, actSvcNoScope} {
		if !have[need] {
			t.Errorf("the permissions matrix has no %s actor", need)
		}
	}
	rs := newRoutes()
	for _, r := range rs {
		if !probeOps[r.OperationID] || permExtraCover[r.OperationID] == "" {
			t.Errorf("%s has no probe or no coverage entry", r.OperationID)
		}
	}
	permCombos = 0
	everyRouteEveryActor(t, f, rs)
	if want := len(rs) * len(f.actors); permCombos != want {
		t.Errorf("exercised %d combinations, want %d", permCombos, want)
	}
}

// svcDesignRoutes is the design's table of the routes the integration API
// added, written independently of routes.go: who may call each one and, for a
// service route, with which scope (docs/design/integration-api.md, "Service
// accounts", SVC-1, SVC-3, SVC-6). A route added to routes.go under these
// prefixes without a row here, or a row whose access or scope differs, fails
// the test below.
var svcDesignRoutes = map[string]struct {
	method, pattern string
	who             string // "admin" or "service"
	scope           string
}{
	"listServiceAccounts":  {"GET", "/api/v1/admin/service-accounts", "admin", ""},
	"createServiceAccount": {"POST", "/api/v1/admin/service-accounts", "admin", ""},
	"updateServiceAccount": {"PUT", "/api/v1/admin/service-accounts/{id}", "admin", ""},
	"deleteServiceAccount": {"DELETE", "/api/v1/admin/service-accounts/{id}", "admin", ""},
	"issueServiceToken":    {"POST", "/api/v1/admin/service-accounts/{id}/tokens", "admin", ""},
	"revokeServiceToken":   {"DELETE", "/api/v1/admin/service-accounts/{id}/tokens/{tokenId}", "admin", ""},
	"reportPressure":       {"PUT", "/api/v1/integration/pressure", "service", auth.ScopePressureWrite},
	"reportDeployments":    {"PUT", "/api/v1/integration/deployments", "service", auth.ScopeDeploymentsWrite},
	"getIntegrationState":  {"GET", "/api/v1/integration/state", "service", auth.ScopeStateRead},
	"serviceReleaseHold":   {"POST", "/api/v1/integration/holds/{id}/release", "service", auth.ScopeHoldsRelease},
}

// SVC-8: the expected answer for every one of these routes and every principal
// comes from svcDesignRoutes, not from the route's own access level, so a
// route registered at the wrong level, with the wrong scope, or without a row
// here fails.
func TestSVC8_DesignTableDecidesEveryNewRouteForEveryPrincipal(t *testing.T) {
	f := newPermFix(t)
	seen := map[string]bool{}
	for _, r := range Routes() {
		inScope := strings.HasPrefix(r.Pattern, "/api/v1/admin/service-accounts") ||
			strings.HasPrefix(r.Pattern, auth.IntegrationPrefix) || r.Access == AccessService
		if !inScope {
			continue
		}
		row, ok := svcDesignRoutes[r.OperationID]
		if !ok {
			t.Errorf("route %s %s (%s) has no row in svcDesignRoutes: add one for all principals", r.Method, r.Pattern, r.OperationID)
			continue
		}
		seen[r.OperationID] = true
		if row.method != r.Method || row.pattern != r.Pattern || row.scope != r.Scope {
			t.Errorf("%s: routes.go has %s %s scope %q, the design table has %s %s scope %q",
				r.OperationID, r.Method, r.Pattern, r.Scope, row.method, row.pattern, row.scope)
		}
		wantAccess := AccessAdmin
		if row.who == "service" {
			wantAccess = AccessService
		}
		if r.Access != wantAccess {
			t.Errorf("%s: access %q, the design says %q", r.OperationID, r.Access, wantAccess)
		}
		if !probeOps[r.OperationID] {
			t.Errorf("%s has no probe", r.OperationID)
			continue
		}
		for _, a := range f.actors {
			rec := f.send(a, f.probe(r.OperationID, a, modeDefault))
			allowed, status := false, http.StatusForbidden
			switch row.who {
			case "admin":
				allowed = a.name == actAdmin || a.name == actLocal
				if a.svc != "" || a.name == actAnonymous {
					status = http.StatusUnauthorized
				}
			case "service":
				allowed = a.name == actSvcScoped
				if a.name != actSvcNoScope {
					status = http.StatusUnauthorized
				}
			}
			switch {
			case allowed && permAuthRefusal(rec):
				t.Errorf("%s as %s: refused (%d %s), the design allows it", r.OperationID, a.name, rec.Code, rec.Body)
			case !allowed && rec.Code != status:
				t.Errorf("%s as %s: status %d, the design says %d: %s", r.OperationID, a.name, rec.Code, status, rec.Body)
			}
		}
	}
	for op := range svcDesignRoutes {
		if !seen[op] {
			t.Errorf("the design table lists %s but routes.go has no such route", op)
		}
	}
}
