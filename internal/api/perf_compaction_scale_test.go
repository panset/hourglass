// SPDX-License-Identifier: MIT

package api

// LED-10 and design A1 ("Tests"): a year of 5-minute debits for several holds
// compacts in bounded steps, each well under one second; finding the due
// groups is served by the partial index over raw debits, not a scan of the
// ledger; and a ledger page read (GET /api/v1/me/ledger as it is today) stays
// fast before and after.
//
//   - Default run (CI): a year for 3 holds (about 315,000 debits).
//   - Full size: HOURGLASS_SCALE_TEST=1 runs a year for 12 holds (about 1.26
//     million debits; seeding and compacting take a minute or two) and logs
//     the same figures. See CONTRIBUTING.md.
//
// The debits are written with plain INSERTs, as a poll writes them (raw
// debit rows of one user, hold and period at the poll time), and the balance
// caches are rebuilt afterwards: going through AppendLedger row by row would
// multiply the seeding time and test nothing more here.

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/collector"
	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/store"
)

type compactionShape struct{ holds, days int }

var (
	compactionReduced = compactionShape{holds: 3, days: 365}
	compactionFull    = compactionShape{holds: 12, days: 365}
)

// stepLimit is the bound every compaction step must stay under; the design
// asks for "well under one second".
const stepLimit = time.Second

// backendDBPath returns the database file of the backend the test created.
// newBackend puts it in its own t.TempDir(), and every TempDir of a test is a
// numbered directory under one root.
func backendDBPath(t *testing.T) string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(filepath.Dir(t.TempDir()), "*", "api.db"))
	if err != nil || len(m) != 1 {
		t.Fatalf("backend database: %v %v", m, err)
	}
	return m[0]
}

func TestLED10_ScaleYearOfDebitsCompactsInBoundedSteps(t *testing.T) {
	sh := compactionReduced
	if os.Getenv("HOURGLASS_SCALE_TEST") != "" {
		sh = compactionFull
	}
	ctx := context.Background()
	now := time.Date(2026, 12, 20, 12, 0, 0, 0, time.UTC)
	start := now.AddDate(0, 0, -sh.days).Truncate(24 * time.Hour)
	b := newBackend(t, backendOpts{start: now})
	raw, err := sql.Open("sqlite", "file:"+backendDBPath(t)+"?_pragma=busy_timeout(10000)")
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()

	// Two holds per user, as in the API scale test.
	var users []sess
	type hh struct{ id, user int64 }
	var holds []hh
	for i := 0; i < sh.holds; i++ {
		if i%2 == 0 {
			users = append(users, b.newUser(fmt.Sprintf("year-%02d", i/2), store.StatusActive, store.RoleDeveloper))
		}
		sid, err := b.st.CreateSetup(ctx, store.Setup{Name: fmt.Sprintf("lab-%02d", i), Enabled: true,
			Labels: map[string]string{"tier": "t1"}, DetailsJSON: `{}`})
		if err != nil {
			t.Fatal(err)
		}
		u := users[len(users)-1].user.ID
		h, err := b.st.CreateHold(ctx, sid, u, start)
		if err != nil {
			t.Fatal(err)
		}
		holds = append(holds, hh{h.ID, u})
	}
	t0 := time.Now()
	debits := 0
	for day := start; day.Before(now); day = day.Add(24 * time.Hour) {
		tx, err := raw.Begin()
		if err != nil {
			t.Fatal(err)
		}
		ins, err := tx.Prepare(`INSERT INTO ledger (user_id, period, type, amount_mt, hold_id, actor_id, reason, created_at)
			VALUES (?, ?, 'debit', -333, ?, NULL, '', ?)`)
		if err != nil {
			t.Fatal(err)
		}
		for p := day.Add(5 * time.Minute); !p.After(day.Add(24*time.Hour)) && p.Before(now); p = p.Add(5 * time.Minute) {
			for _, h := range holds {
				if _, err := ins.Exec(h.user, ledger.PeriodOf(p, time.UTC), h.id, store.FormatTime(p)); err != nil {
					t.Fatal(err)
				}
				debits++
			}
		}
		ins.Close()
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.led.RepairBalances(ctx); err != nil {
		t.Fatal(err)
	}
	t.Logf("seeded %d debits (%d holds, %d days) in %s", debits, sh.holds, sh.days, time.Since(t0).Round(time.Millisecond))

	// Finding due groups is served by the partial index over raw debits. The
	// queries are those of store.DueLedgerGroups; the same check with both
	// raw-debit indexes dropped (rolled back) must show a scan, so the
	// assertion cannot pass without the index.
	const rawDebit = `type = 'debit' AND rollup_count IS NULL AND hold_id IS NOT NULL`
	findQueries := []string{
		`SELECT MIN(created_at) FROM ledger WHERE ` + rawDebit + ` AND created_at >= ? AND created_at < ?`,
		`SELECT user_id, hold_id, period, COUNT(*), SUM(amount_mt), MAX(id) FROM ledger WHERE ` + rawDebit +
			` AND created_at >= ? AND created_at < ? GROUP BY user_id, hold_id, period HAVING COUNT(*) > 1 ORDER BY MAX(id)`,
	}
	from, to := store.FormatTime(start), store.FormatTime(store.LedgerCompactionCutoff(now, time.UTC))
	plan := func(q queryer, query string) string {
		rows, err := q.Query(`EXPLAIN QUERY PLAN `+query, from, to)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			out = append(out, detail)
		}
		return strings.Join(out, "; ")
	}
	usesIndex := func(p string) bool {
		return strings.Contains(p, "INDEX ledger_raw_debit_created") && !strings.Contains(p, "SCAN ledger;") && !strings.HasSuffix(p, "SCAN ledger")
	}
	for _, q := range findQueries {
		if p := plan(raw, q); !usesIndex(p) {
			t.Errorf("finding due groups does not use the raw-debit index: %s", p)
		} else {
			t.Logf("plan: %s", p)
		}
	}
	func() {
		tx, err := raw.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		for _, idx := range []string{"ledger_raw_debit_created", "ledger_raw_debit_hold"} {
			if _, err := tx.Exec(`DROP INDEX ` + idx); err != nil {
				t.Fatal(err)
			}
		}
		for _, q := range findQueries {
			if p := plan(tx, q); usesIndex(p) {
				t.Errorf("the plan check passes without the index: %s", p)
			}
		}
	}()

	// A ledger page read before compaction: the current period (partly due)
	// and a past one (all due), for the user with the most rows.
	dev := users[0]
	pages := []string{"/api/v1/me/ledger", "/api/v1/me/ledger?period=2026-06"}
	readPages := func(when string) []int {
		var sizes []int
		for _, path := range pages {
			var ds []time.Duration
			var rec int
			var n int
			for i := 0; i < 5; i++ {
				t1 := time.Now()
				r := b.serve("GET", path, &dev, false, "")
				ds = append(ds, time.Since(t1))
				rec, n = r.Code, strings.Count(r.Body.String(), `"type":`)
			}
			sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
			if rec != 200 {
				t.Fatalf("%s %s: status %d", when, path, rec)
			}
			if ds[len(ds)-1] > 500*time.Millisecond {
				t.Errorf("%s compaction: %s took %s (over 500 ms)", when, path, ds[len(ds)-1])
			}
			t.Logf("%-6s compaction: GET %-36s %6d entries  median %6.1f ms  worst %6.1f ms", when, path, n,
				ms(ds[len(ds)/2]), ms(ds[len(ds)-1]))
			sizes = append(sizes, n)
		}
		return sizes
	}
	before := readPages("before")
	balanceBefore := make([]int64, len(users))
	for i, u := range users {
		got, err := b.st.GetUser(ctx, u.user.ID)
		if err != nil {
			t.Fatal(err)
		}
		balanceBefore[i] = got.BalanceMT
	}

	// Compact the whole backlog step by step, timing each step.
	var steps []time.Duration
	removed := 0
	t0 = time.Now()
	for {
		t1 := time.Now()
		s, err := b.led.CompactDue(ctx)
		d := time.Since(t1)
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, d)
		removed += s.Removed
		if s.Replaced > store.LedgerCompactionStepRows && s.Groups != 1 {
			t.Errorf("step %d: %d rows in %d groups, over the bound", len(steps), s.Replaced, s.Groups)
		}
		if d > stepLimit {
			t.Errorf("step %d (%d rows) took %s", len(steps), s.Replaced, d)
		}
		if !s.More {
			break
		}
	}
	total := time.Since(t0)
	sorted := append([]time.Duration(nil), steps...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	t.Logf("compacted %d rows away in %d steps (%s): median step %.1f ms, p99 %.1f ms, worst %.1f ms; %d cycles at %d steps per cycle",
		removed, len(steps), total.Round(time.Millisecond), ms(sorted[len(sorted)/2]), ms(sorted[len(sorted)*99/100]),
		ms(sorted[len(sorted)-1]), (len(steps)+collector.CompactStepsPerCycle-1)/collector.CompactStepsPerCycle, collector.CompactStepsPerCycle)
	if removed == 0 {
		t.Fatal("nothing compacted")
	}

	// The idle check once the backlog is gone.
	t1 := time.Now()
	s, err := b.led.CompactDue(ctx)
	if err != nil || s != (ledger.CompactionStep{}) {
		t.Fatalf("idle check: %+v %v", s, err)
	}
	t.Logf("idle check over the compacted year: %.1f ms", ms(time.Since(t1)))
	if d := time.Since(t1); d > stepLimit {
		t.Errorf("idle check took %s", d)
	}

	after := readPages("after")
	if after[0] >= before[0] || after[1] >= before[1] {
		t.Errorf("pages did not shrink: before %v after %v", before, after)
	}
	for i, u := range users {
		got, err := b.st.GetUser(ctx, u.user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.BalanceMT != balanceBefore[i] {
			t.Errorf("user %d balance %d -> %d", u.user.ID, balanceBefore[i], got.BalanceMT)
		}
	}
}

// queryer is what the plan check needs from *sql.DB and *sql.Tx.
type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
}
