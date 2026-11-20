# 66 Independent verification: reports, `reported` extensions, release changes

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/66-verify-reports-release/`.

## Goal
An independent judgement, from the code and the requirements only, of whether the report
routes, the state route, the `reported` extensions and the release changes meet the design.
You did not write this code. Do **not** read the task folders `63-*`, `64-*` or `65-*` under
`.orchestrator/integration-api/tasks/`: the authors' conclusions must not shape yours.

## What to check against
`docs/design/integration-api.md`: "Integration routes", "The `reported` provider and pressure
source" (with Wiring constraints), "Releasing a deployed setup", "Fit with D40", "Non-goals",
and requirements INT-1 to INT-8, HOLD-6 to HOLD-9, CMP-1, CMP-2. Also `docs/decisions.md`
D42 onward: check that each recorded decision is consistent with the design and flag any that
contradicts it.

The code: `git diff 8c73c6e` (commits after it plus the uncommitted working tree).

## How
1. Run `go test -race -count=1 ./...` and `./ci.sh` yourself; report how many tests executed.
2. For each requirement id: name the covering test(s), read them, judge whether they would fail
   if the requirement were broken. For every one of INT-1..INT-7 and HOLD-6..HOLD-9 make one
   small deliberate break in production code, confirm a test named for that id goes red, and
   restore exactly (compare `git diff | shasum` before and after each).
3. Try to break it. At least:
   - money: can any path debit on a stale or missing report; can a failed poll complete a
     pending release or mark a hold deployed; what is charged in the poll where a pending
     setup disappears; can a hold be released twice or `hold.released` / `hold.releasing` fire
     twice when a user release, a service release, an admin force-release and a poll race
     (read the transactions; run the race tests under `-race`);
   - a pending hold still counts toward the hold limit and blocks acquiring the setup;
   - reports: a reader seeing half of a replaced set; equal `observed_at` semantics; the
     2-minute skew; limits (10,000 entries, 32 labels, 1 MB); duplicate names; unknown names;
     `not_configured` storing nothing; a report triggering a poll, debit or event;
   - state: any leak of balances, ledger, settings, audit or users who hold nothing; per-row
     queries; `reported_deployed` taken from the stored report rather than the last poll;
   - service release: reason trimming and length; audit row in the same transaction;
     `released_by` null; event `data` exactly as the design says; 404 and 409;
   - wiring: no core package imports `providers/reported`; `pkg/hourglass/types.go` extension
     interfaces unchanged (`git diff 5808957 -- pkg/hourglass/types.go`); a third-party main
     gets the extensions by blank import (`examples/custom` still builds); the two extensions
     work independently; `max_age` default, minimum and parsing agree between the API's
     `expires_at` and the extension's staleness check; `hourglassd check-provider` prints
     report ages;
   - D40: no resource name, template or override reintroduced anywhere;
   - `hold.releasing` position in `EventTypes()`, not vetoable, accepted by the webhook filter.
4. OpenAPI: each new route and field is described and the contract test really validates the
   success responses (no leftover stub skips).

## Scope
May read: the whole repository except the three task folders named above.
May write: only your task folder. Temporary breaks must be restored. Must not touch git state.

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
