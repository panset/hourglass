# 76b progress

- 2026-11-23 Read brief, 76 RESULT, collector, store and ledger compaction code.
- Decision M1: Run now measures the time compactIfDue takes and deducts it from the wait (internal/collector/collector.go). Smallest change that bounds the gap; no setting, step count unchanged.
- Added TestLED10_CompactionDoesNotStretchThePollInterval, _LongCompactionNeverWaitsPastTheInterval, _NoCompactionKeepsTheFullInterval; checked red with the deduction disabled.
- Decision L1: DueLedgerGroups looks at most store.LedgerCompactionStepDays (60) local days per call; compactAll test helper now stops at the cutoff, not at an empty batch. Added TestLED10_DueSearchStepIsBoundedByDays (red without the bound) and 400 one-row days in the api scale test.
- Decision L2: internal/api race_on/off_test.go; scale test skips wall-clock bounds under -race. TestNFR_LedgerAppendTimeIsLinear predates the job (git log -S: 096a285 and earlier): not edited.
- L4(b)(c): added TestLED7_SpringForwardDayIsOneGroup and TestLED7_TimezoneChangeLeavesRolledUpRowsAlone. L4(a) not done (see RESULT).
- Ran gazelle (BUILD.bazel of internal/api updated), go test ./internal/... green, -race on ledger/collector/store green.
- Next: race scale test in api, ./ci.sh, RESULT.md.
