# 09b fix store: RESULT
Status: done. All checks pass: go test -race, CGO_ENABLED=0 go test, bazel test //internal/store/..., go build ./..., tools/check_core_generic.sh, tools/check_spdx.sh.

## Changed signatures (internal/store, methods on Store and Tx)
- `AppendLedger(ctx, e LedgerEntry, currentPeriod string, now time.Time) (LedgerEntry, error)` (was `(ctx, e, now)`). Sets users.balance_mt to SumLedger(user, currentPeriod), whatever e.Period is.
- `RebuildBalance(ctx, userID int64, currentPeriod string) (int64, error)`: same shape; the parameter is now documented as the current period.
- `Store.Tx(ctx, fn)`: unchanged signature; a panic in fn rolls back, releases the connection, re-panics.

## Changes
- migrations/0002_ledger_no_replace.sql: BEFORE INSERT trigger aborting when NEW.id already exists (blocks REPLACE / OR REPLACE / OR IGNORE on an existing id). 0001 untouched. BUILD.bazel embedsrcs lists it.
- store.go: Tx panic safety; FormatTime comment (UTC, millisecond, fixed width 24, truncated).
- store_test.go: new TestLED1_RejectsInsertOrReplace, TestStoreTxPanicRollsBackAndReleases, TestLED2_CacheIsCurrentPeriodForAnyEntryPeriod (late expiry, late past grant); callers updated; schema_migrations count now 2.

## Decisions for docs/decisions.md
AppendLedger/RebuildBalance take the current period from the caller (timezone-aware).

## Confidence and gaps
Not tested: files outside internal/store (no other callers exist).
