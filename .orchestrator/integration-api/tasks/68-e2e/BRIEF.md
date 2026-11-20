# 68 End-to-end scenario with the `reported` extensions and a scripted reporter

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Ground rules, Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/68-e2e/`.

## Goal
One command runs a real Hourglass stack configured with `provider.type: reported` and
`pressure.type: reported`, plus a scripted reporter playing the customer's service, through
the whole lifecycle in the design, and passes. The existing acceptance run still passes with
the changed release behaviour.

## Context
- Design: `docs/design/integration-api.md`: "How the pieces fit", "Suggested task order" item
  7, "Changes by area" (`deploy/`, `e2e/`). The customer's view: `docs/integrating.md`
  ("The full lifecycle", "If you stop reporting", "React to events").
- Existing end-to-end set-up: `e2e/README.md`, `e2e/run.sh`, `e2e/browser/tests/`, the compose
  files under `deploy/` and the override files under `e2e/`, the test main with the movable
  clock (`e2e/hourglassd-e2e`), `docs/decisions.md` D29, D34, D38.
- Earlier results under `.orchestrator/integration-api/tasks/`: `64-reported-extensions`,
  `65-release-changes` (it names `e2e/browser/tests/acceptance.spec.ts` step A7, about lines
  439-447, which still expects the old refusal text), `67-ui` (labels and selectors for
  Service accounts, token issue, and the "Releasing" state).

## What to build
1. A compose variant (or override) that runs Hourglass with the two `reported` extensions, a
   webhook hook subscribed to `hold.acquired`, `hold.releasing`, `hold.released` and
   `poll.failed` with a secret, and a small scripted reporter/receiver container or process
   (any language already used in the repo; no new heavy dependency) that: records received
   webhooks and verifies `X-Hourglass-Signature`; sends pressure and full-set deployment
   reports on command from the test.
2. A scenario, runnable by one documented command (extend `e2e/run.sh` with a mode, e.g.
   `e2e/run.sh integration`), that drives, with assertions at each step:
   a. admin creates a service account with the four scopes and issues a token through the UI
      (or the admin API if the UI path is impractical; say which and why);
   b. reporter sends pressure and an empty deployment set; a poll succeeds;
   c. a developer acquires a setup in the browser; the receiver got `hold.acquired`;
   d. reporter reports the setup deployed; after the next poll the hold is deployed, metering
      has started and the user's balance goes down on the following poll(s);
   e. the user clicks Release while it is deployed: the UI shows "Releasing", the API answered
      202, the receiver got exactly one `hold.releasing`, and the hold is still charged;
   f. reporter reports the set without the setup; after the next poll the hold is released
      and the receiver got `hold.released`;
   g. reporter goes silent past `max_age`: the next poll fails, `poll.failed` is received, no
      debit is written for a held, deployed setup during the silence, and admins can see the
      outage; a fresh report makes polls succeed again;
   h. service release: with another hold active, the reporter's token calls
      `POST /api/v1/integration/holds/{id}/release` with a reason; the hold ends, the audit
      log shows `hold.service_release` under the account's name;
   i. negative checks from the reporter's side: the token gets 401/anonymous treatment on a
      non-integration route, and a token without a scope gets 403.
   Use the test clock to move time between polls rather than sleeping for real minutes,
   as the existing run does.
3. Update the existing acceptance spec (step A7 and anything else that relied on 409
   `still_deployed`) to the new behaviour, keeping what the step was proving.
4. `e2e/README.md`: how to run the new mode and how long it takes. Keep `bazel test //...`
   hermetic: the new scenario is `manual`/script-run like the existing one.

## Scope
May write: `e2e/`, `deploy/`, `BUILD.bazel` files via Gazelle, `docs/decisions.md` (append
rows with the next free number, e.g. anything the scenario does outside the UI), your task
folder. If the scenario exposes a product bug, do not fix product code: report it with
evidence and status `partial`.
Must not touch: `internal/`, `pkg/`, `providers/`, `ui/src`, `docs/SPEC.md`, `docs/design/`,
`docs/integrating.md`, git state.

## Done when
- The new mode passes from a cold start, twice in a row.
- `e2e/run.sh bundled` passes with the updated acceptance spec (split mode: run it too if time
  allows; say if you did not).
- `./ci.sh` passes.
- All containers and volumes the runs created are removed afterwards.

## Long-running steps
Image builds and each e2e mode take about 12 to 25 minutes; `./ci.sh` 5 to 15. Run each in the
foreground with a 40-minute timeout, output to a log file in your task folder, and read only
the exit code and tail. Follow the contract's three-waits rule.

## Report
Follow the Reporting section of the contract. In RESULT.md give the exact command to bring the
`reported` stack up and leave it running with a known admin login and how to obtain a service
token, because the docs task will run the guide's curl examples against it.
