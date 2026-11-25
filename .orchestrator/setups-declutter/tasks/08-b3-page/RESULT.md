# Result: 08 (B3) The Setups page

Status: **done**. 2026-11-24.

## What was done

DESIGN.md Rev 2 task B3 with Q-A = one line and Q-B = B2.

- **Table** (`ui/src/pages/setups/index.tsx:198`): columns Setup, one `shrink` column per display key (content `ShrinkText`), State, Held by, "Cost (tokens / h)", actions; every non-label column `nowrap`; `dense` table (`:319`). No Weight, Holder's tokens or Held for column. The second Disabled badge beside an admin's setup name is gone; the "requested <time>" line under Releasing moved to the panel.
- **Cost cell** (`index.tsx:160`): a link-styled button, visible text `2.0 now · up to 6.0` (`aria-hidden`), or the `now` value alone when `now` is not below `at_peak`; an `sr-only` phrase gives the accessible name. Second line (`.costNote`, 12em, may wrap) only for the source tag and/or the not-charging reason. The tag is outside the button.
- **Held by cell** (`index.tsx:185`): `You · 3 h` or name (Clip 12 ch) ` · ` duration; second line "overdrawn".
- **Details panel** (`ui/src/pages/setups/SetupPanel.tsx`, new): Price / Holder / Labels and details as DESIGN section 4. The page keeps the setup's id (`index.tsx:98-102`); `snapshotOf` (`SetupPanel.tsx:25`) builds one object from one response (setup, `pricing`, `reported_weights_enabled`, `metering_description`); when the newest response no longer lists the setup the last snapshot is shown whole with the notice. `pricing` is read as optional. `Modal` gets `fallbackFocus` = the h1 (`index.tsx:263,329`). The body holds no focusable control, so focus lands on the Price h3 (`data-autofocus`).
- **Filters**: `labelFilters`, `valuesFor` and the per-key `Select`s removed. **B2**: `exactFilters` state, one value per key (`index.tsx:81-94`); a chip with `selector: null` is a toggle (`LabelChips.tsx:60-72`), filters rows by `labels[key] === value`, combines with the selector and the State filter, is never written into the selector box.
- `ui/src/pages/usage/weight.tsx`: `PRICE_CHANGE_SENTENCE` (`:24`, used by the panel and the acquire dialog; the dialog's text is unchanged), `PriceSourceTag` (`:37`), `sourceWords` (`:56`).
- `ui/src/pages/admin-settings/index.tsx:410`: the hint text only.
- **Shared-component edit (allowed by the brief):** `ui/src/components/index.ts` now exports `ShrinkText`. No other shared file was edited.

## Tests (`ui/src/pages/setups/setups.test.tsx`, 56 tests in the file)

Named as DESIGN section 6: UI-15 (`:629`); UI-16 x2 (`:646`, `:666`); UI-17 (`:681`); UI-18 x7 (`:739`, `:762`, `:847`, `:861`, `:877` optional `pricing`, `:894` `now > at_peak`, `:910` refresh/unlisted/focus); UI-19 (`:190`); UI-20 (`:697`); UI-12 x2 (`:567`, `:581`, the second also covers WGT-14: weight and source read from the panel); SET-6 (`:141`); UI-1 (`:159`); HOLD-6 (`:330`). `now >= at_peak` single value: in the first UI-16 test (equal, above, 0 and 0).
B2: `UI-19 a value with no selector term filters by exact match in the browser` (`:995`), `UI-19 an exact-match chip combines with the selector box and the State filter` (`:1020`), `UI-19 an exact-match filter whose value is no longer in label_index stops filtering` (`:1051`); amended UI-9 tests at `:380` (chip names) and `:455` (renamed from "cannot be activated and says why" to "UI-9 a value with no term is a toggle that never reaches the selector box or the API, and says how it is matched").
Extra: `:838` (no Charging line when `metering_description` is null; "not measured yet"), `:957` (State filter hides the row of an open panel).
Unchanged: the four acquire-dialog UI-12 tests, the other UI-9 tests, HOLD-1/2 tests.

Mutation check (then restored): `now !== at_peak` in the cell, a panel that mixes the newest response's `pricing` into the kept snapshot, no exact-match row filter, no `fallbackFocus`, the panel ignoring `now > at_peak`, the panel requiring `pricing`: 8 tests failed.

## Commands and outcome

- `npx tsc --noEmit -p tsconfig.json` in `ui/`: exit 0 (`tsc.log`). The repo has no separate UI linter.
- `npx vitest run` in `ui/`: 18 files, 276 tests passed (`vitest-all.log`). Setups file alone: 56 passed.
- `./ci.sh` once: exit 0; 40 Bazel test targets pass, including `//ui:test` and `//ui:typecheck` (`ci.log`, `ci.exit`).
- Not run: e2e. No git state changes.

## Files changed

`ui/src/pages/setups/index.tsx`, `SetupPanel.tsx` (new), `LabelChips.tsx`, `setups.module.css`, `setups.test.tsx`; `ui/src/pages/usage/weight.tsx`; `ui/src/pages/admin-settings/index.tsx`; `ui/src/components/index.ts`. Task folder: `PROGRESS.md`, `RESULT.md`, `tsc.log`, `vitest-all.log`, `ci.log`, `ci.exit`.

## For the e2e task: controls and strings

- Table `All setups`; headers `Setup`, the keys, `State`, `Held by`, `Cost (tokens / h)`, `Actions` (sr-only).
- Cost button: role button, accessible name `2.0 tokens per hour now, up to 6.0. Price details for <name>` or `2.0 tokens per hour. Price details for <name>`; visible text `2.0 now · up to 6.0` or `2.0`. Select with `getByRole("button", { name: /Price details for <name>$/ })`. It carries the attribute `data-cost`; the first line of a holder cell is a `span[data-held]` (inline-block), for the one-line-box checks of U20.
- `Details` link: accessible name `Details for <name>`; use `exact: true` (the cost button's name ends with "Price details for <name>").
- Tag: badge text exactly `reported` / `override`, no `title`; sibling sr-only "The price comes from a reported weight." / "The price comes from a weight pinned by an admin."
- Held by: `You · 3 h`, `Sam Lee · 5 d 2 h`; second line `overdrawn`.
- Label cell: `span` with `title` = full value inside a `td` with the Table module's `shrink` class.
- Dialog: role dialog, name `Details: <name>`; button `Close`; h3 `Price` (focused on open), `Holder`, `Labels and details`.
- Panel lines are `dt` / `dd` pairs in a `dl`. `dt` texts: `Weight`, `Attributes`, `Lab pressure`, `Multiplier now`, `Cost now`, `At the highest band`, `Charging`, `Held by`, `Held since`, `Balance`, `Release`, then one `dt` per label key. `dd` examples: `1.15 · reported (sent by automation)` (the weight `1.15` is its own `span`), `memory-gb 512 · storage-gb 4000`, `82%` / `not measured yet`, `2×`, `4.0 / h`, `6.0 / h (3×)` / `6.0 / h`, `Charging since <time>` / `Not charging yet: <reason>` / `Not charging yet.` / `Charging starts: <description>`, `61 tokens` / `−12 tokens, overdrawn`, `Release requested <time>. It completes when the setup is torn down.`, `(empty)`.
- Other panel text: the price sentence; "The bands were changed. The cost at the highest band shows after the next poll."; "Weights are reported by automation, so the price can change while the setup is held."; "No labels."; `pre` labelled `Setup details JSON`; `role="status"` "This setup is no longer listed. These are the last values loaded."
- Exact chip: role button, `aria-pressed`, name `Filter by <key>=<value> (exact match)`; note "Dashed values contain spaces or one of , ( ) = ! and are matched in the browser, not through the selector."
- Page h1 `Setups` now has `tabIndex={-1}` (focus fallback).
- Settings hint: "Comma separated; shown as columns on Setups. Every label of a setup is in its details, so a short list hides nothing."

## Differences from the docs' quoted wording (07 RESULT list)

- Item 10 / ui-notes item 4: the Weight line joins the number and the source words with a middle dot: `1.15 · rule (set by a rate rule)`. The docs say "`1.15`, with its source in words", which does not fix the separator; no doc edit is strictly needed.
- Item 13: "Charging starts: " + description has no closing full stop (DESIGN section 4); the acquire dialog's own line keeps its full stop. The docs quote no punctuation here.
- Not in the docs: a metering hold whose `started_at` is null reads "Charging." (DESIGN does not cover the case; inference that it cannot happen on a real server).
- Not in the docs: the "Release" line name in the Holder section (the docs give the sentence only).
- Everything else in items 1 to 19 is as quoted.

## Deviations from DESIGN.md

1. B2 `exactFilters`: a filter is in force only while its key/value is still in `label_index` with a null selector (`index.tsx:81-83`). Without this a filter whose chip has disappeared would hide rows with no control left to clear it. If the value returns, the filter applies again. Tested (`:1051`).
2. The exact chip has no `title` (the old inert chip had one); its name and the note under the block carry the explanation.
3. The `now > at_peak` replacement sentence is shown as the value of the "At the highest band" line, so the list keeps its shape.
4. The old test "filters by state and by label on the fetched list" is replaced by the UI-19 test as DESIGN says; the old UI-9 "cannot be activated" test is renamed (above).
5. `.releasing` and `.labelSel` CSS removed; `.reason` replaced by `.costNote`; `.chipUnusable` replaced by `.chipExact` (dashed, no strike-through).

## Findings outside my scope

- `docs/design/ledger-labels-onboarding.md:204,372` still states the old UI-9 ("a value with no term cannot be activated"); already reported by task 07.

## Confidence and gaps

- High for behaviour covered by the unit tests, the type check and `./ci.sh`.
- Not verified: anything that needs layout. jsdom does none, so UI-20 itself (no sideways scroll at 1280 px, one-line cost and holder cells, the 12em note) rests on the design's measurements and on B5's U20 step. Inference: the page emits the same structure the prototype measured (`.costNote`, `.overdrawnNote`, inline-block holder line, 12 ch holder clip, dense table).
- Not run in a browser: focus ring on the Price heading, the panel's `dl` grid with a very long label key (keys are capped at 16em and wrap anywhere), a 256 KB details JSON.
- UI-1: the page and panel contain no arithmetic on API numbers; the only numeric operation is the comparison of `now` with `at_peak` (twice). `formatPercent` rounds for display, as on other pages.
- Generic: new assertions use neutral keys (`label-a`..`label-e`, `owner`, `site`, `size`, `not-shown`, `blank`); `memory-gb`/`storage-gb`/`tier` in the UI-12 tests were already there as fixture values.
