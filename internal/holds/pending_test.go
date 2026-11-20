// SPDX-License-Identifier: MIT

package holds

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

func (r *recorder) count(typ string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if e.Type == typ {
			n++
		}
	}
	return n
}

func (r *recorder) last(typ string) hourglass.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.events) - 1; i >= 0; i-- {
		if r.events[i].Type == typ {
			return r.events[i]
		}
	}
	return hourglass.Event{}
}

// HOLD-6: a release of a reported setup is pending, the marker is set once,
// hold.releasing fires once, and a repeat fires nothing.
func TestHOLD6_ReleaseOfDeployedSetupIsPendingAndFiresOnce(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	h, err := e.svc.Acquire(bg, u, e.setup("s", true))
	if err != nil {
		t.Fatal(err)
	}
	e.poll.reported["s"] = true
	e.clk.Advance(time.Minute)
	first, err := e.svc.Release(bg, u, h.ID, ReleaseOpts{})
	if err != nil || !Pending(first) {
		t.Fatalf("first release: %+v, %v", first, err)
	}
	want := e.clk.Now()
	if !first.ReleaseRequestedAt.Equal(want) || first.ReleasedAt != nil {
		t.Fatalf("hold %+v", first)
	}
	if n := e.rec.count(hourglass.EventHoldReleasing); n != 1 {
		t.Fatalf("hold.releasing fired %d times", n)
	}
	ev := e.rec.last(hourglass.EventHoldReleasing)
	if ev.User == nil || ev.User.ID != u || ev.Setup == nil || ev.Setup.Name != "s" || ev.Hold == nil || ev.Hold.ID != h.ID ||
		ev.Hold.ReleaseRequestedAt == nil || len(ev.Data) != 0 {
		t.Fatalf("event %+v data %v", ev, ev.Data)
	}
	if hourglass.Vetoable(hourglass.EventHoldReleasing) {
		t.Fatal("hold.releasing must not be vetoable")
	}

	e.clk.Advance(time.Hour)
	again, err := e.svc.Release(bg, u, h.ID, ReleaseOpts{})
	if err != nil || !Pending(again) || !again.ReleaseRequestedAt.Equal(want) {
		t.Fatalf("repeat: %+v, %v; want the same pending hold", again, err)
	}
	if n := e.rec.count(hourglass.EventHoldReleasing); n != 1 {
		t.Fatalf("repeat fired hold.releasing; total %d", n)
	}
	if n := e.rec.count(hourglass.EventHoldReleased); n != 0 {
		t.Fatalf("hold.released fired %d times for a pending hold", n)
	}
}

// HOLD-6 under concurrency: many simultaneous requests set the marker once and
// fire one event.
func TestHOLD6_ConcurrentRequestsFireOnce(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	h, err := e.svc.Acquire(bg, u, e.setup("s", true))
	if err != nil {
		t.Fatal(err)
	}
	e.poll.reported["s"] = true
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if out, err := e.svc.Release(bg, u, h.ID, ReleaseOpts{}); err != nil || !Pending(out) {
				t.Errorf("release: %+v, %v", out, err)
			}
		}()
	}
	wg.Wait()
	if n := e.rec.count(hourglass.EventHoldReleasing); n != 1 {
		t.Fatalf("hold.releasing fired %d times", n)
	}
}

// HOLD-7: a pending hold stays active and counts toward the hold limit.
func TestHOLD7_PendingHoldStaysActiveAndCountsTowardLimit(t *testing.T) {
	e := newEnv(t)
	e.cfg(func(s *settings.Settings) { s.MaxHoldsPerUser = 1 })
	u := e.user("u", 10, store.RoleDeveloper)
	h, err := e.svc.Acquire(bg, u, e.setup("s", true))
	if err != nil {
		t.Fatal(err)
	}
	e.poll.reported["s"] = true
	if out, err := e.svc.Release(bg, u, h.ID, ReleaseOpts{}); err != nil || !Pending(out) {
		t.Fatalf("release: %+v, %v", out, err)
	}
	if n := e.countHolds(u); n != 1 {
		t.Fatalf("active holds = %d, want 1", n)
	}
	_, err = e.svc.Acquire(bg, u, e.setup("t", true))
	wantCode(t, err, CodeHoldLimit)
	// The setup stays held.
	other := e.user("other", 10, store.RoleDeveloper)
	_, err = e.svc.Acquire(bg, other, 1)
	wantCode(t, err, CodeSetupHeld)
}

// HOLD-8: a service account releases any active hold with a reason, audited
// under its name, with the documented event data.
func TestHOLD8_ServiceReleaseReleasesAnyActiveHold(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	h, err := e.svc.Acquire(bg, u, e.setup("s", true))
	if err != nil {
		t.Fatal(err)
	}
	e.poll.reported["s"] = true // the teardown check is skipped
	e.clk.Advance(time.Hour)
	out, err := e.svc.ReleaseByService(bg, "reaper", h.ID, "  idle for 48 hours  ")
	if err != nil {
		t.Fatal(err)
	}
	if out.ReleasedAt == nil || !out.ReleasedAt.Equal(e.clk.Now()) || out.ReleasedBy != nil ||
		out.ReleasedByService != "reaper" || out.ReleaseReason != "idle for 48 hours" {
		t.Fatalf("hold %+v", out)
	}
	rows, err := e.st.ListAudit(bg, store.AuditFilter{})
	if err != nil || len(rows) != 1 {
		t.Fatalf("audit rows %v, %v", rows, err)
	}
	r := rows[0]
	if r.Action != "hold.service_release" || r.ActorService != "reaper" || r.ActorID != nil || r.Target == "" ||
		!strings.Contains(r.AfterJSON, "idle for 48 hours") {
		t.Fatalf("audit row %+v", r)
	}
	ev := e.rec.last(hourglass.EventHoldReleased)
	if ev.Data["forced"] != false || ev.Data["reason"] != "idle for 48 hours" || ev.Data["service_account"] != "reaper" ||
		len(ev.Data) != 3 || ev.User == nil || ev.User.ID != u || ev.Hold == nil || ev.Hold.ReleasedAt == nil {
		t.Fatalf("event %+v data %v", ev, ev.Data)
	}
	if n := e.countHolds(u); n != 0 {
		t.Fatalf("active holds %d", n)
	}
}

func TestHOLD8_ServiceReleaseRefusals(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	h, err := e.svc.Acquire(bg, u, e.setup("s", true))
	if err != nil {
		t.Fatal(err)
	}
	for _, reason := range []string{"", "   \t\n", strings.Repeat("x", 501), " " + strings.Repeat("é", 501) + " "} {
		_, err := e.svc.ReleaseByService(bg, "reaper", h.ID, reason)
		wantCode(t, err, CodeValidation)
	}
	if n := e.countHolds(u); n != 1 {
		t.Fatal("hold released by an invalid request")
	}
	if rows, _ := e.st.ListAudit(bg, store.AuditFilter{}); len(rows) != 0 {
		t.Fatal("audit row written by a refusal")
	}
	_, err = e.svc.ReleaseByService(bg, "reaper", 4242, "gone")
	wantCode(t, err, CodeNotFound)
	// 500 characters is allowed, counted after trimming and as characters.
	if _, err := e.svc.ReleaseByService(bg, "reaper", h.ID, " "+strings.Repeat("é", 500)+" "); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.ReleaseByService(bg, "reaper", h.ID, "again")
	wantCode(t, err, CodeConflict)
	if n := e.rec.count(hourglass.EventHoldReleased); n != 1 {
		t.Fatalf("hold.released fired %d times", n)
	}
}

// HOLD-8: a service release and an admin force-release each complete a
// pending release at once.
func TestHOLD8_ServiceReleaseCompletesPendingRelease(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	h, _ := e.svc.Acquire(bg, u, e.setup("s", true))
	e.poll.reported["s"] = true
	if out, err := e.svc.Release(bg, u, h.ID, ReleaseOpts{}); err != nil || !Pending(out) {
		t.Fatalf("release: %+v, %v", out, err)
	}
	out, err := e.svc.ReleaseByService(bg, "reaper", h.ID, "torn down")
	if err != nil || out.ReleasedAt == nil || out.ReleasedByService != "reaper" || out.ReleaseRequestedAt == nil {
		t.Fatalf("%+v, %v", out, err)
	}
	if n := e.rec.count(hourglass.EventHoldReleased); n != 1 {
		t.Fatalf("hold.released fired %d times", n)
	}
}

func TestHOLD7_ForceReleaseCompletesPendingRelease(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	admin := e.user("boss", 0, store.RoleAdmin)
	h, _ := e.svc.Acquire(bg, u, e.setup("s", true))
	e.poll.reported["s"] = true
	if out, err := e.svc.Release(bg, u, h.ID, ReleaseOpts{}); err != nil || !Pending(out) {
		t.Fatalf("release: %+v, %v", out, err)
	}
	out, err := e.svc.Release(bg, admin, h.ID, ReleaseOpts{Force: true, Reason: "cleanup"})
	if err != nil || out.ReleasedAt == nil || out.ReleasedBy == nil || *out.ReleasedBy != admin {
		t.Fatalf("%+v, %v", out, err)
	}
	if n := e.countHolds(u); n != 0 {
		t.Fatalf("active holds %d", n)
	}
}

// A user's release racing a service release never releases twice or fires
// two hold.released events.
func TestHOLD8_ConcurrentReleasesFireOnce(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	h, _ := e.svc.Acquire(bg, u, e.setup("s", true))
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = e.svc.Release(bg, u, h.ID, ReleaseOpts{}) }()
		go func() { defer wg.Done(); _, _ = e.svc.ReleaseByService(bg, "reaper", h.ID, "x") }()
	}
	wg.Wait()
	if n := e.rec.count(hourglass.EventHoldReleased); n != 1 {
		t.Fatalf("hold.released fired %d times", n)
	}
}

// raceDispatcher runs a service release in a goroutine from inside the
// Emit of hold.releasing and gives it a moment to finish before the event is
// recorded. It is how a release that races the request is made deterministic.
type raceDispatcher struct {
	*recorder
	svc  func() // the racing release
	done chan struct{}
	once sync.Once
}

func (d *raceDispatcher) Emit(e hourglass.Event) {
	if e.Type == hourglass.EventHoldReleasing {
		d.once.Do(func() {
			go func() { d.svc(); close(d.done) }()
			select {
			case <-d.done:
			case <-time.After(300 * time.Millisecond):
			}
		})
	}
	d.recorder.Emit(e)
}

// EXT-6, HOLD-6, HOLD-8: events for one hold are queued in the order in which
// their state changes committed. A service release that races the request for
// release can commit only after it, so hold.releasing is queued before
// hold.released.
func TestHOLD6_ReleasingIsQueuedBeforeAReleaseThatRacesIt(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	h, err := e.svc.Acquire(bg, u, e.setup("s", true))
	if err != nil {
		t.Fatal(err)
	}
	e.poll.reported["s"] = true
	d := &raceDispatcher{recorder: e.rec, done: make(chan struct{})}
	d.svc = func() {
		for i := 0; i < 100; i++ {
			if _, err := e.svc.ReleaseByService(bg, "reaper", h.ID, "x"); err == nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	e.build(d)
	if out, err := e.svc.Release(bg, u, h.ID, ReleaseOpts{}); err != nil || !Pending(out) {
		t.Fatalf("release: %+v, %v", out, err)
	}
	select {
	case <-d.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the racing service release never finished")
	}
	var got []string
	for _, ty := range e.rec.types() {
		if ty == hourglass.EventHoldReleasing || ty == hourglass.EventHoldReleased {
			got = append(got, ty)
		}
	}
	if len(got) != 2 || got[0] != hourglass.EventHoldReleasing || got[1] != hourglass.EventHoldReleased {
		t.Fatalf("event order %v, want hold.releasing then hold.released", got)
	}
}

// HOLD-6: the marker guard. A request that holds a stale copy of the hold,
// read before another request set the marker, must not fire a second
// hold.releasing. The concurrent test cannot reach this window reliably, so it
// is called directly with the stale copy.
func TestHOLD6_StaleRequestAfterMarkerFiresNothing(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	h, err := e.svc.Acquire(bg, u, e.setup("s", true))
	if err != nil {
		t.Fatal(err)
	}
	setup, err := e.st.GetSetup(bg, h.SetupID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		out, err := e.svc.requestRelease(bg, h, setup)
		if err != nil || !Pending(out) {
			t.Fatalf("request %d: %+v, %v", i, out, err)
		}
	}
	if n := e.rec.count(hourglass.EventHoldReleasing); n != 1 {
		t.Fatalf("hold.releasing fired %d times for 3 requests with the same stale hold", n)
	}
}

// HOLD-8: the store refuses a second service release of the same hold, which is
// the guard when two releases pass the service's early check together.
func TestHOLD8_SecondServiceReleaseConflictsAtTheStore(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	h, err := e.svc.Acquire(bg, u, e.setup("s", true))
	if err != nil {
		t.Fatal(err)
	}
	now := e.clk.Now()
	for i, want := range []error{nil, store.ErrConflict} {
		err := e.st.Tx(bg, func(tx *store.Tx) error { return tx.ReleaseHoldByService(bg, h.ID, "reaper", "x", now) })
		if (want == nil && err != nil) || (want != nil && !errors.Is(err, want)) {
			t.Fatalf("store release %d: %v, want %v", i, err, want)
		}
	}
}

// HOLD-6: SetReleaseRequested sets the marker once and says so.
func TestHOLD6_SetReleaseRequestedReportsFirstOnly(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	h, err := e.svc.Acquire(bg, u, e.setup("s", true))
	if err != nil {
		t.Fatal(err)
	}
	t1 := e.clk.Now()
	for i, want := range []bool{true, false, false} {
		var first bool
		err := e.st.Tx(bg, func(tx *store.Tx) error {
			var err error
			first, err = tx.SetReleaseRequested(bg, h.ID, t1.Add(time.Duration(i)*time.Minute))
			return err
		})
		if err != nil || first != want {
			t.Fatalf("call %d: first=%v err=%v, want %v", i, first, err, want)
		}
	}
	got, _ := e.st.GetHold(bg, h.ID)
	if got.ReleaseRequestedAt == nil || !got.ReleaseRequestedAt.Equal(t1) {
		t.Fatalf("marker %v, want the first request's time %v", got.ReleaseRequestedAt, t1)
	}
}
