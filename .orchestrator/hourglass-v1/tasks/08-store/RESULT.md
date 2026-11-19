# RESULT 08-store

Status: done, with one caveat on `./ci.sh` (below).

## Versions
- `go.mod`: `go 1.26.0` (forced by `golang.org/x/sys` v0.49.0 and `modernc.org/libc` v1.80.1, which need Go >= 1.26). Local toolchain is 1.27.4.
- Bazel Go SDK: 1.27.4 (`MODULE.bazel` `go_sdk.download`).
- rules_go 0.50.1 -> 0.59.0 and gazelle 0.39.1 -> 0.47.0. Reason: rules_go 0.50.1 cannot build the Go 1.26+ stdlib (`go: unknown GOEXPERIMENT coverageredesign`). 0.59.0 / 0.47.0 work with Bazel 7.4.1 (verified; `.bazelversion` unchanged).
- `modernc.org/sqlite` v1.62.0 (pure Go; `go_deps` extension plus `use_repo(go_deps, "org_modernc_sqlite")`). Builds and tests pass with `CGO_ENABLED=0`.

## Files changed
`go.mod`, `go.sum`, `MODULE.bazel`, `MODULE.bazel.lock`, `internal/store/` (`store.go`, `users.go`, `setups.go`, `rules.go`, `holds.go`, `usage.go`, `ledger.go`, `misc.go`, `store_test.go`, `BUILD.bazel`, `migrations/0001_init.sql`), this folder.

## Exported API (package `store`; every method below is on both `*Store` and `*Tx`)
- Core: `Open(path)`, `(*Store).Close`, `(*Store).Tx(ctx, fn)`, `FormatTime`/`ParseTime`/`TimeLayout`; errors `ErrNotFound`, `ErrSetupHeld`, `ErrConflict`.
- Users: `CreateOIDCUser`, `UpsertOIDCLogin` (returns created flag), `CreateLocalAdmin`, `GetUser`, `GetUserBySub`, `GetUserByUsername`, `ListUsers(status)`, `SetUserStatus`, `SetPassword`, `GetUserRoles`, `SetUserRoles`.
- Setups: `CreateSetup`, `UpdateSetup` (labels replaced), `DeleteSetup`, `GetSetup`, `ListSetups(enabledOnly)`.
- Rules: `ListRules(kind)`, `ReplaceRules(kind, rules, updatedBy, now)`.
- Holds: `CreateHold`, `GetHold`, `ActiveHoldForSetup`, `SetMeteringStarted`, `SetDeployed` (set once, return whether set), `ReleaseHold`, `ActiveHolds`, `CountActiveHolds`, `UserHolds`.
- Usage: `InsertUsageSample` (bool inserted), `ListUsageSamples`, `DailyDebits`, `SumDebitsSince`, `PruneUsageSamples`.
- Ledger: `AppendLedger`, `ListLedger`, `SumLedger(user, period, type)`, `RebuildBalance`.
- Pressure: `InsertPressureSample`, `PressureSince`, `LatestPressure`. Orphans: `UpsertOrphan`, `DeleteOrphansNotSeenSince`, `ListOrphans`. Outages: `InsertOutage`, `ListOutages`. Collector: `StartCollectorRun`, `FinishCollectorRun`, `LastSuccessfulRun`, `LatestRun`. Events: `InsertEventDelivery`, `FinishEventDelivery`, `ListEventDeliveries`, `PruneEventDeliveries`. Settings: `GetSetting`, `PutSetting`, `AllSettings`. Audit: `AppendAudit`, `ListAudit(AuditFilter)`. Sessions: `CreateSession`, `GetSession`, `TouchSession`, `DeleteSession`, `DeleteExpiredSessions`, `DeleteUserSessions`.
- Row types: `User`, `Setup`, `Rule`, `Hold`, `UsageSample`, `DayTotal`, `LedgerEntry`, `PressureSample`, `Orphan`, `EventDelivery`, `Setting`, `AuditEntry`, `AuditFilter`, `Session`, `CollectorRun`, `Outage`.

## Tests
20 test functions in `internal/store/store_test.go`, all against real files in `t.TempDir()`: fresh migrate and reopen, newer-DB error, spec column check (columns and table count), users, oidc_sub unique, setups and constraints, labels key index and composite key, rules replace and rollback, hold lifecycle and set-once, concurrent hold creation (`TestHOLD_UniquenessUnderConcurrentCreates`), `TestMET5_UsageSampleIdempotent`, usage sums and prune, ledger triggers, `TestLED2_BalanceCacheMatchesLedger`, tx rollback, pressure/orphans/outages/runs, events/settings/audit, sessions, foreign keys, timestamp format.
Commands: `go test -race ./internal/store` ok; `CGO_ENABLED=0 go test ./internal/store` ok; `bazel build //internal/... //cmd/...` and `bazel test //internal/... //cmd/...` pass (5/5); gazelle diff for `internal` is clean.

## ci.sh caveat
`./ci.sh` exits 1 at its `gazelle update -mode=diff` step, but only because the other agent's new `pkg/hourglasstest` has no `BUILD.bazel` yet (the diff names only that path). It will pass once that package's BUILD file is generated. I did not touch `pkg/`. Steps after the diff (`bazel build //...`, `bazel test //...`) were not run over `//pkg/...`.

## Decisions for docs/decisions.md
1. Toolchain: go directive 1.26.0, SDK 1.27.4, rules_go 0.59.0, gazelle 0.47.0 (see above; supersedes D4/D9 versions).
2. Timestamps stored as `2006-01-02T15:04:05.000Z` (UTC, millisecond, fixed width). `polled_at` uniqueness is therefore per millisecond.
3. `AppendLedger` sets `users.balance_mt` to the sum of the user's entries for the appended entry's period (the newest entry's period is treated as "current"). A pure running total would double-count the LED-3 carryover of a negative balance; callers still can use `RebuildBalance`.
4. `usage_samples.debit_mt` is a positive amount; ledger `amount_mt` is signed (debits negative).
5. `weight`, `pressure`, `multiplier`, `rules.value` are REAL; `sessions.id` is opaque TEXT; `pressure_samples.polled_at` is its primary key (spec lists no id).
6. Added `CHECK` constraints on enum columns (auth_source, status, role, ledger type, period shape) and non-unique indexes (`holds.user_id`, `ledger(user_id, period)`, `usage_samples.polled_at`, a few time columns). `username` is not unique (spec names only oidc_sub, setup name).
7. Deleting a setup that has holds returns `ErrConflict` (holds reference setups; history is kept).
8. Connections use `_txlock=immediate`, `busy_timeout=10000`, `synchronous=NORMAL`.
9. Outages only: `outages(id, started_at, ended_at, reason)` and `schema_migrations(version, name, applied_at)` are the only tables beyond the spec's 15.

## Open questions
None blocking. Confirm decision 3 against how the LED-3 rollover job should write its entries (it should append `expiry`/`carryover` then `allowance`, each with its own period).

## Confidence and gaps
Verified by running the tests above. Not tested: very large databases, crash recovery, behaviour under multiple processes sharing one file.
