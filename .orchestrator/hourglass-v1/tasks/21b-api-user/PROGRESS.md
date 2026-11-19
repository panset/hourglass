# PROGRESS 21b-api-user
- 2026-11-19 store: added internal/store/apiuser_queries.go (+test): HoldCharges, HoldLastSamples, LedgerPeriods, DebitEntries, DebitMultipliers.
- 2026-11-19 api: user_views.go (response shapes, lab loader), handlers_user.go (8 handlers). Helpers prefixed to avoid clashes with the admin task's files.
- 2026-11-19 tests: user_testenv_test.go, user_test.go (real router, store, ledger, holds, collector; fakes for extensions and clock).
- 2026-11-19 checks: go build, go vet, gofmt, both tools scripts clean; gazelle on internal/api and internal/store; bazel test run. Only failures are three stale 21a router tests that expect 501 stubs.
