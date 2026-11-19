# 06 internal/ledger (pure logic)
Job intent: .orchestrator/hourglass-v1/INTENT.md (read it first). Rules: .orchestrator/hourglass-v1/CONTRACT.md.
## Goal
Package `internal/ledger`: ledger entry types, balance, period arithmetic, monthly rollover with expiry and carry-over, grant, reset and threshold-crossing logic as pure functions with unit tests. Database writes come in a later task; this package only computes which entries to write. Spec: docs/SPEC.md "Allowance, grants and running out", LED-1..LED-6, the `ledger` row of the data model, events `balance.low` / `balance.overdrawn`, and the rule "The allowance starts with the role".
## Behaviour
- `Entry{UserID int64; Period string; Type EntryType; AmountMT int64; HoldID *int64; ActorID *int64; Reason string; CreatedAt time.Time}`; types `allowance, grant, debit, expiry, carryover, reset` (LED-1) with `(EntryType).Valid()`. Amounts are signed milli-tokens: debits, expiry and carry-over are negative.
- `Balance(entries []Entry, period string) int64`: sum of AmountMT for that period (LED-2).
- Periods: `PeriodOf(t time.Time, loc *time.Location) string` → `YYYY-MM` in the configured timezone; `ParsePeriod`, `NextPeriod(period)`, `PeriodStart(period, loc) time.Time` (the 1st at 00:00 local, returned in UTC), `NextBoundary(now, loc) time.Time`. Test year wrap, DST zones and a zone where the local month differs from the UTC month.
- `Rollover(userID, closingBalanceMT, allowanceMT int64, oldPeriod, newPeriod string, at time.Time) []Entry` (LED-3): positive closing balance → one `expiry` entry of -balance in the old period; negative → one `carryover` entry of that negative amount in the new period; zero → neither; then always one `allowance` entry of +allowance in the new period. Order of returned entries is expiry/carryover, then allowance. Spec example: closing -40 tokens with allowance 300 opens at 260.
- `PeriodsToClose(lastOpened string, now time.Time, loc) []string`: every period boundary missed while the process was down, in order, so rollover can be replayed one month at a time (carry-over chains through skipped months).
- `FirstAllowance(userID, allowanceMT int64, period string, existing []Entry, at time.Time) *Entry`: the allowance entry for a user given their first role, or nil if an `allowance` entry already exists for that user and period (at most once per period).
- `Grant(userID, actorID, amountMT int64, reason, period string, at time.Time) (Entry, error)` (LED-4): reason required (non-blank), amount non-zero, negative allowed.
- `Reset(userID, actorID, currentBalanceMT, allowanceMT int64, reason, period string, at time.Time) (Entry, error)` (LED-5): a `reset` entry of allowance - current, so the balance becomes exactly the allowance; reason required; a zero-amount reset is still returned (it is audited).
- `Debit(userID, holdID, amountMT int64, period string, at time.Time) Entry`: stores -amountMT; no floor (LED-6).
- `Crossings(beforeMT, afterMT, allowanceMT int64) (low, overdrawn bool)`: low is true when before >= 10% of allowance and after < 10% of allowance; overdrawn when before > 0 and after <= 0. Both can be true at once.
- `CanAcquire(balanceMT int64, enforcement bool) bool`: balance > 0, or enforcement off (HOLD-1, MET-6).
## Scope
May write: `internal/ledger/`, this task folder.
## Build notes
The skeleton from task 02 is in place (see tasks/02-build-skeleton/RESULT.md for versions). Standard library only: add no dependency and do not edit go.mod, MODULE.bazel or root files. Generate your BUILD file with `bazel run //:gazelle -- <your package dir>` and test with both `go test ./<dir>/...` and `bazel test //<dir>/...`. Two other agents are building sibling packages at the same time; Bazel commands in one workspace queue behind each other, so allow up to 5 minutes per Bazel command and never kill the Bazel server. The package must not import any other Hourglass package, and must not mention vCenter, vSphere, sizes (medium/large/...) or any concrete extension outside test data that uses neutral labels (e.g. `tier=a`, `site=x`).
## Tests
Table-driven, named with requirement ids (`TestRULE2_...`). Cover every rule in "Behaviour" including error cases and boundaries. Report the number of test cases executed.
## Done when
Tests pass under go and Bazel, `./tools/check_spdx.sh` passes, RESULT.md lists the exported API and decisions.
