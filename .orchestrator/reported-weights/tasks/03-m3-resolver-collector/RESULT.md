# 03 Milestone 3: the resolver and the collector — RESULT

Status: **done**. `./ci.sh` exit 0 (`ci.log`, `ci.exit`; 40 Bazel tests pass). `go test ./...` exit 0 (`gotest.log`). Every existing test passes unchanged; no existing assertion was edited.

## What was built

1. **`internal/weights`** (new, pure: imports only `internal/rules` and `internal/settings`, no store). `Resolve` (`internal/weights/weights.go:112`) implements A1: order override, usable reported, first matching rule, default; base weight; first matching rule's selector whatever the source; `Reported` whenever the feature is on and the report has an entry; `NotApplied` first of `expired`, `out_of_bounds`, `override`; attributes for a usable entry, also under an override; limits inclusive and read from the `cfg` passed in (so re-checked at every call); age from `observed_at`, clamped at 0, expired only when age > `max_age` (same as `reported.Fresh`); feature off ignores the report completely; rules evaluated through `rules.Weight`, so with the feature off and no override the weight is exactly `rules.Weight`'s.
2. **`internal/collector`** (A2):
   - Step 1 reads `st.GetReportedWeights` (`collector.go:391`); `store.ErrNotFound` means no report; any other error fails the poll ("weights report: ..."). Read always, also with the feature off (the design says "with everything else"). **No store addition was needed**: M2's `GetReportedWeights` is enough.
   - Step 2 calls `weights.Resolve` for every setup at `T` (`collector.go:411`) with the setup's `WeightOverride`.
   - `SetupState.Weight = res.BaseWeight` (`collector.go:420`): pressure source and policy see the base weight.
   - The debit uses `res.Weight`; `Decision.Weight > 0` replaces it with source `policy` (`collector.go:622`); `rule_selector` only when the source is `rule` (`:625`); every sample writes `weight_source` (`:641`). Arithmetic untouched (`metering.DebitMT`, `ChargeableElapsed`).
   - Nothing remembers a removed weight; a pending release's completing poll resolves like any other poll.
3. **`pkg/hourglass/types.go`**: only the comment of `SetupState.Weight` changed (base weight: override, else rate rule, else default; never a reported weight).
4. Test harness: `env` in `internal/collector/collector_test.go` gained a `dbPath` field (needed to open a second connection for the database-error test). No existing test body changed.

## Resolver exported API

```go
package weights
type Source string      // SourceOverride "override", SourceReported "reported", SourceRule "rule", SourceDefault "default", SourcePolicy "policy" (collector only)
type NotApplied string  // "" | NotAppliedExpired "expired" | NotAppliedOutOfBounds "out_of_bounds" | NotAppliedOverride "override"
type Setup struct { Name string; Labels map[string]string; Override *float64 }
type Entry struct { Setup string; Weight float64; Attributes map[string]string }
type Report struct { ObservedAt time.Time; /* entries by name, unexported */ }
func NewReport(observedAt time.Time, entries []Entry) *Report   // nil *Report = no report stored
type Result struct {
    Weight float64; Source Source; BaseWeight float64; RuleSelector string
    Reported *float64; NotApplied NotApplied
    Attributes map[string]string // never nil; a copy; empty unless the entry is usable
}
func Resolve(s Setup, rs []rules.Rule, cfg settings.Settings, report *Report, at time.Time) (Result, error)
// error: the rules.Weight error (bad selector / bad rule value), with a zero Result, also when an override is set.
```
Callers build a `*Report` from `store.GetReportedWeights` (collector, see `collector.go:391-400`) or from `reported.Weights` (milestones 4/5: `Entry{Setup: w.Setup, Weight: w.Weight, Attributes: w.Attributes}`, `ObservedAt`).

## Requirement -> tests

| Requirement | Tests | Mutation that turned it red (all in `mutations.log`, each reverted) |
| --- | --- | --- |
| WGT-1 | `internal/collector/weights_wgt_test.go`: `TestWGT1_ReportedWeightIsCharged` (1.15 x 1 x 5 min = 96; x2 = 192; x3 = 288) | M1: debit uses `state.Weight` (base) instead of the resolved weight |
| WGT-2 | `TestWGT2_OrderOverrideReportedRuleDefault`, `TestWGT2_ReportedBeatsRuleOfWeightZeroAndOfHigherWeight`, `TestWGT2_PolicyWeightReplacesResolvedWeightForThatHoldAndPoll` (collector); `internal/weights/weights_test.go`: `TestWGT2_ResolverTable`, `TestWGT2_ReportedBeatsRuleOfWeightZeroAndOfHigherWeight`, `TestWGT2_OverrideBeatsReportedAndKeepsAttributes`, `TestWGT2_ReportedZeroWithMinimumZero`, `TestWGT2_BadRuleIsAnError` | M2: reported beats override; M3: a matching rule of weight 0 beats reported; M4: `Decision.Weight` ignored; M1 |
| WGT-3 | `TestWGT3_EachFallbackCauseChargesAsWithoutTheFeature` (none sent, expired, left out of latest report, above max, below min, feature off; each for override-, rule- and default-priced setups; compares every sample with a run that never configured the feature, and asserts the source), `TestWGT3_DatabaseErrorReadingTheReportFailsThePoll`, `TestWGT3_HoldMeteredBeforeItsSetupIsReported` (A2 case 1), `TestWGT3_PendingReleaseCompletedAfterSetupLeftTheReport` (A2 case 2, D58) | M5: a database error on the read treated as no report; M6: expiry not checked; M7: switch ignored; M8: limits not checked; M12: poll fails for a setup with no entry; M14: remember the last reported weight of a setup that left the report |
| WGT-4 | `TestWGT4_ChangedWeightPricesTheNextIntervalOnly` (weight observed 1 s before the poll prices the whole interval; all earlier samples and ledger rows deep-equal before/after) | M11: a report observed after the previous poll is not used; M1 |
| WGT-5 | `TestWGT5_EverySampleHasOneOfFiveSourcesAndSelectorOnlyForRule` (one hold per source, start and debit samples) | M9: selector written for every source; M9b: source not written; M4 |
| WGT-6 | `TestWGT6_SnapshotCarriesTheBaseWeight` (collector: pressure-source and policy snapshots and `HoldState.Setup`, rule/default/override setups, disabled setup excluded, with and without a report); `pressure/inventory/wgt6_collector_test.go`: `TestWGT6_InventoryPressureIgnoresReportedWeights` (real `inventory` source through the real collector: raw pressure 3/8 and same smoothed/multiplier with and without a far-off report) | M10: snapshot carries the resolved weight |
| WGT-12 | `TestWGT12_LoweredMaximumChargesTheFallbackAtTheNextPoll` (collector), `TestWGT12_LimitsCheckedAtReadTime` (resolver) | M8 |
| Resolver table (design Tests) | `TestWGT2_ResolverTable`: feature on/off x override none/7/0 x rule weight 2 / weight 0 / not matching / no rules x entry (no report, absent, usable, expired, above max, below min, expired and above max, exactly min, exactly max, exactly `max_age` old, observed in the future) = 264 cases; also asserts equality with `rules.Weight` when off without override | M2, M3, M6, M7, M13 (feature off and a rule: default weight) |
| WGT-19 replay (partial, milestone 7 owns it) | `internal/collector/upgrade_v110_replay_test.go`: `TestWGT19_ReplayV110PollsWithFeatureOff` replays the three polls of `testdata/upgrade-v1.1.0/poll.json` over a migrated copy of its database: snapshot to the pressure source, every policy call (hold state, snapshot, decision), raw/smoothed pressure and multiplier, every usage sample and ledger row written (ids included), balances before and after, hold states; plus each new sample's `weight_source` | M13 |

## Decisions for docs/decisions.md

1. The resolver takes `settings.Settings` whole (it needs `DefaultWeight` and `ReportedWeights`), `[]rules.Rule`, and its own `*weights.Report` type, so `internal/weights` depends on neither the store nor `internal/reported`; each caller converts what it read.
2. The resolver always evaluates the rate rules, also when an override applies, because A1 asks for the first matching rule's selector whatever the source. Consequence: a rule that cannot be evaluated fails the poll even for a setup with an override (today no setup has an override, so nothing changes for existing installations).
3. On a rule error the resolver returns a zero `Result` and the error; each caller applies its own effect (collector: fail the poll, as today).
4. `Result.Attributes` is never nil (a copy, empty when none or not usable), matching the `{}` of B4/C2.
5. The collector reads the stored weights report on every poll, also with the feature off, as A2 step 1 says "with everything else"; a database error on that read therefore fails a poll even with the feature off. With the feature off the resolver ignores the report.
6. The usage sample written at metering start (debit 0) also records `weight_source`, like every other sample (C1: "every new sample").
7. A `nil` maximum, which settings validation allows only with the feature off, bounds nothing in the resolver (never reached while off).

## Files changed

```
internal/weights/BUILD.bazel
internal/weights/weights.go
internal/weights/weights_test.go
internal/collector/BUILD.bazel
internal/collector/collector.go
internal/collector/collector_test.go
internal/collector/weights_wgt_test.go
internal/collector/upgrade_v110_replay_test.go
pkg/hourglass/types.go
pressure/inventory/BUILD.bazel
pressure/inventory/wgt6_collector_test.go
```
Task folder: `.orchestrator/reported-weights/tasks/03-m3-resolver-collector/` (PROGRESS.md, RESULT.md, ci.log, ci.exit, gotest.log, gazelle.log, mutations.log).

## Commands run

- `go test ./internal/weights ./internal/collector ./pressure/inventory` while iterating: pass.
- Mutation script (14 mutations, each applied, the named tests run, the file restored): every mutation turned at least one named test red (`mutations.log`). Final re-run after restore: pass.
- `bazel run //:gazelle` (exit 0, `gazelle.log`); then added the SPDX line Gazelle does not write to the new `internal/weights/BUILD.bazel`; `gazelle -mode=diff` exit 0.
- `go test ./...` exit 0 (`gotest.log`).
- `./ci.sh` exit 0 (`ci.log`).

## Confidence and gaps

- **The v1.1.0 replay runs under `go test` only.** Under Bazel it skips (it checks `TEST_SRCDIR`), because the fixture files are not exported: running it in `bazel test` needs `testdata/upgrade-v1.1.0/poll.json` and `hourglass.sqlite3` added to `exports_files` in the root `BUILD.bazel` and a `data` attribute on `//internal/collector:collector_test` (with a `# keep` comment). The root `BUILD.bazel` and `testdata/` are outside this brief; milestone 7 (WGT-19) or the orchestrator should add the two lines and drop the skip. Under `go test ./...` it ran and passed (verified with `-v`).
- `internal/store/usage.go:23-24` still says `weight_source` is null "for now every sample"; that comment is now stale (store is outside this brief's write list except for a needed addition). Suggest the orchestrator fix the comment.
- `pkg/hourglass/types.go:48` (`Decision.Weight`: "0 means use the weight from the rate rules") is now slightly imprecise (0 means "use the resolved weight"); left unchanged as the brief allows only the `SetupState.Weight` comment. `docs/extending.md` (milestone 7) should state the precedence.
- `internal/api` readers still use the rate rules directly (milestone 5); until then the screens and the charge can differ when the feature is on or an override is set. The API cannot turn the feature on with a weights route yet (milestone 4), so this appears unreachable over HTTP except through the settings PUT plus a store-written report.
