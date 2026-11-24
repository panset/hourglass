# 04 Independent verification of milestones 2 and 3: result

Status: done. Verified at HEAD `1cb441e` (clean tree apart from untracked `.orchestrator` logs). I did not read the RESULT.md, PROGRESS.md or mutations.log files of tasks 02-* and 03-*.

Verdict: WGT-1 to WGT-6, WGT-12 and WGT-17 hold in the code. 27 of 30 planned mutations went red on a test named after the id. The 3 that stayed green are 2 equivalent mutants and 1 test gap. Two more probes found test gaps (P1, P2). Nothing I found is a product bug. The main finding is that the v1.1.0 compatibility replay never runs under `./ci.sh`.

## Findings, ranked by severity

### 1. Medium: the WGT-19 v1.1.0 replay is skipped under Bazel, so `./ci.sh` never runs it

- `internal/collector/upgrade_v110_replay_test.go:62-66`: when `TEST_SRCDIR` is set and `poll.json` is missing, the test calls `t.Skip`.
- No `BUILD.bazel` exports `testdata/upgrade-v1.1.0/*`:
  - The root `BUILD.bazel:11-18` exports only the `d96a950` files.
  - `//internal/collector:collector_test` (`internal/collector/BUILD.bazel:23-42`) has no `data`.
- Confirmed by running `bazel test //... --nocache_test_results --test_arg=-test.v`. Its `internal/collector/collector_test/test.log` shows `--- SKIP: TestWGT19_ReplayV110PollsWithFeatureOff`. Under `go test -count=1 ./...` the test runs and passes.
- Failure scenario: a change breaks feature-off compatibility in a way only the replay checks. `./ci.sh` (the milestone gate) still passes, because the test runs no assertions there.
  - Example: a change to the snapshot or policy-call JSON shape, or to the samples written for the recorded `lab-08` policy hold.
  - The `d96a950` upgrade tests do not have this hole. They are given their fixtures as `data`, and they `t.Fatalf` when a fixture is missing (`internal/app/upgrade_test.go:31-37`).
- The skip is not recorded in `docs/decisions.md` (no mention of "runfiles").
- Fix:
  1. Add the `testdata/upgrade-v1.1.0` files to the root `exports_files`.
  2. Add them as `data` of `//internal/collector:collector_test`.
  3. Find them with the upward search that `internal/app/upgrade_test.go` uses, and remove the `Skip`.
- No other test skips silently under Bazel:
  - Bazel and `go test` ran the same set of 718 top-level test names (diffed).
  - The only SKIP line under Bazel is WGT-19.
  - The three tzdata skips (`internal/ledger/ledger_test.go:17`, `internal/api/admin_system_test.go:181`, `internal/store/ledger_compaction_test.go:23`) did not fire under Bazel on this machine.

### 2. Low: no test checks that a policy's `Decision.Weight` beats an override (WGT-2)

- Code: `internal/collector/collector.go:621` replaces the resolved weight whatever its source. That is correct per WGT-2 and the open point "a policy's Decision.Weight still beats the admin's override".
- Mutation P1 changed `if d.Weight > 0 {` to `if d.Weight > 0 && source != weights.SourceOverride {`. The whole repository, `go test ./...`, stayed green.
- Failure scenario: a later change lets the override win over a Go policy's weight, and nothing fails.
- `TestWGT2_PolicyWeightReplacesResolvedWeightForThatHoldAndPoll` uses a reported weight, not an override.

### 3. Low: the WGT-5 tests miss a selector kept under `policy` for a rule-priced setup

- Mutation P2 changed `collector.go:624` to test the resolver's source instead of the final source. A policy-weighted sample of a setup that a rule matches then keeps `rule_selector`.
- Every `TestWGT*`, including WGT-19, stayed green. Only the pre-existing `TestMET3_PolicyWeightOverride` (`internal/collector/collector_test.go:554`) went red.
- Why WGT-5 misses it: `TestWGT5_EverySampleHasOneOfFiveSourcesAndSelectorOnlyForRule` gives `lab-policy` a reported weight, so the resolved source is `reported`, not `rule`.
- Fix: in that test, leave `lab-policy` out of the report.

### 4. Low: the negative-zero normalisation of `reported_weights.min` is not tested in effect (WGT-17)

- Mutation S4 removed `internal/settings/settings.go:232-234`. All tests stayed green, but `GET` then returns `"min":-0`. I confirmed that Go writes `-0` for a negative zero.
- The case "min of 0 written as -0" (`internal/api/settings_reported_weights_test.go:64`) compares decoded floats with `reflect.DeepEqual`, where `-0 == 0`. Comparing the raw body, or `math.Signbit`, would catch it.
- The design states the -0 rule for report weights only (B1), so this is cosmetic.

### 5. Info

- `internal/weights/weights.go:160-162` (clamping age at zero) is an equivalent mutant. A negative age is never above `max_age`, so the clamp cannot change a result. The future-`observed_at` case is still tested: W7 went red.
- R1 (removing `v = 0` at `internal/reported/weights.go:155`) stayed green. The store appears to return 0 for a stored -0, and `TestWGT8_NegativeZeroStoredAsZero` does check `math.Signbit`. So this is equivalent at the store level. That is inference: I did not inspect SQLite's encoding.
- Stale comment: `internal/store/usage.go:24` says `weight_source` is empty "for now every sample". The collector now writes it (`collector.go:639-641`).

## Per-requirement table

| Id | Verdict | Code | Tests (named) | Mutation | Result |
|---|---|---|---|---|---|
| WGT-1 | holds | `collector.go:617-641` (debit = resolved weight, `metering.DebitMT`) | `TestWGT1_ReportedWeightIsCharged` (96, 192, 288 mt at x1, x2, x3) | C1: debit uses `state.Weight` (base) | red |
| WGT-2 | holds | `weights.go:112-154`; `collector.go:621-623` | `TestWGT2_ResolverTable`, `_OrderOverrideReportedRuleDefault`, `_ReportedBeatsRuleOfWeightZeroAndOfHigherWeight` (resolver and collector), `_OverrideBeatsReportedAndKeepsAttributes`, `_ReportedZeroWithMinimumZero`, `_PolicyWeightReplacesResolvedWeightForThatHoldAndPoll`, `_BadRuleIsAnError` | W1 override loses to report; W2 rule of weight 0 wins; C2 policy weight ignored; plus W3-W5, W7, W8, W11, W12 | all red (gap: finding 2) |
| WGT-3 | holds | `collector.go:388-400` (only non-ErrNotFound fails); `weights.go:125-142` | `TestWGT3_EachFallbackCauseChargesAsWithoutTheFeature` (6 causes x override/rule/default), `_DatabaseErrorReadingTheReportFailsThePoll`, `_HoldMeteredBeforeItsSetupIsReported`, `_PendingReleaseCompletedAfterSetupLeftTheReport` | C3 DB error ignored; C4 missing report fails poll; W9 switch ignored; C9 resolve at previous poll time | red |
| WGT-4 | holds | `collector.go:411-412` (resolved at T); no pro-rating | `TestWGT4_ChangedWeightPricesTheNextIntervalOnly` (report 1 s before the poll; earlier samples and ledger rows deep-equal) | C5 averaged ("pro-rated") weight | red |
| WGT-5 | holds | `collector.go:617-641`; `store/usage.go:36-42`; migration column | `TestWGT5_EverySampleHasOneOfFiveSourcesAndSelectorOnlyForRule` | C6 selector for every source; C7 source not recorded | red (gap: finding 3) |
| WGT-6 | holds | `collector.go:418-421` (`Weight: res.BaseWeight`); `pkg/hourglass/types.go` comment only | `TestWGT6_SnapshotCarriesTheBaseWeight`, `TestWGT6_InventoryPressureIgnoresReportedWeights` | C8 snapshot gets the resolved weight | red (both) |
| WGT-12 | holds (collector and resolver; the readers belong to M5) | `weights.go:139-141, 169-174` (limits read per call) | `TestWGT12_LimitsCheckedAtReadTime`, `TestWGT12_LoweredMaximumChargesTheFallbackAtTheNextPoll` | W10 maximum ignored | red |
| WGT-17 | holds | `settings.go:141-163` (validate), `:190-240` (decode); `handlers_admin.go:842, 855-859` | `TestWGT17_EveryRuleOfTheD1Table`, `_AllFourFieldsRequiredWhenPresent`, `_OnWithoutMaximumMessage`, `_ChangeAuditedWithWholeObjects`, `_PutWithoutTheKeyLeavesItUnchanged`, `_GetAlwaysReturnsTheKeyWithDefaults`, `_StoredValueLoads`, `_ReportedWeightsValidateAndDefaults` | S1 PUT without the key resets it; S2 on without max; S3 fields not required; S5, S6 bounds loosened; S7 key not saved or audited | red (gap: finding 4) |

## Mutation log

All 34 runs are below. P2b and P2c repeat P2 with narrower and wider test sets. Each mutation was applied by exact string replacement, the named tests were run with `go test -count=1 -run <regex>`, and the file was restored from a byte copy (checked equal). After every batch, `git status --short` (without `.orchestrator`) and `git diff --stat` were empty. The full output is in `mutations.txt` in this folder.

| Mutation | Result |
|---|---|
| W1 override does not beat reported | RED |
| W2 rule of weight 0 beats reported | RED |
| W3 min exclusive | RED |
| W4 max exclusive | RED |
| W5 expired at exactly max_age | RED |
| W6 negative-age clamp removed | green (equivalent) |
| W7 future observed_at counts as expired | RED |
| W8 not_applied precedence swapped | RED |
| W9 switch ignored | RED |
| W10 maximum ignored | RED |
| W11 override hides attributes | RED |
| W12 reported not exposed when not usable | RED |
| C1-C9 (collector) | all RED |
| S1-S3, S5-S7 (settings) | RED |
| S4 settings -0 not normalised | green (finding 4) |
| R1 report -0 not normalised | green (equivalent at store level) |
| M1 migration 0008 updates existing rows | RED (`TestNFR_Migration0008UpgradesVersion7DatabaseWithoutRewritingRows`) |
| B1 snapshot drops setup details | RED (`TestHOLD3_DeployedSetOnce`, WGT-19 under go test) |
| P1 override beats policy (whole `./...`) | green (finding 2) |
| P2 policy over rule keeps selector, WGT tests only (P2b: WGT-19 left out, also green) | green (finding 3) |
| P2c the same over `./...` | RED only in `TestMET3_PolicyWeightOverride` |

## Break attempts from the brief

| Check | Outcome |
|---|---|
| Each A1 rule | Covered by the resolver table (2 switch states x 3 override values (none, 7, 0) x 4 rule cases (weight 2, weight 0, no match, no rules) x 11 report shapes) and W1-W12. |
| Limits exactly at min and at max | Usable (table cases "exactly min" and "exactly max"; W3 and W4 red). |
| Negative zero | Normalised in report and setting; finding 4 for the setting's test. |
| `not_applied` precedence | expired > out_of_bounds > override (`weights.go:135-150`; W8 red). |
| Switch off with a stored report, and with an override | The report is ignored: no reported weight, no `not_applied`, no attributes (table; W9 red). |
| Override 0 | In the table. |
| Rule of weight 0 under a usable report | The report wins (W2 red). |
| Report exactly `max_age` old | Usable (`>` at `weights.go:163`; W5 red). |
| `observed_at` in the future | Age clamped at 0, report usable (W7 red). |
| Fallback cases of A2 | Both tested as written: `weights_wgt_test.go:268, 284`. |
| `Decision.Weight` with a reported weight | Tested. With an override it is not tested (finding 2). |
| `rule_selector` per source | Code sets it only for `rule` (`collector.go:624-626`). |
| A path where `SetupState.Weight` could carry a reported weight | None. It is built only at `collector.go:418-421` (base weight) and in `internal/app/checkprovider.go:95`, the check-provider diagnostic, which leaves the weight unset. |
| Remembering a removed weight | None. The resolver is pure, and the collector does not read past samples for the weight. |
| A poll failing because of a missing, expired or unusable report | Never. Only a database error fails it (C4 red; the expired and out-of-bounds causes pass). |
| `PUT` without the key | Leaves the stored value unchanged, with no audit row (S1 red). |
| `PUT` with the key and three fields | 400 naming the missing field (S3 red). |
| `enabled` true with `max` null | 400 on `reported_weights.enabled` (S2 red). |
| Audit before and after | The whole objects (S7 red). |
| Migration 0008 rewriting existing rows | It does not; it only adds tables and columns. The test catches a rewrite (M1 red). |

## Compatibility with v1.1.0

- **Replay.** `TestWGT19_ReplayV110PollsWithFeatureOff` replays the three recorded polls over a copy of `testdata/upgrade-v1.1.0/hourglass.sqlite3` with the switch off. Per poll it compares the snapshot to the pressure source, the policy calls, pressure and multiplier, the new usage samples (every recorded field, with `weight_source` checked separately), the new ledger rows, balances and hold states with `poll.json`. It passes under `go test`. Under `./ci.sh` it is skipped (finding 1).
- **Fixture provenance.** The fixture was committed in `0d20688`, before any product code changed (`git log -- testdata/upgrade-v1.1.0`).
- **Fallback equality.** `TestWGT3_EachFallbackCauseChargesAsWithoutTheFeature` also checks that every fallback cause, including "feature off with a stored report", charges what a database without the feature charges.
- **d96a950 tests.** They pass under `go test` and Bazel. `git diff 0d20688..HEAD --stat -- testdata` is empty. One `d96a950` test was edited: `TestUPG1_ReleaseDatabaseUpgradesAtStartAndKeepsEveryNumber` changed its expected schema version from 7 to 8. This is recorded in D82.
- **Every edited existing test**, judged:

| Test | Change | Weakened? |
|---|---|---|
| `TestUPG1` in `internal/app/upgrade_test.go` | schema version 7 → 8 | no, still exact |
| `TestMigration0005…`, `TestUPG1_Schema5…`, `TestD40_Migration0004…`, `TestMigrateFreshAndReopen`, `TestNFR_Migration0003…` | 7 → 8 | no, still exact |
| `TestSchemaColumnsMatchSpec` | adds the new columns and tables | no |
| `TestADM2_GetSettingsShapeAndRuntime` | adds `reported_weights` to the exact key set | no |
| collector `env` | gains `dbPath` | no assertion change |

No UI test file changed.

## Rules of the job

- `tools/check_core_generic.sh`: OK (198 core files, 209 Go files).
- `git grep` for the first client's names (`lab-connector`, `mem-gb`, `disk-gb`) outside the design doc and `.orchestrator`: no hits.
- `pkg/hourglass/types.go`: only the `SetupState.Weight` comment changed.
- `docs/SPEC.md` and `docs/design/reported-weights.md`: unchanged since `0d20688`.

## Commands run and outcomes

- `./ci.sh > ci.log`: exit 0. All 40 Bazel targets were cache hits ("Executed 0 out of 40 tests: 40 tests pass").
- `go test -count=1 -v ./... > gotest.log`: exit 0. 718 top-level tests passed and none were skipped; 1828 `=== RUN` lines including subtests. 43 distinct `TestWGT*` tests ran, including all those in the table above.
- `bazel test //... --nocache_test_results --test_arg=-test.v > bazel-verbose.log`: all 40 targets executed. The Go tests showed 717 PASS and 1 SKIP (WGT-19), the same name set as `go test`. `//ui:test` and `//ui:typecheck` failed only because they reject the `-test.v` argument (`error TS5023: Unknown compiler option '-test.v'`); they pass in `ci.sh`.
- Mutation runs: `mutations.txt`. Harness: a scratchpad script that copies the file, replaces the text, runs the tests and copies the file back.
- `tools/check_core_generic.sh`: exit 0.

## Files changed

.orchestrator/reported-weights/tasks/04-verify-m3/PROGRESS.md
.orchestrator/reported-weights/tasks/04-verify-m3/RESULT.md
.orchestrator/reported-weights/tasks/04-verify-m3/ci.log
.orchestrator/reported-weights/tasks/04-verify-m3/ci.exit
.orchestrator/reported-weights/tasks/04-verify-m3/gotest.log
.orchestrator/reported-weights/tasks/04-verify-m3/bazel-verbose.log
.orchestrator/reported-weights/tasks/04-verify-m3/mutations.txt

No product file is changed; every mutation was restored.

## Decisions for docs/decisions.md

None taken by this task. If finding 1 is fixed by keeping a skip instead of wiring up the data, record why.

## Requirement id → test names

See the per-requirement table. WGT-7 and WGT-8 tests (milestone 2) exist and pass, but they were outside the ids of this brief. Of them I mutated only R1.

## Confidence and gaps

- **Not checked:**
  - The A3 readers, the report route, the state fields and the UI. These are milestones 4 to 6 and outside this brief. WGT-12 "from the next read" is verified only at the resolver.
  - The e2e runs.
  - Whether the tzdata skips fire on another machine or a Linux CI runner. They did not fire here.
- **Inference, not verified:**
  - That SQLite or its driver turns a stored -0 into 0 (R1). This is inferred from the green mutant and the `Signbit` assertion.
- **`./ci.sh` evidence:** its pass was all cache hits. The uncached Bazel run above shows the same tests pass when they are executed.
