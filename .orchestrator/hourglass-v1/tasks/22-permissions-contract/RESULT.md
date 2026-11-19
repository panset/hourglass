# RESULT 22-permissions-contract

Status: done. `go test -race -count=1 ./internal/api/... ./internal/holds/...` passes, `bazel test //internal/api/...` passes, `./ci.sh` exits 0 (35 of 35 Bazel tests, including `//ui:api_schema_drift_test`, `//ui:test`, `//ui:typecheck`).

## (a) Permissions (`internal/api/permissions_test.go`)
Actors (8): Anonymous, Unassigned (pending), Rejected, Roles_removed, Developer, Admin (OIDC), Local_admin, Local_admin_before_password_change. Real router, store, ledger, holds and collector; only the extensions and the clock are fakes.

Matrix (`TestPermissions/<spec row>/<actor>`). Cells are the spec's Yes/No; the test reads docs/SPEC.md and fails if a cell or row differs. Rejected and Roles_removed behave as Unassigned. Anonymous gets 401 `unauthenticated` on everything but public routes. A deny for a signed-in user is 403 `no_role` (no role), `forbidden` (developer on admin), `password_change_required` (local admin before the change), body exactly `{"error":{"code","message"}}`.

| Spec capability row | Unassigned | Developer | Admin | Route(s) behind it |
| --- | --- | --- | --- | --- |
| Sign in | Yes | Yes | Yes | `GET /auth/oidc/start`, `GET /auth/callback`, `POST /auth/local`, `POST /auth/logout`, `GET /api/v1/session`, `GET /healthz`, `GET /api/v1/auth/methods` |
| See the "not authorized" page only | Yes | (blank) | (blank) | `GET /api/v1/session`: empty `roles` list and only the caller's own identity for Unassigned; roles present otherwise |
| See own balance, current holds, history | No (403 `no_role`) | Yes | Yes | `GET /api/v1/me`, `/me/holds`, `/me/ledger`, `/me/usage` |
| See all setups, holders and holders' balances | No | Yes | Yes | `GET /api/v1/setups`, `GET /api/v1/pressure` |
| Acquire and release a setup for self | No | Yes | Yes | `POST /api/v1/setups/{id}/acquire`, `POST /api/v1/holds/{id}/release` (own hold) |
| See pending access requests; approve or reject | No | No (403 `forbidden`) | Yes | `GET /api/v1/admin/users`, `PUT /api/v1/admin/users/{id}/roles`, `POST /api/v1/admin/users/{id}/reject` |
| Assign and remove roles | No | No | Yes | `PUT /api/v1/admin/users/{id}/roles` |
| Grant tokens to a user | No | No | Yes | `POST /api/v1/admin/users/{id}/grants`, `POST /api/v1/admin/users/{id}/reset` |
| Force-release another user's setup | No | No (403 `forbidden`, from the holds rule) | Yes | `POST /api/v1/holds/{id}/release` with `{"force":true,"reason"}` |
| Add, edit, delete setups | No | No | Yes | `POST /api/v1/admin/setups`, `PUT`/`DELETE /api/v1/admin/setups/{id}` |
| Edit metering settings | No | No | Yes | `GET`/`PUT /api/v1/admin/settings`, `GET`/`PUT /api/v1/admin/rules`, `GET /api/v1/admin/collector`, `GET /api/v1/admin/audit` (the last two have no spec row; filed here) |
| See team usage, move tokens between team members | No | No | No | none in v1 (Manager only): the test asserts no route has `team` in its path |

Local admin (not a spec column): same as Admin, except it cannot acquire (403 `forbidden`, AUTH-8) and has no hold to release; before the password change every data route is 403 `password_change_required`, only `GET /api/v1/session`, `POST /auth/logout` and `POST /auth/local/password` work. `POST /auth/local/password` is the local admin's alone (others 403 `forbidden`); it is covered by `TestPermissions_LocalAdmin` and the exhaustive test.

Other tests in the file:
- `TestPermissions_EveryRouteEveryActor`: all 30 routes x 8 actors = **240 (route, actor) combinations**, status class equals the access level; for anonymous and the three role-less actors, no seeded marker (setup name, label, details, user name and email, audit reason, ledger reason) in any body or header, and every non-exempt route answers 401/403 with exactly the error object. The test asserts it ran 240.
- `TestAUTH3_EveryStateChangingRouteRejectsMissingOrWrongCSRF`: 13 state-changing routes x 7 signed-in actors x 4 token states (missing, wrong, empty, another session's) = 364 requests; the answer is `csrf`, or the actor's earlier refusal; nothing was created.
- `TestPermissions_LocalAdmin` (AUTH-8): the only user routes are list, roles, reject, grants, reset (no route can disable, rename or delete); roles `[]`/developer/both, reject, grants and reset on the local admin are 409 `local_admin_protected` by an OIDC admin and by itself; it cannot acquire, holds nothing, has no ledger rows; `/me` shows `balance_tokens: null`, `can_acquire: false`, reason `local_admin`; user row unchanged.
- `TestPermissions_AdminCannotRemoveOwnAdminRole` (409 `self_demote`), `TestPermissions_ManagerIsNotAssignable` (400 `validation`).
- `TestHOLD2_OwnRoutesCannotTouchAnotherUsersHold`: developer A releasing B's hold, with or without `force`, is 403; plain admin release of another's hold is 403; unknown hold 404; the hold stays active; B's hold is not in A's list; an admin force-release works.
- `TestAUTH4_UnknownAPIPathsAreJSON404WithoutData`: 14 unknown or wrong-method paths under `/api/` and `/auth/` x 8 actors: JSON 404 `not_found`, `no-store`, no data.
- `TestAUTH5_SessionOfUserWithNoRoleIsOwnIdentityOnly`.
- `TestPermissions_MatrixCoversTheSpecAndEveryRoute` (meta): every capability row of the spec table is in the matrix with the same cells, every `Routes()` entry is in a row (or `permExtraCover`) and has a probe. I verified the suite bites: replacing `RoleRoute` by session-only in `guard` made over 160 checks fail (restored).

## (b) Contract (`internal/api/contract_test.go`)
- kin-openapi v0.132.0 (MIT) is a test dependency (go.mod, go.sum, `use_repo`, `internal/api/BUILD.bazel`; the YAML and `docs/SPEC.md` are `data` of `api_test`, found by walking up from the working directory so it works under `go test` and `bazel test`). v0.133.0 pulls `github.com/woodsbury/decimal128` (0BSD), which `tools/check_licenses.sh` refuses, hence the pin.
- `TestContract_RoutesMatch`: (method, path, operationId, x-roles, x-csrf) of the YAML equal `Routes()`. `TestContract_CSRFDeclarationMatchesBehavior`: x-csrf is also checked against the running router (cookie, no token).
- `TestContract_ResponsesMatchSchemas`: a world with data in every part (holds, debits at several pressure levels, a failed poll, an outage, an orphan, failed delivery, grants, reset, roles, reject, forced release, audit rows, a fake OIDC discovery server) is exercised through the real handlers. Every response is validated against its operation (status must be documented; schemas closed unless they declare `additionalProperties`; bodies, headers, content type); request bodies and parameters the test marks valid are validated too; every error response is also checked against `Error`. Every operation has a validated success and a validated error except `getAuthMethods` (documents only 500, which it cannot produce; listed in `errorNotProvokable`).
- Timestamps: the `date-time` format is redefined to the one format (`2006-01-02T15:04:05.000Z`) and counts what it checks (more than 50 values per run); `TestContract_EveryTimestampIsRFC3339UTCWithMilliseconds` also scans raw bodies including audit `before`/`after`.
- Bites shown (then restored): renaming `burn_per_hour_tokens` in `meJSON` fails `getMe` with "property burn_per_hour_token is unsupported"; removing `/api/v1/pressure` from the YAML fails four tests; making `ts` emit `RFC3339` without milliseconds fails every operation with a time field.

### Contract mismatches found and fixed
1. `GET /auth/callback` documented 400 and 500 JSON errors; the handler (internal/auth) only redirects (`302 /login?error=`) and answers JSON 404 when OIDC is off. YAML now documents 302 and 404 only (handler untouched; `Set-Cookie` described as set only on success).
2. `POST /auth/local/password` can answer 429 `rate_limited` (shared failure counter with sign-in) but did not document it: added.
3. Audit `before` of a forced release held `acquired_at`, `metering_started_at`, `deployed_at` with Go's nanosecond RFC 3339 (found by the raw timestamp scan, not by a schema: `before`/`after` are open objects). `internal/holds/holds.go` now writes them with `store.FormatTime`.
4. Two timestamp formats (user routes nanosecond-trimmed, admin routes millisecond): one helper `ts`/`tsPtr` in `internal/api/helpers.go` now formats every API timestamp; `utcTime`/`utcTimePtr` are gone, the user views hold strings; the OpenAPI "Conventions" document the format; `user_test.go` expectations changed from `...:00Z` to `...:00.000Z`.
5. `router_test.go` `TestHealthz_ReportsDatabaseAndPollAge` compared a fake-clock handler with `time.Now()` and failed under `-race` load (age 88 instead of 89 to 95); it now uses the fake clock. (Not a contract mismatch; a flake I hit.)
No schema field was found missing from or extra in the YAML beyond these, and 21d's handlers needed no other change.

## Additional item 1: `can_acquire`
`holds.AcquireBlock(localAdmin, enforcement, balanceMT, activeHolds, maxHolds)` returns `""`, `local_admin`, `insufficient_tokens` or `hold_limit` (that order); `holds.Service.Acquire` uses it for the token and limit checks, `GET /api/v1/me` for `can_acquire` and `cannot_acquire_reason` (local admin: false, `local_admin`). OpenAPI `Me` has both (required, reason nullable enum). Tests: `TestHOLD1_AcquireBlockRule` (holds), `TestHOLD1_MeCanAcquireFollowsTheAcquireRule` (api; also checks acquire answers the same code), `TestLED2_MeShape`. `ui/src/api/schema.gen.ts` regenerated; `ui/src/pages/setups/index.tsx` `acquireBlock` now reads `can_acquire` and `cannot_acquire_reason`; the `usage` page derived nothing and is unchanged except its test fixture. UI tests: setups 15, usage 10 pass; `tsc` clean.

## Additional item 2: one timestamp format
See mismatch 4 above; the helper is `ts`/`tsPtr`, the check is the strict `date-time` validator in the contract test.

## Files changed
- Tests (new): `internal/api/permissions_test.go`, `internal/api/contract_test.go`, `internal/holds/block_test.go`.
- Source: `internal/api/{helpers,admin_common,user_views,handlers_user}.go`, `internal/holds/holds.go`, `api/openapi.yaml`, `ui/src/api/schema.gen.ts`, `ui/src/pages/setups/index.tsx`.
- Tests (edited): `internal/api/{user_test,router_test}.go`, `ui/src/pages/{setups/setups.test.tsx,usage/usage.test.tsx}`.
- Build: `go.mod`, `go.sum`, `MODULE.bazel` (only the `use_repo(go_deps, ...)` line `com_github_getkin_kin_openapi`), `internal/api/BUILD.bazel`, `internal/holds/BUILD.bazel` (gazelle).
- Outside the brief's list, both needed for `./ci.sh`: `BUILD.bazel` (root; `exports_files` now also lists `docs/SPEC.md`, so the test can read the spec as Bazel `data`) and `docs/third-party-licenses.md` (9 rows added by `tools/check_licenses.sh --write`). `MODULE.bazel.lock` did not change.

## Commands
`go test -race -count=1 ./internal/api/... ./internal/holds/...` ok (api 88 s); `bazel test //internal/api/... //internal/holds/...` ok; `./ci.sh` exit 0; `npx vitest run src/pages/setups src/pages/usage` 25 pass.

## Decisions for docs/decisions.md
- All API timestamps: RFC 3339 UTC, milliseconds, `Z`; the audit payload snapshots follow it.
- kin-openapi pinned to v0.132.0 because v0.133.0 needs a 0BSD module the license policy rejects.
- `GET /auth/callback` is documented as redirect-only (302, plus 404 when OIDC is off).
- Capability rows with no spec route are filed under "Edit metering settings" (`GET /admin/collector`, `GET /admin/audit`).
- Pre-change local admin: all data routes 403 `password_change_required` (AUTH-7), pinned by the matrix.

## Confidence and gaps
- Verified by the commands above; the bites were run by hand once and restored (diffed against backups).
- Not exercised: a real OIDC callback with a token exchange (the 302 is the failure redirect; the success redirect is covered in `internal/auth`); the `/healthz` 503 comes from a closed store. `getAuthMethods` has no provoked error.
- Schemas count as closed only where they have `properties` and no `additionalProperties`; `allOf` branches with several object siblings are not closed (none exist now).
- Each fixture costs about 0.9 s under `-race` (argon2 for the local admin), so the new suites add roughly 30 s to the race run; probes create their own targets to avoid more fixtures.
- Observed, not changed: "pending events dropped at shutdown" warnings in several tests.
