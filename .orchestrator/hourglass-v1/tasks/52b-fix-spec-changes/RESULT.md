# 52b result: done

Finding 2 (rename while held): `internal/api/handlers_admin.go` updateSetup refuses a name change with 409 `has_active_hold`, message "a setup with an active hold cannot be renamed; release it first"; labels/details edits still work. OpenAPI updateSetup description updated, `ui/src/api/schema.gen.ts` regenerated. Tests: `TestSET5_D40_HeldSetupCannotBeRenamed` (internal/api/admin_setups_test.go), UI test "shows the API message when renaming a held setup is refused" (the form already shows the API message via errorText).
Finding 1 (upgrade note): `docs/deploying.md` new "Upgrading from a build with the resource name template" under Upgrade. `internal/store/upgradenote.go` detects, just before migrations (current version 1-3), a non-default stored template or any override; `Store.UpgradeNotes()`; `internal/app/app.go` logs each as one Warn line after Open. Reports the override count; template capped at 100 chars. Test `TestD40_Migration0004NotesDroppedNaming` (internal/store/migrate0004_test.go): default, none, template only, overrides only, both, long template, fresh DB, second open.
Finding 5 (acceptance coverage): `e2e/browser/tests/acceptance.spec.ts` A0 asserts URL /setups and Setups nav aria-current; A2 asserts alice lands on /usage. `npx tsc --noEmit` exit 0; spec not run.
Findings 3, 4: left to orchestrator (progress.md) and untouched (ui-notes.md), per brief.

## Files changed
internal/api/handlers_admin.go, internal/api/admin_setups_test.go, api/openapi.yaml, ui/src/api/schema.gen.ts, ui/src/pages/admin-setups/admin-setups.test.tsx, internal/store/upgradenote.go (new), internal/store/store.go, internal/store/migrate0004_test.go, internal/store/BUILD.bazel, internal/app/app.go, docs/deploying.md, e2e/browser/tests/acceptance.spec.ts, tools/check_core_generic.sh, this folder (ci.log).

## Decisions for docs/decisions.md
- Added `internal/store/upgradenote.go` to NAMING_EXEMPT in `tools/check_core_generic.sh` (outside the brief's scope list, one word): the detection must name the removed setting, which rule4 otherwise forbids.
- The upgrade note is advisory: query errors are ignored so it can never block an upgrade.

## Commands
`go test -race -count=1 ./...` exit 0 (30 packages ok). `./ci.sh` exit 0 (35/35 bazel tests, UI tests, typecheck, drift test) after adding upgradenote.go to BUILD.bazel and regenerating the schema.

## Confidence and gaps
No app-level test of the Warn line itself (the store test covers the note; app.go loop is trivial). Acceptance spec not run (no Docker).
