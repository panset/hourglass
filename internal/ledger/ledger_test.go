// SPDX-License-Identifier: MIT

package ledger

import (
	"reflect"
	"testing"
	"time"
)

const tok = 1000 // milli-tokens per token

func mustLoc(t *testing.T, n string) *time.Location {
	t.Helper()
	l, err := time.LoadLocation(n)
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	return l
}

func TestLED1_EntryTypeValid(t *testing.T) {
	for _, c := range []struct {
		t  EntryType
		ok bool
	}{{"allowance", true}, {"grant", true}, {"debit", true}, {"expiry", true}, {"carryover", true}, {"reset", true}, {"", false}, {"Debit", false}, {"refund", false}} {
		if c.t.Valid() != c.ok {
			t.Errorf("%q valid=%v want %v", c.t, c.t.Valid(), c.ok)
		}
	}
}

func TestLED2_Balance(t *testing.T) {
	es := []Entry{
		{Period: "2026-10", AmountMT: 500},
		{Period: "2026-11", AmountMT: 300 * tok},
		{Period: "2026-11", AmountMT: -40 * tok},
		{Period: "2026-12", AmountMT: 7},
	}
	for _, c := range []struct {
		p    string
		want int64
	}{{"2026-11", 260 * tok}, {"2026-10", 500}, {"2027-01", 0}} {
		if got := Balance(es, c.p); got != c.want {
			t.Errorf("%s: %d want %d", c.p, got, c.want)
		}
	}
	if Balance(nil, "2026-11") != 0 {
		t.Error("nil entries")
	}
}

func TestLED3_PeriodOf(t *testing.T) {
	kiritimati := time.FixedZone("UTC+14", 14*3600)
	la := mustLoc(t, "America/Los_Angeles")
	for _, c := range []struct {
		name string
		at   time.Time
		loc  *time.Location
		want string
	}{
		{"utc", time.Date(2026, 11, 30, 23, 59, 59, 0, time.UTC), time.UTC, "2026-11"},
		{"nil loc is utc", time.Date(2026, 11, 30, 23, 59, 59, 0, time.UTC), nil, "2026-11"},
		{"local month ahead of utc", time.Date(2026, 11, 30, 11, 0, 0, 0, time.UTC), kiritimati, "2026-12"},
		{"local month behind utc", time.Date(2026, 12, 1, 3, 0, 0, 0, time.UTC), la, "2026-11"},
		{"year wrap local", time.Date(2026, 12, 31, 20, 0, 0, 0, time.UTC), kiritimati, "2027-01"},
		{"exact boundary", time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC), la, "2026-12"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := PeriodOf(c.at, c.loc); got != c.want {
				t.Errorf("got %s want %s", got, c.want)
			}
		})
	}
}

func TestLED3_ParseAndNextPeriod(t *testing.T) {
	for _, c := range []struct{ in, next string }{{"2026-11", "2026-12"}, {"2026-12", "2027-01"}, {"2024-02", "2024-03"}, {"2026-01", "2026-02"}} {
		y, m, err := ParsePeriod(c.in)
		if err != nil || PeriodOf(time.Date(y, m, 1, 0, 0, 0, 0, time.UTC), nil) != c.in {
			t.Errorf("parse %s: %d %v %v", c.in, y, m, err)
		}
		if got := NextPeriod(c.in); got != c.next {
			t.Errorf("next %s: %s want %s", c.in, got, c.next)
		}
	}
	for _, bad := range []string{"", "2026", "2026-13", "2026-00", "2026-1", "2026/11", "26-11", "2026-11-01", "abcd-ef"} {
		if _, _, err := ParsePeriod(bad); err == nil {
			t.Errorf("%q should fail", bad)
		}
		if NextPeriod(bad) != "" {
			t.Errorf("NextPeriod(%q) should be empty", bad)
		}
		if !PeriodStart(bad, nil).IsZero() {
			t.Errorf("PeriodStart(%q) should be zero", bad)
		}
	}
}

func TestLED3_PeriodStartAndBoundary(t *testing.T) {
	la := mustLoc(t, "America/Los_Angeles")
	syd := mustLoc(t, "Australia/Sydney")
	for _, c := range []struct {
		name   string
		period string
		loc    *time.Location
		want   time.Time
	}{
		{"utc", "2026-11", time.UTC, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)},
		{"nil", "2026-11", nil, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)},
		{"la standard time", "2026-12", la, time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC)},
		{"la daylight time", "2026-07", la, time.Date(2026, 7, 1, 7, 0, 0, 0, time.UTC)},
		{"sydney is previous utc month", "2026-11", syd, time.Date(2026, 10, 31, 13, 0, 0, 0, time.UTC)},
		{"sydney standard", "2026-05", syd, time.Date(2026, 4, 30, 14, 0, 0, 0, time.UTC)},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := PeriodStart(c.period, c.loc)
			if !got.Equal(c.want) || got.Location() != time.UTC {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
	la = mustLoc(t, "America/Los_Angeles")
	for _, c := range []struct {
		name string
		now  time.Time
		loc  *time.Location
		want time.Time
	}{
		{"mid month", time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC), time.UTC, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)},
		{"year wrap", time.Date(2026, 12, 31, 23, 0, 0, 0, time.UTC), time.UTC, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"exactly on boundary moves to next", time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), time.UTC, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"la across DST end", time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), la, time.Date(2026, 11, 1, 7, 0, 0, 0, time.UTC)},
		{"la utc already next month", time.Date(2026, 12, 1, 3, 0, 0, 0, time.UTC), la, time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC)},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := NextBoundary(c.now, c.loc); !got.Equal(c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestLED3_Rollover(t *testing.T) {
	at := time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name    string
		closing int64
		want    []Entry
	}{
		{"positive expires", 120 * tok, []Entry{
			{UserID: 7, Period: "2026-11", Type: TypeExpiry, AmountMT: -120 * tok, CreatedAt: at},
			{UserID: 7, Period: "2026-12", Type: TypeAllowance, AmountMT: 300 * tok, CreatedAt: at}}},
		{"negative carries over", -40 * tok, []Entry{
			{UserID: 7, Period: "2026-12", Type: TypeCarryover, AmountMT: -40 * tok, CreatedAt: at},
			{UserID: 7, Period: "2026-12", Type: TypeAllowance, AmountMT: 300 * tok, CreatedAt: at}}},
		{"zero only allowance", 0, []Entry{
			{UserID: 7, Period: "2026-12", Type: TypeAllowance, AmountMT: 300 * tok, CreatedAt: at}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := Rollover(7, c.closing, 300*tok, "2026-11", "2026-12", at)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %+v want %+v", got, c.want)
			}
		})
	}
	// Spec example: closing -40 with allowance 300 opens at 260.
	got := Rollover(7, -40*tok, 300*tok, "2026-11", "2026-12", at)
	if b := Balance(got, "2026-12"); b != 260*tok {
		t.Errorf("opening balance %d want %d", b, 260*tok)
	}
	// Positive closing: old period zeroed.
	if b := Balance(append([]Entry{{Period: "a", AmountMT: 50}}, Rollover(7, 50, 300, "a", "b", at)...), "a"); b != 0 {
		t.Errorf("old period not zeroed: %d", b)
	}
}

func TestLED3_PeriodsToClose(t *testing.T) {
	la := mustLoc(t, "America/Los_Angeles")
	for _, c := range []struct {
		name string
		last string
		now  time.Time
		loc  *time.Location
		want []string
	}{
		{"up to date", "2026-11", time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC), time.UTC, nil},
		{"one missed", "2026-10", time.Date(2026, 11, 1, 0, 0, 1, 0, time.UTC), time.UTC, []string{"2026-10"}},
		{"several with year wrap", "2026-10", time.Date(2027, 2, 3, 0, 0, 0, 0, time.UTC), time.UTC, []string{"2026-10", "2026-11", "2026-12", "2027-01"}},
		{"timezone decides current", "2026-11", time.Date(2026, 12, 1, 3, 0, 0, 0, time.UTC), la, nil},
		{"timezone boundary passed", "2026-11", time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC), la, []string{"2026-11"}},
		{"last opened in future", "2027-01", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.UTC, nil},
		{"invalid last", "bogus", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.UTC, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := PeriodsToClose(c.last, c.now, c.loc); !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
	// Carry-over chains through skipped months.
	bal := int64(-500 * tok)
	for _, p := range PeriodsToClose("2026-10", time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC), nil) {
		es := Rollover(1, bal, 100*tok, p, NextPeriod(p), time.Time{})
		bal = Balance(es, NextPeriod(p))
	}
	if bal != -300*tok {
		t.Errorf("chained opening %d", bal)
	}
}

func TestROLE_FirstAllowanceOncePerPeriod(t *testing.T) {
	at := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	existing := []Entry{
		{UserID: 1, Period: "2026-11", Type: TypeAllowance},
		{UserID: 2, Period: "2026-10", Type: TypeAllowance},
		{UserID: 3, Period: "2026-11", Type: TypeGrant},
	}
	for _, c := range []struct {
		name string
		user int64
		nilE bool
	}{
		{"already has allowance", 1, true},
		{"allowance only in other period", 2, false},
		{"only a grant", 3, false},
		{"no entries", 4, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := FirstAllowance(c.user, 300*tok, "2026-11", existing, at)
			if (e == nil) != c.nilE {
				t.Fatalf("nil=%v want %v", e == nil, c.nilE)
			}
			if e != nil && (e.Type != TypeAllowance || e.AmountMT != 300*tok || e.UserID != c.user || e.Period != "2026-11" || !e.CreatedAt.Equal(at)) {
				t.Errorf("bad entry %+v", e)
			}
		})
	}
	if FirstAllowance(1, 1, "2026-11", nil, at) == nil {
		t.Error("nil existing")
	}
}

func TestLED4_Grant(t *testing.T) {
	at := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name   string
		amount int64
		reason string
		err    error
	}{
		{"positive", 50 * tok, "bonus", nil},
		{"negative correction", -5 * tok, "fix", nil},
		{"blank reason", 5, "   \t", ErrReasonRequired},
		{"empty reason", 5, "", ErrReasonRequired},
		{"zero amount", 0, "why", ErrZeroAmount},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := Grant(3, 9, c.amount, c.reason, "2026-11", at)
			if err != c.err {
				t.Fatalf("err %v want %v", err, c.err)
			}
			if err != nil {
				return
			}
			if e.Type != TypeGrant || e.AmountMT != c.amount || e.UserID != 3 || e.ActorID == nil || *e.ActorID != 9 || e.Reason != c.reason || e.Period != "2026-11" || e.HoldID != nil {
				t.Errorf("bad entry %+v", e)
			}
		})
	}
}

func TestLED5_Reset(t *testing.T) {
	at := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name    string
		current int64
		want    int64
	}{
		{"clears debt", -40 * tok, 340 * tok},
		{"tops up", 100 * tok, 200 * tok},
		{"reduces surplus", 500 * tok, -200 * tok},
		{"already at allowance still audited", 300 * tok, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := Reset(3, 9, c.current, 300*tok, "cleanup", "2026-11", at)
			if err != nil {
				t.Fatal(err)
			}
			if e.Type != TypeReset || e.AmountMT != c.want || e.ActorID == nil || *e.ActorID != 9 || e.Reason != "cleanup" {
				t.Errorf("bad entry %+v", e)
			}
			if c.current+e.AmountMT != 300*tok {
				t.Errorf("balance after reset %d", c.current+e.AmountMT)
			}
		})
	}
	for _, r := range []string{"", "  "} {
		if _, err := Reset(3, 9, 0, 300, r, "2026-11", at); err != ErrReasonRequired {
			t.Errorf("reason %q: %v", r, err)
		}
	}
}

func TestLED6_Debit(t *testing.T) {
	at := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	for _, amt := range []int64{1, 2500, 1_000_000_000} {
		e := Debit(3, 42, amt, "2026-11", at)
		if e.Type != TypeDebit || e.AmountMT != -amt || e.HoldID == nil || *e.HoldID != 42 || e.UserID != 3 || e.Period != "2026-11" {
			t.Errorf("bad entry %+v", e)
		}
	}
	// No floor: debt keeps growing.
	es := []Entry{{Period: "p", AmountMT: 10}}
	for i := 0; i < 3; i++ {
		es = append(es, Debit(1, 1, 100, "p", at))
	}
	if b := Balance(es, "p"); b != -290 {
		t.Errorf("balance %d", b)
	}
}

func TestEVT_BalanceCrossings(t *testing.T) {
	const a = 300 * tok // 10% = 30000
	for _, c := range []struct {
		name           string
		before, after  int64
		low, overdrawn bool
	}{
		{"no change above", 200 * tok, 150 * tok, false, false},
		{"exactly at threshold before, below after", 30 * tok, 30*tok - 1, true, false},
		{"before just under threshold", 30*tok - 1, 20 * tok, false, false},
		{"after exactly at threshold", 100 * tok, 30 * tok, false, false},
		{"cross low only", 100 * tok, 20 * tok, true, false},
		{"cross zero exactly", 20 * tok, 0, false, true},
		{"low and overdrawn together", 100 * tok, -5 * tok, true, true},
		{"from zero to negative", 0, -1, false, false},
		{"already negative", -5, -10, false, false},
		{"positive to exactly zero from above threshold", 100 * tok, 0, true, true},
		{"balance rising", 10 * tok, 200 * tok, false, false},
		{"tiny positive to zero", 1, 0, false, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			low, over := Crossings(c.before, c.after, a)
			if low != c.low || over != c.overdrawn {
				t.Errorf("got low=%v over=%v want %v %v", low, over, c.low, c.overdrawn)
			}
		})
	}
	// No 10% threshold when the allowance is zero or less: low is never set.
	for _, c := range []struct {
		name           string
		before, after  int64
		allow          int64
		low, overdrawn bool
	}{
		{"zero allowance, falls to negative", 5 * tok, -1 * tok, 0, false, true},
		{"zero allowance, falls to zero", 1, 0, 0, false, true},
		{"zero allowance, falls but stays positive", 100 * tok, 20 * tok, 0, false, false},
		{"zero allowance, already negative", -5 * tok, -10 * tok, 0, false, false},
		{"zero allowance, from zero to negative", 0, -1, 0, false, false},
		{"negative allowance, falls", 100 * tok, 20 * tok, -300 * tok, false, false},
		{"negative allowance, overdrawn", 100 * tok, -5 * tok, -300 * tok, false, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			low, over := Crossings(c.before, c.after, c.allow)
			if low != c.low || over != c.overdrawn {
				t.Errorf("got low=%v over=%v want %v %v", low, over, c.low, c.overdrawn)
			}
		})
	}
}

func TestHOLD1_CanAcquire(t *testing.T) {
	for _, c := range []struct {
		name string
		bal  int64
		enf  bool
		want bool
	}{
		{"positive enforced", 1, true, true},
		{"zero enforced", 0, true, false},
		{"negative enforced", -1, true, false},
		{"zero not enforced", 0, false, true},
		{"negative not enforced", -999, false, true},
		{"positive not enforced", 5, false, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := CanAcquire(c.bal, c.enf); got != c.want {
				t.Errorf("got %v", got)
			}
		})
	}
}
