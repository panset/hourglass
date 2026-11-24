// SPDX-License-Identifier: MIT

package store

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// Tests of the storage half of the ledger roll-up (design A1-A3): LED-7,
// LED-8, LED-9, LED-15 and UPG-1.

func losAngeles(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skipf("no tz database: %v", err)
	}
	return loc
}

// allLedger returns every ledger row in id order.
func allLedger(t *testing.T, s *Store) []LedgerEntry {
	t.Helper()
	return must(collect(ctx, s.db, scanLedger, `SELECT `+ledgerCols+` FROM ledger ORDER BY id`))
}

func markerRows(t *testing.T, s *Store) int {
	t.Helper()
	var n int
	if err := s.sqldb.QueryRow(`SELECT COUNT(*) FROM ledger_compaction`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// compactAll works off everything due at now the way the ledger service will:
// bounded batches, each compacted in one transaction. It returns the number
// of raw rows replaced and checks the marker after every step.
func compactAll(t *testing.T, s *Store, now time.Time, loc *time.Location) int {
	t.Helper()
	var from time.Time
	rows := 0
	for i := 0; ; i++ {
		if i > 10000 {
			t.Fatal("compaction does not terminate")
		}
		due := must(s.DueLedgerGroups(ctx, now, loc, from, LedgerCompactionStepRows))
		if len(due.Groups) == 0 {
			// A step bounded by days may find nothing and still have more to
			// search; it is done only when the search reached the cutoff.
			if !due.Next.Before(LedgerCompactionCutoff(now, loc)) {
				return rows
			}
			from = due.Next
			continue
		}
		err := s.Tx(ctx, func(tx *Tx) error {
			for _, g := range due.Groups {
				n, err := tx.CompactLedgerGroup(ctx, g)
				if err != nil {
					return err
				}
				rows += n
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if n := markerRows(t, s); n != 0 {
			t.Fatalf("marker rows after step %d: %d", i, n)
		}
		from = due.Next
	}
}

// expectMerge is what the ledger must hold after the rows of each id group
// were rolled up: the group's newest row with the summed amount, no actor or
// reason and the count; every other row as it was.
func expectMerge(before []LedgerEntry, groups [][]int64) []LedgerEntry {
	byID := map[int64]LedgerEntry{}
	for _, e := range before {
		byID[e.ID] = e
	}
	drop := map[int64]bool{}
	merged := map[int64]LedgerEntry{}
	for _, g := range groups {
		var newest int64
		var sum int64
		for _, id := range g {
			drop[id] = true
			sum += byID[id].AmountMT
			newest = max(newest, id)
		}
		e := byID[newest]
		e.AmountMT, e.ActorID, e.Reason, e.RollupCount = sum, nil, "", len(g)
		merged[newest] = e
	}
	var out []LedgerEntry
	for _, e := range before {
		if m, ok := merged[e.ID]; ok {
			out = append(out, m)
		} else if !drop[e.ID] {
			out = append(out, e)
		}
	}
	return out
}

func ids(es ...LedgerEntry) []int64 {
	out := make([]int64, len(es))
	for i, e := range es {
		out[i] = e.ID
	}
	return out
}

func diffLedger(t *testing.T, got, want []LedgerEntry) {
	t.Helper()
	if reflect.DeepEqual(got, want) {
		return
	}
	t.Errorf("ledger has %d rows, want %d", len(got), len(want))
	for i := 0; i < max(len(got), len(want)); i++ {
		var g, w any = "-", "-"
		if i < len(got) {
			g = fmt.Sprintf("%+v", got[i])
		}
		if i < len(want) {
			w = fmt.Sprintf("%+v", want[i])
		}
		if g != w {
			t.Errorf("row %d:\n got %v\nwant %v", i, g, w)
		}
	}
}

// LED-7: in a non-UTC zone, raw debits of one user, hold, period and local day
// whose day is earlier than today - 7 become one row with their total, the
// newest id and created_at, and the count. Untouched: the day today - 7 itself
// (the edge), a one-row group, grants and other types, a debit with no hold,
// other users' rows (grouped on their own), and a rolled-up row (final). A
// period boundary inside a local day gives one row per period. A second run
// changes nothing; moving today forward makes the edge day due.
func TestLED7_CompactsDueRawDebitsOnly(t *testing.T) {
	la := losAngeles(t)
	s := openTemp(t)
	u, v := newUser(t, s, "u"), newUser(t, s, "v")
	admin := must(s.CreateLocalAdmin(ctx, "admin", "hash", false, t0))
	h1 := must(s.CreateHold(ctx, newSetup(t, s, "a"), u.ID, t0)).ID
	h2 := must(s.CreateHold(ctx, newSetup(t, s, "b"), u.ID, t0)).ID
	h3 := must(s.CreateHold(ctx, newSetup(t, s, "c"), v.ID, t0)).ID
	at := func(m time.Month, d, h, min int) time.Time { return time.Date(2026, m, d, h, min, 0, 0, la) }
	add := func(user int64, hold *int64, period, typ string, mt int64, when time.Time) LedgerEntry {
		e := LedgerEntry{UserID: user, Period: period, Type: typ, AmountMT: mt, HoldID: hold}
		if typ == "grant" {
			e.ActorID, e.Reason = &admin.ID, "bonus"
		}
		return must(s.AppendLedger(ctx, e, "2026-11", when))
	}
	debit := func(user, hold int64, period string, mt int64, when time.Time) LedgerEntry {
		return add(user, &hold, period, "debit", -mt, when)
	}
	for _, x := range []int64{u.ID, v.ID} {
		add(x, nil, "2026-10", "allowance", 300000, at(10, 1, 0, 0))
		add(x, nil, "2026-11", "allowance", 300000, at(11, 1, 0, 0))
	}
	// Oct 31 (local): a period boundary inside the day, as left by a
	// timezone change: two rows of each period, one roll-up per period.
	c1, c2 := debit(v.ID, h3, "2026-10", 11, at(10, 31, 10, 0)), debit(v.ID, h3, "2026-10", 12, at(10, 31, 12, 0))
	d1, d2 := debit(v.ID, h3, "2026-11", 13, at(10, 31, 18, 0)), debit(v.ID, h3, "2026-11", 14, at(10, 31, 20, 0))
	// Nov 10: three debits of h2 merge; the grant and the hold-less debit
	// between them stay.
	b1 := debit(u.ID, h2, "2026-11", 21, at(11, 10, 8, 0))
	add(u.ID, nil, "2026-11", "grant", 5000, at(11, 10, 8, 30))
	b2 := debit(u.ID, h2, "2026-11", 22, at(11, 10, 9, 0))
	add(u.ID, nil, "2026-11", "debit", -7, at(11, 10, 9, 30))
	b3 := debit(u.ID, h2, "2026-11", 23, at(11, 10, 10, 0))
	// Nov 12 (today - 8): h1 from local midnight to 23:59 local (07:59 UTC
	// the next day) is one group; v's h3 is its own group; h2 polled once.
	a1 := debit(u.ID, h1, "2026-11", 31, at(11, 12, 0, 0))
	e1 := debit(v.ID, h3, "2026-11", 41, at(11, 12, 0, 5))
	debit(u.ID, h2, "2026-11", 51, at(11, 12, 9, 0))
	a2 := debit(u.ID, h1, "2026-11", 32, at(11, 12, 12, 0))
	e2 := debit(v.ID, h3, "2026-11", 42, at(11, 12, 12, 5))
	a3 := debit(u.ID, h1, "2026-11", 33, at(11, 12, 23, 59))
	// Nov 13 (today - 7): not due, from its first millisecond.
	g1 := debit(u.ID, h1, "2026-11", 61, at(11, 13, 0, 0))
	g2 := debit(u.ID, h1, "2026-11", 62, at(11, 13, 1, 0))
	// Recent rows.
	debit(u.ID, h1, "2026-11", 71, at(11, 19, 1, 0))
	debit(u.ID, h1, "2026-11", 72, at(11, 19, 2, 0))

	now := at(11, 20, 10, 0)
	before := allLedger(t, s)
	balances := map[int64]int64{u.ID: must(s.GetUser(ctx, u.ID)).BalanceMT, v.ID: must(s.GetUser(ctx, v.ID)).BalanceMT}
	if n := compactAll(t, s, now, la); n != 12 {
		t.Errorf("replaced %d rows, want 12", n)
	}
	want := expectMerge(before, [][]int64{ids(c1, c2), ids(d1, d2), ids(b1, b2, b3), ids(a1, a2, a3), ids(e1, e2)})
	diffLedger(t, allLedger(t, s), want)
	for id, b := range balances {
		if got := must(s.GetUser(ctx, id)).BalanceMT; got != b {
			t.Errorf("user %d balance cache %d, was %d", id, got, b)
		}
	}
	a := allLedger(t, s)
	for _, e := range a {
		if e.ID == a3.ID && (e.RollupCount != 3 || e.AmountMT != -96 || !e.CreatedAt.Equal(a3.CreatedAt) || e.HoldID == nil || *e.HoldID != h1) {
			t.Errorf("Nov 12 h1 roll-up %+v", e)
		}
	}

	// Idempotent, also at the last instant of the same local date.
	if n := compactAll(t, s, at(11, 20, 23, 59), la); n != 0 {
		t.Errorf("second run replaced %d rows", n)
	}
	diffLedger(t, allLedger(t, s), want)

	// A rolled-up row is final: raw rows that later land on its day form a
	// new group and leave it alone.
	l1 := debit(u.ID, h1, "2026-11", 81, at(11, 12, 5, 0))
	l2 := debit(u.ID, h1, "2026-11", 82, at(11, 12, 6, 0))
	snap := allLedger(t, s)
	if n := compactAll(t, s, now, la); n != 2 {
		t.Errorf("late rows: replaced %d", n)
	}
	diffLedger(t, allLedger(t, s), expectMerge(snap, [][]int64{ids(l1, l2)}))

	// The next local date makes Nov 13 due.
	snap = allLedger(t, s)
	if n := compactAll(t, s, at(11, 21, 0, 0), la); n != 2 {
		t.Errorf("edge day next date: replaced %d", n)
	}
	diffLedger(t, allLedger(t, s), expectMerge(snap, [][]int64{ids(g1, g2)}))
}

// LED-7: the cutoff is the start of the local date today - 7, and in UTC the
// same instants group differently than in Los Angeles.
func TestLED7_CutoffIsLocalDateMinusSeven(t *testing.T) {
	la := losAngeles(t)
	now := time.Date(2026, 11, 20, 0, 30, 0, 0, la) // 08:30 UTC Nov 20
	if got, want := LedgerCompactionCutoff(now, la), time.Date(2026, 11, 13, 0, 0, 0, 0, la); !got.Equal(want) {
		t.Errorf("LA cutoff %v, want %v", got, want)
	}
	late := time.Date(2026, 11, 20, 23, 30, 0, 0, la) // 07:30 UTC Nov 21
	if got, want := LedgerCompactionCutoff(late, la), time.Date(2026, 11, 13, 0, 0, 0, 0, la); !got.Equal(want) {
		t.Errorf("LA cutoff late in the day %v, want %v", got, want)
	}
	if got, want := LedgerCompactionCutoff(late, nil), time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("UTC cutoff %v, want %v", got, want)
	}
}

// The search for due groups takes whole groups up to the row bound, a larger
// group alone, never returns a one-row group, and resumes where it stopped.
func TestLED7_DueLedgerGroupsBoundedWholeGroups(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	var holds []int64
	for i := 0; i < 3; i++ {
		holds = append(holds, must(s.CreateHold(ctx, newSetup(t, s, fmt.Sprintf("s%d", i)), u.ID, t0)).ID)
	}
	day := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	// Day 1: hold 0 has 5 rows, hold 1 has 3, hold 2 has 1. Day 2: hold 0 has 2.
	for i, n := range []int{5, 3, 1} {
		h := holds[i]
		for k := 0; k < n; k++ {
			must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: "debit", AmountMT: -1, HoldID: &h}, "2026-11", day.Add(time.Duration(k+1)*time.Hour)))
		}
	}
	for k := 0; k < 2; k++ {
		must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: "debit", AmountMT: -1, HoldID: &holds[0]}, "2026-11", day.Add(24*time.Hour+time.Duration(k)*time.Hour)))
	}
	now := day.Add(30 * 24 * time.Hour)
	rows := func(d DueLedgerGroups) []int {
		var out []int
		for _, g := range d.Groups {
			out = append(out, g.Rows)
		}
		return out
	}
	// Bound 4: the 5-row group alone; Next stays on day 1 (hold 1 not taken).
	d := must(s.DueLedgerGroups(ctx, now, nil, time.Time{}, 4))
	if !reflect.DeepEqual(rows(d), []int{5}) || !d.Next.Equal(day) {
		t.Fatalf("bound 4: %v next %v", rows(d), d.Next)
	}
	// Bound 10 from the start: day 1's two groups and day 2's; the bound is
	// reached, so Next is the end of day 2.
	d = must(s.DueLedgerGroups(ctx, now, nil, time.Time{}, 10))
	if !reflect.DeepEqual(rows(d), []int{5, 3, 2}) || !d.Next.Equal(day.Add(48*time.Hour)) {
		t.Fatalf("bound 10: %v next %v", rows(d), d.Next)
	}
	// Bound 11: everything, and the search ran to the cutoff.
	d = must(s.DueLedgerGroups(ctx, now, nil, time.Time{}, 11))
	if !reflect.DeepEqual(rows(d), []int{5, 3, 2}) || !d.Next.Equal(LedgerCompactionCutoff(now, nil)) {
		t.Fatalf("bound 11: %v next %v", rows(d), d.Next)
	}
	// Bound 8: day 1 exactly; Next is day 2.
	d = must(s.DueLedgerGroups(ctx, now, nil, time.Time{}, 8))
	if !reflect.DeepEqual(rows(d), []int{5, 3}) || !d.Next.Equal(day.Add(24*time.Hour)) {
		t.Fatalf("bound 8: %v next %v", rows(d), d.Next)
	}
	d = must(s.DueLedgerGroups(ctx, now, nil, d.Next, 8))
	if !reflect.DeepEqual(rows(d), []int{2}) {
		t.Fatalf("resume: %v", rows(d))
	}
	// Nothing due while day 1 is today - 7 (until the end of Nov 8).
	if d = must(s.DueLedgerGroups(ctx, day.Add(8*24*time.Hour-time.Millisecond), nil, time.Time{}, 8)); len(d.Groups) != 0 {
		t.Fatalf("not yet due: %v", rows(d))
	}
	// After compaction only the one-row group is left: nothing returned.
	compactAll(t, s, now, nil)
	if d = must(s.DueLedgerGroups(ctx, now, nil, time.Time{}, 8)); len(d.Groups) != 0 {
		t.Fatalf("after compaction: %v", rows(d))
	}
}

// LED-7 (A1): finding due rows and compacting one group are served by the
// partial indexes over raw debits, never by a scan of the ledger.
func TestLED7_DueSearchUsesRawDebitIndexes(t *testing.T) {
	s := openTemp(t)
	plan := func(q string, args ...any) string {
		rows, err := s.sqldb.Query(`EXPLAIN QUERY PLAN `+q, args...)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			rows.Scan(&id, &parent, &unused, &detail)
			out = append(out, detail)
		}
		return strings.Join(out, "; ")
	}
	a, b := FormatTime(t0), FormatTime(t0.Add(time.Hour))
	for _, c := range []struct{ q, idx string }{
		{`SELECT MIN(created_at) FROM ledger WHERE ` + rawDebit + ` AND created_at >= ? AND created_at < ?`, "ledger_raw_debit_created"},
		{`SELECT user_id, hold_id, period, COUNT(*), SUM(amount_mt), MAX(id) FROM ledger WHERE ` + rawDebit +
			` AND created_at >= ? AND created_at < ? GROUP BY user_id, hold_id, period HAVING COUNT(*) > 1 ORDER BY MAX(id)`, "ledger_raw_debit_created"},
	} {
		p := plan(c.q, a, b)
		if !strings.Contains(p, "USING INDEX "+c.idx) && !strings.Contains(p, "USING COVERING INDEX "+c.idx) {
			t.Errorf("plan of %.50s...: %s", c.q, p)
		}
		if strings.Contains(p, "SCAN ledger") && !strings.Contains(p, "INDEX") {
			t.Errorf("full scan: %s", p)
		}
	}
}

// LED-9 (A2): the ledger refuses every update, every replace, a plain delete
// of a debit and of a non-debit row, and a delete of a non-debit row even
// while the marker is present; a debit delete is allowed only with the marker.
func TestLED9_AppendOnlyExceptCompaction(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	h := must(s.CreateHold(ctx, newSetup(t, s, "a"), u.ID, t0)).ID
	al := must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: "allowance", AmountMT: 300000}, "2026-11", t0))
	db := must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: "debit", AmountMT: -5, HoldID: &h}, "2026-11", t0))
	refused := func(what, stmt string, args ...any) {
		t.Helper()
		_, err := s.sqldb.Exec(stmt, args...)
		if err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("%s: %v", what, err)
		}
	}
	refused("update", `UPDATE ledger SET amount_mt = 1 WHERE id = ?`, db.ID)
	refused("update rollup_count", `UPDATE ledger SET rollup_count = 2 WHERE id = ?`, db.ID)
	refused("replace", `INSERT OR REPLACE INTO ledger (id, user_id, period, type, amount_mt, created_at) VALUES (?, ?, '2026-11', 'debit', 1, 'x')`, db.ID, u.ID)
	refused("delete debit", `DELETE FROM ledger WHERE id = ?`, db.ID)
	refused("delete non-debit", `DELETE FROM ledger WHERE id = ?`, al.ID)
	refused("delete all", `DELETE FROM ledger`)

	// With the marker inside a transaction: a non-debit delete still fails,
	// a debit delete passes (rolled back here).
	tx := must(s.sqldb.Begin())
	if _, err := tx.Exec(`INSERT INTO ledger_compaction (one) VALUES (1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`DELETE FROM ledger WHERE id = ?`, al.ID); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Errorf("non-debit delete with marker: %v", err)
	}
	if _, err := tx.Exec(`UPDATE ledger SET amount_mt = 1 WHERE id = ?`, db.ID); err == nil {
		t.Error("update with marker allowed")
	}
	if _, err := tx.Exec(`DELETE FROM ledger WHERE id = ?`, db.ID); err != nil {
		t.Errorf("debit delete with marker: %v", err)
	}
	tx.Rollback()
	if got := allLedger(t, s); len(got) != 2 || markerRows(t, s) != 0 {
		t.Errorf("after rollback: %d rows, marker %d", len(got), markerRows(t, s))
	}
	if _, err := s.sqldb.Exec(`INSERT INTO ledger_compaction (one) VALUES (2)`); err == nil {
		t.Error("marker table takes a second row")
	}
}

// LED-9: no compaction step leaves the marker behind: a successful step, a
// step with nothing to do, a step that fails inside (forced by a trigger that
// aborts the insert of the rolled-up row, after the deletes) whose caller
// rolls back, and the same failure in a caller that ignores the error and
// commits. A failed step leaves the group untouched.
func TestLED9_MarkerEmptyAfterEveryStep(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	h := must(s.CreateHold(ctx, newSetup(t, s, "a"), u.ID, t0)).ID
	day := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for k := 0; k < 3; k++ {
		must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: "debit", AmountMT: -10, HoldID: &h}, "2026-11", day.Add(time.Duration(k)*time.Hour)))
	}
	now := day.Add(20 * 24 * time.Hour)
	due := must(s.DueLedgerGroups(ctx, now, nil, time.Time{}, LedgerCompactionStepRows))
	if len(due.Groups) != 1 {
		t.Fatalf("groups %+v", due.Groups)
	}
	g := due.Groups[0]
	before := allLedger(t, s)

	if _, err := s.sqldb.Exec(`CREATE TRIGGER test_fail_rollup BEFORE INSERT ON ledger
		WHEN NEW.rollup_count IS NOT NULL BEGIN SELECT RAISE(ABORT, 'forced failure'); END`); err != nil {
		t.Fatal(err)
	}
	// Failure, caller rolls back.
	err := s.Tx(ctx, func(tx *Tx) error {
		_, err := tx.CompactLedgerGroup(ctx, g)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "forced failure") {
		t.Fatalf("forced failure not reported: %v", err)
	}
	if n := markerRows(t, s); n != 0 {
		t.Errorf("marker after failed step (rolled back): %d", n)
	}
	diffLedger(t, allLedger(t, s), before)
	// Failure, caller ignores it and commits: the step undid itself.
	var stepErr error
	if err := s.Tx(ctx, func(tx *Tx) error {
		_, stepErr = tx.CompactLedgerGroup(ctx, g)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if stepErr == nil {
		t.Fatal("step did not fail")
	}
	if n := markerRows(t, s); n != 0 {
		t.Errorf("marker after failed step (committed): %d", n)
	}
	diffLedger(t, allLedger(t, s), before)
	// Failure outside a caller's transaction.
	if _, err := s.CompactLedgerGroup(ctx, g); err == nil || markerRows(t, s) != 0 {
		t.Errorf("own transaction: err %v marker %d", err, markerRows(t, s))
	}
	diffLedger(t, allLedger(t, s), before)
	if _, err := s.sqldb.Exec(`DROP TRIGGER test_fail_rollup`); err != nil {
		t.Fatal(err)
	}

	// Success, then a step with nothing left to do.
	for i, want := range []int{3, 0} {
		var n int
		if err := s.Tx(ctx, func(tx *Tx) error {
			var err error
			n, err = tx.CompactLedgerGroup(ctx, g)
			return err
		}); err != nil || n != want {
			t.Fatalf("step %d: %d %v", i, n, err)
		}
		if m := markerRows(t, s); m != 0 {
			t.Errorf("marker after step %d: %d", i, m)
		}
	}
	if l := allLedger(t, s); len(l) != 1 || l[0].RollupCount != 3 || l[0].AmountMT != -30 {
		t.Errorf("after compaction %+v", l)
	}
	// Still append-only afterwards.
	if _, err := s.sqldb.Exec(`DELETE FROM ledger`); err == nil {
		t.Error("delete allowed after compaction")
	}
}

// LED-15: a rolled-up row gets no multiplier from DebitMultipliers, although
// a usage sample exists at its created_at (the last poll of the day); a raw
// debit of the same hold keeps its multiplier.
func TestLED15_RolledUpRowHasNoMultiplier(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	h := must(s.CreateHold(ctx, newSetup(t, s, "a"), u.ID, t0)).ID
	day := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	var entries []LedgerEntry
	poll := func(at time.Time, mult float64) {
		must(s.InsertUsageSample(ctx, UsageSample{HoldID: h, PolledAt: at, Weight: 1, Multiplier: mult, DebitMT: 10, Reason: "r"}))
		entries = append(entries, must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-11", Type: "debit", AmountMT: -10, HoldID: &h}, "2026-11", at)))
	}
	poll(day.Add(time.Hour), 1)
	poll(day.Add(2*time.Hour), 3)
	recent := day.Add(15 * 24 * time.Hour)
	poll(recent, 2)
	now := day.Add(16 * 24 * time.Hour)

	if m := must(s.DebitMultipliers(ctx, u.ID, "2026-11")); len(m) != 3 {
		t.Fatalf("before: %v", m)
	}
	if n := compactAll(t, s, now, nil); n != 2 {
		t.Fatalf("replaced %d", n)
	}
	m := must(s.DebitMultipliers(ctx, u.ID, "2026-11"))
	if _, ok := m[entries[1].ID]; ok {
		t.Errorf("rolled-up row %d has multiplier %v", entries[1].ID, m[entries[1].ID])
	}
	if len(m) != 1 || m[entries[2].ID] != 2 {
		t.Errorf("after: %v", m)
	}
}

// ledgerSnapshot is the result of every ledger reader the code has (design,
// "What the code does today") over a fixed data set.
type ledgerSnapshot struct {
	Sums          map[string]int64 // user/period/type ("" = all)
	Rebuilt       map[int64]int64  // RebuildBalance per user
	Balances      map[int64]int64  // users.balance_mt before any rebuild
	HoldCharges   map[int64]int64
	Periods       map[int64][]string
	DebitEntries  map[int64][]DebitEntry // whole range, per user
	DayDebits     map[string]int64       // user/local day via DebitEntries on day bounds
	Counts        map[string]int         // CountLedger user/period/type
	LatestAllow   string
	Through       map[int64]int64 // BalanceThrough per entry id
	Latest        map[string]LedgerEntry
	List          map[string][]LedgerEntry // ListLedger user/period
	Stats         map[string][]LedgerStat
	Overdrawn     int
	PeriodTotals  map[string]PeriodTotals
	Multipliers   map[string]map[int64]float64
	TypeTotals    map[string]int64 // SQL GROUP BY user, period, type
	HoldTotals    map[int64]int64  // SQL GROUP BY hold
	RebuildChange int
}

func takeSnapshot(t *testing.T, s *Store, users, holds []int64, actor int64, periods []string, days []time.Time, loc *time.Location) ledgerSnapshot {
	t.Helper()
	sn := ledgerSnapshot{Sums: map[string]int64{}, Rebuilt: map[int64]int64{}, Balances: map[int64]int64{},
		Periods: map[int64][]string{}, DebitEntries: map[int64][]DebitEntry{}, DayDebits: map[string]int64{},
		Counts: map[string]int{}, Through: map[int64]int64{}, Latest: map[string]LedgerEntry{}, List: map[string][]LedgerEntry{},
		Stats: map[string][]LedgerStat{}, PeriodTotals: map[string]PeriodTotals{}, Multipliers: map[string]map[int64]float64{},
		TypeTotals: map[string]int64{}, HoldTotals: map[int64]int64{}}
	for _, u := range users {
		sn.Balances[u] = must(s.GetUser(ctx, u)).BalanceMT
	}
	types := []string{"", "allowance", "grant", "debit", "expiry", "reset"}
	for _, u := range users {
		for _, p := range periods {
			for _, typ := range types {
				k := fmt.Sprintf("%d/%s/%s", u, p, typ)
				sn.Sums[k] = must(s.SumLedger(ctx, u, p, typ))
				if typ != "" {
					sn.Counts[k] = must(s.CountLedger(ctx, u, p, typ))
				}
			}
			k := fmt.Sprintf("%d/%s", u, p)
			sn.List[k] = must(s.ListLedger(ctx, u, p))
			sn.Multipliers[k] = must(s.DebitMultipliers(ctx, u, p))
			for _, e := range sn.List[k] {
				sn.Through[e.ID] = must(s.BalanceThrough(ctx, u, p, e.ID))
			}
		}
		sn.Periods[u] = must(s.LedgerPeriods(ctx, u))
		sn.DebitEntries[u] = must(s.DebitEntries(ctx, u, days[0], days[len(days)-1]))
		for i := 0; i+1 < len(days); i++ {
			var sum int64
			for _, d := range must(s.DebitEntries(ctx, u, days[i], days[i+1])) {
				sum += d.MT
			}
			sn.DayDebits[fmt.Sprintf("%d/%s", u, days[i].Format("01-02"))] = sum
		}
		for _, typ := range types[1:] {
			e, err := s.LatestLedgerEntry(ctx, u, typ, actor)
			if errors.Is(err, ErrNotFound) {
				e = LedgerEntry{ID: -1} // not found
			} else if err != nil {
				t.Fatal(err)
			}
			sn.Latest[fmt.Sprintf("%d/%s", u, typ)] = e
		}
	}
	sn.HoldCharges = must(s.HoldCharges(ctx, holds))
	sn.LatestAllow = must(s.LatestAllowancePeriod(ctx))
	for _, p := range periods {
		sn.Stats[p] = must(s.LedgerStats(ctx, p, 0, ""))
		sn.PeriodTotals[p] = must(s.LedgerPeriodTotals(ctx, p))
	}
	sn.Overdrawn = must(s.CountOverdrawn(ctx))
	rows := must(s.sqldb.Query(`SELECT user_id, period, type, SUM(amount_mt) FROM ledger GROUP BY user_id, period, type`))
	for rows.Next() {
		var u int64
		var p, typ string
		var sum int64
		rows.Scan(&u, &p, &typ, &sum)
		sn.TypeTotals[fmt.Sprintf("%d/%s/%s", u, p, typ)] = sum
	}
	rows.Close()
	rows = must(s.sqldb.Query(`SELECT hold_id, SUM(amount_mt) FROM ledger WHERE hold_id IS NOT NULL GROUP BY hold_id`))
	for rows.Next() {
		var h, sum int64
		rows.Scan(&h, &sum)
		sn.HoldTotals[h] = sum
	}
	rows.Close()
	// The rebuilds last: they write the cache (to the same values).
	for _, u := range users {
		sn.Rebuilt[u] = must(s.RebuildBalance(ctx, u, "2026-11"))
	}
	sn.RebuildChange = must(s.RebuildBalances(ctx, "2026-11")).Changed
	return sn
}

// modelGroups groups raw debits (with a hold, not rolled up) of one user,
// hold, period and local day earlier than cutoff, keeping groups of 2+ rows.
func modelGroups(rows []LedgerEntry, cutoff time.Time, loc *time.Location) [][]int64 {
	type key struct {
		u, h int64
		p    string
		d    string
	}
	m := map[key][]int64{}
	var order []key
	for _, e := range rows {
		if e.Type != "debit" || e.HoldID == nil || e.RollupCount != 0 || !e.CreatedAt.Before(cutoff) {
			continue
		}
		k := key{e.UserID, *e.HoldID, e.Period, e.CreatedAt.In(loc).Format("2006-01-02")}
		if _, ok := m[k]; !ok {
			order = append(order, k)
		}
		m[k] = append(m[k], e.ID)
	}
	var out [][]int64
	for _, k := range order {
		if len(m[k]) > 1 {
			out = append(out, m[k])
		}
	}
	return out
}

// LED-8: a ledger over two periods, three holds and a grant in the middle of a
// day. Every reader gives the same result after compaction, except where the
// merged rows themselves are the difference, and there exactly that:
// ListLedger is the merged list, CountLedger of debits drops by the rows
// removed, DebitEntries is per row (the day's rows become one at the newest
// created_at), DebitMultipliers loses the merged rows (LED-15), and
// BalanceThrough is the running sum of the merged list, unchanged at the last
// entry of every local day and at every kept row, lower at the mid-day grant
// by exactly that day's earlier debits (A3). users.balance_mt is untouched.
func TestLED8_EveryReaderSameAfterCompaction(t *testing.T) {
	la := losAngeles(t)
	s := openTemp(t)
	u1, u2 := newUser(t, s, "u1"), newUser(t, s, "u2")
	admin := must(s.CreateLocalAdmin(ctx, "admin", "hash", false, t0))
	must0(s.SetUserRoles(ctx, u1.ID, []string{RoleDeveloper}))
	must0(s.SetUserRoles(ctx, u2.ID, []string{RoleDeveloper}))
	h1 := must(s.CreateHold(ctx, newSetup(t, s, "a"), u1.ID, t0)).ID
	h2 := must(s.CreateHold(ctx, newSetup(t, s, "b"), u1.ID, t0)).ID
	h3 := must(s.CreateHold(ctx, newSetup(t, s, "c"), u2.ID, t0)).ID
	holdUser := map[int64]int64{h1: u1.ID, h2: u1.ID, h3: u2.ID}

	type ev struct {
		at time.Time
		e  LedgerEntry
	}
	var evs []ev
	at := func(m time.Month, d, h, min int) time.Time { return time.Date(2026, m, d, h, min, 0, 0, la) }
	for _, u := range []int64{u1.ID, u2.ID} {
		evs = append(evs, ev{at(10, 25, 23, 0), LedgerEntry{UserID: u, Period: "2026-10", Type: "allowance", AmountMT: 300000}})
		evs = append(evs, ev{at(11, 1, 0, 0), LedgerEntry{UserID: u, Period: "2026-10", Type: "expiry", AmountMT: -1000}})
		evs = append(evs, ev{at(11, 1, 0, 0), LedgerEntry{UserID: u, Period: "2026-11", Type: "allowance", AmountMT: 300000}})
	}
	grantAt := at(11, 5, 11, 30)
	evs = append(evs, ev{grantAt, LedgerEntry{UserID: u1.ID, Period: "2026-11", Type: "grant", AmountMT: 5000, ActorID: &admin.ID, Reason: "bonus"}})
	evs = append(evs, ev{at(11, 15, 9, 0), LedgerEntry{UserID: u2.ID, Period: "2026-11", Type: "reset", AmountMT: 2000, ActorID: &admin.ID, Reason: "reset"}})
	mult := map[time.Time]float64{}
	for d := at(10, 26, 0, 0); d.Before(at(11, 20, 0, 0)); d = d.AddDate(0, 0, 1) {
		for _, h := range []int64{h1, h2, h3} {
			if h == h2 && d.Day()%2 == 1 {
				continue
			}
			polls := 8
			if h == h3 && d.Day() == 3 {
				polls = 1 // a one-row group
			}
			for i := 0; i < polls; i++ {
				when := d.Add(time.Duration(1+3*i) * time.Hour)
				hh := h
				period := when.In(la).Format("2006-01")
				evs = append(evs, ev{when, LedgerEntry{UserID: holdUser[h], Period: period, Type: "debit",
					AmountMT: -int64(100 + (d.Day()*7+i*3+int(h))%50), HoldID: &hh}})
			}
		}
	}
	sort.SliceStable(evs, func(i, j int) bool { return evs[i].at.Before(evs[j].at) })
	for _, x := range evs {
		e := must(s.AppendLedger(ctx, x.e, "2026-11", x.at))
		if e.Type == "debit" {
			key := x.at.UTC()
			mult[key] = 1 + float64(e.ID%3)
			must(s.InsertUsageSample(ctx, UsageSample{HoldID: *e.HoldID, PolledAt: x.at, Weight: 1, Multiplier: mult[key], DebitMT: -e.AmountMT, Reason: "r"}))
		}
	}

	users, holds := []int64{u1.ID, u2.ID}, []int64{h1, h2, h3}
	periods := []string{"2026-09", "2026-10", "2026-11"}
	var days []time.Time
	for d := at(10, 24, 0, 0); !d.After(at(11, 21, 0, 0)); d = d.AddDate(0, 0, 1) {
		days = append(days, d)
	}
	now := at(11, 20, 10, 0)
	before := allLedger(t, s)
	groups := modelGroups(before, LedgerCompactionCutoff(now, la), la)
	if len(groups) < 40 {
		t.Fatalf("only %d groups in the fixture", len(groups))
	}
	sn0 := takeSnapshot(t, s, users, holds, admin.ID, periods, days, la)
	if sn0.RebuildChange != 0 {
		t.Fatalf("fixture cache not consistent: %d", sn0.RebuildChange)
	}

	removed := 0
	for _, g := range groups {
		removed += len(g)
	}
	if n := compactAll(t, s, now, la); n != removed {
		t.Fatalf("replaced %d rows, model %d", n, removed)
	}
	after := allLedger(t, s)
	want := expectMerge(before, groups)
	diffLedger(t, after, want)
	sn1 := takeSnapshot(t, s, users, holds, admin.ID, periods, days, la)

	same := func(name string, a, b any) {
		t.Helper()
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s changed:\nbefore %v\nafter  %v", name, a, b)
		}
	}
	same("users.balance_mt", sn0.Balances, sn1.Balances)
	same("SumLedger", sn0.Sums, sn1.Sums)
	same("RebuildBalance", sn0.Rebuilt, sn1.Rebuilt)
	same("RebuildBalances changes", sn0.RebuildChange, sn1.RebuildChange)
	same("HoldCharges", sn0.HoldCharges, sn1.HoldCharges)
	same("LedgerPeriods", sn0.Periods, sn1.Periods)
	same("DebitEntries per local day", sn0.DayDebits, sn1.DayDebits)
	same("LatestAllowancePeriod", sn0.LatestAllow, sn1.LatestAllow)
	same("LatestLedgerEntry", sn0.Latest, sn1.Latest)
	same("LedgerStats", sn0.Stats, sn1.Stats)
	same("CountOverdrawn", sn0.Overdrawn, sn1.Overdrawn)
	same("LedgerPeriodTotals", sn0.PeriodTotals, sn1.PeriodTotals)
	same("totals per user/period/type", sn0.TypeTotals, sn1.TypeTotals)
	same("totals per hold", sn0.HoldTotals, sn1.HoldTotals)

	// Readers that differ only by the merged rows.
	inGroup := map[int64]bool{}
	removedBy := map[string]int{} // user/period -> rows removed
	byID := map[int64]LedgerEntry{}
	for _, e := range before {
		byID[e.ID] = e
	}
	for _, g := range groups {
		for _, id := range g {
			inGroup[id] = true
		}
		e := byID[g[0]]
		removedBy[fmt.Sprintf("%d/%s", e.UserID, e.Period)] += len(g) - 1
	}
	for k, n := range sn0.Counts {
		w := n
		if strings.HasSuffix(k, "/debit") {
			w -= removedBy[strings.TrimSuffix(k, "/debit")]
		}
		if sn1.Counts[k] != w {
			t.Errorf("CountLedger %s: %d, want %d (before %d)", k, sn1.Counts[k], w, n)
		}
	}
	for k, list := range sn1.List {
		var w []LedgerEntry
		for _, e := range want {
			if fmt.Sprintf("%d/%s", e.UserID, e.Period) == k {
				w = append(w, e)
			}
		}
		if !reflect.DeepEqual(list, w) {
			t.Errorf("ListLedger %s differs from the merged list", k)
		}
		// BalanceThrough is the running sum of the merged list.
		var run int64
		for _, e := range list {
			run += e.AmountMT
			if sn1.Through[e.ID] != run {
				t.Errorf("BalanceThrough %d = %d, running sum %d", e.ID, sn1.Through[e.ID], run)
			}
		}
		m0, m1 := sn0.Multipliers[k], sn1.Multipliers[k]
		wm := map[int64]float64{}
		for id, v := range m0 {
			if !inGroup[id] {
				wm[id] = v
			}
		}
		if len(m1) == 0 && len(wm) == 0 {
			continue
		}
		if !reflect.DeepEqual(m1, wm) {
			t.Errorf("DebitMultipliers %s: %d entries, want %d", k, len(m1), len(wm))
		}
	}
	for u, list := range sn1.DebitEntries {
		var w []DebitEntry
		var ws []LedgerEntry
		for _, e := range want {
			if e.UserID == u && e.Type == "debit" && !e.CreatedAt.Before(days[0]) && e.CreatedAt.Before(days[len(days)-1]) {
				ws = append(ws, e)
			}
		}
		sort.SliceStable(ws, func(i, j int) bool {
			if !ws[i].CreatedAt.Equal(ws[j].CreatedAt) {
				return ws[i].CreatedAt.Before(ws[j].CreatedAt)
			}
			return ws[i].ID < ws[j].ID
		})
		for _, e := range ws {
			w = append(w, DebitEntry{At: e.CreatedAt, MT: -e.AmountMT})
		}
		if !reflect.DeepEqual(list, w) {
			t.Errorf("DebitEntries user %d: %d entries, want %d (before %d)", u, len(list), len(w), len(sn0.DebitEntries[u]))
		}
	}
	// BalanceThrough unchanged at every kept row and at the last entry of
	// every local day (per user and period); at the mid-day grant it lacks
	// exactly that day's earlier debits of u1.
	lastOfDay := map[string]int64{}
	for _, e := range before {
		lastOfDay[fmt.Sprintf("%d/%s/%s", e.UserID, e.Period, e.CreatedAt.In(la).Format("01-02"))] = e.ID
	}
	for _, id := range lastOfDay {
		if sn0.Through[id] != sn1.Through[id] {
			t.Errorf("BalanceThrough at last entry %d of its day: %d -> %d", id, sn0.Through[id], sn1.Through[id])
		}
	}
	var grant LedgerEntry
	for _, e := range after {
		if _, ok := sn1.Through[e.ID]; ok && !inGroup[e.ID] && sn0.Through[e.ID] != sn1.Through[e.ID] && e.Type != "grant" {
			t.Errorf("BalanceThrough of kept %s row %d changed: %d -> %d", e.Type, e.ID, sn0.Through[e.ID], sn1.Through[e.ID])
		}
		if e.Type == "grant" {
			grant = e
		}
	}
	var earlier int64
	for _, e := range before {
		if e.UserID == u1.ID && e.Type == "debit" && e.ID < grant.ID && inGroup[e.ID] &&
			e.CreatedAt.In(la).Format("01-02") == grantAt.Format("01-02") {
			earlier += e.AmountMT
		}
	}
	if earlier == 0 || sn0.Through[grant.ID]-sn1.Through[grant.ID] != earlier {
		t.Errorf("grant balance %d -> %d, earlier same-day debits %d", sn0.Through[grant.ID], sn1.Through[grant.ID], earlier)
	}
	// Totals per user and local day, from the stored rows.
	dayTotals := func(rows []LedgerEntry) map[string]int64 {
		out := map[string]int64{}
		for _, e := range rows {
			out[fmt.Sprintf("%d/%s/%s", e.UserID, e.Type, e.CreatedAt.In(la).Format("01-02"))] += e.AmountMT
		}
		return out
	}
	same("totals per user, type and local day", dayTotals(before), dayTotals(after))

	// The cache keeps working: the next append of each user agrees with the
	// ledger sum.
	for _, u := range users {
		must(s.AppendLedger(ctx, LedgerEntry{UserID: u, Period: "2026-11", Type: "grant", AmountMT: 1, ActorID: &admin.ID}, "2026-11", now))
		if c, sum := must(s.GetUser(ctx, u)).BalanceMT, must(s.SumLedger(ctx, u, "2026-11", "")); c != sum {
			t.Errorf("user %d after append: cache %d, ledger %d", u, c, sum)
		}
	}
}

// UPG-1: a database built by migrations 0001-0005 (the current release's
// schema) holding ledger data is opened by the new store: it migrates, every
// balance cache, period sum and per-hold charge is the same, every row reads
// back as raw, and they are still the same after compaction.
func TestUPG1_Schema5LedgerUpgradesAndCompacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.db")
	s := openAtVersion(t, path, 5)
	exec := func(q string, args ...any) int64 {
		t.Helper()
		r, err := s.sqldb.Exec(q, args...)
		if err != nil {
			t.Fatalf("%.60s: %v", q, err)
		}
		return must(r.LastInsertId())
	}
	var users, holds []int64
	for i := 0; i < 2; i++ {
		users = append(users, exec(`INSERT INTO users (auth_source, oidc_sub, email, name, status, created_at) VALUES ('oidc', ?, ?, 'U', 'active', ?)`,
			fmt.Sprintf("sub%d", i), fmt.Sprintf("u%d@example.test", i), FormatTime(t0)))
	}
	for i := 0; i < 3; i++ {
		sid := exec(`INSERT INTO setups (name, details_json, enabled) VALUES (?, '{}', 1)`, fmt.Sprintf("lab-%d", i))
		holds = append(holds, exec(`INSERT INTO holds (setup_id, user_id, acquired_at) VALUES (?, ?, ?)`, sid, users[i%2], FormatTime(t0)))
	}
	ins := func(u int64, period, typ string, mt int64, hold any, at time.Time) {
		exec(`INSERT INTO ledger (user_id, period, type, amount_mt, hold_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`, u, period, typ, mt, hold, FormatTime(at))
	}
	start := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, u := range users {
		ins(u, "2026-10", "allowance", 300000, nil, start)
		ins(u, "2026-11", "allowance", 300000, nil, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	}
	for at := start; at.Before(time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)); at = at.Add(time.Hour) {
		for i, h := range holds {
			ins(users[i%2], at.Format("2006-01"), "debit", -int64(50+i+at.Hour()), h, at)
		}
		if at.Hour() == 12 && at.Day() == 3 {
			ins(users[0], "2026-11", "grant", 777, nil, at)
		}
	}
	for _, u := range users {
		must(s.RebuildBalance(ctx, u, "2026-11"))
	}
	type state struct {
		cache, sums map[string]int64
		charges     map[int64]int64
	}
	read := func(s *Store) state {
		st := state{map[string]int64{}, map[string]int64{}, must(s.HoldCharges(ctx, holds))}
		for _, u := range users {
			var c int64
			s.sqldb.QueryRow(`SELECT balance_mt FROM users WHERE id = ?`, u).Scan(&c)
			st.cache[fmt.Sprint(u)] = c
			for _, p := range []string{"2026-10", "2026-11"} {
				st.sums[fmt.Sprintf("%d/%s", u, p)] = must(s.SumLedger(ctx, u, p, ""))
			}
		}
		return st
	}
	old := read(s)
	var rows int
	s.sqldb.QueryRow(`SELECT COUNT(*) FROM ledger`).Scan(&rows)
	s.Close()

	s = must(Open(path))
	defer s.Close()
	var v int
	var ic string
	s.sqldb.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&v)
	s.sqldb.QueryRow(`PRAGMA integrity_check`).Scan(&ic)
	if v != 8 || ic != "ok" {
		t.Fatalf("after upgrade: version %d integrity %q", v, ic)
	}
	if got := read(s); !reflect.DeepEqual(got, old) {
		t.Fatalf("after upgrade:\n%+v\nwas\n%+v", got, old)
	}
	all := allLedger(t, s)
	if len(all) != rows {
		t.Fatalf("rows %d, was %d", len(all), rows)
	}
	for _, e := range all {
		if e.RollupCount != 0 {
			t.Fatalf("row %d not raw after upgrade: %+v", e.ID, e)
		}
	}
	if _, err := s.sqldb.Exec(`DELETE FROM ledger WHERE type = 'debit'`); err == nil {
		t.Fatal("delete allowed after upgrade")
	}
	if rep := must(s.RebuildBalances(ctx, "2026-11")); rep.Changed != 0 {
		t.Errorf("caches needed repair after upgrade: %+v", rep)
	}

	n := compactAll(t, s, time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC), nil)
	if n == 0 {
		t.Fatal("nothing compacted")
	}
	if got := read(s); !reflect.DeepEqual(got, old) {
		t.Fatalf("after compaction:\n%+v\nwas\n%+v", got, old)
	}
	if rep := must(s.RebuildBalances(ctx, "2026-11")); rep.Changed != 0 {
		t.Errorf("caches needed repair after compaction: %+v", rep)
	}
	if after := allLedger(t, s); len(after) >= rows {
		t.Errorf("rows %d after compaction, %d before", len(after), rows)
	}
}

// LED-10 (A1): one search step looks at a bounded number of local days, however
// much one-row-group history lies in front of the first group to compact, so a
// step's cost does not grow with history. The search resumes where it stopped
// and still finds the group.
func TestLED10_DueSearchStepIsBoundedByDays(t *testing.T) {
	s := openTemp(t)
	u := newUser(t, s, "u")
	h := must(s.CreateHold(ctx, newSetup(t, s, "a"), u.ID, t0)).ID
	first := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	const oneRowDays = 200
	for d := 0; d < oneRowDays; d++ { // one raw row a day: never compacted
		must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-01", Type: "debit", AmountMT: -1, HoldID: &h},
			"2026-01", first.AddDate(0, 0, d).Add(time.Hour)))
	}
	for k := 0; k < 3; k++ { // then a day with a group to compact
		must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-01", Type: "debit", AmountMT: -1, HoldID: &h},
			"2026-01", first.AddDate(0, 0, oneRowDays).Add(time.Duration(k+1)*time.Hour)))
	}
	now := first.AddDate(0, 0, oneRowDays+30)
	cutoff := LedgerCompactionCutoff(now, nil)
	var from time.Time
	found, calls := 0, 0
	for from.Before(cutoff) {
		if calls++; calls > 20 {
			t.Fatal("the search does not advance")
		}
		d := must(s.DueLedgerGroups(ctx, now, nil, from, LedgerCompactionStepRows))
		if from.IsZero() {
			from = first
		}
		if walked := d.Next.Sub(from); walked > LedgerCompactionStepDays*24*time.Hour {
			t.Fatalf("call %d walked %s (%d days), over the bound of %d days", calls, walked, int(walked.Hours()/24), LedgerCompactionStepDays)
		}
		found += len(d.Groups)
		from = d.Next
	}
	if found != 1 {
		t.Fatalf("found %d groups, want 1", found)
	}
	if want := (oneRowDays + LedgerCompactionStepDays) / LedgerCompactionStepDays; calls < want {
		t.Fatalf("%d calls, want at least %d: the first call did not stop at the day bound", calls, want)
	}
	if n := compactAll(t, s, now, nil); n != 3 {
		t.Fatalf("compacted %d rows, want 3", n)
	}
}

// LED-7: a spring-forward day has 23 hours and is one group; its neighbours
// are their own groups, whichever side of the clock change a row falls on.
func TestLED7_SpringForwardDayIsOneGroup(t *testing.T) {
	la := losAngeles(t)
	s := openTemp(t)
	u := newUser(t, s, "u")
	h := must(s.CreateHold(ctx, newSetup(t, s, "a"), u.ID, t0)).ID
	at := func(d, hh, mm int) time.Time { return time.Date(2026, time.March, d, hh, mm, 0, 0, la) }
	// March 8, 2026: 02:00 PST became 03:00 PDT.
	whens := []time.Time{at(7, 23, 30), at(7, 23, 50), at(8, 0, 30), at(8, 1, 30), at(8, 3, 30), at(8, 23, 30), at(9, 0, 10), at(9, 0, 20)}
	var rows []LedgerEntry
	for i, w := range whens {
		rows = append(rows, must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-03", Type: "debit", AmountMT: -int64(i + 1), HoldID: &h}, "2026-03", w)))
	}
	if got := at(9, 0, 0).Sub(at(8, 0, 0)); got != 23*time.Hour {
		t.Fatalf("March 8 is %s long, the fixture is wrong", got)
	}
	now := at(25, 12, 0)
	want := [][]int64{{rows[0].ID, rows[1].ID}, {rows[2].ID, rows[3].ID, rows[4].ID, rows[5].ID}, {rows[6].ID, rows[7].ID}}
	if got := modelGroups(allLedger(t, s), LedgerCompactionCutoff(now, la), la); !reflect.DeepEqual(got, want) {
		t.Fatalf("model groups %v, want %v", got, want)
	}
	before := allLedger(t, s)
	if n := compactAll(t, s, now, la); n != len(whens) {
		t.Fatalf("replaced %d rows, want %d", n, len(whens))
	}
	diffLedger(t, allLedger(t, s), expectMerge(before, want))
}

// LED-7: the day boundary is the one in force when a step runs. After a
// timezone change a row already rolled up stays as it is; the new zone only
// decides about rows that are still raw.
func TestLED7_TimezoneChangeLeavesRolledUpRowsAlone(t *testing.T) {
	la := losAngeles(t)
	s := openTemp(t)
	u := newUser(t, s, "u")
	h := must(s.CreateHold(ctx, newSetup(t, s, "a"), u.ID, t0)).ID
	// 23:30 and 00:30 in Los Angeles are one UTC day (06:30 and 07:30 on the
	// 11th) and two Los Angeles days.
	a := time.Date(2026, time.March, 10, 23, 30, 0, 0, la)
	b := time.Date(2026, time.March, 11, 0, 30, 0, 0, la)
	for i, w := range []time.Time{a, b, a.Add(-2 * time.Hour), b.Add(2 * time.Hour)} { // each LA day has two rows
		must(s.AppendLedger(ctx, LedgerEntry{UserID: u.ID, Period: "2026-03", Type: "debit", AmountMT: -int64(i + 1), HoldID: &h}, "2026-03", w))
	}
	now := time.Date(2026, time.March, 25, 12, 0, 0, 0, time.UTC)
	sum := func() int64 { return must(s.SumLedger(ctx, u.ID, "2026-03", "")) }
	total := sum()
	if n := compactAll(t, s, now, la); n != 4 { // two groups of two in Los Angeles
		t.Fatalf("Los Angeles replaced %d rows, want 4", n)
	}
	if n := compactAll(t, s, now, time.UTC); n != 0 { // all rolled up: final
		t.Fatalf("UTC pass replaced %d rows after a Los Angeles pass, want 0", n)
	}
	if got := sum(); got != total {
		t.Fatalf("sum %d -> %d", total, got)
	}
	if n := len(allLedger(t, s)); n != 2 {
		t.Fatalf("%d rows, want the 2 roll-ups", n)
	}
}
