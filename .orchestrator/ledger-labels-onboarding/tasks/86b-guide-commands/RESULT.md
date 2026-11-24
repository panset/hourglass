# 86b result: "Pre-approving SSO users" commands run and corrected

Status: done. Stack: compose project `hourglass-86b` from the images built from the current tree (`deploy/load-images.sh`; Bazel reported them current), torn down afterwards. Transcript: `commands.log`. `./ci.sh` exit 0 (`ci.exit`).

## Making sure requests reached my stack
A host process (`hourglassd`, pid 56366) listens on 127.0.0.1:8080. I mapped my stack to 127.0.0.1:18080 with an override file (`override.yaml`, test-only) and used a standalone container on 18081. `docker ps --filter publish=18080` named my container; 8080 was never called. The guide keeps `localhost:8080`.

## Commands
| Command / block | Outcome |
| --- | --- |
| File way: `cat > preapproved-users.yaml`, `chmod`, `docker run ... HOURGLASS_PREAPPROVED_USERS_FILE`, `docker logs | grep` | Ran, worked as written (port, container name and volume changed only to avoid clashes). Log line with `entries: 2`; Users API lists both with source `config`. Same with the `preapproved_users_file` config key. |
| Invalid file | Start stops (exit 1), message names `users[1]` and never repeats a valid address, as the guide says. |
| Scripted: sign in, `GET /session`, `POST preapprovals`, `GET preapprovals` | Ran, worked as written once the first-run password was changed. Corrected: the guide had no command for that change, so on a fresh stack the block failed with `password_change_required`; I added a block for it and made the sign-in use the new password. |
| Scripted: bulk `split`/`jq` loop | Failed as written on an input with blank lines or a trailing blank line (400 `invalid email ""`). Corrected: `jq -R 'select(test("\\S"))'`. Also added a note that it reuses the earlier cookie jar and `$CSRF`, and that `jq` is needed. Ran 2,500 emails: three calls (1000, 1000, 500). |
| Validation and repeat claims (bad email, unknown role, repeat call) | Behave as the guide says: 400 `validation` naming the value; `updated` on repeat. |
| Admin page way | Matches `ui/src/pages/admin-users/preapprovals.tsx`: heading, emails separated by commas/spaces/new lines, Role select (Developer default), Add, columns Email/Roles/Source (Added here / Config file)/Added by/Signed in ("Not yet"), Remove with confirmation, no Remove on config rows. No change. |
| End to end with `dave` | Done from the shell: pre-approved `dave@example.com`, signed in through Keycloak by posting the login form with curl and replaying the callback on my port. Session: active, role developer, source oidc; balance 300; entry marked used. Browser UI sign-in is covered by e2e step A2p (not A8p, which the brief named; A8p is ledger paging) in `e2e/browser/tests/acceptance.spec.ts`. |

## Files changed
- `docs/deploying.md`: the three `86b` markers removed; password-change block and "Then the three calls (`jq` is needed)" lead-in added; sign-in password in the scripted block changed to `choose-another-long-password`; bulk loop corrected. No other place was changed.
- `docs/progress.md` (line 85): end-to-end steps now run (16/16 bundled, 16/16 split, integration 11/11, 2026-11-23).
- Task folder: `commands.log`, `override.yaml`, `ci.log`, `ci.exit`, `PROGRESS.md`, this file.

## Decisions for docs/decisions.md
None.

## Confidence and gaps
- No browser was driven; the sign-in used curl against Keycloak (Keycloak's cookies are `Secure`, so I passed them by hand).
- The ledger of audit events (`user.preapprove`) was not queried.
- I did not rebuild images beyond `load-images.sh`; it reported them current via the Bazel cache (appears to be current; image IDs unchanged).
- The brief names A8p for the sign-in step; the matching step is A2p.
