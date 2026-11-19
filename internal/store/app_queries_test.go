// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestAUTH6_TouchLocalAdminLoginSetsOnlyLocalUser(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	admin, err := s.CreateLocalAdmin(ctx, "admin", "hash", true, now)
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateOIDCUser(ctx, "sub-1", "a@example.test", "A", now)
	if err != nil {
		t.Fatal(err)
	}
	if admin.LastLoginAt != nil {
		t.Fatalf("fresh admin has last_login_at %v", admin.LastLoginAt)
	}
	at := now.Add(time.Hour)
	if err := s.TouchLocalAdminLogin(ctx, at); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetUser(ctx, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastLoginAt == nil || !got.LastLoginAt.Equal(at) {
		t.Fatalf("admin last_login_at = %v, want %v", got.LastLoginAt, at)
	}
	o, err := s.GetUser(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if (o.LastLoginAt == nil) != (other.LastLoginAt == nil) ||
		(o.LastLoginAt != nil && !o.LastLoginAt.Equal(*other.LastLoginAt)) {
		t.Fatalf("oidc user was touched: before %v, after %v", other.LastLoginAt, o.LastLoginAt)
	}
}

func TestOBS_SumAllDebitsSince(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	t0 := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	if n, err := s.SumAllDebitsSince(ctx, t0); err != nil || n != 0 {
		t.Fatalf("empty: %d %v", n, err)
	}
	u, err := s.CreateOIDCUser(ctx, "s", "e@example.test", "E", t0)
	if err != nil {
		t.Fatal(err)
	}
	setup, err := s.CreateSetup(ctx, Setup{Name: "s1", DetailsJSON: "{}", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	h, err := s.CreateHold(ctx, setup, u.ID, t0)
	if err != nil {
		t.Fatal(err)
	}
	for i, d := range []int64{100, 250, 40} {
		if _, err := s.InsertUsageSample(ctx, UsageSample{HoldID: h.ID, PolledAt: t0.Add(time.Duration(i) * time.Minute), DebitMT: d}); err != nil {
			t.Fatal(err)
		}
	}
	if n, _ := s.SumAllDebitsSince(ctx, t0); n != 390 {
		t.Fatalf("all = %d, want 390", n)
	}
	if n, _ := s.SumAllDebitsSince(ctx, t0.Add(time.Minute)); n != 290 {
		t.Fatalf("since +1m = %d, want 290", n)
	}
}
