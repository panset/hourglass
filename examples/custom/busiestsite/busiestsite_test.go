// SPDX-License-Identifier: MIT

package busiestsite

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

func mustNew(t *testing.T, raw string) hourglass.PressureSource {
	t.Helper()
	s, err := New(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func twoSites() hourglass.Snapshot {
	return hourglasstest.NewSnapshot().
		Setup("a1", hourglasstest.Label("site", "east"), hourglasstest.Weight(1)).
		Setup("a2", hourglasstest.Label("site", "east"), hourglasstest.Weight(1)).
		Setup("b1", hourglasstest.Label("site", "west"), hourglasstest.Weight(1)).
		Setup("b2", hourglasstest.Label("site", "west"), hourglasstest.Weight(3)).
		Hold("a1", 1).Hold("a2", 2).Hold("b2", 3).
		Build()
}

func TestEXT5_BusiestSiteConformance(t *testing.T) {
	hourglasstest.TestPressureSource(t, func() hourglass.PressureSource { return mustNew(t, "") }, twoSites())
}

func TestEXT5_BusiestSiteReportsTheWorstSite(t *testing.T) {
	// east: 2/2 held = 1.0; west: 3/4 held = 0.75. The worst site wins.
	got, err := mustNew(t, "").Pressure(context.Background(), twoSites())
	if err != nil || got != 1 {
		t.Fatalf("got %v, %v; want 1", got, err)
	}
}

func TestEXT5_BusiestSiteIsNotTheLabWideAverage(t *testing.T) {
	s := hourglasstest.NewSnapshot().
		Setup("a1", hourglasstest.Label("site", "east")).
		Setup("b1", hourglasstest.Label("site", "west")).
		Setup("b2", hourglasstest.Label("site", "west")).
		Setup("b3", hourglasstest.Label("site", "west")).
		Hold("a1", 1).Build() // lab-wide 1/4, but east is full
	got, _ := mustNew(t, "").Pressure(context.Background(), s)
	if got != 1 {
		t.Fatalf("got %v, want 1", got)
	}
}

func TestEXT5_BusiestSiteIgnoresUnlabelledAndReleased(t *testing.T) {
	released := hourglasstest.NewSnapshot().
		Setup("a1", hourglasstest.Label("site", "east")).
		Setup("loose").Hold("loose", 1).
		Hold("a1", 1, hourglasstest.Released(hourglasstest.DefaultTime)).Build()
	got, err := mustNew(t, "").Pressure(context.Background(), released)
	if err != nil || got != 0 {
		t.Fatalf("got %v, %v; want 0", got, err)
	}
}

func TestEXT5_BusiestSiteConfigurableLabel(t *testing.T) {
	s := hourglasstest.NewSnapshot().
		Setup("a", hourglasstest.Label("room", "r1"), hourglasstest.Weight(1)).
		Setup("b", hourglasstest.Label("room", "r1"), hourglasstest.Weight(1)).
		Hold("a", 1).Build()
	got, _ := mustNew(t, `{"label":"room"}`).Pressure(context.Background(), s)
	if math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("got %v, want 0.5", got)
	}
}

func TestEXT5_BusiestSiteRejectsBadConfig(t *testing.T) {
	for _, raw := range []string{`{"label":""}`, `{"nope":1}`} {
		if _, err := New(json.RawMessage(raw)); err == nil {
			t.Errorf("config %q: want an error", raw)
		}
	}
}
