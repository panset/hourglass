// SPDX-License-Identifier: MIT

package collector

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

const v110Dir = "../../testdata/upgrade-v1.1.0"

// v110Poll is one poll of testdata/upgrade-v1.1.0/poll.json.
type v110Poll struct {
	At                  time.Time        `json:"at"`
	FakePressureValue   float64          `json:"fake_pressure_value"`
	ProviderAllocations []string         `json:"provider_allocations"`
	BalancesBefore      map[string]int64 `json:"balances_before_mt"`
	BalancesAfter       map[string]int64 `json:"balances_after_mt"`
	Snapshot            any              `json:"snapshot_to_pressure_source"`
	PolicyCalls         any              `json:"policy_calls"`
	RawPressure         float64          `json:"raw_pressure"`
	SmoothedPressure    float64          `json:"smoothed_pressure"`
	Multiplier          float64          `json:"multiplier"`
	SamplesWritten      any              `json:"usage_samples_written"`
	LedgerWritten       any              `json:"ledger_rows_written"`
	HoldsAfter          any              `json:"holds_after"`
}

// asJSON is v as generic JSON, so values compare like the recorded file.
func asJSON(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// WGT-19 (collector part, owned in full by milestone 7): the three polls the
// v1.1.0 release ran over its recorded database, replayed by this collector
// with reported weights off, pass the same snapshot to the pressure source
// and the policy and write the same usage samples, ledger rows, balances and
// hold states. The new weight_source column is the only addition.
func TestWGT19_ReplayV110PollsWithFeatureOff(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(v110Dir, "poll.json"))
	if err != nil {
		// Never skip: a missing fixture must fail (Bazel: is it in the test's data?).
		t.Fatalf("the v1.1.0 fixture is missing: %v", err)
	}
	var doc struct {
		Start    time.Time  `json:"start"`
		Timezone string     `json:"timezone"`
		Polls    []v110Poll `json:"polls"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	db, err := os.ReadFile(filepath.Join(v110Dir, "hourglass.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "copy.db")
	if err := os.WriteFile(path, db, 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(path) // migrates to the current schema
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	loc, err := time.LoadLocation(doc.Timezone)
	if err != nil {
		t.Fatal(err)
	}
	clk := hourglasstest.NewFakeClock(doc.Start)
	prov := hourglasstest.NewFakeProvider()
	prov.SetName("reported")
	ps := hourglasstest.NewFakePressureSource()
	ps.SetName("reported")
	pol := hourglasstest.NewFakePolicy()
	var calls []map[string]any
	pol.SetFunc(func(h hourglass.HoldState, s hourglass.Snapshot) hourglass.Decision {
		d := hourglass.Decision{Meter: true, Reason: "test reason"} // the recorder's recPolicy
		if h.Setup.Name == "lab-08" {
			d = hourglass.Decision{Meter: true, Weight: 3.25, Reason: "policy weight"}
		}
		calls = append(calls, map[string]any{"hold": h, "snapshot": s, "decision": d})
		return d
	})
	rec := &recorder{}
	led := ledger.NewService(st, rec, clk, loc)
	c := New(Deps{Store: st, Provider: prov, Pressure: ps, Policy: pol, Events: rec, Ledger: led, Clock: clk,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})

	var userIDs, holdIDs []int64
	for k := range doc.Polls[0].BalancesBefore {
		id, _ := strconv.ParseInt(k, 10, 64)
		userIDs = append(userIDs, id)
	}
	for k := range doc.Polls[0].HoldsAfter.(map[string]any) {
		id, _ := strconv.ParseInt(k, 10, 64)
		holdIDs = append(holdIDs, id)
	}
	sort.Slice(holdIDs, func(i, j int) bool { return holdIDs[i] < holdIDs[j] })
	periods := []string{"2026-10", "2026-11"}
	balances := func() map[string]int64 {
		m := map[string]int64{}
		for _, id := range userIDs {
			u, err := st.GetUser(bg, id)
			if err != nil {
				t.Fatal(err)
			}
			m[strconv.FormatInt(id, 10)] = u.BalanceMT
		}
		return m
	}
	ledgerRows := func() []store.LedgerEntry {
		var out []store.LedgerEntry
		for _, id := range userIDs {
			for _, p := range periods {
				rows, err := st.ListLedger(bg, id, p)
				if err != nil {
					t.Fatal(err)
				}
				out = append(out, rows...)
			}
		}
		return out
	}
	samples := func() []store.UsageSample {
		var out []store.UsageSample
		for _, h := range holdIDs {
			ss, err := st.ListUsageSamples(bg, h)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, ss...)
		}
		return out
	}

	for i, p := range doc.Polls {
		clk.Advance(5 * time.Minute)
		if !clk.Now().Equal(p.At) {
			t.Fatalf("poll %d: clock %s, recorded %s", i+1, clk.Now(), p.At)
		}
		ps.SetValue(p.FakePressureValue)
		var allocs []hourglass.Allocation
		for _, n := range p.ProviderAllocations {
			allocs = append(allocs, hourglass.Allocation{ResourceName: n})
		}
		prov.SetAllocations(allocs...)
		calls = nil
		if got := balances(); !reflect.DeepEqual(got, p.BalancesBefore) {
			t.Fatalf("poll %d: balances before %v, recorded %v", i+1, got, p.BalancesBefore)
		}
		seenLedger, seenSamples := map[int64]bool{}, map[int64]bool{}
		for _, r := range ledgerRows() {
			seenLedger[r.ID] = true
		}
		for _, s := range samples() {
			seenSamples[s.ID] = true
		}
		if err := c.RunOnce(bg); err != nil {
			t.Fatalf("poll %d: %v", i+1, err)
		}
		snaps := ps.Snapshots()
		if got := asJSON(t, snaps[len(snaps)-1]); !reflect.DeepEqual(got, p.Snapshot) {
			t.Fatalf("poll %d: snapshot to the pressure source\n%v\nrecorded\n%v", i+1, got, p.Snapshot)
		}
		if got := asJSON(t, calls); !reflect.DeepEqual(got, p.PolicyCalls) {
			t.Fatalf("poll %d: policy calls\n%v\nrecorded\n%v", i+1, got, p.PolicyCalls)
		}
		v := c.LastPoll()
		if v.RawPressure != p.RawPressure || v.SmoothedPressure != p.SmoothedPressure || v.Multiplier != p.Multiplier {
			t.Fatalf("poll %d: pressure %v %v %v, recorded %v %v %v", i+1, v.RawPressure, v.SmoothedPressure, v.Multiplier,
				p.RawPressure, p.SmoothedPressure, p.Multiplier)
		}
		var newSamples, newLedger []map[string]any
		for _, x := range samples() {
			if seenSamples[x.ID] {
				continue
			}
			newSamples = append(newSamples, map[string]any{"id": x.ID, "hold_id": x.HoldID, "polled_at": x.PolledAt.UTC().Format(time.RFC3339Nano),
				"weight": x.Weight, "rule_selector": x.RuleSelector, "pressure": x.Pressure, "multiplier": x.Multiplier,
				"debit_mt": x.DebitMT, "reason": x.Reason})
			// The one addition: every new sample records its source.
			want := "default"
			switch {
			case x.Reason == "policy weight":
				want = "policy"
			case x.RuleSelector != "":
				want = "rule"
			}
			if x.WeightSource != want {
				t.Fatalf("poll %d: sample %d source %q, want %q", i+1, x.ID, x.WeightSource, want)
			}
		}
		for _, r := range ledgerRows() {
			if !seenLedger[r.ID] {
				newLedger = append(newLedger, map[string]any{"id": r.ID, "user_id": r.UserID, "period": r.Period, "type": r.Type,
					"amount_mt": r.AmountMT, "hold_id": r.HoldID, "reason": r.Reason, "created_at": r.CreatedAt.UTC().Format(time.RFC3339Nano)})
			}
		}
		sort.Slice(newLedger, func(a, b int) bool { return newLedger[a]["id"].(int64) < newLedger[b]["id"].(int64) })
		sort.Slice(newSamples, func(a, b int) bool { return newSamples[a]["id"].(int64) < newSamples[b]["id"].(int64) })
		recSamples := asJSON(t, p.SamplesWritten).([]any)
		sort.Slice(recSamples, func(a, b int) bool {
			return recSamples[a].(map[string]any)["id"].(float64) < recSamples[b].(map[string]any)["id"].(float64)
		})
		if got := asJSON(t, newSamples); !reflect.DeepEqual(got, recSamples) {
			t.Fatalf("poll %d: usage samples\n%v\nrecorded\n%v", i+1, got, recSamples)
		}
		if got := asJSON(t, newLedger); !reflect.DeepEqual(got, p.LedgerWritten) {
			t.Fatalf("poll %d: ledger rows\n%v\nrecorded\n%v", i+1, got, p.LedgerWritten)
		}
		if got := balances(); !reflect.DeepEqual(got, p.BalancesAfter) {
			t.Fatalf("poll %d: balances after %v, recorded %v", i+1, got, p.BalancesAfter)
		}
		holdState := map[string]any{}
		for _, h := range holdIDs {
			row, err := st.GetHold(bg, h)
			if err != nil {
				t.Fatal(err)
			}
			holdState[strconv.FormatInt(h, 10)] = map[string]any{"released": row.ReleasedAt != nil,
				"release_requested": row.ReleaseRequestedAt != nil, "metering_started": row.MeteringStartedAt != nil,
				"deployed": row.DeployedAt != nil}
		}
		if got := asJSON(t, holdState); !reflect.DeepEqual(got, p.HoldsAfter) {
			t.Fatalf("poll %d: holds\n%v\nrecorded\n%v", i+1, got, p.HoldsAfter)
		}
	}
}
