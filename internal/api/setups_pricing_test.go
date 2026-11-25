// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/metering"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// Tests of the pricing object of GET /api/v1/setups: the multipliers and the
// pressure every cost of one response was computed with
// (docs/design/setups-declutter.md, UI-18; D94(b)).

// pricingBands has a base above 0, so the multiplier before the first poll is
// not 0, and multipliers with decimals.
const pricingBands = `{"base":0.5,"steps":[{"from":0.5,"multiplier":1.25},{"from":0.75,"multiplier":2.5},{"from":0.9,"multiplier":3.75}]}`

// pricingOf is the pricing object of a setups response; it fails the test when
// it is missing or has other keys.
func pricingOf(t *testing.T, list map[string]any) map[string]any {
	t.Helper()
	p, ok := list["pricing"].(map[string]any)
	if !ok {
		t.Fatalf("pricing = %v (%T), want an object", list["pricing"], list["pricing"])
	}
	if len(p) != 3 {
		t.Errorf("pricing keys %v, want multiplier, peak_multiplier and pressure", p)
	}
	for _, k := range []string{"multiplier", "peak_multiplier"} {
		if _, ok := p[k].(float64); !ok {
			t.Errorf("pricing.%s = %v (%T), want a number", k, p[k], p[k])
		}
	}
	if _, ok := p["pressure"]; !ok {
		t.Error("pricing.pressure missing")
	}
	return p
}

// wantCostsFromPricing checks that every setup's two costs are its weight at
// pricing.multiplier and at pricing.peak_multiplier, as the server computes a
// cost, and returns the costs by setup name.
func wantCostsFromPricing(t *testing.T, step string, list map[string]any) map[string][2]float64 {
	t.Helper()
	p := pricingOf(t, list)
	mult, peak := p["multiplier"].(float64), p["peak_multiplier"].(float64)
	out := map[string][2]float64{}
	ss := list["setups"].([]any)
	if len(ss) == 0 {
		t.Fatalf("%s: no setups", step)
	}
	for _, r := range ss {
		s := r.(map[string]any)
		w := s["weight"].(float64)
		c := s["cost_per_hour_tokens"].(map[string]any)
		now, atPeak := c["now"].(float64), c["at_peak"].(float64)
		wantNow, wantPeak := tokens(metering.HourlyCostMT(w, mult)), tokens(metering.HourlyCostMT(w, peak))
		if now != wantNow || atPeak != wantPeak {
			t.Errorf("%s: %v weight %v: costs now %v at_peak %v, want %v and %v from multipliers %v and %v",
				step, s["name"], w, now, atPeak, wantNow, wantPeak, mult, peak)
		}
		out[s["name"].(string)] = [2]float64{now, atPeak}
	}
	return out
}

func TestUI18_SetupsListCarriesTheMultipliersOfItsCosts(t *testing.T) {
	e := newREnv(t, wStart, nil)
	ctx := context.Background()
	if err := e.st.PutSetting(ctx, "bands", pricingBands, nil, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	// lab-01 reported 0, lab-02 reported 1.15, lab-03 reported a weight with many
	// decimals, lab-04 a rule weight (2) whose hold a metering policy prices at 4.
	e.addSetup("lab-04", map[string]string{"tier": "gold"})
	dev := e.newUser("dev-pricing", store.StatusActive, store.RoleDeveloper)
	other := e.newUser("dev-policy", store.StatusActive, store.RoleDeveloper)
	e.acquire(&dev, "lab-02")
	policyHold := e.acquire(&other, "lab-04")
	e.prov.SetAllocations(hourglassAlloc("lab-02"), hourglassAlloc("lab-04"))
	e.policy.SetFunc(func(h hourglass.HoldState, _ hourglass.Snapshot) hourglass.Decision {
		if h.Hold.ID == policyHold {
			return hourglass.Decision{Meter: true, Weight: 4, Reason: "policy weight"}
		}
		return hourglass.Decision{Meter: true, Reason: "metered"}
	})
	e.ok(`{"weights":[{"setup":"lab-01","weight":0},{"setup":"lab-02","weight":1.15},{"setup":"lab-03","weight":1.23456789}]}`)
	sessions := map[string]*sess{"developer": &dev, "admin": &e.adm}

	t.Run("before the first poll", func(t *testing.T) {
		for who, s := range sessions {
			l := e.getOK("/api/v1/setups", s)
			p := pricingOf(t, l)
			if p["pressure"] != nil {
				t.Errorf("%s: pricing.pressure = %v, want null before the first poll", who, p["pressure"])
			}
			if pr := e.getOK("/api/v1/pressure", s); pr["updated_at"] != nil {
				t.Errorf("%s: /pressure updated_at = %v, want null", who, pr["updated_at"])
			}
			if p["multiplier"] != 0.5 || p["peak_multiplier"] != 3.75 {
				t.Errorf("%s: pricing %v, want multiplier 0.5 (the bands' value for pressure 0) and peak 3.75", who, p)
			}
			wantCostsFromPricing(t, who+", before the first poll", l)
		}
	})

	e.pollOK() // metering starts; the sample's multiplier is 2.5 (pressure 0.8)
	e.clk.Advance(5 * time.Minute)
	e.ok(`{"weights":[{"setup":"lab-01","weight":0},{"setup":"lab-02","weight":1.15},{"setup":"lab-03","weight":1.23456789}]}`)
	e.pollOK() // the first debits; lab-04's sample is priced by the policy
	if s := e.lastSample(policyHold); s.WeightSource != "policy" || s.Weight != 4 {
		t.Fatalf("policy sample %+v", s)
	}
	e.clk.Advance(time.Minute)

	var devList, admList map[string]any
	t.Run("after a poll", func(t *testing.T) {
		for who, s := range sessions {
			l := e.getOK("/api/v1/setups", s)
			pr := e.getOK("/api/v1/pressure", s) // at the same fake time
			p := pricingOf(t, l)
			if p["multiplier"] != pr["multiplier"] || p["pressure"] != pr["smoothed"] {
				t.Errorf("%s: pricing %v, /pressure multiplier %v smoothed %v", who, p, pr["multiplier"], pr["smoothed"])
			}
			if p["multiplier"] != 2.5 || p["peak_multiplier"] != 3.75 {
				t.Errorf("%s: pricing %v, want multiplier 2.5 and peak 3.75", who, p)
			}
			if who == "developer" {
				devList = l
			} else {
				admList = l
			}
		}
	})

	t.Run("every cost is its weight at the response's multipliers", func(t *testing.T) {
		costs := wantCostsFromPricing(t, "developer", devList)
		wantCostsFromPricing(t, "admin", admList)
		for name, want := range map[string]float64{"lab-01": 0, "lab-02": 1.15, "lab-03": 1.23456789, "lab-04": 2} {
			if got := byName(t, devList["setups"], "name", name)["weight"]; got != want {
				t.Errorf("%s: weight %v, want %v", name, got, want)
			}
		}
		if c := costs["lab-01"]; c != [2]float64{0, 0} {
			t.Errorf("lab-01 weight 0: costs %v, want 0 and 0", c)
		}
		// The policy-priced hold shows the setup's price (WGT-14), with its own multiplier.
		if m := byName(t, devList["setups"], "name", "lab-04")["metering"].(map[string]any); m["active"] != true {
			t.Errorf("lab-04 metering %v, want active", m)
		}
	})

	t.Run("a developer and an admin get the same pricing", func(t *testing.T) {
		if d, a := pricingOf(t, devList), pricingOf(t, admList); !reflect.DeepEqual(d, a) {
			t.Errorf("developer pricing %v, admin pricing %v", d, a)
		}
	})

	t.Run("after the top band is lowered and before the next poll", func(t *testing.T) {
		if err := e.st.PutSetting(ctx, "bands", `{"base":0.5,"steps":[{"from":0.5,"multiplier":1.25}]}`, nil, e.clk.Now()); err != nil {
			t.Fatal(err)
		}
		for who, s := range sessions {
			l := e.getOK("/api/v1/setups", s)
			p := pricingOf(t, l)
			if p["multiplier"] != 2.5 || p["peak_multiplier"] != 1.25 {
				t.Errorf("%s: pricing %v, want multiplier 2.5 (the sample's) and peak 1.25 (the new bands')", who, p)
			}
			costs := wantCostsFromPricing(t, who+", after the band edit", l)
			if c := costs["lab-02"]; !(c[0] > c[1]) {
				t.Errorf("%s: lab-02 costs %v, want now above at_peak between the band edit and the next poll", who, c)
			}
		}
	})
}
