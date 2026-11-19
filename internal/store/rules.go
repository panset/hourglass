// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"time"
)

// Rule is a row of rules.
type Rule struct {
	ID        int64
	Kind      string
	Position  int
	Selector  string
	Value     float64
	UpdatedBy *int64
	UpdatedAt time.Time
}

func scanRule(r rowScanner) (Rule, error) {
	var (
		x   Rule
		by  sql.NullInt64
		at  string
		err error
	)
	if err = r.Scan(&x.ID, &x.Kind, &x.Position, &x.Selector, &x.Value, &by, &at); err != nil {
		return x, err
	}
	x.UpdatedBy = nullInt64Ptr(by)
	x.UpdatedAt, err = ParseTime(at)
	return x, err
}

// ListRules returns the rules of a kind in position order.
func (q *queries) ListRules(ctx context.Context, kind string) ([]Rule, error) {
	return collect(ctx, q.db, scanRule, `SELECT id, kind, position, selector, value, updated_by, updated_at
		FROM rules WHERE kind = ? ORDER BY position, id`, kind)
}

// ReplaceRules replaces the whole ordered list for a kind in one transaction.
// Positions are assigned from the slice order (0-based); the Position, ID and
// UpdatedAt fields of the input are ignored.
func (q *queries) ReplaceRules(ctx context.Context, kind string, rules []Rule, updatedBy *int64, now time.Time) error {
	return q.atomic(ctx, func(q *queries) error {
		if _, err := q.db.ExecContext(ctx, `DELETE FROM rules WHERE kind = ?`, kind); err != nil {
			return err
		}
		for i, r := range rules {
			if _, err := q.db.ExecContext(ctx, `INSERT INTO rules (kind, position, selector, value, updated_by, updated_at)
				VALUES (?, ?, ?, ?, ?, ?)`, kind, i, r.Selector, r.Value, nullInt(updatedBy), FormatTime(now)); err != nil {
				return mapErr(err)
			}
		}
		return nil
	})
}
