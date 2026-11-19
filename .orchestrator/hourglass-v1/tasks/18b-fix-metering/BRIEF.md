# 18b fix metering verification findings
Job intent: .orchestrator/hourglass-v1/INTENT.md. Rules: .orchestrator/hourglass-v1/CONTRACT.md. Findings with failing tests: .orchestrator/hourglass-v1/tasks/18-verify-metering/RESULT.md and its `evidence/` folder (adapt those tests into the packages' own test files as regression tests).
## Changes
1. Clock stepped backwards must never charge twice: the collector compares T with the newest successful poll of any time; if T is not after it, the poll charges nothing (zero-debit samples are fine) and logs a warning. Regression test from the evidence.
2. `RunOnce` itself runs the ledger rollover check before building the snapshot (keep the call in `Run` harmless or remove it), so a poll at or after a month boundary can never write a debit into a period that has not been opened. Regression test: poll exactly at 00:00 local on the 1st with a user in debt; afterwards the balance cache equals the new period's ledger sum (allowance + carry-over + any debit), and acquire is not wrongly refused.
3. Shortened poll interval must not produce a false outage: the 2x cap is computed from the larger of the current interval and the interval that was in force when the previous successful poll ran (keep the latter in memory in the collector; after a restart fall back to the current setting). Test.
4. Tests that kill the surviving mutations M10 and M4 named in the verification result.
5. `settings.Save` must not call `time.Now()`: add an additive variant taking the timestamp (or a clock) and make the existing function delegate to it, without changing any existing exported signature.
6. Force-release audit row carries `before_json` (the hold before release).
7. `collector.PollView` gains whatever methods are needed to satisfy `holds.PollView` directly (additive), with a compile-time assertion in a test.
8. Not changed, by decision: users without a role are skipped by the rollover (LED-3 says "every user who has a role"), so debt does not follow a user whose roles were removed across a month boundary. Add a test documenting this behaviour, named `TestLED3_RolelessUserSkippedAtRollover_DebtNotCarried`.
## Constraints
Another agent is compiling against these packages right now: make only additive changes to exported APIs. Scope: `internal/collector/`, `internal/settings/`, `internal/holds/`, `internal/ledger/` (tests, and service code only if change 2 needs it), this task folder. No go.mod or MODULE.bazel edits (another agent owns them now); regenerate BUILD files only for the directories you touched.
## Done when
`go test -race -count=1 ./internal/collector/... ./internal/settings/... ./internal/holds/... ./internal/ledger/...` and both tools scripts pass (run `bazel test` on those four once; if it fails for reasons outside these packages, say so). RESULT.md maps each finding to its fix and test.
