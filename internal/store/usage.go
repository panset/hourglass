// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"time"
)

// UsageSample is a row of usage_samples. DebitMT is the positive amount
// charged for the sample.
type UsageSample struct {
	ID           int64
	HoldID       int64
	PolledAt     time.Time
	Weight       float64
	RuleSelector string // empty when null
	Pressure     float64
	Multiplier   float64
	DebitMT      int64
	Reason       string
	// WeightSource is override, reported, rule, default or policy; empty when
	// null (samples written before migration 0008). RuleSelector is set only
	// when the source is rule.
	WeightSource string
}

// DayTotal is the sum of debits for one UTC calendar day (YYYY-MM-DD).
type DayTotal struct {
	Day     string
	DebitMT int64
}

// InsertUsageSample inserts a sample idempotently on (hold_id, polled_at)
// (MET-5) and reports whether a row was inserted.
func (q *queries) InsertUsageSample(ctx context.Context, s UsageSample) (bool, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO usage_samples
		(hold_id, polled_at, weight, rule_selector, pressure, multiplier, debit_mt, reason, weight_source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (hold_id, polled_at) DO NOTHING`,
		s.HoldID, FormatTime(s.PolledAt), s.Weight, nullStr(s.RuleSelector), s.Pressure, s.Multiplier, s.DebitMT, s.Reason,
		nullStr(s.WeightSource))
	if err != nil {
		return false, mapErr(err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListUsageSamples returns a hold's samples in time order.
func (q *queries) ListUsageSamples(ctx context.Context, holdID int64) ([]UsageSample, error) {
	return collect(ctx, q.db, func(r rowScanner) (UsageSample, error) {
		var (
			s    UsageSample
			at   string
			rule sql.NullString
			src  sql.NullString
		)
		err := r.Scan(&s.ID, &s.HoldID, &at, &s.Weight, &rule, &s.Pressure, &s.Multiplier, &s.DebitMT, &s.Reason, &src)
		if err != nil {
			return s, err
		}
		s.RuleSelector = rule.String
		s.WeightSource = src.String
		s.PolledAt, err = ParseTime(at)
		return s, err
	}, `SELECT id, hold_id, polled_at, weight, rule_selector, pressure, multiplier, debit_mt, reason, weight_source
		FROM usage_samples WHERE hold_id = ? ORDER BY polled_at`, holdID)
}

// DailyDebits returns per-UTC-day sums of a user's sampled debits with
// from <= polled_at < to, in day order.
func (q *queries) DailyDebits(ctx context.Context, userID int64, from, to time.Time) ([]DayTotal, error) {
	return collect(ctx, q.db, func(r rowScanner) (DayTotal, error) {
		var d DayTotal
		return d, r.Scan(&d.Day, &d.DebitMT)
	}, `SELECT substr(u.polled_at, 1, 10) AS day, SUM(u.debit_mt)
		FROM usage_samples u JOIN holds h ON h.id = u.hold_id
		WHERE h.user_id = ? AND u.polled_at >= ? AND u.polled_at < ?
		GROUP BY day ORDER BY day`, userID, FormatTime(from), FormatTime(to))
}

// SumDebitsSince sums a user's sampled debits with polled_at >= since.
func (q *queries) SumDebitsSince(ctx context.Context, userID int64, since time.Time) (int64, error) {
	var n sql.NullInt64
	err := q.db.QueryRowContext(ctx, `SELECT SUM(u.debit_mt) FROM usage_samples u
		JOIN holds h ON h.id = u.hold_id WHERE h.user_id = ? AND u.polled_at >= ?`,
		userID, FormatTime(since)).Scan(&n)
	return n.Int64, err
}

// PruneUsageSamples deletes samples polled before cutoff and returns the count.
func (q *queries) PruneUsageSamples(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `DELETE FROM usage_samples WHERE polled_at < ?`, FormatTime(cutoff))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
