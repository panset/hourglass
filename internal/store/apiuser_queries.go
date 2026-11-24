// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"encoding/json"
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
	// WeightSource is the sample's weight_source, empty when null.
	WeightSource string
}

// HoldLastSamples returns the newest usage sample per hold id. Holds without
// a sample are absent from the map.
func (q *queries) HoldLastSamples(ctx context.Context, holdIDs []int64) (map[int64]LastSample, error) {
	out := make(map[int64]LastSample, len(holdIDs))
	for i := 0; i < len(holdIDs); i += idChunk {
		list, args := inList(holdIDs, i, min(i+idChunk, len(holdIDs)))
		// Driven from the holds: one seek per hold on UNIQUE (hold_id,
		// polled_at), never a walk over the samples of every listed hold.
		rows, err := q.db.QueryContext(ctx, `SELECT u.hold_id, u.polled_at, u.weight, u.multiplier, u.reason, u.weight_source
			FROM holds h JOIN usage_samples u ON u.hold_id = h.id
			AND u.polled_at = (SELECT MAX(m.polled_at) FROM usage_samples m WHERE m.hold_id = h.id)
			WHERE h.id IN `+list, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var (
				s   LastSample
				at  string
				src sql.NullString
			)
			if err := rows.Scan(&s.HoldID, &at, &s.Weight, &s.Multiplier, &s.Reason, &src); err != nil {
				rows.Close()
				return nil, err
			}
			s.WeightSource = src.String
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

// WeightWindow names the usage samples one ledger entry stands for: those of
// the hold with From <= polled_at < To. Key identifies the entry in the
// result (the API uses the entry's id).
type WeightWindow struct {
	Key    int64
	HoldID int64
	From   time.Time
	To     time.Time
}

// WeightAggregate sums up the charged samples of one window: only samples
// with a debit above zero count. Sources are the distinct non-null
// weight_source values, in no particular order; NullSources counts the
// samples that have none (written before migration 0008).
type WeightAggregate struct {
	Count       int
	Min, Max    float64
	Sources     []string
	NullSources int
}

// DebitWeightAggregates returns, per window key, the weight range and sources
// of the samples with a debit above zero in the window (design C1). Windows
// with no such sample are absent from the map. It is one statement however
// many windows there are: the windows are bound as one JSON array and each
// is a range seek on UNIQUE (hold_id, polled_at).
func (q *queries) DebitWeightAggregates(ctx context.Context, windows []WeightWindow) (map[int64]WeightAggregate, error) {
	out := make(map[int64]WeightAggregate, len(windows))
	if len(windows) == 0 {
		return out, nil
	}
	type win struct {
		K int64  `json:"k"`
		H int64  `json:"h"`
		F string `json:"f"`
		T string `json:"t"`
	}
	ws := make([]win, len(windows))
	for i, w := range windows {
		ws[i] = win{K: w.Key, H: w.HoldID, F: FormatTime(w.From), T: FormatTime(w.To)}
	}
	arg, err := json.Marshal(ws)
	if err != nil {
		return nil, err
	}
	// CROSS JOIN keeps the windows as the outer loop, so each window is one
	// seek on the (hold_id, polled_at) index.
	rows, err := q.db.QueryContext(ctx, `SELECT w.k, COUNT(*), MIN(u.weight), MAX(u.weight),
			SUM(u.weight_source IS NULL), COALESCE(group_concat(DISTINCT u.weight_source), '')
		FROM (SELECT json_extract(value, '$.k') AS k, json_extract(value, '$.h') AS h,
				json_extract(value, '$.f') AS f, json_extract(value, '$.t') AS t FROM json_each(?)) w
		CROSS JOIN usage_samples u
		WHERE u.hold_id = w.h AND u.polled_at >= w.f AND u.polled_at < w.t AND u.debit_mt > 0
		GROUP BY w.k`, string(arg))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			key  int64
			a    WeightAggregate
			srcs string
		)
		if err := rows.Scan(&key, &a.Count, &a.Min, &a.Max, &a.NullSources, &srcs); err != nil {
			return nil, err
		}
		if srcs != "" {
			a.Sources = strings.Split(srcs, ",")
		}
		out[key] = a
	}
	return out, rows.Err()
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
