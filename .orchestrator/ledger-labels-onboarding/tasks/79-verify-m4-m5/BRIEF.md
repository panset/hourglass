# 79 Independent verification of milestones 4 and 5 (ledger API and ledger UI)

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` ("the spec" means
`docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/79-verify-m4-m5/`.

## Goal

Decide, from the code and the requirements alone, whether LED-11, LED-12,
LED-13, LED-14, LED-15 (at the API) and UI-8 are met and tested, and whether
the API change is backward compatible; list every defect with a severity
(high / medium / low) and a concrete failure scenario.

You did not write this code. Do NOT read the authors' PROGRESS.md or RESULT.md
under `.orchestrator/ledger-labels-onboarding/tasks/77-*` and `78-*`.

## Inputs

- Requirements: the design doc, sections A3, A4, A5, Compatibility,
  Requirements, Tests. Every sentence of A4 and A5 is a requirement.
- Code: `git diff 7b06cec..HEAD` (two commits: the API and the UI). Decisions
  recorded: D72 and D73 in `docs/decisions.md`.

## What to check

1. Each requirement has at least one test named after its id that would fail
   if the requirement were broken. Make one deliberate break each for LED-11,
   LED-12, LED-13, LED-14 and UI-8, confirm red, restore (`git diff` empty at
   the end; say so).
2. A4 in detail: the cursor is applied after merging; walking pages while
   debits are appended returns nothing twice and every unchanged entry once
   (write your own scratch test if the existing one is thin: an active hold
   polled between page requests, limit smaller than the number of holds, a
   grant mid-day, a period with rolled-up and raw rows mixed, a local-day
   boundary in a non-UTC zone); balances across page boundaries; the newest
   entry's balance equals the period's balance; identical `rollup=day`
   response before and after storage compaction, ids included.
3. Backward compatibility: a request with no new parameter gives the same
   entries, order and fields as commit d96a950 plus `entry_count` and
   `next_cursor: null`. Check the recorded pre-change fixture
   (`internal/api/testdata/`) really comes from pre-change behaviour (compare
   with what `git show d96a950:internal/api/handlers_user.go` would produce,
   by reasoning or by running the old handler in a scratch worktree under
   your task folder... if you make a worktree, remove it when done).
   `api/openapi.yaml`: no field, parameter or route removed or renamed
   (`git diff d96a950..HEAD -- api/openapi.yaml`); the contract test passes
   and covers the new parameters and fields; the generated types match
   (regenerate and diff).
4. Existing tests edited (`internal/api/contract_test.go`, `user_test.go`,
   `ui/src/pages/usage/usage.test.tsx`, `e2e/browser/tests/acceptance.spec.ts`):
   is any assertion weaker than before? `integration.spec.ts` must be
   untouched.
5. A5 in detail, by reading the page code and its tests: 20 rows; default and
   switched request; Newer/Older states; visited cursors dropped on month or
   view change; the 60-second poll and the reload after a release refetch the
   page being shown (not the first page); multiplier and "N polls" detail; the
   UI computes no field (UI-1).
6. Run the browser end-to-end acceptance scenario once, bundled mode:
   `e2e/run.sh bundled` (needs Docker; see `e2e/README.md`). It exercises the
   changed `readLedger`. Report pass/fail and, on failure, whether the cause is
   this change. If Docker is unavailable say so; do not skip silently.
7. `./ci.sh` passes; report how many tests executed.

## Scope

May read: the whole repository except the two task folders named above.
May write: your task folder only. Temporary breaks must be restored.
Must not touch: git state beyond restoring your own breaks and removing your
own scratch worktree.

## Done when

RESULT.md starts with a verdict and a findings table (id, severity, file:line,
failure scenario, suggested fix), then the requirement -> test -> "break made,
went red" table, then the end-to-end outcome.

## Long-running steps

`./ci.sh`: about 5 minutes, allow 10 (timeout 600000). `e2e/run.sh bundled`:
12 to 25 minutes; it may exceed one command's 10-minute limit, so run it in
the background writing to `e2e.log` and a done-marker with its exit code, and
block on it with a single wait loop (repeat that one wait at most three times).
Read only exit codes and tails.

## Report

Follow CONTRACT.md. One final reply of at most ten lines: verdict, counts of
high / medium / low findings, e2e outcome, path to RESULT.md.
