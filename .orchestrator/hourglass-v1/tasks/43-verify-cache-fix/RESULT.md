# RESULT 43: re-verify the balance cache after fix 42b

**Verdict: cache correct: yes, as the application runs it.** After 42b the cache equals the current period's ledger sum after every step I could construct once no rollover is pending. That covers 12,000 seeded-random operations, the multi-month rollover with a current-month entry, both timezone moves, a database from before migration 0003, and concurrent rebuilds with writes. Acquire gating followed the ledger in all 602 random checks and in the fixed checks.

Two caveats keep this from being unconditional:
- The original v42 timezone-backwards test still fails as written. Its simulated restart skips the start-up rebuild that `app.Run` performs, and the period-change guard is switched off until that rebuild has run (finding 1).
- The pending-rollover window documented as D-A remains.

The fix introduced nothing that breaks the invariant. Finding 3 below is a ledger problem that predates 42b, not a cache problem.

Clone: `<scratch>/verify-43/hourglass` at `326cc7d` (product tree identical to `04089db`). Mutations were run in a second clone, `<scratch>/verify-43/mut`. `<scratch>` = `/private/tmp/claude-501/-Users-tejaspanse-projects/17dec24c-e7ec-455e-9b87-01d2217d5fc9/scratchpad`. No product code was changed; every mutation was reverted with `git checkout`.

## Findings

| # | Severity | Finding | Reproduction | Suggested fix |
|---|---|---|---|---|
| 1 | Low | The period-change rebuild (42b mechanism b) is disabled while `cachePeriod == ""`, which is its state until `RepairBalances` or a rollover has run (`internal/ledger/service.go:351`: `done != "" && cur != done`). Production is safe only because `app.Run` calls `RepairBalances` before the collector starts (`internal/app/app.go:198`). Any other way of building a `ledger.Service` loses the guard: tests, a future CLI, or a second process. The original v42 test `TestV42_TimezoneChangeBetweenRestarts/backwards` still fails for this reason: the cache stays 1,670 mt under the ledger permanently, as it did before 42b. | `evidence/check1-original-v42.log`. Adding the start-up call to the same test makes it pass with a hard check after every poll (`evidence/v43_tzvariant_test.go.txt`, `TestV43_TimezoneOrigWithStartupRepair`). | Change the condition to `cur != done`, so an unknown period counts as moved. That costs one rebuild at each process's first poll, about 60 ms at full scale. I tried this in the mutation clone: all 14 original v42 cases pass except the D-A window check, `TestV42_RollbackLeavesCache` (see finding 2), and the repository suite stays green. |
| 2 | Info | The original `TestV42_RollbackLeavesCache` fails. Its last hard check (`e.check("rollbacks")`, evidence line 424) runs at Dec 1 00:01 with a rollover pending, which is the documented D-A window (task 42 finding 7). The test's own comment says the mismatch is expected. It failed on `99408df` and on the pre-fix commit too. | `evidence/check1-original-v42.log` | None needed beyond D-A. If the window is ever to be closed, run `RolloverDue` inside the request path. |
| 3 | Low (pre-existing, not caused by 42b) | `RolloverDue` is not idempotent when two callers run it at once. Both read `last` outside the transaction. The second transaction skips the allowance (`has > 0`) but writes the negative carry-over a second time, so a user closing at -50,000 starts December at 200,000 instead of 250,000. The cache equals that wrong ledger. The product serialises callers with `Collector.runMu`, and the collector path passes, so this is reachable only with two processes on one database, for example an overlapping rolling deploy. | `evidence/v43_startup_test.go.txt`, `TestV43_TwoGoroutinesRolloverDue` (fails, by design of the check). Same code on `99408df` (`old/internal/ledger/service.go:276-304`). | Inside each period's transaction, re-read `LatestAllowancePeriod` and skip the period if `next` is already opened. Alternatively, skip all of a user's rollover entries when `has > 0`. |
| 4 | Info | A restart during a pending rollover window now sets every cache to the new period's sum (usually 0). Acquire is then refused with "balance zero or below" until the first poll's rollover commits; that poll runs immediately at start. If that rollover fails, `RunOnce` logs and continues (`collector.go:210`), and the caches stay at 0 until a later rollover succeeds. Without a restart, the same window shows the old balance. Both behaviours are consistent with the ledger; they differ from each other. | `TestV43_StartupRebuildPre0003Database/rollover_pending` (repair changed 11 caches to the November sum, 0). The original v42 `forwards` log shows the same. | Optional: in `app.Run`, run `RolloverDue` before `RepairBalances`, or note it in D-A. |
| 5 | Info | Mechanism (a), the current-period argument in the rollover, is redundant while the end-of-rollover rebuild exists. Reverting it leaves everything green, and no scenario exists where it alone protects correctness, because the rebuild runs in the same transaction before commit. It is still useful: without the rebuild, it alone keeps role holders correct (see the mutation table). | Mutation M1 below | Keep it, since it costs nothing. Not a defect. |

## Check 1: verification 42's original evidence tests

`cache_led2_test.go.txt` was copied unchanged to `internal/collector/v42_orig_test.go`. The adopted `cache_led2_test.go` was moved aside because the helper names collide. Command: `go test -count=1 -v -run TestV42_ ./internal/collector`.

| Test | Result |
|---|---|
| DebitsWithinMonth | PASS |
| RolloverPositiveZeroNegative | PASS |
| DownAcrossTwoBoundaries | PASS (soft lines only in the pending window) |
| **DownAcrossTwoBoundaries_GrantBeforeFirstPoll** (multi-month plus current-month entry; was High) | **PASS** (was FAIL in task 42) |
| OneBoundary_GrantResetBeforeAndAfterRollover | PASS |
| PollExactlyAtMidnight | PASS |
| LatePastPeriodEntry | PASS |
| FirstRoleMidMonthAndDuringPendingRollover | PASS |
| RolesRemovedAndRestored (was Low finding 3) | PASS (was FAIL) |
| ConcurrentAppends same=true / same=false (1,000 debits per writer) | PASS / PASS |
| RollbackLeavesCache | FAIL: D-A window check, finding 2 (also failed before) |
| TimezoneChangeBetweenRestarts/backwards | FAIL as written (harness restart skips the start-up rebuild; finding 1). PASS once the restart calls `RepairBalances` as `app.Run` does, with a hard check after every poll. |
| TimezoneChangeBetweenRestarts/forwards | PASS |

## Check 2

**(a) Randomised** (`evidence/v43_random_test.go.txt`, `TestV43_RandomisedCacheEqualsLedger`, seeds 43000-43199). The setup per seed is 5 users with setups and holds, a weight-2 rule, enforcement on, and a random start date. Each of 60 operations is drawn uniformly from: poll with debits (random pressure band), grant, negative grant, reset, role removed (holds released first, as the API requires), role restored (plus `EnsureFirstAllowanceTx`, as the API does), +1-120 minutes, +1-10 days, +1-3 months, restart (new ledger service and collector over the same store, then `RepairBalances`), and failed poll (injected `DebitTx` error).

- After every operation, every non-admin user, with or without a role, is checked against the ledger. When no rollover is pending, any cache != sum is a hard failure. When one is pending, a mismatch is accepted only if the user has no entry in the current period and the cache equals the sum of a period between the last opened one and the current one (the D-A window). Anything else is reported as "unexplained".
- Each restart asserts that every (user, period) ledger sum is unchanged.
- Result: **200 seeds, 0 failed; 12,000 ops; 0 hard and 0 unexplained mismatches.** That included 1,150 restarts, 1,122 failed polls and 387 polls that ran a rollover. The 11,489 window mismatches come from the frequent clock jumps without a poll. Runtime is about 7-11 s.
- The test is not vacuous: it goes red on 143 of 200 seeds when the end-of-rollover rebuild is reverted.

**(b) The rebuild paths added in 42b**
- **Inside the transaction?** The rollover rebuild runs inside each period's write transaction, after its appends: `tx.RebuildBalances` joins the caller's transaction (`store.go:207`, `atomic` with `root == nil`). The period-moved rebuild (`service.go:352`) is its own transaction, a SELECT plus a single `UPDATE … FROM` with `_txlock=immediate` (`store.go:124`). It is not next to any other write.
- **Half-rebuilt state visible?** No, because the database runs in WAL mode (`journal_mode(WAL)`) and readers see only committed snapshots, and the UPDATE is one statement in one transaction. `TestV43_RepairConcurrentWithWrites` ran 3 goroutines × 100 `RepairBalances` against grants, resets and debits, and a reader took 100 consistent snapshots without a mismatch. Between the per-period transactions of a multi-period close, the cache equals the current period's ledger sum of the moment, which is the window state, not a half-rebuild.
- **Two goroutines at once?** Through the product, no: the only caller is `Collector.RunOnce` under `runMu` (`collector.go:206`), and that path passes. Called directly, concurrent rebuilds are harmless (idempotent). Concurrent `RolloverDue` double-writes negative carry-overs (finding 3, which predates 42b).
- **Cost per poll:** a mutex and a string compare. The rebuilds run only on a rollover or a period move. The full-size scale test passed (`evidence/perf-fullsize.log`): the worst GET was `/admin/users` at 192.2 ms; the collector poll over 100 holds took 196.5 ms; the rebuild of 200 caches over 518,400 ledger rows took 60.2 ms with 0 changed.

**(c) Start-up rebuild** (`evidence/v43_startup_test.go.txt`, log `evidence/check2bc.log`)
- With 200 users, 18,600 rows and 100 caches corrupted, the rebuild took 1.96 ms. It reported changed 100, and the abs and net deltas exactly matched the values computed independently. Every cache equals its sum. A second run took 0.9 ms with changed 0. At full scale (518,400 rows) it takes 60.2 ms (above).
- On an empty database, and on one holding only the local admin, it returns no error and changes nothing.
- For the pre-0003 database, I built one with `99408df` (schema v2: 13 users including positive, negative, role-less and pending; 96 rows) and opened it with the current code, which migrates it to v3. Three cases:
  - Started in the same period: changed 0.
  - Started with one boundary pending: 11 caches set to the November sum.
  - Started with two boundaries pending: same.

  In every case the first poll ran the rollover, cache == sum held after the start, the first poll and the second poll, and the closing values were right (for example, Oct -50,000 became Nov 250,000; Oct positive became 300,000; the role-less user became 0).

## Check 3: Acquire gating

This uses the real `holds.Service.Acquire` with enforcement on.
- **Random sequences:** at the end of each sequence, every role holder tried to acquire a fresh setup. 602 checks, 0 wrong: allowed iff the ledger sum is > 0, or, in a pending window, iff the cache is > 0 (D-A).
- **Fixed scenarios** (`TestV43_GatingAfterOriginalScenarios`): September; after the two-boundary rollover with a December grant; exactly zero; -1 mt. All correct; zero and below are refused.
- **Gap:** check 3 was not wired into the 14 original v42 tests individually. Their end states are covered by the cache invariant plus `AcquireBlock`, which reads only the cache (`holds.go:204`, `:262`).

## Check 4: repository suite

| Command | Outcome |
|---|---|
| `go test -race -count=1 ./...` (pristine clone) | exit 0, 30 packages ok (`evidence/gotest-race.log`) |
| `./ci.sh` (pristine clone) | exit 0, 35 of 35 Bazel tests pass (`evidence/ci.log`) |

## Check 5: mutations

| Mutation | Repository tests red | Task 43 tests red |
|---|---|---|
| M1: rollover appends with `next` instead of `cur` (`service.go:319`) | none | none. Redundant given the rebuild (finding 5) |
| M2: no `RebuildBalances` at the end of the rollover (`service.go:328`) | `TestLED2_LED3_RolesRemovedAcrossBoundaryCacheFollowsLedger` only | random 143/200 seeds; all 215 hard failures are **role-less** users (`evidence/m2-random.log`); `StartupRebuildPre0003Database/same_period` (role-less user) |
| M3: no start-up rebuild in `app.Run` (`app.go:198`) | `TestLED2_StartupRebuildsBalanceCachesBeforeServing` only | Modelled as a restart without `RepairBalances` (`V43_NOREPAIR=1`): random still green; timezone backwards red. That scenario is the one where M3 alone protects correctness, and the app test above guards the mechanism. |
| Extra, M4: period-moved rebuild disabled (`service.go:351`) | `TestLED2_TimezoneChangeBetweenRestartsCacheRepaired/backwards` | `TestV43_TimezoneOrigWithStartupRepair/backwards` |

`evidence/mut-M3-nostartup.log` records a first attempt whose mutation did not compile (vet rejected the comparison). The repository result for M3 in the table is from the rerun with `func() error { return nil }()`.

## Files changed

The product repository was not touched. I wrote only this folder: `RESULT.md`, `PROGRESS.md`, and `evidence/`, which holds the task 43 test sources as `.txt`, the old-database maker, the logs and the mutation runner.

## Decisions for docs/decisions.md

None. (Finding 1's fix, if adopted, would need a line there.)

## Confidence and gaps

- Verified by running: everything in the tables above.
- Not covered:
  - The timezone change is not part of the random operation set (the brief's list does not include it). It is covered by the original v42 test and my variant only.
  - No e2e/Docker run (the brief forbids Docker).
  - Real HTTP concurrency is modelled by goroutines calling the services, not by the HTTP server.
- Inference: finding 3 is reachable in deployment only if two processes share a database. I did not check whether the deploy docs prevent that.
