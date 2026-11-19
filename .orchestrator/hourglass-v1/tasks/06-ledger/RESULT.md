# Result: 06 internal/ledger (done)

Tests: 71 pass lines in `go test -v` (top-level tests plus subtests; 14 top-level test funcs). `go test ./internal/ledger/` ok; `bazel test //internal/ledger/...` passed (1 of 1 target). `./tools/check_spdx.sh` passes.

Files: internal/ledger/{ledger.go,ledger_test.go,BUILD.bazel}. Stdlib only, no other Hourglass imports.

## Exported API
EntryType (+ TypeAllowance, TypeGrant, TypeDebit, TypeExpiry, TypeCarryover, TypeReset; Valid()), Entry, ErrReasonRequired, ErrZeroAmount, ErrBadPeriod,
Balance, PeriodOf, ParsePeriod, NextPeriod, PeriodStart, NextBoundary, Rollover, PeriodsToClose, FirstAllowance, Grant, Reset, Debit, Crossings, CanAcquire (signatures as in the brief; ParsePeriod returns (year, month, error)).

## Decisions for docs/decisions.md
- nil *time.Location is treated as UTC.
- NextPeriod/PeriodStart return "" / zero time for an invalid period; PeriodsToClose returns nil for an invalid lastOpened.
- PeriodsToClose returns the periods to CLOSE: lastOpened up to but excluding the current period of now (each rollover opens the next one). Empty when up to date or lastOpened is in the future.
- NextBoundary is strictly after now (exactly on a boundary returns the following month).
- Crossings compares x*10 against the allowance, avoiding integer-division rounding of 10%.
- Grant/Reset reason is trimmed only for the blank check; stored as given. Entry period is passed in by the caller and not validated in Grant/Reset/Debit.
- Rollover does not validate periods or allowance sign.
- BUILD.bazel carries the hand-added SPDX header line (as in task 02).

## Confidence and gaps
Verified by running go and bazel. A test skips if tzdata is missing (not the case here). bazel.log in this folder holds the first Bazel run.
