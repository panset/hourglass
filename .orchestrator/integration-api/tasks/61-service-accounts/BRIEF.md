# 61 Service accounts: authentication, admin routes, permissions test

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Ground rules, Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/61-service-accounts/`.

## Goal
Service accounts work end to end as a principal: an admin can manage accounts and tokens over
the API, a service token authenticates only under `/api/v1/integration/`, and requirements
SVC-1 to SVC-8 each have a test named after the id that fails when the requirement is broken.

## Context
- Design: `docs/design/integration-api.md`. Read all of it; your sections are "Service accounts"
  (Rules, Scopes, Admin routes) and requirements SVC-1 to SVC-8.
- The data layer is done: read `.orchestrator/integration-api/tasks/60-store-reported/RESULT.md`
  for the store queries you build on.
- `CONTRIBUTING.md`, "Add an API route" (OpenAPI first; contract and permissions tests).
- Existing code: `internal/auth/` (sessions, CSRF, role middleware, `ratelimit.go`),
  `internal/api/routes.go` (access levels), `handlers_admin.go`, `permissions_test.go`,
  `contract_test.go`, `api/openapi.yaml`.

## What to build
1. `internal/auth`: bearer authentication for `hgsa_<token id>_<secret>` tokens. At least 256
   bits of randomness in the secret; only a SHA-256 hash stored; constant-time comparison;
   optional `expires_at`; `last_used_at` written at most once a minute; a service principal in
   the request context (account name, scopes); a `ServiceRoute(scope)` chain. Disabled account,
   revoked, expired or deleted token: 401 on the next request (SVC-5). Missing scope: 403
   `forbidden` with nothing else revealed (SVC-3). Failed bearer attempts are rate-limited per
   client address with the existing limiter and answer 429 `rate_limited` (SVC-7).
2. The boundary, both ways: a service token on any route outside `/api/v1/integration/` is
   ignored and the request is anonymous (SVC-1); a session cookie, even an admin's, does not
   authenticate on `/api/v1/integration/` routes, which read no cookie and need no CSRF (SVC-2).
3. `internal/api`: a new `AccessService` level in `routes.go` carrying the required scope, and
   the six admin routes in the design's table (admin role, CSRF, audit entry each, SVC-6). The
   token appears in the issue response only, never in a list, log, audit entry or error (SVC-4).
   Validate scopes against the four in the design; unknown scope is 400 `validation`.
4. The four integration routes do not exist yet (tasks 63 and 65 add them). So that SVC-1, 2, 3,
   5, 7 and 8 are testable now and the route table is complete, register all four paths at
   `AccessService` with their scopes and OpenAPI entries (`x-roles: service`), with handlers
   that answer 501 `not_implemented` after authentication and scope checks pass. Describe the
   request and response schemas from the design in the OpenAPI file now; tasks 63 and 65 replace
   the handlers. If the contract test cannot tolerate a 501 handler, say how you handled it.
5. `internal/api/permissions_test.go`: every new route (six admin, four integration) for every
   principal: anonymous, unassigned, developer, admin, local admin, service account with the
   scope, service account without the scope (SVC-8). Also assert that a service token with all
   four scopes gets exactly what an anonymous caller gets on every pre-existing route.
6. Regenerate `ui/src/api/schema.gen.ts` (`cd ui && npm run gen:api`) so the drift test passes.
   No UI page: that is task 67.

## Scope
May read: the whole repository.
May write: `internal/auth/`, `internal/api/`, `api/openapi.yaml`, `ui/src/api/schema.gen.ts`,
`internal/store/` (only if a query is missing; list it), wiring in `internal/app/`,
`pkg/hourglass/server` or `cmd/` if the router needs the new dependencies, `BUILD.bazel` files
via Gazelle, `docs/decisions.md` (append rows with the next free number for choices the design
leaves open), your task folder.
Must not touch: `docs/SPEC.md`, `docs/design/`, `docs/integrating.md`, other UI files,
`internal/holds/`, `internal/collector/`, git state.

## Done when
- `TestSVC1_...` to `TestSVC8_...` exist and pass; SVC-4 includes a test that captures the
  server's log output and the audit rows during issue and use of a token and finds neither the
  token nor its secret part in them.
- `go test -race -count=1 ./internal/...` passes; `./ci.sh` passes.
- `tools/check_core_generic.sh` passes.

## Long-running steps
`./ci.sh` runs Bazel and takes roughly 5 to 15 minutes: run it once at the end, in the
foreground, with a 30-minute timeout, output to a log file in your task folder, and read only
the exit code and tail.

## Report
Follow the Reporting section of the contract. In RESULT.md also give: how a handler obtains the
service principal, how to register a service route, how tests mint a service token (helper
names), and the numbers of any decision rows you appended.
