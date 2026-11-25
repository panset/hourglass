# Result: 01-design

Status: done. `DESIGN.md` in this folder covers the nine points of the brief.

## What I did

1. Read the brief, `INTENT.md`, `PLAN.md` and the change request `docs/design/setups-declutter.md`.
2. Read the Setups page, its tests and the shared UI pieces it uses, and traced `cost_per_hour_tokens` through the server.
3. Measured candidate table layouts in headless Chromium with a static HTML prototype (no product code involved).
4. Wrote `DESIGN.md`.

## Main findings

| Finding | Evidence |
| --- | --- |
| Both costs of every Setups row are the resolver's weight times a multiplier read in the same request, rounded to a milli-token. The hold is not consulted, so a policy-priced hold shows the setup's price, not its charge. | `internal/api/handlers_user.go:850-858`; `internal/api/user_views.go:88,96-105,124-129`; `internal/metering/metering.go:116-131,148-150`; `internal/collector/collector.go:616-623` |
| An equation with an equals sign can be false as displayed, from rounding alone. | Ran the page's formatters in node: weight 1.2345 at 1.5× costs 1.852 and would print `1.235 × 1.5× = 1.852`; weight 0.0004 at 3× would print `0 × 3× = 0.001` |
| The multiplier on the page comes from a separate request that keeps stale data on failure. | `ui/src/pages/setups/index.tsx:62,64`; `ui/src/hooks/useApi.ts:44-48`; `ui/src/shell/TopBar.tsx:20` |
| `metering_description` is always null, so a free setup has no "charging starts" text. | `handlers_user.go:887-888`; `api/openapi.yaml:2080-2083` |
| A fixed-width `Clip` on label values cannot meet UI-20. | Measured: five 10 ch clips give a 1452 px table in 1152 px |
| Label columns that share the leftover width, with the tag and the overdrawn word on second lines, fit at 1280 px with five keys. Each label column is 80 px (about 8 to 9 characters). | Measured; `proto/measure.cjs`, `proto/layout-1280-five-keys.png` |
| UI-20 cannot be tested in Vitest (jsdom has no layout). | `ui/package.json:10,27` |
| No URL parameter, stored state or end-to-end step depends on the label dropdowns; one unit test does. | `index.tsx:50-53`; `ui/src/pages/setups/setups.test.tsx:165-176`; `e2e/browser/tests/acceptance.spec.ts:728,770,782` |
| One end-to-end step reads the weight and attributes from the Setups row and must read the panel instead. | `e2e/browser/tests/integration.spec.ts:597-602` |

## Decisions made

- **Equation:** no equals sign, plus an additive `pricing` object (`multiplier`, `peak_multiplier`, `pressure`) on the `GET /api/v1/setups` response, so the panel's figures all come from one response.
- **UI-20:** a `shrink` option on the shared `Table` column; real check as an end-to-end step, mechanism check in Vitest.
- **Panel:** the existing `Modal`, with an initial-focus target and a fix to the Shift+Tab trap; panel state is the setup's id so the content follows the poll.
- **Wireframe:** a note in `docs/ui-notes.md` that it is behind, not a redraw.
- Decisions D93 (owner-directed), D94 (details) and D95 (spec source edits) are drafted in full in `DESIGN.md` section 6.

## Files changed

All inside `.orchestrator/setups-declutter/tasks/01-design/`:

- `DESIGN.md`
- `PROGRESS.md`
- `RESULT.md`
- `proto/measure.cjs` (the measurement script)
- `proto/layout-1280-five-keys.png` (the recommended layout at 1280 px)

Nothing else in the repository was written. No git command that changes state was run.

## Commands run

| Command | Outcome |
| --- | --- |
| `grep`, `sed -n`, `cat`, `ls`, `wc` over the repository | Read only |
| `node measure.cjs` (three versions, in the session scratchpad, using Playwright from `e2e/browser/node_modules`) | Printed table and column widths for each variant; the numbers are in `DESIGN.md` section 3 |
| `node -e` with the page's `Intl.NumberFormat` settings | The rounding examples above |

No unit test, `./ci.sh` or end-to-end run was started.

## Confidence and gaps

- **Verified by reading:** the server computation for every case in section 2, the page's current columns and filters, the Modal's focus rules, the test and end-to-end steps that depend on the old columns.
- **Verified by measurement, Chromium only:** the layout numbers. The prototype copies the page's paddings, font sizes and control sizes by hand; it is not the real page. The build must measure again on the real page (end-to-end step U20).
- **Not measured:** Firefox and Safari. The layout relies on `max-width` on a table cell, which CSS 2.1 leaves undefined; the two browsers appear to honour it, from general knowledge only.
- **Not verified:** how the `v1.1.0` comparison names an added nested object (`internal/api/upgrade_v110_compat_test.go:298`); the build task has to check when it adds `pricing`.
- **Not verified:** that the end of the integration scenario can reach 0× and then 2× in the way section 8 describes. The band values and the smoothing rule were read (`e2e/browser/tests/integration.spec.ts:247-249`; `e2e/browser/tests/acceptance.spec.ts:553-556`), the steps were not run.
- **Not verified:** that `acceptance.spec.ts` needs no edit. Its Setups steps were read and none reads a removed column, but it was not run.
- **Inference:** the Settings and Setup inventory pages can set five display keys and five labels per setup in an end-to-end step. I read that the fields exist, not how long the step takes. Using the admin API instead would extend D63 (`docs/decisions.md:71`).
- The request's status line says it was written against `4c40a4c`; the code I read is `f1a05c2`. I did not diff the two.

## Revision 2 (follow-up task 04)

`DESIGN.md` was revised in place against `.orchestrator/setups-declutter/tasks/03-design-review/REVIEW.md`. Its changelog at the top lists every finding and its resolution. All three must-fix and all nine should-fix findings are accepted; none is rejected.

What changed in substance:

- `pricing` is optional for the page; named test `UI-18 the panel opens without pricing in the response`.
- The table mechanism: every non-label column is `nowrap`; 8 px cell padding; the "requested" time moves to the panel; the duplicate Disabled badge is dropped; the name clip stays at 32 ch.
- `now >= at_peak` shows one value; the panel never shows a lower highest-band cost.
- Both openers open the panel at the top; `initialFocus` is dropped; `Modal` gets a focus fallback for a missing opener.
- Q-A and Q-B are designed as marked switches; section 7 is final; D93 to D95 are redrafted.

Measured (headless Chromium, the product's CSS files, `proto/measure-real-css.cjs`, screenshots `proto/r2-*.png`), 1280 px, five keys, 24-character values:

| Case | Label text width | Visible characters, one line | Setup name limit |
| --- | --- | --- | --- |
| Revision 1 as written | 70 px | 6 to 7, but the cost button and the holder wrap to two lines | not measured |
| Revision 1 with `nowrap` | 47 px | 3 to 4 | 10 |
| Recommended | 64 px | 6 and an ellipsis (3 for wide capitals) | 21, developer and admin with a disabled setup |
| Recommended, two-line labels (Q-A) | 64 px | 14 over two lines | 21 |
| Recommended, duplicate Disabled badge kept | 49 px | 4 | 11 |

Removing the dropdowns is a regression for one kind of value: a display-key value with whitespace or one of `, ( ) = !`. The dropdown matches it in the browser (`ui/src/pages/setups/index.tsx:72`, options from `index.tsx:246-247`); the API gives it no selector term (`internal/rules/selector.go:266-281`) and its chip is inert (`ui/src/pages/setups/LabelChips.tsx:47-61`).

Commands run in this revision: `node real.cjs` twice in the session scratchpad (copied to `proto/measure-real-css.cjs`); read-only `grep` and `sed`. No test, `./ci.sh` or end-to-end run.

Gaps, revision 2:

- The prototype uses the product's CSS files and hand-written markup that mirrors `Table.tsx` and the planned cells. It is still not the built page; step U20 is the real check.
- The visible-character counts are computed from text widths in a canvas with the cell's font, and agree with the screenshots; they depend on the machine's system font.
- Firefox and Safari are still not measured.
- The end-to-end steps U20, S1 and S2 were not run.
- Review notes 1 and 2 are recorded, not acted on.

## Revision 3 (follow-up task 11)

`DESIGN-R3.md` is a delta to `DESIGN.md` for the owner's change after the first build: no label columns, no chips, autocomplete in the selector box, an exact-match token, labels in the panel only, the display label keys field gone from Settings. Designed against the uncommitted tree on `setups-declutter`. No API change.

Findings that shaped it:

- `PUT /api/v1/admin/settings` refuses a body without `display_label_keys` (`internal/api/handlers_admin.go:858-880`), so the Settings page keeps the loaded array and sends it back unchanged.
- Two acceptance steps depend on what is removed: A0 fills and reads the field (`e2e/browser/tests/acceptance.spec.ts:321,337`); A3 reads a label value from the table row (`acceptance.spec.ts:481`). A2f and U20 are rewritten; U22 is new.
- `shrink`, `dense` and `ShrinkText` have no user but Setups (searched `ui/src`), so they are removed.
- Measured (`proto/measure-r3.cjs`, the product's CSS, Chromium): the five-column table fits at 1280 px for a developer and an admin with any name length; it needs 1004 px with a 32-character name.

Removing the setting breaks nothing functionally in other areas. A stored value stays, invisible, changeable only through the API.

Commands: read-only `grep` and `sed`; `node r3.cjs` in the scratchpad (copied to `proto/measure-r3.cjs`). No test or end-to-end run.

Gaps, revision 3:

- The combobox is designed, not prototyped; no browser or screen-reader run.
- That a pick loses the browser's undo history is inference.
- A label key that is not a valid selector key appears not to occur (keys are validated when stored); not handled if it does.
- The table measurement uses hand-written markup that mirrors the built cells, not the built page; step U20 is the real check.
