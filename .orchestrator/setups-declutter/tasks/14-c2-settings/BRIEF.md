# 14 (C2) Admin Settings page without the display-label-keys control
Common rules: `.orchestrator/setups-declutter/tasks/COMMON-R3.md` (read it first; it names the specs).
## Goal
Implement task **C2** of DESIGN-R3.md sections 3 and 6 with the UX of UX-SPEC.md (its Admin Settings section): the display-label-keys control, its heading and help text leave the Settings page with no orphaned gap; the page keeps the `display_label_keys` array it loaded and sends it back unchanged on save (the API rejects a body without it: `internal/api/handlers_admin.go:858-880`), so a stored value survives a save of any other field and no audit entry for that key appears.
## Scope
May write: `ui/src/pages/admin-settings/index.tsx`, `form.ts`, `admin-settings.test.tsx` (and that folder's CSS module if a gap needs closing). Must not touch anything else; no API or server change.
## Done when
- Tests named in DESIGN-R3 for C2 exist and pass, including: no control labelled for display label keys; stored keys (empty, some, 32) are sent back byte-identical on save; every other setting saves as before.
- The admin-settings tests, type check and lint pass. (Tasks 13 and 15 are editing other paths in parallel; a failure in their files is not yours: note it.)
## Long-running steps
None over a minute. Do not run `./ci.sh` or e2e.
## Report
Per COMMON-R3.md.
