# 69 Independent verification: UI and the end-to-end scenario

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/69-verify-ui-e2e/`.

## Goal
An independent judgement of whether the UI changes and the end-to-end scenario meet the
design, made by running them. You did not write them. Do **not** read the task folders `67-*`
and `68-*` under `.orchestrator/integration-api/tasks/`.

## What to check against
`docs/design/integration-api.md`: "How the pieces fit", "Admin routes for service accounts"
(last paragraph), "Releasing a deployed setup", "Changes by area" (`ui/`, `deploy/`, `e2e/`),
"Suggested task order" item 7: the scenario must cover acquire, deploy, release request,
teardown and completion, plus the reporter going silent. `e2e/README.md` tells you how to run.

## How
1. From a cold start (no leftover containers, volumes or images from earlier runs of this
   scenario), run the integration end-to-end mode as `e2e/README.md` documents. Then run
   `e2e/run.sh bundled` (the existing acceptance path, whose release step changed).
2. Read the scenario's test code and judge, step by step, whether each assertion would fail if
   the product misbehaved: in particular that `hold.releasing` is asserted to arrive exactly
   once, that the hold is still charged while releasing, that the silent-reporter phase
   asserts no debit was written and `poll.failed` arrived, that the webhook signature is
   really verified, and that the service release's audit row is asserted. Flag assertions
   that only check "no error" or that sleep-and-hope.
3. With the stack up, check in the browser-driven tests or by hand through the API: the
   Service accounts page lists, creates, edits, deletes, issues and revokes; the token is
   shown once and is not retrievable afterwards from any admin response, the page after a
   reload, browser storage or the URL; "Releasing" replaces the Release button on Setups and
   My usage; the collector view shows the last report times; the admin UI makes no request to
   `/api/v1/integration/`.
4. `cd ui && npm run typecheck && npm test`; report test counts. `./ci.sh`.
5. Confirm everything the runs created (containers, networks, volumes) is removed at the end.

## Scope
May write: only your task folder. Must not touch git state or any other file.

## Done when
RESULT.md holds: what ran, with exit codes and test counts; a numbered list of findings with
severity (high / medium / low), evidence, a failure scenario and a suggested fix; and what you
could not check.

## Long-running steps
Each e2e mode takes about 12 to 25 minutes including image builds; `./ci.sh` 5 to 15. Run each
in the foreground with a 40-minute timeout, output to a log file in your task folder, read
only the exit code and tail. Follow the contract's three-waits rule.

## Report
Follow the Reporting section of the contract; state the count of findings by severity.
