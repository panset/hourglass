# Design plan: a Setups page that leads with cost

Revision 2, 2026-11-24. Base `main` at `f1a05c2`. Source of requirements: `docs/design/setups-declutter.md` (the request). Nothing outside this task folder was changed.

Words used here: "is" marks what I read in the code or measured; "appears to" marks inference. Paths are relative to the repository root.

## Revision 2 changelog

Revision 2 answers `.orchestrator/setups-declutter/tasks/03-design-review/REVIEW.md`. Every finding is accepted except where the table says otherwise.

| Finding | Resolution | Where |
| --- | --- | --- |
| M1 new UI throws against an older API | The page reads `data.pricing?.…`. Without `pricing` the panel leaves out the pressure line, the multiplier line and the `(3×)`; both costs still show. Named test added. | 2, 4, 6 |
| M2 other columns are squeezed and wrap | Confirmed by measurement: the first design as written puts the cost button and the holder on two lines each. Fixed: every non-label column is `nowrap`; the cost cell's second line has a stated width. Measured with the product's CSS files. | 3 |
| M3 wrong character count, narrower UI-20 | Corrected. Measured: 6 characters and an ellipsis (3 for wide capitals) at 1280 px with five keys; UI-20 holds for setup names up to 21 characters, for a developer and for an admin with a disabled setup. The first design with `nowrap` alone gave 3 to 4 characters and a limit of 10, so two compactions were added (8 px cell padding; the "requested" time moves from the row to the panel). The duplicate Disabled badge is dropped. The case UI-20 is claimed for is stated. | 3, D94 |
| S1 `now` above `at_peak` after a band edit | The cell shows one value when `now >= at_peak`. The panel then leaves out the highest-band line and says why. | 3, 4 |
| S2 filter loss for values with no selector term | Confirmed: a regression for display keys only. Recorded; both owner options are designed with their cost. | 5 |
| S3 focus when the opener is gone | `Modal` checks that the opener is still in the page and otherwise focuses a fallback the page names (the h1). | 4 |
| S4 last copy can mix two responses | The panel keeps the setup, `pricing`, `reported_weights_enabled` and `metering_description` of one response together. | 4 |
| S5 U20 can pass vacuously | U20 asserts ten headers, a minimum label width, one-line cost and holder cells, and runs as developer and as admin with a disabled setup. | 3, 8 |
| S6 missed files | Added: `ui/src/pages/admin-shared/fixtures.ts`, `docs/integrating.md:471`, `README.md:9`, `docs/ui-notes.md` items 4, 6, 14 and line 210, `docs/progress.md`. End-to-end selectors use `exact: true`. | 6 |
| S7 20 ch name clip | Accepted. The clip stays at 32 ch. | 3 |
| S8 scroll position of "Details" | Both openers open the panel at the top with focus on the Price heading. The `initialFocus` prop of revision 1 is dropped; the existing `data-autofocus` rule does it. Q6 is gone. | 4 |
| S9 Go test wording before the first poll | Reworded. | 6 |
| Note 1 no sample time in `pricing` | Not added. The top bar has the same limit today; a time can be added later as another field. | |
| Note 2 policy holds, "Charging since" | Noted in D94(d). No change. | |
| Note 3 tag read twice; the middle dot | The badge has no `title`; the `sr-only` text carries the whole phrase in words. | 3 |
| Note 4 panel content | Empty value reads "(empty)"; long values wrap anywhere; the JSON block is not a second scroller inside the panel. | 4 |
| Note 5 tag word in the panel | The panel line starts with the tag word: `override (pinned by an admin)`. | 4 |
| Note 6 customer's keys in the prototype | The new prototype, the fixtures and U20 use neutral keys. | 3, 8 |
| Note 7 one display key | Accepted as is. Measured: one key gets 385 px. | 3 |
| Note 8 the v1.1.0 comparer | `addedSetupsList` gains the one entry `"pricing"`. | 6 |
| Note 9 `(3×)`; Modal changes | `(3×)` is kept. `initialFocus` is dropped; the Shift+Tab fix stays, because a heading still takes focus. | 4 |

Owner questions now open, each built as a marked switch: **Q-A** (label cells one line or two, section 3) and **Q-B** (values that have no selector term, section 5).

## Summary of recommendations

1. The ask is sound and buildable.
2. **Equation:** no equals sign, and one additive `pricing` object on `GET /api/v1/setups`, read as optional by the page.
3. **Table:** label columns are the ones that give way, every other column is one line. At 1280 px with five keys a label shows 6 characters and an ellipsis. That is the honest limit of ten columns in 1152 px; two-line label cells (Q-A) would show 14 to 15 at no extra row height.
4. **Panel:** the existing `Modal`, opened at the top from either control, following the poll.
5. **Filters:** removing the dropdowns loses the only filter for display-key values that contain a space or one of `, ( ) = !`. The default records that (B1); B2 restores it with a browser-side match.

## 1. Evaluation

The request's faults A to F are real: the 13 columns (`ui/src/pages/setups/index.tsx:147-244`), the two-line weight cell (`index.tsx:160-173`), the JSON-only modal (`index.tsx:328-337`), three label filters (`index.tsx:277-295`, `296-309`, `312`). With five keys the table goes from 13 to 10 columns.

What is wrong, contradictory or missing:

| # | Finding | Evidence |
| --- | --- | --- |
| E1 | `metering_description` is always null, so "when charging starts" has no source for a free setup. The acquire dialog's "Charging starts:" line never shows on a real server either. | `internal/api/handlers_user.go:887-888`; `api/openapi.yaml:2080-2083`; `index.tsx:361` |
| E2 | A label value "clipped as the setup name already is (`Clip`)" does not meet UI-20. `Clip` cuts at a fixed width; five clips of 10 ch make a 1452 px table in 1152 px. | `ui/src/components/Table.tsx:73-79`; `ui/src/styles/tokens.css:35-36,110-113`; revision 1 measurement, `proto/measure.cjs` |
| E3 | UI-20 cannot be tested where the other UI tests run: jsdom does no layout. | `ui/package.json:10,27` |
| E4 | The source tag has no accessible text today, only a `title`. | `ui/src/pages/usage/weight.tsx:16` |
| E5 | The page fetches setups and pressure with separate timers, the top bar fetches pressure a third time, and a failed request keeps old data on screen with no time limit. | `index.tsx:62,64`; `ui/src/shell/TopBar.tsx:20`; `ui/src/hooks/useApi.ts:44-48` |
| E6 | A weight × multiplier equation can be false as displayed, from rounding alone. | section 2 |
| E7 | For a hold priced by a Go metering policy, the Setups list shows the resolver's weight and cost, not the charge. By design (WGT-14). | `internal/collector/collector.go:616-623`; `handlers_user.go:850-858`; `docs/design/reported-weights.md:386` |
| E8 | "The chips and the selector box already filter by every label" is not true for a value that has no selector term. The dropdown is the only control that filters by such a value today. | section 5 |
| E9 | `now` can be above `at_peak` between a band edit and the next poll. | `internal/api/user_views.go:88,101-105`; `collector.go:509,542-543` |
| E10 | Six unit tests are affected, not the two blocks the request names. | `ui/src/pages/setups/setups.test.tsx:124,128-131,134-151,165-176,295-306,521-544` |
| E11 | Other text that mentions the label filters or the weight on the row. | `api/openapi.yaml:421-422`; `docs/deploying.md:185`; `docs/integrating.md:471`; `README.md:9`; `e2e/README.md` (W2 bullet); `docs/design/reported-weights.md:431` |

Owner decisions the code makes hard:

- **The source tag on the row, with UI-20.** Beside the cost it adds 77 px and each label loses two characters (measured, case F in section 3). It goes on a second line of the cost cell. So every row of an installation with reported weights is two lines tall (58 px; a row with a button is 65 px already).
- **Held by and Held for in one column, with "overdrawn" in words.** The word goes on a second line of the cell.
- **No label dropdowns** loses a filter for some values (section 5).

None is reopened.

## 2. The equation question

### How the two costs are computed

For every row of `GET /api/v1/setups`, whatever its state:

- weight = the resolver's answer for the setup (`handlers_user.go:850-856`);
- `now` = `HourlyCostMT(weight, l.mult)`, `at_peak` = `HourlyCostMT(weight, l.peakMult)` (`internal/api/user_views.go:124-129`, called at `handlers_user.go:858`);
- `l.mult` is the multiplier stored with the newest pressure sample, or the bands' value for pressure 0 before the first poll; `l.peakMult` is the highest multiplier of the bands as they are set now (`user_views.go:88,96-105`; `internal/metering/metering.go:77-85`);
- `HourlyCostMT` is weight × multiplier × 1000 rounded half away from zero to a whole milli-token, and 0 when either factor is 0 or less (`metering.go:116-131,148-150`).

| Case | What the response holds | Does weight × multiplier give the cost? |
| --- | --- | --- |
| Free setup | Resolver weight, both costs | Yes, up to rounding |
| Held, metering | The same computation; the hold is not consulted (`handlers_user.go:859-868`) | Yes, up to rounding |
| Held, not metering | The same; nothing is charged (`collector.go:616`) | Yes as arithmetic; the reason text says it is not charged yet |
| Hold priced by a `policy` weight | The resolver's weight and cost (`user_views.go:208-215` is the hold view, which differs on purpose) | Yes as arithmetic, but it is not the charge (E7) |
| Weight 0 | Costs 0 and 0 | Yes |
| Multiplier 0 | `now` 0, `at_peak` above 0 | Yes |
| `reported` or `override` weight | The resolver already applied it (`internal/weights/weights.go:121-153`) | Yes, up to rounding |
| Disabled setup (admins) | Computed like any other | Yes |
| After a band edit, before the next poll | `now` from the old bands, `at_peak` from the new ones; `now` can be the larger (E9) | Each cost matches its own multiplier |
| Pressure from `GET /api/v1/pressure` | The same `loadLab` code in another request (`handlers_user.go:639,658`) | Not guaranteed (E5) |

Rounding breaks an equals sign within one response. Checked with the page's formatters (`ui/src/lib/format.ts:14-25,93-95`): weight 1.2345 at 1.5× costs 1.852 and would print `1.235 × 1.5× = 1.852`; weight 0.0004 at 3× would print `0 × 3× = 0.001`; multiplier 1.255 prints as `1.26×`. Weights and multipliers have no limit on decimals (`internal/rules/rules.go:55-64`; `metering.go:44-61`).

### Options

| Option | Can it contradict itself? | Cost |
| --- | --- | --- |
| A. `weight × multiplier = cost` | Yes: rounding and a stale multiplier | None |
| B. Values listed, multiplier from `/pressure` | Yes, whenever the two fetches differ. Also needs the page to work out the highest band's multiplier, which UI-1 forbids | None |
| C. Values listed, no equals sign, multiplier and pressure from the setups response | No | One additive object |

### Recommendation: C

`SetupList` gains one object, required in the schema because the server always sends it:

```yaml
pricing:
  type: object
  required: [multiplier, peak_multiplier, pressure]
  description: What every `cost_per_hour_tokens` in this response was computed with.
  properties:
    multiplier:      {type: number, description: "The multiplier of `cost_per_hour_tokens.now`."}
    peak_multiplier: {type: number, description: "The highest band's multiplier, of `cost_per_hour_tokens.at_peak`."}
    pressure:        {type: number, nullable: true, minimum: 0, maximum: 1,
                      description: "Smoothed pressure of the sample `multiplier` comes from; null before the first successful poll."}
```

Server: `listSetups` writes `l.mult`, `l.peakMult` and `l.sample.Smoothed` (nil when `l.sample` is nil), at `handlers_user.go:882-889`. The same `lab` value the costs use; no extra read.

**The page treats `pricing` as optional (M1).** The API and UI images of a split installation are upgraded separately (`docs/deploying.md:11`), and a render error replaces the page (`ui/src/shell/Layout.tsx:19`). The page reads `data.pricing?.…`, as it already does for other newer fields (`index.tsx:75,358`).

The panel's sentence:

> Cost per hour is the setup's weight times the lab's multiplier, rounded to a thousandth of a token. The multiplier follows lab pressure.

Precedent: My usage writes `weight 2 × 2×` with no equals sign (`ui/src/pages/usage/index.tsx:176`).

The top bar pill and the acquire dialog keep reading `/pressure` (`TopBar.tsx:20`, `index.tsx:353-354`), unchanged by owner decision. They can differ from the panel for up to one poll. Before the first poll the top bar reads "Lab 0%" (`handlers_user.go:657-662`) and the panel reads "not measured yet".

## 3. Table design

### Columns

In order: **Setup**, one per `display_label_keys` entry, **State**, **Held by**, **Cost (tokens / h)**, actions (blank header, announced as "Actions", `Table.tsx:43`).

| Column | Content | Source |
| --- | --- | --- |
| Setup | Name in bold, `Clip` at 32 ch as today. The extra Disabled badge beside the name is dropped (`index.tsx:155`); the State column says it (`index.tsx:180-181`) | `name` |
| label key | The value on one line, cut with an ellipsis, `title` = full value. Header cut the same way, `title` = the key | `labels[key]` |
| State | Unchanged badges | `state`, `deployed` |
| Held by | See below | `holder`, `held_since` |
| Cost (tokens / h) | See below | `cost_per_hour_tokens`, `weight_source`, `metering` |
| actions | "Details" link, then Release, the Releasing badge, or Acquire. The "requested <time>" line under the badge (`index.tsx:217-219`) moves to the panel | unchanged fields |

### Cost cell

Line 1 is a button styled as a link (`styles.linkButton`). Line 2 exists only when there is a tag or a reason.

| Case | Line 1 (button text) | Line 2 |
| --- | --- | --- |
| `now` < `at_peak` | `2.0 now · up to 6.0` | |
| `now` >= `at_peak` (one band, weight 0, or a band edit before the next poll) | `2.0` (the `now` value) | |
| Lab at 0× | `0.0 now · up to 6.0` | |
| `weight_source` `reported` | as above | Badge `reported` (blue) |
| `weight_source` `override` | as above | Badge `override` (grey) |
| `rule`, `default` | as above | none |
| Held, `metering.active` false, reason given | as above | tag if any, then the API's reason text (`index.tsx:136-142`) |
| Free | no "if acquired" | |

- "One value when `now >= at_peak`" compares two API fields; nothing is computed. It never prints "4.0 now · up to 3.0".
- Numbers: a new `formatRate` in `ui/src/lib/format.ts` (1 to 3 decimals, no ` / h`); `formatPerHour` calls it.
- Button content: the visible text with `aria-hidden`, then an `sr-only` phrase in words: "2.0 tokens per hour now, up to 6.0. Price details for setup-01" (or "2.0 tokens per hour. Price details for setup-01"). No `aria-label`. Unit tests find it with `getByRole("button", { name: /Price details for setup-01$/ })`.
- Tag: a new `PriceSourceTag` in `ui/src/pages/usage/weight.tsx`: the same `Badge` and word as `WeightTag`, no `title`, followed by a sibling `sr-only` sentence, "The price comes from a reported weight." or "The price comes from a weight pinned by an admin." A sibling, so the badge's text stays exactly `reported` (`e2e/browser/tests/integration.spec.ts:599`). The tag is outside the button.

### Held by cell

| Case | Line 1 | Line 2 |
| --- | --- | --- |
| Free or disabled | empty | |
| Your hold | `You · 3 h` | |
| Another holder | `Sam Lee · 5 d 2 h` (name in `Clip`, 12 ch) | |
| `holder.overdrawn` | as above | `overdrawn`, bold amber, 12 px (`styles.overdrawn`) |

Duration: `elapsedSince(held_since)` as today (`index.tsx:203`). A pending release shows in the actions column on your own hold only, as today (`index.tsx:213-216`), and for any holder in the panel.

### Layout mechanism

1. **Label columns give way.** `Column` gets `shrink?: boolean` (`Table.tsx:5-13`). A shrink `th` and `td` get a class with `max-width: 0; min-width: 6ch; overflow: hidden; text-overflow: ellipsis; white-space: nowrap`. Each shrink `th` gets `width: (100 / number of shrink columns)%` and `title` = the header. The selector must be `.table .shrink`: `.table td` (`Table.module.css:36-40`) outranks a single class.
2. **Every other column is one line.** Setup, State, Held by, Cost and actions are `nowrap: true` columns (the option exists, `Table.tsx:11-12`). This is what stops the squeeze: percentage widths that total 100% push the other columns to their narrowest content, and a `nowrap` cell's narrowest content is its full line. The two second lines are the exceptions: the cost note is `display: block; width: 12em; white-space: normal`, and the overdrawn word is one word.
3. **8 px side padding.** `Table` gets `dense?: boolean` (side padding 8 px instead of 12 px); Setups uses it.
4. The 6 ch floor means that when nothing is left the table scrolls (`Table.module.css:2-4`) instead of showing empty columns.
5. A label cell's content is always `<span class="shrinkText" title={value}>`. This is the hook for Q-A.

**Switch Q-A.** Default: one line. If the owner allows two lines, the change is one CSS rule (`td.shrink { white-space: normal }` and a two-line clamp with `overflow-wrap: anywhere` on `.shrinkText`), the number in the U20 assertion, and D94(h). No markup or test structure changes.

### Measured

Headless Chromium (Playwright 1.49), system font (the UI's font stack names Inter but ships no font file, so it falls back). The page is built from the product's CSS files as they are (`tokens.css`, `Table.module.css`, `Badge.module.css`, `Button.module.css`, `setups.module.css`) plus the rules above, with the markup `Table.tsx` and the page emit. Script: `proto/measure-real-css.cjs`. Screenshots: `proto/r2-*.png`.

Rows in every case: a deployed hold by another user; an overdrawn holder with an `override` tag; a not-charging hold with a 12-character holder name, a reason and a tag; your own hold with Release; your own hold with a pending release, a 9-character duration and costs of 12.5 and 37.5; two free setups; for the admin, a disabled setup. Five neutral keys (the longest 28 characters), values of 24 characters. Names are `setup-01` to `setup-08` unless stated.

| Case at 1280 px, five keys | Table / available | Label column, text | Visible characters, one line (lower case / digits / wide capitals) | Other cells |
| --- | --- | --- | --- | --- |
| A. Revision 1 as written (other cells may wrap) | 1152 / 1152 | 86, 70 px | 6 / 7 / 3 | Cost button 2 lines, holder 2 lines, "requested" 2 lines |
| B. Revision 1 with `nowrap` on the other columns | 1152 / 1152 | 63, 47 px | 3 / 4 / 2 | one line |
| H. B with 8 px padding | fits | 71, 55 px | 4 / 5 / 2 | one line |
| I. B without the "requested" line | fits | 72, 56 px | 4 / 5 / 2 | one line |
| **C. Recommended (both), developer** | **1152 / 1152** | **80, 64 px** | **6 / 6 / 3, then the ellipsis** | **one line** |
| D. Recommended, admin with a disabled setup | 1152 / 1152 | 80, 64 px | 6 / 6 / 3 | one line |
| E. Recommended, duplicate Disabled badge kept | fits | 65, 49 px | 4 / 4 / 2 | one line |
| F. Recommended, tag beside the cost | fits | 65, 49 px | 4 / 4 / 2 | one line |
| G. Recommended with Q-A two lines | 1152 / 1152 | 80, 64 px | 14 / 14 / 7 over two lines | row heights unchanged (58, 65, 103 px) |

| Other cases, recommended layout | Result |
| --- | --- |
| No key | Fits; the five columns share the width |
| 1, 2, 3, 4 keys | Fits; a label column is 401, 200, 134, 100 px (24, 21, 13, 8 lower-case characters) |
| Six keys | Fits; 67 px, 4 characters |
| Seven keys | 1174 / 1152, scrolls; labels at the 60 px floor |
| Viewport 1200 px | Fits; 5 characters |
| Viewport 1100 px | Scrolls |
| Setup name of 10, 15, 21 characters (developer) | Fits; label text 62, 54, 46 px |
| Setup name of 22 characters or more | Scrolls |
| Admin, one Disabled badge, names up to 21 characters | Fits |
| Admin, duplicate badge kept | Fits only up to 11 characters |

**What UI-20 is claimed for (M3):** 1280 px, five display keys of any length, values up to 24 characters, setup names up to 21 characters, holder names of any length (cut at 12 ch), costs below 100 tokens an hour, a developer or an admin, any mix of the rows above. Outside it the table scrolls, as it does today.

**What the user sees:** 6 characters and an ellipsis per label (`abcdef…`, `10.200…`), 3 for wide capitals. Values of one column often share a prefix, so a column can read the same on every row. The full value is in the `title` and, without hover, in the panel. An admin who shows three keys gets 13 characters.

Limits: `max-width` on a table cell is not defined by CSS 2.1. Chromium honours it (measured). Firefox and Safari appear to; not measured. A browser that ignores it scrolls, as today. A machine with a wider default font has less room.

### How UI-20 is tested

1. **End-to-end step `U20`** in `e2e/browser/tests/integration.spec.ts` (section 8). This is the only real test of UI-20, and it runs only in `e2e/run.sh integration`, not in `./ci.sh`. It cannot pass on an empty or squeezed table: it asserts ten column headers; `scrollWidth <= clientWidth` on the table's scroller; every label cell at least 60 px wide; the cost button and the first line of a holder cell exactly one line box high; and it does so as the developer and as the admin with one setup disabled.
2. **Vitest `UI-20 label columns give way and every other column is one line`**: five label headers and cells have the shrink class and the full text as `title`; the other five columns have the `nowrap` class. Mechanism only.
3. Vitest in `components.test.tsx` for `shrink` and `dense`.

## 4. Details panel design

### Component

Reuse `Modal` (`ui/src/components/Modal.tsx:24-84`). The inline side panel (`ui/src/hooks/usePanelFocus.ts:13`) is not used: an `<aside>` beside the table would take the width UI-20 needs.

Changes to `Modal`, both additive:

1. **Shift+Tab from a focused element that is not in the focusable list goes to the last item.** Today only the first item and the dialog node are handled (`Modal.tsx:58`), so Shift+Tab from a focused heading would leave the dialog.
2. **Focus return (S3).** On close, focus the opener only if it is still in the page (`opener.isConnected`, as `usePanelFocus.ts:53` does); otherwise focus the element of a new optional `fallbackFocus` ref. Setups passes its `h1` (`tabIndex={-1}`). `Modal.tsx:69` has no such check today.

No `initialFocus` prop. `Modal` already focuses the element marked `data-autofocus` when the body holds no focusable control (`Modal.tsx:37-40`). The Price heading carries `tabIndex={-1}` and `data-autofocus`, and the panel body holds no button, link or field. The dialog is the one scroller (`ui/src/components/Modal.module.css:14-15`) and opens at its top.

Title: `Details: setup-01`, as today (`index.tsx:330`). Footer: Close.

### Opening

The cost cell button and the "Details" link (`aria-label` "Details for setup-01", unchanged) open the same panel in the same way: at the top, focus on the "Price" heading. Enter and Space work because both are buttons. Escape or Close returns focus to the control that opened it, or to the h1 when that control is gone.

### State

`details` holds the setup's id, not a copy (`index.tsx:54` holds a copy, which goes stale). On each render the page looks the setup up in `data.setups` (the fetched list, before the State filter).

- **Poll refresh under an open panel:** the panel shows the new values. Focus does not move; `Modal` sets focus only when `open` changes (`Modal.tsx:31,71`).
- **State change while open:** the Holder section appears or goes; the charging line changes.
- **The setup leaves the list** (deleted, disabled for a non-admin, or dropped by the selector): the panel stays open and shows a line with `role="status"`: "This setup is no longer listed. These are the last values loaded." It shows the last snapshot, which is one object holding the setup together with `pricing`, `reported_weights_enabled` and `metering_description` of the last response that contained the setup (S4). The snapshot is replaced as a whole or not at all.
- **The State filter hides the row while open:** the panel keeps following the poll. On close the opener is gone and focus goes to the h1.
- **A failed refresh:** `useApi` keeps the last data (`useApi.ts:44-48`).

### Sections and wording

Headings are `h3`. Lines are a definition list. Long keys and values wrap anywhere.

**Price**

| Line | Wording | Field |
| --- | --- | --- |
| Sentence | "Cost per hour is the setup's weight times the lab's multiplier, rounded to a thousandth of a token. The multiplier follows lab pressure." | fixed text |
| Weight | `1.15`, then the source word and its meaning | `weight` (`formatTokens`), `weight_source` |
| | `rule (set by a rate rule)`; `default (the default weight)`; `reported (sent by automation)`; `override (pinned by an admin)`; any other value: the word as sent | |
| Attributes | `memory-gb 512 · storage-gb 4000`, only when there are any | `weight_attributes` through `attributesLine` (`weight.tsx:24-27`) |
| Lab pressure | `82%`, or "not measured yet" when null. No line without `pricing` | `pricing?.pressure` |
| Multiplier now | `2×`. No line without `pricing` | `pricing?.multiplier` |
| Cost now | `4.0 / h` | `cost_per_hour_tokens.now` |
| At the highest band | `6.0 / h (3×)`; without `pricing`, `6.0 / h` | `cost_per_hour_tokens.at_peak`, `pricing?.peak_multiplier` |
| | When `now > at_peak`, this line is replaced by: "The bands were changed. The cost at the highest band shows after the next poll." | comparison of the two costs |
| Charging | Held and metering: "Charging since " + time. Held, not metering, reason given: "Not charging yet: " + reason. Held, not metering, no reason: "Not charging yet." Not held: "Charging starts: " + description, and no line when it is null | `metering.active`, `metering.started_at`, `metering.reason`, `metering_description` |
| Sentence | "Weights are reported by automation, so the price can change while the setup is held." Only when `reported_weights_enabled` is true | `reported_weights_enabled` |

Both costs show when they are equal. The price-change sentence becomes one exported constant in `weight.tsx`, used by the panel and the acquire dialog (`index.tsx:359`); the dialog's text does not change.

**Holder** (only when `holder` is not null)

| Line | Wording | Field |
| --- | --- | --- |
| Held by | "You" or the full name | `holder.is_you`, `holder.name` |
| Held since | time, then `(5 d 2 h)` | `held_since` (`Time`, `elapsedSince`) |
| Balance | `61 tokens`, or `−12 tokens, overdrawn` | `holder.balance_tokens`, `holder.overdrawn` |
| Release | "Release requested " + time + ". It completes when the setup is torn down." Only when set | `holder.release_requested_at` |

The server sets `overdrawn` at a balance of zero or less (`handlers_user.go:867`), so `0 tokens, overdrawn` can appear.

**Labels and details**

- Every label of `labels`, key and value, in the order sent; an empty value reads "(empty)" as on the chips (`ui/src/pages/setups/LabelChips.tsx:46`); "No labels." when there are none. A setup may have 32 labels (`internal/rules/labels.go:45-47`).
- The details JSON in the same `pre` with `aria-label="Setup details JSON"` (`index.tsx:334`). Inside the panel it has no `max-height` and wraps long lines, so the dialog is the only scroller and the keyboard can reach all of it.

The `admin` block is not shown; Setup inventory has it.

## 5. Filters

### What is removed

From `index.tsx`: the `labelFilters` state (line 53), its test in the row filter (line 72), `valuesFor` (lines 246-247) and the per-key `Select`s (lines 277-295); `.labelSel` in `setups.module.css:9`. The State `Select`, the selector form and `LabelChips` stay.

Nothing else reads `labelFilters`: the four filter values are `useState` in the page (`index.tsx:50-53`), with no URL parameter and no storage. One unit test selects the `size` dropdown (`setups.test.tsx:165-176`). No end-to-end step uses a label dropdown; the only `selectOption` calls are for the month (`e2e/browser/tests/acceptance.spec.ts:728,770,782`).

### Is the removal a regression? (S2, Q-B)

Yes, for one kind of value, on display keys only.

- **Today's dropdown** lists every distinct value of each display key from the fetched list (`index.tsx:246-247`) and filters in the browser by exact equality (`index.tsx:72`). It handles any value.
- **The selector and the chips** cannot express a value that contains whitespace or one of `, ( ) = !`: the API sends `selector: null` for it (`internal/rules/selector.go:266-281`), and the chip is struck through and does nothing (`LabelChips.tsx:47-61`, note at line 92). A label value may hold any characters (`internal/rules/labels.go:45-47`).
- So after the change a value such as `Team A (infra)` on a display key cannot be filtered by anything. For keys that are not display keys nothing changes: such values could never be filtered.

**B1 (default): accept and record.**

- The chip stays as it is: inert, reachable by keyboard, with its reason in the accessible name, the `title` and the note under the block (D74(c), `docs/decisions.md`). That already is "not interactive, reason available"; turning it into plain text would undo D74(c) and change a UI-9 test for no gain.
- Cost: no source file. One sentence in D94 and one in `docs/ui-notes.md`. The existing test `UI-9 a value with no term cannot be activated and says why` (`setups.test.tsx:417-431`) stays as the proof.

**B2: such a chip filters in the browser.**

- A chip whose `selector` is null becomes a toggle: `aria-pressed`, accessible name "Filter by key=value (exact match)", dashed border kept, no strike-through.
- The page keeps `exactFilters: Record<key, value>`: one value per key; the pressed chip again clears it; another value of the same key replaces it. The row filter adds `labels[key] === value` for each entry, which is the comparison line 72 makes today. It combines with the selector and the State filter. It is not written into the selector box, which cannot express it.
- The note under the block becomes: "Dashed values contain spaces or one of , ( ) = ! and are matched in the browser, not through the selector."
- Cost, source: `LabelChips.tsx` (chip rendering, two props), `index.tsx` (state and one filter line), `setups.module.css` (chip style).
- Cost, tests: `setups.test.tsx:350-352` (the chip's name) and `417-431` change; one new test, `UI-19 a value with no selector term filters by exact match in the browser`.
- Cost, documents: UI-9 is amended ("a value with no term cannot be activated" in `docs/design/ledger-labels-onboarding.md:204,372`), D94 supersedes D74(c), `docs/ui-notes.md` item 2. No API change, no end-to-end change (step A2f uses `size=` chips only, `acceptance.spec.ts:398-434`).
- It is confined to build task B3 and its docs, so it can be switched on after the rest is built.

## 6. Files to change

### API

| File | Change |
| --- | --- |
| `internal/api/handlers_user.go` | A `pricingJSON` struct; `"pricing"` in the `listSetups` response (near line 882) |
| `api/openapi.yaml` | `pricing` in `SetupList` (required); the description at 421-422 says only the state filter is applied client-side |
| `ui/src/api/schema.gen.ts` | Regenerate with `npm run gen:api` in `ui/` (`ui/BUILD.bazel:115` fails when stale) |
| `ui/src/pages/admin-shared/fixtures.ts:160-169` | `setupList()` is typed `SetupList`; add `pricing`, or `tsc` fails |
| `ui/src/pages/setups/setups.test.tsx:71-81` | The `list()` fixture gains `pricing` (same reason; one line, in B1 so the tree type-checks after B1) |
| `internal/api/user_test.go:380` | Add `pricing` to the pinned key list |
| `internal/api/upgrade_compat_test.go:195` | Add `pricing` to the fields removed before comparing |
| `internal/api/upgrade_v110_compat_test.go:298` | `addedSetupsList` gains the one entry `"pricing"` (the comparer reports an added object as one path, lines 219-221) |
| `internal/api`, new test | `TestUI18_SetupsListCarriesTheMultipliersOfItsCosts`, below |
| `internal/api/contract_test.go` | No edit; it validates the field against the closed schema (lines 28-34) |
| `docs/deploying.md` | Upgrade notes: `GET /api/v1/setups` gains `pricing`; nothing removed or renamed; the UI works against an API without it. Line 185: "shown as columns on the Setups page" |

`TestUI18_SetupsListCarriesTheMultipliersOfItsCosts` asserts: (1) before the first poll `pricing.pressure` is null, `GET /api/v1/pressure` has `updated_at` null, and `pricing.multiplier` is the bands' value for pressure 0; (2) after a poll `pricing.multiplier` and `pricing.pressure` equal `multiplier` and `smoothed` of `/pressure` at the same clock time; (3) for every setup, with weights 0, 1.15 and one with many decimals and with a policy-priced hold, `now` and `at_peak` equal `HourlyCostMT(weight, pricing.multiplier)` and `HourlyCostMT(weight, pricing.peak_multiplier)` in tokens; (4) after the top band is lowered below the current multiplier and before the next poll, `now > at_peak` and (3) still holds; (5) a developer and an admin get the same `pricing`.

### UI

| File | Change |
| --- | --- |
| `ui/src/components/Table.tsx`, `Table.module.css` | `shrink` on a column; `dense` on the table |
| `ui/src/components/Modal.tsx` | Shift+Tab fix; `isConnected` check; `fallbackFocus` |
| `ui/src/lib/format.ts` | `formatRate`; `formatPerHour` uses it |
| `ui/src/pages/usage/weight.tsx` | `PriceSourceTag`, `sourceWords(source)`, the exported price-change sentence |
| `ui/src/pages/setups/index.tsx` | Columns, cost cell, Held by cell, filters removed, `details` by id, the snapshot, the panel |
| `ui/src/pages/setups/SetupPanel.tsx` (new) | The panel's three sections |
| `ui/src/pages/setups/setups.module.css` | Drop `.labelSel`; add the cost note, the overdrawn line, the panel list |
| `ui/src/pages/admin-settings/index.tsx:410` | Hint: "Comma separated; shown as columns on Setups. Every label of a setup is in its details, so a short list hides nothing." |
| B2 only | `ui/src/pages/setups/LabelChips.tsx` and the lines named in section 5 |

### Unit tests, by requirement

All in `ui/src/pages/setups/setups.test.tsx` unless a file is named.

| Id | Test name | Asserts |
| --- | --- | --- |
| UI-15 | `UI-15 the table has no Weight and no Holder's tokens column; its columns are Setup, the display label keys, State, Held by, Cost and actions` | Header list with one key, five keys, no key |
| UI-16 | `UI-16 the cost cell shows now and at_peak with the unit, and one value when now is not below at_peak` | `8.0 now · up to 12.0`; equal gives `2.0`; `now` 4 with `at_peak` 3 gives `4.0` and no "up to"; 0× gives `0.0 now · up to 6.0`; header "Cost (tokens / h)"; no "if acquired"; the button's name in words |
| UI-16 | `UI-16 the cost cell carries the source tag for reported and override, and none for rule or default` | The word and the `sr-only` sentence per source |
| UI-17 | `UI-17 the Held by cell shows the holder or You, how long, and "overdrawn" in words` | With `Date` fixed (`vi.useFakeTimers({ toFake: ["Date"] })`) |
| UI-18 | `UI-18 the panel opens from the cost cell and from Details, with focus on Price, and returns focus on close` | Both openers; Escape and Close |
| UI-18 | `UI-18 the panel shows weight, source, attributes, pressure and multiplier, both costs, charging, the holder, every label and the details JSON` | One case per source word; the three charging cases; a label that is not a display key; an empty value as "(empty)"; a pending release with its time; the JSON text |
| UI-18 | `UI-18 the panel shows the price-change sentence whenever reported_weights_enabled is true` | Each source, on and off |
| UI-18 | `UI-18 the multiplier and pressure in the panel are those of the setups response` | `/pressure` mocked with other values, and once failing |
| UI-18 | `UI-18 the panel opens without pricing in the response` | A response with no `pricing`: no throw, both costs shown, no pressure or multiplier line (M1) |
| UI-18 | `UI-18 the panel does not show a highest-band cost below the cost now` | `now` 4, `at_peak` 3: the replacement sentence (S1) |
| UI-18 | `UI-18 an open panel follows a refresh; when its setup is no longer listed it keeps one response's values and focus returns to the heading` | Second response after a window `focus` event (`ui/src/hooks/usePolling.ts:28-41`); the second response has another multiplier, and the panel still shows the first one with the notice (S4); after Close the h1 has focus (S3) |
| UI-19 | `UI-19 the page has no per-label dropdown; the State filter, the selector box and the chips work` | One combobox; replaces lines 165-176. The UI-9 block is unchanged under B1 |
| UI-20 | `UI-20 label columns give way and every other column is one line` | Section 3 |
| UI-12 | `UI-12 the cost cell carries a tag for reported and override, and none for rule or default` | Replaces lines 521-533 |
| UI-12, WGT-14 | `UI-12 the panel shows the weight, its source and the attributes; the row shows none of them` | Replaces lines 535-544 |
| UI-12 | `UI-12 the acquire dialog …` (lines 546-566) | Unchanged |
| SET-6 | `SET-6 populated: label columns from display_label_keys, holder, overdrawn word, costs` (line 119) | New header list and formats |
| UI-1 | `UI-1 not-yet-charging setups …` (line 134) | The reason stays; new cost format |
| HOLD-6 | `HOLD-6 a hold with release_requested_at shows Releasing and no Release button; the request time is in the panel` (line 295) | The time moved |
| existing | `Details opens the pretty-printed JSON, read only` (line 195), admin test (line 316) | Still pass; line 320 needs one "Disabled" |
| components | `ui/src/components/components.test.tsx`: `Table marks shrink columns and dense tables`; `Modal keeps Shift+Tab inside when a heading has focus`; `Modal returns focus to the fallback when the opener is gone` | |
| format | `ui/src/lib/format.test.ts`: `formatRate` | |

### End-to-end

| File | Change |
| --- | --- |
| `e2e/browser/tests/integration.spec.ts:597-602` (W2) | The row keeps `reported` (exact) and "Free". The weight `1.15` and the attributes line are read from the panel, opened with `getByRole("button", { name: /Price details for lab-03$/ })` (WGT-14, amended) |
| `integration.spec.ts`, after I9 | Steps `U20`, `S1`, `S2` (section 8) |
| Any new selector on "Details for …" | `exact: true`: Playwright matches a role name as a substring without regard to case, and the cost button's name ends "Price details for lab-03" |
| `e2e/browser/tests/acceptance.spec.ts` | No edit expected (lines 398-434, 478-497, 636-637 read text that stays) |
| `e2e/README.md` | The W2 bullet; the new steps |

### Docs

| File | Change |
| --- | --- |
| `docs/ui-notes.md` | "Setups" (lines 59-85, note 4 at line 81): filter row, columns and example rows, cells, panel. Line 167 hint. Line 210 (the overdrawn marker on Setups is the word). "Differences" item 4 (line 220: no "N / h if acquired"), item 6 (line 222: weight is again not a column), item 14 (line 232: the Setups bullet). New item 15: owner-directed, and `docs/wireframes/project/Setups.dc.html` is behind |
| `docs/wireframes/project/Setups.dc.html` | Not edited (the request allows the note; items 13 and 14 set the precedent, `docs/ui-notes.md:229-232`) |
| `docs/design/reported-weights.md:399,431` | UI-12 as amended; the test plan sentence says "in the details panel" |
| `docs/integrating.md:471` | Attributes are shown "in a setup's details on the Setups page and in the acquire dialog" |
| `README.md:9` | "shows the weight and its source wherever a price is explained" (the Setups row shows the price and the source tag; the weight is in the panel) |
| `docs/progress.md` | A section for this job, as earlier jobs have (lines 75, 98-113) |
| `docs/design/setups-declutter.md` | Committed with the change; status line updated |
| `docs/decisions.md` | D93 to D95 below |
| `docs/SPEC.md` | Not edited |

Drafts for `docs/decisions.md` (the five columns of the existing rows):

| # | Date | Decision | Why | Who |
| --- | --- | --- | --- | --- |
| D93 | 2026-11-24 | **Owner-directed change: the Setups page leads with cost.** `docs/design/setups-declutter.md` is the contract. The owner decided: (a) weight is not a column on Setups; (b) the row shows the cost now and at the highest band, not the current cost alone; (c) the source tag for `reported` and `override` stays on the row, on the cost cell; (d) the calculation is explained on click in a details panel, not on hover; (e) no setting brings the weight column back; (f) the holder's balance is not a column and an overdrawn holder is marked in words; (g) Held by and Held for are one column; (h) the per-key label dropdowns go, the State filter, the selector box and the chips stay; (i) label cells are one line and cut, with the full value in the panel **[Q-A: or "may wrap to two lines"]**; (j) a label value that cannot be written as a selector term has no filter on the page **[Q-B: or "is filtered by its chip in the browser"]**. New requirement ids UI-15 to UI-20; UI-12 is amended; SET-6 and WGT-14 are still met. The acquire dialog, My usage and the admin pages are unchanged. | A customer found the page cluttered and could not tell weight from cost; what a developer acts on is the cost. | owner |
| D94 | 2026-11-24 | Details of the Setups change (D93) that the request left open. **Price explanation.** (a) The panel shows no equation with an equals sign. It states the rule in a sentence and lists weight, multiplier and both costs: the cost is rounded to a thousandth of a token and the page rounds weight and multiplier again for display, so a printed product can be false (weight 1.2345 at 1.5× costs 1.852). (b) `GET /api/v1/setups` gains `pricing` (`multiplier`, `peak_multiplier`, `pressure`, null before the first poll): the values the response's costs were computed with. The panel reads them from there and not from `GET /api/v1/pressure`. Additive. The page reads `pricing` as optional, so a newer UI works against an older API and leaves those lines out. `pricing` carries no sample time. (c) The top bar and the acquire dialog still read `/pressure`; they and the panel can show different multipliers for up to one poll, and before the first poll the top bar reads 0% where the panel reads "not measured yet". (d) For a hold whose weight a metering policy sets, Setups shows the setup's price from the resolver, as WGT-14 says, not the hold's charge; "Charging since" shows once metering has started, also for a policy that later pauses. (e) `metering_description` is always null, so a free setup has no "Charging starts" line. (f) Between a band edit and the next poll the cost now can be above the cost at the highest band. The row then shows the one value, and the panel says the bands were changed instead of showing a lower highest-band cost. **Table.** (g) The unit is in the header, "Cost (tokens / h)"; the cell reads `2.0 now · up to 6.0`, or one value. The source tag and the not-charging reason are a second line of the cost cell; the overdrawn word is a second line of the Held by cell. (h) Label columns are the columns that give way: they share the width the others leave, cut their text with an ellipsis and carry the full text as `title`; every other column is one line; below 6 ch a label the table scrolls. A fixed-width `Clip` cannot meet UI-20. To leave the labels room, the table's cells have 8 px side padding, the time of a release request is in the panel and not on the row, and the second Disabled badge beside an admin's setup name is gone. Measured in Chromium at 1280 px with five keys: a label shows 6 characters and an ellipsis (3 for wide capitals). **[Q-A two lines: 14 characters over two lines, same row height.]** (i) UI-20 is claimed for: 1280 px, five display keys, values up to 24 characters, setup names up to 21 characters, costs below 100 tokens an hour, a developer or an admin. Beyond that, and with seven keys, the table scrolls as before. It is checked in a real browser only by the integration scenario (step U20), in Chromium; the unit test checks the mechanism. Firefox and Safari were not measured. **Filters.** (j) With the dropdowns gone, a value of a display key that holds whitespace or one of `, ( ) = !` cannot be filtered: the dropdown matched it in the browser, and neither the selector nor its chip can. Values of other keys never could be. **[Q-B, B2: such a chip now filters by exact match in the browser; this supersedes D74(c) and amends UI-9.]** **Panel.** (k) It is the `Modal`. Both the cost cell and Details open it at the top with focus on the Price heading. `Modal` gained a fix so Shift+Tab from a focused heading stays inside, and returns focus to a fallback (the page heading) when the control that opened it is gone. (l) It follows the 60-second refresh; if its setup leaves the list it stays open with the values of the last response that held the setup, all from that one response, and says so. (m) It shows a release request for any holder; the row shows "Releasing" only on your own hold. (n) The wireframe `Setups.dc.html` was not redrawn; `docs/ui-notes.md` says it is behind. | The simplest options that meet UI-15 to UI-20 and the request's constraints; (a) and (b) are the only combination that cannot contradict itself; (h) and (i) follow from measurement. | orchestrator |
| D95 | 2026-11-24 | **Edits the source of `docs/SPEC.md` needs for D93.** `docs/SPEC.md` was not edited. (1) "Dashboard pages", Setups row, Shows: "Every setup: name, the chosen label columns, state, holder and how long, cost per hour now and at the highest band; in a details panel: weight and its source, lab pressure and multiplier, the holder's balance, every label, details JSON". (2) The same row, Actions: "Acquire, release own; filter by state and by label (selector and label chips); open the details panel". (3) SET-6: "An admin chooses which label keys appear as columns on the Setups page (`display_label_keys`, default none); labels are filtered with the label chips and the selector." (4) New requirements UI-15 to UI-20 as worded in `docs/design/setups-declutter.md`, and UI-12 as amended there. **[Q-B, B2: UI-9 becomes "… a value with no term is filtered by exact match in the browser …".]** | The spec is generated from a source outside this repository and is never edited here; the owner applies these. | orchestrator |

## 7. Work breakdown

| Task | Writes | After | Parallel with |
| --- | --- | --- | --- |
| B1 API field | `internal/api/handlers_user.go`; `internal/api/user_test.go`, `upgrade_compat_test.go`, `upgrade_v110_compat_test.go`, one new test file; `api/openapi.yaml`; `ui/src/api/schema.gen.ts`; `ui/src/pages/admin-shared/fixtures.ts`; the `pricing` line of the `list()` fixture in `ui/src/pages/setups/setups.test.tsx`; `docs/deploying.md` | none | B2, B4 |
| B2 Shared UI pieces | `ui/src/components/Table.tsx`, `Table.module.css`, `Modal.tsx`, `components.test.tsx`; `ui/src/lib/format.ts`, `format.test.ts` | none | B1, B4 |
| B3 Setups page | `ui/src/pages/setups/*` (after B1 has finished with `setups.test.tsx`); `ui/src/pages/usage/weight.tsx`; `ui/src/pages/admin-settings/index.tsx` | B1, B2 | B4 |
| B4 Docs | `docs/ui-notes.md`, `docs/decisions.md`, `docs/design/reported-weights.md`, `docs/design/setups-declutter.md`, `docs/integrating.md`, `README.md`, `docs/progress.md` | none; wording checked against the page after B3 | B1, B2, B3 |
| B5 End-to-end and screenshots | `e2e/browser/tests/integration.spec.ts`, `e2e/README.md` | B3 | none |
| V Verify | its own task folder | B1 to B5 | none |

Order: B1, B2 and B4 together; then B3; then B5; then V. Each task runs its own tests: B1 `bazel test //internal/api/...` and the UI type check; B2 and B3 the UI tests. `./ci.sh` runs after B3 and after B5. `e2e/run.sh integration` runs in B5; `e2e/run.sh both` once before merge.

Switches: Q-A is one CSS rule in B2's `Table.module.css`, one number in B5's U20 and a phrase in B4. Q-B (B2 option) is inside B3 plus the documents of section 5 in B4.

Requirement to test, in one place:

| Id | Unit | Go | End-to-end |
| --- | --- | --- | --- |
| UI-15 | `UI-15 the table has no Weight …` | | U20 (ten headers) |
| UI-16 | the two `UI-16 …` tests | (4) of `TestUI18_…` for `now > at_peak` | S1, S2 |
| UI-17 | `UI-17 the Held by cell …` | | U20 |
| UI-18 | the seven `UI-18 …` tests | `TestUI18_SetupsListCarriesTheMultipliersOfItsCosts` | W2, U20 |
| UI-19 | `UI-19 the page has no per-label dropdown …` | | A2f (unchanged) |
| UI-20 | `UI-20 label columns give way …` (mechanism) | | U20 (the real test) |
| UI-12 | the two new `UI-12 …` tests, the dialog tests unchanged | `TestUI12_SetupsListReportsWhetherReportedWeightsAreOn` (unchanged) | W2 |
| SET-6 | `SET-6 populated …` | | A3 (unchanged) |
| WGT-14 | `UI-12 the panel shows the weight …` | `TestWGT14_EveryReaderShowsTheWeightThePollCharges` (unchanged) | W2 |

What a verifier should try to break:

1. **Old API.** Serve a setups response with no `pricing` (and one with no `label_index`): open the panel; nothing throws.
2. **Consistency.** Make `/pressure` fail or answer other values: the panel shows the multiplier of its own response. Change the band under an open panel: multiplier and both costs change together. Lower the top band below the current multiplier: no row reads "now X · up to less than X", and the panel shows no lower highest-band cost.
3. **UI-1.** Search the page and the panel for arithmetic on API numbers. `100 / n` for column widths is the only division; comparisons of two costs are allowed.
4. **Generic.** Search code and test assertions for a label key, attribute name or extension name outside fixtures.
5. **Layout, real browser, 1280 px.** The claimed case passes; then leave it: a 22-character name, seven keys, a 63-character key, one key, no key, a cost above 1,000, a 40-character holder name, a long reason, an admin with several disabled setups. In each, no cell other than a label or a second line may wrap, and nothing may overlap. Then Firefox and Safari by hand.
6. **Keyboard and screen reader.** Tab to the cost button, Enter, Escape, focus back on the button. Shift+Tab right after opening stays in the dialog. With the State filter set to Free, open a free setup's panel, let it be acquired, close: focus is on the page heading, not lost. The tag's sentence and the unit are in the accessibility tree. The full label value is in the panel.
7. **Panel under change.** Acquire, release, disable, delete the setup while its panel is open; apply a selector that drops it; fail the refresh. After a delete the panel's multiplier must not change while its costs stay.
8. **Cases.** Weight 0; multiplier 0 at every band; one band; `pricing.pressure` null; `metering.reason` null; an unknown `weight_source`; no labels; an empty label value; 32 labels; `details` null; a 256 KB details JSON.
9. **Unselectable values.** A display key with the value `Team A (infra)`: under B1 the chip is inert and says why; under B2 it filters and combines with the selector and the State filter.
10. **Unchanged surfaces.** The acquire dialog's text, My usage, Setup inventory; the older-release comparisons allow only the added `pricing`.
11. **Nothing lost.** Weight, source, attributes, the holder's balance and the time of a release request are one click away for every row.
12. **Selectors.** In the end-to-end file, every "Details for …" role selector is exact.

## 8. Screenshots and the U20 step

The integration scenario has the ingredients: reported weights are switched on in W1 and sent in W2 (`integration.spec.ts:570-586`), the pressure is what the scripted reporter sends (`e2e/browser/reporter/reporter.ts:140-144`), the admin edits bands on the Settings page (`integration.spec.ts:246-256`), and `shot()` writes full-page PNGs to `e2e/out/integration/screenshots/` (`integration.spec.ts:62-66`). The seed has one label key, `size`, and setups `lab-01` to `lab-06` (`deploy/seed.sample.yaml:12-27`).

New steps at the end of `integration.spec.ts`, after I9, so no earlier step sees the extra labels:

- **U20.** On the admin pages (Settings for the keys, the Setup inventory form for the labels), set five display keys with neutral names and give each setup five labels with 24-character values; disable one setup. D63 lists the only places where this scenario acts outside the UI (`docs/decisions.md:71`); if the build sets the labels through the admin API, D94 must add that. Send weights for every setup again (W6 stopped them). Alice acquires one setup so there is a held row. At 1280 × 900, as alice and then as the admin: the assertions of section 3, then the panel opened from a cost cell and from Details, with weight, source and both costs compared with `GET /api/v1/setups`.
- **S1, lab at 0×.** The scenario's base multiplier is 1 (`integration.spec.ts:247-249`). The admin sets it to 0 and the step waits for the next successful poll; the reporter's pressure is 0.02, below the 5% step. Shots: `S1-setups-0x`, `S1-panel-0x`.
- **S2, a higher band.** The reporter sends 0.95; after the next poll the 15-minute average is above 5% (as acceptance step A5 relies on, `acceptance.spec.ts:553-556`), so the multiplier is 2×. Shots: `S2-setups-2x`, `S2-panel-2x`.

Between S1's band edit and its poll the page is in the `now > at_peak` state only if the top band was lowered; S1 lowers the base, so it is not. The steps add about three polls. To try the page by hand, `e2e/integration-stack.sh up` starts the same stack (`e2e/integration-stack.sh:1-13`).

## 9. Open questions for the owner

| # | Question | Default built until told otherwise |
| --- | --- | --- |
| Q-A | At 1280 px with five keys a label shows 6 characters and an ellipsis on one line, or 14 over two lines with no change in row height (rows with a source tag are two lines tall already). Screenshots: `proto/r2-1280-five-keys-developer.png`, `proto/r2-1280-five-keys-two-line-labels.png`. | One line |
| Q-B | A display-key value with a space or one of `, ( ) = !` loses its only filter. Accept and record (B1), or let its chip filter in the browser (B2: three source files, two tests changed and one added, UI-9 amended)? | B1 |
| Q1 | May `GET /api/v1/setups` gain the additive `pricing` object? | Yes |
| Q3 | A free setup has no "Charging starts" line because the server never sends `metering_description`. Add a describe method to the policy interface? | No |
| Q4 | Mark policy-priced holds in the panel? It needs another API field. | No (`policies/default/default.go:76-77`: the bundled policy never sets a weight) |
| Q5 | Should the acquire dialog take its multiplier from `pricing`? | No; the dialog is unchanged by owner decision |
