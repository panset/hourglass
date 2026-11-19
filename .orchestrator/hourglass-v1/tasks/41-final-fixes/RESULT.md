# RESULT 41-final-fixes

Status: done. All verification commands passed.

## Finding to fix
| Finding | Fix |
|---|---|
| H1 ledger append re-summed the period | `internal/store/ledger.go` AppendLedger keeps `users.balance_mt` incrementally in the same transaction: current-period entry adds its amount; other-period entry leaves it; the user's first current-period entry (rollover) sets it from that entry (one indexed EXISTS probe, O(log n)). `RebuildBalance` stays as the from-scratch path. Tests: `TestNFR_LedgerAppendTimeIsLinear` (10,000 appends; last quarter vs first quarter; fails with the old code at about 4x), `TestLED2_CacheEqualsSumAfterAnyOrderOfWrites` (rollover, late past-period entry), existing LED-2 tests unchanged. |
| H1 admin users list | `LedgerStats(period, userID, status)`: one statement for the list (balance from the cache, month-to-date debits by correlated seeks on a covering index); the pending filter computes nothing for other users; overdrawn count is one aggregate over the cache (`CountOverdrawn`); handler builds views only for returned users. 14 statements regardless of user count. |
| H1 other routes | `HoldLastSamples` was walking every sample of every hold (78 ms for 100 holds): now one seek per hold. `GET /setups` ran `HoldCharges` (sum of every debit of each hold) only to read user names: now loads users only (130 ms to 0.8 ms). `setupView` read the holder's balance from a per-user ledger scan state: now the cache. Remaining loops in `/me/ledger` and `/admin/audit` query once per distinct hold or actor, not per row, and are bounded by page content. |
| Migration | `0003_ledger_covering_indexes.sql` (new; old ones untouched): drops `ledger_user_period`, adds `ledger_user_period_type (user_id, period, type, amount_mt)`, `ledger_hold_type (hold_id, type, amount_mt)` and `ledger_period_type (period, type, amount_mt)`. BUILD embedsrcs updated. |
| Permanent test | `internal/api/perf_scale_test.go` `TestNFR_ApiUnder500msAtSpecScale`. Default: reduced data set at two sizes (12 users/6 setups vs 48/24), asserts every GET route runs the same SQL statement count at both sizes (new `Store.StatementCount()`), status 200 and under 400 ms. `HOURGLASS_SCALE_TEST=1`: full size, every GET route under 500 ms, times logged. Documented in CONTRIBUTING.md. |
| L1 | Deleted `internal/api/user_test.go-E`; `.gitignore` has `*-E`. |
| L2 | Renamed `TestPermissions_LocalAdmin` to `TestAUTH8_LocalAdminIsPermanent`; contract tests renamed `TestUI2_ContractRoutesMatch` and `TestUI2_ContractResponsesMatchSchemas`; added `TestRULE5_EngineIsGenericOverKinds` (rules) and `TestRULE5_RulesTableTakesAnyKindWithoutSchemaChange` (store). |
| L3 | A6 asserts through the UI that the hold-limit message is shown and the third setup's Acquire control is visible and disabled; the API call stays as proof the server refuses, with a comment. |
| L4 | `readLedger` reverses only when the running balance proves newest-first. |
| L5 | Compose files default to `hourglass:dev`, `hourglass-api:dev` + `hourglass-ui:dev`, no 8091, no `HOURGLASS_E2E_CONTROL`. Overrides `e2e/compose.bundled.yaml`, `e2e/compose.split.yaml` (e2e image, control env, port 8091) used by `e2e/run.sh`. `deploy/smoke.sh` loads only product images + simulator. deploy/README.md, README.md updated; docs/deploying.md already said never to deploy the e2e images (no change needed). |
| L6 | `e2e/run.sh`: `npm ci` when node_modules is missing; Chromium or system Chrome (`E2E_BROWSER_CHANNEL=chrome`, read by playwright.config.ts) used if launching, download only otherwise; messages point to the new `e2e/README.md`. |
| L8 | The API already returns `cost_per_hour_tokens` for every setup (also null-free). Setups page cost cell for held-not-yet-metering setups now shows the cost and the metering reason as secondary text. The admin inventory has no cost column. Test updated. |
| L9 | Band rows on Settings and the Audit filters use a bottom-aligned row (`rowBottom`); the per-row hints moved into the labels (`From pressure N (0 to 1)`, `Action (exact name, e.g. grant.create)`); tests and e2e selector updated. Not re-screenshotted (no screenshot step was in the verification list). |
| Housekeeping | `ci.sh` calls the tools scripts with `bash`; they already have shebangs. `check-provider -h` now exits 0 (both mains). |

## Per-route time before and after (worst of 5 before, worst of 7 after; ms)
| Route | Before (task 40) | After |
|---|---|---|
| GET /session | 0.4 | 0.1 |
| GET /me | 112.6 | 4.2 |
| GET /me/holds | 74.5 | 1.9 |
| GET /me/ledger?period=2026-11 (2.3 MB) | 113.2 | 58.0 |
| GET /me/usage?period=2026-11 | 20.4 | 17.5 |
| GET /pressure | 2.9 | 2.7 |
| GET /setups (dev) | 385.1 | 1.0 |
| GET /setups?selector (dev) | 370.9 | 0.9 |
| GET /setups (admin) | 416.4 | 1.0 |
| GET /admin/users | 672.6 | 146.6 |
| GET /admin/users?status=pending | 738.8 | 101.3 |
| GET /admin/rules | 0.6 | 0.4 |
| GET /admin/settings | 0.1 | 0.1 |
| GET /admin/collector | 0.8 | 1.1 |
| GET /admin/audit | 0.1 | 0.1 |
| public routes (/healthz, /auth/*) | not measured | under 0.4 |
| POST acquire | 45.9 | 1.0 |
| One collector poll over 100 holds | 2815 | 196 |
Full log: `perf-fullsize.log`. The remaining cost of the admin list (about 100 ms) is the lab-wide month totals (a scan of 518k covering-index entries) and the per-user month-to-date debit sums (45 ms); both are well inside the 250 ms target.

## Verification
- `go test -race -count=1 ./...`: pass (`gotest-race.log`).
- `./ci.sh`: exit 0 (`ci.log`; 35 of 35 Bazel tests pass).
- Full-size perf test: pass (`perf-fullsize.log`).
- `deploy/smoke.sh bundled` and `split` with the product images: SMOKE OK (`smoke-bundled.log`, `smoke-split.log`).
- `e2e/run.sh both`: exit 0 (bundled 715 s, split 710 s; `e2e-both.log`).

## Decisions for docs/decisions.md
| # | Decision | Why |
|---|---|---|
| D-A | `users.balance_mt` is maintained incrementally on append; no schema column was added (the spec's users table is unchanged). The cache is valid while `currentPeriod` never moves backwards between appends; `RebuildBalance` is the repair. Between a period boundary and the rollover run (at most one poll) the cache still shows the previous period, as it always did; the admin list now reads the cache instead of summing the ledger. | NFR 500 ms at 200 users, 5-minute polls; no spec change. |
| D-B | The admin users `summary.overdrawn_count` is one aggregate over the cache and does not depend on the `status` filter; per-user figures are computed only for returned users. | Pending filter must not compute for other users. |
| D-C | Migration 0003 replaces `ledger_user_period` with covering indexes and adds two more; the ledger is append-only so write cost is small. | Query plans. |
| D-D | The default-run scale test compares SQL statement counts at two sizes (`Store.StatementCount`) rather than timing. | Stable in CI and catches per-row queries. |
| D-E | Compose stacks default to product images; the end-to-end builds and clock port are added only by `e2e/compose.*.yaml`. | A started-by-hand stack must not expose a clock control. |
| D-F | A bad-looking test label "extralarge" was avoided in `internal/api` (core genericity check forbids provider-specific words); the scale test uses `tier` labels. | check_core_generic.sh. |

## Confidence and gaps
- Not re-screenshotted: L8 and L9 are verified by unit tests only, not by looking at the page.
- The linear-time test uses a ratio with slack (2x plus 20 ms); on a very noisy machine it could flake, though the regression it guards produces about 4x.
- Cache validity on a backward move of the period (timezone change) is a documented limitation (D-A), not tested.
