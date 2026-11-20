# RESULT: 50 remove the resource-name template and override (D40)

Status: done (see "Verification" for each command's outcome).

## What changed, in one paragraph
The core now matches a provider's allocation to the setup whose name equals the allocation's
`ResourceName`, and nothing else. The `resource_name_template` setting, its template sandbox, the
per-setup `resource_name_override`, `settings.ResourceName` and `SetupState.ResourceName` are
gone. Migration 0004 drops the column and deletes the stored setting. The `vsphere` provider
gained `pool_name_prefix` / `pool_name_suffix`, and the reference scenario uses
`pool_name_suffix: _RP`.

## Files changed, by area
- Store: `internal/store/setups.go` (Setup has no override; queries drop the column),
  `internal/store/migrations/0004_drop_resource_name_override.sql` (new),
  `internal/store/migrate0004_test.go` (new, upgrade test), `internal/store/store_test.go`
  (schema columns; migration counts 3 to 4; new helper `openAtVersion` builds a database at an
  older schema from the embedded migrations; the 0003 upgrade test now uses it instead of undoing
  0003 by hand, which no longer works once 0004 exists), `internal/store/BUILD.bazel`.
- Settings: `internal/settings/resourcename.go` and `resourcename_test.go` deleted;
  `settings.go` (key, field, default, validation, JSON encode/decode, Load check removed);
  `settings_test.go`; `BUILD.bazel`.
- Core services: `internal/collector/collector.go`, `internal/collector/match.go` (`SetupRef`
  has no ResourceName; `OrphanNames` takes setup names), `internal/collector/collector_test.go`;
  `internal/holds/holds.go` (teardown check uses the setup name), `internal/holds/holds_test.go`
  (`TestHOLD2_ResourceNameUsesOverride` replaced by `TestHOLD2_ReleaseMatchesAllocationBySetupName`, D40).
- App: `internal/app/seed.go` (a seed with `resource_name_override`, any value including null, is
  rejected: "resource_name_override is no longer supported: a provider reports a setup under the
  setup's exact name, so rename the setup or configure the provider's own name mapping"),
  `internal/app/seed_test.go` (new `TestSET7_SeedWithRemovedOverrideKeyRejected`),
  `internal/app/checkprovider.go` (prints setup names; section renamed "enabled setups not
  reported"), `internal/app/checkprovider_test.go`.
- HTTP API: `api/openapi.yaml` (removed `SetupAdminFields.resource_name`,
  `SetupAdminFields.resource_name_override`, `SetupInput.resource_name_override`,
  `Settings.resource_name_template`; Orphan example `lab-09`), `internal/api/admin_views.go`,
  `admin_system.go`, `handlers_admin.go` (SetupInput, audit shape without
  `resource_name_override`, settings key list), `handlers_user.go`, `user_views.go`
  (`reported` matches the setup name and no longer returns an error); tests
  `admin_setups_test.go` (`TestSET4_ResourceNameFromTemplateAndOverride` removed),
  `admin_system_test.go`, `admin_users_test.go`, `contract_test.go`, `permissions_test.go`,
  `user_test.go` (`TestSET6_...` now also asserts that only the exact name marks a setup deployed).
- Public extension API: `pkg/hourglass/types.go`, `pkg/hourglass/doc.go`,
  `pkg/hourglass/hourglass_test.go`, `pkg/hourglasstest/builders.go`,
  `pkg/hourglasstest/hourglasstest_test.go`.
- vsphere extension: `providers/vsphere/config.go`, `vsphere.go`, `vsphere_test.go` (layer 2:
  `TestEXT3_ProviderPoolNamePrefixAndSuffix`, `TestEXT3_PressureSourceAcceptsPoolNameKeys`, config
  parsing), `simulator_test.go` (layer 3: `TestEXT3_SimulatorPoolNameSuffix`).
- UI: `ui/src/api/schema.gen.ts` (regenerated), `pages/admin-settings/{index.tsx,form.ts,admin-settings.test.tsx}`
  (template field and section removed), `pages/admin-setups/{SetupForm.tsx,index.tsx,admin-setups.test.tsx}`
  (override field and "Resource name" column removed), `pages/admin-shared/fixtures.ts`,
  `pages/setups/setups.test.tsx`. No route or landing-page change.
- Deploy / e2e: `deploy/config.sample.yaml` (`pool_name_suffix: _RP` in the provider block only),
  `deploy/seed.sample.yaml` and `deploy/README.md` (comments), `e2e/browser/tests/acceptance.spec.ts`
  (A0 no longer fills a template; it now asserts the label key after reload; pools still `<setup>_RP`).
  `deploy/smoke.sh` needed no change (it never set the template).
- Docs and examples: `README.md`, `docs/deploying.md`, `docs/extending.md`,
  `examples/custom/README.md` (one sentence on provider-side name mapping), `examples/custom/seed.yaml`.
  `CONTRIBUTING.md` had no mention and is unchanged.
- Tooling: `tools/check_core_generic.sh` rule4: `_RP` and `resource_name_template`
  (case-insensitive) in any core file, tests included; only the 0004 migration and its upgrade
  test are exempt (they must name the setting they delete).

## The migration
`internal/store/migrations/0004_drop_resource_name_override.sql`:
`ALTER TABLE setups DROP COLUMN resource_name_override;` and
`DELETE FROM settings WHERE key = 'resource_name_template';`. Runs in the existing per-migration
transaction. Old migrations are untouched. Even without the DELETE, `settings.Load` ignores
unknown keys, so a stored template is never read.

## Public API changes (for extension authors)
- `hourglass.SetupState.ResourceName` removed (breaking). A setup's allocation is the one whose
  `Allocation.ResourceName` equals `SetupState.Name`.
- `Snapshot.AllocationFor(setupName)` kept; it now matches `ResourceName == setupName`.
- `hourglasstest.Resource(name)` SetupOption removed; `SnapshotBuilder.Setup` no longer sets a
  resource name.
- Comment on `Allocation.ResourceName` now says it belongs to the setup with exactly this name.
- HTTP API: fields listed above removed; a `SetupInput` or `Settings` body that still sends them
  is rejected as an unknown field (400 `validation`, `field` = the key), by the existing strict
  decoders.

## Decisions for docs/decisions.md
- vsphere: the pressure source accepts and ignores `pool_name_prefix`/`pool_name_suffix` (one
  shared parser; simpler than per-kind parsing). The sample still gives the suffix only to the
  provider block. The keys are not part of the connection key, so blocks that differ only in
  them share one vCenter session.
- vsphere: every allocation now carries a `pool` label with the pool's own name, whether or not
  a prefix/suffix is configured.
- `check-provider` output wording: `matched setups` lists setup names; `expected but not
  reported` is now `enabled setups not reported`.
- Seed: `resource_name_override` is kept in the decoder only to reject it with a clear message.

## Verification
- `go build ./...`: exit 0.
- `go test -race -count=1 ./...`: exit 0.
- UI `npm run typecheck` and `vitest run`: pass (150 tests).
- `bash tools/check_core_generic.sh`: OK; a planted `_RP` / `resource_name_template` in a core
  file is reported as rule4.
- Upgrade test, real binaries: built `hourglassd` from `git archive HEAD` (previous commit) and
  from the working tree. The old binary created and seeded a database (schema 3) with a setup
  `lab-02` overridden to `pool-two`, a disabled setup, labels, a rate rule; the template setting was
  added. The new binary started on it: healthz ok, schema_migrations 1..4, `setups` has
  `id,name,details_json,enabled` with all 3 rows and details intact, labels (2) and rules (1)
  intact, template row gone, integrity_check ok, polls ok; `check-provider` then reports
  `lab-01` matched and `lab-02` not reported (its old override no longer applies, as D40 intends).
  Also covered in Go by `TestD40_Migration0004DropsResourceNameOverride`.
- `./ci.sh`: exit 0 (spdx, core-genericity, licenses, go build/vet, gazelle diff clean, `bazel build //...`, `bazel test //...` 35/35 pass). Log: `ci.log` in this folder.
- Full-size scale test (`HOURGLASS_SCALE_TEST=1 ... -run NFR_ApiUnder500msAtSpecScale`): PASS; slowest GET `/api/v1/admin/users` worst 145.7 ms, collector poll over 100 holds 258 ms. Log: `scale.log`.
- Verification grep: remaining hits are expected and are not the removed feature:
  the allocation's reported name (`orphans.resource_name` column in 0001_init.sql and
  `store/misc.go`, `Orphan.resource_name` in openapi/UI/handlers/tests, the `static` provider's
  `resource_name` file format in providers/static, README.md and docs/extending.md); the old
  migration 0001 (cannot be edited); the new migration and its BUILD entry; the seed rejection
  in `internal/app/seed.go` and its test (required by the brief); `tools/check_core_generic.sh`;
  `docs/progress.md` (orchestrator-owned, three lines describe the template; left for the
  orchestrator to update).

## Spec text now superseded beyond what D40 names
- SPEC "Extension interface" code: `ResourceName string // matched to a setup by the name template`.
- Data model row `setups`: the `resource_name_override` column.
- API table `GET /api/v1/setups`: "admins additionally get ... resource names".
- Settings page row in the Dashboard pages table: "Edit template".
- "v1 reference scenario": `Provider | vsphere, resource name template {{.SetupName}}_RP`
  (now `pool_name_suffix: _RP` in the provider config).
- First-run checklist: "Real resource pool layout (folders, nesting, naming) matches the template."
- Configuration table, `vsphere` provider row: does not list the new `pool_name_prefix` /
  `pool_name_suffix` keys.
- SPEC line 48 ("a resource pool named from the setup") still reads correctly.

## Confidence and gaps
- e2e (`e2e/run.sh`) and `deploy/smoke.sh` were not run, per the brief; the acceptance spec
  change is untested here. The e2e stack must use `deploy/config.sample.yaml` for A1+ to see the
  `lab-03` pool as `lab-03`: both compose files mount `./config.sample.yaml` (verified,
  `deploy/compose.bundled.yaml:32`, `deploy/compose.split.yaml:32`), but the stack was not run.
- I did not touch the running demo stack.
