// SPDX-License-Identifier: MIT

package store

import (
	"errors"
	"testing"
	"time"
)

func TestADM1_LedgerStatsAndTotals(t *testing.T) {
	s := openTemp(t)
	a, b := newUser(t, s, "a"), newUser(t, s, "b")
	add := func(u User, period, typ string, mt int64, actor *int64) LedgerEntry {
		return must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: period, Type: typ, AmountMT: mt, ActorID: actor}, "2026-11", t0))
	}
	add(a, "2026-10", "allowance", 5000, nil)
	add(a, "2026-11", "allowance", 300000, nil)
	add(a, "2026-11", "debit", -2500, nil)
	g := add(a, "2026-11", "grant", 10000, &b.ID)
	add(b, "2026-11", "grant", -4000, &a.ID)

	stats := must(s.LedgerStats(ctx, "2026-11", 0, ""))
	if len(stats) != 2 || stats[0].UserID != a.ID || stats[0].BalanceMT != 307500 || stats[0].DebitedMT != 2500 || stats[0].Entries != 1 {
		t.Fatalf("stats %+v", stats)
	}
	if stats[1].BalanceMT != -4000 || stats[1].DebitedMT != 0 || stats[1].Entries != 1 {
		t.Fatalf("stats[1] %+v", stats[1])
	}
	one := must(s.LedgerStats(ctx, "2026-11", b.ID, ""))
	if len(one) != 1 || one[0].UserID != b.ID {
		t.Fatalf("filtered stats %+v", one)
	}
	tot := must(s.LedgerPeriodTotals(ctx, "2026-11"))
	if tot.DebitedMT != 2500 || tot.GrantedMT != 6000 || tot.GrantCount != 2 {
		t.Fatalf("totals %+v", tot)
	}
	if e := must(s.LatestLedgerEntry(ctx, a.ID, "grant", b.ID)); e.ID != g.ID {
		t.Fatalf("latest entry %+v", e)
	}
	if _, err := s.LatestLedgerEntry(ctx, a.ID, "reset", b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing entry: %v", err)
	}
	if n := must(s.BalanceThrough(ctx, a.ID, "2026-11", g.ID)); n != 307500 {
		t.Fatalf("balance through grant = %d", n)
	}
	if n := must(s.BalanceThrough(ctx, a.ID, "2026-11", g.ID-1)); n != 297500 {
		t.Fatalf("balance before grant = %d", n)
	}
}

func TestADM1_RolesAndHoldCounts(t *testing.T) {
	s := openTemp(t)
	a, b := newUser(t, s, "a"), newUser(t, s, "b")
	must0(s.SetUserRoles(ctx, a.ID, []string{RoleDeveloper, RoleAdmin}))
	must0(s.SetUserRoles(ctx, b.ID, []string{RoleDeveloper}))
	roles := must(s.AllUserRoles(ctx))
	if len(roles) != 2 || len(roles[a.ID]) != 2 || roles[b.ID][0] != RoleDeveloper {
		t.Fatalf("roles %+v", roles)
	}
	if n := must(s.CountUsersWithRole(ctx, RoleAdmin)); n != 1 {
		t.Fatalf("admins %d", n)
	}
	sid := newSetup(t, s, "s1")
	if n := must(s.CountHoldsForSetup(ctx, sid)); n != 0 {
		t.Fatalf("holds %d", n)
	}
	h := must(s.CreateHold(ctx, sid, a.ID, t0))
	must0(s.ReleaseHold(ctx, h.ID, &a.ID, "", t0))
	must(s.CreateHold(ctx, sid, b.ID, t0))
	if n := must(s.CountHoldsForSetup(ctx, sid)); n != 2 {
		t.Fatalf("holds %d", n)
	}
}

func TestADM3_AuditCountAndUsageReason(t *testing.T) {
	s := openTemp(t)
	a := newUser(t, s, "a")
	for _, act := range []string{"x", "x", "y"} {
		must(s.AppendAudit(ctx, AuditEntry{ActorID: &a.ID, Action: act, Target: "t"}, t0))
	}
	if n := must(s.CountAudit(ctx, AuditFilter{})); n != 3 {
		t.Fatalf("all %d", n)
	}
	if n := must(s.CountAudit(ctx, AuditFilter{Action: "x", ActorID: &a.ID, Limit: 1})); n != 2 {
		t.Fatalf("x %d", n)
	}
	sid := newSetup(t, s, "s1")
	h := must(s.CreateHold(ctx, sid, a.ID, t0))
	if r := must(s.LatestUsageReason(ctx, h.ID)); r != "" {
		t.Fatalf("reason %q", r)
	}
	for i, r := range []string{"first", "second"} {
		must(s.InsertUsageSample(ctx, UsageSample{HoldID: h.ID, PolledAt: t0.Add(time.Duration(i) * time.Minute), Weight: 1, Multiplier: 1, Reason: r}))
	}
	if r := must(s.LatestUsageReason(ctx, h.ID)); r != "second" {
		t.Fatalf("reason %q", r)
	}
}

func TestADM4_CollectorQueries(t *testing.T) {
	s := openTemp(t)
	for i, ok := range []bool{true, false, true, false} {
		at := t0.Add(time.Duration(i) * time.Hour)
		id := must(s.StartCollectorRun(ctx, at))
		must0(s.FinishCollectorRun(ctx, id, at.Add(time.Second), ok, map[bool]string{true: "", false: "boom"}[ok]))
	}
	runs := must(s.ListCollectorRuns(ctx, 3))
	if len(runs) != 3 || runs[0].StartedAt.Before(runs[1].StartedAt) {
		t.Fatalf("runs %+v", runs)
	}
	if f := must(s.LastFailedRun(ctx)); f.Error != "boom" || !f.StartedAt.Equal(t0.Add(3*time.Hour)) {
		t.Fatalf("failed run %+v", f)
	}
	for i := 0; i < 3; i++ {
		start := t0.Add(time.Duration(i) * 24 * time.Hour)
		must(s.InsertOutage(ctx, Outage{StartedAt: start, EndedAt: start.Add(30 * time.Minute), Reason: "gap"}))
	}
	out := must(s.ListOutagesSince(ctx, t0.Add(12*time.Hour)))
	if len(out) != 2 || out[0].StartedAt.Before(out[1].StartedAt) {
		t.Fatalf("outages %+v", out)
	}
	for i, ok := range []bool{true, false, false, false} {
		id := must(s.InsertEventDelivery(ctx, "hold.acquired", "h", "{}", t0.Add(time.Duration(i)*time.Minute)))
		must0(s.FinishEventDelivery(ctx, id, t0.Add(time.Duration(i)*time.Minute), ok, "err"))
	}
	must(s.InsertEventDelivery(ctx, "hold.acquired", "h", "{}", t0.Add(time.Hour))) // unfinished
	fd := must(s.ListFailedDeliveries(ctx, 2))
	if len(fd) != 2 || fd[0].StartedAt.Before(fd[1].StartedAt) || (fd[0].OK == nil || *fd[0].OK) {
		t.Fatalf("failed deliveries %+v", fd)
	}
	if _, err := s.LastFailedRun(ctx); err != nil {
		t.Fatal(err)
	}
}
