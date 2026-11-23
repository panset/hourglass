// SPDX-License-Identifier: MIT

package collector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// LED-10 in the collector: compaction runs in bounded steps after the poll,
// each step holding runMu alone, at most compactSteps steps per cycle; a
// backlog is worked off over later cycles; a failed step fails no poll and is
// retried on the next cycle; once nothing is due it is checked once a day.

// compactLedger is the real ledger service with CompactDue counted, and
// optionally failed or preceded by a hook.
type compactLedger struct {
	*ledger.Service
	mu     sync.Mutex
	calls  int
	fail   error
	before func(n int) // called with the 0-based call number, under runMu
}

func (l *compactLedger) CompactDue(ctx context.Context) (ledger.CompactionStep, error) {
	l.mu.Lock()
	n, before := l.calls, l.before
	l.calls++
	l.mu.Unlock()
	if before != nil {
		before(n)
	}
	l.mu.Lock()
	fail := l.fail
	l.mu.Unlock()
	if fail != nil {
		return ledger.CompactionStep{}, fail
	}
	return l.Service.CompactDue(ctx)
}

func (l *compactLedger) count() int { l.mu.Lock(); defer l.mu.Unlock(); return l.calls }

func (l *compactLedger) setFail(err error) { l.mu.Lock(); l.fail = err; l.mu.Unlock() }

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (w *lockedBuf) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}
func (w *lockedBuf) String() string { w.mu.Lock(); defer w.mu.Unlock(); return w.b.String() }

// runner drives Collector.Run one cycle at a time: a cycle is the poll, the
// prune, the compaction and the wait, which ends when Run asks for the wait.
type runner struct {
	t      *testing.T
	e      *env
	c      *Collector
	led    *compactLedger
	logs   *lockedBuf
	ticks  chan time.Time
	waits  chan time.Duration
	cancel context.CancelFunc
	done   chan error
	steps  []int // CompactDue calls made in each cycle
	// lastWait is the duration Run asked to wait at the end of the last cycle.
	lastWait time.Duration
}

// startRun starts Run with the given step budget and returns once the first
// cycle has ended.
func (e *env) startRun(stepsPerCycle int, before func(r *runner, n int)) *runner {
	e.t.Helper()
	r := &runner{t: e.t, e: e, logs: &lockedBuf{}, ticks: make(chan time.Time), waits: make(chan time.Duration, 1), done: make(chan error, 1)}
	r.led = &compactLedger{Service: e.led.Service}
	if before != nil {
		r.led.before = func(n int) { before(r, n) }
	}
	r.c = New(Deps{Store: e.st, Pressure: e.pressure, Policy: e.policy, Events: e.rec, Ledger: r.led, Clock: e.clk,
		Logger: slog.New(slog.NewTextHandler(r.logs, nil)),
		After:  func(d time.Duration) <-chan time.Time { return r.afterWait(d) }})
	r.c.compactSteps = stepsPerCycle
	ctx, cancel := context.WithCancel(bg)
	r.cancel = cancel
	go func() { r.done <- r.c.Run(ctx) }()
	e.t.Cleanup(r.stop)
	r.wait()
	return r
}

func (r *runner) afterWait(d time.Duration) <-chan time.Time {
	r.waits <- d
	return r.ticks
}

func (r *runner) wait() {
	r.t.Helper()
	before := 0
	for _, n := range r.steps {
		before += n
	}
	select {
	case r.lastWait = <-r.waits:
	case <-time.After(30 * time.Second):
		r.t.Fatal("cycle did not end")
	}
	r.steps = append(r.steps, r.led.count()-before)
}

// cycle advances the clock by d and runs one more cycle.
func (r *runner) cycle(d time.Duration) {
	r.t.Helper()
	r.e.clk.Advance(d)
	r.ticks <- r.e.clk.Now()
	r.wait()
}

func (r *runner) stop() {
	if r.cancel == nil {
		return
	}
	r.cancel()
	<-r.done
	r.cancel = nil
}

// lastCycleSteps is the number of CompactDue calls of the last cycle.
func (r *runner) lastCycleSteps() int { return r.steps[len(r.steps)-1] }

// polledNow checks that the last cycle polled at the clock's time and the
// poll succeeded.
func (r *runner) polledNow() {
	r.t.Helper()
	v := r.c.LastPoll()
	if !v.OK || !v.Time.Equal(r.e.clk.Now().UTC().Truncate(time.Millisecond)) {
		r.t.Fatalf("no successful poll at %s (last %s)", r.e.clk.Now(), v.Time)
	}
	runs, err := r.e.st.ListCollectorRuns(bg, 1)
	if err != nil || len(runs) != 1 || runs[0].OK == nil || !*runs[0].OK {
		r.t.Fatalf("last run: %+v %v", runs, err)
	}
	if f := r.e.rec.ofType(hourglass.EventPollFailed); len(f) != 0 {
		r.t.Fatalf("poll.failed emitted: %+v", f)
	}
}

// backlog writes perDay raw debits for each hold on each of days days from
// Dec 1, 2029: all due at the env's clock (Jan 1, 2030), whose cutoff is the
// start of Dec 25.
func (e *env) backlog(holds []store.Hold, days, perDay int) {
	e.t.Helper()
	start := time.Date(2029, 12, 1, 0, 0, 0, 0, time.UTC)
	for d := 0; d < days; d++ {
		err := e.st.Tx(bg, func(tx *store.Tx) error {
			for _, h := range holds {
				hid := h.ID
				for k := 0; k < perDay; k++ {
					at := start.AddDate(0, 0, d).Add(time.Duration(k) * 5 * time.Minute)
					if _, err := tx.AppendLedger(bg, store.LedgerEntry{UserID: h.UserID, Period: "2029-12", Type: "debit",
						AmountMT: -5, HoldID: &hid}, e.led.CurrentPeriod(), at); err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			e.t.Fatal(err)
		}
	}
}

func (e *env) dueGroups() int {
	e.t.Helper()
	d, err := e.st.DueLedgerGroups(bg, e.clk.Now(), time.UTC, time.Time{}, 1<<30)
	if err != nil {
		e.t.Fatal(err)
	}
	return len(d.Groups)
}

// twoHolds is the basic setup plus a second hold of the same user.
func (e *env) twoHolds() []store.Hold {
	u, h := e.basic()
	s := e.setup("lab-02", true, map[string]string{"tier": "gold"})
	return []store.Hold{h, e.hold(s, u)}
}

// TestLED10_StepsReleaseTheLockBetweenThem: each step holds runMu, and a poll
// waiting for runMu runs between two steps of one cycle's backlog instead of
// waiting for the whole cycle.
func TestLED10_StepsReleaseTheLockBetweenThem(t *testing.T) {
	e := newEnv(t, false)
	holds := e.twoHolds()
	e.backlog(holds, 12, 288) // 6,912 rows: several 2,000-row steps
	pollDone := make(chan error, 1)
	var between bool
	r := e.startRun(CompactStepsPerCycle, func(r *runner, n int) {
		if r.c.runMu.TryLock() {
			r.c.runMu.Unlock()
			t.Errorf("step %d runs without runMu", n)
		}
		if n == 0 {
			// A poll arrives during the first step and waits for runMu.
			e.clk.Advance(time.Minute)
			go func() { pollDone <- r.c.RunOnce(bg) }()
			time.Sleep(50 * time.Millisecond)
			return
		}
		if between {
			return
		}
		// A later step of the same cycle: the waiting poll must have had runMu
		// in between. sync.Mutex lets the unlocking goroutine win a race or
		// two, then hands the lock to a waiter queued for over 1 ms
		// (starvation mode), so the poll gets in after a step or two.
		select {
		case err := <-pollDone:
			between = true
			if err != nil {
				t.Errorf("poll between steps: %v", err)
			}
		case <-time.After(300 * time.Millisecond):
		}
	})
	if !between {
		t.Fatalf("no poll ran between the %d steps of the first cycle: runMu was held across steps", r.lastCycleSteps())
	}
	if got := r.lastCycleSteps(); got < 2 || got > CompactStepsPerCycle {
		t.Fatalf("%d steps in the first cycle", got)
	}
	r.polledNow()
}

// TestLED10_BacklogFinishesOverLaterCycles: with a step budget smaller than
// the backlog, each cycle runs at most its budget, polls every cycle, logs the
// rows removed, and the backlog is gone a few cycles later without waiting for
// the next daily prune.
func TestLED10_BacklogFinishesOverLaterCycles(t *testing.T) {
	e := newEnv(t, false)
	holds := e.twoHolds()
	e.backlog(holds, 12, 288)
	r := e.startRun(2, nil)
	if got := r.lastCycleSteps(); got != 2 {
		t.Fatalf("first cycle ran %d steps, want the budget of 2", got)
	}
	if e.dueGroups() == 0 {
		t.Fatal("the backlog finished in one cycle: it should not fit in 2 steps")
	}
	r.polledNow()
	for i := 0; e.dueGroups() > 0; i++ {
		if i == 5 {
			t.Fatalf("backlog not finished after %d cycles: %v steps", len(r.steps), r.steps)
		}
		r.cycle(poll)
		r.polledNow()
		if got := r.lastCycleSteps(); got > 2 {
			t.Fatalf("cycle %d ran %d steps, over the budget of 2", len(r.steps), got)
		}
	}
	if len(r.steps) < 2 {
		t.Fatalf("finished in %d cycles", len(r.steps))
	}
	// Once a step finds nothing more, the following cycles do not step.
	r.cycle(poll)
	if r.lastCycleSteps() != 0 {
		r.cycle(poll)
	}
	for i := 0; i < 3; i++ {
		r.cycle(poll)
		if got := r.lastCycleSteps(); got != 0 {
			t.Fatalf("a step ran %d times after the backlog was done", got)
		}
	}
	// Every hold's day is one row now, and the removed rows were logged.
	var raw, rolled int
	l, err := e.st.ListLedger(bg, holds[0].UserID, "2029-12") // both holds are this user's
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range l {
		if x.RollupCount > 0 {
			rolled++
		} else {
			raw++
		}
	}
	if raw != 0 || rolled != 24 {
		t.Fatalf("December: %d raw, %d rolled-up rows", raw, rolled)
	}
	logged := 0
	for _, line := range strings.Split(r.logs.String(), "\n") {
		if strings.Contains(line, `msg="compacted ledger"`) {
			var rows, groups int
			if i := strings.Index(line, " rows="); i >= 0 {
				fmt.Sscanf(line[i:], " rows=%d groups=%d", &rows, &groups)
			}
			if rows <= 0 {
				t.Errorf("logged without rows: %s", line)
			}
			logged += rows
		}
	}
	if logged != 24*287 {
		t.Fatalf("logged %d rows removed, want %d", logged, 24*287)
	}
}

// TestLED10_FailedStepFailsNoPollAndIsRetried: a failing step is logged, the
// poll of every cycle still succeeds, the step is retried on the very next
// cycle, and the backlog finishes once the failure clears.
func TestLED10_FailedStepFailsNoPollAndIsRetried(t *testing.T) {
	e := newEnv(t, false)
	holds := e.twoHolds()
	e.backlog(holds, 3, 288)
	due := e.dueGroups()
	r := e.startRun(CompactStepsPerCycle, func(r *runner, n int) {
		if n == 0 {
			r.led.setFail(errors.New("injected compaction failure"))
		}
	})
	if got := r.lastCycleSteps(); got != 1 {
		t.Fatalf("first cycle: %d steps, want 1 (stop at the failure)", got)
	}
	r.polledNow()
	if e.dueGroups() != due {
		t.Fatal("a failed step changed the backlog")
	}
	if !strings.Contains(r.logs.String(), "injected compaction failure") {
		t.Fatal("the failure was not logged")
	}
	r.cycle(poll) // still failing: retried, the poll is fine
	if got := r.lastCycleSteps(); got != 1 {
		t.Fatalf("second cycle: %d steps, want 1 retry", got)
	}
	r.polledNow()
	r.led.setFail(nil)
	r.cycle(poll)
	r.polledNow()
	if got := r.lastCycleSteps(); got < 1 {
		t.Fatal("not retried after the failure cleared")
	}
	if n := e.dueGroups(); n != 0 {
		t.Fatalf("%d due groups left after the retry", n)
	}
}

// TestLED10_IdleCheckRunsOnceADay: with nothing due, the first cycle checks
// once and later cycles do not, until a day after the previous check (with the
// daily prune); rows that became due in between are compacted then.
func TestLED10_IdleCheckRunsOnceADay(t *testing.T) {
	e := newEnv(t, false)
	holds := e.twoHolds()
	r := e.startRun(CompactStepsPerCycle, nil)
	if got := r.lastCycleSteps(); got != 1 {
		t.Fatalf("start-up check: %d steps, want 1", got)
	}
	e.backlog(holds, 2, 288) // appears due after the check; waits for the next one
	for i := 0; i < 5; i++ {
		r.cycle(poll)
		if got := r.lastCycleSteps(); got != 0 {
			t.Fatalf("cycle %d checked %d times within the day", len(r.steps), got)
		}
	}
	if e.dueGroups() == 0 {
		t.Fatal("rows compacted before the daily check")
	}
	r.cycle(24*time.Hour - 25*time.Minute) // exactly a day after the first check
	if got := r.lastCycleSteps(); got != 1 {
		t.Fatalf("daily check: %d steps, want 1", got)
	}
	r.polledNow()
	if n := e.dueGroups(); n != 0 {
		t.Fatalf("%d due groups left after the daily check", n)
	}
	r.cycle(poll)
	if got := r.lastCycleSteps(); got != 0 {
		t.Fatalf("checked again %d times after the daily check", got)
	}
}

// TestLED10_CompactionDoesNotStretchThePollInterval drives Run as production
// does (one loop, nothing else polling) with a backlog of many steps and steps
// that each take time on the clock: the wait Run asks for is the poll interval
// less the time the cycle spent compacting, so the next poll starts one
// interval after the compaction began, however long the backlog makes it run.
func TestLED10_CompactionDoesNotStretchThePollInterval(t *testing.T) {
	const stepTime = 7 * time.Second
	e := newEnv(t, false)
	holds := e.twoHolds()
	e.backlog(holds, 12, 288) // 6,912 rows: several steps per cycle
	r := e.startRun(CompactStepsPerCycle, func(r *runner, n int) { e.clk.Advance(stepTime) })
	// The first cycle's steps ran on the fake clock; check each cycle until
	// the backlog is gone: compaction time plus wait is exactly one interval.
	for cycles := 0; ; cycles++ {
		steps := r.lastCycleSteps()
		if want := poll - time.Duration(steps)*stepTime; r.lastWait != want {
			t.Fatalf("cycle %d: %d steps of %s, Run waited %s, want %s (the interval less the compaction time)",
				cycles, steps, stepTime, r.lastWait, want)
		}
		if steps < 2 && cycles == 0 {
			t.Fatalf("the backlog took %d steps: the test needs several", steps)
		}
		if e.dueGroups() == 0 {
			break
		}
		if cycles == 5 {
			t.Fatalf("backlog not finished after %d cycles", cycles)
		}
		r.cycle(poll)
	}
}

// TestLED10_LongCompactionNeverWaitsPastTheInterval: when the compaction alone
// runs longer than the interval, Run does not add a further wait on top, and
// the wait is never negative.
func TestLED10_LongCompactionNeverWaitsPastTheInterval(t *testing.T) {
	e := newEnv(t, false)
	holds := e.twoHolds()
	e.backlog(holds, 12, 288)
	r := e.startRun(CompactStepsPerCycle, func(r *runner, n int) { e.clk.Advance(2 * poll) })
	if r.lastCycleSteps() < 1 || r.lastWait != 0 {
		t.Fatalf("%d steps, wait %s: want a zero wait after a compaction longer than the interval", r.lastCycleSteps(), r.lastWait)
	}
}

// TestLED10_NoCompactionKeepsTheFullInterval: a cycle that does not compact
// waits the whole interval, as before the roll-up existed.
func TestLED10_NoCompactionKeepsTheFullInterval(t *testing.T) {
	e := newEnv(t, false)
	e.twoHolds()
	r := e.startRun(CompactStepsPerCycle, nil)
	r.cycle(poll)
	if r.lastCycleSteps() != 0 || r.lastWait != poll {
		t.Fatalf("%d steps, wait %s, want 0 steps and %s", r.lastCycleSteps(), r.lastWait, poll)
	}
}
