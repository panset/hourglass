// SPDX-License-Identifier: MIT

package api

import "net/http"

// Handlers of the admin routes. This file belongs to the task that
// implements the admin API (users, roles, reject, grants, reset, admin
// setups, rules, settings, collector, audit). Every handler here is a stub
// that answers 501 until that task replaces its body; the method names and
// signatures are fixed because the router refers to them.
//
// Each handler runs behind the admin middleware chain (admin role, password
// guard, CSRF).

// listAdminUsers is GET /api/v1/admin/users.
func (a *API) listAdminUsers(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// setUserRoles is PUT /api/v1/admin/users/{id}/roles.
func (a *API) setUserRoles(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// rejectUser is POST /api/v1/admin/users/{id}/reject.
func (a *API) rejectUser(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// grantTokens is POST /api/v1/admin/users/{id}/grants.
func (a *API) grantTokens(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// resetBalance is POST /api/v1/admin/users/{id}/reset.
func (a *API) resetBalance(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// createSetup is POST /api/v1/admin/setups.
func (a *API) createSetup(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// updateSetup is PUT /api/v1/admin/setups/{id}.
func (a *API) updateSetup(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// deleteSetup is DELETE /api/v1/admin/setups/{id}.
func (a *API) deleteSetup(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// getRules is GET /api/v1/admin/rules.
func (a *API) getRules(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// replaceRules is PUT /api/v1/admin/rules.
func (a *API) replaceRules(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// getSettings is GET /api/v1/admin/settings.
func (a *API) getSettings(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// updateSettings is PUT /api/v1/admin/settings.
func (a *API) updateSettings(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// getCollector is GET /api/v1/admin/collector.
func (a *API) getCollector(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

// listAudit is GET /api/v1/admin/audit.
func (a *API) listAudit(w http.ResponseWriter, r *http.Request) { notImplemented(w) }
