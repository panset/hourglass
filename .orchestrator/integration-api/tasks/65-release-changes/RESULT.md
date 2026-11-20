# Result: 65 pending release, hold.releasing, collector completion, service release

Status: done. `go test -race -count=1 ./...` passes (gotest.log), `tools/check_core_generic.sh` OK, `./ci.sh` exits 0 (37 of 37 Bazel tests; ci.log, ci.exit).

## What was built
- `pkg/hourglass`: `EventHoldReleasing` (after `hold.deployed` in `EventTypes()`, not vetoable), `Hold.ReleaseRequestedAt` and `release_requested_at` in the event JSON. The webhook `types` filter accepts it through `EventTypes()`.
- `internal/holds/holds.go`: `Release` returns a still-active hold with `release_requested_at` set when the setup is reported and `release_requires_teardown` is on (`Pending(h)`); the marker is written once in a transaction (`requestRelease`), `hold.releasing` fires only for the call that wrote it. `ReleaseByService` (reason trimmed, 1 to 500 characters, audit `hold.service_release` with `actor_service` in the same transaction, `released_by_service`, event data with `service_account`).
- `internal/collector/collector.go`: `completeRelease` runs after metering inside the poll transaction, for each hold with the marker whose setup is not reported; owner as `released_by`; event after commit. A failed poll never reaches it (HOLD-9).
- `internal/api`: user release answers 202 for a pending hold; `serviceReleaseHold` implemented; `integrationStubs` is now empty; `release_requested_at` on the Hold JSON and on the user setups-list holder; admin audit list shows a service account as actor (D56).
- `api/openapi.yaml`: 202 response, field on Hold and Holder, service release description; `still_deployed` removed. `ui/src/api/schema.gen.ts` regenerated (`npm run gen:api`; pnpm is not installed here).

## The poll in which the setup disappears (D58)
Metering runs first under the existing rules. The policy sees the setup unreported; the bundled policy keeps metering a hold that already started, so that poll's interval is charged. Then the hold is released with `released_at` = poll time. No new charge rule.

## Decisions appended: D56 (audit actor name for a service account), D57 (repeat request on a pending hold is always the same 202), D58 (completion poll semantics), D59 (`still_deployed` removed; `release_requested_at` kept out of OpenAPI `required` so `ui/src/pages` fixtures still compile).

## Tests (names cite ids)
- `internal/holds/pending_test.go`: `TestHOLD6_ReleaseOfDeployedSetupIsPendingAndFiresOnce`, `TestHOLD6_ConcurrentRequestsFireOnce`, `TestHOLD7_PendingHoldStaysActiveAndCountsTowardLimit`, `TestHOLD7_ForceReleaseCompletesPendingRelease`, `TestHOLD8_ServiceReleaseReleasesAnyActiveHold`, `TestHOLD8_ServiceReleaseRefusals`, `TestHOLD8_ServiceReleaseCompletesPendingRelease`, `TestHOLD8_ConcurrentReleasesFireOnce`.
- `internal/collector/pending_test.go`: `TestHOLD7_PendingHoldIsMeteredThenCompletedWhenSetupDisappears`, `TestHOLD7_HoldWithoutPendingReleaseIsLeftAlone`, `TestHOLD7_ReleaseRacingPollReleasesOnce` (-race), `TestHOLD9_FailedPollLeavesPendingReleasePending`, `TestCMP1_PlainProviderBehavesAsBefore`.
- `internal/api/release_pending_test.go`: `TestHOLD6_UserReleaseOfDeployedSetupAnswers202Once`, `TestHOLD7_PendingReleaseCompletesAtThePollWithoutTheSetup`, `TestHOLD7_AdminForceReleaseCompletesPendingRelease`, `TestHOLD8_ServiceReleaseOverHTTP`, `TestHOLD9_FailedPollDoesNotCompleteReleaseOverHTTP`.
- Also: webhook accepts `hold.releasing`; EXT6 event table and event JSON keys updated; contract test validates 202 and the service release (200, 400, 404, 409).

## Existing tests changed because of the new behaviour
`internal/holds/holds_test.go`: `TestHOLD2_ReleaseRefusedWhileReportedAllowedAfter` (renamed `..._ReleaseWhileReportedIsPendingAllowedAfter`), `TestHOLD2_ReleaseMatchesAllocationBySetupName`, `TestHOLD2_AdminReleasesOwnHoldLikeAnyone`. `internal/api/user_test.go`: `TestHOLD3_MyHoldsShapeAndStateFilter`, the force-release test (still deployed now 202), `TestSET6_SetupsListForDeveloper` (new key). `internal/api/contract_test.go` (409 became 202, service release cases), `helpers_test.go` (still_deployed rows), `permissions_test.go` (stub map emptied), `pkg/hourglass` tests (event table, hold keys).

## Things outside my scope that depend on the old behaviour (tasks 67 and 68)
- `ui/src/pages/setups/setups.test.tsx:249-256` and `ui/src/pages/usage/usage.test.tsx:206-210` mock a 409 `still_deployed`; `ui/src/pages/usage/index.tsx:225` says release "is refused while it is still" deployed; `holdApi.ts` and both pages treat any 2xx as "Released ..." (a 202 would show that toast).
- `e2e/browser/tests/acceptance.spec.ts:439-447` (step A7) expects the refusal text and then clicks Release again; `e2e/out/*/steps.json` hold the same wording.
- The UI minimum needed: none under `ui/src` besides the generated schema.

## Confidence and gaps
- `docs/extending.md`, `docs/deploying.md`, `docs/integrating.md` not touched (docs task).
- `apierr.NotImplemented` and its test uses remain, but nothing returns it now; left in place.
- The admin audit page shows a service actor as id 0 with the account name; the actor schema was not extended.
- The user-release race (poll completes the hold between the user's check and write) answers 409 `conflict`; appears acceptable, not otherwise specified.
