# 76 Independent verification of milestones 2 and 3: result

**Verdict: PASS with findings.** LED-7, LED-8, LED-9, LED-15 and UPG-1 are met and have
tests named after their ids that went red when I broke the code. LED-10 is met in
practice and its named tests went red under three breaks, but one part of it, "delays
no poll past the poll interval", is not tested on the path production uses (M1). There
are no high findings, 1 medium and 4 low. `git diff` is empty at the end (checked with
`git diff --quiet`). I did not read the task 74 and 75 PROGRESS/RESULT files.

## Findings

| id | sev | file:line | failure scenario | suggested fix |
| --- | --- | --- | --- | --- |
| M1 | medium | `internal/collector/collector.go:173-186`, `:225-253`; test `internal/collector/compact_led10_test.go:202` | In production the only poller is `Run`'s own loop (`internal/app/app.go:276`). Nothing else calls `RunOnce`, so releasing `runMu` between steps never lets a poll in. The next poll waits for the whole cycle's compaction and then the full interval: the gap between polls is interval + poll + up to 20 steps. D71(a) sets no time budget. Measured locally the worst step is 20 ms (about 0.4 s per cycle), so there is no real harm today. On slow storage, steps near the design's one-second limit add about 20 s to every cycle while a backlog lasts, which puts polls literally "past the poll interval". MET-4's 2× limit (2 min at the 1-min minimum) is only reached if steps exceed about 3-6 s. `TestLED10_StepsReleaseTheLockBetweenThem` proves the lock is released by starting a second, concurrent `RunOnce`, which production never does. No test checks how long `Run` waits between polls while a backlog exists. | Count the time spent in the cycle against the wait: compute `After(interval - elapsed)` from the cycle start. Or stop `compactIfDue` once it has run for a small share of the interval. Add a test in which steps advance the fake clock and assert the duration `Run` passes to `After`. |
| L1 | low | `internal/store/ledger_compaction.go:92-130`; `internal/ledger/service.go` (`compactFrom` reset) | One-row groups are never compacted, so they stay raw and stay in both partial indexes forever. For example, a hold polled once before midnight is a one-row group. A day that holds only one-row groups does not count toward `maxRows`, so one step walks every such day (2 statements per day) inside one IMMEDIATE transaction. Every daily idle check starts again from the beginning. Cost grows with history and has no bound. I measured it with the sqlite3 CLI on a scratch DB: 3 years × 12 holds with one-row groups every day is 1,096 days walked in about 50 ms, so it is harmless at realistic sizes. D70(c)'s "the indexes cover about a week of rows" is not exact. | Optional: count walked days toward the step bound, so a step ends after N days even with no rows. Correct D70(c). |
| L2 | low | `internal/api/perf_compaction_scale_test.go:200-203`; `internal/store/store_test.go:843-846` (pre-existing) | Timing assertions. Under `-race` the scale test fails, but only on the 500 ms page bound before compaction (1.7 s and 2.3 s). Step bounds still hold (worst 541 ms under race). In my first uncached `bazel test //...` run, `TestNFR_LedgerAppendTimeIsLinear` failed (212 ms → 476 ms) under parallel load. It passed 5/5 alone and in a second uncached full run. That test appends hold-less debits, which are outside the new partial indexes, so the flake appears unrelated to this change (inference). Risk: an occasional red `./ci.sh` on a loaded machine. | Leave it, or relax/skip the timing checks under `-race` and on a loaded host. Re-run before concluding a regression. |
| L3 | low | `docs/decisions.md` D70(b); `internal/store/migrations/0006_ledger_rollup.sql:10-22` | D70(b) says a marker committed by mistake "would make the next step fail instead of silently allowing deletes". In fact a committed marker would also let every connection delete any debit row, because the trigger only checks `EXISTS`. I found no code path that can commit the marker: success, a failed step and a caller that ignores the error and commits are all covered by tests and by my breaks. So this is an inaccurate rationale, not a live bug. | Reword D70(b). Optionally have the collector check at start that `ledger_compaction` is empty and log an error if not. |
| L4 | low | `internal/store/ledger_compaction_test.go:895`, `:146`, `:659` | Coverage gaps, no defect found. (a) UPG-1 builds schema 5 from the embedded migrations 0001-0005 plus raw inserts, not from a database written by the `d96a950` binary. Those migration files are unchanged since `4e8686e` (diff stat), so the schema is the same, but rows written by old code paths are not exercised. (b) DST is covered only by the fall-back day (Nov 1 2026, inside the LED-8 fixture), not by a spring-forward day. (c) No test cuts the same day in two timezones across a timezone change. | Optional: add a spring-forward day to the LED-8 fixture. |

What I tried that did **not** fail (no finding):

- **Group key.** `user_id, hold_id, period` plus local-day bounds `[start, end)` computed in Go, in the same way in the search and in the step, so DST days are consistent.
- **Kept row.** Id is `MAX(id)` and `created_at` is read from that row; actor is NULL, reason `''`, `rollup_count` is the count.
- **Edge and finality.** The 7-day edge is `time.Date(y, m, d-7)` in the configured location. One-row groups and rolled-up rows are never touched.
- **Atomicity.** A whole step is one IMMEDIATE transaction with a per-group SAVEPOINT. If the context is cancelled, `ROLLBACK TO` can fail, but the outer `Store.Tx` then rolls back, so nothing is half done.
- **Marker table.** Only `compactGroup` inserts into it (grep over `internal/` and `cmd/`).
- **Triggers.** `ledger_no_update` and `ledger_no_replace` are unchanged, and no table has a foreign key to `ledger`.

## Requirement → test → break made, went red

| req | tests | break made (restored with `git checkout -- <file>`) | red? |
| --- | --- | --- | --- |
| LED-7 | `TestLED7_CompactsDueRawDebitsOnly`, `TestLED7_CutoffIsLocalDateMinusSeven`, `TestLED7_DueLedgerGroupsBoundedWholeGroups`, `TestLED7_DueSearchUsesRawDebitIndexes` | (1) cutoff `d-7` → `d-6` in `LedgerCompactionCutoff` | yes: 3 LED7 tests |
| | | (2) kept id `MAX(id)` → `MIN(id)` in `compactGroup` | yes: LED7_CompactsDue…, LED8 |
| LED-8 | `TestLED8_EveryReaderSameAfterCompaction` | (1) group sum `SUM` → `MIN*COUNT` | yes: 9 reader comparisons |
| | | (2) inserted `created_at` = day start instead of the newest row's | yes: ListLedger differs |
| LED-9 | `TestLED9_AppendOnlyExceptCompaction`, `TestLED9_MarkerEmptyAfterEveryStep` | (1) trigger without `OLD.type <> 'debit'` | yes: non-debit delete with marker |
| | | (2) marker not deleted at the end of the step | yes: marker after step 0 |
| | | (3) no `ROLLBACK TO` savepoint on error | yes: marker after failed step (committed) |
| LED-10 | `TestLED10_StepsReleaseTheLockBetweenThem`, `_BacklogFinishesOverLaterCycles`, `_FailedStepFailsNoPollAndIsRetried`, `_IdleCheckRunsOnceADay` (collector); `_CompactDueStepsAreBounded`, `_CompactDueAgainWhenNothingDueChangesNothing`, `_CompactDueFailedStepChangesNothingAndIsRetried` (ledger); `_ScaleYearOfDebitsCompactsInBoundedSteps` (api) | (1) `runMu` held across all of a cycle's steps | yes: StepsReleaseTheLock |
| | | (2) per-cycle step budget removed | yes: BacklogFinishesOverLaterCycles |
| | | (3) a failed step clears `compactPending` (no retry) | yes: FailedStepFailsNoPoll… |
| LED-15 | `TestLED15_RolledUpRowHasNoMultiplier` | `AND l.rollup_count IS NULL` removed from `DebitMultipliers` | yes: LED15 and LED8 |
| UPG-1 | `TestUPG1_Schema5LedgerUpgradesAndCompacts` | not broken (the brief asks for breaks for LED-7 to LED-10 only); read: starts from `openAtVersion(…, 5)`, which really runs migrations 0001-0005, then adds 2 users, 3 holds, a month of hourly debits and a grant with raw SQL, then calls `Open` → version 6 | n/a |

Logs of every break: `break-led7.log`, `break-led7b.log`, `break-led8.log`, `break-led8b.log`,
`break-led9.log`, `break-led9b.log`, `break-led9c.log`, `break-led15.log`, `break-led10a.log`,
`break-led10b.log`, `break-led10c.log` in this folder.

## Other checks from the brief

- **A3 readers.** I grepped every SQL statement on `ledger` in `internal/` (store only; no SQL on `ledger` elsewhere). Readers and their behaviour after compaction:
  - `ListLedger`, `SumLedger`, `RebuildBalance(s)`, `HoldCharges`, `DebitEntries`, `LedgerPeriods`, `CountLedger`, `LatestAllowancePeriod`, `LatestLedgerEntry`, `BalanceThrough`, `LedgerStats`, `CountOverdrawn` and `LedgerPeriodTotals` are all snapshotted by the LED-8 test.
  - The `AppendLedger` "earlier entry" check (`id < ?`) keeps working because ids are preserved.
  - `DebitMultipliers` is the only join on `created_at`, and it is fixed (LED-15).
  - `CountLedger` counts rows but is only called with `allowance` (`internal/ledger/service.go:335`, `:379`).
  - `SumDebitsSince` (24 h burn) reads `usage_samples`, not the ledger.
  - `getMyUsage` buckets `DebitEntries` by local day over period bounds that are local midnights, so its sums are unchanged.
  - Nothing was missed.
- **Due-group search plan.** I checked it myself with the sqlite3 CLI on a scratch DB: migrations 0001-0006, 200,001 debits of which 150,000 were rolled up, then `ANALYZE`.
  - The `MIN` query and the group query: `SEARCH ledger USING INDEX ledger_raw_debit_created (created_at>? AND created_at<?)`.
  - The step's delete: `SEARCH … USING INDEX ledger_raw_debit_hold (hold_id=? AND created_at>? AND created_at<?)`.
  - There is no scan. The scale test also proves the plan check fails without the indexes.
- **Scale test** (no race, CI size): 315,789 debits, 179 steps, worst step 20.0 ms, idle check 0.0 ms, ledger page 51-68 ms before compaction and 0.7-12 ms after.
- **Races.** `go test -race -count=1 ./internal/store ./internal/ledger ./internal/collector`: pass (`race.log`). The api LED tests under `-race` fail only on timing (L2, `race-api.log`).
- **Edits to existing tests** (`git diff 4e8686e..HEAD -- '*_test.go'`). None weakens an assertion.
  - The schema version and migration-count bumps 5 → 6 are required.
  - The expected ledger index list gains the two new indexes, which is stricter.
  - The schema column map gains `rollup_count` and `ledger_compaction`.
  - `TestNFR_Migration0003UpgradesVersion2Database` now writes its schema-2 ledger rows with raw SQL plus `RebuildBalance` instead of `AppendLedger`. That is necessary, because `AppendLedger` now reads back `rollup_count`, which does not exist at schema 2. The rows and cache values are the same, and the post-upgrade assertions are unchanged, including the exact `300000-1670+1234` after an append.
- **`./ci.sh`**: exit 0 (`ci.log`, `ci.exit`), but Bazel served all 39 test targets from cache ("Executed 0 out of 39"). I then ran `bazel test //... --nocache_test_results` twice:
  - Run 1: 39 executed, 38 passed, `store_test` failed on the timing flake L2 (`bazel-nocache.log`).
  - Run 2: **39 out of 39 executed, 39 passed**, exit 0 (`bazel-nocache2.log`).
- **`tools/check_core_generic.sh`**: OK (169 core files, 183 Go files, no violations).

## Note for later milestones (not a finding against M2/M3)

The acceptance scenario's month jump moves the clock about 12 days forward. The first
cycle after the jump runs the daily prune, so compaction will merge the pre-jump days'
debits (any day earlier than the new today - 7). Any end-to-end assertion that re-reads
single debits of those days after the jump would then see roll-ups. Worth checking in
milestones 5 and 8.

## Commands run

- `go test -race -count=1` on store, ledger and collector (pass).
- `go test -race -count=1 -run 'LED10|LED7|LED8' ./internal/api/` (timing fail only).
- `go test -count=1 -run LED10_Scale -v ./internal/api/` (pass).
- 11 break-and-run cycles (above).
- 5× `TestNFR_LedgerAppendTimeIsLinear` alone (pass).
- `./ci.sh` (exit 0, cached).
- `bazel test //... --nocache_test_results` ×2.
- `tools/check_core_generic.sh`.
- The sqlite3 CLI plan and one-row-group walk checks on scratch DBs in the session scratchpad.

## Files changed

Only this task folder: PROGRESS.md, RESULT.md and the logs. No source file is changed;
`git diff --quiet` succeeds.

## Confidence and gaps

- I did not run the end-to-end suites; they are out of scope.
- M1's impact on slow storage is inference from the code and the measured step times. I did not reproduce it on slow disks.
- I did not break UPG-1, `TestLED10_CompactDueAgainWhenNothingDueChangesNothing` or `TestLED7_DueSearchUsesRawDebitIndexes` individually. The last of these the scale test backs with a check that the plan fails without the index.
- The CLAUDE.md token rule on `git diff` applies to user-requested reviews in the main session. This brief explicitly gave the diff as input, so I read the code directly.

## Decisions for docs/decisions.md

None. A correction to D70(b) and D70(c) is suggested in L3 and L1.
