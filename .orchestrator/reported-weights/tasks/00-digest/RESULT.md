# 00 Digest (condensed by the orchestrator from the scout's reply; the scout had no write tool)

## Where the docs deliverables land
- docs/integrating.md: jobs table at 46-57 (fifth row); Part 2 sections Authentication 296, Report what is deployed 307 (the model), Report lab pressure 346, If you stop reporting 364 (says an expired report fails polls: weights need a contrast note), Reconcile 420, Errors 491, Verify 509, Checklist 537; scopes at 219 and 228. File is self-contained: no links to the design or decisions.
- docs/deploying.md: Settings stored in the database 178; integration API 449 (max_age text 468-470, scopes 476); Upgrade 557 with the 0006/0007 entry at 575 as the pattern (bold-led bullets); Publishing the images 51.
- docs/extending.md: Snapshot, HoldState and Decision 110; `Decision.Weight` 155 and 177; line 158 "`SetupState.Weight` is already resolved from the rate rules" must change.
- README.md: integration paragraph at 151.
- Local server for running examples: `e2e/integration-stack.sh up|token|down` (bundled stack, reported provider and pressure, account `docs-example` with a hardcoded scope list that needs `weights:write`; app on localhost:8080, clock control 8091, webhook 9100). Ports 8080, 8081, 8990, 8091, 9100 must be free; the owner's own processes or the `hourglass-tls` stack on those ports have broken e2e runs before.

## D78 fixtures
- testdata/upgrade-d96a950/: README.md, hourglass.sqlite3, responses.json, openapi.yaml, recorder_upg_record_test.go.txt, recorder_testenv.patch.
- Consumers: internal/app/upgrade_test.go `TestUPG1_...`; internal/api/upgrade_compat_test.go `TestUPG2_...`, `TestUPG3_...`. Bazel data wiring: root BUILD.bazel exports, internal/app/BUILD.bazel, internal/api/BUILD.bazel.

## Version and release
- internal/version/version.go: `Version = "0.0.0-dev"`; nothing stamps it. The v1.1.0 binary reports `0.0.0-dev`; the owner chose to release v1.1.0 without stamping.
- v1.1.0 steps: fast-forward main, push main, annotated tag, push tag, watch .github/workflows/release.yml (runs deploy/push-images.sh TAG latest; no tests; a tag with a hyphen leaves latest alone).

## Test layout
- internal/api/permissions_test.go: `capMatrix`, `permExtraCover`, `TestPermissions_EveryRouteEveryActor`, `TestPermissions_MatrixCoversTheSpecAndEveryRoute`; service_testenv_test.go: `mintServiceAccount`, `serveService`, `allScopes()`, `scopesWithout(s)`; service_accounts_test.go `TestSVC3_EachIntegrationRouteRequiresItsScope`.
- Route row: internal/api/routes.go:86-89; scopes internal/auth/service.go:40-47.
- Places that hardcode the four scopes: ui/src/pages/admin-service-accounts/api.ts and its test; e2e/browser/tests/integration.spec.ts:265,281; docs/integrating.md:219,228; docs/deploying.md:476; e2e/integration-stack.sh; possibly internal/store/integration_queries_test.go.
- internal/api testenv: `newBackend`, `b.serve`, `b.newUser`; internal/collector: `newEnv` in collector_test.go.
- e2e: e2e/run.sh modes; e2e/browser/tests/integration.spec.ts (steps I0-I9); reporter e2e/browser/reporter/reporter.ts; e2e/README.md documents steps.
- Timings: ci.sh about 5 min cached, 10+ uncached; e2e bundled 712 s, split 771 s, integration 830 s. Known timing flake: `TestNFR_LedgerAppendTimeIsLinear`.

## Ids
WGT-n and UI-12..14 are free. Highest UI id UI-11. Next decision D81.
