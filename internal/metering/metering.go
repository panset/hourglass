// SPDX-License-Identifier: MIT

// Package metering holds the pure arithmetic of the burn rate: pressure bands,
// smoothing, milli-token debits, chargeable elapsed time and run-out projection.
package metering

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// MilliPerToken is the number of milli-tokens in one token.
const MilliPerToken = 1000

// DefaultWindow is the pressure smoothing window.
const DefaultWindow = 15 * time.Minute

// maxProjectionHours bounds a run-out projection (100 years); beyond it there is no projection.
const maxProjectionHours = 24 * 365 * 100

// Step means "pressure >= From gets Multiplier".
type Step struct {
	From       float64
	Multiplier float64
}

// Bands maps pressure to a multiplier. Below the first step the multiplier is Base.
type Bands struct {
	Base  float64
	Steps []Step
}

// DefaultBands returns the spec's default bands (0, then 0.50 to 1x, 0.75 to 2x, 0.90 to 3x).
func DefaultBands() Bands {
	return Bands{Base: 0, Steps: []Step{{0.50, 1}, {0.75, 2}, {0.90, 3}}}
}

func finiteNonNeg(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 }

// Validate checks steps are strictly ascending with From in (0,1], and that
// base and multipliers are finite and non-negative.
func (b Bands) Validate() error {
	if !finiteNonNeg(b.Base) {
		return fmt.Errorf("base multiplier %v must be finite and non-negative", b.Base)
	}
	prev := 0.0
	for i, s := range b.Steps {
		if math.IsNaN(s.From) || !(s.From > 0 && s.From <= 1) {
			return fmt.Errorf("step %d: from %v must be in (0,1]", i, s.From)
		}
		if i > 0 && !(s.From > prev) {
			return errors.New("steps must be strictly ascending")
		}
		if !finiteNonNeg(s.Multiplier) {
			return fmt.Errorf("step %d: multiplier %v must be finite and non-negative", i, s.Multiplier)
		}
		prev = s.From
	}
	return nil
}

// Multiplier returns the multiplier for a pressure; lower bounds are inclusive.
// Steps are expected to be valid (ascending).
func (b Bands) Multiplier(pressure float64) float64 {
	m := b.Base
	for _, s := range b.Steps {
		if pressure >= s.From {
			m = s.Multiplier
		}
	}
	return m
}

// Max returns the highest multiplier among base and steps.
func (b Bands) Max() float64 {
	m := b.Base
	for _, s := range b.Steps {
		if s.Multiplier > m {
			m = s.Multiplier
		}
	}
	return m
}

// ValidPressure reports whether p is finite and within [0,1].
func ValidPressure(p float64) bool {
	return !math.IsNaN(p) && !math.IsInf(p, 0) && p >= 0 && p <= 1
}

// Sample is one raw pressure reading.
type Sample struct {
	At  time.Time
	Raw float64
}

// Smooth is the arithmetic mean of Raw over samples with now-window < At <= now.
// It returns false when no sample falls in the window. Input order is irrelevant.
func Smooth(samples []Sample, now time.Time, window time.Duration) (float64, bool) {
	from := now.Add(-window)
	var sum float64
	n := 0
	for _, s := range samples {
		if s.At.After(from) && !s.At.After(now) {
			sum += s.Raw
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

func toMT(v float64) int64 {
	r := math.Round(v) // half away from zero
	if r >= math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(r)
}

// DebitMT is weight x multiplier x elapsed hours x 1000, rounded half away
// from zero to a milli-token. Zero, negative or non-finite inputs give 0.
func DebitMT(weight, multiplier float64, elapsed time.Duration) int64 {
	if !(weight > 0) || !(multiplier > 0) || elapsed <= 0 || math.IsInf(weight, 0) || math.IsInf(multiplier, 0) {
		return 0
	}
	return toMT(weight * multiplier * elapsed.Hours() * MilliPerToken)
}

// ChargeableElapsed implements MET-4: the time since the previous successful
// poll, charged only if the gap is at most twice the poll interval; longer
// gaps are not charged and are reported as an outage.
func ChargeableElapsed(prev, now time.Time, pollInterval time.Duration) (time.Duration, bool) {
	if prev.IsZero() || !now.After(prev) {
		return 0, false
	}
	gap := now.Sub(prev)
	if gap > 2*pollInterval {
		return 0, true
	}
	return gap, false
}

// HourlyCostMT is the cost per hour in milli-tokens.
func HourlyCostMT(weight, multiplier float64) int64 {
	return DebitMT(weight, multiplier, time.Hour)
}

// ProjectedRunOut projects when the balance reaches zero at the average hourly
// burn of the last 24 hours. No burn means no projection (false); a balance
// at or below zero runs out now. Projections beyond 100 years give false.
func ProjectedRunOut(balanceMT int64, burnLast24hMT int64, now time.Time) (time.Time, bool) {
	if burnLast24hMT <= 0 {
		return time.Time{}, false
	}
	if balanceMT <= 0 {
		return now, true
	}
	hours := float64(balanceMT) * 24 / float64(burnLast24hMT)
	if hours > maxProjectionHours {
		return time.Time{}, false
	}
	return now.Add(time.Duration(hours * float64(time.Hour))), true
}
