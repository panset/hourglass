# Result: 14 (C2) Admin Settings page without the display-label-keys control

Status: done.

## What I did
- `ui/src/pages/admin-settings/form.ts:12-13,47,94`: the form carries `displayLabelKeys: string[]` straight from the loaded settings and puts the same array in the PUT body. No join/split, so empty, any order and 32 keys round-trip unchanged; since it is never edited, `formDirty` and the audit diff never see it change.
- `ui/src/pages/admin-settings/index.tsx:406-409`: the "Display label keys" Field (label, hint "Comma separated; shown as columns on Setups…", error slot) is removed. Default weight stays alone in the `.row` wrapper (one flex child, so no gap; card's normal spacing above the rules table). No CSS change.
- Server field error for `display_label_keys` (the open point): the page's general save alert at `index.tsx:292` renders `errorText(save.error)`, i.e. message plus "(display_label_keys)", for any field. It was already shown there, not dropped, so no code change was needed; a test pins it.

## Tests (`ui/src/pages/admin-settings/admin-settings.test.tsx:179-254`)
- `UI-23 Settings has no display label keys field and saves the stored keys unchanged` (DESIGN-R3 name; loads ["size","team"], edits Default weight, asserts body and no control or text matching /display label|columns on Setups/).
- `UI-23 sends %s back byte-identical when another field is saved` x3: no keys, some keys, 32 keys; asserts the raw request text contains `"display_label_keys":<JSON of stored>`.
- `UI-23 every other setting saves as before, with the stored keys alongside`: default weight, allowance, max holds, poll, base multiplier, a band multiplier, enforcement.
- `UI-23 a server error for display_label_keys, which has no field now, shows in the page's save error`: alert role holds the message, no "Settings saved", edits stay unsaved.

## Commands
- `npx vitest run src/pages/admin-settings`: exit 0, 30 tests passed (24 before + 6 new). Log: vitest.log.
- `npx tsc --noEmit -p tsconfig.json` (whole UI): exit 0. Log: tsc.log.
- `tools/check_spdx.sh`, `tools/check_core_generic.sh`: exit 0. The repo has no separate JS linter; tsc is the UI lint in ci.sh/Bazel.

## UX-SPEC change list (mine)
- 28 (Rate rules card: Default weight alone, no gap, no "Display label keys" text, saving another field keeps stored keys): done.
- Section 7 point on a `display_label_keys` server error: done (shown in the general save error; tested).

## Deviations
- None. DESIGN-R3's UI-23 test name kept as written; extra cases added under the UI-23 prefix.

## Confidence and gaps
- High for behaviour in jsdom. Visual "no gap" was not checked in a browser (inference from the flex `.row` with one child); C4 screenshots will show it.
- The general alert shows the raw key `(display_label_keys)` after the message, the page's existing convention for every field error; not reworded.
