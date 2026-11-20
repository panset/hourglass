# 62c Verify the fixes to service-account authentication: RESULT

Status: done. Date: 2026-11-20. I did not read tasks/62b-*. Judged against the six rulings in BRIEF.md.

## Tests
`go test -race -count=1 -v ./internal/auth/... ./internal/api/...` (log: gotest.log): exit 0, 3 packages ok
(auth, api, api/apierr), 154 top-level tests, 361 PASS including subtests, 0 FAIL, 0 SKIP. `./ci.sh` not run (per brief).
SVC-1..SVC-7 each have a passing test: SVC1 `service_accounts_test.go:68`, SVC2 `:115` and `service_test.go:323`,
SVC3 `:146`, SVC4 `:192`, SVC5 `:286`, SVC6 `:372`, SVC7 `:453` and `service_test.go:199,239,261`.

## Items
1. Closed. `internal/auth/service.go:225-235`: the token is verified first; only `errBadToken` reaches the limiter, so a valid
   token always authenticates and is never counted. Over limit gives 429 `rate_limited` with a fixed body, after the same
   lookup and comparison (`:158-186`), so no signal about id existence. Address is the host of `RemoteAddr` (`:202`);
   no forwarded headers read. Tests: `TestSVC7_FailedBearerAttemptsAreRateLimitedPerAddress`, `TestSVC7_LimiterPerAddress`.
   Break: a valid token also calls the limiter and gets 429 when limited -> red: TestSVC7 (api and auth), TestSVC3, TestSVC5.
2. Closed. `service.go:294-319`: check and record under one mutex; sweep at most once a minute (`:300`); map capped at
   10,000 (`:304`). Tests `TestSVC7_ConcurrentFailuresCannotExceedTheLimit` (200 goroutines, exactly 5 pass, -race) and
   `TestSVC7_LimiterSweepAndCap`.
3. Closed. `service.go:104` rejects empty, leading-zero and non-digit ids. Test `TestSVC_TokenIDMustBeCanonicalDecimal`
   (`service_test.go:294`: `+1`, `01`, `001`, `-1`, `0`, `1.0`, `0x1`, Arabic-Indic digit, empty). Break: removed the
   leading-zero check -> red: that test.
4. Closed. `service_accounts_test.go:555` `TestSVC8_DesignTableDecidesEveryNewRouteForEveryPrincipal` checks routes.go against an
   independent table (`:534`, ten routes: method, pattern, who, scope), fails on a route with no row or a stale row, and
   exercises all actors. Break: DELETE service-accounts/{id} set to AccessRole -> red: that test ("access role, the design
   says admin"; "as Developer: status 204, the design says 403"). Missing row path is `:565`. (I did not break "new route with no row".)
5. Closed. `service_test.go:323` `TestSVC2_AuthenticateIgnoresTheCookieOnIntegrationPaths` (with a control that the cookie
   is honoured outside the prefix). Break: removed the `IsIntegrationPath` skip in `middleware.go:28` -> red: that test.
6. Closed. `docs/decisions.md` D50 (name reuse allowed, no tombstone, ids in audit targets) and D47 (describes behaviour 1,
   including canonical ids, 429 for further failures, single-lock limiter, sweep/cap).

Restore check: `git diff | shasum` before and after all breaks is `04f551bb…`; the diff plus untracked non-.orchestrator files hash
matches `baseline.sha` (`3a3e8e18…`). Breaks touched service.go, routes.go, middleware.go, all restored from byte copies.

## New findings
None above Low. Observations:
- Low (by design, D47): when 10,000 addresses are live in the limiter, a failure from a new address answers 429 instead of 401.
  Only failed attempts are affected; valid tokens never are. Memory stays bounded.
- Low: a malformed-shape token returns before the DB lookup (`service.go:159-162`), so it is slightly faster than a well-formed
  one. It leaks nothing about whether an id exists.
- I found no way to lock out a valid caller (limiter is consulted only after a failed verification) or to bypass it
  (key is the connection host; failures are reserved atomically).

## Confidence and gaps
High on the observed test and mutation results. Not checked: reverse-proxy topology behaviour (compose not run), bazel
test targets, timing measurement, the four integration route bodies (still 501 stubs).
