# Result: 61 service accounts (auth, admin routes, permissions test)

Status: done. `go test -race -count=1 ./internal/...` passes, `tools/check_core_generic.sh` passes, `./ci.sh` exits 0 (`ci.log` in this folder; 36 of 36 Bazel tests pass).

## What was built
- **Bearer auth** (`internal/auth/service.go`). Tokens are `hgsa_<token id>_<64 hex>`. The secret is 32 random bytes from `crypto/rand`, and only hex SHA-256(secret) is stored (`NewServiceSecret` :70, `FormatServiceToken` :87).
  - `verifyServiceToken` (:153) does one store lookup per request and compares in constant time, against a dummy hash when the id is unknown. It also checks that the account is enabled and the token has not expired, then calls the store's once-a-minute `TouchServiceToken`.
  - `RequireService` (:210) handles the limit. It checks the per-address limiter first (429 `rate_limited` with `Retry-After`). A refused token counts as a failure and gets a fixed 401 `unauthenticated`.
  - `RequireScope` answers a missing scope with a fixed 403 `forbidden` ("not allowed").
  - `ServiceRoute(scope, h)` (:258) chains the two and panics on an unknown scope. There is no CSRF check.
- **Boundary.** `Authenticate` does not read the cookie for paths under `/api/v1/integration/` (`internal/auth/middleware.go:28`, SVC-2). Only `ServiceRoute` reads `Authorization`, so on every other route a token is ignored (SVC-1).
- **`internal/api`.**
  - `routes.go` gets `AccessService` and a new `Route.Scope` field. The existing rows gained a trailing `""`.
  - There are ten new rows. The order is list, create, update, issue, revoke, delete-account, so that the router test's walk through the routes does not delete account 1 before the token routes use it.
  - In `api.go:152`, `guard(Route, h)` maps `AccessService` to `ServiceRoute` and panics if a service route is outside the prefix.
  - The six admin handlers are in `handlers_service.go`. Each runs one transaction and writes one audit row. The token appears only in the issue response.
  - The four integration handlers are 501 `not_implemented` stubs in `handlers_integration.go`.
  - `apierr` gains `stale_report`, `not_configured` (409) and `not_implemented` (501).
- **`api/openapi.yaml`.**
  - Ten operations; the integration ones have `x-roles: service`, `x-csrf: false` and `security: serviceToken`.
  - New schemas: the integration request and response schemas from the design (PressureReport/Result, DeploymentsReport/Result, IntegrationState, ServiceReleaseRequest), plus the ServiceAccount family.
  - Also added: an http-bearer `serviceToken` scheme, `ServiceUnauthenticated` and `ServiceForbidden` responses, the three new error codes, and convention text for the service operations.
- **UI types.** `ui/src/api/schema.gen.ts` is regenerated, and the drift test passes. There is no UI page.

## How to use it (for tasks 63, 65, 67)
- **Get the caller in a handler:** `p, ok := auth.ServiceFrom(r.Context())` gives a `*auth.ServicePrincipal{AccountID, AccountName, Scopes, TokenID}`. It is always set behind an `AccessService` route.
- **Register a service route:** add `{"PUT", "/api/v1/integration/...", AccessService, "opId", auth.ScopeX}` to `routes`, add the handler to `handlers()`, and describe the operation in OpenAPI with `x-roles: service`, `x-csrf: false`. The scope constants are `auth.ScopeStateRead`, `ScopePressureWrite`, `ScopeDeploymentsWrite` and `ScopeHoldsRelease`.
- **When you replace a stub:**
  - Remove its op from `integrationStubs` (`internal/api/permissions_test.go`). The permissions test then expects a 2xx, and the contract test requires a validated success.
  - For `getIntegrationState`, also remove the `AccessService` skip in `perf_scale_test.go` `scaleRoutes` and time the route with a token.
  - The probes in `permFix.probe` for the four routes are valid requests. Adjust them if the real handler needs data (for example, a configured `reported` extension).
- **Mint tokens in tests** (`internal/api/service_testenv_test.go`):
  - `b.mintServiceAccount(name, scopes...) svcAccount{acct, token, tokID}` and `b.mintServiceToken(accountID, *expiresAt) (token, id)`.
  - `bearer(token)` returns the header pair for `serve`, and `b.serveService(method, path, token, body)` sends a request with only a token.
  - `allScopes()` and `scopesWithout(s)` build scope lists. In the permissions fixture, `f.svcAll` and `f.svcWithout[scope]` are ready-made tokens.
  - For contract-validated calls there is `contract.callBearer`.
  - In `internal/auth`, the test env is `newSvcEnv`.

## Tests
- `internal/api/service_accounts_test.go`:
  - `TestSVC1_`: on every other route, a token with all four scopes gets the same status, body and Set-Cookie as an anonymous caller. `last_used_at` stays null, and wrong tokens sent there are not counted as failures.
  - `TestSVC2_`: an admin, local admin or developer cookie, with or without CSRF, gets 401. A cookie does not lift a token that lacks the scope, and no CSRF is needed.
  - `TestSVC3_`: there is one identical 403 body that names no scope, and each route opens only with its own scope.
  - `TestSVC4_`: the log, the audit rows and the audit API never contain the token, its secret or its hash. It captures the server log and the audit rows while a token is issued, used, refused, listed, revoked and deleted. The stored hash is SHA-256 of the secret.
  - `TestSVC5_`: covers revoked (rotation keeps the sibling token working), expired, disabled and then re-enabled, deleted, and a removed scope (403).
  - `TestSVC6_`: every other principal is refused and changes neither data nor the audit log; an admin or local admin writes exactly one entry with the right action, actor and target.
  - `TestSVC7_`: after five failures, the address gets 429 even with a valid token. Another address and session users are unaffected.
  - `TestSVC8_`: checks that all seven principals are present and runs the route-by-actor check on the ten new routes.
- `internal/auth/service_test.go`: the format and hash, expiry to the instant on a fake clock, eight malformed or wrong tokens giving the same 401, `last_used_at` once a minute, the middleware boundary, the limiter window, and a panic on an unknown scope.
- `permissions_test.go` gains two actors, `Service_account_with_scope` and `Service_account_without_scope`, in every existing matrix test, plus probes and coverage entries for the ten routes. Service actors count as anonymous outside the integration routes.
- `contract_test.go` validates success and error responses of the six admin routes and the 401 and 403 of the four integration routes.
- **Mutation checks**, run by hand: each change below made its test fail.

  | Change | Failing test |
  | --- | --- |
  | Removed the scope check | SVC3 |
  | Removed the enabled check | SVC5 |
  | Removed the expiry check | SVC5 |
  | Removed the limiter's failure count | SVC7 |
  | Logged the token | SVC4 |
  | Accepted an admin cookie on integration routes | SVC2, SVC8 |

## Contract test and the 501 stubs
The contract test cannot validate a 501 response, because it is not documented on the operations. I did not add 501 to the published operations. Instead, the "no success response validated" check skips any op in `integrationStubs`, and `callBearer` returns a stub's 501 without validating it. The four ops still get validated error responses (401 and 403). See D49.

## Existing tests changed
- `router_test.go`: the route count is now 40. `concretePath` fills in `{tokenId}`, and `seed` creates service account 1 and token 1.
- `perf_scale_test.go`: skips service routes.
- `permissions_test.go`: the exhaustive loop is now `everyRouteEveryActor(t, f, routes)`, and the role-less no-data rule exempts a scoped service actor on its own route.

## Decisions appended
D47 (limiter and client address), D48 (admin input rules, token encoding, audit vocabulary), D49 (501 stubs and the contract-test exemption).

## Files changed
- Go and API: `api/openapi.yaml`; `internal/auth/{auth.go,middleware.go,service.go,service_test.go,BUILD.bazel}`; `internal/api/{api.go,routes.go,handlers_service.go,handlers_integration.go,apierr/apierr.go,BUILD.bazel}`.
- Tests: `internal/api/{service_accounts_test.go,service_testenv_test.go,permissions_test.go,contract_test.go,router_test.go,perf_scale_test.go}`.
- Other: `ui/src/api/schema.gen.ts`; `docs/decisions.md`; this folder (`PROGRESS.md`, `RESULT.md`, `ci.log`).
- Nothing under `internal/store` was needed.

## Commands
`go test -race -count=1 ./internal/...` (pass); `tools/check_core_generic.sh` (OK); `cd ui && npm run gen:api`; `bazel run //:gazelle -- update -build_file_name=BUILD.bazel`; `./ci.sh` (exit 0).

## Confidence and gaps
- **Client address behind a proxy.** The limiter keys on `RemoteAddr` only. Behind a reverse proxy, all service callers share the proxy's address, so one misbehaving client can get the others limited. I chose not to trust `X-Forwarded-For` (D47), and this is worth an owner glance.
- **Two clocks in API tests.** The API test backend gives `auth` the real clock and the API the fake clock. So the SVC-5 expiry case in `internal/api` uses a token that has already expired; the to-the-instant transition is covered in `internal/auth` with a fake clock.
- **Account response has no `created_by`.** The design does not list it, so the response leaves it out. The UI task may want it.
- **Integration handlers are stubs.** Request and response schemas follow the design but are unexercised until tasks 63 and 65; the state schema's nullability of `source`, `provider` and `smoothed` is my inference.
