# RESULT: 07 (B4) Docs

Status: done. Date 2026-11-24.

## What was done

Every doc item of DESIGN.md section 6 ("Docs") and of the change request's "What else changes" that falls in this task's scope, with Q-A = one line clipped and Q-B = B2 applied.

| Item | Where | Done |
| --- | --- | --- |
| ui-notes "Setups": filter row, columns, sample rows, cells, panel | `docs/ui-notes.md` section "Setups" (intro paragraph, items 2 to 5) | yes; acquire dialog item renumbered 6 and marked unchanged |
| ui-notes filter row: B2 dashed chip filters in the browser (UI-9 amended, D74(c) superseded) | `docs/ui-notes.md`, "Available labels" bullet | yes |
| ui-notes line 167 hint | Settings item 3, "Display label keys" | yes (wireframe hint kept, built hint quoted) |
| ui-notes line 210 overdrawn marker | "Reusable components" | yes |
| ui-notes Differences item 4, 6, 14 | "Differences from the spec" | yes |
| ui-notes new item 15 (owner-directed; wireframe is behind) | "Differences from the spec" | yes |
| ui-notes "Spec asks for" item 4 (weight column) | same section | yes |
| Wireframe `Setups.dc.html` | not edited; ui-notes intro and item 15 say it is behind | noted as behind (request allows it) |
| reported-weights.md UI-12 amended (line 399), test-plan sentence (line 431) | `docs/design/reported-weights.md` | yes; also a WGT-14 note (values, not placement), from the request's "Amended" list |
| integrating.md:471 | "in a setup's details on the Setups page and in the acquire dialog" | yes |
| README.md:9 | "wherever a price is explained (…)" | yes; README.md:7 also gained the B2 chip clause and one sentence on the cost-led row |
| setups-declutter.md status line, corrections | status line + a "Corrections found while designing" list after it; owner decisions table untouched | yes |
| decisions.md D93 (owner), D94 (details incl. no equals sign, `pricing`, B2 chip filter, measured limits), D95 (SPEC-source edits incl. Setups row, UI-9, UI-12, UI-15..UI-20 text, `pricing`) | end of `docs/decisions.md` | yes, five-column format of existing rows |
| progress.md job section with UI-15..UI-20, UI-12, SET-6/WGT-14 and their test names | `docs/progress.md`, new section "A Setups page that leads with cost" before "Milestones"; "Last updated" 2026-11-24 | yes |
| docs/SPEC.md | not edited | per rule |

The measured limits are in D94(h)/(i): 6 visible characters and an ellipsis at 1280 px with five keys (3 for wide capitals; 13 with three keys, 4 with six, seven scroll); UI-20 holds for setup names up to 21 characters.

## Files changed

- `docs/ui-notes.md`
- `docs/decisions.md` (D93, D94, D95 appended)
- `docs/design/reported-weights.md`
- `docs/design/setups-declutter.md`
- `docs/integrating.md`
- `README.md`
- `docs/progress.md`
- this task folder: `PROGRESS.md`, `RESULT.md`

## Commands

No tests. `./ci.sh` has no docs or Markdown lint (grep found none), so nothing was run. No git state changes.

## Deviations from DESIGN.md

- D93 to D95 are written for B2 and one line (owner answers), not the B1 default; the Q-A/Q-B brackets are resolved. D94 gained (n), the Settings hint, so the wireframe note moved to (o).
- D95 adds item (7), the `pricing` object in the spec's API section, and states the UI-9 amendment as item (4).
- DESIGN says "requirement list ... `docs/progress.md`"; the repo has no separate requirement register (requirement text lives in design docs and in the SPEC-source D-entry), so the table with test names is in `docs/progress.md` and the full text is in D95.

## Out of scope, needs another task

- `docs/design/ledger-labels-onboarding.md:204,372`: the UI-9 text "a value with no term cannot be activated" should be amended for B2 (DESIGN section 5 names this file). Not in this brief's write list; not edited. D94(j) and D95(4) record the amendment.
- `docs/deploying.md:185` and its upgrade note belong to task 05; `e2e/README.md` (W2 bullet) to B5.

## UI wording quoted in the docs (check against the built page)

All in `docs/ui-notes.md` unless named; also repeated in `docs/decisions.md` D94/D95 and `docs/progress.md`.

1. Header "Cost (tokens / h)"; actions header announced as "Actions".
2. Cost cell text `2.0 now · up to 6.0`, one value form `2.0`, `0.0 now · up to 6.0`; no "if acquired".
3. Cost button accessible name "2.0 tokens per hour now, up to 6.0. Price details for setup-01".
4. Tag words `reported`, `override`; hidden sentences "The price comes from a reported weight." and "The price comes from a weight pinned by an admin."
5. Held by `You · 3 h`, `Sam Lee · 5 d 2 h`; second line "overdrawn".
6. "Details" link, accessible name "Details for setup-01"; "Releasing" badge; buttons "Release", "Acquire".
7. Dialog title "Details: setup-01"; button "Close"; headings "Price", "Holder", "Labels and details".
8. Panel sentence "Cost per hour is the setup's weight times the lab's multiplier, rounded to a thousandth of a token. The multiplier follows lab pressure."
9. Line names: Weight, Attributes, Lab pressure, Multiplier now, Cost now, At the highest band, Held by, Held since, Balance (and "Charging").
10. Source words `rule (set by a rate rule)`, `default (the default weight)`, `reported (sent by automation)`, `override (pinned by an admin)`.
11. "not measured yet"; `6.0 / h (3×)`; `4.0 / h`; `82%`; `2×`.
12. "The bands were changed. The cost at the highest band shows after the next poll."
13. "Charging since …", "Not charging yet: …", "Not charging yet.", "Charging starts: …".
14. "Weights are reported by automation, so the price can change while the setup is held." (existing constant).
15. Balance `61 tokens`, `−12 tokens, overdrawn`; "Release requested … . It completes when the setup is torn down."
16. "(empty)", "No labels.", JSON block label "Setup details JSON".
17. "This setup is no longer listed. These are the last values loaded."
18. Dashed chip accessible name "Filter by site=Team A (infra) (exact match)" (pattern "Filter by key=value (exact match)"); note "Dashed values contain spaces or one of , ( ) = ! and are matched in the browser, not through the selector."
19. Settings hint "Comma separated; shown as columns on Setups. Every label of a setup is in its details, so a short list hides nothing." (also D94(n)).
20. `README.md:7` and `docs/progress.md` paraphrase the same (no exact UI strings beyond `You · 3 h`, `2.0 now · up to 6.0`, "overdrawn").

## Confidence and gaps

- High that every listed doc item is covered. The text describes the page as designed (DESIGN.md Rev 2 with B2/one line); the page was not built when this was written (`ui/src/pages/setups/` had no `SetupPanel.tsx`), so the wording list above must be checked after B3.
- Inference: D94(b) says the openapi, generated types and contract/compatibility tests were updated; at writing time `api/openapi.yaml` and the Go tests were modified by task 05, but the generated types were not checked.
- `docs/design/setups-declutter.md` status says "designed and built ... for release v1.3.0"; true once B3 lands and before merge. The progress.md section does not claim the merge or release.
- Sample rows in ui-notes (82%, 2×, top band 3×) are illustrative values consistent with weight × multiplier; they are not from a run.
