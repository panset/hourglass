// SPDX-License-Identifier: MIT

// Package events delivers hourglass.Events to the configured hooks
// (EXT-6, EXT-7). hold.acquiring is delivered synchronously through Veto;
// every other event goes through one in-memory, in-order background queue
// through Emit. Delivery is at most once and nothing is retried.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/panset/hourglass/pkg/hourglass"
)

// DefaultTimeout is the per-call timeout for a hook (EXT-6).
const DefaultTimeout = 30 * time.Second

// Recorder persists delivery outcomes (the event_deliveries table).
// *store.Store satisfies it.
type Recorder interface {
	InsertEventDelivery(ctx context.Context, eventType, hook, payloadJSON string, started time.Time) (int64, error)
	FinishEventDelivery(ctx context.Context, id int64, finished time.Time, ok bool, errMsg string) error
}

// Clock supplies the time used for delivery records.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// VetoError is returned by Veto when a hook rejects or times out.
type VetoError struct {
	Hook string
	Err  error
}

// Error is the hook's own message.
func (e *VetoError) Error() string { return e.Err.Error() }

// Unwrap returns the hook's error.
func (e *VetoError) Unwrap() error { return e.Err }

// Option configures a Dispatcher.
type Option func(*Dispatcher)

// WithTimeout sets the per-call hook timeout. Non-positive values are ignored.
func WithTimeout(d time.Duration) Option {
	return func(x *Dispatcher) {
		if d > 0 {
			x.timeout = d
		}
	}
}

// WithClock sets the clock used for record times.
func WithClock(c Clock) Option {
	return func(x *Dispatcher) {
		if c != nil {
			x.clock = c
		}
	}
}

// WithLogger sets the logger.
func WithLogger(l *slog.Logger) Option {
	return func(x *Dispatcher) {
		if l != nil {
			x.log = l
		}
	}
}

// Dispatcher delivers events to hooks in config order.
type Dispatcher struct {
	hooks   []hourglass.Hook
	rec     Recorder
	timeout time.Duration
	clock   Clock
	log     *slog.Logger

	mu       sync.Mutex
	queue    []hourglass.Event
	inflight bool
	started  bool
	closed   bool
	changed  chan struct{} // closed and replaced on every queue state change
	wake     chan struct{} // buffered 1: tells the worker there is work
	cancel   context.CancelFunc
	done     chan struct{} // closed when the worker exits
}

// New returns a Dispatcher over hooks (in config order). rec may be nil.
func New(hooks []hourglass.Hook, rec Recorder, opts ...Option) *Dispatcher {
	d := &Dispatcher{
		hooks:   append([]hourglass.Hook(nil), hooks...),
		rec:     rec,
		timeout: DefaultTimeout,
		clock:   systemClock{},
		log:     slog.Default(),
		changed: make(chan struct{}),
		wake:    make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	for _, o := range opts {
		o(d)
	}
	return d
}

// Veto delivers a vetoable event inside the caller's request. The first hook
// error or timeout stops delivery and is returned as *VetoError.
func (d *Dispatcher) Veto(ctx context.Context, e hourglass.Event) error {
	if !hourglass.Vetoable(e.Type) {
		return fmt.Errorf("events: Veto called with non-vetoable event %q", e.Type)
	}
	for _, h := range d.hooks {
		if err := d.deliver(ctx, h, e); err != nil {
			return &VetoError{Hook: h.Name(), Err: err}
		}
	}
	return nil
}

// Emit queues a non-vetoable event for background delivery. It never blocks.
// A vetoable event is a programming error: it is logged and not delivered.
func (d *Dispatcher) Emit(e hourglass.Event) {
	if hourglass.Vetoable(e.Type) {
		d.log.Error("events: Emit called with vetoable event, not delivered", "type", e.Type)
		return
	}
	if len(d.hooks) == 0 {
		return
	}
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		d.log.Warn("events: emit after close, event dropped", "type", e.Type)
		return
	}
	d.queue = append(d.queue, e)
	d.signalLocked()
	d.mu.Unlock()
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

// Pending returns the number of events queued or being delivered.
func (d *Dispatcher) Pending() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.pendingLocked()
}

func (d *Dispatcher) pendingLocked() int {
	n := len(d.queue)
	if d.inflight {
		n++
	}
	return n
}

func (d *Dispatcher) signalLocked() {
	close(d.changed)
	d.changed = make(chan struct{})
}

// Start launches the delivery goroutine. Events emitted earlier are kept.
// It is a no-op when called again or after Close.
func (d *Dispatcher) Start(ctx context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.started || d.closed {
		return
	}
	d.started = true
	wctx, cancel := context.WithCancel(ctx)
	d.cancel = cancel
	go d.run(wctx)
}

// Flush waits until nothing is queued or in flight, or ctx ends.
func (d *Dispatcher) Flush(ctx context.Context) error {
	for {
		d.mu.Lock()
		if d.pendingLocked() == 0 {
			d.mu.Unlock()
			return nil
		}
		ch := d.changed
		d.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Close stops accepting events, drops those still queued (EXT-7) and waits
// for the in-flight call, bounded by ctx. If ctx ends first the call is
// cancelled and the context error returned.
func (d *Dispatcher) Close(ctx context.Context) error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	dropped := len(d.queue)
	d.queue = nil
	started := d.started
	d.signalLocked()
	d.mu.Unlock()
	if dropped > 0 {
		d.log.Warn("events: pending events dropped at shutdown", "dropped", dropped)
	}
	if !started {
		return nil
	}
	select {
	case d.wake <- struct{}{}:
	default:
	}
	select {
	case <-d.done:
		d.cancel()
		return nil
	case <-ctx.Done():
		d.cancel()
		return ctx.Err()
	}
}

func (d *Dispatcher) run(ctx context.Context) {
	defer close(d.done)
	for {
		d.mu.Lock()
		if d.closed {
			d.mu.Unlock()
			return
		}
		if len(d.queue) == 0 {
			d.mu.Unlock()
			select {
			case <-d.wake:
				continue
			case <-ctx.Done():
				return
			}
		}
		e := d.queue[0]
		d.queue[0] = hourglass.Event{}
		d.queue = d.queue[1:]
		d.inflight = true
		d.signalLocked()
		d.mu.Unlock()

		for _, h := range d.hooks {
			d.mu.Lock()
			stop := d.closed
			d.mu.Unlock()
			if stop || ctx.Err() != nil {
				break
			}
			if err := d.deliver(ctx, h, e); err != nil {
				d.log.Warn("events: hook failed", "hook", h.Name(), "type", e.Type, "error", err)
			}
		}

		d.mu.Lock()
		d.inflight = false
		d.signalLocked()
		d.mu.Unlock()
	}
}

// deliver makes one (event, hook) call with its own timeout, recovers
// panics, records the outcome and returns the hook's error.
func (d *Dispatcher) deliver(ctx context.Context, h hourglass.Hook, e hourglass.Event) error {
	name := h.Name()
	started := d.clock.Now()
	rctx := context.WithoutCancel(ctx)
	var id int64
	var haveID bool
	if d.rec != nil {
		payload, merr := json.Marshal(e)
		if merr != nil {
			d.log.Error("events: marshal payload", "type", e.Type, "error", merr)
			payload = []byte("{}")
		}
		var ierr error
		id, ierr = d.rec.InsertEventDelivery(rctx, e.Type, name, string(payload), started)
		if ierr != nil {
			d.log.Error("events: record delivery start", "hook", name, "type", e.Type, "error", ierr)
		} else {
			haveID = true
		}
	}

	cctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	res := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				res <- fmt.Errorf("hook %s panicked: %v", name, r)
			}
		}()
		res <- h.Handle(cctx, e)
	}()
	var err error
	select {
	case err = <-res:
		if err != nil && errors.Is(err, context.DeadlineExceeded) && cctx.Err() != nil {
			err = fmt.Errorf("hook %s timed out", name)
		}
	case <-cctx.Done():
		if errors.Is(cctx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("hook %s timed out", name)
		} else {
			err = cctx.Err()
		}
	}

	if haveID {
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		if ferr := d.rec.FinishEventDelivery(rctx, id, d.clock.Now(), err == nil, msg); ferr != nil {
			d.log.Error("events: record delivery outcome", "hook", name, "type", e.Type, "error", ferr)
		}
	}
	return err
}
