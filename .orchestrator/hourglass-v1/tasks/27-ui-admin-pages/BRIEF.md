# 27 UI pages: Users, Setup inventory, Settings, Audit log
Job intent: .orchestrator/hourglass-v1/INTENT.md (read it first). Rules: .orchestrator/hourglass-v1/CONTRACT.md.
## Goal
Implement the four admin pages, replacing the placeholders in `ui/src/pages/admin-users/`, `admin-setups/`, `admin-settings/`, `admin-audit/`. Spec: docs/SPEC.md "Dashboard pages" rows Users, Setup inventory, Settings, Audit log; ADM-1..ADM-5, SET-1..SET-6, RULE-3, LED-4, LED-5, and the rules list under "Roles and permissions".
## Users (`/admin/users`)
Pending access requests on top (name, email, first sign-in time; oldest first) with Approve (choose role(s)) and Reject. Then every user: roles, balance, month-to-date burn, active holds; overdrawn users first, in the order the API returns. Actions: change roles (checkboxes for developer/admin; the local admin row has no role or balance actions), grant tokens (amount, may be negative, plus required reason), reset balance (required reason, confirm). API refusals (`self_demote`, `has_active_hold`, `local_admin_protected`) are shown as messages.
## Setup inventory (`/admin/setups`)
Setups with labels, weight, resource name, enabled flag, last seen by the provider; orphans listed separately. Add and edit in a form: name, labels as key/value rows with the SET-3 limits surfaced from API validation errors, a JSON editor for details (a textarea with JSON syntax validation and pretty-print; no editor library), resource name override, enabled. Disable, delete (confirm), and force-release of a held setup (required reason).
## Settings (`/admin/settings`)
Editable: resource name template, rate rules (ordered list: add, edit, delete, move up/down; each shows its match count; setups falling through to the default are listed; invalid selector errors shown against the offending rule), default weight, display label keys, pressure bands and multipliers (ordered thresholds), default allowance, hold limit, teardown before release, poll interval, enforcement (with a clear shadow-mode indication). Read-only: collector health (last successful poll, last error, outage history, recent failed event deliveries) and the active provider, pressure source, metering policy, hooks and OIDC issuer. Rules save through `PUT /admin/rules`, settings through `PUT /admin/settings`; unsaved-changes indication; nothing is saved implicitly.
## Audit log (`/admin/audit`)
No wireframe exists: build from the spec row. Admin actions, newest first: time, actor, action, target, and an expandable before/after JSON view; filters by actor and action; paging with limit/offset.
## Scope
May write: `ui/src/pages/admin-users/`, `ui/src/pages/admin-setups/`, `ui/src/pages/admin-settings/`, `ui/src/pages/admin-audit/`, this task folder.
## Context
Read tasks/25-ui-scaffold/RESULT.md first (layout, how to add a page, shared components, API client, testing setup), then `docs/ui-notes.md` (the wireframes in words: follow its layout, column headers and labels exactly; where it lists a difference from the spec, the spec wins) and the raw artboards under `docs/wireframes/project/` when a detail is unclear. API shapes: `ui/src/api/schema.gen.ts` (generated from `api/openapi.yaml`); do not edit the generated file or the YAML: if a field you need is missing, render what exists and list the gap in RESULT.md.
## Rules
- No business logic in the UI (UI-1): every number shown comes from an API field; formatting only. Never compute balances, costs, weights, projections or match counts client-side.
- Hiding is not security (UI-5): render from the session roles, but assume the API enforces.
- Live data refreshes every 60 seconds through the scaffold polling hook (UI-6); after a successful action refetch immediately.
- Every state-changing action shows the API error message from `{error:{code,message}}` on failure, and disables its button while in flight.
- Work only inside your page directories and add new shared pieces under your own directory; do not edit shared components, the router, the top bar, package.json or lockfiles (another agent is building the other pages at the same time). If a shared component needs a change, wrap it locally and note it.
- Tests (Vitest + Testing Library, with the API mocked at the fetch layer using typed fixtures): each page renders loading, empty, populated and error states; each action calls the right endpoint with the right body and CSRF header; role-dependent elements. Test files next to the pages.
- SPDX line at the top of every source file. Verify with `bazel test //ui/...` and `bazel build //ui/...` (allow 15 minutes; commands queue behind other agents) and `./tools/check_spdx.sh`.
## Done when
Checks pass; RESULT.md lists each displayed value with the API field it comes from, and any gaps.
