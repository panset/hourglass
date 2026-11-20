# 51 admins land on Setups; My usage moves to /usage
Job intent: .orchestrator/hourglass-v1/INTENT.md. Rules: .orchestrator/hourglass-v1/CONTRACT.md (if `.md` files cannot be written, use `.txt` and put the result in your reply). Decision D41 in docs/decisions.md is the contract for this change and overrides the paths in the spec's Dashboard pages table for these two rows only. Read tasks/50-remove-resource-name's result first: that task has just changed parts of the UI.
## Change (UI only)
- `/` becomes a landing route with no page of its own: once the session is known, a user with the `admin` role (the local admin and OIDC admins alike) is sent to `/setups`; any other user with a role is sent to `/usage`. Role-less users still see only the not-authorized page; signed-out users the sign-in page; the forced password change still comes first.
- My usage moves from `/` to `/usage` (navigation link, lazy route, tests). The local admin, who has no balance, still has the My usage entry behaving as it does today (keep whatever it showed; do not add or remove navigation entries).
- The backend keeps redirecting to `/` after sign-in (UI-7: Go code names no UI route; the source-scan test's list of UI routes in `internal/api/registration_test.go` gains `/usage`, which is the only Go change allowed).
- A deep link is respected: opening `/admin/users` or `/usage` directly stays there after sign-in when the browser returns to it; only `/` redirects.
- Update tests (shell routing by role, navigation, page tests that assumed `/`), `e2e/browser/tests/acceptance.spec.ts` where it navigates to or expects My usage at `/`, `docs/ui-notes.md` is NOT edited (it describes the wireframes), README.md / docs that mention the My usage path.
## Out of scope
Everything else. Do not start or stop Docker containers (a demo stack is running for the owner) and do not run `deploy/smoke.sh` or `e2e/run.sh`.
## Verification
`bazel test //ui/...`, `bazel build //ui/...`, `go test ./internal/api/...`, `./ci.sh` exit 0, `./tools/check_spdx.sh`.
## Done when
All pass; the result file lists changed routes, files and any selector the end-to-end spec uses that changed.
