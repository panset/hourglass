// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/store"
)

// Handlers of the service-account admin routes (docs/design/integration-api.md,
// "Admin routes for service accounts", SVC-4, SVC-6). They run behind the
// admin chain (admin role, password guard, CSRF) and write one audit row each,
// in the same transaction as the change.
//
// Audit actions:
//
//	service_account.create        POST service-accounts
//	service_account.update        PUT service-accounts/{id}
//	service_account.delete        DELETE service-accounts/{id}
//	service_account.token_issue   POST service-accounts/{id}/tokens
//	service_account.token_revoke  DELETE service-accounts/{id}/tokens/{tokenId}
//
// The token itself appears only in the body of the issue response. Audit rows
// carry the token id and expiry, never the token, its secret or its hash.
const (
	auditServiceCreate      = "service_account.create"
	auditServiceUpdate      = "service_account.update"
	auditServiceDelete      = "service_account.delete"
	auditServiceTokenIssue  = "service_account.token_issue"
	auditServiceTokenRevoke = "service_account.token_revoke"
)

// Input limits for service accounts (D47).
const (
	maxServiceNameLen        = 64
	maxServiceDescriptionLen = 500
)

var serviceNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// serviceTokenView is one token in a listing: no secret, no hash.
type serviceTokenView struct {
	ID         int64   `json:"id"`
	CreatedAt  string  `json:"created_at"`
	ExpiresAt  *string `json:"expires_at"`
	LastUsedAt *string `json:"last_used_at"`
}

// serviceAccountView is the ServiceAccount schema.
type serviceAccountView struct {
	ID          int64              `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Scopes      []string           `json:"scopes"`
	Enabled     bool               `json:"enabled"`
	CreatedAt   string             `json:"created_at"`
	Tokens      []serviceTokenView `json:"tokens"`
}

func tokenView(t store.ServiceToken) serviceTokenView {
	return serviceTokenView{ID: t.ID, CreatedAt: ts(t.CreatedAt), ExpiresAt: tsPtr(t.ExpiresAt), LastUsedAt: tsPtr(t.LastUsedAt)}
}

func accountView(a store.ServiceAccount, toks []store.ServiceToken) serviceAccountView {
	v := serviceAccountView{ID: a.ID, Name: a.Name, Description: a.Description, Scopes: orderScopes(a.Scopes),
		Enabled: a.Enabled, CreatedAt: ts(a.CreatedAt), Tokens: []serviceTokenView{}}
	for _, t := range toks {
		v.Tokens = append(v.Tokens, tokenView(t))
	}
	return v
}

// serviceAudit is the audit snapshot of an account.
func serviceAudit(a store.ServiceAccount) map[string]any {
	return map[string]any{"name": a.Name, "description": a.Description, "scopes": orderScopes(a.Scopes), "enabled": a.Enabled}
}

// orderScopes returns the scopes in the design's order, without duplicates.
func orderScopes(in []string) []string {
	out := []string{}
	for _, s := range auth.Scopes() {
		if hasString(in, s) {
			out = append(out, s)
		}
	}
	return out
}

// parseScopes validates a scope list: every entry one of the four scopes
// (unknown is 400 validation, field "scopes").
func parseScopes(in []string) ([]string, error) {
	for _, s := range in {
		if !auth.ValidScope(s) {
			return nil, adminInvalid("scopes", "unknown scope %q; valid scopes are %s", s, strings.Join(auth.Scopes(), ", "))
		}
	}
	return orderScopes(in), nil
}

func parseDescription(p *string) (string, error) {
	if p == nil {
		return "", nil
	}
	d := strings.TrimSpace(*p)
	if utf8.RuneCountInString(d) > maxServiceDescriptionLen {
		return "", adminInvalid("description", "description must be at most %d characters", maxServiceDescriptionLen)
	}
	return d, nil
}

// pathInt parses a named positive integer path value (400 validation otherwise).
func pathInt(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, name+" must be a positive integer", name)
		return 0, false
	}
	return id, true
}

// listServiceAccounts is GET /api/v1/admin/service-accounts.
func (a *API) listServiceAccounts(w http.ResponseWriter, r *http.Request) {
	list, err := a.d.Store.ListServiceAccounts(r.Context())
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	out := []serviceAccountView{}
	for _, x := range list {
		out = append(out, accountView(x.ServiceAccount, x.Tokens))
	}
	writeJSON(w, http.StatusOK, map[string]any{"service_accounts": out})
}

type createServiceAccountRequest struct {
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Scopes      *[]string `json:"scopes"`
}

// createServiceAccount is POST /api/v1/admin/service-accounts. A new account
// is enabled and has no token.
func (a *API) createServiceAccount(w http.ResponseWriter, r *http.Request) {
	caller, ok := adminCaller(w, r)
	if !ok {
		return
	}
	var req createServiceAccountRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	var err error
	switch {
	case name == "":
		err = adminInvalid("name", "name is required")
	case len(name) > maxServiceNameLen || !serviceNameRE.MatchString(name):
		err = adminInvalid("name", "name must be 1 to %d letters, digits, '.', '_' or '-', starting with a letter or digit", maxServiceNameLen)
	case req.Scopes == nil:
		err = adminInvalid("scopes", "scopes is required (it may be empty)")
	}
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	desc, err := parseDescription(req.Description)
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	scopes, err := parseScopes(*req.Scopes)
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	ctx := r.Context()
	now := a.d.Clock.Now()
	var acct store.ServiceAccount
	err = a.d.Store.Tx(ctx, func(tx *store.Tx) error {
		by := caller.ID
		var err error
		acct, err = tx.CreateServiceAccount(ctx, store.ServiceAccount{Name: name, Description: desc, Scopes: scopes,
			Enabled: true, CreatedBy: &by}, now)
		if errors.Is(err, store.ErrConflict) {
			return adminRefuse(apierr.Conflict, "a service account with this name already exists")
		}
		if err != nil {
			return err
		}
		return putAudit(ctx, tx, caller.ID, auditServiceCreate, targetID("service_account", acct.ID), nil, serviceAudit(acct), now)
	})
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, accountView(acct, nil))
}

type updateServiceAccountRequest struct {
	Description *string   `json:"description"`
	Scopes      *[]string `json:"scopes"`
	Enabled     *bool     `json:"enabled"`
}

// updateServiceAccount is PUT /api/v1/admin/service-accounts/{id}: a full
// replacement of description, scopes and enabled (all three required). The
// name cannot change. A disabled account's tokens are refused from the next
// request on (SVC-5).
func (a *API) updateServiceAccount(w http.ResponseWriter, r *http.Request) {
	caller, ok := adminCaller(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req updateServiceAccountRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	var err error
	switch {
	case req.Description == nil:
		err = adminInvalid("description", "description is required")
	case req.Scopes == nil:
		err = adminInvalid("scopes", "scopes is required (it may be empty)")
	case req.Enabled == nil:
		err = adminInvalid("enabled", "enabled is required")
	}
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	desc, err := parseDescription(req.Description)
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	scopes, err := parseScopes(*req.Scopes)
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	ctx := r.Context()
	now := a.d.Clock.Now()
	err = a.d.Store.Tx(ctx, func(tx *store.Tx) error {
		old, err := tx.GetServiceAccount(ctx, id)
		if err != nil {
			return err
		}
		next, err := tx.UpdateServiceAccount(ctx, id, desc, scopes, *req.Enabled)
		if err != nil {
			return err
		}
		return putAudit(ctx, tx, caller.ID, auditServiceUpdate, targetID("service_account", id), serviceAudit(old), serviceAudit(next), now)
	})
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	a.writeServiceAccount(w, r, http.StatusOK, id)
}

// deleteServiceAccount is DELETE /api/v1/admin/service-accounts/{id}. Its
// tokens go with it; holds and audit rows keep the name as text.
func (a *API) deleteServiceAccount(w http.ResponseWriter, r *http.Request) {
	caller, ok := adminCaller(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	now := a.d.Clock.Now()
	err := a.d.Store.Tx(ctx, func(tx *store.Tx) error {
		old, err := tx.GetServiceAccount(ctx, id)
		if err != nil {
			return err
		}
		if err := tx.DeleteServiceAccount(ctx, id); err != nil {
			return err
		}
		return putAudit(ctx, tx, caller.ID, auditServiceDelete, targetID("service_account", id), serviceAudit(old), nil, now)
	})
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

type issueServiceTokenRequest struct {
	ExpiresAt *string `json:"expires_at"`
}

// issuedTokenView is the IssuedServiceToken schema: the only response that
// ever carries a token (SVC-4).
type issuedTokenView struct {
	serviceTokenView
	Token string `json:"token"`
}

// issueServiceToken is POST /api/v1/admin/service-accounts/{id}/tokens. The
// body is optional; expires_at, when given, must be in the future.
func (a *API) issueServiceToken(w http.ResponseWriter, r *http.Request) {
	caller, ok := adminCaller(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req issueServiceTokenRequest
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBodyBytes+1))
	switch {
	case err != nil:
		apierr.Write(w, http.StatusBadRequest, apierr.Validation, "request body could not be read")
		return
	case len(bytes.TrimSpace(raw)) > 0:
		r.Body = io.NopCloser(bytes.NewReader(raw))
		if !decodeJSON(w, r, &req) {
			return
		}
	}
	now := a.d.Clock.Now()
	var expires *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339Nano, *req.ExpiresAt)
		switch {
		case err != nil:
			apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "expires_at must be an RFC 3339 timestamp", "expires_at")
			return
		case !t.After(now):
			apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "expires_at must be in the future", "expires_at")
			return
		}
		t = t.UTC()
		expires = &t
	}
	secret, hash, err := auth.NewServiceSecret()
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	ctx := r.Context()
	var tok store.ServiceToken
	err = a.d.Store.Tx(ctx, func(tx *store.Tx) error {
		if _, err := tx.GetServiceAccount(ctx, id); err != nil {
			return err
		}
		var err error
		if tok, err = tx.InsertServiceToken(ctx, id, hash, expires, now); err != nil {
			return err
		}
		return putAudit(ctx, tx, caller.ID, auditServiceTokenIssue, targetID("service_account", id), nil,
			map[string]any{"token_id": tok.ID, "expires_at": tsPtr(tok.ExpiresAt)}, now)
	})
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, issuedTokenView{serviceTokenView: tokenView(tok), Token: auth.FormatServiceToken(tok.ID, secret)})
}

// revokeServiceToken is DELETE /api/v1/admin/service-accounts/{id}/tokens/{tokenId}.
// The token is refused from the next request on (SVC-5).
func (a *API) revokeServiceToken(w http.ResponseWriter, r *http.Request) {
	caller, ok := adminCaller(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	tokenID, ok := pathInt(w, r, "tokenId")
	if !ok {
		return
	}
	ctx := r.Context()
	now := a.d.Clock.Now()
	err := a.d.Store.Tx(ctx, func(tx *store.Tx) error {
		if err := tx.RevokeServiceToken(ctx, id, tokenID); err != nil {
			return err
		}
		return putAudit(ctx, tx, caller.ID, auditServiceTokenRevoke, targetID("service_account", id),
			map[string]any{"token_id": tokenID}, nil, now)
	})
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// writeServiceAccount answers with one account and its tokens.
func (a *API) writeServiceAccount(w http.ResponseWriter, r *http.Request, status int, id int64) {
	v, err := a.serviceAccountByID(r.Context(), id)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, status, v)
}

func (a *API) serviceAccountByID(ctx context.Context, id int64) (serviceAccountView, error) {
	list, err := a.d.Store.ListServiceAccounts(ctx)
	if err != nil {
		return serviceAccountView{}, err
	}
	for _, x := range list {
		if x.ID == id {
			return accountView(x.ServiceAccount, x.Tokens), nil
		}
	}
	return serviceAccountView{}, store.ErrNotFound
}
