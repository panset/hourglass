// SPDX-License-Identifier: MIT

package api

// Access is the who-column of the spec's HTTP API table, spelled like the
// x-roles values of api/openapi.yaml.
type Access string

// Access levels.
const (
	AccessPublic     Access = "public"      // no session needed
	AccessSession    Access = "session"     // any signed-in user, even one with no role
	AccessRole       Access = "role"        // at least one role (AUTH-4)
	AccessAdmin      Access = "admin"       // the admin role
	AccessLocalAdmin Access = "local-admin" // the permanent local admin account
)

// Route is one operation of api/openapi.yaml.
type Route struct {
	Method      string
	Pattern     string // ServeMux path pattern, e.g. /api/v1/setups/{id}/acquire
	Access      Access
	OperationID string
}

// StateChanging reports whether the route requires the CSRF token (AUTH-3):
// every method except GET, HEAD and OPTIONS, except the public pre-session
// sign-in call, which has no session to carry a token.
func (r Route) StateChanging() bool {
	if r.Access == AccessPublic {
		return false
	}
	return r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS"
}

// routes is the single source for registration (New) and for the contract
// test against api/openapi.yaml. Order follows the spec's HTTP API table.
var routes = []Route{
	{"GET", "/auth/oidc/start", AccessPublic, "oidcStart"},
	{"GET", "/auth/callback", AccessPublic, "oidcCallback"},
	{"POST", "/auth/local", AccessPublic, "localSignIn"},
	{"POST", "/auth/local/password", AccessLocalAdmin, "changeLocalPassword"},
	{"POST", "/auth/logout", AccessSession, "logout"},
	{"GET", "/healthz", AccessPublic, "getHealth"},
	{"GET", "/api/v1/auth/methods", AccessPublic, "getAuthMethods"},
	{"GET", "/api/v1/session", AccessSession, "getSession"},
	{"GET", "/api/v1/me", AccessRole, "getMe"},
	{"GET", "/api/v1/me/holds", AccessRole, "listMyHolds"},
	{"GET", "/api/v1/me/ledger", AccessRole, "listMyLedger"},
	{"GET", "/api/v1/me/usage", AccessRole, "getMyUsage"},
	{"GET", "/api/v1/pressure", AccessRole, "getPressure"},
	{"GET", "/api/v1/setups", AccessRole, "listSetups"},
	{"POST", "/api/v1/setups/{id}/acquire", AccessRole, "acquireSetup"},
	{"POST", "/api/v1/holds/{id}/release", AccessRole, "releaseHold"},
	{"GET", "/api/v1/admin/users", AccessAdmin, "listAdminUsers"},
	{"PUT", "/api/v1/admin/users/{id}/roles", AccessAdmin, "setUserRoles"},
	{"POST", "/api/v1/admin/users/{id}/reject", AccessAdmin, "rejectUser"},
	{"POST", "/api/v1/admin/users/{id}/grants", AccessAdmin, "grantTokens"},
	{"POST", "/api/v1/admin/users/{id}/reset", AccessAdmin, "resetBalance"},
	{"POST", "/api/v1/admin/setups", AccessAdmin, "createSetup"},
	{"PUT", "/api/v1/admin/setups/{id}", AccessAdmin, "updateSetup"},
	{"DELETE", "/api/v1/admin/setups/{id}", AccessAdmin, "deleteSetup"},
	{"GET", "/api/v1/admin/rules", AccessAdmin, "getRules"},
	{"PUT", "/api/v1/admin/rules", AccessAdmin, "replaceRules"},
	{"GET", "/api/v1/admin/settings", AccessAdmin, "getSettings"},
	{"PUT", "/api/v1/admin/settings", AccessAdmin, "updateSettings"},
	{"GET", "/api/v1/admin/collector", AccessAdmin, "getCollector"},
	{"GET", "/api/v1/admin/audit", AccessAdmin, "listAudit"},
}

// Routes returns a copy of the route table.
func Routes() []Route { return append([]Route(nil), routes...) }

// authServed are the operations internal/auth mounts itself through
// (*auth.Service).Register, with its own middleware chain. New skips them
// when registering handlers; they stay in the table so the contract test and
// the router test see every route.
var authServed = map[string]bool{
	"oidcStart": true, "oidcCallback": true, "localSignIn": true,
	"changeLocalPassword": true, "logout": true, "getAuthMethods": true, "getSession": true,
}
