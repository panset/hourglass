# Report: integration API (push model)

Written by the report agent (task 72); saved by the orchestrator, which corrected the attribution of the rulings behind D47, D50 and D61: they were the orchestrator's, not the owner's.

Date 2026-11-20. Base `fd948a5` (origin/main). Contract: `docs/design/integration-api.md`. Nothing is pushed.

Result in one paragraph: every requirement has a passing test named after its id, and every one was broken on purpose by an independent agent and the test went red. `./ci.sh` passes from a clean clone of `cc6a4ca`. The final commit `b8ccacd` (the last low findings) was checked only by its own author and by `./ci.sh`, not by an independent agent. Four items still stand under "Not done" (section 5).

## 1. What was built

| Milestone | Commit | Content |
|---|---|---|
| 0 | `5808957` | Your edits to the design doc and the draft guide, committed unchanged |
| 1 | `d03efa0` | Migration 0005, store queries, `internal/reported` (ordering, staleness, atomic replace) |
| 2 | `8c73c6e` | Service accounts: scoped bearer tokens, six admin routes, failed-attempt limiter, permissions matrix |
| 3 | `6e45118` | Report routes (pressure, deployments) and the state route, OpenAPI |
| 4 | `8a19e98` | `reported` provider and pressure source, `check-provider` ages |
| 5 | `5aa31c4` | Pending release, `hold.releasing`, collector completion, service release route |
| 6 | `88ae112` | UI: Service accounts page, "Releasing" state, last report times |
| 7 | `8aabcfc` | End-to-end scenario with scripted reporter and webhook receiver |
| 8 | `cc6a4ca` | Docs: guide run against the live server, banner removed; extending, deploying (Upgrade entry), README, progress |
| 8b | `b8ccacd` | Closes the final verification's low findings (CMP-2 test, decisions table, D60 residuals, README paragraph) |

## 2. Requirements

"Mutation" = an agent broke the production code on purpose and the named test went red. Verifiers: V62 (data layer, service accounts; V62c re-checked its fixes), V66 (reports, extensions, release), V69 (UI and end-to-end), V71 (final, from a clean clone; its mutations M1-M10, M9 re-run as M9b). E2E = step of `e2e/run.sh integration` (I1 account and token via UI, I2 first reports, I3 acquire, I4 reported deployed starts metering, I5 release pending, I6 teardown completes it, I7 reporter silent past `max_age`, I8 service release, I9 negative checks, I10 account lifecycle in UI).
Source note: V71's full report could not be written to a file by the harness, so this table is rebuilt from the RESULT.md files (V71's own RESULT says every id is covered and mutation-checked). Test names are the main ones; each id has more.

| Id | Tests | Mutation-checked | Independent check | E2E |
|---|---|---|---|---|
| SVC-1 | `TestSVC1_ServiceTokenIsAnonymousOutsideIntegrationRoutes`, `TestSVC1_SVC2_BoundaryInTheMiddleware` | yes | V62, V62c, V71 | I9 |
| SVC-2 | `TestSVC2_SessionCookieDoesNotAuthenticateOnIntegrationRoutes`, `TestSVC2_AuthenticateIgnoresTheCookieOnIntegrationPaths` | yes (the second added after V62) | V62, V62c | I9, I10 (admin UI sends no integration requests) |
| SVC-3 | `TestSVC3_EachIntegrationRouteRequiresItsScope` | yes | V62 | I9 |
| SVC-4 | `TestSVC4_TokenIsShownOnceAndNeverLoggedOrAudited`, `TestSVC4_TokenFormatAndStoredHash` | yes | V62 (plus a security read of the token path) | I1 (token absent from storage, URL, page after reload) |
| SVC-5 | `TestSVC5_RevokedExpiredDisabledOrDeletedTokenIsRefused`, `TestSVC5_ExpiryAndBadTokensAre401` | yes | V62 | I10 |
| SVC-6 | `TestSVC6_AccountAndTokenChangesAreAdminOnlyAndAudited` | yes | V62 | I1, I10 |
| SVC-7 | `TestSVC7_FailedBearerAttemptsAreRateLimitedPerAddress`, `_LimiterPerAddress`, `_ConcurrentFailuresCannotExceedTheLimit`, `_LimiterSweepAndCap` | yes | V62, V62c | none (guide run: 429 observed live in task 70) |
| SVC-8 | `TestSVC8_DesignTableDecidesEveryNewRouteForEveryPrincipal`, `TestSVC8_PermissionsCoverEveryNewRouteForEveryPrincipal`, `TestPermissions_EveryRouteEveryActor` (8 principals) | yes (the independent table added after V62 found the original test weak) | V62, V62c | none |
| INT-1 | `TestINT1_PressureValueValidated` (api), `TestINT1_PressureValueValidatedAndNothingStored` | yes | V62 (store), V66 (HTTP) | none (guide run, task 70) |
| INT-2 | `TestINT2_DeploymentsReplaceWholeSet`, `TestINT2_ReadersNeverSeeAMixOfTwoReports`, `TestINT2_ReplaceIsAtomicForConcurrentReaders` | yes | V62, V66 | none |
| INT-3 | `TestINT3_OlderReportRefusedEqualReplaces`, `TestINT3_ReportDoesNotPollDebitOrEmit` | yes | V62, V66 | none (guide run: older 409, equal 200) |
| INT-4 | `TestINT4_NotConfiguredRefusedAndNothingStored` | yes | V66 | none (guide run, task 70) |
| INT-5 | `TestINT5_NoReportThenStaleThenFresh`, `TestINT5_StalePressureAloneFailsThePoll`, `TestINT5_RunPublishesTheStoredReportsToTheExtensions`, mixed-pairing tests, `TestEXT5_Conformance` | yes; V66 found two gaps, both closed and re-broken by V71 (M1, M2, M6, M7) | V66, V71 | I7 |
| INT-6 | `TestINT6_ReportsSurviveReopen` (reported, providers/reported) | yes | V62, V66 | none |
| INT-7 | `TestINT7_ReportedSetupWithHoldGetsDeployedAndMeteringStarts`, `TestINT7_AdminCollectorShowsReportTimes` | yes | V66, V69 | I2, I4 |
| INT-8 | `TestINT8_StateHasEverySetupAndHoldAndNothingElse` (exact key list) | yes | V66 | I2 |
| HOLD-6 | `TestHOLD6_ReleaseOfDeployedSetupIsPendingAndFiresOnce`, `TestHOLD6_UserReleaseOfDeployedSetupAnswers202Once`, `TestHOLD6_ConcurrentRequestsFireOnce`, `TestHOLD6_StaleRequestAfterMarkerFiresNothing`, `TestHOLD6_ReleasingIsQueuedBeforeAReleaseThatRacesIt` | yes | V66, V69, V71 (M3, M4, M8) | I5 (also on the Setups row) |
| HOLD-7 | `TestHOLD7_PendingHoldIsMeteredThenCompletedWhenSetupDisappears`, `TestHOLD7_PendingHoldStaysActiveAndCountsTowardLimit` | yes | V66, V69 | I5, I6 |
| HOLD-8 | `TestHOLD8_ServiceReleaseReleasesAnyActiveHold`, `TestHOLD8_ServiceReleaseOverHTTP`, `TestHOLD8_SecondServiceReleaseConflictsAtTheStore` | yes | V66, V69, V71 (M5) | I8 (audit entry checks actor, hold id, setup, reason) |
| HOLD-9 | `TestHOLD9_FailedPollLeavesPendingReleasePending`, `TestHOLD9_FailedPollDoesNotCompleteReleaseOverHTTP` | yes | V66 | I7 (poll fails, hold stays) |
| CMP-1 | `TestCMP1_PlainProviderBehavesAsBefore` | yes, by V71 only (M9b, M10); V66 read it but did not mutate | V66, V71 | existing acceptance run A0-A9 passes (bundled and split) |
| CMP-2 | `./ci.sh` (exit 0, 37-38 Bazel tests); `TestCMP2_CIRunsEveryRequiredStep` added in 71b | the new test: author only (mutation.log in 71b) | `./ci.sh` by V62, V66, V71 (V71 fresh, uncached, on `cc6a4ca`); the new test: none | none |

Not requirement-numbered but covered: the UI (170 unit tests, V69), and D43's removal of the refusal (e2e A7 in the acceptance run now proves the pending release).

## 3. What independent verification found, and what was done

Real defects and weak tests only; no verifier found a production defect that broke a numbered requirement except the first.

V62 (service accounts):
- High: a valid token got 429 once an address had 5 failures, so anyone behind the documented reverse proxy could lock every integration out. Fixed: token verified first, only failures are limited (D47, orchestrator ruling, marked REVIEW). V62c confirmed.
- Low: limiter sweep was O(n) per failure and unbounded; fixed (sweep once a minute, cap 10,000).
- Low: check-then-record race let more than 5 attempts through; fixed (one lock).
- Low: `hgsa_+1_...` and `hgsa_01_...` also authenticated; fixed (canonical ids only).
- Low: SVC-8 derived expectations from the code under test; fixed (independent table of the ten routes).
- Low: the cookie skip in `Authenticate` was untested; fixed.
- Low: a deleted account's name could be reused; closed by orchestrator ruling D50 (no tombstone).

V66 (reports, extensions, release):
- Medium: no test failed when the pressure staleness check was removed (money risk); fixed, re-broken by V71.
- Medium: the line publishing stored reports to the extensions in `app.Run` was untested; fixed with a composition-root test, re-broken by V71.
- Low: two concurrency guards (`hold.releasing` once, service release once) were untested; fixed with store-level tests.
- Low: `hold.releasing` could be queued after a `hold.released` for the same hold; fixed by queuing it inside the transaction (D60); residual cases remain (section 5).
- Low: dead `not_implemented` 501 code and stub maps; removed (D49 superseded).
- Low: mixed `reported`/non-reported pairings untested; two tests added.
- Two owner-visible behaviours (release decided from the last poll; immediate release with no successful poll): the orchestrator kept them for the owner to confirm, now D61 (REVIEW).

V69 (UI, end-to-end): five low findings, all weak assertions in the e2e (fixed sleep for "no second event", Setups row, Settings report times, audit target and reason, token not in storage and no integration requests from the UI). Fixed in 69b; the scenario was re-run from a fresh stack (11 of 11 steps). No product bug.

V71 (final): five low findings. Closed in 71b: CMP-2 had no test, decisions D51-D53 columns swapped, D60 residual cases not marked REVIEW, README had a link but no paragraph. Not closed: wireframes (section 5).

Task 68 (author, not a verifier) reported no product bug; task 70 corrected the guide in about ten places where it differed from the running server (response shapes, equal `observed_at` accepted, last interval charged on completion, `username` empty for OIDC users, and others).

## 4. Decisions recorded (`docs/decisions.md`, D42 to D63)

REVIEW first:
- **D43 REVIEW** (owner): a user's release of a still-deployed setup returns 202, sets `release_requested_at`, fires `hold.releasing`; no more 409 `still_deployed`, also with `vsphere`; cannot be cancelled.
- **D60 REVIEW** (agent): `hold.releasing` is queued inside the transaction to keep it ahead of `hold.released`; three residual cases listed (commit failure, early delivery, failed owner lookup).
- **D61 REVIEW** (orchestrator kept the existing behaviour; owner to confirm): a release is pending only if the last successful poll reported the setup; after a failed poll it follows the last good view; with no successful poll since start it is immediate.

Others:
- D42 (owner): the change itself, and the six edits the spec's source needs (requirements, HOLD-2 wording, routes, data model, roles column, `hold.releasing` event).
- D44: `last_used_at` written at most once a minute, in one conditional UPDATE in the store.
- D45: `internal/reported` validates value range, names, skew (2 min); size limits stay in the handler; times truncated to ms.
- D46: single-row report tables without id; replace and reads in one transaction.
- **D47 REVIEW** (orchestrator ruling on V62): failed-attempt limiter rules (5 per 15 min per address, valid token never limited, no `X-Forwarded-For`, 429 after the limit).
- D48: account and token input rules, token format `hgsa_<id>_<64 hex>`, audit action names.
- D49: temporary 501 stubs for the routes; superseded, removed.
- D50 (orchestrator ruling on V62): a deleted account's name can be reused; holds and audit store names as text; no tombstone.
- D51: the API learns `reported` and `max_age` from config alone (no extension import); unconfigured gives 409 `not_configured`.
- D52: state fields for reports are null when the extension is not `reported`; poll and pressure fields' sources.
- D53: report check order: scope, `not_configured`, body, ordering; limits 10,000 entries, 32 labels, 1 MB.
- D54: extensions read reports through the small public package `pkg/storedreports`, published by `internal/app` (process-wide).
- D55: extensions parse `max_age` themselves; `check-provider` prints report ages.
- D56: audit listing shows a service actor under the account's name instead of "system".
- D57: a repeat user release of a pending hold answers 202 and changes nothing.
- D58: the poll that completes a pending release charges its interval first, then releases; no new charge rule.
- D59: `still_deployed` error code removed; `release_requested_at` kept out of the OpenAPI `required` list (changed in D62).
- D62: last report times exposed as two new fields on the existing admin collector view; `release_requested_at` now required in OpenAPI.
- D63: what the end-to-end scenario does outside the UI (reporter, clock, Keycloak call, admin API reads, second account for negative checks).

## 5. Not done, or not verified

Not done:
- **Wireframes still say a release is refused.** `docs/wireframes/project/*.dc.html` was left untouched on purpose (V71 finding 5, deliberately not fixed in 71b). They are out of date against D43.
- **D60 residual cases (a) to (c)** stand: a failed commit can leave one delivered `hold.releasing`; a webhook can arrive before the commit; a failed owner lookup skips the event. The design answer is that receivers treat `GET /state` as authoritative; the guide says so. Case (b) is inferred from code structure, not observed.
- **D61 behaviours** stand pending your review: with no successful poll since start, a release of a deployed setup is immediate and the service sees `hold.released`, not `hold.releasing`.
- **Spec source edits:** `docs/SPEC.md` is unchanged as required; the six edits in D42 still have to be made in its source.

Not independently verified:
- **Commit `b8ccacd` (task 71b)** was checked by its author and `./ci.sh` only. That covers `TestCMP2_CIRunsEveryRequiredStep`, the decisions table edits, D60 and the README paragraph. No independent agent re-broke the new test or read the edits.
- Task 70's docs changes outside the live-run table (`extending.md`, `deploying.md`, README) were checked by V71 only partly (guide examples re-run; the rest by reading).

Not verified at all, or only by inference:
- Reverse-proxy behaviour of the limiter (D47) was never run in the compose topology; it is inferred from `docs/deploying.md`. Timing differences of the 401/429 paths were analysed by reading, not measured.
- Not covered by any end-to-end run: `hold.releasing` receipt after a service release of a pending hold; the `vsphere` provider with the new release behaviour beyond the acceptance run's step A7; the `hold.acquiring` veto delivery re-run (task 70); a delivery from the server using the exact secret in the guide's config snippet.
- V71 did not re-run the pending-release path or webhooks live (V69 and task 70 did), did not test `not_configured` or the size limits live (task 70 did), and did not open the UI.
- The end-to-end scenario needs Docker and is outside `bazel test //...` by design (D63), so `./ci.sh` does not run it. It last ran green (11 of 11 steps) in task 69b, before the docs commit and 71b; later commits touched docs and a test, not product code (inference from commit subjects).
- `ui/src/api/schema.gen.ts` had one enum member removed by hand in 66b, not regenerated; the drift test passes.
- Verifier mutations were hand-picked, not a full mutation run. Two verifiers saw mostly cached Bazel results; V71 and the Go suites under `-race` were fresh.
- V71's clock-control port was moved to 18091 with a scratch override because host port 8091 is used by an unrelated process on this machine.
- D45 accepts that reports less than 1 ms apart can compare equal after truncation; not checked beyond that.

## 6. State of the repository

Branch `main`, ahead of `origin/main` (`fd948a5`), nothing pushed, no history rewritten. No uncommitted changes apart from untracked task logs and the scratch folder `tasks/70-docs/work/`. `docs/SPEC.md` and `docs/design/integration-api.md` unchanged since `5808957`. A docs-only commit after `b8ccacd` corrects the attribution of D47, D50 and D61, marks D47 REVIEW and saves this report.
