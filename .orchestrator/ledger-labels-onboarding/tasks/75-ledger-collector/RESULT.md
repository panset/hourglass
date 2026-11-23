# Task 75 result: ledger service and collector, bounded compaction (LED-10)

Status: done. `go vet ./internal/...` and `go test ./internal/...` pass; `./ci.sh` exited 0 (`ci.exit`, log `ci.log`: "39 tests pass").

## What was built

- **`ledger.Service.CompactDue(ctx) (CompactionStep, error)`** (`internal/ledger/service.go`, after `NewService`):
  - One bounded step in one `st.Tx`: `tx.DueLedgerGroups(now, loc, from, compactRows)`, then `tx.CompactLedgerGroup` for each group.
  - `CompactionStep{Groups, Replaced, Removed, More}`. `Removed` = `Replaced - Groups`, the rows the ledger lost. `More` = the search's `Next` is before `store.LedgerCompactionCutoff(now, loc)`.
  - It does not call `AppendLedger` and does not touch `users.balance_mt`.
  - The resume position (`compactFrom`) lives in memory, guarded by `compactMu`, which also serialises steps.
    - After a successful step with `More`, it moves to `Next`.
    - When a step finds nothing more, it goes back to the zero time, so the next (daily) check also sees raw rows that landed behind it.
    - A failed step leaves it unchanged, so the retry redoes the same groups.
  - `compactRows` defaults to `store.LedgerCompactionStepRows` (2,000). Only the ledger package's tests lower it.
- **Collector** (`internal/collector/collector.go`):
  - The `Ledger` interface gains `CompactDue`. `*ledger.Service` and the test wrappers that embed it satisfy it; `internal/app` needed no change.
  - New constant `CompactStepsPerCycle = 20`. New fields `compactPending` (guarded by `runMu`) and `compactSteps`.
  - `pruneIfDue` (daily, and the first cycle after start) sets `compactPending`.
  - `Run` calls `compactIfDue` after `pruneIfDue`, so the poll of each cycle runs first. `compactIfDue` runs up to `compactSteps` calls of `compactStep`. Each call takes `runMu` for one step only.
  - Each cycle's steps stop early when:
    - nothing more is due (pending is cleared);
    - a step fails (it is logged as "ledger compaction step failed; retried next cycle", pending stays set, and it is retried next cycle);
    - ctx is done.
  - `"compacted ledger" rows=<removed> groups=<n>` is logged when rows removed > 0, as the prune steps log.
  - A failing step is outside `RunOnce`, so it never fails a poll.
  - A backlog keeps `compactPending` set, so it continues on every following cycle, not only daily.
  - `RunOnce` is unchanged.

## Measured step times (scale test, this machine, `go test`)

| Run | Data | Steps | Median step | p99 | Worst step | Total | Cycles at 20 steps |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Default (CI) | 3 holds × 365 days, 315,789 debits | 179 | 13.3 ms | 21.3 ms | 27.8 ms | 2.5 s | 9 |
| `HOURGLASS_SCALE_TEST=1` | 12 holds × 365 days, 1,263,156 debits | 716 | 17.0 ms | 28.8 ms | 59.8 ms | 13.3 s | 36 |

- **Idle check** (CompactDue with nothing due, over the compacted year): 0.0 ms.
- **Find-query plans:** both use `SEARCH ledger USING INDEX ledger_raw_debit_created (created_at>? AND created_at<?)`.
- **`GET /api/v1/me/ledger`** (worst of 5, user with 2 holds, full run):

| Page | Before | After |
| --- | --- | --- |
| Current period | 11,233 entries, 57.9 ms | 4,345 entries, 17.0 ms |
| `?period=2026-06` | 17,280 entries, 87.9 ms | 60 entries, 1.0 ms |

- Every balance cache is unchanged after compaction.
- **Steps per cycle: 20.** At the full-scale worst step that is about 1.2 s of compaction per cycle. The shortest poll interval allowed is 1 minute, and MET-4 counts an outage at twice the interval.
- At the spec's scale (100 holds, 5-minute polls), a year of history is about 10.5 M debits, or about 5,300 steps. Worked off at 20 steps per 5-minute cycle, that takes about 22 hours (inference from the measured rows per step).

## Requirement -> test -> file

| Requirement | Test | File |
| --- | --- | --- |
| LED-10 (bounded steps, More, sums and caches unchanged) | `TestLED10_CompactDueStepsAreBounded` | `internal/ledger/compact_test.go` |
| LED-10 (again when nothing due changes nothing) | `TestLED10_CompactDueAgainWhenNothingDueChangesNothing` | `internal/ledger/compact_test.go` |
| LED-10 (failed step changes nothing, no marker, retry redoes same group) | `TestLED10_CompactDueFailedStepChangesNothingAndIsRetried` | `internal/ledger/compact_test.go` |
| LED-10 (lock released between steps; a poll runs between two steps) | `TestLED10_StepsReleaseTheLockBetweenThem` | `internal/collector/compact_led10_test.go` |
| LED-10 (step budget per cycle; backlog finishes over later cycles; rows-removed log) | `TestLED10_BacklogFinishesOverLaterCycles` | `internal/collector/compact_led10_test.go` |
| LED-10 (failed step fails no poll, retried next cycle) | `TestLED10_FailedStepFailsNoPollAndIsRetried` | `internal/collector/compact_led10_test.go` |
| LED-10 (daily cadence once idle) | `TestLED10_IdleCheckRunsOnceADay` | `internal/collector/compact_led10_test.go` |
| LED-10 / A1 scale (year of debits, step < 1 s, index plan, page read before/after) | `TestLED10_ScaleYearOfDebitsCompactsInBoundedSteps` | `internal/api/perf_compaction_scale_test.go` |

## Deliberate breaks (each red, then restored; suite green after)

| Break | Red test |
| --- | --- |
| `runMu` held across all steps of a cycle (lock moved from `compactStep` to `compactIfDue`) | `TestLED10_StepsReleaseTheLockBetweenThem` ("no poll ran between the 4 steps of the first cycle") |
| A failed step clears `compactPending` | `TestLED10_FailedStepFailsNoPollAndIsRetried` |
| Step budget ignored (loop until done) | `TestLED10_BacklogFinishesOverLaterCycles` |
| `compactPending` never consulted (a check every cycle) | `TestLED10_BacklogFinishesOverLaterCycles`, `TestLED10_IdleCheckRunsOnceADay` |
| Service step bound ignored (`maxRows` = 1<<30) | `TestLED10_CompactDueStepsAreBounded`, `TestLED10_CompactDueFailedStepChangesNothingAndIsRetried` |

The scale test also checks itself. Inside a rolled-back transaction it drops both raw-debit indexes and asserts that the plan check then fails, so it cannot pass without the index.

## Test notes

- **Lock test.** A poll is started from inside the first step and waits for `runMu`. A later step of the same cycle must see that poll finished.
  - This relies on `sync.Mutex` handing the lock to a waiter queued for more than 1 ms (starvation mode, documented in the `sync` package). The poll gets in after a step or two.
  - Measured: the poll ran within the first cycle's 4–5 steps in every run, including `-race -count=3`.
- **Collector tests** drive `Run` through the injected `After`, one cycle at a time. The clock is fake. The backlog is written with `AppendLedger` into December 2029, before the default clock of 1 Jan 2030.
- **Scale test.** It seeds with plain `INSERT`s on a second connection (the rows a poll writes), then `RepairBalances`. That is about 12 s by default and 57 s at full size.
  - It finds the backend's database file as the only `*/api.db` under the test's TempDir root, because `newBackend` does not expose the path. `testenv_test.go` is outside this task's scope.
  - The plan check copies the two find queries of `store.DueLedgerGroups`. A change to those query strings would not be caught here. The store test `TestLED7_DueSearchUsesRawDebitIndexes` copies them the same way.

## Existing tests

No existing test was changed or weakened. `TestMET1_RunPollsImmediatelyThenEveryInterval` now runs compaction through `Run` with nothing due and passes as written.

## Files changed

- Modified: `internal/ledger/service.go`, `internal/collector/collector.go`, and `internal/ledger/BUILD.bazel`, `internal/collector/BUILD.bazel`, `internal/api/BUILD.bazel` (Gazelle; the new test files only).
- Added: `internal/ledger/compact_test.go`, `internal/collector/compact_led10_test.go`, `internal/api/perf_compaction_scale_test.go`.
- `internal/store/`: no change.

## Commands run

- `go test ./internal/ledger/ ./internal/collector/ -run LED10`: pass; also `-race -count=3`: pass.
- `go test ./internal/api/ -run TestLED10_Scale -v`, and the same with `HOURGLASS_SCALE_TEST=1`: pass (figures above).
- The deliberate-break runs in the table above.
- `go vet ./internal/...`, `go test ./internal/...`: pass.
- `bazel run //:gazelle -- update`: added the three test files to their BUILD files.
- `./ci.sh > ci.log`: exit 0.

## Decisions for docs/decisions.md

1. **Steps per cycle: 20** (`collector.CompactStepsPerCycle`), each step at most `store.LedgerCompactionStepRows` = 2,000 rows (a larger single group alone). The worst measured step is 60 ms over 1.26 M debits, so a cycle's compaction is about 1.2 s at worst, far inside the 1-minute minimum poll interval. There is a fixed step count only, with no wall-time budget.
2. **Cadence.** The daily `pruneIfDue` (and the first cycle after start) marks compaction pending. Each cycle then runs up to 20 steps after the poll and the prune, and the flag stays set until a step finds nothing more due. A backlog is therefore worked off on consecutive cycles. Once idle, the check runs once a day with the prune.
3. **Failure.** A failed step is logged at error level and ends that cycle's compaction. It leaves the flag set, so it is retried on the next cycle, not the next day. It never fails or delays a poll beyond that one step.
4. **Order within a cycle.** Poll first, then prune, then compaction. On the first start after upgrade the first poll is not delayed by the backlog.
5. **Resume position.** It is held in memory by the ledger service and moves to the search's `Next` after each step that reports more. After a step that finds nothing more, it goes back to the beginning, so each daily check also picks up raw rows that landed behind it (for example after a clock step back). A restart also starts from the beginning. The cost of that walk is one indexed query when no due raw rows remain (measured 0.0 ms).
6. **Log line.** `"compacted ledger"` with `rows` (the net rows removed: rows replaced minus roll-up rows written) and `groups`, only when `rows` > 0.
7. **Scale test sizing.** CI runs a year for 3 holds; `HOURGLASS_SCALE_TEST=1` runs a year for 12 holds, the same switch as the existing API scale test. The test asserts each step under 1 s, the ledger page under 500 ms (the NFR bound) before and after, and an index plan for the find queries.

## Confidence and gaps

- Verified by tests: everything in the requirement table.
- The poll delay is inferred from the measured step times and the step budget. No test asserts a wall-clock bound on a whole cycle: a time-based assertion would be flaky on slow CI.
- The lock test relies on Go's documented mutex starvation hand-off. It was stable in every run here, but it waits up to 300 ms per step when the hand-off has not happened yet, so it takes about 1 s.
- Step times were measured on this machine only; slower disks will be slower. The 20-step budget leaves a margin of about 50× against a 1-minute interval at the measured worst step.
- The CONTRIBUTING.md note that the header comment points to does not yet describe the new scale test. That file is outside this task's write scope, so the orchestrator or the docs task should add it.
