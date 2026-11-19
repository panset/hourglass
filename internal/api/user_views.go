// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/panset/hourglass/internal/collector"
	"github.com/panset/hourglass/internal/metering"
	"github.com/panset/hourglass/internal/rules"
	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
)

// Response shapes of the user-facing routes (api/openapi.yaml schemas Hold,
// MeteringState, CostPerHour, ...). Nullable fields are pointers so that they
// are always present and null, as the schema says.

type meteringJSON struct {
	Active          bool       `json:"active"`
	StartedAt       *time.Time `json:"started_at"`
	ExpectedStartAt *time.Time `json:"expected_start_at"` // always null: the UI shows the reason text
	Reason          *string    `json:"reason"`
}

type costJSON struct {
	Now    float64 `json:"now"`
	AtPeak float64 `json:"at_peak"`
}

type userRefJSON struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type holdJSON struct {
	ID                int64             `json:"id"`
	SetupID           int64             `json:"setup_id"`
	SetupName         string            `json:"setup_name"`
	SetupLabels       map[string]string `json:"setup_labels"`
	UserID            int64             `json:"user_id"`
	Active            bool              `json:"active"`
	Weight            float64           `json:"weight"`
	AcquiredAt        time.Time         `json:"acquired_at"`
	MeteringStartedAt *time.Time        `json:"metering_started_at"`
	DeployedAt        *time.Time        `json:"deployed_at"`
	ReleasedAt        *time.Time        `json:"released_at"`
	Deployed          bool              `json:"deployed"`
	Metering          meteringJSON      `json:"metering"`
	CostPerHour       *costJSON         `json:"cost_per_hour_tokens"`
	ChargedTokens     float64           `json:"charged_tokens"`
	ReleasedBy        *userRefJSON      `json:"released_by"`
	ReleaseReason     *string           `json:"release_reason"`
	Forced            bool              `json:"forced"`
}

// utcTime returns t in UTC (RFC 3339 "Z" form on the wire).
func utcTime(t time.Time) time.Time { return t.UTC() }

func utcTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// lab is the per-request view of the lab every user route needs: settings,
// rate rules, the latest pressure sample and the collector's poll view.
type lab struct {
	now      time.Time
	set      settings.Settings
	rules    []rules.Rule
	sample   *store.PressureSample // newest pressure sample, nil before the first poll
	mult     float64               // multiplier now
	peakMult float64               // highest band's multiplier
	view     collector.PollView
}

func (a *API) loadLab(ctx context.Context) (*lab, error) {
	st := a.d.Store
	set, err := settings.Load(ctx, st)
	if err != nil {
		return nil, err
	}
	rows, err := st.ListRules(ctx, rules.KindRate)
	if err != nil {
		return nil, err
	}
	l := &lab{now: a.d.Clock.Now().UTC(), set: set, peakMult: set.Bands.Max()}
	for _, r := range rows {
		l.rules = append(l.rules, rules.Rule{ID: r.ID, Kind: r.Kind, Position: r.Position, Selector: r.Selector,
			Value: strconv.FormatFloat(r.Value, 'f', -1, 64)})
	}
	latest, err := st.LatestPressure(ctx, 1)
	if err != nil {
		return nil, err
	}
	if len(latest) > 0 {
		l.sample = &latest[0]
		l.mult = latest[0].Multiplier
	} else {
		l.mult = set.Bands.Multiplier(0)
	}
	if a.d.Collector != nil {
		l.view = a.d.Collector.LastPoll()
	}
	return l, nil
}

// weight resolves a setup's weight from the rate rules; selector is "" when
// the default weight applied.
func (l *lab) weight(s store.Setup) (w float64, selector string, err error) {
	w, selector, _, err = rules.Weight(l.rules, s.Labels, l.set.DefaultWeight)
	return w, selector, err
}

func (l *lab) resourceName(s store.Setup) (string, error) {
	return settings.ResourceName(l.set.ResourceNameTemplate, s.Name, s.Labels, s.ResourceNameOverride)
}

// reported reports whether the provider reported the setup's resource in the
// latest successful poll.
func (l *lab) reported(s store.Setup) (bool, error) {
	if !l.view.OK {
		return false, nil
	}
	name, err := l.resourceName(s)
	if err != nil {
		return false, err
	}
	return l.view.Reported(name), nil
}

func (l *lab) cost(weight float64) costJSON {
	return costJSON{
		Now:    tokens(metering.HourlyCostMT(weight, l.mult)),
		AtPeak: tokens(metering.HourlyCostMT(weight, l.peakMult)),
	}
}

// holdData is what the hold views need beyond the hold rows themselves.
type holdData struct {
	setups  map[int64]store.Setup
	users   map[int64]store.User
	charges map[int64]int64
	samples map[int64]store.LastSample
}

func (a *API) loadHoldData(ctx context.Context, holds []store.Hold) (*holdData, error) {
	st := a.d.Store
	ids := make([]int64, len(holds))
	for i, h := range holds {
		ids[i] = h.ID
	}
	d := &holdData{setups: map[int64]store.Setup{}, users: map[int64]store.User{}}
	setups, err := st.ListSetups(ctx, false)
	if err != nil {
		return nil, err
	}
	for _, s := range setups {
		d.setups[s.ID] = s
	}
	users, err := st.ListUsers(ctx, "")
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		d.users[u.ID] = u
	}
	if d.charges, err = st.HoldCharges(ctx, ids); err != nil {
		return nil, err
	}
	if d.samples, err = st.HoldLastSamples(ctx, ids); err != nil {
		return nil, err
	}
	return d, nil
}

// holdWeight is the weight a hold is charged with: what the policy last
// decided with (it may override the rate rules), else the resolved weight.
func (l *lab) holdWeight(h store.Hold, s store.Setup, d *holdData) (float64, error) {
	if ls, ok := d.samples[h.ID]; ok {
		return ls.Weight, nil
	}
	w, _, err := l.weight(s)
	return w, err
}

// meteringOf is a hold's metering state. The reason is the metering policy's
// own words from the latest successful poll that saw the hold; null before
// such a poll (and for a released hold).
func meteringOf(h store.Hold, view collector.PollView) meteringJSON {
	m := meteringJSON{Active: h.MeteringStartedAt != nil, StartedAt: utcTimePtr(h.MeteringStartedAt)}
	if dec, ok := view.Decisions[h.ID]; ok && h.ReleasedAt == nil {
		m.Reason = nullIfEmpty(dec.Reason)
	}
	return m
}

// holdViews builds the Hold objects of the OpenAPI for the given rows, in
// order.
func (a *API) holdViews(ctx context.Context, l *lab, holds []store.Hold) ([]holdJSON, error) {
	d, err := a.loadHoldData(ctx, holds)
	if err != nil {
		return nil, err
	}
	out := make([]holdJSON, 0, len(holds))
	for _, h := range holds {
		s := d.setups[h.SetupID]
		w, err := l.holdWeight(h, s, d)
		if err != nil {
			return nil, err
		}
		v := holdJSON{
			ID: h.ID, SetupID: h.SetupID, SetupName: s.Name, SetupLabels: labelsNonNil(s.Labels), UserID: h.UserID,
			Active: h.ReleasedAt == nil, Weight: w, AcquiredAt: utcTime(h.AcquiredAt),
			MeteringStartedAt: utcTimePtr(h.MeteringStartedAt), DeployedAt: utcTimePtr(h.DeployedAt), ReleasedAt: utcTimePtr(h.ReleasedAt),
			Metering:      meteringOf(h, l.view),
			ChargedTokens: tokens(d.charges[h.ID]),
			ReleaseReason: nullIfEmpty(h.ReleaseReason),
		}
		if v.Active {
			if v.Deployed, err = l.reported(s); err != nil {
				return nil, err
			}
			c := l.cost(w)
			v.CostPerHour = &c
		}
		if h.ReleasedBy != nil {
			ru := d.users[*h.ReleasedBy]
			v.ReleasedBy = &userRefJSON{ID: *h.ReleasedBy, Name: userDisplayName(ru)}
			v.Forced = *h.ReleasedBy != h.UserID
		}
		out = append(out, v)
	}
	return out, nil
}

// detailsJSON returns stored details unmodified (SET-2); an empty value is null.
func detailsJSON(s string) json.RawMessage {
	if s == "" {
		return json.RawMessage("null")
	}
	return json.RawMessage(s)
}
