# 85 End-to-end scenarios and compatibility tests

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; "the
spec" means `docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/85-e2e-compat/`.

## Goal

Milestone 8, first half: the end-to-end steps the design's Tests section asks
for, and tests named after UPG-1 (from a real old database), UPG-2 and UPG-3
that fail when the requirement is broken. `e2e/run.sh both` and the integration
scenario pass.

## Context

- Read the design doc sections Tests (all the end-to-end bullets),
  Compatibility, Requirements (UPG-1 to UPG-3), then `e2e/README.md`,
  CONTRIBUTING.md and D63 in `docs/decisions.md` (where end-to-end tests may
  act outside the UI).
- Everything else is built and committed (D69 to D76). The acceptance
  scenario's `readLedger` already uses the Every poll view and walks pages
  with Older, but it has NEVER been run in a browser: expect to debug it.
- An earlier verification noted that the scenario's month jump triggers
  storage compaction of the days before the jump.
- A store-level UPG-1 test exists (`internal/store`), built by applying
  migrations 0001-0005. What is missing is a test from a database written by
  the real release code at commit `d96a950`.
- Existing UPG-relevant tests: `internal/api/testdata/led11_pre_change_responses.json`
  (ledger response recorded before the change) and the contract test.
- Ports: the end-to-end stacks use fixed host ports (8081 for the identity
  provider among them). If a port is held by a container you did not start
  (the owner's `hourglass-tls` compose project may be running), do NOT stop
  it: report `blocked` with what holds the port.

## What to build

1. End-to-end steps in `e2e/browser/tests` (in the acceptance scenario unless
   the suite's structure says otherwise):
   - a developer pages the ledger with Older and Newer and switches the view;
   - a developer filters setups with a chip;
   - an SSO user sees "Hello, {name}" in the top bar;
   - a pre-approved user who has never signed in lands on My usage with a
     balance at first sign-in, without seeing the not-authorized page. Add a
     user to `deploy/keycloak/realm-hourglass.json` that no other scenario
     signs in, and pre-approve them through the admin page in the scenario;
   - compaction through the real stack: move the test clock more than 8 days
     past a day with debits, and check that day's ledger shows one row per
     hold with its poll count and that the period's closing balance did not
     change.
   Keep every existing assertion as strong as it is. Do not edit
   `e2e/browser/tests/integration.spec.ts`'s ledger call (leave the file alone
   unless a new step truly belongs there; its `debitsOf` must stay untouched).
2. `TestUPG1_...` from a real old database: produce a SQLite database with
   ledger data using the code at `d96a950` (for example a `git worktree` of
   that commit under your task folder, running its server or its store against
   a temp directory with the simulator/test clock, or a small Go program in
   that worktree) and commit the resulting small database (or a SQL dump of
   it) as test data with a note saying exactly how it was produced. The test
   opens a copy with the new store, starts the app's migration path with the
   same configuration, and checks every balance and per-hold charge is the
   same, before and after compaction. Remove the worktree when done.
3. `TestUPG2_...`: with no pre-approvals and no new request parameters, the
   responses of `GET /api/v1/me/ledger`, `GET /api/v1/setups` and
   `GET /api/v1/session`, and the SSO sign-in of a new user (status, events,
   audit), differ from `d96a950` only by the added fields (`entry_count`,
   `next_cursor`, `label_index`, `display_name`). Record the `d96a950`
   responses from the old code (same worktree) as golden files, as was done
   for the ledger; reuse the ledger one.
4. `TestUPG3_...`: the contract test passes under that name or a test of that
   name asserts that every route, parameter and response field of the
   `d96a950` `api/openapi.yaml` (commit it as test data, or read a recorded
   list) still exists in today's file.

## Scope

May write: `e2e/` (not the ledger call of `integration.spec.ts`),
`deploy/keycloak/realm-hourglass.json`, deploy files the end-to-end stack needs
for the new user (list them), `internal/**/*_test.go` and `testdata/` for the
UPG tests, `BUILD.bazel` files Gazelle updates, your task folder.
Product code under `internal/` or `ui/src/` only to fix a defect an end-to-end
run exposes: make the minimal fix, add or extend a unit test for it, and list
it prominently in RESULT.md.
Must not touch: `docs/` (task 86), the design doc, git state other than your
own worktree. Do not weaken any existing test.

## Done when

- `./ci.sh` exits 0 (record `ci.exit`).
- `e2e/run.sh both` exits 0, and the integration scenario
  (`e2e/run.sh integration`) exits 0; record each exit code in a file and the
  count of passed tests in RESULT.md.
- One deliberate break each for UPG-2 and UPG-3 went red and was restored.
- RESULT.md has the requirement -> test -> file table and the list of
  end-to-end steps added with their test titles.

## Long-running steps

`./ci.sh`: about 5 minutes. `e2e/run.sh <mode>`: 12 to 25 minutes per mode,
needs Docker; `both` can take 50 minutes. These exceed one command's 10-minute
limit: run each in the background writing to a log and a done-marker with the
exit code, and block with a single wait loop per run (at most three waits per
run). Read only exit codes and tails. Iterate on a single spec or mode while
debugging before the full runs.

## Report

Follow CONTRACT.md: PROGRESS.md, RESULT.md with "Decisions for
docs/decisions.md", one final reply of at most ten lines.
