# 77 Ledger API: roll-up view, paging, balances

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; "the
spec" means `docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/77-ledger-api/`.

## Goal

Milestone 4 of the design's "Suggested task order": `GET /api/v1/me/ledger`
gains `rollup`, `limit`, `cursor`, `entry_count` and `next_cursor` exactly as
section A4 says, with tests named after LED-11, LED-12, LED-13 and LED-14 that
fail when the requirement is broken, and with `api/openapi.yaml`, the contract
test and `ui/src/api/schema.gen.ts` updated together.

## Context

- Read the design doc sections "What the code does today" (Ledger), A3, A4,
  Compatibility (API, Numbers), Requirements and Tests before writing code.
  Every sentence of A4 is a requirement: the cursor applies to entries after
  merging, never to stored rows; balances hold across pages; a request with no
  new parameter is answered as today.
- Storage compaction is built and committed (D70, D71 in `docs/decisions.md`;
  `internal/store/ledger_compaction.go`, `LedgerEntry.RollupCount`,
  `DebitMultipliers` already skips rolled-up rows).
- The handler is `listMyLedger` in `internal/api/handlers_user.go`.
- CONTRIBUTING.md says how the generated UI types are regenerated; use that
  command, do not hand-edit the generated file.

## What to build

1. The three parameters, their validation (400 `validation`), `entry_count` on
   every entry, `next_cursor` (null when `limit` is absent or nothing older).
2. The `rollup=day` view: merge every debit row of the same hold and local day
   (configured timezone), raw or rolled up; id and `created_at` of the newest
   merged row; `multiplier` null when `entry_count` > 1; `actor_name` and
   `reason` null. A single raw debit in that view keeps its multiplier.
3. Opaque cursor meaning "entries of this view whose id is lower than this
   position". Choose the simplest encoding and record it.
4. `balance_after_tokens` as A4 defines it, in every view and on every page.
5. Tests:
   - `TestLED11_...`: no new parameter -> one entry per stored row, unpaged,
     newest first, today's fields plus `entry_count` and `next_cursor: null`.
     Compare against a response recorded from the handler's pre-change
     behaviour for the same data (for example a golden built from the existing
     tests' expectations), not only against the new code's own output.
   - `TestLED12_...`: the `rollup=day` response is identical (ids included)
     before and after storage compaction of the same data; `entry_count` and
     null multiplier.
   - `TestLED13_...`: `limit` bounds; walking cursors to the end returns no
     entry twice and every unchanged entry exactly once while new debits are
     appended between requests (including today's entry of an active hold
     moving to the front); every bad `rollup`, `limit`, `cursor` case is 400
     `validation` (cursor without limit, malformed cursor, limit 0, 201, text).
   - `TestLED14_...`: in both views, across page boundaries, each entry's
     balance equals the next older entry's balance plus its own amount, and
     the newest entry's balance equals the period's balance; also after
     compaction and with a grant in the middle of a day.
   - LED-15 at the API level if not already covered: a compacted row shows no
     multiplier in the default view.
6. The existing end-to-end client `e2e/browser/tests/integration.spec.ts` calls
   this route with no parameters; do not edit it. Existing handler and contract
   tests must pass unweakened.

## Scope

May read: the whole repository.
May write: `internal/api/`, `internal/store/` (new read queries only),
`api/openapi.yaml`, `ui/src/api/schema.gen.ts` (generated), `BUILD.bazel` files
Gazelle updates, your task folder. If the generated types make an existing UI
file fail to type-check, make the minimal edit under `ui/src/` and list it; the
ledger page itself is task 78.
Must not touch: `docs/` (including `docs/decisions.md`), `e2e/`, git state.

## Done when

- The tests above pass; `go test ./internal/...` passes; `./ci.sh` exits 0
  (record the exit code in `ci.exit`).
- One deliberate break per requirement LED-11 to LED-14 made its named test go
  red, then restored; RESULT.md says which.
- RESULT.md has the requirement -> test -> file table and three example
  responses' shapes described in words (not pasted JSON walls).

## Long-running steps

`./ci.sh`: allow 10 minutes (timeout 600000), output to `ci.log` in your task
folder, read the exit code and tail only.

## Report

Follow CONTRACT.md: PROGRESS.md as you go, RESULT.md with "Decisions for
docs/decisions.md" (cursor encoding, where merging happens, anything else left
open), one final reply of at most ten lines.
