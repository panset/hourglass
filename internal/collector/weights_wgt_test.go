// SPDX-License-Identifier: MIT

package collector

import (
	"database/sql"
	"fmt"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/holds"
	"github.com/panset/hourglass/internal/reported"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// ---- reported weights harness (docs/design/reported-weights.md, A2) ----

// reportedOn turns reported weights on with the given limits and max_age 10m.
func (e *env) reportedOn(min, max float64) {
	e.t.Helper()
	e.setting("reported_weights", fmt.Sprintf(`{"enabled":true,"min":%v,"max":%v,"max_age":"10m"}`, min, max))
}

// reportedOff stores the setting switched off with limits kept.
func (e *env) reportedOff() {
	e.t.Helper()
	e.setting("reported_weights", `{"enabled":false,"min":0,"max":20,"max_age":"10m"}`)
}

// submit stores a weights report through internal/reported, observed now.
func (e *env) submit(ws ...reported.Weight) {
	e.t.Helper()
	e.submitAt(e.clk.Now(), ws...)
}

func (e *env) submitAt(observed time.Time, ws ...reported.Weight) {
	e.t.Helper()
	if ws == nil {
		ws = []reported.Weight{}
	}
	if _, err := reported.New(e.st, e.clk).SubmitWeights(bg, "reporter", &observed, ws,
		reported.WeightLimits{Min: 0, Max: 1e12}); err != nil {
		e.t.Fatal(err)
	}
}

// storeRaw writes a weights report straight to the store, bypassing the
// limits (a report accepted under limits that were wider then).
func (e *env) storeRaw(observed time.Time, ws ...store.ReportedWeight) {
	e.t.Helper()
	if err := e.st.ReplaceReportedWeights(bg, store.ReportedHeader{ObservedAt: observed, ReceivedAt: observed,
		ServiceAccount: "reporter"}, ws); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) override(setupID int64, w float64) {
	e.t.Helper()
	if err := e.st.SetSetupWeightOverride(bg, setupID, &w); err != nil {
		e.t.Fatal(err)
	}
}

func rw(name string, w float64) reported.Weight { return reported.Weight{Setup: name, Weight: w} }

func lastSample(t *testing.T, e *env, holdID int64) store.UsageSample {
	t.Helper()
	s := e.samples(holdID)
	if len(s) == 0 {
		t.Fatal("no usage sample")
	}
	return s[len(s)-1]
}

func wantSample(t *testing.T, s store.UsageSample, weight float64, source, selector string, debit int64) {
	t.Helper()
	if s.Weight != weight || s.WeightSource != source || s.RuleSelector != selector || s.DebitMT != debit {
		t.Fatalf("sample weight %v source %q selector %q debit %d; want %v %q %q %d",
			s.Weight, s.WeightSource, s.RuleSelector, s.DebitMT, weight, source, selector, debit)
	}
}

// ---- WGT-1 ----

func TestWGT1_ReportedWeightIsCharged(t *testing.T) {
	for _, tc := range []struct {
		pressure float64
		mult     float64
		debit    int64
	}{{0.6, 1, 96}, {0.8, 2, 192}, {0.95, 3, 288}} { // 1.15 x mult x 5/60 x 1000, rounded
		e := newEnv(t, false)
		e.pressure.SetValue(tc.pressure)
		e.reportedOn(0, 20)
		u, h := e.basic() // rule tier=gold -> 2
		e.submit(rw("lab-01", 1.15))
		e.runOK()
		e.step(poll)
		s := lastSample(t, e, h.ID)
		if s.Multiplier != tc.mult {
			t.Fatalf("multiplier %v", s.Multiplier)
		}
		wantSample(t, s, 1.15, "reported", "", tc.debit)
		if got := e.debits(u); !eqInts(got, []int64{tc.debit}) {
			t.Fatalf("pressure %v: debits %v want %d", tc.pressure, got, tc.debit)
		}
	}
}

// ---- WGT-2 ----

func TestWGT2_OrderOverrideReportedRuleDefault(t *testing.T) {
	e := newEnv(t, false)
	e.reportedOn(0, 20)
	e.rules("tier=gold", 2)
	u := e.user("dev")
	sOv := e.setup("lab-01", true, map[string]string{"tier": "gold"})
	sRep := e.setup("lab-02", true, map[string]string{"tier": "gold"})
	sRule := e.setup("lab-03", true, map[string]string{"tier": "gold"})
	sDef := e.setup("lab-04", true, nil)
	e.setting("max_holds_per_user", "10")
	e.override(sOv, 3)
	hOv, hRep, hRule, hDef := e.hold(sOv, u), e.hold(sRep, u), e.hold(sRule, u), e.hold(sDef, u)
	e.submit(rw("lab-01", 1.15), rw("lab-02", 1.15)) // lab-03 and lab-04 not reported
	e.runOK()
	e.step(poll)
	wantSample(t, lastSample(t, e, hOv.ID), 3, "override", "", 250)
	wantSample(t, lastSample(t, e, hRep.ID), 1.15, "reported", "", 96)
	wantSample(t, lastSample(t, e, hRule.ID), 2, "rule", "tier=gold", 167)
	wantSample(t, lastSample(t, e, hDef.ID), 1, "default", "", 83)
}

func TestWGT2_ReportedBeatsRuleOfWeightZeroAndOfHigherWeight(t *testing.T) {
	for _, ruleW := range []float64{0, 9} {
		e := newEnv(t, false)
		e.reportedOn(0, 20)
		e.rules("tier=gold", ruleW)
		u := e.user("dev")
		h := e.hold(e.setup("lab-01", true, map[string]string{"tier": "gold"}), u)
		e.submit(rw("lab-01", 1.15))
		e.runOK()
		e.step(poll)
		wantSample(t, lastSample(t, e, h.ID), 1.15, "reported", "", 96)
	}
}

func TestWGT2_PolicyWeightReplacesResolvedWeightForThatHoldAndPoll(t *testing.T) {
	e := newEnv(t, false)
	e.reportedOn(0, 20)
	u, h := e.basic()
	e.submit(rw("lab-01", 1.15))
	e.runOK()
	e.policy.SetDefault(hourglass.Decision{Meter: true, Weight: 4, Reason: "policy"})
	e.step(poll)
	wantSample(t, lastSample(t, e, h.ID), 4, "policy", "", 333)
	e.policy.SetDefault(hourglass.Decision{Meter: true, Reason: "test"})
	e.submit(rw("lab-01", 1.15)) // keep the report fresh
	e.step(poll)
	wantSample(t, lastSample(t, e, h.ID), 1.15, "reported", "", 96) // only that poll
	if got := e.debits(u); !eqInts(got, []int64{333, 96}) {
		t.Fatalf("debits %v", got)
	}
}

// ---- WGT-3 ----

// Each fallback cause charges exactly what the same data charges with the
// feature never configured, for a setup priced by override, rule or default.
func TestWGT3_EachFallbackCauseChargesAsWithoutTheFeature(t *testing.T) {
	type pricing struct {
		name     string
		labels   map[string]string
		override *float64
	}
	three := 3.0
	pricings := []pricing{
		{"override", map[string]string{"tier": "gold"}, &three},
		{"rule", map[string]string{"tier": "gold"}, nil},
		{"default", nil, nil},
	}
	causes := []struct {
		name  string
		apply func(e *env)
	}{
		{"none sent", func(e *env) { e.reportedOn(0, 20) }},
		{"expired", func(e *env) { e.reportedOn(0, 20); e.submitAt(e.clk.Now().Add(-11*time.Minute), rw("lab-01", 1.15)) }},
		{"left out of the latest report", func(e *env) {
			e.reportedOn(0, 20)
			e.submit(rw("lab-01", 1.15))
			e.submit(rw("lab-02", 1.15))
		}},
		{"above the maximum", func(e *env) {
			e.reportedOn(0, 20)
			e.storeRaw(e.clk.Now(), store.ReportedWeight{SetupName: "lab-01", Weight: 25})
		}},
		{"below the minimum", func(e *env) {
			e.reportedOn(0.5, 20)
			e.storeRaw(e.clk.Now(), store.ReportedWeight{SetupName: "lab-01", Weight: 0.25})
		}},
		{"feature off", func(e *env) {
			e.reportedOff()
			e.storeRaw(e.clk.Now(), store.ReportedWeight{SetupName: "lab-01", Weight: 1.15})
		}},
	}
	run := func(p pricing, apply func(e *env)) []store.UsageSample {
		e := newEnv(t, false)
		e.rules("tier=gold", 2)
		u := e.user("dev")
		s := e.setup("lab-01", true, p.labels)
		if p.override != nil {
			e.override(s, *p.override)
		}
		h := e.hold(s, u)
		if apply != nil {
			apply(e)
		}
		e.runOK()
		e.step(poll)
		return e.samples(h.ID)
	}
	for _, p := range pricings {
		base := run(p, nil)
		for _, c := range causes {
			got := run(p, c.apply)
			if len(got) != len(base) {
				t.Fatalf("%s/%s: %d samples, want %d", p.name, c.name, len(got), len(base))
			}
			for i := range got {
				g, b := got[i], base[i]
				if g.Weight != b.Weight || g.DebitMT != b.DebitMT || g.RuleSelector != b.RuleSelector ||
					g.WeightSource != p.name || b.WeightSource != p.name {
					t.Fatalf("%s/%s: sample %d %+v, without the feature %+v", p.name, c.name, i, g, b)
				}
			}
		}
	}
}

// A database error reading the stored report fails the poll.
func TestWGT3_DatabaseErrorReadingTheReportFailsThePoll(t *testing.T) {
	e := newEnv(t, false)
	e.reportedOn(0, 20)
	u, _ := e.basic()
	e.submit(rw("lab-01", 1.15))
	e.runOK()
	u2 := url.URL{Scheme: "file", Opaque: (&url.URL{Path: e.dbPath}).EscapedPath()}
	db, err := sql.Open("sqlite", u2.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`ALTER TABLE reported_weights RENAME TO reported_weights_gone`); err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(poll)
	if err := e.c.RunOnce(bg); err == nil {
		t.Fatal("poll succeeded with the weights report unreadable")
	}
	if len(e.debits(u)) != 0 {
		t.Fatalf("failed poll debited: %v", e.debits(u))
	}
}

// A2, first case: a hold metered before its setup is reported is charged at
// the fallback weight; once reported, the next poll charges the report.
func TestWGT3_HoldMeteredBeforeItsSetupIsReported(t *testing.T) {
	e := newEnv(t, false)
	e.reportedOn(0, 20)
	_, h := e.basic()
	e.submit(rw("lab-other", 1.15)) // the report does not hold lab-01 yet
	e.runOK()
	wantSample(t, lastSample(t, e, h.ID), 2, "rule", "tier=gold", 0) // metering starts
	e.step(poll)
	wantSample(t, lastSample(t, e, h.ID), 2, "rule", "tier=gold", 167)
	e.submit(rw("lab-01", 1.15))
	e.step(poll)
	wantSample(t, lastSample(t, e, h.ID), 1.15, "reported", "", 96)
}

// A2, second case (D58): the poll that completes a pending release charges
// its interval at the fallback weight when the setup has left the report.
func TestWGT3_PendingReleaseCompletedAfterSetupLeftTheReport(t *testing.T) {
	e, _, user, h := pendingFixtureReported(t)
	wantSample(t, lastSample(t, e, h.ID), 1.15, "reported", "", 96)
	before := len(e.debits(user))
	e.submit()   // the client drops lab-01 from its weights report first...
	e.deployed() // ...then from its deployments
	e.step(poll)
	cur := e.getHold(h.ID)
	if cur.ReleasedAt == nil || !cur.ReleasedAt.Equal(e.clk.Now()) {
		t.Fatalf("pending release not completed: %+v", cur)
	}
	wantSample(t, lastSample(t, e, h.ID), 2, "rule", "tier=gold", 167)
	if got := e.debits(user); len(got) != before+1 || got[len(got)-1] != 167 {
		t.Fatalf("debits %v", got)
	}
}

// pendingFixtureReported is pendingFixture with a rule (tier=gold -> 2) and
// a reported weight of 1.15 for lab-01.
func pendingFixtureReported(t *testing.T) (*env, *holds.Service, int64, store.Hold) {
	t.Helper()
	e := newEnv(t, true)
	e.reportedOn(0, 20)
	e.rules("tier=gold", 2)
	user := e.user("dev")
	h := e.hold(e.setup("lab-01", true, map[string]string{"tier": "gold"}), user)
	e.submit(rw("lab-01", 1.15))
	e.deployed("lab-01")
	e.runOK()
	e.step(poll)
	svc := e.holdsSvc()
	if _, err := svc.Release(bg, user, h.ID, holds.ReleaseOpts{}); err != nil {
		t.Fatal(err)
	}
	if cur := e.getHold(h.ID); cur.ReleaseRequestedAt == nil || cur.ReleasedAt != nil {
		t.Fatalf("release not pending: %+v", cur)
	}
	return e, svc, user, h
}

// ---- WGT-4 ----

func TestWGT4_ChangedWeightPricesTheNextIntervalOnly(t *testing.T) {
	e := newEnv(t, false)
	e.reportedOn(0, 20)
	u, h := e.basic()
	e.submit(rw("lab-01", 1.15))
	e.runOK()
	e.step(poll)
	samplesBefore := e.samples(h.ID)
	ledgerBefore, err := e.st.ListLedger(bg, u, e.led.CurrentPeriod())
	if err != nil {
		t.Fatal(err)
	}
	// A new weight observed one second before the next poll prices the whole
	// interval that poll charges.
	e.clk.Advance(poll - time.Second)
	e.submit(rw("lab-01", 2.3))
	e.step(time.Second)
	samples := e.samples(h.ID)
	wantSample(t, samples[len(samples)-1], 2.3, "reported", "", 192) // 2.3 x 5/60 x 1000
	if !reflect.DeepEqual(samples[:len(samplesBefore)], samplesBefore) {
		t.Fatalf("earlier samples changed:\n%+v\n%+v", samples[:len(samplesBefore)], samplesBefore)
	}
	ledger, err := e.st.ListLedger(bg, u, e.led.CurrentPeriod())
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger) != len(ledgerBefore)+1 || !reflect.DeepEqual(ledger[:len(ledgerBefore)], ledgerBefore) {
		t.Fatalf("earlier ledger rows changed:\n%+v\n%+v", ledger, ledgerBefore)
	}
}

// ---- WGT-5 ----

func TestWGT5_EverySampleHasOneOfFiveSourcesAndSelectorOnlyForRule(t *testing.T) {
	e := newEnv(t, false)
	e.reportedOn(0, 20)
	e.rules("tier=gold", 2)
	e.setting("max_holds_per_user", "10")
	u := e.user("dev")
	gold := map[string]string{"tier": "gold"}
	ids := map[string]int64{}
	for _, n := range []string{"override", "reported", "rule", "default", "policy"} {
		labels := gold
		if n == "default" {
			labels = nil
		}
		s := e.setup("lab-"+n, true, labels)
		if n == "override" {
			e.override(s, 3)
		}
		ids[n] = e.hold(s, u).ID
	}
	e.policy.SetFunc(func(h hourglass.HoldState, _ hourglass.Snapshot) hourglass.Decision {
		if h.Setup.Name == "lab-policy" {
			return hourglass.Decision{Meter: true, Weight: 5, Reason: "p"}
		}
		return hourglass.Decision{Meter: true, Reason: "test"}
	})
	e.submit(rw("lab-reported", 1.15), rw("lab-policy", 1.15), rw("lab-override", 1.15))
	e.runOK()
	e.step(poll)
	valid := map[string]bool{"override": true, "reported": true, "rule": true, "default": true, "policy": true}
	for n, id := range ids {
		ss := e.samples(id)
		if len(ss) != 2 {
			t.Fatalf("%s: %d samples", n, len(ss))
		}
		for _, s := range ss {
			if !valid[s.WeightSource] || s.WeightSource != n {
				t.Fatalf("%s: source %q", n, s.WeightSource)
			}
			if (s.RuleSelector != "") != (s.WeightSource == "rule") {
				t.Fatalf("%s: selector %q with source %q", n, s.RuleSelector, s.WeightSource)
			}
		}
	}
}

// ---- WGT-6 ----

// The snapshot passed to the pressure source and to the policy carries the
// base weight of every enabled setup, whatever is reported.
func TestWGT6_SnapshotCarriesTheBaseWeight(t *testing.T) {
	e := newEnv(t, false)
	e.reportedOn(0, 20)
	e.rules("tier=gold", 2)
	e.setting("max_holds_per_user", "10")
	u := e.user("dev")
	gold := map[string]string{"tier": "gold"}
	sRule := e.setup("lab-01", true, gold)
	sDef := e.setup("lab-02", true, nil)
	sOv := e.setup("lab-03", true, gold)
	e.setup("lab-04", false, gold) // disabled: not in the snapshot
	e.override(sOv, 3)
	e.hold(sRule, u)
	e.hold(sDef, u)
	e.hold(sOv, u)
	want := map[string]float64{"lab-01": 2, "lab-02": 1, "lab-03": 3}
	var policySnaps []hourglass.Snapshot
	var policySetups []hourglass.SetupState
	e.policy.SetFunc(func(h hourglass.HoldState, s hourglass.Snapshot) hourglass.Decision {
		policySnaps = append(policySnaps, s)
		policySetups = append(policySetups, h.Setup)
		return hourglass.Decision{Meter: true}
	})
	check := func(label string, setups []hourglass.SetupState) {
		t.Helper()
		got := map[string]float64{}
		for _, s := range setups {
			got[s.Name] = s.Weight
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: weights %v want %v", label, got, want)
		}
	}
	for i, report := range [][]reported.Weight{nil, {rw("lab-01", 7), rw("lab-02", 0.5), rw("lab-03", 9), rw("lab-04", 4)}} {
		if report != nil {
			e.submit(report...)
		}
		policySnaps, policySetups = nil, nil
		if i == 0 {
			e.runOK()
		} else {
			e.step(poll)
		}
		snaps := e.pressure.Snapshots()
		check("pressure source", snaps[len(snaps)-1].Setups)
		for _, s := range policySnaps {
			check("policy snapshot", s.Setups)
		}
		for _, s := range policySetups {
			if s.Weight != want[s.Name] {
				t.Fatalf("policy hold setup %s weight %v", s.Name, s.Weight)
			}
		}
	}
	// The report was applied to the debits all the same.
	if s := e.samples(1); s[len(s)-1].WeightSource != "reported" || s[len(s)-1].Weight != 7 {
		t.Fatalf("debit did not use the report: %+v", s[len(s)-1])
	}
}

// ---- WGT-12 ----

func TestWGT12_LoweredMaximumChargesTheFallbackAtTheNextPoll(t *testing.T) {
	e := newEnv(t, false)
	e.reportedOn(0, 20)
	_, h := e.basic()
	e.submit(rw("lab-01", 15))
	e.runOK()
	e.step(poll)
	wantSample(t, lastSample(t, e, h.ID), 15, "reported", "", 1250)
	e.reportedOn(0, 10) // the admin lowers the maximum; no new report
	e.step(poll)
	wantSample(t, lastSample(t, e, h.ID), 2, "rule", "tier=gold", 167)
}
