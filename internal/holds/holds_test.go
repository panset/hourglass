// SPDX-License-Identifier: MIT

package holds

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/events"
	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

var bg = context.Background()

type fakePoll struct {
	ok       bool
	reported map[string]bool
}

func (p *fakePoll) OK() bool               { return p.ok }
func (p *fakePoll) Reported(n string) bool { return p.reported[n] }

// recorder is a Dispatcher that records the order of Veto and Emit calls.
type recorder struct {
	mu      sync.Mutex
	order   []string
	events  []hourglass.Event
	vetoErr error
}

func (r *recorder) Veto(_ context.Context, e hourglass.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.order = append(r.order, e.Type)
	r.events = append(r.events, e)
	if r.vetoErr != nil {
		return &events.VetoError{Hook: "rec", Err: r.vetoErr}
	}
	return nil
}
func (r *recorder) Emit(e hourglass.Event) {
	r.mu.Lock()
	r.order = append(r.order, e.Type)
	r.events = append(r.events, e)
	r.mu.Unlock()
}
func (r *recorder) types() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.order...)
}

type env struct {
	t        *testing.T
	st       *store.Store
	clk      *hourglasstest.FakeClock
	rec      *recorder
	poll     *fakePoll
	svc      *Service
	prov     bool
	cfgAdmin int64
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	e := &env{t: t, st: st, clk: hourglasstest.NewFakeClock(time.Time{}), rec: &recorder{}, prov: true}
	e.poll = &fakePoll{ok: true, reported: map[string]bool{}}
	e.build(e.rec)
	return e
}

func (e *env) build(d Dispatcher) {
	e.svc = NewService(Deps{Store: e.st, Events: d, Poll: func() PollView {
		if e.poll == nil {
			return nil
		}
		return e.poll
	}, ProviderConfigured: e.prov, Clock: e.clk})
}

func (e *env) user(name string, balanceTokens int64, roles ...string) int64 {
	e.t.Helper()
	u, err := e.st.CreateOIDCUser(bg, name, name+"@example.test", name, e.clk.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	if len(roles) > 0 {
		if err := e.st.SetUserRoles(bg, u.ID, roles); err != nil {
			e.t.Fatal(err)
		}
	}
	if balanceTokens != 0 {
		if _, err := e.st.AppendLedger(bg, store.LedgerEntry{UserID: u.ID, Period: "2030-01", Type: "allowance", AmountMT: balanceTokens * 1000},
			"2030-01", e.clk.Now()); err != nil {
			e.t.Fatal(err)
		}
	}
	return u.ID
}

func (e *env) setup(name string, enabled bool) int64 {
	e.t.Helper()
	id, err := e.st.CreateSetup(bg, store.Setup{Name: name, Enabled: enabled, DetailsJSON: `{"note":"x"}`,
		Labels: map[string]string{"team": "blue"}})
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) admin() int64 {
	if e.cfgAdmin == 0 {
		e.cfgAdmin = e.user("cfgadmin", 0, store.RoleAdmin)
	}
	return e.cfgAdmin
}

func (e *env) cfg(f func(*settings.Settings)) {
	e.t.Helper()
	old, err := settings.Load(bg, e.st)
	if err != nil {
		e.t.Fatal(err)
	}
	n := old
	f(&n)
	if _, err := settings.Save(bg, e.st, old, n, e.admin()); err != nil {
		e.t.Fatal(err)
	}
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || CodeOf(err) != code {
		t.Fatalf("err = %v (code %q), want code %q", err, CodeOf(err), code)
	}
}

func (e *env) countHolds(user int64) int {
	n, err := e.st.CountActiveHolds(bg, user)
	if err != nil {
		e.t.Fatal(err)
	}
	return n
}

func TestHOLD1_RefusalsInOrder(t *testing.T) {
	e := newEnv(t)
	e.cfg(func(s *settings.Settings) { s.Enforcement = true; s.MaxHoldsPerUser = 1 })
	rich := e.user("rich", 100, store.RoleDeveloper)
	broke := e.user("broke", 0, store.RoleDeveloper)
	norole := e.user("norole", 100)
	free := e.setup("free", true)
	off := e.setup("off", false)
	taken := e.setup("taken", true)
	if _, err := e.svc.Acquire(bg, rich, taken); err != nil {
		t.Fatal(err)
	}

	// 1. role
	_, err := e.svc.Acquire(bg, norole, 9999)
	wantCode(t, err, CodeForbidden)
	// 2. existence, then enabled, then free, all before balance and limit
	_, err = e.svc.Acquire(bg, broke, 9999)
	wantCode(t, err, CodeNotFound)
	_, err = e.svc.Acquire(bg, broke, off)
	wantCode(t, err, CodeSetupDisabled)
	_, err = e.svc.Acquire(bg, broke, taken)
	wantCode(t, err, CodeSetupHeld)
	// 3. balance before limit: rich is at the limit, broke is not
	_, err = e.svc.Acquire(bg, broke, free)
	wantCode(t, err, CodeInsufficientTokens)
	// 4. limit
	_, err = e.svc.Acquire(bg, rich, free)
	wantCode(t, err, CodeHoldLimit)
	// 5. veto comes after the limit
	e.rec.vetoErr = errors.New("no")
	other := e.user("other", 100, store.RoleDeveloper)
	_, err = e.svc.Acquire(bg, other, free)
	wantCode(t, err, CodeHookVeto)
}

func TestMET6_ShadowModeAllowsZeroBalance(t *testing.T) {
	e := newEnv(t)
	u := e.user("zero", 0, store.RoleDeveloper)
	neg := e.user("neg", -5, store.RoleDeveloper)
	a, b := e.setup("a", true), e.setup("b", true)
	if _, err := e.svc.Acquire(bg, u, a); err != nil { // enforcement defaults to off
		t.Fatalf("shadow mode: %v", err)
	}
	if _, err := e.svc.Acquire(bg, neg, b); err != nil {
		t.Fatalf("shadow mode negative: %v", err)
	}
	e.cfg(func(s *settings.Settings) { s.Enforcement = true })
	c := e.setup("c", true)
	_, err := e.svc.Acquire(bg, u, c)
	wantCode(t, err, CodeInsufficientTokens)
}

func TestHOLD1_LimitTwoThirdRefused(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	for _, n := range []string{"a", "b"} {
		if _, err := e.svc.Acquire(bg, u, e.setup(n, true)); err != nil {
			t.Fatal(err)
		}
	}
	_, err := e.svc.Acquire(bg, u, e.setup("c", true))
	wantCode(t, err, CodeHoldLimit)
	if n := e.countHolds(u); n != 2 {
		t.Fatalf("holds = %d", n)
	}
}

func TestHOLD5_LoweringLimitKeepsHoldsAndBlocksNew(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	for _, n := range []string{"a", "b"} {
		if _, err := e.svc.Acquire(bg, u, e.setup(n, true)); err != nil {
			t.Fatal(err)
		}
	}
	e.cfg(func(s *settings.Settings) { s.MaxHoldsPerUser = 1 })
	if n := e.countHolds(u); n != 2 {
		t.Fatalf("holds after lowering = %d, want 2 kept", n)
	}
	_, err := e.svc.Acquire(bg, u, e.setup("c", true))
	wantCode(t, err, CodeHoldLimit)
	if n := e.countHolds(u); n != 2 {
		t.Fatalf("holds = %d", n)
	}
}

func TestHOLD1_VetoReturnsHookMessageAndCreatesNothing(t *testing.T) {
	e := newEnv(t)
	hook := hourglasstest.NewFakeHook()
	hook.SetError(hourglass.EventHoldAcquiring, errors.New("lab is closed on Fridays"))
	e.build(events.New([]hourglass.Hook{hook}, e.st))
	u := e.user("u", 10, store.RoleDeveloper)
	s := e.setup("s", true)
	_, err := e.svc.Acquire(bg, u, s)
	wantCode(t, err, CodeHookVeto)
	if err.Error() != "lab is closed on Fridays" {
		t.Fatalf("message %q", err.Error())
	}
	if n := e.countHolds(u); n != 0 {
		t.Fatal("hold created despite veto")
	}
	if _, err := e.st.ActiveHoldForSetup(bg, s); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("setup held after veto: %v", err)
	}
}

func TestHOLD1_VetoTimeout(t *testing.T) {
	e := newEnv(t)
	hook := hourglasstest.NewFakeHook()
	gate := make(chan struct{})
	defer close(gate)
	hook.SetGate(gate)
	e.build(events.New([]hourglass.Hook{hook}, e.st, events.WithTimeout(50*time.Millisecond)))
	u := e.user("u", 10, store.RoleDeveloper)
	_, err := e.svc.Acquire(bg, u, e.setup("s", true))
	wantCode(t, err, CodeHookVeto)
	if n := e.countHolds(u); n != 0 {
		t.Fatal("hold created after veto timeout")
	}
}

func TestHOLD1_ConcurrentAcquireOfOneSetup(t *testing.T) {
	e := newEnv(t)
	s := e.setup("s", true)
	const n = 8
	ids := make([]int64, n)
	for i := range ids {
		ids[i] = e.user(string(rune('a'+i)), 10, store.RoleDeveloper)
	}
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = e.svc.Acquire(bg, ids[i], s)
		}(i)
	}
	close(start)
	wg.Wait()
	won := 0
	for _, err := range errs {
		if err == nil {
			won++
		} else if CodeOf(err) != CodeSetupHeld {
			t.Fatalf("loser got %v", err)
		}
	}
	if won != 1 {
		t.Fatalf("winners = %d, want 1", won)
	}
	if got := e.rec.types(); count(got, hourglass.EventHoldAcquired) != 1 {
		t.Fatalf("acquired events %v", got)
	}
}

func count(l []string, s string) int {
	n := 0
	for _, x := range l {
		if x == s {
			n++
		}
	}
	return n
}

func TestHOLD1_ConcurrentAcquireRespectsLimit(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	var sids []int64
	for i := 0; i < 6; i++ {
		sids = append(sids, e.setup(string(rune('a'+i)), true))
	}
	var wg sync.WaitGroup
	for _, s := range sids {
		wg.Add(1)
		go func(s int64) { defer wg.Done(); _, _ = e.svc.Acquire(bg, u, s) }(s)
	}
	wg.Wait()
	if n := e.countHolds(u); n != 2 {
		t.Fatalf("holds = %d, want exactly the limit 2", n)
	}
}

func TestHOLD1_LocalAdminCannotAcquire(t *testing.T) {
	e := newEnv(t)
	a, err := e.st.CreateLocalAdmin(bg, "admin", "hash", false, e.clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.Acquire(bg, a.ID, e.setup("s", true))
	wantCode(t, err, CodeForbidden)
}

func TestHOLD3_AcquireEventsOrderAndContent(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	s := e.setup("s", true)
	h, err := e.svc.Acquire(bg, u, s)
	if err != nil {
		t.Fatal(err)
	}
	if h.UserID != u || h.SetupID != s || !h.AcquiredAt.Equal(e.clk.Now()) || h.ReleasedAt != nil {
		t.Fatalf("hold %+v", h)
	}
	if got := e.rec.types(); len(got) != 2 || got[0] != hourglass.EventHoldAcquiring || got[1] != hourglass.EventHoldAcquired {
		t.Fatalf("events %v", got)
	}
	// nothing is charged at acquire
	if n, _ := e.st.SumLedger(bg, u, "2030-01", "debit"); n != 0 {
		t.Fatalf("debits = %d", n)
	}
	for _, ev := range e.rec.events {
		if ev.User == nil || ev.User.ID != u || len(ev.User.Roles) != 1 {
			t.Fatalf("%s user %+v", ev.Type, ev.User)
		}
		if ev.Setup == nil || ev.Setup.Name != "s" || ev.Setup.Labels["team"] != "blue" || string(ev.Setup.Details) != `{"note":"x"}` {
			t.Fatalf("%s setup %+v", ev.Type, ev.Setup)
		}
		if ev.Hold == nil || ev.Hold.UserID != u || ev.Hold.SetupName != "s" {
			t.Fatalf("%s hold %+v", ev.Type, ev.Hold)
		}
	}
	if e.rec.events[1].Hold.ID != h.ID {
		t.Fatal("hold.acquired lacks the hold id")
	}
}

func TestHOLD2_ReleaseWhileReportedIsPendingAllowedAfter(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	h, err := e.svc.Acquire(bg, u, e.setup("s", true))
	if err != nil {
		t.Fatal(err)
	}
	e.poll.reported["s"] = true
	pend, err := e.svc.Release(bg, u, h.ID, ReleaseOpts{})
	if err != nil || !Pending(pend) {
		t.Fatalf("release while reported: %+v, %v; want a pending release", pend, err)
	}
	if n := e.countHolds(u); n != 1 {
		t.Fatal("hold released while the setup is deployed")
	}
	// A pending release is completed by the collector, not by a second click.
	e.poll.reported["s"] = false
	if again, err := e.svc.Release(bg, u, h.ID, ReleaseOpts{}); err != nil || !Pending(again) {
		t.Fatalf("repeat request: %+v, %v; want the same pending answer", again, err)
	}
	// A hold on a setup that is not reported is released at once.
	h2, err := e.svc.Acquire(bg, u, e.setup("t", true))
	if err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(time.Hour)
	out, err := e.svc.Release(bg, u, h2.ID, ReleaseOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if out.ReleasedAt == nil || !out.ReleasedAt.Equal(e.clk.Now()) || out.ReleasedBy == nil || *out.ReleasedBy != u {
		t.Fatalf("released hold %+v", out)
	}
}

func TestHOLD2_ReleaseProceedsWhenCheckDoesNotApply(t *testing.T) {
	cases := map[string]func(e *env){
		"no provider":      func(e *env) { e.prov = false; e.build(e.rec) },
		"setting off":      func(e *env) { e.cfg(func(s *settings.Settings) { s.ReleaseRequiresTeardown = false }) },
		"no poll yet":      func(e *env) { e.poll = nil },
		"last poll failed": func(e *env) { e.poll.ok = false },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			u := e.user("u", 10, store.RoleDeveloper)
			h, err := e.svc.Acquire(bg, u, e.setup("s", true))
			if err != nil {
				t.Fatal(err)
			}
			e.poll.reported["s"] = true
			setup(e)
			if _, err := e.svc.Release(bg, u, h.ID, ReleaseOpts{}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// D40: the teardown check matches the provider's allocation by the setup's
// exact name; a reported name that only resembles it does not block release.
func TestHOLD2_ReleaseMatchesAllocationBySetupName(t *testing.T) {
	e := newEnv(t)
	u := e.user("u", 10, store.RoleDeveloper)
	h, err := e.svc.Acquire(bg, u, e.setup("s", true))
	if err != nil {
		t.Fatal(err)
	}
	e.poll.reported["s-other"] = true
	e.poll.reported["S"] = true
	if _, err := e.svc.Release(bg, u, h.ID, ReleaseOpts{}); err != nil {
		t.Fatal(err)
	}
	h, err = e.svc.Acquire(bg, u, e.setup("t", true))
	if err != nil {
		t.Fatal(err)
	}
	e.poll.reported["t"] = true
	out, err := e.svc.Release(bg, u, h.ID, ReleaseOpts{})
	if err != nil || !Pending(out) {
		t.Fatalf("an exact name match must make the release pending: %+v, %v", out, err)
	}
}

func TestHOLD2_ForceReleaseNeedsAdminAndReasonAndIsAudited(t *testing.T) {
	e := newEnv(t)
	dev := e.user("dev", 10, store.RoleDeveloper)
	other := e.user("other", 10, store.RoleDeveloper)
	admin := e.user("boss", 0, store.RoleAdmin)
	norole := e.user("norole", 0)
	h, err := e.svc.Acquire(bg, dev, e.setup("s", true))
	if err != nil {
		t.Fatal(err)
	}
	e.poll.reported["s"] = true

	_, err = e.svc.Release(bg, other, h.ID, ReleaseOpts{})
	wantCode(t, err, CodeForbidden)
	_, err = e.svc.Release(bg, other, h.ID, ReleaseOpts{Force: true, Reason: "x"})
	wantCode(t, err, CodeForbidden)
	_, err = e.svc.Release(bg, dev, h.ID, ReleaseOpts{Force: true, Reason: "x"}) // owner, not admin
	wantCode(t, err, CodeForbidden)
	_, err = e.svc.Release(bg, norole, h.ID, ReleaseOpts{Force: true, Reason: "x"})
	wantCode(t, err, CodeForbidden)
	_, err = e.svc.Release(bg, admin, h.ID, ReleaseOpts{}) // admin, not owner, not forced
	wantCode(t, err, CodeForbidden)
	for _, r := range []string{"", "   \t"} {
		_, err = e.svc.Release(bg, admin, h.ID, ReleaseOpts{Force: true, Reason: r})
		wantCode(t, err, CodeValidation)
	}
	if rows, _ := e.st.ListAudit(bg, store.AuditFilter{}); len(rows) != 0 {
		t.Fatalf("audit rows after refusals: %d", len(rows))
	}

	out, err := e.svc.Release(bg, admin, h.ID, ReleaseOpts{Force: true, Reason: "owner on leave"}) // still reported
	if err != nil {
		t.Fatal(err)
	}
	if out.ReleasedBy == nil || *out.ReleasedBy != admin || out.ReleaseReason != "owner on leave" {
		t.Fatalf("hold %+v", out)
	}
	rows, _ := e.st.ListAudit(bg, store.AuditFilter{})
	if len(rows) == 1 {
		var before map[string]any
		if err := json.Unmarshal([]byte(rows[0].BeforeJSON), &before); err != nil || before["released_at"] != nil ||
			before["hold_id"] != float64(h.ID) || before["setup"] != "s" || before["user_id"] != float64(dev) {
			t.Fatalf("before_json %q (%v)", rows[0].BeforeJSON, err)
		}
	}
	if len(rows) != 1 || rows[0].Action != AuditForceRelease || rows[0].ActorID == nil || *rows[0].ActorID != admin {
		t.Fatalf("audit %+v", rows)
	}
	if rows[0].Target != "hold:"+itoa(h.ID) {
		t.Fatalf("target %q", rows[0].Target)
	}
	// already released: conflict (api/openapi.yaml), for the owner and for an
	// admin; a user who may not release the hold is told forbidden, not that
	// the hold exists and is released.
	_, err = e.svc.Release(bg, admin, h.ID, ReleaseOpts{Force: true, Reason: "again"})
	wantCode(t, err, CodeConflict)
	_, err = e.svc.Release(bg, dev, h.ID, ReleaseOpts{})
	wantCode(t, err, CodeConflict)
	stranger := e.user("stranger", 11, store.RoleDeveloper)
	_, err = e.svc.Release(bg, stranger, h.ID, ReleaseOpts{})
	wantCode(t, err, CodeForbidden)
	_, err = e.svc.Release(bg, dev, 4242, ReleaseOpts{})
	wantCode(t, err, CodeNotFound)
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestHOLD2_AdminReleasesOwnHoldLikeAnyone(t *testing.T) {
	e := newEnv(t)
	admin := e.user("boss", 10, store.RoleAdmin)
	h, err := e.svc.Acquire(bg, admin, e.setup("s", true))
	if err != nil {
		t.Fatal(err)
	}
	e.poll.reported["s"] = true
	if out, err := e.svc.Release(bg, admin, h.ID, ReleaseOpts{}); err != nil || !Pending(out) {
		t.Fatalf("admin release while reported: %+v, %v; want a pending release", out, err)
	}
	e.poll.reported["s"] = false
	if _, err := e.svc.Release(bg, admin, h.ID, ReleaseOpts{}); err != nil {
		t.Fatal(err)
	}
	if rows, _ := e.st.ListAudit(bg, store.AuditFilter{}); len(rows) != 0 {
		t.Fatal("plain release must not be audited")
	}
}

func TestHOLD3_ReleaseEvent(t *testing.T) {
	e := newEnv(t)
	dev := e.user("dev", 10, store.RoleDeveloper)
	admin := e.user("boss", 0, store.RoleAdmin)
	h1, _ := e.svc.Acquire(bg, dev, e.setup("a", true))
	h2, _ := e.svc.Acquire(bg, dev, e.setup("b", true))
	if _, err := e.svc.Release(bg, dev, h1.ID, ReleaseOpts{Reason: "done"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Release(bg, admin, h2.ID, ReleaseOpts{Force: true, Reason: "cleanup"}); err != nil {
		t.Fatal(err)
	}
	var rel []hourglass.Event
	for _, ev := range e.rec.events {
		if ev.Type == hourglass.EventHoldReleased {
			rel = append(rel, ev)
		}
	}
	if len(rel) != 2 {
		t.Fatalf("released events %d", len(rel))
	}
	if rel[0].Data["forced"] != false || rel[0].Data["reason"] != "done" || rel[0].User.ID != dev || rel[0].Hold.ReleasedAt == nil || rel[0].Setup.Name != "a" {
		t.Fatalf("event 0 %+v data %v", rel[0], rel[0].Data)
	}
	if rel[1].Data["forced"] != true || rel[1].Data["reason"] != "cleanup" || rel[1].User.ID != dev {
		t.Fatalf("event 1 data %v", rel[1].Data)
	}
	want := []string{"hold.acquiring", "hold.acquired", "hold.acquiring", "hold.acquired", "hold.released", "hold.released"}
	got := e.rec.types()
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order %v", got)
		}
	}
}

func TestHOLD1_ReleasedSetupCanBeAcquiredAgain(t *testing.T) {
	e := newEnv(t)
	a := e.user("a", 10, store.RoleDeveloper)
	b := e.user("b", 10, store.RoleDeveloper)
	s := e.setup("s", true)
	h, err := e.svc.Acquire(bg, a, s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Release(bg, a, h.ID, ReleaseOpts{}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Acquire(bg, b, s); err != nil {
		t.Fatal(err)
	}
}

// gateDispatcher holds every Veto until n callers are inside it, so all of
// them are past the pre-check before any creates its hold.
type gateDispatcher struct {
	n       int
	mu      sync.Mutex
	in      int
	release chan struct{}
}

func (g *gateDispatcher) Veto(context.Context, hourglass.Event) error {
	g.mu.Lock()
	g.in++
	if g.in == g.n {
		close(g.release)
	}
	g.mu.Unlock()
	<-g.release
	return nil
}
func (g *gateDispatcher) Emit(hourglass.Event) {}

// HOLD-1 (finding 6, mutation M4): two acquires both pass the pre-check
// (one hold of a limit of two) and both are past the veto before either
// writes; the in-transaction re-check must still keep the user at the limit.
func TestHOLD1_LimitRecheckedInTransactionDeterministic(t *testing.T) {
	e := newEnv(t)
	g := &gateDispatcher{n: 2, release: make(chan struct{})}
	e.build(g)
	u := e.user("u", 10, store.RoleDeveloper)
	if _, err := e.st.CreateHold(bg, e.setup("held", true), u, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	s1, s2 := e.setup("a", true), e.setup("b", true)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, s := range []int64{s1, s2} {
		wg.Add(1)
		go func(i int, s int64) { defer wg.Done(); _, errs[i] = e.svc.Acquire(bg, u, s) }(i, s)
	}
	wg.Wait()
	if n := e.countHolds(u); n != 2 {
		t.Fatalf("holds = %d, want the limit 2 (errors %v)", n, errs)
	}
	won := 0
	for _, err := range errs {
		if err == nil {
			won++
		} else if CodeOf(err) != CodeHoldLimit {
			t.Fatalf("loser got %v", err)
		}
	}
	if won != 1 {
		t.Fatalf("winners = %d, want 1", won)
	}
}
