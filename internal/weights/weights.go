// SPDX-License-Identifier: MIT

// Package weights is the one weight resolver (docs/design/reported-weights.md,
// A1): given a setup, the rate rules, the settings, the stored weights report
// and a time, it says which weight applies, where it comes from, and what the
// setup's reported weight is and why it is not applied. It is pure: it reads
// no database and keeps no state, so every reader gets the same answer for
// the same inputs.
package weights

import (
	"maps"
	"time"

	"github.com/panset/hourglass/internal/rules"
	"github.com/panset/hourglass/internal/settings"
)

// Source says where a weight came from. The resolver returns the first four;
// SourcePolicy is the collector's, for a poll whose metering policy set
// Decision.Weight above 0.
type Source string

// The weight sources, in the resolver's order of precedence.
const (
	SourceOverride Source = "override"
	SourceReported Source = "reported"
	SourceRule     Source = "rule"
	SourceDefault  Source = "default"
	SourcePolicy   Source = "policy"
)

// NotApplied says why a setup's reported weight is not the weight applied.
// The empty value means it is applied, or there is no reported weight.
type NotApplied string

// The reasons, in the order the resolver picks the first that holds.
const (
	NotAppliedExpired     NotApplied = "expired"
	NotAppliedOutOfBounds NotApplied = "out_of_bounds"
	NotAppliedOverride    NotApplied = "override"
)

// Setup is what the resolver needs of a setup.
type Setup struct {
	Name   string
	Labels map[string]string
	// Override is the setup's weight override (design D2), nil when none.
	Override *float64
}

// Entry is one weight of the stored weights report.
type Entry struct {
	Setup      string
	Weight     float64
	Attributes map[string]string
}

// Report is the stored weights report as the resolver reads it. A nil
// *Report means no report is stored.
type Report struct {
	ObservedAt time.Time
	entries    map[string]Entry
}

// NewReport returns the report observed at observedAt holding entries. Names
// are unique in a stored report; if one repeats here, the last entry wins.
func NewReport(observedAt time.Time, entries []Entry) *Report {
	r := &Report{ObservedAt: observedAt, entries: make(map[string]Entry, len(entries))}
	for _, e := range entries {
		r.entries[e.Setup] = e
	}
	return r
}

// Result is the resolver's answer for one setup at one time.
type Result struct {
	// Weight is the weight to apply.
	Weight float64
	// Source is override, reported, rule or default.
	Source Source
	// BaseWeight is the weight without any report: the override, else the
	// first matching rate rule, else the default weight.
	BaseWeight float64
	// RuleSelector is the selector of the first matching rate rule, whatever
	// the source; empty when no rule matches.
	RuleSelector string
	// Reported is the stored entry's weight when reported weights are on and
	// the stored report has an entry for the setup, usable or not; else nil.
	Reported *float64
	// NotApplied is why Reported is not applied: the first of expired,
	// out_of_bounds, override that holds; empty otherwise.
	NotApplied NotApplied
	// Attributes are the entry's attributes when the entry is usable (an
	// override does not hide them). Never nil: empty when there are none or
	// the entry is not usable.
	Attributes map[string]string
}

// Resolve returns the weight of setup s at time at (A1). The weight is the
// first that exists of: the override, a usable reported weight, the first
// matching rate rule, the default weight. An entry is usable when reported
// weights are on, the report's observed_at is no older than max_age at at,
// and the weight lies within the minimum and maximum of cfg, both inclusive.
// With reported weights off the report is ignored entirely.
//
// A rate rule that cannot be evaluated (a selector that does not parse, or
// the matching rule's value is not a weight) is returned as the error of
// rules.Weight, with a zero Result: each caller keeps its own effect for it.
// The rules are evaluated whatever the source, so an override does not hide
// such an error.
func Resolve(s Setup, rs []rules.Rule, cfg settings.Settings, report *Report, at time.Time) (Result, error) {
	ruleW, sel, matched, err := rules.Weight(rs, s.Labels, cfg.DefaultWeight)
	if err != nil {
		return Result{}, err
	}
	res := Result{Weight: ruleW, Source: SourceDefault, BaseWeight: ruleW, Attributes: map[string]string{}}
	if matched {
		res.Source, res.RuleSelector = SourceRule, sel
	}
	if s.Override != nil {
		res.Weight, res.Source, res.BaseWeight = *s.Override, SourceOverride, *s.Override
	}

	rw := cfg.ReportedWeights
	if !rw.Enabled || report == nil {
		return res, nil
	}
	e, ok := report.entries[s.Name]
	if !ok {
		return res, nil
	}
	w := e.Weight
	res.Reported = &w
	switch {
	case expired(report.ObservedAt, at, rw.MaxAge):
		res.NotApplied = NotAppliedExpired
		return res, nil
	case !within(w, rw):
		res.NotApplied = NotAppliedOutOfBounds
		return res, nil
	}
	// Usable: its attributes are shown even under an override.
	res.Attributes = maps.Clone(e.Attributes)
	if res.Attributes == nil {
		res.Attributes = map[string]string{}
	}
	if s.Override != nil {
		res.NotApplied = NotAppliedOverride
		return res, nil
	}
	res.Weight, res.Source = w, SourceReported
	return res, nil
}

// expired reports whether a report observed at observed is older than maxAge
// at at. Age counts from observed_at and is never below zero (D46).
func expired(observed, at time.Time, maxAge time.Duration) bool {
	age := at.Sub(observed)
	if age < 0 {
		age = 0
	}
	return age > maxAge
}

// within reports whether w lies within the limits now in force, both
// inclusive. A nil maximum (possible only while the feature is off, which
// never reaches here) bounds nothing.
func within(w float64, rw settings.ReportedWeights) bool {
	if w < rw.Min {
		return false
	}
	return rw.Max == nil || w <= *rw.Max
}
