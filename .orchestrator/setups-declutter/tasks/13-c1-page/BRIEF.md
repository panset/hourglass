# 13 (C1) Setups page: autocomplete selector, exact-match token, no label columns or chips
Common rules: `.orchestrator/setups-declutter/tasks/COMMON-R3.md` (read it first; it names the specs).
## Goal
Implement task **C1** of DESIGN-R3.md section 6 with the UX of UX-SPEC.md: the table is Setup, State, Held by, Cost, actions; the "Available labels" chips are gone; the selector box is an accessible combobox suggesting keys, operators and values from `label_index`; a value the selector cannot express is suggested and becomes a removable exact-match token applied in the browser; labels are shown in the details panel only; dead shared code from the first build (`shrink`, `dense`, `ShrinkText`, `LabelChips`, `labelTerms`, chip filter) is removed. Unit tests named per DESIGN-R3.md section 4 (UI-15, UI-19, UI-20 as amended, UI-21..UI-23), wording per UX-SPEC.
Also fix these findings of the first verification (`tasks/10-verify/RESULT.md`, minors 1, 2, 4): a not-charging reason with no spaces overflows the cost cell (`.costNote` needs to wrap); `.sr-only` texts added by this change escape the table scroller and make the page scroll sideways at 768/375 px (fix for the Setups page's own hidden texts; if the root cause is the shared `.sr-only` rule or Table scroller, fix it there and list other pages affected); a weight below 0.0005 prints `0` beside a non-zero cost (format without computing: follow UX-SPEC if it says how, else show the API value unrounded).
## Scope
May write: `ui/src/pages/setups/*` (create/delete files as DESIGN-R3 says), `ui/src/components/Table.tsx`, `Table.module.css`, `index.ts`, `components.test.tsx`, `ui/src/lib/format.ts`, `format.test.ts`, and the shared stylesheet that defines `.sr-only` only if that is the root cause of the sideways scroll. Must not touch: `ui/src/pages/admin-settings/*` (task 14), docs and `api/` (task 15), e2e (task 16), server code, the acquire dialog's behaviour or text, My usage output.
## Done when
- Every unit test DESIGN-R3 names for C1 exists and passes; the kept tests for UI-16, UI-17, UI-18, UI-12, WGT-14 and optional `pricing` still pass; a response without `label_index` gives a plain text box and no throw; free typing and invalid-selector errors behave as before.
- No reference to `shrink`, `ShrinkText`, `dense`, `LabelChips`, `applyChip` or chip classes remains in `ui/src`.
- Whole UI suite, type check and lint pass; then `./ci.sh` exits 0 (run once at the end; tasks 14 and 15 run in parallel in other paths: a failure in their files is not yours, note it).
- A mutation check: break at least four new behaviours (suggestion at caret, pick edits one token only, exact-match token filters, Escape closes list only), confirm a named test goes red, restore.
## Long-running steps
UI suite under a minute; `./ci.sh` under a minute warm, a few minutes cold. Do not run e2e.
## Report
Per COMMON-R3.md. RESULT.md also lists the accessible names, roles, visible strings and test hooks of the new controls, for the e2e task.
