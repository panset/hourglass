# Progress, task 01 fixtures v1.1.0

- 2026-11-23 Read brief, intent, contract, design (Compatibility, Tests), D78, the d96a950 fixture folder and its recorder.
- 2026-11-23 Decision: v1.1.0's test environment already has `dbCopy` and `emitter`; only a `dbPath` option was missing, so `recorder_testenv.patch` adds just that.
- 2026-11-23 Decision: "admin setup views" are served by `GET /api/v1/setups` as admin and by the responses of `POST` and `PUT /api/v1/admin/setups` (`admin_views.go` has no route of its own; `GET /api/v1/admin/setups` does not exist at v1.1.0).
- 2026-11-23 Decision: made the recording reproducible by normalising wall-clock and random values in the database after the store is closed (admin created_at and hash, migrations applied_at, token last_used_at, event outbox emptied, sessions emptied, labels reinserted in key order, VACUUM). Ledger, holds, samples, rules, settings, setups, users, reports are untouched.
- 2026-11-23 Recorded in a detached worktree of v1.1.0 under the scratch directory; two runs and a third run from a clean worktree following the README gave identical files (`cmp`).
- 2026-11-23 Copied outputs, `openapi.yaml` (git show v1.1.0:api/openapi.yaml), wrote README, removed the worktree.
- 2026-11-23 Started `./ci.sh` (log `ci.log`, exit code `ci.exit`).
