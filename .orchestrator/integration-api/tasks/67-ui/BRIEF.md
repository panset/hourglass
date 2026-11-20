# 67 UI: Service accounts page, "Releasing" state, last report times

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Ground rules, Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/67-ui/`.

## Goal
The admin UI can do everything the six service-account admin routes offer, the Setups page
shows a pending release, and the collector view shows when the last reports arrived.

## Context
- Design: `docs/design/integration-api.md`: "Admin routes for service accounts" (last
  paragraph), "Releasing a deployed setup" (the Setups page bullet), "Changes by area" (`ui/`).
- Backend facts: `.orchestrator/integration-api/tasks/61-service-accounts/RESULT.md` (admin
  routes), `tasks/65-release-changes/RESULT.md` (202 on release, `release_requested_at` on the
  hold JSON, and its list of UI spots that depended on the old `still_deployed` refusal).
- Existing UI: `ui/src/pages/` and `ui/src/pages/admin/`, the generated client in
  `ui/src/api/`, `docs/ui-notes.md` for the house layout, the existing admin pages for style,
  navigation and tests. Follow the existing patterns; add no new dependency.

## What to build
1. **Service accounts** admin page, in the admin navigation, admin-only: list accounts with
   scopes, enabled flag, and each token's id, created, expiry and last-used times; create
   (name, description, scopes as four checkboxes); edit description, scopes, enabled; delete
   with confirmation; issue a token (optional expiry); revoke a token. After issuing, the token
   is displayed once with a copy button and a warning that it cannot be shown again; it is
   held only in component state, never in storage, the URL or a log, and is gone when the
   dialog closes.
2. **Setups page**: a hold with `release_requested_at` set shows "Releasing" with the request
   time in place of the Release button. A release that answers 202 moves the row to that state
   without an error message. Remove UI handling that only existed for the old `still_deployed`
   refusal if it is now dead.
3. **Collector view** (wherever admins see poll status today): the last pressure report and
   last deployment report times. The admin UI must not call `/api/v1/integration/` routes
   (they refuse cookies). If no admin route exposes the report times today, add the smallest
   read-only addition to the existing admin collector/status response (fields for each
   report's `observed_at`, `received_at` and account name, null when absent or when that
   extension is not `reported`), with OpenAPI, contract coverage, a permissions-matrix row if
   it is a new route, and regenerated types. Record that choice in `docs/decisions.md`.
4. Tests in the style of the existing UI tests for each of the three, including: the token is
   shown once and not after closing; the Releasing state renders and the button is absent.

## Scope
May write: `ui/` (not lockfiles unless unavoidable; no new dependencies), and for item 3 only:
`api/openapi.yaml`, `internal/api/`, `ui/src/api/schema.gen.ts`, `docs/decisions.md` (append
with the next free number), your task folder.
Must not touch: `docs/SPEC.md`, `docs/design/`, `docs/integrating.md`, `e2e/`, other Go
packages, git state.

## Done when
`cd ui && npm run typecheck && npm test` pass; `./ci.sh` passes (it builds the UI under Bazel
and runs the API schema drift test).

## Long-running steps
`./ci.sh` takes roughly 5 to 15 minutes: run it once at the end, foreground, 30-minute timeout,
output to a log file in your task folder, read only the exit code and tail.

## Report
Follow the Reporting section of the contract. In RESULT.md list the visible labels and the
selectors or roles an end-to-end browser test can use for: opening Service accounts, creating
an account, issuing a token and reading it, and the "Releasing" state on Setups.

## Addendum from task 65
- `ui/src/pages` `setups.test.tsx` and `usage.test.tsx` mock the old 409; `usage/index.tsx`
  (about line 225) says release is refused; both pages show "Released ..." on any 2xx, so a
  202 would wrongly show that toast. Fix both pages (My usage needs the "Releasing" state too
  wherever it lists the user's holds with a Release button) and their tests.
- `release_requested_at` was left out of OpenAPI `required` for Hold and Holder so that the
  page fixtures compiled. The server always sends it (null when unset): make it `required`
  and nullable in `api/openapi.yaml`, regenerate the types, and update the fixtures.
- pnpm may not be installed on this machine; `npm run gen:api` works.
