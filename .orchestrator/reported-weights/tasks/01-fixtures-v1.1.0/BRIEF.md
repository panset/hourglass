# 01 Record the v1.1.0 compatibility fixtures
Job intent: .orchestrator/reported-weights/INTENT.md (read it first). Contract: .orchestrator/reported-weights/CONTRACT.md (binding).
## Goal
`testdata/upgrade-v1.1.0/` holds fixtures recorded from the code of release `v1.1.0` (tag `v1.1.0`, commit `753cc50`), in the way D78 did for `d96a950` (read D78 in docs/decisions.md, `testdata/upgrade-d96a950/` and its README, and CONTRIBUTING.md "Upgrade fixtures"). They exist so that later tests for WGT-19 and WGT-20 (read both in docs/design/reported-weights.md, and its Compatibility and Tests sections) compare the new code against what v1.1.0 actually produced. No product code has changed yet on this branch, and none may change in this task.
## What to record
1. A database created and written by v1.1.0 code (`hourglass.sqlite3`): several setups with labels that hit different rate rules and the default weight (include a rule of weight 0 and a non-integer weight), users with roles, active holds that are metering, a released hold, a hold with a pending release if the test env can make one, ledger data over more than one poll and more than one day (debits, a grant), usage samples, non-default settings where harmless, a service account with every v1.1.0 scope, and stored deployments and pressure reports if the test env supports the `reported` extensions.
2. `responses.json`: the v1.1.0 responses, at a fixed clock, of every route the design touches: `GET /api/v1/setups`, `GET /api/v1/me`, `GET /api/v1/me/holds`, `GET /api/v1/me/ledger` (default, `rollup=day`), the admin setup views (`GET /api/v1/admin/setups` and whatever admin_views.go serves), `GET /api/v1/admin/users`, `GET /api/v1/admin/rules`, `GET` and `PUT /api/v1/admin/settings` (a PUT with the eight current keys), `POST` and `PUT /api/v1/admin/setups` (without any new field), `GET /api/v1/admin/collector`, `GET /api/v1/admin/service-accounts`, `GET /api/v1/integration/state`. Record request, status and body for each, for the principals that make sense (developer, admin, service account).
3. `poll.json`: for one or more collector polls run by v1.1.0 code over (a copy of) that database at fixed clock times: the exact snapshot (`SetupState` list and everything else) passed to the pressure source and to the metering policy, the pressure and multiplier, and every usage sample and ledger debit the poll wrote, with balances before and after. Later tests replay the same poll on the new code and require identical debits and snapshots (WGT-19).
4. That release's `api/openapi.yaml`, copied unchanged.
5. The recorder itself, committed beside the fixtures as text (not compiled in this tree), and a README with the exact steps to reproduce, as `testdata/upgrade-d96a950/` does.
## How
Record in a separate detached worktree of the tag, outside the repository, e.g. `git worktree add --detach <scratchdir>/hg-v1.1.0 v1.1.0`, and remove it with `git worktree remove` when done. That one worktree command pair is the only git state change you may make. Make the recording deterministic (fixed clock, fixed ids) so a second run produces identical files; run it twice and diff.
## Scope
May read: whole repo. May write: `testdata/upgrade-v1.1.0/` (new) and this task folder; any BUILD.bazel change Gazelle or the SPDX/licence checks strictly need for the new folder (say so in RESULT.md). Must not touch: any product code, any existing test, `testdata/upgrade-d96a950/`.
## Done when
- The files above exist; the recording is reproducible; nothing in them names a real person, credential, or the first client (CONTRACT.md).
- `./ci.sh` passes (exit 0) with the new folder present, and the existing d96a950 tests pass unchanged.
- RESULT.md lists every recorded route with principal and status, describes the database contents (rows per table), and says what a consuming test must do to load each file.
## Long-running steps
`./ci.sh`: several minutes, use a 30 minute timeout, log to this task folder.
## Report
Per CONTRACT.md "Reporting".
