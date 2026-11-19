// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"time"
)

// Queries added for internal/collector (task 17).

// LastSuccessfulRunBefore returns the newest successful collector run that
// started strictly before t, or ErrNotFound. The collector uses it as the
// "previous successful poll" (MET-4); excluding runs at t itself means two
// collectors polling at the same instant compute the same elapsed time, and
// the usage-sample uniqueness then keeps the debit single (MET-5).
func (q *queries) LastSuccessfulRunBefore(ctx context.Context, t time.Time) (CollectorRun, error) {
	c, err := scanRun(q.db.QueryRowContext(ctx, `SELECT `+runCols+` FROM collector_runs
		WHERE ok = 1 AND started_at < ? ORDER BY started_at DESC, id DESC LIMIT 1`, FormatTime(t)))
	return c, mapErr(err)
}

// InsertPressureSampleIfAbsent inserts p unless a sample with the same
// polled_at exists; it reports whether a row was written (MET-5).
func (q *queries) InsertPressureSampleIfAbsent(ctx context.Context, p PressureSample) (bool, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO pressure_samples (polled_at, source, raw, smoothed, multiplier)
		VALUES (?, ?, ?, ?, ?) ON CONFLICT (polled_at) DO NOTHING`,
		FormatTime(p.PolledAt), p.Source, p.Raw, p.Smoothed, p.Multiplier)
	if err != nil {
		return false, mapErr(err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
