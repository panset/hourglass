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
	"github.com/panset/hourglass/internal/weights"
)

// Response shapes of the user-facing routes (api/openapi.yaml schemas Hold,
// MeteringState, CostPerHour, ...). Nullable fields are pointers so that they
// are always present and null, as the schema says.

type meteringJSON struct {
	Active          bool    `json:"active"`
	StartedAt       *string `json:"started_at"`
	ExpectedStartAt *string `json:"expected_start_at"` // always null: the UI shows the reason text
	Reason          *string `json:"reason"`
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
	ID                 int64             `json:"id"`
	SetupID            int64             `json:"setup_id"`
	SetupName          string            `json:"setup_name"`
	SetupLabels        map[string]string `json:"setup_labels"`
	UserID             int64             `json:"user_id"`
	Active             bool              `json:"active"`
	Weight             float64           `json:"weight"`
	WeightSource       *string           `json:"weight_source"`
	WeightAttributes   map[string]string `json:"weight_attributes"`
	AcquiredAt         string            `json:"acquired_at"`
	MeteringStartedAt  *string           `json:"metering_started_at"`
	DeployedAt         *string           `json:"deployed_at"`
	ReleaseRequestedAt *string           `json:"release_requested_at"`
	ReleasedAt         *string           `json:"released_at"`
	Deployed           bool              `json:"deployed"`
	Metering           meteringJSON      `json:"metering"`
	CostPerHour        *costJSON         `json:"cost_per_hour_tokens"`
	ChargedTokens      float64           `json:"charged_tokens"`
	ReleasedBy         *userRefJSON      `json:"released_by"`
	ReleaseReason      *string           `json:"release_reason"`
	Forced             bool              `json:"forced"`
}

// lab is the per-request view of the lab every user route needs: settings,
// rate rules, the stored weights report, the latest pressure sample and the
// collector's poll view.
type lab struct {
	now      time.Time
	set      settings.Settings
	rules    []rules.Rule
	report   *weights.Report       // nil when reported weights are off or before the first report
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
	if l.report, err = a.weightReport(ctx, set.ReportedWeights); err != nil {
		return nil, err
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

// resolve is the weight resolver's answer for a setup at the request's time
// (A1, A3). A rate rule that cannot be evaluated fails the request.
func (l *lab) resolve(s store.Setup) (weights.Result, error) {
	return weights.Resolve(resolverSetup(s), l.rules, l.set, l.report, l.now)
}

// reported reports whether the provider reported an allocation with the
// setup's exact name (D40) in the latest successful poll.
func (l *lab) reported(s store.Setup) bool {
	return l.view.OK && l.view.Reported(s.Name)
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

// usersByID reads every user once, keyed by id.
func (a *API) usersByID(ctx context.Context) (map[int64]store.User, error) {
	users, err := a.d.Store.ListUsers(ctx, "")
	if err != nil {
		return nil, err
	}
	out := make(map[int64]store.User, len(users))
	for _, u := range users {
		out[u.ID] = u
	}
	return out, nil
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
	if d.users, err = a.usersByID(ctx); err != nil {
		return nil, err
	}
	if d.charges, err = st.HoldCharges(ctx, ids); err != nil {
		return nil, err
	}
	if d.samples, err = st.HoldLastSamples(ctx, ids); err != nil {
		return nil, err
	}
	return d, nil
}

// shownWeight is the weight, source and attributes a hold or a burn figure
// shows (A3). Source is nil only for a released hold whose last sample was
// written before migration 0008.
type shownWeight struct {
	weight     float64
	source     *string
	attributes map[string]string
}

// holdWeight is the weight a hold shows (A3). An active hold shows what its
// setup resolves to now, except when its latest usage sample's source is
// policy: then that sample's weight and source. A released hold shows the
// weight and source of its last sample, what it was last charged at, with no
// attributes (a sample does not keep them); without a sample it shows what
// its setup resolves to now.
func (l *lab) holdWeight(h store.Hold, s store.Setup, d *holdData) (shownWeight, error) {
	ls, sampled := d.samples[h.ID]
	if h.ReleasedAt != nil && sampled {
		return shownWeight{weight: ls.Weight, source: nullIfEmpty(ls.WeightSource), attributes: map[string]string{}}, nil
	}
	res, err := l.resolve(s)
	if err != nil {
		return shownWeight{}, err
	}
	return activeWeight(res, ls, sampled), nil
}

// activeWeight is the weight of an active hold from the resolver's answer and
// the hold's latest sample, if any (A3). A sample with no source counts as not
// policy.
func activeWeight(res weights.Result, ls store.LastSample, sampled bool) shownWeight {
	if sampled && ls.WeightSource == string(weights.SourcePolicy) {
		src := ls.WeightSource
		return shownWeight{weight: ls.Weight, source: &src, attributes: res.Attributes}
	}
	src := string(res.Source)
	return shownWeight{weight: res.Weight, source: &src, attributes: res.Attributes}
}

// resolverSetup is what the resolver needs of a stored setup.
func resolverSetup(s store.Setup) weights.Setup {
	return weights.Setup{Name: s.Name, Labels: s.Labels, Override: s.WeightOverride}
}

// meteringOf is a hold's metering state. The reason is the metering policy's
// own words from the latest successful poll that saw the hold; null before
// such a poll (and for a released hold).
func meteringOf(h store.Hold, view collector.PollView) meteringJSON {
	m := meteringJSON{Active: h.MeteringStartedAt != nil, StartedAt: tsPtr(h.MeteringStartedAt)}
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
		sw, err := l.holdWeight(h, s, d)
		if err != nil {
			return nil, err
		}
		w := sw.weight
		v := holdJSON{
			ID: h.ID, SetupID: h.SetupID, SetupName: s.Name, SetupLabels: labelsNonNil(s.Labels), UserID: h.UserID,
			Active: h.ReleasedAt == nil, Weight: w, WeightSource: sw.source, WeightAttributes: sw.attributes,
			AcquiredAt:        ts(h.AcquiredAt),
			MeteringStartedAt: tsPtr(h.MeteringStartedAt), DeployedAt: tsPtr(h.DeployedAt),
			ReleaseRequestedAt: tsPtr(h.ReleaseRequestedAt), ReleasedAt: tsPtr(h.ReleasedAt),
			Metering:      meteringOf(h, l.view),
			ChargedTokens: tokens(d.charges[h.ID]),
			ReleaseReason: nullIfEmpty(h.ReleaseReason),
		}
		if v.Active {
			v.Deployed = l.reported(s)
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
