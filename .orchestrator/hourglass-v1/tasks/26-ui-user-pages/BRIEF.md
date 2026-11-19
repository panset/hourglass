# 26 UI pages: My usage and Setups
Job intent: .orchestrator/hourglass-v1/INTENT.md (read it first). Rules: .orchestrator/hourglass-v1/CONTRACT.md.
## Goal
Implement the two pages every user with a role sees, replacing the placeholders in `ui/src/pages/usage/` and `ui/src/pages/setups/`. Spec: docs/SPEC.md "Dashboard pages" rows My usage and Setups and the whole "Behaviour worth pinning down" list; HOLD-1, HOLD-2, SET-6.
## My usage (`/`)
Balance, current burn per hour, projected run-out date ("not this month" when the API gives none), lab pressure; current holds with a Release action; tokens per day this month as a simple chart (hand-drawn SVG bars, no chart library); ledger history; a month picker for past months that reloads usage and ledger. The local admin sees no balance and no holds section (it cannot acquire).
## Setups (`/setups`)
Every setup: name, the admin-chosen label columns (`display_label_keys` from the response), weight, state, holder, holder's balance, held for, current cost per hour. Filters: by state and by each displayed label (client-side filtering of the fetched list is fine; a free-text selector box that passes `selector=` to the API and shows its parse error is the label filter of record). "Details" opens the setup's details JSON, pretty-printed, read-only. Acquire asks for confirmation and shows the cost per hour at the current multiplier and at the highest band (both from the API). Release on own holds; a refusal shows the API message ("This setup is still deployed. Tear it down, then release."). Overdrawn users are flagged as the wireframe shows; when the API reports the user cannot acquire (zero balance with enforcement on, or at the hold limit) the Acquire buttons are disabled with the reason. The local admin sees no Acquire button. Admins additionally see disabled setups marked as such.
## Scope
May write: `ui/src/pages/usage/`, `ui/src/pages/setups/`, this task folder.
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
