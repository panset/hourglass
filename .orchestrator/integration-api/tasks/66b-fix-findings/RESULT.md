# 66b Fix the findings of verification 66

Status: done. `go test -race -count=1 ./...` exit 0 (32 packages ok, `gotest.log`). `./ci.sh` exit 0 (`ci.exit`, `ci.log`; 37 of 37 Bazel tests pass, 14 run fresh, `gazelle` diff clean).

## One line per finding
1. Medium, pressure staleness: fixed. `TestINT5_StalePressureAloneFailsThePoll` and `TestINT5_NoPressureReportYetFailsThePoll` (providers/reported/reported_test.go). Red when `if v.Age > p.maxAge` in `Pressure` becomes `if false`, and when the no-report branch returns a value.
2. Medium, wiring: fixed. `TestINT5_RunPublishesTheStoredReportsToTheExtensions` (internal/app/storedreports_test.go; test-local extensions read `pkg/storedreports`, the core imports no extension). Red when the `storedreports.Publish` line in `app.Run` is replaced by a no-op.
3. Low, race guards: fixed. `TestHOLD6_StaleRequestAfterMarkerFiresNothing` (red when the `first` guard is removed), `TestHOLD6_SetReleaseRequestedReportsFirstOnly` (red when `setOnce` drops `IS NULL`), `TestHOLD8_SecondServiceReleaseConflictsAtTheStore` (red when `AND released_at IS NULL` is dropped from `ReleaseHoldByService`). All run under `-race`.
4. Low, event order: real violation (the emit was after the commit, with no ordering against other emitters); fixed without a queue redesign by queuing `hold.releasing` inside the marker transaction (internal/holds/holds.go `requestRelease`), D60. `TestHOLD6_ReleasingIsQueuedBeforeAReleaseThatRacesIt` is red against the previous code. Residual: a failed commit leaves one queued event (recorded in D60).
5. Low, dead stub code: fixed. Removed `apierr.NotImplemented`, the `not_implemented` enum value and status in `api/openapi.yaml` and `ui/src/api/schema.gen.ts`, `integrationStubs` and its branches in permissions/contract tests, the 501 in `reachedHandler`. D49 marked "Superseded: all four routes are implemented".
6. Low, mixed pairings: fixed. `TestINT5_ReportedProviderWithAnotherPressureSource`, `TestINT5_ReportedPressureWithNoProviderOrAnotherProvider` (no provider, and static provider). Red when the pressure source is made to require a deployment report, and when the provider is made to require a pressure report.
7. Two behaviours: closed, D61 (**REVIEW**). Pinned by `TestHOLD6_ReleaseIsDecidedFromTheLastPollNotTheStoredReport` (red when the decision reads the stored report), `TestHOLD9_ReleaseAfterAFailedPollFollowsTheLastSuccessfulView` (red when a failed poll clears the view), `TestHOLD9_ReleaseWithNoSuccessfulPollIsImmediate` (red when a release is made pending without a successful poll).

## Mutation proof (ruling 8)
Script: scratchpad `mut66b.py`. Every mutation printed RED and "restored OK" (file sha and `git diff | shasum` unchanged; whole-tree `git diff | shasum` `5513dc38...` before and after). One first attempt was invalid (a build failure for an unused variable) and one did not bite (a mutation gated by `pv.OK()`); both were replaced with valid mutations that went red.

## Files changed
providers/reported/reported_test.go, providers/reported/BUILD.bazel, internal/app/storedreports_test.go (new), internal/app/BUILD.bazel, internal/holds/holds.go, internal/holds/pending_test.go, internal/holds/BUILD.bazel, internal/collector/pending_test.go, internal/api/apierr/apierr.go, internal/api/permissions_test.go, internal/api/contract_test.go, internal/api/service_accounts_test.go, api/openapi.yaml, ui/src/api/schema.gen.ts, docs/decisions.md (D49 note, D60, D61), this task folder.

## Decisions for docs/decisions.md
D60 (emit of `hold.releasing` inside the transaction), D61 (REVIEW: release decision from the last successful poll, with a failed poll, with no poll).

## Confidence and gaps
- `schema.gen.ts` was edited by hand (one enum member), not regenerated; `ui:api_schema_drift_test` passes.
- The ordering test relies on a 300 ms wait inside the emit hook to let a racing release finish in the old code; on the new code the release blocks on the write lock, so the test is deterministic in the red direction and cannot go red on the fixed code. Inference: it could only miss a regression on a machine slower than 300 ms for one release.
- Bazel reused cached results for 23 of 37 tests; the changed packages ran fresh.
