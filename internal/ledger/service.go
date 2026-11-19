// SPDX-License-Identifier: MIT

package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// Errors returned by Service.
var (
	ErrLocalAdmin = errors.New("ledger: the local admin account has no balance")
	ErrNoRole     = errors.New("ledger: user has no role")
)

// Audit actions written by Grant and Reset.
const (
	AuditGrant = "ledger.grant"
	AuditReset = "ledger.reset"
)

// Emitter receives balance and period events. *events.Dispatcher satisfies it.
type Emitter interface{ Emit(hourglass.Event) }

// Clock is the injected time source.
type Clock interface{ Now() time.Time }

// BalanceCrossings reports which balance thresholds a write crossed (D15) and the
// events to emit for them. Methods that run inside a caller's transaction
// return it instead of emitting; the caller passes Events to
// Service.EmitEvents after the commit.
type BalanceCrossings struct {
	Low, Overdrawn bool
	BalanceMT      int64
	AllowanceMT    int64
	Events         []hourglass.Event
}

// Service is the only writer of the ledger (LED-1..6): it owns the balance
// rules and the balance events.
type Service struct {
	st    *store.Store
	emit  Emitter
	clock Clock
	loc   *time.Location

	// cacheMu guards cachePeriod: the period every users.balance_mt cache was
	// last rebuilt for from the ledger (RepairBalances or a rollover). When the
	// current period differs from it, RolloverDue rebuilds (see there).
	cacheMu     sync.Mutex
	cachePeriod string
}

// NewService returns a ledger service. A nil loc means UTC and a nil emitter
// drops events.
func NewService(st *store.Store, emit Emitter, clock Clock, loc *time.Location) *Service {
	if loc == nil {
		loc = time.UTC
	}
	return &Service{st: st, emit: emit, clock: clock, loc: loc}
}

// CurrentPeriod is the period (YYYY-MM) of the clock in the configured zone.
func (s *Service) CurrentPeriod() string { return PeriodOf(s.clock.Now(), s.loc) }

// EmitEvents emits events after the transaction that produced them committed.
func (s *Service) EmitEvents(evs ...hourglass.Event) {
	if s.emit == nil {
		return
	}
	for _, e := range evs {
		s.emit.Emit(e)
	}
}

func hgUser(u store.User, roles []string) *hourglass.User {
	return &hourglass.User{ID: u.ID, Username: u.Username, Email: u.Email, Name: u.Name, Roles: roles}
}

func tokens(mt int64) float64 { return float64(mt) / 1000 }

// balanceCrossings applies the D15 rules to a balance change and builds the events.
func balanceCrossings(u *hourglass.User, before, after, allowance int64, at time.Time) BalanceCrossings {
	c := BalanceCrossings{BalanceMT: after, AllowanceMT: allowance}
	c.Low, c.Overdrawn = Crossings(before, after, allowance)
	data := func() map[string]any {
		return map[string]any{"balance_tokens": tokens(after), "allowance_tokens": tokens(allowance)}
	}
	if c.Low {
		c.Events = append(c.Events, hourglass.Event{Type: hourglass.EventBalanceLow, Time: at, User: u, Data: data()})
	}
	if c.Overdrawn {
		c.Events = append(c.Events, hourglass.Event{Type: hourglass.EventBalanceOverdrawn, Time: at, User: u, Data: data()})
	}
	return c
}

// DebitTx appends a debit of amountMT (positive) for a hold to the period of
// at, inside the caller's transaction, and reports the thresholds crossed.
// The caller emits Crossings.Events after commit (EmitEvents).
func (s *Service) DebitTx(tx *store.Tx, userID, holdID, amountMT int64, at time.Time) (BalanceCrossings, error) {
	ctx := context.Background()
	u, err := tx.GetUser(ctx, userID)
	if err != nil {
		return BalanceCrossings{}, err
	}
	roles, err := tx.GetUserRoles(ctx, userID)
	if err != nil {
		return BalanceCrossings{}, err
	}
	cfg, err := settings.Load(ctx, tx)
	if err != nil {
		return BalanceCrossings{}, err
	}
	cur := s.CurrentPeriod()
	before, err := tx.SumLedger(ctx, userID, cur, "")
	if err != nil {
		return BalanceCrossings{}, err
	}
	e := Debit(userID, holdID, amountMT, PeriodOf(at, s.loc), at)
	if _, err := tx.AppendLedger(ctx, toStore(e), cur, at); err != nil {
		return BalanceCrossings{}, err
	}
	after, err := tx.SumLedger(ctx, userID, cur, "")
	if err != nil {
		return BalanceCrossings{}, err
	}
	return balanceCrossings(hgUser(u, roles), before, after, cfg.DefaultAllowanceMT, at), nil
}

func toStore(e Entry) store.LedgerEntry {
	return store.LedgerEntry{UserID: e.UserID, Period: e.Period, Type: string(e.Type), AmountMT: e.AmountMT,
		HoldID: e.HoldID, ActorID: e.ActorID, Reason: e.Reason, CreatedAt: e.CreatedAt}
}

func fromStore(e store.LedgerEntry) Entry {
	return Entry{UserID: e.UserID, Period: e.Period, Type: EntryType(e.Type), AmountMT: e.AmountMT,
		HoldID: e.HoldID, ActorID: e.ActorID, Reason: e.Reason, CreatedAt: e.CreatedAt}
}

// eligible loads a user for Grant/Reset: not the local admin, with a role.
func eligible(ctx context.Context, tx *store.Tx, userID int64) (store.User, []string, error) {
	u, err := tx.GetUser(ctx, userID)
	if err != nil {
		return u, nil, err
	}
	if u.AuthSource == store.AuthLocal {
		return u, nil, ErrLocalAdmin
	}
	roles, err := tx.GetUserRoles(ctx, userID)
	if err != nil {
		return u, nil, err
	}
	if len(roles) == 0 {
		return u, nil, ErrNoRole
	}
	return u, roles, nil
}

// Grant adds amountMT (negative allowed, LED-4) to a user's current period
// with a required reason, recording the actor, and writes the audit row in
// the same transaction. Balance events are emitted after commit.
func (s *Service) Grant(ctx context.Context, actorID, userID, amountMT int64, reason string) (Entry, error) {
	return s.adjust(ctx, actorID, userID, AuditGrant, func(cur int64, now time.Time, _ int64) (Entry, error) {
		return Grant(userID, actorID, amountMT, reason, s.CurrentPeriod(), now)
	})
}

// Reset brings a user's balance to exactly the default allowance (LED-5)
// with a required reason; a reset that changes nothing is still written.
func (s *Service) Reset(ctx context.Context, actorID, userID int64, reason string) (Entry, error) {
	return s.adjust(ctx, actorID, userID, AuditReset, func(cur int64, now time.Time, allowance int64) (Entry, error) {
		return Reset(userID, actorID, cur, allowance, reason, s.CurrentPeriod(), now)
	})
}

func (s *Service) adjust(ctx context.Context, actorID, userID int64, action string,
	build func(cur int64, now time.Time, allowance int64) (Entry, error)) (Entry, error) {
	var (
		out Entry
		cx  BalanceCrossings
	)
	now := s.clock.Now()
	err := s.st.Tx(ctx, func(tx *store.Tx) error {
		u, roles, err := eligible(ctx, tx, userID)
		if err != nil {
			return err
		}
		cfg, err := settings.Load(ctx, tx)
		if err != nil {
			return err
		}
		cur := s.CurrentPeriod()
		before, err := tx.SumLedger(ctx, userID, cur, "")
		if err != nil {
			return err
		}
		e, err := build(before, now, cfg.DefaultAllowanceMT)
		if err != nil {
			return err
		}
		se, err := tx.AppendLedger(ctx, toStore(e), cur, now)
		if err != nil {
			return err
		}
		out = fromStore(se)
		after, err := tx.SumLedger(ctx, userID, cur, "")
		if err != nil {
			return err
		}
		bj, _ := json.Marshal(map[string]any{"balance_mt": before})
		aft, _ := json.Marshal(map[string]any{"balance_mt": after, "entry_id": se.ID, "amount_mt": e.AmountMT, "reason": e.Reason})
		actor := actorID
		if _, err := tx.AppendAudit(ctx, store.AuditEntry{ActorID: &actor, Action: action,
			Target: fmt.Sprintf("user:%d", userID), BeforeJSON: string(bj), AfterJSON: string(aft)}, now); err != nil {
			return err
		}
		cx = balanceCrossings(hgUser(u, roles), before, after, cfg.DefaultAllowanceMT, now)
		return nil
	})
	if err != nil {
		return Entry{}, err
	}
	s.EmitEvents(cx.Events...)
	return out, nil
}

// EnsureFirstAllowanceTx writes the allowance entry for the current period
// when a user is first given a role, unless one exists (at most once per
// period). It does nothing for the local admin or a user without a role. If a
// rollover is pending (the newest allowance period is older than the current
// one) it also does nothing: the rollover gives every role holder the
// allowance, and writing one early would hide the pending rollover.
func (s *Service) EnsureFirstAllowanceTx(tx *store.Tx, userID int64, at time.Time) error {
	ctx := context.Background()
	u, err := tx.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if u.AuthSource == store.AuthLocal {
		return nil
	}
	roles, err := tx.GetUserRoles(ctx, userID)
	if err != nil || len(roles) == 0 {
		return err
	}
	cur := s.CurrentPeriod()
	latest, err := tx.LatestAllowancePeriod(ctx)
	if err != nil {
		return err
	}
	if latest != "" && latest < cur {
		return nil
	}
	n, err := tx.CountLedger(ctx, userID, cur, string(TypeAllowance))
	if err != nil || n > 0 {
		return err
	}
	cfg, err := settings.Load(ctx, tx)
	if err != nil {
		return err
	}
	e := FirstAllowance(userID, cfg.DefaultAllowanceMT, cur, nil, at)
	_, err = tx.AppendLedger(ctx, toStore(*e), cur, at)
	return err
}

// RolloverDue closes every period boundary passed since the last opened
// period (LED-3, D17, D21), in order, one transaction per period, and emits
// period.closed after each commit. The last opened period is the newest
// allowance entry in the ledger; an empty ledger closes nothing. A period in
// which no user qualifies writes nothing and is not reported as closed.
func (s *Service) RolloverDue(ctx context.Context) ([]string, error) {
	last, err := s.st.LatestAllowancePeriod(ctx)
	if err != nil || last == "" {
		return nil, err
	}
	var closed []string
	for _, old := range PeriodsToClose(last, s.clock.Now(), s.loc) {
		next := NextPeriod(old)
		n := 0
		at := s.clock.Now()
		var rep store.BalanceRepair
		cur := s.CurrentPeriod()
		err := s.st.Tx(ctx, func(tx *store.Tx) error {
			cfg, err := settings.Load(ctx, tx)
			if err != nil {
				return err
			}
			users, err := tx.ListRoleHolders(ctx)
			if err != nil {
				return err
			}
			for _, u := range users {
				closing, err := tx.SumLedger(ctx, u.ID, old, "")
				if err != nil {
					return err
				}
				has, err := tx.CountLedger(ctx, u.ID, next, string(TypeAllowance))
				if err != nil {
					return err
				}
				for _, e := range Rollover(u.ID, closing, cfg.DefaultAllowanceMT, old, next, at) {
					if e.Type == TypeAllowance && has > 0 {
						continue
					}
					// The cache follows the real current period, not next: when
					// several periods are closed at once, the entries of the older
					// ones must not touch it (LED-2).
					if _, err := tx.AppendLedger(ctx, toStore(e), cur, at); err != nil {
						return err
					}
				}
				n++
			}
			// Belt and braces (LED-2): whatever the incremental updates did, every
			// cache leaves this transaction equal to the ledger sum of the
			// current period, from one grouped statement.
			rep, err = tx.RebuildBalances(ctx, cur)
			return err
		})
		if err != nil {
			return closed, err
		}
		s.setCachePeriod(cur)
		if rep.Changed > 0 {
			slog.Info("ledger rollover corrected balance caches", "period", old, "current", cur,
				"changed", rep.Changed, "abs_delta_mt", rep.AbsDeltaMT, "net_delta_mt", rep.NetDeltaMT)
		}
		if n == 0 {
			continue
		}
		closed = append(closed, old)
		slog.Info("ledger period closed", "period", old, "next", next, "users", n)
		s.EmitEvents(hourglass.Event{Type: hourglass.EventPeriodClosed, Time: at,
			Data: map[string]any{"period": old, "next": next, "users": n}})
	}
	// The current period moved without a rollover to rebuild the caches (the
	// configured timezone moved it backwards and the clock has now come back to
	// a period already opened): appends since then assumed the caches held this
	// period, so rebuild them all from the ledger.
	// A period not yet recorded in this process counts as moved, so the guard
	// does not depend on the caller having rebuilt at start-up.
	if cur, done := s.CurrentPeriod(), s.getCachePeriod(); cur != done {
		rep, err := s.RepairBalances(ctx)
		if err != nil {
			return closed, err
		}
		if rep.Changed > 0 {
			slog.Warn("ledger balance caches rebuilt after the current period moved", "from", done, "to", cur,
				"changed", rep.Changed, "abs_delta_mt", rep.AbsDeltaMT, "net_delta_mt", rep.NetDeltaMT)
		}
	}
	return closed, nil
}

func (s *Service) setCachePeriod(p string) { s.cacheMu.Lock(); s.cachePeriod = p; s.cacheMu.Unlock() }

func (s *Service) getCachePeriod() string {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	return s.cachePeriod
}

// RepairBalances rebuilds every user's balance cache from the ledger for the
// current period (LED-2) in one grouped statement and reports what changed.
// The application calls it at start-up, before the first poll and before
// serving requests; a non-zero change means a cache was wrong.
func (s *Service) RepairBalances(ctx context.Context) (store.BalanceRepair, error) {
	cur := s.CurrentPeriod()
	rep, err := s.st.RebuildBalances(ctx, cur)
	if err != nil {
		return rep, err
	}
	s.setCachePeriod(cur)
	return rep, nil
}

// Balance is a user's balance in the current period, from the ledger.
func (s *Service) Balance(ctx context.Context, userID int64) (int64, error) {
	return s.st.SumLedger(ctx, userID, s.CurrentPeriod(), "")
}

// MonthToDateBurn is the sum of the user's debits this period, as a
// positive amount.
func (s *Service) MonthToDateBurn(ctx context.Context, userID int64) (int64, error) {
	n, err := s.st.SumLedger(ctx, userID, s.CurrentPeriod(), string(TypeDebit))
	return -n, err
}

// BurnLast24h is what the user's holds were charged in the last 24 hours.
func (s *Service) BurnLast24h(ctx context.Context, userID int64) (int64, error) {
	return s.st.SumDebitsSince(ctx, userID, s.clock.Now().Add(-24*time.Hour))
}
