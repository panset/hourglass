// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"time"
)

// TouchLocalAdminLogin records a sign-in of the local admin account. There is
// exactly one local account, so it needs no id. internal/auth does not set
// last_login_at for local sign-ins; the API layer calls this after a
// successful POST /auth/local.
func (q *queries) TouchLocalAdminLogin(ctx context.Context, at time.Time) error {
	_, err := q.db.ExecContext(ctx, `UPDATE users SET last_login_at = ? WHERE auth_source = ?`,
		FormatTime(at), AuthLocal)
	return err
}

// SumAllDebitsSince sums every sampled debit (milli-tokens) with
// polled_at >= since, across all users. It feeds the tokens-debited metric.
func (q *queries) SumAllDebitsSince(ctx context.Context, since time.Time) (int64, error) {
	var n sql.NullInt64
	err := q.db.QueryRowContext(ctx, `SELECT SUM(debit_mt) FROM usage_samples WHERE polled_at >= ?`,
		FormatTime(since)).Scan(&n)
	return n.Int64, err
}
