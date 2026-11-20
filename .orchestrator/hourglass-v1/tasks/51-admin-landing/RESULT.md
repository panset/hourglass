# RESULT: 51 admins land on Setups; My usage moves to /usage (D41)
Status: done.

## Routes
- `/` has no page: Gate redirects to `/setups` for the admin role (local admin included), `/usage` for any other role. No-role, signed-out and forced-password-change handling unchanged and still first. `/login`, `/unauthorized`, `/change-password` for a signed-in user now go through the same landing rule.
- `/usage` is My usage (nav link, lazy route). Deep links (`/usage`, `/admin/users`, ...) are not redirected. Local admin keeps the My usage entry unchanged. Navigation entries not added or removed.

## Files changed
- ui/src/routes.tsx, ui/src/shell/Gate.tsx, ui/src/shell/TopBar.tsx (dropped `end` on `/`), ui/src/pages/not-found/index.tsx (link to /usage), ui/src/pages/usage/index.tsx (comment)
- Tests: ui/src/shell/Gate.test.tsx (new: admin and local admin land on /setups; deep link /usage; no role on `/`; developer lands /usage), ui/src/pages/login/login.test.tsx, ui/src/pages/change-password/change-password.test.tsx (local admin now lands on Setups)
- internal/api/registration_test.go (`/usage` added to uiRouteLiterals; only Go change)
- e2e/browser/tests/acceptance.spec.ts (`reloadHome` and the carryover poll go to `/usage`; A2 `goto("/")` kept, alice is a developer and lands on My usage)
- docs/deploying.md (lands on My usage `/usage`). README.md had no mention. docs/ui-notes.md untouched.

## Changed e2e selectors
None; only URLs. Admin sign-in (`adminSignIn`) waits for the Main nav, still true on Setups.

## Verification
bazel test //ui/... pass; bazel build //ui/... ok; go test ./internal/api/... ok; ./ci.sh exit 0; ./tools/check_spdx.sh exit 0. Docker, smoke.sh and e2e/run.sh not touched or run.

## Confidence and gaps
E2E spec not executed. Backend still redirects to `/` after sign-in; a pre-login deep link is not remembered (not added: beyond the brief).
