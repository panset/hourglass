# 07b fix findings from the core verification
Rules: .orchestrator/hourglass-v1/CONTRACT.md. Findings: .orchestrator/hourglass-v1/tasks/07-verify-core/RESULT.md (read its findings list).
## Changes (exactly these, nothing else)
1. `internal/rules`: add a test `TestRULE1_EqualPositionsKeepInputOrder` with at least 20 rules sharing one Position where only stable sorting gives the right first match; confirm it fails if `sort.SliceStable` is swapped for `sort.Slice`, then restore.
2. `internal/ledger` `Crossings`: when allowanceMT <= 0, `low` is always false (there is no 10% threshold); `overdrawn` unchanged. Add cases `TestLED_Crossings` -> rename the test to `TestEVT_BalanceCrossings` and add the allowance-0 cases.
3. `internal/metering` `ProjectedRunOut`: check the balance first, so balance <= 0 returns (now, true) even with zero burn. Add the case; rename the test to `TestUI_ProjectedRunOut`.
4. Rename `TestRULE_FirstAllowanceOncePerPeriod` to `TestROLE_FirstAllowanceOncePerPeriod`.
## Scope
May write: `internal/rules/`, `internal/ledger/`, `internal/metering/`, this task folder. Other agents are changing go.mod/MODULE.bazel and other packages; do not touch them and do not run Bazel. Verify with `go test ./internal/rules/... ./internal/ledger/... ./internal/metering/...` and `go vet` on those packages.
## Done when
Those tests pass; RESULT.md lists the diff in a few lines.
