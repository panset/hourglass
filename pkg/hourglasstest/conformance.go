// SPDX-License-Identifier: MIT

package hourglasstest

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/pkg/hourglass"
)

// reporter is the part of *testing.T the checks use, so the package's own
// tests can capture failures from deliberately broken implementations.
type reporter interface {
	Helper()
	Errorf(format string, args ...any)
}

type check struct {
	name string
	fn   func(r reporter)
}

// run executes each check as a subtest.
func run(t *testing.T, checks []check) {
	t.Helper()
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			t.Helper()
			guarded(t, c.fn)
		})
	}
}

// guarded runs fn and turns a panic into a reported failure.
func guarded(r reporter, fn func(reporter)) {
	defer func() {
		if p := recover(); p != nil {
			r.Errorf("panicked: %v", p)
		}
	}()
	fn(r)
}

const (
	promptly = time.Second            // bound for "returns promptly" after cancellation
	grace    = 500 * time.Millisecond // slack beyond a context deadline
)

// within runs fn in a goroutine and reports whether it returned within d. A
// panic in fn is re-raised in the caller. The goroutine is abandoned on timeout.
func within(d time.Duration, fn func()) bool {
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		fn()
	}()
	select {
	case p := <-done:
		if p != nil {
			panic(p)
		}
		return true
	case <-time.After(d):
		return false
	}
}

func cancelled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

// --- ResourceProvider ---

// TestProvider checks the ResourceProvider contract. newProvider must return
// a provider ready to List; it is called afresh for each check that needs one.
func TestProvider(t *testing.T, newProvider func() hourglass.ResourceProvider) {
	t.Helper()
	run(t, providerChecks(newProvider))
}

func sortedAllocs(in []hourglass.Allocation) []hourglass.Allocation {
	out := cloneAllocations(in)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ResourceName < out[j].ResourceName })
	return out
}

func providerChecks(newProvider func() hourglass.ResourceProvider) []check {
	return []check{
		{"Name_is_non_empty_and_stable", func(r reporter) {
			r.Helper()
			p := newProvider()
			n := p.Name()
			if n == "" {
				r.Errorf("Name() is empty; it must identify the provider in logs and config")
			}
			if n2 := p.Name(); n2 != n {
				r.Errorf("Name() is not stable: %q then %q", n, n2)
			}
		}},
		{"List_honours_context_cancellation_promptly", func(r reporter) {
			r.Helper()
			p := newProvider()
			if !within(promptly, func() { _, _ = p.List(cancelled()) }) {
				r.Errorf("List did not return within %v of being called with a cancelled context", promptly)
			}
		}},
		{"List_is_repeatable_when_the_backend_is_unchanged", func(r reporter) {
			r.Helper()
			p := newProvider()
			a, err := p.List(context.Background())
			if err != nil {
				r.Errorf("first List returned an error: %v", err)
				return
			}
			b, err := p.List(context.Background())
			if err != nil {
				r.Errorf("second List returned an error: %v", err)
				return
			}
			if !reflect.DeepEqual(sortedAllocs(a), sortedAllocs(b)) {
				r.Errorf("two List calls with no backend change returned different sets: %v vs %v", a, b)
			}
		}},
		{"No_allocation_has_an_empty_ResourceName", func(r reporter) {
			r.Helper()
			p := newProvider()
			a, err := p.List(context.Background())
			if err != nil {
				r.Errorf("List returned an error: %v", err)
				return
			}
			for i, al := range a {
				if al.ResourceName == "" {
					r.Errorf("allocation %d has an empty ResourceName (labels %v)", i, al.Labels)
				}
			}
		}},
		{"List_is_safe_for_concurrent_use", func(r reporter) {
			r.Helper()
			p := newProvider()
			var wg sync.WaitGroup
			errs := make(chan string, 8)
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() {
						if x := recover(); x != nil {
							errs <- fmt.Sprint("panic: ", x)
						}
					}()
					for j := 0; j < 5; j++ {
						if _, err := p.List(context.Background()); err != nil {
							errs <- err.Error()
							return
						}
					}
				}()
			}
			wg.Wait()
			close(errs)
			for e := range errs {
				r.Errorf("concurrent List failed: %s", e)
			}
		}},
	}
}

// --- PressureSource ---

// TestPressureSource checks the PressureSource contract against each given
// snapshot and an empty one. With no snapshots, a small default is used.
func TestPressureSource(t *testing.T, newSource func() hourglass.PressureSource, snapshots ...hourglass.Snapshot) {
	t.Helper()
	run(t, pressureChecks(newSource, snapshots))
}

func pressureInputs(snaps []hourglass.Snapshot) []hourglass.Snapshot {
	in := []hourglass.Snapshot{{}}
	if len(snaps) == 0 {
		snaps = []hourglass.Snapshot{
			NewSnapshot().Setup("setup-a", Label("tier", "a")).Hold("setup-a", 1).Allocation("setup-a").Build(),
		}
	}
	return append(in, snaps...)
}

func pressureChecks(newSource func() hourglass.PressureSource, snaps []hourglass.Snapshot) []check {
	inputs := pressureInputs(snaps)
	return []check{
		{"Name_is_non_empty_and_stable", func(r reporter) {
			r.Helper()
			s := newSource()
			n := s.Name()
			if n == "" {
				r.Errorf("Name() is empty")
			}
			if n2 := s.Name(); n2 != n {
				r.Errorf("Name() is not stable: %q then %q", n, n2)
			}
		}},
		{"Pressure_is_finite_and_within_0_to_1_or_an_error", func(r reporter) {
			r.Helper()
			s := newSource()
			for i, in := range inputs {
				v, err := s.Pressure(context.Background(), cloneSnapshot(in))
				if err != nil {
					continue // an error is allowed; an out-of-range value is not
				}
				if !finite(v) {
					r.Errorf("snapshot %d: pressure %v is not finite", i, v)
				} else if v < 0 || v > 1 {
					r.Errorf("snapshot %d: pressure %v is outside [0,1]; return an error instead", i, v)
				}
			}
		}},
		{"Pressure_honours_context_cancellation_promptly", func(r reporter) {
			r.Helper()
			s := newSource()
			if !within(promptly, func() { _, _ = s.Pressure(cancelled(), cloneSnapshot(inputs[len(inputs)-1])) }) {
				r.Errorf("Pressure did not return within %v with a cancelled context", promptly)
			}
		}},
		{"Pressure_does_not_mutate_the_snapshot", func(r reporter) {
			r.Helper()
			s := newSource()
			for i, in := range inputs {
				before := cloneSnapshot(in)
				arg := cloneSnapshot(in)
				_, _ = s.Pressure(context.Background(), arg)
				if !reflect.DeepEqual(before, arg) {
					r.Errorf("snapshot %d was modified by Pressure", i)
				}
			}
		}},
	}
}

// --- MeteringPolicy ---

// PolicyCase is one input to the policy conformance suite.
type PolicyCase struct {
	Name     string
	Hold     hourglass.HoldState
	Snapshot hourglass.Snapshot
}

// NewPolicyCase builds a case from the hold at index holdIndex of s.
func NewPolicyCase(name string, s hourglass.Snapshot, holdIndex int, user hourglass.User) PolicyCase {
	return PolicyCase{Name: name, Hold: NewHoldState(s, holdIndex, user), Snapshot: s}
}

// TestMeteringPolicy checks the MeteringPolicy contract against each case.
// With no cases, one default case (a deployed hold with a matching
// allocation) is used.
func TestMeteringPolicy(t *testing.T, newPolicy func() hourglass.MeteringPolicy, cases ...PolicyCase) {
	t.Helper()
	run(t, policyChecks(newPolicy, cases))
}

func policyChecks(newPolicy func() hourglass.MeteringPolicy, cases []PolicyCase) []check {
	if len(cases) == 0 {
		s := NewSnapshot().Setup("setup-a", Label("tier", "a")).
			Hold("setup-a", 1, Metering(DefaultTime), Deployed(DefaultTime)).Allocation("setup-a").Build()
		cases = []PolicyCase{NewPolicyCase("default", s, 0, hourglass.User{ID: 1, Username: "user-1", Roles: []string{"developer"}})}
	}
	each := func(r reporter, f func(r reporter, p hourglass.MeteringPolicy, c PolicyCase)) {
		r.Helper()
		p := newPolicy()
		for _, c := range cases {
			f(r, p, c)
		}
	}
	return []check{
		{"Name_is_non_empty_and_stable", func(r reporter) {
			r.Helper()
			p := newPolicy()
			n := p.Name()
			if n == "" {
				r.Errorf("Name() is empty")
			}
			if n2 := p.Name(); n2 != n {
				r.Errorf("Name() is not stable: %q then %q", n, n2)
			}
		}},
		{"Weight_is_finite_and_not_negative", func(r reporter) {
			each(r, func(r reporter, p hourglass.MeteringPolicy, c PolicyCase) {
				d, err := p.Decide(context.Background(), cloneHoldState(c.Hold), cloneSnapshot(c.Snapshot))
				if err != nil {
					return
				}
				if !finite(d.Weight) || d.Weight < 0 {
					r.Errorf("case %q: Weight %v must be finite and >= 0", c.Name, d.Weight)
				}
			})
		}},
		{"Reason_is_set_when_Meter_is_true", func(r reporter) {
			each(r, func(r reporter, p hourglass.MeteringPolicy, c PolicyCase) {
				d, err := p.Decide(context.Background(), cloneHoldState(c.Hold), cloneSnapshot(c.Snapshot))
				if err == nil && d.Meter && d.Reason == "" {
					r.Errorf("case %q: Meter is true but Reason is empty; the dashboard shows the reason", c.Name)
				}
			})
		}},
		{"Decide_is_deterministic_for_the_same_input", func(r reporter) {
			each(r, func(r reporter, p hourglass.MeteringPolicy, c PolicyCase) {
				d1, e1 := p.Decide(context.Background(), cloneHoldState(c.Hold), cloneSnapshot(c.Snapshot))
				d2, e2 := p.Decide(context.Background(), cloneHoldState(c.Hold), cloneSnapshot(c.Snapshot))
				if (e1 == nil) != (e2 == nil) || (e1 != nil && e1.Error() != e2.Error()) {
					r.Errorf("case %q: error differed between identical calls: %v vs %v", c.Name, e1, e2)
				} else if e1 == nil && d1 != d2 {
					r.Errorf("case %q: decisions differed between identical calls: %+v vs %+v", c.Name, d1, d2)
				}
			})
		}},
		{"Decide_honours_context_cancellation_promptly", func(r reporter) {
			each(r, func(r reporter, p hourglass.MeteringPolicy, c PolicyCase) {
				if !within(promptly, func() {
					_, _ = p.Decide(cancelled(), cloneHoldState(c.Hold), cloneSnapshot(c.Snapshot))
				}) {
					r.Errorf("case %q: Decide did not return within %v with a cancelled context", c.Name, promptly)
				}
			})
		}},
		{"Decide_does_not_mutate_its_inputs", func(r reporter) {
			each(r, func(r reporter, p hourglass.MeteringPolicy, c PolicyCase) {
				h, s := cloneHoldState(c.Hold), cloneSnapshot(c.Snapshot)
				_, _ = p.Decide(context.Background(), h, s)
				if !reflect.DeepEqual(h, c.Hold) {
					r.Errorf("case %q: the HoldState was modified by Decide", c.Name)
				}
				if !reflect.DeepEqual(s, c.Snapshot) {
					r.Errorf("case %q: the Snapshot was modified by Decide", c.Name)
				}
			})
		}},
	}
}

// --- Hook ---

// TestHook checks the Hook contract. newHook must return a hook that is safe
// to call with empty events; point it at a test double, not a live service.
func TestHook(t *testing.T, newHook func() hourglass.Hook) {
	t.Helper()
	run(t, hookChecks(newHook))
}

func hookChecks(newHook func() hourglass.Hook) []check {
	handle := func(h hourglass.Hook, typ string) (ok bool) {
		const deadline = 500 * time.Millisecond
		ctx, cancel := context.WithTimeout(context.Background(), deadline)
		defer cancel()
		// A returned error is legal; a panic is re-raised by within and reported by guarded.
		return within(deadline+grace, func() {
			_ = h.Handle(ctx, hourglass.Event{Type: typ, Time: DefaultTime})
		})
	}
	return []check{
		{"Name_is_non_empty_and_stable", func(r reporter) {
			r.Helper()
			h := newHook()
			n := h.Name()
			if n == "" {
				r.Errorf("Name() is empty")
			}
			if n2 := h.Name(); n2 != n {
				r.Errorf("Name() is not stable: %q then %q", n, n2)
			}
		}},
		{"Handle_accepts_every_event_type_with_nil_User_Setup_Hold", func(r reporter) {
			r.Helper()
			h := newHook()
			for _, typ := range hourglass.EventTypes() {
				// A returned error is legal; a panic is not (guarded turns it into a failure).
				if !handle(h, typ) {
					r.Errorf("event %q: Handle did not return within its context deadline", typ)
					return
				}
			}
		}},
		{"Handle_returns_within_the_context_deadline", func(r reporter) {
			r.Helper()
			h := newHook()
			for _, typ := range hourglass.EventTypes() {
				if !handle(h, typ) {
					r.Errorf("event %q: Handle ignored the context deadline", typ)
					return
				}
			}
		}},
		{"Handle_tolerates_an_unknown_event_type", func(r reporter) {
			r.Helper()
			h := newHook()
			if !handle(h, "test.unknown-event") {
				r.Errorf("Handle did not return for an unknown event type")
			}
		}},
	}
}
