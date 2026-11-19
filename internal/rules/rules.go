// SPDX-License-Identifier: MIT

package rules

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

// KindRate is the rule kind for rate rules (RULE-1).
const KindRate = "rate"

// Rule is one row of a rule table. Value is opaque to the engine (RULE-5).
type Rule struct {
	ID       int64
	Kind     string
	Position int
	Selector string
	Value    string
}

// ordered returns the rules of kind sorted by ascending Position (stable).
func ordered(rules []Rule, kind string) []Rule {
	out := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if r.Kind == kind {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out
}

// Evaluate returns the first rule of kind, in ascending Position, whose
// selector matches labels, or nil. A rule whose selector does not parse
// yields an error naming the rule.
func Evaluate(rules []Rule, kind string, labels map[string]string) (*Rule, error) {
	for _, r := range ordered(rules, kind) {
		ok, err := Match(r.Selector, labels)
		if err != nil {
			return nil, fmt.Errorf("rule %d: %w", r.ID, err)
		}
		if ok {
			r := r
			return &r, nil
		}
	}
	return nil, nil
}

// ParseWeight parses a non-negative, finite decimal such as "0", "1", "0.5"
// or ".5". Signs, exponents, hex, inf, nan and whitespace are rejected.
func ParseWeight(s string) (float64, error) {
	if !validDecimal(s) {
		return 0, fmt.Errorf("weight %q: must be a non-negative decimal number", s)
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, fmt.Errorf("weight %q: out of range", s)
	}
	return f, nil
}

func validDecimal(s string) bool {
	digits, dot := 0, false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			digits++
		case c == '.' && !dot:
			dot = true
		default:
			return false
		}
	}
	return digits > 0
}

// Weight returns the weight and selector of the first matching rate rule, or
// defaultWeight with matched=false when none matches. A matching rule whose
// value is not a valid weight is an error.
func Weight(rules []Rule, labels map[string]string, defaultWeight float64) (weight float64, selector string, matched bool, err error) {
	r, err := Evaluate(rules, KindRate, labels)
	if err != nil {
		return defaultWeight, "", false, err
	}
	if r == nil {
		return defaultWeight, "", false, nil
	}
	w, err := ParseWeight(r.Value)
	if err != nil {
		return defaultWeight, "", false, fmt.Errorf("rule %d: %w", r.ID, err)
	}
	return w, r.Selector, true, nil
}

// MatchCounts reports, for RULE-3, how many label sets each rule of kind wins
// (first-match semantics) and the indexes of label sets that match no rule.
// Every rule of the kind appears in perRule, with 0 if it wins nothing. A rule
// with an unparseable selector never matches.
func MatchCounts(rules []Rule, kind string, labelSets []map[string]string) (perRule map[int64]int, unmatched []int) {
	rs := ordered(rules, kind)
	sels := make([]Selector, len(rs))
	valid := make([]bool, len(rs))
	perRule = make(map[int64]int, len(rs))
	for i, r := range rs {
		s, err := Parse(r.Selector)
		sels[i], valid[i] = s, err == nil
		perRule[r.ID] += 0
	}
	unmatched = []int{}
	for idx, labels := range labelSets {
		won := false
		for i, r := range rs {
			if valid[i] && sels[i].Matches(labels) {
				perRule[r.ID]++
				won = true
				break
			}
		}
		if !won {
			unmatched = append(unmatched, idx)
		}
	}
	return perRule, unmatched
}
