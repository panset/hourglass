// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// Pre-approved SSO users (docs/design/ledger-labels-onboarding.md, D1 to D4).
// A pre-approval is an email and a non-empty role set, from the database list
// (managed by the admin routes below) or from the read-only file overlay
// (preapproved_users_file, loaded by internal/app). The effective roles for
// an email are the union of both sources.
//
// Audit actions written here:
//
//	user.preapprove     at sign-in, no actor (shown as "system")
//	preapproval.add     POST preapprovals
//	preapproval.delete  DELETE preapprovals/{id}
const (
	auditUserPreapprove   = "user.preapprove"
	auditPreapprovalAdd   = "preapproval.add"
	auditPreapprovalDel   = "preapproval.delete"
	maxPreapprovalEmails  = 1000 // per POST (D3)
	maxPreapprovalEmailCh = 254  // characters after trimming (D3)
)

// Sources of a pre-approval, as the list and the user.preapprove audit entry
// name them.
const (
	PreapprovalSourceDatabase = "database"
	PreapprovalSourceConfig   = "config"
)

// PreapprovalEntry is one entry of the file overlay, already validated by
// ParsePreapprovalEmail and ParsePreapprovalRoles.
type PreapprovalEntry struct {
	Email string
	Roles []string
}

// NormalizeEmail is the form pre-approval emails are stored and compared in:
// surrounding whitespace trimmed, lower case. The ID token's email claim is
// compared in the same form.
func NormalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// ParsePreapprovalEmail validates one email of a pre-approval (D3) and returns
// it normalised: at most 254 characters after trimming, exactly one "@" with
// text on both sides, and no whitespace, comma or "*". The admin route and the
// file overlay use this one function.
func ParsePreapprovalEmail(s string) (string, error) {
	e := strings.TrimSpace(s)
	bad := func(why string) (string, error) {
		return "", fmt.Errorf("invalid email %q: %s", shorten(e, 80), why)
	}
	switch {
	case e == "":
		return bad("it is empty")
	case !utf8.ValidString(e):
		return bad("it is not valid UTF-8")
	case utf8.RuneCountInString(e) > maxPreapprovalEmailCh:
		return bad(fmt.Sprintf("it is longer than %d characters", maxPreapprovalEmailCh))
	case strings.ContainsFunc(e, unicode.IsSpace):
		return bad("it contains whitespace")
	case strings.ContainsAny(e, ",*"):
		return bad(`it contains "," or "*"`)
	}
	local, domain, _ := strings.Cut(e, "@")
	if strings.Count(e, "@") != 1 || local == "" || domain == "" {
		return bad(`it must have exactly one "@" with text on both sides`)
	}
	return NormalizeEmail(e), nil
}

// ParsePreapprovalRoles validates the roles of a pre-approval as
// setUserRoles validates roles, and refuses an empty set. The result is
// sorted.
func ParsePreapprovalRoles(in []string) ([]string, error) {
	roles, err := normaliseRoles(in)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, adminInvalid("roles", "roles must not be empty")
	}
	return roles, nil
}

func shorten(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "..."
}

// grantRolesTx is the part of an approval that PUT users/{id}/roles and a
// pre-approved sign-in share, inside the caller's transaction: it replaces the
// role set and, when the set is not empty, makes the user active and credits
// the period's first allowance. approved is the user.approved event to emit
// after commit, when the user was not active before.
func grantRolesTx(ctx context.Context, tx *store.Tx, led *ledger.Service, u store.User, roles []string, now time.Time) (statusAfter string, approved *hourglass.Event, err error) {
	if err := tx.SetUserRoles(ctx, u.ID, roles); err != nil {
		return "", nil, err
	}
	statusAfter = u.Status
	if len(roles) == 0 {
		return statusAfter, nil, nil
	}
	if led == nil {
		return "", nil, errors.New("api: ledger service is not configured")
	}
	if u.Status != store.StatusActive {
		if err := tx.SetUserStatus(ctx, u.ID, store.StatusActive); err != nil {
			return "", nil, err
		}
		statusAfter = store.StatusActive
		approved = &hourglass.Event{Type: hourglass.EventUserApproved, Time: now,
			User: &hourglass.User{ID: u.ID, Username: u.Username, Email: u.Email, Name: u.Name, Roles: roles}}
	}
	if err := led.EnsureFirstAllowanceTx(tx, u.ID, now); err != nil {
		return "", nil, err
	}
	return statusAfter, approved, nil
}

// PreapproverDeps is what the sign-in approval step needs.
type PreapproverDeps struct {
	Store  *store.Store
	Ledger *ledger.Service
	Events auth.Emitter // receives user.approved after commit; may be nil
	Clock  Clock
	Logger *slog.Logger
	// File is the validated file overlay; nil or empty means no file.
	File []PreapprovalEntry
}

// Preapprover applies pre-approvals at the end of an SSO sign-in. It
// implements auth.Preapprover; internal/app builds it and hands it to both
// internal/auth (the sign-in step) and the API (the file entries of the list).
type Preapprover struct {
	d    PreapproverDeps
	file map[string][]string // normalised email -> roles
}

var _ auth.Preapprover = (*Preapprover)(nil)

// NewPreapprover builds the approval step. The file entries must already be
// valid and must not repeat an email; NewPreapprover keeps its own copy.
func NewPreapprover(d PreapproverDeps) *Preapprover {
	if d.Clock == nil {
		d.Clock = systemClock{}
	}
	if d.Logger == nil {
		d.Logger = slog.New(slog.DiscardHandler)
	}
	p := &Preapprover{d: d, file: make(map[string][]string, len(d.File))}
	files := make([]PreapprovalEntry, 0, len(d.File))
	for _, e := range d.File {
		roles := append([]string(nil), e.Roles...)
		p.file[e.Email] = roles
		files = append(files, PreapprovalEntry{Email: e.Email, Roles: roles})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Email < files[j].Email })
	p.d.File = files
	return p
}

// fileEntries returns the file overlay sorted by email; nil-safe.
func (p *Preapprover) fileEntries() []PreapprovalEntry {
	if p == nil {
		return nil
	}
	return p.d.File
}

// ApplyPreapproval approves the SSO user userID when a pre-approval exists for
// email, the ID token's email claim as sent. The caller (internal/auth) has
// checked that the claim is not empty and that email_verified is not false.
// In one transaction it checks again that the user is a pending SSO user,
// sets the union of the matching entries' roles, makes the user active,
// credits the first allowance, marks an unused database entry as used and
// writes user.preapprove with no actor. user.approved is emitted after
// commit. approved is false, with no error, when nothing applied.
func (p *Preapprover) ApplyPreapproval(ctx context.Context, userID int64, email string) (approved bool, err error) {
	key := NormalizeEmail(email)
	if key == "" {
		return false, nil
	}
	now := p.d.Clock.Now()
	var ev *hourglass.Event
	err = p.d.Store.Tx(ctx, func(tx *store.Tx) error {
		u, err := tx.GetUser(ctx, userID)
		if err != nil {
			return err
		}
		// Only a pending SSO user: never the local admin, a rejected user or
		// an active one (even with no roles).
		if u.AuthSource != store.AuthOIDC || auth.IsLocalAdmin(u) || u.Status != store.StatusPending {
			return nil
		}
		var sources []string
		set := map[string]bool{}
		db, err := tx.GetPreapprovalByEmail(ctx, key)
		switch {
		case err == nil:
			sources = append(sources, PreapprovalSourceDatabase)
			for _, r := range db.Roles {
				set[r] = true
			}
		case errors.Is(err, store.ErrNotFound):
		default:
			return err
		}
		if fr, ok := p.file[key]; ok {
			sources = append(sources, PreapprovalSourceConfig)
			for _, r := range fr {
				set[r] = true
			}
		}
		if len(sources) == 0 || len(set) == 0 {
			return nil
		}
		roles := make([]string, 0, len(set))
		for r := range set {
			roles = append(roles, r)
		}
		sort.Strings(roles)
		before, err := tx.GetUserRoles(ctx, u.ID)
		if err != nil {
			return err
		}
		statusAfter, approvedEv, err := grantRolesTx(ctx, tx, p.d.Ledger, u, roles, now)
		if err != nil {
			return err
		}
		if db.ID != 0 {
			if _, err := tx.MarkPreapprovalUsed(ctx, db.ID, u.ID, now); err != nil {
				return err
			}
		}
		if err := putAuditAs(ctx, tx, nil, auditUserPreapprove, targetID("user", u.ID),
			map[string]any{"roles": nonNil(before), "status": u.Status},
			map[string]any{"roles": roles, "status": statusAfter, "sources": sources}, now); err != nil {
			return err
		}
		ev, approved = approvedEv, true
		return nil
	})
	if err != nil {
		return false, err
	}
	if ev != nil && p.d.Events != nil {
		p.d.Events.Emit(*ev)
	}
	return approved, nil
}

// ---- admin routes (D3) ----

// preapprovalUserRef names a user in the pre-approval list.
type preapprovalUserRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type preapprovalView struct {
	ID        *int64              `json:"id"`
	Email     string              `json:"email"`
	Roles     []string            `json:"roles"`
	Source    string              `json:"source"`
	CreatedAt *string             `json:"created_at"`
	CreatedBy *preapprovalUserRef `json:"created_by"`
	UsedAt    *string             `json:"used_at"`
	UsedBy    *preapprovalUserRef `json:"used_by"`
}

// listPreapprovals is GET /api/v1/admin/preapprovals (PRE-5, PRE-6): every
// entry of both sources by email, a database entry before a file entry for
// the same email.
func (a *API) listPreapprovals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := a.d.Store.ListPreapprovals(ctx)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	users, err := a.d.Store.ListUsers(ctx, "")
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	byID := make(map[int64]store.User, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}
	ref := func(id *int64) *preapprovalUserRef {
		if id == nil {
			return nil
		}
		name := "unknown user"
		if u, ok := byID[*id]; ok {
			name = userDisplayName(u)
		}
		return &preapprovalUserRef{ID: *id, Name: name}
	}
	out := make([]preapprovalView, 0, len(rows)+len(a.d.Preapprovals.fileEntries()))
	for _, p := range rows {
		id, created := p.ID, ts(p.CreatedAt)
		out = append(out, preapprovalView{ID: &id, Email: p.Email, Roles: nonNil(p.Roles), Source: PreapprovalSourceDatabase,
			CreatedAt: &created, CreatedBy: ref(p.CreatedBy), UsedAt: tsPtr(p.UsedAt), UsedBy: ref(p.UsedBy)})
	}
	for _, e := range a.d.Preapprovals.fileEntries() {
		out = append(out, preapprovalView{Email: e.Email, Roles: nonNil(e.Roles), Source: PreapprovalSourceConfig})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Email != out[j].Email {
			return out[i].Email < out[j].Email
		}
		return out[i].Source == PreapprovalSourceDatabase && out[j].Source != PreapprovalSourceDatabase
	})
	writeJSON(w, http.StatusOK, struct {
		Preapprovals []preapprovalView `json:"preapprovals"`
	}{out})
}

// addPreapprovals is POST /api/v1/admin/preapprovals (PRE-5): 1 to 1,000
// emails with one role set, all or nothing. An email already in the database
// list gets the new roles and keeps its use record. An email given twice in
// one request counts once.
func (a *API) addPreapprovals(w http.ResponseWriter, r *http.Request) {
	caller, ok := adminCaller(w, r)
	if !ok {
		return
	}
	var body struct {
		Emails *[]string `json:"emails"`
		Roles  *[]string `json:"roles"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	switch {
	case body.Emails == nil:
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "emails is required", "emails")
		return
	case len(*body.Emails) == 0 || len(*body.Emails) > maxPreapprovalEmails:
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation,
			fmt.Sprintf("emails must hold 1 to %d addresses (it holds %d)", maxPreapprovalEmails, len(*body.Emails)), "emails")
		return
	case body.Roles == nil:
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "roles is required", "roles")
		return
	}
	emails := make([]string, 0, len(*body.Emails))
	seen := make(map[string]bool, len(*body.Emails))
	for _, raw := range *body.Emails {
		e, err := ParsePreapprovalEmail(raw)
		if err != nil {
			apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, err.Error(), "emails")
			return
		}
		if !seen[e] {
			seen[e] = true
			emails = append(emails, e)
		}
	}
	roles, err := ParsePreapprovalRoles(*body.Roles)
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	sort.Strings(emails)
	ctx := r.Context()
	now := a.d.Clock.Now()
	added, updated := 0, 0
	err = a.d.Store.Tx(ctx, func(tx *store.Tx) error {
		added, updated = 0, 0
		for _, e := range emails {
			created, err := tx.UpsertPreapproval(ctx, e, roles, caller.ID, now)
			if err != nil {
				return err
			}
			if created {
				added++
			} else {
				updated++
			}
		}
		return putAudit(ctx, tx, caller.ID, auditPreapprovalAdd, "preapprovals", nil,
			map[string]any{"emails": emails, "roles": roles}, now)
	})
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Added   int `json:"added"`
		Updated int `json:"updated"`
	}{added, updated})
}

// deletePreapproval is DELETE /api/v1/admin/preapprovals/{id} (PRE-5). It
// removes one database entry and changes no user.
func (a *API) deletePreapproval(w http.ResponseWriter, r *http.Request) {
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
		p, err := tx.DeletePreapproval(ctx, id)
		if err != nil {
			return err
		}
		return putAudit(ctx, tx, caller.ID, auditPreapprovalDel, targetID("preapproval", id),
			map[string]any{"email": p.Email, "roles": nonNil(p.Roles)}, nil, now)
	})
	if err != nil {
		a.adminFail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
