// SPDX-License-Identifier: MIT

package collector

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/panset/hourglass/internal/holds"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// vetoless adapts the recorder to holds.Dispatcher.
type vetoless struct{ *recorder }

func (vetoless) Veto(context.Context, hourglass.Event) error { return nil }

// holdsSvc is a holds service that reads this collector's latest poll.
func (e *env) holdsSvc() *holds.Service {
	return holds.NewService(holds.Deps{Store: e.st, Events: vetoless{e.rec}, ProviderConfigured: e.prov != nil, Clock: e.clk,
		Poll: func() holds.PollView { return e.c.LastPoll().ForHolds() }})
}

func (e *env) deployed(names ...string) {
	var a []hourglass.Allocation
	for _, n := range names {
		a = append(a, hourglass.Allocation{ResourceName: n})
	}
	e.prov.SetAllocations(a...)
}

// pendingFixture: a deployed, metering hold with a pending release.
func pendingFixture(t *testing.T) (e *env, svc *holds.Service, user int64, h store.Hold) {
	t.Helper()
	e = newEnv(t, true)
	user = e.user("dev")
	h = e.hold(e.setup("lab-01", true, nil), user)
	e.deployed("lab-01")
	e.runOK()    // deployed, metering starts
	e.step(poll) // first debit
	svc = e.holdsSvc()
	out, err := svc.Release(bg, user, h.ID, holds.ReleaseOpts{})
	if err != nil || !holds.Pending(out) {
		t.Fatalf("release: %+v, %v; want pending", out, err)
	}
	return e, svc, user, h
}

// HOLD-7: a pending hold stays active and is debited while the setup is still
// reported, and completes at the first successful poll without the setup.
func TestHOLD7_PendingHoldIsMeteredThenCompletedWhenSetupDisappears(t *testing.T) {
	e, _, user, h := pendingFixture(t)
	before := len(e.debits(user))
	e.step(poll) // still reported
	cur := e.getHold(h.ID)
	if cur.ReleasedAt != nil || cur.ReleaseRequestedAt == nil || len(e.debits(user)) != before+1 {
		t.Fatalf("pending hold after a poll with the setup reported: %+v, debits %v", cur, e.debits(user))
	}
	if len(e.rec.ofType(hourglass.EventHoldReleased)) != 0 {
		t.Fatal("hold.released fired while the setup is reported")
	}

	e.deployed() // torn down
	e.step(poll)
	cur = e.getHold(h.ID)
	if cur.ReleasedAt == nil || !cur.ReleasedAt.Equal(e.clk.Now()) || cur.ReleasedBy == nil || *cur.ReleasedBy != user ||
		cur.ReleasedByService != "" {
		t.Fatalf("hold after the poll without the setup: %+v", cur)
	}
	// Decision recorded here: the poll in which the setup disappears still
	// meters the interval under the policy's decision, then releases.
	if got := len(e.debits(user)); got != before+2 {
		t.Fatalf("debits %v: the completing poll must charge its interval per the policy", e.debits(user))
	}
	evs := e.rec.ofType(hourglass.EventHoldReleased)
	if len(evs) != 1 || evs[0].Data["forced"] != false || evs[0].Data["reason"] != "" || len(evs[0].Data) != 2 ||
		evs[0].User == nil || evs[0].User.ID != user || evs[0].Hold == nil || evs[0].Hold.ReleasedAt == nil {
		t.Fatalf("hold.released events %+v", evs)
	}
	e.step(poll)
	if len(e.rec.ofType(hourglass.EventHoldReleased)) != 1 || len(e.debits(user)) != before+2 {
		t.Fatal("a released hold was completed or charged again")
	}
}

// HOLD-9: a failed poll never completes a pending release; the next successful
// poll without the setup does.
func TestHOLD9_FailedPollLeavesPendingReleasePending(t *testing.T) {
	e, _, _, h := pendingFixture(t)
	e.deployed() // the setup is gone, but the provider cannot say so
	e.prov.SetError(errors.New("backend unreachable"))
	e.clk.Advance(poll)
	if err := e.c.RunOnce(bg); err == nil {
		t.Fatal("poll should have failed")
	}
	if cur := e.getHold(h.ID); cur.ReleasedAt != nil || cur.ReleaseRequestedAt == nil {
		t.Fatalf("failed poll changed the hold: %+v", cur)
	}
	e.pressure.SetError(errors.New("pressure down")) // a failing pressure source fails the poll too
	e.prov.SetError(nil)
	e.clk.Advance(poll)
	if err := e.c.RunOnce(bg); err == nil {
		t.Fatal("poll should have failed")
	}
	if cur := e.getHold(h.ID); cur.ReleasedAt != nil {
		t.Fatalf("a poll that failed after the provider answered completed the release: %+v", cur)
	}
	if len(e.rec.ofType(hourglass.EventHoldReleased)) != 0 {
		t.Fatal("hold.released fired on a failed poll")
	}
	e.pressure.SetError(nil)
	e.step(poll)
	if cur := e.getHold(h.ID); cur.ReleasedAt == nil {
		t.Fatalf("next successful poll did not complete the release: %+v", cur)
	}
	if len(e.rec.ofType(hourglass.EventHoldReleased)) != 1 {
		t.Fatal("hold.released must fire once")
	}
}

// A hold without a pending release is never completed, whether or not the
// setup is reported.
func TestHOLD7_HoldWithoutPendingReleaseIsLeftAlone(t *testing.T) {
	e := newEnv(t, true)
	u := e.user("dev")
	h := e.hold(e.setup("lab-01", true, nil), u)
	e.deployed("lab-01")
	e.runOK()
	e.deployed()
	e.step(poll)
	if cur := e.getHold(h.ID); cur.ReleasedAt != nil {
		t.Fatalf("hold released without a request: %+v", cur)
	}
}

// A user's release racing a poll never releases twice or fires hold.released
// twice. Run under -race.
func TestHOLD7_ReleaseRacingPollReleasesOnce(t *testing.T) {
	e, svc, user, h := pendingFixture(t)
	e.deployed()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = svc.Release(bg, user, h.ID, holds.ReleaseOpts{}) }()
		go func() { defer wg.Done(); _ = e.c.RunOnce(bg) }()
	}
	wg.Wait()
	e.runOK()
	if cur := e.getHold(h.ID); cur.ReleasedAt == nil {
		t.Fatalf("not released: %+v", cur)
	}
	if n := len(e.rec.ofType(hourglass.EventHoldReleased)); n != 1 {
		t.Fatalf("hold.released fired %d times", n)
	}
	if n := len(e.rec.ofType(hourglass.EventHoldReleasing)); n != 1 {
		t.Fatalf("hold.releasing fired %d times", n)
	}
}

// CMP-1: a deployment with no service account and no stored-report extension
// (a plain provider) behaves as before through acquire, poll and release.
// Only HOLD-6 and HOLD-7 differ: a release while deployed is pending, not refused.
func TestCMP1_PlainProviderBehavesAsBefore(t *testing.T) {
	e := newEnv(t, true)
	svc := e.holdsSvc()
	u := e.user("dev")
	e.allowance(u)
	s := e.setup("lab-01", true, nil)

	h, err := svc.Acquire(bg, u, s)
	if err != nil {
		t.Fatal(err)
	}
	if h.ReleaseRequestedAt != nil {
		t.Fatalf("a new hold: %+v", h)
	}
	e.runOK()
	if cur := e.getHold(h.ID); cur.DeployedAt != nil || cur.MeteringStartedAt == nil {
		t.Fatalf("not deployed, metering starts (fake policy): %+v", cur)
	}
	e.deployed("lab-01")
	e.step(poll)
	if cur := e.getHold(h.ID); cur.DeployedAt == nil {
		t.Fatalf("a reported setup gets deployed_at: %+v", cur)
	}
	if len(e.debits(u)) != 1 {
		t.Fatalf("debits %v", e.debits(u))
	}
	if len(e.rec.ofType(hourglass.EventHoldDeployed)) != 1 {
		t.Fatalf("events %v", e.rec.types())
	}
	// With the setup torn down, release is immediate and unchanged.
	e.deployed()
	e.step(poll)
	out, err := svc.Release(bg, u, h.ID, holds.ReleaseOpts{})
	if err != nil || out.ReleasedAt == nil || out.ReleasedBy == nil || *out.ReleasedBy != u ||
		out.ReleaseRequestedAt != nil || out.ReleasedByService != "" {
		t.Fatalf("release: %+v, %v", out, err)
	}
	if len(e.rec.ofType(hourglass.EventHoldReleasing)) != 0 {
		t.Fatal("hold.releasing fired for an immediate release")
	}
	rel := e.rec.ofType(hourglass.EventHoldReleased)
	if len(rel) != 1 || rel[0].Data["forced"] != false || rel[0].Data["reason"] != "" || len(rel[0].Data) != 2 {
		t.Fatalf("hold.released %+v", rel)
	}
	// A later poll charges nothing and creates nothing more.
	n := len(e.debits(u))
	e.step(poll)
	if len(e.debits(u)) != n || len(e.rec.ofType(hourglass.EventHoldReleased)) != 1 {
		t.Fatal("a released hold was touched by a later poll")
	}
	// A setting that is off keeps release immediate while deployed.
	e.setting("release_requires_teardown", "false")
	h2, err := svc.Acquire(bg, u, s)
	if err != nil {
		t.Fatal(err)
	}
	e.deployed("lab-01")
	e.step(poll)
	if out, err := svc.Release(bg, u, h2.ID, holds.ReleaseOpts{}); err != nil || out.ReleasedAt == nil {
		t.Fatalf("release with the setting off: %+v, %v", out, err)
	}
}

// D61 (REVIEW), part b. Whether a user's release is pending is decided from
// the view of the last successful poll. A failed poll does not replace that
// view, so a release made after a failed poll follows what the last successful
// poll saw: pending when it reported the setup.
func TestHOLD9_ReleaseAfterAFailedPollFollowsTheLastSuccessfulView(t *testing.T) {
	e := newEnv(t, true)
	user := e.user("dev")
	h := e.hold(e.setup("lab-01", true, nil), user)
	e.deployed("lab-01")
	e.runOK()
	e.deployed() // the setup is gone, but the provider cannot say so
	e.prov.SetError(errors.New("backend unreachable"))
	e.clk.Advance(poll)
	if err := e.c.RunOnce(bg); err == nil {
		t.Fatal("poll should have failed")
	}
	out, err := e.holdsSvc().Release(bg, user, h.ID, holds.ReleaseOpts{})
	if err != nil || !holds.Pending(out) || out.ReleasedAt != nil {
		t.Fatalf("release after a failed poll: %+v, %v; want pending, from the last successful view", out, err)
	}
	if n := len(e.rec.ofType(hourglass.EventHoldReleasing)); n != 1 {
		t.Fatalf("hold.releasing fired %d times", n)
	}
}

// D61 (REVIEW), part b, the other side. With no successful poll in this
// process (a restart, or only failed polls), there is no view, so the same
// release is immediate even though the setup may well be deployed.
func TestHOLD9_ReleaseWithNoSuccessfulPollIsImmediate(t *testing.T) {
	e := newEnv(t, true)
	user := e.user("dev")
	h := e.hold(e.setup("lab-01", true, nil), user)
	e.deployed("lab-01")
	e.prov.SetError(errors.New("backend unreachable"))
	e.clk.Advance(poll)
	if err := e.c.RunOnce(bg); err == nil {
		t.Fatal("poll should have failed")
	}
	out, err := e.holdsSvc().Release(bg, user, h.ID, holds.ReleaseOpts{})
	if err != nil || holds.Pending(out) || out.ReleasedAt == nil {
		t.Fatalf("release with no successful poll: %+v, %v; want an immediate release", out, err)
	}
	if n := len(e.rec.ofType(hourglass.EventHoldReleasing)); n != 0 {
		t.Fatalf("hold.releasing fired %d times", n)
	}
	if n := len(e.rec.ofType(hourglass.EventHoldReleased)); n != 1 {
		t.Fatalf("hold.released fired %d times", n)
	}
}
