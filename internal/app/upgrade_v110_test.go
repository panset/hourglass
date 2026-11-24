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
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

// findV110File walks up from the test's directory (Bazel: its runfiles) to
// testdata/upgrade-v1.1.0/<name>. A missing file fails the test: it never
// skips.
func findV110File(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		p := filepath.Join(dir, "testdata", "upgrade-v1.1.0", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("testdata/upgrade-v1.1.0/%s not found above the working directory (Bazel: is it in the test's data?)", name)
		}
		dir = parent
	}
}

// tableDump is every row of every table of a database, as text, by table.
// Columns are named, so the dump of a migrated database can be cut down to
// the columns the old one had.
type tableDump struct {
	columns map[string][]string
	rows    map[string][]string // sorted, one string per row
}

func dumpTables(t *testing.T, path string, only map[string][]string) tableDump {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	names, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for names.Next() {
		var n string
		if err := names.Scan(&n); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, n)
	}
	names.Close()
	d := tableDump{columns: map[string][]string{}, rows: map[string][]string{}}
	for _, tb := range tables {
		cols := only[tb]
		if only != nil && cols == nil {
			continue // a table the old database did not have
		}
		if cols == nil {
			ci, err := db.Query(`SELECT name FROM pragma_table_info('` + tb + `') ORDER BY cid`)
			if err != nil {
				t.Fatal(err)
			}
			for ci.Next() {
				var c string
				_ = ci.Scan(&c)
				cols = append(cols, c)
			}
			ci.Close()
		}
		d.columns[tb] = cols
		q := `SELECT "` + strings.Join(cols, `", "`) + `" FROM "` + tb + `"`
		rs, err := db.Query(q)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		for rs.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rs.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			var parts []string
			for _, v := range vals {
				parts = append(parts, fmt.Sprintf("%T:%v", v, v))
			}
			d.rows[tb] = append(d.rows[tb], strings.Join(parts, "|"))
		}
		rs.Close()
		sort.Strings(d.rows[tb])
	}
	return d
}

// allNull reports, for each column of the table that the old database lacked,
// how many rows hold a value.
func nonNullInNewColumns(t *testing.T, path, table string, newCols []string) map[string]int {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	out := map[string]int{}
	for _, c := range newCols {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM "` + table + `" WHERE "` + c + `" IS NOT NULL`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		out[c] = n
	}
	return out
}

// v110Numbers is what testdata/upgrade-v1.1.0/responses.json records of the
// numbers the release must keep.
type v110Numbers struct {
	Balances map[string]int64 `json:"balances_mt"`
	Sums     map[string]int64 `json:"period_sums_mt"`
	Charges  map[string]int64 `json:"hold_charges_mt"`
	HoldIDs  map[string]int64 `json:"hold_ids"`
	UserIDs  map[string]int64 `json:"user_ids"`
	Rows     map[string]int   `json:"rows_per_table"`
}

// copyV110Database writes the release's database to a new config's database
// path and returns the config path, the database path, the recorded numbers
// and the rows of every table as the release wrote them.
func copyV110Database(t *testing.T) (cfg, db string, f v110Numbers, before tableDump) {
	t.Helper()
	raw, err := os.ReadFile(findV110File(t, "hourglass.sqlite3"))
	if err != nil {
		t.Fatalf("the v1.1.0 fixture is missing: %v", err)
	}
	b, err := os.ReadFile(findV110File(t, "responses.json"))
	if err != nil {
		t.Fatalf("the v1.1.0 fixture is missing: %v", err)
	}
	var g struct {
		Facts v110Numbers `json:"facts"`
	}
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	f = g.Facts
	if len(f.UserIDs) != 6 || len(f.Balances) != 6 || len(f.Charges) != 4 || len(f.HoldIDs) != 5 || f.Rows["ledger"] != 121 || f.Rows["usage_samples"] != 157 {
		t.Fatalf("the recorded numbers look wrong: %+v", f)
	}
	// The release's configuration: the keys it knew, no new one.
	cfg, db = writeConfig(t, "timezone: Europe/Berlin\n")
	if err := os.WriteFile(db, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	before = dumpTables(t, db, nil)
	rawDB, err := sql.Open("sqlite", "file:"+db+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	var version int
	if err := rawDB.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	_ = rawDB.Close()
	if version != 7 {
		t.Fatalf("the fixture is at schema %d, want 7", version)
	}
	for tb, n := range f.Rows {
		if len(before.rows[tb]) != n {
			t.Fatalf("table %s: %d rows in the fixture, %d recorded", tb, len(before.rows[tb]), n)
		}
	}
	return cfg, db, f, before
}

// checkMigratedShape checks what the migration added: schema 8, two columns
// that are null in every row that existed, two empty tables, a sound file.
// firstNewSampleID is the id from which a usage sample is new (0: all null).
func checkMigratedShape(t *testing.T, db string, before tableDump, oldSamples int) {
	t.Helper()
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
	var oldNonNull int
	if err := mig.QueryRow(`SELECT COUNT(*) FROM usage_samples WHERE weight_source IS NOT NULL AND id <= ?`, oldSamples).Scan(&oldNonNull); err != nil {
		t.Fatal(err)
	}
	_ = mig.Close()
	if migrated != 8 || integrity != "ok" {
		t.Fatalf("after the start: schema version %d (want 8), integrity %q", migrated, integrity)
	}
	if oldNonNull != 0 {
		t.Errorf("usage_samples.weight_source is set in %d of the rows the release wrote, want none", oldNonNull)
	}
	full := dumpTables(t, db, nil)
	var addedTables []string
	for tb := range full.columns {
		if _, ok := before.columns[tb]; !ok {
			addedTables = append(addedTables, tb)
			if len(full.rows[tb]) != 0 {
				t.Errorf("the new table %s has %d rows after the upgrade, want none", tb, len(full.rows[tb]))
			}
		}
	}
	sort.Strings(addedTables)
	if want := []string{"reported_weights", "reported_weights_meta"}; !reflect.DeepEqual(addedTables, want) {
		t.Errorf("new tables %v, want %v", addedTables, want)
	}
	addedCols := map[string][]string{}
	for tb, cols := range full.columns {
		if before.columns[tb] == nil {
			continue
		}
		old := map[string]bool{}
		for _, c := range before.columns[tb] {
			old[c] = true
		}
		for _, c := range cols {
			if !old[c] {
				addedCols[tb] = append(addedCols[tb], c)
			}
		}
	}
	if want := map[string][]string{"usage_samples": {"weight_source"}, "setups": {"weight_override"}}; !reflect.DeepEqual(addedCols, want) {
		t.Errorf("new columns %v, want %v", addedCols, want)
	}
	for tb, cols := range addedCols {
		for c, n := range nonNullInNewColumns(t, db, tb, cols) {
			// setups.weight_override is null everywhere; the samples a poll of
			// the new code writes after the start are not "rows of the release".
			if n != 0 && !(tb == "usage_samples" && c == "weight_source") {
				t.Errorf("%s.%s is not null in %d rows after the upgrade", tb, c, n)
			}
		}
	}
}

func checkRecordedNumbers(t *testing.T, db string, f v110Numbers) {
	t.Helper()
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var users, holdIDs []int64
	for _, id := range f.UserIDs {
		users = append(users, id)
	}
	for _, id := range f.HoldIDs {
		holdIDs = append(holdIDs, id)
	}
	got := readUpgradeNumbers(t, st, users, holdIDs)
	if !reflect.DeepEqual(got.Balances, f.Balances) {
		t.Errorf("balances after the upgrade %v, recorded %v", got.Balances, f.Balances)
	}
	if !reflect.DeepEqual(got.Sums, f.Sums) {
		t.Errorf("period sums after the upgrade %v, recorded %v", got.Sums, f.Sums)
	}
	if !reflect.DeepEqual(got.Charges, f.Charges) {
		t.Errorf("hold charges after the upgrade %v, recorded %v", got.Charges, f.Charges)
	}
	if rep, err := st.RebuildBalances(context.Background(), "2026-11"); err != nil || rep.Changed != 0 {
		t.Errorf("balance caches need repair after the upgrade: %+v, %v", rep, err)
	}
}

// WGT-19 (migration): opening the release's database with the new code, which
// is what the application does first at start, takes schema 7 to 8 and
// rewrites no row: every row of every table the release had, in the columns it
// had, is the same afterwards (schema_migrations gains the row of 0008 and
// nothing else changes). The new columns are null and the new tables are empty.
func TestWGT19_MigrationRewritesNoRowOfV110Database(t *testing.T) {
	_, db, f, before := copyV110Database(t)
	st, err := store.Open(db) // migrates to the current schema
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	after := dumpTables(t, db, before.columns)
	for tb, want := range before.rows {
		got := after.rows[tb]
		if tb == "schema_migrations" {
			have := map[string]bool{}
			for _, x := range got {
				have[x] = true
			}
			for _, x := range want {
				if !have[x] {
					t.Errorf("schema_migrations: the row %q of the release is gone or changed", x)
				}
			}
			if len(got) != len(want)+1 {
				t.Errorf("schema_migrations has %d rows after the migration, want %d", len(got), len(want)+1)
			}
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("table %s: the rows changed by the migration (%d before, %d after)\nfirst difference: %s", tb, len(want), len(got), firstRowDifference(want, got))
		}
	}
	checkMigratedShape(t, db, before, f.Rows["usage_samples"])
	checkRecordedNumbers(t, db, f)
}

// WGT-19 (application start): the new binary's Run, started on a copy of the
// release's database with the release's configuration, migrates it and runs
// its first poll. The poll is the only thing that touches the old rows: no old
// row of any table is changed or removed, except that the first poll completes
// the pending release of hold 3 (its setup is not deployed in this run) and
// appends to the tables a poll writes, and the start records the seed marker in
// settings. Every balance and per-hold charge is the one recorded from v1.1.0.
func TestWGT19_ApplicationStartUpgradesV110DatabaseAndKeepsEveryNumber(t *testing.T) {
	cfg, db, f, before := copyV110Database(t)
	// The clock is before the day the compaction would roll up, so nothing is
	// compacted: the compaction is not a migration.
	r := startWithClock(t, cfg, db, hourglasstest.NewFakeClock(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)))
	waitForFirstPoll(t, db, len(before.rows["collector_runs"]))
	if strings.Contains(r.out.String(), "some were wrong") {
		t.Errorf("start-up had to repair a balance cache:\n%s", r.out.String())
	}
	if err := r.stop(); err != nil {
		t.Fatal(err)
	}
	after := dumpTables(t, db, before.columns)
	mayGrow := map[string]bool{"usage_samples": true, "collector_runs": true, "pressure_samples": true, "settings": true}
	for tb, want := range before.rows {
		got := after.rows[tb]
		if tb == "schema_migrations" {
			continue // checked below: the row of 0008 is the only addition
		}
		switch {
		case tb == "holds":
			// The pending release of hold 3 is completed by the first poll.
			diff := 0
			for _, x := range want {
				if !contains(got, x) {
					diff++
				}
			}
			if len(got) != len(want) || diff != 1 {
				t.Errorf("holds: %d rows changed, want exactly the pending release of hold 3 (%d before, %d after)", diff, len(want), len(got))
			}
		case mayGrow[tb]:
			for _, x := range want {
				if !contains(got, x) {
					t.Errorf("table %s: the row %q of the release is gone or changed", tb, x)
				}
			}
		default:
			if !reflect.DeepEqual(got, want) {
				t.Errorf("table %s: rows changed by the start (%d before, %d after)\nfirst difference: %s", tb, len(want), len(got), firstRowDifference(want, got))
			}
		}
	}
	checkMigratedShape(t, db, before, f.Rows["usage_samples"])
	checkRecordedNumbers(t, db, f)
}

// waitForFirstPoll blocks until the application's first poll has finished
// (the database has more runs than the release's and the newest one has its
// finish time), so the test stops it after, not before. The run's row is
// inserted when the poll starts, so its presence alone is not enough.
func waitForFirstPoll(t *testing.T, db string, runsBefore int) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		h, err := sql.Open("sqlite", "file:"+db+"?mode=ro")
		if err != nil {
			t.Fatal(err)
		}
		var n int
		err = h.QueryRow(`SELECT COUNT(*) FROM collector_runs WHERE
			(SELECT finished_at FROM collector_runs ORDER BY id DESC LIMIT 1) IS NOT NULL`).Scan(&n)
		_ = h.Close()
		if err == nil && n > runsBefore {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the application's first poll did not finish")
}

func contains(rows []string, x string) bool {
	for _, r := range rows {
		if r == x {
			return true
		}
	}
	return false
}

func firstRowDifference(want, got []string) string {
	seen := map[string]int{}
	for _, x := range want {
		seen[x]++
	}
	for _, x := range got {
		if seen[x] > 0 {
			seen[x]--
		} else {
			return "new or changed: " + x
		}
	}
	for x, n := range seen {
		if n > 0 {
			return "missing or changed: " + x
		}
	}
	return "none"
}
