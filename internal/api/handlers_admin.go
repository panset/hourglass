// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/rules"
	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// Handlers of the admin routes. Each runs behind the admin middleware chain
// (admin role, password guard, CSRF). Business rules live in internal/ledger,
// internal/settings, internal/rules and the store; the handlers validate
// input, run one transaction with its audit row (ADM-3) and shape the reply.
//
// Audit actions (the vocabulary of the audit log):
//
//	user.roles        PUT users/{id}/roles
//	user.reject       POST users/{id}/reject
//	ledger.grant      POST users/{id}/grants   (written by internal/ledger)
//	ledger.reset      POST users/{id}/reset    (written by internal/ledger)
//	setup.create, setup.update, setup.delete
//	rules.replace     PUT rules
//	settings.update   PUT settings
//	hold.force_release  written by internal/holds

// ---- users ----

// listAdminUsers is GET /api/v1/admin/users (ADM-1, ADM-5).
func (a *API) listAdminUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := r.URL.Query().Get("status")
	switch status {
	case "", store.StatusPending, store.StatusActive, store.StatusRejected:
	default:
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "status must be pending, active or rejected", "status")
		return
	}
	s, err := a.loadAdminState(ctx, 0, status)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	all, err := a.d.Store.ListUsers(ctx, "")
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	totals, err := a.d.Store.LedgerPeriodTotals(ctx, s.period)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	type summary struct {
		UserCount      int     `json:"user_count"`
		AdminCount     int     `json:"admin_count"`
		OverdrawnCount int     `json:"overdrawn_count"`
		Debited        float64 `json:"debited_this_month_tokens"`
		Granted        float64 `json:"granted_this_month_tokens"`
		GrantCount     int     `json:"grant_count_this_month"`
	}
	overdrawn, err := a.d.Store.CountOverdrawn(ctx)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	sum := summary{OverdrawnCount: overdrawn, Debited: tokens(totals.DebitedMT), Granted: tokens(totals.GrantedMT), GrantCount: totals.GrantCount}
	pending := 0
	users := []adminUserView{}
	for _, u := range all {
		switch u.Status {
		case store.StatusPending:
			pending++
		case store.StatusActive:
			sum.UserCount++
		}
		for _, role := range s.roles[u.ID] {
			if role == store.RoleAdmin {
				sum.AdminCount++
			}
		}
		if status == "" || u.Status == status {
			users = append(users, s.userView(u))
		}
	}
	sortUsers(users, status == store.StatusPending)
	writeJSON(w, http.StatusOK, struct {
		Users        []adminUserView `json:"users"`
		PendingCount int             `json:"pending_count"`
		Summary      summary         `json:"summary"`
		Period       string          `json:"period"`
		PeriodEndsAt string          `json:"period_ends_at"`
		Allowance    float64         `json:"default_allowance_tokens"`
	}{users, pending, sum, s.period, ts(ledger.NextBoundary(s.now, a.d.Location)), tokens(s.cfg.DefaultAllowanceMT)})
}

// setUserRoles is PUT /api/v1/admin/users/{id}/roles. It replaces the role
// set; a non-empty set approves a pending or rejected user, credits the
// allowance once per period through the ledger service and emits
// user.approved the first time the user is approved.
func (a *API) setUserRoles(w http.ResponseWriter, r *http.Request) {
	caller, ok := adminCaller(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Roles *[]string `json:"roles"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Roles == nil {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "roles is required", "roles")
		return
	}
	roles, err := normaliseRoles(*body.Roles)
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	ctx := r.Context()
	now := a.d.Clock.Now()
	var approved *hourglass.Event
	err = a.d.Store.Tx(ctx, func(tx *store.Tx) error {
		u, err := tx.GetUser(ctx, id)
		if err != nil {
			return err
		}
		if auth.IsLocalAdmin(u) {
			return adminRefuse(apierr.LocalAdminProtected, "the local admin's roles cannot be changed")
		}
		before, err := tx.GetUserRoles(ctx, id)
		if err != nil {
			return err
		}
		hadAdmin, willHaveAdmin := hasString(before, store.RoleAdmin), hasString(roles, store.RoleAdmin)
		if id == caller.ID && hadAdmin && !willHaveAdmin {
			return adminRefuse(apierr.SelfDemote, "you cannot remove your own admin role")
		}
		if hadAdmin && !willHaveAdmin {
			n, err := tx.CountUsersWithRole(ctx, store.RoleAdmin)
			if err != nil {
				return err
			}
			if n <= 1 {
				return adminRefuse(apierr.LastAdmin, "at least one admin must remain")
			}
		}
		if len(roles) == 0 {
			n, err := tx.CountActiveHolds(ctx, id)
			if err != nil {
				return err
			}
			if n > 0 {
				return adminRefuse(apierr.HasActiveHold, "the user holds a setup; force-release it first")
			}
		}
		if err := tx.SetUserRoles(ctx, id, roles); err != nil {
			return err
		}
		statusAfter := u.Status
		if len(roles) > 0 {
			if a.d.Ledger == nil {
				return errors.New("api: ledger service is not configured")
			}
			if u.Status != store.StatusActive {
				if err := tx.SetUserStatus(ctx, id, store.StatusActive); err != nil {
					return err
				}
				statusAfter = store.StatusActive
				approved = &hourglass.Event{Type: hourglass.EventUserApproved, Time: now,
					User: &hourglass.User{ID: u.ID, Username: u.Username, Email: u.Email, Name: u.Name, Roles: roles}}
			}
			if err := a.d.Ledger.EnsureFirstAllowanceTx(tx, id, now); err != nil {
				return err
			}
		}
		return putAudit(ctx, tx, caller.ID, auditUserRoles, targetID("user", id),
			map[string]any{"roles": nonNil(before), "status": u.Status},
			map[string]any{"roles": roles, "status": statusAfter}, now)
	})
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	if approved != nil && a.d.Events != nil {
		a.d.Events.Emit(*approved)
	}
	a.writeUser(w, r, http.StatusOK, id)
}

// rejectUser is POST /api/v1/admin/users/{id}/reject (ADM-5).
func (a *API) rejectUser(w http.ResponseWriter, r *http.Request) {
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
		u, err := tx.GetUser(ctx, id)
		if err != nil {
			return err
		}
		if auth.IsLocalAdmin(u) {
			return adminRefuse(apierr.LocalAdminProtected, "the local admin cannot be rejected")
		}
		if u.Status != store.StatusPending {
			return adminRefuse(apierr.Conflict, "only a pending request can be rejected")
		}
		if err := tx.SetUserStatus(ctx, id, store.StatusRejected); err != nil {
			return err
		}
		return putAudit(ctx, tx, caller.ID, auditUserReject, targetID("user", id),
			map[string]any{"status": u.Status}, map[string]any{"status": store.StatusRejected}, now)
	})
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	a.writeUser(w, r, http.StatusOK, id)
}

// grantTokens is POST /api/v1/admin/users/{id}/grants (LED-4).
func (a *API) grantTokens(w http.ResponseWriter, r *http.Request) {
	caller, ok := adminCaller(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Amount *float64 `json:"amount_tokens"`
		Reason *string  `json:"reason"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Amount == nil {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "amount_tokens is required", "amount_tokens")
		return
	}
	mt, err := milliFromTokens(*body.Amount)
	if err != nil {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "amount_tokens is out of range", "amount_tokens")
		return
	}
	if mt == 0 {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "amount_tokens must not be zero", "amount_tokens")
		return
	}
	reason := ""
	if body.Reason != nil {
		reason = strings.TrimSpace(*body.Reason)
	}
	if reason == "" {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "reason is required", "reason")
		return
	}
	if a.d.Ledger == nil {
		a.writeError(w, r, errors.New("api: ledger service is not configured"))
		return
	}
	if _, err := a.d.Ledger.Grant(r.Context(), caller.ID, id, mt, reason); err != nil {
		a.writeError(w, r, err)
		return
	}
	a.writeBalanceChange(w, r, caller, id, string(ledger.TypeGrant))
}

// resetBalance is POST /api/v1/admin/users/{id}/reset (LED-5).
func (a *API) resetBalance(w http.ResponseWriter, r *http.Request) {
	caller, ok := adminCaller(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Reason *string `json:"reason"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	reason := ""
	if body.Reason != nil {
		reason = strings.TrimSpace(*body.Reason)
	}
	if reason == "" {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "reason is required", "reason")
		return
	}
	if a.d.Ledger == nil {
		a.writeError(w, r, errors.New("api: ledger service is not configured"))
		return
	}
	if _, err := a.d.Ledger.Reset(r.Context(), caller.ID, id, reason); err != nil {
		a.writeError(w, r, err)
		return
	}
	a.writeBalanceChange(w, r, caller, id, string(ledger.TypeReset))
}

// writeBalanceChange answers 201 with the entry just written and the user.
func (a *API) writeBalanceChange(w http.ResponseWriter, r *http.Request, caller *auth.User, userID int64, typ string) {
	ctx := r.Context()
	e, err := a.d.Store.LatestLedgerEntry(ctx, userID, typ, caller.ID)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	after, err := a.d.Store.BalanceThrough(ctx, userID, e.Period, e.ID)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	uv, err := a.userViewByID(ctx, userID)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	type entry struct {
		ID           int64    `json:"id"`
		CreatedAt    string   `json:"created_at"`
		Period       string   `json:"period"`
		Type         string   `json:"type"`
		Amount       float64  `json:"amount_tokens"`
		BalanceAfter float64  `json:"balance_after_tokens"`
		Multiplier   *float64 `json:"multiplier"`
		SetupName    *string  `json:"setup_name"`
		HoldID       *int64   `json:"hold_id"`
		ActorName    *string  `json:"actor_name"`
		Reason       *string  `json:"reason"`
	}
	writeJSON(w, http.StatusCreated, struct {
		Entry entry         `json:"entry"`
		User  adminUserView `json:"user"`
	}{entry{ID: e.ID, CreatedAt: ts(e.CreatedAt), Period: e.Period, Type: e.Type, Amount: tokens(e.AmountMT),
		BalanceAfter: tokens(after), HoldID: e.HoldID, ActorName: nullIfEmpty(caller.Name), Reason: nullIfEmpty(e.Reason)}, uv})
}

func (a *API) writeUser(w http.ResponseWriter, r *http.Request, status int, id int64) {
	uv, err := a.userViewByID(r.Context(), id)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, status, uv)
}

func normaliseRoles(in []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, r := range in {
		if r != store.RoleDeveloper && r != store.RoleAdmin {
			return nil, adminInvalid("roles", "unknown role %q (valid: developer, admin)", r)
		}
		if seen[r] {
			return nil, adminInvalid("roles", "duplicate role %q", r)
		}
		seen[r] = true
		out = append(out, r)
	}
	sort.Strings(out)
	return out, nil
}

func hasString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// ---- setups ----

// maxDetailsBytes is the SET-2 limit.
const maxDetailsBytes = 256 * 1024

// maxSetupNameRunes bounds a setup name (the spec sets no limit).
const maxSetupNameRunes = 200

// setupInput is the validated SetupInput.
type setupInput struct {
	name     string
	labels   map[string]string
	details  string
	enabled  bool
	override string
}

// parseSetupInput reads and validates a SetupInput body (SET-1..SET-4). It
// writes the 400 itself and returns false on a bad body.
func (a *API) parseSetupInput(w http.ResponseWriter, r *http.Request) (setupInput, bool) {
	var body struct {
		Name     *string            `json:"name"`
		Labels   *map[string]string `json:"labels"`
		Details  json.RawMessage    `json:"details"`
		Enabled  *bool              `json:"enabled"`
		Override *string            `json:"resource_name_override"`
	}
	if !decodeJSON(w, r, &body) {
		return setupInput{}, false
	}
	bad := func(field, format string, args ...any) (setupInput, bool) {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, fmt.Sprintf(format, args...), field)
		return setupInput{}, false
	}
	if body.Name == nil {
		return bad("name", "name is required")
	}
	name := strings.TrimSpace(*body.Name)
	switch {
	case name == "":
		return bad("name", "name must not be empty")
	case utf8.RuneCountInString(name) > maxSetupNameRunes:
		return bad("name", "name is longer than %d characters", maxSetupNameRunes)
	case strings.IndexFunc(name, unicode.IsControl) >= 0:
		return bad("name", "name must not contain control characters")
	}
	if body.Labels == nil {
		return bad("labels", "labels is required (use {} for none)")
	}
	if err := rules.ValidateLabels(*body.Labels); err != nil {
		return bad("labels", "%v", err)
	}
	if len(body.Details) == 0 {
		return bad("details", "details is required")
	}
	if len(body.Details) > maxDetailsBytes {
		return bad("details", "details is %d bytes (at most %d)", len(body.Details), maxDetailsBytes)
	}
	if !utf8.Valid(body.Details) {
		return bad("details", "details must be valid UTF-8")
	}
	if body.Enabled == nil {
		return bad("enabled", "enabled is required")
	}
	override := ""
	if body.Override != nil {
		override = strings.TrimSpace(*body.Override)
	}
	if utf8.RuneCountInString(override) > 255 || strings.IndexFunc(override, unicode.IsControl) >= 0 {
		return bad("resource_name_override", "resource_name_override is not a valid resource name")
	}
	cfg, err := settings.Load(r.Context(), a.d.Store)
	if err != nil {
		a.writeError(w, r, err)
		return setupInput{}, false
	}
	if _, err := settings.ResourceName(cfg.ResourceNameTemplate, name, *body.Labels, override); err != nil {
		return bad("resource_name_override", "the resource name cannot be derived: %v", err)
	}
	return setupInput{name: name, labels: *body.Labels, details: string(body.Details), enabled: *body.Enabled, override: override}, true
}

// setupAudit is the audit snapshot of a setup. details is recorded as a size
// and a SHA-256 only: it is opaque, up to 256 KB, and may hold anything.
func setupAudit(s store.Setup) map[string]any {
	sum := sha256.Sum256([]byte(s.DetailsJSON))
	return map[string]any{"name": s.Name, "labels": s.Labels, "enabled": s.Enabled,
		"resource_name_override": nullIfEmpty(s.ResourceNameOverride),
		"details_bytes":          len(s.DetailsJSON), "details_sha256": hex.EncodeToString(sum[:])}
}

const msgNameTaken = "a setup with this name already exists"

// createSetup is POST /api/v1/admin/setups (SET-1..SET-4).
func (a *API) createSetup(w http.ResponseWriter, r *http.Request) {
	caller, ok := adminCaller(w, r)
	if !ok {
		return
	}
	in, ok := a.parseSetupInput(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	now := a.d.Clock.Now()
	var id int64
	err := a.d.Store.Tx(ctx, func(tx *store.Tx) error {
		s := store.Setup{Name: in.name, Labels: in.labels, DetailsJSON: in.details, Enabled: in.enabled, ResourceNameOverride: in.override}
		var err error
		if id, err = tx.CreateSetup(ctx, s); err != nil {
			if errors.Is(err, store.ErrConflict) {
				return adminRefuse(apierr.Conflict, msgNameTaken)
			}
			return err
		}
		s.ID = id
		return putAudit(ctx, tx, caller.ID, auditSetupCreate, targetID("setup", id), nil, setupAudit(s), now)
	})
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	a.writeSetup(w, r, caller, http.StatusCreated, id)
}

// updateSetup is PUT /api/v1/admin/setups/{id} (SET-1, SET-5).
func (a *API) updateSetup(w http.ResponseWriter, r *http.Request) {
	caller, ok := adminCaller(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	in, ok := a.parseSetupInput(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	now := a.d.Clock.Now()
	err := a.d.Store.Tx(ctx, func(tx *store.Tx) error {
		old, err := tx.GetSetup(ctx, id)
		if err != nil {
			return err
		}
		if old.Enabled && !in.enabled {
			if err := noActiveHold(ctx, tx, id, "a setup with an active hold cannot be disabled; force-release it first"); err != nil {
				return err
			}
		}
		s := store.Setup{ID: id, Name: in.name, Labels: in.labels, DetailsJSON: in.details, Enabled: in.enabled, ResourceNameOverride: in.override}
		if err := tx.UpdateSetup(ctx, s); err != nil {
			if errors.Is(err, store.ErrConflict) {
				return adminRefuse(apierr.Conflict, msgNameTaken)
			}
			return err
		}
		return putAudit(ctx, tx, caller.ID, auditSetupUpdate, targetID("setup", id), setupAudit(old), setupAudit(s), now)
	})
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	a.writeSetup(w, r, caller, http.StatusOK, id)
}

// deleteSetup is DELETE /api/v1/admin/setups/{id} (SET-1, SET-5). The schema
// keeps holds (and the ledger and usage rows that point at them) for good, so
// a setup that was ever held cannot be deleted: it answers 409 conflict and
// tells the admin to disable it instead.
func (a *API) deleteSetup(w http.ResponseWriter, r *http.Request) {
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
		old, err := tx.GetSetup(ctx, id)
		if err != nil {
			return err
		}
		if err := noActiveHold(ctx, tx, id, "a setup with an active hold cannot be deleted; force-release it first"); err != nil {
			return err
		}
		n, err := tx.CountHoldsForSetup(ctx, id)
		if err != nil {
			return err
		}
		if n > 0 {
			return adminRefuse(apierr.Conflict, "this setup has hold history and cannot be deleted; disable it instead")
		}
		if err := tx.DeleteSetup(ctx, id); err != nil {
			if errors.Is(err, store.ErrConflict) {
				return adminRefuse(apierr.Conflict, "this setup has hold history and cannot be deleted; disable it instead")
			}
			return err
		}
		return putAudit(ctx, tx, caller.ID, auditSetupDelete, targetID("setup", id), setupAudit(old), nil, now)
	})
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func noActiveHold(ctx context.Context, tx *store.Tx, setupID int64, msg string) error {
	_, err := tx.ActiveHoldForSetup(ctx, setupID)
	switch {
	case err == nil:
		return adminRefuse(apierr.HasActiveHold, msg)
	case errors.Is(err, store.ErrNotFound):
		return nil
	default:
		return err
	}
}

// writeSetup answers with the admin view of a setup.
func (a *API) writeSetup(w http.ResponseWriter, r *http.Request, caller *auth.User, status int, id int64) {
	ctx := r.Context()
	x, err := a.d.Store.GetSetup(ctx, id)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	s, err := a.loadAdminState(ctx, noLedgerStats, "")
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	v, err := a.setupView(ctx, s, x, caller.ID)
	if err == nil {
		err = writeSetupJSON(w, status, v)
	}
	if err != nil {
		a.writeError(w, r, err)
	}
}

// ---- rules ----

type ruleOut struct {
	ID         int64   `json:"id"`
	Position   int     `json:"position"`
	Selector   string  `json:"selector"`
	Weight     float64 `json:"weight"`
	MatchCount int     `json:"match_count"`
}

type setupRef struct {
	ID     int64             `json:"id"`
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
}

// rulesResponse builds the RulesResponse: ordered rules with first-match
// counts over the enabled setups, the default weight and the fall-through
// setups (RULE-3).
func (a *API) rulesResponse(r *http.Request) (any, error) {
	ctx := r.Context()
	rows, err := a.d.Store.ListRules(ctx, rules.KindRate)
	if err != nil {
		return nil, err
	}
	cfg, err := settings.Load(ctx, a.d.Store)
	if err != nil {
		return nil, err
	}
	setups, err := a.d.Store.ListSetups(ctx, true)
	if err != nil {
		return nil, err
	}
	sets := make([]map[string]string, len(setups))
	for i, s := range setups {
		sets[i] = s.Labels
	}
	counts, unmatched := rules.MatchCounts(toRulesRules(rows), rules.KindRate, sets)
	out := make([]ruleOut, len(rows))
	for i, x := range rows {
		out[i] = ruleOut{ID: x.ID, Position: i + 1, Selector: x.Selector, Weight: x.Value, MatchCount: counts[x.ID]}
	}
	refs := []setupRef{}
	for _, idx := range unmatched {
		s := setups[idx]
		refs = append(refs, setupRef{ID: s.ID, Name: s.Name, Labels: s.Labels})
	}
	return struct {
		Rules     []ruleOut  `json:"rules"`
		Default   float64    `json:"default_weight"`
		Unmatched []setupRef `json:"unmatched_setups"`
	}{out, cfg.DefaultWeight, refs}, nil
}

// getRules is GET /api/v1/admin/rules (RULE-3).
func (a *API) getRules(w http.ResponseWriter, r *http.Request) {
	resp, err := a.rulesResponse(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// replaceRules is PUT /api/v1/admin/rules (RULE-1, RULE-3, RULE-4). The rules
// are read fresh by every poll, so the change applies from the next poll.
func (a *API) replaceRules(w http.ResponseWriter, r *http.Request) {
	caller, ok := adminCaller(w, r)
	if !ok {
		return
	}
	var body struct {
		Rules *[]struct {
			Selector *string  `json:"selector"`
			Weight   *float64 `json:"weight"`
		} `json:"rules"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Rules == nil {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "rules is required", "rules")
		return
	}
	next := make([]store.Rule, len(*body.Rules))
	for i, in := range *body.Rules {
		sel := fmt.Sprintf("rules[%d].selector", i)
		if in.Selector == nil {
			apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "selector is required", sel)
			return
		}
		if _, err := rules.Parse(*in.Selector); err != nil {
			apierr.WriteField(w, http.StatusBadRequest, apierr.InvalidSelector, err.Error(), sel)
			return
		}
		wf := fmt.Sprintf("rules[%d].weight", i)
		if in.Weight == nil {
			apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "weight is required", wf)
			return
		}
		if *in.Weight < 0 || *in.Weight > maxTokens {
			apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "weight must be between 0 and 1e12", wf)
			return
		}
		next[i] = store.Rule{Kind: rules.KindRate, Selector: *in.Selector, Value: *in.Weight}
	}
	ctx := r.Context()
	now := a.d.Clock.Now()
	err := a.d.Store.Tx(ctx, func(tx *store.Tx) error {
		old, err := tx.ListRules(ctx, rules.KindRate)
		if err != nil {
			return err
		}
		if err := tx.ReplaceRules(ctx, rules.KindRate, next, &caller.ID, now); err != nil {
			return err
		}
		return putAudit(ctx, tx, caller.ID, auditRulesReplace, "rules:rate", ruleSnapshot(old), ruleSnapshot(next), now)
	})
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	a.getRules(w, r)
}

// ruleSnapshot is the audit form of a rule list (ADM-3): an object, because
// the audit schema's before and after are objects, holding the whole ordered list.
func ruleSnapshot(rs []store.Rule) map[string]any {
	out := make([]map[string]any, len(rs))
	for i, x := range rs {
		out[i] = map[string]any{"position": i + 1, "selector": x.Selector, "weight": x.Value}
	}
	return map[string]any{"rules": out}
}

// ---- settings ----

type runtimeView struct {
	Provider       *string    `json:"provider"`
	PressureSource string     `json:"pressure_source"`
	MeteringPolicy policyView `json:"metering_policy"`
	Hooks          []string   `json:"hooks"`
	OIDCIssuer     *string    `json:"oidc_issuer"`
	Timezone       string     `json:"timezone"`
}

// policyView names the metering policy. The extension interfaces have no
// describe method, so description is always null.
type policyView struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

func (a *API) runtimeView() runtimeView {
	rt := a.d.Runtime
	hooks := rt.Hooks
	if hooks == nil {
		hooks = []string{}
	}
	return runtimeView{Provider: nullIfEmpty(rt.Provider), PressureSource: rt.PressureSource,
		MeteringPolicy: policyView{Name: rt.MeteringPolicy}, Hooks: hooks, OIDCIssuer: nullIfEmpty(rt.OIDCIssuer), Timezone: a.d.Location.String()}
}

func (a *API) writeSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := settings.Load(r.Context(), a.d.Store)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Settings settings.Settings `json:"settings"`
		Runtime  runtimeView       `json:"runtime"`
	}{cfg, a.runtimeView()})
}

// getSettings is GET /api/v1/admin/settings (ADM-2).
func (a *API) getSettings(w http.ResponseWriter, r *http.Request) { a.writeSettings(w, r) }

// settingsKeys are the keys a PUT body must all carry.
var settingsKeys = []string{settings.KeyResourceNameTemplate, settings.KeyDefaultWeight, settings.KeyDisplayLabelKeys,
	settings.KeyBands, settings.KeyDefaultAllowance, settings.KeyMaxHoldsPerUser, settings.KeyReleaseRequiresTeardown,
	settings.KeyPollInterval, settings.KeyEnforcement}

// updateSettings is PUT /api/v1/admin/settings (ADM-2). The body is the whole
// Settings object; changes apply from the next poll.
func (a *API) updateSettings(w http.ResponseWriter, r *http.Request) {
	caller, ok := adminCaller(w, r)
	if !ok {
		return
	}
	var raw json.RawMessage
	if !decodeJSON(w, r, &raw) {
		return
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil || keys == nil {
		apierr.Write(w, http.StatusBadRequest, apierr.Validation, "request body must be a JSON object")
		return
	}
	for _, k := range settingsKeys {
		if _, ok := keys[k]; !ok {
			apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, k+" is required", k)
			return
		}
	}
	var next settings.Settings
	if err := json.Unmarshal(raw, &next); err != nil {
		a.writeError(w, r, err) // *settings.FieldError names the key
		return
	}
	ctx := r.Context()
	now := a.d.Clock.Now()
	err := a.d.Store.Tx(ctx, func(tx *store.Tx) error {
		old, err := settings.Load(ctx, tx)
		if err != nil {
			return err
		}
		changes, err := settings.SaveAt(ctx, tx, old, next, caller.ID, now)
		if err != nil || len(changes) == 0 {
			return err
		}
		before, after := map[string]json.RawMessage{}, map[string]json.RawMessage{}
		for _, c := range changes {
			before[c.Key], after[c.Key] = json.RawMessage(c.Before), json.RawMessage(c.After)
		}
		return putAudit(ctx, tx, caller.ID, auditSettingsUpdate, "settings", before, after, now)
	})
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	a.writeSettings(w, r)
}

// ---- collector ----

// getCollector is GET /api/v1/admin/collector (ADM-4, HOLD-4, EXT-6, EXT-7).
func (a *API) getCollector(w http.ResponseWriter, r *http.Request) {
	resp, err := a.collectorView(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// listAudit is GET /api/v1/admin/audit (ADM-3).
func (a *API) listAudit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	bad := func(field, msg string) {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, msg, field)
	}
	f := store.AuditFilter{Action: q.Get("action"), Limit: 50}
	if v := q.Get("actor"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			bad("actor", "actor must be a user id")
			return
		}
		f.ActorID = &n
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			bad("limit", "limit must be an integer from 1 to 200")
			return
		}
		f.Limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			bad("offset", "offset must be a non-negative integer")
			return
		}
		f.Offset = n
	}
	resp, err := a.auditView(r, f)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
