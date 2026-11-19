// SPDX-License-Identifier: MIT

package hourglasstest

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/pkg/hourglass"
)

// collector captures failures instead of failing the real test.
type collector struct {
	mu   sync.Mutex
	msgs []string
}

func (c *collector) Helper() {}
func (c *collector) Errorf(f string, a ...any) {
	c.mu.Lock()
	c.msgs = append(c.msgs, fmt.Sprintf(f, a...))
	c.mu.Unlock()
}

// failures runs the checks and returns failure messages keyed by check name.
func failures(checks []check) map[string][]string {
	out := map[string][]string{}
	for _, c := range checks {
		col := &collector{}
		guarded(col, c.fn)
		if len(col.msgs) > 0 {
			out[c.name] = col.msgs
		}
	}
	return out
}

func mustFail(t *testing.T, got map[string][]string, name string) {
	t.Helper()
	if len(got[name]) == 0 {
		t.Errorf("check %q did not fail against the broken implementation; failures: %v", name, got)
	}
}

// --- fakes pass their own suites ---

func TestEXT5_FakeProviderConforms(t *testing.T) {
	TestProvider(t, func() hourglass.ResourceProvider {
		p := NewFakeProvider()
		p.SetAllocations(hourglass.Allocation{ResourceName: "r1", Labels: map[string]string{"tier": "a"}}, hourglass.Allocation{ResourceName: "r2"})
		return p
	})
}

func TestEXT5_FakePressureSourceConforms(t *testing.T) {
	TestPressureSource(t, func() hourglass.PressureSource {
		p := NewFakePressureSource()
		p.SetValue(0.4)
		return p
	}, NewSnapshot().Setup("s1", Label("tier", "a")).Hold("s1", 1).Build())
}

func TestEXT5_FakePolicyConforms(t *testing.T) {
	s := NewSnapshot().Setup("s1", Label("tier", "a"), Weight(2)).Hold("s1", 7, Metering(DefaultTime)).Allocation("s1").Build()
	TestMeteringPolicy(t, func() hourglass.MeteringPolicy { return NewFakePolicy() },
		NewPolicyCase("deployed", s, 0, hourglass.User{ID: 7, Roles: []string{"developer"}}))
}

func TestEXT5_FakeHookConforms(t *testing.T) {
	TestHook(t, func() hourglass.Hook { return NewFakeHook() })
}

func TestEXT5_SuitesRunWithDefaults(t *testing.T) {
	TestPressureSource(t, func() hourglass.PressureSource { return NewFakePressureSource() })
	TestMeteringPolicy(t, func() hourglass.MeteringPolicy { return NewFakePolicy() })
}

// --- broken implementations are caught ---

type brokenProvider struct {
	name     string
	list     func(ctx context.Context) ([]hourglass.Allocation, error)
	flipName bool
	n        int
}

func (b *brokenProvider) Name() string {
	if b.flipName {
		b.n++
		return fmt.Sprint("p", b.n)
	}
	return b.name
}
func (b *brokenProvider) List(ctx context.Context) ([]hourglass.Allocation, error) {
	return b.list(ctx)
}

func okList(context.Context) ([]hourglass.Allocation, error) {
	return []hourglass.Allocation{{ResourceName: "r"}}, nil
}

func TestEXT5_BrokenProviderIsCaught(t *testing.T) {
	cases := []struct {
		check string
		p     func() hourglass.ResourceProvider
	}{
		{"Name_is_non_empty_and_stable", func() hourglass.ResourceProvider { return &brokenProvider{list: okList} }},
		{"Name_is_non_empty_and_stable", func() hourglass.ResourceProvider { return &brokenProvider{flipName: true, list: okList} }},
		{"List_honours_context_cancellation_promptly", func() hourglass.ResourceProvider {
			return &brokenProvider{name: "p", list: func(ctx context.Context) ([]hourglass.Allocation, error) {
				time.Sleep(2 * time.Second)
				return nil, nil
			}}
		}},
		{"List_is_repeatable_when_the_backend_is_unchanged", func() hourglass.ResourceProvider {
			var mu sync.Mutex
			n := 0
			return &brokenProvider{name: "p", list: func(context.Context) ([]hourglass.Allocation, error) {
				mu.Lock()
				defer mu.Unlock()
				n++
				return []hourglass.Allocation{{ResourceName: fmt.Sprint("r", n)}}, nil
			}}
		}},
		{"No_allocation_has_an_empty_ResourceName", func() hourglass.ResourceProvider {
			return &brokenProvider{name: "p", list: func(context.Context) ([]hourglass.Allocation, error) {
				return []hourglass.Allocation{{ResourceName: ""}}, nil
			}}
		}},
		{"List_is_safe_for_concurrent_use", func() hourglass.ResourceProvider {
			return &brokenProvider{name: "p", list: func(context.Context) ([]hourglass.Allocation, error) {
				return nil, errors.New("boom")
			}}
		}},
		{"List_is_safe_for_concurrent_use", func() hourglass.ResourceProvider {
			return &brokenProvider{name: "p", list: func(context.Context) ([]hourglass.Allocation, error) { panic("racy") }}
		}},
	}
	for i, c := range cases {
		c := c
		t.Run(fmt.Sprint(i, "_", c.check), func(t *testing.T) {
			t.Parallel()
			mustFail(t, failures(providerChecks(c.p)), c.check)
		})
	}
}

type brokenPressure struct {
	name string
	fn   func(ctx context.Context, s hourglass.Snapshot) (float64, error)
}

func (b brokenPressure) Name() string { return b.name }
func (b brokenPressure) Pressure(ctx context.Context, s hourglass.Snapshot) (float64, error) {
	return b.fn(ctx, s)
}

func TestEXT5_BrokenPressureSourceIsCaught(t *testing.T) {
	val := func(v float64) func(context.Context, hourglass.Snapshot) (float64, error) {
		return func(context.Context, hourglass.Snapshot) (float64, error) { return v, nil }
	}
	cases := []struct {
		check string
		p     hourglass.PressureSource
	}{
		{"Name_is_non_empty_and_stable", brokenPressure{"", val(0.5)}},
		{"Pressure_is_finite_and_within_0_to_1_or_an_error", brokenPressure{"p", val(1.5)}},
		{"Pressure_is_finite_and_within_0_to_1_or_an_error", brokenPressure{"p", val(-0.1)}},
		{"Pressure_is_finite_and_within_0_to_1_or_an_error", brokenPressure{"p", val(math.NaN())}},
		{"Pressure_is_finite_and_within_0_to_1_or_an_error", brokenPressure{"p", val(math.Inf(1))}},
		{"Pressure_honours_context_cancellation_promptly", brokenPressure{"p", func(context.Context, hourglass.Snapshot) (float64, error) {
			time.Sleep(2 * time.Second)
			return 0, nil
		}}},
		{"Pressure_does_not_mutate_the_snapshot", brokenPressure{"p", func(_ context.Context, s hourglass.Snapshot) (float64, error) {
			if len(s.Setups) > 0 {
				s.Setups[0].Labels["tier"] = "mutated"
			}
			s.Allocations = append(s.Allocations[:0], hourglass.Allocation{ResourceName: "x"})
			s.Holds = append(s.Holds, hourglass.Hold{})
			return 0.5, nil
		}}},
	}
	snap := NewSnapshot().Setup("s1", Label("tier", "a")).Hold("s1", 1).Allocation("s1").Build()
	for i, c := range cases {
		c := c
		t.Run(fmt.Sprint(i, "_", c.check), func(t *testing.T) {
			t.Parallel()
			mustFail(t, failures(pressureChecks(func() hourglass.PressureSource { return c.p }, []hourglass.Snapshot{snap})), c.check)
		})
	}
}

type brokenPolicy struct {
	name string
	fn   func(ctx context.Context, h hourglass.HoldState, s hourglass.Snapshot) (hourglass.Decision, error)
}

func (b brokenPolicy) Name() string { return b.name }
func (b brokenPolicy) Decide(ctx context.Context, h hourglass.HoldState, s hourglass.Snapshot) (hourglass.Decision, error) {
	return b.fn(ctx, h, s)
}

func TestEXT5_BrokenPolicyIsCaught(t *testing.T) {
	dec := func(d hourglass.Decision) func(context.Context, hourglass.HoldState, hourglass.Snapshot) (hourglass.Decision, error) {
		return func(context.Context, hourglass.HoldState, hourglass.Snapshot) (hourglass.Decision, error) {
			return d, nil
		}
	}
	var mu sync.Mutex
	counter := 0.0
	cases := []struct {
		check string
		p     hourglass.MeteringPolicy
	}{
		{"Name_is_non_empty_and_stable", brokenPolicy{"", dec(hourglass.Decision{Meter: true, Weight: 1, Reason: "r"})}},
		{"Weight_is_finite_and_not_negative", brokenPolicy{"p", dec(hourglass.Decision{Weight: -1})}},
		{"Weight_is_finite_and_not_negative", brokenPolicy{"p", dec(hourglass.Decision{Weight: math.NaN()})}},
		{"Weight_is_finite_and_not_negative", brokenPolicy{"p", dec(hourglass.Decision{Weight: math.Inf(1)})}},
		{"Reason_is_set_when_Meter_is_true", brokenPolicy{"p", dec(hourglass.Decision{Meter: true, Weight: 1})}},
		{"Decide_is_deterministic_for_the_same_input", brokenPolicy{"p", func(context.Context, hourglass.HoldState, hourglass.Snapshot) (hourglass.Decision, error) {
			mu.Lock()
			defer mu.Unlock()
			counter++
			return hourglass.Decision{Meter: true, Weight: counter, Reason: "r"}, nil
		}}},
		{"Decide_honours_context_cancellation_promptly", brokenPolicy{"p", func(context.Context, hourglass.HoldState, hourglass.Snapshot) (hourglass.Decision, error) {
			time.Sleep(2 * time.Second)
			return hourglass.Decision{}, nil
		}}},
		{"Decide_does_not_mutate_its_inputs", brokenPolicy{"p", func(_ context.Context, h hourglass.HoldState, s hourglass.Snapshot) (hourglass.Decision, error) {
			h.Setup.Labels["tier"] = "mutated"
			s.Setups[0].Labels["tier"] = "mutated"
			return hourglass.Decision{Meter: true, Weight: 1, Reason: "r"}, nil
		}}},
	}
	snap := NewSnapshot().Setup("s1", Label("tier", "a")).Hold("s1", 1).Build()
	pc := []PolicyCase{NewPolicyCase("c", snap, 0, hourglass.User{ID: 1})}
	for i, c := range cases {
		c := c
		t.Run(fmt.Sprint(i, "_", c.check), func(t *testing.T) {
			t.Parallel()
			mustFail(t, failures(policyChecks(func() hourglass.MeteringPolicy { return c.p }, pc)), c.check)
		})
	}
}

type brokenHook struct {
	name string
	fn   func(ctx context.Context, e hourglass.Event) error
}

func (b brokenHook) Name() string                                        { return b.name }
func (b brokenHook) Handle(ctx context.Context, e hourglass.Event) error { return b.fn(ctx, e) }

func TestEXT5_BrokenHookIsCaught(t *testing.T) {
	ok := func(context.Context, hourglass.Event) error { return nil }
	cases := []struct {
		check string
		h     hourglass.Hook
	}{
		{"Name_is_non_empty_and_stable", brokenHook{"", ok}},
		{"Handle_accepts_every_event_type_with_nil_User_Setup_Hold", brokenHook{"h", func(_ context.Context, e hourglass.Event) error {
			_ = e.User.Username // nil dereference
			return nil
		}}},
		{"Handle_returns_within_the_context_deadline", brokenHook{"h", func(context.Context, hourglass.Event) error {
			time.Sleep(2 * time.Second)
			return nil
		}}},
		{"Handle_tolerates_an_unknown_event_type", brokenHook{"h", func(_ context.Context, e hourglass.Event) error {
			if !strings.HasPrefix(e.Type, "hold.") && !strings.HasPrefix(e.Type, "balance.") &&
				!strings.HasPrefix(e.Type, "user.") && !strings.HasPrefix(e.Type, "period.") && !strings.HasPrefix(e.Type, "poll.") {
				panic("unknown event " + e.Type)
			}
			return nil
		}}},
	}
	for i, c := range cases {
		c := c
		t.Run(fmt.Sprint(i, "_", c.check), func(t *testing.T) {
			t.Parallel()
			mustFail(t, failures(hookChecks(func() hourglass.Hook { return c.h })), c.check)
		})
	}
}

// --- fakes ---

func TestFakeClock(t *testing.T) {
	var _ interface{ Now() time.Time } = (*FakeClock)(nil)
	c := NewFakeClock(time.Time{})
	if !c.Now().Equal(DefaultTime) {
		t.Fatalf("default start = %v", c.Now())
	}
	c.Advance(90 * time.Minute)
	if got := c.Now(); !got.Equal(DefaultTime.Add(90 * time.Minute)) {
		t.Fatalf("after Advance: %v", got)
	}
	want := time.Date(2031, 5, 6, 0, 0, 0, 0, time.UTC)
	c.Set(want)
	if !c.Now().Equal(want) {
		t.Fatal("Set did not take effect")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Advance(time.Second)
				_ = c.Now()
				c.Set(want)
			}
		}()
	}
	wg.Wait()
}

func TestFakeProviderBehaviour(t *testing.T) {
	p := NewFakeProvider()
	p.SetName("x")
	in := hourglass.Allocation{ResourceName: "r", Labels: map[string]string{"tier": "a"}}
	p.SetAllocations(in)
	in.Labels["tier"] = "changed"
	got, err := p.List(context.Background())
	if err != nil || p.Name() != "x" || got[0].Labels["tier"] != "a" {
		t.Fatalf("got %v %v name %q", got, err, p.Name())
	}
	got[0].Labels["tier"] = "z"
	again, _ := p.List(context.Background())
	if again[0].Labels["tier"] != "a" {
		t.Fatal("List result aliases internal state")
	}
	boom := errors.New("boom")
	p.SetError(boom)
	if _, err := p.List(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if _, err := p.List(cancelled()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled err = %v", err)
	}
	if p.Calls() != 4 {
		t.Fatalf("Calls = %d", p.Calls())
	}
}

func TestFakePressureSourceRecordsSnapshots(t *testing.T) {
	p := NewFakePressureSource()
	p.SetValue(0.7)
	s := NewSnapshot().Setup("s1", Label("tier", "a")).Build()
	v, err := p.Pressure(context.Background(), s)
	if v != 0.7 || err != nil {
		t.Fatalf("%v %v", v, err)
	}
	s.Setups[0].Labels["tier"] = "changed"
	seen := p.Snapshots()
	if len(seen) != 1 || seen[0].Setups[0].Labels["tier"] != "a" {
		t.Fatalf("seen = %+v", seen)
	}
	p.SetError(errors.New("e"))
	if _, err := p.Pressure(context.Background(), s); err == nil || p.Calls() != 2 {
		t.Fatalf("err %v calls %d", err, p.Calls())
	}
}

func TestFakePolicyDecisions(t *testing.T) {
	snap := NewSnapshot().Setup("s1", Weight(3)).Setup("s2", Weight(5)).Hold("s1", 1).Hold("s2", 1).Build()
	p := NewFakePolicy()
	d, _ := p.Decide(context.Background(), NewHoldState(snap, 0, hourglass.User{}), snap)
	if !d.Meter || d.Weight != 1 {
		t.Fatalf("default = %+v", d)
	}
	p.SetFunc(func(h hourglass.HoldState, _ hourglass.Snapshot) hourglass.Decision {
		return hourglass.Decision{Meter: h.Setup.Name == "s2", Weight: h.Setup.Weight, Reason: "by setup"}
	})
	d1, _ := p.Decide(context.Background(), NewHoldState(snap, 0, hourglass.User{}), snap)
	d2, _ := p.Decide(context.Background(), NewHoldState(snap, 1, hourglass.User{}), snap)
	if d1.Meter || !d2.Meter || d2.Weight != 5 {
		t.Fatalf("func decisions %+v %+v", d1, d2)
	}
	p.SetError(errors.New("e"))
	if _, err := p.Decide(context.Background(), hourglass.HoldState{}, snap); err == nil {
		t.Fatal("expected error")
	}
	if p.Calls() != 4 {
		t.Fatalf("calls = %d", p.Calls())
	}
}

func TestFakeHookRecordsErrorsAndBlocks(t *testing.T) {
	h := NewFakeHook()
	boom := errors.New("veto")
	h.SetError(hourglass.EventHoldAcquiring, boom)
	for _, typ := range []string{hourglass.EventHoldAcquired, hourglass.EventHoldAcquiring} {
		err := h.Handle(context.Background(), hourglass.Event{Type: typ})
		if (typ == hourglass.EventHoldAcquiring) != errors.Is(err, boom) {
			t.Fatalf("%s: err = %v", typ, err)
		}
	}
	ev := h.Events()
	if len(ev) != 2 || ev[0].Type != hourglass.EventHoldAcquired || len(h.EventsOfType(hourglass.EventHoldAcquiring)) != 1 {
		t.Fatalf("events = %+v", ev)
	}
	h.SetError(hourglass.EventHoldAcquiring, nil)
	if err := h.Handle(context.Background(), hourglass.Event{Type: hourglass.EventHoldAcquiring}); err != nil {
		t.Fatal(err)
	}

	gate := make(chan struct{})
	h.SetGate(gate)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := h.Handle(ctx, hourglass.Event{Type: hourglass.EventPollFailed}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked hook err = %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- h.Handle(context.Background(), hourglass.Event{Type: hourglass.EventPollFailed}) }()
	close(gate)
	if err := <-done; err != nil {
		t.Fatalf("released hook err = %v", err)
	}
}

func TestFakesConcurrentUse(t *testing.T) {
	p, ps, pol, h := NewFakeProvider(), NewFakePressureSource(), NewFakePolicy(), NewFakeHook()
	snap := NewSnapshot().Setup("s1").Hold("s1", 1).Build()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				p.SetAllocations(hourglass.Allocation{ResourceName: "r"})
				_, _ = p.List(context.Background())
				ps.SetValue(0.5)
				_, _ = ps.Pressure(context.Background(), snap)
				pol.SetName("n")
				_, _ = pol.Decide(context.Background(), hourglass.HoldState{}, snap)
				h.SetName("n")
				_ = h.Handle(context.Background(), hourglass.Event{Type: hourglass.EventUserPending})
				_ = h.Events()
			}
		}()
	}
	wg.Wait()
	if len(h.Events()) != 400 {
		t.Fatalf("events = %d", len(h.Events()))
	}
}

func TestSnapshotBuilder(t *testing.T) {
	at := time.Date(2031, 1, 2, 3, 4, 5, 0, time.UTC)
	s := NewSnapshot().At(at).
		Setup("s1", Label("tier", "a"), Weight(2), Resource("pool-1")).
		Setup("s2").
		Hold("s1", 9, Metering(at), Deployed(at)).
		Hold("s2", 9, Released(at)).
		Allocation("pool-1", "tier", "a").
		ProviderConfigured(false).Build()
	if !s.Time.Equal(at) || s.ProviderConfigured || len(s.Setups) != 2 || len(s.Holds) != 2 {
		t.Fatalf("snapshot = %+v", s)
	}
	if s.Setups[0].Weight != 2 || s.Setups[1].Weight != 1 || s.Setups[1].ResourceName != "s2" {
		t.Fatalf("setups = %+v", s.Setups)
	}
	if s.Holds[0].ID != 1 || s.Holds[1].ID != 2 || s.Holds[0].MeteringStartedAt == nil || s.Holds[1].ReleasedAt == nil {
		t.Fatalf("holds = %+v", s.Holds)
	}
	if a, ok := s.AllocationFor("s1"); !ok || a.Labels["tier"] != "a" {
		t.Fatalf("allocation = %+v %v", a, ok)
	}
	hs := NewHoldState(s, 0, hourglass.User{ID: 9})
	if hs.Setup.Name != "s1" || hs.Allocation == nil || hs.User.ID != 9 {
		t.Fatalf("holdstate = %+v", hs)
	}
}
