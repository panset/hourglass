# Result: 82 pre-approved SSO users, server half (D1 to D4, PRE-1 to PRE-8)

Status: done. Every "Done when" item holds (see "Commands run").

## What was built

- **Migration `0007_preapprovals.sql`** (`internal/store/migrations/0007_preapprovals.sql`): the `preapprovals` table exactly as D2 gives it. Per D70(a), `0006` is untouched.
- **Store queries** (`internal/store/preapprovals.go`): `ListPreapprovals` (by email), `GetPreapprovalByEmail`, `UpsertPreapproval` (replaces roles only; keeps id, creator, creation time and use record; reports whether it added), `DeletePreapproval` (returns the removed row), `MarkPreapprovalUsed` (only when `used_at IS NULL`). The store keeps what it is given; callers normalise.
- **Sign-in step, auth half** (`internal/auth/auth.go`, `internal/auth/oidc.go`): `auth.Preapprover` interface and `auth.WithPreapprover`. `internal/auth` imports neither the ledger service nor the API. After `UpsertOIDCLogin`, `applyPreapproval` calls the step only for a pending SSO user (status `pending`, auth source `oidc`), a non-empty email claim (after trimming) and an `email_verified` claim that is not false. Only `claims.Email` is passed; no other claim is read for matching. A step error is logged (`applying a pre-approval failed; the user stays pending`, with `user_id`, never the address) and the sign-in continues. `user.pending` is emitted for a new user unless the step approved them.
- **Sign-in step, transaction** (`internal/api/preapproval.go`, `Preapprover.ApplyPreapproval`): in one `Store.Tx` it re-reads the user and returns without change unless the user is an SSO user (`auth_source = oidc`, not the local admin) with status `pending`; looks the normalised email up in the database list and in the file overlay; sets the union of roles through `grantRolesTx`; marks an unused database entry as used (`used_at`, `used_by`); writes `user.preapprove` with no actor. `user.approved` is emitted after commit with the payload `setUserRoles` uses.
- **Shared helper** `grantRolesTx` (`internal/api/preapproval.go`): set roles, set status `active` when not active (and build the `user.approved` event), `Ledger.EnsureFirstAllowanceTx`. `setUserRoles` (`internal/api/handlers_admin.go`) now calls it instead of its inline copy, so the two cannot drift. `putAudit` gained a sibling `putAuditAs` with an optional actor (`internal/api/admin_common.go`).
- **Admin routes** (`internal/api/preapproval.go`, `internal/api/routes.go`, `internal/api/api.go`): `listPreapprovals`, `addPreapprovals`, `deletePreapproval`, all `AccessAdmin` (admin role, password guard, CSRF on POST and DELETE). OpenAPI paths and schemas in `api/openapi.yaml`; `ui/src/api/schema.gen.ts` regenerated with `cd ui && npm run gen:api` (199 lines added, `npm run typecheck` clean). No UI fixture needed a change.
- **Validators** shared by the route and the file: `api.NormalizeEmail` (trim, lower case), `api.ParsePreapprovalEmail` (D3 rules), `api.ParsePreapprovalRoles` (`normaliseRoles` plus non-empty).
- **File overlay**: config key `preapproved_users_file`, env `HOURGLASS_PREAPPROVED_USERS_FILE`, shown as a path in `Config.Redacted()` (`internal/config/config.go`). Loader `app.LoadPreapprovedUsers` (`internal/app/preapproved.go`): `.json` (any case) is JSON, anything else YAML converted to JSON, decoded with unknown fields refused; at most 10,000 entries; same email and role rules as the route; roles default to `[developer]`; a repeated email (after normalising) is an error. Error type `PreapprovedFileError`, message `preapproved_users_file: <path>: users[i]: <reason>`. `app.Run` reads it once, before the database is opened (an invalid file leaves no database), and logs `pre-approved users file read` with `file` and `entries` only. The file is only read (`os.ReadFile`), never written, never copied to the database.
- **Wiring** (`internal/app/app.go`): `api.NewPreapprover(...)` built after the ledger service; passed to `auth.New` with `auth.WithPreapprover` and to `api.Deps.Preapprovals`.
- `deploy/config.sample.yaml`: a commented example of the key and the file format.
- Gazelle updated `internal/{api,app,auth,store}/BUILD.bazel`.

## Routes, in words, for the UI task (D5, UI-11)

All three need the `admin` role (an OIDC admin or the local admin after its first-run password change); POST and DELETE need the `X-CSRF-Token` header. Anyone else gets 401 `unauthenticated` (no session) or 403 (`no_role`, `forbidden`, `password_change_required`, `csrf`).

**`GET /api/v1/admin/preapprovals`** returns `200` with an object holding one field, `preapprovals`: an array sorted by email, and for an email in both sources the database entry comes before the file entry. Each element has:
- `id`: integer for a database entry, `null` for a file entry (so a file row has no Remove button);
- `email`: trimmed, lower case;
- `roles`: non-empty array of `developer` / `admin`, sorted;
- `source`: `"database"` (UI: "Added here") or `"config"` (UI: "Config file");
- `created_at`: timestamp string (RFC 3339 UTC, milliseconds, `Z`) for a database entry, `null` for a file entry;
- `created_by`: `{ "id": <user id>, "name": <display name> }` of the admin who added it ("unknown user" if the id no longer resolves), `null` for a file entry;
- `used_at`: timestamp of the first sign-in it approved, `null` until used and always `null` for a file entry (UI: date or "Not yet");
- `used_by`: `{ "id", "name" }` of the user that first sign-in approved, `null` until used and always `null` for a file entry.

**`POST /api/v1/admin/preapprovals`** takes `{ "emails": [string, ...], "roles": [ "developer" | "admin", ... ] }`; both fields are required, unknown fields are refused. `emails` holds 1 to 1,000 strings as sent (the count is checked before de-duplication). Each email is trimmed and lower-cased; an email that appears more than once in one request counts once. An email already in the database list gets the new roles and keeps its id, creator, creation time and use record. The answer is `200` with `{ "added": <int>, "updated": <int> }`, counted over distinct emails. Any problem is `400` with `{"error":{"code":"validation","message":...,"field":...}}` and nothing is stored: `field` is `"emails"` for a missing/empty/over-long list or the first invalid email (the message is `invalid email "<value>": <reason>`, the value shortened to 80 characters), and `"roles"` for missing, empty, unknown or duplicate roles. Emails are checked before roles. The UI shows `error.message` next to the text area and keeps the text.

**`DELETE /api/v1/admin/preapprovals/{id}`** removes one database entry: `204` with no body, `404 not_found` for an unknown id, `400 validation` (field `id`) for a non-positive or non-numeric id. It changes no user.

Audit actions: `preapproval.add` (target `preapprovals`, `after` = `{"emails": [sorted, normalised, distinct], "roles": [...]}`), `preapproval.delete` (target `preapproval:<id>`, `before` = `{"email", "roles"}`), `user.preapprove` (target `user:<id>`, no actor, shown as `system`; `before` = `{"roles": [...], "status": "pending"}`, `after` = `{"roles": [...], "status": "active", "sources": ["database"] | ["config"] | ["database","config"]}`).

## Requirement -> test -> file

| Requirement | Tests | File |
| --- | --- | --- |
| PRE-1 | `TestPRE1_NewUserIsActiveWithRolesAndAllowanceAtFirstSignIn` (active, roles, 300-token allowance, first session shows roles, `GET /me` works); `TestPRE1_PendingUserFromAnEarlierSignInIsApprovedAtTheNextSignIn` (and adding an entry approves nobody at once) | `internal/api/preapproval_signin_test.go` |
| PRE-2 | `TestPRE2_MatchIgnoresCaseAndSurroundingWhitespace` (both sides, database and file); `TestPRE2_NoMatchWithoutAVerifiedNonEmptyEmailClaim` (`email_verified` false / "false" / true / "true" / absent, empty email, no email claim with the address in `preferred_username`, `upn` and `name`, other address) | `internal/api/preapproval_signin_test.go` |
| PRE-2 | `TestPRE2_EmailVerifiedClaimGatesTheStep` (absent, null, true, "true", " TRUE ", false, "false", "False", "yes", 1); `TestPRE2_OnlyANonEmptyEmailClaimIsUsed` (empty, blank, absent with the address in other claims, claim passed as sent); `TestPRE2_EmailVerifiedParsing` | `internal/auth/preapproval_test.go` |
| PRE-3 | `TestPRE3_RejectedActiveAndLocalAdminAreNeverChanged` (rejected user with a listed email, active user whose roles an admin removed, local admin called directly with a listed email, deleting every database entry changes no user, a step without the file entry changes no approved user) | `internal/api/preapproval_signin_test.go` |
| PRE-3 | `TestPRE3_OnlyAPendingSSOUserReachesTheStep` (active with no roles, rejected, pending from an earlier sign-in reaches it again, local sign-in never reaches it) | `internal/auth/preapproval_test.go` |
| PRE-4 | `TestPRE4_AuditEventsAndFirstUse` (one `user.preapprove`, no actor, exact before/after JSON, "system" in the audit list, `user.approved` only with its payload, first use recorded; a second SSO identity with the same email approved and audited, first use kept; a later sign-in writes nothing); `TestPRE4_NoEntriesNoFileSignInAsBefore` (pending, only `user.pending`, no audit) | `internal/api/preapproval_signin_test.go` |
| PRE-4 | `TestPRE4_ApprovedNewUserGetsNoPendingEvent` | `internal/auth/preapproval_test.go` |
| PRE-5 | `TestPRE5_ListShowsBothSourcesByEmail`; `TestPRE5_AddCountsUpsertKeepsUseAndIsAudited` (duplicates in a request count once); `TestPRE5_AddAcceptsOneThousandRefusesOneThousandAndOne` (and zero); `TestPRE5_AddValidatesEveryEmailAndRoleAllOrNothing` (254 accepted, 255 refused, empty, blank, no `@`, two `@`, nothing before or after `@`, space, tab, comma, `*`, wildcard domain, each among valid emails stores nothing and names the first bad value; unknown, empty, duplicate, missing roles; missing or null emails; unknown field); `TestPRE5_DeleteRemovesOneDatabaseEntry`; `TestPRE5_RoutesRefusedForEveryPrincipalButAnAdmin` (anonymous, pending, rejected, roles removed, developer, local admin before password change, service account with and without scopes refused and nothing stored; admin and local admin allowed; CSRF required) | `internal/api/preapproval_routes_test.go` |
| PRE-5 | `TestPermissions_EveryRouteEveryActor`, `TestPermissions_MatrixCoversTheSpecAndEveryRoute` (three new probes, `permExtraCover` entries, marker `MKR-preapproval`); `TestContract_CSRFDeclarationMatchesBehavior`, `TestUI2_ContractResponsesMatchSchemas` (success and error responses of all three validated against the schemas, including a used entry and file entries); `TestAPI_RouteTableIsConsistent` (43 routes), `TestAUTH4_AdminSessionReachesEveryRealHandler` | `internal/api/permissions_test.go`, `contract_test.go`, `router_test.go` |
| PRE-5 | `TestPRE5_PreapprovalQueries` | `internal/store/preapprovals_test.go` |
| PRE-6 | `TestPRE6_FileLoaderReadsYAMLAndJSON`; `TestPRE6_InvalidFileIsAnErrorNamingTheEntry` (missing file, bad YAML, bad JSON, YAML in a `.json` file, unknown top and entry keys, not a list, missing email, each email rule, unknown/empty/duplicate role, repeated email in another case, 10,001 entries; 10,000 accepted; no valid address in any message); `TestPRE6_InvalidFileStopsStartup` (no database created; env variable); `TestPRE6_ValidFileIsAReadOnlyOverlay` (count logged, no address logged, path in the redacted config, list through the API shows `config` entries, file bytes, mode and mtime unchanged, nothing in the table); `TestPRE6_NoFileStartsAsBefore` | `internal/app/preapproved_test.go` |
| PRE-6 | `TestPRE6_PreapprovedUsersFileKey` | `internal/config/config_test.go` |
| PRE-6 | `TestPRE6_NoStepSignInAsBefore` | `internal/auth/preapproval_test.go` |
| PRE-7 | `TestPRE7_UnionOfBothSources` (database developer + file admin = both, sources `["database","config"]`, the granted admin reaches an admin route; a file entry alone, sources `["config"]`) | `internal/api/preapproval_signin_test.go` |
| PRE-8 | `TestPRE8_FailedApprovalTransactionLeavesTheUserPending` (a trigger makes the step's last write fail: sign-in succeeds, user pending with no role, no allowance, entry not marked used, error logged, `user.pending` emitted; next sign-in approves once the fault is gone) | `internal/api/preapproval_signin_test.go` |
| PRE-8 | `TestPRE8_FailedStepStillSignsInAndLeavesUserPending` | `internal/auth/preapproval_test.go` |

Existing tests extended, not loosened: schema version pins 6 -> 7 (`internal/store/store_test.go`, `integration_queries_test.go`, `ledger_compaction_test.go`, `migrate0004_test.go`), `preapprovals` added to `TestSchemaColumnsMatchSpec` (the table count check is unchanged and now expects one more table), route count 40 -> 43, router seed adds pre-approval 1. The test backend (`internal/api/testenv_test.go`) now builds the ledger before the auth service so every backend carries the real pre-approval step; options `preFile` and `emitter` were added. The auth fake issuer (`internal/auth/oidc_test.go`) gained a `claims` override knob.

## Decisions for docs/decisions.md

1. The approval step is implemented in `internal/api` (`Preapprover`), which already holds the store, the ledger service, the audit helper and `setUserRoles`; it shares `grantRolesTx` with `setUserRoles`. `internal/auth` defines the `Preapprover` interface and receives it through `auth.WithPreapprover`; `internal/app` builds it and hands it to both auth and the API. No new package.
2. `email_verified`: absent or JSON null is verified (D1); `true` and the string `"true"` (trimmed, any case) are verified; `false`, `"false"` and any other value (another string, a number, an object) are not. The conservative reading of "not false" for values the design does not name.
3. Matching compares `strings.ToLower(strings.TrimSpace(...))` of the claim with the stored, already normalised email. The callback skips the step unless the trimmed claim is non-empty; the claim itself is not validated against the email rules.
4. A new SSO user whose approval step fails gets `user.pending`, since they are pending.
5. The step runs one write transaction for every sign-in of a pending SSO user, even with no entries and no file; with nothing to match it writes nothing, so the sign-in's observable behaviour is unchanged.
6. `POST` answers `200` with `{added, updated}` counted over distinct emails. An email repeated in one request (after normalising) counts once and is not an error. The 1 to 1,000 limit counts the array as sent.
7. An update of an existing email replaces only its roles; id, `created_by`, `created_at`, `used_at` and `used_by` stay.
8. Email length is counted in Unicode characters after trimming. Emails are checked before roles; the 400 names the first bad email (`field` = `emails`, value shortened to 80 characters in the message) or the role problem (`field` = `roles`).
9. `created_by` and `used_by` in the list are objects `{id, name}` (name by the shared display-name function, "unknown user" when the id no longer resolves), or null.
10. The list is sorted by email; for an email in both sources, the database entry comes first.
11. Audit: `user.preapprove` targets `user:<id>`, `before` `{roles, status}`, `after` `{roles, status, sources}` with sources in the order database, config. `preapproval.add` targets `preapprovals` with `after` `{emails, roles}` (emails sorted, normalised, distinct). `preapproval.delete` targets `preapproval:<id>` with `before` `{email, roles}` and `after` null.
12. `user.approved` for a pre-approved sign-in carries the union of roles, in the payload shape `setUserRoles` uses.
13. File: read in `app.Run` before the database is opened, so an invalid file leaves no database, like a bad TLS file (D64). An error names the entry by index (`users[i]`) and the reason; it never repeats a valid address (an invalid value is quoted). An empty file, or one with `users: []`, is a file with no entries. `.json` is matched case-insensitively, as for the seed. The log line is `pre-approved users file read` with `file` and `entries`.
14. File entries are never marked used; their `used_at` and `used_by` are always null, even after they approved someone (the audit log records each approval).
15. The local admin is excluded twice: the callback only runs the step for SSO users, and the step's transaction checks `auth_source = oidc` again.

## Deliberate breaks (each went red, then the file was restored byte for byte)

| Requirement | Break | Went red |
| --- | --- | --- |
| PRE-2 | `emailVerified` returns true at once (`internal/auth/oidc.go`) | `TestPRE2_EmailVerifiedClaimGatesTheStep`, `TestPRE2_EmailVerifiedParsing` (auth); `TestPRE2_NoMatchWithoutAVerifiedNonEmptyEmailClaim` (api) |
| PRE-3 | the callback gate's `su.Status != pending` check removed (`internal/auth/oidc.go`) | `TestPRE3_OnlyAPendingSSOUserReachesTheStep` |
| PRE-3 | the step's in-transaction check reduced to "not rejected" (`internal/api/preapproval.go`) | `TestPRE3_RejectedActiveAndLocalAdminAreNeverChanged` |
| PRE-5 | `GET /api/v1/admin/preapprovals` set to `AccessRole` in the route table | `TestPRE5_RoutesRefusedForEveryPrincipalButAnAdmin` |
| PRE-8 | a failed step reported as approved in `applyPreapproval` | `TestPRE8_FailedStepStillSignsInAndLeavesUserPending` (auth), `TestPRE8_FailedApprovalTransactionLeavesTheUserPending` (api) |

Note on the PRE-5 break: the exhaustive `TestPermissions_EveryRouteEveryActor` derives its expectation from the route table, so a wrong access level in the table does not make it fail on its own; the contract test (x-roles against the table) and `TestPRE5_RoutesRefusedForEveryPrincipalButAnAdmin` (which hard-codes "admin only") do. Only the latter was run during the break.

## Files changed

- New: `internal/store/migrations/0007_preapprovals.sql`, `internal/store/preapprovals.go`, `internal/store/preapprovals_test.go`, `internal/api/preapproval.go`, `internal/api/preapproval_signin_test.go`, `internal/api/preapproval_routes_test.go`, `internal/auth/preapproval_test.go`, `internal/app/preapproved.go`, `internal/app/preapproved_test.go`.
- Changed: `internal/auth/auth.go`, `internal/auth/oidc.go`, `internal/auth/oidc_test.go`, `internal/api/handlers_admin.go`, `internal/api/admin_common.go`, `internal/api/api.go`, `internal/api/routes.go`, `internal/api/testenv_test.go`, `internal/api/permissions_test.go`, `internal/api/contract_test.go`, `internal/api/router_test.go`, `internal/config/config.go`, `internal/config/config_test.go`, `internal/app/app.go`, `internal/store/store_test.go`, `internal/store/integration_queries_test.go`, `internal/store/ledger_compaction_test.go`, `internal/store/migrate0004_test.go`, `api/openapi.yaml`, `ui/src/api/schema.gen.ts` (generated), `deploy/config.sample.yaml`, `internal/{api,app,auth,store}/BUILD.bazel` (Gazelle).
- Not touched: `cmd/`, `docs/`, `e2e/`, `deploy/keycloak/`, the admin Users page, the ledger compaction code, git state.

## Commands run

- `go build ./...`, `go vet ./internal/...`: clean.
- `go test ./internal/...`: exit 0.
- `go test -race -count=1 ./internal/auth/ ./internal/api/`: both `ok` (auth 77 s, api 537 s).
- `tools/check_core_generic.sh`: `core-genericity: OK (185 core files, 197 Go files checked, no violations)`.
- `cd ui && npm run gen:api`, then `npm run typecheck`: clean.
- `bazel run //:gazelle`: updated `internal/{api,app,auth,store}/BUILD.bazel`.
- `./ci.sh > ci.log 2>&1`: exit 0 (`ci.exit` = 0); bazel test: 39 tests pass, including `//ui:api_schema_drift_test`.

## Confidence and gaps

- Verified by tests: every rule of D1, the D3 routes and validation, the D4 loader and start-up behaviour, as listed in the table above.
- The sign-in tests in `internal/api` use a minimal issuer that does not check the code or PKCE verifier; the full flow checks stay in `internal/auth`'s own tests. A real identity provider was not used (the end-to-end scenario is a later task).
- `gofmt -l internal/auth` lists `oidc_test.go`; it was already listed before this task touched it (struct field alignment in `fakeIssuer`), and CI does not run gofmt; it was not reformatted, to keep the diff to the added `claims` field.
- Not done here (other tasks): the admin page (D5, UI-11), `docs/deploying.md` (D6), e2e scenarios and the Keycloak user, `docs/decisions.md`.
