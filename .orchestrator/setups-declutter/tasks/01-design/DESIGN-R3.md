# Design, revision 3: no label columns, no chips, autocomplete in the selector box

Date 2026-11-24. A delta to `DESIGN.md` (revision 2), which stays as it is. Designed against the uncommitted working tree on branch `setups-declutter` (the first build, tasks 05 to 09). Nothing outside this task folder was changed.

Source of the change: the amendment at the end of `.orchestrator/setups-declutter/INTENT.md` and the last two entries of `DECISIONS.md` (the owner's words and R3-1 to R3-3).

Words used here: "is" marks what I read in the tree or measured; "appears to" marks inference. Paths are relative to the repository root. Line numbers are those of the working tree.

## Summary

| Before (first build) | After |
| --- | --- |
| Table: Setup, one column per display label key, State, Held by, Cost, actions | Table: Setup, State, Held by, Cost, actions |
| "Available labels" chips under the selector box | Gone. The selector box suggests label keys and values as the user types |
| A chip for a value with no selector term filters by exact match | Such a value is suggested after `key=`; picking it adds a removable exact-match token beside the box |
| Labels in columns and in the panel | Labels in the panel only |
| Settings page has a "Display label keys" field | No field. The API field and the stored value stay; the Settings page sends the stored value back unchanged |

Stays as built: the cost cell, the Held by cell, the details panel (`ui/src/pages/setups/SetupPanel.tsx`), `pricing` on `GET /api/v1/setups`, the `Modal` changes, `formatRate`, `PriceSourceTag`. No API change of any kind in this revision.

Three findings that shape the design:

1. **The Settings page must keep sending `display_label_keys`.** `PUT /api/v1/admin/settings` refuses a body without it (`internal/api/handlers_admin.go:858-860,876-880`). "Nothing reads it" therefore cannot mean "the page drops it": the page keeps the array it loaded and sends it back untouched.
2. **Two acceptance steps break if only the page changes.** A0 fills and reads the "Display label keys" field (`e2e/browser/tests/acceptance.spec.ts:321,337`), and A3 reads the label value `large` from the table row (`acceptance.spec.ts:481`).
3. **Label discovery now rests on the suggestion list.** The chips existed so that a developer can see which labels exist (`docs/design/ledger-labels-onboarding.md:16`). So the list must open without typing (click or Arrow Down in an empty box), not only narrow what was typed.

## 1. Autocomplete design

### The grammar being completed

From the parser, `internal/rules/selector.go`:

| Form | Where |
| --- | --- |
| Requirements joined by `,`; an empty selector matches everything | `selector.go:135-156` |
| `!key` (key absent) | `selector.go:164-175` |
| `key` alone (key present) | `selector.go:176-186` |
| `key=value`, `key==value` | `selector.go:188-193` |
| `key!=value` | `selector.go:194-199` |
| `key in (v1,v2)`, `key notin (v1,v2)`; a list value may not be empty | `selector.go:203-241` |
| Spaces, tabs and newlines are allowed around every token | `selector.go:95-99` |
| A key or value token is any run of characters other than whitespace and `, ( ) = !` | `selector.go:103-113` |
| `key=` with nothing after it is valid and matches an empty value | `selector.go:244-258`, `266-281` |

The page does not parse or validate. It only finds which token the caret is in, to choose what to suggest. The API remains the one judge of a selector (`internal/api/handlers_user.go:792-800`).

### Where suggestions come from

`label_index` of the setups response the page already has (`ui/src/pages/setups/index.tsx:77`; built at `handlers_user.go:831-835,883`): every key and value over every setup the caller may see, sorted, ignoring the selector, each value with its ready-made term or `selector: null`. No new API. The index ignores the selector, so the list does not shrink as the user filters.

### What is suggested at each caret position

The page takes the text of the current requirement (between top-level commas; a comma inside parentheses does not split) up to the caret, and classifies it. "Partial" is the token the caret is in or just after.

| Caret position | Example (`|` is the caret) | Suggested | Picking does |
| --- | --- | --- | --- |
| Start of a requirement: empty box, after a top-level comma, or inside a first token | `|`, `site=sjc,|`, `si|`, `site=sjc, ti|` | Keys of `label_index` matching the partial | Replaces the whole token with the key and adds `=`; the value list opens at once. Does not apply the filter |
| After `!` at the start of a requirement | `!|`, `!ti|` | Keys | Replaces the token with the key; adds nothing. Does not apply |
| After a complete key and a space | `site |`, `site no|` | The four operators: `=`, `!=`, `in (`, `notin (` | Inserts the operator; the value list opens |
| After `=`, `==` or `!=` | `site=|`, `site=s|`, `site != |` | Values of that key matching the partial. After `=` and `==` also the values with no selector term, marked "exact match", and the empty value as "(empty)". After `!=`: only values that have a term, and "(empty)" | A value with a term: replaces the value token; applies the filter. "(empty)": leaves `key=`; applies. A value with no term: see "The exact-match token" |
| Inside an `in (` or `notin (` list | `site in (|`, `site in (ams, s|` | Values of that key that have a term, are not empty and are not already in the list | Replaces the list item. Adds no comma and no `)`. Does not apply |
| After a closed list or a complete value followed by a space | `site in (a,b) |`, `site=sjc |` | Nothing | |
| The key before the caret is not in `label_index` | `nosuch=|` | Nothing | |

Rules that hold everywhere:

- **Replace the whole token.** A pick replaces the token under the caret from its first to its last character, also when the caret is in the middle (`si|te=sjc` and a pick of `size` gives `size=sjc`; `=` is not added when an operator already follows). The caret goes to the end of what was inserted.
- **A value is inserted only as the API wrote it.** For a value with a term the page inserts `value`, which the API has already checked can stand as a bare token (`selector.go:270-279`). The page builds no escaping, because the grammar has none.
- **Applying.** Picking a value after `=`, `==` or `!=` applies the whole box, as the Filter button does (`index.tsx:111-114`): the one-action filtering the chips gave. If another requirement in the box is unfinished, the API's parse error shows under the box as today. Picking a key, an operator or a list item does not apply.
- **Matching** ignores case. Keys or values that start with the partial come first, then those that contain it; inside each group the API's order (sorted) is kept.

### Limits

- At most 50 options are rendered (`MAX_OPTIONS`). When more match, a last line that cannot be picked says "N more. Keep typing to narrow." One key may have any number of values and an installation any number of keys (32 labels per setup, `internal/rules/labels.go:15`, but no limit across setups).
- The work per keystroke is one pass over the keys, or over one key's values. With 500 keys of 50 values each that is 500 or 50 comparisons. No debounce is needed.
- The list is 8 options tall and scrolls; the active option is kept in view.

### Empty index, older API

`data?.label_index ?? []` (`index.tsx:77`) already covers an API that sends no `label_index`. With an empty index there are no key or value suggestions and the list never opens for them; the box is a plain text box that works as today. The placeholder is omitted, as today (`placeholderFrom`, now in `selectorSuggest.ts`). The operator suggestions need no index and still show.

### The exact-match token (R3-2)

- **When offered.** After `key=` or `key==`, a value whose `selector` is null is in the list with the words "exact match" beside it.
- **Picking it** removes the current requirement (`key=partial`, with one adjoining comma) from the box and adds a token for that key and value. The box is not applied by this pick; nothing about the token goes to the API. Focus stays in the box.
- **The token** shows beside the Filter button in a list named "Exact-match filters": the text `key = value`, the words "exact match", and a button named "Remove exact-match filter key=value". An empty list is not rendered.
- **Filtering.** Rows are kept when `labels[key] === value`, the comparison the first build already makes (`index.tsx:84-87`), in the browser over the fetched list.
- **Combining.** Token AND selector (API) AND State filter (browser). One value per key: picking another exact value of the same key replaces the token; picking the same value again changes nothing.
- **Contradiction.** A token `owner = Team A (infra)` with a selector `owner!=…` or `!owner` simply matches nothing; the table says "No setups match the filters." (`index.tsx:320`) and the token is there to remove. The page does not detect it.
- **Clearing.** The token's button. After removal focus goes to the selector box, because the button is gone. Editing or clearing the box does not touch tokens. A reload clears everything, as it does for every filter (all state is local, `index.tsx:53-57`).
- **A token whose value disappears from `label_index`** stays shown and in force and matches nothing. The first build dropped such a filter silently because its chip had vanished (`index.tsx:80-83`); with a token the control to clear it is still on screen, so the rule goes.
- **Not offered** after `!=` or in a list: the owner asked for exact matching, and a negative or multi-value browser filter is a new feature.

### ARIA and keys

The combobox pattern with a list popup (WAI-ARIA 1.2), focus always in the input.

| Element | Attributes |
| --- | --- |
| The input (still inside `Field`, so label, error and `aria-invalid` are wired as today, `ui/src/components/Field.tsx:21-43`) | `role="combobox"`, `aria-autocomplete="list"`, `aria-expanded`, `aria-controls` = the list's id, `aria-activedescendant` = the active option's id, absent when none is active; `autocomplete="off"` |
| The list | `role="listbox"`, `aria-label="Label suggestions"` |
| An option | `role="option"`, a stable id, `aria-selected="true"` on the active one. Text: the key; or the value with the resulting term in muted text (`site=sjc`); or the value with "exact match" |
| The "N more" line | `role="presentation"`, not an option |
| Announcements | One `role="status"` line, visually hidden: "12 suggestions", "No suggestions", "Exact-match filter owner=Team A (infra) added", "… removed" |

| Key or action | List closed | List open |
| --- | --- | --- |
| Typing, deleting, pasting | Opens if there are suggestions for the caret position | Narrows; no option is active after an edit |
| Arrow Down / Arrow Up | Opens; Down makes the first option active, Up the last | Moves the active option, wrapping |
| Enter | Submits the form, as today (`index.tsx:283`) | With an active option: picks it and does not submit. With none active: closes the list and submits |
| Escape | Nothing. It never clears the text | Closes the list; the text stays |
| Tab | Moves focus | Closes the list, picks nothing, moves focus |
| Home, End, Left, Right | The caret, as in any text box | The caret; the list follows the new position |
| Click or tap in the box | Opens if there are suggestions | |
| Click or tap on an option | | Picks it. `mousedown` on the list is prevented so the input keeps focus |
| Blur | | Closes |

- **No option is active until the user moves into the list.** So Enter on typed text always submits that text. This is what keeps free typing exactly as today.
- **Composition (IME).** Between `compositionstart` and `compositionend` the list does not recompute and Enter, Escape and the arrows are left to the input method.
- **Focus by Tab alone does not open the list**; a click, a keystroke or Arrow Down does.
- **Undo.** A pick sets the value from code, so the browser's undo history for the box appears to be lost at that point (inference; not measured).

### Free typing and errors are unchanged

The form, its submit handler, the `selector=` request and the error under the box are not touched: `applySelector` (`index.tsx:111-114`), `getSetups` (`index.tsx:22-23`), `selectorError` (`index.tsx:255,284`). The existing test `RULE-2 the label selector goes to the API as selector= and a parse error is shown` (`ui/src/pages/setups/setups.test.tsx:212`) must pass unedited. Tests find the box with `getByLabelText("Label selector")`, which still works.

### Files

| File | Content |
| --- | --- |
| `ui/src/pages/setups/selectorSuggest.ts` (new) | Pure functions: `contextAt(text, caret)` (which position, which key, the partial, the token's range); `suggest(context, labelIndex)` (the option list and the count beyond 50); `applyPick(text, context, option)` (new text, new caret, whether to apply, or the exact token); `placeholderFrom` (moved from `LabelChips.tsx:14-17`) |
| `ui/src/pages/setups/SelectorBox.tsx` (new) | The combobox: the `Field` and `Input`, the list, the keys, the status line. Props: `value`, `onChange`, `onApply(text)`, `onExact(key, value)`, `labelIndex`, `error`, `placeholder` |
| `ui/src/pages/setups/index.tsx` | Uses `SelectorBox`; renders the token list; keeps `exactFilters` |
| `ui/src/pages/setups/setups.module.css` | The list, options, token styles; `.selectorInput` widens from 260 to 360 px, now that the chips' row is gone. The list sits under the box with a `z-index` below the dialog's 40 (`ui/src/components/Modal.module.css:5`) |

## 2. What is removed from the first build

| File | Removed | Kept |
| --- | --- | --- |
| `ui/src/pages/setups/index.tsx` | `keys` (line 74); the label columns and the `ShrinkText` import (lines 10, 200); `dense` (line 319); `LabelChips` and its import (lines 17, 299-307); `activateChip` (lines 104-109); `splitTerms`, `applyChip` import (line 18); the `exactInForce` rule (lines 80-83); the header comment's "admin-chosen label columns (SET-6)" (line 2) | Everything else: cost cell (160-183), Held by cell (185-195), `nowrap` on the five columns, the panel and snapshot (96-102, 324-332), `exactFilters` state (57) and its row filter (84-87), the acquire dialog |
| `ui/src/pages/setups/LabelChips.tsx` | The whole file. `placeholderFrom` moves | |
| `ui/src/pages/setups/labelTerms.ts` | The whole file: `splitTerms`, `isEqualityTermOf` and `applyChip` served only the chips (`index.tsx:18,106,302`). The new code has its own caret-aware splitter | |
| `ui/src/pages/setups/setups.module.css` | The chip and label-block classes (lines 28-40: `.chips`, `.labels`, `.labelsHead`, `.labelsTitle`, `.labelKey`, `.labelKeyName`, `.chipList`, `.chip…`) | `.costNote`, `.holderLine`, `.overdrawnNote`, the panel classes |
| `ui/src/components/Table.tsx`, `Table.module.css` | `shrink` on `Column`, the width and `title` on shrink headers, `ShrinkText`, `dense`, and their CSS (`Table.tsx:14-19,33,44-60,80,99-106`; `Table.module.css:42-62`). Setups was the only user (`ui/src/pages/setups/index.tsx:10,200,319` are the only uses outside the component and its test) | `nowrap`, `Clip` |
| `ui/src/components/index.ts:16` | The `ShrinkText` export | |
| `ui/src/components/components.test.tsx` | The test `Table marks shrink columns and dense tables` (line 62) and the `ShrinkText` import (line 5) | The three `Modal` tests |
| `ui/src/pages/setups/setups.test.tsx` | The `UI-9 label chips on Setups` block (lines 379-565); the `UI-19 exact-match chips` block (973 to the end); `UI-20 label columns give way …` (697); the `FIVE_KEYS` fixtures; the label-column parts of `SET-6 populated …` (141) and `UI-15 …` (629) | UI-16, UI-17, UI-18, UI-12, UI-1, HOLD and the other tests, with the header list corrected |
| `ui/src/pages/admin-settings/index.tsx:410-411` | The "Display label keys" field | See section 3 |

Not removed: `Modal.fallbackFocus` and the Shift+Tab fix, `formatRate`, `weight.tsx` additions, `SetupPanel.tsx`, and everything task 05 did for `pricing`.

One fix to fold in, since the file is open: verification minor 1, `overflow-wrap: anywhere` on `.costNote`, so a one-word reason does not run over the actions column (`tasks/10-verify/RESULT.md`, minor 1).

## 3. Removing the display-label-keys setting (R3-1)

Every reader and writer in the repository, and what each does afterwards. The API and the store are not changed: nothing is removed or renamed.

| Where | What it does today | After |
| --- | --- | --- |
| `internal/settings/settings.go:25,61,85,113-124,259-261,312-318` | Defines, defaults, validates, stores and loads the setting | Unchanged |
| `internal/api/handlers_admin.go:858-860,876-880` | `PUT /api/v1/admin/settings` requires the key in the body | Unchanged. An API client that sends it is unaffected |
| `internal/api/handlers_user.go:827-830,883` | `GET /api/v1/setups` returns `display_label_keys` | Unchanged. This is now the only reader of the stored value, and no page reads the response field |
| `api/openapi.yaml:417,2053,2069,2370,2373` | Documents both fields as required | Fields and `required` lists unchanged. Description text only (section 5) |
| `ui/src/pages/setups/index.tsx:74` | Reads the keys for the columns | Line removed |
| `ui/src/pages/admin-settings/index.tsx:410-411` | The input | Removed |
| `ui/src/pages/admin-settings/form.ts:12,46,93-96` | Turns the array into text and back, sends it in the PUT | The form keeps the loaded array as it is (`displayLabelKeys: string[]`) and sends that array back. No text conversion, no editing. Needed because of the required key; it also means a new UI works against an older API |
| `ui/src/api/schema.gen.ts` | Generated types include the field | Regenerated only if the description text changes |
| Test fixtures: `ui/src/pages/setups/setups.test.tsx:74`, `ui/src/pages/admin-shared/fixtures.ts:163`, `ui/src/pages/admin-settings/admin-settings.test.tsx:12`, `ui/src/shell/Gate.test.tsx:127`, `ui/src/pages/login/login.test.tsx:75`, `ui/src/pages/change-password/change-password.test.tsx:52` | Carry the field | Unchanged; the types still require it |
| Go tests: `internal/settings/settings_test.go:39,64-66,147,189-191`; `internal/api/user_test.go:370,380-382`; `admin_system_test.go:123,159,209,237-238`; `contract_test.go:428`; `permissions_test.go:273` | Pin validation, storage and both API fields | Unchanged, and they must stay green: they are now the proof that the API kept the field |
| Upgrade fixtures: `testdata/upgrade-d96a950/*`, `testdata/upgrade-v1.1.0/*` (the v1.1.0 database stores `["tier","zone"]`, `testdata/upgrade-v1.1.0/README.md:6`) | Recorded responses hold the field | Unchanged; the comparisons pass because the API is the same |
| `e2e/browser/tests/acceptance.spec.ts:321,337` (A0) | Fills the field with `size` and reads it back after a reload | Both lines removed, or A0 fails on a missing label |
| `e2e/browser/tests/acceptance.spec.ts:481` (A3) | Reads `large` from the table row | Reads the label from the details panel instead |
| `e2e/browser/tests/integration.spec.ts:1018-1021,1063` (U20) | Sets five keys on Settings and checks the response field | Removed with the rest of the old U20 (section 4) |
| `deploy/seed.sample.yaml:7` | A comment listing "display label keys: size" as a setting to make by hand | Line removed |
| `README.md:31` | Quick start step 5: set the display label keys to `size` | Step removed; later steps renumbered |
| `deploy/README.md:46` | "One setting … is not seedable; set it in Admin > Settings" | Sentence removed |
| `docs/deploying.md:185,539` | The settings table row; "One setting is not seedable …" | Row kept and reworded (section 5); the sentence at 539 removed |
| `docs/decisions.md:41` (D33) | Records that the key is set by hand on first run | Left as history; D97 says it no longer applies |
| `docs/SPEC.md:229,270,307,335,390,457` | SET-6, ADM-2, the Settings row, the data model, the route table, the reference scenario | Not edited; D98 lists the edits for the spec's source |
| `docs/wireframes/project/AdminSettings.dc.html:63`, `Setups.dc.html:35,43` | Draw the field and the size column | Not edited; `docs/ui-notes.md` says they are behind |
| `.orchestrator/reported-weights/demo/setup.sh:20` (another job's demo) | Sets `["size","team"]` through the API | Still accepted; has no visible effect. Not touched |
| Config file, CLI | No key and no command (the setting lives in the database only; nothing in `cmd/` or `deploy/config.sample.yaml` names it) | Nothing to do |

What this breaks or changes in other areas:

- **Nothing breaks functionally.** My usage's Labels column on holds, Setup inventory's Labels column, rate rules and their match counts, and the integration state route do not read the setting.
- **A stored value becomes invisible and can be changed only through the API.** An installation that set five keys keeps them in the database and in `GET /api/v1/setups`.
- **The audit log** still records a change of the key made through the API (`handlers_admin.go:902-911`); the Settings page can no longer cause one.
- **An older UI bundle with the new API** (split mode, UI not yet upgraded) still shows the columns and the field and works.
- **What users lose:** no way to compare setups by a label at a glance or to scan one label down the table. That is the owner's decision (R3-3); the panel and the selector are the two ways left.
- **Sharp edge kept on purpose:** if the server were changed to make the key optional, a new UI that dropped the key would fail against an older API with "display_label_keys is required". Hence the round trip and no server change.

## 4. Requirements

### Amended text

- **UI-15** The Setups table's columns are Setup, State, Held by, Cost and actions. It has no Weight column, no Holder's tokens column and no label column.
- **UI-19** The page has no per-label dropdown and no label chips. Labels are filtered with the selector box (UI-21) and with exact-match tokens (UI-22). The State filter works as before.
- **UI-20** At 1280 px the Setups table does not scroll horizontally. (Measured with the product's CSS, `proto/measure-r3.cjs`: it fits for a developer and for an admin with a disabled setup, with a setup name of any length, since the name is cut at 32 ch; the table needs 1004 px with a 32-character name. It scrolls at 1024 px with such a name.)
- **UI-9** (in `docs/design/ledger-labels-onboarding.md`) is replaced by UI-21 and UI-22: "The Setups page offers every key and value of `label_index` as suggestions in the Label selector box; the placeholder is taken from the index." SET-8 (`label_index`) is unchanged.
- **SET-6** is retired: "`display_label_keys` is stored, validated and returned by the API for compatibility. No page reads it and the Settings page does not edit it."
- **UI-18** is unchanged and now carries R3-3 alone: every label of a setup is in the panel.

### New

- **UI-21** The Label selector box is a combobox that suggests, for the caret's position, label keys, operators, or the values of the key being written, all taken from `label_index`. Picking a suggestion replaces the token under the caret; picking a value after `=`, `==` or `!=` applies the filter. It is operable with the keyboard and by touch, and names itself to assistive technology. A selector typed by hand is sent as typed and an invalid one shows the API's message, as before.
- **UI-22** A label value that cannot be written in a selector is suggested after `key=` and, when picked, becomes an exact-match token beside the box. The token filters the fetched list in the browser, combines with the selector and the State filter, holds one value per key, never reaches the API, and is removed with its own button.
- **UI-23** The Settings page has no display label keys field. Saving settings sends the stored `display_label_keys` back unchanged.

### Unit tests

`ui/src/pages/setups/selectorSuggest.test.ts` (new; pure functions, so most cases live here):

| Id | Test name |
| --- | --- |
| UI-21 | `UI-21 suggests keys at the start of a requirement, after a comma and after !` |
| UI-21 | `UI-21 suggests the values of the key after =, == and !=, and nothing for a key that is not in the index` |
| UI-21 | `UI-21 suggests operators after a key and a space` |
| UI-21 | `UI-21 inside an in or notin list suggests values that have a term, are not empty and are not in the list yet` |
| UI-21 | `UI-21 a comma inside parentheses does not start a new requirement` |
| UI-21 | `UI-21 matches ignore case; prefix matches come before contains matches, each in the index's order` |
| UI-21 | `UI-21 renders at most 50 options and reports how many more match` |
| UI-21 | `UI-21 a pick replaces the whole token under the caret, also from the middle, and puts the caret after it` |
| UI-21 | `UI-21 a key that is a prefix of another key is completed only when picked` |
| UI-21 | `UI-21 the empty value is offered as (empty) and leaves key=` |
| UI-22 | `UI-22 a value with no term is offered only after = and ==, and its pick removes the requirement from the text` |
| UI-9 | `UI-9 the placeholder is the first non-null term of the index` (moved from `setups.test.tsx:473`) |

`ui/src/pages/setups/setups.test.tsx`:

| Id | Test name |
| --- | --- |
| UI-15 | `UI-15 the table's columns are Setup, State, Held by, Cost and actions, whatever display_label_keys holds` (replaces line 629; asserts the header list with `display_label_keys` of one key and of five) |
| UI-19 | `UI-19 the page has no per-label dropdown and no label chips; the State filter and the selector box work` (replaces line 190; asserts no "Available labels" heading, no button named "Filter by …", and that State is the only `select` element. The selector box now also has the role combobox, so the old "one combobox" assertion is wrong) |
| UI-20 | `UI-20 every column of the Setups table is one line` (mechanism: the five columns have the `nowrap` class and there is no label header) |
| UI-21 | `UI-21 the selector box is a combobox: typing opens the list, Arrow Down and Enter pick, and a picked value filters through the API` |
| UI-21 | `UI-21 Enter with no active option submits the typed text; Escape closes the list and keeps the text; Tab picks nothing` |
| UI-21 | `UI-21 the list opens on click and on Arrow Down in an empty box, and not on focus alone` |
| UI-21 | `UI-21 aria-activedescendant follows the active option and the status line announces the count` |
| UI-21 | `UI-21 with an empty index or no label_index the box is a plain text box with no placeholder` |
| UI-21 | `UI-21 keys pressed during composition are left to the input method` |
| UI-22 | `UI-22 picking a value with no selector term adds an exact-match token, filters the rows in the browser and sends nothing to the API` |
| UI-22 | `UI-22 a token combines with the selector and the State filter; another value of the same key replaces it; the same value again changes nothing` |
| UI-22 | `UI-22 removing a token restores the rows and returns focus to the selector box` |
| UI-22 | `UI-22 a token whose value is no longer in label_index stays shown and removable` |
| SET-6 (retired) | `SET-6 populated …` (line 141) is renamed `the populated table shows holder, overdrawn word and costs, and no label column` |
| RULE-2 | `RULE-2 the label selector goes to the API as selector= and a parse error is shown` (line 212): unedited |

`ui/src/pages/admin-settings/admin-settings.test.tsx`:

| Id | Test name |
| --- | --- |
| UI-23 | `UI-23 Settings has no display label keys field and saves the stored keys unchanged` (loads `["size","team"]`, edits another field, asserts the PUT body holds `display_label_keys: ["size","team"]` and that no control is labelled "Display label keys") |

Go: no new test. `TestSET6_SetupsListForDeveloper` (`internal/api/user_test.go:370-382`) and the settings tests stay and pin the kept API.

### End-to-end

| Step | File | Change |
| --- | --- | --- |
| A0 | `e2e/browser/tests/acceptance.spec.ts:321,337` | Remove the two "Display label keys" lines |
| A2f | `acceptance.spec.ts:398-434` | Rewritten for UI-21, same assertions on requests and rows: type `si`, pick `size` from the list, the box reads `size=`, pick `large`, the page asks the API with `size=large` and lists lab-03 and lab-04; put the caret in the value, pick `medium`, the request is `size=medium`; clear the box and press Enter, six rows. Also: no "Available labels" heading. The placeholder assertion (line 406) stays |
| A3 | `acceptance.spec.ts:481` | `row toContainText("large")` becomes: open the panel with the "Price details for lab-03" button, the `size` label reads `large`, close |
| W2 | `e2e/browser/tests/integration.spec.ts:736-760` | Unchanged |
| U20 | `integration.spec.ts:1013-1100` | Restated for UI-15 and UI-20: no Settings step. The admin adds a setup with a 32-character name, pins one weight and disables one setup, alice holds one. At 1280 × 900, as alice and as the admin: exactly the five headers in order; no sideways scroll of the table; cost button and holder line one line box high; the panel checks as today, which include every label (R3-3) |
| U22 | `integration.spec.ts`, after U20 | New, for UI-22: in Setup inventory the admin gives one setup a label whose value holds a space and parentheses. Alice types the key and `=`, picks the value marked "exact match": the token shows, one row is listed, the box holds no term for that key, and no request carried the value. With State set to Free the row count follows. Removing the token brings the rows back |
| S1, S2 | `integration.spec.ts:1104-1160` | Logic unchanged. Screenshots retaken without label columns. Panel shots are taken of the viewport, not the full page (verification minor 3) |
| | `e2e/README.md:13,51` | A2f and U20 bullets rewritten; U22 added |

## 5. Docs

Sentences written in task 07 (and two from task 05) that are now wrong, and what replaces them.

| File and line | Now says | Change |
| --- | --- | --- |
| `docs/ui-notes.md:63` | The section describes D93, UI-15 to UI-20 | Add D96, UI-21 to UI-23 |
| `docs/ui-notes.md:68` | No select per display key; labels are filtered with the selector box and the chips | No select and no chips; the selector box suggests keys and values |
| `docs/ui-notes.md:69` | The selector input, 260 px | 360 px; the combobox behaviour of section 1 in a short list |
| `docs/ui-notes.md:70` | The "Available labels" block | Removed; replaced by the suggestion list and the exact-match token |
| `docs/ui-notes.md:71` | Columns include one per display label key | Setup, State, Held by, Cost (tokens / h), actions; the example rows lose the `size` cells |
| `docs/ui-notes.md:83` | Label columns give way (UI-20, D94(h)) | Removed. One line: every column is one line; the table fits at 1280 px |
| `docs/ui-notes.md:92` | Note 5: the size column appears because an admin listed it | Removed; "Labels are shown in the details panel only" |
| `docs/ui-notes.md:178` | The "Display label keys" input and its hints | "The wireframe's Display label keys input is not built (D96)" |
| `docs/ui-notes.md:240` (item 13) | Mentions the "Available labels" chips (UI-9) | Add "replaced by the selector box's suggestions, item 15" |
| `docs/ui-notes.md:249` (item 15) | The D93 summary with label columns and chips | Rewritten for the final page; says `Setups.dc.html` and `AdminSettings.dc.html` are behind |
| `docs/decisions.md` D93 to D95 (lines 101-103) | The first decisions | **Left as written.** They record what was decided that morning; the table's habit is a later entry that supersedes (as D86 does for D84(g)). Add D96, D97, D98 below |
| `docs/design/setups-declutter.md` | The request, with label columns and chips | Not rewritten. A new last section "Amendment, 2026-11-24" with the owner's words, R3-1 to R3-3, the amended UI-15, UI-19, UI-20, the retired SET-6, the new UI-21 to UI-23, and one line each for what it overrides: "The table" label-columns bullet, "Filters", the Settings help-text sentence (line 68), UI-20's wording, "SET-6 is still met". The status line points to it |
| `docs/design/reported-weights.md:399,431` | UI-12 as amended in task 07 | Check only: the tag on the cost cell and the panel are unchanged. No edit expected |
| `docs/integrating.md:471` | Attributes in the details panel and the acquire dialog | Still true. No edit |
| `README.md:7` | "The Setups page lists every label as a clickable chip that fills the selector, or, for a value the selector cannot express, filters by exact match in the browser." | "On the Setups page the Label selector box suggests label keys and values as you type; a value the selector cannot express is matched exactly in the browser." The next sentence stays |
| `README.md:9` | Weight and source wherever a price is explained | Still true. No edit |
| `README.md:31` | Quick start step 5 | Removed (section 3) |
| `docs/progress.md:122,124,134,138,139,141,147` | The first build's summary and requirement table | Rewritten rows for UI-15, UI-19, UI-20; SET-6 retired; new rows UI-21 to UI-23; line 124 says the token replaces the chip; line 147's note on `max-width` is removed. Line 75 (history of an earlier job) stays |
| `docs/design/ledger-labels-onboarding.md:204,372` | The inert chip; UI-9 as amended in task 07 | A short "Amended by D96" note at both places: UI-9 is replaced by UI-21 and UI-22 of `docs/design/setups-declutter.md`. Lines 197-206 (the chip design) get the same one-line note, not a rewrite |
| `docs/deploying.md:185` | "Label keys shown as columns … the label chips filter by every key" | "Stored and returned by the API for compatibility. Not used by any page since v1.3.0 and not editable on the Settings page" |
| `docs/deploying.md:539` | One setting is not seedable | Sentence removed |
| `docs/deploying.md`, "Upgrading to v1.3.0" (added in task 05, about lines 635-640) | `pricing`; the label columns | Add: the Setups page no longer shows label columns or the "Available labels" block; the display label keys field is gone from Settings; a stored value is kept and still returned by `GET /api/v1/setups` and the settings routes, and can be changed only through the API; a script that sets it keeps working |
| `deploy/README.md:46`, `deploy/seed.sample.yaml:7` | The hand-set setting | Removed (section 3) |
| `e2e/README.md:13,51` | A2f with chips; U20 with five display keys | Section 4 |
| `api/openapi.yaml:417-423` (description changed in task 05) | "Also returns `display_label_keys` (SET-6) … the state filter, and the exact match of a label value that has no selector term, are applied client-side" | "Also returns `display_label_keys`, kept for compatibility; the Setups page does not read it. … The state filter and exact-match label tokens are applied client-side on this list." |
| `api/openapi.yaml:2069,2373` | The two properties | A `description` on each: "Kept for compatibility; no page reads it since v1.3.0." No `deprecated` flag and no change to `required` |
| `ui/src/api/schema.gen.ts` | | Regenerated after the description edits (`ui/BUILD.bazel:115`) |

Drafts for `docs/decisions.md`:

| # | Date | Decision | Why | Who |
| --- | --- | --- | --- | --- |
| D96 | 2026-11-24 | **Owner-directed amendment to D93, after the first screenshots.** In the owner's words: "In the 'Setups' page, lets remove the label columns. Its looks very much cluttered" and "Lets get rid of the Available labels as well. We should instead just let users start typing in the 'Label Selector' checkbox and we could autofill values." Decided: (a) the Setups table has no label columns; its columns are Setup, State, Held by, Cost and actions; (b) labels are shown in the details panel only; (c) the "Available labels" chips are removed and the Label selector box suggests label keys and values as the user types; (d) a value the selector cannot express is still suggested and, when picked, is matched exactly in the browser and shown as a removable token beside the box; (e) the "Display label keys" field is removed from the Settings page; the API field and the stored value stay and nothing reads them. This supersedes D93 (h) as far as the chips, D93 (i) and (j), and D94 (h), (i) and (j). UI-15, UI-19 and UI-20 are amended, SET-6 is retired, UI-9 is replaced by the new UI-21 and UI-22, and UI-23 is new; the amendment section of `docs/design/setups-declutter.md` has the wording. | With five label columns the table showed six characters of each label and still looked cluttered; the selector box is where a developer already filters. | owner |
| D97 | 2026-11-24 | Details of D96 that the owner left open. **Suggestions.** (a) They come from `label_index` of the setups response; no API change. (b) What is suggested follows the caret: keys at the start of a requirement and after `!`; the operators `=`, `!=`, `in (`, `notin (` after a key and a space; the key's values after `=`, `==`, `!=` and inside a list. The page does not parse or validate a selector; the API stays the only judge. (c) A pick replaces the whole token under the caret. Picking a key adds `=`. Picking a value after `=`, `==` or `!=` applies the filter at once, as a chip did; other picks do not. (d) Matching ignores case; prefix matches first, then contains, in the API's order. At most 50 options are shown, with a line saying how many more match. (e) The list opens on typing, on a click in the box and on Arrow Down, so the labels can be discovered without typing; it does not open on focus alone. No option is active until the user moves into the list, so Enter on typed text submits it as before; Escape closes the list and never clears the text; Tab picks nothing. Keys pressed during input-method composition are left alone. **Exact-match token.** (f) Offered only after `key=` and `key==`. Picking it takes the requirement out of the box and adds the token; one value per key; picking the same value again changes nothing. (g) A token whose value no longer exists stays shown and in force until removed; it matches nothing. A token and a contradicting selector match nothing; the page does not detect it. (h) Tokens and the State filter are applied in the browser, the selector by the API. All filters are lost on reload, as before. **Setting.** (i) `PUT /api/v1/admin/settings` still requires `display_label_keys`, so the Settings page keeps the array it loaded and sends it back unchanged. The server was not changed to make the key optional: a newer UI would then fail against an older API. (j) The setting is still validated, stored, audited and returned by `GET /api/v1/setups`; a value can be changed only through the API. D33's instruction to set it by hand on first run no longer applies; the two acceptance steps that used it were changed. **Code.** (k) The `shrink` and `dense` options of the shared table and the `ShrinkText` helper, added for the label columns, are removed; nothing else used them. (l) UI-20 now reads "no horizontal scroll at 1280 px"; measured in Chromium, the five columns need 1004 px with a 32-character setup name. It is still checked in a real browser only by the integration scenario. (m) The wireframes `Setups.dc.html` and `AdminSettings.dc.html` were not redrawn. | The simplest options that meet D96 and keep free typing of a selector exactly as it was. | orchestrator |
| D98 | 2026-11-24 | **Edits the source of `docs/SPEC.md` needs for D93 and D96. Replaces D95 in full.** `docs/SPEC.md` was not edited. (1) "Dashboard pages", Setups row, Shows: "Every setup: name, state, holder and how long, cost per hour now and at the highest band; in a details panel: weight and its source, lab pressure and multiplier, the holder's balance, every label, details JSON". (2) The same row, Actions: "Acquire, release own; filter by state and by label (a selector box that suggests label keys and values); open the details panel". (3) SET-6: "`display_label_keys` is stored and returned by the API for compatibility; no page reads it." (4) ADM-2 and the Settings row of "Dashboard pages": remove "display label keys" from what an admin edits on the page. (5) The route table, `GET /api/v1/setups`: "also returns the display label keys (kept for compatibility)". (6) The reference scenario table: remove the row "Display label keys: size". (7) The data model row for `settings` may keep its mention; the key is still stored. (8) New requirements UI-15 to UI-23 as worded in `docs/design/setups-declutter.md` and its amendment, UI-12 as amended there, and UI-9 replaced by UI-21 and UI-22. | The spec is generated from a source outside this repository and is never edited here; the owner applies these. | orchestrator |

## 6. Work breakdown

| Task | Writes | After | Parallel with |
| --- | --- | --- | --- |
| C1 Setups page and shared clean-up | `ui/src/pages/setups/*` (new `selectorSuggest.ts`, `selectorSuggest.test.ts`, `SelectorBox.tsx`; `LabelChips.tsx` and `labelTerms.ts` deleted); `ui/src/components/Table.tsx`, `Table.module.css`, `index.ts`, `components.test.tsx` | none | C2, C3 |
| C2 Settings page | `ui/src/pages/admin-settings/index.tsx`, `form.ts`, `admin-settings.test.tsx` | none | C1, C3 |
| C3 Docs and API descriptions | `api/openapi.yaml` (descriptions only), `ui/src/api/schema.gen.ts` (regenerated), `docs/ui-notes.md`, `docs/decisions.md`, `docs/design/setups-declutter.md`, `docs/design/ledger-labels-onboarding.md`, `docs/progress.md`, `docs/deploying.md`, `README.md`, `deploy/README.md`, `deploy/seed.sample.yaml` | none; wording checked against the page after C1 | C1, C2 |
| C4 End-to-end and screenshots | `e2e/browser/tests/acceptance.spec.ts`, `e2e/browser/tests/integration.spec.ts`, `e2e/README.md` | C1, C2 | none |
| V2 Verify | its own task folder | C1 to C4 | none |

C1 does the page first and the shared clean-up last inside the one task, so the tree type-checks at each step (the page imports `ShrinkText` until its columns are gone). `./ci.sh` runs after C1, C2 and C3 are all in, and after C4. `e2e/run.sh both` and `e2e/run.sh integration` run in C4: this time the acceptance spec changes, so both stacks must be run again.

What the verifier should try to break, new parts:

1. **Caret in the middle.** `si|te=sjc`, `site=s|jc,tier=a`, `site in (a|ms, sjc)`, a caret between two spaces, a caret right after a comma inside parentheses. The right list shows, and a pick replaces one token and nothing else.
2. **Pasting.** Paste a whole valid selector, an invalid one, text with newlines and tabs, a value with spaces. Nothing is applied by the paste; the list matches the caret; Enter submits exactly what was pasted.
3. **Composition.** With an input method (or synthetic composition events), Enter and the arrows during composition neither pick nor submit; the list updates after composition ends.
4. **500 keys, one key with 2,000 values.** Typing stays responsive; at most 50 options are in the DOM; the "N more" line is right; the list scrolls and keeps the active option in view.
5. **A key that is a prefix of another** (`site`, `site-code`, `sitex`): typing `site` lists all three; `site=` gives only `site`'s values; picking `site` does not turn into `site-code`.
6. **Values with spaces, commas, parentheses, `=`, `!`, quotes, angle brackets, emoji, 63 characters, and the empty value.** Those with no term appear only after `=` and `==`, as exact matches; nothing is ever inserted into the box that the API then refuses; text is rendered as text, never as markup.
7. **Duplicate token.** The same value twice: one token. Another value of the same key: replaced. Two keys: two tokens, both in force.
8. **Token plus contradicting selector** (`owner!=…`, `!owner`, another `owner=` value): empty table with the "No setups match the filters." message, token still removable, no error.
9. **Escape.** With the list open it closes the list only; the text and the tokens stay. With the list closed it does nothing. Inside nothing here does Escape clear the box. With the details panel open, Escape closes the panel as before.
10. **Focus after picking.** Focus stays in the box after every pick, with the caret after the inserted text; after removing a token focus is in the box; Tab from the box goes to the Filter button and picks nothing.
11. **Screen reader.** `aria-expanded`, `aria-controls`, `aria-activedescendant` change as stated; the status line announces counts and token changes; an exact option says "exact match"; the token's button has its full name. Listen with a real reader if one is at hand.
12. **Old API.** A setups response with no `label_index` (and one with no `pricing`): no throw, no list, plain text box, no placeholder. A settings PUT against an API that requires `display_label_keys` succeeds.
13. **Free typing is unchanged.** Every selector form of section 1 typed by hand and submitted with Enter and with the Filter button reaches the API byte for byte (after the trim the page already does); an invalid one shows the API's message under the box and marks the box invalid.
14. **Applying on pick.** Picking a value while another requirement is unfinished sends the box as it is and shows the API's error; nothing is lost from the box.
15. **Touch and pointer.** Tap on an option picks it without the list closing first; scrolling the list by touch does not pick; a click outside closes the list.
16. **Settings.** The stored keys survive a save of any other field, including when they are empty and when there are 32 of them; no audit entry for the key appears after such a save; no control is labelled "Display label keys".
17. **Dead code.** No reference to `shrink`, `ShrinkText`, `dense`, `LabelChips`, `applyChip`, `chip` classes or `display_label_keys` remains in `ui/src` outside fixtures, `form.ts` and the generated schema.
18. **Layout.** At 1280 px, developer and admin, a 32-character name, a 60-character name, a long reason of one word, a pending release: no sideways scroll, no overlap. The suggestion list is not cut by the table below it and sits under the open dialog's backdrop, not over it.
19. **Kept parts still hold.** The first verification's matrix for UI-16, UI-17, UI-18, UI-12 and WGT-14 (`tasks/10-verify/RESULT.md`).

## 7. What I checked as my own reviewer

| Question | Finding |
| --- | --- |
| Can the Settings page stop sending the key? | No. The PUT requires it (`handlers_admin.go:858-880`). Designed as a round trip; test UI-23 |
| Does anything besides the Setups page read the setting? | No page. The setups handler passes it through (`handlers_user.go:827,883`). Searched Go, UI, deploy, e2e, docs and the other jobs' folders (section 3) |
| Do the Go and compatibility tests need edits? | No, because the API is untouched. If one fails, the build removed something it should not have |
| Does any end-to-end step depend on what goes? | Yes: A0 (two lines), A2f (chips), A3 (label in the row), U20 (Settings field, ten headers). W2, S1, S2 read text that stays. The integration scenario's other steps do not touch labels on Setups |
| Does the chips' discovery purpose survive? | Only if the list opens without typing. Hence click and Arrow Down on an empty box, and a placeholder |
| Does free typing stay byte for byte? | Yes by construction: no option is active until the user moves into the list, the form and request code are not edited, and RULE-2's test must pass unedited |
| Can a pick produce a selector the API refuses? | Not by its own insertion: keys come from `label_index`, values are inserted only when the API gave them a term, an empty value is never put in a list. A pick can apply a box that the user left invalid elsewhere; the error shows |
| A key in `label_index` that is not a valid selector key? | Label keys are validated when stored (`labels.go:29-66`), so this appears not to occur; `LabelTerm` would give every value of such a key `selector: null` (`selector.go:267`), and those values would then be offered as exact matches after a key the API refuses. Edge case for the verifier: the key would also be offered as a key. Not handled |
| Does the exact token still have a control when its value vanishes? | Yes, the token itself; the first build's silent-drop rule is removed |
| Is the role change of the box a break for tests? | `getByLabelText("Label selector")` and Playwright's `getByLabel` still find it. The unit test that counted comboboxes is restated (UI-19). No end-to-end step uses `getByRole("textbox")` on it (searched) |
| Is removing `shrink` and `dense` safe? | Yes: Setups is the only user (searched `ui/src`). The table then has default padding again; measured to fit at 1280 px |
| Does the table still need `nowrap` and the 12 em cost note? | Kept. Without percentage columns nothing squeezes, but they cost nothing and keep the cells one line on narrow windows |
| Split-mode upgrades | New UI with old API: no `label_index` on a very old API gives a plain box; Settings save works. Old UI with new API: unchanged API, works |
| Does `pricing` or the panel change? | No |
| Is the generic rule kept? | The suggestion code names no key; fixtures use neutral keys; U22's label is a neutral key with a value holding a space and parentheses |
| UI-1 | The suggestion code counts options and compares strings; it touches no API number |
| What I did not check | No browser run of the combobox (it is not built). Screen-reader behaviour. Safari and Firefox. Whether a pick breaks the browser's undo (stated as inference). How long U22 adds to the integration run (one Setup inventory edit and no poll; appears small) |

## Open questions for the owner

| # | Question | Default built until told otherwise |
| --- | --- | --- |
| R3-Q1 | Should picking a value apply the filter at once (as a chip did), or only fill the box so the user presses Enter? | Apply at once |
| R3-Q2 | Should the list open when the user clicks into the empty box, to show which labels exist? Without it there is no way to discover labels except typing a first letter. | Yes, on click and Arrow Down; not on focus by Tab |
| R3-Q3 | An installation that already stored display label keys keeps them, invisibly. Leave them, or clear them in a migration? | Leave them (R3-1 says the stored value stays) |
