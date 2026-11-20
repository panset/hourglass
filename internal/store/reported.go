// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"encoding/json"
	"time"
)

// ReportedPressure is the single row of reported_pressure.
type ReportedPressure struct {
	Value          float64
	ObservedAt     time.Time
	ReceivedAt     time.Time
	ServiceAccount string
}

// ReportedHeader is the single row of reported_deployments_meta. It exists
// even when the deployed set is empty.
type ReportedHeader struct {
	ObservedAt     time.Time
	ReceivedAt     time.Time
	ServiceAccount string
}

// ReportedDeployment is a row of reported_deployments.
type ReportedDeployment struct {
	SetupName string
	Labels    map[string]string
}

// GetReportedPressure returns the stored pressure report, or ErrNotFound
// before the first one.
func (q *queries) GetReportedPressure(ctx context.Context) (ReportedPressure, error) {
	var (
		p        ReportedPressure
		obs, rec string
	)
	err := q.db.QueryRowContext(ctx, `SELECT value, observed_at, received_at, service_account FROM reported_pressure`).
		Scan(&p.Value, &obs, &rec, &p.ServiceAccount)
	if err != nil {
		return p, mapErr(err)
	}
	if p.ObservedAt, err = ParseTime(obs); err != nil {
		return p, err
	}
	p.ReceivedAt, err = ParseTime(rec)
	return p, err
}

// ReplaceReportedPressure replaces the stored pressure report. It does not
// compare timestamps; ordering is the caller's rule (internal/reported), which
// runs it in the same transaction.
func (q *queries) ReplaceReportedPressure(ctx context.Context, p ReportedPressure) error {
	return q.atomic(ctx, func(q *queries) error {
		if _, err := q.db.ExecContext(ctx, `DELETE FROM reported_pressure`); err != nil {
			return err
		}
		_, err := q.db.ExecContext(ctx, `INSERT INTO reported_pressure (value, observed_at, received_at, service_account)
			VALUES (?, ?, ?, ?)`, p.Value, FormatTime(p.ObservedAt), FormatTime(p.ReceivedAt), p.ServiceAccount)
		return err
	})
}

// GetReportedDeployments returns the stored header and deployed set (ordered
// by setup name) from one consistent snapshot, or ErrNotFound before the first
// report. An empty set is a non-nil empty slice.
func (q *queries) GetReportedDeployments(ctx context.Context) (ReportedHeader, []ReportedDeployment, error) {
	var (
		h    ReportedHeader
		set  []ReportedDeployment
		rerr error
	)
	rerr = q.atomic(ctx, func(q *queries) error {
		var obs, rec string
		if err := q.db.QueryRowContext(ctx, `SELECT observed_at, received_at, service_account FROM reported_deployments_meta`).
			Scan(&obs, &rec, &h.ServiceAccount); err != nil {
			return mapErr(err)
		}
		var err error
		if h.ObservedAt, err = ParseTime(obs); err != nil {
			return err
		}
		if h.ReceivedAt, err = ParseTime(rec); err != nil {
			return err
		}
		set, err = collect(ctx, q.db, func(r rowScanner) (ReportedDeployment, error) {
			var (
				d  ReportedDeployment
				lj string
			)
			if err := r.Scan(&d.SetupName, &lj); err != nil {
				return d, err
			}
			if err := json.Unmarshal([]byte(lj), &d.Labels); err != nil {
				return d, err
			}
			if d.Labels == nil {
				d.Labels = map[string]string{}
			}
			return d, nil
		}, `SELECT setup_name, labels_json FROM reported_deployments ORDER BY setup_name`)
		if set == nil {
			set = []ReportedDeployment{}
		}
		return err
	})
	if rerr != nil {
		return ReportedHeader{}, nil, rerr
	}
	return h, set, nil
}

// ReplaceReportedDeployments replaces the header and the whole set in one
// transaction: all or nothing, and a reader sees either the old report or the
// new one. A duplicate setup name returns ErrConflict and changes nothing.
func (q *queries) ReplaceReportedDeployments(ctx context.Context, h ReportedHeader, set []ReportedDeployment) error {
	return q.atomic(ctx, func(q *queries) error {
		if _, err := q.db.ExecContext(ctx, `DELETE FROM reported_deployments`); err != nil {
			return err
		}
		if _, err := q.db.ExecContext(ctx, `DELETE FROM reported_deployments_meta`); err != nil {
			return err
		}
		if _, err := q.db.ExecContext(ctx, `INSERT INTO reported_deployments_meta (observed_at, received_at, service_account)
			VALUES (?, ?, ?)`, FormatTime(h.ObservedAt), FormatTime(h.ReceivedAt), h.ServiceAccount); err != nil {
			return err
		}
		for _, d := range set {
			labels := d.Labels
			if labels == nil {
				labels = map[string]string{}
			}
			lj, err := json.Marshal(labels)
			if err != nil {
				return err
			}
			if _, err := q.db.ExecContext(ctx, `INSERT INTO reported_deployments (setup_name, labels_json) VALUES (?, ?)`,
				d.SetupName, string(lj)); err != nil {
				return mapErr(err)
			}
		}
		return nil
	})
}
