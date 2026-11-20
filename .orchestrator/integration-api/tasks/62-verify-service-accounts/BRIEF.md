# 62 Independent verification: data layer and service accounts

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/62-verify-service-accounts/`.

## Goal
An independent judgement, from the code and the requirements only, of whether the data layer
and service accounts meet the design. You did not write this code. Do **not** read
`.orchestrator/integration-api/tasks/60-*/` or `tasks/61-*/` (briefs, PROGRESS, RESULT): the
authors' conclusions must not shape yours.

## What to check against
`docs/design/integration-api.md`: sections "Service accounts", "Rules shared by both report
routes", "Data model", and requirements SVC-1 to SVC-8, INT-2, INT-3, INT-6 (INT-1's value
validation where it exists in `internal/reported`). The four integration routes are expected to
be registered with authentication and scope checks but to answer 501 for now; their behaviour
is a later task and is out of scope, their authentication is in scope.

The code: commit `d03efa0` (data layer) plus the uncommitted working tree (service accounts).
Use `git diff 5808957` and `git status` to see everything that changed since the design commit.

## How
1. Run the tests yourself: `go test -race -count=1 ./internal/...`, and `./ci.sh`. Report how
   many tests executed, not only that they passed.
2. For each requirement id above: name the test(s) that cover it, read them, and judge whether
   the test would fail if the requirement were broken. For at least SVC-1, SVC-2, SVC-3, SVC-5,
   INT-2 and INT-3, make one small deliberate break in the production code, confirm a test named
   for that id goes red, and restore the code exactly (`git diff` must be unchanged afterwards;
   check with `git stash`-free means, e.g. compare `git diff | shasum` before and after).
3. Security read of the token path, trying to break it: token generation entropy and source;
   what is stored; constant-time comparison and whether a timing or error-message difference
   reveals whether a token id exists; parsing of malformed `Authorization` headers and
   malformed tokens (empty parts, extra underscores, huge input); expiry and disabled checks on
   every request; whether any route outside `/api/v1/integration/` can be reached with a bearer
   token (including path tricks: trailing slashes, case, encoded slashes, `..`); whether any
   integration route honours a cookie; whether the rate limiter can be bypassed (spoofable
   client-address headers) or used to lock out a legitimate caller with a valid token; whether
   the token or its secret can reach a log line, audit row, error body or list response;
   whether deleting an account leaves usable tokens; scope validation on create and update.
4. Data model: compare migration `0005` column by column with the design's table; cascade
   delete; names stored as text; no resource name, template or override anywhere (D40).
5. Permissions test: confirm it really iterates every new route for all seven principals, and
   that a route added to `routes.go` without a matrix row would fail the test.
6. `tools/check_core_generic.sh` passes; SPDX headers present.

## Scope
May read: the whole repository except the two task folders named above.
May write: only your task folder. Temporary breaks to production code must be restored.
Must not touch: git state, anything else.

## Done when
RESULT.md holds: a table of requirement id, covering tests, mutation result, verdict
(pass / fail / weak test); a numbered list of findings with severity (high / medium / low),
`path:line`, a concrete failure scenario and a suggested fix; and what you could not check.

## Long-running steps
`./ci.sh` takes roughly 5 to 15 minutes: run it once, foreground, 30-minute timeout, output to
a log file in your task folder, read only the exit code and tail.

## Report
Follow the Reporting section of the contract; the ten-line reply states the count of findings
by severity.
