# Result, task 01: v1.1.0 compatibility fixtures

Status: done. `./ci.sh` exit code 0 (`ci.exit`, `ci.log`; 39 bazel tests pass, cached because no product file changed; core-genericity, licence and Gazelle checks pass).

## What was done
Recorded `testdata/upgrade-v1.1.0/` from the code of tag `v1.1.0` (commit 753cc50) in a detached worktree outside the repository (created and removed with `git worktree add --detach` / `git worktree remove --force`, the only git state change). Two runs into two directories, and a third from a clean worktree following the README, wrote byte-identical files (`cmp`, `diff -r`). No product code, existing test or `testdata/upgrade-d96a950/` file was touched. No BUILD.bazel change was needed (the folder holds no Go source, only `.txt`, `.patch`, `.json`, `.md`, `.yaml` and a database).

## Files changed (complete list)
- testdata/upgrade-v1.1.0/README.md
- testdata/upgrade-v1.1.0/hourglass.sqlite3
- testdata/upgrade-v1.1.0/responses.json
- testdata/upgrade-v1.1.0/poll.json
- testdata/upgrade-v1.1.0/openapi.yaml
- testdata/upgrade-v1.1.0/recorder_upg_v110_record_test.go.txt
- testdata/upgrade-v1.1.0/recorder_testenv.patch
- .orchestrator/reported-weights/tasks/01-fixtures-v1.1.0/PROGRESS.md, RESULT.md, ci.log, ci.exit

## Database contents (rows per table; `responses.json` `facts.rows_per_table`)
users 6 (local admin, dev, dev2, other with no activity, norole with no role, ops with developer and admin), user_roles 6, setups 8, setup_labels 10, rules 3 (kind rate: `tier=gold` 2.5, `memory-gb=64` 4, `tier=free` 0), holds 5 (4 active and metering, one of them with a pending release, 1 released), ledger 121 (4 allowance, 1 grant, 116 debits, over 3 local days in Europe/Berlin: 2026-11-02, 2026-11-03, 2026-11-12), usage_samples 157 (including a policy weight of 3.25 with a null rule selector, a weight 0, a non-integer weight), pressure_samples 34, collector_runs 34, outages 1 (a nine-day gap), settings 3 (`default_weight` 1.25, `display_label_keys`, `max_holds_per_user` 4), service_accounts 2 (`reporter` with all four v1.1.0 scopes, `viewer` with `state:read`), service_tokens 2, reported_pressure 1 (0.8), reported_deployments_meta 1, reported_deployments 5 (four setups and the unknown `lab-99`), audit_log 1 (the grant), orphans 0, preapprovals 0, ledger_compaction 0, event_deliveries 0, sessions 0, schema_migrations 7. Clock at the end 2026-11-12T09:25:00Z.

## Recorded routes (`responses.json`, at 2026-11-12T09:25:00Z, fresh process, no poll since start)
| name | request | principal | status |
| --- | --- | --- | --- |
| setups_dev | GET /api/v1/setups | developer | 200 |
| setups_dev_selector | GET /api/v1/setups?selector=tier%3Dgold | developer | 200 |
| setups_dev2 | GET /api/v1/setups | developer (holder) | 200 |
| setups_admin | GET /api/v1/setups | admin | 200 |
| setups_norole | GET /api/v1/setups | user with no role | 403 |
| session_dev | GET /api/v1/session | developer | 200 |
| me_dev, me_dev2, me_other, me_admin | GET /api/v1/me | developer, holder, no activity, admin | 200 |
| me_norole | GET /api/v1/me | user with no role | 403 |
| me_holds_dev, me_holds_dev2 | GET /api/v1/me/holds | developer, holder | 200 |
| me_holds_dev2_active | GET /api/v1/me/holds?state=active | holder | 200 |
| me_ledger_dev, me_ledger_dev2 | GET /api/v1/me/ledger | developer, holder | 200 |
| me_ledger_dev_day, me_ledger_dev2_day | GET /api/v1/me/ledger?rollup=day | developer, holder | 200 |
| me_usage_dev, me_usage_dev2 | GET /api/v1/me/usage | developer, holder | 200 |
| pressure_dev | GET /api/v1/pressure | developer | 200 |
| admin_users | GET /api/v1/admin/users | admin | 200 |
| admin_users_ops | GET /api/v1/admin/users | developer and admin | 200 |
| admin_users_dev | GET /api/v1/admin/users | developer | 403 |
| admin_rules | GET /api/v1/admin/rules | admin | 200 |
| admin_settings | GET /api/v1/admin/settings | admin | 200 |
| admin_collector | GET /api/v1/admin/collector | admin | 200 |
| admin_service_accounts | GET /api/v1/admin/service-accounts | admin | 200 |
| integration_state_reporter | GET /api/v1/integration/state | service account, every scope | 200 |
| integration_state_viewer | GET /api/v1/integration/state | service account, state:read only | 200 |
| integration_state_dev | GET /api/v1/integration/state | developer session | 401 |
| integration_state_anonymous | GET /api/v1/integration/state | no credential | 401 |
| admin_settings_put | PUT /api/v1/admin/settings (the eight current keys, from the GET) | admin | 200 |
| admin_settings_put_dev | PUT /api/v1/admin/settings | developer | 403 |
| admin_setup_post | POST /api/v1/admin/setups (lab-09, no new field) | admin | 201 |
| admin_setup_post_no_rule | POST /api/v1/admin/setups (lab-10, disabled, matches no rule) | admin | 201 |
| admin_setup_put | PUT /api/v1/admin/setups/9 | admin | 200 |
| admin_setup_put_held | PUT /api/v1/admin/setups/1 (held setup, labels changed) | admin | 200 |
| admin_setup_post_dev | POST /api/v1/admin/setups | developer | 403 |
| setups_admin_after_writes, admin_users_after_writes | GET /api/v1/setups, GET /api/v1/admin/users | admin | 200 |
| admin_audit_after_writes | GET /api/v1/admin/audit | admin | 200 |

Note: "the admin setup views" have no route of their own at v1.1.0 (`admin_views.go` builds the responses of `GET /api/v1/setups` as admin and of the admin setup POST and PUT); there is no `GET /api/v1/admin/setups`. Extra routes beyond the brief (session, me/usage, pressure, audit, refusals) were added because the design's A3 readers use them.

`poll.json` `responses_after_polls` (same process, after the three polls, so the poll view is set): setups_dev, setups_admin, me_holds_dev2, me_usage_dev, me_ledger_dev2, admin_users, admin_collector, all 200.

## poll.json
Three polls at 09:30, 09:35, 09:40Z (fake pressure 0.8, 0.5, 0.95; multipliers 2, 1, 2); the third poll no longer reports `lab-03`, so its pending release completes (hold 3 released). Each poll holds: balances before and after, the exact `Snapshot` the pressure source got, every metering-policy call (hold state, snapshot, decision), raw and smoothed pressure, multiplier, usage samples and ledger rows written, hold states. Debits of poll 1: hold 1 417, hold 2 0, hold 3 667, hold 5 542 milli-tokens.

## What a consuming test must do to load each file
- `hourglass.sqlite3`: `newBackend(t, backendOpts{start: <responses.json now>, loc: Europe/Berlin, dbCopy: <path>, runtime: Runtime{Provider: "reported", PressureSource: "reported", MeteringPolicy: "fake-policy", ReportedProviderMaxAge: 10m, ReportedPressureMaxAge: 15m}})`; name the fake provider and pressure source `reported`; make the fake policy decide as `recPolicy` in the recorder. The `.db` extension is git-ignored, hence `.sqlite3`.
- `responses.json`: replay each `request` for its `principal` on that backend in file order (the writes change the copy, later entries depend on them), and compare `status` and `response`. Users are told apart by email (`dev@example.test`, ...); `facts.user_ids`, `setup_ids`, `hold_ids` give the ids. Service tokens are `hgsa_<id>_<recSecret(name)>`, or a test mints its own.
- `poll.json`: start a second backend on a fresh copy at `start`, then for each poll advance the clock to `at`, set the fake pressure value and allocations, run one poll, and compare the snapshot, policy calls, samples, ledger rows, balances and hold states. WGT-19 requires them identical with reported weights off.
- `openapi.yaml`: for the WGT-20 contract comparison (no route, parameter or field of it removed).

## Decisions for docs/decisions.md
1. The v1.1.0 fixtures are in `testdata/upgrade-v1.1.0/` and follow D78: database, responses, polls, the release's `openapi.yaml`, the recorder as text and a README.
2. Normalised, nothing else: wall-clock and random values the store or auth service stamp (see README) are replaced so that a second run is identical; the event outbox and sessions are emptied.
3. The service tokens' secrets are derived from fixed labels in the recorder so the database and the responses are reproducible; they open nothing but the fixture.
4. Extra routes recorded beyond the brief: session, me/usage, pressure, admin audit, and the refusals for a developer, a user with no role, an anonymous caller and a session on a service route.

## Requirement id -> test
None: this task writes fixtures only. The later tests for WGT-19 and WGT-20 consume them.

## Confidence and gaps
- Is verified: two runs and a clean-worktree run produce identical files; ci.sh exit 0; the recorded numbers match hand calculation (2.5 x 2 x 5/60 h = 417 milli-tokens).
- The compaction (roll-up) of old debit rows is not exercised: the ledger rows are raw. The recorder has no hook to run it and the design does not ask for it.
- A pending release with the setup still deployed, a policy weight and weight 0 are present; a hold before its first debit (metering started in the poll) is not, because all holds are old. Appears sufficient for WGT-19 and WGT-20.
- The fixture was recorded with `Runtime` names `reported` for provider and pressure source because the reports routes answer `not_configured` otherwise; the recorded `GET /api/v1/admin/settings` runtime block therefore shows those names.
- `PROGRESS.md` and `ci.log` are in this folder; `ci.log` is a 81-line summary (bazel outputs cached).
