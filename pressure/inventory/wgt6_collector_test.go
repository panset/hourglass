// SPDX-License-Identifier: MIT

package inventory

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/collector"
	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/reported"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

// wgt6Run polls twice with the real inventory source over three setups
// (rule tier=gold -> 2, default 1, override 3), one of each held, and returns
// the second poll's view and the last usage sample of the first hold. With
// report set, a weights report far from the base weights is in force.
func wgt6Run(t *testing.T, report []reported.Weight) (collector.PollView, store.UsageSample) {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	clk := hourglasstest.NewFakeClock(time.Time{})
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(st.PutSetting(ctx, "reported_weights", `{"enabled":true,"min":0,"max":20,"max_age":"10m"}`, nil, clk.Now()))
	must(st.PutSetting(ctx, "max_holds_per_user", "10", nil, clk.Now()))
	must(st.ReplaceRules(ctx, "rate", []store.Rule{{Selector: "tier=gold", Value: 2}}, nil, clk.Now()))
	u, err := st.CreateOIDCUser(ctx, "dev", "dev@example.test", "dev", clk.Now())
	must(err)
	must(st.SetUserRoles(ctx, u.ID, []string{store.RoleDeveloper}))
	var holdIDs []int64
	for i, labels := range []map[string]string{{"tier": "gold"}, nil, {"tier": "gold"}, {"tier": "gold"}} {
		id, err := st.CreateSetup(ctx, store.Setup{Name: []string{"lab-01", "lab-02", "lab-03", "lab-04"}[i], Enabled: true,
			Labels: labels, DetailsJSON: `{}`})
		must(err)
		if i == 2 {
			three := 3.0
			must(st.SetSetupWeightOverride(ctx, id, &three))
		}
		if i < 2 { // lab-01 and lab-02 held; lab-03 and lab-04 free
			h, err := st.CreateHold(ctx, id, u.ID, clk.Now())
			must(err)
			holdIDs = append(holdIDs, h.ID)
		}
	}
	if report != nil {
		_, err := reported.New(st, clk).SubmitWeights(ctx, "reporter", nil, report, reported.WeightLimits{Min: 0, Max: 20})
		must(err)
	}
	pol := hourglasstest.NewFakePolicy()
	pol.SetDefault(hourglass.Decision{Meter: true})
	led := ledger.NewService(st, nil, clk, time.UTC)
	c := collector.New(collector.Deps{Store: st, Pressure: source{}, Policy: pol, Ledger: led, Clock: clk,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	must(c.RunOnce(ctx))
	clk.Advance(5 * time.Minute)
	must(c.RunOnce(ctx))
	ss, err := st.ListUsageSamples(ctx, holdIDs[0])
	must(err)
	return c.LastPoll(), ss[len(ss)-1]
}

// WGT-6 with the bundled inventory source: a weights report changes neither
// pressure nor multiplier. Base weights 2 + 1 held of 2 + 1 + 3 + 2 enabled.
func TestWGT6_InventoryPressureIgnoresReportedWeights(t *testing.T) {
	without, s0 := wgt6Run(t, nil)
	with, s1 := wgt6Run(t, []reported.Weight{
		{Setup: "lab-01", Weight: 0.1}, {Setup: "lab-02", Weight: 0.1}, {Setup: "lab-03", Weight: 19}, {Setup: "lab-04", Weight: 19}})
	if want := 3.0 / 8; without.RawPressure != want || with.RawPressure != want {
		t.Fatalf("raw pressure without %v, with %v; want %v", without.RawPressure, with.RawPressure, want)
	}
	if with.SmoothedPressure != without.SmoothedPressure || with.Multiplier != without.Multiplier {
		t.Fatalf("view with report %+v, without %+v", with, without)
	}
	// The report was in force for the debit all the same.
	if s0.WeightSource != "rule" || s1.WeightSource != "reported" || s1.Weight != 0.1 {
		t.Fatalf("samples without %+v, with %+v", s0, s1)
	}
}
