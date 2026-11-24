# Result: 10 Milestone 7b, end-to-end reported weights

Status: DONE. `e2e/run.sh integration` exits 0 on the current tree with 17 steps, all `pass`, none skipped (I0-I10 and W1-W6, I9 including the weights 403 check). `./ci.sh` exits 0. No product bug found.

## Runs
- Run 1 (integration-run1.log): exit 1. W1 failed (test read `reported_weights` at the top of the settings response; it is under `settings`). W2 failed (the Setups page was loaded before the report and not refetched; the server answers live). W3-W6 failed as a cascade (W2 failed before the acquire); W6 also had a wrong expectation (below).
- Run 2 (integration-run2.log): exit 1. W3: the default "By day" ledger view merges the two debits into one row, "weight 1.15 to 1.40 (reported)" (WGT-15 day view), so the per-debit text is only in "Every poll". W6: `last_error` on the collector route is the latest failed poll ever (I7's), not "the last poll"; the test expected null.
- Run 3 (integration.log, exit file integration.exit): exit 0, 1368 s. steps.json: 17 pass.
- `./ci.sh` (ci.log, ci.exit): exit 0, 40 of 40 tests pass.
- Ports 8080, 8091, 8081, 8990, 8443 are free after the runs (lsof empty); the run script tore its stack down.

## Files changed (complete list)
e2e/browser/reporter/reporter.ts
e2e/browser/tests/integration.spec.ts
e2e/integration-stack.sh
e2e/README.md
.orchestrator/reported-weights/tasks/10-m7b-e2e/PROGRESS.md
.orchestrator/reported-weights/tasks/10-m7b-e2e/RESULT.md
.orchestrator/reported-weights/tasks/10-m7b-e2e/integration.log
.orchestrator/reported-weights/tasks/10-m7b-e2e/integration-run1.log
.orchestrator/reported-weights/tasks/10-m7b-e2e/integration-run2.log
.orchestrator/reported-weights/tasks/10-m7b-e2e/integration.exit
.orchestrator/reported-weights/tasks/10-m7b-e2e/ci.log
.orchestrator/reported-weights/tasks/10-m7b-e2e/ci.exit

## Test fixes made in this task (all test-side, none weakens a step)
1. W1: settings read as `.settings.reported_weights`.
2. W2: `alice.reload()` after opening Setups, before asserting the row.
3. W3: the day view is asserted as `weight 1.15 to 1.40 (reported)`, then "Every poll" is clicked and `weight 1.4 (reported)` asserted (more than before).
4. W6: instead of `last_error` null, asserts `last_successful_poll_at` is later than `last_error.at` and the newest run is ok.

## W step -> what it shows -> result
| Step | Shows (design Tests bullet) | Result |
| --- | --- | --- |
| W1 | admin grants `weights:write` in the UI (403 before, 409 not_configured with the setting off), turns reported weights on with maximum 3 in Settings; five scopes | pass |
| W2 | reporter sends weights for every setup; state route and Setups page show weight, reported tag, attributes before acquire; acquire dialog shows the warning | pass |
| W3 | hold charged at 1.15 (source reported); report changes to 1.4, screens follow at once, next debit at 1.4; day and every-poll ledger views | pass |
| W4 | release; reporter keeps weight through teardown; last debit at the reported weight 1.4; released hold shows it | pass |
| W5 | over-maximum report refused 400, changes nothing, shown on Settings with account and message; next poll succeeds | pass |
| W6 | weights stop, deployments and pressure continue; after max_age price is the rate rule (weight 2), not_applied expired, polls succeed, Setup inventory says so | pass |
| I9 | a state:read-only token gets 403 on the weights route as on the others | pass |

## Decisions for docs/decisions.md
1. I1 keeps four scopes; W1 grants the fifth through the UI and asserts five (from the earlier work).
2. Scenario maximum weight 3, maximum age 10m, W6 uses the test clock; W5 runs before W6 so the refusal is the later event.
3. Neutral names only (`memory-gb`, `storage-gb`, `tier`).
4. Observation for the docs task: the user screens read the stored report live (a loaded page needs a refetch to show a change); a ledger row in the By day view shows a weight range ("weight 1.15 to 1.40 (reported)").

## Confidence and gaps
- Verified by a full run: all 17 steps passed in the final run. The acceptance spec and `bundled` mode were not touched or run (no shared helper changed).
- Not verified: the flakiness of timing-bound waits across repeated runs (one passing run only).
- `integration-run1.log` and `integration-run2.log` are kept as evidence of the failures fixed above.
