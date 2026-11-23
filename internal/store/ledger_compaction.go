// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Ledger roll-up in storage (design A1, A2; LED-7..9). A raw debit is a debit
// row with a hold and a null rollup_count. A roll-up group is every raw debit
// of one user, hold and period whose created_at falls in one local calendar
// day. A group is due when its day is earlier than today - 7 (local dates).
// internal/ledger.Service is the only caller: it finds due groups with
// DueLedgerGroups and compacts them with CompactLedgerGroup inside its own
// transaction, a bounded number of rows per transaction.

// LedgerCompactionStepRows is the suggested row bound of one compaction
// transaction: whole groups up to this many rows, a single larger group alone.
// At the default 5-minute poll a hold's day is 288 rows.
const LedgerCompactionStepRows = 2000

// rawDebit is the predicate of a raw debit. It matches the WHERE clause of the
// partial indexes of migration 0006 term for term, so queries using it can use
// those indexes.
const rawDebit = `type = 'debit' AND rollup_count IS NULL AND hold_id IS NOT NULL`

// LedgerGroup is one roll-up group as DueLedgerGroups found it. CompactLedgerGroup
// re-reads the group's rows from its key (UserID, HoldID, Period, DayStart,
// DayEnd); Rows, AmountMT and NewestID are what the find saw.
type LedgerGroup struct {
	UserID   int64
	HoldID   int64
	Period   string
	DayStart time.Time // start of the local day, inclusive
	DayEnd   time.Time // start of the next local day, exclusive
	Rows     int
	AmountMT int64
	NewestID int64
}

// DueLedgerGroups is one batch of due groups and where the next search starts.
type DueLedgerGroups struct {
	Groups []LedgerGroup
	// Next is the position to pass as from to the next call. It never moves
	// past a group that was not returned, so a caller that failed to compact
	// a batch can retry from the from it used; a caller that compacted the
	// batch continues from Next.
	Next time.Time
}

// LedgerCompactionCutoff is the start of the local date today - 7 in loc (nil
// means UTC), as an instant: a raw debit is due exactly when its created_at
// is before it (its local day is earlier than today - 7).
func LedgerCompactionCutoff(now time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	y, m, d := now.In(loc).Date()
	return time.Date(y, m, d-7, 0, 0, 0, 0, loc)
}

// localDay returns the local day [start, end) in loc that holds t.
func localDay(t time.Time, loc *time.Location) (time.Time, time.Time) {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc), time.Date(y, m, d+1, 0, 0, 0, 0, loc)
}

// DueLedgerGroups returns the next due roll-up groups of more than one row,
// oldest local day first, taking whole groups up to maxRows rows in total (a
// single larger group is returned alone). It searches raw debits with
// from <= created_at < LedgerCompactionCutoff(now, loc); pass the zero time
// to start from the beginning. Days whose raw debits are all one-row groups
// are skipped. An empty Groups means nothing is due at or after from.
//
// Each day costs two statements on the partial index ledger_raw_debit_created
// (the oldest raw debit at or after a position, then that day's groups), so
// the search never reads compacted history. Local days are cut in Go because
// SQLite cannot convert to a named zone; SQL gets the day's UTC bounds.
func (q *queries) DueLedgerGroups(ctx context.Context, now time.Time, loc *time.Location, from time.Time, maxRows int) (DueLedgerGroups, error) {
	if loc == nil {
		loc = time.UTC
	}
	if maxRows < 1 {
		maxRows = 1
	}
	cutoff := LedgerCompactionCutoff(now, loc)
	out := DueLedgerGroups{Next: from}
	total := 0
	for out.Next.Before(cutoff) {
		var first sql.NullString
		if err := q.db.QueryRowContext(ctx, `SELECT MIN(created_at) FROM ledger
			WHERE `+rawDebit+` AND created_at >= ? AND created_at < ?`,
			FormatTime(out.Next), FormatTime(cutoff)).Scan(&first); err != nil {
			return out, err
		}
		if !first.Valid {
			out.Next = cutoff
			break
		}
		at, err := ParseTime(first.String)
		if err != nil {
			return out, err
		}
		start, end := localDay(at, loc)
		groups, err := collect(ctx, q.db, func(r rowScanner) (LedgerGroup, error) {
			g := LedgerGroup{DayStart: start, DayEnd: end}
			return g, r.Scan(&g.UserID, &g.HoldID, &g.Period, &g.Rows, &g.AmountMT, &g.NewestID)
		}, `SELECT user_id, hold_id, period, COUNT(*), SUM(amount_mt), MAX(id) FROM ledger
			WHERE `+rawDebit+` AND created_at >= ? AND created_at < ?
			GROUP BY user_id, hold_id, period HAVING COUNT(*) > 1 ORDER BY MAX(id)`,
			FormatTime(start), FormatTime(end))
		if err != nil {
			return out, err
		}
		for _, g := range groups {
			if total > 0 && total+g.Rows > maxRows {
				out.Next = start // the rest of this day is still raw
				return out, nil
			}
			out.Groups = append(out.Groups, g)
			total += g.Rows
		}
		out.Next = end
		if total >= maxRows {
			break
		}
	}
	return out, nil
}

// CompactLedgerGroup replaces the raw debits of one group (its user, hold,
// period and local day [DayStart, DayEnd)) with one debit row: the id and
// created_at of the newest of them, their summed amount, no actor or reason,
// and rollup_count set to the number of rows replaced (LED-7). It returns that
// number, or 0 when the group now has fewer than two raw rows (nothing is
// changed). It never touches users.balance_mt: the sums are unchanged.
//
// It joins the caller's transaction when called on a Tx (the ledger service
// does), and runs in its own otherwise. The step is wrapped in a savepoint:
// it sets the compaction marker, deletes the group's rows, inserts the new
// row and clears the marker, and on any error rolls all of that back before
// returning, so neither a half-compacted group nor the marker can survive it
// even if the caller went on to commit (LED-9).
func (q *queries) CompactLedgerGroup(ctx context.Context, g LedgerGroup) (int, error) {
	var n int
	err := q.atomic(ctx, func(q *queries) error {
		if _, err := q.db.ExecContext(ctx, `SAVEPOINT ledger_compaction_step`); err != nil {
			return err
		}
		var err error
		if n, err = q.compactGroup(ctx, g); err != nil {
			_, _ = q.db.ExecContext(ctx, `ROLLBACK TO ledger_compaction_step`)
			_, _ = q.db.ExecContext(ctx, `RELEASE ledger_compaction_step`)
			n = 0
			return err
		}
		_, err = q.db.ExecContext(ctx, `RELEASE ledger_compaction_step`)
		return err
	})
	return n, err
}

func (q *queries) compactGroup(ctx context.Context, g LedgerGroup) (int, error) {
	const key = rawDebit + ` AND hold_id = ? AND user_id = ? AND period = ? AND created_at >= ? AND created_at < ?`
	args := []any{g.HoldID, g.UserID, g.Period, FormatTime(g.DayStart), FormatTime(g.DayEnd)}
	if _, err := q.db.ExecContext(ctx, `INSERT INTO ledger_compaction (one) VALUES (1)`); err != nil {
		return 0, err
	}
	var (
		count    int
		sum      int64
		newestID sql.NullInt64
	)
	if err := q.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(amount_mt), 0), MAX(id) FROM ledger
		INDEXED BY ledger_raw_debit_hold WHERE `+key, args...).Scan(&count, &sum, &newestID); err != nil {
		return 0, err
	}
	if count > 1 {
		var at string
		if err := q.db.QueryRowContext(ctx, `SELECT created_at FROM ledger WHERE id = ?`, newestID.Int64).Scan(&at); err != nil {
			return 0, err
		}
		res, err := q.db.ExecContext(ctx, `DELETE FROM ledger INDEXED BY ledger_raw_debit_hold WHERE `+key, args...)
		if err != nil {
			return 0, err
		}
		if del, _ := res.RowsAffected(); del != int64(count) {
			return 0, fmt.Errorf("store: compaction deleted %d rows, expected %d", del, count)
		}
		if _, err := q.db.ExecContext(ctx, `INSERT INTO ledger
			(id, user_id, period, type, amount_mt, hold_id, actor_id, reason, created_at, rollup_count)
			VALUES (?, ?, ?, 'debit', ?, ?, NULL, '', ?, ?)`,
			newestID.Int64, g.UserID, g.Period, sum, g.HoldID, at, count); err != nil {
			return 0, err
		}
	} else {
		count = 0
	}
	if _, err := q.db.ExecContext(ctx, `DELETE FROM ledger_compaction`); err != nil {
		return 0, err
	}
	return count, nil
}
