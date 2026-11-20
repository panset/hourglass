// SPDX-License-Identifier: MIT

package collector

import (
	"sort"

	"github.com/panset/hourglass/pkg/hourglass"
)

// DedupAllocations drops repeated resource names, keeping the first of each
// (a duplicated name counts once).
func DedupAllocations(list []hourglass.Allocation) []hourglass.Allocation {
	seen := make(map[string]bool, len(list))
	var out []hourglass.Allocation
	for _, a := range list {
		if seen[a.ResourceName] {
			continue
		}
		seen[a.ResourceName] = true
		out = append(out, a)
	}
	return out
}

// OrphanNames returns, sorted, the reported resource names that equal the name
// of one of the setups while no active hold is on that setup (HOLD-4, D40).
// Allocations matching no setup are not orphans.
func OrphanNames(setupNames []string, held map[string]bool, reported map[string]hourglass.Allocation) []string {
	var names []string
	seen := map[string]bool{}
	for _, rn := range setupNames {
		if _, ok := reported[rn]; ok && rn != "" && !held[rn] && !seen[rn] {
			seen[rn] = true
			names = append(names, rn)
		}
	}
	sort.Strings(names)
	return names
}

// SetupRef is a setup as seen by Reconcile.
type SetupRef struct {
	Name    string // an allocation with exactly this resource name is the setup's (D40)
	Enabled bool
	Held    bool // has an active hold
}

// Reconciliation is how a provider's allocations line up with the setups.
type Reconciliation struct {
	// Matched are the setups (enabled or not) whose resource is reported.
	Matched []SetupRef
	// Missing are the enabled setups whose resource is not reported.
	Missing []SetupRef
	// Orphans are reported resource names matching a setup without an active hold.
	Orphans []string
	// Ignored counts distinct reported allocations matching no setup.
	Ignored int
}

// Reconcile applies the collector's matching rules to one provider listing.
func Reconcile(setups []SetupRef, allocs []hourglass.Allocation) Reconciliation {
	reported := map[string]hourglass.Allocation{}
	for _, a := range DedupAllocations(allocs) {
		reported[a.ResourceName] = a
	}
	var r Reconciliation
	names := make([]string, 0, len(setups))
	held := map[string]bool{}
	known := map[string]bool{}
	for _, s := range setups {
		names = append(names, s.Name)
		if s.Held {
			held[s.Name] = true
		}
		if s.Name != "" {
			known[s.Name] = true
		}
		_, ok := reported[s.Name]
		switch {
		case ok && s.Name != "":
			r.Matched = append(r.Matched, s)
		case s.Enabled:
			r.Missing = append(r.Missing, s)
		}
	}
	r.Orphans = OrphanNames(names, held, reported)
	for name := range reported {
		if !known[name] {
			r.Ignored++
		}
	}
	return r
}
