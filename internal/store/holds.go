// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Hold is a row of holds.
type Hold struct {
	ID                int64
	SetupID           int64
	UserID            int64
	AcquiredAt        time.Time
	MeteringStartedAt *time.Time
	DeployedAt        *time.Time
	ReleasedAt        *time.Time
	ReleasedBy        *int64
	ReleaseReason     string
}

const holdCols = `id, setup_id, user_id, acquired_at, metering_started_at, deployed_at,
	released_at, released_by, release_reason`

func scanHold(r rowScanner) (Hold, error) {
	var (
		h                         Hold
		acq                       string
		metering, deployed, relAt sql.NullString
		by                        sql.NullInt64
		reason                    sql.NullString
		err                       error
	)
	if err = r.Scan(&h.ID, &h.SetupID, &h.UserID, &acq, &metering, &deployed, &relAt, &by, &reason); err != nil {
		return h, err
	}
	if h.AcquiredAt, err = ParseTime(acq); err != nil {
		return h, err
	}
	if h.MeteringStartedAt, err = parseNullTime(metering); err != nil {
		return h, err
	}
	if h.DeployedAt, err = parseNullTime(deployed); err != nil {
		return h, err
	}
	if h.ReleasedAt, err = parseNullTime(relAt); err != nil {
		return h, err
	}
	h.ReleasedBy, h.ReleaseReason = nullInt64Ptr(by), reason.String
	return h, nil
}

// CreateHold inserts a hold. If the setup already has an active hold it
// returns ErrSetupHeld.
func (q *queries) CreateHold(ctx context.Context, setupID, userID int64, now time.Time) (Hold, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO holds (setup_id, user_id, acquired_at) VALUES (?, ?, ?)`,
		setupID, userID, FormatTime(now))
	if err != nil {
		err = mapErr(err)
		if errors.Is(err, ErrConflict) {
			return Hold{}, ErrSetupHeld
		}
		return Hold{}, err
	}
	id, _ := res.LastInsertId()
	return q.GetHold(ctx, id)
}

// GetHold returns a hold by id.
func (q *queries) GetHold(ctx context.Context, id int64) (Hold, error) {
	h, err := scanHold(q.db.QueryRowContext(ctx, `SELECT `+holdCols+` FROM holds WHERE id = ?`, id))
	return h, mapErr(err)
}

// ActiveHoldForSetup returns the unreleased hold of a setup, or ErrNotFound.
func (q *queries) ActiveHoldForSetup(ctx context.Context, setupID int64) (Hold, error) {
	h, err := scanHold(q.db.QueryRowContext(ctx,
		`SELECT `+holdCols+` FROM holds WHERE setup_id = ? AND released_at IS NULL`, setupID))
	return h, mapErr(err)
}

// setOnce sets a nullable timestamp column only if it is still null.
func (q *queries) setOnce(ctx context.Context, col string, id int64, t time.Time) (bool, error) {
	res, err := q.db.ExecContext(ctx, `UPDATE holds SET `+col+` = ? WHERE id = ? AND `+col+` IS NULL`, FormatTime(t), id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		if _, err := q.GetHold(ctx, id); err != nil {
			return false, err
		}
	}
	return n > 0, nil
}

// SetMeteringStarted records metering_started_at once; it reports whether it
// was set by this call (false if it already had a value).
func (q *queries) SetMeteringStarted(ctx context.Context, id int64, t time.Time) (bool, error) {
	return q.setOnce(ctx, "metering_started_at", id, t)
}

// SetDeployed records deployed_at once; it reports whether it was set by this call.
func (q *queries) SetDeployed(ctx context.Context, id int64, t time.Time) (bool, error) {
	return q.setOnce(ctx, "deployed_at", id, t)
}

// ReleaseHold marks an active hold released. releasedBy is nil for a release
// by the system. Releasing an already released hold returns ErrConflict.
func (q *queries) ReleaseHold(ctx context.Context, id int64, releasedBy *int64, reason string, now time.Time) error {
	res, err := q.db.ExecContext(ctx, `UPDATE holds SET released_at = ?, released_by = ?, release_reason = ?
		WHERE id = ? AND released_at IS NULL`, FormatTime(now), nullInt(releasedBy), nullStr(reason), id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := q.GetHold(ctx, id); err != nil {
			return err
		}
		return ErrConflict
	}
	return nil
}

// ActiveHolds returns every unreleased hold, oldest first.
func (q *queries) ActiveHolds(ctx context.Context) ([]Hold, error) {
	return collect(ctx, q.db, scanHold, `SELECT `+holdCols+` FROM holds WHERE released_at IS NULL ORDER BY id`)
}

// CountActiveHolds counts a user's unreleased holds.
func (q *queries) CountActiveHolds(ctx context.Context, userID int64) (int, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM holds WHERE user_id = ? AND released_at IS NULL`, userID).Scan(&n)
	return n, err
}

// UserHolds returns a user's current and past holds, newest first.
func (q *queries) UserHolds(ctx context.Context, userID int64) ([]Hold, error) {
	return collect(ctx, q.db, scanHold, `SELECT `+holdCols+` FROM holds WHERE user_id = ? ORDER BY id DESC`, userID)
}
