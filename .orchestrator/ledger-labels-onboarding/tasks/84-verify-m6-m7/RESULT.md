# 84 Independent verification of milestones 6 and 7: result

Verdict: **PASS with findings.** SET-8, UI-9, AUTH-9, UI-10, PRE-1 to PRE-8 and UI-11 are met in the code
at `b69e395`, and each has a test named after its id that went red when its requirement was broken. The
security read found no way for a person to get a role they should not under the design's trust model
(an identity provider that controls its users' email addresses). Findings: **0 high, 1 medium, 5 low.**
The medium is a test gap: the in-transaction status re-check, which D1 requires and which is what stops a
race, can be deleted without any test failing. The code itself does the re-check correctly; a probe
confirmed it.

Gates: `./ci.sh` exited 0, but every one of its bazel test targets came from cache. An uncached `bazel test //... --nocache_test_results` executed and passed all 39 of 39 targets (`bazel-nocache.log`). `go test -race` passed for auth, api and store with no data race reported (`race.log`). `tools/check_core_generic.sh` passed.

## Findings

| id | sev | file:line | failure scenario | suggested fix |
| --- | --- | --- | --- | --- |
| F1 | medium | `internal/api/preapproval.go:214` (test gap) | The re-check of `u.Status != pending` inside the approval transaction is not covered. Deleting it (break `pre3a`) keeps every Go test green, because the callback already filters on the status it read before the transaction (`internal/auth/oidc.go:302`). A later refactor could drop the re-check without a test failing. Then an admin who rejects a pending user between `UpsertOIDCLogin` and the approval transaction (or a second, concurrent sign-in) would be overridden, and the rejected user would become active with the entry's roles. The in-transaction source / local-admin check is likewise untested on its own (break `pre3c` stays green, because the local admin is also `active`). | Add a PRE-3 test that calls `Preapprover.ApplyPreapproval` directly with a matching database entry and a matching file entry for a `rejected` user and for an `active` user with no roles, and expects `false`, nothing written, and no event. Add one that runs several `ApplyPreapproval` calls at once for one pending user and expects one approval, one `user.preapprove` entry, one allowance and one `user.approved`. The temporary probes described below do both and pass against the current code. |
| F2 | low | `internal/api/preapproval.go:60` (`NormalizeEmail`) | `strings.ToLower` is Unicode lower-casing. It maps U+212A KELVIN SIGN to ASCII `k`, so the claim `Kate@example.com` matches the entry `kate@example.com`. A probe confirmed this: the user became active with the entry's `admin` role. Scenario: a provider lets users edit their own email, enforces uniqueness on the exact text and sends no `email_verified` claim (which counts as verified). An attacker sets the look-alike address and gets Kate's pre-approved role, including `admin`. This is low because the design and the guide already require a provider that controls its users' addresses. | Lower-case ASCII only on both sides. Or refuse to match when the trimmed claim is not ASCII but the entry is, or require `NormalizeEmail(claim)` to equal the claim with only ASCII case changed. Add a PRE-2 case for it. |
| F3 | low | `internal/app/preapproved.go:57-68`, via `config.YAMLToJSON` | A YAML file with several documents (`---`) loads only the first one. The `dec.More()` check sees only the JSON made from that document, so the rest are dropped without an error (probe `y6.yaml`). Scenario: an admin joins two lists with `---`. Everyone in the second list stays pending, and start-up reports success with a count that is too low. D4 asks for strict decoding. | Refuse a YAML stream with more than one document. Use `yaml.NewDecoder` and check that a second `Decode` returns `io.EOF`. The seed loader appears to share `YAMLToJSON` (inference), so it has the same gap. |
| F4 | low | `internal/app/preapproved.go:63-65` | The JSON decode is not fully strict. Keys match case-insensitively (`{"Users":[{"EMAIL":...}]}` is accepted), and a key repeated in one entry keeps its last value: `{"email":"a@…","email":"b@…"}` loads only `b@…` (probe `j2.json`). Scenario: a generated file has a merge mistake, one address is silently dropped, and that user stays pending. YAML refuses the same duplicate (probe `y.yaml`), so the two formats behave differently. | Check keys with a token-level pass, or decode into `map[string]json.RawMessage` and compare the key set exactly. |
| F5 | low | `internal/api/preapproval.go:78` | Only `unicode.IsSpace` characters are refused. Zero-width and other format characters (U+200B, U+200C, U+FEFF), which often come along when emails are pasted from documents or spreadsheets, are accepted and stored (probe `y3.yaml`). That entry can never match a real claim, and the admin page shows it looking exactly like the correct address. Scenario: an admin pastes a list from a document, the people on it stay pending, and the page looks correct. | Also refuse characters that are not graphic (`!unicode.IsGraphic` or the `Cf` category) in `ParsePreapprovalEmail`, for the route and the file. |
| F6 | low | `internal/auth/oidc.go:251-258` | `given_name`, `family_name` and `preferred_username` are decoded as Go `string`. A provider that sends one of them as a non-string (a number or an object; `null` is fine) now fails sign-in with "bad id token claims", where `d96a950` signed the user in. This is a small departure from "sign-in behaves exactly as today" (D1, UPG-2), and only for unusual providers. | Decode the three fallback claims as `json.RawMessage` or `any`, and use them only when they are strings. |

Checked and not a finding:

- **D74(b) versus B2.** It does not contradict B2. B2 says "if the box holds another `key=value` term for the same key, that term is replaced", which assumes a single such term. D74(b) covers the case B2 leaves open: when the box holds several equality terms for the key, the first is replaced and the rest are dropped. Two equality terms for one key AND together and match nothing unless they are equal, so dropping them gives the only useful result. Toggle, replace and append behave as B2 says (`ui/src/pages/setups/labelTerms.ts:34-45`), and `!=`, `in`, `notin` and bare-key terms are left alone.
- **D75.** Consistent with C1 and C2.
- **D76.** Consistent with D1 to D4. (d) treats `email_verified` values the design does not name (numbers, other strings) as not verified. That is stricter than "not false", but it fails closed, and the design names only booleans and the two strings. (f) says a new user whose approval fails gets `user.pending`, which matches "stays pending".
- **AUTH-5 tests and the router field-list test.** They gained `display_name` and nothing else. `internal/auth/session_test.go` still checks the exact length, and `internal/api/router_test.go:244` still compares the full sorted key list.

## Requirement -> test -> break

Each break was one `perl` edit to production code. The targeted tests were run, and the file was restored
from a copy (logs: `break-<name>.log`). `git diff` was empty for each file after each break.

| req | test(s) that went red | break made |
| --- | --- | --- |
| SET-8 | `TestSET8_LabelIndexIgnoresSelector` | `set8`: index built after the selector filter (`handlers_user.go`) |
| SET-8 | `TestSET8_LabelTerm`, `TestSET8_LabelIndexTermsMatchExactlyTheirSetups` | `set8c`: `LabelTerm` with the character check and the parser check both removed. (`set8b`, the character check alone, stayed green: the parser check behind it still refuses. That is defence in depth, not a gap.) |
| UI-9 | 3 UI-9 tests (toggle, whole terms, keyboard) | `ui9`: toggle-off removed from `applyChip` |
| UI-9 | "a value with no term cannot be activated and says why" | `ui9b`: a null-term chip made clickable |
| AUTH-9 | `TestAUTH9_StoredNameFallbackOrder`, `TestAUTH9_NextSignInFillsEmptyStoredName` | `auth9`: given/family fallback removed (`oidc.go`) |
| AUTH-9 | `TestAUTH9_SessionDisplayName`, `TestAUTH9_SessionAndAPIShareOneDisplayNameFunction`, `TestAUTH9_NoRoleSessionHoldsDisplayNameAndNothingElseNew` | `auth9b`: session `display_name` set to `u.Name` instead of `DisplayName` |
| UI-10 | "shows no greeting at all when display_name is empty" | `ui10`: greeting always rendered |
| UI-10 | "greets an SSO user…", "greets the local admin", UI-5 Gate test | `ui10b`: "Hello, " dropped |
| PRE-1 | both `TestPRE1_*` | `pre1`: first allowance skipped in the pre-approval path |
| PRE-2 | `TestPRE2_MatchIgnoresCaseAndSurroundingWhitespace` | `pre2`: no lower-casing in `NormalizeEmail` |
| PRE-2 | `TestPRE2_NoMatchWithoutAVerifiedNonEmptyEmailClaim`, `TestPRE2_EmailVerifiedClaimGatesTheStep`, `TestPRE2_EmailVerifiedParsing` | `pre2b`: string `"false"` counted as verified |
| PRE-3 | `TestPRE3_OnlyAPendingSSOUserReachesTheStep` | `pre3b`: callback status filter removed |
| PRE-3 | `TestPRE3_RejectedActiveAndLocalAdminAreNeverChanged` | `pre3ab`: callback filter and in-tx re-check both removed |
| PRE-3 | **none (F1)** | `pre3a`: in-tx status re-check removed; `pre3c`: in-tx source / local-admin check removed |
| PRE-4 | `TestPRE4_AuditEventsAndFirstUse`, `TestPRE4_ApprovedNewUserGetsNoPendingEvent` | `pre4`: `user.pending` emitted for an approved new user |
| PRE-4 | `TestPRE4_AuditEventsAndFirstUse` | `pre4b`: first use never marked |
| PRE-5 | `TestPRE5_RoutesRefusedForEveryPrincipalButAnAdmin`, `TestUI2_ContractRoutesMatch` | `pre5`: `POST /api/v1/admin/preapprovals` set to `AccessRole` in the route table. The exhaustive permissions test stays green, as the author said, because it takes its expectation from the table; the two tests named here catch the change. |
| PRE-5 | `TestPRE5_AddValidatesEveryEmailAndRoleAllOrNothing` | `pre5b`: an invalid email after a valid one accepted (not all or nothing) |
| PRE-6 | `TestPRE6_InvalidFileIsAnErrorNamingTheEntry` | `pre6`: `DisallowUnknownFields` removed |
| PRE-6 | `TestPRE6_InvalidFileIsAnErrorNamingTheEntry` | `pre6b`: repeated email accepted |
| PRE-6 | `TestPRE6_ValidFileIsAReadOnlyOverlay` | `pre6c`: file entries not passed to the Preapprover in `app.go` |
| PRE-7 | `TestPRE7_UnionOfBothSources` | `pre7`: file roles ignored when a database entry exists |
| PRE-8 | `TestPRE8_FailedApprovalTransactionLeavesTheUserPending`, `TestPRE8_FailedStepStillSignsInAndLeavesUserPending` | `pre8`: a failed step aborts the sign-in |
| UI-11 | 6 UI-11 tests (split and post) | `ui11`: emails split on commas only |

Every working-tree change I made was undone. At the end `git diff` was empty: outside `.orchestrator/` the tree matched HEAD `b69e395`, and no git state was touched.

## Security read: conclusions

Sign-in path: `internal/auth/oidc.go:250-333`, `internal/api/preapproval.go:192-272`,
`internal/store/preapprovals.go`.

- **Who can be approved.** Only a pending SSO user. The callback filters on source and status (`oidc.go:302`). The transaction re-reads the user and checks source, local admin and status again (`preapproval.go:208-216`). A probe that approved a `rejected` user directly with a matching entry got `false` and no writes. Rejected, active (with or without roles) and local users are never changed (TestPRE3 and the probe). Deleting an entry touches only the `preapprovals` row (`store/preapprovals.go:92-105`).
- **Races.** The store opens every transaction with `_txlock=immediate` (`internal/store/store.go:125`), so approval transactions are serialised against each other and against an admin's reject or role change. Whichever commits second sees the first one's status. Eight concurrent `ApplyPreapproval` calls for one pending user (a probe run under `-race`) gave 1 approval, 1 audit entry, 1 allowance and 1 `user.approved`. A rejection that commits first wins. An approval that commits first is followed by the rejection, so the admin's later action stands. The only gap is the test coverage in F1.
- **`email_verified`.** Absent and `null` count as verified. A boolean is taken as is. A string counts as verified only if it is `"true"` (trimmed, any case). Every other JSON type (number, array, object, any other string) counts as not verified (`oidc.go:319-334`). This is tested in `TestPRE2_EmailVerifiedParsing` and through the full flow.
- **Claim used.** Only `claims.Email` reaches the step (`oidc.go:279`). `users.email`, `preferred_username`, `name` and `upn` are never consulted (the TestPRE2 "address in other claims" case).
- **Normalisation.** Entries from both the route and the file go through `ParsePreapprovalEmail`, then `NormalizeEmail` (TrimSpace plus ToLower), and the claim goes through the same `NormalizeEmail`, so all three sources are normalised the same way. Internal whitespace cannot match, because entries refuse it. A trailing dot is compared literally: `a@b.test.` matches only `a@b.test.`, which fails closed. Gaps: Unicode lower-casing (F2) and invisible characters (F5).
- **File loader.** It is read before the database is opened, has a 10,000-entry limit, applies the route's email and role rules, refuses repeats and unknown keys, and names entries by index. Gaps: multi-document YAML (F3) and JSON key case and duplicate keys (F4). It is never written: the code only uses `os.ReadFile`, and TestPRE6 checks the modification time and contents.
- **`admin` through pre-approval.** Allowed, as the design's open-point default says, and audited (`user.preapprove` after-roles, TestPRE7).
- **Failure half-way.** Roles, status, allowance, first-use mark and audit are written in one transaction. A forced failure at the last write (the audit insert) rolls everything back. The user stays pending with no role, no allowance and an unused entry, and gets `user.pending` (TestPRE8). The event is emitted only after commit (`preapproval.go:268`).
- **Sessions.** Roles and status are loaded from the database on every request (`internal/auth/session.go:139-152`). The session created after approval therefore carries the roles (TestPRE1). A probe confirmed that a stale pending session from an earlier sign-in elsewhere also shows `active` / `[developer]` after the approval, and can call `/api/v1/me`.
- **Events.** An approved new user gets one `user.approved`, with the union of roles, and no `user.pending`. A user who was not approved gets `user.pending` as before, including when the step fails. Signing in again while active emits nothing (TestPRE4).
- **Admin routes.** All three are `AccessAdmin` in `routes.go:66-68`. CSRF is required on POST and DELETE (TestPRE5 checks the `csrf` error code). TestPRE5 runs every principal of the permissions fixture: anonymous, no role, rejected, roles removed, developer, admin, local admin, local admin before the password change, and a service account with and without scope. A wrong level in the table is caught (break `pre5`). Validation is all or nothing: every email is checked before roles, and both before the transaction. Audit: `preapproval.add` holds the sorted distinct emails and roles with the admin as actor. `preapproval.delete` holds the removed email and roles. `user.preapprove` has no actor (listed as "system"), `before` {roles, status}, and `after` {roles, status, sources}.
- **Logging.** At start-up only the path and the count are logged (`app.go:172`; TestPRE6 checks that no address is logged). The sign-in failure log has `user_id` and the error. The redacted config view shows the path (`TestPRE6_PreapprovedUsersFileKey`). One note: a start-up error for an invalid file quotes the invalid value, shortened to 80 characters. The design asks for the entry to be named, so this is acceptable.

## Other checks (brief items 4 to 8)

- **`setUserRoles` refactor.** Behaviour is unchanged compared with `git show 36a93b0:internal/api/handlers_admin.go`. The order is the same: set roles, then, when roles are not empty, the ledger nil check, activation and the `user.approved` event, then the allowance, then the audit with the same before and after. `u.ID` is the same user as the path id. Its existing tests are not edited in these commits.
- **Compatibility.** `git diff d96a950..HEAD -- api/openapi.yaml` removes and renames nothing. The `-` lines are reformatting of the ledger schema (milestone 4) and edited descriptions. `Session` gains only `display_name`, and `SetupList` gains only `label_index`. Regenerating `ui/src/api/schema.gen.ts` with `node scripts/gen-api.mjs` gives an identical file. With no entries and no file, a new SSO user is pending, gets `user.pending`, gets no audit entry and gets a pending session (`TestPRE4_NoEntriesNoFileSignInAsBefore`, `TestPRE6_NoStepSignInAsBefore`). F6 is the only difference found.
- **Labels and greeting against B1, B2, C1 and C2.** Every bullet is implemented. The index is built over the rows the caller may see (`ListSetups(ctx, !isAdmin)`) before the selector filter, and is sorted and distinct. Terms come from `rules.LabelTerm` and are checked against the parser. A null term is shown as an `aria-disabled` chip with an explanation. The placeholder comes from the index. The two numbers (12 and 8) are unchanged. The greeting is cut at 240 px with an ellipsis, has a `title`, and is absent when the name is empty.
- **Existing tests edited.** No assertion is weaker. The edits are: the migration version 6 -> 7 (five store tests), the route count 40 -> 43, `display_name` added to the two AUTH-5 field lists and the router key list, `label_index` added to the SET-6 key list (still an exact key check), "Dana Dev" -> "Hello, Dana Dev", an extra mocked route in admin-users, the `preapprovals` columns added to the schema spec, and fixture additions (the fake issuer's optional claims, a seeded pre-approval and permission markers).
- **Gates.** `tools/check_core_generic.sh` passes (exit 0, `core-generic.log`). Race and CI results are in the summary at the top.

## Commands run

- `go test -count=1` of auth, api, store, app, rules and config at baseline: all ok (`baseline-go.log`).
- 26 break runs (`break-*.log`).
- Temporary probe test files `internal/api/zz_verify84_test.go` and `internal/app/zz_verify84_test.go`, each created, run and deleted in the same command.
- `node scripts/gen-api.mjs` into the scratchpad, then `cmp`.
- `bash tools/check_core_generic.sh`.
- `go test -race -count=1 ./internal/auth/ ./internal/api/ ./internal/store/` (`race.log`).
- `./ci.sh` (`ci.log`, `ci.exit`).
- `bazel test //... --nocache_test_results` (`bazel-nocache.log`).

Files changed: only this task folder (PROGRESS.md, RESULT.md, logs, baseline.sha).

## Confidence and gaps

- I did not run the end-to-end stack (Keycloak). The sign-in path was exercised with the in-process OIDC issuers of the tests.
- The F2 attack scenario depends on how a given provider behaves; I did not check any real provider (inference).
- That the seed loader shares the F3 behaviour is inferred from the shared `config.YAMLToJSON`. I did not test it.
- I did not read the authors' PROGRESS.md or RESULT.md files of tasks 80 to 83.
