// SPDX-License-Identifier: MIT

package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/panset/hourglass/internal/store"
)

// healthResponse is the openapi Health schema.
type healthResponse struct {
	Status             string `json:"status"` // ok | degraded
	Database           string `json:"database"`
	LastPollAgeSeconds *int64 `json:"last_poll_age_seconds"` // null before the first successful poll
}

// getHealth is GET /healthz (non-functional requirements, "Observability"):
// 200 when the database answers, 503 with the same body when it does not.
// The age is that of the last successful poll recorded in the database, so
// it survives a restart. It reveals no user or usage data.
func (a *API) getHealth(w http.ResponseWriter, r *http.Request) {
	res := healthResponse{Status: "ok", Database: "ok"}
	run, err := a.d.Store.LastSuccessfulRun(r.Context())
	switch {
	case err == nil:
		age := int64(a.d.Clock.Now().Sub(run.StartedAt) / time.Second)
		if age < 0 {
			age = 0
		}
		res.LastPollAgeSeconds = &age
	case errors.Is(err, store.ErrNotFound):
		// reachable, no successful poll yet
	default:
		a.d.Logger.Error("health check: database unreachable", "err", err)
		res.Status, res.Database = "degraded", "error"
		writeJSON(w, http.StatusServiceUnavailable, res)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
