// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
)

// LatestAllowancePeriod returns the newest period (YYYY-MM) that holds an
// allowance entry for any user, or "" when the ledger has none. The ledger
// service derives the last opened period from it (no separate state).
func (q *queries) LatestAllowancePeriod(ctx context.Context) (string, error) {
	var p sql.NullString
	err := q.db.QueryRowContext(ctx, `SELECT MAX(period) FROM ledger WHERE type = 'allowance'`).Scan(&p)
	return p.String, err
}

// CountLedger counts a user's entries of one type in a period.
func (q *queries) CountLedger(ctx context.Context, userID int64, period, typ string) (int, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger WHERE user_id = ? AND period = ? AND type = ?`,
		userID, period, typ).Scan(&n)
	return n, err
}

// ListRoleHolders returns the users that have at least one role and are not
// the local admin account, ordered by id.
func (q *queries) ListRoleHolders(ctx context.Context) ([]User, error) {
	return collect(ctx, q.db, scanUser, `SELECT `+userCols+` FROM users
		WHERE auth_source <> 'local' AND EXISTS (SELECT 1 FROM user_roles r WHERE r.user_id = users.id)
		ORDER BY id`)
}
