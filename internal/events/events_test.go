// SPDX-License-Identifier: MIT

package events

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

var quiet = WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

type rec struct {
	ID       int64
	Type     string
	Hook     string
	Payload  string
	OK       *bool
	Err      string
	Start    time.Time
	Finished time.Time
}

type memRec struct {
	mu   sync.Mutex
	rows []*rec
}

func (m *memRec) InsertEventDelivery(_ context.Context, et, hook, payload string, started time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := &rec{ID: int64(len(m.rows) + 1), Type: et, Hook: hook, Payload: payload, Start: started}
	m.rows = append(m.rows, r)
	return r.ID, nil
}

func (m *memRec) FinishEventDelivery(_ context.Context, id int64, fin time.Time, ok bool, msg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.rows[id-1]
	r.OK, r.Err, r.Finished = &ok, msg, fin
	return nil
}

func (m *memRec) all() []rec {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []rec
	for _, r := range m.rows {
		out = append(out, *r)
	}
	return out
}

// funcHook is a hook backed by a function, with a shared call log.
type funcHook struct {
	name string
	fn   func(ctx context.Context, e hourglass.Event) error
	log  *callLog
}

func (h *funcHook) Name() string { return h.name }
func (h *funcHook) Handle(ctx context.Context, e hourglass.Event) error {
	if h.log != nil {
		h.log.add(h.name + ":" + e.Type + ":" + dataID(e))
	}
	if h.fn == nil {
		return nil
	}
	return h.fn(ctx, e)
}

func dataID(e hourglass.Event) string {
	if s, ok := e.Data["id"].(string); ok {
		return s
	}
	return ""
}

type callLog struct {
	mu sync.Mutex
	s  []string
}

func (c *callLog) add(x string) { c.mu.Lock(); c.s = append(c.s, x); c.mu.Unlock() }
func (c *callLog) get() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.s...)
}

func ev(t, id string) hourglass.Event {
	return hourglass.Event{Type: t, Time: hourglasstest.DefaultTime, Data: map[string]any{"id": id}}
}

func started(t *testing.T, d *Dispatcher) {
	t.Helper()
	d.Start(context.Background())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = d.Close(ctx)
	})
}

func flush(t *testing.T, d *Dispatcher) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := d.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestEXT6_OrderAcrossEventsAndHooks(t *testing.T) {
	l := &callLog{}
	hs := []hourglass.Hook{&funcHook{name: "a", log: l}, &funcHook{name: "b", log: l}}
	d := New(hs, &memRec{}, quiet)
	started(t, d)
	d.Emit(ev(hourglass.EventHoldAcquired, "1"))
	d.Emit(ev(hourglass.EventHoldReleased, "2"))
	d.Emit(ev(hourglass.EventBalanceLow, "3"))
	flush(t, d)
	eq(t, l.get(), []string{
		"a:hold.acquired:1", "b:hold.acquired:1",
		"a:hold.released:2", "b:hold.released:2",
		"a:balance.low:3", "b:balance.low:3",
	})
}

func TestEXT6_SlowHookDoesNotBlockEmitNorReorder(t *testing.T) {
	l := &callLog{}
	gate := make(chan struct{})
	slow := &funcHook{name: "slow", log: l, fn: func(ctx context.Context, e hourglass.Event) error {
		if dataID(e) == "1" {
			<-gate
		}
		return nil
	}}
	d := New([]hourglass.Hook{slow}, &memRec{}, quiet)
	started(t, d)
	begin := time.Now()
	for _, id := range []string{"1", "2", "3"} {
		d.Emit(ev(hourglass.EventHoldAcquired, id))
	}
	if el := time.Since(begin); el > 100*time.Millisecond {
		t.Fatalf("Emit blocked for %v", el)
	}
	if d.Pending() == 0 {
		t.Fatal("expected pending events")
	}
	close(gate)
	flush(t, d)
	eq(t, l.get(), []string{"slow:hold.acquired:1", "slow:hold.acquired:2", "slow:hold.acquired:3"})
	if d.Pending() != 0 {
		t.Fatalf("pending %d", d.Pending())
	}
}

func TestEXT6_TimeoutIgnoringCtxRecordedNextHookRuns(t *testing.T) {
	l := &callLog{}
	block := make(chan struct{})
	defer close(block)
	stuck := &funcHook{name: "stuck", log: l, fn: func(context.Context, hourglass.Event) error { <-block; return nil }}
	next := &funcHook{name: "next", log: l}
	r := &memRec{}
	d := New([]hourglass.Hook{stuck, next}, r, quiet, WithTimeout(50*time.Millisecond))
	started(t, d)
	d.Emit(ev(hourglass.EventHoldAcquired, "1"))
	d.Emit(ev(hourglass.EventHoldReleased, "2"))
	flush(t, d)
	eq(t, l.get(), []string{"stuck:hold.acquired:1", "next:hold.acquired:1", "stuck:hold.released:2", "next:hold.released:2"})
	rows := r.all()
	if len(rows) != 4 {
		t.Fatalf("rows %d", len(rows))
	}
	if rows[0].OK == nil || *rows[0].OK || rows[0].Err != "hook stuck timed out" {
		t.Fatalf("row0 %+v", rows[0])
	}
	if rows[1].OK == nil || !*rows[1].OK {
		t.Fatalf("row1 %+v", rows[1])
	}
}

func TestEXT6_PanicRecovered(t *testing.T) {
	l := &callLog{}
	boom := &funcHook{name: "boom", log: l, fn: func(context.Context, hourglass.Event) error { panic("kaput") }}
	after := &funcHook{name: "after", log: l}
	r := &memRec{}
	d := New([]hourglass.Hook{boom, after}, r, quiet)
	started(t, d)
	d.Emit(ev(hourglass.EventHoldAcquired, "1"))
	d.Emit(ev(hourglass.EventHoldReleased, "2"))
	flush(t, d)
	eq(t, l.get(), []string{"boom:hold.acquired:1", "after:hold.acquired:1", "boom:hold.released:2", "after:hold.released:2"})
	rows := r.all()
	if *rows[0].OK || rows[0].Err == "" {
		t.Fatalf("panic not recorded: %+v", rows[0])
	}
}

func TestEXT6_VetoStopsAtFirstErrorReturnsMessage(t *testing.T) {
	l := &callLog{}
	a := &funcHook{name: "a", log: l}
	b := &funcHook{name: "b", log: l, fn: func(context.Context, hourglass.Event) error { return errors.New("no capacity today") }}
	c := &funcHook{name: "c", log: l}
	r := &memRec{}
	d := New([]hourglass.Hook{a, b, c}, r, quiet)
	err := d.Veto(context.Background(), ev(hourglass.EventHoldAcquiring, "1"))
	var ve *VetoError
	if !errors.As(err, &ve) || ve.Hook != "b" || err.Error() != "no capacity today" {
		t.Fatalf("err %v", err)
	}
	eq(t, l.get(), []string{"a:hold.acquiring:1", "b:hold.acquiring:1"})
	rows := r.all()
	if len(rows) != 2 || !*rows[0].OK || *rows[1].OK || rows[1].Err != "no capacity today" {
		t.Fatalf("rows %+v", rows)
	}
}

func TestEXT6_VetoAllOKReturnsNil(t *testing.T) {
	d := New([]hourglass.Hook{&funcHook{name: "a"}}, &memRec{}, quiet)
	if err := d.Veto(context.Background(), ev(hourglass.EventHoldAcquiring, "1")); err != nil {
		t.Fatal(err)
	}
}

func TestEXT6_VetoTimeout(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	h := &funcHook{name: "slow", fn: func(context.Context, hourglass.Event) error { <-block; return nil }}
	r := &memRec{}
	d := New([]hourglass.Hook{h}, r, quiet, WithTimeout(50*time.Millisecond))
	err := d.Veto(context.Background(), ev(hourglass.EventHoldAcquiring, "1"))
	var ve *VetoError
	if !errors.As(err, &ve) || err.Error() != "hook slow timed out" {
		t.Fatalf("err %v", err)
	}
	if rows := r.all(); len(rows) != 1 || *rows[0].OK || rows[0].Err != "hook slow timed out" {
		t.Fatalf("rows %+v", rows)
	}
}

func TestEXT6_VetoHonoursCtxAwareHookTimeout(t *testing.T) {
	h := hourglasstest.NewFakeHook()
	h.SetGate(make(chan struct{}))
	d := New([]hourglass.Hook{h}, nil, quiet, WithTimeout(30*time.Millisecond))
	err := d.Veto(context.Background(), ev(hourglass.EventHoldAcquiring, "1"))
	if err == nil || err.Error() != "hook fake-hook timed out" {
		t.Fatalf("err %v", err)
	}
}

func TestEXT6_WrongEntryPointNotDelivered(t *testing.T) {
	h := hourglasstest.NewFakeHook()
	r := &memRec{}
	d := New([]hourglass.Hook{h}, r, quiet)
	started(t, d)
	d.Emit(ev(hourglass.EventHoldAcquiring, "1"))
	if err := d.Veto(context.Background(), ev(hourglass.EventHoldAcquired, "2")); err == nil {
		t.Fatal("expected error")
	}
	flush(t, d)
	if len(h.Events()) != 0 || len(r.all()) != 0 {
		t.Fatal("delivered despite programming error")
	}
}

func TestEXT6_RecordsOKAndError(t *testing.T) {
	clock := hourglasstest.NewFakeClock(time.Time{})
	good := hourglasstest.NewFakeHook()
	good.SetName("good")
	bad := hourglasstest.NewFakeHook()
	bad.SetName("bad")
	bad.SetError(hourglass.EventBalanceLow, errors.New("webhook 500"))
	r := &memRec{}
	d := New([]hourglass.Hook{good, bad}, r, quiet, WithClock(clock))
	started(t, d)
	e := ev(hourglass.EventBalanceLow, "x")
	d.Emit(e)
	flush(t, d)
	rows := r.all()
	if len(rows) != 2 {
		t.Fatalf("rows %d", len(rows))
	}
	if rows[0].Hook != "good" || rows[0].Type != "balance.low" || !*rows[0].OK || rows[0].Err != "" {
		t.Fatalf("%+v", rows[0])
	}
	if rows[1].Hook != "bad" || *rows[1].OK || rows[1].Err != "webhook 500" {
		t.Fatalf("%+v", rows[1])
	}
	if rows[0].Payload == "" || rows[0].Payload == "{}" || rows[0].Start != clock.Now() || rows[0].Finished.IsZero() {
		t.Fatalf("%+v", rows[0])
	}
}

func TestEXT7_NoRetries(t *testing.T) {
	h := hourglasstest.NewFakeHook()
	h.SetError(hourglass.EventHoldReleased, errors.New("fail"))
	d := New([]hourglass.Hook{h}, &memRec{}, quiet)
	started(t, d)
	d.Emit(ev(hourglass.EventHoldReleased, "1"))
	flush(t, d)
	time.Sleep(50 * time.Millisecond)
	if n := len(h.Events()); n != 1 {
		t.Fatalf("calls %d", n)
	}
}

func TestEXT7_ClosePendingDropped(t *testing.T) {
	gate := make(chan struct{})
	l := &callLog{}
	h := &funcHook{name: "h", log: l, fn: func(context.Context, hourglass.Event) error { <-gate; return nil }}
	d := New([]hourglass.Hook{h}, &memRec{}, quiet)
	d.Start(context.Background())
	for _, id := range []string{"1", "2", "3"} {
		d.Emit(ev(hourglass.EventHoldAcquired, id))
	}
	for i := 0; i < 200 && len(l.get()) == 0; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	go func() { time.Sleep(30 * time.Millisecond); close(gate) }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := d.Close(ctx); err != nil {
		t.Fatal(err)
	}
	eq(t, l.get(), []string{"h:hold.acquired:1"})
	d.Emit(ev(hourglass.EventHoldAcquired, "4"))
	if d.Pending() != 0 {
		t.Fatalf("pending %d", d.Pending())
	}
}

func TestEXT7_CloseBoundedByCtx(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	h := &funcHook{name: "h", fn: func(context.Context, hourglass.Event) error { <-block; return nil }}
	d := New([]hourglass.Hook{h}, nil, quiet)
	d.Start(context.Background())
	d.Emit(ev(hourglass.EventHoldAcquired, "1"))
	time.Sleep(20 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := d.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err %v", err)
	}
}

func TestEXT6_ZeroHooksNoop(t *testing.T) {
	r := &memRec{}
	d := New(nil, r, quiet)
	started(t, d)
	d.Emit(ev(hourglass.EventHoldAcquired, "1"))
	if d.Pending() != 0 {
		t.Fatal("pending with zero hooks")
	}
	if err := d.Veto(context.Background(), ev(hourglass.EventHoldAcquiring, "1")); err != nil {
		t.Fatal(err)
	}
	flush(t, d)
	if len(r.all()) != 0 {
		t.Fatal("recorded with zero hooks")
	}
}

func TestEXT7_StoreSatisfiesRecorder(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var _ Recorder = st
	h := hourglasstest.NewFakeHook()
	h.SetError(hourglass.EventPollFailed, errors.New("nope"))
	d := New([]hourglass.Hook{h}, st, quiet)
	started(t, d)
	d.Emit(ev(hourglass.EventPollFailed, "1"))
	flush(t, d)
	rows, err := st.ListEventDeliveries(context.Background(), 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("%v %v", rows, err)
	}
	if rows[0].OK == nil || *rows[0].OK || rows[0].Error != "nope" || rows[0].FinishedAt == nil {
		t.Fatalf("%+v", rows[0])
	}
}
