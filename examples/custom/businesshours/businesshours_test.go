// SPDX-License-Identifier: MIT

package businesshours

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

func mustNew(t *testing.T, raw string) hourglass.MeteringPolicy {
	t.Helper()
	p, err := New(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// decide runs the policy for a hold on a setup with the given labels at at.
func decide(t *testing.T, p hourglass.MeteringPolicy, at time.Time, labels ...string) hourglass.Decision {
	t.Helper()
	var opts []hourglasstest.SetupOption
	for i := 0; i+1 < len(labels); i += 2 {
		opts = append(opts, hourglasstest.Label(labels[i], labels[i+1]))
	}
	snap := hourglasstest.NewSnapshot().At(at).Setup("rig", opts...).Hold("rig", 1).Build()
	d, err := p.Decide(context.Background(), hourglasstest.NewHoldState(snap, 0, hourglass.User{ID: 1}), snap)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// 2030-01-01 is a Tuesday; 2030-01-05 a Saturday.
var (
	tueNoon  = time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	tueNight = time.Date(2030, 1, 1, 22, 0, 0, 0, time.UTC)
	satNoon  = time.Date(2030, 1, 5, 12, 0, 0, 0, time.UTC)
)

func TestEXT5_BusinessHoursConformance(t *testing.T) {
	mk := func(at time.Time, labels ...string) hourglasstest.PolicyCase {
		var opts []hourglasstest.SetupOption
		for i := 0; i+1 < len(labels); i += 2 {
			opts = append(opts, hourglasstest.Label(labels[i], labels[i+1]))
		}
		s := hourglasstest.NewSnapshot().At(at).Setup("rig", opts...).Hold("rig", 1).Build()
		return hourglasstest.NewPolicyCase(at.Format(time.RFC3339), s, 0, hourglass.User{ID: 1})
	}
	hourglasstest.TestMeteringPolicy(t, func() hourglass.MeteringPolicy { return mustNew(t, "") },
		mk(tueNoon), mk(tueNoon, "gpu", "true"), mk(tueNight), mk(satNoon))
}

func TestEXT5_BusinessHoursMetersOnlyInsideTheWindow(t *testing.T) {
	p := mustNew(t, "")
	if d := decide(t, p, tueNoon); !d.Meter || d.Weight != 0 || d.Reason == "" {
		t.Errorf("tuesday noon: %+v", d)
	}
	for name, at := range map[string]time.Time{"night": tueNight, "weekend": satNoon,
		"before start": time.Date(2030, 1, 1, 8, 59, 0, 0, time.UTC),
		"at end":       time.Date(2030, 1, 1, 18, 0, 0, 0, time.UTC)} {
		if d := decide(t, p, at); d.Meter || d.Reason == "" {
			t.Errorf("%s: %+v", name, d)
		}
	}
}

func TestEXT5_BusinessHoursGPUWeightOverride(t *testing.T) {
	p := mustNew(t, `{"gpu_weight": 3}`)
	if d := decide(t, p, tueNoon, "gpu", "true"); !d.Meter || d.Weight != 3 {
		t.Errorf("gpu=true: %+v", d)
	}
	for _, v := range []string{"false", "yes"} {
		if d := decide(t, p, tueNoon, "gpu", v); d.Weight != 0 {
			t.Errorf("gpu=%s: weight %v, want 0 (rate rules apply)", v, d.Weight)
		}
	}
	// Outside hours nothing is charged, GPU or not.
	if d := decide(t, p, tueNight, "gpu", "true"); d.Meter {
		t.Errorf("night: %+v", d)
	}
}

func TestEXT5_BusinessHoursUsesConfiguredTimezone(t *testing.T) {
	p := mustNew(t, `{"timezone":"Asia/Tokyo"}`)
	// 12:00 UTC is 21:00 in Tokyo: outside 09:00-18:00 there.
	if d := decide(t, p, tueNoon); d.Meter {
		t.Errorf("tokyo evening: %+v", d)
	}
	// 02:00 UTC is 11:00 in Tokyo.
	if d := decide(t, p, time.Date(2030, 1, 1, 2, 0, 0, 0, time.UTC)); !d.Meter {
		t.Errorf("tokyo morning: %+v", d)
	}
}

func TestEXT5_BusinessHoursConfigurableDays(t *testing.T) {
	if d := decide(t, mustNew(t, `{"days":["sat"]}`), satNoon); !d.Meter {
		t.Errorf("saturday allowed: %+v", d)
	}
}

func TestEXT5_BusinessHoursRejectsBadConfig(t *testing.T) {
	for _, raw := range []string{
		`{"timezone":"Mars/Base"}`, `{"start":"9am"}`, `{"start":"18:00","end":"09:00"}`,
		`{"days":["funday"]}`, `{"gpu_weight":-1}`, `{"surprise":1}`,
	} {
		if _, err := New(json.RawMessage(raw)); err == nil {
			t.Errorf("config %s: want an error", raw)
		}
	}
}
