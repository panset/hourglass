// SPDX-License-Identifier: MIT

// Package api is the HTTP layer: one http.ServeMux with every route of
// api/openapi.yaml behind the middleware its access level needs, plus
// recovery, request logging, CORS and Cache-Control.
//
// Handlers live in three places so parallel work never collides:
//
//	handlers_user.go   the "any role" data routes (me, pressure, setups, acquire, release)
//	handlers_admin.go  the admin routes
//	health.go          GET /healthz
//
// The /auth/* routes, GET /api/v1/auth/methods and GET /api/v1/session are
// served by internal/auth.
package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/collector"
	"github.com/panset/hourglass/internal/events"
	"github.com/panset/hourglass/internal/holds"
	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/store"
)

// Clock supplies the time.
type Clock interface{ Now() time.Time }

// Runtime holds the names of the configured extensions, for the read-only
// "runtime" part of the settings and collector responses (names only, never
// config blocks or secrets). Empty Provider means none is configured.
type Runtime struct {
	Provider       string
	PressureSource string
	MeteringPolicy string
	Hooks          []string
	OIDCIssuer     string // the issuer URL (not a secret); empty when OIDC is not configured
}

// Deps is everything a handler may use. Handlers read it through the *API
// receiver (a.d). Fields a handler does not need may be nil in tests; New
// itself needs Store, Auth and Logger.
type Deps struct {
	Store     *store.Store
	Auth      *auth.Service
	Holds     *holds.Service
	Ledger    *ledger.Service
	Events    *events.Dispatcher
	Collector *collector.Collector // LastPoll() is the in-memory poll view
	Clock     Clock
	Location  *time.Location // the configured time zone (month boundary, YYYY-MM-DD days)
	Logger    *slog.Logger
	Runtime   Runtime
	// Fallback serves every path that is not under /api/ or /auth/ and is
	// not /healthz (the UI, later). Nil means a plain 404.
	Fallback http.Handler
}

// API holds the dependencies; handler methods hang off it.
type API struct{ d Deps }

// New builds the router. It panics when a route of the table has no handler
// (a programming error, caught by the tests).
func New(d Deps) http.Handler {
	h, _ := build(d)
	return h
}

// registrar is the only way to add a handler to the public listener's mux:
// it records every pattern, and registrar.register holds the only
// Handle call in this package that the source scan in registration_test.go
// allows. Auth routes arrive through the same function value.
type registrar struct {
	mux      *http.ServeMux
	patterns []string
}

func (r *registrar) register(pattern string, h http.Handler) {
	r.patterns = append(r.patterns, pattern)
	r.mux.Handle(pattern, h)
}

func build(d Deps) (http.Handler, *registrar) {
	if d.Store == nil || d.Auth == nil {
		panic("api: Store and Auth are required")
	}
	if d.Logger == nil {
		d.Logger = slog.New(slog.DiscardHandler)
	}
	if d.Clock == nil {
		d.Clock = systemClock{}
	}
	if d.Location == nil {
		d.Location = time.UTC
	}
	a := &API{d: d}
	reg := &registrar{mux: http.NewServeMux()}
	d.Auth.Register(reg.register)
	hs := a.handlers()
	for _, r := range routes {
		if authServed[r.OperationID] {
			continue
		}
		h, ok := hs[r.OperationID]
		if !ok {
			panic(fmt.Sprintf("api: no handler for operation %s", r.OperationID))
		}
		reg.register(r.Method+" "+r.Pattern, a.guard(r.Access, h))
	}
	// Anything else under /api/ or /auth/ is a JSON not_found, never HTML.
	reg.register("/api/", http.HandlerFunc(notFound))
	reg.register("/auth/", http.HandlerFunc(notFound))
	if d.Fallback != nil {
		reg.register("/", d.Fallback)
	} else {
		reg.register("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	}

	// Outermost first: log, recover, no-store, CORS, session, last-login hook.
	var h http.Handler = reg.mux
	h = a.trackLocalLogin(h)
	h = d.Auth.Authenticate(h)
	h = d.Auth.CORS(h)
	h = noStore(h)
	h = a.recoverer(h)
	h = a.logRequests(h)
	return h, reg
}

// guard wraps a handler in the middleware chain of an access level. Role and
// admin routes get role/admin check, then the forced-password-change guard,
// then CSRF on state-changing methods (so 401 comes before 403, and CSRF is
// only checked for a caller that is allowed in at all).
func (a *API) guard(acc Access, h http.Handler) http.Handler {
	switch acc {
	case AccessPublic:
		return h
	case AccessRole:
		return a.d.Auth.RoleRoute(h)
	case AccessAdmin:
		return a.d.Auth.AdminRoute(h)
	default:
		panic("api: access level " + string(acc) + " is only served by internal/auth")
	}
}

// handlers maps operation ids to handler methods. Each later task replaces
// the body of its own stubs; this map is not edited by them.
func (a *API) handlers() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"getHealth": a.getHealth,
		// handlers_user.go
		"getMe":        a.getMe,
		"listMyHolds":  a.listMyHolds,
		"listMyLedger": a.listMyLedger,
		"getMyUsage":   a.getMyUsage,
		"getPressure":  a.getPressure,
		"listSetups":   a.listSetups,
		"acquireSetup": a.acquireSetup,
		"releaseHold":  a.releaseHold,
		// handlers_admin.go
		"listAdminUsers": a.listAdminUsers,
		"setUserRoles":   a.setUserRoles,
		"rejectUser":     a.rejectUser,
		"grantTokens":    a.grantTokens,
		"resetBalance":   a.resetBalance,
		"createSetup":    a.createSetup,
		"updateSetup":    a.updateSetup,
		"deleteSetup":    a.deleteSetup,
		"getRules":       a.getRules,
		"replaceRules":   a.replaceRules,
		"getSettings":    a.getSettings,
		"updateSettings": a.updateSettings,
		"getCollector":   a.getCollector,
		"listAudit":      a.listAudit,
	}
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func notFound(w http.ResponseWriter, _ *http.Request) {
	apierr.WriteCode(w, apierr.NotFound, "not found")
}

// isAPIPath reports whether a path belongs to the JSON API surface.
func isAPIPath(p string) bool {
	return p == "/healthz" || strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/auth/")
}
