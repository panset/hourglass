// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// Queries used by the user-facing API handlers (internal/api/user_*.go).

// idChunk bounds the number of ids bound in one IN (...) list, well below
// SQLite's host-parameter limit.
const idChunk = 500

func inList(ids []int64, from, to int) (string, []any) {
	args := make([]any, 0, to-from)
	for _, id := range ids[from:to] {
		args = append(args, id)
	}
	return "(" + strings.TrimSuffix(strings.Repeat("?,", to-from), ",") + ")", args
}

// HoldCharges returns, per hold id, the total debited for the hold as a
// positive amount, from the ledger (which is kept; usage samples are pruned).
// Holds with no debit are absent from the map.
func (q *queries) HoldCharges(ctx context.Context, holdIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(holdIDs))
	for i := 0; i < len(holdIDs); i += idChunk {
		list, args := inList(holdIDs, i, min(i+idChunk, len(holdIDs)))
		rows, err := q.db.QueryContext(ctx, `SELECT hold_id, -SUM(amount_mt) FROM ledger
			WHERE type = 'debit' AND hold_id IN `+list+` GROUP BY hold_id`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, mt int64
			if err := rows.Scan(&id, &mt); err != nil {
				rows.Close()
				return nil, err
			}
			out[id] = mt
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return out, nil
}

// LastSample is the newest usage sample of a hold: the weight and reason the
// metering policy last decided with.
type LastSample struct {
	HoldID     int64
	PolledAt   time.Time
	Weight     float64
	Multiplier float64
	Reason     string
}

// HoldLastSamples returns the newest usage sample per hold id. Holds without
// a sample are absent from the map.
func (q *queries) HoldLastSamples(ctx context.Context, holdIDs []int64) (map[int64]LastSample, error) {
	out := make(map[int64]LastSample, len(holdIDs))
	for i := 0; i < len(holdIDs); i += idChunk {
		list, args := inList(holdIDs, i, min(i+idChunk, len(holdIDs)))
		// Driven from the holds: one seek per hold on UNIQUE (hold_id,
		// polled_at), never a walk over the samples of every listed hold.
		rows, err := q.db.QueryContext(ctx, `SELECT u.hold_id, u.polled_at, u.weight, u.multiplier, u.reason
			FROM holds h JOIN usage_samples u ON u.hold_id = h.id
			AND u.polled_at = (SELECT MAX(m.polled_at) FROM usage_samples m WHERE m.hold_id = h.id)
			WHERE h.id IN `+list, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var (
				s  LastSample
				at string
			)
			if err := rows.Scan(&s.HoldID, &at, &s.Weight, &s.Multiplier, &s.Reason); err != nil {
				rows.Close()
				return nil, err
			}
			if s.PolledAt, err = ParseTime(at); err != nil {
				rows.Close()
				return nil, err
			}
			out[s.HoldID] = s
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return out, nil
}

// LedgerPeriods lists the periods in which the user has ledger entries,
// newest first.
func (q *queries) LedgerPeriods(ctx context.Context, userID int64) ([]string, error) {
	return collect(ctx, q.db, func(r rowScanner) (string, error) {
		var p string
		return p, r.Scan(&p)
	}, `SELECT DISTINCT period FROM ledger WHERE user_id = ? ORDER BY period DESC`, userID)
}

// DebitEntry is one ledger debit: when it was written and its positive size.
type DebitEntry struct {
	At time.Time
	MT int64
}

// DebitEntries lists the user's ledger debits with from <= created_at < to,
// oldest first. The caller buckets them into days in the configured
// timezone.
func (q *queries) DebitEntries(ctx context.Context, userID int64, from, to time.Time) ([]DebitEntry, error) {
	return collect(ctx, q.db, func(r rowScanner) (DebitEntry, error) {
		var (
			d  DebitEntry
			at string
			n  sql.NullInt64
		)
		if err := r.Scan(&at, &n); err != nil {
			return d, err
		}
		d.MT = -n.Int64
		var err error
		d.At, err = ParseTime(at)
		return d, err
	}, `SELECT created_at, amount_mt FROM ledger
		WHERE user_id = ? AND type = 'debit' AND created_at >= ? AND created_at < ?
		ORDER BY created_at, id`, userID, FormatTime(from), FormatTime(to))
}

// DebitMultipliers returns, per ledger entry id, the pressure multiplier the
// debit was charged at, found through the usage sample with the same hold and
// poll time (the collector writes a debit with created_at = polled_at).
// Debits whose sample was pruned are absent, and so are rolled-up debits
// (LED-15): such a row stands for many polls, and the sample at its
// created_at is only the last of them.
func (q *queries) DebitMultipliers(ctx context.Context, userID int64, period string) (map[int64]float64, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT l.id, u.multiplier FROM ledger l
		JOIN usage_samples u ON u.hold_id = l.hold_id AND u.polled_at = l.created_at
		WHERE l.user_id = ? AND l.period = ? AND l.type = 'debit' AND l.rollup_count IS NULL`, userID, period)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]float64{}
	for rows.Next() {
		var (
			id int64
			m  float64
		)
		if err := rows.Scan(&id, &m); err != nil {
			return nil, err
		}
		out[id] = m
	}
	return out, rows.Err()
}
