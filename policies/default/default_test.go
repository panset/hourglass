// SPDX-License-Identifier: MIT

package defaultpolicy

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

var t0 = hourglasstest.DefaultTime

func mk(t *testing.T, cfg string) hourglass.MeteringPolicy {
	t.Helper()
	var raw json.RawMessage
	if cfg != "" {
		raw = json.RawMessage(cfg)
	}
	p, err := New(raw)
	if err != nil {
		t.Fatalf("New(%q): %v", cfg, err)
	}
	return p
}

func TestEXT5_Conformance(t *testing.T) {
	s1 := hourglasstest.NewSnapshot().At(t0).Setup("a").Hold("a", 1).Allocation("a").Build()
	s2 := hourglasstest.NewSnapshot().At(t0.Add(3*time.Hour)).Setup("a").Hold("a", 1, hourglasstest.Acquired(t0)).Build()
	s3 := hourglasstest.NewSnapshot().At(t0).Setup("a").Hold("a", 1, hourglasstest.Metering(t0)).Build()
	u := hourglass.User{ID: 1}
	hourglasstest.TestMeteringPolicy(t, func() hourglass.MeteringPolicy { return mk(t, "") },
		hourglasstest.NewPolicyCase("allocation", s1, 0, u),
		hourglasstest.NewPolicyCase("elapsed", s2, 0, u),
		hourglasstest.NewPolicyCase("started", s3, 0, u))
}

func TestMET7_Decide(t *testing.T) {
	acq := t0
	cases := []struct {
		name     string
		cfg      string
		age      time.Duration
		alloc    bool
		provider bool
		started  bool
		want     bool
		reason   string
	}{
		{"waiting default", "", time.Hour, false, true, false, false, "waiting for the provider to report the resource or 2h since acquire"},
		{"allocation trigger", "", time.Minute, true, true, false, true, "resource reported by provider"},
		{"time trigger exact", "", 2 * time.Hour, false, true, false, true, "2h since acquire"},
		{"time trigger just before", "", 2*time.Hour - time.Second, false, true, false, false, ""},
		{"already started continues", `{"start_after":"off","start_on_allocation":false}`, 0, false, true, true, true, "metering already started"},
		{"already started even if allocation gone", "", time.Minute, false, true, true, true, "metering already started"},
		{"start_after 0", `{"start_after":"0"}`, 0, false, true, false, true, "metering from acquire"},
		{"start_after 0s", `{"start_after":"0s"}`, 0, false, true, false, true, "metering from acquire"},
		{"start_after 90m", `{"start_after":"90m"}`, 90 * time.Minute, false, true, false, true, "1h30m since acquire"},
		{"time off, no alloc", `{"start_after":"off"}`, 100 * time.Hour, false, true, false, false, "waiting for the provider to report the resource"},
		{"time off, alloc", `{"start_after":"off"}`, time.Hour, true, true, false, true, "resource reported by provider"},
		{"alloc off, alloc present", `{"start_on_allocation":false}`, time.Hour, true, true, false, false, "waiting for 2h since acquire"},
		{"alloc off, time elapsed", `{"start_on_allocation":false}`, 3 * time.Hour, true, true, false, true, "2h since acquire"},
		{"both off", `{"start_after":"off","start_on_allocation":false}`, 100 * time.Hour, true, true, false, false, "not metering: no trigger is enabled"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := hourglasstest.NewSnapshot().At(acq.Add(c.age)).ProviderConfigured(c.provider).Setup("a")
			if c.started {
				b.Hold("a", 1, hourglasstest.Acquired(acq), hourglasstest.Metering(acq))
			} else {
				b.Hold("a", 1, hourglasstest.Acquired(acq))
			}
			if c.alloc {
				b.Allocation("a")
			}
			s := b.Build()
			d, err := mk(t, c.cfg).Decide(context.Background(), hourglasstest.NewHoldState(s, 0, hourglass.User{ID: 1}), s)
			if err != nil {
				t.Fatal(err)
			}
			if d.Meter != c.want || d.Weight != 0 {
				t.Fatalf("got %+v want meter=%v weight 0", d, c.want)
			}
			if c.reason != "" && d.Reason != c.reason {
				t.Fatalf("reason %q want %q", d.Reason, c.reason)
			}
			if d.Reason == "" {
				t.Fatal("empty reason")
			}
		})
	}
}

func TestEXT4_NoProviderOnlyTimeTrigger(t *testing.T) {
	p := mk(t, "")
	for _, c := range []struct {
		age  time.Duration
		want bool
	}{{time.Minute, false}, {119 * time.Minute, false}, {2 * time.Hour, true}, {24 * time.Hour, true}} {
		s := hourglasstest.NewSnapshot().At(t0.Add(c.age)).ProviderConfigured(false).Setup("a").Hold("a", 1, hourglasstest.Acquired(t0)).Build()
		d, err := p.Decide(context.Background(), hourglasstest.NewHoldState(s, 0, hourglass.User{ID: 1}), s)
		if err != nil || d.Meter != c.want {
			t.Fatalf("age %v: %+v %v want %v", c.age, d, err, c.want)
		}
	}
}

func TestMET7_Factory(t *testing.T) {
	good := []string{"", "{}", "null", `{"start_after":"2h"}`, `{"start_after":"off"}`, `{"start_after":"0"}`,
		`{"start_on_allocation":false}`, `{"start_after":"off","start_on_allocation":false}`}
	for _, g := range good {
		if _, err := New(json.RawMessage(g)); err != nil {
			t.Errorf("%q rejected: %v", g, err)
		}
	}
	if _, err := New(nil); err != nil {
		t.Errorf("nil rejected: %v", err)
	}
	bad := []string{`{"start_after":"-1h"}`, `{"start_after":"soon"}`, `{"start_after":""}`, `{"start_after":2}`,
		`{"start_on_allocation":"yes"}`, `{"bogus":1}`, `{`, `[]`, `{} {}`}
	for _, b := range bad {
		if _, err := New(json.RawMessage(b)); err == nil {
			t.Errorf("%q accepted", b)
		}
	}
}

func TestMET7_Defaults(t *testing.T) {
	p := mk(t, "{}").(*policy)
	if p.startAfter != 2*time.Hour || !p.timeTrigger || !p.onAllocation {
		t.Fatalf("defaults wrong: %+v", p)
	}
}

func TestEXT1_Registered(t *testing.T) {
	if _, err := hourglass.NewMeteringPolicy("default", nil); err != nil {
		t.Fatal(err)
	}
}

func TestMET7_AllocationEitherSourceSuffices(t *testing.T) {
	p := mk(t, "")
	ctx := context.Background()
	base := func(withAlloc bool) hourglass.Snapshot {
		b := hourglasstest.NewSnapshot().At(t0.Add(time.Minute)).Setup("a").Hold("a", 1, hourglasstest.Acquired(t0))
		if withAlloc {
			b = b.Allocation("a")
		}
		return b.Build()
	}
	cases := []struct {
		name      string
		snapAlloc bool
		holdAlloc bool
		want      bool
	}{
		{"neither", false, false, false},
		{"snapshot only", true, false, true},
		{"hold state only", false, true, true},
		{"both", true, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := base(c.snapAlloc)
			hs := hourglasstest.NewHoldState(base(false), 0, hourglass.User{ID: 1})
			if c.snapAlloc {
				hs = hourglasstest.NewHoldState(s, 0, hourglass.User{ID: 1})
			}
			hs.Allocation = nil
			if c.holdAlloc {
				hs.Allocation = &hourglass.Allocation{ResourceName: "a"}
			}
			d, err := p.Decide(ctx, hs, s)
			if err != nil || d.Meter != c.want {
				t.Fatalf("%+v %v want meter=%v", d, err, c.want)
			}
		})
	}
}

func TestMET7_ClockSkewReasonIsSensible(t *testing.T) {
	p := mk(t, `{"start_after":"0"}`)
	s := hourglasstest.NewSnapshot().At(t0.Add(-time.Minute)).ProviderConfigured(false).Setup("a").Hold("a", 1, hourglasstest.Acquired(t0)).Build()
	d, err := p.Decide(context.Background(), hourglasstest.NewHoldState(s, 0, hourglass.User{ID: 1}), s)
	if err != nil || d.Meter || strings.Contains(d.Reason, "0s") {
		t.Fatalf("%+v %v", d, err)
	}
}
