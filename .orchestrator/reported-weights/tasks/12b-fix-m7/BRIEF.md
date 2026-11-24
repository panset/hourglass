# 12b Fix the findings of verification 12
Job intent: .orchestrator/reported-weights/INTENT.md (read it first). Contract: .orchestrator/reported-weights/CONTRACT.md (binding).
## Goal
Close the findings in `.orchestrator/reported-weights/tasks/12-verify-m7/RESULT.md` (read its findings list in full; it has the scenarios and lines).
1. Low 1: no test checks the value of `reported_weights_enabled` on `GET /api/v1/setups` (`internal/api/handlers_user.go:885`). Add assertions named after UI-12 or WGT-14 (the field exists for the acquire dialog's warning): true with the setting on, false with it off, for a developer and an admin. Break the handler once (hard-code it), see red, restore.
2. Low 2 and 3: `docs/integrating.md` near lines 429, 489 and 491 is wrong for a setup with an override. Correct the text from the code and the design (B4: `not_applied` is `override`; `reported` still shows the stored weight; attributes of a usable entry are still returned). If you can, confirm with a quick throwaway Go test or by reading the WGT-13 tests; do not start a server on ports 8080, 8081, 8091, 8990, 8443.
3. Low 4: `docs/deploying.md:216` says the Setups page shows `not_applied`; it is the admin Setup inventory that shows why a reported weight is not applied. Correct it.
4. Low 5: `docs/progress.md:102` omits that `TestUPG2_...` (a d96a950 compatibility test) was extended for the added fields (D85(l)); add it, and make its sentence about the decisions' last row true (the orchestrator is adding that row as D90).
5. Draft for the orchestrator, in this task folder as `SPEC_EDITS_ROW.md`: the body of one decisions row, in exactly the style and granularity of D79 in docs/decisions.md (read D79), titled "**Edits the source of `docs/SPEC.md` needs for D80.**", numbered items, each saying what the spec text should become. Cover everything the design's deliverable 5 lists: the "What is billed" principle, RULE-1, MET-3, the `usage_samples` and `setups` rows, the two new tables, the settings table, the HTTP API table (the new route, the state route's fields, the changed responses), the Roles and permissions table (the new scope), the Dashboard pages rows the UI changed, and requirements WGT-1 to WGT-23 and UI-12 to UI-14 as written in the design, adjusted where a recorded decision (D81 to D88) fixed a detail. One line of text, no line breaks, no pipe characters inside it (it goes into a Markdown table cell). Do not edit docs/decisions.md or docs/SPEC.md; read SPEC.md to name its sections correctly.
The info items need no change unless trivial; say what you did with each.
## Scope
May write: `internal/api/*_test.go`, `docs/integrating.md`, `docs/deploying.md`, `docs/progress.md`, this task folder. No product code. Leave the uncommitted `e2e/` changes alone.
## Done when
`./ci.sh` exits 0; RESULT.md lists each finding and what closed it.
## Long-running steps
`./ci.sh`: 30 minute timeout, log into this task folder.
## Report
Per CONTRACT.md "Reporting".
