# 75 Ledger service and collector: bounded compaction

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; "the
spec" means `docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/75-ledger-collector/`.

## Goal

Milestone 3 of the design's "Suggested task order": `internal/ledger.Service`
gains the compaction operation and the collector runs it in bounded steps,
with tests named after LED-10 that fail when the requirement is broken, and the
scale test.

## Context

- Read the design doc sections "What the code does today" (Ledger), A1 (all the
  Rules), Compatibility, Requirements (LED-10) and Tests before writing code.
- Task 74 built the store half and it is committed: read
  `.orchestrator/ledger-labels-onboarding/tasks/74-store-compaction/RESULT.md`
  for `DueLedgerGroups`, `CompactLedgerGroup`, `LedgerCompactionStepRows`
  (2,000 rows) and the resume position. D70 in `docs/decisions.md` records the
  choices.

## What to build

1. `ledger.Service` operation (for example `CompactDue`): one bounded step in
   one transaction; returns rows removed and whether more is due. The ledger
   service stays the only writer of the ledger; it does not go through
   `AppendLedger` and does not touch `users.balance_mt`.
2. Collector: calls it as it calls `RolloverDue`, but never holding `runMu`
   across more than one step; at most a fixed number of steps per cycle; a
   backlog is worked off over following cycles; once nothing is due the check
   runs once a day with the other pruning (`pruneIfDue`). A failed step is
   logged, retried on a later cycle, and never fails a poll. Log rows removed
   when above zero, as the prune steps do. The first start after upgrade on a
   database with months of history is the case to design for: no poll may be
   delayed past the poll interval (MET-4 treats a poll delayed past twice the
   interval as an outage).
3. Tests named `TestLED10_...`: steps are bounded and the lock is released
   between them (a poll can run between two steps of a backlog); a backlog
   larger than one cycle's step budget finishes over later cycles; a failed
   step fails no poll and is retried; running again when nothing is due changes
   nothing; the daily cadence once idle.
4. Scale test beside `internal/api/perf_scale_test.go` (follow its conventions,
   including how it is tagged or sized for Bazel): a year of 5-minute debits
   for several holds compacts with every step well under one second; finding
   due groups does not scan the ledger (assert on `EXPLAIN QUERY PLAN` or an
   equivalent that would fail without the index); a ledger page read
   (`GET /api/v1/me/ledger` as it is today) stays fast before and after.

## Scope

May read: the whole repository.
May write: `internal/ledger/`, `internal/collector/`, the new scale test file
under `internal/api/`, `BUILD.bazel` files Gazelle updates, your task folder.
`internal/store/` only if task 74's functions need a small change; list it.
Must not touch: `docs/SPEC.md`, the design doc, `docs/decisions.md`, the ledger
HTTP handler (task 77), `api/openapi.yaml`, `ui/`, `e2e/`, git state.
Do not weaken or delete any existing test. If compaction turned on in the
collector makes an existing test fail, work out whether the test or the code is
right; a contradiction with the design is a reason to stop and report.

## Done when

- The tests above exist and pass; `go test ./internal/...` passes.
- `./ci.sh` exits 0 (run it once at the end; record the exit code in `ci.exit`).
- One deliberate break made a `TestLED10_` test go red, then restored; RESULT.md
  says which.
- RESULT.md gives the measured step times and the steps-per-cycle number, and a
  table: requirement id -> test name -> file.

## Long-running steps

`./ci.sh`: allow 10 minutes (timeout 600000), output to `ci.log` in your task
folder, read only the exit code and tail. The scale test may take a minute or
more to build its data.

## Report

Follow CONTRACT.md: PROGRESS.md as you go, RESULT.md with "Decisions for
docs/decisions.md" (steps per cycle, cadence, anything else left open), one
final reply of at most ten lines.
