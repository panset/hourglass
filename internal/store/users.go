// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"time"
)

// Auth sources, statuses and roles.
const (
	AuthOIDC  = "oidc"
	AuthLocal = "local"

	StatusPending  = "pending"
	StatusActive   = "active"
	StatusRejected = "rejected"

	RoleDeveloper = "developer"
	RoleAdmin     = "admin"
)

// User is a row of users.
type User struct {
	ID                 int64
	AuthSource         string
	OIDCSub            string // empty for local
	Username           string
	Email              string
	Name               string
	Status             string
	LabelsJSON         string
	PasswordHash       string
	MustChangePassword bool
	BalanceMT          int64
	CreatedAt          time.Time
	LastLoginAt        *time.Time
}

const userCols = `id, auth_source, oidc_sub, username, email, name, status, labels_json,
	password_hash, must_change_password, balance_mt, created_at, last_login_at`

func scanUser(r rowScanner) (User, error) {
	var (
		u                     User
		sub, uname, ph, login sql.NullString
		created               string
		must                  int
	)
	if err := r.Scan(&u.ID, &u.AuthSource, &sub, &uname, &u.Email, &u.Name, &u.Status,
		&u.LabelsJSON, &ph, &must, &u.BalanceMT, &created, &login); err != nil {
		return u, err
	}
	u.OIDCSub, u.Username, u.PasswordHash = sub.String, uname.String, ph.String
	u.MustChangePassword = must != 0
	var err error
	if u.CreatedAt, err = ParseTime(created); err != nil {
		return u, err
	}
	u.LastLoginAt, err = parseNullTime(login)
	return u, err
}

func (q *queries) getUser(ctx context.Context, where string, args ...any) (User, error) {
	u, err := scanUser(q.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE `+where, args...))
	return u, mapErr(err)
}

// CreateOIDCUser inserts a pending OIDC user with no roles.
func (q *queries) CreateOIDCUser(ctx context.Context, sub, email, name string, now time.Time) (User, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO users
		(auth_source, oidc_sub, email, name, status, created_at, last_login_at)
		VALUES ('oidc', ?, ?, ?, 'pending', ?, ?)`, sub, email, name, FormatTime(now), FormatTime(now))
	if err != nil {
		return User{}, mapErr(err)
	}
	id, _ := res.LastInsertId()
	return q.GetUser(ctx, id)
}

// UpsertOIDCLogin creates a pending user for sub if none exists, otherwise
// refreshes email, name and last_login_at. A sign-in that yields no name keeps
// the stored name (D100); a non-empty name replaces it. created reports
// whether it was new.
func (q *queries) UpsertOIDCLogin(ctx context.Context, sub, email, name string, now time.Time) (u User, created bool, err error) {
	err = q.atomic(ctx, func(q *queries) error {
		res, err := q.db.ExecContext(ctx, `UPDATE users SET email = ?,
			name = CASE WHEN ? = '' THEN name ELSE ? END, last_login_at = ?
			WHERE oidc_sub = ?`, email, name, name, FormatTime(now), sub)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			created = true
			if u, err = q.CreateOIDCUser(ctx, sub, email, name, now); err != nil {
				return err
			}
			return nil
		}
		u, err = q.GetUserBySub(ctx, sub)
		return err
	})
	return u, created, err
}

// CreateLocalAdmin inserts the local admin row: active, with the admin role.
func (q *queries) CreateLocalAdmin(ctx context.Context, username, passwordHash string, mustChange bool, now time.Time) (User, error) {
	var u User
	err := q.atomic(ctx, func(q *queries) error {
		res, err := q.db.ExecContext(ctx, `INSERT INTO users
			(auth_source, username, name, status, password_hash, must_change_password, created_at)
			VALUES ('local', ?, ?, 'active', ?, ?, ?)`,
			username, username, passwordHash, boolInt(mustChange), FormatTime(now))
		if err != nil {
			return mapErr(err)
		}
		id, _ := res.LastInsertId()
		if err := q.SetUserRoles(ctx, id, []string{RoleAdmin}); err != nil {
			return err
		}
		u, err = q.GetUser(ctx, id)
		return err
	})
	return u, err
}

// GetUser returns a user by id.
func (q *queries) GetUser(ctx context.Context, id int64) (User, error) {
	return q.getUser(ctx, `id = ?`, id)
}

// GetUserBySub returns an OIDC user by subject.
func (q *queries) GetUserBySub(ctx context.Context, sub string) (User, error) {
	return q.getUser(ctx, `oidc_sub = ?`, sub)
}

// GetUserByUsername returns a user by username.
func (q *queries) GetUserByUsername(ctx context.Context, username string) (User, error) {
	return q.getUser(ctx, `username = ? ORDER BY id LIMIT 1`, username)
}

// ListUsers lists users ordered by id; a non-empty status filters.
func (q *queries) ListUsers(ctx context.Context, status string) ([]User, error) {
	if status == "" {
		return collect(ctx, q.db, scanUser, `SELECT `+userCols+` FROM users ORDER BY id`)
	}
	return collect(ctx, q.db, scanUser, `SELECT `+userCols+` FROM users WHERE status = ? ORDER BY id`, status)
}

func (q *queries) updateUser(ctx context.Context, query string, args ...any) error {
	res, err := q.db.ExecContext(ctx, query, args...)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetUserStatus sets a user's status.
func (q *queries) SetUserStatus(ctx context.Context, id int64, status string) error {
	return q.updateUser(ctx, `UPDATE users SET status = ? WHERE id = ?`, status, id)
}

// SetPassword stores a password hash and the must-change flag.
func (q *queries) SetPassword(ctx context.Context, id int64, hash string, mustChange bool) error {
	return q.updateUser(ctx, `UPDATE users SET password_hash = ?, must_change_password = ? WHERE id = ?`,
		hash, boolInt(mustChange), id)
}

// GetUserRoles returns a user's roles, sorted.
func (q *queries) GetUserRoles(ctx context.Context, id int64) ([]string, error) {
	return collect(ctx, q.db, func(r rowScanner) (string, error) {
		var s string
		return s, r.Scan(&s)
	}, `SELECT role FROM user_roles WHERE user_id = ? ORDER BY role`, id)
}

// SetUserRoles replaces a user's role set.
func (q *queries) SetUserRoles(ctx context.Context, id int64, roles []string) error {
	return q.atomic(ctx, func(q *queries) error {
		if _, err := q.db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id = ?`, id); err != nil {
			return err
		}
		for _, r := range roles {
			if _, err := q.db.ExecContext(ctx, `INSERT INTO user_roles (user_id, role) VALUES (?, ?) ON CONFLICT DO NOTHING`, id, r); err != nil {
				return mapErr(err)
			}
		}
		return nil
	})
}
