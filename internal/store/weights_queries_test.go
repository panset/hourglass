// SPDX-License-Identifier: MIT

package store

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

// The weights report is stored whole with its header, read back in name
// order, and a failed replace leaves the previous report intact (WGT-7).
func TestWGT7_StoreReplacesWeightsReportWhole(t *testing.T) {
	s := openTemp(t)
	if _, _, err := s.GetReportedWeights(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("before first: %v", err)
	}
	h := ReportedHeader{ObservedAt: t0, ReceivedAt: t0.Add(time.Second), ServiceAccount: "svc"}
	set := []ReportedWeight{
		{"lab-07", 0.8, nil},
		{"lab-03", 1.15, map[string]string{"memory-gb": "512", "tier": "gold"}},
	}
	must(s, s.ReplaceReportedWeights(ctx, h, set))
	gh, gs, err := s.GetReportedWeights(ctx)
	want := []ReportedWeight{
		{"lab-03", 1.15, map[string]string{"memory-gb": "512", "tier": "gold"}},
		{"lab-07", 0.8, map[string]string{}},
	}
	if err != nil || gh != h || !reflect.DeepEqual(gs, want) {
		t.Fatalf("got %+v %+v %v", gh, gs, err)
	}

	// A new report replaces the whole set: lab-03 is gone.
	h2 := ReportedHeader{ObservedAt: t0.Add(time.Minute), ReceivedAt: t0.Add(time.Minute), ServiceAccount: "svc2"}
	must(s, s.ReplaceReportedWeights(ctx, h2, []ReportedWeight{{SetupName: "lab-09", Weight: 2}}))
	gh, gs, err = s.GetReportedWeights(ctx)
	if err != nil || gh != h2 || !reflect.DeepEqual(gs, []ReportedWeight{{"lab-09", 2, map[string]string{}}}) {
		t.Fatalf("second report: %+v %+v %v", gh, gs, err)
	}

	// The empty set still has a header.
	h3 := ReportedHeader{ObservedAt: t0.Add(2 * time.Minute), ReceivedAt: t0.Add(2 * time.Minute), ServiceAccount: "svc"}
	must(s, s.ReplaceReportedWeights(ctx, h3, nil))
	gh, gs, err = s.GetReportedWeights(ctx)
	if err != nil || gh != h3 || gs == nil || len(gs) != 0 {
		t.Fatalf("empty set: %+v %#v %v", gh, gs, err)
	}

	// A duplicate name fails and leaves the previous report intact.
	must(s, s.ReplaceReportedWeights(ctx, h2, []ReportedWeight{{SetupName: "lab-09", Weight: 2}}))
	h4 := ReportedHeader{ObservedAt: t0.Add(time.Hour), ReceivedAt: t0.Add(time.Hour), ServiceAccount: "x"}
	err = s.ReplaceReportedWeights(ctx, h4, []ReportedWeight{{SetupName: "a", Weight: 1}, {SetupName: "a", Weight: 2}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	if gh, gs, _ = s.GetReportedWeights(ctx); gh != h2 || len(gs) != 1 || gs[0].SetupName != "lab-09" {
		t.Errorf("failed replace changed state: %+v %+v", gh, gs)
	}

	// The weights report is separate from the deployments report.
	if _, _, err := s.GetReportedDeployments(ctx); !errors.Is(err, ErrNotFound) {
		t.Errorf("deployments report appeared: %v", err)
	}
}

// A concurrent reader of the weights report never sees a header from one
// report with the set of another (WGT-7).
func TestWGT7_StoreWeightsReplaceIsAtomicForReaders(t *testing.T) {
	s := openTemp(t)
	mk := func(i int) (ReportedHeader, []ReportedWeight) {
		h := ReportedHeader{ObservedAt: t0.Add(time.Duration(i) * time.Second), ReceivedAt: t0, ServiceAccount: "svc"}
		var set []ReportedWeight
		for j := 0; j < i%5; j++ {
			set = append(set, ReportedWeight{SetupName: fmt.Sprintf("lab-%02d", j), Weight: float64(i)})
		}
		return h, set
	}
	h0, s0 := mk(0)
	must(s, s.ReplaceReportedWeights(ctx, h0, s0))
	var wg sync.WaitGroup
	stop := make(chan struct{})
	bad := make(chan string, 1)
	report := func(m string) {
		select {
		case bad <- m:
		default:
		}
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			h, set, err := s.GetReportedWeights(ctx)
			if err != nil {
				report(err.Error())
				return
			}
			i := int(h.ObservedAt.Sub(t0) / time.Second)
			if len(set) != i%5 {
				report("mixed report")
				return
			}
			for _, w := range set {
				if w.Weight != float64(i) {
					report("weight from another report")
					return
				}
			}
		}
	}()
	for i := 1; i <= 60; i++ {
		h, set := mk(i)
		if err := s.ReplaceReportedWeights(ctx, h, set); err != nil {
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

// The stored weights report survives closing and reopening the database
// (WGT-7, B3).
func TestWGT7_StoreWeightsReportSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.db")
	s := must(Open(path))
	h := ReportedHeader{ObservedAt: t0, ReceivedAt: t0.Add(time.Second), ServiceAccount: "svc"}
	set := []ReportedWeight{{"lab-03", 1.15, map[string]string{"memory-gb": "512"}}}
	must(s, s.ReplaceReportedWeights(ctx, h, set))
	s.Close()
	s = must(Open(path))
	defer s.Close()
	gh, gs, err := s.GetReportedWeights(ctx)
	if err != nil || gh != h || !reflect.DeepEqual(gs, set) {
		t.Fatalf("after reopen: %+v %+v %v", gh, gs, err)
	}
}

// usage_samples.weight_source is written and read back; an empty source is
// stored as null, as every existing caller writes it for now.
func TestUsageSampleWeightSourceRoundTrip(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "a")
	h := must(s.CreateHold(ctx, newSetup(t, s, "lab-01"), u.ID, t0))
	must(s.InsertUsageSample(ctx, UsageSample{HoldID: h.ID, PolledAt: t0, Weight: 1.15, Multiplier: 1, DebitMT: 96, Reason: "r", WeightSource: "reported"}))
	must(s.InsertUsageSample(ctx, UsageSample{HoldID: h.ID, PolledAt: t0.Add(time.Minute), Weight: 1, RuleSelector: "k=v", Multiplier: 1, DebitMT: 17, Reason: "r"}))
	got := must(s.ListUsageSamples(ctx, h.ID))
	if len(got) != 2 || got[0].WeightSource != "reported" || got[1].WeightSource != "" || got[1].RuleSelector != "k=v" {
		t.Fatalf("samples: %+v", got)
	}
	var null int
	s.sqldb.QueryRow(`SELECT COUNT(*) FROM usage_samples WHERE weight_source IS NULL`).Scan(&null)
	if null != 1 {
		t.Errorf("null sources = %d, want 1", null)
	}
	last := must(s.HoldLastSamples(ctx, []int64{h.ID}))
	if last[h.ID].WeightSource != "" || last[h.ID].Weight != 1 {
		t.Errorf("last sample: %+v", last[h.ID])
	}
	must(s.InsertUsageSample(ctx, UsageSample{HoldID: h.ID, PolledAt: t0.Add(2 * time.Minute), Weight: 3, Multiplier: 1, DebitMT: 50, Reason: "r", WeightSource: "policy"}))
	if last = must(s.HoldLastSamples(ctx, []int64{h.ID})); last[h.ID].WeightSource != "policy" {
		t.Errorf("last sample: %+v", last[h.ID])
	}
}

// setups.weight_override: CreateSetup writes it, the reads return it,
// UpdateSetup leaves it alone, SetSetupWeightOverride sets and clears it.
func TestSetupWeightOverrideColumn(t *testing.T) {
	s := openTemp(t)
	w := 2.5
	id := must(s.CreateSetup(ctx, Setup{Name: "lab-01", Enabled: true, WeightOverride: &w}))
	plain := newSetup(t, s, "lab-02")
	if got := must(s.GetSetup(ctx, id)); got.WeightOverride == nil || *got.WeightOverride != 2.5 {
		t.Fatalf("created: %+v", got)
	}
	if got := must(s.GetSetup(ctx, plain)); got.WeightOverride != nil {
		t.Fatalf("no override: %+v", got)
	}
	// An update that carries no override (every existing caller) keeps it.
	must(s, s.UpdateSetup(ctx, Setup{ID: id, Name: "lab-01b", Enabled: true}))
	if got := must(s.GetSetup(ctx, id)); got.WeightOverride == nil || *got.WeightOverride != 2.5 || got.Name != "lab-01b" {
		t.Fatalf("after update: %+v", got)
	}
	zero := 0.0
	must(s, s.SetSetupWeightOverride(ctx, plain, &zero))
	list := must(s.ListSetups(ctx, false))
	if len(list) != 2 || list[1].WeightOverride == nil || *list[1].WeightOverride != 0 {
		t.Fatalf("list: %+v", list)
	}
	must(s, s.SetSetupWeightOverride(ctx, id, nil))
	if got := must(s.GetSetup(ctx, id)); got.WeightOverride != nil {
		t.Fatalf("cleared: %+v", got)
	}
	if err := s.SetSetupWeightOverride(ctx, 9999, &w); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id: %v", err)
	}
}

// Migration 0008 on a database at schema version 7 with rows in every table
// it touches and in their neighbours: it adds the two tables and two columns,
// rewrites no existing row (every old column of every old row is unchanged),
// the new columns are null on old rows, and the file passes integrity_check.
func TestNFR_Migration0008UpgradesVersion7DatabaseWithoutRewritingRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.db")
	s := openAtVersion(t, path, 7)
	a := newUser(t, s, "a")
	// Raw inserts: CreateSetup and the sample insert name the new columns.
	sr := must(s.sqldb.Exec(`INSERT INTO setups (name, details_json, enabled) VALUES ('lab-01', '{"x":1}', 1), ('lab-02', '{}', 0)`))
	setupID := must(sr.LastInsertId()) - 1
	must(s.sqldb.Exec(`INSERT INTO setup_labels (setup_id, key, value) VALUES (?, 'tier', 'gold')`, setupID))
	hr := must(s.sqldb.Exec(`INSERT INTO holds (setup_id, user_id, acquired_at, metering_started_at) VALUES (?, ?, ?, ?)`,
		setupID, a.ID, FormatTime(t0), FormatTime(t0)))
	holdID := must(hr.LastInsertId())
	for i := 1; i <= 3; i++ {
		must(s.sqldb.Exec(`INSERT INTO usage_samples (hold_id, polled_at, weight, rule_selector, pressure, multiplier, debit_mt, reason)
			VALUES (?, ?, 1.5, ?, 0.2, 1, 25, 'metered')`, holdID, FormatTime(t0.Add(time.Duration(i)*time.Minute)),
			map[bool]any{true: "tier=gold", false: nil}[i%2 == 1]))
		must(s.sqldb.Exec(`INSERT INTO ledger (user_id, period, type, amount_mt, hold_id, created_at) VALUES (?, '2026-11', 'debit', -25, ?, ?)`,
			a.ID, holdID, FormatTime(t0.Add(time.Duration(i)*time.Minute))))
	}
	must(s.sqldb.Exec(`INSERT INTO reported_deployments_meta VALUES (?, ?, 'svc')`, FormatTime(t0), FormatTime(t0)))
	must(s.sqldb.Exec(`INSERT INTO reported_deployments VALUES ('lab-01', '{"k":"v"}')`))
	must(s.sqldb.Exec(`INSERT INTO settings (key, value_json, updated_at) VALUES ('default_weight', '2', ?)`, FormatTime(t0)))

	tables := []string{"users", "setups", "setup_labels", "holds", "usage_samples", "ledger",
		"reported_deployments_meta", "reported_deployments", "settings", "schema_migrations"}
	before := map[string][]map[string]any{}
	for _, tb := range tables {
		before[tb] = dumpTable(t, s.sqldb, tb)
	}
	s.Close()

	s = must(Open(path))
	defer s.Close()
	var v int
	var ic string
	s.sqldb.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&v)
	s.sqldb.QueryRow(`PRAGMA integrity_check`).Scan(&ic)
	if v != 8 || ic != "ok" {
		t.Fatalf("after upgrade: version %d, integrity %q", v, ic)
	}
	for _, tb := range tables {
		after := dumpTable(t, s.sqldb, tb)
		if tb == "schema_migrations" {
			after = after[:len(after)-1] // the row for 0008 itself
		}
		if len(after) != len(before[tb]) {
			t.Fatalf("%s: %d rows after, %d before", tb, len(after), len(before[tb]))
		}
		for i, row := range before[tb] {
			for col, val := range row {
				if !reflect.DeepEqual(after[i][col], val) {
					t.Errorf("%s row %d: %s = %v, was %v", tb, i, col, after[i][col], val)
				}
			}
			for col, val := range after[i] {
				if _, old := row[col]; !old && val != nil {
					t.Errorf("%s row %d: new column %s = %v, want null", tb, i, col, val)
				}
			}
		}
	}
	// The old rows read through the new store: null source, no override.
	if got := must(s.ListUsageSamples(ctx, holdID)); len(got) != 3 || got[0].WeightSource != "" || got[0].RuleSelector != "tier=gold" {
		t.Errorf("samples after upgrade: %+v", got)
	}
	if got := must(s.GetSetup(ctx, setupID)); got.WeightOverride != nil || got.Labels["tier"] != "gold" {
		t.Errorf("setup after upgrade: %+v", got)
	}
	if _, _, err := s.GetReportedWeights(ctx); !errors.Is(err, ErrNotFound) {
		t.Errorf("weights report after upgrade: %v", err)
	}
	if _, set, err := s.GetReportedDeployments(ctx); err != nil || len(set) != 1 {
		t.Errorf("deployments report after upgrade: %+v %v", set, err)
	}
}

// dumpTable returns every row of table, in rowid (or key) order, as column ->
// value.
func dumpTable(t *testing.T, db *sql.DB, table string) []map[string]any {
	t.Helper()
	order := "rowid"
	if table == "schema_migrations" {
		order = "version"
	}
	rows, err := db.Query(`SELECT * FROM ` + table + ` ORDER BY ` + order)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols := must(rows.Columns())
	var out []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		m := map[string]any{}
		for i, c := range cols {
			m[c] = vals[i]
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
