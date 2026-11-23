# Task 74 result: migration 0006 and store-level ledger compaction

Status: done. `go test ./internal/...` passes; `./ci.sh` exited 0 (log: `ci.log` in this folder, "39 tests pass").

## What was built

- **Migration `internal/store/migrations/0006_ledger_rollup.sql`** (ledger change only):
  - `ALTER TABLE ledger ADD COLUMN rollup_count INTEGER` (null for raw rows).
  - Marker table `ledger_compaction (one INTEGER PRIMARY KEY CHECK (one = 1))`: at most one row.
  - `ledger_no_delete` dropped and recreated: aborts `WHEN OLD.type <> 'debit' OR NOT EXISTS (SELECT 1 FROM ledger_compaction)`. `ledger_no_update` and `ledger_no_replace` are unchanged.
  - Two partial indexes over raw debits (`WHERE type = 'debit' AND rollup_count IS NULL AND hold_id IS NOT NULL`): `ledger_raw_debit_created (created_at)` finds the oldest due day and groups one day; `ledger_raw_debit_hold (hold_id, created_at)` lets a step reach one hold's rows of one day directly.
  - 0006 holds the ledger change only. The `preapprovals` table is **not** in it, so task 82 adds it as `0007`.
- **Store** (`internal/store/ledger.go`, `internal/store/ledger_compaction.go`, `internal/store/apiuser_queries.go:150`):
  - `LedgerEntry.RollupCount int` (0 = stored null), `ledgerCols` and `scanLedger` read `rollup_count`.
  - `DebitMultipliers` skips rows with a non-null `rollup_count` (A3, LED-15).
  - `LedgerCompactionCutoff(now, loc)` (`ledger_compaction.go:57`): start of local date today - 7. A raw debit is due when `created_at < cutoff`.
  - `DueLedgerGroups(ctx, now, loc, from, maxRows)` (`ledger_compaction.go:82`): walks local days from `from`. Each day costs two statements on the partial index: the MIN of raw due `created_at` at or after the position, then `GROUP BY user_id, hold_id, period HAVING COUNT(*) > 1` over that day's UTC bounds, which Go computes from the zone. It returns whole groups up to `maxRows` rows (a single larger group is returned alone) and `Next`. `Next` never passes a group that was not returned: it stays at the day start when a day is cut, and it reaches the cutoff when the search is exhausted. Days that hold only one-row groups are skipped. Compacted history is never read.
  - `CompactLedgerGroup(ctx, g)` (`ledger_compaction.go:147`) is a `queries` method, so it works on `*Tx` (it joins the ledger service's transaction, like `AppendLedger`) and on `*Store`. Inside a `SAVEPOINT` it inserts the marker, re-reads the group by its key (raw debits of user, hold, period and `[DayStart, DayEnd)`), deletes them (checking the row count), inserts one row (newest id and created_at, sum, null actor, empty reason, `rollup_count` = n), then deletes the marker. On any error it rolls back to the savepoint, so neither a half-compacted group nor the marker survives, even if the caller commits anyway. It returns 0 and changes nothing when fewer than 2 raw rows remain (idempotent). It never touches `users.balance_mt`.
  - `LedgerCompactionStepRows = 2000`: the suggested bound for task 75.
- Nothing is wired into `internal/ledger` or the collector (task 75).

## Requirement -> test -> file

| Requirement | Test | File |
| --- | --- | --- |
| LED-7 | `TestLED7_CompactsDueRawDebitsOnly` | `internal/store/ledger_compaction_test.go:146` |
| LED-7 | `TestLED7_CutoffIsLocalDateMinusSeven` | same file :241 |
| LED-7 | `TestLED7_DueLedgerGroupsBoundedWholeGroups` | same file :258 |
| LED-7 (A1 index) | `TestLED7_DueSearchUsesRawDebitIndexes` | same file :322 |
| LED-8 | `TestLED8_EveryReaderSameAfterCompaction` | same file :659 |
| LED-9 | `TestLED9_AppendOnlyExceptCompaction` | same file :358 |
| LED-9 | `TestLED9_MarkerEmptyAfterEveryStep` | same file :407 |
| LED-15 | `TestLED15_RolledUpRowHasNoMultiplier` | same file :489 |
| UPG-1 | `TestUPG1_Schema5LedgerUpgradesAndCompacts` | same file :895 |

What the tests cover:

- **LED-7** runs in America/Los_Angeles. Merged: a group from local 00:00 to 23:59 (which crosses the UTC date), a group with a grant and a hold-less debit between its rows (both stay), a per-user split on the same day, and a period boundary inside a local day (one roll-up per period). Never touched: the today - 7 day from its first millisecond, a one-row group, recent rows, and a rolled-up row when late raw rows land on its day (they form their own group). A second run changes nothing, also at 23:59 of the same date. The next local date makes the edge day due. The whole ledger is compared row by row with the expected list.
- **LED-8** uses two periods (plus an empty third), three holds, a mid-day grant, a reset, an expiry, a one-row group and usage samples. The fixture has more than 40 groups.
  - Unchanged, with an exact equality check: `SumLedger` (every type), `RebuildBalance`, `RebuildBalances` (0 changes), `HoldCharges`, `LedgerPeriods`, `LatestAllowancePeriod`, `LatestLedgerEntry`, `LedgerStats`, `CountOverdrawn`, `LedgerPeriodTotals`, `users.balance_mt`, `DebitEntries` summed per local day, and the totals per user/period/type, per hold, and per user/type/local day.
  - Differs only by the merged rows, each asserted exactly:
    - `ListLedger` equals the merged list.
    - `CountLedger` of debits drops by the rows removed.
    - `DebitEntries` (per row) equals the merged list's debits.
    - `DebitMultipliers` loses exactly the merged ids.
    - `BalanceThrough` equals the running sum of the merged list. It is unchanged at the last entry of every local day and at every kept row. At the mid-day grant it is lower by exactly that day's earlier debits (the A3 effect).
  - An `AppendLedger` after compaction keeps the cache equal to the ledger sum.
- **UPG-1:** the database is built with `openAtVersion(…, 5)` and raw SQL. After `Open` it is at version 6, `integrity_check` passes, `balance_mt`, the period sums and `HoldCharges` are identical, every row is raw, a delete is still refused, and the caches need no repair. All of these figures are still identical after compaction.

No existing reader gave an answer after compaction that the design does not allow for (brief item 4): no contradiction found.

## Deliberate breaks (each red, then restored; suite green after)

| Break | Test(s) that went red |
| --- | --- |
| LED-7: cutoff `d-7` changed to `d-6` in `LedgerCompactionCutoff` | `TestLED7_CompactsDueRawDebitsOnly`, `TestLED7_CutoffIsLocalDateMinusSeven`, `TestLED7_DueLedgerGroupsBoundedWholeGroups` |
| LED-7: days cut in UTC instead of the zone (`localDay(at, time.UTC)`) | `TestLED7_CompactsDueRawDebitsOnly` |
| LED-9: trigger condition reduced to `WHEN NOT EXISTS (marker)` (dropped the `OLD.type <> 'debit'` term) | `TestLED9_AppendOnlyExceptCompaction` ("non-debit delete with marker") |
| LED-9: `ROLLBACK TO` the savepoint removed from `CompactLedgerGroup` | `TestLED9_MarkerEmptyAfterEveryStep` (marker committed: 1) |
| LED-15: `AND l.rollup_count IS NULL` removed from `DebitMultipliers` | `TestLED15_RolledUpRowHasNoMultiplier`, `TestLED8_EveryReaderSameAfterCompaction` |

## Existing tests updated (not weakened)

Adding migration 0006 made these literal expectations out of date. Each was extended; no assertion was removed or loosened:

- `store_test.go` `TestMigrateFreshAndReopen`: `schema_migrations` rows 5 -> 6.
- `store_test.go` `TestSchemaColumnsMatchSpec`: `ledger` gains `rollup_count`, and the table `ledger_compaction` (`one`) is added to the expected table list.
- `store_test.go` `TestNFR_Migration0003UpgradesVersion2Database`:
  - version and rows 5 -> 6.
  - The expected ledger index list gains `ledger_raw_debit_created` and `ledger_raw_debit_hold`.
  - Its fixture now writes the version-2 ledger rows with raw `INSERT`s, then `RebuildBalance`, instead of `AppendLedger`. `AppendLedger` reads back `rollup_count`, which a version-2 schema lacks. The test already used raw inserts for holds for the same reason. The resulting caches are identical (the 2026-11 sums), and every later assertion is unchanged.
- `migrate0004_test.go` `TestD40_Migration0004DropsResourceNameOverride`: version and rows 5 -> 6.
- `integration_queries_test.go` `TestMigration0005UpgradesVersion4Database`: version 5 -> 6.
- The existing `TestLED_TriggersRejectUpdateDelete` (every ledger delete fails) passes as written.

## Files changed

- Added: `internal/store/migrations/0006_ledger_rollup.sql`, `internal/store/ledger_compaction.go`, `internal/store/ledger_compaction_test.go`.
- Modified: `internal/store/ledger.go`, `internal/store/apiuser_queries.go`, `internal/store/BUILD.bazel` (edited by hand; the Gazelle check in ci.sh passes), and the tests `internal/store/store_test.go`, `internal/store/migrate0004_test.go` and `internal/store/integration_queries_test.go`.
- No file outside `internal/store/` needed a change (`LedgerEntry` literals are keyed).

## Commands run

- `go vet ./internal/...` and `go test ./internal/...`: pass.
- Deliberate-break runs: see the table above.
- A throwaway timing test, deleted afterwards: 30 holds × 60 days of 5-minute debits (518k rows), compacted with bound 2000. Worst step 46 ms, worst `DueLedgerGroups` call 10 ms, 300 steps.
- `./ci.sh > ci.log`: exit 0.

## Decisions for docs/decisions.md

1. Migration 0006 holds the ledger change only: `rollup_count`, the marker table, the new `ledger_no_delete` and two partial indexes. The `preapprovals` table goes in `0007` (task 82).
2. The marker table is `ledger_compaction`, with a single column `one INTEGER PRIMARY KEY CHECK (one = 1)`, so it can never hold more than one row. A marker committed by mistake would make the next step fail on the UNIQUE constraint rather than silently allow deletes.
3. The index shape is two partial indexes over raw debits (`type = 'debit' AND rollup_count IS NULL AND hold_id IS NOT NULL`):
   - `(created_at)`, to find the oldest due day and group one day;
   - `(hold_id, created_at)`, for a step's re-read and delete of one group (forced with `INDEXED BY`).
   Both hold only raw debits, so after the backlog they cover about a week of rows plus the one-row groups that remain.
4. The search walks by local day. The store takes `now`, the zone and a resume position `from`. Go computes each day's UTC bounds, because SQLite cannot convert to a named zone. `Next` lets the caller resume without rescanning, and one-row groups are skipped.
5. The suggested step bound is 2,000 rows (`store.LedgerCompactionStepRows`): whole groups up to 2,000 rows, a larger group alone. Measured at 46 ms or less per step on a 518k-row ledger. Task 75 confirms it with the scale test.
6. A compaction step is wrapped in a SAVEPOINT, so it is atomic and leaves no marker even inside a caller's transaction that commits after a failed step.
7. `LedgerEntry.RollupCount` is an `int`, with 0 for a stored null, rather than a pointer.
8. A rolled-up row keeps `hold_id`, `user_id`, `period` and the newest row's `created_at`. It has a null `actor_id`, an empty `reason`, and type `debit`.

## Confidence and gaps

- Verified by tests: everything in the requirement table. The marker is never visible to another connection; this is inferred from SQLite transaction isolation and was not tested directly.
- `DueLedgerGroups` loops over days without a cap on the number of days with only one-row groups visited in one call. Each such day costs two indexed statements; a year of them appears to cost tens of milliseconds (inference; not measured separately).
- A cursor held in memory by the service resets on restart. Days with only one-row groups are then walked once again, which is cheap.
- The 46 ms step time was measured on this machine, inside `go test`. The scale test of task 75 is the real check.
- The `INDEXED BY` clauses make a step fail loudly if the partial index is ever dropped or its predicate changed. That is intended, but a future index change must keep the predicate.
