// SPDX-License-Identifier: MIT

package collector

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

var bg = context.Background()

const poll = 5 * time.Minute

// ---- harness ----

type recorder struct {
	mu     sync.Mutex
	ev     []hourglass.Event
	onEmit func(hourglass.Event)
}

func (r *recorder) Emit(e hourglass.Event) {
	if r.onEmit != nil {
		r.onEmit(e)
	}
	r.mu.Lock()
	r.ev = append(r.ev, e)
	r.mu.Unlock()
}

func (r *recorder) types() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []string{}
	for _, e := range r.ev {
		out = append(out, e.Type)
	}
	return out
}

func (r *recorder) ofType(typ string) []hourglass.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []hourglass.Event
	for _, e := range r.ev {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

// failingLedger wraps the real service and fails DebitTx on demand, after
// the usage sample of the same transaction was written.
type failingLedger struct {
	*ledger.Service
	mu   sync.Mutex
	fail error
}

func (f *failingLedger) DebitTx(tx *store.Tx, userID, holdID, amountMT int64, at time.Time) (ledger.BalanceCrossings, error) {
	f.mu.Lock()
	err := f.fail
	f.mu.Unlock()
	if err != nil {
		return ledger.BalanceCrossings{}, err
	}
	return f.Service.DebitTx(tx, userID, holdID, amountMT, at)
}

func (f *failingLedger) setFail(err error) { f.mu.Lock(); f.fail = err; f.mu.Unlock() }

type env struct {
	t        *testing.T
	st       *store.Store
	clk      *hourglasstest.FakeClock
	rec      *recorder
	prov     *hourglasstest.FakeProvider
	pressure *hourglasstest.FakePressureSource
	policy   *hourglasstest.FakePolicy
	led      *failingLedger
	c        *Collector
}

func newEnv(t *testing.T, withProvider bool) *env {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	e := &env{t: t, st: st, clk: hourglasstest.NewFakeClock(time.Time{}), rec: &recorder{},
		pressure: hourglasstest.NewFakePressureSource(), policy: hourglasstest.NewFakePolicy()}
	e.pressure.SetValue(0.6) // 1x
	e.policy.SetDefault(hourglass.Decision{Meter: true, Reason: "test"})
	e.led = &failingLedger{Service: ledger.NewService(st, e.rec, e.clk, time.UTC)}
	if withProvider {
		e.prov = hourglasstest.NewFakeProvider()
	}
	e.c = e.newCollector()
	return e
}

func (e *env) newCollector() *Collector {
	d := Deps{Store: e.st, Pressure: e.pressure, Policy: e.policy, Events: e.rec, Ledger: e.led, Clock: e.clk,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if e.prov != nil {
		d.Provider = e.prov
	}
	return New(d)
}

func (e *env) user(sub string) int64 {
	e.t.Helper()
	u, err := e.st.CreateOIDCUser(bg, sub, sub+"@example.test", sub, e.clk.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.st.SetUserRoles(bg, u.ID, []string{store.RoleDeveloper}); err != nil {
		e.t.Fatal(err)
	}
	return u.ID
}

func (e *env) setup(name string, enabled bool, labels map[string]string) int64 {
	e.t.Helper()
	id, err := e.st.CreateSetup(bg, store.Setup{Name: name, Enabled: enabled, Labels: labels, DetailsJSON: `{"k":"v"}`})
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) hold(setupID, userID int64) store.Hold {
	e.t.Helper()
	h, err := e.st.CreateHold(bg, setupID, userID, e.clk.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	return h
}

func (e *env) rules(sel string, w float64) {
	e.t.Helper()
	if err := e.st.ReplaceRules(bg, "rate", []store.Rule{{Selector: sel, Value: w}}, nil, e.clk.Now()); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) setting(key, value string) {
	e.t.Helper()
	if err := e.st.PutSetting(bg, key, value, nil, e.clk.Now()); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) allowance(userID int64) {
	e.t.Helper()
	if err := e.st.Tx(bg, func(tx *store.Tx) error { return e.led.EnsureFirstAllowanceTx(tx, userID, e.clk.Now()) }); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) runOK() {
	e.t.Helper()
	if err := e.c.RunOnce(bg); err != nil {
		e.t.Fatalf("RunOnce at %s: %v", e.clk.Now(), err)
	}
}

// step advances the clock by d and polls.
func (e *env) step(d time.Duration) {
	e.t.Helper()
	e.clk.Advance(d)
	e.runOK()
}

func (e *env) samples(holdID int64) []store.UsageSample {
	e.t.Helper()
	s, err := e.st.ListUsageSamples(bg, holdID)
	if err != nil {
		e.t.Fatal(err)
	}
	return s
}

func (e *env) debits(userID int64) []int64 {
	e.t.Helper()
	l, err := e.st.ListLedger(bg, userID, e.led.CurrentPeriod())
	if err != nil {
		e.t.Fatal(err)
	}
	var out []int64
	for _, x := range l {
		if x.Type == "debit" {
			out = append(out, -x.AmountMT)
		}
	}
	return out
}

func (e *env) getHold(id int64) store.Hold {
	e.t.Helper()
	h, err := e.st.GetHold(bg, id)
	if err != nil {
		e.t.Fatal(err)
	}
	return h
}

func eqInts(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// basic: one user holding one weight-2 setup (rule tier=gold -> 2).
func (e *env) basic() (userID int64, h store.Hold) {
	e.rules("tier=gold", 2)
	u := e.user("dev")
	s := e.setup("lab-01", true, map[string]string{"tier": "gold"})
	return u, e.hold(s, u)
}

// ---- tests ----

func TestMET3_SpecNumbersAcrossBands(t *testing.T) {
	for _, tc := range []struct {
		pressure float64
		mult     float64
		debit    int64
	}{{0.3, 0, 0}, {0.6, 1, 167}, {0.8, 2, 333}, {0.95, 3, 500}} {
		e := newEnv(t, false)
		e.pressure.SetValue(tc.pressure)
		u, h := e.basic()
		e.runOK()    // metering starts, no charge
		e.step(poll) // weight 2 x mult x 5 min
		s := e.samples(h.ID)
		if len(s) != 2 || s[1].DebitMT != tc.debit || s[1].Multiplier != tc.mult || s[1].Weight != 2 || s[1].RuleSelector != "tier=gold" || s[1].Reason != "test" {
			t.Fatalf("pressure %v: samples %+v", tc.pressure, s)
		}
		var want []int64
		if tc.debit > 0 {
			want = []int64{tc.debit}
		}
		if got := e.debits(u); !eqInts(got, want) {
			t.Fatalf("pressure %v: debits %v want %v", tc.pressure, got, want)
		}
	}
}

func TestMET2_SmoothingOverThreePolls(t *testing.T) {
	e := newEnv(t, false)
	_, h := e.basic()
	raws := []float64{0.4, 0.7, 0.9, 0.2}
	wantSmoothed := []float64{0.4, 0.55, 2.0 / 3, 0.6} // window (T-15m, T]
	wantMult := []float64{0, 1, 1, 1}
	for i, r := range raws {
		if i > 0 {
			e.clk.Advance(poll)
		}
		e.pressure.SetValue(r)
		e.runOK()
		v := e.c.LastPoll()
		if math.Abs(v.SmoothedPressure-wantSmoothed[i]) > 1e-9 || v.Multiplier != wantMult[i] || v.RawPressure != r {
			t.Fatalf("poll %d: view %+v", i, v)
		}
	}
	ps, _ := e.st.LatestPressure(bg, 10)
	if len(ps) != 4 || math.Abs(ps[1].Smoothed-2.0/3) > 1e-9 || ps[0].Source != e.pressure.Name() {
		t.Fatalf("pressure samples %+v", ps)
	}
	if s := e.samples(h.ID); s[2].DebitMT != 167 || math.Abs(s[2].Pressure-2.0/3) > 1e-9 {
		t.Fatalf("samples %+v", s)
	}
}

func TestMET3_MultiplierChangeAcrossBandChangesDebit(t *testing.T) {
	e := newEnv(t, false)
	u, _ := e.basic()
	e.runOK()
	e.step(poll)
	e.step(poll)
	e.pressure.SetValue(1.0)
	e.step(poll) // (0.6+0.6+1)/3 = 0.733 -> 1x
	e.step(poll) // (0.6+1+1)/3 = 0.867 -> 2x
	e.step(poll) // 1.0 -> 3x
	if got, want := e.debits(u), []int64{167, 167, 167, 333, 500}; !eqInts(got, want) {
		t.Fatalf("debits %v want %v", got, want)
	}
}

func TestD19_MeteringStartPollChargesNothing(t *testing.T) {
	e2 := newEnv(t, false)
	u2, h2 := e2.basic()
	e2.policy.SetDefault(hourglass.Decision{Meter: false})
	e2.runOK()
	e2.step(poll)
	if s := e2.samples(h2.ID); len(s) != 0 || e2.getHold(h2.ID).MeteringStartedAt != nil {
		t.Fatalf("policy says no: samples %+v", s)
	}
	e2.policy.SetDefault(hourglass.Decision{Meter: true, Reason: "now"})
	e2.step(poll)
	got := e2.getHold(h2.ID)
	if got.MeteringStartedAt == nil || !got.MeteringStartedAt.Equal(e2.clk.Now()) {
		t.Fatalf("metering_started_at = %v want %v", got.MeteringStartedAt, e2.clk.Now())
	}
	s := e2.samples(h2.ID)
	if len(s) != 1 || s[0].DebitMT != 0 || len(e2.debits(u2)) != 0 {
		t.Fatalf("start poll: samples %+v debits %v", s, e2.debits(u2))
	}
	if ev := e2.rec.ofType(hourglass.EventHoldMeteringStarted); len(ev) != 1 || ev[0].Hold.ID != h2.ID || ev[0].Hold.MeteringStartedAt == nil {
		t.Fatalf("events %+v", ev)
	}
	e2.step(poll)
	if d := e2.debits(u2); !eqInts(d, []int64{167}) {
		t.Fatalf("after start: %v", d)
	}
	if len(e2.rec.ofType(hourglass.EventHoldMeteringStarted)) != 1 {
		t.Fatal("metering_started fired twice")
	}
}

func TestMET4_ExtensionFailureChargesNobody(t *testing.T) {
	boom := errors.New("boom")
	for _, tc := range []struct {
		name string
		set  func(e *env, on bool)
	}{
		{"provider", func(e *env, on bool) {
			if on {
				e.prov.SetError(boom)
			} else {
				e.prov.SetError(nil)
			}
		}},
		{"pressure", func(e *env, on bool) {
			if on {
				e.pressure.SetError(boom)
			} else {
				e.pressure.SetError(nil)
			}
		}},
		{"policy", func(e *env, on bool) {
			if on {
				e.policy.SetError(boom)
			} else {
				e.policy.SetError(nil)
			}
		}},
		{"policy_panic", func(e *env, on bool) {
			if on {
				e.policy.SetFunc(func(hourglass.HoldState, hourglass.Snapshot) hourglass.Decision { panic("kaboom") })
			} else {
				e.policy.SetFunc(nil)
			}
		}},
		{"policy_bad_weight", func(e *env, on bool) {
			if on {
				e.policy.SetDefault(hourglass.Decision{Meter: true, Weight: -1})
			} else {
				e.policy.SetDefault(hourglass.Decision{Meter: true, Reason: "test"})
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, true)
			u, h := e.basic()
			e.runOK()
			e.step(poll)
			before := e.c.LastPoll()
			tc.set(e, true)
			e.clk.Advance(poll)
			err := e.c.RunOnce(bg)
			if err == nil {
				t.Fatal("poll succeeded")
			}
			if s := e.samples(h.ID); len(s) != 2 {
				t.Fatalf("samples written by failed poll: %+v", s)
			}
			if ps, _ := e.st.LatestPressure(bg, 10); len(ps) != 2 {
				t.Fatalf("pressure samples %d", len(ps))
			}
			run, _ := e.st.LatestRun(bg)
			if run.OK == nil || *run.OK || run.Error == "" {
				t.Fatalf("run %+v", run)
			}
			pf := e.rec.ofType(hourglass.EventPollFailed)
			if len(pf) != 1 || pf[0].Data["error"] != err.Error() {
				t.Fatalf("poll.failed %+v", pf)
			}
			if after := e.c.LastPoll(); !after.Time.Equal(before.Time) {
				t.Fatal("failed poll moved LastPoll")
			}
			// Previous successful poll did not move: the next poll charges 10 min (2x interval).
			tc.set(e, false)
			e.step(poll)
			if d := e.debits(u); !eqInts(d, []int64{167, 333}) {
				t.Fatalf("debits %v", d)
			}
		})
	}
}

func TestMET4_PressureOutOfRangeFailsPoll(t *testing.T) {
	for _, v := range []float64{-0.01, 1.01, math.NaN(), math.Inf(1)} {
		e := newEnv(t, false)
		_, h := e.basic()
		e.pressure.SetValue(v)
		if err := e.c.RunOnce(bg); err == nil {
			t.Fatalf("pressure %v accepted", v)
		}
		if len(e.samples(h.ID)) != 0 || e.getHold(h.ID).MeteringStartedAt != nil || len(e.rec.ofType(hourglass.EventPollFailed)) != 1 {
			t.Fatalf("pressure %v: wrote state", v)
		}
	}
	e := newEnv(t, false)
	e.basic()
	for _, v := range []float64{0, 1} {
		e.pressure.SetValue(v)
		e.step(poll)
	}
}

func TestMET4_GapExactlyTwiceIntervalChargedLongerNot(t *testing.T) {
	e := newEnv(t, false)
	u, _ := e.basic()
	e.runOK()
	e.step(poll)
	e.step(2 * poll) // exactly 2x: charged in full
	if d := e.debits(u); !eqInts(d, []int64{167, 333}) {
		t.Fatalf("debits %v", d)
	}
	from := e.clk.Now()
	e.step(2*poll + time.Millisecond) // longer: not charged, outage written
	if d := e.debits(u); !eqInts(d, []int64{167, 333}) {
		t.Fatalf("long gap charged: %v", d)
	}
	o, err := e.st.ListOutages(bg, 10)
	if err != nil || len(o) != 1 || !o[0].StartedAt.Equal(from) || !o[0].EndedAt.Equal(e.clk.Now()) || o[0].Reason == "" {
		t.Fatalf("outages %+v %v", o, err)
	}
	e.step(poll) // back to normal
	if d := e.debits(u); !eqInts(d, []int64{167, 333, 167}) {
		t.Fatalf("debits %v", d)
	}
}

func TestMET5_RestartIdempotence(t *testing.T) {
	e := newEnv(t, false)
	u, h := e.basic()
	e.runOK()
	e.step(poll)
	// A second collector (a restart) over the same database at the same instant.
	c2 := e.newCollector()
	if err := c2.RunOnce(bg); err != nil {
		t.Fatal(err)
	}
	if d := e.debits(u); !eqInts(d, []int64{167}) {
		t.Fatalf("debits %v", d)
	}
	if len(e.samples(h.ID)) != 2 {
		t.Fatal("duplicate sample")
	}
	// Two collectors racing at the next instant: still exactly one debit.
	e.clk.Advance(poll)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, c := range []*Collector{e.c, c2} {
		wg.Add(1)
		go func(c *Collector) { defer wg.Done(); errs <- c.RunOnce(bg) }(c)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if d := e.debits(u); !eqInts(d, []int64{167, 167}) {
		t.Fatalf("debits %v", d)
	}
	// The second collector keeps charging normally afterwards.
	e.clk.Advance(poll)
	if err := c2.RunOnce(bg); err != nil {
		t.Fatal(err)
	}
	if d := e.debits(u); !eqInts(d, []int64{167, 167, 167}) {
		t.Fatalf("debits %v", d)
	}
}

func TestMET3_CrashMidTransactionLeavesNothing(t *testing.T) {
	e := newEnv(t, true)
	u, h := e.basic()
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-01"}, hourglass.Allocation{ResourceName: "lab-02"})
	e.setup("lab-02", true, nil) // free: its allocation is an orphan
	e.runOK()
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-01"}, hourglass.Allocation{ResourceName: "lab-03"})
	e.setup("lab-03", true, nil)
	e.led.setFail(errors.New("disk on fire"))
	e.clk.Advance(poll)
	if err := e.c.RunOnce(bg); err == nil {
		t.Fatal("poll succeeded")
	}
	if s := e.samples(h.ID); len(s) != 1 {
		t.Fatalf("samples %+v", s)
	}
	if ps, _ := e.st.LatestPressure(bg, 10); len(ps) != 1 {
		t.Fatalf("pressure samples %+v", ps)
	}
	if o, _ := e.st.ListOrphans(bg); len(o) != 1 || o[0].ResourceName != "lab-02" {
		t.Fatalf("orphans %+v", o)
	}
	if len(e.debits(u)) != 0 {
		t.Fatal("debit written")
	}
	run, _ := e.st.LatestRun(bg)
	if run.OK == nil || *run.OK {
		t.Fatalf("run %+v", run)
	}
	if n := len(e.rec.ofType(hourglass.EventPollFailed)); n != 1 {
		t.Fatalf("poll.failed %d", n)
	}
	e.led.setFail(nil)
	e.step(poll)
	if d := e.debits(u); !eqInts(d, []int64{333}) {
		t.Fatalf("debits %v", d)
	}
}

func TestMET3_PolicyWeightOverride(t *testing.T) {
	e := newEnv(t, false)
	u, h := e.basic()
	e.policy.SetDefault(hourglass.Decision{Meter: true, Weight: 3, Reason: "override"})
	e.runOK()
	e.step(poll)
	s := e.samples(h.ID)
	if s[1].Weight != 3 || s[1].RuleSelector != "" || s[1].DebitMT != 250 || s[1].Reason != "override" {
		t.Fatalf("sample %+v", s[1])
	}
	if d := e.debits(u); !eqInts(d, []int64{250}) {
		t.Fatalf("debits %v", d)
	}
	// Default weight: selector null too.
	e2 := newEnv(t, false)
	u2 := e2.user("x")
	h2 := e2.hold(e2.setup("plain", true, nil), u2)
	e2.runOK()
	e2.step(poll)
	if s := e2.samples(h2.ID); s[1].Weight != 1 || s[1].RuleSelector != "" || s[1].DebitMT != 83 {
		t.Fatalf("default sample %+v", s[1])
	}
}

func TestHOLD4_OrphanRecordedAndCleared(t *testing.T) {
	e := newEnv(t, true)
	u := e.user("dev")
	held := e.setup("held", true, nil)
	e.setup("free", true, nil)
	e.setup("off", false, nil)
	h := e.hold(held, u)
	e.prov.SetAllocations(
		hourglass.Allocation{ResourceName: "held"},
		hourglass.Allocation{ResourceName: "free", Labels: map[string]string{"path": "/x"}},
		hourglass.Allocation{ResourceName: "off"},
		hourglass.Allocation{ResourceName: "free"}, // duplicate counts once
	)
	e.runOK()
	o, _ := e.st.ListOrphans(bg)
	if len(o) != 2 || o[0].ResourceName != "free" || o[0].LabelsJSON != `{"path":"/x"}` || o[1].ResourceName != "off" {
		t.Fatalf("orphans %+v", o)
	}
	first := o[0].FirstSeenAt
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "held"}, hourglass.Allocation{ResourceName: "free"})
	e.step(poll)
	o, _ = e.st.ListOrphans(bg)
	if len(o) != 1 || o[0].ResourceName != "free" || !o[0].FirstSeenAt.Equal(first) || !o[0].LastSeenAt.Equal(e.clk.Now()) {
		t.Fatalf("orphans %+v", o)
	}
	if err := e.st.ReleaseHold(bg, h.ID, nil, "", e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "held"})
	e.step(poll)
	o, _ = e.st.ListOrphans(bg)
	if len(o) != 1 || o[0].ResourceName != "held" {
		t.Fatalf("after release %+v", o)
	}
	e.prov.SetAllocations()
	e.step(poll)
	if o, _ = e.st.ListOrphans(bg); len(o) != 0 {
		t.Fatalf("not cleared %+v", o)
	}
}

func TestHOLD4_UnmatchedAllocationIgnored(t *testing.T) {
	e := newEnv(t, true)
	e.setup("lab-01", true, nil)
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "someone-elses-thing"}, hourglass.Allocation{ResourceName: "x"}, hourglass.Allocation{ResourceName: "x"})
	e.runOK()
	if o, _ := e.st.ListOrphans(bg); len(o) != 0 {
		t.Fatalf("orphans %+v", o)
	}
	snaps := e.pressure.Snapshots()
	if got := snaps[len(snaps)-1].Allocations; len(got) != 2 {
		t.Fatalf("snapshot allocations %+v (duplicates count once)", got)
	}
	if v := e.c.LastPoll(); !v.Reported("x") || v.Reported("lab-01") {
		t.Fatalf("view %+v", v)
	}
}

func TestSET5_DisabledSetupAbsentFromSnapshot(t *testing.T) {
	e := newEnv(t, false)
	e.setup("on", true, nil)
	e.setup("off", false, nil)
	e.runOK()
	snaps := e.pressure.Snapshots()
	s := snaps[len(snaps)-1]
	if len(s.Setups) != 1 || s.Setups[0].Name != "on" || s.Setups[0].Weight != 1 || s.Setups[0].ResourceName != "on" || s.ProviderConfigured {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestHOLD3_NoProviderNeverDeployed(t *testing.T) {
	e := newEnv(t, false)
	_, h := e.basic()
	e.runOK()
	e.step(poll)
	if e.getHold(h.ID).DeployedAt != nil || len(e.rec.ofType(hourglass.EventHoldDeployed)) != 0 {
		t.Fatal("deployed set without provider")
	}
	if v := e.c.LastPoll(); !v.OK || v.ProviderConfigured || v.Reported("lab-01") {
		t.Fatalf("view %+v", v)
	}
}

func TestHOLD3_DeployedSetOnce(t *testing.T) {
	e := newEnv(t, true)
	_, h := e.basic()
	e.policy.SetDefault(hourglass.Decision{Meter: false})
	e.runOK()
	if e.getHold(h.ID).DeployedAt != nil {
		t.Fatal("deployed before reported")
	}
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-01"})
	e.step(poll)
	at := e.clk.Now()
	if d := e.getHold(h.ID).DeployedAt; d == nil || !d.Equal(at) {
		t.Fatalf("deployed_at %v", d)
	}
	e.prov.SetAllocations()
	e.step(poll)
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-01"})
	e.step(poll)
	if d := e.getHold(h.ID).DeployedAt; !d.Equal(at) {
		t.Fatalf("deployed_at moved to %v", d)
	}
	ev := e.rec.ofType(hourglass.EventHoldDeployed)
	if len(ev) != 1 || ev[0].Setup.Name != "lab-01" || string(ev[0].Setup.Details) != `{"k":"v"}` || ev[0].User.Username == "" && ev[0].User.ID == 0 {
		t.Fatalf("deployed events %+v", ev)
	}
	if v := e.c.LastPoll(); !v.LastSeen["lab-01"].Equal(e.clk.Now()) || !v.Reported("lab-01") {
		t.Fatalf("view %+v", v)
	}
}

func TestHOLD3_EventsAfterCommitInOrder(t *testing.T) {
	e := newEnv(t, true)
	e.setting(settings.KeyDefaultAllowance, "0.3") // 300 mt
	u, h := e.basic()
	e.allowance(u)
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-01"})
	var mu sync.Mutex
	var problems []string
	e.rec.onEmit = func(ev hourglass.Event) {
		// Emitted after commit: the poll's writes are visible from outside the transaction.
		run, err := e.st.LatestRun(bg)
		mu.Lock()
		defer mu.Unlock()
		if err != nil || run.OK == nil {
			problems = append(problems, ev.Type+": run not finished at emit")
		}
		if ev.Type == hourglass.EventHoldMeteringStarted {
			if hh, _ := e.st.GetHold(bg, h.ID); hh.MeteringStartedAt == nil {
				problems = append(problems, "metering_started emitted before commit")
			}
		}
	}
	e.runOK()
	e.step(poll) // 300 -> 133: low (below 30)? no; 133 >= 30
	e.step(poll) // 133 -> -34: low and overdrawn
	want := []string{hourglass.EventHoldMeteringStarted, hourglass.EventHoldDeployed,
		hourglass.EventBalanceLow, hourglass.EventBalanceOverdrawn}
	got := e.rec.types()
	if len(got) != len(want) {
		t.Fatalf("events %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("events %v want %v", got, want)
		}
	}
	if len(problems) > 0 {
		t.Fatal(problems)
	}
}

func TestLED_BalanceCrossingEvents(t *testing.T) {
	e := newEnv(t, false)
	e.setting(settings.KeyDefaultAllowance, "0.4") // 400 mt; 10% = 40
	u, _ := e.basic()
	e.allowance(u)
	e.runOK()
	e.step(poll) // 233
	if n := len(e.rec.ofType(hourglass.EventBalanceLow)); n != 0 {
		t.Fatal("early low")
	}
	e.step(poll) // 66
	e.step(poll) // -101: low (66 >= 40 -> below) and overdrawn
	low, over := e.rec.ofType(hourglass.EventBalanceLow), e.rec.ofType(hourglass.EventBalanceOverdrawn)
	if len(low) != 1 || len(over) != 1 || low[0].User.ID != u {
		t.Fatalf("low %+v over %+v", low, over)
	}
	e.step(poll) // further down: no repeat
	if len(e.rec.ofType(hourglass.EventBalanceLow)) != 1 || len(e.rec.ofType(hourglass.EventBalanceOverdrawn)) != 1 {
		t.Fatal("repeated crossing events")
	}
}

func TestMET6_ShadowModeStillDebits(t *testing.T) {
	for _, enforcement := range []string{"false", "true"} {
		e := newEnv(t, false)
		e.setting(settings.KeyEnforcement, enforcement)
		u, _ := e.basic()
		e.runOK()
		e.step(poll)
		if d := e.debits(u); !eqInts(d, []int64{167}) {
			t.Fatalf("enforcement %s: debits %v", enforcement, d)
		}
	}
}

func TestRULE4_ChangesApplyFromNextPollNotRetroactive(t *testing.T) {
	e := newEnv(t, false)
	u, h := e.basic()
	e.runOK()
	e.step(poll)
	e.rules("tier=gold", 4)
	e.setting(settings.KeyBands, `{"base":0,"steps":[{"from":0.5,"multiplier":2}]}`)
	e.step(poll) // weight 4 x 2x x 5 min
	if d := e.debits(u); !eqInts(d, []int64{167, 667}) {
		t.Fatalf("debits %v", d)
	}
	s := e.samples(h.ID)
	if s[1].Weight != 2 || s[1].DebitMT != 167 || s[2].Weight != 4 || s[2].Multiplier != 2 {
		t.Fatalf("samples %+v", s)
	}
	e.setting(settings.KeyPollInterval, `"1m"`) // gap cap follows the new interval at the next poll
	e.step(poll)
	if d := e.debits(u); len(d) != 2 {
		t.Fatalf("5 min gap with a 1 min interval should be an outage: %v", d)
	}
}

func TestMET1_LastPollViewRaceWithConcurrentReader(t *testing.T) {
	e := newEnv(t, true)
	_, _ = e.basic()
	if v := e.c.LastPoll(); v.OK {
		t.Fatal("OK before first poll")
	}
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-01"})
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			v := e.c.LastPoll()
			_ = v.Reported("lab-01")
			for range v.LastSeen {
			}
		}
	}()
	for i := 0; i < 10; i++ {
		e.step(poll)
	}
	close(stop)
	wg.Wait()
	v := e.c.LastPoll()
	if !v.OK || !v.Time.Equal(e.clk.Now()) || !v.Reported("lab-01") || v.Multiplier != 1 {
		t.Fatalf("view %+v", v)
	}
	v.Resources["mutated"] = true
	if e.c.LastPoll().Reported("mutated") {
		t.Fatal("LastPoll leaks internal state")
	}
}

func TestMET1_RunPollsImmediatelyThenEveryInterval(t *testing.T) {
	e := newEnv(t, false)
	u, _ := e.basic()
	e.allowance(u)
	// Old rows that the daily prune removes.
	if _, err := e.st.InsertEventDelivery(bg, "x", "h", "{}", e.clk.Now().Add(-100*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	ticks := make(chan time.Time)
	waits := make(chan time.Duration, 10)
	c := New(Deps{Store: e.st, Pressure: e.pressure, Policy: e.policy, Events: e.rec, Ledger: e.led, Clock: e.clk,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		After:  func(d time.Duration) <-chan time.Time { waits <- d; return ticks }})
	ctx, cancel := context.WithCancel(bg)
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	if d := <-waits; d != poll {
		t.Fatalf("wait %v", d)
	}
	if !c.LastPoll().OK {
		t.Fatal("no immediate poll")
	}
	if d, _ := e.st.ListEventDeliveries(bg, 10); len(d) != 0 {
		t.Fatalf("not pruned: %+v", d)
	}
	e.setting(settings.KeyPollInterval, `"2m"`)
	e.clk.Advance(2 * time.Minute)
	ticks <- e.clk.Now()
	if d := <-waits; d != 2*time.Minute {
		t.Fatalf("interval not re-read: %v", d)
	}
	// The first poll started metering; the second charges 2 min: 2 x 1 x 2/60 h = 67.
	if d := e.debits(u); !eqInts(d, []int64{67}) {
		t.Fatalf("debits %v", d)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned %v", err)
	}
}
