// SPDX-License-Identifier: MIT

package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/holds"
	"github.com/panset/hourglass/internal/reported"
	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
)

// Handlers of the integration routes (docs/design/integration-api.md,
// "Integration routes"). They run behind auth.ServiceRoute: a service token
// with the route's scope, no cookie, no CSRF. A handler reads its caller with
// auth.ServiceFrom(r.Context()).

// Limits of a deployments report (the 1 MB body limit is MaxBodyBytes).
const (
	maxReportedDeployments = 10000
	maxReportedLabels      = 32
)

type pressureReportRequest struct {
	Value      *float64 `json:"value"`
	ObservedAt *string  `json:"observed_at"`
}

type pressureResultJSON struct {
	Value      float64 `json:"value"`
	ObservedAt string  `json:"observed_at"`
	ReceivedAt string  `json:"received_at"`
	ExpiresAt  string  `json:"expires_at"`
}

type deploymentReportRequest struct {
	Setup  string            `json:"setup"`
	Labels map[string]string `json:"labels"`
}

type deploymentsReportRequest struct {
	ObservedAt  *string                   `json:"observed_at"`
	Deployments []deploymentReportRequest `json:"deployments"`
}

type deploymentsResultJSON struct {
	ObservedAt string   `json:"observed_at"`
	ReceivedAt string   `json:"received_at"`
	ExpiresAt  string   `json:"expires_at"`
	Accepted   int      `json:"accepted"`
	Unknown    []string `json:"unknown"`
}

// parseObservedAt reads the optional observed_at of a report.
func parseObservedAt(w http.ResponseWriter, raw *string) (*time.Time, bool) {
	if raw == nil {
		return nil, true
	}
	t, err := time.Parse(time.RFC3339Nano, *raw)
	if err != nil {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "observed_at must be an RFC 3339 timestamp", "observed_at")
		return nil, false
	}
	return &t, true
}

// notConfigured answers 409 not_configured naming the config key (INT-4).
func notConfigured(w http.ResponseWriter, key string) {
	apierr.WriteCode(w, apierr.NotConfigured, fmt.Sprintf("%s is not set to reported in the server configuration; the report was not stored", key))
}

// reportPressure is PUT /api/v1/integration/pressure (scope pressure:write).
func (a *API) reportPressure(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.ServiceFrom(r.Context())
	if !ok {
		apierr.WriteCode(w, apierr.Unauthenticated, "authentication required")
		return
	}
	maxAge := a.d.Runtime.ReportedPressureMaxAge
	if maxAge <= 0 {
		notConfigured(w, "pressure.type")
		return
	}
	var req pressureReportRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Value == nil {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "value is required", "value")
		return
	}
	observed, ok := parseObservedAt(w, req.ObservedAt)
	if !ok {
		return
	}
	res, err := a.reported.SubmitPressure(r.Context(), p.AccountName, *req.Value, observed)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pressureResultJSON{
		Value: res.Value, ObservedAt: ts(res.ObservedAt), ReceivedAt: ts(res.ReceivedAt),
		ExpiresAt: ts(reported.ExpiresAt(res.ObservedAt, maxAge)),
	})
}

// reportDeployments is PUT /api/v1/integration/deployments (scope deployments:write).
func (a *API) reportDeployments(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.ServiceFrom(r.Context())
	if !ok {
		apierr.WriteCode(w, apierr.Unauthenticated, "authentication required")
		return
	}
	maxAge := a.d.Runtime.ReportedProviderMaxAge
	if maxAge <= 0 {
		notConfigured(w, "provider.type")
		return
	}
	var req deploymentsReportRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Deployments == nil {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "deployments is required (an empty list means nothing is deployed)", "deployments")
		return
	}
	if len(req.Deployments) > maxReportedDeployments {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, fmt.Sprintf("at most %d deployments per report", maxReportedDeployments), "deployments")
		return
	}
	set := make([]reported.Deployment, len(req.Deployments))
	for i, d := range req.Deployments {
		if len(d.Labels) > maxReportedLabels {
			apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, fmt.Sprintf("deployments[%d]: at most %d labels per entry", i, maxReportedLabels), "deployments")
			return
		}
		set[i] = reported.Deployment{Setup: d.Setup, Labels: d.Labels}
	}
	observed, ok := parseObservedAt(w, req.ObservedAt)
	if !ok {
		return
	}
	res, err := a.reported.SubmitDeployments(r.Context(), p.AccountName, observed, set)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	setups, err := a.d.Store.ListSetups(r.Context(), false)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	known := make(map[string]bool, len(setups))
	for _, s := range setups {
		known[s.Name] = true
	}
	out := deploymentsResultJSON{
		ObservedAt: ts(res.ObservedAt), ReceivedAt: ts(res.ReceivedAt),
		ExpiresAt: ts(reported.ExpiresAt(res.ObservedAt, maxAge)), Unknown: []string{},
	}
	for _, d := range set {
		if known[d.Setup] {
			out.Accepted++
		} else {
			out.Unknown = append(out.Unknown, d.Setup)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type stateHolderJSON struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}

type stateHoldJSON struct {
	ID                 int64           `json:"id"`
	User               stateHolderJSON `json:"user"`
	AcquiredAt         string          `json:"acquired_at"`
	MeteringStartedAt  *string         `json:"metering_started_at"`
	DeployedAt         *string         `json:"deployed_at"`
	ReleaseRequestedAt *string         `json:"release_requested_at"`
}

type stateSetupJSON struct {
	Name             string            `json:"name"`
	Labels           map[string]string `json:"labels"`
	Details          any               `json:"details"`
	Enabled          bool              `json:"enabled"`
	ReportedDeployed bool              `json:"reported_deployed"`
	Hold             *stateHoldJSON    `json:"hold"`
	Weight           *stateWeightJSON  `json:"weight"`
}

type statePressureReportedJSON struct {
	Value      float64 `json:"value"`
	ObservedAt string  `json:"observed_at"`
	ExpiresAt  string  `json:"expires_at"`
}

type integrationStateJSON struct {
	ServerTime string `json:"server_time"`
	Poll       struct {
		IntervalSeconds int     `json:"interval_seconds"`
		LastOKAt        *string `json:"last_ok_at"`
	} `json:"poll"`
	Pressure struct {
		Source     *string                    `json:"source"`
		Reported   *statePressureReportedJSON `json:"reported"`
		Smoothed   *float64                   `json:"smoothed"`
		Multiplier float64                    `json:"multiplier"`
	} `json:"pressure"`
	Deployments struct {
		Provider   *string `json:"provider"`
		ObservedAt *string `json:"observed_at"`
		ExpiresAt  *string `json:"expires_at"`
	} `json:"deployments"`
	Weights stateWeightsJSON `json:"weights"`
	Setups  []stateSetupJSON `json:"setups"`
}

// getIntegrationState is GET /api/v1/integration/state (scope state:read).
// It reads in a fixed number of queries whatever the number of setups.
func (a *API) getIntegrationState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st := a.d.Store
	set, err := settings.Load(ctx, st)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	l, err := a.loadLab(ctx)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	setups, err := st.ListSetups(ctx, false)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	active, err := st.ActiveHolds(ctx)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	users, err := a.usersByID(ctx)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	var out integrationStateJSON
	out.ServerTime = ts(l.now)
	out.Poll.IntervalSeconds = int(set.PollInterval / time.Second)
	if l.view.OK {
		t := ts(l.view.Time)
		out.Poll.LastOKAt = &t
	}
	rt := a.d.Runtime
	out.Pressure.Source = nullIfEmpty(rt.PressureSource)
	out.Pressure.Multiplier = l.mult
	if l.sample != nil {
		v := l.sample.Smoothed
		out.Pressure.Smoothed = &v
	}
	if rt.ReportedPressureMaxAge > 0 {
		switch pr, err := a.reported.Pressure(ctx); {
		case err == nil:
			out.Pressure.Reported = &statePressureReportedJSON{Value: pr.Value, ObservedAt: ts(pr.ObservedAt),
				ExpiresAt: ts(reported.ExpiresAt(pr.ObservedAt, rt.ReportedPressureMaxAge))}
		case err != reported.ErrNoReport:
			a.writeError(w, r, err)
			return
		}
	}
	out.Deployments.Provider = nullIfEmpty(rt.Provider)
	deployed := map[string]bool{}
	if rt.ReportedProviderMaxAge > 0 {
		switch dr, err := a.reported.Deployments(ctx); {
		case err == nil:
			o, e := ts(dr.ObservedAt), ts(reported.ExpiresAt(dr.ObservedAt, rt.ReportedProviderMaxAge))
			out.Deployments.ObservedAt, out.Deployments.ExpiresAt = &o, &e
			for _, d := range dr.Deployments {
				deployed[d.Setup] = true
			}
		case err != reported.ErrNoReport:
			a.writeError(w, r, err)
			return
		}
	}
	report := l.report
	out.Weights = stateWeightsTop(l.set.ReportedWeights, report)
	holdOf := make(map[int64]store.Hold, len(active))
	for _, h := range active {
		holdOf[h.SetupID] = h
	}
	out.Setups = make([]stateSetupJSON, 0, len(setups))
	for _, s := range setups {
		row := stateSetupJSON{Name: s.Name, Labels: labelsNonNil(s.Labels), Details: detailsJSON(s.DetailsJSON),
			Enabled: s.Enabled, ReportedDeployed: deployed[s.Name]}
		// The weight a poll now would charge (B4). A rate rule that cannot be
		// evaluated fails the request, as it fails the poll.
		res, err := l.resolve(s)
		if err != nil {
			a.writeError(w, r, err)
			return
		}
		row.Weight = stateWeight(res)
		if h, ok := holdOf[s.ID]; ok {
			u := users[h.UserID]
			row.Hold = &stateHoldJSON{ID: h.ID, AcquiredAt: ts(h.AcquiredAt), MeteringStartedAt: tsPtr(h.MeteringStartedAt),
				DeployedAt: tsPtr(h.DeployedAt), ReleaseRequestedAt: tsPtr(h.ReleaseRequestedAt),
				User: stateHolderJSON{ID: u.ID, Username: u.Username, Email: u.Email, Name: u.Name}}
		}
		out.Setups = append(out.Setups, row)
	}
	writeJSON(w, http.StatusOK, out)
}

// serviceReleaseHold is POST /api/v1/integration/holds/{id}/release (scope holds:release).
// The reason is required (400 validation, field reason); the teardown check is
// skipped; a pending release is completed (HOLD-8). internal/holds decides.
func (a *API) serviceReleaseHold(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.ServiceFrom(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, apierr.Unauthenticated, "a valid service token is required")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req serviceReleaseRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if a.d.Holds == nil {
		a.writeError(w, r, errors.New("api: holds service is not configured"))
		return
	}
	h, err := a.d.Holds.ReleaseByService(r.Context(), p.AccountName, id, req.Reason)
	if err != nil {
		var he *holds.Error
		if errors.As(err, &he) && he.Code == holds.CodeValidation {
			apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, he.Message, "reason")
			return
		}
		a.writeError(w, r, err)
		return
	}
	a.holdResponse(w, r, http.StatusOK, h)
}

type serviceReleaseRequest struct {
	Reason string `json:"reason"`
}
