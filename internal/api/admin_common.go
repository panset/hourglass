// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/store"
)

// Audit actions written by the admin handlers (ADM-3). The ledger service
// writes ledger.grant and ledger.reset, and the holds service writes
// hold.force_release, in the same transaction as their change.
const (
	auditUserRoles      = "user.roles"
	auditUserReject     = "user.reject"
	auditSetupCreate    = "setup.create"
	auditSetupUpdate    = "setup.update"
	auditSetupDelete    = "setup.delete"
	auditRulesReplace   = "rules.replace"
	auditSettingsUpdate = "settings.update"
)

// adminErr is a refusal raised inside an admin transaction: it carries the
// API error code, a message that is safe to show, and the offending field.
type adminErr struct{ code, msg, field string }

func (e *adminErr) Error() string { return e.msg }

func adminRefuse(code, msg string) error { return &adminErr{code: code, msg: msg} }

func adminInvalid(field, format string, args ...any) error {
	return &adminErr{code: apierr.Validation, msg: fmt.Sprintf(format, args...), field: field}
}

// adminFail answers with an *adminErr, or hands anything else to writeError.
func (a *API) adminFail(w http.ResponseWriter, r *http.Request, err error) {
	var ae *adminErr
	if errors.As(err, &ae) {
		apierr.WriteField(w, apierr.Status(ae.code), ae.code, ae.msg, ae.field)
		return
	}
	a.writeError(w, r, err)
}

// adminCaller returns the signed-in admin. The admin middleware guarantees
// one; the false case is a programming error answered as 401.
func adminCaller(w http.ResponseWriter, r *http.Request) (*auth.User, bool) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, apierr.Unauthenticated, "sign in required")
		return nil, false
	}
	return u, true
}

// putAudit appends one audit row inside the caller's transaction. before and
// after are marshalled as JSON; nil means null. Callers pass only values that
// are safe to store (never secrets, never setup details).
func putAudit(ctx context.Context, tx *store.Tx, actorID int64, action, target string, before, after any, at time.Time) error {
	enc := func(v any) (string, error) {
		if v == nil {
			return "", nil
		}
		b, err := json.Marshal(v)
		return string(b), err
	}
	bj, err := enc(before)
	if err != nil {
		return err
	}
	aj, err := enc(after)
	if err != nil {
		return err
	}
	_, err = tx.AppendAudit(ctx, store.AuditEntry{ActorID: &actorID, Action: action, Target: target,
		BeforeJSON: bj, AfterJSON: aj}, at)
	return err
}

func targetID(kind string, id int64) string { return kind + ":" + strconv.FormatInt(id, 10) }
