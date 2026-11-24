# 87 Final independent verification

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first;
its "Done means" is what you are checking). Agent contract:
`.orchestrator/hourglass-v1/CONTRACT.md` ("the spec" means
`docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/87-final-verify/`.

## Goal

Decide whether the branch `ledger-labels-onboarding` at its current HEAD is
ready to be released: every requirement has a passing test named after its id,
`./ci.sh` passes, the end-to-end runs pass against the code of this tree, and
the documents are true. List every defect with a severity and a failure
scenario.

You did not write any of this. Earlier milestone verifications exist (tasks
76, 79, 84); you may read their RESULT.md files to avoid repeating their
deliberate breaks, but do not take authors' RESULT.md files (74, 75, 77, 78,
80-83, 85, 85b, 86a, 86b and the `b` fix tasks) as evidence: check the tree.

## What to check

1. Requirement table. For each of LED-7 to LED-15, UI-8 to UI-11, SET-8,
   AUTH-9, PRE-1 to PRE-8, UPG-1 to UPG-3: the test name(s) carrying the id,
   the file, and that they ran and passed in an uncached run. Write the table
   to `REQUIREMENTS.md` in your task folder (id, requirement in a few words,
   tests, file).
2. Deliberate breaks not yet made by an independent agent: UPG-1, UPG-2,
   UPG-3; the status re-check in the approval transaction (a `TestPRE3_` test
   must go red); the two Go tests task 79b added (paging with `limit` below
   the number of active holds; newest balance of a past period); ASCII-only
   email folding (PRE-2). Restore each; `git diff` empty at the end.
3. Existing tests across the whole branch: `git diff d96a950..HEAD -- '*_test.go' '*.test.tsx' 'e2e/'`
   for files that existed at `d96a950`. Is any assertion weaker than before?
   In particular `e2e/browser/tests/acceptance.spec.ts` (`readLedger` and the
   two steps changed to wait for the page) and
   `e2e/browser/tests/integration.spec.ts` (must be unchanged:
   `git diff d96a950..HEAD -- e2e/browser/tests/integration.spec.ts` empty).
   `docs/SPEC.md` and the design doc must be unchanged since `414b51d`.
4. `./ci.sh` exits 0; then an uncached test run (`bazel test //... --nocache_test_results`
   or the equivalent the repository uses) and report executed counts.
   `tools/check_core_generic.sh` passes. If the UI has a lint script
   (`ui/package.json`), run it or say why it cannot run.
5. End-to-end, on this HEAD: `e2e/run.sh both` and `e2e/run.sh integration`.
   IMPORTANT: two host processes listen on IPv4 localhost on this machine,
   `hourglass` on 127.0.0.1:8080 and `hd` on 127.0.0.1:8091, and they are not
   part of the test. Establish with evidence which server the browser tests
   actually talk to in each mode (the URL the suite uses, the published
   container ports, and a request that only the container built from this tree
   can answer, for example a route or field added on this branch plus the
   container's own log showing the request). If any mode's tests can reach a
   host process instead of the stack, that is a high finding: report it; do
   not kill those processes.
   The owner's `hourglass-tls` compose project is stopped on purpose; do not
   start or touch it, nor the `platform-lab` containers.
6. Documents against the code: `docs/deploying.md` (Upgrade entry, Data
   retention, disk, timezone note, configuration reference, "Pre-approving SSO
   users" with the three statements D6 requires and no leftover `86b:`
   marker), `docs/progress.md`, `README.md`, `docs/ui-notes.md`,
   `deploy/config.sample.yaml`. Re-run the scripted pre-approval commands from
   the guide once against a stack built from this tree (your own compose
   project on free ports; tear it down) and say whether they work as written.
   No real addresses, passwords or secrets anywhere in the diff
   (`git diff d96a950..HEAD`): check the committed fixtures under
   `testdata/upgrade-d96a950/` and the realm file too.
7. `docs/decisions.md` D69 to D79: does any entry contradict the design doc
   or the code? Is any choice visible in the code but not recorded?
8. Compatibility section of the design, bullet by bullet, one line each: met,
   with the evidence.

## Scope

May read: the whole repository. May write: your task folder only. Temporary
breaks must be restored. Must not touch: git state beyond restoring your own
breaks; containers you did not start; host processes.

## Done when

RESULT.md starts with a verdict (ready / not ready), the findings table (id,
severity, file:line, failure scenario, suggested fix), the end-to-end evidence
of which server was tested, exit codes of every run (also as files:
`ci.exit`, `e2e-both.exit`, `e2e-integration.exit`), and the HEAD commit it
all applies to. `REQUIREMENTS.md` exists.

## Long-running steps

`./ci.sh` about 5 minutes; uncached Bazel tests about 10; `e2e/run.sh both` up
to 50 minutes and `integration` up to 25: run each in the background to a log
with a done-marker holding the exit code, and block with one wait loop per
wait (at most three waits per run). Do not run two end-to-end runs at once.
Read exit codes and tails only.

## Report

Follow CONTRACT.md. One final reply of at most ten lines: verdict, counts of
high / medium / low findings, the three exit codes, paths to RESULT.md and
REQUIREMENTS.md.
