// SPDX-License-Identifier: MIT

// Package collector is the metering poll (MET-1..MET-6): it builds the
// Snapshot, asks the configured extensions, and writes the pressure sample,
// usage samples and ledger debits of one poll in a single transaction.
package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/metering"
	"github.com/panset/hourglass/internal/rules"
	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

const (
	// ExtensionTimeout bounds each provider, pressure source and policy call.
	ExtensionTimeout = 30 * time.Second
	// SmoothingWindow is the pressure averaging window ("Burn rate").
	SmoothingWindow = metering.DefaultWindow
	// UsageRetentionMonths, EventRetention and PruneEvery drive the daily prune.
	UsageRetentionMonths = 13
	EventRetention       = 90 * 24 * time.Hour
	PruneEvery           = 24 * time.Hour
	// CompactStepsPerCycle is the most ledger compaction steps one collector
	// cycle runs (design A1, LED-10). Each step is one transaction of at most
	// store.LedgerCompactionStepRows rows and holds runMu alone; a backlog
	// larger than this is worked off over the following cycles.
	CompactStepsPerCycle = 20
)

// Clock supplies the time; production passes a wall clock, tests a fake.
type Clock interface{ Now() time.Time }

// Emitter queues non-vetoable events (internal/events.Dispatcher).
type Emitter interface{ Emit(hourglass.Event) }

// Ledger is the part of *ledger.Service the collector uses.
type Ledger interface {
	DebitTx(tx *store.Tx, userID, holdID, amountMT int64, at time.Time) (ledger.BalanceCrossings, error)
	RolloverDue(ctx context.Context) ([]string, error)
	CompactDue(ctx context.Context) (ledger.CompactionStep, error)
}

// Deps are the collector's collaborators. Provider, Events, Logger and After
// are optional; the others are required.
type Deps struct {
	Store    *store.Store
	Provider hourglass.ResourceProvider // nil: no provider configured
	Pressure hourglass.PressureSource
	Policy   hourglass.MeteringPolicy
	Events   Emitter
	Ledger   Ledger
	Clock    Clock
	Logger   *slog.Logger
	// After is how Run waits between polls; defaults to time.After.
	After func(time.Duration) <-chan time.Time
}

// PollView is the in-memory result of the latest successful poll (D20).
type PollView struct {
	OK                 bool // false until the first successful poll
	Time               time.Time
	ProviderConfigured bool
	// Resources is the set of resource names the provider reported.
	Resources map[string]bool
	// LastSeen is, per setup name, the last successful poll at which the
	// provider reported the setup's resource (since this process started).
	LastSeen         map[string]time.Time
	RawPressure      float64
	SmoothedPressure float64
	Multiplier       float64
	// Decisions is, per hold id, what the metering policy decided for each
	// hold that was active at the latest successful poll (meter flag and the
	// policy's own reason). A hold the poll has not seen has no entry.
	Decisions map[int64]hourglass.Decision
}

// HoldsView adapts a PollView to holds.PollView, whose OK is a method (a
// struct cannot have both a field and a method named OK).
type HoldsView struct{ PollView }

// OK reports whether a successful poll exists (holds.PollView).
func (v HoldsView) OK() bool { return v.PollView.OK }

// ForHolds returns v as a holds.PollView.
func (v PollView) ForHolds() HoldsView { return HoldsView{v} }

// Reported reports whether the provider reported resourceName in the
// latest successful poll.
func (v PollView) Reported(resourceName string) bool { return v.Resources[resourceName] }

func (v PollView) clone() PollView {
	out := v
	out.Resources = make(map[string]bool, len(v.Resources))
	for k, b := range v.Resources {
		out.Resources[k] = b
	}
	out.LastSeen = make(map[string]time.Time, len(v.LastSeen))
	for k, t := range v.LastSeen {
		out.LastSeen[k] = t
	}
	out.Decisions = make(map[int64]hourglass.Decision, len(v.Decisions))
	for k, d := range v.Decisions {
		out.Decisions[k] = d
	}
	return out
}

// Collector runs polls. It is safe for concurrent use; polls never overlap.
type Collector struct {
	d   Deps
	log *slog.Logger

	runMu     sync.Mutex // serialises polls, prunes and compaction steps
	lastSeen  map[string]time.Time
	lastPrune time.Time
	// compactPending is set by the daily prune and cleared when a compaction
	// step finds nothing more due; guarded by runMu.
	compactPending bool
	// compactSteps is the step budget of one cycle (CompactStepsPerCycle).
	compactSteps int
	// lastInterval is the poll interval in force at the previous successful
	// poll of this process (zero after a restart).
	lastInterval time.Duration

	viewMu sync.RWMutex
	view   PollView
}

// New returns a collector. It panics when a required dependency is missing.
func New(d Deps) *Collector {
	if d.Store == nil || d.Pressure == nil || d.Policy == nil || d.Ledger == nil || d.Clock == nil {
		panic("collector: Store, Pressure, Policy, Ledger and Clock are required")
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.After == nil {
		d.After = time.After
	}
	return &Collector{d: d, log: d.Logger.With("component", "collector"), lastSeen: map[string]time.Time{},
		compactSteps: CompactStepsPerCycle}
}

// LastPoll returns a copy of the latest successful poll's view.
func (c *Collector) LastPoll() PollView {
	c.viewMu.RLock()
	defer c.viewMu.RUnlock()
	return c.view.clone()
}

// Run polls immediately, then every poll interval (re-read each cycle) until
// ctx ends. RunOnce runs the ledger rollover before each poll; once a day it
// prunes old rows and starts the ledger compaction, which then runs a bounded
// number of steps after each poll until nothing is due; the time those steps
// take is deducted from the wait, so compaction never stretches the gap
// between polls. It returns ctx.Err().
func (c *Collector) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_ = c.RunOnce(ctx) // failures are recorded, logged and emitted by RunOnce
		c.pruneIfDue(ctx)
		compactStart := c.d.Clock.Now()
		c.compactIfDue(ctx)
		compacting := c.d.Clock.Now().Sub(compactStart)
		interval := settings.Defaults().PollInterval
		if cfg, err := settings.Load(ctx, c.d.Store); err != nil {
			c.log.Error("load settings for poll interval", "err", err, "fallback", interval)
		} else {
			interval = cfg.PollInterval
		}
		// Time spent compacting comes out of the wait, so a backlog adds
		// nothing to the gap between two polls (LED-10).
		if compacting > 0 {
			interval = max(interval-compacting, 0)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.d.After(interval):
		}
	}
}

func (c *Collector) pruneIfDue(ctx context.Context) {
	c.runMu.Lock()
	defer c.runMu.Unlock()
	now := c.d.Clock.Now()
	if !c.lastPrune.IsZero() && now.Sub(c.lastPrune) < PruneEvery {
		return
	}
	c.lastPrune = now
	c.compactPending = true // checked by compactIfDue, after this lock is released
	st := c.d.Store
	if n, err := st.PruneUsageSamples(ctx, now.AddDate(0, -UsageRetentionMonths, 0)); err != nil {
		c.log.Error("prune usage samples", "err", err)
	} else if n > 0 {
		c.log.Info("pruned usage samples", "rows", n)
	}
	if n, err := st.PruneEventDeliveries(ctx, now.Add(-EventRetention)); err != nil {
		c.log.Error("prune event deliveries", "err", err)
	} else if n > 0 {
		c.log.Info("pruned event deliveries", "rows", n)
	}
	if n, err := st.DeleteExpiredSessions(ctx, now); err != nil {
		c.log.Error("delete expired sessions", "err", err)
	} else if n > 0 {
		c.log.Info("deleted expired sessions", "rows", n)
	}
}

// compactIfDue runs the pending ledger compaction (design A1, LED-10): at most
// compactSteps steps, each holding runMu for that one step only, so a poll
// (or another writer) can run between two steps. It stops early when nothing
// more is due or a step fails. A failed step is logged and retried on the next
// cycle; it never fails a poll. The compaction becomes pending once a day,
// with the prune, and stays pending across cycles until a step finds nothing
// more due.
func (c *Collector) compactIfDue(ctx context.Context) {
	for i := 0; i < c.compactSteps; i++ {
		if !c.compactStep(ctx) {
			return
		}
	}
}

// compactStep runs one compaction step under runMu and reports whether the
// cycle may run another.
func (c *Collector) compactStep(ctx context.Context) bool {
	c.runMu.Lock()
	defer c.runMu.Unlock()
	if !c.compactPending || ctx.Err() != nil {
		return false
	}
	st, err := c.d.Ledger.CompactDue(ctx)
	if err != nil {
		c.log.Error("ledger compaction step failed; retried next cycle", "err", err)
		return false
	}
	if st.Removed > 0 {
		c.log.Info("compacted ledger", "rows", st.Removed, "groups", st.Groups)
	}
	if !st.More {
		c.compactPending = false
		return false
	}
	return true
}

// RunOnce performs one poll at T = clock.Now(). Any error fails the whole
// poll: nothing is written except the failed run record, poll.failed is
// emitted and the error is returned.
func (c *Collector) RunOnce(ctx context.Context) error {
	c.runMu.Lock()
	defer c.runMu.Unlock()
	// Open any due period first so this poll's debit never lands in a period
	// that has not been opened (LED-3).
	if closed, err := c.d.Ledger.RolloverDue(ctx); err != nil {
		c.log.Error("ledger rollover failed", "err", err)
	} else if len(closed) > 0 {
		c.log.Info("ledger periods closed", "periods", closed)
	}
	t := c.d.Clock.Now().UTC().Truncate(time.Millisecond) // the store keeps milliseconds
	runID, err := c.d.Store.StartCollectorRun(ctx, t)
	if err != nil {
		err = fmt.Errorf("collector: record run start: %w", err)
		c.fail(ctx, 0, t, err)
		return err
	}
	res, err := c.poll(ctx, t, runID)
	if err != nil {
		c.fail(ctx, runID, t, err)
		return err
	}
	c.publish(res)
	for _, e := range res.events {
		c.emit(e)
	}
	c.log.Info("poll ok", "time", t, "holds", res.holds, "debits", res.debits,
		"raw_pressure", res.view.RawPressure, "smoothed", res.view.SmoothedPressure, "multiplier", res.view.Multiplier)
	return nil
}

func (c *Collector) emit(e hourglass.Event) {
	if c.d.Events != nil {
		c.d.Events.Emit(e)
	}
}

func (c *Collector) fail(ctx context.Context, runID int64, t time.Time, err error) {
	c.log.Error("poll failed", "time", t, "err", err)
	if runID != 0 {
		if ferr := c.d.Store.FinishCollectorRun(context.WithoutCancel(ctx), runID, c.d.Clock.Now(), false, err.Error()); ferr != nil {
			c.log.Error("record failed run", "err", ferr)
		}
	}
	c.emit(hourglass.Event{Type: hourglass.EventPollFailed, Time: t, Data: map[string]any{"error": err.Error()}})
}

func (c *Collector) publish(r *result) {
	c.lastInterval = r.interval
	for name, at := range r.seen {
		c.lastSeen[name] = at
	}
	v := r.view
	v.LastSeen = c.lastSeen
	v = v.clone()
	c.viewMu.Lock()
	c.view = v
	c.viewMu.Unlock()
}

// result is what a successful poll hands back for after-commit work.
type result struct {
	view   PollView
	seen   map[string]time.Time // setup name -> T, for setups reported this poll
	events []hourglass.Event
	holds  int
	debits int
	// interval is the poll interval setting this poll ran under.
	interval time.Duration
}

// setupInfo is one setup resolved for this poll.
type setupInfo struct {
	row      store.Setup
	state    hourglass.SetupState
	selector string // matching rule's selector; empty when the default applied
}

// holdInfo is one active hold with its setup, user and decision.
type holdInfo struct {
	row      store.Hold
	setup    *setupInfo
	state    hourglass.HoldState
	decision hourglass.Decision
}

func (c *Collector) poll(ctx context.Context, t time.Time, runID int64) (*result, error) {
	st := c.d.Store

	// 1. Read everything fresh (RULE-4, ADM-2).
	cfg, err := settings.Load(ctx, st)
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	setupRows, err := st.ListSetups(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("list setups: %w", err)
	}
	ruleRows, err := st.ListRules(ctx, rules.KindRate)
	if err != nil {
		return nil, fmt.Errorf("list rate rules: %w", err)
	}
	holdRows, err := st.ActiveHolds(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active holds: %w", err)
	}
	var prev time.Time
	// The newest successful poll of any time, so a clock stepped backwards
	// cannot charge the same interval twice.
	switch run, err := st.LastSuccessfulRunBefore(ctx, t.AddDate(100, 0, 0)); {
	case err == nil:
		prev = run.StartedAt
	case !errors.Is(err, store.ErrNotFound):
		return nil, fmt.Errorf("previous successful poll: %w", err)
	}
	history, err := st.PressureSince(ctx, t.Add(-SmoothingWindow))
	if err != nil {
		return nil, fmt.Errorf("pressure history: %w", err)
	}

	// 2. Resolve weights.
	rs := make([]rules.Rule, len(ruleRows))
	for i, r := range ruleRows {
		rs[i] = rules.Rule{ID: r.ID, Kind: r.Kind, Position: r.Position, Selector: r.Selector,
			Value: strconv.FormatFloat(r.Value, 'f', -1, 64)}
	}
	setups := make([]*setupInfo, 0, len(setupRows))
	byID := make(map[int64]*setupInfo, len(setupRows))
	for _, s := range setupRows {
		w, sel, matched, err := rules.Weight(rs, s.Labels, cfg.DefaultWeight)
		if err != nil {
			return nil, fmt.Errorf("setup %q: rate rules: %w", s.Name, err)
		}
		if !matched {
			sel = ""
		}
		si := &setupInfo{row: s, selector: sel, state: hourglass.SetupState{
			Setup:  hourglass.Setup{Name: s.Name, Labels: s.Labels, Details: json.RawMessage(s.DetailsJSON)},
			Weight: w,
		}}
		setups = append(setups, si)
		byID[s.ID] = si
	}

	// 3. Ask the provider; duplicated resource names count once.
	var allocs []hourglass.Allocation
	reported := map[string]hourglass.Allocation{}
	if p := c.d.Provider; p != nil {
		list, err := callExt(ctx, func(ctx context.Context) ([]hourglass.Allocation, error) { return p.List(ctx) })
		if err != nil {
			return nil, fmt.Errorf("provider %s: %w", p.Name(), err)
		}
		allocs = DedupAllocations(list)
		for _, a := range allocs {
			reported[a.ResourceName] = a
		}
	}
	// D40: an allocation belongs to the setup whose name equals its resource name.
	allocationFor := func(si *setupInfo) *hourglass.Allocation {
		if a, ok := reported[si.state.Name]; ok {
			return &a
		}
		return nil
	}

	// 4. Snapshot (enabled setups only: SET-5), pressure, then the policy.
	snap := hourglass.Snapshot{Time: t, ProviderConfigured: c.d.Provider != nil, Allocations: allocs}
	for _, si := range setups {
		if si.row.Enabled {
			snap.Setups = append(snap.Setups, si.state)
		}
	}
	users := map[int64]hourglass.User{}
	holds := make([]*holdInfo, 0, len(holdRows))
	for _, h := range holdRows {
		si := byID[h.SetupID]
		if si == nil {
			return nil, fmt.Errorf("hold %d: setup %d not found", h.ID, h.SetupID)
		}
		u, ok := users[h.UserID]
		if !ok {
			row, err := st.GetUser(ctx, h.UserID)
			if err != nil {
				return nil, fmt.Errorf("hold %d: user %d: %w", h.ID, h.UserID, err)
			}
			roles, err := st.GetUserRoles(ctx, h.UserID)
			if err != nil {
				return nil, fmt.Errorf("hold %d: roles of user %d: %w", h.ID, h.UserID, err)
			}
			u = hourglass.User{ID: row.ID, Username: row.Username, Email: row.Email, Name: row.Name, Roles: roles}
			users[h.UserID] = u
		}
		hh := toHold(h, si.row.Name)
		snap.Holds = append(snap.Holds, hh)
		holds = append(holds, &holdInfo{row: h, setup: si,
			state: hourglass.HoldState{Hold: hh, Setup: si.state, User: u, Allocation: allocationFor(si)}})
	}
	ps := c.d.Pressure
	raw, err := callExt(ctx, func(ctx context.Context) (float64, error) { return ps.Pressure(ctx, snap) })
	if err != nil {
		return nil, fmt.Errorf("pressure source %s: %w", ps.Name(), err)
	}
	if !metering.ValidPressure(raw) {
		return nil, fmt.Errorf("pressure source %s: value %v is outside 0 to 1", ps.Name(), raw)
	}
	pol := c.d.Policy
	for _, h := range holds {
		hs := h.state
		d, err := callExt(ctx, func(ctx context.Context) (hourglass.Decision, error) { return pol.Decide(ctx, hs, snap) })
		if err != nil {
			return nil, fmt.Errorf("metering policy %s: hold %d: %w", pol.Name(), h.row.ID, err)
		}
		if !validWeight(d.Weight) {
			return nil, fmt.Errorf("metering policy %s: hold %d: weight %v must be finite and not negative", pol.Name(), h.row.ID, d.Weight)
		}
		h.decision = d
	}

	// 6. Smoothing over successful polls in the window, this one included.
	samples := make([]metering.Sample, 0, len(history)+1)
	for _, p := range history {
		if !p.PolledAt.Equal(t) { // a duplicate poll at T must not count twice
			samples = append(samples, metering.Sample{At: p.PolledAt, Raw: p.Raw})
		}
	}
	samples = append(samples, metering.Sample{At: t, Raw: raw})
	smoothed, _ := metering.Smooth(samples, t, SmoothingWindow)
	mult := cfg.Bands.Multiplier(smoothed)

	// 7. Chargeable time since the previous successful poll (MET-4, D12).
	if t.Before(prev) {
		c.log.Warn("clock is before the previous successful poll; nothing charged", "time", t, "previous", prev)
	}
	capInterval := cfg.PollInterval
	if c.lastInterval > capInterval {
		// The wait before this poll was scheduled under the longer interval.
		capInterval = c.lastInterval
	}
	elapsed, outage := metering.ChargeableElapsed(prev, t, capInterval)

	// 8. One transaction for everything this poll writes.
	res := &result{
		view: PollView{OK: true, Time: t, ProviderConfigured: c.d.Provider != nil, Resources: map[string]bool{},
			RawPressure: raw, SmoothedPressure: smoothed, Multiplier: mult, Decisions: map[int64]hourglass.Decision{}},
		seen:  map[string]time.Time{},
		holds: len(holds), interval: cfg.PollInterval,
	}
	for name := range reported {
		res.view.Resources[name] = true
	}
	for _, h := range holds {
		res.view.Decisions[h.row.ID] = h.decision
	}
	for _, si := range setups {
		if allocationFor(si) != nil {
			res.seen[si.row.Name] = t
		}
	}
	err = st.Tx(ctx, func(tx *store.Tx) error {
		res.events, res.debits = nil, 0
		if _, err := tx.InsertPressureSampleIfAbsent(ctx, store.PressureSample{
			PolledAt: t, Source: ps.Name(), Raw: raw, Smoothed: smoothed, Multiplier: mult}); err != nil {
			return fmt.Errorf("pressure sample: %w", err)
		}
		if outage {
			reason := fmt.Sprintf("no successful poll for %s (more than twice the poll interval of %s); the gap was not charged",
				t.Sub(prev), capInterval)
			if _, err := tx.InsertOutage(ctx, store.Outage{StartedAt: prev, EndedAt: t, Reason: reason}); err != nil {
				return fmt.Errorf("outage: %w", err)
			}
			c.log.Warn("outage: gap not charged", "from", prev, "to", t)
		}
		for _, h := range holds {
			evs, debited, err := c.applyHold(ctx, tx, h, t, smoothed, mult, elapsed, allocationFor(h.setup) != nil)
			if err != nil {
				return fmt.Errorf("hold %d: %w", h.row.ID, err)
			}
			res.events = append(res.events, evs...)
			if debited {
				res.debits++
			}
		}
		// HOLD-7: after metering, complete every pending release whose setup
		// is no longer reported. Only a successful poll gets here (HOLD-9).
		for _, h := range holds {
			ev, err := c.completeRelease(ctx, tx, h, t, allocationFor(h.setup) != nil)
			if err != nil {
				return fmt.Errorf("hold %d: complete release: %w", h.row.ID, err)
			}
			if ev != nil {
				res.events = append(res.events, *ev)
			}
		}
		if err := writeOrphans(ctx, tx, setups, holds, reported, t); err != nil {
			return err
		}
		return tx.FinishCollectorRun(ctx, runID, c.d.Clock.Now(), true, "")
	})
	if err != nil {
		return nil, fmt.Errorf("write poll: %w", err)
	}
	return res, nil
}

// applyHold writes one hold's part of the poll inside tx and returns the
// events to emit after commit (metering_started, deployed, balance events).
func (c *Collector) applyHold(ctx context.Context, tx *store.Tx, h *holdInfo, t time.Time,
	pressure, mult float64, elapsed time.Duration, reportedNow bool) ([]hourglass.Event, bool, error) {
	cur, err := tx.GetHold(ctx, h.row.ID) // fresh: may have been released or advanced meanwhile
	if err != nil {
		return nil, false, err
	}
	if cur.ReleasedAt != nil {
		return nil, false, nil // released since the read; nothing after release is charged
	}
	hold := toHold(cur, h.setup.row.Name)
	setup := h.setup.state.Setup
	user := h.state.User
	event := func(typ string) hourglass.Event {
		hc, sc, uc := hold, setup, user
		return hourglass.Event{Type: typ, Time: t, User: &uc, Setup: &sc, Hold: &hc}
	}
	var started, deployed bool
	if c.d.Provider != nil && reportedNow && cur.DeployedAt == nil {
		if deployed, err = tx.SetDeployed(ctx, cur.ID, t); err != nil {
			return nil, false, fmt.Errorf("set deployed: %w", err)
		}
		if deployed {
			tt := t
			hold.DeployedAt = &tt
		}
	}
	var balanceEvents []hourglass.Event
	debited := false
	if d := h.decision; d.Meter {
		weight, selector := h.setup.state.Weight, h.setup.selector
		if d.Weight > 0 {
			weight, selector = d.Weight, ""
		}
		var debit int64
		if cur.MeteringStartedAt == nil {
			if started, err = tx.SetMeteringStarted(ctx, cur.ID, t); err != nil {
				return nil, false, fmt.Errorf("set metering started: %w", err)
			}
			if started {
				tt := t
				hold.MeteringStartedAt = &tt
			}
		} else {
			debit = metering.DebitMT(weight, mult, elapsed)
		}
		inserted, err := tx.InsertUsageSample(ctx, store.UsageSample{HoldID: cur.ID, PolledAt: t, Weight: weight,
			RuleSelector: selector, Pressure: pressure, Multiplier: mult, DebitMT: debit, Reason: d.Reason})
		if err != nil {
			return nil, false, fmt.Errorf("usage sample: %w", err)
		}
		if inserted && debit > 0 { // already present: this poll was charged before (MET-5)
			cx, err := c.d.Ledger.DebitTx(tx, cur.UserID, cur.ID, debit, t)
			if err != nil {
				return nil, false, fmt.Errorf("ledger debit: %w", err)
			}
			balanceEvents = cx.Events
			debited = true
		}
	}
	var evs []hourglass.Event
	if started {
		evs = append(evs, event(hourglass.EventHoldMeteringStarted))
	}
	if deployed {
		evs = append(evs, event(hourglass.EventHoldDeployed))
	}
	return append(evs, balanceEvents...), debited, nil
}

// completeRelease releases a hold whose release was requested while its setup
// was deployed, once the setup is no longer reported (HOLD-7). The hold is
// read again inside tx, so a hold released or requested meanwhile is seen as
// it now is. released_by is the owner; the poll time is the release time. It
// returns the hold.released event to emit after commit, or nil.
func (c *Collector) completeRelease(ctx context.Context, tx *store.Tx, h *holdInfo, t time.Time, reportedNow bool) (*hourglass.Event, error) {
	if reportedNow {
		return nil, nil
	}
	cur, err := tx.GetHold(ctx, h.row.ID)
	if err != nil {
		return nil, err
	}
	if cur.ReleasedAt != nil || cur.ReleaseRequestedAt == nil {
		return nil, nil
	}
	owner := cur.UserID
	if err := tx.ReleaseHold(ctx, cur.ID, &owner, "", t); err != nil {
		return nil, err
	}
	if cur, err = tx.GetHold(ctx, cur.ID); err != nil {
		return nil, err
	}
	hold, setup, user := toHold(cur, h.setup.row.Name), h.setup.state.Setup, h.state.User
	return &hourglass.Event{Type: hourglass.EventHoldReleased, Time: t, User: &user, Setup: &setup, Hold: &hold,
		Data: map[string]any{"forced": false, "reason": ""}}, nil
}

// writeOrphans records allocations that match a setup with no active hold
// (disabled setups included) and drops orphans not reported this poll
// (HOLD-4). Allocations that match no setup are ignored.
func writeOrphans(ctx context.Context, tx *store.Tx, setups []*setupInfo, holds []*holdInfo,
	reported map[string]hourglass.Allocation, t time.Time) error {
	heldNames := map[string]bool{}
	for _, h := range holds {
		heldNames[h.setup.state.Name] = true
	}
	setupNames := make([]string, len(setups))
	for i, si := range setups {
		setupNames[i] = si.state.Name
	}
	names := OrphanNames(setupNames, heldNames, reported)
	for _, rn := range names {
		labels := reported[rn].Labels
		if labels == nil {
			labels = map[string]string{}
		}
		b, err := json.Marshal(labels)
		if err != nil {
			return fmt.Errorf("orphan %q labels: %w", rn, err)
		}
		if err := tx.UpsertOrphan(ctx, rn, string(b), t); err != nil {
			return fmt.Errorf("orphan %q: %w", rn, err)
		}
	}
	if _, err := tx.DeleteOrphansNotSeenSince(ctx, t); err != nil {
		return fmt.Errorf("clear stale orphans: %w", err)
	}
	return nil
}

func validWeight(w float64) bool { return !math.IsNaN(w) && !math.IsInf(w, 0) && w >= 0 }

func toHold(h store.Hold, setupName string) hourglass.Hold {
	return hourglass.Hold{ID: h.ID, SetupName: setupName, UserID: h.UserID, AcquiredAt: h.AcquiredAt,
		MeteringStartedAt: h.MeteringStartedAt, DeployedAt: h.DeployedAt,
		ReleaseRequestedAt: h.ReleaseRequestedAt, ReleasedAt: h.ReleasedAt}
}

// callExt runs one extension call with ExtensionTimeout. A panic becomes an
// error, and a call that ignores its context is abandoned at the deadline.
func callExt[T any](ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	cctx, cancel := context.WithTimeout(ctx, ExtensionTimeout)
	defer cancel()
	type out struct {
		v   T
		err error
	}
	ch := make(chan out, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				var zero T
				ch <- out{zero, fmt.Errorf("panicked: %v", r)}
			}
		}()
		v, err := fn(cctx)
		ch <- out{v, err}
	}()
	select {
	case o := <-ch:
		return o.v, o.err
	case <-cctx.Done():
		var zero T
		return zero, fmt.Errorf("no answer: %w", cctx.Err())
	}
}
