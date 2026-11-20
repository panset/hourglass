# 66 Independent verification: reports, `reported` extensions, release changes

Status: done. Verdict: the report routes, the state route, the `reported` extensions and the
release changes meet the design. I found no production defect that breaks a numbered
requirement. Two tests are weak in ways that matter for money or wiring (findings 1 and 2), and
there are four low findings. I did not read task folders 63, 64 or 65.

## Commands and outcome

| Command | Outcome |
| --- | --- |
| `./ci.sh > ci.log` (foreground job, 30 min limit) | exit 0 (`ci.exit`). SPDX, genericity, licenses, go build, CGO-off build, vet, gazelle diff, `bazel build //...`, `bazel test //...`: "Executed 0 out of 37 tests: 37 tests pass". Every Bazel test result was **cached** from an earlier identical run, so Bazel ran no new tests. |
| `go test -race -count=1 -v ./... > gotest.log` | exit 0. 32 packages ok, 4 packages have no test files. **570 top-level tests**, 1232 including subtests, 0 failed, 0 skipped. |
| `go test -race -count=10 -run 'Concurrent\|Racing\|ReadersNever\|AtomicForConcurrent'` (holds, collector, api, reported) | all ok |
| `bash tools/check_core_generic.sh` | OK (157 core files) |
| `go build ./examples/...` | ok |
| `grep providers/reported internal pkg` | no core import |
| `git diff 5808957 -- pkg/hourglass/types.go` | one change only: the field `Hold.ReleaseRequestedAt`, which "Changes by area" sanctions. No interface changed. |

The mutations ran in a scratch copy of the working tree
(`/private/tmp/claude-501/-Users-tejaspanse-projects-hourglass/837e20c6-4260-4543-8af0-879d5dd3b3e0/scratchpad/mut/hg`)
so that they could not disturb `ci.sh`. The script was `scratchpad/mutate.sh`. For each
mutation it compared the file's shasum and `git diff | shasum` before and after the break, and
every one printed "restored OK". The real repository's `git diff | shasum` was
`0e162f157d00…` at the start and is the same at the end.

## Requirements

| Id | Covering tests (read) | Mutation in production code → result | Verdict |
| --- | --- | --- | --- |
| INT-1 | `TestINT1_PressureValueValidated` (api), `TestINT1_PressureValueValidatedAndNothingStored` (reported) | `value > 1` → `> 2` in `internal/reported/reported.go:118`: both red. NaN/Inf check removed: the reported test goes red; the api test stays green because JSON cannot carry NaN and the decoder already rejects `1e999` | pass |
| INT-2 | `TestINT2_DeploymentsReplaceWholeSet`, `TestINT2_ReadersNeverSeeAMixOfTwoReports` (api), `TestINT2_ReplaceIsAtomicForConcurrentReaders`, `TestINT2_FailedReplaceIsAllOrNothing` (reported) | DELETE of the old set dropped (`internal/store/reported.go:120`): 3 tests red. Read of header plus set taken outside a transaction (`store/reported.go:75`): `TestINT2_ReplaceIsAtomicForConcurrentReaders` red; the api reader test stays green | pass |
| INT-3 | `TestINT3_OlderReportRefusedEqualReplaces`, `TestINT3_ReportDoesNotPollDebitOrEmit` (api), `TestINT3_Older*`/`Equal*` (reported) | Pressure stale check removed: 2 red. Equal deployments refused (`!After`): 2 red. Skew widened by 1 s: api test red | pass |
| INT-4 | `TestINT4_NotConfiguredRefusedAndNothingStored` | `maxAge <= 0` → `< 0`, pressure route and then deployments route: red each time | pass |
| INT-5 | `TestINT5_NoReportThenStaleThenFresh`, `TestINT5_StaleDeploymentsAloneFailTheProviderOnly`, `TestINT5_Config`, `TestINT5_ProviderMapsEachDeploymentWithLabels`, plus `TestEXT5_Conformance` | Provider staleness check removed or loosened by 1 m: red. No-report returns `nil, nil`: red. **Pressure staleness check removed (`providers/reported/reported.go:148` → `if false`): no test fails in `./...`** | weak test (finding 1) |
| INT-6 | `TestINT6_ReportsSurviveReopen` (reported, providers/reported) | `store.Open` purges the three report tables: both red | pass |
| INT-7 | `TestINT7_ReportedSetupWithHoldGetsDeployedAndMeteringStarts`, `TestINT7_ReportedSetupWithNoHoldIsAnOrphan` (real collector, default policy) | `SetDeployed` disabled (`collector.go:528`): red. Provider renames to `x_RP`: both red | pass |
| INT-8 | `TestINT8_StateHasEverySetupAndHoldAndNothingElse` (exact key allow-list, banned words), `TestINT8_StateWithoutReportedExtensions` | (extra) `reported_deployed` taken from the last poll: red. `username` filled from `Name`: red | pass |
| HOLD-6 | `TestHOLD6_ReleaseOfDeployedSetupIsPendingAndFiresOnce`, `TestHOLD6_ConcurrentRequestsFireOnce` (holds), `TestHOLD6_UserReleaseOfDeployedSetupAnswers202Once` (api) | `requestRelease` skipped: 3 red. 202 → 200: red. The early return on a repeat and the `!first` guard (`holds.go:424`) each removed alone: green, because each covers the other; removed together: 3 red. `!first` removed alone, 20 runs of the concurrent test under `-race`: never red | pass (concurrency test weak, finding 3) |
| HOLD-7 | `TestHOLD7_PendingHoldIsMeteredThenCompletedWhenSetupDisappears`, `_HoldWithoutPendingReleaseIsLeftAlone`, `_ReleaseRacingPollReleasesOnce` (collector), `_PendingHoldStaysActiveAndCountsTowardLimit`, `_ForceReleaseCompletesPendingRelease` (holds), `_PendingReleaseCompletesAtThePollWithoutTheSetup`, `_AdminForceReleaseCompletesPendingRelease` (api) | Never complete: 3 red. Complete while still reported: 2 red. Pending hold not metered: red. Pending hold left out of the limit count: red. Hold without a request completed: red | pass |
| HOLD-8 | `TestHOLD8_ServiceReleaseReleasesAnyActiveHold`, `_ServiceReleaseRefusals`, `_ServiceReleaseCompletesPendingRelease`, `_ConcurrentReleasesFireOnce` (holds), `_ServiceReleaseOverHTTP` (api) | `service_account` dropped from `data`: 2 red. Audit actor dropped: 2 red. No trimming: 3 red. **`AND released_at IS NULL` dropped from `ReleaseHoldByService` (`store/holds.go:132`): green** | pass (race test weak, finding 3) |
| HOLD-9 | `TestHOLD9_FailedPollLeavesPendingReleasePending` (collector), `TestHOLD9_FailedPollDoesNotCompleteReleaseOverHTTP` (api) | Provider error treated as an empty list: both red. Pressure error treated as 0: collector test red | pass |
| CMP-1 | `TestCMP1_PlainProviderBehavesAsBefore` (read: acquire, poll, deploy, debit, immediate release with a plain provider) | not mutated (not required) | pass on reading |
| CMP-2 | `./ci.sh` | exit 0 (Bazel tests cached) | pass |

## Findings

1. **medium (weak test, money).** `providers/reported/reported.go:148`. No test anywhere in the
   repository fails when the pressure source's staleness check is removed. In
   `TestINT5_NoReportThenStaleThenFresh` both reports go stale together, so the provider fails
   first. `TestINT5_StaleDeploymentsAloneFailTheProviderOnly` covers only the provider.
   Failure scenario: a refactor drops or inverts the check. The customer's reporter keeps sending
   deployments but its pressure feed stops. Every poll then charges at the hours-old pressure
   multiplier, which breaks INT-5 ("nobody is charged on old data"), and CI stays green.
   Fix: add `TestINT5_StalePressureAloneFailsThePoll`, with fresh deployments, pressure older
   than its `max_age`, an expected error naming the pressure report's age, and zero samples and
   debits.
2. **medium (untested wiring).** `internal/app/app.go:185`. This one line publishes the stored
   reports to the extensions in the real server. I replaced it with a no-op and every test in
   `./...` still passed. The extension tests publish the reader by hand
   (`providers/reported/reported_test.go:94`), and no test starts `hourglass.Run` /
   `hourglassd` with `provider: reported`.
   Failure scenario: the line is lost in a refactor. Every poll then fails with "the server
   publishes no stored reports", and nothing goes red before a customer hits it.
   Fix: add a composition-root test (`internal/app` or `pkg/hourglass/server`) that runs `Run`
   with both `reported` extensions, sends a report through the API and expects a successful
   poll. The planned e2e scenario (design task 7) could also cover this.
3. **low (weak race tests).** `internal/holds/holds.go:424` (`if !first`) and
   `internal/store/holds.go:132` (`AND released_at IS NULL`) are the guards that stop a second
   `hold.releasing` or a second release when requests race. Removing either one alone leaves
   `TestHOLD6_ConcurrentRequestsFireOnce` and `TestHOLD8_ConcurrentReleasesFireOnce` green,
   including 20 runs under `-race`. The goroutines rarely interleave between the out-of-tx read
   and the tx, so the earlier out-of-tx checks answer first. The code itself is correct: every
   release path is a conditional UPDATE in a `BEGIN IMMEDIATE` transaction, the collector
   re-reads the hold inside its tx, and events fire only after a successful update.
   Failure scenario: someone "simplifies" one of these guards and a real concurrent click plus
   service call fires `hold.released` twice, with no test to catch it.
   Fix: add store-level tests that call `ReleaseHoldByService` / `SetReleaseRequested` twice on
   the same row and assert `ErrConflict` / `false`. Or add a hook between the read and the tx so
   that the race is deterministic.
4. **low (event order).** `internal/holds/holds.go:433`. `hold.releasing` is emitted after its
   transaction commits, with no ordering against other emitters. A service release, a force
   release or a poll completion that commits in that gap can enqueue `hold.released` before
   `hold.releasing`.
   Failure scenario: the customer's automation receives `hold.releasing` after `hold.released`
   for the same hold and starts a teardown of a setup that is already free. If it re-reads the
   state it sees no hold and has to cope with that. The impact is small because the teardown was
   wanted anyway.
   Fix: document in `docs/integrating.md` that the hold in the state read is authoritative and
   events may arrive out of order. Alternatively, skip the emit when a re-read shows the hold
   already released.
5. **low (leftover).** `internal/api/apierr/apierr.go:37` and `api/openapi.yaml:1382,1406`
   still carry `not_implemented` (501). The stub machinery is also still there:
   `integrationStubs` (empty) in `internal/api/permissions_test.go:612`, and the bypasses that
   use it at `internal/api/contract_test.go:365,742` and `permissions_test.go:526`. That code
   is now dead. D49 describes it as temporary. The map is empty, so nothing skips validation
   today.
   Failure scenario: a future route answers 501 and is added back to the map, and its success
   response is never contract-checked. Also, clients see an error code that the published
   contract never returns.
   Fix: remove `NotImplemented`, the enum entry and the stub branches, and mark D49 as
   superseded.
6. **low (test gap).** The design says the two extensions work independently (`provider:
   reported` with `pressure: inventory`, or `pressure: reported` with no provider). That is
   covered only at the route level (`TestINT4_` subcases) and by two `reported` extensions with
   different `max_age`. No collector test runs one `reported` extension next to a non-reported
   one or with no provider.
   Failure scenario: a change makes the pressure source require a published deployment report.
   Then `pressure: reported` without a provider fails every poll, and no test notices.
   Fix: add one collector test for each mixed configuration.

Checked and found to hold, with no finding:
- **Money.** A missing or stale report fails the poll before the write transaction (`collector.go:353-409`), so a failed poll writes no debit, no `deployed_at` and no completion (HOLD-9 mutations confirm this).
- **The poll where a pending setup disappears.** It charges its interval under the policy and then releases, as D58 says.
- **Pending holds.** They count toward the limit (`CountActiveHolds` uses `released_at IS NULL`) and keep the setup held (`setup_held`).
- **Reports.** Reports never trigger a poll, an event or a debit. `not_configured` is checked before the body and stores nothing. Equal `observed_at` replaces the stored report. The 2-minute skew is checked. The limits are 10,000 entries, 32 labels and 1 MB (`MaxBodyBytes = 1<<20`). Duplicate and empty names are 400. Unknown names are listed in order.
- **State route.** It makes a fixed number of queries, follows the key allow-list, and takes `reported_deployed` from the stored report.
- **Service release.** It trims the reason and counts characters (runes). The audit entry is in the same tx. `released_by` is null. The event data is exactly `{forced, reason, service_account}`. It answers 404 and 409 as the design says.
- **`max_age`.** The default (10m), the minimum (1m) and the parsing agree between `config.ReportedMaxAge` and `providers/reported.parseMaxAge`. The config side ignores unknown keys but the extension's factory rejects them, so startup fails rather than disagreeing. `expires_at = observed_at + max_age` and the poll fails only when `age > max_age`, so the two agree.
- **`check-provider`.** It prints the report ages (`TestCheckProviderPrintsReportAges`).
- **Third-party main.** `examples/custom` builds. Its `labd` main does not import `providers/reported`, which is optional.
- **`hold.releasing`.** It comes after `hold.deployed` in `EventTypes()`, is not vetoable and is accepted by the webhook filter (tested).
- **D40.** No added line reintroduces a resource name, template or override. The only `ResourceName` uses are the provider's `Allocation` field and orphans.
- **OpenAPI.** All four integration routes and `release_requested_at` (Hold, Holder) are described, and the 202 of the user release is documented. The contract test validates a success response for every operation. The 202 and the state with a pending hold are exercised (`contract_test.go:653,710-717`).

## Decisions D42 onward

None of them contradicts the design.
- **D42, D43.** They match the owner decisions.
- **D45, D46, D53.** They refine the open details consistently.
- **D51, D54, D55.** They keep the core free of extension imports.
- **D52.** It nulls the report fields when the extension is not `reported`, which the design does not forbid.
- **D57.** It keeps a repeat release at 202 even if the setup has meanwhile disappeared, which is consistent with "a repeated request returns the same 202".
- **D58.** It charges the completion poll, which is consistent with "after metering" and "the user pays until teardown is real".
- **D59.** It removes `still_deployed`, which is consistent with D43.
- **D49.** It is now obsolete (finding 5).

## Confidence and gaps

- The Bazel test step of `ci.sh` reported only cached results. I did not force a re-run with `--nocache_test_results`.
- I did not run the UI. `docs/integrating.md`, the e2e scenario and the UI "Releasing" state are later tasks and were not checked.
- My mutations were single, hand-picked breaks, not a full mutation run. A green result after a mutation means a gap in the tests, not a bug in the code. I found no bug in the production code.
- Two behaviours are unchanged from before this work and in line with the design, but owner-visible:
  - The pending-release decision uses the last *successful poll*, not the stored report. A release made between a report and the next poll is decided on the older data.
  - When the last poll failed (for example, a stale report), a user release is immediate even if the setup may still be deployed (HOLD-2's existing `pv.OK()` gate). The service then learns of it from `hold.released`, not `hold.releasing`.
- I did not check that a reporter's ms truncation cannot reorder two reports less than 1 ms apart (D45 accepts this).

## Files changed

Only this task folder: `PROGRESS.md`, `RESULT.md`, `ci.log`, `ci.exit`, `gotest.log`.
