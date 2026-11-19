// SPDX-License-Identifier: MIT

package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
)

// Sizes of the lists on the collector page (OpenAPI: up to 20 runs and
// failed deliveries; the outage history is bounded the same way).
const (
	collectorListLimit = 20
	outageListLimit    = 50
)

type collectorExtensions struct {
	Provider       *string  `json:"provider"`
	PressureSource string   `json:"pressure_source"`
	MeteringPolicy string   `json:"metering_policy"`
	Hooks          []string `json:"hooks"`
}

type collectorRunOut struct {
	ID         int64   `json:"id"`
	StartedAt  string  `json:"started_at"`
	FinishedAt *string `json:"finished_at"`
	OK         bool    `json:"ok"`
	Error      *string `json:"error"`
}

type collectorLastError struct {
	Message string `json:"message"`
	At      string `json:"at"`
}

type collectorOutageOut struct {
	StartedAt string  `json:"started_at"`
	EndedAt   *string `json:"ended_at"`
	Minutes   int     `json:"duration_minutes"`
}

type collectorOrphanSetup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type collectorOrphanOut struct {
	ResourceName string                `json:"resource_name"`
	FirstSeenAt  string                `json:"first_seen_at"`
	LastSeenAt   string                `json:"last_seen_at"`
	Setup        *collectorOrphanSetup `json:"setup"`
	Labels       map[string]string     `json:"labels"`
}

type collectorDeliveryOut struct {
	ID         int64   `json:"id"`
	EventType  string  `json:"event_type"`
	Hook       string  `json:"hook"`
	Error      string  `json:"error"`
	StartedAt  string  `json:"started_at"`
	FinishedAt *string `json:"finished_at"`
}

type collectorOutagesMonth struct {
	Count   int `json:"count"`
	Minutes int `json:"minutes_not_charged"`
}

type collectorOut struct {
	Extensions           collectorExtensions    `json:"extensions"`
	LastSuccessfulPollAt *string                `json:"last_successful_poll_at"`
	LastError            *collectorLastError    `json:"last_error"`
	MatchedSetups        int                    `json:"matched_setups"`
	OrphanCount          int                    `json:"orphan_count"`
	OutagesThisMonth     collectorOutagesMonth  `json:"outages_this_month"`
	RecentRuns           []collectorRunOut      `json:"recent_runs"`
	Outages              []collectorOutageOut   `json:"outages"`
	Orphans              []collectorOrphanOut   `json:"orphans"`
	FailedDeliveries     []collectorDeliveryOut `json:"failed_deliveries"`
	QueueLength          int                    `json:"queue_length"`
}

// collectorView builds the Collector response (ADM-4, HOLD-4, EXT-6, EXT-7).
func (a *API) collectorView(r *http.Request) (collectorOut, error) {
	ctx := r.Context()
	st := a.d.Store
	now := a.d.Clock.Now()
	rt := a.runtimeView()
	out := collectorOut{
		Extensions: collectorExtensions{Provider: rt.Provider, PressureSource: rt.PressureSource,
			MeteringPolicy: rt.MeteringPolicy.Name, Hooks: rt.Hooks},
	}
	cfg, err := settings.Load(ctx, st)
	if err != nil {
		return out, err
	}

	runs, err := st.ListCollectorRuns(ctx, collectorListLimit)
	if err != nil {
		return out, err
	}
	out.RecentRuns = make([]collectorRunOut, len(runs))
	for i, x := range runs {
		out.RecentRuns[i] = collectorRunOut{ID: x.ID, StartedAt: ts(x.StartedAt), FinishedAt: tsPtr(x.FinishedAt),
			OK: x.OK != nil && *x.OK, Error: nullIfEmpty(x.Error)}
	}

	switch run, err := st.LastSuccessfulRun(ctx); {
	case err == nil:
		out.LastSuccessfulPollAt = nullIfEmpty(ts(run.StartedAt))
	case !isNotFound(err):
		return out, err
	}
	switch run, err := st.LastFailedRun(ctx); {
	case err == nil:
		at := run.StartedAt
		if run.FinishedAt != nil {
			at = *run.FinishedAt
		}
		out.LastError = &collectorLastError{Message: run.Error, At: ts(at)}
	case !isNotFound(err):
		return out, err
	}

	monthStart := ledger.PeriodStart(ledger.PeriodOf(now, a.d.Location), a.d.Location)
	monthOutages, err := st.ListOutagesSince(ctx, monthStart)
	if err != nil {
		return out, err
	}
	for _, o := range monthOutages {
		out.OutagesThisMonth.Count++
		out.OutagesThisMonth.Minutes += outageMinutes(o)
	}
	outs, err := st.ListOutages(ctx, outageListLimit)
	if err != nil {
		return out, err
	}
	out.Outages = make([]collectorOutageOut, len(outs))
	for i, o := range outs {
		e := ts(o.EndedAt)
		out.Outages[i] = collectorOutageOut{StartedAt: ts(o.StartedAt), EndedAt: &e, Minutes: outageMinutes(o)}
	}

	setups, err := st.ListSetups(ctx, false)
	if err != nil {
		return out, err
	}
	byResource := map[string]store.Setup{}
	poll := a.pollView()
	for _, s := range setups {
		name, err := settings.ResourceName(cfg.ResourceNameTemplate, s.Name, s.Labels, s.ResourceNameOverride)
		if err != nil {
			continue
		}
		byResource[name] = s
		if s.Enabled && poll.OK && poll.Reported(name) {
			out.MatchedSetups++
		}
	}
	orphans, err := st.ListOrphans(ctx)
	if err != nil {
		return out, err
	}
	out.OrphanCount = len(orphans)
	out.Orphans = make([]collectorOrphanOut, len(orphans))
	for i, o := range orphans {
		labels := map[string]string{}
		_ = json.Unmarshal([]byte(o.LabelsJSON), &labels) // provider-specific; non-string labels are dropped
		x := collectorOrphanOut{ResourceName: o.ResourceName, FirstSeenAt: ts(o.FirstSeenAt), LastSeenAt: ts(o.LastSeenAt), Labels: labelsNonNil(labels)}
		if s, ok := byResource[o.ResourceName]; ok {
			x.Setup = &collectorOrphanSetup{ID: s.ID, Name: s.Name}
		}
		out.Orphans[i] = x
	}

	dels, err := st.ListFailedDeliveries(ctx, collectorListLimit)
	if err != nil {
		return out, err
	}
	out.FailedDeliveries = make([]collectorDeliveryOut, len(dels))
	for i, d := range dels {
		out.FailedDeliveries[i] = collectorDeliveryOut{ID: d.ID, EventType: d.EventType, Hook: d.Hook, Error: d.Error,
			StartedAt: ts(d.StartedAt), FinishedAt: tsPtr(d.FinishedAt)}
	}
	if a.d.Events != nil {
		out.QueueLength = a.d.Events.Pending()
	}
	return out, nil
}

func outageMinutes(o store.Outage) int {
	d := o.EndedAt.Sub(o.StartedAt)
	if d < 0 {
		return 0
	}
	return int(d / time.Minute)
}

type auditActor struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type auditEntryOut struct {
	ID        int64          `json:"id"`
	Actor     auditActor     `json:"actor"`
	Action    string         `json:"action"`
	Target    string         `json:"target"`
	Before    map[string]any `json:"before"`
	After     map[string]any `json:"after"`
	CreatedAt string         `json:"created_at"`
}

type auditListOut struct {
	Entries []auditEntryOut `json:"entries"`
	Total   int             `json:"total"`
	Limit   int             `json:"limit"`
	Offset  int             `json:"offset"`
}

// auditView builds the AuditList response (ADM-3).
func (a *API) auditView(r *http.Request, f store.AuditFilter) (auditListOut, error) {
	ctx := r.Context()
	res := auditListOut{Limit: f.Limit, Offset: f.Offset, Entries: []auditEntryOut{}}
	entries, err := a.d.Store.ListAudit(ctx, f)
	if err != nil {
		return res, err
	}
	if res.Total, err = a.d.Store.CountAudit(ctx, f); err != nil {
		return res, err
	}
	names := map[int64]string{}
	nameOf := func(id int64) (string, error) {
		if n, ok := names[id]; ok {
			return n, nil
		}
		u, err := a.d.Store.GetUser(ctx, id)
		switch {
		case err == nil:
			names[id] = u.Name
		case isNotFound(err):
			names[id] = "unknown user"
		default:
			return "", err
		}
		return names[id], nil
	}
	for _, e := range entries {
		x := auditEntryOut{ID: e.ID, Action: e.Action, Target: e.Target, CreatedAt: ts(e.CreatedAt),
			Before: decodeObject(e.BeforeJSON), After: decodeObject(e.AfterJSON)}
		if e.ActorID == nil {
			x.Actor = auditActor{Name: "system"}
		} else {
			n, err := nameOf(*e.ActorID)
			if err != nil {
				return res, err
			}
			x.Actor = auditActor{ID: *e.ActorID, Name: n}
		}
		res.Entries = append(res.Entries, x)
	}
	return res, nil
}

// decodeObject parses a stored audit JSON object; an empty value is null. Any
// other JSON value (an older row that stored a bare array) is returned as
// {"value": ...} rather than dropped.
func decodeObject(s string) map[string]any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil || v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{"value": v}
}
