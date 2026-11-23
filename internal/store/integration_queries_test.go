// SPDX-License-Identifier: MIT

package store

import (
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func newAccount(t *testing.T, s *Store, name string) ServiceAccount {
	t.Helper()
	return must(s.CreateServiceAccount(ctx, ServiceAccount{Name: name, Description: "d", Scopes: []string{"state:read"}, Enabled: true}, t0))
}

func TestServiceAccountQueries(t *testing.T) {
	s := openTemp(t)
	admin := newUser(t, s, "admin")
	a := must(s.CreateServiceAccount(ctx, ServiceAccount{Name: "deployer", Description: "x", Scopes: []string{"state:read", "pressure:write"}, Enabled: true, CreatedBy: &admin.ID}, t0))
	if a.ID == 0 || a.Name != "deployer" || !a.Enabled || !reflect.DeepEqual(a.Scopes, []string{"state:read", "pressure:write"}) ||
		a.CreatedBy == nil || *a.CreatedBy != admin.ID || !a.CreatedAt.Equal(t0) {
		t.Fatalf("created: %+v", a)
	}
	if _, err := s.CreateServiceAccount(ctx, ServiceAccount{Name: "deployer"}, t0); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate name: %v", err)
	}
	if g := must(s.GetServiceAccountByName(ctx, "deployer")); g.ID != a.ID {
		t.Errorf("by name: %+v", g)
	}
	if _, err := s.GetServiceAccount(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("get unknown: %v", err)
	}
	if _, err := s.GetServiceAccountByName(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("by name unknown: %v", err)
	}
	b := newAccount(t, s, "another")
	exp := t0.Add(time.Hour)
	tok1 := must(s.InsertServiceToken(ctx, a.ID, "hash1", &exp, t0))
	tok2 := must(s.InsertServiceToken(ctx, a.ID, "hash2", nil, t0.Add(time.Second)))
	if tok1.ExpiresAt == nil || !tok1.ExpiresAt.Equal(exp) || tok2.ExpiresAt != nil || tok2.LastUsedAt != nil || tok1.TokenHash != "hash1" {
		t.Errorf("tokens: %+v %+v", tok1, tok2)
	}
	if _, err := s.InsertServiceToken(ctx, 999, "h", nil, t0); !errors.Is(err, ErrNotFound) {
		t.Errorf("token for unknown account: %v", err)
	}
	list := must(s.ListServiceAccounts(ctx))
	if len(list) != 2 || list[0].Name != "another" || len(list[0].Tokens) != 0 || list[0].Tokens == nil ||
		list[1].Name != "deployer" || len(list[1].Tokens) != 2 || list[1].Tokens[0].ID != tok1.ID {
		t.Errorf("list: %+v", list)
	}
	_ = b

	u := must(s.UpdateServiceAccount(ctx, a.ID, "new", []string{"holds:release"}, false))
	if u.Description != "new" || u.Enabled || !reflect.DeepEqual(u.Scopes, []string{"holds:release"}) || u.Name != "deployer" {
		t.Errorf("updated: %+v", u)
	}
	if _, err := s.UpdateServiceAccount(ctx, 999, "", nil, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("update unknown: %v", err)
	}
	if u := must(s.UpdateServiceAccount(ctx, a.ID, "", nil, true)); u.Scopes == nil || len(u.Scopes) != 0 {
		t.Errorf("empty scopes: %#v", u.Scopes)
	}

	if err := s.DeleteServiceAccount(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete unknown: %v", err)
	}
}

func TestServiceTokenLookupRevokeTouch(t *testing.T) {
	s := openTemp(t)
	a := newAccount(t, s, "svc")
	other := newAccount(t, s, "other")
	tok := must(s.InsertServiceToken(ctx, a.ID, "hash", nil, t0))

	gt, ga, err := s.GetServiceTokenWithAccount(ctx, tok.ID)
	if err != nil || gt.TokenHash != "hash" || ga.Name != "svc" || !ga.Enabled || ga.Scopes[0] != "state:read" {
		t.Fatalf("lookup: %+v %+v %v", gt, ga, err)
	}
	if _, _, err := s.GetServiceTokenWithAccount(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("lookup unknown: %v", err)
	}

	// last_used_at: at most once a minute.
	if ok := must(s.TouchServiceToken(ctx, tok.ID, t0)); !ok {
		t.Error("first touch not written")
	}
	if ok := must(s.TouchServiceToken(ctx, tok.ID, t0.Add(59*time.Second))); ok {
		t.Error("touch within a minute written")
	}
	gt, _, _ = s.GetServiceTokenWithAccount(ctx, tok.ID)
	if gt.LastUsedAt == nil || !gt.LastUsedAt.Equal(t0) {
		t.Errorf("last_used_at = %v", gt.LastUsedAt)
	}
	if ok := must(s.TouchServiceToken(ctx, tok.ID, t0.Add(time.Minute))); !ok {
		t.Error("touch after a minute not written")
	}

	if err := s.RevokeServiceToken(ctx, other.ID, tok.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoke under wrong account: %v", err)
	}
	if err := s.RevokeServiceToken(ctx, a.ID, tok.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.GetServiceTokenWithAccount(ctx, tok.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoked token found: %v", err)
	}
	if err := s.RevokeServiceToken(ctx, a.ID, tok.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoke twice: %v", err)
	}
}

func TestServiceAccountDeleteCascadesAndKeepsHistory(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	setup := newSetup(t, s, "lab")
	a := newAccount(t, s, "svc")
	tok := must(s.InsertServiceToken(ctx, a.ID, "hash", nil, t0))
	h := must(s.CreateHold(ctx, setup, u.ID, t0))
	if err := s.ReleaseHoldByService(ctx, h.ID, "svc", "torn down", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	must(s.AppendAudit(ctx, AuditEntry{ActorService: "svc", Action: "hold.service_release", Target: "hold:1"}, t0))

	if err := s.DeleteServiceAccount(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.GetServiceTokenWithAccount(ctx, tok.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("token survived account delete: %v", err)
	}
	var n int
	s.sqldb.QueryRow(`SELECT COUNT(*) FROM service_tokens`).Scan(&n)
	if n != 0 {
		t.Errorf("service_tokens rows = %d", n)
	}
	if g := must(s.GetHold(ctx, h.ID)); g.ReleasedByService != "svc" {
		t.Errorf("hold lost the account name: %+v", g)
	}
	if e := must(s.ListAudit(ctx, AuditFilter{})); len(e) != 1 || e[0].ActorService != "svc" {
		t.Errorf("audit: %+v", e)
	}
}

func TestHoldReleaseRequestedAndService(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	h := must(s.CreateHold(ctx, newSetup(t, s, "lab"), u.ID, t0))
	if h.ReleaseRequestedAt != nil || h.ReleasedByService != "" {
		t.Fatalf("new hold: %+v", h)
	}
	if ok := must(s.SetReleaseRequested(ctx, h.ID, t0.Add(time.Minute))); !ok {
		t.Error("first request not set")
	}
	if ok := must(s.SetReleaseRequested(ctx, h.ID, t0.Add(time.Hour))); ok {
		t.Error("second request set again")
	}
	if _, err := s.SetReleaseRequested(ctx, 999, t0); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown hold: %v", err)
	}
	g := must(s.GetHold(ctx, h.ID))
	if g.ReleaseRequestedAt == nil || !g.ReleaseRequestedAt.Equal(t0.Add(time.Minute)) {
		t.Errorf("release_requested_at = %v", g.ReleaseRequestedAt)
	}
	if act := must(s.ActiveHolds(ctx)); len(act) != 1 || act[0].ReleaseRequestedAt == nil {
		t.Errorf("active holds scan: %+v", act)
	}
	if uh := must(s.UserHolds(ctx, u.ID)); len(uh) != 1 || uh[0].ReleaseRequestedAt == nil {
		t.Errorf("user holds scan: %+v", uh)
	}

	if err := s.ReleaseHoldByService(ctx, h.ID, "svc", "idle", t0.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	g = must(s.GetHold(ctx, h.ID))
	if g.ReleasedAt == nil || g.ReleasedBy != nil || g.ReleasedByService != "svc" || g.ReleaseReason != "idle" {
		t.Errorf("service release: %+v", g)
	}
	if err := s.ReleaseHoldByService(ctx, h.ID, "svc", "again", t0); !errors.Is(err, ErrConflict) {
		t.Errorf("release twice: %v", err)
	}
	if err := s.ReleaseHoldByService(ctx, 999, "svc", "", t0); !errors.Is(err, ErrNotFound) {
		t.Errorf("release unknown: %v", err)
	}
}

func TestAuditActorService(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	must(s.AppendAudit(ctx, AuditEntry{ActorService: "svc", Action: "hold.service_release", Target: "hold:7"}, t0))
	must(s.AppendAudit(ctx, AuditEntry{ActorID: &u.ID, Action: "x"}, t0))
	es := must(s.ListAudit(ctx, AuditFilter{}))
	if len(es) != 2 {
		t.Fatalf("entries: %+v", es)
	}
	if es[1].ActorService != "svc" || es[1].ActorID != nil || es[0].ActorService != "" || es[0].ActorID == nil {
		t.Errorf("actors: %+v", es)
	}
}

func TestReportedPressureQueries(t *testing.T) {
	s := openTemp(t)
	if _, err := s.GetReportedPressure(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("before first: %v", err)
	}
	p := ReportedPressure{Value: 0.25, ObservedAt: t0, ReceivedAt: t0.Add(time.Second), ServiceAccount: "svc"}
	if err := s.ReplaceReportedPressure(ctx, p); err != nil {
		t.Fatal(err)
	}
	if g := must(s.GetReportedPressure(ctx)); g != p {
		t.Errorf("got %+v want %+v", g, p)
	}
	p2 := ReportedPressure{Value: 1, ObservedAt: t0.Add(time.Minute), ReceivedAt: t0.Add(time.Minute), ServiceAccount: "svc2"}
	if err := s.ReplaceReportedPressure(ctx, p2); err != nil {
		t.Fatal(err)
	}
	if g := must(s.GetReportedPressure(ctx)); g != p2 {
		t.Errorf("after replace %+v", g)
	}
	var n int
	s.sqldb.QueryRow(`SELECT COUNT(*) FROM reported_pressure`).Scan(&n)
	if n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}

func TestReportedDeploymentsQueries(t *testing.T) {
	s := openTemp(t)
	if _, _, err := s.GetReportedDeployments(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("before first: %v", err)
	}
	h := ReportedHeader{ObservedAt: t0, ReceivedAt: t0, ServiceAccount: "svc"}
	set := []ReportedDeployment{{"lab-b", map[string]string{"c": "1"}}, {"lab-a", nil}}
	if err := s.ReplaceReportedDeployments(ctx, h, set); err != nil {
		t.Fatal(err)
	}
	gh, gs, err := s.GetReportedDeployments(ctx)
	want := []ReportedDeployment{{"lab-a", map[string]string{}}, {"lab-b", map[string]string{"c": "1"}}}
	if err != nil || gh != h || !reflect.DeepEqual(gs, want) {
		t.Fatalf("got %+v %+v %v", gh, gs, err)
	}

	// The empty set still has a header.
	h2 := ReportedHeader{ObservedAt: t0.Add(time.Minute), ReceivedAt: t0.Add(time.Minute), ServiceAccount: "svc"}
	if err := s.ReplaceReportedDeployments(ctx, h2, nil); err != nil {
		t.Fatal(err)
	}
	gh, gs, err = s.GetReportedDeployments(ctx)
	if err != nil || gh != h2 || gs == nil || len(gs) != 0 {
		t.Fatalf("empty set: %+v %#v %v", gh, gs, err)
	}

	// A duplicate name fails and leaves the previous report intact.
	h3 := ReportedHeader{ObservedAt: t0.Add(time.Hour), ReceivedAt: t0.Add(time.Hour), ServiceAccount: "x"}
	err = s.ReplaceReportedDeployments(ctx, h3, []ReportedDeployment{{SetupName: "a"}, {SetupName: "a"}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	if gh, gs, _ = s.GetReportedDeployments(ctx); gh != h2 || len(gs) != 0 {
		t.Errorf("failed replace changed state: %+v %+v", gh, gs)
	}
}

func TestReportedPersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.db")
	s := must(Open(path))
	must(s, s.ReplaceReportedPressure(ctx, ReportedPressure{Value: 0.5, ObservedAt: t0, ReceivedAt: t0, ServiceAccount: "svc"}))
	must(s, s.ReplaceReportedDeployments(ctx, ReportedHeader{ObservedAt: t0, ReceivedAt: t0, ServiceAccount: "svc"},
		[]ReportedDeployment{{SetupName: "lab", Labels: map[string]string{"k": "v"}}}))
	s.Close()
	s = must(Open(path))
	defer s.Close()
	if p := must(s.GetReportedPressure(ctx)); p.Value != 0.5 {
		t.Errorf("pressure: %+v", p)
	}
	if _, set, _ := s.GetReportedDeployments(ctx); len(set) != 1 || set[0].SetupName != "lab" {
		t.Errorf("set: %+v", set)
	}
}

// A concurrent reader of the deployed set never sees a header from one report
// with the set of another.
func TestReportedDeploymentsReplaceIsAtomic(t *testing.T) {
	s := openTemp(t)
	mk := func(i int) (ReportedHeader, []ReportedDeployment) {
		h := ReportedHeader{ObservedAt: t0.Add(time.Duration(i) * time.Second), ReceivedAt: t0, ServiceAccount: "svc"}
		var set []ReportedDeployment
		for j := 0; j < i%5; j++ {
			set = append(set, ReportedDeployment{SetupName: string(rune('a' + j))})
		}
		return h, set
	}
	h0, s0 := mk(0)
	must(s, s.ReplaceReportedDeployments(ctx, h0, s0))
	var wg sync.WaitGroup
	stop := make(chan struct{})
	bad := make(chan string, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			h, set, err := s.GetReportedDeployments(ctx)
			if err != nil {
				select {
				case bad <- err.Error():
				default:
				}
				return
			}
			i := int(h.ObservedAt.Sub(t0) / time.Second)
			if len(set) != i%5 {
				select {
				case bad <- "mixed report":
				default:
				}
				return
			}
		}
	}()
	for i := 1; i <= 60; i++ {
		h, set := mk(i)
		if err := s.ReplaceReportedDeployments(ctx, h, set); err != nil {
			t.Fatal(err)
		}
	}
	close(stop)
	wg.Wait()
	select {
	case m := <-bad:
		t.Fatal(m)
	default:
	}
}

// Migration 0005 on a version-4 database: existing holds and audit rows keep
// their data and read back with the new columns empty.
func TestMigration0005UpgradesVersion4Database(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.db")
	s := openAtVersion(t, path, 4)
	for _, q := range []string{
		`INSERT INTO users (id, auth_source, email, name, status, created_at) VALUES (1, 'oidc', 'a@example.test', 'A', 'active', '2026-11-01T00:00:00.000Z')`,
		`INSERT INTO setups (id, name, details_json, enabled) VALUES (1, 'lab-01', '{}', 1)`,
		`INSERT INTO holds (setup_id, user_id, acquired_at) VALUES (1, 1, '2026-11-01T00:00:00.000Z')`,
		`INSERT INTO audit_log (actor_id, action, target, created_at) VALUES (1, 'x', 'y', '2026-11-01T00:00:00.000Z')`,
	} {
		if _, err := s.sqldb.Exec(q); err != nil {
			t.Fatalf("%.60s: %v", q, err)
		}
	}
	s.Close()
	s = must(Open(path))
	defer s.Close()
	var v int
	var ic string
	s.sqldb.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&v)
	s.sqldb.QueryRow(`PRAGMA integrity_check`).Scan(&ic)
	if v != 6 || ic != "ok" {
		t.Fatalf("version %d integrity %q", v, ic)
	}
	h := must(s.GetHold(ctx, 1))
	if h.ReleaseRequestedAt != nil || h.ReleasedByService != "" || h.UserID != 1 {
		t.Errorf("hold: %+v", h)
	}
	if e := must(s.ListAudit(ctx, AuditFilter{})); len(e) != 1 || e[0].ActorService != "" || e[0].ActorID == nil {
		t.Errorf("audit: %+v", e)
	}
}
