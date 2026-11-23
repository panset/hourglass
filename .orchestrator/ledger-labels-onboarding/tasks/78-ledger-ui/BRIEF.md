# 78 Ledger UI on My usage, and the acceptance scenario's ledger helper

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; "the
spec" means `docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/78-ledger-ui/`.

## Goal

Milestone 5 of the design's "Suggested task order": the Ledger table on My
usage as section A5 says (UI-8), with tests named after UI-8 that fail when the
requirement is broken, and `readLedger` in
`e2e/browser/tests/acceptance.spec.ts` changed to use the Every poll view and
walk the pages with Older.

## Context

- Read the design doc sections A4, A5, Requirements (UI-8) and Tests (the
  end-to-end bullets) first, then `docs/ui-notes.md` and CONTRIBUTING.md for
  the UI's conventions and how UI tests are named after requirement ids.
- The API is built and committed (D72 in `docs/decisions.md`). The response
  entries are the OpenAPI schema `LedgerViewEntry`; the UI's `LedgerEntry`
  alias in `ui/src/pages/usage/holdApi.ts` still points at `LedgerEntry` and
  should point at `LedgerViewEntry` to read `entry_count`.
- A cursor does not encode the view or period, so the page must drop its
  visited cursors when the month or the view changes.

## What to build

Everything in A5: 20 rows per page; the By day (default, `rollup=day&limit=20`)
/ Every poll (`limit=20`) switch; Newer and Older with the stated disabled
states, keeping visited cursors client-side; month or view change returns to
the first page; the 60-second poll and the reload after a release refetch the
page being shown; the Type cell shows the multiplier only when not null; the
Detail cell shows "lab-06 · 288 polls" when `entry_count` > 1. The UI formats
fields and computes none (UI-1).

Tests, named after UI-8 in the repository's convention: at most 20 rows; the
default request and the switched request (assert the query strings); Older
follows `next_cursor`, Newer returns to the page before; both disabled at the
ends; month change and view change go back to the first page and drop cursors;
the refetch keeps the current page; the poll-count detail text.

`readLedger` in the acceptance scenario: switch to Every poll and walk every
page with Older, collecting rows, so its existing assertions on single debits
and their multipliers ("Debit · 2×") stay exactly as strong. Do not replace
them with API calls and do not change the assertions that consume its result.
Note that the scenario's month jump moves the test clock, after which the
collector compacts earlier days: check where `readLedger` is called relative to
the jump and say in RESULT.md whether any existing assertion would now read
compacted days. If one would, that is a finding to report, not something to
paper over. You are not asked to run the end-to-end suite (task 85 does), but
the spec must type-check and lint.

## Scope

May read: the whole repository.
May write: `ui/src/pages/usage/`, shared UI components if the page needs one
(list them), `ui/` test files, `e2e/browser/tests/acceptance.spec.ts`
(`readLedger` and what it needs only), `docs/ui-notes.md`, your task folder.
Must not touch: `e2e/browser/tests/integration.spec.ts`, `internal/`, `api/`,
`ui/src/api/schema.gen.ts`, `docs/decisions.md`, the design doc, git state.
Do not weaken an existing test.

## Done when

- UI unit tests and type-check pass; `./ci.sh` exits 0 (record `ci.exit`).
- One deliberate break made a UI-8 test go red, then restored.
- RESULT.md has the requirement -> test -> file table.

## Long-running steps

`./ci.sh`: allow 10 minutes (timeout 600000), output to `ci.log` in your task
folder, read the exit code and tail only.

## Report

Follow CONTRACT.md: PROGRESS.md, RESULT.md with "Decisions for
docs/decisions.md", one final reply of at most ten lines.
