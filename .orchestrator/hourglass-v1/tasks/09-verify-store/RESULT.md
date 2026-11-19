# 09 verify internal/store: RESULT
Status: done. Verdict: PASS WITH FINDINGS. The schema matches the spec exactly and nearly every behaviour holds, but there are two real defects (ledger append-only bypass, Tx panic leak) and one design defect (balance cache depends on call order). Fix 1-3 before building the metering and rollover code on top.

## Findings
1. HIGH, migrations/0001_init.sql:79-87 (ledger triggers). `INSERT OR REPLACE INTO ledger (id, ...)` with an existing id silently overwrites the row: REPLACE deletes without firing the DELETE trigger (recursive_triggers is off). Verified on a raw handle: amount_mt rewritten from 5000 to 1. UPDATE, DELETE, UPDATE OR REPLACE and INSERT..ON CONFLICT DO UPDATE are all rejected. Fix: add `BEFORE INSERT ... WHEN EXISTS (SELECT 1 FROM ledger WHERE id = NEW.id)` abort trigger, or set `_pragma=recursive_triggers(1)`. Repro: evidence/zz_verify_test.go.txt TestZZLedgerBypass.
2. HIGH, store.go:105-115 (`Store.Tx`). A panic inside fn is not recovered: no rollback, the pooled connection stays in BEGIN IMMEDIATE holding the write lock. After a recovered panic the next write blocked beyond 5 s (it will fail with SQLITE_BUSY after the 10 s busy timeout, and every later write behaves the same). Error returns roll back correctly. Fix: `defer` that rolls back and re-panics. Repro: TestZZTxRollbackAndPanic.
3. MEDIUM, ledger.go:43-62 (AppendLedger). Cache is set to the sum of the appended entry's period, not the current period. See "Balance cache" below. Repro: TestZZBalanceOrders.
4. LOW, store.go:41 `FormatTime`. Timestamps are truncated to milliseconds, not rounded; a time with sub-ms precision does not round-trip (999.999999 ms becomes .999). Harmless for 5-minute polls; callers must not rely on nanoseconds. A non-UTC input is correctly converted to UTC and sorts lexically (all values 24 chars).
5. LOW, holds.go:112 `ActiveHolds`: query plan is `SCAN holds` (the partial unique index is keyed on setup_id, so it cannot serve `ORDER BY id`). 0.10 ms at 100 holds; it grows with all-time hold history, but is acceptable for v1.
6. INFO, `rules.value` is REAL and `rules` has an extra index `rules_kind_position`; the spec says value is the weight, so REAL is consistent. Not a defect.

## Check 1: tests
`go test -race`: 20 top-level tests PASS. `CGO_ENABLED=0 go test`: 20 PASS. `bazel test //internal/store/...`: 1 test target PASSED (0.3 s). No failures.

## Check 2: schema versus spec
All 15 spec tables and every named column exist (queried via pragma_table_info from a fresh database; the package's own TestSchemaColumnsMatchSpec also asserts exact column counts). Keys and indexes: users.oidc_sub UNIQUE; setups.name UNIQUE; setup_labels PK(setup_id,key) plus index (key,value); user_roles PK(user_id,role); holds partial unique index on setup_id WHERE released_at IS NULL; usage_samples UNIQUE(hold_id,polled_at); ledger and sessions as specced. Beyond the spec, besides outages and schema_migrations: no extra tables and no extra columns. Extra indexes only, all justified: rules_kind_position, holds_user, usage_samples_polled_at (prune and range scans), ledger_user_period, event_deliveries_started_at (prune), audit_log_created, sessions_user. Extra CHECK constraints (status, role, type, period GLOB, booleans) are justified and match the spec's enumerations. Primary keys the spec does not name: pressure_samples.polled_at and orphans.resource_name, settings.key (natural keys; fine, but two pressure samples in the same millisecond would conflict).

## Check 3: behaviour (my throwaway tests, evidence/)
- Pass: foreign_keys=1, journal_mode=wal and busy_timeout=10000 on 12 distinct pooled connections.
- Pass: ledger UPDATE and DELETE rejected via raw handle (but see finding 1).
- Pass: duplicate (hold_id, polled_at) returns inserted=false, one row kept, first debit unchanged.
- Pass: 20 rounds of two goroutines creating a hold on one setup: exactly one wins, the other gets ErrSetupHeld.
- Pass: duplicate setup name gives ErrConflict on create and on update; deleting a setup with labels cascades them away; a setup with an active or a released hold returns ErrConflict; missing id gives ErrNotFound; re-holding after release works.
- Pass: reopen is idempotent (one schema_migrations row); a database at version 99 fails with "newer than this binary"; a bad path fails cleanly.
- Pass: Tx returning an error rolls back. FAIL: Tx that panics (finding 2).
- Pass: 20 reader goroutines against a 1.5 s write transaction produced no SQLITE_BUSY.

## Check 4: balance cache
`AppendLedger` inserts, then runs `RebuildBalance(user, e.Period)` which sets `users.balance_mt` to that period's sum. So after any call the cache equals the sum of the period of the most recently appended entry, and equals the current period's sum only if that entry was in the current period (or the two sums coincide).
- LED-3 in the specified order (expiry in old period, then carryover, then allowance in the new period): correct at the end (expiry leaves 0, allowance sets 300000). Intermediate states are also consistent for a positive balance (0 until allowance); for a negative balance, the carryover sets the cache to the debt, and the allowance then fixes it.
- Wrong whenever the last append targets a non-current period: (a) a late correction (grant, debit) written to a past period: cache became 7000 while the November sum was 300000; (b) an expiry (old period) written after carryover/allowance or after a new-period debit: cache became 0 while November sum was 299000; (c) a retried or resumed rollover job that re-appends the old-period expiry last. It is repaired only by the next append in the current period or an explicit RebuildBalance(current).
- Recommended smallest change: give AppendLedger a `currentPeriod string` argument (caller computes it from the configured timezone and `now`) and rebuild that period instead of e.Period; entries for other periods then never move the cache. Same-signature alternative: keep it and require callers to call RebuildBalance(current) afterwards, which is easy to forget.

## Check 5: scale (200 users, 100 setups, 100 active holds, 312k samples incl. one user with 112k; ANALYZE run; best of 5)
DailyDebits one user, one month: 0.06 ms (user with 2k samples), 3.21 ms (112k-sample user). SumDebitsSince: 0.24 ms / 11.15 ms. ActiveHolds 0.10 ms. ListSetups (100 with labels) 0.16 ms. Plans: usage uses the unique autoindex (hold_id, polled_at range) joined from holds_user; temp b-tree for GROUP BY only. No missing index.

## Check 6
check_core_generic.sh OK (40 core files, no violations); check_spdx.sh exit 0. Non-test imports: context, database/sql, embed, errors, fmt, net/url, sort, strconv, strings, time, modernc.org/sqlite and modernc.org/sqlite/lib. Tests import only stdlib.

## Check 7: mutations (each reverted; package passes again)
Partial index made non-partial: TestHOLD_CreateReleaseAndOnce red. Delete trigger removed: TestLED_TriggersRejectUpdateDelete red. Sample ON CONFLICT removed: TestMET5_UsageSampleIdempotent red. setOnce overwrite allowed: TestHOLD_CreateReleaseAndOnce red. Cache rebuilt for the wrong period: TestLED2_BalanceCacheMatchesLedger red. Note: the existing suite does not cover findings 1-3 (no REPLACE test, no panic test, no past-period append test).

## Decisions for docs/decisions.md
None made by me. Suggest recording: AppendLedger cache semantics (finding 3), millisecond timestamp truncation.

## Files changed
Only this folder: RESULT.md, PROGRESS.md, evidence/*.go.txt (throwaway tests). Clone: /private/tmp/claude-501/-Users-tejaspanse-projects/17dec24c-e7ec-455e-9b87-01d2217d5fc9/scratchpad/verify-09/hourglass (product source restored to original after mutations).

## Confidence and gaps
Not checked: users/rules/misc.go CRUD beyond the builder's own tests; behaviour under file-system failure; very long busy contention (writers). Scale run used synthetic uniform data in one machine's page cache.
