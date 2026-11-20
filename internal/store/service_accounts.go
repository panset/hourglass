// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// ServiceAccount is a row of service_accounts.
type ServiceAccount struct {
	ID          int64
	Name        string
	Description string
	Scopes      []string
	Enabled     bool
	CreatedBy   *int64
	CreatedAt   time.Time
}

// ServiceToken is a row of service_tokens. TokenHash is the only stored form
// of the secret; the store never sees the token itself.
type ServiceToken struct {
	ID         int64
	AccountID  int64
	TokenHash  string
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
}

// ServiceAccountWithTokens is a listing row: the account and its tokens
// (oldest first). Each token carries its hash; callers must not expose it.
type ServiceAccountWithTokens struct {
	ServiceAccount
	Tokens []ServiceToken
}

// LastUsedMinInterval is the least time between two recorded last_used_at
// values of one token (TouchServiceToken).
const LastUsedMinInterval = time.Minute

const saCols = `id, name, description, scopes_json, enabled, created_by, created_at`

func scanServiceAccount(r rowScanner) (ServiceAccount, error) {
	var (
		a       ServiceAccount
		scopes  string
		en      int
		by      sql.NullInt64
		created string
		err     error
	)
	if err = r.Scan(&a.ID, &a.Name, &a.Description, &scopes, &en, &by, &created); err != nil {
		return a, err
	}
	if err = json.Unmarshal([]byte(scopes), &a.Scopes); err != nil {
		return a, err
	}
	if a.Scopes == nil {
		a.Scopes = []string{}
	}
	a.Enabled, a.CreatedBy = en != 0, nullInt64Ptr(by)
	a.CreatedAt, err = ParseTime(created)
	return a, err
}

const stCols = `id, account_id, token_hash, created_at, expires_at, last_used_at`

func scanServiceToken(r rowScanner) (ServiceToken, error) {
	var (
		t                 ServiceToken
		created           string
		expires, lastUsed sql.NullString
		err               error
	)
	if err = r.Scan(&t.ID, &t.AccountID, &t.TokenHash, &created, &expires, &lastUsed); err != nil {
		return t, err
	}
	if t.CreatedAt, err = ParseTime(created); err != nil {
		return t, err
	}
	if t.ExpiresAt, err = parseNullTime(expires); err != nil {
		return t, err
	}
	t.LastUsedAt, err = parseNullTime(lastUsed)
	return t, err
}

func scopesJSON(scopes []string) (string, error) {
	if scopes == nil {
		scopes = []string{}
	}
	b, err := json.Marshal(scopes)
	return string(b), err
}

// CreateServiceAccount inserts an account. A duplicate name returns ErrConflict.
func (q *queries) CreateServiceAccount(ctx context.Context, a ServiceAccount, now time.Time) (ServiceAccount, error) {
	sj, err := scopesJSON(a.Scopes)
	if err != nil {
		return ServiceAccount{}, err
	}
	res, err := q.db.ExecContext(ctx, `INSERT INTO service_accounts (name, description, scopes_json, enabled, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, a.Name, a.Description, sj, boolInt(a.Enabled), nullInt(a.CreatedBy), FormatTime(now))
	if err != nil {
		return ServiceAccount{}, mapErr(err)
	}
	id, _ := res.LastInsertId()
	return q.GetServiceAccount(ctx, id)
}

// GetServiceAccount returns an account by id, or ErrNotFound.
func (q *queries) GetServiceAccount(ctx context.Context, id int64) (ServiceAccount, error) {
	a, err := scanServiceAccount(q.db.QueryRowContext(ctx, `SELECT `+saCols+` FROM service_accounts WHERE id = ?`, id))
	return a, mapErr(err)
}

// GetServiceAccountByName returns an account by its unique name, or ErrNotFound.
func (q *queries) GetServiceAccountByName(ctx context.Context, name string) (ServiceAccount, error) {
	a, err := scanServiceAccount(q.db.QueryRowContext(ctx, `SELECT `+saCols+` FROM service_accounts WHERE name = ?`, name))
	return a, mapErr(err)
}

// ListServiceAccounts returns every account by name with its tokens (id,
// created, expiry, last used), in a constant number of statements.
func (q *queries) ListServiceAccounts(ctx context.Context) ([]ServiceAccountWithTokens, error) {
	var out []ServiceAccountWithTokens
	err := q.atomic(ctx, func(q *queries) error {
		accts, err := collect(ctx, q.db, scanServiceAccount, `SELECT `+saCols+` FROM service_accounts ORDER BY name`)
		if err != nil {
			return err
		}
		toks, err := collect(ctx, q.db, scanServiceToken, `SELECT `+stCols+` FROM service_tokens ORDER BY id`)
		if err != nil {
			return err
		}
		idx := make(map[int64]int, len(accts))
		out = make([]ServiceAccountWithTokens, len(accts))
		for i, a := range accts {
			out[i] = ServiceAccountWithTokens{ServiceAccount: a, Tokens: []ServiceToken{}}
			idx[a.ID] = i
		}
		for _, t := range toks {
			if i, ok := idx[t.AccountID]; ok {
				out[i].Tokens = append(out[i].Tokens, t)
			}
		}
		return nil
	})
	return out, err
}

// UpdateServiceAccount changes description, scopes and enabled. The name is
// not changeable. It returns ErrNotFound for an unknown id.
func (q *queries) UpdateServiceAccount(ctx context.Context, id int64, description string, scopes []string, enabled bool) (ServiceAccount, error) {
	sj, err := scopesJSON(scopes)
	if err != nil {
		return ServiceAccount{}, err
	}
	res, err := q.db.ExecContext(ctx, `UPDATE service_accounts SET description = ?, scopes_json = ?, enabled = ? WHERE id = ?`,
		description, sj, boolInt(enabled), id)
	if err != nil {
		return ServiceAccount{}, mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ServiceAccount{}, ErrNotFound
	}
	return q.GetServiceAccount(ctx, id)
}

// DeleteServiceAccount deletes an account; its tokens go with it (cascade).
// Holds and audit rows keep the account's name as text.
func (q *queries) DeleteServiceAccount(ctx context.Context, id int64) error {
	res, err := q.db.ExecContext(ctx, `DELETE FROM service_accounts WHERE id = ?`, id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// InsertServiceToken stores a token's hash for an account (the secret is never
// passed here). An unknown account returns ErrNotFound.
func (q *queries) InsertServiceToken(ctx context.Context, accountID int64, tokenHash string, expiresAt *time.Time, now time.Time) (ServiceToken, error) {
	res, err := q.db.ExecContext(ctx, `INSERT INTO service_tokens (account_id, token_hash, created_at, expires_at)
		VALUES (?, ?, ?, ?)`, accountID, tokenHash, FormatTime(now), nullTime(expiresAt))
	if err != nil {
		if isForeignKey(err) {
			return ServiceToken{}, ErrNotFound
		}
		return ServiceToken{}, mapErr(err)
	}
	id, _ := res.LastInsertId()
	t, err := scanServiceToken(q.db.QueryRowContext(ctx, `SELECT `+stCols+` FROM service_tokens WHERE id = ?`, id))
	return t, mapErr(err)
}

// GetServiceTokenWithAccount looks a token up by id, joined with its account,
// or returns ErrNotFound. It does not judge expiry or the enabled flag.
func (q *queries) GetServiceTokenWithAccount(ctx context.Context, tokenID int64) (ServiceToken, ServiceAccount, error) {
	row := q.db.QueryRowContext(ctx, `SELECT t.id, t.account_id, t.token_hash, t.created_at, t.expires_at, t.last_used_at,
		a.id, a.name, a.description, a.scopes_json, a.enabled, a.created_by, a.created_at
		FROM service_tokens t JOIN service_accounts a ON a.id = t.account_id WHERE t.id = ?`, tokenID)
	var (
		t                 ServiceToken
		a                 ServiceAccount
		tCreated          string
		expires, lastUsed sql.NullString
		scopes, aCreated  string
		en                int
		by                sql.NullInt64
	)
	if err := row.Scan(&t.ID, &t.AccountID, &t.TokenHash, &tCreated, &expires, &lastUsed,
		&a.ID, &a.Name, &a.Description, &scopes, &en, &by, &aCreated); err != nil {
		return t, a, mapErr(err)
	}
	var err error
	if t.CreatedAt, err = ParseTime(tCreated); err != nil {
		return t, a, err
	}
	if t.ExpiresAt, err = parseNullTime(expires); err != nil {
		return t, a, err
	}
	if t.LastUsedAt, err = parseNullTime(lastUsed); err != nil {
		return t, a, err
	}
	if err = json.Unmarshal([]byte(scopes), &a.Scopes); err != nil {
		return t, a, err
	}
	if a.Scopes == nil {
		a.Scopes = []string{}
	}
	a.Enabled, a.CreatedBy = en != 0, nullInt64Ptr(by)
	a.CreatedAt, err = ParseTime(aCreated)
	return t, a, err
}

// RevokeServiceToken deletes one token of an account. The account id is part
// of the match, so a token id under the wrong account is ErrNotFound.
func (q *queries) RevokeServiceToken(ctx context.Context, accountID, tokenID int64) error {
	res, err := q.db.ExecContext(ctx, `DELETE FROM service_tokens WHERE id = ? AND account_id = ?`, tokenID, accountID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchServiceToken records last_used_at = now, but at most once per
// LastUsedMinInterval: the rule lives here, in one conditional UPDATE, so
// every caller gets it. It reports whether a value was written. Auth calls it
// on each accepted request and ignores the bool.
func (q *queries) TouchServiceToken(ctx context.Context, tokenID int64, now time.Time) (bool, error) {
	res, err := q.db.ExecContext(ctx, `UPDATE service_tokens SET last_used_at = ?
		WHERE id = ? AND (last_used_at IS NULL OR last_used_at <= ?)`,
		FormatTime(now), tokenID, FormatTime(now.Add(-LastUsedMinInterval)))
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
