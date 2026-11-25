# 08 (B3) The Setups page
Common rules: `.orchestrator/setups-declutter/tasks/COMMON.md` (read it first; it names the other files to read).
## Goal
Implement task **B3** of DESIGN.md section 7: the Setups table (columns, cost cell, Held by cell, clipped label columns), the details panel (Price / Holder / Labels and details), the filter row without per-label dropdowns, and **Q-B option B2** (a chip whose label value cannot be written as a selector term applies an exact-match filter in the browser and combines with the selector box and the State filter; UI-9 amended), exactly as DESIGN.md sections 3, 4, 5 and 6 specify, with a unit test named after each of UI-15..UI-20 and the amended UI-12 / SET-6 / WGT-14 tests.
## Context
- Finished before you: `tasks/05-b1-api/RESULT.md` (the `pricing` object and generated types; `pricing` is OPTIONAL to the page: a response without it must not throw) and `tasks/06-b2-shared-ui/RESULT.md` (the props/classes/helpers added to Table, Modal and format). Read both.
- Docs were written in parallel: `tasks/07-b4-docs/RESULT.md` lists every doc sentence that quotes UI wording. Use that wording in the page where DESIGN.md gives it; where you must differ, list the difference in RESULT.md (do not edit docs).
## Scope
May write: `ui/src/pages/setups/*`, `ui/src/pages/usage/weight.tsx`, `ui/src/pages/admin-settings/index.tsx` (only what DESIGN.md says for these two). If a shared component from task 06 needs a fix, you may edit `ui/src/components/{Table.tsx,Table.module.css,Modal.tsx,components.test.tsx}` `ui/src/components/index.ts` (to export `ShrinkText`) and `ui/src/lib/format*.ts`, and must list each such edit. Must not touch: the acquire dialog's behaviour or text, My usage output, admin pages beyond the one help text, API/server code, docs, e2e.
## Done when
- Every unit test named in DESIGN.md section 7 for UI-15..UI-20, UI-12, SET-6, WGT-14 exists under its name and passes; the optional-`pricing` test and the `now >= at_peak` single-value test pass; the B2 chip-filter tests pass.
- The whole UI unit suite, type check and lint pass; then `./ci.sh` passes (run it once, at the end).
- No arithmetic on API numbers in the page or panel (UI-1); no customer label key/attribute/extension name outside fixtures.
## Long-running steps
UI unit suite under a minute; `./ci.sh` under a minute warm, a few minutes cold. Do not run e2e.
## Report
Per COMMON.md. In RESULT.md also list: the accessible names and visible strings of the new controls (for the e2e task), and any differences from the docs' quoted wording.
