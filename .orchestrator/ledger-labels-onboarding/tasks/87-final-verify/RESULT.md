# 87 Final independent verification: result

**Verdict: not ready, narrowly.** I found no product defect. All 26 requirement ids have tests named after them, and those tests ran and passed in uncached runs. Every deliberate break in scope turned a named test red. `./ci.sh` passes, and so does the uncached Bazel run. The compatibility section is met. Two things block release:

- **F1 (high, test harness).** The end-to-end suite can silently reach the owner's host processes. The 85b gate runs did reach the stacks (evidence below), so this does not invalidate them. It still has to be fixed or explicitly accepted by the owner.
- **F2 (medium).** `docs/progress.md` is untrue in three places, and the owner's "Done means" requires it to be current.

Both fixes are small. Findings: **1 high, 1 medium, 4 low.**

HEAD everything applies to: `5b0d35ba483a5e5b7aa33ac07169f054f7f9f16b` (branch `ledger-labels-onboarding`). `git diff` was empty at the end, and the tracked tree is clean.

## Findings

| id | severity | file:line | failure scenario | suggested fix |
| --- | --- | --- | --- | --- |
| F1 | high | `e2e/browser/playwright.config.ts:23`; `e2e/browser/tests/acceptance.spec.ts:17-19`; `e2e/browser/tests/integration.spec.ts:24-25` | The suite uses `http://localhost:8080` (app) and `http://localhost:8091` (test clock). The owner's `hourglass` and `hd` listen on IPv4 `127.0.0.1:8080` and `127.0.0.1:8091`. Docker Desktop publishes the stack on the IPv6 wildcard `*:8080` / `*:8091`. Node resolves `localhost` to `::1` first, then `127.0.0.1`. **I proved the fallback:** with a listener on `127.0.0.1:18080` only, both Node `fetch` and Playwright's Chromium reached it through `http://localhost:18080` (`serverAddr` 127.0.0.1). So whenever the Docker listener is absent or refuses, test traffic silently goes to the host processes: a container that crashes or restarts mid-run, a port that was not published, or a step run before `up --wait` succeeds. Clock `PUT`s would go to `hd`, and the admin and SSO flows to the owner's server. The result is most likely a confusing failure, but it could be a false pass for any check those servers happen to satisfy. It also sends test credentials to the owner's processes. | Point the suite at an address only the stack holds: publish to and use `127.0.0.1` on ports nobody else uses, or `http://[::1]:…`. Alternatively, have `e2e/run.sh` refuse to start when anything else listens on 8080, 8081, 8091 or 8990 (`lsof`), and add a preflight that asks the stack for something only it can answer, such as the container's hostname. |
| F2 | medium | `docs/progress.md:84`, `:90`, `:69` | **:84** still says the pre-approval commands are marked `<!-- 86b: run and correct -->`, have not been run, and "the job is not done until they have been". In fact the markers are gone (none in `docs/deploying.md`), 86b ran the commands, and I re-ran them; they work. **:90** says `docs/extending.md` "still says" that `user.approved` comes only from an admin; `docs/extending.md` was already updated in `45b1d65`. **:69** says "(D69 to D78)" and that the spec edits "are to be recorded in decisions.md by the orchestrator"; D79 records them. A reader of the progress page concludes the job is unfinished and that a documentation gap exists that does not. | Delete :84 and :90 and change :69 to "D69 to D79 … listed in D79". |
| F3 | low (test gap) | `internal/api/upgrade_compat_test.go:141`; `testdata/upgrade-d96a950/responses.json` | Every recorded session (`session_dev`, `session_admin`, `session_norole`) has a non-empty `name`. A change that made the session's `name` carry the display name instead of the stored name therefore passes UPG-2: break `upg2-session-name` stayed green. The design requires that "`name` stays in the response with its current meaning". The AUTH-9 tests do catch this break (`TestAUTH9_SessionAndAPIShareOneDisplayNameFunction` and `TestAUTH9_SessionDisplayName` went red; `break-upg2-session-name-others.log`), so the requirement is covered, just not by UPG-2. | Optional: record one more user with an empty name in the fixture. |
| F4 | low | `pkg/hourglass/events.go:16` | The public event constant's comment says `user.approved` fires when "an admin assigns a first role". It now also fires for a pre-approved sign-in. A hook author reading the package doc misses that case. `docs/extending.md` is correct. | Update the comment. |
| F5 | low | `ui/src/pages/admin-users/preapprovals.test.tsx:155` (10 occurrences) | The test emails use `x.com`, a real registered domain. They are not real people's addresses, but the brief asks for no real addresses and the rest of the branch uses `example.com` / `example.test`. | Replace with `example.test`. |
| F6 | low (informational) | `testdata/upgrade-d96a950/hourglass.sqlite3` | The committed fixture holds the local admin's argon2id hash of the unit-test password and four hashed session ids that expired an hour after creation. Neither is a live credential (the README says the password is the test one, and `session.go` stores hashes). A real password must never be used to regenerate this fixture. | None needed. Optionally state this in `testdata/upgrade-d96a950/README.md`. |

Checked and not a defect: `docs/SPEC.md` and the design doc are unchanged since `414b51d`. `e2e/browser/tests/integration.spec.ts` is unchanged since `d96a950`. `testdata/upgrade-d96a950/openapi.yaml` is byte-identical to `git show d96a950:api/openapi.yaml`.

## End-to-end: which server was tested

**Scope change (owner's direction, relayed by the coordinator):** I did not complete the full reruns. `e2e/run.sh both` was started at 16:31. I stopped it during bundled step A5, when the change arrived, and tore its stack down (`docker compose … down -v`). `e2e/run.sh integration` was not run. **The gate is task 85b's passing runs at commit `b1872cc`:**

- `both`: exit 0, bundled 16 of 16 steps and split 16 of 16.
- `integration`: exit 0, 11 of 11 steps.

**Premise checked:** `git diff b1872cc..HEAD --stat` lists only `docs/deploying.md`, `docs/progress.md` and `.orchestrator/` files. No code or test changed after the gate.

Evidence of which server answered:

- **Base URLs:** app `http://localhost:8080`, clock `http://localhost:8091`, simulator `http://localhost:8990`, Keycloak `http://localhost:8081` / `keycloak.localhost:8081`. The integration scenario uses the same app and clock URLs.
- **Published ports** (`e2e-bundled-ports.txt`, taken during my bundled run): `hourglass-bundled-hourglass-1` (image `hourglass-e2e:dev`) published `0.0.0.0:8080` and `0.0.0.0:8091`. `lsof` showed Docker on `*:8080`, `*:8091`, `*:8990` and `*:8081` (IPv6, dual stack). The owner's `hourglass` (pid 56366) was on `127.0.0.1:8080` and `hd` (pid 6357) on `127.0.0.1:8091`. Split mode publishes the same ports, from `hourglass-ui` (8080) and `hourglass-api` (8091). Integration uses the bundled files plus an override that adds no ports.
- **Address chosen:** Node resolves `localhost` to `[::1, 127.0.0.1]`. A connect probe to `localhost` on 8080, 8091, 8990 and 8081 while the stack was up landed on `::1` every time, which is the Docker listener. The host processes have no IPv6 listener, so `::1` can only be the stack. The fallback to `127.0.0.1` when `::1` refuses is F1.
- **Requests only this branch can answer, in the container's own log:**
  - Bundled (my partial run, `e2e-bundled-partial-container.log`): the container logged the browser's `GET` and `POST /api/v1/admin/preapprovals` with status 200 during A2p. That route exists only on this branch. It also logged 68 `/api/v1/session` requests.
  - Split (85b's final run, `e2e/out/split/containers.log`): `hourglass-api-1` logged 9 `/api/v1/admin/preapprovals` and 138 `/api/v1/me/ledger` requests. It also logged `compacted ledger` twice, which can only happen if the 9-day clock move on `:8091` reached that container, not `hd`.
  - Integration (85b, `e2e/out/integration/containers.log`): the container logged `/api/v1/admin/preapprovals` and 13 `/api/v1/me/ledger` requests among 910 request lines.

  85b's own bundled artifacts in `e2e/out/bundled` were overwritten when my run started (`run.sh` clears the directory), so for bundled mode the container evidence is from my partial run on HEAD, with the same image and ports.

**Conclusion:** in every mode the gate runs talked to the stack built from the tree. The configuration leaves the host processes reachable by fallback (F1).

## Exit codes

| run | exit | file |
| --- | --- | --- |
| `./ci.sh` | 0 (Bazel: 39 of 39 pass, cached) | `ci.exit`, `ci.log` |
| `bazel test //... --nocache_test_results` | 0 (39 of 39 executed, all pass) | `bazel-uncached.exit`, `bazel-uncached.log` |
| `go test -count=1 -v ./...` on an exact copy of HEAD | 0 (all 80 named Go tests `--- PASS`) | `gotest-v.log` |
| `npx vitest run --reporter=verbose` (ui) | 0 (218 of 218 pass; 47 carry UI-8 to UI-11) | `vitest-verbose.log` |
| `npx tsc --noEmit -p tsconfig.json` (ui `typecheck`) | 0 | `tsc.log` |
| `tools/check_core_generic.sh` | 0 (runs inside `./ci.sh`) | `ci.log` |
| `e2e/run.sh both` | skipped at the owner's direction; my started run was stopped at bundled A5, so no exit code. Gate: 85b at `b1872cc`, exit 0 | none (no `e2e-both.exit` written) |
| `e2e/run.sh integration` | skipped at the owner's direction. Gate: 85b at `b1872cc`, exit 0 | none (no `e2e-integration.exit` written) |

UI lint: `ui/package.json` has no lint script (only `dev`, `build`, `test`, `typecheck`, `gen:api`), so I ran `typecheck` instead.

## Deliberate breaks (item 2)

All breaks were made in a scratch copy of HEAD (`rsync` of the tree, with its own `.git`). That way `./ci.sh` and the Bazel run in the real tree were never disturbed. Each break was one `perl` edit, followed by `go test -count=1` on the targeted tests and a `git checkout` in the copy; the copy's diff was empty after each one. Logs are `break-<name>.log` in this folder. The baseline was green (`break-baseline.log`).

| break | change | red tests |
| --- | --- | --- |
| upg1-migration | 0007 also adds 1000 mt to every user with ledger rows | `TestUPG1_ReleaseDatabaseUpgradesAtStartAndKeepsEveryNumber` |
| upg1-holdcharges | `HoldCharges` ignores rolled-up rows | `TestUPG1_ReleaseDatabaseUpgradesAtStartAndKeepsEveryNumber` |
| upg2-no-pending-event | no `user.pending` for a new SSO user | `TestUPG2_ResponsesAndSignInDifferFromTheReleaseOnlyByAddedFields` |
| upg2-ledger-order | default ledger oldest first | `TestUPG2_…` |
| upg2-setups-field | `matched_rule_selector` renamed | `TestUPG2_…` |
| upg2-session-name | session `name` = display name | **stayed green** (F3); AUTH-9 tests went red |
| upg3-param | `period` parameter removed from `/me/ledger` in `api/openapi.yaml` | `TestUPG3_NoRouteParameterOrFieldOfTheReleaseIsRemoved` |
| pre3-recheck | status re-check in the approval transaction removed (`internal/api/preapproval.go`) | `TestPRE3_ApplyPreapprovalRechecksTheUserInsideItsTransaction`, `TestPRE3_ConcurrentApplyPreapprovalApprovesOnce` |
| pre2-ascii | `NormalizeEmail` uses `strings.ToLower` (Unicode folding) | `TestPRE2_FoldsASCIICaseOnlyLookAlikesNeverMatch` |
| led13-rows-first | cursor applied to stored rows before merging | `TestLED13_PagingWithMoreActiveHoldsThanLimit` (the walk never ends) |
| led14-anchor-current | balances anchored to the user's current balance cache | `TestLED14_NewestBalanceInAPastPeriod`, the only LED-11 to LED-15 test that went red |

## Existing tests (item 3)

I checked `git diff d96a950..HEAD` for the 18 test, spec and e2e files that existed at `d96a950`. No assertion was weakened or removed:

- **Field-list pins:** AUTH-5 ×3 and the setups and ledger key sets gained exactly `display_name`, `label_index` and `entry_count`. The route count went from 40 to 43, and the schema version and migration count from 5 to 7.
- **`TestNFR_Migration0003UpgradesVersion2Database`:** this test now writes raw inserts plus `RebuildBalance` instead of `AppendLedger` on a v2 schema. That is needed because `AppendLedger` reads `rollup_count`, and its assertions stand or gained the two new indexes.
- **`Gate.test.tsx`:** now expects "Hello, Dana Dev", which is UI-10.
- **`acceptance.spec.ts`:**
  - Only the `readLedger` header lines were removed. The function now selects Every poll and walks every page with Older, and it keeps the same per-row balance and multiplier assertions.
  - The 85b change to A7 adds a wait for "Current holds · N of 2 allowed" before counting rows. That is stronger: an unrendered section used to count as 0.
  - `openUsage` (added on this branch) waits for the heading or the SSO link.
- **`integration.spec.ts`:** unchanged (empty diff).

## Documents (item 6)

- **`docs/deploying.md`:**
  - Upgrade entry: no down migration, older binary refuses, backup, first start longer for the index build (lines 575-585).
  - Data retention, ledger roll-up and disk space with offline `VACUUM` (587-617).
  - Timezone note (132).
  - Configuration reference row and redacted view: the code logs the path (`internal/config/config.go`) and `pre-approved users file read` with the count (`internal/app/app.go:173`).
  - "Pre-approving SSO users", including the three D6 statements: email claim only from a provider that controls addresses, may grant admin, hand-editing SQLite unsupported (318-322).
  - No `86b:` marker left.
  - The step limits it quotes (20 steps, 2,000 rows) match `collector.CompactStepsPerCycle` and `store.LedgerCompactionStepRows`.
- **Guide commands re-run** against my own container of `hourglass:dev` built from HEAD, on port 18080, then torn down (`guide-commands.txt`): the file example, the first-run password change, the three calls and the `split`/`jq` loop all worked as written. The only changes were the host port (8080 is taken by the owner's process), the container name and the volume name.
- **`docs/progress.md`** is untrue in three places (F2).
- **`README.md`, `docs/ui-notes.md` and `deploy/config.sample.yaml`** (commented example, lines 14-21) are current.
- **Secrets:** a Haiku `secret-check` pass over `d96a950..HEAD` (the grep scan plus a read, including the fixture database queried read-only and the realm file) found no live credential. Only `dev-only-*` development passwords, placeholder passwords and `example.*` addresses appear, apart from F5 and F6.

## Decisions D69 to D79 (item 7)

No entry contradicts the design or the code. Spot checks against the code:

- D70(e) 2,000 rows and D70(d) 60 days (`internal/store/ledger_compaction.go:23,30`); D70(c) index names (asserted in `store_test.go`).
- D71(a) 20 steps (`internal/collector/collector.go:40`).
- D74(e) 12 values and 8 keys (`ui/src/pages/setups/LabelChips.tsx:10`).
- D75(d) 240 px (`ui/src/shell/TopBar.module.css:54`).
- D76(d) `email_verified` parsing (`internal/auth/oidc.go:322-336`).
- D76(f) a failed step on a new user still gets `user.pending` (`oidc.go:282`).
- D77(a) ASCII-only folding (`internal/api/preapproval.go:62`).
- D79 lists the spec edits.

I found no choice in the code that is unrecorded. The only stale reference is progress.md's "(D69 to D78)" (F2).

## Compatibility section of the design (item 8)

- **Configuration: met.** The one new optional key defaults to empty. `TestPRE6_NoFileStartsAsBefore` passes, and `TestUPG1_…` starts with a config of the release's keys only.
- **Database: met.** 0006 and 0007 are additive except for the trigger. There is no down migration, and the `deploying.md` Upgrade entry says an older binary refuses the database, rows cannot be expanded, the way back is the backup, and the first start may be longer. UPG-1 upgrades the real `d96a950` database at start (two breaks went red).
- **API: met.** `TestUPG3_…` checks that no route, parameter or field of the release's contract is missing (break went red). `TestUPG2_…` and `TestLED11_…` check the unparameterised ledger, setups and session responses, which differ only by `entry_count`/`next_cursor: null`, `label_index` and `display_name` (three breaks went red).
- **Numbers: met.** `TestLED8_EveryReaderSameAfterCompaction` covers SumLedger, HoldCharges, LedgerPeriodTotals, LedgerStats, CountOverdrawn, BalanceThrough and the other readers. UPG-1 compares balances and hold charges after compaction.
- **What users can see: met.** Every poll shows single debits and multipliers for the last week: e2e `readLedger` asserts it, UI-8 tests cover it, and per-poll detail stays in `usage_samples`, whose pruning is unchanged.
- **Sign-in: met.** With no pre-approvals a new user is `pending` with `user.pending` (`TestPRE4_NoEntriesNoFileSignInAsBefore`, `TestUPG2_…` sign-in subtest, break `upg2-no-pending-event` went red).
- **Hooks: met.** `pkg/hourglass` is unchanged (no new event type), and `user.approved` is emitted with the `setUserRoles` payload (`TestPRE4_AuditEventsAndFirstUse`). Only the package comment is stale (F4).
- **Timezone: met.** There is a note in `deploying.md:132`, and `TestLED7_TimezoneChangeLeavesRolledUpRowsAlone` passes.
- **Disk: met.** `deploying.md` "Disk space" covers the slower growth and offline `VACUUM`.

## Commands run

- `./ci.sh`.
- `bazel test //... --nocache_test_results`.
- In a scratch copy of HEAD: `go test -count=1 -v ./...` and the 16 break runs (`tools/brk.sh` in the scratchpad).
- In `ui/`: `npx vitest run --reporter=verbose` and `npx tsc --noEmit -p tsconfig.json`.
- `e2e/run.sh both`, stopped at the owner's direction, with its stack torn down.
- The `docker run` and `curl` guide commands on port 18080, torn down afterwards.
- A Node and Playwright localhost-fallback probe on `127.0.0.1:18080` (my own listener).
- A Haiku secret-check agent over the diff.

## Files changed

Only this folder:

- `PROGRESS.md`, `RESULT.md`, `REQUIREMENTS.md`
- `ci.exit`, `ci.log`, `bazel-uncached.exit`, `bazel-uncached.log`
- `gotest-v.log`, `vitest-verbose.log`, `tsc.log`, `break-*.log`
- `e2e-both.log`, `e2e-bundled-ports.txt`, `e2e-bundled-partial-container*.log`, `e2e-teardown.log`
- `guide-commands.txt`

Side effects outside the folder (untracked or temporary):

- `e2e/out/bundled` now holds my partial run instead of 85b's bundled artifacts. 85b's logs in its task folder are intact.
- The secret-check agent briefly created `-wal` and `-shm` files next to the fixture database while reading it, and removed them. The fixture is unchanged and `git status` is clean.
- I did not touch the `hourglass-tls` or `platform-lab` containers, and I sent no request to the host processes on 8080 and 8091.

## Confidence and gaps

- **High confidence:** the requirement table, the breaks, the CI and test runs, the documents and the compatibility section.
- **Bundled-mode evidence of which server answered** comes from my partial run on HEAD (through A5), not from 85b's bundled run, whose container log is gone. The ports, image and compose files are the same.
- **I did not rerun e2e at HEAD** (owner's direction). That is acceptable because only documentation changed since `b1872cc` (verified).
- **F1's trigger conditions** (container crash, missing publish) are inferred. The fallback mechanism itself was demonstrated.

## Decisions for docs/decisions.md

None.
