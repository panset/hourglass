// SPDX-License-Identifier: MIT

package ledger

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

// LED-10 at the service: CompactDue is one bounded step in one transaction,
// resumes where the previous step stopped, changes nothing when nothing is
// due, and a failed step changes nothing and is redone by the next call.

// compactEnv is a ledger service over a database whose path the test knows,
// so a second connection can inspect the schema and inject failures.
type compactEnv struct {
	*env
	path  string
	raw   *sql.DB
	users []int64
	holds []int64 // holds[i] belongs to users[i%len(users)]
}

// Nov 20, 12:00 UTC: the cutoff is the start of Nov 13, so Nov 1..12 are due.
var compactNow = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func newCompactEnv(t *testing.T, nUsers, nHolds int) *compactEnv {
	t.Helper()
	path := filepath.Join(t.TempDir(), "h.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	clk := hourglasstest.NewFakeClock(compactNow)
	r := &rec{}
	e := &compactEnv{env: &env{t, st, clk, r, NewService(st, r, clk, time.UTC)}, path: path}
	// The store registered the driver; this connection only reads, except
	// where a test injects a failure.
	if e.raw, err = sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(10000)"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.raw.Close() })
	for i := 0; i < nUsers; i++ {
		e.users = append(e.users, e.user(fmt.Sprintf("u%d", i), store.RoleDeveloper))
	}
	for i := 0; i < nHolds; i++ {
		sid, err := st.CreateSetup(bg, store.Setup{Name: fmt.Sprintf("lab-%02d", i), Enabled: true,
			Labels: map[string]string{}, DetailsJSON: `{}`})
		if err != nil {
			t.Fatal(err)
		}
		h, err := st.CreateHold(bg, sid, e.users[i%nUsers], compactNow.AddDate(0, -1, 0))
		if err != nil {
			t.Fatal(err)
		}
		e.holds = append(e.holds, h.ID)
	}
	return e
}

// debits writes n raw debits of 7 mt for hold i on the given day, 5 minutes
// apart from 00:05, as the collector would.
func (e *compactEnv) debits(i int, day time.Time, n int) {
	e.t.Helper()
	hid, uid := e.holds[i], e.users[i%len(e.users)]
	err := e.st.Tx(bg, func(tx *store.Tx) error {
		for k := 1; k <= n; k++ {
			at := day.Add(time.Duration(k) * 5 * time.Minute)
			if _, err := tx.AppendLedger(bg, store.LedgerEntry{UserID: uid, Period: PeriodOf(at, time.UTC), Type: "debit",
				AmountMT: -7, HoldID: &hid}, e.svc.CurrentPeriod(), at); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		e.t.Fatal(err)
	}
}

// snapshot is every user's ledger of November and their cached balances.
func (e *compactEnv) snapshot() (rows [][]store.LedgerEntry, caches []int64) {
	e.t.Helper()
	for _, u := range e.users {
		l, err := e.st.ListLedger(bg, u, "2026-11")
		if err != nil {
			e.t.Fatal(err)
		}
		rows = append(rows, l)
		caches = append(caches, e.cache(u))
	}
	return rows, caches
}

func (e *compactEnv) count(q string) int {
	e.t.Helper()
	var n int
	if err := e.raw.QueryRow(q).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *compactEnv) dueGroups() int {
	e.t.Helper()
	d, err := e.st.DueLedgerGroups(bg, e.clk.Now(), time.UTC, time.Time{}, 1<<30)
	if err != nil {
		e.t.Fatal(err)
	}
	return len(d.Groups)
}

func (e *compactEnv) step() CompactionStep {
	e.t.Helper()
	s, err := e.svc.CompactDue(bg)
	if err != nil {
		e.t.Fatal(err)
	}
	return s
}

func day(d int) time.Time { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC) }

// TestLED10_CompactDueStepsAreBounded: every step takes whole groups up to
// the bound (a larger group alone), More is true exactly until the backlog is
// gone, and the steps together compact every due group without changing a sum
// or a cached balance.
func TestLED10_CompactDueStepsAreBounded(t *testing.T) {
	e := newCompactEnv(t, 2, 4)
	const bound = 120
	e.svc.compactRows = bound
	for d := 1; d <= 6; d++ {
		for i := range e.holds {
			e.debits(i, day(d), 50)
		}
	}
	e.debits(0, day(7), 200) // one group larger than the bound
	e.debits(1, day(18), 30) // recent: never due
	rowsBefore := e.count(`SELECT COUNT(*) FROM ledger`)
	_, cachesBefore := e.snapshot()
	sums := map[int64]int64{}
	for _, u := range e.users {
		sums[u] = e.sum(u, "2026-11")
	}
	var steps []CompactionStep
	for len(steps) < 100 {
		s := e.step()
		steps = append(steps, s)
		if s.Replaced > bound && s.Groups != 1 {
			t.Errorf("step %d: %d rows in %d groups, over the bound %d", len(steps), s.Replaced, s.Groups, bound)
		}
		if s.Removed != s.Replaced-s.Groups {
			t.Errorf("step %d: removed %d, replaced %d in %d groups", len(steps), s.Removed, s.Replaced, s.Groups)
		}
		if !s.More {
			break
		}
		if s.Groups == 0 {
			t.Errorf("step %d did nothing but reports more", len(steps))
		}
	}
	// 24 groups of 50, two per step; the group of 200 alone; then a last
	// step that finds nothing more.
	if len(steps) != 14 || steps[12].Groups != 1 || steps[12].Replaced != 200 || steps[13] != (CompactionStep{}) {
		t.Fatalf("steps: %+v", steps)
	}
	removed := 0
	for _, s := range steps {
		removed += s.Removed
	}
	if got := e.count(`SELECT COUNT(*) FROM ledger`); removed != 24*49+199 || got != rowsBefore-removed {
		t.Fatalf("removed %d, ledger %d -> %d", removed, rowsBefore, got)
	}
	if n := e.dueGroups(); n != 0 {
		t.Fatalf("%d due groups left", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM ledger WHERE rollup_count IS NULL AND created_at >= '2026-11-18'`); n != 30 {
		t.Fatalf("recent rows changed: %d raw", n)
	}
	if _, caches := e.snapshot(); !reflect.DeepEqual(caches, cachesBefore) {
		t.Fatalf("balance caches %v -> %v", cachesBefore, caches)
	}
	for _, u := range e.users {
		if got := e.sum(u, "2026-11"); got != sums[u] {
			t.Fatalf("user %d sum %d -> %d", u, sums[u], got)
		}
	}
}

// TestLED10_CompactDueAgainWhenNothingDueChangesNothing: once the backlog is
// gone, and on a ledger with nothing due at all, further steps report nothing
// and leave every row and cache as it was.
func TestLED10_CompactDueAgainWhenNothingDueChangesNothing(t *testing.T) {
	e := newCompactEnv(t, 1, 2)
	e.debits(0, day(18), 40) // recent only
	e.debits(1, day(19), 40)
	rows, caches := e.snapshot()
	for i := 0; i < 3; i++ {
		if s := e.step(); s != (CompactionStep{}) {
			t.Fatalf("nothing due, step %d: %+v", i, s)
		}
	}
	if r, c := e.snapshot(); !reflect.DeepEqual(r, rows) || !reflect.DeepEqual(c, caches) {
		t.Fatal("a step with nothing due changed the ledger")
	}
	e.debits(0, day(3), 40)
	e.debits(1, day(4), 1) // a one-row group stays as it is
	for s := e.step(); s.More; s = e.step() {
	}
	rows, caches = e.snapshot()
	for i := 0; i < 3; i++ {
		if s := e.step(); s != (CompactionStep{}) {
			t.Fatalf("after the backlog, step %d: %+v", i, s)
		}
	}
	if r, c := e.snapshot(); !reflect.DeepEqual(r, rows) || !reflect.DeepEqual(c, caches) {
		t.Fatal("a second run changed the ledger")
	}
	if n := e.count(`SELECT COUNT(*) FROM ledger WHERE rollup_count IS NOT NULL`); n != 1 {
		t.Fatalf("%d rolled-up rows, want 1", n)
	}
}

// TestLED10_CompactDueFailedStepChangesNothingAndIsRetried: a step that fails
// half way (after deleting a group's rows) leaves the ledger, the caches and
// the marker as they were, and the next call redoes the same groups from the
// same position; the backlog then finishes as if nothing had failed.
func TestLED10_CompactDueFailedStepChangesNothingAndIsRetried(t *testing.T) {
	e := newCompactEnv(t, 1, 2)
	e.svc.compactRows = 100
	for d := 1; d <= 3; d++ {
		e.debits(0, day(d), 60)
		e.debits(1, day(d), 60)
	}
	if s := e.step(); s.Groups != 1 || !s.More { // day 1, hold 0
		t.Fatalf("first step: %+v", s)
	}
	rows, caches := e.snapshot()
	if _, err := e.raw.Exec(`CREATE TRIGGER test_fail_rollup BEFORE INSERT ON ledger
		WHEN NEW.rollup_count IS NOT NULL BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.CompactDue(bg); err == nil {
		t.Fatal("step succeeded with the injected failure")
	}
	if r, c := e.snapshot(); !reflect.DeepEqual(r, rows) || !reflect.DeepEqual(c, caches) {
		t.Fatal("a failed step changed the ledger")
	}
	if n := e.count(`SELECT COUNT(*) FROM ledger_compaction`); n != 0 {
		t.Fatalf("marker left behind: %d", n)
	}
	if _, err := e.raw.Exec(`DROP TRIGGER test_fail_rollup`); err != nil {
		t.Fatal(err)
	}
	// The retry takes day 1, hold 1: the position did not move past it.
	s := e.step()
	if s.Groups != 1 || s.Replaced != 60 {
		t.Fatalf("retry: %+v", s)
	}
	var at string
	if err := e.raw.QueryRow(`SELECT created_at FROM ledger WHERE rollup_count IS NOT NULL ORDER BY id DESC LIMIT 1`).Scan(&at); err != nil {
		t.Fatal(err)
	}
	if at[:10] != "2026-11-01" {
		t.Fatalf("retry compacted %s, not the failed group of Nov 1", at)
	}
	for s.More {
		s = e.step()
	}
	if n := e.dueGroups(); n != 0 {
		t.Fatalf("%d due groups left", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM ledger WHERE type = 'debit'`); n != 6 {
		t.Fatalf("%d debit rows, want 6", n)
	}
}
