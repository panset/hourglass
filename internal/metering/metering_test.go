// SPDX-License-Identifier: MIT

package metering

import (
	"math"
	"testing"
	"time"
)

var cases int

func TestMain(m *testing.M) {
	c := m.Run()
	println("metering table cases executed:", cases)
	if c != 0 {
		panic("fail")
	}
}

func TestMET3_DefaultBandsMultiplier(t *testing.T) {
	b := DefaultBands()
	tests := []struct {
		p, want float64
	}{
		{0, 0}, {0.4999, 0}, {0.50, 1}, {0.7499, 1}, {0.75, 2}, {0.8999, 2}, {0.90, 3}, {1.0, 3},
	}
	for _, tc := range tests {
		cases++
		if got := b.Multiplier(tc.p); got != tc.want {
			t.Errorf("Multiplier(%v)=%v want %v", tc.p, got, tc.want)
		}
	}
}

func TestMET3_BaseAndMax(t *testing.T) {
	tests := []struct {
		name    string
		b       Bands
		low     float64
		wantMax float64
	}{
		{"default", DefaultBands(), 0, 3},
		{"base highest", Bands{Base: 5, Steps: []Step{{0.5, 1}}}, 0.1, 5},
		{"no steps", Bands{Base: 2}, 0.9, 2},
	}
	for _, tc := range tests {
		cases++
		if got := tc.b.Max(); got != tc.wantMax {
			t.Errorf("%s Max=%v want %v", tc.name, got, tc.wantMax)
		}
		if tc.name == "base highest" && tc.b.Multiplier(tc.low) != 5 {
			t.Errorf("base not used below first step")
		}
	}
}

func TestMET3_BandsValidate(t *testing.T) {
	tests := []struct {
		name string
		b    Bands
		ok   bool
	}{
		{"default", DefaultBands(), true},
		{"no steps", Bands{Base: 1}, true},
		{"from 1 ok", Bands{Steps: []Step{{1, 2}}}, true},
		{"from 0", Bands{Steps: []Step{{0, 1}}}, false},
		{"from >1", Bands{Steps: []Step{{1.01, 1}}}, false},
		{"from NaN", Bands{Steps: []Step{{math.NaN(), 1}}}, false},
		{"equal froms", Bands{Steps: []Step{{0.5, 1}, {0.5, 2}}}, false},
		{"descending", Bands{Steps: []Step{{0.8, 1}, {0.5, 2}}}, false},
		{"neg base", Bands{Base: -1}, false},
		{"NaN base", Bands{Base: math.NaN()}, false},
		{"inf base", Bands{Base: math.Inf(1)}, false},
		{"neg mult", Bands{Steps: []Step{{0.5, -1}}}, false},
		{"inf mult", Bands{Steps: []Step{{0.5, math.Inf(1)}}}, false},
		{"NaN mult", Bands{Steps: []Step{{0.5, math.NaN()}}}, false},
	}
	for _, tc := range tests {
		cases++
		if err := tc.b.Validate(); (err == nil) != tc.ok {
			t.Errorf("%s: err=%v wantOK=%v", tc.name, err, tc.ok)
		}
	}
}

func TestMET3_ValidPressure(t *testing.T) {
	tests := []struct {
		p  float64
		ok bool
	}{
		{0, true}, {1, true}, {0.5, true}, {-0.01, false}, {1.01, false},
		{math.NaN(), false}, {math.Inf(1), false}, {math.Inf(-1), false},
	}
	for _, tc := range tests {
		cases++
		if got := ValidPressure(tc.p); got != tc.ok {
			t.Errorf("ValidPressure(%v)=%v", tc.p, got)
		}
	}
}

func TestMET3_Smooth(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	w := DefaultWindow
	s := func(ago time.Duration, v float64) Sample { return Sample{now.Add(-ago), v} }
	tests := []struct {
		name string
		in   []Sample
		want float64
		ok   bool
	}{
		{"empty", nil, 0, false},
		{"single", []Sample{s(time.Minute, 0.4)}, 0.4, true},
		{"mean", []Sample{s(0, 0.2), s(5*time.Minute, 0.4), s(10*time.Minute, 0.6)}, 0.4, true},
		{"window start exclusive", []Sample{s(w, 1.0), s(time.Minute, 0.2)}, 0.2, true},
		{"only at window start", []Sample{s(w, 1.0)}, 0, false},
		{"now inclusive", []Sample{s(0, 0.8)}, 0.8, true},
		{"future excluded", []Sample{s(-time.Second, 1.0), s(time.Minute, 0.2)}, 0.2, true},
		{"order irrelevant", []Sample{s(10*time.Minute, 0.6), s(0, 0.2), s(5*time.Minute, 0.4)}, 0.4, true},
	}
	for _, tc := range tests {
		cases++
		got, ok := Smooth(tc.in, now, w)
		if ok != tc.ok || math.Abs(got-tc.want) > 1e-12 {
			t.Errorf("%s: got %v,%v want %v,%v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestMET3_DebitMT(t *testing.T) {
	tests := []struct {
		name    string
		w, m    float64
		elapsed time.Duration
		want    int64
	}{
		{"5 min 1x", 1, 1, 5 * time.Minute, 83},
		{"15h 0x", 1, 0, 15 * time.Hour, 0},
		{"15h 1x", 1, 1, 15 * time.Hour, 15000},
		{"15h 3x", 1, 3, 15 * time.Hour, 45000},
		{"5 min 2x weight 1.5", 1.5, 2, 5 * time.Minute, 250},
		{"zero weight", 0, 1, time.Hour, 0},
		{"neg weight", -1, 1, time.Hour, 0},
		{"neg mult", 1, -1, time.Hour, 0},
		{"zero elapsed", 1, 1, 0, 0},
		{"neg elapsed", 1, 1, -time.Hour, 0},
		{"NaN weight", math.NaN(), 1, time.Hour, 0},
		{"inf weight", math.Inf(1), 1, time.Hour, 0},
		{"overflow clamps", 1e300, 1e10, time.Hour, math.MaxInt64},
	}
	for _, tc := range tests {
		cases++
		if got := DebitMT(tc.w, tc.m, tc.elapsed); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}

func TestMET3_DebitRounding(t *testing.T) {
	// 0.0036 weight-hours = 3.6 mt; 3.6 s at weight 1, 1x = 1 mt exactly; 1.8 s = 0.5 mt rounds up to 1.
	tests := []struct {
		elapsed time.Duration
		want    int64
	}{
		{1800 * time.Millisecond, 1}, // 0.5 mt, half away from zero
		{1799 * time.Millisecond, 0}, // just below half
		{5400 * time.Millisecond, 2}, // 1.5 mt
		{3600 * time.Millisecond, 1},
	}
	for _, tc := range tests {
		cases++
		if got := DebitMT(1, 1, tc.elapsed); got != tc.want {
			t.Errorf("elapsed %v got %d want %d", tc.elapsed, got, tc.want)
		}
	}
}

func TestMET4_ChargeableElapsed(t *testing.T) {
	poll := 5 * time.Minute
	prev := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		prev   time.Time
		now    time.Time
		want   time.Duration
		outage bool
	}{
		{"no earlier poll", time.Time{}, prev, 0, false},
		{"normal", prev, prev.Add(poll), poll, false},
		{"exactly 2x charged", prev, prev.Add(2 * poll), 2 * poll, false},
		{"just over 2x outage", prev, prev.Add(2*poll + time.Nanosecond), 0, true},
		{"long gap outage", prev, prev.Add(3 * time.Hour), 0, true},
		{"now equals prev", prev, prev, 0, false},
		{"now before prev", prev, prev.Add(-time.Minute), 0, false},
		{"short gap", prev, prev.Add(time.Second), time.Second, false},
	}
	for _, tc := range tests {
		cases++
		got, out := ChargeableElapsed(tc.prev, tc.now, poll)
		if got != tc.want || out != tc.outage {
			t.Errorf("%s: got %v,%v want %v,%v", tc.name, got, out, tc.want, tc.outage)
		}
	}
}

func TestMET3_HourlyCostMT(t *testing.T) {
	tests := []struct {
		w, m float64
		want int64
	}{
		{1, 0, 0}, {1, 1, 1000}, {1, 3, 3000}, {2.5, 2, 5000}, {0, 3, 0}, {-1, 1, 0},
	}
	for _, tc := range tests {
		cases++
		if got := HourlyCostMT(tc.w, tc.m); got != tc.want {
			t.Errorf("HourlyCostMT(%v,%v)=%d want %d", tc.w, tc.m, got, tc.want)
		}
	}
}

func TestUI_ProjectedRunOut(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		bal    int64
		burn   int64
		want   time.Time
		wantOK bool
	}{
		{"no burn", 1000, 0, time.Time{}, false},
		{"negative burn", 1000, -5, time.Time{}, false},
		{"no burn and no balance", 0, 0, now, true},
		{"no burn and negative balance", -10, 0, now, true},
		{"zero balance", 0, 24000, now, true},
		{"negative balance", -10, 24000, now, true},
		{"10 hours at 1000 mt/h", 10000, 24000, now.Add(10 * time.Hour), true},
		{"half hour", 500, 24000, now.Add(30 * time.Minute), true},
		{"absurdly far", math.MaxInt64, 1, time.Time{}, false},
	}
	for _, tc := range tests {
		cases++
		got, ok := ProjectedRunOut(tc.bal, tc.burn, now)
		if ok != tc.wantOK || !got.Equal(tc.want) {
			t.Errorf("%s: got %v,%v want %v,%v", tc.name, got, ok, tc.want, tc.wantOK)
		}
	}
}
