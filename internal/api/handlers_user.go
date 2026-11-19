// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/holds"
	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/metering"
	"github.com/panset/hourglass/internal/rules"
	"github.com/panset/hourglass/internal/store"
)

// Handlers of the "any role" data routes (HOLD-1, HOLD-2, SET-6, LED-2, UI-6).
// Each runs behind the role middleware chain (role, password guard, CSRF), so
// auth.UserFrom(r.Context()) is an active user with at least one role. Every
// number is computed here or in the service packages (UI-1); the response
// shapes are the schemas of api/openapi.yaml.

// userCaller returns the signed-in user. The middleware guarantees one; a miss is
// answered as 401 rather than a panic.
func userCaller(w http.ResponseWriter, r *http.Request) (*auth.User, bool) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		apierr.WriteCode(w, apierr.Unauthenticated, "sign in required")
		return nil, false
	}
	return u, true
}

// ---- GET /api/v1/me ----

type burningJSON struct {
	HoldID     int64   `json:"hold_id"`
	SetupID    int64   `json:"setup_id"`
	SetupName  string  `json:"setup_name"`
	Weight     float64 `json:"weight"`
	Multiplier float64 `json:"multiplier"`
	TokensHour float64 `json:"tokens_per_hour"`
}

type meJSON struct {
	Period            string        `json:"period"`
	PeriodEndsAt      string        `json:"period_ends_at"`
	Balance           *float64      `json:"balance_tokens"`
	Allowance         float64       `json:"allowance_tokens"`
	Granted           float64       `json:"granted_tokens"`
	Carryover         float64       `json:"carryover_tokens"`
	Debited           float64       `json:"debited_tokens"`
	BurnPerHour       float64       `json:"burn_per_hour_tokens"`
	Burning           []burningJSON `json:"burning"`
	Projection        string        `json:"projection"`
	ProjectedRunoutAt *string       `json:"projected_runout_at"`
	HoldCount         int           `json:"hold_count"`
	HoldLimit         int           `json:"hold_limit"`
	Enforcement       bool          `json:"enforcement"`
	Overdrawn         bool          `json:"overdrawn"`
	// CanAcquire and CannotAcquireReason come from holds.AcquireBlock, the rule
	// Acquire itself applies (UI-1): the UI shows them and decides nothing.
	CanAcquire          bool    `json:"can_acquire"`
	CannotAcquireReason *string `json:"cannot_acquire_reason"`
}

// Projection values of the Me schema.
const (
	projectionDate         = "date"
	projectionNotThisMonth = "not_this_month"
	projectionNotBurning   = "not_burning"
)

// getMe is GET /api/v1/me. Balance and debits come from the ledger sums of
// the current period (LED-2); the burn is weight x current multiplier summed
// over the user's metering holds; the projection is
// metering.ProjectedRunOut over the last 24 hours of debits.
func (a *API) getMe(w http.ResponseWriter, r *http.Request) {
	u, ok := userCaller(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	l, err := a.loadLab(ctx)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	period := ledger.PeriodOf(l.now, a.d.Location)
	periodEnd := ledger.NextBoundary(l.now, a.d.Location)
	me := meJSON{
		Period:       period,
		PeriodEndsAt: ts(periodEnd),
		Burning:      []burningJSON{},
		Projection:   projectionNotBurning,
		Enforcement:  l.set.Enforcement,
	}
	if auth.IsLocalAdmin(u.User) {
		// No balance, no allowance, cannot hold (LED-1).
		me.CannotAcquireReason = nullIfEmpty(holds.AcquireBlock(true, l.set.Enforcement, 0, 0, 0))
		writeJSON(w, http.StatusOK, me)
		return
	}
	st := a.d.Store
	sum := func(typ string) int64 {
		n, e := st.SumLedger(ctx, u.ID, period, typ)
		if e != nil && err == nil {
			err = e
		}
		return n
	}
	balance, granted, carry, debits, allowance := sum(""), sum(string(ledger.TypeGrant)), sum(string(ledger.TypeCarryover)),
		sum(string(ledger.TypeDebit)), sum(string(ledger.TypeAllowance))
	burn24, err2 := st.SumDebitsSince(ctx, u.ID, l.now.Add(-24*time.Hour))
	if err == nil {
		err = err2
	}
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	if allowance <= 0 {
		allowance = l.set.DefaultAllowanceMT
	}
	count, err := st.CountActiveHolds(ctx, u.ID)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	all, err := st.UserHolds(ctx, u.ID)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	var active []store.Hold
	for _, h := range all {
		if h.ReleasedAt == nil {
			active = append(active, h)
		}
	}
	d, err := a.loadHoldData(ctx, active)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	var burnMT int64
	for _, h := range active {
		if h.MeteringStartedAt == nil {
			continue
		}
		s := d.setups[h.SetupID]
		wt, err := l.holdWeight(h, s, d)
		if err != nil {
			a.writeError(w, r, err)
			return
		}
		mt := metering.HourlyCostMT(wt, l.mult)
		burnMT += mt
		me.Burning = append(me.Burning, burningJSON{HoldID: h.ID, SetupID: h.SetupID, SetupName: s.Name,
			Weight: wt, Multiplier: l.mult, TokensHour: tokens(mt)})
	}
	b := tokens(balance)
	me.Balance, me.Allowance, me.Granted, me.Carryover, me.Debited = &b, tokens(allowance), tokens(granted), tokens(carry), tokens(-debits)
	me.BurnPerHour = tokens(burnMT)
	me.HoldCount, me.HoldLimit = count, l.set.MaxHoldsPerUser
	me.Overdrawn = balance <= 0
	reason := holds.AcquireBlock(false, l.set.Enforcement, balance, count, l.set.MaxHoldsPerUser)
	me.CanAcquire, me.CannotAcquireReason = reason == "", nullIfEmpty(reason)
	if at, ok := metering.ProjectedRunOut(balance, burn24, l.now); ok {
		if at.Before(periodEnd) {
			me.Projection, me.ProjectedRunoutAt = projectionDate, tsPtr(&at)
		} else {
			me.Projection = projectionNotThisMonth
		}
	}
	writeJSON(w, http.StatusOK, me)
}

// ---- GET /api/v1/me/holds ----

// listMyHolds is GET /api/v1/me/holds[?state=active|released], newest first.
func (a *API) listMyHolds(w http.ResponseWriter, r *http.Request) {
	u, ok := userCaller(w, r)
	if !ok {
		return
	}
	state := r.URL.Query().Get("state")
	if state != "" && state != "active" && state != "released" {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "state must be active or released", "state")
		return
	}
	ctx := r.Context()
	rows, err := a.d.Store.UserHolds(ctx, u.ID)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	var keep []store.Hold
	for _, h := range rows {
		if state == "" || (state == "active") == (h.ReleasedAt == nil) {
			keep = append(keep, h)
		}
	}
	l, err := a.loadLab(ctx)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	views, err := a.holdViews(ctx, l, keep)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"holds": views})
}

// ---- GET /api/v1/me/ledger ----

type ledgerEntryJSON struct {
	ID           int64    `json:"id"`
	CreatedAt    string   `json:"created_at"`
	Period       string   `json:"period"`
	Type         string   `json:"type"`
	Amount       float64  `json:"amount_tokens"`
	BalanceAfter float64  `json:"balance_after_tokens"`
	Multiplier   *float64 `json:"multiplier"`
	SetupName    *string  `json:"setup_name"`
	HoldID       *int64   `json:"hold_id"`
	ActorName    *string  `json:"actor_name"`
	Reason       *string  `json:"reason"`
}

// availablePeriods lists the user's periods with ledger entries, newest
// first, always including the current one (the Month select needs it).
func (a *API) availablePeriods(r *http.Request, userID int64, current string) ([]string, error) {
	ps, err := a.d.Store.LedgerPeriods(r.Context(), userID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ps)+1)
	has := false
	for _, p := range ps {
		has = has || p == current
	}
	if !has {
		out = append(out, current)
	}
	out = append(out, ps...)
	// A period after the current one (clock moved back) sorts first.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] > out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}

// listMyLedger is GET /api/v1/me/ledger[?period=YYYY-MM], newest first. The
// running balance is the sum of the period's entries up to each entry (LED-2).
func (a *API) listMyLedger(w http.ResponseWriter, r *http.Request) {
	u, ok := userCaller(w, r)
	if !ok {
		return
	}
	current := ledger.PeriodOf(a.d.Clock.Now(), a.d.Location)
	period, ok := periodParam(w, r, current)
	if !ok {
		return
	}
	ctx := r.Context()
	st := a.d.Store
	rows, err := st.ListLedger(ctx, u.ID, period)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	periods, err := a.availablePeriods(r, u.ID, current)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	mults, err := st.DebitMultipliers(ctx, u.ID, period)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	setups, err := st.ListSetups(ctx, false)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	setupName := make(map[int64]string, len(setups))
	for _, s := range setups {
		setupName[s.ID] = s.Name
	}
	actors := map[int64]string{}
	holdSetup := map[int64]string{}
	entries := make([]ledgerEntryJSON, len(rows))
	var running int64
	for i, e := range rows {
		running += e.AmountMT
		v := ledgerEntryJSON{ID: e.ID, CreatedAt: ts(e.CreatedAt), Period: e.Period, Type: e.Type,
			Amount: tokens(e.AmountMT), BalanceAfter: tokens(running), HoldID: e.HoldID, Reason: nullIfEmpty(e.Reason)}
		if m, ok := mults[e.ID]; ok {
			v.Multiplier = &m
		}
		if e.HoldID != nil {
			name, seen := holdSetup[*e.HoldID]
			if !seen {
				h, err := st.GetHold(ctx, *e.HoldID)
				if err != nil {
					a.writeError(w, r, err)
					return
				}
				name = setupName[h.SetupID]
				holdSetup[*e.HoldID] = name
			}
			v.SetupName = nullIfEmpty(name)
		}
		if e.ActorID != nil {
			name, seen := actors[*e.ActorID]
			if !seen {
				au, err := st.GetUser(ctx, *e.ActorID)
				if err != nil {
					a.writeError(w, r, err)
					return
				}
				name = userDisplayName(au)
				actors[*e.ActorID] = name
			}
			v.ActorName = nullIfEmpty(name)
		}
		entries[len(rows)-1-i] = v // newest first
	}
	writeJSON(w, http.StatusOK, map[string]any{"period": period, "available_periods": periods, "entries": entries})
}

// ---- GET /api/v1/me/usage ----

type usageDayJSON struct {
	Date    string  `json:"date"`
	Debited float64 `json:"debited_tokens"`
}

// getMyUsage is GET /api/v1/me/usage[?period=YYYY-MM]: tokens debited per
// calendar day in the configured timezone, zero days included. The current
// period lists days up to today; a past period lists the whole month; a
// future period lists none (api/openapi.yaml).
func (a *API) getMyUsage(w http.ResponseWriter, r *http.Request) {
	u, ok := userCaller(w, r)
	if !ok {
		return
	}
	now := a.d.Clock.Now()
	loc := a.d.Location
	current := ledger.PeriodOf(now, loc)
	period, ok := periodParam(w, r, current)
	if !ok {
		return
	}
	year, month, _ := ledger.ParsePeriod(period)
	from, to := ledger.PeriodStart(period, loc), ledger.PeriodStart(ledger.NextPeriod(period), loc)
	entries, err := a.d.Store.DebitEntries(r.Context(), u.ID, from, to)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	perDay := map[string]int64{}
	var total int64
	for _, e := range entries {
		perDay[e.At.In(loc).Format("2006-01-02")] += e.MT
		total += e.MT
	}
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day() // days in the month
	switch {
	case period == current:
		last = now.In(loc).Day()
	case period > current:
		last = 0
	}
	days := make([]usageDayJSON, 0, last)
	for d := 1; d <= last; d++ {
		key := time.Date(year, month, d, 0, 0, 0, 0, loc).Format("2006-01-02")
		days = append(days, usageDayJSON{Date: key, Debited: tokens(perDay[key])})
	}
	periods, err := a.availablePeriods(r, u.ID, current)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"period": period, "available_periods": periods,
		"total_debited_tokens": tokens(total), "days": days})
}

// ---- GET /api/v1/pressure ----

type bandStepJSON struct {
	From       float64 `json:"from"`
	Multiplier float64 `json:"multiplier"`
}

type pressurePointJSON struct {
	At         string  `json:"at"`
	Raw        float64 `json:"raw"`
	Smoothed   float64 `json:"smoothed"`
	Multiplier float64 `json:"multiplier"`
}

// pressureHistory is how far back GET /api/v1/pressure reports samples.
const pressureHistory = 7 * 24 * time.Hour

// getPressure is GET /api/v1/pressure: the latest sample, the bands from the
// settings, 7 days of history and the setup counts for the card.
func (a *API) getPressure(w http.ResponseWriter, r *http.Request) {
	if _, ok := userCaller(w, r); !ok {
		return
	}
	ctx := r.Context()
	l, err := a.loadLab(ctx)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	samples, err := a.d.Store.PressureSince(ctx, l.now.Add(-pressureHistory))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	history := make([]pressurePointJSON, len(samples))
	for i, s := range samples {
		history[i] = pressurePointJSON{At: ts(s.PolledAt), Raw: s.Raw, Smoothed: s.Smoothed, Multiplier: s.Multiplier}
	}
	steps := make([]bandStepJSON, len(l.set.Bands.Steps))
	for i, s := range l.set.Bands.Steps {
		steps[i] = bandStepJSON{From: s.From, Multiplier: s.Multiplier}
	}
	out := map[string]any{
		"raw": 0.0, "smoothed": 0.0, "multiplier": l.mult,
		"bands":      map[string]any{"base": l.set.Bands.Base, "steps": steps},
		"source":     a.d.Runtime.PressureSource,
		"updated_at": (*string)(nil),
		"history":    history,
	}
	if s := l.sample; s != nil {
		out["raw"], out["smoothed"], out["source"] = s.Raw, s.Smoothed, s.Source
		out["updated_at"] = ts(s.PolledAt)
	}
	setups, err := a.d.Store.ListSetups(ctx, true)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	deployed := 0
	for _, s := range setups {
		rep, err := l.reported(s)
		if err != nil {
			a.writeError(w, r, err)
			return
		}
		if rep {
			deployed++
		}
	}
	out["setups_enabled"], out["setups_deployed"] = len(setups), deployed
	writeJSON(w, http.StatusOK, out)
}

// ---- GET /api/v1/setups ----

type holderJSON struct {
	UserID  int64   `json:"user_id"`
	HoldID  int64   `json:"hold_id"`
	Name    string  `json:"name"`
	Balance float64 `json:"balance_tokens"`
	Over    bool    `json:"overdrawn"`
	IsYou   bool    `json:"is_you"`
}

type setupAdminJSON struct {
	Enabled      bool    `json:"enabled"`
	ResourceName string  `json:"resource_name"`
	Override     *string `json:"resource_name_override"`
	LastSeenAt   *string `json:"last_seen_at"`
	MatchedRule  *string `json:"matched_rule_selector"`
}

type setupJSON struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels"`
	Weight      float64           `json:"weight"`
	State       string            `json:"state"`
	Holder      *holderJSON       `json:"holder"`
	HeldSince   *string           `json:"held_since"`
	Deployed    bool              `json:"deployed"`
	Metering    *meteringJSON     `json:"metering"`
	CostPerHour costJSON          `json:"cost_per_hour_tokens"`
	Details     any               `json:"details"`
	Admin       *setupAdminJSON   `json:"admin"`
}

// listSetups is GET /api/v1/setups[?selector=]. Every user with a role sees
// every enabled setup and the holder's balance by design; admins also get
// disabled setups and the admin sub-object. The summary counts ignore the
// selector.
func (a *API) listSetups(w http.ResponseWriter, r *http.Request) {
	u, ok := userCaller(w, r)
	if !ok {
		return
	}
	var sel *rules.Selector
	if raw := r.URL.Query().Get("selector"); strings.TrimSpace(raw) != "" {
		s, err := rules.Parse(raw)
		if err != nil {
			apierr.WriteField(w, http.StatusBadRequest, apierr.InvalidSelector, err.Error(), "selector")
			return
		}
		sel = &s
	}
	ctx := r.Context()
	isAdmin := u.HasRole(store.RoleAdmin)
	l, err := a.loadLab(ctx)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	rows, err := a.d.Store.ListSetups(ctx, !isAdmin)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	active, err := a.d.Store.ActiveHolds(ctx)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	holdOf := make(map[int64]store.Hold, len(active))
	for _, h := range active {
		holdOf[h.SetupID] = h
	}
	holders, err := a.usersByID(ctx) // the setups list shows holders only, not charges
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	keys := l.set.DisplayLabelKeys
	if keys == nil {
		keys = []string{}
	}
	var summary struct{ Enabled, Deployed, Free int }
	list := make([]setupJSON, 0, len(rows))
	for _, s := range rows {
		rep, err := l.reported(s)
		if err != nil {
			a.writeError(w, r, err)
			return
		}
		h, held := holdOf[s.ID]
		if s.Enabled {
			summary.Enabled++
			if rep {
				summary.Deployed++
			}
			if !held {
				summary.Free++
			}
		}
		if sel != nil && !sel.Matches(s.Labels) {
			continue
		}
		wt, matched, err := l.weight(s)
		if err != nil {
			a.writeError(w, r, err)
			return
		}
		v := setupJSON{ID: s.ID, Name: s.Name, Labels: labelsNonNil(s.Labels), Weight: wt, State: "free",
			Deployed: rep, CostPerHour: l.cost(wt), Details: detailsJSON(s.DetailsJSON)}
		switch {
		case held:
			v.State = "held"
			hu := holders[h.UserID]
			at := ts(h.AcquiredAt)
			m := meteringOf(h, l.view)
			v.HeldSince, v.Metering = &at, &m
			v.Holder = &holderJSON{UserID: h.UserID, HoldID: h.ID, Name: userDisplayName(hu), Balance: tokens(hu.BalanceMT),
				Over: hu.BalanceMT <= 0, IsYou: h.UserID == u.ID}
		case !s.Enabled:
			v.State = "disabled"
		}
		if isAdmin {
			res, err := l.resourceName(s)
			if err != nil {
				a.writeError(w, r, err)
				return
			}
			adm := &setupAdminJSON{Enabled: s.Enabled, ResourceName: res, Override: nullIfEmpty(s.ResourceNameOverride),
				MatchedRule: nullIfEmpty(matched)}
			if t, ok := l.view.LastSeen[s.Name]; ok {
				adm.LastSeenAt = tsPtr(&t)
			}
			v.Admin = adm
		}
		list = append(list, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"setups": list, "display_label_keys": keys,
		"summary": map[string]int{"enabled": summary.Enabled, "deployed": summary.Deployed, "free": summary.Free},
		// The policy interface has no describe method, so there is no text to show.
		"metering_description": nil,
	})
}

// ---- POST /api/v1/setups/{id}/acquire, POST /api/v1/holds/{id}/release ----

// holdResponse writes one hold as the response of acquire/release.
func (a *API) holdResponse(w http.ResponseWriter, r *http.Request, status int, h store.Hold) {
	l, err := a.loadLab(r.Context())
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	views, err := a.holdViews(r.Context(), l, []store.Hold{h})
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, status, views[0])
}

// acquireSetup is POST /api/v1/setups/{id}/acquire (HOLD-1). The refusals and
// their order are internal/holds'; the local admin is refused as forbidden.
func (a *API) acquireSetup(w http.ResponseWriter, r *http.Request) {
	u, ok := userCaller(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if a.d.Holds == nil {
		a.writeError(w, r, errors.New("api: holds service is not configured"))
		return
	}
	h, err := a.d.Holds.Acquire(r.Context(), u.ID, id)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	a.holdResponse(w, r, http.StatusCreated, h)
}

type releaseRequest struct {
	Force  bool   `json:"force"`
	Reason string `json:"reason"`
}

// releaseHold is POST /api/v1/holds/{id}/release (HOLD-2: owner release and
// admin force-release). The body is optional; an empty one is a normal
// release. Who may do what is decided by internal/holds.
func (a *API) releaseHold(w http.ResponseWriter, r *http.Request) {
	u, ok := userCaller(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req releaseRequest
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBodyBytes+1))
	switch {
	case err != nil:
		apierr.Write(w, http.StatusBadRequest, apierr.Validation, "request body could not be read")
		return
	case len(bytes.TrimSpace(body)) > 0:
		r.Body = io.NopCloser(bytes.NewReader(body))
		if !decodeJSON(w, r, &req) {
			return
		}
	}
	if a.d.Holds == nil {
		a.writeError(w, r, errors.New("api: holds service is not configured"))
		return
	}
	h, err := a.d.Holds.Release(r.Context(), u.ID, id, holds.ReleaseOpts{Force: req.Force, Reason: req.Reason})
	if err != nil {
		var he *holds.Error
		if errors.As(err, &he) && he.Code == holds.CodeValidation {
			// The only validation refusal of a release is the missing reason.
			apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, he.Message, "reason")
			return
		}
		a.writeError(w, r, err)
		return
	}
	a.holdResponse(w, r, http.StatusOK, h)
}
