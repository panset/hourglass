// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

// findUpgradeFile walks up from the test's directory (Bazel: its runfiles) to
// testdata/upgrade-d96a950/<name>.
func findUpgradeFile(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		p := filepath.Join(dir, "testdata", "upgrade-d96a950", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("testdata/upgrade-d96a950/%s not found above the working directory (Bazel: is it in the test's data?)", name)
		}
		dir = parent
	}
}

// upgradeNumbers are the figures the release must keep: every balance cache,
// every period sum of every user, and the charge of every hold.
type upgradeNumbers struct {
	Balances map[string]int64 `json:"balances_mt"`
	Sums     map[string]int64 `json:"period_sums_mt"`
	Charges  map[string]int64 `json:"hold_charges_mt"`
	HoldIDs  []int64          `json:"hold_ids"`
}

func readUpgradeNumbers(t *testing.T, st *store.Store, userIDs []int64, holdIDs []int64) upgradeNumbers {
	t.Helper()
	ctx := context.Background()
	n := upgradeNumbers{Balances: map[string]int64{}, Sums: map[string]int64{}, Charges: map[string]int64{}, HoldIDs: holdIDs}
	for _, id := range userIDs {
		u, err := st.GetUser(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		n.Balances[fmt.Sprint(id)] = u.BalanceMT
		for _, p := range []string{"2026-10", "2026-11"} {
			s, err := st.SumLedger(ctx, id, p, "")
			if err != nil {
				t.Fatal(err)
			}
			n.Sums[fmt.Sprintf("%d/%s", id, p)] = s
		}
	}
	ch, err := st.HoldCharges(ctx, holdIDs)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range ch {
		n.Charges[fmt.Sprint(k)] = v
	}
	return n
}

// UPG-1: a database written by the release at d96a950 (testdata/upgrade-d96a950,
// with ledger data) is upgraded by starting the new binary's Run with a
// configuration of the release's keys only. The schema is migrated, the
// balances and per-hold charges recorded from the release are the same after
// the start and after compaction, and the stored rows are fewer after it.
func TestUPG1_ReleaseDatabaseUpgradesAtStartAndKeepsEveryNumber(t *testing.T) {
	raw, err := os.ReadFile(findUpgradeFile(t, "hourglass.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Facts struct {
			upgradeNumbers
			UserIDs map[string]int64 `json:"user_ids"`
		} `json:"facts"`
	}
	b, err := os.ReadFile(findUpgradeFile(t, "responses.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	want := g.Facts.upgradeNumbers
	var users []int64
	for _, id := range g.Facts.UserIDs {
		users = append(users, id)
	}
	if len(users) != 4 || len(want.Balances) != 4 || len(want.Charges) != 2 || want.Balances[fmt.Sprint(g.Facts.UserIDs["dev"])] >= 0 {
		t.Fatalf("the recorded numbers look wrong: %+v", g.Facts)
	}

	// The release's configuration: the keys it knew, no new one.
	cfg, db := writeConfig(t, "timezone: Europe/Berlin\n")
	if err := os.WriteFile(db, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	// The database is the release's: schema version 5, 59 rows, the recorded
	// balance caches (read raw: the new store refuses an unmigrated database).
	raw5, err := sql.Open("sqlite", "file:"+db+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	var version, rows int
	if err := raw5.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := raw5.QueryRow(`SELECT COUNT(*) FROM ledger`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if version != 5 || rows != 59 {
		t.Fatalf("the fixture is at schema %d with %d ledger rows, want 5 and 59", version, rows)
	}
	for id, mt := range want.Balances {
		var got int64
		if err := raw5.QueryRow(`SELECT balance_mt FROM users WHERE id = ?`, id).Scan(&got); err != nil || got != mt {
			t.Fatalf("user %s: balance %d (%v) in the fixture, recorded %d", id, got, err, mt)
		}
	}
	_ = raw5.Close()

	// The clock is two days after the first debits: no day is older than a week, so
	// the start migrates and nothing is compacted yet.
	r := startWithClock(t, cfg, db, hourglasstest.NewFakeClock(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)))
	if strings.Contains(r.out.String(), "some were wrong") {
		t.Errorf("start-up had to repair a balance cache:\n%s", r.out.String())
	}
	if err := r.stop(); err != nil {
		t.Fatal(err)
	}

	mig, err := sql.Open("sqlite", "file:"+db+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	var migrated int
	var integrity string
	if err := mig.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&migrated); err != nil {
		t.Fatal(err)
	}
	if err := mig.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		t.Fatal(err)
	}
	_ = mig.Close()
	if migrated != 8 || integrity != "ok" {
		t.Fatalf("after the start: schema version %d (want 8), integrity %q", migrated, integrity)
	}
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if got := readUpgradeNumbers(t, st, users, want.HoldIDs); !reflect.DeepEqual(got, want) {
		t.Fatalf("after the upgrade start:\n%+v\nrecorded from the release\n%+v", got, want)
	}
	rowsBefore := countLedger(t, st, users)
	if rowsBefore != 59 {
		t.Errorf("%d ledger rows after the upgrade, the release wrote 59", rowsBefore)
	}
	if rep, err := st.RebuildBalances(ctx, "2026-11"); err != nil || rep.Changed != 0 {
		t.Errorf("balance caches need repair after the upgrade: %+v, %v", rep, err)
	}

	// Compaction at a date well after the data: the days older than a week are rolled up.
	berlin, _ := time.LoadLocation("Europe/Berlin")
	led := ledger.NewService(st, nil, hourglasstest.NewFakeClock(time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)), berlin)
	replaced := 0
	for i := 0; ; i++ {
		if i > 100 {
			t.Fatal("compaction does not finish")
		}
		s, err := led.CompactDue(ctx)
		if err != nil {
			t.Fatal(err)
		}
		replaced += s.Replaced
		if s.Replaced == 0 && !s.More {
			break
		}
	}
	if replaced == 0 {
		t.Fatal("nothing was compacted")
	}
	if got := readUpgradeNumbers(t, st, users, want.HoldIDs); !reflect.DeepEqual(got, want) {
		t.Fatalf("after compaction:\n%+v\nrecorded from the release\n%+v", got, want)
	}
	if after := countLedger(t, st, users); after >= rowsBefore {
		t.Errorf("%d rows after compaction, %d before", after, rowsBefore)
	}
	if rep, err := st.RebuildBalances(ctx, "2026-11"); err != nil || rep.Changed != 0 {
		t.Errorf("balance caches need repair after compaction: %+v, %v", rep, err)
	}
}

func countLedger(t *testing.T, st *store.Store, users []int64) int {
	t.Helper()
	n := 0
	for _, u := range users {
		for _, p := range []string{"2026-10", "2026-11"} {
			rows, err := st.ListLedger(context.Background(), u, p)
			if err != nil {
				t.Fatal(err)
			}
			n += len(rows)
		}
	}
	return n
}
