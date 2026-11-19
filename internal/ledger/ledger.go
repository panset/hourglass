// SPDX-License-Identifier: MIT

// Package ledger holds the pure ledger logic: entry types, balance, period
// arithmetic, monthly rollover, grants, resets and threshold crossings. It
// computes which entries to write; it never touches a database.
package ledger

import (
	"errors"
	"strings"
	"time"
)

// EntryType is the kind of a ledger entry (LED-1).
type EntryType string

const (
	TypeAllowance EntryType = "allowance"
	TypeGrant     EntryType = "grant"
	TypeDebit     EntryType = "debit"
	TypeExpiry    EntryType = "expiry"
	TypeCarryover EntryType = "carryover"
	TypeReset     EntryType = "reset"
)

// Valid reports whether t is one of the LED-1 entry types.
func (t EntryType) Valid() bool {
	switch t {
	case TypeAllowance, TypeGrant, TypeDebit, TypeExpiry, TypeCarryover, TypeReset:
		return true
	}
	return false
}

// Entry is one append-only ledger row. Amounts are signed milli-tokens.
type Entry struct {
	UserID    int64
	Period    string
	Type      EntryType
	AmountMT  int64
	HoldID    *int64
	ActorID   *int64
	Reason    string
	CreatedAt time.Time
}

var (
	ErrReasonRequired = errors.New("ledger: reason is required")
	ErrZeroAmount     = errors.New("ledger: amount must be non-zero")
	ErrBadPeriod      = errors.New("ledger: period must be YYYY-MM")
)

// Balance sums the amounts of entries in the given period (LED-2).
func Balance(entries []Entry, period string) int64 {
	var sum int64
	for _, e := range entries {
		if e.Period == period {
			sum += e.AmountMT
		}
	}
	return sum
}

// PeriodOf returns the YYYY-MM period of t in loc (nil means UTC).
func PeriodOf(t time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	return t.In(loc).Format("2006-01")
}

// ParsePeriod parses YYYY-MM into year and month.
func ParsePeriod(p string) (year int, month time.Month, err error) {
	if len(p) != 7 || p[4] != '-' {
		return 0, 0, ErrBadPeriod
	}
	t, err := time.Parse("2006-01", p)
	if err != nil {
		return 0, 0, ErrBadPeriod
	}
	return t.Year(), t.Month(), nil
}

// NextPeriod returns the period after p. An invalid p returns "".
func NextPeriod(p string) string {
	y, m, err := ParsePeriod(p)
	if err != nil {
		return ""
	}
	return time.Date(y, m+1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
}

// PeriodStart returns the 1st of the period at 00:00 in loc, in UTC. An
// invalid p returns the zero time.
func PeriodStart(p string, loc *time.Location) time.Time {
	y, m, err := ParsePeriod(p)
	if err != nil {
		return time.Time{}
	}
	if loc == nil {
		loc = time.UTC
	}
	return time.Date(y, m, 1, 0, 0, 0, 0, loc).UTC()
}

// NextBoundary returns the first period start strictly after now.
func NextBoundary(now time.Time, loc *time.Location) time.Time {
	return PeriodStart(NextPeriod(PeriodOf(now, loc)), loc)
}

// Rollover returns the entries that close oldPeriod and open newPeriod
// (LED-3): expiry or carryover (or neither), then the allowance.
func Rollover(userID, closingBalanceMT, allowanceMT int64, oldPeriod, newPeriod string, at time.Time) []Entry {
	var out []Entry
	switch {
	case closingBalanceMT > 0:
		out = append(out, Entry{UserID: userID, Period: oldPeriod, Type: TypeExpiry, AmountMT: -closingBalanceMT, CreatedAt: at})
	case closingBalanceMT < 0:
		out = append(out, Entry{UserID: userID, Period: newPeriod, Type: TypeCarryover, AmountMT: closingBalanceMT, CreatedAt: at})
	}
	out = append(out, Entry{UserID: userID, Period: newPeriod, Type: TypeAllowance, AmountMT: allowanceMT, CreatedAt: at})
	return out
}

// PeriodsToClose lists, in order, every period from lastOpened up to but not
// including the current period of now, so rollover can be replayed monthly.
func PeriodsToClose(lastOpened string, now time.Time, loc *time.Location) []string {
	if _, _, err := ParsePeriod(lastOpened); err != nil {
		return nil
	}
	cur := PeriodOf(now, loc)
	var out []string
	for p := lastOpened; p < cur; p = NextPeriod(p) {
		out = append(out, p)
	}
	return out
}

// FirstAllowance returns the allowance entry for a user's first role, or nil
// if an allowance entry already exists for that user and period.
func FirstAllowance(userID, allowanceMT int64, period string, existing []Entry, at time.Time) *Entry {
	for _, e := range existing {
		if e.UserID == userID && e.Period == period && e.Type == TypeAllowance {
			return nil
		}
	}
	return &Entry{UserID: userID, Period: period, Type: TypeAllowance, AmountMT: allowanceMT, CreatedAt: at}
}

// Grant builds a grant entry (LED-4).
func Grant(userID, actorID, amountMT int64, reason, period string, at time.Time) (Entry, error) {
	if strings.TrimSpace(reason) == "" {
		return Entry{}, ErrReasonRequired
	}
	if amountMT == 0 {
		return Entry{}, ErrZeroAmount
	}
	return Entry{UserID: userID, Period: period, Type: TypeGrant, AmountMT: amountMT, ActorID: &actorID, Reason: reason, CreatedAt: at}, nil
}

// Reset builds a reset entry bringing the balance to exactly the allowance
// (LED-5). A zero amount is still returned because it is audited.
func Reset(userID, actorID, currentBalanceMT, allowanceMT int64, reason, period string, at time.Time) (Entry, error) {
	if strings.TrimSpace(reason) == "" {
		return Entry{}, ErrReasonRequired
	}
	return Entry{UserID: userID, Period: period, Type: TypeReset, AmountMT: allowanceMT - currentBalanceMT, ActorID: &actorID, Reason: reason, CreatedAt: at}, nil
}

// Debit builds a debit entry; there is no floor (LED-6).
func Debit(userID, holdID, amountMT int64, period string, at time.Time) Entry {
	return Entry{UserID: userID, Period: period, Type: TypeDebit, AmountMT: -amountMT, HoldID: &holdID, CreatedAt: at}
}

// Crossings reports threshold crossings caused by a balance change: low when
// the balance falls from at-or-above 10% of the allowance to below it, and
// overdrawn when it falls from positive to zero or below.
func Crossings(beforeMT, afterMT, allowanceMT int64) (low, overdrawn bool) {
	// 10% compared without integer division: x*10 vs allowance.
	low = beforeMT*10 >= allowanceMT && afterMT*10 < allowanceMT
	overdrawn = beforeMT > 0 && afterMT <= 0
	return
}

// CanAcquire reports whether a hold may be acquired (HOLD-1, MET-6).
func CanAcquire(balanceMT int64, enforcement bool) bool {
	return balanceMT > 0 || !enforcement
}
