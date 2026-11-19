// SPDX-License-Identifier: MIT

package api

import "net/http"

// Handlers of the "any role" data routes. This file belongs to the task that
// implements the user-facing data API (me, me/holds, me/ledger, me/usage,
// pressure, setups, acquire, release). Every handler here is a stub that
// answers 501 until that task replaces its body; the method names and
// signatures are fixed because the router refers to them.
//
// Each handler runs behind the role middleware chain (role, password guard,
// CSRF), so it may rely on auth.UserFrom(r.Context()) being an active user
// with at least one role.

// getMe is GET /api/v1/me.
func (a *API) getMe(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// listMyHolds is GET /api/v1/me/holds.
func (a *API) listMyHolds(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// listMyLedger is GET /api/v1/me/ledger.
func (a *API) listMyLedger(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// getMyUsage is GET /api/v1/me/usage.
func (a *API) getMyUsage(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// getPressure is GET /api/v1/pressure.
func (a *API) getPressure(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// listSetups is GET /api/v1/setups.
func (a *API) listSetups(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// acquireSetup is POST /api/v1/setups/{id}/acquire.
func (a *API) acquireSetup(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// releaseHold is POST /api/v1/holds/{id}/release (owner release and admin
// force-release).
func (a *API) releaseHold(w http.ResponseWriter, r *http.Request) { notImplemented(w) }
