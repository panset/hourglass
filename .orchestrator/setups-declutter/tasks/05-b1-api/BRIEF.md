# 05 (B1) Additive `pricing` object on GET /api/v1/setups
Common rules: `.orchestrator/setups-declutter/tasks/COMMON.md` (read it first; it names the other files to read).
## Goal
Implement task **B1** of DESIGN.md section 7 exactly as DESIGN.md sections 2 and 6 ("API") specify: the additive `pricing` object computed from the same per-request snapshot as the costs, with openapi, generated UI types, contract/compat tests, fixtures and the `docs/deploying.md` upgrade note.
## Scope
May write: `internal/api/handlers_user.go`; `internal/api/user_test.go`, `internal/api/upgrade_compat_test.go`, `internal/api/upgrade_v110_compat_test.go`, one new test file in `internal/api/`; `api/openapi.yaml`; `ui/src/api/schema.gen.ts` (by the repo's generator, not by hand, if one exists); `ui/src/pages/admin-shared/fixtures.ts`; ONLY the `pricing` line(s) of the `list()` fixture in `ui/src/pages/setups/setups.test.tsx`; `docs/deploying.md`; BUILD files if gazelle requires it for the new test file. Must not touch anything else.
## Done when
- `TestUI18_SetupsListCarriesTheMultipliersOfItsCosts` (all its cases per DESIGN.md) and the unchanged `TestUI12_…`, `TestWGT14_…` pass; `bazel test //internal/api/...` passes; the contract test and the older-release compat tests pass and allow only the added `pricing`.
- The UI type check passes with the regenerated types.
- Nothing in the existing response is removed or renamed.
## Long-running steps
`bazel test //internal/api/...` about 1 min uncached. Do not run `./ci.sh` or e2e (other agents are mid-edit in the tree; a UI test failure outside your files is not yours: note it and move on).
## Report
Per COMMON.md.
