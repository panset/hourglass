# 05 Milestone 4: the scope, the report route and the state fields — RESULT

Status: **done**. `./ci.sh` exit 0 (`ci.log`, `ci.exit`; 40 of 40 Bazel tests pass). `go test ./...` exit 0 (`gotest.log`). `go test -race` on the touched api tests exit 0 (`race.log`). UI typecheck and vitest pass with the regenerated types and no UI source change (`ui.log`). `tools/check_core_generic.sh` exit 0.

## What was built

1. **Scope (D3).** `auth.ScopeWeightsWrite = "weights:write"`, appended to `auth.Scopes()` after `holds:release` (`internal/auth/service.go:40-52`). Nothing grants it to an existing account; scopes are stored per account and only an admin's edit adds one.
2. **`PUT /api/v1/integration/weights` (B1 to B3)**, `internal/api/handlers_weights.go`:
   - Route row `{"PUT", "/api/v1/integration/weights", AccessService, "reportWeights", auth.ScopeWeightsWrite}` (`internal/api/routes.go:90`), handler wired in `api.go`.
   - Order of D53: scope (the route guard), then the switch (`reportWeights`, `:74`; off is 409 `not_configured` before the body is read: "reported weights are off: an admin turns them on with the reported_weights setting; the report was not stored", `:41`), then the body, then ordering (`reported.SubmitWeights`).
   - Body: the shared `decodeJSON` reads the top level with `weights` as `json.RawMessage`, so only top-level problems (not JSON, bare `NaN`/`Infinity`, unknown top-level field, over 1 MB, wrong content type) get the shared answers. Missing or null `weights` is 400 `validation`, field `weights`; not a list is 400 field `weights`; more than 10,000 entries is 400 field `weights` (`submitWeights`, `:103`).
   - Each entry is decoded by `decodeWeightEntry` (`:182`) into `map[string]json.RawMessage` and checked by hand: not an object, unknown field, setup missing / not a string / null / empty, weight missing / not a JSON number (string, null, boolean, object, list), weight not representable as a float64 (`1e400`), attribute value not a string, attributes not an object. Then `reported.NewWeightChecker(limits).Check` does the value rules (repeated name, finite, inclusive min/max, 16 attributes, label key rule, 63 characters). The first invalid entry is reported as 400 `validation`, `field` `weights`, message from `reported.EntryMessage` (position from 0, name when a non-empty string).
   - The 200 response is `{observed_at, received_at, expires_at, accepted, unknown}` with `expires_at = observed_at + max_age` of the setting.
   - The last 400 refusal: `reportWeights` wraps the writer in `refusalCapture` (`:49`) and, after the inner handler, records `{At: clock now, ServiceAccount, Message}` with `a.reported.RecordWeightsRefusal` for any 400 from this route (`:99`), whichever check produced it. 409s and the off answer are not recorded.
   - Nothing is stored on any refusal (all checks run before `SubmitWeights`; its transaction is the only write). The handler never calls the collector, the ledger or the event dispatcher.
3. **`GET /api/v1/integration/state` (B4)**, `internal/api/handlers_integration.go:300-325` and helpers in `handlers_weights.go:281` onward: top-level `weights {enabled, min, max, observed_at, expires_at}` (times null with the feature off or before the first report; the report is not read with the feature off), and on every setup `weight {applied, source, reported, not_applied, attributes}` from `weights.Resolve` at the lab's time with the setup's override. Added fields only; no selector is returned.
4. **OpenAPI** (`api/openapi.yaml`): path `/api/v1/integration/weights` (`x-roles: service`, `x-csrf: false`, `serviceToken`, 200/400/401/403/409/429/500), `ServiceScope` enum gains `weights:write`, new schemas `WeightsReport`, `WeightReport` (`additionalProperties: false`), `WeightsReportResult`, `StateWeights`, `StateSetupWeight`; `IntegrationState.weights` and `StateSetup.weight` added and required (always present); the state route's description updated. `cd ui && npm run gen:api` regenerated `ui/src/api/schema.gen.ts`. No UI source needed a change (the scope checkbox list in `ui/src/pages/admin-service-accounts/api.ts` still lists four scopes: milestone 6).
5. `bazel run //:gazelle` added the two new Go files and the `//internal/weights` dep to `internal/api/BUILD.bazel`.

## Requirement -> tests (all in `internal/api/weights_report_test.go` unless noted), with the mutation that turned each red (`mutations.log`; each file restored byte for byte, WGT tests green after)

| Requirement | Tests | Mutation -> red |
| --- | --- | --- |
| WGT-7 | `TestWGT7_ReportReplacesTheWholeSet` (default `observed_at`, response shape, whole-set replace, empty list), `TestWGT7_ReadersNeverSeeAMixOfTwoReports` (concurrent PUTs and state reads), `TestWGT7_OrderingAndClockSkew` (older 409 `stale_report` and unchanged, equal replaces, 2 min + 1 ms 400 and unchanged, exactly 2 min accepted, bad timestamp 400), `TestWGT7_OrderingIndependentOfTheDeploymentsReport`, `TestWGT7_ReportSurvivesARestart` (store closed, a new backend opened on the same file), `TestWGT7_ReportNeverPollsDebitsOrEmits` (B3) | M7a handler passes nil `observed_at`; M7b store keeps old rows on replace; M7c the handler runs a poll |
| WGT-8 | `TestWGT8_EachInvalidEntryRefusesTheWholeReport` (22 subtests: weight missing, `"1.15"`, null, boolean, object, `1e400`, `-1e400`, below min, above max; name missing, not a string, null, empty, repeated; 17 attributes, bad key, 64-character value, non-string value, attributes not an object; unknown field with and without a name; entry not an object; each asserts 400, field `weights`, the exact position/name prefix, the reason, stored report deep-equal to before, and the previous report still applied in the state), `TestWGT8_FirstInvalidEntryReported`, `TestWGT8_LimitsInclusiveAndNegativeZero` (exactly min and max accepted; `-0` and `-0.0` stored and returned as 0), `TestWGT8_BodyRefusals` (bare `NaN`, `Infinity`, truncated body get "request body is not valid JSON" with no field; missing/null `weights` 400 field `weights`; non-list `weights`; unknown top-level field), `TestWGT8_EntryAndBodyLimits` (10,001 refused, 10,000 accepted, a body over 1 MB refused), `TestWGT8_LastRefusalKeptForAdmins` (B2: time, account, message; 200, 409 stale and 409 off keep it; a not-JSON 400 replaces it) | M8a unknown entry field accepted; M8b string weight passed to the number decoder; M8c name left out of the message; M8d field not `weights`; M8e refusal not recorded; M8f null `weights` accepted; M8g entry limit not checked |
| WGT-9 | `TestWGT9_UnknownNameIgnoredAndListed` (listed in order, stored, other setups' weight objects unchanged, never in the state) | M9 unknown names counted as accepted |
| WGT-10 | `TestWGT10_OffAnswersNotConfiguredWhateverTheBody` (valid, invalid entry, not JSON, no `weights`, empty body: 409 naming `reported_weights` and an admin, nothing stored; with a stored report, off stores nothing new) | M10 switch not checked |
| WGT-11 | `TestWGT11_RouteNeedsWeightsWriteAndTheScopeOpensNothingElse` (every permissions-fixture principal: only the all-scopes account gets 200, the every-other-scope account 403 `forbidden`, every user kind and no credentials 401; an account with only `weights:write` refused on every non-public route: 403 on the other service routes, 401 elsewhere), `TestWGT11_ScopeAddedLastAndNotGrantedToExistingAccounts`; and the existing `TestSVC3_EachIntegrationRouteRequiresItsScope`, `TestSVC8_PermissionsCoverEveryNewRouteForEveryPrincipal`, `TestSVC8_DesignTableDecidesEveryNewRouteForEveryPrincipal`, `TestPermissions_EveryRouteEveryActor`, `TestSVC2_...` now run the route (probe and table rows added) | M11 route row under `state:read` |
| WGT-13 | `TestWGT13_StateWeightFields` (before the first report; setups priced by override, reported, rule, default; `not_applied` `override` with attributes kept, `out_of_bounds` beating `override`, `out_of_bounds` on a disabled setup, `expired` beating both, exactly `max_age` still usable; top-level object exactly; feature off: every setup `reported`/`not_applied` null, attributes `{}`, times null, `max` null, override still applies; no `tier=gold`, `selector` or `matched_rule` in any body); `TestINT8_*` walks the new keys (allow-list) | M13a applied = base weight; M13b `not_applied` never set; M13c report times filled with no report |
| WGT-21 | `TestWGT21_StoredWeightFollowsTheNameAndTheSwitch` (setup created later under a stored name; free setup renamed onto a stored name through the admin PUT; the old name's weight leaves the renamed setup; override set in the store survives a rename onto a reported name, `not_applied` `override`; switch off keeps the stored report, on again within `max_age` applies; expired, then `max_age` 30m revives it with the new `expires_at`) | M21a override not passed to the state resolver; M21b `max_age` fixed at 10m in the state |
| Design Tests item: tightening the maximum | `TestWGT12_TighteningTheMaximumOverHTTP` (a metered hold charged at the reported 15; maximum lowered to 10: the state shows `rule`/`out_of_bounds` at once, the next report containing 15 is refused whole and stores nothing, the next poll succeeds and samples weight 2 source `rule`, and again after `max_age`). The admin-view half (excluded count and refusal on the collector view) is milestone 5 (WGT-22). | M12 the route does not enforce the maximum |
| Contract test | `TestUI2_ContractResponsesMatchSchemas` (`internal/api/contract_test.go`): the route validated on a real 409 off, 200 (with attributes and an unknown name), 400 and 409 stale; the state 200 validated afterwards with a reported setup | (coverage check fails without the 200: seen before adding it) |

## Existing assertions changed, and why

All are the additive requirement (a fifth scope, a new route, added state fields), none weakens a check:

1. `internal/api/router_test.go` `TestAPI_RouteTableIsConsistent`: route count 43 -> 44 (one new operation).
2. `internal/api/permissions_test.go`: `probeOps` gains `reportWeights`; `probe` gains its request (`{"weights":[]}`, scope `weights:write`); `permExtraCover` gains its row; `newPermFix` turns `reported_weights` on (min 0, max 20, 10m) so the scoped account's probe reaches a 200 (no other route reads the setting in this milestone).
3. `internal/api/service_accounts_test.go`: `svcDesignRoutes` gains the route row (service, `weights:write`); the comment "The four routes use the four scopes" now reads "The integration routes use the scopes, one each" (the assertion was already generic over `Scopes()`).
4. `internal/api/integration_reports_test.go`: the INT-8 allow-list `stateKeys` gains `weights` at the top level, `weight` on `setups[]`, and the two new objects' keys; `walkStateKeys` skips `attributes` as it skips the free-form `labels` and `details`. The banned-word check on the state body is unchanged and passes.
5. `internal/api/contract_test.go`: added the weights calls (and the setting turned on through the store) to the integration block; nothing removed.
6. `internal/api/testenv_test.go`: `backendOpts.dbPath` (open an existing file, for the restart test) and `backend.dbPath`. Harness only.

No test that pins exactly four scopes existed in `internal/` (checked: `TestSVC3_` compares with `len(allScopes())`); `ui/src/pages/admin-service-accounts/api.ts` and its test pin four and were left for milestone 6.

## Decisions for docs/decisions.md

1. The 409 `not_configured` message is "reported weights are off: an admin turns them on with the reported_weights setting; the report was not stored", and the switch is checked before the body is read, so an empty or non-JSON body also gets 409.
2. Every 400 from the weights route is recorded as the last refusal (an entry refusal, a body that is not JSON, missing `weights`, too many entries, over 1 MB, a bad or future `observed_at`), because B2 says "the most recent report refused with 400". The 409s (off, stale) are not. The time is the server clock at refusal; recording is done by capturing the handler's own 400 answer, so the stored message is exactly the one returned.
3. Entry reasons added by the handler (the value reasons are D82's): `must be an object with setup, weight and optional attributes`; `unknown field "x" (an entry holds only setup, weight and attributes)` (the first unknown key in sorted order); `setup must be a non-empty string` also for a missing, null or non-string name; `weight is required`; `weight must be a JSON number, not a string|null|a boolean|an object|a list`; `weight 1e400 is not a finite 64-bit number`; `attributes must be an object of strings`; `attributes "k" must be a string`. Check order inside one entry: object shape, unknown field, name, weight type, attribute types, then `WeightChecker` (repeat, finite, limits, attribute count, key rule, value length).
4. A null `attributes` is treated as absent (as Go's decoder treats a null map in the deployments report); the OpenAPI schema does not mark it nullable.
5. A non-list `weights` is 400 `weights must be a list`, field `weights`; more than 10,000 entries is checked in the handler before any entry, with the D82 message and field `weights`.
6. `accepted` counts entries whose name matches a setup (disabled setups included), as the deployments report does; `unknown` lists the others in report order.
7. On the state route, a rate rule that cannot be evaluated fails the request with 500, as it fails the poll (A1 names the poll, the user views and the admin views; the state route reports what a poll would charge, so it follows the poll). Rules are validated when saved, so this appears unreachable in practice. The stored weights report is read only when the setting is on.
8. `StateSetup.weight` and `IntegrationState.weights` are listed as required in the OpenAPI schemas because they are always present; `StateSetupWeight.source` is the four resolver sources (never `policy` here).
9. The permissions fixture (`newPermFix`) and the contract test turn the setting on through the store (`PutSetting`), not the admin route, to keep the fixtures' other calls unchanged.

## Files changed

```
api/openapi.yaml
ui/src/api/schema.gen.ts
internal/auth/service.go
internal/api/BUILD.bazel
internal/api/api.go
internal/api/routes.go
internal/api/handlers_integration.go
internal/api/handlers_service.go
internal/api/handlers_weights.go
internal/api/contract_test.go
internal/api/integration_reports_test.go
internal/api/permissions_test.go
internal/api/router_test.go
internal/api/service_accounts_test.go
internal/api/testenv_test.go
internal/api/weights_report_test.go
```
`internal/api/handlers_service.go` changed only a comment ("one of the four scopes" -> "one of auth.Scopes()"). Task folder: `.orchestrator/reported-weights/tasks/05-m4-route-state/` (PROGRESS.md, RESULT.md, ci.log, ci.exit, gotest.log, race.log, ui.log, genapi.log, gazelle.log, mutations.log).

## Commands run

- `go test ./internal/api/ ./internal/auth/` while iterating; `go test ./internal/api -run WGT -count=1`: pass.
- `go test ./...`: exit 0 (`gotest.log`).
- `go test -race -count=1 ./internal/api/ -run 'WGT|INT8|SVC|Permissions|Contract|UI2'`: exit 0 (`race.log`).
- Mutation script (19 mutations, scratchpad `mutate.py`): every one RED; M7a needed a second form to compile. Files restored and compared (`cmp`); WGT tests exit 0 after.
- `cd ui && npm run gen:api` (twice: the second after a YAML quoting fix), `npm run typecheck && npx vitest run`: exit 0, 218 tests.
- `bazel run //:gazelle`: exit 0. `tools/check_core_generic.sh`: exit 0.
- `./ci.sh`: first run exit 3 (`//ui:api_schema_drift_test`: the schema was generated before the YAML fix); after regenerating, exit 0.

## Confidence and gaps

- The restart test reopens the same SQLite file in a new backend (new `reported.Service`, so the in-memory refusal is gone); it does not start a separate process. No mutation was found that breaks persistence alone without breaking every test.
- "Atomic replace" over HTTP is checked by concurrent writes and state reads (no read saw two weights); the store-level atomicity tests are milestone 2's.
- The admin collector view of the last refusal and of the excluded count is milestone 5 (WGT-22); here the refusal is checked on the `reported.Service` the route writes to (`TestWGT8_LastRefusalKeptForAdmins` builds an `API` value directly to reach it).
- Other readers (setups list, admin views, holds, burn) still use the rate rules directly (milestone 5), so with the feature on they can differ from the state route until then.
- `docs/integrating.md` still says scopes are four and documents no weights route: milestone 7.
