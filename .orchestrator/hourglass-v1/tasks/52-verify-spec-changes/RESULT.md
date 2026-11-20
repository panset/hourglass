# 52 Verification of D40 and D41 (commit 1ba15db)

## Verdict
PASS with minor findings. No defect in code or behaviour was found. D40 and D41 do what the decisions say. The findings are stale documentation, one upgrade-behaviour gap and one rename edge case. Nothing blocks.

Setup: clone at `<scratch>/verify-52/hourglass` (HEAD 1ba15db), second clone `old` at 1efc0e9. No Docker; the live server ran on 127.0.0.1:18080 with a temp DB and was stopped afterwards (migration tests used 18081-18085). Ports 8080/8081/8990/8091 and the owner's containers were not touched. My throwaway tests are copied to `my-tests/` as `.txt` (not applied to the repo).

## Findings (numbered)
1. LOW, upgrade behaviour. Migration 0004 silently drops a stored `resource_name_template` and per-setup overrides. An existing deployment that set the template `{{.SetupName}}_RP` (the documented reference scenario) and has no `pool_name_suffix` in its provider config upgrades without an error: its pools are reported as `lab-01_RP`, match no setup, so nothing deploys, release is never blocked, and nothing is logged. `docs/deploying.md` "Upgrade" does not mention it. Reproduce: `old.db` upgraded as in check 5, then `check-provider` shows setups "not reported". Fix: add an upgrade note to `docs/deploying.md` (and release notes), and optionally log a warning at migration or start when the template or an override was dropped.
2. LOW, edge case. Renaming a setup while it is held (admin PUT) is accepted. `deployed_at` and `metering_started_at` stay, metering continues, but the match is now by the new name: the provider's old-named allocation is ignored (not an orphan) and release is allowed at once, so the teardown guard is silently lost. Reproduce: my `TestV52_RenameWhileHeld` (deploy "old", rename to "new", release succeeds while provider still reports "old"). Fix, if wanted: refuse rename of a held setup (409), or document it in `docs/extending.md`/ADM text. Not wrong under the D40 rule, but it is the one place where "exact name" and a live hold can diverge.
3. LOW, stale docs. `docs/progress.md` lines 88-89 still tell the owner to set the resource-name template (`{{.SetupName}}_RP`) in Settings; line 105 says the pool layout must match "the template"; line 50 is history. D40 and D41 are not in the "decisions to review" table (they are owner-directed, but progress.md should say the spec changed). INTENT says keep progress.md current. Fix: update those lines.
4. LOW, stale docs. `docs/ui-notes.md` (transcription of the wireframes) still describes the Resource name column, the "Resource name override" input, the "Resource name template" setting (lines 117-131, 151, 207) and My usage at `/` (line 33). `docs/SPEC.md` still shows SET-4 (known, D40 says the source needs the edit). Wireframe HTML under `docs/wireframes/` also still shows them. Fix: annotate ui-notes.md with a "superseded by D40/D41" note.
5. INFO, coverage gap (acceptance spec). No assertion was added for D41 landing (admin lands on `/setups`) in `acceptance.spec.ts`. Existing alice step does `goto("/")` then expects "My usage", which now exercises the developer redirect. Unit tests (`Gate.test.tsx`) cover the rest, and I verified it in a real browser (check 7).
6. INFO. Setups differing only by case (`lab-02`, `LAB-02`) can coexist; whitespace is trimmed on save (`"lab-02 "` gives 409 conflict). This is consistent with exact matching and vSphere's case sensitivity. An old client that still sends `resource_name_override` or `resource_name_template` now gets 400 `unknown field` (as intended, additionalProperties false).
7. INFO. A local admin opening `/usage` sees the page with "The built-in administrator has no balance, holds or ledger." (sane). Admin-only (non-local) users see "0 tokens" in the top bar. Neither is new.

## Check 1: suites
- `go test -race -count=1 ./...`: exit 0, 30 packages ok, 491 top-level Test functions (1093 passing test results including subtests), 0 failures.
- `./ci.sh`: exit 0 (SPDX, core-generic, licenses, go build, no-tag build, vet, gazelle diff clean, `bazel build //...`, `bazel test //...`: 35 of 35 pass). UI: 15 files, 153 tests pass; typecheck and `//ui:api_schema_drift_test` pass. (A first ci.sh run failed only because my own temporary test file changed a BUILD file; removed and re-run clean.)

## Check 2: D40 completeness
Greps over the whole tree (excluding lockfiles, node_modules, SPEC.md, wireframes) for `ResourceName(`, `resource_name_override`, `resourceName`, `resource_name`, template, `NameTemplate`, `SetupName`, override. Remaining hits, classified:
- Legitimate: `Allocation.ResourceName` (`pkg/hourglass/types.go`, collector, match.go, `store.Orphan`), orphan `resource_name` in `api/openapi.yaml:1640`, `schema.gen.ts:1160`, `admin_system.go:54`, UI column "Resource name reported" (`ui/src/pages/admin-setups/index.tsx:146`, orphans); vsphere `pool_name_prefix/suffix` (`providers/vsphere/config.go`, `vsphere.go`); migration `0004_drop_resource_name_override.sql`, `0001_init.sql:31` (history, byte-identical), `migrate0004_test.go`; seed rejection key `RemovedOverride` in `internal/app/seed.go:39-41,102-103`; `tools/check_core_generic.sh:22` (guard regex for `_RP|resource_name_template`); `SetupName` fields on Hold; docs that describe the new rule (`extending.md:275`, `deploying.md:221,277`, `deploy/README.md:36`, `examples/custom/README.md`).
- Leftovers: only findings 3 and 4 (docs). No code, API, OpenAPI, generated schema, UI source, seed format or sample file implements or mentions the template or override. `text/template` is not imported anywhere in the core. `ui/src/api/schema.gen.ts` is in sync (drift test passes).
- Public `pkg/hourglass` surface changes for extension authors (git diff 1efc0e9..HEAD): (a) `SetupState.ResourceName` removed; (b) `hourglasstest.Resource(...)` option removed, `SnapshotBuilder.Setup` no longer sets a resource name; (c) `Snapshot.AllocationFor(name)` now matches `Allocation.ResourceName == setup name` (returns false only if setup not enabled or not reported); (d) `Allocation.ResourceName` comment changed; (e) `doc.go` text. No additions. `docs/extending.md` matches all of this (struct listing, provider section, check-provider sample output, vsphere keys); no remaining `hourglasstest.Resource` anywhere. `examples/custom` (README, `seed.yaml`, filedir) reflect the exact-name rule and show how to map names in `List`; their tests pass.

## Check 3: matching rule (my tests, real collector + holds service + default policy `start_on_allocation:true, start_after:off`)
Pass (file `my-tests/verify52_collector_test.go.txt`):
- Allocation named exactly like a setup (`Lab-01`): hold gets `deployed_at` and `metering_started_at`; later poll debits.
- `lab-01` (case), `"Lab-01 "`, `" Lab-01"`, `Lab-01_RP`, `x-Lab-01`, `Lab-0l`: no deploy, no metering. They also do not block release.
- Release refused (`still_deployed`) while reported; allowed after the allocation disappears.
- Orphan recorded only for the exact-name setup with no hold (`free`), not for `FREE` or `nosuch` (ignored).
- Two setups cannot share a name (store UNIQUE; API 409, also after trimming). Case variants are distinct (finding 6).
- Rename while held: see finding 2.

## Check 4: vsphere naming (my tests, in-process govmomi simulator and existing layer-2 tests)
`my-tests/verify52_vsphere_test.go.txt`, pools `hg-lab-01_RP` (with nested `hg-nested_RP`), `lab-02`, `hg-lab-03`, `lab-04_RP`, `hg-_RP`:
- neither: all six pools by own name; suffix `_RP`: `hg-`(empty prefix rule; remainder `hg-` is non-empty so it is reported), `hg-lab-01`, `hg-nested`, `lab-04`; prefix `hg-`: `lab-01_RP`, `lab-03`, `nested_RP`, `_RP`; both: only `lab-01` and `nested`.
- Pools not matching are not reported; with both affixes `lab-02` (named like a setup, no affix) is NOT reported; exact prefix+suffix pool (empty remainder) skipped; nested pools work; `pool` label carries the original name and `parent` is original.
- The pressure source accepts the same block (and returns a value against the simulator); unknown keys (`pool_name_prefx`, `pool_name_template`, `resource_name_template`) are rejected by provider and pressure source.
- `hourglassd check-provider` with `static` provider prints setup names under "matched setups", "enabled setups not reported", "orphans", "ignored allocations" (exact-name matching only: `lab-01` matched, `lab-01_RP` and `Lab-02` ignored).

## Check 5: migration 0004
- Fresh DB (sample seed): versions 1-4, 6 setups, 2 rules, no `resource_name*` column.
- Upgrade: DB made by the real 1efc0e9 store code (setup with override `custom-pool` and labels, setup without, stored `resource_name_template` and `default_weight`, rate rule, OIDC developer, hold, three ledger rows). Opened by the new `hourglassd`: both setups, all labels, the rule, the hold, ledger rows (5000/-1200/-300, balance 3500) and `default_weight` intact; `resource_name_override` column and `resource_name_template` row gone; schema_migrations 1-4; server started and polled.
- Old migrations 0001-0003 are byte-identical to 1efc0e9 (`cmp`).
- Old binary on the upgraded DB: exits with `store: database schema version 4 is newer than this binary (3)`; no change to the DB.

## Check 6: seed
- Seed with a setup carrying `resource_name_override`: start fails with `seed: setups[1] (second): resource_name_override is no longer supported: ... rename the setup or configure the provider's own name mapping`; the first (valid) setup in the same file was NOT imported (0 setups). `deploy/seed.sample.yaml` imports 6 setups and 2 rules.

## Check 7: D41 in Chrome (Playwright, channel `chrome`, 1280x800)
- Local admin: `/` to `/login`; sign-in to `/change-password`; after the change lands on `/setups`, nav highlight "Setups" (`aria-current`).
- `/` as admin (also admin-only and admin+developer users) lands on `/setups`; developer lands on `/usage`, heading "My usage", nav shows "My usage, Setups", highlight "My usage". `/usage` as admin shows My usage with highlight "My usage". `/` never renders a page of its own.
- Deep link `/admin/users` as admin stays on `/admin/users` (highlight "Users"); also `/admin/settings`, `/admin/audit`.
- Developer on `/admin/users`, `/admin/settings`, `/admin/audit`: "Page not found", no admin API request is made by the page, and direct `curl` of `/api/v1/admin/users` gives 403 (role-less also 403).
- Role-less user: every URL tried (`/`, `/admin/users`, `/admin/settings`, `/setups`, `/usage`, `/admin/audit`, `/xyz`, `/login`, `/change-password`, `/unauthorized`) shows only the not-authorized page (no nav, no data).
- `/login` and `/change-password` for a signed-in admin or developer redirect to the role landing page.
- Back button: `goto('/')` after `/admin/audit` ends on `/setups` and Back returns to `/admin/audit`, then `/setups`; no loop (redirect uses replace). Developer: same, no loop.
- Screenshots: `screens/admin-landing.png`, `screens/developer-landing.png`.
- Developer fixtures were inserted directly into the temp DB (users, roles, sessions). The developer showed "0 tokens / Overdrawn" because no allowance row was inserted; unrelated to D41.

## Check 8: Settings and Setup inventory
- Settings has no template field (labels: enforcement, teardown, allowance, max setups per developer, poll interval, bands, default weight, display label keys); saving max setups 2 to 3 sends a PUT without `resource_name_template` (200), persists across reload; set back to 2.
- Setup inventory table has no resource-name column (only the orphans table's "Resource name reported"); edit panel has Name, Labels, Details, Enabled, no override field; editing labels (size=xlarge, added site=rtp) sends a PUT without `resource_name_override` (200) and persists after reload. Screenshots: `screens/settings.png`, `screens/setup-inventory.png`.

## Check 9
Route count unchanged at 30: 30 operationIds in `api/openapi.yaml`, `router_test.go:45` asserts 30, `contract_test.go:727` requires at least 30. The permissions body only dropped the template key. `uiRouteLiterals` in `registration_test.go:105` now includes `/usage` (diff 1efc0e9..HEAD). All pass in check 1.

## Check 10: acceptance spec (read only, not run)
Diff since 1efc0e9: `reloadHome` and one poll loop go to `/usage` instead of `/`; the template field fill and its post-reload assertion were replaced by an assertion on "Display label keys" = `size` (the same step still asserts poll interval, band, enforcement after reload); log text adjusted; POOL_A comment. No assertion removed or loosened; the saved-and-reloaded check kept its strength with a different field. Coverage not added for the admin landing (finding 5).

## Confidence and gaps
Not run: the acceptance spec (needs Docker), `./ci.sh` on a second platform, real vCenter. Developer sessions were synthesized in the DB rather than created through OIDC. UI test counts come from Vitest (153) and Go from `go test` output; the pre-existing `docs/wireframes` HTML was not inspected beyond grep.
