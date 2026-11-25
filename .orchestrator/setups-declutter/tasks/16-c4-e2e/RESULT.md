# 16 (C4) End-to-end for Revision 3 and the owner screenshots: result

Status: **done**. 2026-11-24. No git state changed, no product code touched, demo stack and its connector left stopped, no volume removed. Docker stacks are down after every run (`run.sh` tears them down).

## What I did

- `e2e/browser/tests/acceptance.spec.ts`
  - A0 (`:314`): the two "Display label keys" lines are gone. It now asserts that the page has no such control or text, and that `display_label_keys` from `GET /api/v1/admin/settings` is the same after the Settings save as before it (UI-23).
  - A2f (`:405`), rewritten for UI-21:
    - No "Available labels" text and no "Filter by …" button.
    - The placeholder is `Type or pick a label, e.g. size=…`.
    - A click in the empty box opens Label keys with `size, values: extralarge, large, medium`. Escape closes the list and the box stays empty.
    - Type `si`, pick `size`: the box reads `size=` and keeps focus, and Values of size shows exactly extralarge, large, medium. Picking `large` sends `selector=size=large` and lists lab-03 and lab-04.
    - With the caret after `size=`, picking `medium` replaces the whole value: `size=medium`, lab-01 and lab-02.
    - Empty box and Enter: no selector, six rows, no Clear filters.
  - A3 (`:508`): the row has 5 cells. The label is read from the details panel (`size` = `large`), not from the row.
- `e2e/browser/tests/integration.spec.ts`
  - `checkPanel` (`:253`) uses the new panel wording ("Cost now … tokens / h", "Up to … at the highest multiplier (N×)"). It also checks the section order (Price, Holder when held, Labels, Raw details), that the Labels list holds exactly the setup's keys, sorted, and that the JSON block is present exactly when there are details. Panel shots are viewport shots.
  - `measureSetups` (`:314`) has no label cells. It measures the name, state, cost text and Held by line as one line each. Cost lines are counted by the distinct tops of the text's line boxes, because the nested cost spans give 4 client rects on one visible line (run 1).
  - U20 (`:1053`): no Settings step.
    - Set-up: five display keys stored through the admin API (no page edits them since D96), five 24-character labels per setup, a lab-05 weight override, a 32-character setup, lab-06 disabled, alice holding lab-02.
    - At 1280 × 900, for developer (6 rows) and admin (7 rows): exactly `Setup, State, Held by, Cost (tokens / h), Actions` while the API still returns the five keys. Every row of the response has 5 cells, and the full long name is in its title.
    - Also checked: a held row, a tagged row, lab-06 for the admin only, no label value anywhere in the table body, no sideways scroll of the table or the page (1152/1152, 1280/1280), and every cost cell against `now`/`at_peak`.
    - The panel is opened from the cost cell and from Details.
    - It cannot pass vacuously: the header list, row counts, cell counts and row kinds must all hold.
  - U22 (`:1138`), new. Fixture: `group-name` = `Unit B (west)` on lab-01 (free) and lab-02 (held), set in Setup inventory. Checks:
    - `label_index` gives the value `selector: null`.
    - A click shows exactly `label_index`'s keys in order, each option named with its first three values.
    - Typing `key-o` lists one key. Picking it gives `key-one=` and exactly that key's values from `label_index`. Picking lab-03's value sends the term and leaves 1 row. Clear filters restores all rows with focus in the box.
    - `group-name=` offers "Unit B (west), exact match". Picking it adds the token under "Exact match:", empties the box, keeps focus there and lists the 2 rows ("2 setups shown").
    - State Free leaves 1 row. The contradicting selector `size=large` gives "No setups match the filters." with the token still shown.
    - No setups request carried the value.
    - The token's button removes it: all rows back, focus in the box, no filters-in-force line.
    - An invalid selector (`key-one in ()`) shows the API's own message (`selector: expected a value in the in list at position 13`), the forms line and `aria-invalid`, and the rows stay.
    - Keyboard only:
      - Tab into the box opens nothing; Arrow Down opens with the first option active (`aria-activedescendant`) and typing clears it.
      - Arrow Down + Enter picks the key, and Arrow Down + Enter picks the exact value.
      - Tab reaches Filter and then the token's ×, and Enter on × removes the token.
      - Escape closes the list and keeps the text; Enter submits `size=medium` (2 rows).
  - S1 (`:1347`): asserts that the Settings save kept the stored display keys (UI-23 on the real stack, with a non-empty value). The panel reads `0.0 tokens / h`, Up to `… (3×)` and Charging `Since …`. The window is 1280 × 1100 for the panel shot.
  - S2 (`:1392`): the same at 1280 × 1100.
  - S3 (`:1420`), new: the owner shots S3 (list open on Label keys) and S4 (a token, `size=medium` in the box, the count and Clear filters).
  - Fixed an existing race in W5 (`:891`). W1 leaves the admin on Settings, so `nav` to Settings did not fetch the collector again and the refusal warning was missing in run 2 (it passed in run 1 by timing). The fix is a reload after `nav`.
  - The header comment is updated.
- `e2e/README.md`:
  - Line 13 (A2f) and line 51 (U20) are corrected.
  - A0 and A3 are adjusted; U22 and S3 are added, with a note on the S1 shot.
  - The "outside the UI" sentence now names the API write of the display keys.

## Commands and outcome

| Run | Outcome |
| --- | --- |
| `npx tsc --noEmit -p tsconfig.json` (e2e/browser) | exit 0 after every edit |
| `e2e/run.sh integration` run 1 | exit 1, 1494 s. 21/22 passed; U20 failed on the cost measurement artifact above (screenshot showed one line) |
| run 2 | exit 1, 1434 s. U20, U22, S1-S3 passed; W5 race (above) |
| run 3 | exit 1, 4968 s. All through U22 passed; S1/S2 poll waits timed out, S3 hit the test timeout |
| `e2e/run.sh both` run 1 | bundled exit 1, 5600 s: A0, A2f, A3 passed; A4, A5, A7, A8, A8p, A8c, A9 poll waits timed out. **split exit 0, 2000 s, 16/16 steps** |
| **`e2e/run.sh integration` run 4** | **exit 0, 1491 s, 1 Playwright test, 22/22 steps** (I0-I10, W1-W6, U20, U22, S1, S2, S3); `run4-steps.json` |
| **`e2e/run.sh bundled`** | **exit 0, 774 s, 16/16 steps** (A0-A9 with A2g, A2f, A2p, A7g, A8p, A8c); `bundled2-steps.json` |

- Runs 3 and both-1 failed for an environmental reason. The pause fell while they ran, and the stack wrote no log line for 720 s (integration) and for 1083 s and 409 s (bundled), so collector polls stopped. These were not product or spec failures.
- `both` was not repeated as a whole: split had already passed with the final acceptance spec (the coordinator's "do not repeat a run that already passed"), so only bundled was rerun.

Logs are `e2e-integration*.log`, `e2e-both*.log` and `e2e-bundled.log`.

## Grep check

`grep -rnE 'Display label keys|chip|Chip|Filter by|Available labels|shrink|Label size|Holder.s tokens|columnheader|At the highest band|labels: cells' e2e/browser/tests e2e/browser/reporter e2e/README.md` matches only absence assertions:
- `acceptance.spec.ts:323,411,412`
- `integration.spec.ts:1060`
- comments and asserted text

No step reads label columns, chips, the display-keys field, a Weight or Holder's tokens column, or a JSON-only modal.

## Screenshots (1280 px, reported weights on)

Panel and list shots are of the viewport. The panels are 1280 × 1100; the pages and list shots are 1280 wide.

- `screenshots/S1-setups-0x.png` and `screenshots/S1-panel-0x.png`
- `screenshots/S2-setups-2x.png` and `screenshots/S2-panel-2x.png`
- `screenshots/S3-list-open.png` and `screenshots/S4-token-and-selector.png`
- Also: `U20-setups-developer.png`, `U20-setups-admin.png`, `U22-keys-on-click.png`

## Deviations

1. U20 stores the display keys through the admin API, because no page can store them since D96. Without this, "whatever display_label_keys holds" could not be shown with a non-empty value.
2. The exact value is put on two setups (one free, one held), not one, so that the State filter visibly changes the count.
3. S3/S4 were added as their own step after S2, so the costs are above zero.
4. The W5 fix is outside the brief's steps but inside my file. It is a test race, not a product bug.
5. `both` was completed as split (run 1) plus a bundled rerun, not one green `both` invocation.

## Product bugs

None found.

## Confidence and gaps

- High: every changed and new step was executed and green on the real stacks, and the screenshots were reviewed by eye.
- Chromium only.
- The keyboard path was checked through Playwright key events, not a real screen reader.
- The poll-starvation failures are inferred to come from the pause, from the log gaps. Inference.
