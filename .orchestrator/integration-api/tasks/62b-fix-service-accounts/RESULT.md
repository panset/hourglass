# 62b Fix the findings of verification 62 - RESULT

Status: done. `go test -race -count=1 ./internal/...` exit 0 (gotest.log); `./ci.sh` exit 0, 36 of 36 bazel tests pass (ci.log).

| Finding | Outcome |
| --- | --- |
| 1 High: valid token refused with 429 | fixed. `RequireService` (internal/auth/service.go) verifies first; a valid token always authenticates and is not a failure; only a failed attempt from an over-limit address gets 429 (same lookup first, one fixed body, so no id-existence signal). Tests: `TestSVC7_FailedBearerAttemptsAreRateLimitedPerAddress` (api), `TestSVC7_LimiterPerAddress` (auth). D47 rewritten in place. |
| 2 Sweep cost | fixed. Sweep at most once a minute, map capped at 10,000 addresses. Test: `TestSVC7_LimiterSweepAndCap`. |
| 3 Race in check-then-record | fixed. `addrLimiter.attempt` checks and records under one lock. Test: `TestSVC7_ConcurrentFailuresCannotExceedTheLimit` (200 goroutines, exactly 5 pass; run under -race). |
| 4 Non-canonical token ids | fixed. Only `[1-9][0-9]*` accepted; others are a 401 failure. Test: `TestSVC_TokenIDMustBeCanonicalDecimal`. |
| 5 Weak SVC-8 test | fixed. Independent design table `svcDesignRoutes`; a route under the admin service-accounts or integration prefixes without a row, or with another access or scope, fails; expected result per all principals comes from the table. Test: `TestSVC8_DesignTableDecidesEveryNewRouteForEveryPrincipal`. Mutation checked: DELETE account route set to AccessRole now fails it. |
| 6 Cookie-skip untested | fixed. Test: `TestSVC2_AuthenticateIgnoresTheCookieOnIntegrationPaths` (with a control outside the prefix). |
| 7 Names reusable after deletion | closed, D50 (new row). |

Files changed: internal/auth/service.go, internal/auth/service_test.go, internal/api/service_accounts_test.go, docs/decisions.md, this folder. No openapi or UI change was needed.

## Confidence and gaps
The sweep and cap are tested with a synthetic clock, not under load. The 429 timing is equal by construction (verification precedes the limiter), not measured. Behind a proxy all callers share one failure budget (stated in D47).
