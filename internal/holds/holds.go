// SPDX-License-Identifier: MIT

// Package holds implements acquiring and releasing a setup (HOLD-1, HOLD-2,
// HOLD-3, HOLD-5, MET-6), independent of HTTP. Metering start and deployment
// are the collector's job; this package owns the two user-driven transitions
// and their events.
package holds

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/panset/hourglass/internal/events"
	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// Error codes. They equal the codes of api/openapi.yaml (ErrorCode).
const (
	CodeForbidden          = "forbidden"
	CodeNotFound           = "not_found"
	CodeValidation         = "validation"
	CodeSetupDisabled      = "setup_disabled"
	CodeSetupHeld          = "setup_held"
	CodeInsufficientTokens = "insufficient_tokens"
	CodeHoldLimit          = "hold_limit"
	CodeHookVeto           = "hook_veto"
	CodeStillDeployed      = "still_deployed"
	CodeConflict           = "conflict" // releasing a hold that is already released
)

// MessageStillDeployed is the refusal shown when a release needs teardown first.
const MessageStillDeployed = "This setup is still deployed. Tear it down, then release."

// AuditForceRelease is the audit_log action written by a forced release.
const AuditForceRelease = "hold.force_release"

// Error is a refusal with a stable code. Message is safe to show the user.
type Error struct {
	Code    string
	Message string
}

// Error returns the message.
func (e *Error) Error() string { return e.Message }

// CodeOf returns the code of a *Error in err's chain, or "" for any other error.
func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func refuse(code, msg string) error { return &Error{Code: code, Message: msg} }

// PollView is the latest successful poll, supplied by the collector.
type PollView interface {
	// OK reports whether a successful poll exists.
	OK() bool
	// Reported reports whether the provider reported the resource in it.
	Reported(resourceName string) bool
}

// Dispatcher delivers events. *events.Dispatcher satisfies it.
type Dispatcher interface {
	Veto(ctx context.Context, e hourglass.Event) error
	Emit(e hourglass.Event)
}

// Clock is the injected time source.
type Clock interface{ Now() time.Time }

// Deps are the collaborators of a Service.
type Deps struct {
	Store *store.Store
	// Events may be nil: nothing is vetoed or delivered.
	Events Dispatcher
	// Poll returns the latest successful poll view, or nil when there is none
	// yet. It may be nil.
	Poll func() PollView
	// ProviderConfigured reports whether a resource provider is configured.
	ProviderConfigured bool
	Clock              Clock
	// Logger defaults to slog.Default().
	Logger *slog.Logger
}

// Service acquires and releases holds.
type Service struct {
	d   Deps
	log *slog.Logger
}

// NewService returns a Service.
func NewService(d Deps) *Service {
	l := d.Logger
	if l == nil {
		l = slog.Default()
	}
	return &Service{d: d, log: l}
}

// Hold is a hold as returned by this package.
type Hold = store.Hold

// ReleaseOpts qualify a release.
type ReleaseOpts struct {
	// Force releases regardless of the provider; admin only, Reason required.
	Force  bool
	Reason string
}

func hgUser(u store.User, roles []string) *hourglass.User {
	return &hourglass.User{ID: u.ID, Username: u.Username, Email: u.Email, Name: u.Name, Roles: roles}
}

func hgSetup(s store.Setup) *hourglass.Setup {
	labels := make(map[string]string, len(s.Labels))
	for k, v := range s.Labels {
		labels[k] = v
	}
	var details json.RawMessage
	if s.DetailsJSON != "" {
		details = json.RawMessage(s.DetailsJSON)
	}
	return &hourglass.Setup{Name: s.Name, Labels: labels, Details: details}
}

func hgHold(h store.Hold, setupName string) *hourglass.Hold {
	return &hourglass.Hold{ID: h.ID, SetupName: setupName, UserID: h.UserID, AcquiredAt: h.AcquiredAt,
		MeteringStartedAt: h.MeteringStartedAt, DeployedAt: h.DeployedAt, ReleasedAt: h.ReleasedAt}
}

func hasRole(roles []string, r string) bool {
	for _, x := range roles {
		if x == r {
			return true
		}
	}
	return false
}

func (s *Service) emit(e hourglass.Event) {
	if s.d.Events != nil {
		s.d.Events.Emit(e)
	}
}

// Acquire claims a setup for a user (HOLD-1). The checks run in a fixed
// order and the first failure is returned as a *Error. Nothing is charged
// (lifecycle step 1).
//
// The hold.acquiring event carries a Hold with ID 0 and AcquiredAt set to
// the time the hold will get, because the hold does not exist yet.
func (s *Service) Acquire(ctx context.Context, userID, setupID int64) (Hold, error) {
	st := s.d.Store
	u, err := st.GetUser(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return Hold{}, refuse(CodeForbidden, "You are not allowed to acquire setups.")
	}
	if err != nil {
		return Hold{}, err
	}
	roles, err := st.GetUserRoles(ctx, userID)
	if err != nil {
		return Hold{}, err
	}
	if len(roles) == 0 || u.AuthSource == store.AuthLocal {
		return Hold{}, refuse(CodeForbidden, "You are not allowed to acquire setups.")
	}

	setup, err := st.GetSetup(ctx, setupID)
	if errors.Is(err, store.ErrNotFound) {
		return Hold{}, refuse(CodeNotFound, "Setup not found.")
	}
	if err != nil {
		return Hold{}, err
	}
	if !setup.Enabled {
		return Hold{}, refuse(CodeSetupDisabled, "This setup is disabled.")
	}
	if _, err := st.ActiveHoldForSetup(ctx, setupID); err == nil {
		return Hold{}, refuse(CodeSetupHeld, "This setup is already held.")
	} else if !errors.Is(err, store.ErrNotFound) {
		return Hold{}, err
	}

	cfg, err := settings.Load(ctx, st)
	if err != nil {
		return Hold{}, err
	}
	if cfg.Enforcement && u.BalanceMT <= 0 { // MET-6: shadow mode never blocks
		return Hold{}, refuse(CodeInsufficientTokens, "Your token balance is zero or below.")
	}
	n, err := st.CountActiveHolds(ctx, userID)
	if err != nil {
		return Hold{}, err
	}
	if n >= cfg.MaxHoldsPerUser {
		return Hold{}, limitErr(cfg.MaxHoldsPerUser)
	}

	now := s.d.Clock.Now()
	hu, hs := hgUser(u, roles), hgSetup(setup)
	if s.d.Events != nil {
		err := s.d.Events.Veto(ctx, hourglass.Event{Type: hourglass.EventHoldAcquiring, Time: now, User: hu, Setup: hs,
			Hold: &hourglass.Hold{SetupName: setup.Name, UserID: userID, AcquiredAt: now}})
		if err != nil {
			var ve *events.VetoError
			if errors.As(err, &ve) {
				return Hold{}, refuse(CodeHookVeto, ve.Error())
			}
			return Hold{}, err
		}
	}

	// The hook ran outside any transaction; re-check the limit with the
	// write lock held. The partial unique index decides a race for the setup.
	var h Hold
	err = st.Tx(ctx, func(tx *store.Tx) error {
		n, err := tx.CountActiveHolds(ctx, userID)
		if err != nil {
			return err
		}
		if n >= cfg.MaxHoldsPerUser {
			return limitErr(cfg.MaxHoldsPerUser)
		}
		h, err = tx.CreateHold(ctx, setupID, userID, now)
		return err
	})
	if errors.Is(err, store.ErrSetupHeld) {
		return Hold{}, refuse(CodeSetupHeld, "This setup is already held.")
	}
	if err != nil {
		return Hold{}, err
	}
	s.emit(hourglass.Event{Type: hourglass.EventHoldAcquired, Time: now, User: hu, Setup: hs, Hold: hgHold(h, setup.Name)})
	return h, nil
}

func limitErr(max int) error {
	return refuse(CodeHoldLimit, fmt.Sprintf("You already hold the maximum of %d setups.", max))
}

// Release ends a hold (HOLD-2). The owner may release their own hold; an
// admin may do the same and may force-release any hold with a reason.
// A forced release is audited in the same transaction.
func (s *Service) Release(ctx context.Context, actorID, holdID int64, opts ReleaseOpts) (Hold, error) {
	st := s.d.Store
	_, err := st.GetUser(ctx, actorID)
	if errors.Is(err, store.ErrNotFound) {
		return Hold{}, refuse(CodeForbidden, "You are not allowed to release this hold.")
	}
	if err != nil {
		return Hold{}, err
	}
	aroles, err := st.GetUserRoles(ctx, actorID)
	if err != nil {
		return Hold{}, err
	}
	isAdmin := hasRole(aroles, store.RoleAdmin)
	if len(aroles) == 0 {
		return Hold{}, refuse(CodeForbidden, "You are not allowed to release this hold.")
	}

	h, err := st.GetHold(ctx, holdID)
	if errors.Is(err, store.ErrNotFound) {
		return Hold{}, refuse(CodeNotFound, "Hold not found.")
	}
	if err != nil {
		return Hold{}, err
	}
	owner := h.UserID == actorID
	if (!owner && !(isAdmin && opts.Force)) || (opts.Force && !isAdmin) {
		return Hold{}, refuse(CodeForbidden, "You are not allowed to release this hold.")
	}
	if h.ReleasedAt != nil { // after the permission check, so a stranger learns nothing about the hold
		return Hold{}, refuse(CodeConflict, "This hold is already released.")
	}
	reason := strings.TrimSpace(opts.Reason)
	if opts.Force && reason == "" {
		return Hold{}, refuse(CodeValidation, "A reason is required to force-release a hold.")
	}

	setup, err := st.GetSetup(ctx, h.SetupID)
	if err != nil {
		return Hold{}, err
	}
	if !opts.Force {
		cfg, err := settings.Load(ctx, st)
		if err != nil {
			return Hold{}, err
		}
		if cfg.ReleaseRequiresTeardown && s.d.ProviderConfigured && s.d.Poll != nil {
			if pv := s.d.Poll(); pv != nil && pv.OK() {
				name, err := settings.ResourceName(cfg.ResourceNameTemplate, setup.Name, setup.Labels, setup.ResourceNameOverride)
				if err != nil {
					return Hold{}, err
				}
				if pv.Reported(name) {
					return Hold{}, refuse(CodeStillDeployed, MessageStillDeployed)
				}
			}
		}
	}

	now := s.d.Clock.Now()
	var out Hold
	err = st.Tx(ctx, func(tx *store.Tx) error {
		if err := tx.ReleaseHold(ctx, holdID, &actorID, opts.Reason, now); err != nil {
			return err
		}
		if opts.Force {
			before, _ := json.Marshal(map[string]any{"hold_id": holdID, "setup": setup.Name, "user_id": h.UserID,
				"acquired_at": h.AcquiredAt, "metering_started_at": h.MeteringStartedAt, "deployed_at": h.DeployedAt,
				"released_at": nil})
			after, _ := json.Marshal(map[string]any{"hold_id": holdID, "setup": setup.Name, "user_id": h.UserID, "reason": reason})
			if _, err := tx.AppendAudit(ctx, store.AuditEntry{ActorID: &actorID, Action: AuditForceRelease,
				Target: fmt.Sprintf("hold:%d", holdID), BeforeJSON: string(before), AfterJSON: string(after)}, now); err != nil {
				return err
			}
		}
		out, err = tx.GetHold(ctx, holdID)
		return err
	})
	if errors.Is(err, store.ErrConflict) || errors.Is(err, store.ErrNotFound) {
		return Hold{}, refuse(CodeConflict, "This hold is already released.") // released concurrently
	}
	if err != nil {
		return Hold{}, err
	}

	owneru, err := st.GetUser(ctx, h.UserID)
	if err != nil {
		s.log.Warn("holds: load hold owner for event", "hold", holdID, "err", err)
		return out, nil
	}
	oroles, _ := st.GetUserRoles(ctx, h.UserID)
	s.emit(hourglass.Event{Type: hourglass.EventHoldReleased, Time: now, User: hgUser(owneru, oroles), Setup: hgSetup(setup),
		Hold: hgHold(out, setup.Name), Data: map[string]any{"forced": opts.Force, "reason": opts.Reason}})
	return out, nil
}
