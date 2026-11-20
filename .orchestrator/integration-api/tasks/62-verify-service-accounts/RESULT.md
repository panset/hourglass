# 62 Independent verification: data layer and service accounts — RESULT

Status: done. Date: 2026-11-20. Scope: commit `d03efa0` plus the uncommitted working tree
(service accounts), judged against `docs/design/integration-api.md` only. I did not read
`tasks/60-*` or `tasks/61-*`.

## Tests run

| Command | Outcome |
| --- | --- |
| `go test -race -count=1 -v ./internal/...` (log: `gotest.log`) | exit 0; 16 packages; **396 top-level tests, 857 tests incl. subtests run, 857 PASS, 0 FAIL, 0 SKIP** |
| `./ci.sh` (log: `ci.log`) | exit 0; SPDX, core-generic, licenses, build, vet and gazelle pass; bazel `Executed 0 out of 36 tests: 36 tests pass` (**all 36 bazel test targets were cache hits**; the cache is keyed on content, so the result is valid, but bazel re-ran nothing; the Go suite above is the fresh run) |
| `tools/check_core_generic.sh` | OK (152 core files, 165 Go files, no violations) |
| `tools/check_spdx.sh` and a header check on each new untracked file | pass; every new file starts with an SPDX MIT line |

## Requirements

Mutation = one deliberate break in production code, the named test run, then the file restored
from a byte copy. Afterwards the hash of `git diff` plus all untracked non-`.orchestrator` files
(`0b925c6f…`) matched the baseline taken before any change (`baseline.sha`); `git diff | shasum`
is unchanged (`701dc2a1…`). Runner: scratchpad `mut/mutate.py`, output `mut/mut.log`.

| Id | Covering tests | Mutation | Result | Verdict |
| --- | --- | --- | --- | --- |
| SVC-1 | `TestSVC1_ServiceTokenIsAnonymousOutsideIntegrationRoutes` (api), `TestSVC1_SVC2_BoundaryInTheMiddleware` (auth), `TestPermissions_EveryRouteEveryActor` (service actors send a token on every route) | AccessRole routes accept a bearer token via `RequireService` (`internal/api/api.go` guard) | red: TestSVC1 | pass |
| SVC-2 | `TestSVC2_SessionCookieDoesNotAuthenticateOnIntegrationRoutes`, `TestSVC1_SVC2_BoundaryInTheMiddleware`, permissions matrix | (a) cookie skip removed in `Authenticate` and `RequireService` accepts a signed-in user; (b) only the cookie skip removed | (a) red: TestSVC2, TestPermissions_EveryRouteEveryActor (4 integration subtests), TestSVC1_SVC2_Boundary. (b) all green (see finding 6) | pass |
| SVC-3 | `TestSVC3_EachIntegrationRouteRequiresItsScope` (also checks the fixed body names no scope and the four routes use four distinct scopes) | `RequireScope` check disabled | red: TestSVC3 | pass |
| SVC-4 | `TestSVC4_TokenIsShownOnceAndNeverLoggedOrAudited` (api: issue response, store hash, listing, update, refusal bodies, server log, audit rows, audit API), `TestSVC4_TokenFormatAndStoredHash` (auth) | token hash added to the issue audit row | red: TestSVC4 | pass |
| SVC-5 | `TestSVC5_RevokedExpiredDisabledOrDeletedTokenIsRefused` (api), `TestSVC5_ExpiryAndBadTokensAre401` (auth, expiry to the millisecond with a fake clock) | (a) `!a.Enabled` check removed; (b) expiry check removed | (a) red: TestSVC5 (api); (b) red: both TestSVC5 | pass |
| SVC-6 | `TestSVC6_AccountAndTokenChangesAreAdminOnlyAndAudited`, contract test validation cases (`contract_test.go:676-690`) | (a) delete writes no audit row; (b) DELETE account route set to AccessRole | (a) red: TestSVC6; (b) red: TestSVC6, but TestSVC8 stayed green (finding 5) | pass |
| SVC-7 | `TestSVC7_FailedBearerAttemptsAreRateLimitedPerAddress` (api), `TestSVC7_LimiterPerAddress` (auth) | failures never recorded | red: TestSVC7 | pass (but see finding 1 on the behaviour the test pins) |
| SVC-8 | `TestSVC8_PermissionsCoverEveryNewRouteForEveryPrincipal`, `TestPermissions_EveryRouteEveryActor`, `TestPermissions_MatrixCoversTheSpecAndEveryRoute` | see SVC-6 (b) | the matrix does not notice a wrong access level on a new route | weak test (finding 5); coverage itself verified |
| INT-1 (store/service part) | `TestINT1_PressureValueValidatedAndNothingStored` (reported) | upper bound widened to 2 | red: TestINT1 | pass for `internal/reported`; the HTTP 400 is not testable yet (route is 501) |
| INT-2 | `TestINT2_ReplaceIsAtomicForConcurrentReaders`, `TestINT2_FailedReplaceIsAllOrNothing` (reported), `TestReportedDeploymentsReplaceIsAtomic` (store) | (a) read of header and set not in one transaction; (b) replace not in a transaction | (a) red in both packages, 4 of 4 runs in reported; (b) red: TestINT2_FailedReplace | pass |
| INT-3 | `TestINT3_OlderPressureRefusedNothingChanges`, `TestINT3_EqualPressureReplacesAndIsIdempotent`, `TestINT3_OlderDeploymentsRefusedNothingChanges`, `TestINT3_EqualDeploymentsIdempotent` | (a) stale check removed for pressure; (b) equal deployments report refused | (a) red: OlderPressure; (b) red: EqualDeployments | pass for `internal/reported`; the 409 `stale_report` mapping is not testable yet (route is 501) |
| INT-6 | `TestINT6_ReportsSurviveReopen` (reported), `TestReportedPersistsAcrossReopen` (store) | none (persistence is the file database itself; the test closes and reopens the same file, which I read) | n/a | pass |

## Security read of the token path

Verified (code read and a temporary probe test, deleted after the run; file list and hash unchanged):

- Entropy: 32 bytes from `crypto/rand`, hex (`internal/auth/service.go:70-77`). Stored: hex SHA-256 of the secret only (`:80-83`, `internal/store/service_accounts.go:189-201`).
- Comparison: `subtle.ConstantTimeCompare` of hashes; an unknown id compares against a dummy hash after the same one-row query; unknown, wrong, expired, disabled, deleted and malformed all give the same 401 body (`service.go:153-181`, `:136-140`). `last_used_at` is written only on success. No error-message difference; timing difference limited to an indexed lookup hit or miss, and ids are sequential and not secret anyway.
- Parsing: `Bearer` without a space, `Bearer `, tab separator, `Basic`, extra `_x`, `hgsa__…`, trailing text and a 500 kB token are all refused 401 without panics; scheme is case-insensitive. Non-canonical ids (`hgsa_+1_…`, `hgsa_01_…`) are accepted (finding 4).
- Every request re-reads the token and account (`GetServiceTokenWithAccount`, a JOIN, so a deleted account's tokens fail even without the cascade; the cascade is present and `foreign_keys(1)` is set, `internal/store/store.go:121`).
- Path tricks with a full-scope token and with an admin cookie: `..`, `%2e%2e`, `%2F`, `..%2F`, trailing slash, upper case, `//`, `/./`, `/me/../integration/state`: every one is a 404 JSON or a 307 to the cleaned path (re-evaluated on the follow-up request). No non-integration handler was reached with the token; no integration handler with the cookie. Structurally, bearer is read only by `RequireService`, which `guard` attaches only to `AccessService` routes and panics if their pattern is outside the prefix (`internal/api/api.go` guard).
- No integration route honours a cookie: `RequireService` never consults `UserFrom`; `Authenticate` also skips the session under the prefix.
- Spoofed `X-Forwarded-For` is ignored (`service.go:197-202`).
- Token never seen in the server log, audit rows, audit API, listing, update response or error bodies (TestSVC4, which first checks the log captured the requests).
- Scope validation on create and update: unknown scope is 400, duplicates dropped (`handlers_service.go:103-110`; contract test `:676`, `:679`). `ServiceRoute` panics on an unknown scope at registration.
- Data model vs design table: `service_accounts`, `service_tokens` (with `ON DELETE CASCADE`), `reported_pressure`, `reported_deployments_meta`, `reported_deployments` (`setup_name` PK), `holds.release_requested_at`, `holds.released_by_service`, `audit_log.actor_service` all match column for column (`internal/store/migrations/0005_integration_api.sql`). Service account names are text on holds and audit rows. No resource name, template or override anywhere in the diff (D40).
- Permissions test: 8 actors (anonymous, unassigned, developer, admin, local admin, local admin before password change, service with scope, service without scope) times every route; combination count asserted; a route added to `routes.go` without a probe or matrix/extra-cover entry fails `TestPermissions_MatrixCoversTheSpecAndEveryRoute` (`permissions_test.go:1180-1188`) and `probe` fatals.

## Findings

1. **High** `internal/auth/service.go:217-222` (behaviour recorded as D47). Once an address has 5 failures, every bearer request from it gets 429, a valid token included, and the address is `r.RemoteAddr`. `docs/deploying.md:190` requires a reverse proxy in front of Hourglass and `:197` says Hourglass reads no `X-Forwarded-*`; the split compose deployment also puts `hourglass-ui` as a reverse proxy in front of the API (`docs/deploying.md:11,36`). Failure scenario: in any documented deployment every client shares the proxy's address, so anyone who can reach the URL sends 5 requests with `Authorization: Bearer x` every 15 minutes and every legitimate integration (reports, state, release) gets 429 indefinitely; reports then go stale and, per INT-5, every poll fails. Fix: do not refuse a token that verifies (with 256-bit secrets the limit only needs to throttle failures, e.g. delay or refuse only the verification of further failures), or key the limiter by (address, token id); optionally a configured trusted-proxy list. Needs the owner's call since D47 chose this deliberately.
2. **Low** `internal/auth/service.go:297-312`. Above 1024 tracked addresses every failure sweeps the whole map under the global lock, and entries live 15 minutes. Failure scenario: a client with many source addresses (an IPv6 /64) keeps more than 1024 live entries, making each failed attempt O(n) under a lock that every bearer request takes; memory grows with the number of addresses. Fix: sweep on a timer or at most once per interval, and cap the map (e.g. LRU).
3. **Low** `internal/auth/service.go:218-226`. Check-then-record is not atomic: N concurrent bad requests from one address all pass `limited` before any `fail` is recorded, so more than 5 attempts can be made per window (the local sign-in path serializes with `localMu`, this one does not). Failure scenario: a burst of 1,000 parallel guesses is all verified. Impact is low with 256-bit secrets. Fix: reserve the attempt inside the limiter under one lock (increment, then verify).
4. **Low** `internal/auth/service.go:102`. `strconv.ParseInt` accepts `+1` and `01`, so `hgsa_+1_<secret>` and `hgsa_01_<secret>` authenticate as token 1 (verified by probe). Not an access bypass (the secret is still required), but the token has several valid spellings, which defeats exact-match token scanning or deny-listing of a leaked token string. Fix: require `idPart` to match `^[1-9][0-9]*$`.
5. **Low (weak test)** `internal/api/permissions_test.go` `gateDenial` / `TestSVC8_…`. The expected answer for each new route is derived from `r.Access` in `routes.go`, the code under test; there is no independent table for the ten new routes (the spec routes have `capMatrix` against `docs/SPEC.md`). Mutation: DELETE `/api/v1/admin/service-accounts/{id}` changed to `AccessRole`; TestSVC8 and the matrix stayed green, only TestSVC6's explicit "access must be admin" check caught it. Fix: give the new routes an explicit expected-access row in the matrix (admin for the six, service plus scope for the four), checked against the design's tables.
6. **Low (untested defence in depth)** `internal/auth/middleware.go:28-31`. Removing the `IsIntegrationPath` skip in `Authenticate` leaves every test green: SVC-2 holds anyway because `RequireService` ignores the session. Failure scenario: a later change that reads `UserFrom` in an integration handler (e.g. for audit actor) would silently start honouring cookies on those routes with no test noticing the skip had gone. Fix: a test that an integration request carrying a valid session cookie leaves no user in the context (e.g. a handler-level assertion, or a session side effect such as last-seen not being updated).
7. **Low** `internal/store/migrations/0005_integration_api.sql:9` with the design's "names stored as text". `name` is unique only among live accounts, so after deleting `deployer` a new `deployer` can be created and holds/audit rows written by the two are indistinguishable. Failure scenario: an auditor attributes a release made by the old, deleted account to the new one. Fix: refuse reuse of a deleted name (keep a tombstone) or also record the account id as plain text.

## What I could not check

- The four integration route bodies are 501 stubs (out of scope), so INT-1's 400, INT-3's 409 `stale_report` and the 1 MB / 10,000-entry limits are not testable over HTTP yet; INT-2/3 were judged at the `internal/reported` and store layers.
- Bazel tests were cache hits in `./ci.sh`; I did not force a bazel re-run (the fresh `go test -race` run covers the same Go code).
- Finding 1's deployment impact is inferred from `docs/deploying.md`; I did not run the compose stack.
- The timing analysis is by reading; I did not measure response-time distributions.
- UI (Service accounts page) and `api/openapi.yaml` content were not reviewed beyond the contract test passing.

## Files changed

Only this task folder: `PROGRESS.md`, `RESULT.md`, `baseline.sha`, `ci.log`, `gotest.log`.
Temporary production edits (17 mutations) and one temporary test file
`internal/api/zz_v62_probe_test.go` were restored or deleted; tree hash verified equal to the baseline.

## Confidence and gaps

High confidence in the mutation results and the probe results (observed). Severity of finding 1
depends on deployment topology (inference from the deploy docs). No decisions were made for
`docs/decisions.md`.
