// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/store"
)

// This file is the HTTP test of the spec's "Roles and permissions" table
// (AUTH-4, AUTH-5, AUTH-8 and the rules under the table), driven through the
// real router, store and services. Three layers:
//
//   - TestPermissions: one subtest per capability row of the spec x actor.
//   - TestPermissions_EveryRouteEveryActor: every route of Routes() x actor.
//   - the specific rules (CSRF, local admin, own holds, unknown paths) and a
//     meta-test that fails when the matrix and the spec or the route table drift.

// ---- actors ----

// permActor is one column of the matrix: who sends the request.
type permActor struct {
	name       string
	s          *sess // nil: anonymous
	class      string
	localAdmin bool
	mustChange bool // the local admin before the forced password change
	// svc is set for a service account: it sends a bearer token and no
	// session. Outside /api/v1/integration/ it is anonymous (SVC-1).
	svc string
}

// Service-account actors (SVC-8).
const (
	svcScoped   = "scoped"   // holds the scope of the route (all four scopes)
	svcUnscoped = "unscoped" // holds every scope except the route's
)

// Spec columns an actor falls under (the Manager column is out of scope for v1).
const (
	classNone       = ""           // anonymous
	classUnassigned = "unassigned" // pending, rejected, roles removed
	classDeveloper  = "developer"
	classAdmin      = "admin" // OIDC admin and the local admin
)

func (a permActor) signedIn() bool { return a.s != nil }
func (a permActor) hasRole() bool  { return a.class == classDeveloper || a.class == classAdmin }
func (a permActor) isAdmin() bool  { return a.class == classAdmin }

// Actor names, used as subtest names.
const (
	actAnonymous  = "Anonymous"
	actUnassigned = "Unassigned"
	actRejected   = "Rejected"
	actRemoved    = "Roles_removed"
	actDeveloper  = "Developer"
	actAdmin      = "Admin"
	actLocal      = "Local_admin"
	actLocalFirst = "Local_admin_before_password_change"
	actSvcScoped  = "Service_account_with_scope"
	actSvcNoScope = "Service_account_without_scope"
)

// Markers seeded into data the role-less actors must never see.
var permMarkers = []string{"MKR-setup", "MKR-label", "MKR-details", "MKR-victim", "MKR-audit", "MKR-ledger", "MKR-service"}

type permFix struct {
	*backend
	actors []permActor
	victim sess // a developer holding a marked setup
	n      int  // counter for unique names
	// service tokens: one with every scope, and per scope one with every
	// other scope (the "without the scope" actor of SVC-8)
	svcAll     svcAccount
	svcWithout map[string]svcAccount
}

func newPermFix(t *testing.T) *permFix {
	t.Helper()
	b := newBackend(t, backendOpts{start: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC),
		runtime: Runtime{ReportedProviderMaxAge: 10 * time.Minute, ReportedPressureMaxAge: 10 * time.Minute}})
	f := &permFix{backend: b}
	ctx := context.Background()
	if err := b.st.PutSetting(ctx, "max_holds_per_user", "50", nil, b.clk.Now()); err != nil {
		t.Fatal(err)
	}

	pending := b.newUser("pending-user", store.StatusPending)
	rejected := b.newUser("rejected-user", store.StatusRejected)
	removed := b.newUser("removed-user", store.StatusActive, store.RoleDeveloper)
	if err := b.st.SetUserRoles(ctx, removed.user.ID, nil); err != nil {
		t.Fatal(err)
	}
	dev := b.newUser("dev-user", store.StatusActive, store.RoleDeveloper)
	adm := b.newUser("admin-user", store.StatusActive, store.RoleAdmin)
	local := b.session(b.admin)
	f.actors = []permActor{
		{name: actAnonymous, class: classNone},
		{name: actUnassigned, s: &pending, class: classUnassigned},
		{name: actRejected, s: &rejected, class: classUnassigned},
		{name: actRemoved, s: &removed, class: classUnassigned},
		{name: actDeveloper, s: &dev, class: classDeveloper},
		{name: actAdmin, s: &adm, class: classAdmin},
		{name: actLocal, s: &local, class: classAdmin, localAdmin: true},
		{name: actLocalFirst, s: &local, class: classAdmin, localAdmin: true, mustChange: true},
		{name: actSvcScoped, class: classNone, svc: svcScoped},
		{name: actSvcNoScope, class: classNone, svc: svcUnscoped},
	}
	f.svcAll = b.mintServiceAccount("perm-svc-all", allScopes()...)
	f.svcWithout = map[string]svcAccount{"": b.mintServiceAccount("perm-svc-none")}
	for _, sc := range allScopes() {
		f.svcWithout[sc] = b.mintServiceAccount("perm-svc-without-"+strings.ReplaceAll(sc, ":", "-"), scopesWithout(sc)...)
	}

	// Data with markers: a setup held by a marked user, details, labels, a
	// ledger reason and an audit row.
	f.victim = b.newUser("MKR-victim", store.StatusActive, store.RoleDeveloper)
	sid, err := b.st.CreateSetup(ctx, store.Setup{Name: "MKR-setup-held", Enabled: true,
		Labels: map[string]string{"site": "MKR-label"}, DetailsJSON: `{"note":"MKR-details"}`})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.st.CreateHold(ctx, sid, f.victim.user.ID, b.clk.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := b.st.CreateSetup(ctx, store.Setup{Name: "MKR-setup-free", Enabled: true,
		Labels: map[string]string{"site": "MKR-label"}, DetailsJSON: `{"note":"MKR-details"}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.led.Grant(ctx, b.admin.ID, f.victim.user.ID, 5000, "MKR-ledger reason"); err != nil {
		t.Fatal(err)
	}
	actor := b.admin.ID
	if _, err := b.st.AppendAudit(ctx, store.AuditEntry{ActorID: &actor, Action: "setup.update", Target: "setup:1",
		BeforeJSON: `{"reason":"MKR-audit reason"}`, AfterJSON: `{"reason":"MKR-audit reason 2"}`}, b.clk.Now()); err != nil {
		t.Fatal(err)
	}
	// A marked service account with a token: only admins may see it.
	mkr := b.mintServiceAccount("MKR-service-account", auth.ScopeStateRead)
	if _, err := b.st.UpdateServiceAccount(ctx, mkr.acct.ID, "MKR-service description", mkr.acct.Scopes, true); err != nil {
		t.Fatal(err)
	}
	return f
}

// newServiceAccount creates a fresh account (no token) for a probe to change.
func (f *permFix) newServiceAccount() int64 {
	f.t.Helper()
	a, err := f.st.CreateServiceAccount(context.Background(), store.ServiceAccount{Name: fmt.Sprintf("perm-target-%d", f.uniq()),
		Scopes: []string{auth.ScopeStateRead}, Enabled: true}, f.clk.Now())
	if err != nil {
		f.t.Fatal(err)
	}
	return a.ID
}

func (f *permFix) actor(name string) permActor {
	for _, a := range f.actors {
		if a.name == name {
			return a
		}
	}
	f.t.Fatalf("no actor %q", name)
	return permActor{}
}

func (f *permFix) uniq() int { f.n++; return f.n }

func (f *permFix) newSetup(enabled bool) int64 {
	f.t.Helper()
	id, err := f.st.CreateSetup(context.Background(), store.Setup{Name: fmt.Sprintf("perm-%d", f.uniq()), Enabled: enabled,
		Labels: map[string]string{}, DetailsJSON: `{}`})
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}

// newHold creates a fresh setup held by the user and returns the hold's id.
func (f *permFix) newHold(userID int64) int64 {
	f.t.Helper()
	h, err := f.st.CreateHold(context.Background(), f.newSetup(true), userID, f.clk.Now())
	if err != nil {
		f.t.Fatal(err)
	}
	return h.ID
}

func (f *permFix) newPending() sess {
	return f.newUser(fmt.Sprintf("subject-%d", f.uniq()), store.StatusPending)
}

// setMustChange sets the local admin's forced password change flag.
func (f *permFix) setMustChange(v bool) {
	f.t.Helper()
	if err := f.st.SetPassword(context.Background(), f.admin.ID, f.admin.PasswordHash, v); err != nil {
		f.t.Fatal(err)
	}
}

// ---- probes: one valid request per operation ----

// probeMode says how a release probe is formed.
type probeMode int

const (
	modeDefault probeMode = iota // own hold for a developer or admin, the victim's otherwise
	modeSelf                     // the actor's own hold (the victim's when the actor has none)
	modeForce                    // force-release of the victim's hold, with a reason
)

type permProbe struct {
	method, path, body string
	as                 *sess  // overrides the actor's session (logout destroys the one it is given)
	scope              string // an integration route's scope: picks the service actor's token
}

// probeOps lists the operations probe can build; the meta-test compares it
// with Routes().
var probeOps = map[string]bool{
	"oidcStart": true, "oidcCallback": true, "localSignIn": true, "changeLocalPassword": true, "logout": true,
	"getHealth": true, "getAuthMethods": true, "getSession": true,
	"getMe": true, "listMyHolds": true, "listMyLedger": true, "getMyUsage": true, "getPressure": true,
	"listSetups": true, "acquireSetup": true, "releaseHold": true,
	"listAdminUsers": true, "setUserRoles": true, "rejectUser": true, "grantTokens": true, "resetBalance": true,
	"createSetup": true, "updateSetup": true, "deleteSetup": true, "getRules": true, "replaceRules": true,
	"getSettings": true, "updateSettings": true, "getCollector": true, "listAudit": true,
	"listServiceAccounts": true, "createServiceAccount": true, "updateServiceAccount": true, "deleteServiceAccount": true,
	"issueServiceToken": true, "revokeServiceToken": true,
	"reportPressure": true, "reportDeployments": true, "getIntegrationState": true, "serviceReleaseHold": true,
}

const permSettingsBody = `{"default_weight":1,"display_label_keys":["tier"],` +
	`"bands":{"base":0,"steps":[{"from":0.5,"multiplier":1},{"from":0.8,"multiplier":2}]},"default_allowance":250.5,` +
	`"max_holds_per_user":50,"release_requires_teardown":false,"poll_interval":"10m","enforcement":false}`

// probe builds a valid request for the operation as the actor. Every call
// creates its own targets (a new setup, hold or pending user), so probes can
// run in any order on one backend.
func (f *permFix) probe(op string, a permActor, mode probeMode) permProbe {
	f.t.Helper()
	id := func(n int64) string { return fmt.Sprint(n) }
	switch op {
	case "oidcStart":
		return permProbe{method: "GET", path: "/auth/oidc/start"}
	case "oidcCallback":
		return permProbe{method: "GET", path: "/auth/callback"}
	case "localSignIn":
		return permProbe{method: "POST", path: "/auth/local", body: `{}`}
	case "changeLocalPassword":
		return permProbe{method: "POST", path: "/auth/local/password", body: `{"current_password":"wrong","new_password":"another long password"}`}
	case "logout":
		if a.signedIn() {
			s := f.session(a.s.user)
			return permProbe{method: "POST", path: "/auth/logout", as: &s}
		}
		return permProbe{method: "POST", path: "/auth/logout"}
	case "getHealth":
		return permProbe{method: "GET", path: "/healthz"}
	case "getAuthMethods":
		return permProbe{method: "GET", path: "/api/v1/auth/methods"}
	case "getSession":
		return permProbe{method: "GET", path: "/api/v1/session"}
	case "getMe":
		return permProbe{method: "GET", path: "/api/v1/me"}
	case "listMyHolds":
		return permProbe{method: "GET", path: "/api/v1/me/holds"}
	case "listMyLedger":
		return permProbe{method: "GET", path: "/api/v1/me/ledger"}
	case "getMyUsage":
		return permProbe{method: "GET", path: "/api/v1/me/usage"}
	case "getPressure":
		return permProbe{method: "GET", path: "/api/v1/pressure"}
	case "listSetups":
		return permProbe{method: "GET", path: "/api/v1/setups"}
	case "acquireSetup":
		return permProbe{method: "POST", path: "/api/v1/setups/" + id(f.newSetup(true)) + "/acquire"}
	case "releaseHold":
		own := a.signedIn() && !a.localAdmin && a.hasRole()
		switch {
		case mode == modeForce:
			return permProbe{method: "POST", path: "/api/v1/holds/" + id(f.victimHold()) + "/release", body: `{"force":true,"reason":"permissions test"}`}
		case own:
			return permProbe{method: "POST", path: "/api/v1/holds/" + id(f.newHold(a.s.user.ID)) + "/release"}
		case a.localAdmin && mode == modeDefault:
			return permProbe{method: "POST", path: "/api/v1/holds/" + id(f.victimHold()) + "/release", body: `{"force":true,"reason":"permissions test"}`}
		default:
			return permProbe{method: "POST", path: "/api/v1/holds/" + id(f.victimHold()) + "/release"}
		}
	case "listAdminUsers":
		return permProbe{method: "GET", path: "/api/v1/admin/users"}
	case "setUserRoles":
		return permProbe{method: "PUT", path: "/api/v1/admin/users/" + id(f.newPending().user.ID) + "/roles", body: `{"roles":["developer"]}`}
	case "rejectUser":
		return permProbe{method: "POST", path: "/api/v1/admin/users/" + id(f.newPending().user.ID) + "/reject"}
	case "grantTokens":
		return permProbe{method: "POST", path: "/api/v1/admin/users/" + id(f.victim.user.ID) + "/grants", body: `{"amount_tokens":1,"reason":"permissions test"}`}
	case "resetBalance":
		return permProbe{method: "POST", path: "/api/v1/admin/users/" + id(f.victim.user.ID) + "/reset", body: `{"reason":"permissions test"}`}
	case "createSetup":
		return permProbe{method: "POST", path: "/api/v1/admin/setups", body: setupBody(fmt.Sprintf("created-%d", f.uniq()), `{}`, true)}
	case "updateSetup":
		return permProbe{method: "PUT", path: "/api/v1/admin/setups/" + id(f.newSetup(true)), body: setupBody(fmt.Sprintf("edited-%d", f.uniq()), `{}`, true)}
	case "deleteSetup":
		return permProbe{method: "DELETE", path: "/api/v1/admin/setups/" + id(f.newSetup(false))}
	case "getRules":
		return permProbe{method: "GET", path: "/api/v1/admin/rules"}
	case "replaceRules":
		return permProbe{method: "PUT", path: "/api/v1/admin/rules", body: `{"rules":[]}`}
	case "getSettings":
		return permProbe{method: "GET", path: "/api/v1/admin/settings"}
	case "updateSettings":
		return permProbe{method: "PUT", path: "/api/v1/admin/settings", body: permSettingsBody}
	case "getCollector":
		return permProbe{method: "GET", path: "/api/v1/admin/collector"}
	case "listAudit":
		return permProbe{method: "GET", path: "/api/v1/admin/audit"}
	case "listServiceAccounts":
		return permProbe{method: "GET", path: "/api/v1/admin/service-accounts"}
	case "createServiceAccount":
		return permProbe{method: "POST", path: "/api/v1/admin/service-accounts",
			body: fmt.Sprintf(`{"name":"perm-created-%d","description":"permissions test","scopes":["state:read"]}`, f.uniq())}
	case "updateServiceAccount":
		return permProbe{method: "PUT", path: "/api/v1/admin/service-accounts/" + id(f.newServiceAccount()),
			body: `{"description":"edited","scopes":["state:read","holds:release"],"enabled":true}`}
	case "deleteServiceAccount":
		return permProbe{method: "DELETE", path: "/api/v1/admin/service-accounts/" + id(f.newServiceAccount())}
	case "issueServiceToken":
		return permProbe{method: "POST", path: "/api/v1/admin/service-accounts/" + id(f.newServiceAccount()) + "/tokens", body: `{}`}
	case "revokeServiceToken":
		acct := f.newServiceAccount()
		_, tok := f.mintServiceToken(acct, nil)
		return permProbe{method: "DELETE", path: "/api/v1/admin/service-accounts/" + id(acct) + "/tokens/" + id(tok)}
	case "reportPressure":
		return permProbe{method: "PUT", path: "/api/v1/integration/pressure", body: `{"value":0.5}`, scope: auth.ScopePressureWrite}
	case "reportDeployments":
		return permProbe{method: "PUT", path: "/api/v1/integration/deployments", body: `{"deployments":[]}`, scope: auth.ScopeDeploymentsWrite}
	case "getIntegrationState":
		return permProbe{method: "GET", path: "/api/v1/integration/state", scope: auth.ScopeStateRead}
	case "serviceReleaseHold":
		return permProbe{method: "POST", path: "/api/v1/integration/holds/" + id(f.victimHold()) + "/release",
			body: `{"reason":"permissions test"}`, scope: auth.ScopeHoldsRelease}
	}
	f.t.Fatalf("no probe for operation %q", op)
	return permProbe{}
}

// victimHold returns a fresh hold of the marked victim.
func (f *permFix) victimHold() int64 { return f.newHold(f.victim.user.ID) }

// send runs a probe as the actor with the actor's valid CSRF token.
func (f *permFix) send(a permActor, p permProbe) *httptest.ResponseRecorder {
	f.t.Helper()
	if a.localAdmin {
		f.setMustChange(a.mustChange)
	}
	s := a.s
	if p.as != nil {
		s = p.as
	}
	if tok := f.svcToken(a, p.scope); tok != "" {
		return f.serve(p.method, p.path, s, true, p.body, bearer(tok)...)
	}
	return f.serve(p.method, p.path, s, true, p.body)
}

// svcToken is the bearer token a service actor sends for a route of the
// given scope ("" for a route outside /api/v1/integration/): the all-scopes
// token, or one holding every scope but the route's. Others send none.
func (f *permFix) svcToken(a permActor, scope string) string {
	switch a.svc {
	case svcScoped:
		return f.svcAll.token
	case svcUnscoped:
		return f.svcWithout[scope].token
	}
	return ""
}

// ---- expectations ----

// permDenial is a refusal by the access rules: status and error code.
type permDenial struct {
	status int
	code   string
}

// gateDenial is what the access level of a route (AUTH-4, AUTH-3, AUTH-7)
// answers for an actor whose request is otherwise valid; nil means the
// request reaches the handler.
func gateDenial(r Route, a permActor) *permDenial {
	deny := func(code string) *permDenial { return &permDenial{apierr.Status(code), code} }
	switch r.Access {
	case AccessPublic:
		return nil
	case AccessSession:
		if !a.signedIn() {
			return deny(apierr.Unauthenticated)
		}
		return nil
	case AccessRole:
		switch {
		case !a.signedIn():
			return deny(apierr.Unauthenticated)
		case !a.hasRole():
			return deny(apierr.NoRole)
		case a.mustChange:
			return deny(apierr.PasswordChangeRequired)
		}
		return nil
	case AccessAdmin:
		switch {
		case !a.signedIn():
			return deny(apierr.Unauthenticated)
		case !a.hasRole():
			return deny(apierr.NoRole)
		case !a.isAdmin():
			return deny(apierr.Forbidden)
		case a.mustChange:
			return deny(apierr.PasswordChangeRequired)
		}
		return nil
	case AccessLocalAdmin:
		switch {
		case !a.signedIn():
			return deny(apierr.Unauthenticated)
		case !a.localAdmin:
			return deny(apierr.Forbidden)
		}
		return nil
	case AccessService:
		// A session, even an admin's, is not read here (SVC-2); only a
		// service token with the route's scope gets in (SVC-3).
		switch a.svc {
		case svcScoped:
			return nil
		case svcUnscoped:
			return deny(apierr.Forbidden)
		}
		return deny(apierr.Unauthenticated)
	}
	panic("unknown access level " + string(r.Access))
}

// permAuthRefusal reports whether a response is one of the access refusals.
func permAuthRefusal(rec *httptest.ResponseRecorder) bool {
	if rec.Code == http.StatusUnauthorized {
		return true
	}
	if rec.Code != http.StatusForbidden {
		return false
	}
	switch c := permErrCode(rec); c {
	case apierr.NoRole, apierr.Forbidden, apierr.CSRF, apierr.PasswordChangeRequired, apierr.Unauthenticated:
		return true
	}
	return false
}

func permErrCode(rec *httptest.ResponseRecorder) string {
	var b struct{ Error struct{ Code string } }
	_ = json.Unmarshal(rec.Body.Bytes(), &b)
	return b.Error.Code
}

// permExactError asserts the body is exactly {"error":{"code","message"}} and
// returns the code.
func permExactError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &top); err != nil {
		t.Errorf("body is not JSON: %v: %q", err, rec.Body)
		return ""
	}
	raw, ok := top["error"]
	if len(top) != 1 || !ok {
		t.Errorf("body is not exactly the error object: %s", rec.Body)
		return ""
	}
	var inner map[string]string
	if err := json.Unmarshal(raw, &inner); err != nil || len(inner) != 2 || inner["code"] == "" || inner["message"] == "" {
		t.Errorf("error object is not exactly {code, message}: %s", rec.Body)
		return ""
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("error Content-Type = %q", ct)
	}
	return inner["code"]
}

// permWantDenied asserts a refusal with the exact status, code and body.
func permWantDenied(t *testing.T, rec *httptest.ResponseRecorder, d permDenial) {
	t.Helper()
	if rec.Code != d.status {
		t.Errorf("status = %d, want %d: %s", rec.Code, d.status, rec.Body)
	}
	if c := permExactError(t, rec); c != d.code {
		t.Errorf("code = %q, want %q", c, d.code)
	}
}

// permWantAllowed asserts the request got past the access rules and, for a
// route that has data, succeeded.
func permWantAllowed(t *testing.T, r Route, rec *httptest.ResponseRecorder) {
	t.Helper()
	if permAuthRefusal(rec) {
		t.Errorf("refused although allowed: %d %s", rec.Code, rec.Body)
		return
	}
	if rec.Code >= 500 {
		t.Errorf("server error: %d %s", rec.Code, rec.Body)
		return
	}
	// Probes of the public pre-session calls and the wrong-password change
	// are valid requests that the handler may answer with a 4xx of its own.
	switch r.OperationID {
	case "oidcStart", "oidcCallback", "localSignIn", "changeLocalPassword":
		return
	}
	if rec.Code >= 400 {
		t.Errorf("status = %d, want success: %s", rec.Code, rec.Body)
	}
}

func permContainsMarker(rec *httptest.ResponseRecorder) string {
	hay := strings.ToLower(rec.Body.String())
	for k, vs := range rec.Header() {
		hay += "\n" + strings.ToLower(k) + ": " + strings.ToLower(strings.Join(vs, ","))
	}
	for _, m := range permMarkers {
		if strings.Contains(hay, strings.ToLower(m)) {
			return m
		}
	}
	return ""
}

// ---- the matrix: the capability rows of the spec ----

type capRow struct {
	name                         string // the spec's wording
	unassigned, developer, admin bool   // the spec's Yes/No cells
	ops                          []string
	force                        bool // release probes are forced releases
	pageOnly                     bool // the row is about the not-authorized page (checked on the session body)
	noV1Route                    bool // a Manager-only capability: no v1 route may exist
}

// capMatrix has one row per capability row of docs/SPEC.md "Roles and
// permissions", in the spec's order. Every route of Routes() appears in at
// least one row or in permExtraCover.
var capMatrix = []capRow{
	{name: "Sign in", unassigned: true, developer: true, admin: true,
		ops: []string{"oidcStart", "oidcCallback", "localSignIn", "logout", "getSession", "getHealth", "getAuthMethods"}},
	{name: `See the "not authorized" page only`, unassigned: true, pageOnly: true, ops: []string{"getSession"}},
	{name: "See own balance, current holds, history", developer: true, admin: true,
		ops: []string{"getMe", "listMyHolds", "listMyLedger", "getMyUsage"}},
	{name: "See all setups, holders and holders' balances", developer: true, admin: true,
		ops: []string{"listSetups", "getPressure"}},
	{name: "Acquire and release a setup for self", developer: true, admin: true,
		ops: []string{"acquireSetup", "releaseHold"}},
	{name: "See pending access requests; approve or reject", admin: true,
		ops: []string{"listAdminUsers", "setUserRoles", "rejectUser"}},
	{name: "Assign and remove roles", admin: true, ops: []string{"setUserRoles"}},
	{name: "Grant tokens to a user", admin: true, ops: []string{"grantTokens", "resetBalance"}},
	{name: "Force-release another user's setup", admin: true, force: true, ops: []string{"releaseHold"}},
	{name: "Add, edit, delete setups", admin: true, ops: []string{"createSetup", "updateSetup", "deleteSetup"}},
	{name: "Edit metering settings", admin: true,
		ops: []string{"getSettings", "updateSettings", "getRules", "replaceRules", "getCollector", "listAudit"}},
	{name: "See team usage, move tokens between team members", noV1Route: true},
}

// permExtraCover lists routes no spec row names, with where they are covered.
// The service-account and integration routes come from
// docs/design/integration-api.md, not the spec's table (D42).
var permExtraCover = map[string]string{
	"changeLocalPassword":  "TestAUTH8_LocalAdminIsPermanent (AUTH-8) and the exhaustive test",
	"listServiceAccounts":  "TestSVC8_ (SVC-6, SVC-8) and the exhaustive test",
	"createServiceAccount": "TestSVC8_ (SVC-6, SVC-8) and the exhaustive test",
	"updateServiceAccount": "TestSVC8_ (SVC-6, SVC-8) and the exhaustive test",
	"deleteServiceAccount": "TestSVC8_ (SVC-6, SVC-8) and the exhaustive test",
	"issueServiceToken":    "TestSVC8_ (SVC-6, SVC-8) and the exhaustive test",
	"revokeServiceToken":   "TestSVC8_ (SVC-6, SVC-8) and the exhaustive test",
	"reportPressure":       "TestSVC8_ (SVC-1 to SVC-3, SVC-8) and the exhaustive test",
	"reportDeployments":    "TestSVC8_ (SVC-1 to SVC-3, SVC-8) and the exhaustive test",
	"getIntegrationState":  "TestSVC8_ (SVC-1 to SVC-3, SVC-8) and the exhaustive test",
	"serviceReleaseHold":   "TestSVC8_ (SVC-1 to SVC-3, SVC-8) and the exhaustive test",
}

func routeOf(t *testing.T, op string) Route {
	t.Helper()
	for _, r := range Routes() {
		if r.OperationID == op {
			return r
		}
	}
	t.Fatalf("no route for operation %q", op)
	return Route{}
}

// cellAllowed is the matrix's reading of a spec cell for an actor and route.
func cellAllowed(row capRow, r Route, a permActor) bool {
	switch {
	case r.Access == AccessPublic:
		return true
	case !a.signedIn():
		return false
	case r.Access == AccessSession:
		return true
	case a.mustChange:
		return false // AUTH-7: nothing but the password change and sign-out
	}
	switch a.class {
	case classUnassigned:
		return row.unassigned
	case classDeveloper:
		return row.developer
	}
	return row.admin
}

// ruleRefusal is a refusal by rule, past the access rules: the local admin
// cannot hold setups (AUTH-8, LED-1), so it cannot acquire and has no hold of
// its own to release; a developer cannot force-release (HOLD-2).
func ruleRefusal(op string, mode probeMode, a permActor) (permDenial, bool) {
	forbidden := permDenial{http.StatusForbidden, apierr.Forbidden}
	switch {
	case a.localAdmin && op == "acquireSetup",
		a.localAdmin && op == "releaseHold" && mode == modeSelf,
		a.class == classDeveloper && op == "releaseHold" && mode == modeForce:
		return forbidden, true
	}
	return permDenial{}, false
}

// expect is what a request of the actor must get: ok, or a refusal; a zero
// refusal means the spec says no but nothing in the code refuses.
func expect(row capRow, r Route, a permActor, mode probeMode) (ok bool, d permDenial) {
	if ref, refused := ruleRefusal(r.OperationID, mode, a); refused && !a.mustChange {
		return false, ref
	}
	if cellAllowed(row, r, a) {
		return true, permDenial{}
	}
	if g := gateDenial(r, a); g != nil {
		return false, *g
	}
	return false, permDenial{}
}

func TestPermissions(t *testing.T) {
	f := newPermFix(t)
	for _, row := range capMatrix {
		t.Run(strings.ReplaceAll(row.name, " ", "_"), func(t *testing.T) {
			if row.noV1Route {
				for _, r := range Routes() {
					if strings.Contains(r.Pattern, "team") {
						t.Errorf("v1 has no team route, found %s %s", r.Method, r.Pattern)
					}
				}
				return
			}
			for _, a := range f.actors {
				t.Run(a.name, func(t *testing.T) {
					for _, op := range row.ops {
						r := routeOf(t, op)
						mode := modeSelf
						if row.force {
							mode = modeForce
						}
						rec := f.send(a, f.probe(op, a, mode))
						if row.pageOnly {
							f.checkNotAuthorizedPage(t, a, rec)
						} else if ok, d := expect(row, r, a, mode); ok {
							permWantAllowed(t, r, rec)
						} else if d == (permDenial{}) {
							t.Errorf("%s: the spec says no for %s but the route's access level lets it through", op, a.name)
						} else {
							permWantDenied(t, rec, d)
						}
						if a.class == classUnassigned || !a.signedIn() {
							if m := permContainsMarker(rec); m != "" {
								t.Errorf("%s: response holds seeded data %q", op, m)
							}
						}
					}
				})
			}
		})
	}
}

// checkNotAuthorizedPage: a signed-in actor with no role gets the empty role
// list and only their own identity (AUTH-5); anyone with a role gets roles.
func (f *permFix) checkNotAuthorizedPage(t *testing.T, a permActor, rec *httptest.ResponseRecorder) {
	t.Helper()
	if !a.signedIn() {
		permWantDenied(t, rec, permDenial{401, apierr.Unauthenticated})
		return
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("session: %d %s", rec.Code, rec.Body)
	}
	var v struct {
		UserID int64    `json:"user_id"`
		Email  string   `json:"email"`
		Roles  []string `json:"roles"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if v.UserID != a.s.user.ID || v.Email != a.s.user.Email {
		t.Errorf("session identity = %d %q, want the actor's own %d %q", v.UserID, v.Email, a.s.user.ID, a.s.user.Email)
	}
	if a.class == classUnassigned && len(v.Roles) != 0 {
		t.Errorf("a user with no role has roles %v", v.Roles)
	}
	if a.class != classUnassigned && len(v.Roles) == 0 {
		t.Errorf("%s has no roles in the session", a.name)
	}
	if m := permContainsMarker(rec); m != "" {
		t.Errorf("session shows seeded data %q", m)
	}
}

// ---- every route x every actor ----

// permCombos counts the (route, actor) pairs the exhaustive test exercised.
var permCombos int

func TestPermissions_EveryRouteEveryActor(t *testing.T) {
	f := newPermFix(t)
	permCombos = 0
	everyRouteEveryActor(t, f, Routes())
	if want := len(Routes()) * len(f.actors); permCombos != want {
		t.Errorf("exercised %d (route, actor) combinations, want %d", permCombos, want)
	}
}

// everyRouteEveryActor sends a valid probe of each route as each actor and
// checks the access rules' answer.
func everyRouteEveryActor(t *testing.T, f *permFix, rs []Route) {
	exempt := func(r Route) bool { // routes that stay open to a user with no role (AUTH-4)
		return r.Pattern == "/healthz" || strings.HasPrefix(r.Pattern, "/auth/") ||
			(r.Method == "GET" && (r.Pattern == "/api/v1/auth/methods" || r.Pattern == "/api/v1/session"))
	}
	for _, r := range rs {
		t.Run(r.Method+" "+strings.ReplaceAll(r.Pattern, "/", "_"), func(t *testing.T) {
			for _, a := range f.actors {
				permCombos++
				rec := f.send(a, f.probe(r.OperationID, a, modeDefault))
				d := gateDenial(r, a)
				ref, refused := ruleRefusal(r.OperationID, modeDefault, a)
				switch {
				case d != nil:
					permWantDenied(t, rec, *d)
				case refused:
					permWantDenied(t, rec, ref)
				default:
					permWantAllowed(t, r, rec)
				}
				// A role-less actor gets no data. The one exception is a service
				// account on an integration route it holds the scope for.
				roleless := (!a.signedIn() || a.class == classUnassigned) && !(r.Access == AccessService && d == nil)
				if roleless {
					if m := permContainsMarker(rec); m != "" {
						t.Errorf("%s as %s: response holds seeded data %q: %s", r.OperationID, a.name, m, rec.Body)
					}
					if !exempt(r) {
						if rec.Code != 401 && rec.Code != 403 {
							t.Errorf("%s as %s: status %d, want 401 or 403 (no data for a user with no role)", r.OperationID, a.name, rec.Code)
						}
						permExactError(t, rec)
					}
				}
				if t.Failed() {
					t.Logf("failing combination: %s %s as %s", r.Method, r.Pattern, a.name)
					return
				}
			}
		})
	}
}

// TestPermissions_NoRoleSessionIsOwnIdentityOnly: GET /api/v1/session for a
// user with no role returns an empty role list and only that user's identity.
func TestAUTH5_SessionOfUserWithNoRoleIsOwnIdentityOnly(t *testing.T) {
	f := newPermFix(t)
	for _, name := range []string{actUnassigned, actRejected, actRemoved} {
		a := f.actor(name)
		rec := f.send(a, f.probe("getSession", a, modeDefault))
		f.checkNotAuthorizedPage(t, a, rec)
		var m map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		if roles, ok := m["roles"].([]any); !ok || len(roles) != 0 {
			t.Errorf("%s: roles = %v, want []", name, m["roles"])
		}
		for k := range m {
			switch k {
			case "user_id", "username", "email", "name", "display_name", "auth_source", "status", "roles", "csrf_token", "must_change_password", "is_local_admin":
			default:
				t.Errorf("%s: session has an unexpected field %q", name, k)
			}
		}
	}
}

// ---- state-changing routes and CSRF (AUTH-3) ----

func TestAUTH3_EveryStateChangingRouteRejectsMissingOrWrongCSRF(t *testing.T) {
	f := newPermFix(t)
	n := 0
	for _, r := range Routes() {
		if !r.StateChanging() {
			continue
		}
		for _, a := range f.actors {
			if !a.signedIn() {
				continue
			}
			p := f.probe(r.OperationID, a, modeDefault)
			if a.localAdmin {
				f.setMustChange(a.mustChange)
			}
			s := a.s
			if p.as != nil {
				s = p.as
			}
			// What an actor that passes the access rules gets is the csrf refusal;
			// anyone the rules already refuse gets that refusal and still nothing else.
			want := permDenial{http.StatusForbidden, apierr.CSRF}
			if d := gateDenial(r, a); d != nil {
				want = *d
			}
			other := f.session(f.victim.user) // a valid token, but of another session
			for name, hdr := range map[string][]string{
				"missing": nil,
				"wrong":   {"X-CSRF-Token", "not-the-token"},
				"empty":   {"X-CSRF-Token", ""},
				"foreign": {"X-CSRF-Token", other.csrf},
			} {
				n++
				rec := f.serve(p.method, p.path, s, false, p.body, hdr...)
				if rec.Code != want.status || permErrCode(rec) != want.code {
					t.Errorf("%s %s as %s with a %s token: %d %q, want %d %q", r.Method, r.Pattern, a.name, name,
						rec.Code, permErrCode(rec), want.status, want.code)
				}
				permExactError(t, rec)
			}
		}
	}
	if n == 0 {
		t.Fatal("no state-changing route was exercised")
	}
	// Nothing was created by a rejected request.
	if list, err := f.st.ListSetups(context.Background(), false); err != nil {
		t.Fatal(err)
	} else {
		for _, s := range list {
			if strings.HasPrefix(s.Name, "created-") || strings.HasPrefix(s.Name, "edited-") {
				t.Errorf("a request without a valid CSRF token changed setup %q", s.Name)
			}
		}
	}
	// The same requests with the right token are not refused as csrf.
	adm := f.actor(actAdmin)
	if rec := f.send(adm, f.probe("createSetup", adm, modeDefault)); rec.Code != http.StatusCreated {
		t.Errorf("createSetup with the right token: %d %s", rec.Code, rec.Body)
	}
}

// ---- the local admin (AUTH-8) ----

func TestAUTH8_LocalAdminIsPermanent(t *testing.T) {
	f := newPermFix(t)
	ctx := context.Background()
	local := f.actor(actLocal)
	adm := f.actor(actAdmin)
	localID := f.admin.ID

	// No route can disable, rename or delete a user: the only routes that
	// address a user by id are roles, reject, grants and reset.
	userRoutes := map[string]bool{}
	for _, r := range Routes() {
		if strings.Contains(r.Pattern, "/users") {
			userRoutes[r.Method+" "+r.Pattern] = true
		}
	}
	for _, k := range []string{"GET /api/v1/admin/users", "PUT /api/v1/admin/users/{id}/roles", "POST /api/v1/admin/users/{id}/reject",
		"POST /api/v1/admin/users/{id}/grants", "POST /api/v1/admin/users/{id}/reset"} {
		if !userRoutes[k] {
			t.Errorf("expected route %s", k)
		}
		delete(userRoutes, k)
	}
	for k := range userRoutes {
		t.Errorf("unreviewed user route %s: check that it cannot disable, rename, delete or demote the local admin (AUTH-8)", k)
	}

	// Attempts, by an OIDC admin and by the local admin itself.
	for _, by := range []permActor{adm, local} {
		f.setMustChange(false)
		id := fmt.Sprint(localID)
		for _, tc := range []struct{ method, path, body string }{
			{"PUT", "/api/v1/admin/users/" + id + "/roles", `{"roles":[]}`},
			{"PUT", "/api/v1/admin/users/" + id + "/roles", `{"roles":["developer"]}`},
			{"PUT", "/api/v1/admin/users/" + id + "/roles", `{"roles":["admin","developer"]}`},
			{"POST", "/api/v1/admin/users/" + id + "/reject", ``},
			{"POST", "/api/v1/admin/users/" + id + "/grants", `{"amount_tokens":5,"reason":"x"}`},
			{"POST", "/api/v1/admin/users/" + id + "/reset", `{"reason":"x"}`},
		} {
			rec := f.serve(tc.method, tc.path, by.s, true, tc.body)
			wantCode := apierr.LocalAdminProtected // also when the local admin demotes itself: that check comes first
			if rec.Code != apierr.Status(wantCode) || permErrCode(rec) != wantCode {
				t.Errorf("%s %s %s by %s: %d %s, want %d %s", tc.method, tc.path, tc.body, by.name, rec.Code, rec.Body, apierr.Status(wantCode), wantCode)
			}
			permExactError(t, rec)
		}
	}

	// It cannot acquire, and nothing was written for it.
	f.setMustChange(false)
	sid := f.newSetup(true)
	rec := f.serve("POST", fmt.Sprintf("/api/v1/setups/%d/acquire", sid), local.s, true, "")
	permWantDenied(t, rec, permDenial{http.StatusForbidden, apierr.Forbidden})
	if n, err := f.st.CountActiveHolds(ctx, localID); err != nil || n != 0 {
		t.Errorf("local admin holds %d setups (err %v)", n, err)
	}
	if rows, err := f.st.ListLedger(ctx, localID, "2026-11"); err != nil || len(rows) != 0 {
		t.Errorf("local admin has %d ledger rows (err %v)", len(rows), err)
	}
	// No balance, no hold limit: the UI shows neither.
	rec = f.serve("GET", "/api/v1/me", local.s, false, "")
	var me map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil || rec.Code != 200 {
		t.Fatalf("me: %d %s", rec.Code, rec.Body)
	}
	if me["balance_tokens"] != nil || me["can_acquire"] != false || me["cannot_acquire_reason"] != "local_admin" {
		t.Errorf("local admin /me: balance %v can_acquire %v reason %v", me["balance_tokens"], me["can_acquire"], me["cannot_acquire_reason"])
	}

	// Still the permanent admin.
	u, err := f.st.GetUser(ctx, localID)
	if err != nil {
		t.Fatal(err)
	}
	roles, _ := f.st.GetUserRoles(ctx, localID)
	if u.Username != "admin" || u.Status != store.StatusActive || len(roles) != 1 || roles[0] != store.RoleAdmin {
		t.Errorf("local admin changed: username %q status %q roles %v", u.Username, u.Status, roles)
	}

	// The password route is the local admin's alone, even before the change.
	for _, a := range f.actors {
		rec := f.send(a, f.probe("changeLocalPassword", a, modeDefault))
		if d := gateDenial(routeOf(t, "changeLocalPassword"), a); d != nil {
			permWantDenied(t, rec, *d)
		} else if permAuthRefusal(rec) || rec.Code >= 500 {
			t.Errorf("%s: password route refused or failed: %d %s", a.name, rec.Code, rec.Body)
		}
	}
}

// An admin cannot remove their own admin role.
func TestPermissions_AdminCannotRemoveOwnAdminRole(t *testing.T) {
	f := newPermFix(t)
	adm := f.actor(actAdmin)
	for _, body := range []string{`{"roles":[]}`, `{"roles":["developer"]}`} {
		rec := f.serve("PUT", fmt.Sprintf("/api/v1/admin/users/%d/roles", adm.s.user.ID), adm.s, true, body)
		permWantDenied(t, rec, permDenial{http.StatusConflict, apierr.SelfDemote})
	}
	roles, _ := f.st.GetUserRoles(context.Background(), adm.s.user.ID)
	if len(roles) != 1 || roles[0] != store.RoleAdmin {
		t.Errorf("roles after the attempts = %v", roles)
	}
}

// The manager role is for later: it cannot be assigned in v1.
func TestPermissions_ManagerIsNotAssignable(t *testing.T) {
	f := newPermFix(t)
	adm := f.actor(actAdmin)
	target := f.newPending()
	rec := f.serve("PUT", fmt.Sprintf("/api/v1/admin/users/%d/roles", target.user.ID), adm.s, true, `{"roles":["manager"]}`)
	if rec.Code != http.StatusBadRequest || permErrCode(rec) != apierr.Validation {
		t.Errorf("assigning manager: %d %s", rec.Code, rec.Body)
	}
	if roles, _ := f.st.GetUserRoles(context.Background(), target.user.ID); len(roles) != 0 {
		t.Errorf("roles after = %v", roles)
	}
}

// ---- "own" routes cannot be used on someone else's hold ----

func TestHOLD2_OwnRoutesCannotTouchAnotherUsersHold(t *testing.T) {
	f := newPermFix(t)
	devA := f.actor(actDeveloper)
	hold := f.victimHold() // developer B's hold
	path := fmt.Sprintf("/api/v1/holds/%d/release", hold)
	for _, body := range []string{``, `{"force":true,"reason":"let me"}`} {
		rec := f.serve("POST", path, devA.s, true, body)
		permWantDenied(t, rec, permDenial{http.StatusForbidden, apierr.Forbidden})
		if m := permContainsMarker(rec); m != "" {
			t.Errorf("refusal shows %q", m)
		}
	}
	// A plain admin release of someone else's hold is not a force-release either.
	adm := f.actor(actAdmin)
	permWantDenied(t, f.serve("POST", path, adm.s, true, ``), permDenial{http.StatusForbidden, apierr.Forbidden})
	h, err := f.st.GetHold(context.Background(), hold)
	if err != nil || h.ReleasedAt != nil {
		t.Errorf("hold changed by a refused request: %+v %v", h, err)
	}
	// The hold does not show in A's own list.
	rec := f.serve("GET", "/api/v1/me/holds", devA.s, false, "")
	if strings.Contains(rec.Body.String(), `"id":`+fmt.Sprint(hold)+",") || permContainsMarker(rec) != "" {
		t.Errorf("developer A's holds list shows developer B's hold: %s", rec.Body)
	}
	// A hold that does not exist is a 404, not a hint about other ids.
	rec = f.serve("POST", "/api/v1/holds/999999/release", devA.s, true, ``)
	permWantDenied(t, rec, permDenial{http.StatusNotFound, apierr.NotFound})
	// The admin's force-release works and is the only way.
	rec = f.serve("POST", path, adm.s, true, `{"force":true,"reason":"permissions test"}`)
	if rec.Code != http.StatusOK {
		t.Errorf("force-release by an admin: %d %s", rec.Code, rec.Body)
	}
}

// ---- unknown paths under /api/ and /auth/ ----

func TestAUTH4_UnknownAPIPathsAreJSON404WithoutData(t *testing.T) {
	f := newPermFix(t)
	paths := []struct{ method, path string }{
		{"GET", "/api/"}, {"GET", "/api/v1"}, {"GET", "/api/v1/nope"}, {"GET", "/api/v2/me"},
		{"GET", "/api/v1/setups/1"}, {"DELETE", "/api/v1/setups"}, {"PUT", "/api/v1/me"},
		{"GET", "/api/v1/admin"}, {"POST", "/api/v1/admin/users"}, {"GET", "/api/v1/admin/users/1"},
		{"GET", "/api/static/app.js"}, {"GET", "/api/v1/../me"}, {"GET", "/auth/nope"}, {"POST", "/auth/oidc/start"},
	}
	for _, a := range f.actors {
		for _, p := range paths {
			rec := f.send(a, permProbe{method: p.method, path: p.path})
			if rec.Code != http.StatusNotFound && rec.Code != http.StatusMovedPermanently && rec.Code != http.StatusTemporaryRedirect {
				t.Errorf("%s %s as %s: status %d, want 404", p.method, p.path, a.name, rec.Code)
				continue
			}
			if rec.Code == http.StatusNotFound {
				if c := permExactError(t, rec); c != apierr.NotFound {
					t.Errorf("%s %s as %s: code %q", p.method, p.path, a.name, c)
				}
			}
			if m := permContainsMarker(rec); m != "" {
				t.Errorf("%s %s as %s: shows %q", p.method, p.path, a.name, m)
			}
			if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") && rec.Code == http.StatusNotFound {
				t.Errorf("%s %s: Cache-Control = %q", p.method, p.path, cc)
			}
		}
	}
}

// ---- the meta-test: the matrix cannot silently rot ----

// specRoles reads the capability table of docs/SPEC.md "Roles and permissions".
func specRoles(t *testing.T) map[string][3]bool {
	t.Helper()
	path := findUp(t, "docs/SPEC.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	i := strings.Index(text, "## Roles and permissions")
	if i < 0 {
		t.Fatal("docs/SPEC.md has no 'Roles and permissions' section")
	}
	rows := map[string][3]bool{}
	for _, line := range strings.Split(text[i:], "\n")[1:] {
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "Rules:") {
			break
		}
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if len(cells) != 5 || cells[0] == "Capability" || strings.HasPrefix(cells[0], "---") {
			continue
		}
		yes := func(s string) bool { return s == "Yes" }
		rows[cells[0]] = [3]bool{yes(cells[1]), yes(cells[2]), yes(cells[3])}
	}
	return rows
}

// findUp finds a repository file from the test's working directory upwards
// (go test runs in the package directory, Bazel in the runfiles tree).
func findUp(t *testing.T, rel string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		p := filepath.Join(dir, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("%s not found above the working directory (Bazel: is it in the test's data?)", rel)
		}
		dir = parent
	}
}

func TestPermissions_MatrixCoversTheSpecAndEveryRoute(t *testing.T) {
	spec := specRoles(t)
	if len(spec) == 0 {
		t.Fatal("found no capability rows in the spec")
	}
	have := map[string]bool{}
	for _, row := range capMatrix {
		have[row.name] = true
		cells, ok := spec[row.name]
		if !ok {
			t.Errorf("matrix row %q is not a capability row of docs/SPEC.md", row.name)
			continue
		}
		if cells != [3]bool{row.unassigned, row.developer, row.admin} {
			t.Errorf("matrix row %q says unassigned/developer/admin = %v, the spec says %v", row.name,
				[3]bool{row.unassigned, row.developer, row.admin}, cells)
		}
		if len(row.ops) == 0 && !row.noV1Route {
			t.Errorf("matrix row %q has no route", row.name)
		}
	}
	for name := range spec {
		if !have[name] {
			t.Errorf("capability row %q of docs/SPEC.md has no row in the matrix", name)
		}
	}

	covered := map[string]bool{}
	for _, row := range capMatrix {
		for _, op := range row.ops {
			covered[op] = true
		}
	}
	for op := range permExtraCover {
		covered[op] = true
	}
	for _, r := range Routes() {
		if !covered[r.OperationID] {
			t.Errorf("route %s %s (%s) is in no matrix row", r.Method, r.Pattern, r.OperationID)
		}
		if !probeOps[r.OperationID] {
			t.Errorf("route %s %s has no probe request", r.Method, r.Pattern)
		}
	}
	routeOps := map[string]bool{}
	for _, r := range Routes() {
		routeOps[r.OperationID] = true
	}
	var stale []string
	for op := range covered {
		if !routeOps[op] {
			stale = append(stale, op)
		}
	}
	for op := range probeOps {
		if !routeOps[op] {
			stale = append(stale, op)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("matrix names operations that are not routes: %v", stale)
	}
}
