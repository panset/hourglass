# RESULT 17b-ledger-service

Status: done.

## Files changed
- internal/ledger/service.go, service_test.go, BUILD.bazel (regenerated, SPDX line kept). ledger.go untouched.
- internal/store/ledgersvc_queries.go, ledgersvc_queries_test.go, BUILD.bazel (regenerated). New store methods only; no existing store code changed.
- This folder (PROGRESS.md, RESULT.md).

## Exported API (package ledger, new)
- `NewService(st *store.Store, emit Emitter, clock Clock, loc *time.Location) *Service`; interfaces `Emitter{Emit(hourglass.Event)}`, `Clock{Now()}`.
- `CurrentPeriod()`, `EmitEvents(...hourglass.Event)`.
- `DebitTx(tx, userID, holdID, amountMT, at) (BalanceCrossings, error)`; `BalanceCrossings{Low, Overdrawn, BalanceMT, AllowanceMT, Events}`: the caller calls `EmitEvents(c.Events...)` after commit.
- `Grant(ctx, actorID, userID, amountMT, reason) (Entry, error)`, `Reset(ctx, actorID, userID, reason) (Entry, error)`; errors `ErrLocalAdmin`, `ErrNoRole` (plus the existing `ErrReasonRequired`, `ErrZeroAmount`); audit actions `AuditGrant` = "ledger.grant", `AuditReset` = "ledger.reset".
- `EnsureFirstAllowanceTx(tx, userID, at) error`.
- `RolloverDue(ctx) ([]string, error)`.
- `Balance`, `MonthToDateBurn` (positive), `BurnLast24h` (positive, from usage_samples).
- Store additions: `LatestAllowancePeriod`, `CountLedger`, `ListRoleHolders`.

## Decisions for docs/decisions.md
1. Rollover emits no balance.low/overdrawn: by D15 a rollover cannot cross either threshold (the new allowance is never below 10% of itself and debt does not go from positive to zero).
2. A period in which no user qualifies writes nothing and is not reported or emitted as closed (otherwise the newest-allowance rule would re-close it on every run).
3. `EnsureFirstAllowanceTx` does nothing while a rollover is pending (newest allowance period older than the current one); the rollover then gives the user the allowance. Otherwise an early allowance in the new month would hide the pending rollover from everyone else (D21).
4. Rollover skips the allowance entry (not the expiry/carryover) for a user who already has an allowance in the new period, so it cannot double-credit.
5. Rollover allowance = default allowance setting at the time it runs; the cache period passed to AppendLedger is the period being opened, ending at the real current period.
6. Audit rows: action `ledger.grant`/`ledger.reset`, target `user:<id>`, before_json `{"balance_mt"}`, after_json `{"balance_mt","entry_id","amount_mt","reason"}`.
7. Grant/Reset read the balance from the ledger sum, not the cache. Debits are written to the period of `at`; the cache always follows the clock's current period.
8. Balance events carry `balance_tokens` and `allowance_tokens` as float tokens (milli / 1000), with the user (id, username, email, name, roles).

## Checks
go test -race -count=1 ./internal/ledger/... ./internal/store/... pass; bazel test //internal/ledger/... //internal/store/... pass; check_spdx.sh and check_core_generic.sh OK; go vet clean.

## Confidence and gaps
Tests use a real store and FakeClock; no events dispatcher integration test (a recording Emitter stands in). Not tested: concurrent rollover from two processes (the store is single-writer; a second run sees the first's allowance and does nothing). `gofmt -l` flags internal/auth/password.go, which is another agent's file.
