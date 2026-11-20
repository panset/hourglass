# 71 Final verification against the design's Requirements

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first; its "Done means" is your
checklist).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/71-final-verify/`.

## Goal
An independent final judgement, from a clean clone, of whether the job is done as the owner
defined it. You wrote none of this. You may read earlier *verification* results
(`62-*`, `62c-*`, `66-*`, `69-*`) to avoid repeating their mutation work, but not the build
tasks' RESULT.md files; judge the code and documents themselves.

## How
1. Clean clone: `git clone` the repository's current HEAD (local path) into a scratch
   directory outside the repo, and run `./ci.sh` there. Confirm the working tree of the real
   repo is clean apart from untracked logs first; if it is not, say what is uncommitted.
2. Requirement coverage table: for every id in the design's Requirements section (SVC-1..8,
   INT-1..8, HOLD-6..9, CMP-1, CMP-2): the test(s) named after the id (`path:line`), whether
   the test ran and passed in your run, and whether it was mutation-checked by an earlier
   verification or by you. Mutation-check yourself, in the scratch clone, the tests added
   after the earlier verifications: the INT-5 pressure-staleness test, the stored-report
   wiring test in `internal/app`, the double-release / double-event guard tests, the
   mixed-extension collector tests, and the event-ordering fix of D60 (read
   `internal/holds/holds.go` around the `hold.releasing` enqueue: is the in-transaction
   enqueue correct, what happens if the commit fails, can the event be lost or duplicated).
   Any id without a test named after it is a finding.
3. Owner decisions in INTENT.md: confirm each holds in the code (no pull model; service
   tokens only under `/api/v1/integration/`; no role; holds:release; 202 + releasing +
   completion at the first successful poll; state includes username, email, name; D40: grep
   for any reintroduced resource name, template or override).
4. Rules: `docs/SPEC.md` unchanged since `5808957` (`git diff 5808957 -- docs/SPEC.md` empty)
   and `docs/design/integration-api.md` unchanged since then; `tools/check_core_generic.sh`
   passes; `pkg/hourglass/types.go` extension interfaces unchanged; nothing pushed
   (`git status -sb` shows the branch ahead of origin; no history rewritten: `5808957` and
   `fd948a5` are ancestors of HEAD); each milestone commit message cites requirement ids.
5. `docs/decisions.md` D42 onward: every row consistent with the design and with the code;
   D42 lists the spec-source edits the design's Documentation deliverables item 2 asks for;
   the behaviour change is marked REVIEW.
6. Documents: `docs/integrating.md` has no draft banner; bring the `reported` stack up as
   `e2e/README.md` describes and re-run a sample of at least eight of the guide's examples
   yourself (both PUTs, state, service release, and four error rows), comparing with the
   guide's text; check `docs/extending.md`, `docs/deploying.md` (Upgrade entry for migration
   0005 and the changed release behaviour), `README.md` and `docs/progress.md` against the
   design's Documentation deliverables; grep the docs for leftover `still_deployed` or
   statements the code contradicts. Tear the stack down.
7. OpenAPI: every new route and field present, `x-roles: service` on the four integration
   routes, contract test covers them.

## Scope
May write: only your task folder and a scratch directory outside the repository. Must not
touch git state or any tracked file.

## Done when
`REPORT.md` in your task folder holds: the coverage table; the result of each numbered check;
a numbered list of findings with severity (high / medium / low), evidence, failure scenario
and suggested fix; and "not verified" items. RESULT.md is a ten-line summary pointing at it.

## Long-running steps
`./ci.sh` from a clean clone rebuilds everything: allow 40 minutes. Bringing the stack up:
several minutes. Foreground, generous timeout, log to your task folder, read only exit code
and tail.

## Report
Follow the Reporting section of the contract; state the count of findings by severity.
