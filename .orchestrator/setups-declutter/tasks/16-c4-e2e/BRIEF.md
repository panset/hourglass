# 16 (C4) End-to-end for Revision 3 and the owner screenshots
Common rules: `.orchestrator/setups-declutter/tasks/COMMON-R3.md` (read it first; it names the specs).
## Goal
Implement task **C4** of DESIGN-R3.md sections 4 ("End-to-end") and 6: acceptance steps A0, A2f, A3 redesigned (no display-keys field, no label read from a table row, filtering through the autocomplete), integration U20 rewritten for the five-column table, U22 new (autocomplete and exact-match token in a real browser), S1 and S2 kept; then both e2e commands green and the owner screenshots retaken.
## Context
- The page as built: `tasks/13-c1-page/RESULT.md` (roles, names, strings, hooks); Settings: `tasks/14-c2-settings/RESULT.md`. How the first e2e task ran things and what it hit: `tasks/09-b5-e2e/RESULT.md` (note: the demo's host connector `lab-connector.py` must not be running; the `hourglass-demo` stack is stopped by agreement and stays stopped; never remove its volumes).
- UX-SPEC.md section "acceptance checks" says what the owner screenshots must show.
## Scope
May write: `e2e/browser/tests/acceptance.spec.ts`, `e2e/browser/tests/integration.spec.ts`, `e2e/README.md`. Must not touch product code or docs; if a step fails because of a product bug, do not fix it: report it with step, expected, actual and screenshot path.
## Done when
- `e2e/run.sh both` exits 0 and `e2e/run.sh integration` exits 0, with the changed and new steps executed, not skipped (report steps executed per mode).
- U20 cannot pass vacuously; U22 exercises: suggestions on click and on typing, pick of key then value filters the table, an unselectable value (space and parentheses) becomes a token and filters, token removal, an invalid selector shows the API's error, keyboard-only path.
- Owner screenshots at 1280 px viewport (viewport shots for the panel, not full-page, so the backdrop covers the page), reported weights on: Setups page and open panel at 0x and at a higher band, plus one of the open suggestion list and one with an exact-match token. Copy to `.orchestrator/setups-declutter/tasks/16-c4-e2e/screenshots/` and list absolute paths.
- A grep of `e2e/` finds no step that reads label columns, chips, the display-keys field, the old Weight / Holder's tokens columns or the JSON-only modal.
## Long-running steps
`e2e/run.sh both` about 26 min; `e2e/run.sh integration` about 25 min. Run each in the foreground with a 50-minute limit into a log in your task folder; wait once each. They use the same ports: run them one after the other.
## Report
Per COMMON-R3.md, plus the screenshot paths.

## Added after verification (read before starting)
- `tasks/13-c1-page/RESULT.md` has a "Fix round" section: use its roles, names and strings. An empty result now reads "No setups match the filters." in the table as well as the status line when a filter is in force.
- `e2e/README.md:13,51` is stale (found by task 17): correct it.
- Take the owner panel screenshots at a 1280 x 1100 viewport so the panel is not cut at "Raw details" (UX acceptance P5).
- This is the first run of the new page against the real server: the checks so far used a mocked API. In U22 or a new step, also confirm on the real stack that the panel's weight, source, multiplier and both costs equal `GET /api/v1/setups`, that `label_index` drives the suggestions, and that Settings saves with the display keys preserved (A0).
