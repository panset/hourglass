# Progress

Last updated: 2026-11-20

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

Also worth a look, though not marked: D33 (reference-scenario settings are
not seedable), D34 and D38 (what the end-to-end test does outside the UI),
D35 (the OIDC callback always redirects to `/`), D36 (how the balance
cache is kept).
