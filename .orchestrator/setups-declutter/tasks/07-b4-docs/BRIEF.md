# 07 (B4) Docs
Common rules: `.orchestrator/setups-declutter/tasks/COMMON.md` (read it first; it names the other files to read).
## Goal
Implement task **B4** of DESIGN.md section 7: every document change in DESIGN.md section 6 ("Docs") and the change request's "What else changes", with the owner's answers applied (Q-A one line; Q-B = B2: unselectable label values are filtered by their chip in the browser, UI-9 amended accordingly; the D93+ entries are written for B2, not B1).
## Scope
May write: `docs/ui-notes.md`, `docs/decisions.md`, `docs/design/reported-weights.md`, `docs/design/setups-declutter.md` (status line and the corrections DESIGN.md lists; the owner decisions are not edited), `docs/integrating.md`, `README.md`, `docs/progress.md`. Must not touch `docs/SPEC.md`, `docs/deploying.md` (task 05 owns it), the wireframe HTML unless DESIGN.md says to edit it, or any code.
## Done when
- Each doc item in DESIGN.md section 6 and in the change request's "What else changes" is done or explicitly noted as "behind" where the request allows it.
- `docs/decisions.md` has the owner-directed entry for this change request, the SPEC-source edit note, the additive `pricing` field, the no-equals-sign explanation, the B2 chip filter, and the measured layout limits (6 visible label characters at 1280 px with five keys; UI-20 holds for setup names up to 21 characters), numbered from D93 with the file's existing format.
- Requirement ids UI-15..UI-20 are recorded where this repo keeps its requirement list (find it; `docs/progress.md` or similar), with the test names from DESIGN.md section 7.
- Docs describe the page as designed; the page is being built in parallel, so list in RESULT.md every sentence that quotes UI wording (labels, button names, sentences in the panel) so a later task can check them against the built page.
## Long-running steps
None. If the repo has a docs/markdown lint in `./ci.sh`, run only that part.
## Report
Per COMMON.md.
