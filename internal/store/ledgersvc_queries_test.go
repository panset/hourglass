// SPDX-License-Identifier: MIT

package store

import "testing"

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
