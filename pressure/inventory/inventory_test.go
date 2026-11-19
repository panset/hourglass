// SPDX-License-Identifier: MIT

package inventory

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

func newSource() hourglass.PressureSource {
	s, err := New(nil)
	if err != nil {
		panic(err)
	}
	return s
}

func TestEXT5_Conformance(t *testing.T) {
	snaps := []hourglass.Snapshot{
		hourglasstest.NewSnapshot().Setup("a").Setup("b", hourglasstest.Weight(3)).Hold("b", 1).Build(),
		hourglasstest.NewSnapshot().Setup("a", hourglasstest.Weight(0)).Hold("a", 1).Build(),
	}
	hourglasstest.TestPressureSource(t, newSource, snaps...)
}

func TestEXT3_Pressure(t *testing.T) {
	rel := hourglasstest.Released(hourglasstest.DefaultTime)
	cases := []struct {
		name string
		snap hourglass.Snapshot
		want float64
	}{
		{"no setups", hourglasstest.NewSnapshot().Build(), 0},
		{"no holds", hourglasstest.NewSnapshot().Setup("a").Build(), 0},
		{"all weight zero", hourglasstest.NewSnapshot().Setup("a", hourglasstest.Weight(0)).Hold("a", 1).Build(), 0},
		{"one of four", hourglasstest.NewSnapshot().Setup("a").Setup("b").Setup("c").Setup("d").Hold("a", 1).Build(), 0.25},
		{"weighted", hourglasstest.NewSnapshot().Setup("a").Setup("b", hourglasstest.Weight(3)).Hold("b", 1).Build(), 0.75},
		{"all held", hourglasstest.NewSnapshot().Setup("a").Setup("b").Hold("a", 1).Hold("b", 2).Build(), 1},
		{"released hold ignored", hourglasstest.NewSnapshot().Setup("a").Setup("b").Hold("a", 1, rel).Build(), 0},
		{"two holds one setup counted once", hourglasstest.NewSnapshot().Setup("a").Setup("b").Hold("a", 1).Hold("a", 2).Build(), 0.5},
		{"hold on setup not in snapshot", hourglasstest.NewSnapshot().Setup("a").Hold("gone", 1).Build(), 0},
		{"allocations ignored", hourglasstest.NewSnapshot().Setup("a").Setup("b").ProviderConfigured(true).Allocation("a").Allocation("b").Build(), 0},
		{"no provider", hourglasstest.NewSnapshot().Setup("a").Setup("b").ProviderConfigured(false).Hold("a", 1).Build(), 0.5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := newSource().Pressure(context.Background(), c.snap)
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(got-c.want) > 1e-9 || got < 0 || got > 1 {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestEXT1_Factory(t *testing.T) {
	cases := []struct {
		name string
		cfg  string
		nilc bool
		ok   bool
	}{
		{"nil config", "", true, true},
		{"empty config", "", false, true},
		{"empty object", "{}", false, true},
		{"null", "null", false, true},
		{"unknown field", `{"x":1}`, false, false},
		{"malformed", `{`, false, false},
		{"not an object", `[1]`, false, false},
		{"trailing data", `{} {}`, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var raw json.RawMessage
			if !c.nilc {
				raw = json.RawMessage(c.cfg)
			}
			s, err := New(raw)
			if (err == nil) != c.ok {
				t.Fatalf("err=%v want ok=%v", err, c.ok)
			}
			if c.ok && s.Name() != "inventory" {
				t.Fatalf("name %q", s.Name())
			}
		})
	}
}

func TestEXT1_Registered(t *testing.T) {
	s, err := hourglass.NewPressureSource("inventory", nil)
	if err != nil || s == nil {
		t.Fatalf("not registered: %v", err)
	}
}
