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

// AppendLedger appends an entry and, in the same transaction, sets
// users.balance_mt to the sum of the user's ledger entries for the entry's
// period (LED-2: the cache is the current period's sum, and the period of the
// newest entry is the current one). CreatedAt is set from now.
func (q *queries) AppendLedger(ctx context.Context, e LedgerEntry, now time.Time) (LedgerEntry, error) {
	var out LedgerEntry
	err := q.atomic(ctx, func(q *queries) error {
		res, err := q.db.ExecContext(ctx, `INSERT INTO ledger (user_id, period, type, amount_mt, hold_id, actor_id, reason, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			e.UserID, e.Period, e.Type, e.AmountMT, nullInt(e.HoldID), nullInt(e.ActorID), e.Reason, FormatTime(now))
		if err != nil {
			return mapErr(err)
		}
		id, _ := res.LastInsertId()
		if _, err := q.RebuildBalance(ctx, e.UserID, e.Period); err != nil {
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

// RebuildBalance recomputes users.balance_mt from the ledger for period
// (LED-2) and returns it.
func (q *queries) RebuildBalance(ctx context.Context, userID int64, period string) (int64, error) {
	var sum int64
	err := q.atomic(ctx, func(q *queries) error {
		var err error
		if sum, err = q.SumLedger(ctx, userID, period, ""); err != nil {
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
