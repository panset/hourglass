// SPDX-License-Identifier: MIT

package reported

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/panset/hourglass/internal/rules"
	"github.com/panset/hourglass/internal/store"
)

// Limits of one weights report (docs/design/reported-weights.md, B1).
const (
	// MaxWeightEntries is the largest number of entries in one report.
	MaxWeightEntries = 10000
	// MaxWeightAttributes is the largest number of attributes of one entry.
	MaxWeightAttributes = 16
	// MaxAttributeValueLength is the longest attribute value, in characters.
	MaxAttributeValueLength = 63
)

// WeightLimits are the minimum and maximum weight in force, both inclusive.
// They come from the caller (the reported_weights setting), like max_age.
type WeightLimits struct {
	Min float64
	Max float64
}

// Weight is one entry of a weights report. Attributes may be nil on input; a
// read returns a non-nil map.
type Weight struct {
	Setup      string
	Weight     float64
	Attributes map[string]string
}

// Weights is the stored weights report with its age. Weights is non-nil and
// empty when the report said no setup has a weight.
type Weights struct {
	ObservedAt     time.Time
	ReceivedAt     time.Time
	ServiceAccount string
	Weights        []Weight // ordered by setup name
	// Age is the time since ObservedAt, measured at read time (never negative).
	Age time.Duration
}

// EntryError is a weights report refused because of one entry (B2). Index is
// the entry's position in the report's list, from 0. Setup is the entry's
// name when it has a non-empty string name, else empty. Its message is the
// one the API returns; errors.As finds a *ValidationError in it, so the API
// maps it to 400 validation.
type EntryError struct {
	Index  int
	Setup  string
	Reason string
}

// Error returns the entry's position, its name when it has one, and the
// reason, for example `weights[3] (setup "lab-03"): weight 25 is above the
// maximum 20`.
func (e *EntryError) Error() string { return EntryMessage(e.Index, e.Setup, e.Reason) }

// Unwrap returns the same message as a *ValidationError.
func (e *EntryError) Unwrap() error { return &ValidationError{Message: e.Error()} }

// EntryMessage formats the refusal of entry index (from 0) of a weights
// report. setup is omitted when empty. The HTTP handler uses it for problems
// it finds while decoding an entry (a wrong type, an unknown field), so every
// entry refusal has the same form.
func EntryMessage(index int, setup, reason string) string {
	if setup == "" {
		return fmt.Sprintf("weights[%d]: %s", index, reason)
	}
	return fmt.Sprintf("weights[%d] (setup %q): %s", index, setup, reason)
}

func formatWeight(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// WeightChecker checks the entries of one report in order: a setup name that
// is empty or repeated, a weight that is not finite or outside the limits, and
// attributes that break the limits of B1. Use one checker per report; a
// handler that decodes entries itself calls Check after decoding each one, so
// the first invalid entry is the one reported.
type WeightChecker struct {
	limits WeightLimits
	seen   map[string]int
}

// NewWeightChecker returns a checker for one report under limits.
func NewWeightChecker(limits WeightLimits) *WeightChecker {
	return &WeightChecker{limits: limits, seen: map[string]int{}}
}

// Check validates entry index and records its name. It returns an
// *EntryError, or nil.
func (c *WeightChecker) Check(index int, w Weight) error {
	fail := func(format string, args ...any) error {
		return &EntryError{Index: index, Setup: w.Setup, Reason: fmt.Sprintf(format, args...)}
	}
	if w.Setup == "" {
		return fail("setup must be a non-empty string")
	}
	if first, dup := c.seen[w.Setup]; dup {
		return fail("setup is repeated (first at weights[%d])", first)
	}
	c.seen[w.Setup] = index
	switch {
	case math.IsNaN(w.Weight) || math.IsInf(w.Weight, 0):
		return fail("weight must be a finite number")
	case w.Weight < c.limits.Min:
		return fail("weight %s is below the minimum %s", formatWeight(w.Weight), formatWeight(c.limits.Min))
	case w.Weight > c.limits.Max:
		return fail("weight %s is above the maximum %s", formatWeight(w.Weight), formatWeight(c.limits.Max))
	}
	if n := len(w.Attributes); n > MaxWeightAttributes {
		return fail("attributes has %d entries (at most %d)", n, MaxWeightAttributes)
	}
	if err := rules.ValidateLabels(w.Attributes); err != nil {
		return fail("attributes: %v", err)
	}
	return nil
}

// SubmitWeights validates and stores the complete weights report from
// account, replacing the previous report and header atomically (B1, B3). An
// empty or nil list means no setup has a reported weight and is stored as
// such. Every entry is checked with a WeightChecker under limits; the first
// invalid entry refuses the whole report with an *EntryError and nothing is
// stored. More than MaxWeightEntries entries is a *ValidationError. Ordering
// and clock skew are those of the other reports (a nil observedAt means the
// receipt time, an older observed_at is a *StaleError, an equal one replaces,
// more than MaxClockSkew ahead is a *ValidationError), compared with the
// stored weights report only. Negative zero is stored as 0. Matching names to
// setups is the caller's job: every valid name is stored.
func (s *Service) SubmitWeights(ctx context.Context, account string, observedAt *time.Time, weights []Weight, limits WeightLimits) (Weights, error) {
	if len(weights) > MaxWeightEntries {
		return Weights{}, &ValidationError{Message: fmt.Sprintf("weights has %d entries (at most %d)", len(weights), MaxWeightEntries)}
	}
	check := NewWeightChecker(limits)
	set := make([]store.ReportedWeight, 0, len(weights))
	for i, w := range weights {
		if err := check.Check(i, w); err != nil {
			return Weights{}, err
		}
		v := w.Weight
		if v == 0 {
			v = 0 // negative zero counts as 0
		}
		attrs := make(map[string]string, len(w.Attributes))
		for k, a := range w.Attributes {
			attrs[k] = a
		}
		set = append(set, store.ReportedWeight{SetupName: w.Setup, Weight: v, Attributes: attrs})
	}
	observed, now, err := s.resolve(observedAt)
	if err != nil {
		return Weights{}, err
	}
	hdr := store.ReportedHeader{ObservedAt: observed, ReceivedAt: now, ServiceAccount: account}
	err = s.st.Tx(ctx, func(tx *store.Tx) error {
		cur, _, err := tx.GetReportedWeights(ctx)
		switch {
		case errors.Is(err, store.ErrNotFound):
		case err != nil:
			return err
		case observed.Before(cur.ObservedAt):
			return &StaleError{Stored: cur.ObservedAt, Reported: observed}
		}
		return tx.ReplaceReportedWeights(ctx, hdr, set)
	})
	if err != nil {
		return Weights{}, err
	}
	return weightsFrom(hdr, set, now), nil
}

// Weights returns the stored weights report with its age, or ErrNoReport. The
// header and the set come from one snapshot. Whether the report is usable
// (the setting, max_age, the limits) is the caller's question.
func (s *Service) Weights(ctx context.Context) (Weights, error) {
	h, set, err := s.st.GetReportedWeights(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return Weights{}, ErrNoReport
	}
	if err != nil {
		return Weights{}, err
	}
	return weightsFrom(h, set, s.clock.Now()), nil
}

func weightsFrom(h store.ReportedHeader, set []store.ReportedWeight, now time.Time) Weights {
	out := Weights{ObservedAt: h.ObservedAt, ReceivedAt: h.ReceivedAt, ServiceAccount: h.ServiceAccount,
		Weights: make([]Weight, 0, len(set)), Age: age(h.ObservedAt, now)}
	for _, w := range set {
		attrs := w.Attributes
		if attrs == nil {
			attrs = map[string]string{}
		}
		out.Weights = append(out.Weights, Weight{Setup: w.SetupName, Weight: w.Weight, Attributes: attrs})
	}
	sort.Slice(out.Weights, func(i, j int) bool { return out.Weights[i].Setup < out.Weights[j].Setup })
	return out
}

// Refusal is the last weights report refused with 400 (B2): when, by which
// account, and the message returned.
type Refusal struct {
	At             time.Time
	ServiceAccount string
	Message        string
}

// refusalLog keeps the most recent refused weights report in memory. It is
// lost at a restart, like the poll view (D20). The zero value is ready.
type refusalLog struct {
	mu   sync.Mutex
	last *Refusal
}

// RecordWeightsRefusal keeps r as the last refused weights report, replacing
// the one kept before. The HTTP handler calls it for every report it refuses
// with 400.
func (s *Service) RecordWeightsRefusal(r Refusal) {
	s.refusals.mu.Lock()
	defer s.refusals.mu.Unlock()
	s.refusals.last = &r
}

// LastWeightsRefusal returns the last refused weights report recorded on this
// Service, and false when none was recorded since the process started.
func (s *Service) LastWeightsRefusal() (Refusal, bool) {
	s.refusals.mu.Lock()
	defer s.refusals.mu.Unlock()
	if s.refusals.last == nil {
		return Refusal{}, false
	}
	return *s.refusals.last, true
}
