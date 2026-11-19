// SPDX-License-Identifier: MIT

// Package businesshours is a MeteringPolicy that charges only during
// working hours, and charges GPU rigs at a higher weight.
//
// It shows the two things a policy can say beyond "yes" or "no":
//
//   - Decision.Reason is shown to the user on the dashboard, so it should
//     explain the verdict in plain words ("outside business hours ...").
//   - Decision.Weight overrides the weight resolved from the rate rules.
//     Zero means "no override", so a policy that has no opinion about the
//     weight leaves it at 0.
//
// Config (all optional):
//
//	metering:
//	  type: business-hours
//	  config:
//	    timezone: Europe/Berlin   # default UTC
//	    start: "09:00"            # default 09:00
//	    end: "18:00"              # default 18:00, must be after start
//	    days: [mon, tue, wed, thu, fri]
//	    gpu_weight: 2             # weight for setups labelled gpu=true
package businesshours

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/panset/hourglass/pkg/hourglass"
)

// Name is the name the config file uses to select this policy.
const Name = "business-hours"

func init() {
	hourglass.RegisterMeteringPolicy(Name, New)
}

type config struct {
	Timezone  string   `json:"timezone"`
	Start     string   `json:"start"`
	End       string   `json:"end"`
	Days      []string `json:"days"`
	GPUWeight *float64 `json:"gpu_weight"`
}

type policy struct {
	loc        *time.Location
	start, end int // minutes after midnight
	days       map[time.Weekday]bool
	gpuWeight  float64
	window     string // for reasons, e.g. "09:00-18:00 UTC"
}

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// New is the factory. Every setting has a default, so an empty block works.
func New(raw json.RawMessage) (hourglass.MeteringPolicy, error) {
	var c config
	if t := bytes.TrimSpace(raw); len(t) > 0 && string(t) != "null" {
		dec := json.NewDecoder(bytes.NewReader(t))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&c); err != nil {
			return nil, fmt.Errorf("business-hours: invalid config: %w", err)
		}
	}
	p := &policy{days: map[time.Weekday]bool{}, gpuWeight: 2}

	tz := orDefault(c.Timezone, "UTC")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("business-hours: timezone %q: %w", tz, err)
	}
	p.loc = loc

	startS, endS := orDefault(c.Start, "09:00"), orDefault(c.End, "18:00")
	if p.start, err = parseClock(startS); err != nil {
		return nil, fmt.Errorf("business-hours: start: %w", err)
	}
	if p.end, err = parseClock(endS); err != nil {
		return nil, fmt.Errorf("business-hours: end: %w", err)
	}
	if p.start >= p.end {
		return nil, fmt.Errorf("business-hours: start %s must be before end %s", startS, endS)
	}

	days := c.Days
	if len(days) == 0 {
		days = []string{"mon", "tue", "wed", "thu", "fri"}
	}
	for _, d := range days {
		wd, ok := weekdays[strings.ToLower(d)]
		if !ok {
			return nil, fmt.Errorf("business-hours: day %q: want mon, tue, wed, thu, fri, sat or sun", d)
		}
		p.days[wd] = true
	}

	if c.GPUWeight != nil {
		w := *c.GPUWeight
		if math.IsNaN(w) || math.IsInf(w, 0) || w < 0 {
			return nil, fmt.Errorf("business-hours: gpu_weight %v must be a finite number >= 0", w)
		}
		p.gpuWeight = w
	}
	p.window = fmt.Sprintf("%s-%s %s", startS, endS, tz)
	return p, nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func parseClock(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("%q: want HH:MM", s)
	}
	return t.Hour()*60 + t.Minute(), nil
}

func (*policy) Name() string { return Name }

// Decide meters when the poll time falls inside the configured window. The
// poll time (Snapshot.Time) is used, not the wall clock, so the decision is
// the same for every hold in one poll and tests can pin it.
//
// Inside the window, a setup labelled gpu=true is charged at gpu_weight; the
// weight of every other setup is left at 0 so the rate rules apply.
func (p *policy) Decide(ctx context.Context, h hourglass.HoldState, s hourglass.Snapshot) (hourglass.Decision, error) {
	if err := ctx.Err(); err != nil {
		return hourglass.Decision{}, err
	}
	now := s.Time.In(p.loc)
	minutes := now.Hour()*60 + now.Minute()
	if !p.days[now.Weekday()] || minutes < p.start || minutes >= p.end {
		return hourglass.Decision{Reason: "outside business hours (" + p.window + ")"}, nil
	}
	if h.Setup.Labels["gpu"] == "true" {
		return hourglass.Decision{
			Meter:  true,
			Weight: p.gpuWeight,
			Reason: fmt.Sprintf("business hours, GPU rig at weight %g", p.gpuWeight),
		}, nil
	}
	return hourglass.Decision{Meter: true, Reason: "business hours"}, nil
}
