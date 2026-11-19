# PROGRESS 17-collector
- 2026-11-19 Read brief, INTENT, CONTRACT, spec sections, RESULT.md of 08, 09, 09b, 10, 11, 16, 16b, 17b and the code of store, settings, metering, rules, ledger service, hourglass, hourglasstest.
- 2026-11-19 Decision: previous successful poll comes from the database (latest ok run started strictly before T) so a restart charges the real gap and two collectors at the same T compute the same elapsed; the usage-sample conflict then skips the second debit (MET-5). Needs two new store queries in internal/store/collector_queries.go.
- Next: write store queries + test, then internal/collector and tests.
- 2026-11-19 Wrote internal/store/collector_queries.go (+test): LastSuccessfulRunBefore, InsertPressureSampleIfAbsent.
- 2026-11-19 Wrote internal/collector/collector.go and collector_test.go (21 tests); go test -race passes. Mutations (drop MET-5 skip; drop event emission) turn tests red.
- 2026-11-19 Gazelle for internal/collector and internal/store, SPDX line added; bazel test //internal/collector/... //internal/store/... pass; check_spdx and check_core_generic OK (renamed a test label from a size-like key to tier=gold).
- 2026-11-19 RESULT.md written. Done.
