# UX acceptance: Setups page, details panel, Settings (task 18)

Date 2026-11-24. Judged against `tasks/12-ux/UX-SPEC.md`.

## Verdict

**Accept.** 0 must-fix, 1 should-fix, 5 polish. The should-fix is a one-line change and comes from my own spec, not from a builder's mistake; the build can ship without it if the owner prefers not to touch the page again.

The page is uncluttered, the cost is the first thing read after the setup's name, and a newcomer can find and use the labels without the chips. Every must and should item of the change list is built as specified or in an acceptable variant. All eight deviations the page builder lists are acceptable (his report lists eight, not five).

## How this was checked

- The UI was built from the working tree with `vite build` into `tasks/12-ux/build/dist` (the production bundle; nothing in the tree was edited) and served from that folder by a small local server.
- Chromium (the Playwright install under `e2e/browser`), API mocked by route interception. Driver: `acceptance/drive.cjs`; facts it read from the page: `acceptance/drive.out`, `drive2.out` and the two `.log.json` files.
- Runs: 1280 × 900 as a developer and as an admin, mouse and keyboard; 390 × 844 with touch emulation (Chromium reports a coarse pointer there); Settings at 1280 and 390.
- Screenshots: `acceptance/*.png` (63 files). Names start with the width.

Not checked, and so not covered by the verdict: a screen reader (roles, names and status texts were read from the DOM, not listened to); Safari and Firefox; a real phone; a real input method; the real server; the page with an empty `label_index` (deviation 1, accepted on the builder's description); the docs of task 15. The first run of my driver lost four admin steps and two phone steps to its own mistakes (a click that landed on a navigation link; a click on a control the open list covered). The admin steps were run again and passed (`drive2.out`); the two phone steps (typing a key, typing a value) are covered at 390 px only by the tap run in `drive2.out` and by the screenshots up to the point of failure.

## Findings

### Must-fix

None.

### Should-fix

**S1. The Filter button turns solid blue on the first keystroke and stays blue while the suggestion list is open.**

- Screenshots: `acceptance/1280-03-list-typing-key.png`, `1280-06-list-typing-value.png`, `1280-09-list-many.png`.
- Expected (UX-SPEC 3.5, item B18, nice): the button is the primary variant "while the trimmed text differs from the applied selector". I wrote that; the build does exactly it.
- What the real thing shows: during every typing session a second filled blue control appears beside the box and competes with the Acquire buttons, the one filled control the page is meant to have (UX-SPEC section 2). It also points the newcomer at the wrong action: with `lar` typed and `size=large` offered in the list, the loud button sends `lar`, which the API reads as "has a label named lar" and answers with an empty table.
- Smallest change: make the button primary only when the text is unapplied **and the list is closed**. `ui/src/pages/setups/index.tsx:151`, `const unapplied = selectorDraft.trim() !== selector;` feeds `variant` at `:352`. Keep `unapplied` for the `aria-describedby` text (`:354`, `:358`) and use `unapplied && !listOpen` for `variant`; `SelectorBox.tsx:58` holds `open` and would report it through a new optional callback prop. If that is more than the builder wants to do now, the fallback is to drop the `variant` line and keep only the described-by text.

### Polish

**P1. At 390 px the placeholder is cut inside its example**: "Type or pick a label, e.g. owne".
- Screenshot: `acceptance/390-00-page-full.png`. Measured: the placeholder needs 256 px, the box has 192 px.
- Expected: UX-SPEC 3.1 only promised the full text at 1280 px, where it fits (256 of 306 px). The instruction half is readable on the phone.
- Smallest change, if wanted: in `setups.module.css`, on the selector input's `::placeholder` rule, nothing fixes it by CSS alone; the clean fix is for `SelectorBox.tsx` to pass the short form "Type or pick a label" when `window.matchMedia("(max-width: 480px)")` matches. Leave it if that feels heavy.

**P2. In a long list the caption scrolls away.** After a few Arrow Downs in "Values of subnet" nothing says which key's values are shown, except the box itself.
- Screenshot: `acceptance/1280-09-list-many.png` (caption still in view at the third option; it leaves at the ninth). The builder notes it too.
- Expected: UX-SPEC pinned only the "N more" line.
- Smallest change: `setups.module.css:28`, add `position: sticky; top: 0; background: var(--color-bg);` to `.caption`.

**P3. When the API refuses a selector, the line under the filter row still says "4 setups shown"** (the count of the last filter that worked) while the box holds the refused text. Deviation 8.
- Screenshot: `acceptance/1280-11-invalid-selector.png`, `390-11-invalid-selector.png`.
- Expected: UX-SPEC did not cover it. The help line directly above says "The table still shows the last filter that worked.", so the two agree. Acceptable as built.
- Smallest change, if wanted: leave the count out of that line while `selectorError` is set (`index.tsx`, the count span of the filters-in-force line near `:371`).

**P4. The list can be wider than the box.** With a 28-character key the list is 406 px under a 360 px box, so its right edge and the box's do not line up.
- Screenshot: `acceptance/1280-02b-list-active-and-hover.png`.
- Expected: UX-SPEC 3.2 allows it ("at least as wide as the box, at most 520 px"). It is the right trade: the key is shown whole.
- Smallest change: none recommended.

**P5. The owner's panel screenshots will cut the dialog at "Raw details" in a 900 px tall window.** The dialog is 90% of the window's height and scrolls inside.
- Screenshot: `acceptance/1280-21-panel-from-cost-0x.png` (cut at the JSON block), against `1280-18-panel-free-setup-labels.png` (scrolled to the end).
- Expected: UX-SPEC section 9 asks for "the whole dialog inside the viewport". The dialog is inside; its last section is below the fold.
- Smallest change: for task C4 only, take S1 and S2 panel shots in a 1280 × 1100 window. No product change.

One observation with no action for the builders: a newcomer who types `large` and presses Enter without picking gets an empty table, because Enter sends what was typed (required, so that hand-written selectors work). The list had offered `size=large` and "Clear filters" is on screen, so it is recoverable. S1 removes the push toward it.

## Section 9 checks

| # | Check | Result | Evidence |
| --- | --- | --- | --- |
| 1 | Five headers, no label column, no "Available labels", no chip | Pass | Headers read: Setup, State, Held by, Cost (tokens / h), Actions. `1280-01-page-2x.png` |
| 2 | No sideways scroll at 1280 px with a 32-character name | Pass | Page 1280 of 1280, table 1152 of 1152, developer and admin |
| 3 | Tagged row with no reason is one line; tag beside the cost | Pass | `1280-01-page-2x.png`; only lab-03 (reason) and lab-04 (overdrawn) have a second line |
| 4 | Cost in text colour, dotted underline, cost now bolder; Details the only blue link | Pass | Computed: `rgb(26,26,26)`, underline dotted `#8a8a8a`, 4 px offset, weight 600; blue controls in a row: "Details" only |
| 5 | `0.0 now · up to X` at 0× | Pass | `1280-20-page-0x.png`. One value for equal costs: not driven by me; the builder's unit test covers it |
| 6 | "You" bold; 20-character holder not cut; "overdrawn" in words | Pass | "Priya Natarajan-Okafor · 9 d" (22 characters) whole, "overdrawn" under it |
| 7 | Placeholder whole at 1280 px, begins "Type or pick a label" | Pass | "Type or pick a label, e.g. owner=team-b", 256 of 306 px. Cut at 390 px: P1 |
| 8 | Click, arrow button and Arrow Down open "Label keys" with value samples; Tab does not | Pass | `1280-02-list-empty-box.png`; after Tab from State: focus in the box, no list |
| 9 | Part of a value lists `key=value` under "Values"; one pick filters | Pass | `lar` → `size=large`, `size=extralarge`; pick → request `size=large`, 2 rows, box `size=large`. `1280-06-list-typing-value.png` |
| 10 | Key pick gives `key=` and "Values of {key}"; value pick applies | Pass | `si`, Arrow Down, Enter → `site=`; `s`, Arrow Down, Enter → request `site=sjc`. `1280-04`, `1280-05` |
| 11 | Hand-typed selector and Enter sends exactly that | Pass | `site in (ams, fra)` sent as typed; 4 rows |
| 12 | Escape closes and keeps text; Tab picks nothing | Pass | Box `subnet=` after Escape; Tab with an option active: box unchanged, focus on Filter, no request |
| 13 | "No label matches" line | Pass | `No label matches "zzz".`, `No value of size matches "xl".` `1280-08-list-no-match.png` |
| 14 | "N more" visible without scrolling | Pass | "100 more. Keep typing to narrow." in view at 1280 and 390. `1280-09-list-many.png` |
| 15 | Invalid selector: border, message, help line; no control moves | Pass | State, box and Filter rectangles identical before and after, at 1280 (both roles) and 390. `1280-11`, `390-11` |
| 16 | List not cut by the table, inside the window at 390 px, under the dialog | Pass | 390: list 326 px, right edge 358 of 390. `z-index` 30 against the dialog's 40; the list closes when focus leaves the box, so the two were never on screen together |
| 17 | Exact pick: term leaves the box, token after "Exact match:", rows filtered, no request | Pass | Box empty, 1 row, no new request, no request ever held the value. `1280-12`, `1280-13` |
| 18 | Line shows count and Clear filters only while a filter is in force | Pass | Absent at rest and after Clear filters; present for State alone, selector alone, token alone |
| 19 | Clear filters resets all and focus is in the box; × likewise | Pass | Box empty, State "All states", request with no selector, focus in the box, both cases |
| 20 | Nothing left: message, and Clear filters still on screen | Pass | `1280-14-filters-leave-nothing.png` |
| 21 | Panel from cost and from Details, focus on Price, costs first with "tokens / h", no "band" | Pass | `1280-16`, `1280-21`; text read from the dialog |
| 22 | Sections Price, Holder (held only), Labels, Raw details | Pass | h3 list read; lab-01 (free) has no Holder |
| 23 | Every label, sorted, monospace; long text wraps; "(empty)" | Pass | Six keys in order; `1280-18-panel-free-setup-labels.png`. A 54-character value was in the builder's run, not mine |
| 24 | Price and Holder values at one x | Pass | 557 px for all four lists at 1280; 172 px at 390 |
| 25 | 390 px: dialog fits, labels one column | Pass | Dialog 16 to 374 of 390; `390-16`, `390-18` |
| 26 | Tab order; focus ring on every stop | Pass | State, box, Filter, token ×, Clear filters, cost, Details, Acquire; each with a 2 px solid outline. `1280-15-cost-focus-ring.png` (taken with Tab) |
| 27 | Screen reader | **Not run.** From the DOM: combobox named "Label selector"; option names as specified; status texts as in 3.10 | Needs a listen with VoiceOver before release if the owner wants it |
| 28 | Settings: Default weight alone, no gap, no "Display label keys" text | Pass (layout and text). Saving was not driven; task 14's tests cover the round trip | `1280-30-settings-rate-rules-card.png`, `390-30-…`; no match for "display label" or "columns on Setups" in the page text |

## Change list, item by item

"As specified" means I saw it in the browser unless a note says otherwise.

| Item | Level | Result |
| --- | --- | --- |
| A1 cost button look | must | As specified |
| A2 cost now bold, rest regular or muted | must | As specified |
| A3 tag on line 1; second line reason only | must | As specified |
| A4 filter row top-aligned, form does not wrap | must | As specified, at 1280 and 390 |
| A5 Price order | must | As specified, including the price-change sentence before the explanation |
| A6 "tokens / h", "Up to", "(3×)" | must | As specified. The "multipliers were changed" text was not driven |
| A7 headings Labels, Raw details | must | As specified |
| A8 scroller `position: relative` | must | As specified: page 390 of 390 while the table is 1027 px wide |
| A9 16 px costs | should | As specified |
| A10 Charging values | should | As specified for "Since …" and "Not yet: …"; "Yes", "Not yet", "Starts:" not driven |
| A11 10 em first column | should | As specified |
| A12 labels list | should | As specified |
| A13 hint under the labels | should | As specified; absent with no labels |
| A14 holder 24 ch, "You" bold | should | As specified |
| A15 monospace typed text | should | As specified |
| A16 "None." | nice | As specified (admin, lab-07) |
| B1 placeholder | must | As specified; also skips an empty-value term, which is right |
| B2 captions | must | As specified |
| B3 value samples on keys | must | As specified |
| B4 values at the start of a requirement | must | As specified. Not offered when an operator already follows the token: acceptable |
| B5 no repeated term | must | As specified |
| B6 token on the filters-in-force line | must | As specified: 28 × 28 px ×, monospace, list named "Exact match:" |
| B7 Clear filters | must | As specified |
| B8 active and hover looks; 36 and 44 px | must | As specified. With a coarse pointer options are 44 px or taller (a wrapped key is 62 px) |
| B9 list geometry | must | As specified |
| B10 status texts | must | Done differently, acceptable: the token texts carry the count in the same sentence (deviation 2) |
| B11 groups and option names | must | Done differently, acceptable: names are `aria-label`s with the specified wording (deviation 3) |
| B12 count | should | As specified |
| B13 no-match line | should | As specified |
| B14 arrow button | should | As specified: opens, closes, keeps focus in the box, not a Tab stop |
| B15 operator words | should | As specified |
| B16 pinned "N more" | should | As specified |
| B17 help line under the error | should | As specified |
| B18 Filter primary while unapplied | nice | As specified, and the specification was wrong: S1 |
| Section 7 Settings | | As specified |

Missing: nothing.

## The builder's deviations

| # | Deviation | Judgement |
| --- | --- | --- |
| 1 | No combobox at all with an empty or missing index | Acceptable. With no labels there is nothing to pick. Not driven by me |
| 2 | Token announcement and count in one sentence | Acceptable, and better than the spec: one status line cannot say two things in turn |
| 3 | Option names by `aria-label` | Acceptable. The visible text and the name say the same |
| 4 | Enter during composition is cancelled on the box | Acceptable on the stated reasoning. Not testable here |
| 5 | `formatWeight` for a weight three decimals would print as 0 | Acceptable. Panel only; it removes "weight 0 with a price" |
| 6 | Home and End clear the active option | Acceptable. Seen: after Home the list follows the caret and nothing is active |
| 7 | `=` and `!=` close the gap to the key | Acceptable, and what a user expects: `site=` |
| 8 | The filters-in-force line shows the last good count under a refused selector | Acceptable: P3 |

## The stated limit

The builder reports that the table needs 1184 px, 32 px more than it has at 1280 px, when costs are above 10,000 tokens per hour on rows that also have a 32-character name and a 24-character holder. I did not reproduce the scroll: with a 32-character name, a 24-character overdrawn holder, `10,800.0 now · up to 32,400.0` and a tag on one row, and `36,000.0 now · up to 108,000.0` with a tag on another, the table is exactly as wide as its box (`acceptance/1280-22-page-costs-above-10000.png`, table 1152 of 1152). His case presumably has more digits. Either way the limit is acceptable: the monthly allowance in the sample settings is 300 tokens, so an hourly cost of 10,000 is far outside what an installation shows, and when it happens the table scrolls inside its own box as every Hourglass table does.

## A designer's reading of the real thing

- **Uncluttered?** Yes. At rest the page is a title, one filter row and five columns. Rows are one line except where there is something to say (a not-charging reason, "overdrawn").
- **Cost first?** Yes. The two bold things on a row are the name and the cost now; the tag sits quietly beside it; "up to" recedes. At 0× the column of `0.0 now` reads as "free right now" with the ceiling beside it.
- **Can a newcomer find and use labels?** Yes. One click in the box shows every key with a sample of its values, which is the old "Available labels" block on demand. Typing `lar` and picking `size=large` filters in one action. The arrow in the box makes it look like the State select beside it.
- **Does the list behave while typing?** Yes. It narrows per keystroke, never takes Enter away from typed text, follows the caret, reopens after a comma for the next requirement, and says when nothing matches. The one blemish is the blue Filter button (S1).
- **Does the filters-in-force line earn its place?** Yes. It is absent at rest, and when present it answers "is a filter on, how many are left, how do I get back" in one line. It is also the only place an exact-match filter could live.
- **Is the panel clear?** Yes. The price is repeated first in the row's own words, the caveat follows, the working is below in smaller type, and labels have their own heading in the same typeface as the selector box.
- **Is Settings tidy?** Yes. Default weight stands alone above the rules table with the card's normal spacing; no trace of the removed field.
