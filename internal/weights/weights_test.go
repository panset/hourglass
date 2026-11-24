// SPDX-License-Identifier: MIT

package weights

import (
	"fmt"
	"maps"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/rules"
	"github.com/panset/hourglass/internal/settings"
)

var at = time.Date(2026, 11, 23, 10, 15, 0, 0, time.UTC)

func f(v float64) *float64 { return &v }

// cfgOn: reported weights on, limits [0.5, 20], max_age 10m, default 1.
func cfgOn() settings.Settings {
	c := settings.Defaults()
	c.ReportedWeights = settings.ReportedWeights{Enabled: true, Min: 0.5, Max: f(20), MaxAge: 10 * time.Minute}
	return c
}

func cfgOff() settings.Settings {
	c := cfgOn()
	c.ReportedWeights.Enabled = false // limits stay stored, as after switching off
	return c
}

var gold = map[string]string{"tier": "gold"}

func rateRules(w string) []rules.Rule {
	return []rules.Rule{{ID: 1, Kind: rules.KindRate, Position: 1, Selector: "tier=gold", Value: w}}
}

// entryCase is one shape of the stored report as seen from setup lab-03.
type entryCase struct {
	name     string
	report   func() *Report
	weight   float64 // the entry's weight when present
	present  bool
	expired  bool
	inBounds bool
}

func reportWith(observed time.Time, w float64) *Report {
	return NewReport(observed, []Entry{
		{Setup: "lab-03", Weight: w, Attributes: map[string]string{"memory-gb": "512", "tier": "gold"}},
		{Setup: "lab-07", Weight: 0.8},
	})
}

func entryCases() []entryCase {
	fresh := at.Add(-5 * time.Minute)
	stale := at.Add(-11 * time.Minute)
	return []entryCase{
		{name: "no report", report: func() *Report { return nil }},
		{name: "absent", report: func() *Report { return NewReport(fresh, []Entry{{Setup: "lab-07", Weight: 0.8}}) }},
		{name: "usable", report: func() *Report { return reportWith(fresh, 1.15) }, weight: 1.15, present: true, inBounds: true},
		{name: "expired", report: func() *Report { return reportWith(stale, 1.15) }, weight: 1.15, present: true, expired: true, inBounds: true},
		{name: "above max", report: func() *Report { return reportWith(fresh, 25) }, weight: 25, present: true},
		{name: "below min", report: func() *Report { return reportWith(fresh, 0.25) }, weight: 0.25, present: true},
		{name: "expired and above max", report: func() *Report { return reportWith(stale, 25) }, weight: 25, present: true, expired: true},
		{name: "exactly min", report: func() *Report { return reportWith(fresh, 0.5) }, weight: 0.5, present: true, inBounds: true},
		{name: "exactly max", report: func() *Report { return reportWith(fresh, 20) }, weight: 20, present: true, inBounds: true},
		{name: "exactly max_age old", report: func() *Report { return reportWith(at.Add(-10*time.Minute), 3) }, weight: 3, present: true, inBounds: true},
		{name: "observed in the future", report: func() *Report { return reportWith(at.Add(time.Minute), 3) }, weight: 3, present: true, inBounds: true},
	}
}

// The resolver table test of the design's Tests section: every combination of
// override, reported entry, feature on/off and matching rule (incl. weight 0).
func TestWGT2_ResolverTable(t *testing.T) {
	ruleCases := []struct {
		name    string
		rules   []rules.Rule
		labels  map[string]string
		ruleW   float64
		matched bool
	}{
		{"rule weight 2", rateRules("2"), gold, 2, true},
		{"rule weight 0", rateRules("0"), gold, 0, true},
		{"no matching rule", rateRules("2"), map[string]string{"tier": "free"}, 1, false},
		{"no rules", nil, gold, 1, false},
	}
	for _, on := range []bool{true, false} {
		for _, ov := range []*float64{nil, f(7), f(0)} {
			for _, rc := range ruleCases {
				for _, ec := range entryCases() {
					cfg := cfgOff()
					if on {
						cfg = cfgOn()
					}
					name := fmt.Sprintf("on=%v/override=%v/%s/%s", on, fmtPtr(ov), rc.name, ec.name)
					t.Run(name, func(t *testing.T) {
						got, err := Resolve(Setup{Name: "lab-03", Labels: rc.labels, Override: ov}, rc.rules, cfg, ec.report(), at)
						if err != nil {
							t.Fatal(err)
						}
						// Expected, from the A1 table and rules.
						base, src, sel := rc.ruleW, SourceDefault, ""
						if rc.matched {
							src, sel = SourceRule, "tier=gold"
						}
						if ov != nil {
							base, src = *ov, SourceOverride
						}
						wantW := base
						var wantRep *float64
						var wantNA NotApplied
						wantAttrs := map[string]string{}
						if on && ec.present {
							wantRep = f(ec.weight)
							usable := !ec.expired && ec.inBounds
							switch {
							case ec.expired:
								wantNA = NotAppliedExpired
							case !ec.inBounds:
								wantNA = NotAppliedOutOfBounds
							case ov != nil:
								wantNA = NotAppliedOverride
							}
							if usable {
								wantAttrs = map[string]string{"memory-gb": "512", "tier": "gold"}
								if ov == nil {
									wantW, src = ec.weight, SourceReported
								}
							}
						}
						if got.Weight != wantW || got.Source != src || got.BaseWeight != base || got.RuleSelector != sel ||
							fmtPtr(got.Reported) != fmtPtr(wantRep) || got.NotApplied != wantNA ||
							got.Attributes == nil || !maps.Equal(got.Attributes, wantAttrs) {
							t.Fatalf("got %+v (reported %s)\nwant weight %v source %s base %v selector %q reported %s not_applied %q attrs %v",
								got, fmtPtr(got.Reported), wantW, src, base, sel, fmtPtr(wantRep), wantNA, wantAttrs)
						}
						// With the feature off and no override: exactly rules.Weight.
						if !on && ov == nil {
							w, _, _, _ := rules.Weight(rc.rules, rc.labels, cfg.DefaultWeight)
							if got.Weight != w {
								t.Fatalf("feature off: weight %v, rules.Weight %v", got.Weight, w)
							}
						}
					})
				}
			}
		}
	}
}

func fmtPtr(p *float64) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprint(*p)
}

func TestWGT2_ReportedBeatsRuleOfWeightZeroAndOfHigherWeight(t *testing.T) {
	for _, rw := range []string{"0", "9"} {
		got, err := Resolve(Setup{Name: "lab-03", Labels: gold}, rateRules(rw), cfgOn(), reportWith(at, 1.15), at)
		if err != nil || got.Weight != 1.15 || got.Source != SourceReported || got.RuleSelector != "tier=gold" {
			t.Fatalf("rule %s: %+v, %v", rw, got, err)
		}
		if want, _ := rules.ParseWeight(rw); got.BaseWeight != want {
			t.Fatalf("rule %s: base %v", rw, got.BaseWeight)
		}
	}
}

func TestWGT2_OverrideBeatsReportedAndKeepsAttributes(t *testing.T) {
	got, err := Resolve(Setup{Name: "lab-03", Labels: gold, Override: f(4)}, rateRules("2"), cfgOn(), reportWith(at, 1.15), at)
	if err != nil || got.Weight != 4 || got.Source != SourceOverride || got.BaseWeight != 4 ||
		got.NotApplied != NotAppliedOverride || *got.Reported != 1.15 || got.Attributes["memory-gb"] != "512" {
		t.Fatalf("%+v, %v", got, err)
	}
}

// A weight of 0 is usable when the minimum allows it, and makes the setup free.
func TestWGT2_ReportedZeroWithMinimumZero(t *testing.T) {
	cfg := cfgOn()
	cfg.ReportedWeights.Min = 0
	got, err := Resolve(Setup{Name: "lab-03", Labels: gold}, rateRules("2"), cfg, reportWith(at, 0), at)
	if err != nil || got.Weight != 0 || got.Source != SourceReported || got.NotApplied != "" {
		t.Fatalf("%+v, %v", got, err)
	}
}

// WGT-12 at the resolver: the limits are read at each call, so a lower
// maximum excludes the stored weight with no new report.
func TestWGT12_LimitsCheckedAtReadTime(t *testing.T) {
	rep := reportWith(at.Add(-time.Minute), 15)
	s := Setup{Name: "lab-03", Labels: gold}
	cfg := cfgOn()
	if got, _ := Resolve(s, rateRules("2"), cfg, rep, at); got.Source != SourceReported || got.Weight != 15 {
		t.Fatalf("before: %+v", got)
	}
	cfg.ReportedWeights.Max = f(10)
	got, _ := Resolve(s, rateRules("2"), cfg, rep, at)
	if got.Source != SourceRule || got.Weight != 2 || got.NotApplied != NotAppliedOutOfBounds || *got.Reported != 15 || len(got.Attributes) != 0 {
		t.Fatalf("after: %+v", got)
	}
}

// A rule that cannot be evaluated is the caller's error, also under an override.
func TestWGT2_BadRuleIsAnError(t *testing.T) {
	bad := []rules.Rule{{ID: 9, Kind: rules.KindRate, Position: 1, Selector: "tier=gold", Value: "x"}}
	for _, ov := range []*float64{nil, f(3)} {
		if _, err := Resolve(Setup{Name: "lab-03", Labels: gold, Override: ov}, bad, cfgOn(), reportWith(at, 1.15), at); err == nil {
			t.Fatalf("override %s: no error", fmtPtr(ov))
		}
	}
	unparsable := []rules.Rule{{ID: 9, Kind: rules.KindRate, Position: 1, Selector: "tier in (", Value: "1"}}
	if _, err := Resolve(Setup{Name: "lab-03", Labels: gold}, unparsable, cfgOn(), nil, at); err == nil {
		t.Fatal("unparsable selector: no error")
	}
}

// The result must not alias the report's attribute map.
func TestResolveAttributesAreACopy(t *testing.T) {
	rep := reportWith(at, 1.15)
	got, _ := Resolve(Setup{Name: "lab-03"}, nil, cfgOn(), rep, at)
	got.Attributes["memory-gb"] = "1"
	again, _ := Resolve(Setup{Name: "lab-03"}, nil, cfgOn(), rep, at)
	if again.Attributes["memory-gb"] != "512" {
		t.Fatal("attributes alias the report")
	}
}
