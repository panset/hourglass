# 76b Fix the findings of verification 76: result

**Status: done.** All five findings are closed or answered; L4(a) is deliberately not done (reason below).

## Findings and what closed them

| id | outcome | change |
| --- | --- | --- |
| M1 | fixed | Code change, `internal/collector/collector.go` (`Run`): the time `compactIfDue` takes (read from the injected clock) is deducted from the wait, `max(interval - compacting, 0)`. Step count per cycle is unchanged (20), no setting, no time budget. So compaction adds nothing to the gap between polls; a cycle without compaction waits the full interval as before. Tests in `internal/collector/compact_led10_test.go`, driving `Run` (the production loop) with a 6,912-row backlog and steps that advance the fake clock 7 s each: `TestLED10_CompactionDoesNotStretchThePollInterval` (wait == interval - steps x 7 s every cycle, until the backlog is gone), `TestLED10_LongCompactionNeverWaitsPastTheInterval` (wait is 0, never negative, when compaction alone exceeds the interval), `TestLED10_NoCompactionKeepsTheFullInterval`. With the deduction disabled the first two fail (checked, then restored). The test runner now records the wait `Run` passes to `After`. |
| L1 | fixed | `internal/store/ledger_compaction.go`: `DueLedgerGroups` looks at most `LedgerCompactionStepDays` (60) local days per call, so one step's cost does not grow with history. `Next` tells the caller where to go on, and `ledger.Service.CompactDue` already returns `More` while `Next` is before the cutoff, so no change was needed there. Test `TestLED10_DueSearchStepIsBoundedByDays` (200 one-row days then one group; fails with the bound removed). The scale test (`internal/api/perf_compaction_scale_test.go`) now seeds 400 days of one-row groups: the first search step finds nothing, walks at most 60 days (1.9 ms measured), and the idle check afterwards takes 7 bounded steps (12.5 ms in total). The helper `compactAll` in the store test now ends at the cutoff instead of at the first empty batch (needed because an empty batch no longer means done; not a weaker check). |
| L2 | fixed in the new tests; pre-existing flake named | `internal/api/race_on_test.go` and `race_off_test.go` (build tags) give `raceEnabled`; the scale test skips its wall-clock bounds (500 ms page, 1 s step and idle) under `-race` and logs the figures. Row, step and plan assertions still run. `go test -race -run LED10_Scale ./internal/api` now passes (253 s). The "known timing flake" is `TestNFR_LedgerAppendTimeIsLinear` (`internal/store/store_test.go:843`). It existed before this job (`git log -S` shows it in `096a285`, long before `4e8686e`), so I did not edit it: it is the verifier's unrelated flake under parallel load, and a re-run is the right response. It passed in this task's `go test`, `-race` and `./ci.sh` runs. |
| L3 | wording only | See "Decisions for docs/decisions.md". The optional start-up check that the marker table is empty is not added: no code path can commit the marker (76 checked), so a check would be dead code and `internal/` has no start-up hook for it that is not a new feature. |
| L4 | partly fixed | (b) `TestLED7_SpringForwardDayIsOneGroup`: March 8 2026 in Los Angeles (a 23-hour day) with rows before and after the 02:00 jump is one group, its neighbours are their own groups, and the result equals the model. (c) `TestLED7_TimezoneChangeLeavesRolledUpRowsAlone`: rows rolled up under Los Angeles are not touched by a later UTC pass (and the sum is unchanged). (a) not done: a database written by the `d96a950` binary cannot be produced inside a test without building that old revision, which needs git state and a second toolchain run. The raw inserts in `TestUPG1_Schema5LedgerUpgradesAndCompacts` use the same columns the old `AppendLedger` wrote, and migrations 0001-0005 are unchanged since `4e8686e`, so the gap is small. Left to the e2e upgrade run of the later milestones. |

Residual (inference): if the compaction of one cycle alone takes longer than the poll interval, the gap between polls is that duration. Steps measured 20-60 ms, so this needs about 3 s per step at the minimum interval; the design asks for steps under one second.

## Files changed

- `internal/collector/collector.go`, `internal/collector/compact_led10_test.go`
- `internal/store/ledger_compaction.go`, `internal/store/ledger_compaction_test.go`
- `internal/api/perf_compaction_scale_test.go`, `internal/api/race_on_test.go` (new), `internal/api/race_off_test.go` (new), `internal/api/BUILD.bazel` (Gazelle)
- This folder: PROGRESS.md, RESULT.md, ci.log, ci.exit

No existing test was weakened or deleted; no `docs/`, `api/`, `ui/`, `e2e/` or git state touched.

## Commands run and outcome

- `go test -count=1 ./internal/...`: pass.
- `go test -race -count=1 ./internal/ledger/ ./internal/collector/ ./internal/store/`: pass. `go test -race -run LED10_Scale ./internal/api`: pass.
- Breaks: deduction removed (2 collector tests red), day bound removed (store test red); both restored.
- `bazel run //:gazelle` then `./ci.sh`: exit 0 (`ci.exit`), 39 of 39 tests pass; the api, collector, ledger and store test targets were executed, not cached (9 executed).

## Decisions for docs/decisions.md

1. Replace the second sentence of D70(b) ("a marker committed by mistake would make the next step fail on the key instead of silently allowing deletes") with: "The table can hold at most one row, so a second marker insert fails on the key. A marker that stayed committed would allow deletes of any debit row on every connection, because the trigger only checks that the table is not empty; the step's SAVEPOINT (f) is what keeps the marker from being committed."
2. Replace "After the backlog they cover about a week of rows." in D70(c) with: "They cover every debit that is still raw: the last week of rows plus, for good, any group of one row (a group of one row is never compacted), so the index grows only with those."
3. Add to D70(d): "One search call looks at no more than `store.LedgerCompactionStepDays` (60) local days, so days that hold only one-row groups cannot make one step walk all of history; a step that finds nothing but has not reached the cutoff reports `More` and the search goes on in the next step."
4. Add to D71(a): "The time a cycle spends compacting is deducted from the wait before the next poll (`max(interval - compacting, 0)`), so a backlog adds nothing to the gap between polls; there is still no time budget on the steps."

## Confidence and gaps

- M1 is shown with a fake clock whose steps take 7 s; I did not run a real slow disk.
- The daily idle check now takes a few extra bounded steps when old one-row days exist (7 for 400 days), and re-walks them once a day; this is the cost of bounding a step, not a growth in step cost.
