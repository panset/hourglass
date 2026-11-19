// SPDX-License-Identifier: MIT

// Package defaultpolicy is the bundled metering policy. It starts metering
// when the provider reports the hold's resource or when a time has passed
// since acquire, whichever is first, and keeps metering until release.
package defaultpolicy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/panset/hourglass/pkg/hourglass"
)

// Name is the registration name.
const Name = "default"

func init() {
	hourglass.RegisterMeteringPolicy(Name, New)
}

type policy struct {
	startAfter   time.Duration
	timeTrigger  bool
	onAllocation bool
}

// New builds the policy from {"start_after": "2h", "start_on_allocation": true}.
// Missing fields take those defaults.
func New(config json.RawMessage) (hourglass.MeteringPolicy, error) {
	var cfg struct {
		StartAfter        *string `json:"start_after"`
		StartOnAllocation *bool   `json:"start_on_allocation"`
	}
	if trimmed := bytes.TrimSpace(config); len(trimmed) > 0 {
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("default policy: invalid config: %w", err)
		}
		if dec.More() {
			return nil, fmt.Errorf("default policy: invalid config: trailing data")
		}
	}
	p := &policy{startAfter: 2 * time.Hour, timeTrigger: true, onAllocation: true}
	if cfg.StartAfter != nil {
		v := strings.TrimSpace(*cfg.StartAfter)
		switch {
		case v == "off":
			p.timeTrigger = false
		case v == "0":
			p.startAfter = 0
		default:
			d, err := time.ParseDuration(v)
			if err != nil {
				return nil, fmt.Errorf(`default policy: start_after %q: want a duration like "2h", "0" or "off": %w`, *cfg.StartAfter, err)
			}
			if d < 0 {
				return nil, fmt.Errorf("default policy: start_after %q must not be negative", *cfg.StartAfter)
			}
			p.startAfter = d
		}
	}
	if cfg.StartOnAllocation != nil {
		p.onAllocation = *cfg.StartOnAllocation
	}
	return p, nil
}

func (*policy) Name() string { return Name }

// Decide implements hourglass.MeteringPolicy. Weight is always 0 so the
// rate rules apply.
func (p *policy) Decide(_ context.Context, h hourglass.HoldState, s hourglass.Snapshot) (hourglass.Decision, error) {
	if h.Hold.MeteringStartedAt != nil {
		return hourglass.Decision{Meter: true, Reason: "metering already started"}, nil
	}
	if p.onAllocation {
		if _, ok := s.AllocationFor(h.Hold.SetupName); ok {
			return hourglass.Decision{Meter: true, Reason: "resource reported by provider"}, nil
		}
	}
	if p.timeTrigger && s.Time.Sub(h.Hold.AcquiredAt) >= p.startAfter {
		if p.startAfter == 0 {
			return hourglass.Decision{Meter: true, Reason: "metering from acquire"}, nil
		}
		return hourglass.Decision{Meter: true, Reason: compact(p.startAfter) + " since acquire"}, nil
	}
	var waits []string
	if p.onAllocation {
		waits = append(waits, "the provider to report the resource")
	}
	if p.timeTrigger {
		waits = append(waits, compact(p.startAfter)+" since acquire")
	}
	reason := "not metering: no trigger is enabled"
	if len(waits) > 0 {
		reason = "waiting for " + strings.Join(waits, " or ")
	}
	return hourglass.Decision{Reason: reason}, nil
}

// compact formats 2h0m0s as 2h and 1h30m0s as 1h30m.
func compact(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}
