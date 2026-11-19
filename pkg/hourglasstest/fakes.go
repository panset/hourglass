// SPDX-License-Identifier: MIT

package hourglasstest

import (
	"context"
	"sync"

	"github.com/panset/hourglass/pkg/hourglass"
)

// FakeProvider is a hourglass.ResourceProvider with settable allocations and
// error. It returns ctx.Err() when the context is already done.
type FakeProvider struct {
	mu          sync.Mutex
	name        string
	allocations []hourglass.Allocation
	err         error
	calls       int
}

// NewFakeProvider returns a provider named "fake-provider" with no allocations.
func NewFakeProvider() *FakeProvider { return &FakeProvider{name: "fake-provider"} }

// SetName changes the value returned by Name.
func (p *FakeProvider) SetName(n string) { p.mu.Lock(); p.name = n; p.mu.Unlock() }

// SetAllocations replaces the allocations List returns.
func (p *FakeProvider) SetAllocations(a ...hourglass.Allocation) {
	p.mu.Lock()
	p.allocations = cloneAllocations(a)
	p.mu.Unlock()
}

// SetError makes List fail with err; nil clears it.
func (p *FakeProvider) SetError(err error) { p.mu.Lock(); p.err = err; p.mu.Unlock() }

// Calls reports how many times List was called.
func (p *FakeProvider) Calls() int { p.mu.Lock(); defer p.mu.Unlock(); return p.calls }

// Name implements hourglass.ResourceProvider.
func (p *FakeProvider) Name() string { p.mu.Lock(); defer p.mu.Unlock(); return p.name }

// List implements hourglass.ResourceProvider.
func (p *FakeProvider) List(ctx context.Context) ([]hourglass.Allocation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.err != nil {
		return nil, p.err
	}
	return cloneAllocations(p.allocations), nil
}

// FakePressureSource is a hourglass.PressureSource with a settable value and
// error that records every snapshot it was given.
type FakePressureSource struct {
	mu    sync.Mutex
	name  string
	value float64
	err   error
	seen  []hourglass.Snapshot
}

// NewFakePressureSource returns a source named "fake-pressure" reporting 0.
func NewFakePressureSource() *FakePressureSource {
	return &FakePressureSource{name: "fake-pressure"}
}

// SetName changes the value returned by Name.
func (p *FakePressureSource) SetName(n string) { p.mu.Lock(); p.name = n; p.mu.Unlock() }

// SetValue sets the pressure returned (not clamped, so tests can probe callers).
func (p *FakePressureSource) SetValue(v float64) { p.mu.Lock(); p.value = v; p.mu.Unlock() }

// SetError makes Pressure fail with err; nil clears it.
func (p *FakePressureSource) SetError(err error) { p.mu.Lock(); p.err = err; p.mu.Unlock() }

// Snapshots returns copies of the snapshots seen, oldest first.
func (p *FakePressureSource) Snapshots() []hourglass.Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]hourglass.Snapshot, len(p.seen))
	for i, s := range p.seen {
		out[i] = cloneSnapshot(s)
	}
	return out
}

// Calls reports how many times Pressure was called.
func (p *FakePressureSource) Calls() int { p.mu.Lock(); defer p.mu.Unlock(); return len(p.seen) }

// Name implements hourglass.PressureSource.
func (p *FakePressureSource) Name() string { p.mu.Lock(); defer p.mu.Unlock(); return p.name }

// Pressure implements hourglass.PressureSource.
func (p *FakePressureSource) Pressure(ctx context.Context, s hourglass.Snapshot) (float64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen = append(p.seen, cloneSnapshot(s))
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if p.err != nil {
		return 0, p.err
	}
	return p.value, nil
}

// FakePolicy is a hourglass.MeteringPolicy. The decision for each hold comes
// from the function set with SetFunc, or else from the default decision.
type FakePolicy struct {
	mu    sync.Mutex
	name  string
	def   hourglass.Decision
	fn    func(hourglass.HoldState, hourglass.Snapshot) hourglass.Decision
	err   error
	calls int
}

// NewFakePolicy returns a policy named "fake-policy" whose default decision
// is {Meter: true, Weight: 1, Reason: "default"}.
func NewFakePolicy() *FakePolicy {
	return &FakePolicy{name: "fake-policy", def: hourglass.Decision{Meter: true, Weight: 1, Reason: "default"}}
}

// SetName changes the value returned by Name.
func (p *FakePolicy) SetName(n string) { p.mu.Lock(); p.name = n; p.mu.Unlock() }

// SetDefault sets the decision used when no function is set.
func (p *FakePolicy) SetDefault(d hourglass.Decision) { p.mu.Lock(); p.def = d; p.mu.Unlock() }

// SetFunc sets a per-hold decision function; nil restores the default.
func (p *FakePolicy) SetFunc(f func(hourglass.HoldState, hourglass.Snapshot) hourglass.Decision) {
	p.mu.Lock()
	p.fn = f
	p.mu.Unlock()
}

// SetError makes Decide fail with err; nil clears it.
func (p *FakePolicy) SetError(err error) { p.mu.Lock(); p.err = err; p.mu.Unlock() }

// Calls reports how many times Decide was called.
func (p *FakePolicy) Calls() int { p.mu.Lock(); defer p.mu.Unlock(); return p.calls }

// Name implements hourglass.MeteringPolicy.
func (p *FakePolicy) Name() string { p.mu.Lock(); defer p.mu.Unlock(); return p.name }

// Decide implements hourglass.MeteringPolicy. The decision function runs
// without the fake's lock held, so it may call back into the fake.
func (p *FakePolicy) Decide(ctx context.Context, h hourglass.HoldState, s hourglass.Snapshot) (hourglass.Decision, error) {
	p.mu.Lock()
	p.calls++
	fn, def, err := p.fn, p.def, p.err
	p.mu.Unlock()
	if cerr := ctx.Err(); cerr != nil {
		return hourglass.Decision{}, cerr
	}
	if err != nil {
		return hourglass.Decision{}, err
	}
	if fn != nil {
		return fn(h, s), nil
	}
	return def, nil
}

// FakeHook is a hourglass.Hook that records events in order.
type FakeHook struct {
	mu     sync.Mutex
	name   string
	events []hourglass.Event
	errs   map[string]error
	gate   <-chan struct{}
}

// NewFakeHook returns a hook named "fake-hook".
func NewFakeHook() *FakeHook { return &FakeHook{name: "fake-hook", errs: map[string]error{}} }

// SetName changes the value returned by Name.
func (h *FakeHook) SetName(n string) { h.mu.Lock(); h.name = n; h.mu.Unlock() }

// SetError makes Handle fail for events of eventType; nil clears it.
func (h *FakeHook) SetError(eventType string, err error) {
	h.mu.Lock()
	if err == nil {
		delete(h.errs, eventType)
	} else {
		h.errs[eventType] = err
	}
	h.mu.Unlock()
}

// SetGate makes Handle block, after recording the event, until gate is
// closed or sent to, or the call's context ends (then it returns ctx.Err()).
// A nil gate removes blocking. Use it to test timeouts and slow hooks.
func (h *FakeHook) SetGate(gate <-chan struct{}) { h.mu.Lock(); h.gate = gate; h.mu.Unlock() }

// Events returns the recorded events, oldest first.
func (h *FakeHook) Events() []hourglass.Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]hourglass.Event(nil), h.events...)
}

// EventsOfType returns the recorded events with the given type.
func (h *FakeHook) EventsOfType(eventType string) []hourglass.Event {
	var out []hourglass.Event
	for _, e := range h.Events() {
		if e.Type == eventType {
			out = append(out, e)
		}
	}
	return out
}

// Name implements hourglass.Hook.
func (h *FakeHook) Name() string { h.mu.Lock(); defer h.mu.Unlock(); return h.name }

// Handle implements hourglass.Hook.
func (h *FakeHook) Handle(ctx context.Context, e hourglass.Event) error {
	h.mu.Lock()
	h.events = append(h.events, e)
	gate, err := h.gate, h.errs[e.Type]
	h.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}
