# Progress

Last updated: 2026-11-23

Status of Hourglass v1 against the spec in [SPEC.md](SPEC.md). Decisions are
in [decisions.md](decisions.md). The task-by-task record (briefs, results,
verification reports) is under `.orchestrator/hourglass-v1/`.

## Summary

All ten milestones are built, verified by agents that did not write the
code, and pushed. The four "done" criteria hold:

| Criterion | Status | Evidence |
|---|---|---|
| From a clean checkout, `bazel build //...` and `bazel test //...` pass, and plain `go build ./...` gives an API-only binary | Yes | Fresh clone of the pushed repository: 254 targets built, 35 of 35 test targets pass; the `go build` binary answers `/healthz` and serves no UI. `./ci.sh` passes without `node_modules`. |
| The spec's acceptance path passes end to end in bundled and split mode | Yes | `e2e/run.sh both`: all eight acceptance steps pass through the real UI in a headless browser in each mode (about 12 minutes per mode). |
| HTTP tests cover every cell of the permissions table, including that a user with no role gets no data from any route | Yes | `internal/api/permissions_test.go`: every v1 cell of the table, and all 30 routes for each of 8 kinds of caller (240 combinations). A deliberately seeded leak turned the suite red. |
| Everything is pushed and this file is current | Yes | `main` equals `origin/main` on `github.com/panset/hourglass`. |

Nothing has been run against a real vCenter. See "First run against a real
vCenter" below.

## Changes to the spec made at the owner's direction (2026-11-20)

These two changes were requested after v1 was complete. They are built,
independently verified and pushed. `./ci.sh` passes on them (35 of 35 Bazel
test targets) and the end-to-end acceptance test passes on them in both
bundled and split mode.

- **D40:** the core no longer has a resource-name template or a per-setup
  override (supersedes SET-4). A provider's allocation belongs to a setup
  when its reported name equals the setup's name. The `vsphere` extension
  takes `pool_name_prefix` and `pool_name_suffix` in its own config; the
  reference scenario uses suffix `_RP`. Migration 0004 drops the column.
  Renaming a setup while it is held is refused.
- **D41:** admins land on Setups after sign-in; My usage moves from `/` to
  `/usage`.

`docs/SPEC.md` is a copy of the source document and still shows the old
text for both; the source needs the same edits.

## The integration API (owner-directed, 2026-11-20)

Built to [design/integration-api.md](design/integration-api.md) (D42 to D63). The task record is under `.orchestrator/integration-api/`. `docs/SPEC.md` is not edited; D42 lists the edits the spec's source needs.

**Finished**

- Service accounts: admin routes and UI page, scoped bearer tokens (`hgsa_<id>_<secret>`, hash stored, shown once), expiry, revoke, audit, a failed-attempt rate limit per client address (SVC-1 to SVC-8).
- `PUT /api/v1/integration/deployments` and `/pressure`, `GET /api/v1/integration/state`, `POST /api/v1/integration/holds/{id}/release`, in `api/openapi.yaml` (INT-1 to INT-4, INT-8, HOLD-8).
- The `reported` provider and pressure source, with `max_age`, and the public `pkg/storedreports` they read through (INT-5 to INT-7). `check-provider` prints report ages.
- Pending release: a user's release of a still-reported setup is accepted, marked Releasing, fires `hold.releasing`, and completes at the first poll where the setup is gone (HOLD-6, HOLD-7, HOLD-9, CMP-1, CMP-2). `still_deployed` is gone (D43, D59).
- UI: Service accounts page, Releasing state, last report times on Settings.
- An end-to-end scenario with a scripted reporter and webhook receiver (`e2e/run.sh integration`, 10 of 10 steps, about 13 minutes) and the acceptance path updated for the new release.
- Documentation: `docs/integrating.md` was run example by example against a live `reported` stack and corrected; `extending.md`, `deploying.md` (config, `max_age`, service accounts, proxy note, upgrade entry for migration 0005), the README and `ui-notes.md` describe what was built. Three verification tasks (after service accounts, after the release changes, after the end-to-end scenario) each found issues that were fixed and re-checked.

**Not finished, not verified, or left as is**

- Nothing in this work has been run against a real vCenter, and the `reported` extensions have only been driven by scripted reporters.
- The rate limit counts failures per connection address and does not read `X-Forwarded-For`, so behind a proxy all callers share one failure budget; valid tokens are never refused (D47).
- Events stay at most once and in memory: a receiver that is down misses `hold.acquired` and `hold.releasing` for good. The state route is how a service catches up (documented in the guide).
- If no poll has succeeded since the process started, a user's release is immediate even when the setup is deployed (D61).
- OIDC users have an empty `username` in event JSON and in the state response; the email is the stable key.
- The integration end-to-end scenario is not part of `bazel test //...` (D63).
- `docs/SPEC.md` and the spec's source still describe the old HOLD-2 and lack the integration requirements (D42).

## Ledger roll-up, labels, greeting, pre-approved users (owner-directed, 2026-11-23)

Built on the branch `ledger-labels-onboarding`, cut from `main` at `d96a950`, to [design/ledger-labels-onboarding.md](design/ledger-labels-onboarding.md) (D69 to D78). The task record is under `.orchestrator/ledger-labels-onboarding/`. Nothing here is merged to `main`, tagged or released: that waits for the owner's go-ahead and version number. `docs/SPEC.md` is not edited; the edits its source needs are listed in the job report and are to be recorded in `decisions.md` by the orchestrator.

**Built**

- Ledger roll-up: migration `0006` (a `rollup_count` column, a marker table, a delete trigger that allows only the compaction of debit rows, two partial indexes) and a bounded daily compaction step run by the collector: per-poll debits of days earlier than seven days before today become one row per user, hold and day, with the same total, id order and balances (LED-7 to LED-10, LED-15; D70, D71).
- `GET /api/v1/me/ledger`: `rollup=day`, `limit`, `cursor`, `entry_count`, `next_cursor`; without the new parameters it answers as before (LED-11 to LED-14; D72). My usage pages the ledger 20 rows at a time, By day or Every poll (UI-8; D73).
- Label discovery: `label_index` on `GET /api/v1/setups` and "Available labels" chips on Setups (SET-8, UI-9; D74).
- Greeting: name fallback claims at sign-in, `display_name` in the session and "Hello, {name}" in the top bar (AUTH-9, UI-10; D75).
- Pre-approved SSO users: migration `0007`, the sign-in step, three admin routes, the optional `preapproved_users_file`, and the Pre-approved users section on Users (PRE-1 to PRE-8, UI-11; D76, D77).
- Compatibility tests against fixtures recorded from the previous release (`testdata/upgrade-d96a950/`) (UPG-1 to UPG-3; D78).
- Independent verification ran after milestones 3, 5 and 7 and its findings were fixed in their own commits.
- Documentation: `docs/deploying.md` (upgrade entry for migrations 0006 and 0007, data retention and disk space, the timezone note, the configuration reference, the "Pre-approving SSO users" section), `docs/ui-notes.md`, `README.md`, `deploy/README.md` (a fourth development user, `dave`) and `CONTRIBUTING.md` (the compaction scale test and the upgrade fixtures).

**Not finished, not verified, or left as is**

- The commands in "Pre-approving SSO users" in `docs/deploying.md` (the file and the scripted way) are marked `<!-- 86b: run and correct -->`: they were written from the code and have not been run against the bundled stack. The job is not done until they have been.
- The new end-to-end steps (A2g, A2f, A2p, A7g, A8p, A8c) type-check and list but had not run in a browser when their task finished, because the owner's `hourglass-tls` project held ports 8081 and 8990. `e2e/run.sh both` and `e2e/run.sh integration` have to pass before a release.
- The roll-up window (seven days) is fixed. Compacted rows cannot be expanded again, and an older binary refuses the upgraded database; the way back is the backup taken before the upgrade.
- Disk: the file stops growing at the old rate but does not shrink until an offline `VACUUM`.
- The release itself (merge, version number, tag, image workflow) has not been done. No version tag exists yet, and `internal/version` has not been checked for what the released binary reports.
- `docs/SPEC.md` and the spec's source still lack the new requirements and table rows.
- `docs/extending.md` still says `user.approved` is emitted when an admin assigns a first role; it is now also emitted for a pre-approved sign-in, with the same payload.

## Milestones

| # | Milestone | Status |
|---|---|---|
| 0 | Repository: license, README stub, .gitignore, spec copy | Done |
| - | Spike: govmomi simulator resource pools and cluster figures | Done (D5 to D8) |
| 1 | Build skeleton: Go module, Bazel bzlmod, rules_go, Gazelle, CI script | Verified |
| 2 | Pure core logic: selectors and rate rules, bands and smoothing, debits, ledger and rollover | Verified |
| 3 | SQLite store and migrations | Verified |
| 4 | Extension API, fakes, conformance suites, `inventory`, `default`, `static`, `webhook` | Verified |
| 5 | Collector and event queue (plus settings, ledger service, hold rules) | Verified |
| 6 | Auth and JSON API, `api/openapi.yaml`, contract test | Verified |
| 7 | React + TypeScript UI | Verified |
| 8 | vSphere provider and pressure source, `check-provider` | Verified |
| 9 | Images, compose stacks, end-to-end acceptance path in both modes | Verified |
| 10 | README quick start, `docs/extending.md`, `docs/deploying.md`, `CONTRIBUTING.md`, `examples/custom` | Verified |

## What verification found and what was done about it

Each milestone was checked by a separate agent working in a clean clone.
The findings that mattered:

- **Store:** ledger rows could be rewritten with `INSERT OR REPLACE`; a
  panic inside a transaction left the write lock held. Both fixed.
- **Metering:** a clock stepping backwards could double-charge; a poll at
  exactly midnight on the 1st could run ahead of the rollover. Both fixed.
- **Security:** an admin could save a resource-name template that hung the
  collector (the template has since been removed, D40); a route registered outside the route table would have escaped
  the permissions tests; rule changes were audited without before and after
  values; deleting every setup would re-apply the seed. All fixed. No
  critical or high findings; a user with no role got no data from any
  route under attack, and planted secrets never appeared in logs, responses
  or the database.
- **Performance:** the admin Users list took about 0.6 s at spec scale
  (200 users, 100 setups, 18 days of 5-minute polls) against the 0.5 s
  limit. Fixed: every route now answers in under 0.5 s on that data (the
  Users list in about 0.15 s) and a poll takes about 0.2 s. A scale test
  guards it.
- **Balance cache:** the first version of that performance fix let the
  cached balance drift after a rollover that closed two or more months.
  Verification caught it before it was pushed. Fixed and re-verified with
  200 randomised operation sequences: the cached balance equalled the
  ledger after every step. The ledger itself was never wrong.
- **UI:** a failed page load left a blank page; side panels did not take
  keyboard focus. Both fixed. The pages were viewed in a real browser at
  two desktop widths with no clipped, overlapping or missing content.

## Not finished, not verified, or left as is

- **Real vCenter:** nothing has been run against one. All vSphere code is
  tested against a hand-written fake and the govmomi simulator only.
- **Stray binary in history:** a compiled 32 MB `simulator` binary was
  committed by mistake in `b349a63` and removed in `74c76ba`. It is still in
  the pushed history; removing it needs a history rewrite (D39).
- **Commit size:** several commits are milestone-sized (thousands of lines)
  rather than small.
- **Admin lockout:** the local admin sign-in limit is per account, so
  anyone who can reach the sign-in route can keep it locked out (D31).
- **Debt and role removal:** debt is not carried for a user who has no role
  at the moment of a rollover (D23).
- **Two processes on one database:** two rollovers running at the same
  instant could write a carry-over twice. The spec is single-instance and
  the process serialises its own rollovers, so this needs two processes on
  one database file, which `docs/deploying.md` says not to do.
- **Reference scenario settings:** the `size` display column is an admin
  setting, set once in Settings after first start; the seed file covers
  setups and rate rules only (D33).
- **Not covered:** mobile screen widths; accessibility beyond keyboard
  focus in dialogs and panels; a browser download quirk on this machine
  (Playwright's Chromium install can hang; `e2e/README.md` has the
  workaround).
- **Out of scope by the spec:** deploying or tearing down anything, the
  manager role and team budgets, notifications, token transfers.

## First run against a real vCenter

Mocks cannot settle these (from the spec, with what was learned):

- [ ] The read-only vCenter role has every privilege the provider needs
      (the built-in Read-only role appears sufficient; inferred, not run).
- [ ] The real resource pool names match `pool_name_prefix` /
      `pool_name_suffix` in the vsphere config. Clusters inside
      host sub-folders are not discovered (D27).
- [ ] The pressure source's cluster numbers agree with what vCenter shows.
      They are read as reservations, not live usage (D6).
- [ ] TLS and certificate settings work against the real endpoint with
      `insecure_skip_verify: false`.
- [ ] A poll finishes well inside the poll interval at real inventory size.

`hourglassd check-provider` supports that run.

## Decisions to review

All decisions are in [decisions.md](decisions.md). The ones marked
**REVIEW** there:

| Decision | In one line |
|---|---|
| D3 | Orchestration files (`.orchestrator/`) are tracked in this public repository. |
| D6 | vSphere "allocation" is read as reservations. |
| D8 | The local stacks use a simulator image built from this repository. |
| D10 | An `outages` table was added to the spec's 15 tables. |
| D11 | Band lower bounds are inclusive (exactly 50% gives 1x). |
| D12 | A gap longer than twice the poll interval is not charged at all. |
| D13 | Label values with spaces or punctuation cannot be named in a rule. |
| D19 | Charging starts at the poll after metering starts; time between the last poll and a release is not charged. |
| D22 | A debit is booked in the month of its poll. |
| D23 | Debt is not carried for a user with no role at rollover. |
| D27 | Clusters inside vCenter folders are not discovered. |
| D28 | Third-party builds import `pkg/hourglass/server` as well as `pkg/hourglass`. |
| D29 | The end-to-end run uses a test main with a movable clock. |
| D30 | Three build- or test-only npm packages with licenses outside the spec's list are allowlisted. |
| D31 | The local admin sign-in limit is per account. |
| D39 | The stray binary stays in history unless you ask for a rewrite. |
| D40 | Owner-directed: resource naming moved out of the core into the provider. |
| D41 | Owner-directed: admins land on Setups; My usage is at `/usage`. |
| D43 | Owner-directed: a user's release of a still-deployed setup is accepted and pending, not refused (`still_deployed` is gone). |
| D60 | `hold.releasing` is queued inside the transaction; three residual cases (failed commit, webhook before the state shows it, owner lookup failure) are covered by idempotent handlers and reconciliation. |
| D47 | Failed service-token attempts are limited per connection address; a valid token is never refused; behind a proxy all callers share one failure budget. |
| D61 | A release made with no successful poll since start-up is immediate even if the setup is deployed; release is decided from the last poll, not the stored report. |

Also worth a look, though not marked: D33 (reference-scenario settings are
not seedable), D34 and D38 (what the end-to-end test does outside the UI),
D35 (the OIDC callback always redirects to `/`), D36 (how the balance
cache is kept), D42 (the integration API and the spec-source edits it needs), D47 (the rate limit behind a proxy), D44 to D46 (the stored-report rules), D48 (service-account
naming and token format), D50 (a deleted account's name can be reused), D51
to D55 (how the core learns about `reported` and how the extension reads the
reports), D57 and D58 (repeat release clicks; the completing poll is still
charged), D60 (`hold.releasing` is queued inside the transaction).
