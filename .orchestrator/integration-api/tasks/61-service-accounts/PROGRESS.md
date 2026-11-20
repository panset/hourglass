# Progress: 61 service accounts

- 2026-11-20 Read BRIEF, INTENT, CONTRACT, design doc, task 60 RESULT, CONTRIBUTING "Add an API route", internal/auth, internal/api.
- 2026-11-20 internal/auth/service.go: token format, secret/hash, verify, RequireService, RequireScope, ServiceRoute, per-address limiter. Authenticate skips the cookie under /api/v1/integration/ (SVC-2).
- 2026-11-20 internal/api: AccessService + Route.Scope, six admin routes (handlers_service.go), four 501 stubs (handlers_integration.go), apierr codes not_implemented, stale_report, not_configured.
- 2026-11-20 api/openapi.yaml: ten operations, schemas, serviceToken security scheme, x-roles service.
- 2026-11-20 Tests: permissions matrix gains two service actors and probes for the ten routes; contract test exercises the admin routes and the integration refusals (stubs exempt from the success check, D49); TestSVC1..8 in internal/api/service_accounts_test.go; unit tests in internal/auth/service_test.go. Mutation checks: removing the scope, enabled, expiry, limiter or cookie boundary, or logging the token, each fails its SVC test.
- 2026-11-20 Decisions D47, D48, D49 appended. schema.gen.ts regenerated; Gazelle updated BUILD files. `go test -race ./internal/...` and check_core_generic.sh pass.
- 2026-11-20 ./ci.sh exit 0 (ci.log). RESULT.md written. Done.
