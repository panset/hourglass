// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"time"
)

// LedgerEntry is a row of the append-only ledger. AmountMT is signed.
type LedgerEntry struct {
	ID        int64
	UserID    int64
	Period    string // YYYY-MM
	Type      string
	AmountMT  int64
	HoldID    *int64
	ActorID   *int64
	Reason    string
	CreatedAt time.Time
}

const ledgerCols = `id, user_id, period, type, amount_mt, hold_id, actor_id, reason, created_at`

func scanLedger(r rowScanner) (LedgerEntry, error) {
	var (
		e         LedgerEntry
		hold, act sql.NullInt64
		at        string
		err       error
	)
	if err = r.Scan(&e.ID, &e.UserID, &e.Period, &e.Type, &e.AmountMT, &hold, &act, &e.Reason, &at); err != nil {
		return e, err
	}
	e.HoldID, e.ActorID = nullInt64Ptr(hold), nullInt64Ptr(act)
	e.CreatedAt, err = ParseTime(at)
	return e, err
}

// AppendLedger appends an entry and, in the same transaction, keeps
// users.balance_mt equal to the sum of the user's ledger entries for
// currentPeriod (LED-2), whatever period the entry itself belongs to. The
// cache is maintained incrementally, so an append costs O(log n) however long
// the ledger is: an entry of the current period adds its amount, an entry of
// another period leaves the cache alone, and the first entry the user gets in
// the current period (the rollover) starts the cache from scratch, since the
// cache still held the previous period. currentPeriod (YYYY-MM, from the
// configured timezone) must not move backwards between appends; RebuildBalance
// is the from-scratch repair. CreatedAt is set from now.
func (q *queries) AppendLedger(ctx context.Context, e LedgerEntry, currentPeriod string, now time.Time) (LedgerEntry, error) {
	var out LedgerEntry
	err := q.atomic(ctx, func(q *queries) error {
		res, err := q.db.ExecContext(ctx, `INSERT INTO ledger (user_id, period, type, amount_mt, hold_id, actor_id, reason, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			e.UserID, e.Period, e.Type, e.AmountMT, nullInt(e.HoldID), nullInt(e.ActorID), e.Reason, FormatTime(now))
		if err != nil {
			return mapErr(err)
		}
		id, _ := res.LastInsertId()
		var earlier bool
		if err := q.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM ledger WHERE user_id = ? AND period = ? AND id < ?)`,
			e.UserID, currentPeriod, id).Scan(&earlier); err != nil {
			return err
		}
		var delta int64 // the entry's own contribution to the current period
		if e.Period == currentPeriod {
			delta = e.AmountMT
		}
		stmt := `UPDATE users SET balance_mt = balance_mt + ? WHERE id = ?`
		if !earlier {
			stmt = `UPDATE users SET balance_mt = ? WHERE id = ?`
		}
		if _, err := q.db.ExecContext(ctx, stmt, delta, e.UserID); err != nil {
			return err
		}
		out, err = scanLedger(q.db.QueryRowContext(ctx, `SELECT `+ledgerCols+` FROM ledger WHERE id = ?`, id))
		return err
	})
	return out, err
}

// ListLedger returns a user's entries for a period in append order.
func (q *queries) ListLedger(ctx context.Context, userID int64, period string) ([]LedgerEntry, error) {
	return collect(ctx, q.db, scanLedger, `SELECT `+ledgerCols+` FROM ledger
		WHERE user_id = ? AND period = ? ORDER BY id`, userID, period)
}

// SumLedger sums a user's entries for a period; a non-empty typ restricts to
// one entry type.
func (q *queries) SumLedger(ctx context.Context, userID int64, period, typ string) (int64, error) {
	var n sql.NullInt64
	query := `SELECT SUM(amount_mt) FROM ledger WHERE user_id = ? AND period = ?`
	args := []any{userID, period}
	if typ != "" {
		query += ` AND type = ?`
		args = append(args, typ)
	}
	err := q.db.QueryRowContext(ctx, query, args...).Scan(&n)
	return n.Int64, err
}

// RebuildBalance recomputes users.balance_mt from the ledger for
// currentPeriod (LED-2), and returns the sum. It
// is the from-scratch path; AppendLedger maintains the cache incrementally.
func (q *queries) RebuildBalance(ctx context.Context, userID int64, currentPeriod string) (int64, error) {
	var sum int64
	err := q.atomic(ctx, func(q *queries) error {
		var err error
		if sum, err = q.SumLedger(ctx, userID, currentPeriod, ""); err != nil {
			return err
		}
		res, err := q.db.ExecContext(ctx, `UPDATE users SET balance_mt = ? WHERE id = ?`, sum, userID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
	return sum, err
}
