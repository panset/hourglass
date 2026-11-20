// SPDX-License-Identifier: MIT

package api

import (
	"net/http"

	"github.com/panset/hourglass/internal/api/apierr"
)

// Handlers of the integration routes (docs/design/integration-api.md,
// "Integration routes"). They run behind auth.ServiceRoute: a service token
// with the route's scope, no cookie, no CSRF. A handler reads its caller with
// auth.ServiceFrom(r.Context()).
//
// The routes are registered now so that the boundary (SVC-1 to SVC-3, SVC-5,
// SVC-7, SVC-8) is enforced and tested on the complete route table. Their
// bodies answer 501 not_implemented after authentication and the scope check
// pass; the report and state routes and the service release replace them.

func notImplemented(w http.ResponseWriter, _ *http.Request) {
	apierr.WriteCode(w, apierr.NotImplemented, "not implemented yet")
}

// reportPressure is PUT /api/v1/integration/pressure (scope pressure:write).
func (a *API) reportPressure(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }

// reportDeployments is PUT /api/v1/integration/deployments (scope deployments:write).
func (a *API) reportDeployments(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }

// getIntegrationState is GET /api/v1/integration/state (scope state:read).
func (a *API) getIntegrationState(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }

// serviceReleaseHold is POST /api/v1/integration/holds/{id}/release (scope holds:release).
func (a *API) serviceReleaseHold(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
