// SPDX-License-Identifier: MIT

package ledger

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

var bg = context.Background()

type rec struct {
	mu sync.Mutex
	ev []hourglass.Event
}

func (r *rec) Emit(e hourglass.Event) { r.mu.Lock(); r.ev = append(r.ev, e); r.mu.Unlock() }
func (r *rec) types() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, e := range r.ev {
		out = append(out, e.Type)
	}
	return out
}

type env struct {
	t   *testing.T
	st  *store.Store
	clk *hourglasstest.FakeClock
	rec *rec
	svc *Service
}

func newEnv(t *testing.T, loc *time.Location, start time.Time) *env {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	clk := hourglasstest.NewFakeClock(start)
	r := &rec{}
	return &env{t, st, clk, r, NewService(st, r, clk, loc)}
}

func (e *env) user(sub string, roles ...string) int64 {
	e.t.Helper()
	u, err := e.st.CreateOIDCUser(bg, sub, sub+"@example.test", sub, e.clk.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	if len(roles) > 0 {
		if err := e.st.SetUserRoles(bg, u.ID, roles); err != nil {
			e.t.Fatal(err)
		}
	}
	return u.ID
}

// seed writes an entry for the current period without events.
func (e *env) seed(user int64, period string, typ EntryType, mt int64) {
	e.t.Helper()
	if _, err := e.st.AppendLedger(bg, store.LedgerEntry{UserID: user, Period: period, Type: string(typ), AmountMT: mt}, e.svc.CurrentPeriod(), e.clk.Now()); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) sum(user int64, period string) int64 {
	n, err := e.st.SumLedger(bg, user, period, "")
	if err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *env) cache(user int64) int64 {
	u, err := e.st.GetUser(bg, user)
	if err != nil {
		e.t.Fatal(err)
	}
	return u.BalanceMT
}

func (e *env) setAllowance(mt int64) {
	e.t.Helper()
	v := strconv.FormatFloat(float64(mt)/1000, 'f', -1, 64)
	if err := e.st.PutSetting(bg, settings.KeyDefaultAllowance, v, nil, e.clk.Now()); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) admin() int64 {
	a, err := e.st.CreateLocalAdmin(bg, "admin", "x", false, e.clk.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.st.SetUserRoles(bg, a.ID, []string{store.RoleAdmin}); err != nil {
		e.t.Fatal(err)
	}
	return a.ID
}

var nov = time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)

func TestLED3_ServiceRolloverBalances(t *testing.T) {
	for _, c := range []struct {
		name    string
		closing int64
		want    int64
	}{{"positive expires", 120 * tok, 300 * tok}, {"zero", 0, 300 * tok}, {"negative carries over (spec example)", -40 * tok, 260 * tok}} {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, time.UTC, nov)
			u := e.user("a", store.RoleDeveloper)
			e.seed(u, "2026-11", TypeAllowance, 300*tok)
			e.seed(u, "2026-11", TypeDebit, c.closing-300*tok)
			if got := e.sum(u, "2026-11"); got != c.closing {
				t.Fatalf("setup balance %d", got)
			}
			e.clk.Set(time.Date(2026, 12, 1, 0, 0, 5, 0, time.UTC))
			closed, err := e.svc.RolloverDue(bg)
			if err != nil || len(closed) != 1 || closed[0] != "2026-11" {
				t.Fatalf("closed %v %v", closed, err)
			}
			if got := e.sum(u, "2026-12"); got != c.want {
				t.Errorf("new period %d want %d", got, c.want)
			}
			if e.cache(u) != c.want {
				t.Errorf("cache %d want %d", e.cache(u), c.want)
			}
			if c.closing > 0 && e.sum(u, "2026-11") != 0 {
				t.Errorf("old period not zeroed: %d", e.sum(u, "2026-11"))
			}
		})
	}
}

func TestLED3_ServicePeriodClosedEvent(t *testing.T) {
	e := newEnv(t, time.UTC, nov)
	u1, u2 := e.user("a", store.RoleDeveloper), e.user("b", store.RoleDeveloper)
	e.seed(u1, "2026-11", TypeAllowance, 300*tok)
	e.seed(u2, "2026-11", TypeAllowance, 300*tok)
	e.clk.Set(time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC))
	if _, err := e.svc.RolloverDue(bg); err != nil {
		t.Fatal(err)
	}
	if len(e.rec.ev) != 1 || e.rec.ev[0].Type != hourglass.EventPeriodClosed {
		t.Fatalf("events %v", e.rec.types())
	}
	d := e.rec.ev[0].Data
	if d["period"] != "2026-11" || d["next"] != "2026-12" || d["users"] != 2 {
		t.Errorf("data %v", d)
	}
}

func TestLED3_ServiceTwoSkippedMonthsCarryDebt(t *testing.T) {
	e := newEnv(t, time.UTC, nov)
	e.setAllowance(100 * tok)
	u := e.user("a", store.RoleDeveloper)
	e.seed(u, "2026-11", TypeAllowance, 100*tok)
	e.seed(u, "2026-11", TypeDebit, -600*tok) // closing -500
	e.clk.Set(time.Date(2027, 1, 5, 0, 0, 0, 0, time.UTC))
	closed, err := e.svc.RolloverDue(bg)
	if err != nil || len(closed) != 2 || closed[0] != "2026-11" || closed[1] != "2026-12" {
		t.Fatalf("closed %v %v", closed, err)
	}
	if got := e.sum(u, "2026-12"); got != -400*tok {
		t.Errorf("december %d", got)
	}
	if got := e.sum(u, "2027-01"); got != -300*tok {
		t.Errorf("january %d", got)
	}
	if got := e.cache(u); got != -300*tok {
		t.Errorf("cache %d", got)
	}
	if got := e.types(); got != 2 {
		t.Errorf("period.closed events %d", got)
	}
}

func (e *env) types() int {
	n := 0
	for _, s := range e.rec.types() {
		if s == hourglass.EventPeriodClosed {
			n++
		}
	}
	return n
}

func TestLED3_ServiceIdempotentAndEmptyLedger(t *testing.T) {
	e := newEnv(t, time.UTC, nov)
	u := e.user("a", store.RoleDeveloper)
	if closed, err := e.svc.RolloverDue(bg); err != nil || len(closed) != 0 {
		t.Fatalf("fresh install: %v %v", closed, err)
	}
	e.seed(u, "2026-11", TypeAllowance, 300*tok)
	e.clk.Set(time.Date(2026, 12, 3, 0, 0, 0, 0, time.UTC))
	if closed, _ := e.svc.RolloverDue(bg); len(closed) != 1 {
		t.Fatalf("first run %v", closed)
	}
	before, _ := e.st.ListLedger(bg, u, "2026-12")
	closed, err := e.svc.RolloverDue(bg)
	after, _ := e.st.ListLedger(bg, u, "2026-12")
	if err != nil || len(closed) != 0 || len(before) != len(after) || e.types() != 1 {
		t.Errorf("second run wrote: %v %v %d/%d events %d", closed, err, len(before), len(after), e.types())
	}
}

func TestLED3_ServiceSkipsNoRoleAndLocalAdmin(t *testing.T) {
	e := newEnv(t, time.UTC, nov)
	dev := e.user("dev", store.RoleDeveloper)
	norole := e.user("none")
	admin := e.admin()
	e.seed(dev, "2026-11", TypeAllowance, 300*tok)
	e.clk.Set(time.Date(2026, 12, 1, 1, 0, 0, 0, time.UTC))
	if _, err := e.svc.RolloverDue(bg); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{norole, admin} {
		if n := e.sum(id, "2026-12"); n != 0 {
			t.Errorf("user %d got %d", id, n)
		}
		if es, _ := e.st.ListLedger(bg, id, "2026-12"); len(es) != 0 {
			t.Errorf("user %d has entries", id)
		}
	}
	if e.sum(dev, "2026-12") != 300*tok {
		t.Error("developer missing allowance")
	}
	if e.rec.ev[0].Data["users"] != 1 {
		t.Errorf("users %v", e.rec.ev[0].Data["users"])
	}
}

func TestLED3_ServiceAllowanceChangeAppliesNextRollover(t *testing.T) {
	e := newEnv(t, time.UTC, nov)
	u := e.user("a", store.RoleDeveloper)
	e.seed(u, "2026-11", TypeAllowance, 300*tok)
	e.setAllowance(500 * tok)
	if e.sum(u, "2026-11") != 300*tok {
		t.Fatal("changing the setting must not touch the current period")
	}
	e.clk.Set(time.Date(2026, 12, 1, 0, 1, 0, 0, time.UTC))
	if _, err := e.svc.RolloverDue(bg); err != nil {
		t.Fatal(err)
	}
	if got := e.sum(u, "2026-12"); got != 500*tok {
		t.Errorf("december %d", got)
	}
}

func TestLED3_ServiceTimezoneBoundary(t *testing.T) {
	la := mustLoc(t, "America/Los_Angeles")
	// 2026-12-01T07:30Z is still 23:30 on Nov 30 in Los Angeles.
	e := newEnv(t, la, time.Date(2026, 12, 1, 7, 30, 0, 0, time.UTC))
	u := e.user("a", store.RoleDeveloper)
	if e.svc.CurrentPeriod() != "2026-11" {
		t.Fatalf("period %s", e.svc.CurrentPeriod())
	}
	e.seed(u, "2026-11", TypeAllowance, 300*tok)
	if closed, _ := e.svc.RolloverDue(bg); len(closed) != 0 {
		t.Fatalf("closed too early: %v", closed)
	}
	e.clk.Set(time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC))
	if e.svc.CurrentPeriod() != "2026-12" {
		t.Fatal("period should have changed")
	}
	if closed, _ := e.svc.RolloverDue(bg); len(closed) != 1 || closed[0] != "2026-11" {
		t.Fatalf("closed %v", closed)
	}
	if e.sum(u, "2026-12") != 300*tok {
		t.Error("allowance missing")
	}
}

func TestROLE_ServiceFirstAllowanceOncePerPeriod(t *testing.T) {
	e := newEnv(t, time.UTC, nov)
	e.setAllowance(250 * tok)
	u := e.user("a")
	admin := e.admin()
	norole := e.user("b")
	ensure := func(id int64) {
		t.Helper()
		if err := e.st.Tx(bg, func(tx *store.Tx) error { return e.svc.EnsureFirstAllowanceTx(tx, id, e.clk.Now()) }); err != nil {
			t.Fatal(err)
		}
	}
	ensure(u) // no role yet
	if e.sum(u, "2026-11") != 0 {
		t.Error("no role: no allowance")
	}
	if err := e.st.SetUserRoles(bg, u, []string{store.RoleDeveloper}); err != nil {
		t.Fatal(err)
	}
	ensure(u)
	ensure(u)
	if es, _ := e.st.ListLedger(bg, u, "2026-11"); len(es) != 1 || es[0].AmountMT != 250*tok {
		t.Errorf("entries %+v", es)
	}
	ensure(admin)
	ensure(norole)
	for _, id := range []int64{admin, norole} {
		if es, _ := e.st.ListLedger(bg, id, "2026-11"); len(es) != 0 {
			t.Errorf("user %d got an allowance", id)
		}
	}
	if e.cache(u) != 250*tok {
		t.Errorf("cache %d", e.cache(u))
	}
}

func TestROLE_ServiceFirstAllowanceDefersToPendingRollover(t *testing.T) {
	e := newEnv(t, time.UTC, nov)
	old := e.user("a", store.RoleDeveloper)
	e.seed(old, "2026-11", TypeAllowance, 300*tok)
	e.clk.Set(time.Date(2026, 12, 1, 0, 0, 1, 0, time.UTC))
	late := e.user("b", store.RoleDeveloper)
	if err := e.st.Tx(bg, func(tx *store.Tx) error { return e.svc.EnsureFirstAllowanceTx(tx, late, e.clk.Now()) }); err != nil {
		t.Fatal(err)
	}
	if e.sum(late, "2026-12") != 0 {
		t.Fatal("allowance written while a rollover is pending")
	}
	if closed, _ := e.svc.RolloverDue(bg); len(closed) != 1 {
		t.Fatalf("rollover hidden: %v", closed)
	}
	if e.sum(late, "2026-12") != 300*tok || e.sum(old, "2026-12") != 300*tok {
		t.Error("both users should hold the allowance after rollover")
	}
}

func TestLED4_ServiceGrant(t *testing.T) {
	e := newEnv(t, time.UTC, nov)
	u := e.user("a", store.RoleDeveloper)
	admin := e.admin()
	norole := e.user("n")
	e.seed(u, "2026-11", TypeAllowance, 300*tok)

	ent, err := e.svc.Grant(bg, admin, u, 50*tok, "hackathon")
	if err != nil || ent.Type != TypeGrant || ent.ActorID == nil || *ent.ActorID != admin || ent.Reason != "hackathon" {
		t.Fatalf("grant %+v %v", ent, err)
	}
	if e.sum(u, "2026-11") != 350*tok || e.cache(u) != 350*tok {
		t.Error("balance")
	}
	if _, err := e.svc.Grant(bg, admin, u, -10*tok, "correction"); err != nil {
		t.Errorf("negative grant: %v", err)
	}
	if _, err := e.svc.Grant(bg, admin, u, 5, "  "); !errors.Is(err, ErrReasonRequired) {
		t.Errorf("blank reason: %v", err)
	}
	if _, err := e.svc.Grant(bg, admin, u, 0, "x"); !errors.Is(err, ErrZeroAmount) {
		t.Errorf("zero: %v", err)
	}
	if _, err := e.svc.Grant(bg, admin, admin, 5, "x"); !errors.Is(err, ErrLocalAdmin) {
		t.Errorf("local admin: %v", err)
	}
	if _, err := e.svc.Grant(bg, admin, norole, 5, "x"); !errors.Is(err, ErrNoRole) {
		t.Errorf("no role: %v", err)
	}
	rows, _ := e.st.ListAudit(bg, store.AuditFilter{Action: AuditGrant})
	if len(rows) != 2 || rows[0].ActorID == nil || *rows[0].ActorID != admin || rows[0].Target == "" {
		t.Errorf("audit rows %+v", rows)
	}
	// Rejected calls write nothing.
	if es, _ := e.st.ListLedger(bg, u, "2026-11"); len(es) != 3 {
		t.Errorf("entries %d", len(es))
	}
}

func TestLED5_ServiceReset(t *testing.T) {
	e := newEnv(t, time.UTC, nov)
	e.setAllowance(300 * tok)
	u := e.user("a", store.RoleDeveloper)
	admin := e.admin()
	e.seed(u, "2026-11", TypeCarryover, -40*tok)
	e.seed(u, "2026-11", TypeAllowance, 300*tok)
	e.seed(u, "2026-11", TypeDebit, -500*tok) // balance -240
	ent, err := e.svc.Reset(bg, admin, u, "start clean")
	if err != nil || ent.Type != TypeReset || ent.AmountMT != 540*tok {
		t.Fatalf("reset %+v %v", ent, err)
	}
	if e.sum(u, "2026-11") != 300*tok || e.cache(u) != 300*tok {
		t.Errorf("balance %d cache %d", e.sum(u, "2026-11"), e.cache(u))
	}
	if ent, err = e.svc.Reset(bg, admin, u, "again"); err != nil || ent.AmountMT != 0 {
		t.Errorf("no-op reset must still be written: %+v %v", ent, err)
	}
	if _, err := e.svc.Reset(bg, admin, u, ""); !errors.Is(err, ErrReasonRequired) {
		t.Errorf("reason: %v", err)
	}
	if _, err := e.svc.Reset(bg, admin, admin, "x"); !errors.Is(err, ErrLocalAdmin) {
		t.Errorf("local admin: %v", err)
	}
	if _, err := e.svc.Reset(bg, admin, e.user("n"), "x"); !errors.Is(err, ErrNoRole) {
		t.Errorf("no role: %v", err)
	}
	if rows, _ := e.st.ListAudit(bg, store.AuditFilter{Action: AuditReset}); len(rows) != 2 {
		t.Errorf("audit rows %d", len(rows))
	}
}

func TestLED_ServiceCrossingsEmittedOnce(t *testing.T) {
	e := newEnv(t, time.UTC, nov)
	e.setAllowance(300 * tok)
	u := e.user("a", store.RoleDeveloper)
	e.seed(u, "2026-11", TypeAllowance, 300*tok)
	h, err := e.st.CreateSetup(bg, store.Setup{Name: "s", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	hold, err := e.st.CreateHold(bg, h, u, e.clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	emitted := 0
	debit := func(mt int64) BalanceCrossings {
		var c BalanceCrossings
		err := e.st.Tx(bg, func(tx *store.Tx) error {
			var err error
			c, err = e.svc.DebitTx(tx, u, hold.ID, mt, e.clk.Now())
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(e.rec.ev) != emitted {
			t.Fatal("DebitTx must not emit")
		}
		e.svc.EmitEvents(c.Events...)
		emitted += len(c.Events)
		return c
	}
	if c := debit(100 * tok); c.Low || c.Overdrawn { // 200
		t.Errorf("no crossing expected %+v", c)
	}
	if c := debit(175 * tok); !c.Low || c.Overdrawn { // 25 < 30
		t.Errorf("low expected %+v", c)
	}
	if c := debit(5 * tok); c.Low || c.Overdrawn { // 20, already low
		t.Errorf("low must fire once %+v", c)
	}
	if c := debit(30 * tok); c.Low || !c.Overdrawn { // -10
		t.Errorf("overdrawn expected %+v", c)
	}
	if c := debit(30 * tok); c.Low || c.Overdrawn { // -40
		t.Errorf("overdrawn must fire once %+v", c)
	}
	got := e.rec.types()
	if len(got) != 2 || got[0] != hourglass.EventBalanceLow || got[1] != hourglass.EventBalanceOverdrawn {
		t.Fatalf("events %v", got)
	}
	d := e.rec.ev[0].Data
	if d["balance_tokens"] != 25.0 || d["allowance_tokens"] != 300.0 || e.rec.ev[0].User == nil || e.rec.ev[0].User.ID != u {
		t.Errorf("event %+v", e.rec.ev[0])
	}
	if b, _ := e.svc.Balance(bg, u); b != -40*tok || e.cache(u) != -40*tok {
		t.Errorf("balance %d", b)
	}
	if burn, _ := e.svc.MonthToDateBurn(bg, u); burn != 340*tok {
		t.Errorf("burn %d", burn)
	}
}

func TestLED_ServiceOneDebitBothEvents(t *testing.T) {
	e := newEnv(t, time.UTC, nov)
	e.setAllowance(300 * tok)
	u := e.user("a", store.RoleDeveloper)
	e.seed(u, "2026-11", TypeAllowance, 300*tok)
	s, _ := e.st.CreateSetup(bg, store.Setup{Name: "s", Enabled: true})
	h, _ := e.st.CreateHold(bg, s, u, e.clk.Now())
	var c BalanceCrossings
	if err := e.st.Tx(bg, func(tx *store.Tx) error {
		var err error
		c, err = e.svc.DebitTx(tx, u, h.ID, 400*tok, e.clk.Now())
		return err
	}); err != nil || !c.Low || !c.Overdrawn || len(c.Events) != 2 {
		t.Errorf("%+v %v", c, err)
	}
}

func TestLED4_ServiceGrantEmitsCrossing(t *testing.T) {
	e := newEnv(t, time.UTC, nov)
	e.setAllowance(300 * tok)
	u := e.user("a", store.RoleDeveloper)
	admin := e.admin()
	e.seed(u, "2026-11", TypeAllowance, 300*tok)
	if _, err := e.svc.Grant(bg, admin, u, -320*tok, "correction"); err != nil {
		t.Fatal(err)
	}
	if got := e.rec.types(); len(got) != 2 {
		t.Errorf("events %v", got)
	}
}

func TestLED_ServiceBurnLast24h(t *testing.T) {
	e := newEnv(t, time.UTC, nov)
	u := e.user("a", store.RoleDeveloper)
	s, _ := e.st.CreateSetup(bg, store.Setup{Name: "s", Enabled: true})
	h, _ := e.st.CreateHold(bg, s, u, nov.Add(-72*time.Hour))
	for _, c := range []struct {
		ago time.Duration
		mt  int64
	}{{30 * time.Hour, 900}, {23 * time.Hour, 100}, {time.Hour, 50}} {
		if _, err := e.st.InsertUsageSample(bg, store.UsageSample{HoldID: h.ID, PolledAt: nov.Add(-c.ago), DebitMT: c.mt}); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := e.svc.BurnLast24h(bg, u); err != nil || got != 150 {
		t.Errorf("burn %d %v", got, err)
	}
}
