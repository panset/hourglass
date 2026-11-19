# 07b fix core: RESULT

**Status: done.** All four brief changes applied; `go test` and `go vet` pass for `internal/rules`, `internal/ledger`, `internal/metering`.

## Diff (my files only)
- `internal/ledger/ledger.go` `Crossings`: `low` now requires `allowanceMT > 0`; `overdrawn` unchanged. Doc comment updated.
- `internal/ledger/ledger_test.go`: `TestLED_Crossings` renamed `TestEVT_BalanceCrossings`; added a second table of allowance 0 and negative cases (low always false). `TestRULE_FirstAllowanceOncePerPeriod` renamed `TestROLE_FirstAllowanceOncePerPeriod`.
- `internal/metering/metering.go` `ProjectedRunOut`: balance <= 0 is checked before burn, so (now, true) with zero burn. Doc comment updated.
- `internal/metering/metering_test.go`: `TestDash_ProjectedRunOut` renamed `TestUI_ProjectedRunOut`; "no burn and no balance" now expects (now, true); added "no burn and negative balance".
- `internal/rules/rules_test.go`: new `TestRULE1_EqualPositionsKeepInputOrder` (one Position-9 rule, then 20 Position-5 rules, first match must be ID 1).

## Mutation check
Swapping `sort.SliceStable` to `sort.Slice` in `rules.go` makes the new test fail (`rule 10, want 1`); the 24-equal-Position version I tried first survived because `sort.Slice` is the identity when all keys are equal, so a mixed-Position layout is required. `rules.go` restored and green.

## Notes
- Did not touch go.mod, MODULE.bazel, ci.sh, or other packages; did not run Bazel. Other modified files in `git status` belong to other agents.
