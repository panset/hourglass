// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"time"
)

// Queries added for the admin API handlers (task 21c). They are read-only
// except where a name says otherwise; every write the admin API makes goes
// through the existing methods inside one Store.Tx.

// LedgerStat summarises one user's ledger: the balance and debits of a period
// and the number of entries ever written (0 means the user never had a role).
type LedgerStat struct {
	UserID    int64
	BalanceMT int64 // sum of every entry of the period
	DebitedMT int64 // debits of the period as a positive number
	Entries   int   // 1 when the user has any entry in any period (rows exist only then)
}

// LedgerStats returns one row per user that has any ledger entry, in one
// statement for the whole list: the balance from the users.balance_mt cache,
// the month-to-date debits and whether the user has any entry from covering
// indexes. A non-zero userID restricts the result to that user and a
// non-empty status to users with that status; nothing is computed for the
// users left out.
func (q *queries) LedgerStats(ctx context.Context, period string, userID int64, status string) ([]LedgerStat, error) {
	query := `SELECT u.id, u.balance_mt,
		COALESCE((SELECT -SUM(l.amount_mt) FROM ledger l WHERE l.user_id = u.id AND l.period = ?1 AND l.type = 'debit'), 0)
		FROM users u WHERE EXISTS (SELECT 1 FROM ledger l WHERE l.user_id = u.id)
		AND (?2 = 0 OR u.id = ?2) AND (?3 = '' OR u.status = ?3) ORDER BY u.id`
	return collect(ctx, q.db, func(r rowScanner) (LedgerStat, error) {
		var s LedgerStat
		s.Entries = 1
		return s, r.Scan(&s.UserID, &s.BalanceMT, &s.DebitedMT)
	}, query, period, userID, status)
}

// CountOverdrawn counts the users (not the local admin) that hold a role, have
// a ledger and a cached balance of zero or below: one aggregate over the
// balance cache, whatever the list a caller returns.
func (q *queries) CountOverdrawn(ctx context.Context) (int, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users u
		WHERE u.auth_source <> 'local'
		AND EXISTS (SELECT 1 FROM user_roles r WHERE r.user_id = u.id)
		AND EXISTS (SELECT 1 FROM ledger l WHERE l.user_id = u.id)
		AND u.balance_mt <= 0`).Scan(&n)
	return n, err
}

// PeriodTotals are the lab-wide ledger figures of one period.
type PeriodTotals struct {
	DebitedMT  int64 // all debits, positive
	GrantedMT  int64 // sum of grant entries (negative grants reduce it)
	GrantCount int   // number of grant entries
}

// LedgerPeriodTotals sums debits and grants of a period over every user.
func (q *queries) LedgerPeriodTotals(ctx context.Context, period string) (PeriodTotals, error) {
	var t PeriodTotals
	err := q.db.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(CASE WHEN type = 'debit' THEN -amount_mt ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN type = 'grant' THEN amount_mt ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN type = 'grant' THEN 1 ELSE 0 END), 0)
		FROM ledger WHERE period = ?`, period).Scan(&t.DebitedMT, &t.GrantedMT, &t.GrantCount)
	return t, err
}

// LatestLedgerEntry returns the newest entry of a type written for a user by
// an actor, or ErrNotFound.
func (q *queries) LatestLedgerEntry(ctx context.Context, userID int64, typ string, actorID int64) (LedgerEntry, error) {
	e, err := scanLedger(q.db.QueryRowContext(ctx, `SELECT `+ledgerCols+` FROM ledger
		WHERE user_id = ? AND type = ? AND actor_id = ? ORDER BY id DESC LIMIT 1`, userID, typ, actorID))
	return e, mapErr(err)
}

// BalanceThrough sums a user's entries of a period up to and including the
// entry with id entryID: the balance right after that entry.
func (q *queries) BalanceThrough(ctx context.Context, userID int64, period string, entryID int64) (int64, error) {
	var n sql.NullInt64
	err := q.db.QueryRowContext(ctx, `SELECT SUM(amount_mt) FROM ledger
		WHERE user_id = ? AND period = ? AND id <= ?`, userID, period, entryID).Scan(&n)
	return n.Int64, err
}

// AllUserRoles returns every user's roles (sorted), keyed by user id. Users
// without a role are absent.
func (q *queries) AllUserRoles(ctx context.Context) (map[int64][]string, error) {
	type row struct {
		id   int64
		role string
	}
	rows, err := collect(ctx, q.db, func(r rowScanner) (row, error) {
		var x row
		return x, r.Scan(&x.id, &x.role)
	}, `SELECT user_id, role FROM user_roles ORDER BY user_id, role`)
	if err != nil {
		return nil, err
	}
	out := map[int64][]string{}
	for _, x := range rows {
		out[x.id] = append(out[x.id], x.role)
	}
	return out, nil
}

// CountUsersWithRole counts the users that have role.
func (q *queries) CountUsersWithRole(ctx context.Context, role string) (int, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_roles WHERE role = ?`, role).Scan(&n)
	return n, err
}

// CountHoldsForSetup counts every hold, active or released, of a setup.
func (q *queries) CountHoldsForSetup(ctx context.Context, setupID int64) (int, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM holds WHERE setup_id = ?`, setupID).Scan(&n)
	return n, err
}

// LatestUsageReason returns the policy reason text of a hold's newest usage
// sample, or "" when it has none.
func (q *queries) LatestUsageReason(ctx context.Context, holdID int64) (string, error) {
	var s sql.NullString
	err := q.db.QueryRowContext(ctx, `SELECT reason FROM usage_samples WHERE hold_id = ?
		ORDER BY polled_at DESC LIMIT 1`, holdID).Scan(&s)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return s.String, err
}

// CountAudit counts the audit entries a filter matches (Limit and Offset of
// the filter are ignored).
func (q *queries) CountAudit(ctx context.Context, f AuditFilter) (int, error) {
	query := `SELECT COUNT(*) FROM audit_log WHERE 1 = 1`
	var args []any
	if f.ActorID != nil {
		query += ` AND actor_id = ?`
		args = append(args, *f.ActorID)
	}
	if f.Action != "" {
		query += ` AND action = ?`
		args = append(args, f.Action)
	}
	var n int
	err := q.db.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

// ListCollectorRuns returns the newest runs, newest first.
func (q *queries) ListCollectorRuns(ctx context.Context, limit int) ([]CollectorRun, error) {
	return collect(ctx, q.db, scanRun, `SELECT `+runCols+` FROM collector_runs ORDER BY id DESC LIMIT ?`, limit)
}

// LastFailedRun returns the newest run that finished with ok = 0, or ErrNotFound.
func (q *queries) LastFailedRun(ctx context.Context) (CollectorRun, error) {
	c, err := scanRun(q.db.QueryRowContext(ctx, `SELECT `+runCols+` FROM collector_runs
		WHERE ok = 0 ORDER BY id DESC LIMIT 1`))
	return c, mapErr(err)
}

// ListOutagesSince returns the outages that started at or after since,
// newest first.
func (q *queries) ListOutagesSince(ctx context.Context, since time.Time) ([]Outage, error) {
	return collect(ctx, q.db, func(r rowScanner) (Outage, error) {
		var o Outage
		var s, e string
		if err := r.Scan(&o.ID, &s, &e, &o.Reason); err != nil {
			return o, err
		}
		var err error
		if o.StartedAt, err = ParseTime(s); err != nil {
			return o, err
		}
		o.EndedAt, err = ParseTime(e)
		return o, err
	}, `SELECT id, started_at, ended_at, reason FROM outages WHERE started_at >= ?
		ORDER BY started_at DESC, id DESC`, FormatTime(since))
}

// ListFailedDeliveries returns the newest event deliveries that finished with
// ok = 0, newest first.
func (q *queries) ListFailedDeliveries(ctx context.Context, limit int) ([]EventDelivery, error) {
	return collect(ctx, q.db, func(r rowScanner) (EventDelivery, error) {
		var (
			d   EventDelivery
			s   string
			fin sql.NullString
			ok  sql.NullInt64
			err error
		)
		if err = r.Scan(&d.ID, &d.EventType, &d.Hook, &d.PayloadJSON, &ok, &d.Error, &s, &fin); err != nil {
			return d, err
		}
		d.OK = nullBoolPtr(ok)
		if d.StartedAt, err = ParseTime(s); err != nil {
			return d, err
		}
		d.FinishedAt, err = parseNullTime(fin)
		return d, err
	}, `SELECT id, event_type, hook, payload_json, ok, error, started_at, finished_at
		FROM event_deliveries WHERE ok = 0 ORDER BY started_at DESC, id DESC LIMIT ?`, limit)
}
