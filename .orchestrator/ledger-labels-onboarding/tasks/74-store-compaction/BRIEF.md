# 74 Migration 0006 and store: ledger compaction at the storage level

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; "the
spec" means `docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/74-store-compaction/`.

## Goal

Milestone 2 of the design's "Suggested task order": the storage half of the
ledger roll-up, with tests named after LED-7, LED-8, LED-9, LED-15 and UPG-1
that fail when the requirement is broken.

## Context

- Read the design doc sections "What the code does today" (Ledger), A1, A2, A3,
  Compatibility, Requirements and Tests in full before writing code. They name
  the files and functions.
- The branch is `ledger-labels-onboarding`; the working tree is clean apart
  from D69 in `docs/decisions.md`.
- The next task (75) adds `ledger.Service.CompactDue` and the collector loop on
  top of what you build. `internal/ledger.Service` must stay the only writer of
  the ledger, so shape the store functions to be called from there inside its
  transaction (follow how the existing ledger service calls store `...Tx`
  functions). Do not wire the collector.

## What to build

1. Migration `0006`: `ledger.rollup_count`, the replaced `ledger_no_delete`
   trigger with the one-row marker table (A2), and an index that lets "find the
   next due groups" avoid scanning the whole ledger (A1; a partial index over
   raw debits by `created_at` is the suggested shape). The `preapprovals` table
   is NOT part of this task; say in RESULT.md that 0006 holds the ledger change
   only so task 82 adds `0007` (this will be recorded as a decision).
2. Store: `LedgerEntry.RollupCount`, `ledgerCols`, `scanLedger`; the
   `DebitMultipliers` filter (A3); the queries a bounded compaction step needs:
   find due groups up to a row bound given `today` and the timezone (the local
   day cannot be computed in SQLite for a named zone: group in Go or pass day
   boundaries), and compact one group inside a caller's transaction (set
   marker, delete the group's rows, insert the one row with the newest id,
   clear marker). Rows with no `hold_id`, non-debit rows, already rolled-up
   rows and one-row groups are never touched.
3. Tests (names carry the ids, e.g. `TestLED7_...`):
   - LED-7: what is merged and what is never changed, including the 7-day edge
     (`day < today - 7`) in a non-UTC timezone and a period boundary inside a
     day.
   - LED-8: the before-and-after reader test the design asks for: two periods,
     three holds, a grant in the middle of a day; snapshot every reader listed
     in "What the code does today" (`SumLedger`, `RebuildBalance`,
     `RebuildBalances`, `HoldCharges`, `DebitEntries`, `LedgerPeriods`,
     `CountLedger`, `LatestAllowancePeriod`, `BalanceThrough`,
     `LatestLedgerEntry`, `ListLedger`, the admin totals in
     `apiadmin_queries.go`), compact, compare. Where a reader legitimately
     differs only by the merged rows themselves (`ListLedger`, `CountLedger`,
     `DebitEntries` if it is per row), assert exactly the expected difference,
     not "anything goes". Also `users.balance_mt` untouched, and totals per
     user/period/type, per hold, per user and local day.
   - LED-9: every case in A2's test list, including "marker table empty after
     a failed step" (force a failure inside the step).
   - LED-15: a rolled-up row gets no multiplier from `DebitMultipliers`.
   - UPG-1 migration test: a database at schema 5 with ledger data (build it by
     applying migrations 0001-0005 only, or a fixture made that way), opened by
     the new store; every balance and per-hold charge the same afterwards, also
     after compaction.
4. If any existing reader would give a different answer after compaction other
   than as A3 describes, that contradicts the design: stop and report it.

## Scope

May read: the whole repository.
May write: `internal/store/` (including `migrations/`), your task folder. If a
compile fix is unavoidable elsewhere because `LedgerEntry` gained a field, make
the minimal edit and list it.
Must not touch: `docs/SPEC.md`, the design doc, `docs/decisions.md`,
`api/openapi.yaml`, `ui/`, `e2e/`, git state (no commit, stash, reset, checkout).
Do not weaken or delete any existing test. If an existing test asserts that
every ledger delete fails, keep it passing as written (it should).

## Done when

- The tests above exist and pass; `go test ./internal/...` passes.
- `./ci.sh` exits 0 (run it once at the end).
- For LED-7, LED-9 and LED-15 you made one deliberate break each, saw the named
  test go red, and restored the code; RESULT.md says which break and which test.
- RESULT.md has a table: requirement id -> test name -> file.

## Long-running steps

`./ci.sh` runs go build, vet, Gazelle check, `bazel build //...` and
`bazel test //...`; allow 10 minutes (timeout 600000), send output to
`ci.log` in your task folder and read only the exit code and tail. New Go files
may need `bazel run //:gazelle` to update `BUILD.bazel` (see CONTRIBUTING.md).

## Report

Follow the reporting rules in CONTRACT.md: PROGRESS.md as you go, RESULT.md at
the end with a "Decisions for docs/decisions.md" section (step bound chosen,
index shape, marker table name, anything else the design left open), and one
final reply of at most ten lines.
