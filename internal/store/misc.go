// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"time"
)

// ---- pressure_samples ----

// PressureSample is a row of pressure_samples.
type PressureSample struct {
	PolledAt   time.Time
	Source     string
	Raw        float64
	Smoothed   float64
	Multiplier float64
}

func scanPressure(r rowScanner) (PressureSample, error) {
	var (
		p   PressureSample
		at  string
		err error
	)
	if err = r.Scan(&at, &p.Source, &p.Raw, &p.Smoothed, &p.Multiplier); err != nil {
		return p, err
	}
	p.PolledAt, err = ParseTime(at)
	return p, err
}

// InsertPressureSample inserts a sample; a repeated polled_at is ErrConflict.
func (q *queries) InsertPressureSample(ctx context.Context, p PressureSample) error {
	_, err := q.db.ExecContext(ctx, `INSERT INTO pressure_samples (polled_at, source, raw, smoothed, multiplier)
		VALUES (?, ?, ?, ?, ?)`, FormatTime(p.PolledAt), p.Source, p.Raw, p.Smoothed, p.Multiplier)
	return mapErr(err)
}

// PressureSince lists samples with polled_at >= since in time order.
func (q *queries) PressureSince(ctx context.Context, since time.Time) ([]PressureSample, error) {
	return collect(ctx, q.db, scanPressure, `SELECT polled_at, source, raw, smoothed, multiplier
		FROM pressure_samples WHERE polled_at >= ? ORDER BY polled_at`, FormatTime(since))
}

// LatestPressure returns the newest n samples, newest first.
func (q *queries) LatestPressure(ctx context.Context, n int) ([]PressureSample, error) {
	return collect(ctx, q.db, scanPressure, `SELECT polled_at, source, raw, smoothed, multiplier
		FROM pressure_samples ORDER BY polled_at DESC LIMIT ?`, n)
}

// ---- orphans ----

// Orphan is a row of orphans.
type Orphan struct {
	ResourceName string
	FirstSeenAt  time.Time
	LastSeenAt   time.Time
	LabelsJSON   string
}

// UpsertOrphan inserts an orphan or, if resource_name exists, updates
// last_seen_at and labels while keeping first_seen_at.
func (q *queries) UpsertOrphan(ctx context.Context, resourceName, labelsJSON string, seen time.Time) error {
	if labelsJSON == "" {
		labelsJSON = "{}"
	}
	_, err := q.db.ExecContext(ctx, `INSERT INTO orphans (resource_name, first_seen_at, last_seen_at, labels_json)
		VALUES (?, ?, ?, ?) ON CONFLICT (resource_name) DO UPDATE
		SET last_seen_at = excluded.last_seen_at, labels_json = excluded.labels_json`,
		resourceName, FormatTime(seen), FormatTime(seen), labelsJSON)
	return err
}

// DeleteOrphansNotSeenSince deletes orphans whose last_seen_at is before
// poll, that is, those not seen in the poll at that time. It returns the count.
func (q *queries) DeleteOrphansNotSeenSince(ctx context.Context, poll time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `DELETE FROM orphans WHERE last_seen_at < ?`, FormatTime(poll))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ListOrphans lists orphans ordered by resource name.
func (q *queries) ListOrphans(ctx context.Context) ([]Orphan, error) {
	return collect(ctx, q.db, func(r rowScanner) (Orphan, error) {
		var o Orphan
		var first, last string
		if err := r.Scan(&o.ResourceName, &first, &last, &o.LabelsJSON); err != nil {
			return o, err
		}
		var err error
		if o.FirstSeenAt, err = ParseTime(first); err != nil {
			return o, err
		}
		o.LastSeenAt, err = ParseTime(last)
		return o, err
	}, `SELECT resource_name, first_seen_at, last_seen_at, labels_json FROM orphans ORDER BY resource_name`)
}

// ---- outages ----

// Outage is a row of outages: a gap that was not charged.
type Outage struct {
	ID        int64
	StartedAt time.Time
	EndedAt   time.Time
	Reason    string
}

// InsertOutage records an uncharged gap.
func (q *queries) InsertOutage(ctx context.Context, o Outage) (int64, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO outages (started_at, ended_at, reason) VALUES (?, ?, ?)`,
		FormatTime(o.StartedAt), FormatTime(o.EndedAt), o.Reason)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListOutages returns the newest n outages, newest first.
func (q *queries) ListOutages(ctx context.Context, limit int) ([]Outage, error) {
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
	}, `SELECT id, started_at, ended_at, reason FROM outages ORDER BY started_at DESC, id DESC LIMIT ?`, limit)
}

// ---- collector_runs ----

// CollectorRun is a row of collector_runs. OK and FinishedAt are nil until
// the run finishes.
type CollectorRun struct {
	ID         int64
	StartedAt  time.Time
	FinishedAt *time.Time
	OK         *bool
	Error      string
}

func scanRun(r rowScanner) (CollectorRun, error) {
	var (
		c   CollectorRun
		s   string
		fin sql.NullString
		ok  sql.NullInt64
		err error
	)
	if err = r.Scan(&c.ID, &s, &fin, &ok, &c.Error); err != nil {
		return c, err
	}
	if c.StartedAt, err = ParseTime(s); err != nil {
		return c, err
	}
	c.OK = nullBoolPtr(ok)
	c.FinishedAt, err = parseNullTime(fin)
	return c, err
}

const runCols = `id, started_at, finished_at, ok, error`

// StartCollectorRun inserts an unfinished run and returns its id.
func (q *queries) StartCollectorRun(ctx context.Context, started time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO collector_runs (started_at) VALUES (?)`, FormatTime(started))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FinishCollectorRun records the outcome of a run.
func (q *queries) FinishCollectorRun(ctx context.Context, id int64, finished time.Time, ok bool, errMsg string) error {
	res, err := q.db.ExecContext(ctx, `UPDATE collector_runs SET finished_at = ?, ok = ?, error = ? WHERE id = ?`,
		FormatTime(finished), boolInt(ok), errMsg, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// LastSuccessfulRun returns the newest run with ok = 1, or ErrNotFound.
func (q *queries) LastSuccessfulRun(ctx context.Context) (CollectorRun, error) {
	c, err := scanRun(q.db.QueryRowContext(ctx, `SELECT `+runCols+` FROM collector_runs
		WHERE ok = 1 ORDER BY id DESC LIMIT 1`))
	return c, mapErr(err)
}

// LatestRun returns the newest run, or ErrNotFound.
func (q *queries) LatestRun(ctx context.Context) (CollectorRun, error) {
	c, err := scanRun(q.db.QueryRowContext(ctx, `SELECT `+runCols+` FROM collector_runs ORDER BY id DESC LIMIT 1`))
	return c, mapErr(err)
}

// ---- event_deliveries ----

// EventDelivery is a row of event_deliveries. OK and FinishedAt are nil
// until the delivery finishes.
type EventDelivery struct {
	ID          int64
	EventType   string
	Hook        string
	PayloadJSON string
	OK          *bool
	Error       string
	StartedAt   time.Time
	FinishedAt  *time.Time
}

// InsertEventDelivery records the start of a delivery and returns its id.
func (q *queries) InsertEventDelivery(ctx context.Context, eventType, hook, payloadJSON string, started time.Time) (int64, error) {
	if payloadJSON == "" {
		payloadJSON = "{}"
	}
	res, err := q.db.ExecContext(ctx, `INSERT INTO event_deliveries (event_type, hook, payload_json, started_at)
		VALUES (?, ?, ?, ?)`, eventType, hook, payloadJSON, FormatTime(started))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FinishEventDelivery records the outcome of a delivery.
func (q *queries) FinishEventDelivery(ctx context.Context, id int64, finished time.Time, ok bool, errMsg string) error {
	res, err := q.db.ExecContext(ctx, `UPDATE event_deliveries SET finished_at = ?, ok = ?, error = ? WHERE id = ?`,
		FormatTime(finished), boolInt(ok), errMsg, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListEventDeliveries returns the newest deliveries, newest first.
func (q *queries) ListEventDeliveries(ctx context.Context, limit int) ([]EventDelivery, error) {
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
		FROM event_deliveries ORDER BY started_at DESC, id DESC LIMIT ?`, limit)
}

// PruneEventDeliveries deletes deliveries started before cutoff.
func (q *queries) PruneEventDeliveries(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `DELETE FROM event_deliveries WHERE started_at < ?`, FormatTime(cutoff))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ---- settings ----

// Setting is a row of settings.
type Setting struct {
	Key       string
	ValueJSON string
	UpdatedBy *int64
	UpdatedAt time.Time
}

func scanSetting(r rowScanner) (Setting, error) {
	var (
		s   Setting
		by  sql.NullInt64
		at  string
		err error
	)
	if err = r.Scan(&s.Key, &s.ValueJSON, &by, &at); err != nil {
		return s, err
	}
	s.UpdatedBy = nullInt64Ptr(by)
	s.UpdatedAt, err = ParseTime(at)
	return s, err
}

// GetSetting returns one setting, or ErrNotFound.
func (q *queries) GetSetting(ctx context.Context, key string) (Setting, error) {
	s, err := scanSetting(q.db.QueryRowContext(ctx,
		`SELECT key, value_json, updated_by, updated_at FROM settings WHERE key = ?`, key))
	return s, mapErr(err)
}

// PutSetting inserts or replaces a setting.
func (q *queries) PutSetting(ctx context.Context, key, valueJSON string, updatedBy *int64, now time.Time) error {
	_, err := q.db.ExecContext(ctx, `INSERT INTO settings (key, value_json, updated_by, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value_json = excluded.value_json,
		updated_by = excluded.updated_by, updated_at = excluded.updated_at`,
		key, valueJSON, nullInt(updatedBy), FormatTime(now))
	return mapErr(err)
}

// AllSettings returns every setting ordered by key.
func (q *queries) AllSettings(ctx context.Context) ([]Setting, error) {
	return collect(ctx, q.db, scanSetting, `SELECT key, value_json, updated_by, updated_at FROM settings ORDER BY key`)
}

// ---- audit_log ----

// AuditEntry is a row of audit_log. ActorID is nil for the system.
type AuditEntry struct {
	ID         int64
	ActorID    *int64
	Action     string
	Target     string
	BeforeJSON string // empty when null
	AfterJSON  string // empty when null
	CreatedAt  time.Time
}

// AuditFilter narrows ListAudit. Zero values mean no filter; a zero Limit
// means 100.
type AuditFilter struct {
	ActorID *int64
	Action  string
	Limit   int
	Offset  int
}

// AppendAudit appends an audit entry and returns its id.
func (q *queries) AppendAudit(ctx context.Context, e AuditEntry, now time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO audit_log (actor_id, action, target, before_json, after_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, nullInt(e.ActorID), e.Action, e.Target,
		nullStr(e.BeforeJSON), nullStr(e.AfterJSON), FormatTime(now))
	if err != nil {
		return 0, mapErr(err)
	}
	return res.LastInsertId()
}

// ListAudit returns audit entries newest first.
func (q *queries) ListAudit(ctx context.Context, f AuditFilter) ([]AuditEntry, error) {
	query := `SELECT id, actor_id, action, target, before_json, after_json, created_at FROM audit_log WHERE 1 = 1`
	var args []any
	if f.ActorID != nil {
		query += ` AND actor_id = ?`
		args = append(args, *f.ActorID)
	}
	if f.Action != "" {
		query += ` AND action = ?`
		args = append(args, f.Action)
	}
	if f.Limit <= 0 {
		f.Limit = 100
	}
	query += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, f.Limit, f.Offset)
	return collect(ctx, q.db, func(r rowScanner) (AuditEntry, error) {
		var (
			e        AuditEntry
			actor    sql.NullInt64
			bef, aft sql.NullString
			at       string
			err      error
		)
		if err = r.Scan(&e.ID, &actor, &e.Action, &e.Target, &bef, &aft, &at); err != nil {
			return e, err
		}
		e.ActorID, e.BeforeJSON, e.AfterJSON = nullInt64Ptr(actor), bef.String, aft.String
		e.CreatedAt, err = ParseTime(at)
		return e, err
	}, query, args...)
}

// ---- sessions ----

// Session is a row of sessions.
type Session struct {
	ID         string
	UserID     int64
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

// CreateSession inserts a session; created and last seen are both now.
func (q *queries) CreateSession(ctx context.Context, id string, userID int64, now, expires time.Time) error {
	_, err := q.db.ExecContext(ctx, `INSERT INTO sessions (id, user_id, created_at, last_seen_at, expires_at)
		VALUES (?, ?, ?, ?, ?)`, id, userID, FormatTime(now), FormatTime(now), FormatTime(expires))
	return mapErr(err)
}

// GetSession returns a session by id, or ErrNotFound. Expiry is not checked.
func (q *queries) GetSession(ctx context.Context, id string) (Session, error) {
	var (
		s       Session
		c, l, e string
	)
	err := q.db.QueryRowContext(ctx, `SELECT id, user_id, created_at, last_seen_at, expires_at
		FROM sessions WHERE id = ?`, id).Scan(&s.ID, &s.UserID, &c, &l, &e)
	if err != nil {
		return s, mapErr(err)
	}
	if s.CreatedAt, err = ParseTime(c); err != nil {
		return s, err
	}
	if s.LastSeenAt, err = ParseTime(l); err != nil {
		return s, err
	}
	s.ExpiresAt, err = ParseTime(e)
	return s, err
}

// TouchSession sets last_seen_at and expires_at.
func (q *queries) TouchSession(ctx context.Context, id string, lastSeen, expires time.Time) error {
	res, err := q.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ?, expires_at = ? WHERE id = ?`,
		FormatTime(lastSeen), FormatTime(expires), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSession deletes one session (no error if absent).
func (q *queries) DeleteSession(ctx context.Context, id string) error {
	_, err := q.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// DeleteExpiredSessions deletes sessions with expires_at <= now.
func (q *queries) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := q.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, FormatTime(now))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteUserSessions deletes every session of a user.
func (q *queries) DeleteUserSessions(ctx context.Context, userID int64) (int64, error) {
	res, err := q.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
