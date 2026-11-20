// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

var (
	ctx = context.Background()
	t0  = time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "hourglass.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func newUser(t *testing.T, s *Store, sub string) User {
	t.Helper()
	return must(s.CreateOIDCUser(ctx, sub, sub+"@example.test", "User "+sub, t0))
}

func newSetup(t *testing.T, s *Store, name string) int64 {
	t.Helper()
	return must(s.CreateSetup(ctx, Setup{Name: name, Enabled: true, Labels: map[string]string{"k": "v"}}))
}

func TestMigrateFreshAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var mode string
	s.sqldb.QueryRow(`PRAGMA journal_mode`).Scan(&mode)
	if mode != "wal" {
		t.Errorf("journal_mode = %q", mode)
	}
	var fk int
	s.sqldb.QueryRow(`PRAGMA foreign_keys`).Scan(&fk)
	if fk != 1 {
		t.Error("foreign_keys off")
	}
	newUser(t, s, "a")
	s.Close()

	s, err = Open(path) // re-running migrations is a no-op
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var n int
	s.sqldb.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n)
	if n != 5 {
		t.Errorf("schema_migrations rows = %d", n)
	}
	if us, _ := s.ListUsers(ctx, ""); len(us) != 1 {
		t.Errorf("users after reopen = %d", len(us))
	}
}

func TestMigrateNewerDatabaseIsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.db")
	s, _ := Open(path)
	s.sqldb.Exec(`INSERT INTO schema_migrations VALUES (999, 'future', 'x')`)
	s.Close()
	if _, err := Open(path); err == nil {
		t.Fatal("expected error for newer schema")
	}
}

func TestSchemaColumnsMatchSpec(t *testing.T) {
	spec := map[string][]string{
		"users":            {"id", "auth_source", "oidc_sub", "username", "email", "name", "status", "labels_json", "password_hash", "must_change_password", "balance_mt", "created_at", "last_login_at"},
		"user_roles":       {"user_id", "role"},
		"setups":           {"id", "name", "details_json", "enabled"},
		"setup_labels":     {"setup_id", "key", "value"},
		"rules":            {"id", "kind", "position", "selector", "value", "updated_by", "updated_at"},
		"holds":            {"id", "setup_id", "user_id", "acquired_at", "metering_started_at", "deployed_at", "released_at", "released_by", "release_reason", "release_requested_at", "released_by_service"},
		"usage_samples":    {"id", "hold_id", "polled_at", "weight", "rule_selector", "pressure", "multiplier", "debit_mt", "reason"},
		"ledger":           {"id", "user_id", "period", "type", "amount_mt", "hold_id", "actor_id", "reason", "created_at"},
		"pressure_samples": {"polled_at", "source", "raw", "smoothed", "multiplier"},
		"orphans":          {"resource_name", "first_seen_at", "last_seen_at", "labels_json"},
		"event_deliveries": {"id", "event_type", "hook", "payload_json", "ok", "error", "started_at", "finished_at"},
		"settings":         {"key", "value_json", "updated_by", "updated_at"},
		"audit_log":        {"id", "actor_id", "actor_service", "action", "target", "before_json", "after_json", "created_at"},
		"sessions":         {"id", "user_id", "created_at", "last_seen_at", "expires_at"},
		"collector_runs":   {"id", "started_at", "finished_at", "ok", "error"},
		"outages":          {"id", "started_at", "ended_at", "reason"},

		"service_accounts":          {"id", "name", "description", "scopes_json", "enabled", "created_by", "created_at"},
		"service_tokens":            {"id", "account_id", "token_hash", "created_at", "expires_at", "last_used_at"},
		"reported_pressure":         {"value", "observed_at", "received_at", "service_account"},
		"reported_deployments_meta": {"observed_at", "received_at", "service_account"},
		"reported_deployments":      {"setup_name", "labels_json"},
	}
	s := openTemp(t)
	for table, cols := range spec {
		rows, err := s.sqldb.Query(`SELECT name FROM pragma_table_info(?)`, table)
		if err != nil {
			t.Fatal(err)
		}
		have := map[string]bool{}
		for rows.Next() {
			var n string
			rows.Scan(&n)
			have[n] = true
		}
		rows.Close()
		for _, c := range cols {
			if !have[c] {
				t.Errorf("%s: missing column %s", table, c)
			}
		}
		if len(have) != len(cols) {
			t.Errorf("%s: has %d columns, spec has %d", table, len(have), len(cols))
		}
	}
	var n int
	s.sqldb.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN ('schema_migrations')`).Scan(&n)
	if n != len(spec) {
		t.Errorf("table count = %d, want %d", n, len(spec))
	}
}

func TestUsersRoundTrip(t *testing.T) {
	s := openTemp(t)
	u, created, err := s.UpsertOIDCLogin(ctx, "sub-1", "a@example.test", "A", t0)
	if err != nil || !created || u.Status != StatusPending || u.AuthSource != AuthOIDC {
		t.Fatalf("upsert new: %+v %v %v", u, created, err)
	}
	if roles, _ := s.GetUserRoles(ctx, u.ID); len(roles) != 0 {
		t.Errorf("new user has roles %v", roles)
	}
	u2, created, err := s.UpsertOIDCLogin(ctx, "sub-1", "b@example.test", "B", t0.Add(time.Hour))
	if err != nil || created || u2.ID != u.ID || u2.Email != "b@example.test" || u2.Name != "B" ||
		!u2.LastLoginAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("upsert existing: %+v %v %v", u2, created, err)
	}
	admin, err := s.CreateLocalAdmin(ctx, "admin", "hash1", true, t0)
	if err != nil || admin.AuthSource != AuthLocal || !admin.MustChangePassword || admin.OIDCSub != "" {
		t.Fatalf("local admin: %+v %v", admin, err)
	}
	if roles, _ := s.GetUserRoles(ctx, admin.ID); !reflect.DeepEqual(roles, []string{"admin"}) {
		t.Errorf("admin roles %v", roles)
	}
	if _, err := s.CreateLocalAdmin(ctx, "admin2", "h", false, t0); err != nil {
		t.Error("second local row with null sub must be allowed:", err)
	}
	if g, _ := s.GetUserByUsername(ctx, "admin"); g.ID != admin.ID {
		t.Error("get by username")
	}
	if g, _ := s.GetUserBySub(ctx, "sub-1"); g.ID != u.ID {
		t.Error("get by sub")
	}
	if _, err := s.GetUser(ctx, 9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing user: %v", err)
	}
	if err := s.SetUserStatus(ctx, u.ID, StatusActive); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPassword(ctx, admin.ID, "hash2", false); err != nil {
		t.Fatal(err)
	}
	if g, _ := s.GetUser(ctx, admin.ID); g.PasswordHash != "hash2" || g.MustChangePassword {
		t.Errorf("password: %+v", g)
	}
	if l, _ := s.ListUsers(ctx, StatusPending); len(l) != 0 {
		t.Errorf("pending list %v", l)
	}
	if l, _ := s.ListUsers(ctx, StatusActive); len(l) != 3 {
		t.Errorf("active list %d", len(l))
	}
	s.SetUserRoles(ctx, u.ID, []string{"developer", "admin"})
	s.SetUserRoles(ctx, u.ID, []string{"developer"})
	if roles, _ := s.GetUserRoles(ctx, u.ID); !reflect.DeepEqual(roles, []string{"developer"}) {
		t.Errorf("roles replaced: %v", roles)
	}
	if err := s.SetUserRoles(ctx, u.ID, []string{"bogus"}); err == nil {
		t.Error("bogus role accepted")
	}
	if roles, _ := s.GetUserRoles(ctx, u.ID); len(roles) != 1 {
		t.Errorf("failed replace must roll back, got %v", roles)
	}
}

func TestUserOIDCSubUnique(t *testing.T) {
	s := openTemp(t)
	newUser(t, s, "dup")
	if _, err := s.CreateOIDCUser(ctx, "dup", "", "", t0); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate sub: %v", err)
	}
}

func TestSetupsRoundTripAndConstraints(t *testing.T) {
	s := openTemp(t)
	id, err := s.CreateSetup(ctx, Setup{Name: "alpha", DetailsJSON: `{"x":1}`,
		Enabled: true, Labels: map[string]string{"size": "l", "site": "s1"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSetup(ctx, Setup{Name: "alpha"}); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate name: %v", err)
	}
	got, _ := s.GetSetup(ctx, id)
	want := Setup{ID: id, Name: "alpha", DetailsJSON: `{"x":1}`, Enabled: true,
		Labels: map[string]string{"size": "l", "site": "s1"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v", got)
	}
	want.Labels = map[string]string{"site": "s2"}
	want.Enabled = false
	if err := s.UpdateSetup(ctx, want); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.GetSetup(ctx, id); !reflect.DeepEqual(got, want) {
		t.Errorf("after update %+v", got)
	}
	newSetup(t, s, "beta")
	if l, _ := s.ListSetups(ctx, false); len(l) != 2 || l[0].Name != "alpha" || l[1].Labels["k"] != "v" {
		t.Errorf("list all %+v", l)
	}
	if l, _ := s.ListSetups(ctx, true); len(l) != 1 || l[0].Name != "beta" {
		t.Errorf("list enabled %+v", l)
	}
	if err := s.UpdateSetup(ctx, Setup{ID: 999, Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing: %v", err)
	}
	if err := s.DeleteSetup(ctx, id); err != nil {
		t.Fatal(err)
	}
	var n int
	s.sqldb.QueryRow(`SELECT COUNT(*) FROM setup_labels WHERE setup_id = ?`, id).Scan(&n)
	if n != 0 {
		t.Error("labels not deleted with setup")
	}
	if err := s.DeleteSetup(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete missing: %v", err)
	}
}

func TestSetupLabelsKeyValueIndexAndKey(t *testing.T) {
	s := openTemp(t)
	var n int
	s.sqldb.QueryRow(`SELECT COUNT(*) FROM pragma_index_list('setup_labels') WHERE name = 'setup_labels_key_value'`).Scan(&n)
	if n != 1 {
		t.Error("missing key+value index")
	}
	id := newSetup(t, s, "a")
	if _, err := s.sqldb.Exec(`INSERT INTO setup_labels VALUES (?, 'k', 'other')`, id); err == nil {
		t.Error("composite key not enforced")
	}
}

func TestRulesReplace(t *testing.T) {
	s := openTemp(t)
	admin := newUser(t, s, "adm")
	rs := []Rule{{Selector: "a=1", Value: 2}, {Selector: "b=2", Value: 0}, {Selector: "c=3", Value: 0.5}}
	if err := s.ReplaceRules(ctx, "rate", rs, &admin.ID, t0); err != nil {
		t.Fatal(err)
	}
	got, _ := s.ListRules(ctx, "rate")
	if len(got) != 3 || got[0].Selector != "a=1" || got[2].Position != 2 || got[2].Value != 0.5 ||
		*got[0].UpdatedBy != admin.ID || !got[0].UpdatedAt.Equal(t0) {
		t.Fatalf("rules %+v", got)
	}
	s.ReplaceRules(ctx, "other", []Rule{{Selector: "z", Value: 1}}, nil, t0)
	s.ReplaceRules(ctx, "rate", []Rule{{Selector: "n=1", Value: 4}}, nil, t0)
	if got, _ = s.ListRules(ctx, "rate"); len(got) != 1 || got[0].Selector != "n=1" || got[0].UpdatedBy != nil {
		t.Errorf("after replace %+v", got)
	}
	if o, _ := s.ListRules(ctx, "other"); len(o) != 1 {
		t.Error("other kind disturbed")
	}
	// A failing replace rolls back (bad foreign key on updated_by).
	bad := int64(424242)
	if err := s.ReplaceRules(ctx, "rate", []Rule{{Selector: "x", Value: 1}}, &bad, t0); err == nil {
		t.Fatal("expected error")
	}
	if got, _ = s.ListRules(ctx, "rate"); len(got) != 1 || got[0].Selector != "n=1" {
		t.Errorf("rollback failed: %+v", got)
	}
}

func TestHOLD_CreateReleaseAndOnce(t *testing.T) {
	s := openTemp(t)
	u1, u2 := newUser(t, s, "u1"), newUser(t, s, "u2")
	sid := newSetup(t, s, "s")
	h, err := s.CreateHold(ctx, sid, u1.ID, t0)
	if err != nil || h.ReleasedAt != nil || h.MeteringStartedAt != nil {
		t.Fatalf("%+v %v", h, err)
	}
	if _, err := s.CreateHold(ctx, sid, u2.ID, t0); !errors.Is(err, ErrSetupHeld) {
		t.Fatalf("second hold: %v", err)
	}
	if n, _ := s.CountActiveHolds(ctx, u1.ID); n != 1 {
		t.Errorf("count %d", n)
	}
	if ok, err := s.SetMeteringStarted(ctx, h.ID, t0.Add(time.Minute)); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if ok, _ := s.SetMeteringStarted(ctx, h.ID, t0.Add(time.Hour)); ok {
		t.Error("metering_started_at overwritten")
	}
	s.SetDeployed(ctx, h.ID, t0.Add(2*time.Minute))
	if ok, _ := s.SetDeployed(ctx, h.ID, t0.Add(3*time.Minute)); ok {
		t.Error("deployed_at overwritten")
	}
	if _, err := s.SetDeployed(ctx, 999, t0); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing hold: %v", err)
	}
	got, _ := s.GetHold(ctx, h.ID)
	if !got.MeteringStartedAt.Equal(t0.Add(time.Minute)) || !got.DeployedAt.Equal(t0.Add(2*time.Minute)) {
		t.Errorf("once semantics: %+v", got)
	}
	if a, _ := s.ActiveHolds(ctx); len(a) != 1 {
		t.Errorf("active %d", len(a))
	}
	if err := s.ReleaseHold(ctx, h.ID, &u1.ID, "done", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.ReleaseHold(ctx, h.ID, &u1.ID, "again", t0); !errors.Is(err, ErrConflict) {
		t.Errorf("double release: %v", err)
	}
	if err := s.ReleaseHold(ctx, 999, nil, "", t0); !errors.Is(err, ErrNotFound) {
		t.Errorf("release missing: %v", err)
	}
	got, _ = s.GetHold(ctx, h.ID)
	if got.ReleasedAt == nil || *got.ReleasedBy != u1.ID || got.ReleaseReason != "done" {
		t.Errorf("released: %+v", got)
	}
	// After release the setup can be held again; history is kept.
	h2, err := s.CreateHold(ctx, sid, u2.ID, t0.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := s.CountActiveHolds(ctx, u1.ID); n != 0 {
		t.Errorf("count after release %d", n)
	}
	if hs, _ := s.UserHolds(ctx, u1.ID); len(hs) != 1 {
		t.Errorf("history %d", len(hs))
	}
	if a, err := s.ActiveHoldForSetup(ctx, sid); err != nil || a.ID != h2.ID {
		t.Errorf("active for setup %+v %v", a, err)
	}
}

func TestHOLD_UniquenessUnderConcurrentCreates(t *testing.T) {
	s := openTemp(t)
	sid := newSetup(t, s, "s")
	const n = 8
	users := make([]User, n)
	for i := range users {
		users[i] = newUser(t, s, string(rune('a'+i)))
	}
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				_, errs[i] = s.CreateHold(ctx, sid, users[i].ID, t0)
			} else {
				errs[i] = s.Tx(ctx, func(tx *Tx) error {
					_, err := tx.CreateHold(ctx, sid, users[i].ID, t0)
					return err
				})
			}
		}()
	}
	wg.Wait()
	ok, held := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrSetupHeld):
			held++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 || held != n-1 {
		t.Errorf("ok=%d held=%d", ok, held)
	}
}

func TestMET5_UsageSampleIdempotent(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	h := must(s.CreateHold(ctx, newSetup(t, s, "s"), u.ID, t0))
	smp := UsageSample{HoldID: h.ID, PolledAt: t0, Weight: 2, RuleSelector: "k=v", Pressure: 0.5, Multiplier: 1, DebitMT: 167, Reason: "r"}
	if ins, err := s.InsertUsageSample(ctx, smp); !ins || err != nil {
		t.Fatal(ins, err)
	}
	smp.DebitMT = 999
	if ins, err := s.InsertUsageSample(ctx, smp); ins || err != nil {
		t.Fatalf("second insert reported %v %v", ins, err)
	}
	got, _ := s.ListUsageSamples(ctx, h.ID)
	if len(got) != 1 || got[0].DebitMT != 167 || got[0].RuleSelector != "k=v" || !got[0].PolledAt.Equal(t0) {
		t.Errorf("samples %+v", got)
	}
	smp.PolledAt = t0.Add(5 * time.Minute)
	smp.RuleSelector = ""
	if ins, _ := s.InsertUsageSample(ctx, smp); !ins {
		t.Error("different polled_at must insert")
	}
	got, _ = s.ListUsageSamples(ctx, h.ID)
	if got[1].RuleSelector != "" {
		t.Error("null selector not preserved")
	}
}

func TestUsageSumsAndPrune(t *testing.T) {
	s := openTemp(t)
	u1, u2 := newUser(t, s, "u1"), newUser(t, s, "u2")
	h1 := must(s.CreateHold(ctx, newSetup(t, s, "s1"), u1.ID, t0))
	h2 := must(s.CreateHold(ctx, newSetup(t, s, "s2"), u2.ID, t0))
	add := func(h Hold, at time.Time, d int64) {
		s.InsertUsageSample(ctx, UsageSample{HoldID: h.ID, PolledAt: at, Weight: 1, Pressure: 0, Multiplier: 1, DebitMT: d})
	}
	day1 := time.Date(2026, 11, 1, 23, 55, 0, 0, time.UTC)
	day2 := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	add(h1, day1, 10)
	add(h1, day2, 20)
	add(h1, day2.Add(5*time.Minute), 30)
	add(h2, day2, 1000)
	days, err := s.DailyDebits(ctx, u1.ID, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || !reflect.DeepEqual(days, []DayTotal{{"2026-11-01", 10}, {"2026-11-02", 50}}) {
		t.Errorf("days %+v %v", days, err)
	}
	if n, _ := s.SumDebitsSince(ctx, u1.ID, day2); n != 50 {
		t.Errorf("since = %d", n)
	}
	if n, _ := s.SumDebitsSince(ctx, u1.ID, day2.Add(24*time.Hour)); n != 0 {
		t.Errorf("empty since = %d", n)
	}
	if n, _ := s.PruneUsageSamples(ctx, day2); n != 1 {
		t.Errorf("pruned %d", n)
	}
	if l, _ := s.ListUsageSamples(ctx, h1.ID); len(l) != 2 {
		t.Errorf("remaining %d", len(l))
	}
}

func TestLED_TriggersRejectUpdateDelete(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: "allowance", AmountMT: 300000}, "2026-11", t0)
	if _, err := s.sqldb.Exec(`UPDATE ledger SET amount_mt = 1`); err == nil {
		t.Error("UPDATE allowed")
	}
	if _, err := s.sqldb.Exec(`DELETE FROM ledger`); err == nil {
		t.Error("DELETE allowed")
	}
	if n, _ := s.SumLedger(ctx, u.ID, "2026-11", ""); n != 300000 {
		t.Errorf("sum %d", n)
	}
}

func TestLED1_RejectsInsertOrReplace(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	e, err := s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: "allowance", AmountMT: 5000}, "2026-11", t0)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT OR REPLACE INTO ledger (id, user_id, period, type, amount_mt, created_at) VALUES (?, ?, '2026-11', 'allowance', 1, 'x')`,
		`REPLACE INTO ledger (id, user_id, period, type, amount_mt, created_at) VALUES (?, ?, '2026-11', 'allowance', 1, 'x')`,
		`INSERT OR IGNORE INTO ledger (id, user_id, period, type, amount_mt, created_at) VALUES (?, ?, '2026-11', 'allowance', 1, 'x')`,
	} {
		if _, err := s.sqldb.Exec(stmt, e.ID, u.ID); err == nil {
			t.Errorf("allowed: %s", stmt)
		}
	}
	if n, _ := s.SumLedger(ctx, u.ID, "2026-11", ""); n != 5000 {
		t.Errorf("sum %d", n)
	}
	// A fresh id still inserts.
	if _, err := s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: "grant", AmountMT: 1}, "2026-11", t0); err != nil {
		t.Fatal(err)
	}
}

func TestStoreTxPanicRollsBackAndReleases(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	func() {
		defer func() {
			if r := recover(); r != "boom" {
				t.Errorf("recovered %v", r)
			}
		}()
		s.Tx(ctx, func(tx *Tx) error {
			tx.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: "grant", AmountMT: 5}, "2026-11", t0)
			panic("boom")
		})
	}()
	done := make(chan error, 1)
	go func() {
		_, err := s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: "grant", AmountMT: 7}, "2026-11", t0)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("write blocked after panic in Tx")
	}
	if n, _ := s.SumLedger(ctx, u.ID, "2026-11", ""); n != 7 {
		t.Errorf("sum %d (panicked tx not rolled back)", n)
	}
}

func TestLED2_CacheIsCurrentPeriodForAnyEntryPeriod(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	add := func(period, typ string, amt int64) {
		t.Helper()
		if _, err := s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: period, Type: typ, AmountMT: amt}, "2026-12", t0); err != nil {
			t.Fatal(err)
		}
	}
	cache := func() int64 { g, _ := s.GetUser(ctx, u.ID); return g.BalanceMT }
	add("2026-11", "allowance", 300000)
	add("2026-12", "allowance", 300000)
	add("2026-12", "debit", -1000)
	// Expiry of the old period written after the new allowance.
	add("2026-11", "expiry", -300000)
	if cache() != 299000 {
		t.Errorf("after late expiry %d", cache())
	}
	// Late grant to a past period.
	add("2026-11", "grant", 7000)
	if cache() != 299000 {
		t.Errorf("after late past grant %d", cache())
	}
	if n, _ := s.RebuildBalance(ctx, u.ID, "2026-12"); n != 299000 || cache() != 299000 {
		t.Errorf("rebuild %d cache %d", n, cache())
	}
}

func TestLED2_BalanceCacheMatchesLedger(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	admin := newUser(t, s, "adm")
	h := must(s.CreateHold(ctx, newSetup(t, s, "s"), u.ID, t0))
	add := func(typ string, amt int64, actor *int64, hold *int64) {
		if _, err := s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: typ, AmountMT: amt, ActorID: actor, HoldID: hold, Reason: "x"}, "2026-11", t0); err != nil {
			t.Fatal(err)
		}
	}
	add("allowance", 300000, nil, nil)
	add("debit", -1500, nil, &h.ID)
	add("grant", 5000, &admin.ID, nil)
	add("debit", -300000, nil, &h.ID)
	add("grant", -250, &admin.ID, nil)
	want := int64(300000 - 1500 + 5000 - 300000 - 250)
	if g, _ := s.GetUser(ctx, u.ID); g.BalanceMT != want {
		t.Errorf("cache %d want %d", g.BalanceMT, want)
	}
	if n, _ := s.SumLedger(ctx, u.ID, "2026-11", ""); n != want {
		t.Errorf("sum %d", n)
	}
	if n, _ := s.SumLedger(ctx, u.ID, "2026-11", "debit"); n != -301500 {
		t.Errorf("debit sum %d", n)
	}
	// Corrupt the cache, then rebuild.
	s.sqldb.Exec(`UPDATE users SET balance_mt = 7 WHERE id = ?`, u.ID)
	if n, err := s.RebuildBalance(ctx, u.ID, "2026-11"); err != nil || n != want {
		t.Fatalf("rebuild %d %v", n, err)
	}
	if g, _ := s.GetUser(ctx, u.ID); g.BalanceMT != want {
		t.Errorf("after rebuild %d", g.BalanceMT)
	}
	if l, _ := s.ListLedger(ctx, u.ID, "2026-11"); len(l) != 5 || l[1].HoldID == nil || *l[2].ActorID != admin.ID {
		t.Errorf("list %+v", l)
	}
	// Rollover: expiry in the old period zeroes the cache; the new period starts fresh.
	add("expiry", -want, nil, nil)
	if g, _ := s.GetUser(ctx, u.ID); g.BalanceMT != 0 {
		t.Errorf("after expiry %d", g.BalanceMT)
	}
	s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-12", Type: "allowance", AmountMT: 300000}, "2026-12", t0)
	if g, _ := s.GetUser(ctx, u.ID); g.BalanceMT != 300000 {
		t.Errorf("new period %d", g.BalanceMT)
	}
	if _, err := s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "bad", Type: "grant"}, "2026-12", t0); err == nil {
		t.Error("bad period accepted")
	}
	if _, err := s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-12", Type: "nope"}, "2026-12", t0); err == nil {
		t.Error("bad type accepted")
	}
	if g, _ := s.GetUser(ctx, u.ID); g.BalanceMT != 300000 {
		t.Errorf("failed append changed cache: %d", g.BalanceMT)
	}
}

func TestLedgerAppendInsideTxRollsBack(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	boom := errors.New("boom")
	err := s.Tx(ctx, func(tx *Tx) error {
		tx.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: "grant", AmountMT: 5}, "2026-11", t0)
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if g, _ := s.GetUser(ctx, u.ID); g.BalanceMT != 0 {
		t.Errorf("balance %d", g.BalanceMT)
	}
	if n, _ := s.SumLedger(ctx, u.ID, "2026-11", ""); n != 0 {
		t.Errorf("ledger %d", n)
	}
}

func TestPressureOrphansOutagesRuns(t *testing.T) {
	s := openTemp(t)
	for i := 0; i < 5; i++ {
		err := s.InsertPressureSample(ctx, PressureSample{PolledAt: t0.Add(time.Duration(i) * time.Minute), Source: "src", Raw: 0.1 * float64(i), Smoothed: 0.2, Multiplier: 1.5})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := s.InsertPressureSample(ctx, PressureSample{PolledAt: t0, Source: "src"}); !errors.Is(err, ErrConflict) {
		t.Errorf("dup pressure: %v", err)
	}
	if l, _ := s.PressureSince(ctx, t0.Add(3*time.Minute)); len(l) != 2 || !l[0].PolledAt.Before(l[1].PolledAt) {
		t.Errorf("since %+v", l)
	}
	if l, _ := s.LatestPressure(ctx, 2); len(l) != 2 || !l[0].PolledAt.Equal(t0.Add(4*time.Minute)) || l[0].Multiplier != 1.5 {
		t.Errorf("latest %+v", l)
	}

	s.UpsertOrphan(ctx, "r1", `{"a":"1"}`, t0)
	s.UpsertOrphan(ctx, "r2", "", t0)
	poll2 := t0.Add(5 * time.Minute)
	s.UpsertOrphan(ctx, "r1", `{"a":"2"}`, poll2)
	if n, _ := s.DeleteOrphansNotSeenSince(ctx, poll2); n != 1 {
		t.Errorf("deleted %d", n)
	}
	os, _ := s.ListOrphans(ctx)
	if len(os) != 1 || os[0].ResourceName != "r1" || !os[0].FirstSeenAt.Equal(t0) || !os[0].LastSeenAt.Equal(poll2) || os[0].LabelsJSON != `{"a":"2"}` {
		t.Errorf("orphans %+v", os)
	}

	s.InsertOutage(ctx, Outage{StartedAt: t0, EndedAt: t0.Add(time.Hour), Reason: "down"})
	s.InsertOutage(ctx, Outage{StartedAt: t0.Add(2 * time.Hour), EndedAt: t0.Add(3 * time.Hour)})
	if l, _ := s.ListOutages(ctx, 10); len(l) != 2 || l[1].Reason != "down" || !l[1].EndedAt.Equal(t0.Add(time.Hour)) {
		t.Errorf("outages %+v", l)
	}

	if _, err := s.LatestRun(ctx); !errors.Is(err, ErrNotFound) {
		t.Errorf("no runs: %v", err)
	}
	r1 := must(s.StartCollectorRun(ctx, t0))
	if r, _ := s.LatestRun(ctx); r.ID != r1 || r.OK != nil || r.FinishedAt != nil {
		t.Errorf("unfinished %+v", r)
	}
	s.FinishCollectorRun(ctx, r1, t0.Add(time.Second), true, "")
	r2 := must(s.StartCollectorRun(ctx, t0.Add(5*time.Minute)))
	s.FinishCollectorRun(ctx, r2, t0.Add(6*time.Minute), false, "provider down")
	if r, _ := s.LastSuccessfulRun(ctx); r.ID != r1 || !*r.OK {
		t.Errorf("last ok %+v", r)
	}
	if r, _ := s.LatestRun(ctx); r.ID != r2 || *r.OK || r.Error != "provider down" {
		t.Errorf("latest %+v", r)
	}
	if err := s.FinishCollectorRun(ctx, 999, t0, true, ""); !errors.Is(err, ErrNotFound) {
		t.Error(err)
	}
}

func TestEventDeliveriesSettingsAudit(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	d1 := must(s.InsertEventDelivery(ctx, "hold.acquired", "hook-a", `{"a":1}`, t0))
	d2 := must(s.InsertEventDelivery(ctx, "hold.released", "hook-a", "", t0.Add(time.Hour)))
	s.FinishEventDelivery(ctx, d1, t0.Add(time.Second), false, "timeout")
	l, _ := s.ListEventDeliveries(ctx, 10)
	if len(l) != 2 || l[0].ID != d2 || l[0].OK != nil || *l[1].OK || l[1].Error != "timeout" || l[1].PayloadJSON != `{"a":1}` {
		t.Errorf("deliveries %+v", l)
	}
	if n, _ := s.PruneEventDeliveries(ctx, t0.Add(time.Minute)); n != 1 {
		t.Errorf("pruned %d", n)
	}

	if _, err := s.GetSetting(ctx, "k"); !errors.Is(err, ErrNotFound) {
		t.Error(err)
	}
	s.PutSetting(ctx, "k", `1`, &u.ID, t0)
	s.PutSetting(ctx, "k", `2`, nil, t0.Add(time.Minute))
	s.PutSetting(ctx, "a", `"x"`, &u.ID, t0)
	g, _ := s.GetSetting(ctx, "k")
	if g.ValueJSON != `2` || g.UpdatedBy != nil || !g.UpdatedAt.Equal(t0.Add(time.Minute)) {
		t.Errorf("setting %+v", g)
	}
	if all, _ := s.AllSettings(ctx); len(all) != 2 || all[0].Key != "a" || *all[0].UpdatedBy != u.ID {
		t.Errorf("all %+v", all)
	}

	for i, act := range []string{"grant", "reset", "grant", "setup.delete"} {
		actor := &u.ID
		if i == 3 {
			actor = nil
		}
		s.AppendAudit(ctx, AuditEntry{ActorID: actor, Action: act, Target: "t", BeforeJSON: `{}`}, t0.Add(time.Duration(i)*time.Minute))
	}
	all, _ := s.ListAudit(ctx, AuditFilter{})
	if len(all) != 4 || all[0].Action != "setup.delete" || all[0].ActorID != nil || all[3].BeforeJSON != `{}` || all[3].AfterJSON != "" {
		t.Errorf("audit %+v", all)
	}
	if l, _ := s.ListAudit(ctx, AuditFilter{ActorID: &u.ID, Action: "grant"}); len(l) != 2 || l[0].ID < l[1].ID {
		t.Errorf("filtered %+v", l)
	}
	if l, _ := s.ListAudit(ctx, AuditFilter{Limit: 1, Offset: 1}); len(l) != 1 || l[0].Action != "grant" {
		t.Errorf("paged %+v", l)
	}
}

func TestSessions(t *testing.T) {
	s := openTemp(t)
	u1, u2 := newUser(t, s, "u1"), newUser(t, s, "u2")
	s.CreateSession(ctx, "s1", u1.ID, t0, t0.Add(time.Hour))
	s.CreateSession(ctx, "s2", u1.ID, t0, t0.Add(48*time.Hour))
	s.CreateSession(ctx, "s3", u2.ID, t0, t0.Add(48*time.Hour))
	if err := s.CreateSession(ctx, "s1", u1.ID, t0, t0); !errors.Is(err, ErrConflict) {
		t.Errorf("dup session: %v", err)
	}
	g, err := s.GetSession(ctx, "s1")
	if err != nil || g.UserID != u1.ID || !g.ExpiresAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("%+v %v", g, err)
	}
	s.TouchSession(ctx, "s1", t0.Add(10*time.Minute), t0.Add(2*time.Hour))
	if g, _ = s.GetSession(ctx, "s1"); !g.LastSeenAt.Equal(t0.Add(10*time.Minute)) || !g.ExpiresAt.Equal(t0.Add(2*time.Hour)) {
		t.Errorf("touch %+v", g)
	}
	if err := s.TouchSession(ctx, "nope", t0, t0); !errors.Is(err, ErrNotFound) {
		t.Error(err)
	}
	if n, _ := s.DeleteExpiredSessions(ctx, t0.Add(3*time.Hour)); n != 1 {
		t.Errorf("expired %d", n)
	}
	if n, _ := s.DeleteUserSessions(ctx, u1.ID); n != 1 {
		t.Errorf("user sessions %d", n)
	}
	s.DeleteSession(ctx, "s3")
	if _, err := s.GetSession(ctx, "s3"); !errors.Is(err, ErrNotFound) {
		t.Error(err)
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	s := openTemp(t)
	if _, err := s.CreateHold(ctx, 1, 1, t0); err == nil || errors.Is(err, ErrSetupHeld) {
		t.Errorf("hold with missing refs: %v", err)
	}
	u := newUser(t, s, "u")
	sid := newSetup(t, s, "s")
	s.CreateHold(ctx, sid, u.ID, t0)
	if err := s.DeleteSetup(ctx, sid); !errors.Is(err, ErrConflict) {
		t.Errorf("delete held setup: %v", err)
	}
}

func TestTimestampsAreFixedWidthUTC(t *testing.T) {
	s := openTemp(t)
	loc := time.FixedZone("x", 5*3600)
	u := must(s.CreateOIDCUser(ctx, "u", "", "", time.Date(2026, 1, 2, 3, 4, 5, 0, loc)))
	var raw string
	s.sqldb.QueryRow(`SELECT created_at FROM users WHERE id = ?`, u.ID).Scan(&raw)
	if raw != "2026-01-01T22:04:05.000Z" {
		t.Errorf("raw %q", raw)
	}
	var _ sql.NullString
}

// appendTimes appends n debits for one user in one transaction and returns the
// time each block of `block` appends took.
func appendTimes(t *testing.T, s *Store, userID int64, n, block int) []time.Duration {
	t.Helper()
	var out []time.Duration
	err := s.Tx(ctx, func(tx *Tx) error {
		for i := 0; i < n; i += block {
			start := time.Now()
			for j := 0; j < block; j++ {
				if _, err := tx.AppendLedger(ctx, LedgerEntry{UserID: userID, Period: "2026-11", Type: "debit", AmountMT: -3}, "2026-11", t0); err != nil {
					return err
				}
			}
			out = append(out, time.Since(start))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestNFR_LedgerAppendTimeIsLinear: appending must not re-sum the period's
// ledger (the cache is maintained incrementally), so the cost of an append does
// not grow with the entries already there. With a re-sum the last blocks of
// 10,000 appends cost several times the first; here they cost about the same.
func TestNFR_LedgerAppendTimeIsLinear(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	const n, block = 10000, 500
	blocks := appendTimes(t, s, u.ID, n, block)
	sum := func(d []time.Duration) (t time.Duration) {
		for _, x := range d {
			t += x
		}
		return
	}
	first, last := sum(blocks[:len(blocks)/4]), sum(blocks[len(blocks)*3/4:])
	t.Logf("first quarter %s, last quarter %s for %d appends", first, last, n)
	if last > 2*first+20*time.Millisecond {
		t.Errorf("append cost grows with the ledger: first quarter %s, last quarter %s", first, last)
	}
	if g, _ := s.GetUser(ctx, u.ID); g.BalanceMT != -3*n {
		t.Errorf("cache %d want %d", g.BalanceMT, -3*n)
	}
	if got, _ := s.SumLedger(ctx, u.ID, "2026-11", ""); got != -3*n {
		t.Errorf("sum %d", got)
	}
}

// LED-2: after any order of writes, including a rollover and a late entry for
// a past period, the cache equals the current period's ledger sum.
func TestLED2_CacheEqualsSumAfterAnyOrderOfWrites(t *testing.T) {
	s := openTemp(t)
	a, b := newUser(t, s, "a"), newUser(t, s, "b")
	// Only the user just written is checked: another user's cache is the
	// previous period's until their own rollover entry is appended.
	check := func(u User, cur string) {
		t.Helper()
		{
			want, _ := s.SumLedger(ctx, u.ID, cur, "")
			if g, _ := s.GetUser(ctx, u.ID); g.BalanceMT != want {
				t.Fatalf("user %d cache %d, ledger sum %d", u.ID, g.BalanceMT, want)
			}
		}
	}
	add := func(u User, period, typ string, amt int64, cur string) {
		t.Helper()
		if _, err := s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: period, Type: typ, AmountMT: amt}, cur, t0); err != nil {
			t.Fatal(err)
		}
		check(u, cur)
	}
	add(a, "2026-10", "allowance", 300000, "2026-10")
	add(b, "2026-10", "allowance", 300000, "2026-10")
	add(a, "2026-10", "debit", -1000, "2026-10")
	add(a, "2026-09", "expiry", -5, "2026-10") // late entry for a past period
	// Rollover: expiry and carryover in the old period, then the new allowance.
	add(a, "2026-10", "expiry", -299000, "2026-11")
	add(a, "2026-11", "carryover", 100, "2026-11")
	add(a, "2026-11", "allowance", 300000, "2026-11")
	add(b, "2026-10", "grant", 7, "2026-11") // past-period entry before b's first current entry
	add(b, "2026-11", "allowance", 300000, "2026-11")
	add(a, "2026-11", "debit", -42, "2026-11")
	add(a, "2026-10", "grant", 9, "2026-11") // late past-period entry
	add(b, "2026-11", "debit", -1, "2026-11")
	for _, u := range []User{a, b} {
		if _, err := s.RebuildBalance(ctx, u.ID, "2026-11"); err != nil {
			t.Fatal(err)
		}
	}
	check(a, "2026-11")
	check(b, "2026-11")
}

// RULE-5: later rule kinds (per-selector hold limits, allowance rules) must not
// need a schema change: the rules table keeps any kind, each ordered on its own.
func TestRULE5_RulesTableTakesAnyKindWithoutSchemaChange(t *testing.T) {
	s := openTemp(t)
	for _, kind := range []string{"rate", "hold_limit", "allowance"} {
		if err := s.ReplaceRules(ctx, kind, []Rule{{Selector: "a=1", Value: 1}, {Selector: "", Value: 2}}, nil, t0); err != nil {
			t.Fatalf("kind %q: %v", kind, err)
		}
	}
	for _, kind := range []string{"rate", "hold_limit", "allowance"} {
		got, err := s.ListRules(ctx, kind)
		if err != nil || len(got) != 2 || got[0].Position != 0 || got[1].Position != 1 || got[1].Value != 2 {
			t.Errorf("kind %q: %+v %v", kind, got, err)
		}
	}
}

// Migration 0003 on a database made by schema version 2 (adopted from
// verification task 42): the version-2 schema is built by running the
// embedded migrations 0001-0002 only (openAtVersion) and populated, then Open
// upgrades it. Rows, balance
// caches and the append path survive, the old index is replaced, the file
// passes integrity_check and a reopen runs nothing.
func TestNFR_Migration0003UpgradesVersion2Database(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.db")
	s := openAtVersion(t, path, 2)
	a, b := newUser(t, s, "a"), newUser(t, s, "b")
	// Raw insert: at schema version 2 the holds table lacks the later columns
	// that CreateHold reads back.
	setupID := newSetup(t, s, "lab-01")
	hr := must(s.sqldb.Exec(`INSERT INTO holds (setup_id, user_id, acquired_at) VALUES (?, ?, ?)`, setupID, a.ID, FormatTime(t0)))
	holdID := must(hr.LastInsertId())
	h := Hold{ID: holdID}
	for _, u := range []int64{a.ID, b.ID} {
		must(s.AppendLedger(ctx, LedgerEntry{UserID: u, Period: "2026-10", Type: "allowance", AmountMT: 300000}, "2026-10", t0))
		must(s.AppendLedger(ctx, LedgerEntry{UserID: u, Period: "2026-11", Type: "allowance", AmountMT: 300000}, "2026-11", t0))
	}
	for i := 0; i < 10; i++ {
		must(s.AppendLedger(ctx, LedgerEntry{UserID: a.ID, Period: "2026-11", Type: "debit", AmountMT: -167, HoldID: &h.ID}, "2026-11", t0))
	}
	s.Close()

	s = must(Open(path))
	defer func() { s.Close() }()
	var v, rows int
	var ic string
	s.sqldb.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&v)
	s.sqldb.QueryRow(`SELECT COUNT(*) FROM ledger`).Scan(&rows)
	s.sqldb.QueryRow(`PRAGMA integrity_check`).Scan(&ic)
	if v != 5 || rows != 14 || ic != "ok" {
		t.Fatalf("after upgrade: version %d, ledger rows %d, integrity %q", v, rows, ic)
	}
	var idx []string
	r, err := s.sqldb.Query(`SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = 'ledger' AND sql IS NOT NULL ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	for r.Next() {
		var n string
		r.Scan(&n)
		idx = append(idx, n)
	}
	r.Close()
	if want := []string{"ledger_hold_type", "ledger_period_type", "ledger_user_period_type"}; !reflect.DeepEqual(idx, want) {
		t.Errorf("ledger indexes %v, want %v", idx, want)
	}
	for _, u := range []User{a, b} {
		got := must(s.GetUser(ctx, u.ID)).BalanceMT
		if sum := must(s.SumLedger(ctx, u.ID, "2026-11", "")); got != sum {
			t.Errorf("user %d: cache %d, ledger %d", u.ID, got, sum)
		}
	}
	must(s.AppendLedger(ctx, LedgerEntry{UserID: a.ID, Period: "2026-11", Type: "grant", AmountMT: 1234}, "2026-11", t0))
	if got, sum := must(s.GetUser(ctx, a.ID)).BalanceMT, must(s.SumLedger(ctx, a.ID, "2026-11", "")); got != sum || got != 300000-1670+1234 {
		t.Errorf("append after upgrade: cache %d, ledger %d", got, sum)
	}
	s.Close()
	s = must(Open(path)) // reopen: nothing to run
	var n int
	s.sqldb.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n)
	if n != 5 {
		t.Errorf("schema_migrations rows = %d after reopen", n)
	}
}

// openAtVersion builds a database at schema version v by running only the
// embedded migrations up to v, as an older binary would have, and returns a
// store over it that has not run the later ones. Upgrade tests populate it
// and then call Open.
func openAtVersion(t *testing.T, path string, v int) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, m := range must(loadMigrations()) {
		if m.version > v {
			break
		}
		if _, err := db.Exec(m.sql); err != nil {
			t.Fatalf("migration %d: %v", m.version, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations VALUES (?, ?, ?)`, m.version, m.name, FormatTime(t0)); err != nil {
			t.Fatal(err)
		}
	}
	return &Store{queries: newQueries(db), sqldb: db}
}
