// SPDX-License-Identifier: MIT

package store

import (
	"errors"
	"testing"
	"time"
)

// Store queries of the preapprovals table (docs/design/ledger-labels-onboarding.md, D2).
func TestPRE5_PreapprovalQueries(t *testing.T) {
	s := openTemp(t)
	admin := newUser(t, s, "admin-sub")
	user := newUser(t, s, "user-sub")
	created, err := s.UpsertPreapproval(ctx, "b@example.test", []string{RoleDeveloper}, admin.ID, t0)
	if err != nil || !created {
		t.Fatalf("add: %v %v", created, err)
	}
	if created, err := s.UpsertPreapproval(ctx, "a@example.test", []string{RoleAdmin}, admin.ID, t0); err != nil || !created {
		t.Fatalf("add a: %v %v", created, err)
	}
	b, err := s.GetPreapprovalByEmail(ctx, "b@example.test")
	if err != nil || b.Email != "b@example.test" || len(b.Roles) != 1 || b.CreatedBy == nil || *b.CreatedBy != admin.ID ||
		!b.CreatedAt.Equal(t0) || b.UsedAt != nil || b.UsedBy != nil {
		t.Fatalf("get %+v %v", b, err)
	}
	if _, err := s.GetPreapprovalByEmail(ctx, "B@example.test"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the store compares exactly; normalising is the caller's: %v", err)
	}
	marked, err := s.MarkPreapprovalUsed(ctx, b.ID, user.ID, t0.Add(time.Minute))
	if err != nil || !marked {
		t.Fatalf("mark: %v %v", marked, err)
	}
	if marked, err := s.MarkPreapprovalUsed(ctx, b.ID, admin.ID, t0.Add(time.Hour)); err != nil || marked {
		t.Fatalf("second mark: %v %v", marked, err)
	}
	// An upsert replaces the roles only.
	if created, err := s.UpsertPreapproval(ctx, "b@example.test", []string{RoleAdmin, RoleDeveloper}, user.ID, t0.Add(2*time.Hour)); err != nil || created {
		t.Fatalf("upsert: %v %v", created, err)
	}
	b2, _ := s.GetPreapprovalByEmail(ctx, "b@example.test")
	if b2.ID != b.ID || *b2.CreatedBy != admin.ID || !b2.CreatedAt.Equal(t0) || *b2.UsedBy != user.ID ||
		!b2.UsedAt.Equal(t0.Add(time.Minute)) || len(b2.Roles) != 2 {
		t.Fatalf("after upsert %+v", b2)
	}
	all, err := s.ListPreapprovals(ctx)
	if err != nil || len(all) != 2 || all[0].Email != "a@example.test" || all[1].Email != "b@example.test" {
		t.Fatalf("list %+v %v", all, err)
	}
	gone, err := s.DeletePreapproval(ctx, b.ID)
	if err != nil || gone.Email != "b@example.test" {
		t.Fatalf("delete %+v %v", gone, err)
	}
	if _, err := s.DeletePreapproval(ctx, b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	// The email column is unique.
	if _, err := s.sqldb.Exec(`INSERT INTO preapprovals (email, roles_json, created_at) VALUES ('a@example.test', '["developer"]', 'x')`); err == nil {
		t.Fatal("duplicate email stored")
	}
}
