# 10 Milestone 7b: the end-to-end integration scenario reports weights
Job intent: .orchestrator/reported-weights/INTENT.md (read it first). Contract: .orchestrator/reported-weights/CONTRACT.md (binding).
## Goal
`e2e/run.sh integration` passes with new steps that cover, through the real UI and the real server, the six end-to-end bullets in the Tests section of docs/design/reported-weights.md:
1. An admin turns reported weights on with a maximum and gives the account the scope through the UI.
2. The scripted reporter sends weights for every setup; the Setups page shows the reported weight and attributes before acquire, and the acquire dialog shows the warning.
3. A hold is charged at the reported weight; the reporter changes the weight and the next debit follows.
4. The user releases; the reporter keeps reporting the weight through teardown, and the last debit is at the reported weight.
5. The reporter stops sending weights while still sending deployments and pressure; after `max_age` the price returns to the rate rule with polls still succeeding.
6. A report over the maximum is refused, changes nothing, and appears on the Settings page.
## Context
Read `e2e/README.md`, `e2e/run.sh`, `e2e/browser/tests/integration.spec.ts`, `e2e/browser/reporter/reporter.ts`, `e2e/integration-stack.sh`, D63 in docs/decisions.md, and `.orchestrator/reported-weights/tasks/00-digest/RESULT.md` (e2e facts and hazards). The product and UI are complete (see `.orchestrator/reported-weights/tasks/08-m6-ui/RESULT.md` for the strings and controls the pages show). The reporter sends weights before deployments (design B3). Use the test clock to pass `max_age`, as the scenario already does. Step I1 asserts the account's exact scopes: extend it to the fifth scope where the scenario grants it (additive, design D3). `e2e/integration-stack.sh` creates the `docs-example` account with a hardcoded scope list: add `weights:write` so the docs task can run examples. Update `e2e/README.md` for the new steps. Neutral names only (`memory-gb`, `storage-gb`, `tier`).
If a step shows a product bug, do not patch product code: report it with evidence (screenshot path, response body) and mark the task partial.
## Hazards
Ports 8080, 8081, 8990, 8091 and 9100 must be free and owned by the stack: before a run, check what listens on them (`lsof -iTCP -sTCP:LISTEN -P | grep -E ':(8080|8081|8990|8091|9100|8443) '`). If something of the owner's is listening (for example a `hourglass-tls` compose project or a host process), do not stop or kill it: report `blocked` with what you found. Only stop containers this scenario's own compose project started.
## Scope
May write: `e2e/**`, this task folder. Must not touch: product code, `ui/src`, `api/`, `docs/`, `testdata/`.
## Done when
`e2e/run.sh integration` exits 0 with every old and new step passing (list the steps from `e2e/out/integration/steps.json`); the acceptance scenario is untouched or still passing if a shared helper changed (then run `e2e/run.sh bundled` too); `./ci.sh` exits 0; RESULT.md maps each of the six bullets to a step id.
## Long-running steps
`e2e/run.sh integration`: about 15 to 20 minutes, 45 minute timeout, log into this task folder. `./ci.sh`: 30 minutes.
## Report
Per CONTRACT.md "Reporting".
