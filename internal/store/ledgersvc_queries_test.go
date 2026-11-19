// SPDX-License-Identifier: MIT

package store

import (
	"fmt"
	"testing"
)

func TestLED3_ServiceQueries(t *testing.T) {
	s := openTemp(t)
	if p, err := s.LatestAllowancePeriod(ctx); err != nil || p != "" {
		t.Fatalf("empty ledger: %q %v", p, err)
	}
	dev, none := newUser(t, s, "dev"), newUser(t, s, "none")
	admin := must(s.CreateLocalAdmin(ctx, "admin", "hash", false, t0))
	must0(s.SetUserRoles(ctx, dev.ID, []string{RoleDeveloper}))
	must0(s.SetUserRoles(ctx, admin.ID, []string{RoleAdmin}))
	_ = none
	for _, c := range []struct{ p, typ string }{{"2026-10", "allowance"}, {"2026-11", "grant"}, {"2026-09", "allowance"}} {
		must(s.AppendLedger(ctx, LedgerEntry{UserID: dev.ID, Period: c.p, Type: c.typ, AmountMT: 1}, "2026-11", t0))
	}
	if p, _ := s.LatestAllowancePeriod(ctx); p != "2026-10" {
		t.Errorf("latest allowance %q", p)
	}
	if n, _ := s.CountLedger(ctx, dev.ID, "2026-10", "allowance"); n != 1 {
		t.Errorf("count %d", n)
	}
	if n, _ := s.CountLedger(ctx, dev.ID, "2026-11", "allowance"); n != 0 {
		t.Errorf("count %d", n)
	}
	us, err := s.ListRoleHolders(ctx)
	if err != nil || len(us) != 1 || us[0].ID != dev.ID {
		t.Errorf("role holders %+v %v", us, err)
	}
}

func must0(err error) {
	if err != nil {
		panic(err)
	}
}

// RebuildBalances sets every cache to the current-period ledger sum (zero for
// a user without entries), reports the change, and costs the same number of
// statements whatever the number of users (one grouped statement, LED-2).
func TestLED2_RebuildBalancesGroupedAndReported(t *testing.T) {
	s := openTemp(t)
	cost := func(n int) (int64, []User) {
		var us []User
		for i := 0; i < n; i++ {
			u := newUser(t, s, fmt.Sprintf("u%d-%d", n, i))
			must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: "allowance", AmountMT: 300000}, "2026-11", t0))
			must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-12", Type: "grant", AmountMT: int64(1000 * (i + 1))}, "2026-11", t0))
			us = append(us, u)
		}
		before := s.StatementCount()
		rep, err := s.RebuildBalances(ctx, "2026-12")
		if err != nil {
			t.Fatal(err)
		}
		if rep.Changed == 0 {
			t.Fatalf("nothing changed for %d users", n)
		}
		return s.StatementCount() - before, us
	}
	small, _ := cost(2)
	large, us := cost(40)
	if small != large {
		t.Errorf("statements: %d for 2 users, %d for 40 more: not one grouped statement", small, large)
	}
	for i, u := range us {
		got := must(s.GetUser(ctx, u.ID)).BalanceMT
		if want := int64(1000 * (i + 1)); got != want {
			t.Errorf("user %d cache %d, want %d", u.ID, got, want)
		}
	}
	// A user with no entry in the period gets zero; the report counts and sums
	// the changes.
	none := newUser(t, s, "none")
	must(s.AppendLedger(ctx, LedgerEntry{UserID: none.ID, Period: "2026-12", Type: "allowance", AmountMT: 7000}, "2026-12", t0))
	must(s.AppendLedger(ctx, LedgerEntry{UserID: us[0].ID, Period: "2026-12", Type: "grant", AmountMT: 500}, "2026-12", t0))
	rep := must(s.RebuildBalances(ctx, "2027-01"))
	var abs int64 = 7000
	for i := range us {
		abs += int64(1000 * (i + 1))
	}
	abs += 500 + 2000 + 1000 // us[0]'s extra grant and the two first users of the small set
	if rep.Changed != len(us)+3 || rep.AbsDeltaMT != abs || rep.NetDeltaMT != -abs {
		t.Errorf("report %+v, want %d changed, %d abs", rep, len(us)+3, abs)
	}
	if got := must(s.GetUser(ctx, none.ID)).BalanceMT; got != 0 {
		t.Errorf("no entry in 2027-01: cache %d", got)
	}
	if rep := must(s.RebuildBalances(ctx, "2027-01")); rep != (BalanceRepair{}) {
		t.Errorf("second rebuild changed %+v", rep)
	}
}
