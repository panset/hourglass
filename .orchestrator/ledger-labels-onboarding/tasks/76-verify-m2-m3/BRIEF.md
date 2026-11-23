# 76 Independent verification of milestones 2 and 3 (ledger roll-up in storage and collector)

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` ("the spec" means
`docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/76-verify-m2-m3/`.

## Goal

Decide, from the code and the requirements alone, whether LED-7, LED-8, LED-9,
LED-10, LED-15 and UPG-1 are met and tested, and list every defect with a
severity (high / medium / low) and a concrete failure scenario.

You did not write this code. Do NOT read the authors' PROGRESS.md or RESULT.md
files under `.orchestrator/ledger-labels-onboarding/tasks/74-*` and `75-*`:
judge the code, not their conclusions.

## Inputs

- Requirements: the design doc, sections "What the code does today" (Ledger),
  A1, A2, A3, Non-goals, Compatibility, Requirements, Tests.
- Code: `git diff 4e8686e..HEAD -- internal/` (commits 6ba2458 and the one
  after it). Decisions recorded: D70 and D71 in `docs/decisions.md`.

## What to check

1. Each requirement above has at least one test named after its id, and the
   test would fail if the requirement were broken. For LED-7, LED-8, LED-9 and
   LED-10 make one small deliberate break in the code each, run the named
   tests, confirm red, and restore (`git diff` must be empty when you finish;
   say so).
2. Correctness of compaction: group definition (user, hold, period, local day
   in the configured timezone, including DST days and a period boundary inside
   a day), the 7-day edge, the kept id and `created_at`, null actor and empty
   reason, one-row groups untouched, idempotence, atomicity on crash or error,
   the marker never committed and never left behind, the trigger still refusing
   everything else, nothing else in the code inserting into the marker table.
3. A3: every reader the design lists returns the same result after compaction
   apart from the merged rows; look for any ledger reader the design or the
   tests missed (grep the store and API for queries on `ledger`), including
   anything that joins on `created_at` or counts rows.
4. LED-10: `runMu` and SQLite's write lock are never held across more than one
   step; a poll can run between steps; a backlog cannot delay a poll past the
   interval; a failed step fails no poll; the idle cadence. Look for races
   (run the collector and ledger tests with `-race`).
5. The search for due groups does not scan the ledger (check the query plan
   yourself on a database built by the tests or a quick script).
6. Existing tests: `git diff 4e8686e..HEAD -- '*_test.go'` shows edits to
   `internal/store/store_test.go`, `migrate0004_test.go` and
   `integration_queries_test.go`. Judge whether any edit weakens what the test
   asserted before.
7. UPG-1: the migration test really starts from a schema-5 database with data.
8. `./ci.sh` passes; report how many tests executed (not only cached).
9. `tools/check_core_generic.sh` passes.

## Scope

May read: the whole repository except the two task folders named above.
May write: your task folder only. Temporary breaks in code must be restored.
Must not touch: git state (no commit, stash, reset, checkout of files other
than restoring your own deliberate breaks with `git checkout -- <file>`).

## Done when

RESULT.md starts with a verdict and a findings table (id, severity, file:line,
failure scenario, suggested fix), followed by the requirement -> test -> "break
made, went red" table. Zero findings is an acceptable answer only if you tried
to break it.

## Long-running steps

`./ci.sh`: allow 10 minutes (timeout 600000), output to `ci.log` in your task
folder, read the exit code and tail only.

## Report

Follow CONTRACT.md. One final reply of at most ten lines: verdict, counts of
high / medium / low findings, path to RESULT.md.
