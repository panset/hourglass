// SPDX-License-Identifier: MIT

package hourglass

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeProvider struct{ cfg json.RawMessage }

func (fakeProvider) Name() string                               { return "fake" }
func (fakeProvider) List(context.Context) ([]Allocation, error) { return nil, nil }

type fakePressure struct{}

func (fakePressure) Name() string                                        { return "fake" }
func (fakePressure) Pressure(context.Context, Snapshot) (float64, error) { return 0, nil }

type fakePolicy struct{}

func (fakePolicy) Name() string { return "fake" }
func (fakePolicy) Decide(context.Context, HoldState, Snapshot) (Decision, error) {
	return Decision{}, nil
}

type fakeHook struct{}

func (fakeHook) Name() string                        { return "fake" }
func (fakeHook) Handle(context.Context, Event) error { return nil }

func provF(config json.RawMessage) (ResourceProvider, error) { return fakeProvider{cfg: config}, nil }
func presF(json.RawMessage) (PressureSource, error)          { return fakePressure{}, nil }
func polF(json.RawMessage) (MeteringPolicy, error)           { return fakePolicy{}, nil }
func hookF(json.RawMessage) (Hook, error)                    { return fakeHook{}, nil }

// clean resets the registries before and after a test so registrations
// never leak between cases.
func clean(t *testing.T) {
	t.Helper()
	resetRegistries()
	t.Cleanup(resetRegistries)
}

// kinds drives the same registry checks over all four extension kinds.
var kinds = []struct {
	name     string
	register func(string)
	regNil   func(string)
	build    func(string, json.RawMessage) (any, error)
	list     func() []string
}{
	{"provider",
		func(n string) { RegisterProvider(n, provF) },
		func(n string) { RegisterProvider(n, nil) },
		func(n string, c json.RawMessage) (any, error) { return NewProvider(n, c) },
		Providers},
	{"pressure source",
		func(n string) { RegisterPressureSource(n, presF) },
		func(n string) { RegisterPressureSource(n, nil) },
		func(n string, c json.RawMessage) (any, error) { return NewPressureSource(n, c) },
		PressureSources},
	{"metering policy",
		func(n string) { RegisterMeteringPolicy(n, polF) },
		func(n string) { RegisterMeteringPolicy(n, nil) },
		func(n string, c json.RawMessage) (any, error) { return NewMeteringPolicy(n, c) },
		MeteringPolicies},
	{"hook",
		func(n string) { RegisterHook(n, hookF) },
		func(n string) { RegisterHook(n, nil) },
		func(n string, c json.RawMessage) (any, error) { return NewHook(n, c) },
		Hooks},
}

func mustPanic(t *testing.T, want string, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		r := recover()
		if r == nil {
			t.Fatalf("no panic, want one containing %q", want)
		}
		if !strings.Contains(fmt.Sprint(r), want) {
			t.Fatalf("panic %q, want it to contain %q", r, want)
		}
	}()
	f()
}

func TestEXT1_RegisterAndBuild(t *testing.T) {
	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			clean(t)
			k.register("b")
			k.register("a")
			if got, want := k.list(), []string{"a", "b"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("list = %v, want %v", got, want)
			}
			for _, cfg := range []json.RawMessage{nil, {}, json.RawMessage(`{"x":1}`)} {
				v, err := k.build("a", cfg)
				if err != nil || v == nil {
					t.Fatalf("build(a, %q) = %v, %v", cfg, v, err)
				}
			}
			// Other kinds are unaffected.
			for _, o := range kinds {
				if o.name != k.name && len(o.list()) != 0 {
					t.Errorf("%s registry not empty: %v", o.name, o.list())
				}
			}
		})
	}
}

func TestEXT1_ConfigPassedToFactory(t *testing.T) {
	clean(t)
	RegisterProvider("p", provF)
	cfg := json.RawMessage(`{"url":"x"}`)
	p, err := NewProvider("p", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(p.(fakeProvider).cfg); got != string(cfg) {
		t.Fatalf("factory got %q, want %q", got, cfg)
	}
}

func TestEXT1_FactoryErrorReturned(t *testing.T) {
	clean(t)
	boom := errors.New("bad config")
	RegisterHook("h", func(json.RawMessage) (Hook, error) { return nil, boom })
	if _, err := NewHook("h", nil); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

func TestEXT1_RegisterPanics(t *testing.T) {
	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			clean(t)
			mustPanic(t, "empty name", func() { k.register("") })
			mustPanic(t, "nil factory", func() { k.regNil("x") })
			k.register("x")
			mustPanic(t, "twice", func() { k.register("x") })
			if got := k.list(); !reflect.DeepEqual(got, []string{"x"}) {
				t.Fatalf("list = %v, want [x]", got)
			}
		})
	}
}

func TestEXT2_UnknownNameListsRegistered(t *testing.T) {
	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			clean(t)
			_, err := k.build("nope", nil)
			if err == nil || !strings.Contains(err.Error(), "registered: none") {
				t.Fatalf("empty registry err = %v", err)
			}
			k.register("zeta")
			k.register("alpha")
			k.register("mid")
			_, err = k.build("nope", nil)
			if err == nil {
				t.Fatal("no error for unknown name")
			}
			want := fmt.Sprintf(`hourglass: unknown %s "nope" (registered: alpha, mid, zeta)`, k.name)
			if err.Error() != want {
				t.Fatalf("err = %q, want %q", err, want)
			}
		})
	}
}

func TestEXT1_ListingIsSortedCopy(t *testing.T) {
	clean(t)
	for _, n := range []string{"c", "a", "b"} {
		RegisterProvider(n, provF)
	}
	got := Providers()
	got[0] = "mutated"
	if again := Providers(); !reflect.DeepEqual(again, []string{"a", "b", "c"}) {
		t.Fatalf("Providers() = %v", again)
	}
}

func TestEXT1_ConcurrentRegisterAndLookup(t *testing.T) {
	clean(t)
	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			for _, k := range kinds {
				k.register(fmt.Sprintf("ext%02d", i))
			}
		}(i)
		go func(i int) {
			defer wg.Done()
			for _, k := range kinds {
				_ = k.list()
				_, _ = k.build(fmt.Sprintf("ext%02d", i), nil)
			}
		}(i)
	}
	wg.Wait()
	for _, k := range kinds {
		if got := len(k.list()); got != n {
			t.Errorf("%s: %d registered, want %d", k.name, got, n)
		}
	}
}

// TestEXT6_EventTypesMatchSpecTable checks the list against the spec's
// event table, in table order.
func TestEXT6_EventTypesMatchSpecTable(t *testing.T) {
	table := []struct {
		name     string
		vetoable bool
	}{
		{"hold.acquiring", true},
		{"hold.acquired", false},
		{"hold.metering_started", false},
		{"hold.deployed", false},
		{"hold.released", false},
		{"balance.low", false},
		{"balance.overdrawn", false},
		{"user.pending", false},
		{"user.approved", false},
		{"period.closed", false},
		{"poll.failed", false},
	}
	got := EventTypes()
	if len(got) != len(table) {
		t.Fatalf("EventTypes() has %d entries, want %d", len(got), len(table))
	}
	for i, row := range table {
		if got[i] != row.name {
			t.Errorf("EventTypes()[%d] = %q, want %q", i, got[i], row.name)
		}
		if Vetoable(row.name) != row.vetoable {
			t.Errorf("Vetoable(%q) = %v, want %v", row.name, !row.vetoable, row.vetoable)
		}
	}
	if Vetoable("unknown.event") {
		t.Error("Vetoable(unknown) = true")
	}
	got[0] = "mutated"
	if EventTypes()[0] != EventHoldAcquiring {
		t.Error("EventTypes() returns a shared slice")
	}
}

func TestEXT1_SnapshotHelpers(t *testing.T) {
	s := Snapshot{
		Time: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC),
		Setups: []SetupState{
			{Setup: Setup{Name: "alpha", Labels: map[string]string{"tier": "a"}}, Weight: 2},
			{Setup: Setup{Name: "beta"}, Weight: 1},
			{Setup: Setup{Name: "gamma"}, Weight: 1},
		},
		Allocations: []Allocation{
			{ResourceName: "alpha", Labels: map[string]string{"k": "v"}},
			{ResourceName: "pool-beta"},
			{ResourceName: "stray"},
		},
		ProviderConfigured: true,
	}

	st, ok := s.SetupByName("alpha")
	if !ok || st.Weight != 2 || st.Labels["tier"] != "a" {
		t.Fatalf("SetupByName(alpha) = %+v, %v", st, ok)
	}
	if _, ok := s.SetupByName("missing"); ok {
		t.Error("SetupByName(missing) found something")
	}

	a, ok := s.AllocationFor("alpha")
	if !ok || a.ResourceName != "alpha" || a.Labels["k"] != "v" {
		t.Fatalf("AllocationFor(alpha) = %+v, %v", a, ok)
	}
	for _, name := range []string{"beta", "gamma", "missing", "stray"} {
		if a, ok := s.AllocationFor(name); ok {
			t.Errorf("AllocationFor(%s) = %+v, want none", name, a)
		}
	}

	var empty Snapshot
	if _, ok := empty.AllocationFor("alpha"); ok {
		t.Error("empty snapshot returned an allocation")
	}
}

func TestEXT5_RunWithoutServerImportNamesThePackage(t *testing.T) {
	// This test binary does not link internal/app, so Run has no implementation.
	err := Run(context.Background(), Options{})
	if err == nil || !strings.Contains(err.Error(), `"github.com/panset/hourglass/pkg/hourglass/server"`) {
		t.Fatalf("err = %v", err)
	}
}
