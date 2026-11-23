// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// Preapproval is a row of preapprovals: an email (trimmed, lower case) and
// the roles a pending SSO user with that email claim gets at sign-in. The
// caller normalises and validates; the store keeps what it is given.
type Preapproval struct {
	ID        int64
	Email     string
	Roles     []string
	CreatedBy *int64
	CreatedAt time.Time
	UsedAt    *time.Time // first use; nil until used
	UsedBy    *int64
}

const preapprovalCols = `id, email, roles_json, created_by, created_at, used_at, used_by`

func scanPreapproval(r rowScanner) (Preapproval, error) {
	var (
		p                 Preapproval
		rolesJSON         string
		createdBy, usedBy sql.NullInt64
		created           string
		used              sql.NullString
	)
	if err := r.Scan(&p.ID, &p.Email, &rolesJSON, &createdBy, &created, &used, &usedBy); err != nil {
		return p, err
	}
	if err := json.Unmarshal([]byte(rolesJSON), &p.Roles); err != nil {
		return p, err
	}
	if p.Roles == nil {
		p.Roles = []string{}
	}
	p.CreatedBy, p.UsedBy = nullInt64Ptr(createdBy), nullInt64Ptr(usedBy)
	var err error
	if p.CreatedAt, err = ParseTime(created); err != nil {
		return p, err
	}
	p.UsedAt, err = parseNullTime(used)
	return p, err
}

// ListPreapprovals returns every database entry ordered by email.
func (q *queries) ListPreapprovals(ctx context.Context) ([]Preapproval, error) {
	return collect(ctx, q.db, scanPreapproval, `SELECT `+preapprovalCols+` FROM preapprovals ORDER BY email, id`)
}

// GetPreapprovalByEmail returns the entry for an email (already normalised),
// or ErrNotFound.
func (q *queries) GetPreapprovalByEmail(ctx context.Context, email string) (Preapproval, error) {
	p, err := scanPreapproval(q.db.QueryRowContext(ctx, `SELECT `+preapprovalCols+` FROM preapprovals WHERE email = ?`, email))
	return p, mapErr(err)
}

// UpsertPreapproval adds an entry, or replaces the roles of the entry with the
// same email; the existing entry keeps its id, creator, creation time and use
// record. created reports whether a row was added.
func (q *queries) UpsertPreapproval(ctx context.Context, email string, roles []string, by int64, now time.Time) (created bool, err error) {
	rj, err := json.Marshal(roles)
	if err != nil {
		return false, err
	}
	err = q.atomic(ctx, func(q *queries) error {
		res, err := q.db.ExecContext(ctx, `UPDATE preapprovals SET roles_json = ? WHERE email = ?`, string(rj), email)
		if err != nil {
			return mapErr(err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			return nil
		}
		created = true
		_, err = q.db.ExecContext(ctx, `INSERT INTO preapprovals (email, roles_json, created_by, created_at)
			VALUES (?, ?, ?, ?)`, email, string(rj), by, FormatTime(now))
		return mapErr(err)
	})
	return created, err
}

// DeletePreapproval removes one entry and returns it, or ErrNotFound.
func (q *queries) DeletePreapproval(ctx context.Context, id int64) (Preapproval, error) {
	var p Preapproval
	err := q.atomic(ctx, func(q *queries) error {
		var err error
		p, err = scanPreapproval(q.db.QueryRowContext(ctx, `SELECT `+preapprovalCols+` FROM preapprovals WHERE id = ?`, id))
		if err != nil {
			return mapErr(err)
		}
		_, err = q.db.ExecContext(ctx, `DELETE FROM preapprovals WHERE id = ?`, id)
		return err
	})
	return p, err
}

// MarkPreapprovalUsed records the first use of an entry. An entry that was
// already used keeps its first use; marked reports whether this call set it.
func (q *queries) MarkPreapprovalUsed(ctx context.Context, id, userID int64, now time.Time) (marked bool, err error) {
	res, err := q.db.ExecContext(ctx, `UPDATE preapprovals SET used_at = ?, used_by = ?
		WHERE id = ? AND used_at IS NULL`, FormatTime(now), userID, id)
	if err != nil {
		return false, mapErr(err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
