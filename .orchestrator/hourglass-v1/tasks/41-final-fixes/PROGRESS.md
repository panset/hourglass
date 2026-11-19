# PROGRESS 41-final-fixes
- 2026-11-19 H1: incremental balance cache, migration 0003 (covering indexes), LedgerStats/CountOverdrawn rewrite, admin users list, HoldLastSamples seek, setups list without charges; store statement counter.
- 2026-11-19 Tests: TestNFR_ApiUnder500msAtSpecScale (reduced + HOURGLASS_SCALE_TEST=1), TestNFR_LedgerAppendTimeIsLinear, TestLED2_CacheEqualsSumAfterAnyOrderOfWrites, RULE-5 tests; AUTH-8/UI-2 renames.
- 2026-11-19 L1 L3 L4 L5 L6 L8 L9 and housekeeping done (see RESULT.md).
- 2026-11-19 go test -race, ci.sh exit 0, full-size perf, both smokes pass. e2e/run.sh both started (log e2e-both.log).
- 2026-11-19 e2e/run.sh both passed; done.
