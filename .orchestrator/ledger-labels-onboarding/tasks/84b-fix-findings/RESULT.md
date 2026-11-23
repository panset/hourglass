# 84b Fix the findings of verification 84: result

Status: done. All six findings are closed. `go test ./internal/...` passes, `go test -race ./internal/auth/ ./internal/api/` passes (`race.log`), `tools/check_core_generic.sh` passes (`core.log`), `./ci.sh` exited 0 (`ci.exit` = 0, `ci.log`); api, app, auth and config test targets executed (8 of 39 ran, not cached).

## Findings and what closed them

| id | closed by |
| --- | --- |
| F1 | New `internal/api/preapproval_recheck_test.go`. `TestPRE3_ApplyPreapprovalRechecksTheUserInsideItsTransaction` calls `Preapprover.ApplyPreapproval` directly (matching database entry and matching file entry) for a rejected user, an active user with no roles, an active user with a role, and a pending local admin; it expects `false`, no error, no change, no event and no `user.preapprove` entry. `TestPRE3_ConcurrentApplyPreapprovalApprovesOnce` runs 8 calls at once: one approval, one `user.approved`, one audit entry, one allowance. Proof: with `u.Status != pending` removed from `internal/api/preapproval.go` (the in-transaction check) both tests went red (rejected, active, active with a role, concurrent); with the source / local-admin part removed the first test went red; file restored after each. |
| F2 | One shared `NormalizeEmail` (`internal/api/preapproval.go`): TrimSpace plus ASCII-only lower-casing. It is the only email normaliser (route, file loader and claim all go through it; `admin_views.go` lower-casing is a sort key, not an email comparison). `TestPRE2_FoldsASCIICaseOnlyLookAlikesNeverMatch` covers Kelvin sign and dotted capital I in the claim against ASCII entries (database and file), and a Kelvin-sign entry matching only itself. Red when `ToLower` is put back. |
| F3 | `config.YAMLToJSONSingle` (new, `internal/config/config.go`) refuses a second YAML document; used only by the pre-approval loader. `TestPRE6_InvalidFileIsAnErrorNamingTheEntry` case "second YAML document". |
| F4 | `checkJSONKeys` in `internal/app/preapproved.go` refuses a duplicate key at any depth and a known key (`users`, `email`, `roles`) in another case (`unknown field "EMAIL"`). Runs on the JSON and on YAML converted to JSON. YAML duplicate keys were already refused by the decoder; cases added for them. PRE-6 cases: duplicate entry key, duplicate top key, key in another case (JSON and YAML). |
| F5 | `ParsePreapprovalEmail` refuses Unicode Cc and Cf characters ("it contains a control or invisible character") for the route and the file. `ApplyPreapproval` also returns false for a claim holding one. Tests: PRE-5 `bad` cases (zero-width space, BOM, control, bidi override), PRE-6 file cases, `TestPRE2_ClaimWithInvisibleCharacterNeverMatches`. Red when the route check is removed. The claim-side check is defence in depth: a claim with an invisible character already cannot equal any stored entry, so that test stays green without it. Surrounding whitespace (including U+0085, which `TrimSpace` treats as space) is still trimmed from a claim, as before. |
| F6 | `internal/auth/oidc.go`: `given_name`, `family_name`, `preferred_username` decode as `json.RawMessage` and count as empty unless a string (`claimString`). `name` stays a Go `string`, exactly as at d96a950 (`git show d96a950:internal/auth/oidc.go`: `Name string`; a non-string `name` made `Claims` fail with "bad id token claims", `null` was an empty name). `TestAUTH9_NonStringFallbackClaimsNeverFailSignIn` and `TestAUTH9_NameClaimBehavesAsBefore` (number, object, array, bool refused; null accepted). Red when the given-name claim is a `string` again. |

## Seed loader (not changed)

`internal/app/seed.go:66-72` uses `config.YAMLToJSON` and then `DisallowUnknownFields`, with no key check. By reading the code (not run): a second YAML document is silently dropped, a repeated JSON key keeps the last value, JSON keys match without regard to case; a repeated YAML key is an error from the YAML decoder. So the seed loader has the gaps F3 and F4 describe. The owner may want the same strictness there; it was out of scope.

## Files changed

`internal/api/preapproval.go`, `internal/api/BUILD.bazel`, `internal/app/preapproved.go`, `internal/config/config.go`, `internal/auth/oidc.go`; tests: `internal/api/preapproval_recheck_test.go` (new), `internal/api/preapproval_signin_test.go`, `internal/api/preapproval_routes_test.go`, `internal/app/preapproved_test.go`, `internal/auth/greeting_test.go`. No existing assertion was weakened or removed. Gazelle diff is clean.

## Decisions for docs/decisions.md

- Email comparison folds ASCII letters only (one `NormalizeEmail`, used by the admin route, the file loader and the claim); a non-ASCII look-alike matches only itself. Surrounding Unicode whitespace is still trimmed.
- An email containing a control or format character (Unicode Cc, Cf) is invalid in the admin route and in the file; a claim containing one never matches.
- The pre-approval file must be one YAML document with no repeated key and keys spelled exactly (`users`, `email`, `roles`); otherwise start-up stops with a message. The seed loader is unchanged and does not have these checks.
- `given_name`, `family_name` and `preferred_username` that are not strings count as empty and never fail a sign-in; `name` keeps the d96a950 behaviour (a non-string `name` fails the sign-in, `null` is empty).

## Confidence and gaps

- Seed-loader behaviour is from code reading, not a probe.
- The full-suite race run of `internal/api` took about 9 minutes under load; it passed.
- A non-string `name` still refusing a sign-in is inherited from d96a950 on purpose, per the brief; it is not a new departure.
