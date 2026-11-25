# 13 (C1) Setups page: result

Status: **done**. 2026-11-24. Nothing committed; no git state changed.

## What I did

- The Setups table is Setup, State, Held by, Cost (tokens / h), actions. No label column, no "Available labels" block, no chip.
- The Label selector box is a combobox (`SelectorBox.tsx`) that suggests keys, operators and values from `label_index` for the caret's position (`selectorSuggest.ts`, pure functions). A typed value is also offered as its whole term under "Values".
- A value no selector can express is suggested with the note "exact match"; picking it takes its requirement out of the box and adds a removable token on a filters-in-force line, applied in the browser. Nothing about it goes to the API.
- The filters-in-force line (tokens, row count, "Clear filters") shows only while State, the applied selector or a token is in force.
- Cost cell, Held by cell, filter row and details panel follow UX-SPEC sections 2 to 5.
- Dead code of the first build is gone: `LabelChips.tsx`, `labelTerms.ts`, `shrink`, `dense`, `ShrinkText`, chip classes, the chip filter.
- The three findings of the first verification are fixed (below).

## Evidence

Paths are under `ui/src/`.

| Thing | Where |
| --- | --- |
| Caret classification, suggestions, pick | `pages/setups/selectorSuggest.ts:88` (`contextAt`), `:217` (`suggest`), `:278` (`applyPick`), `:21` (`placeholderFrom`) |
| Combobox, list, keys, composition | `pages/setups/SelectorBox.tsx:199` (role and ARIA), `:134` (composition), `:155` (Enter), `:163` (Escape), `:242` (listbox), `:278` ("N more") |
| Tokens, row filter, Clear filters, status line | `pages/setups/index.tsx:95`, `:121`, `:135`, `:371`, `:399` |
| Cost and holder cells | `pages/setups/index.tsx:218` to `:247` |
| Panel order and wording | `pages/setups/SetupPanel.tsx:58` to `:159` |
| Minor 1, reason with no space | `pages/setups/setups.module.css:69` (`overflow-wrap: anywhere`). Browser: a 72-character one-word reason stays inside its cell |
| Minor 2, sideways page scroll | Root cause is the shared table scroller, not `.sr-only`: `components/Table.module.css:5` (`position: relative`). Browser: page `scrollWidth` equals the window at 768, 390 and 375 px. `styles/tokens.css` is not changed |
| Minor 4, weight below 0.0005 | `lib/format.ts:31` (`formatWeight`): when three decimals would print `0` for a weight that is not zero, the API value is printed unrounded (`0.0004`). Used on the panel's Weight line only. UX-SPEC says nothing on this, so the brief's fallback applies |
| Shared table | `components/Table.tsx` is identical to `main` again; `components/index.ts` likewise |

Other pages touched by the scroller fix: every page that uses the shared `Table` (My usage, Setup inventory, Users, Service accounts, Audit log and the rest). Effect: hidden text in a cell (the "Actions" header) now scrolls with the table. No visible change at widths where the table fits. Not checked page by page (inference from the CSS).

## Controls, for the e2e task

| Control | Role and accessible name | Visible text / notes |
| --- | --- | --- |
| Selector box | `combobox` "Label selector" (plain `textbox` when `label_index` is empty or absent); `getByLabel("Label selector")` works in both | Placeholder `Type or pick a label, e.g. {term}`; `aria-expanded`, `aria-controls`, `aria-activedescendant`, `aria-autocomplete="list"` |
| Arrow in the box | `button` "Show label suggestions", `tabindex="-1"` | No text. Absent with an empty index |
| Suggestion list | `listbox` "Label suggestions" | Present only while open |
| Group | `group` named by its caption | "Label keys", "Values", "Values of {key}", "Match {key}" |
| Key option | `option` "`{key}, values: {a, b, c}`" then ", …" when more | Left text is the key. Use a regex (`/^size/`) or the full name |
| Operator option | `option` "`=, is`", "`!=, is not`", "`in (, is one of`", "`notin (, is none of`" | |
| Value option | `option` "`{value}`" (name is the value alone) | After `=`, `==`, `!=` and in lists |
| Whole-term option | `option` "`{key}={value}`" | Under "Values" |
| Empty value | `option` "`(empty), empty value`" | |
| Exact value | `option` "`{value}, exact match`" after `key=`; "`{key}={value}, exact match`" under "Values" | |
| No-match line | not an option | `No label matches "{typed}".` / `No value of {key} matches "{typed}".` |
| More line | not an option | `{n} more. Keep typing to narrow.` |
| Filter | `button` "Filter" | Description "The typed selector is not applied yet." while the box differs from the applied selector |
| Selector error | text under the box | The API's message; then `Forms: key=value, key!=value, key in (a,b), key notin (a,b), key, !key. Commas mean AND. The table still shows the last filter that worked.` The API's message is also in the status line, so `getByText(message)` needs `{ exact: true }` |
| Token list | `list` "Exact match:" | Lead-in text `Exact match:` |
| Token | `listitem`, text `{key}={value}` (also its `title`) | |
| Token remove | `button` "Remove exact-match filter {key}={value}" | `×` |
| Count | text | `1 setup shown` / `{n} setups shown` (no full stop). The status line says the same with a full stop: use `{ exact: true }` |
| Clear filters | `button` "Clear filters" | |
| Status line | `role="status"`, visually hidden, present once the list has loaded | `{n} suggestions.`, `1 suggestion.`, `No suggestions.`, `{n} setups shown.`, `1 setup shown.`, `No setups match the filters.`, `Exact-match filter {k}={v} added. {count text}`, `… removed. {count text}`, `Selector not accepted. {message}`, `Filters cleared.` |
| Cost button | `button` "`{now} tokens per hour now, up to {peak}. Price details for {setup}`" (unchanged) | Hook `[data-cost]` kept. "Details for lab-02" as a substring also matches this button: use `exact: true` |
| Details | `button` "Details for {setup}" | `Details` |
| Held by line | hook `[data-held]` kept | `You · 3 h` |
| Panel | `dialog` "Details: {setup}" | Headings (h3): Price, Holder, Labels, Raw details. Lines (`dt`): Cost now, Up to, Charging, Weight, Attributes, Lab pressure, Multiplier now; Held by, Held since, Balance, Release; then one `dt` per label key, sorted |
| Panel values | | `1.6 tokens / h`; `2.4 tokens / h at the highest multiplier (3×)`; Charging `Since {time}`, `Yes`, `Not yet: {reason}`, `Not yet`, `Starts: {description}`; empty label value `(empty)`; no labels `No labels.`; no details `None.` (the JSON block, named "Setup details JSON", is then absent) |

## Files changed

- New: `ui/src/pages/setups/selectorSuggest.ts`, `selectorSuggest.test.ts`, `SelectorBox.tsx`
- Changed: `ui/src/pages/setups/index.tsx`, `SetupPanel.tsx`, `setups.module.css`, `setups.test.tsx`; `ui/src/components/Table.tsx`, `Table.module.css`, `index.ts`, `components.test.tsx`; `ui/src/lib/format.ts`, `format.test.ts`
- Deleted: `ui/src/pages/setups/LabelChips.tsx`, `labelTerms.ts`
- Task folder: `PROGRESS.md`, `RESULT.md`, `tsc.log`, `vitest.log`, `mutation.log`, `ci.log`, `ci.exit`, `browser/` (script, log, screenshots)

## Commands

| Command | Outcome |
| --- | --- |
| `npx tsc --noEmit -p tsconfig.json` (in `ui/`) | exit 0 |
| `npx vitest run` (in `ui/`) | exit 0. 19 files, **300 tests, 300 passed**. Setups folder: 73 (59 in `setups.test.tsx`, 14 in `selectorSuggest.test.ts`) |
| Mutation check, 10 mutations (`mutation.log`) | every one red; files restored and compared byte for byte |
| `node browser/check.cjs` (Chromium, vite dev server, API mocked) | 60 checks, 0 failed, at 1280 (developer and admin), 768, 390, 375 px |
| `./ci.sh`, once | **exit 0**; 40 Bazel tests pass, including `//ui:test`, `//ui:typecheck`, `//internal/api:api_test` |
| Lint | The repository has no lint step for the UI beyond `tsc` and the SPDX, licence and generic checks in `ci.sh`; all passed |
| `grep -rnE "shrink|ShrinkText|dense|LabelChips|applyChip|labelTerms" ui/src` | no match. `chip` remains only as the product's `Chip` component, the `--radius-chip` token and other pages' own classes |

Mutations and the test that went red:

| Mutation | Red test (one of) |
| --- | --- |
| Suggestion ignores the caret | `UI-21 a pick replaces the whole token under the caret, also from the middle, and puts the caret after it` |
| A value pick replaces from the start of the requirement | same, and `UI-21 the selector box is a combobox: …` |
| Tokens do not filter the rows | `UI-22 picking a value with no selector term adds an exact-match token, …` |
| Escape also clears the text | `UI-21 Enter with no active option submits the typed text; Escape closes the list and keeps the text; Tab picks nothing` |
| Enter with no active option picks | same |
| An exact pick applies the box | `UI-22 a value with no term is offered only after = and ==, …` |
| The list opens on focus | `UI-21 the list opens on click and on Arrow Down in an empty box, and not on focus alone` |
| Tab picks the active option | `UI-21 Enter with no active option …` |
| The same exact value twice adds a second token | `UI-22 a token combines with the selector and the State filter; …` |
| Keys handled during composition | `UI-21 keys pressed during composition are left to the input method` |

## Tests named by DESIGN-R3 for C1

All exist under the names of DESIGN-R3 section 4 and pass, with these changes of name or content:

- `UI-9 the placeholder is the first non-null term of the index` is now `UI-9 the placeholder is the first term of the index that is at most 16 characters, after the words Type or pick a label` (UX-SPEC B1).
- Added for UX-SPEC: `UI-21 typed text at the start of a requirement also lists the values that match, as whole terms` (B4), `UI-21 awkward text never throws …`, `UI-21 a typed value is listed under Values …`, `UI-21 more than 50 matches …`, `UI-21 the placeholder names a term …`, `UI-21 a refused selector shows the API's message and the forms line …` (B17, B10), `UI-21 the Filter button says when the typed selector is not applied yet` (B18), `UI-22 a value with no term typed at the start of a requirement …`, `UI-22 Clear filters resets …` (B7), `UI-22 Clear filters is announced, and the 60-second refresh announces nothing`, `UI-16 the tag is beside the cost and the second line holds only a not-charging reason` (A3), `UI-18 a weight that three decimals would show as 0 …`.
- `RULE-2 …` passes unedited.
- Kept tests whose assertions I had to change, and why: `UI-1 not-yet-charging …` and the populated-table test read the cost through the button's visible text, because the cost now is in its own element (A2); `UI-17` reads the third cell, not the fourth (the label column is gone); the `UI-18` tests use the panel's new wording (`tokens / h`, "Up to", "Since", "Not yet", "Starts:", the four headings, "None."). The UI-12, WGT-14, HOLD and acquire-dialog assertions are unedited.

## UX-SPEC change list

| # | Level | State |
| --- | --- | --- |
| A1 cost button look | must | done |
| A2 cost now bold, rest regular or muted; name unchanged | must | done |
| A3 tag on line 1, 8 px after; second line is the reason only, wraps anywhere | must | done (browser: gap 8 px, same line) |
| A4 filter row top-aligned, button offset, form does not wrap | must | done (browser: tops equal before and after an error, at five widths) |
| A5 panel Price order | must | done |
| A6 `tokens / h`, "Up to", "(3×)", the multipliers text | must | done |
| A7 headings Labels and Raw details | must | done |
| A8 scroller `position: relative` | must | done |
| A9 16 px costs | should | done |
| A10 Charging values | should | done |
| A11 10 em first column, 8.5 em below 480 px | should | done (browser: Price and Holder values at one x) |
| A12 labels list | should | done |
| A13 hint under the labels | should | done |
| A14 holder name 24 ch, "You" bold | should | done |
| A15 monospace typed text, sans placeholder | should | done |
| A16 "None." | nice | done |
| B1 placeholder | must | done; an empty-value term (`tier=`) is also skipped as an example |
| B2 captions | must | done |
| B3 key note | must | done |
| B4 values at the start of a requirement | must | done; not offered where an operator already follows the token (the term would be written in front of it) |
| B5 no repeated term | must | done |
| B6 tokens on the filters-in-force line | must | done |
| B7 Clear filters | must | done |
| B8 active and hover looks, 36 and 44 px | must | done (36 px and the bar seen in the browser; 44 px is a `pointer: coarse` rule, not exercised) |
| B9 list geometry, z-index 30 | must | done (browser) |
| B10 status texts | must | done, with one combination: see deviations |
| B11 groups, key option name | must | done |
| B12 count | should | done |
| B13 no-match line | should | done |
| B14 arrow button | should | done |
| B15 operator words | should | done |
| B16 pinned "N more" | should | done (outside the scroller) |
| B17 forms line | should | done |
| B18 Filter primary and described | nice | done |
| Not mine: section 7 (Settings, task 14), docs, e2e, the owner screenshots | | not done here |

## Deviations

1. **Empty or missing `label_index`: no combobox at all.** DESIGN-R3 says the operator suggestions still show without an index; the brief says "a plain text box". I followed the brief: no role, no list, no arrow, no placeholder.
2. **Token announcements carry the count in the same sentence** ("Exact-match filter owner=Team A (infra) added. 2 setups shown."). UX-SPEC lists the two texts separately; one status line cannot hold both at once without the first being replaced before it is read.
3. **Option names are `aria-label`s**, not text plus a hidden "values:" span as UX-SPEC 3.10 words it. The spoken text is the one specified. Reason: the name computed from separate elements differs between engines (a space before the comma in jsdom).
4. **Enter during composition is cancelled on the box** so it cannot submit the form. Inference: in browsers the input method has already taken the key; not tried with a real input method.
5. **`formatWeight`** is a new exported function in `lib/format.ts` (brief minor 4). Not used by the acquire dialog or My usage, which still print three decimals.
6. **Home and End clear the active option** (the list is recomputed for the new caret position). UX-SPEC says they "never move the active option"; they do not move it to another option.
7. Operator picks `=` and `!=` close the gap to the key (`site ` becomes `site=`); `in (` keeps the space. DESIGN-R3 says only "inserts the operator".
8. While a refused selector is the applied one, the filters-in-force line still shows (the selector is not empty) with the count of the rows of the last filter that worked. UX-SPEC does not cover the case.

## Findings outside my paths

- `ui/src/pages/admin-settings/index.tsx:423` has its own table with a visually hidden "Actions" header and no scroller around it. At narrow widths it may widen that page the way minor 2 describes. Not checked; task 14's file.
- At 1280 px the table fits with a 32-character name, a 24-character holder name held for days with "overdrawn", and a cost of `123.456 now · up to 370.368` with a tag (columns 273 + 166 + 259 + 292 + 163 = 1153 in a 1152 px box, no scroll reported by the browser). With costs above 10,000 tokens per hour on the same row set it needs 1184 px and scrolls by 32 px. The wider holder name (A14) is what uses the room.

## Confidence and gaps

- High: suggestion logic, pick behaviour, tokens, filters and wording (unit tests, mutation check). Layout at the five widths (Chromium, mocked API).
- Not done: a screen reader; Safari and Firefox; a touch device (the 44 px option height, tap outside); a real input method; the real server (e2e is task 16). The browser check used the vite dev server, not the production bundle (CSS is the same source).
- Inference: that other pages using the shared table look the same after the scroller fix.
- The suggestion list's group caption scrolls away with a long list; only the "N more" line is pinned, as specified.

## Fix round (task 19, 2026-11-24)

Status: **done**. From `tasks/17-verify/RESULT.md` and `tasks/12-ux/UX-ACCEPTANCE.md`. Wrote only `ui/src/pages/setups/*` and one cell of `docs/progress.md`. No git state change.

### Fixed

| # | Finding | Fix | Test that is red without it (`fixround-revert.log`) |
| --- | --- | --- | --- |
| 1 | Verify should-fix 1: "No setups to show." under a selector that matches nothing | `index.tsx`: the empty message is "No setups match the filters." whenever a filter is in force, "No setups to show." otherwise | `UI-21 a selector that matches no setup says the filters match nothing, in the table as in the status line` |
| 2 | Verify should-fix 2: `docs/progress.md` names a test that does not exist | The UI-9 row now cites `UI-9 the placeholder is the first term of the index that is at most 16 characters, after the words Type or pick a label`. The other 46 test names in lines 116 to 158 were checked by script against the test files: all exist (the one other miss is the old name `SET-6 populated …`, quoted as history) | n/a |
| 3a | Verify minor 1: `site \|=sjc` pick `!=` gave `site!==sjc`; `site \|sjc` pick `=` deleted `sjc` | `selectorSuggest.ts`: operators are offered only where nothing follows the caret in its requirement | `UI-21 a pick never leaves two operators, deletes the word after the caret or joins two list items` |
| 3b | Verify minor 1: `site in (ams,\| sjc)` pick gave `(ams,lon sjc)` | A list pick adds a comma when, and only when, another item follows with nothing but spaces between: `(ams,lon, sjc)`. Elsewhere still no comma and no `)` | same test |
| 3c | Verify minor 3: an exact-match pick can leave an applied requirement in force that the box no longer shows | Made visible, no behaviour change: while the applied selector is not empty and differs from the box, the filters-in-force line reads **Selector in force: `{applied selector}`**. Filter applies the box as before | `UI-22 when an exact-match pick leaves the box different from the applied selector, the line names the selector in force` |
| 3d | Verify minor 6: a setup with no `labels` object crashed the page with a token in force | `index.tsx`: `s.labels?.[key]` | `UI-22 a setup that comes without a labels object does not match a token and does not break the page` |
| 4 | UX S1: Filter filled blue while the list is open | Filter is the primary variant only while the typed text is unapplied **and the list is closed** (`SelectorBox` reports through a new optional `onListOpen` prop). The described-by text is unchanged | `UI-21 the Filter button is the filled button only while typed text is unapplied and the list is closed` |
| 5 | UX P1: placeholder cut at 390 px | In a window of 480 px or less the placeholder is "Type or pick a label" (no example); `placeholderFrom(index, narrow)` | `UI-9 in a narrow window the placeholder has no example`, and one in `selectorSuggest.test.ts` |
| 6 | UX P2: caption scrolls away | `.caption` is sticky at the top of the list's scroller | CSS only; browser check below |

### Left as is

- Verify minor 4, typing past a space in a value closes the list: not trivial (a space ends a token everywhere in the caret logic); values that differ after the first word are picked from the list.
- Verify minor 5, costs above 10,000 tokens per hour scroll the table at 1280 px: accepted by the UX report; the table scrolls inside its own box.
- Verify minor 2, Settings sending the keys as first loaded: outside my paths (`admin-settings/form.ts`).
- Verify minor 7, the scroller's `position: relative` and the cost note's `overflow-wrap` have no unit test: jsdom applies no CSS. Both are in `browser/check.cjs`; the e2e step covers the table layout.
- UX P3 and P4: no change, as the designer says. P5 is for the e2e task (panel shots in a 1280 × 1100 window).

### For the e2e task: what changed in roles, names, strings

- **New visible string**: `Selector in force: {selector}` on the filters-in-force line, only while the applied selector is not empty and differs from the box's trimmed text. It also shows while a user types a new selector over an applied one, until Filter or a pick applies it. The selector itself is in a `code` element.
- **Changed table message**: an applied selector (or State, or a token) that leaves no row gives **No setups match the filters.** also when the API returned no setup at all. "No setups to show." appears only with no filter in force.
- **Placeholder**: at a viewport of 480 px or narrower it is `Type or pick a label`; wider, `Type or pick a label, e.g. {term}` as before. A2f at 1280 px is unaffected.
- **Filter button**: same role, name and description. It has the primary look only with unapplied text and the list closed.
- **Operators** are no longer offered when text follows the caret in the same requirement (`site |=sjc`). `site ` at the end of a requirement lists them as before.
- **List pick** before another item (`(ams,| sjc)`) inserts `value,`.
- No role, accessible name or test hook changed. No existing test was renamed.

### Commands

| Command | Outcome |
| --- | --- |
| `npx tsc --noEmit -p tsconfig.json` | exit 0 |
| `npx vitest run` | exit 0; 19 files, **307 of 307** (Setups folder 80: 64 in `setups.test.tsx`, 16 in `selectorSuggest.test.ts`) |
| Each fix reverted in turn (7 reverts) | each makes exactly its test red; files restored and compared |
| `node browser/check.cjs` (Chromium, 1280 developer and admin, 768, 390, 375) | 70 checks, 0 failed: placeholder reads whole at every width (125 of 192 px at 390), caption in view in a scrolled list, Filter not filled while the list is open |
| `./ci.sh` | exit 0; 40 tests pass, 4 executed, among them `//ui:test` and `//ui:typecheck` |

### Confidence and gaps

- The sticky caption and the narrow placeholder were seen in Chromium only. The placeholder follows the window width through `matchMedia`; a browser without it keeps the long form.
- "Selector in force" is my wording; the UX designer has not seen it.
