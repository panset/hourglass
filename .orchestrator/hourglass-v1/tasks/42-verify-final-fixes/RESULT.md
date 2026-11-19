# RESULT 42-verify-final-fixes

Status: done. Verified commit `d8e7e71` (fix commit `096a285`; previous commit `99408df`), fresh clones under
`<scratch>/verify-42/{hourglass,prev}`.

## Verdict

**Not clean: one high-severity regression in the balance cache (money-critical) and one medium regression.** The
incremental `users.balance_mt` maintenance in task 41 is right in every ordinary scenario I tried (debits, rollover
with positive, zero and negative closing balances, a poll at exactly 00:00, grants and resets before and after the
rollover, late past-period entries, first roles, concurrent appends, rollbacks). It goes wrong, permanently, when a
rollover closes **two or more periods** while the user already has an entry in the current period: the cache ends up
holding two allowances (in my test +295 tokens over the ledger). It also stays wrong forever after the configured
timezone moves the period backwards, where the old code healed itself on the next append. In both cases the ledger
itself is correct and `RebuildBalance` repairs the cache, but nothing in production calls `RebuildBalance`.
`users.balance_mt` gates Acquire (`internal/holds/holds.go:204`) and is what the admin list, the top bar and the
Setups holder column show.

Everything else passes: migration 0003 (fresh and upgrade from data made by the previous commit, old migrations
byte-identical), the full-size performance run (every route well under 500 ms; admin users 161 ms worst), response
correctness (identical to the previous commit and to independent sums), `./ci.sh` exit 0, `go test -race`, both
smoke stacks with product images and no clock port, `e2e/run.sh both` (exit 0), and the screenshots.

## Findings

| # | Severity | Finding |
|---|---|---|
| 1 | **High** | Multi-period rollover + an existing current-period entry corrupts the balance cache (regression) |
| 2 | Medium | Timezone moved backwards between restarts: cache permanently wrong (regression); debits in the closed period never carried (pre-existing) |
| 3 | Low | A role-less user's cache keeps last month's balance across a boundary; the admin list now shows it |
| 4 | Low | CI scale test cannot catch a per-user query in the `status=pending` path; `/admin/audit` is timed on an empty table |
| 5 | Low | `readLedger` now silently accepts an oldest-first table; ledger order is never asserted |
| 6 | Low | Audit filter label suggests `grant.create`, an action name that is never written (grants are `ledger.grant`) |
| 7 | Info | Pending-rollover window: the cache shows the previous period until the first poll (documented D-A) |
| 8 | Info | Settings bands: the Multiplier column sits 2 px further right for bands 2-3 than for band 1 |

### 1. High: multi-period rollover with an existing current-period entry (regression)

- **What happens.** `RolloverDue` (`internal/ledger/service.go:281-307`) closes each pending period with
  `AppendLedger(..., currentPeriod = next, ...)`, i.e. it tells the store that the period being opened is "current"
  even when the real current period is later. `AppendLedger` (`internal/store/ledger.go:61-75`) resets the cache
  when it sees the user's first entry of the period it was given and otherwise adds. When the real current period
  already holds an entry (say a December grant) and the rollover closes September and then October, the September
  close resets the cache to October figures, the October allowance sets it, and the November close then sees "an
  earlier December entry exists", so it **adds** to a cache that holds October's balance instead of resetting it.
- **Reproduction** (`evidence/cache_led2_test.go.txt`, `TestV42_DownAcrossTwoBoundaries_GrantBeforeFirstPoll`;
  log `evidence/cache_led2_new.log`): allowance in September; clock to Dec 1 03:00 (service was down); admin grants
  5 tokens; first poll runs the three closes. Result: cache 600,000 mt, December ledger sum 305,000 mt (+295 tokens
  created in the cache); `RebuildBalance` moves it back to 305,000. The same test **passes on `99408df`**
  (`evidence/cache_led2_prev_99408df.log`), where every append rebuilt from scratch.
- **When it is reachable.** Two or more pending periods (the server was down across two month boundaries, or a
  rollover failed in an earlier poll: `RunOnce` logs a `RolloverDue` error and carries on with the poll,
  `internal/collector/collector.go:210-214`, writing current-period debits) plus any current-period entry written
  before the closing rollover: an admin grant or reset in the window between the HTTP server starting and the
  collector's first `RunOnce` (both start concurrently, `internal/app/app.go:239-249`), or the debits of the failed
  rollover case. Rare, but the effect is a wrong balance that never heals, lets an overdrawn user acquire, and shows
  the wrong figure everywhere.
- **Suggested fix.** In `RolloverDue` pass `s.CurrentPeriod()` (the real current period) to `AppendLedger` instead
  of `next`; closing entries of older periods then leave the cache alone and only the entries of the real current
  period touch it. I tried this one-line change in the clone: the scenario above passes and the existing
  `internal/ledger`, `internal/collector`, `internal/store` and `internal/api` suites still pass (reverted
  afterwards). More robust still: end each rollover transaction with `RebuildBalance` for every role holder (one
  indexed sum per user, once a month). Add the scenario as a permanent LED-2 test.

### 2. Medium: timezone moved backwards between restarts (regression for the cache; ledger loss pre-existing)

- **Reproduction** (`TestV42_TimezoneChangeBetweenRestarts/backwards`): zone UTC+2, the December rollover has run at
  Dec 1 00:10 local; restart at Nov 30 23:00 UTC with zone UTC. The current period is now November again. Every
  debit for the next hour is written to **November**, a period that was already closed (its expiry is written), and
  `AppendLedger` adds it to a cache that holds December's balance. When December comes back at 00:00 UTC nothing
  re-closes November (the newest allowance is already December), and the cache keeps those debits.
- **Outcome in the test (12 polls, weight 2, 1x):** November ends at -1,670 mt and is never carried; December ledger
  sum 300,332 mt; cache 298,662 mt, off by -1,670 mt for good. So, in money terms: the **ledger creates money** (the
  user escapes the debits charged into the closed period; pre-existing, same on `99408df`), while the **cache charges
  them** (shows 1.67 tokens less than the ledger). On `99408df` the cache re-synced at the first December append;
  now nothing repairs it (no production caller of `RebuildBalance`; grep shows only tests and the definition).
  During the backward window the cache shows December's balance while the "current period" is November; the old
  code would have shown November's sum (about 0) and blocked Acquire.
- **Forwards** (`.../forwards`, UTC to UTC+2 at Nov 30 22:30 UTC): the November rollover runs at the first poll,
  two hours early; cache and ledger agree throughout; nothing lost or created beyond the period ending early.
- **Suggested fix.** At startup, refuse to start (or log loudly and do not write) when `CurrentPeriod()` is older than
  the newest allowance period, and rebuild every user's cache at startup (200 users, one indexed sum each). Record the
  rule in docs/decisions.md.

### 3. Low: role-less user's cache across a boundary

`TestV42_RolesRemovedAndRestored`: a user whose roles are removed before a boundary is skipped by the rollover
(LED-3, correct), so no entry is appended and the cache keeps last month's figure (200,000 mt) while the new period's
sum is 0. Same on `99408df` (pre-existing), but the admin list now reads the cache (`apiadmin_queries.go:31`), so
the admin Users page shows that stale balance for the user; before task 41 it showed the period sum. Not counted as
overdrawn (needs a role). Restoring the role writes the first allowance, which resets the cache correctly. Fix: when
roles are removed (or in the rollover for every user with a ledger), rebuild the cache, or display no balance for a
user with no role.

### 4. Low: gaps in the CI scale guard

Mutation (`SumLedger` per returned user in `listAdminUsers`, `internal/api/handlers_admin.go`): the default
`TestNFR_ApiUnder500msAtSpecScale` goes red for `/admin/users` (27 vs 63 statements) as intended. But
`/admin/users?status=pending` stayed green: both reduced data sets seed exactly one pending user, so a per-user query
in the pending path cannot change the count. Similarly the scale data set writes no audit rows, so `/admin/audit`
(47 B, 0.1 ms) is not measured at scale. Fix: seed pending users and audit rows in proportion to the size.

### 5. Low: `readLedger` change (check 8)

The old code reversed in both branches (dead branch). The new code reverses only when the running balance proves
newest-first and otherwise returns the table as it is. That fixes the dead branch, but a table that is oldest first,
or whose running balance is inconsistent, is now accepted silently; neither version asserts the order or the running
balance. Suggest `expect(newestFirst).toBe(true)` when there are two or more rows. The other spec changes do not
weaken anything: the label rename follows the UI and keeps the same value assertions; A6 adds a visible-and-disabled
UI check and keeps the API refusal with `hold_limit` and the exact message (the status check is still `>= 400`, as
before).

### 6. Low: audit filter example

`ui/src/pages/admin-audit/index.tsx:74` labels the field "Action (exact name, e.g. grant.create)"; grants are audited
as `ledger.grant` (`internal/ledger/service.go:26`), so typing the example finds nothing. Pre-existing in
`api/openapi.yaml:880,1732`, now more prominent. Use a real action name such as `ledger.grant`.

### 7. Info: pending-rollover window

Between a boundary and the first poll after it (at most one poll interval), the cache still holds the previous period
(e.g. 307,000 mt while the new period's sum is 0). This is the documented D-A behaviour and the rollover reads the
closing balance from the ledger, not the cache, so no money moves. It strictly violates "cache equals the
current-period sum after every step", so I list it.

### 8. Info: Settings band row alignment

The inputs of each band row are vertically aligned (bounding boxes: same y for "From pressure 1" and "Multiplier 1"
at both sizes). The Multiplier column for bands 2-3 sits 2 px further right than for band 1 because the column width
follows the label text. Cosmetic.

## Checks

1. **Balance cache** (`evidence/cache_led2_test.go.txt`, real store, real `ledger.Service`, real collector, fake
   clock; invariant checked for every non-admin user after every step; RebuildBalance no-op after each scenario).
   Pass: debits within a month (50 polls); rollover with positive, zero and negative closing balances (zero ends at
   300, negative at 260 tokens); down across two boundaries with no early entry; poll exactly at 00:00; grant and
   reset before and after the rollover and in the pending window (reset after rollover gives exactly 300); late past
   period debit after the rollover and during a pending rollover (the rollover closes from the ledger, cache right
   afterwards); first role mid-month and during a pending rollover; 2 x 1,000 concurrent debits for the same user and
   for different users (all rows written, cache exact; `_txlock=immediate` serialises writers); rollback of an append,
   including a first-of-period append (cache unchanged). Fail: findings 1, 2, 3; soft mismatches in the pending
   window (finding 7). Same file on `99408df`: only the roles and pending-window cases differ, which shows 1 and 2 are
   regressions.
2. **Migration 0003**: 0001 and 0002 byte-identical to `99408df` (sha256 equal); BUILD embedsrcs lists 0003. Fresh
   database: version 3, indexes `ledger_hold_type`, `ledger_period_type`, `ledger_user_period_type`. Database made by
   the previous commit's code with holds, polls and ledger data (`evidence/migration_make_old_db_test.go.txt`), then
   opened by the new code (`evidence/migration_upgrade_test.go.txt`, log `evidence/migration_upgrade.log`): version 3,
   `ledger_user_period` dropped, integrity_check ok, 31 rows kept, caches equal ledger sums, an append afterwards keeps
   the cache right, reopen is a no-op. The previous code refuses the upgraded file ("schema version 3 is newer than
   this binary (2)").
3. **Performance** (`perf-fullsize.log`, `HOURGLASS_SCALE_TEST=1`, 518,400 samples seeded in 48 s), worst of 7:
   /session 0.2, /me 4.3, /me/holds 2.0, /me/ledger 62.0 (2.3 MB), /me/usage 19.3, /pressure 2.7, /setups dev 1.2,
   /setups?selector 1.0, /setups admin 1.1, /admin/users 160.8, /admin/users?status=pending 106.2, /admin/rules 0.6,
   /admin/settings 0.1, /admin/collector 0.8, /admin/audit 0.1 (empty), public routes under 0.3 ms; POST acquire 1.4 ms;
   one collector poll over 100 holds 198 ms. The default-size test is in `internal/api/BUILD.bazel` (runs in
   `bazel test //...`, seen passing in `ci.log`) and went red under the mutation (finding 4 for its gap).
4. **Responses** (`evidence/responses_test.go.txt`): seeded overdrawn users (negative and exactly zero), three pending
   users created at different times, a user with no role, a rejected user, a user with two holds, a held-not-metering
   setup, a free setup and a disabled one; ran polls. Every balance, month-to-date burn, active-hold count and
   overdrawn flag equals SQL over the ledger and holds tables; overdrawn first; pending list only pending users, oldest
   first (pend-b, pend-a, pend-c); `pending_count` 3; `overdrawn_count` 2; `/setups` holder, hold id, holder balance
   and overdrawn flag match; the admin sees the disabled setup, the developer does not; every setup has a cost. The
   four JSON responses are identical to the previous commit's (timestamps excluded).
5. `./ci.sh` exit 0 in 2 min 44 s (`ci.log`, 35 of 35 Bazel tests); `go test -race -count=1 ./...` exit 0 (`gotest-race.log`).
6. `deploy/smoke.sh bundled` and `split`: SMOKE OK (`smoke-bundled.log`, `smoke-split.log`). `docker compose config`
   (`evidence/compose-*.config.yaml`): published ports 8080, 8081, 8990 only; no 8091 and no `HOURGLASS_E2E_CONTROL`.
   `docker ps` while up (`evidence/docker-ps-*.txt`): images `hourglass:dev` / `hourglass-api:dev` + `hourglass-ui:dev`,
   ports 8080, 8081, 8990; entrypoints `/usr/local/bin/hourglassd` and `/usr/local/bin/hourglass-ui`. `e2e/run.sh both`: exit 0, bundled and split each 1 passed, A0-A9 (`e2e-both.log`).
7. **Pages** (`screens/`, 616 KB; script `evidence/playwright-v42/`): bundled stack, product images, admin signed in
   locally, alice through Keycloak and approved; alice holds lab-03 (pool created, Deployed, metering) and lab-04 (held,
   no pool, not metering); four setups free. Setups at both sizes, developer and admin: every row shows a cost
   ("0.0 / h if acquired" for free rows, "0.0 / h" for both held rows, with the metering reason under lab-04). All
   costs are 0.0 because the sample config's base multiplier is 0 and the simulator's pressure is 1%; the column is
   filled for every state. The hold-limit message and disabled Acquire buttons show for alice. Settings: band inputs
   aligned (`settings-bands-*.png`; finding 8). Audit log: Actor select, Action input and Apply button share one
   baseline at both sizes. Nothing broken.
8. **Spec diff**: see finding 5.

## Files changed

None in the repository. Written: this folder (`RESULT.md`, `PROGRESS.md`, logs, `evidence/`, `screens/`). Clones
under the scratch directory.

## Commands run

`go test` of my tests on both commits; `HOURGLASS_SCALE_TEST=1 go test ./internal/api -run NFR_ApiUnder500msAtSpecScale`;
the mutation run (restored, `git status` clean); `./ci.sh`; `go test -race -count=1 ./...`; `docker compose config`;
`deploy/smoke.sh bundled|split`; Playwright screenshot script; `e2e/run.sh both`.

## Confidence and gaps

- Finding 1 is verified with a deterministic test on both commits; how often its trigger occurs in practice
  (grant in the start-up window, or a failed rollover) is inference.
- Finding 2's figures depend on the burn rate and the size of the timezone shift; the mechanism is verified.
- I did not exercise process restarts of the real binary for check 1; the scenarios use the real services with a fake
  clock, which is what the brief asked.
- Costs on the screenshots are 0.0 because of the sample bands; a non-zero figure was checked in the API test
  (2 and 3 tokens per hour), not on screen.
