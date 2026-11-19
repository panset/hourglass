# RESULT 42b-fix-cache

Status: done. All seven changes are in the working tree. Every fix has a test that fails without it (shown by a
run against the unmodified commit `132a9a4` in a scratch clone, or by a mutation of the fix in this tree, restored
afterwards) and passes with it.

## Findings of task 42 mapped to fixes and tests

| # | Finding | Fix | Test(s) (red before, green after) |
|---|---|---|---|
| 1 High | Multi-period rollover + existing current-period entry: two allowances in the cache | `RolloverDue` appends with the real current period (`internal/ledger/service.go:319`), and every rollover transaction ends with `tx.RebuildBalances(ctx, cur)` (`service.go:328`): one grouped SELECT + one `UPDATE … FROM (grouped sums)` for all users (`internal/store/ledger.go:138`) | `TestLED2_LED3_MultiPeriodRolloverWithCurrentPeriodGrantKeepsCache` (`internal/collector/cache_led2_test.go:177`): on `132a9a4` cache 600,000 vs ledger 305,000; now 305,000, checked after every step. Mutations: append with `next` and no rebuild: red; only the one-line fix: green; only the rebuild: green (each layer suffices alone). `TestLED2_RebuildBalancesGroupedAndReported` (`internal/store/ledgersvc_queries_test.go:47`): same statement count for 2 and 42 users, zero for a user with no entries, exact change report |
| 2 Medium | Timezone moved backwards: cache wrong for good | (a) Start-up self-repair: `app.repairBalances` (`internal/app/app.go:198`, `:300`) calls `ledger.Service.RepairBalances` (`service.go:376`) before the collector and HTTP start; logs `changed`, `abs_delta_mt`, `net_delta_mt`, at WARN when non-zero. (b) The service remembers the period it last rebuilt for; when `RolloverDue` finds the current period differs from it without a rollover (zone moved back, then the clock returned to an opened period), it rebuilds every cache (`service.go:351`). (c) `docs/deploying.md` Rules: two sentences on changing `timezone` | `TestLED2_TimezoneChangeBetweenRestartsCacheRepaired` (`cache_led2_test.go:444`), now a hard check after every poll in both directions: red with `RepairBalances` made a no-op, red with (b) disabled. `TestLED2_StartupRebuildsBalanceCachesBeforeServing` (`internal/app/app_test.go:566`): corrupt cache repaired, WARN line with count/total, logged before `http listening`; a clean start logs INFO with 0; red with the start-up call removed |
| 3 Low | Role-less user's stale cache shown in the admin list | `GET /api/v1/admin/users` returns `balance_tokens` and `month_to_date_burn_tokens` null for a user with no role (`internal/api/admin_views.go:192`); `api/openapi.yaml` makes the burn nullable; `ui/src/api/schema.gen.ts` regenerated with `npm run gen:api`; the Users page shows a dash in both columns and no Grant button for a role-less user. Also the rollover rebuild now sets a role-less user's cache to the current-period sum | `TestADM1_LED3_UserWithNoRoleHasNullBalanceAndBurn` (`internal/api/admin_users_test.go:387`, red on `132a9a4`: 287.5 / 12.5); contract world gets a demoted user so `TestUI2_ContractResponsesMatchSchemas` validates the null (red if the schema is not nullable); `TestLED2_LED3_RolesRemovedAcrossBoundaryCacheFollowsLedger` (red on `132a9a4`, cache 200,000 vs 0); UI `ADM-1 LED-3 a user with no role shows a dash … (null figures / stale figures)` (stale case red against the old page) |
| 4 Low | Scale test blind to a per-user query in `status=pending` | `seedScale` creates `max(2, users/6)` pending users (2 and 8 at the two reduced sizes, 33 at full size) | Mutation (one `SumLedger` per returned user in the pending path of `listAdminUsers`): new test red ("16 statements … and 22"), old test green; restored |
| 5 Low | `readLedger` silently accepts oldest-first | `e2e/browser/tests/acceptance.spec.ts` `readLedger` asserts every adjacent pair satisfies the newest-first running balance (tolerance 0.002 for display rounding), then returns oldest first | Throwaway Playwright check on `page.setContent` tables built from the real function text: newest-first accepted and reversed; oldest-first fails with "ledger not newest first at row 1". Exercised for real in `e2e/run.sh both` |
| 6 Low | Audit filter example `grant.create` is never written | Label "Action (exact name, e.g. ledger.grant)" (`ui/src/pages/admin-audit/index.tsx:74`); `api/openapi.yaml` description and example use `ledger.grant`; UI test fixtures use `ledger.grant` | UI `ADM-3 the action filter's example is a name the server writes (ledger.grant)`: red against the old page |
| 7, 8 Info | Pending-rollover window; Settings band column offset | Not in the brief; unchanged. Finding 7 is narrower now: a grant/reset/debit in the window resets the cache to the new period's sum, and only users with no entry yet in the new period still show the previous period until the poll | The adopted tests keep `soft` (logged, not failed) only for those window steps |

## Change 7: verifier evidence adopted

- `cache_led2_test.go.txt` → `internal/collector/cache_led2_test.go` (12 tests, renamed `TestLED2_…`/`TestLED3_…`). Changes from
  the verifier's file: hard checks where the invariant now holds (grant in the pending window, timezone polls); the
  rollback test now runs the rollover and checks; the timezone test calls `RepairBalances` at the simulated restart
  as the application does; the multi-period test asserts the exact 305,000 and a second poll; the concurrency test
  uses 300 debits per writer instead of 1,000 (1,000 hit `SQLITE_BUSY` after SQLite's 10 s busy timeout when the
  whole suite ran under `-race`; 300 passed `-race -count=3`).
- `responses_test.go.txt` → `internal/api/admin_responses_test.go` (`TestADM1_LED2_ResponsesMatchLedgerAndHolds`). The
  verifier's raw-DB hook (`RawDBForV42`) does not exist in the product, so the independent figures are summed in Go
  from `ListLedger` rows, `ActiveHolds`, `GetUserRoles` and `ListSetups` instead of the cache. Expectations updated
  for change 3 (role-less, pending and rejected users: null balance and burn).
- `migration_make_old_db_test.go.txt` + `migration_upgrade_test.go.txt` → `TestNFR_Migration0003UpgradesVersion2Database`
  (`internal/store/store_test.go:916`): builds the version-2 schema in-process by undoing 0003 on a populated
  database (0003 only changes indexes), then checks version 3, rows kept, the three new indexes and no
  `ledger_user_period`, integrity_check ok, caches equal ledger sums, an append afterwards, and a no-op reopen.
  The "previous binary refuses version 3" check is already `TestMigrateNewerDatabaseIsError`.
- Not adopted: `playwright-v42/shots.spec.ts` and `bands.spec.ts` are screenshot harnesses (their sign-in, approval,
  acquire and metering steps are covered by acceptance A0-A9; the band offset is finding 8, out of scope); the logs,
  compose configs and `docker ps` captures are evidence, not tests.

## Verification

| Command | Outcome | Log |
|---|---|---|
| `go test -race -count=1 ./...` | exit 0, 30 packages ok | `gotest-race.log` |
| `./ci.sh` | exit 0, 35 of 35 Bazel tests pass (incl. `//ui:test`, `//ui:typecheck`, `//ui:api_schema_drift_test`) | `ci.log` |
| `HOURGLASS_SCALE_TEST=1 go test ./internal/api -run NFR_ApiUnder500msAtSpecScale -v -count=1 -timeout 20m` | pass; worst: `/admin/users` 148.6 ms, `?status=pending` 108.9 ms; collector poll 199.9 ms; new log line: rebuild of all caches over 518,400 ledger rows 61.0 ms, 0 changed | `perf-fullsize.log` |
| `e2e/run.sh both` | exit 0: bundled 1 passed (A0-A9, 715 s), split 1 passed (708 s); each stack logged `balance caches checked against the ledger … changed 0` at start-up (`e2e/out/*/containers.log`) | `e2e-both.log` |

## Files changed

Go: `internal/ledger/service.go`, `internal/store/ledger.go`, `internal/app/app.go`, `internal/api/admin_views.go`.
Tests: `internal/collector/cache_led2_test.go` (new), `internal/collector/BUILD.bazel`, `internal/store/ledgersvc_queries_test.go`,
`internal/store/store_test.go`, `internal/app/app_test.go`, `internal/api/admin_responses_test.go` (new),
`internal/api/BUILD.bazel`, `internal/api/admin_users_test.go`, `internal/api/contract_test.go`,
`internal/api/perf_scale_test.go`, `e2e/browser/tests/acceptance.spec.ts`.
Contract and UI: `api/openapi.yaml`, `ui/src/api/schema.gen.ts` (generated), `ui/src/pages/admin-users/index.tsx`,
`ui/src/pages/admin-users/admin-users.test.tsx`, `ui/src/pages/admin-shared/fixtures.ts`,
`ui/src/pages/admin-audit/index.tsx`, `ui/src/pages/admin-audit/admin-audit.test.tsx`.
Docs: `docs/deploying.md`. No migration was needed. Task folder: `PROGRESS.md`, `RESULT.md`, logs.

## Decisions for docs/decisions.md

| Decision | Why |
|---|---|
| The rollover rebuilds the cache of **every** user (not only the role holders it wrote entries for) at the end of each period's transaction | One grouped statement either way; it also clears a role-less user's stale figure (finding 3) and makes the invariant hold for every user |
| The start-up rebuild covers every user and logs `changed`, `abs_delta_mt` (sum of absolute changes) and `net_delta_mt`; WARN when anything changed, INFO otherwise; a failure stops start-up | Brief; a non-zero change means a cache was wrong, which an operator should see |
| `ledger.Service` remembers (in memory) the period it last rebuilt the caches for; `RolloverDue` rebuilds them all when the current period differs without a rollover | The start-up rebuild alone does not cover finding 2: after a backwards zone change the clock returns to an already-opened period, no rollover runs, and appends would add to a cache that holds the other period. Costs nothing in normal operation (a string compare per poll) |
| `month_to_date_burn_tokens` is null for a user with no role (pending, rejected, roles removed); it stays 0 for the local admin | Brief asks null for role-less users; the local admin's 0 is unchanged to keep this change narrow |
| The Users page shows a dash and no Grant button for a user with no role, whatever figures the API sends | The API refuses grants to a role-less user (`ErrNoRole`); a dash matches the null |
| Changing the configured timezone across a month boundary is documented as "change it just after a rollover" rather than refused at start | Brief; refusing to start would need a rule about what "older" means after a deliberate change |
| The adopted concurrency test uses 300 debits per writer | 1,000 exceeded SQLite's 10 s busy timeout under `-race` with the whole suite running; two back-to-back writers for 10 s do not occur in the product |

## Confidence and gaps

- Verified by tests: each fix above. The rollover's own rebuild masks the one-line fix (mutation M3 shows the rebuild
  alone passes the multi-period test), so the one-line fix is proven separately by mutation M2.
- Pre-existing, not fixed (out of scope): after a backwards zone change the **ledger** still writes debits into a
  period already closed, and they are never carried (finding 2, "ledger creates money"); the cache now matches the
  ledger throughout, so it shows those debits in the backward window and the new month's figure after it. The docs
  sentence is the mitigation.
- Finding 4's audit half (`/admin/audit` timed on an empty table) is not addressed: the brief named only the pending path.
- The pending-rollover window (finding 7, documented D-A) remains for users with no entry yet in the new period.
- After all four verification runs, only a doc comment was edited (`internal/store/ledger.go`, `AppendLedger`); `go test` of store, ledger and collector re-run green afterwards.
