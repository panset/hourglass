# Result 10: verification of the Setups declutter build

Date 2026-11-24. Branch `setups-declutter`, working tree against `main` at `f1a05c2`. Nothing in product code was changed; every deliberate break was restored (see "Tree state").

## Scope as verified

The orchestrator reduced the scope while this task ran: the owner has decided that the label columns and the "Available labels" chips will be replaced by autocomplete in the selector box. From that point I skipped label columns, label clipping and UI-20 layout, the chips, the B2 exact-match filter and the SET-6 header assertions (break-list items 5 label parts and 9). Work on those parts done before the message is kept in "Out of reduced scope" at the end, for whoever redoes them; it is not part of the verdict.

## Verdict

**Kept scope (cost cell, Held by cell, details panel, `pricing`, UI-1, generic, unchanged surfaces): ready. 0 blockers, 0 should fix, 6 minor.**

**The branch as a whole is not for merge as it stands**, for a reason outside what I verified: it still contains the label columns, the chips and the B2 filter, and the documents (D93(h) to (j), D94(h) to (j), D95, `docs/ui-notes.md`, `README.md:7`, `docs/progress.md`, `docs/deploying.md:185`, the `listSetups` description in `api/openapi.yaml`) describe them as owner decisions. Those need the redo and a second verification. That is the orchestrator's call, not a finding against the kept code.

## Findings

No blocker. No should-fix.

### Minor

1. **A not-charging reason with no space in it runs out of the cost cell.** `ui/src/pages/setups/setups.module.css` `.costNote` has `white-space: normal` and a 12em width but no `overflow-wrap`. Measured in Chromium at 1280 px with a 72-character reason of one word: the text sticks out of the cell by 226 px, over the actions column (`browser/layout.json`, case `long-reason-one-word`; screenshot `browser/shots/layout-long-reason-one-word.png`). Scenario: a metering policy returns a reason holding a long identifier or URL. A reason of ordinary words wraps correctly (row 150 px tall). Fix: `overflow-wrap: anywhere` on `.costNote`.
2. **At narrow widths the whole page scrolls sideways, not only the table.** `.sr-only` is `position: absolute` and `.scroller` (`ui/src/components/Table.module.css:2`) is not a positioned box, so hidden text inside the table is not clipped by the scroller. Measured at 768 and 375 px: page `scrollWidth` 833. The header's hidden "Actions" text causes this on `main` too; the new hidden texts in each cost button and tag are a second source (805 with only the header text hidden, 768 with all hidden; `browser/narrow.cjs`). Scenario: on a phone the top bar and filters slide sideways with the table. Fix: `position: relative` on `.scroller`.
3. **The owner's two panel screenshots look broken, though the page is not.** `tasks/09-b5-e2e/screenshots/S1-panel-0x.png` and `S2-panel-2x.png` are full-page shots: the dimmed backdrop ends at 900 px with undimmed rows below, and the dialog is cut inside the JSON block. Content is right (see "Screenshots"). Fix: take the panel shots with the viewport, not `fullPage`. They will be retaken after the label redo anyway.
4. **A weight below 0.0005 prints as `0` beside a cost above zero.** Panel line "Weight" uses `formatTokens` (3 decimals): weight 0.0004 with cost 0.001 reads `0` and `0.001 / h` (`browser/behave.json`, "8 tiny weight"). D94(a) records this as the reason for having no equals sign; the reader still sees a zero weight with a price. Fix, if wanted: more decimals for the panel's weight only.
5. **UI-18 "when charging starts" never shows for a free setup.** `internal/api/handlers_user.go:888` always sends `metering_description: null`, so the panel has no Charging line until a setup is held (checked in the browser and by the unit test at `setups.test.tsx:838`). Recorded as D94(e) and accepted in `DECISIONS.md`; listed so the owner sees the requirement is met only as far as the API has text.
6. **Focus return after a mouse click was checked in Chromium only.** `Modal.tsx:43` takes `document.activeElement` as the opener. Safari does not focus a button on click, so there the opener would be `body`, which is connected, and the fallback heading is not used (inference from the code; not run, only Chromium is installed). Same behaviour as every existing dialog on `main`. Fix, if wanted: treat `body` as "no opener".

## Requirements matrix (kept scope)

"Red" means I broke the product code, ran the tests, saw the named test fail, and restored the file (26 breaks, `mut/results.txt`).

| Item | Tests named after it | Can it fail? | Verdict |
| --- | --- | --- | --- |
| UI-15 no Weight, no Holder's tokens column | `UI-15 the table has no Weight …` (`setups.test.tsx:629`) | Red when a Weight column is added (M01) | Met for those two columns. The label-column part of the header list is out of scope |
| UI-16 cost cell | two `UI-16 …` tests (`:646`, `:666`); Go subtest "after the top band is lowered"; e2e S1, S2 | Red for: always a range (M02), no tag for `override` (M03), unit dropped (M04), hidden tag sentence gone (M22), button name changed (M21) | Met. One value when `now` is not below `at_peak`; browser: no row reads "now X · up to less" |
| UI-17 Held by cell | `UI-17 the Held by cell …` (`:681`) | Red for: overdrawn word gone (M05), duration gone (M06) | Met |
| UI-18 panel | seven `UI-18 …` tests (`:739` to `:910`) plus `:838`, `:957`; `TestUI18_SetupsListCarriesTheMultipliersOfItsCosts` (5 subtests); e2e W2, U20, S1, S2 | Red for: Details opens nothing (M10, 11 tests), balance gone (M07), sentence tied to source (M08), one label only (M09), attributes gone (M11), pressure line gone (M13), charging gone (M14), JSON gone (M15), lower peak shown (M25); Go red for wrong multiplier (M23) and renamed key (M24, 7 tests) | Met, with minor 5 |
| UI-12 amended | `UI-12 the cost cell carries a tag …` (`:567`), `UI-12 the panel shows the weight …` (`:581`), dialog tests unchanged; `TestUI12_…` | Red (M03, M11, M12) | Met |
| WGT-14 | `UI-12 the panel shows the weight …`; `TestWGT14_EveryReaderShowsTheWeightThePollCharges`; e2e W2 reads the panel | Red when the panel prints another number as the weight (M12) | Met |
| UI-19, UI-20, SET-6 | | | Not assessed (reduced scope) |
| Constraint UI-1 | `UI-1 not-yet-charging setups …` (`:159`) | Not broken deliberately | Met by reading: no arithmetic on an API number in `index.tsx` or `SetupPanel.tsx`; two comparisons of `now` with `at_peak`; `LabelChips.tsx:52` subtracts two list lengths |
| Constraint: explanation does not contradict itself | `UI-18 the multiplier and pressure in the panel are those of the setups response`; `… keeps one response's values …`; Go test | Red (M23); builders' mutation of a mixed snapshot | Met. Browser: `/pressure` answering 5× or failing, the panel still shows its own response's 2×; after a delete under the open panel the multiplier, pressure, costs and price-change sentence all stay from the last response that held the setup |
| Constraint: additive API | `TestUI2_ContractResponsesMatchSchemas`, `TestUPG2_…`, `TestWGT20_…` (two), `TestSET6_SetupsListForDeveloper` | Red when the key is renamed (M24) | Met. Against `main`: the handler gains one map entry and one struct; the schema gains `pricing` (required, three fields); nothing removed or renamed |
| Constraint: generic | `tools/check_core_generic.sh` (in `./ci.sh`) | Not broken | Met. No customer key in changed code or assertions; `memory-gb`, `storage-gb`, `tier` are fixture values that were there before |
| Constraint: accessible | first `UI-18` test; `components.test.tsx` Modal tests | Red (M21, M22); builders broke the Shift+Tab and fallback lines | Met in Chromium, with minor 6 |
| Constraint: nothing lost | `UI-12 the panel shows …`, `HOLD-6 … the request time is in the panel` | Red (M07, M11, M12) | Met |
| Done when 1 | | | UI-15 to UI-18 and UI-12 pass (276 of 276). UI-19 and UI-20 tests pass today but are out of scope |
| Done when 2 | | | `./ci.sh` exit 0. `e2e/run.sh both`: see "Commands". Integration mode: task 09's run, not repeated by me |
| Done when 3 | | | Present; see "Screenshots" |
| Done when 4 | | | Done for the kept parts; wording matches the page. The label and chip passages will need rewriting |

## Break list (DESIGN.md section 7), kept items

Real Chromium at 1280 × 900, the UI built from this tree (`bazel-bin/ui/dist`), the API answered by a mock I control (`browser/lib.cjs`, `browser/behave.cjs`; results `browser/behave.json`: 71 checks, 70 pass, and the one "fail" is my own check comparing a name cut at 60 characters; focus was on the cost button).

- **1 Old API.** Response without `pricing`, `label_index` and `reported_weights_enabled`: page and panel render, no error; no pressure or multiplier line; `8.0 / h` and `12.0 / h` without brackets.
- **2 Consistency.** Covered above. Band change under an open panel: multiplier and both costs change together, focus stays on the Price heading. `now > at_peak`: every row one value, accessible name "16.0 tokens per hour. Price details for …", panel line reads "The bands were changed. …".
- **3, 4.** Read and searched; see matrix.
- **6 Keyboard.** Tab reaches the cost button (2 px focus ring); Enter opens with focus on the Price heading (ring shown), dialog at its top; Shift+Tab goes to Close and stays inside; Tab stays inside; Escape returns focus to the cost button; Space on Details opens the same panel; Close by keyboard returns focus to Details. The page behind cannot be clicked. PageDown, Space and End scroll a long panel from the heading. Opener gone (State = Free, the setup is acquired under the open panel, close): focus on the h1. The accessibility tree of a row holds the unit, the tag sentence and "overdrawn".
- **7 Panel under change.** Release requested: the Release line appears. Released: Holder section and Charging line go. Disabled, admin: still listed, no notice. Failed refresh: last values kept, no notice. Deleted: notice, all values from the last response that held it; close puts focus on the h1; the next panel shows its own values.
- **8 Cases.** Weight 0 (`0.0`, `0 · default (the default weight)`, `0.0 / h (0×)`); multiplier 0 at every band; `pricing.pressure` null ("not measured yet"); reason null ("Not charging yet."); unknown source (no tag, word as sent); source null; no labels; empty value "(empty)"; 32 labels with a 63-character key and a 200-character value (no sideways scroll); `details` null; a 256 KB JSON (opens in 61 ms, wraps, the dialog is the only scroller); a poll does not move the panel's scroll position; panel at 375 px stays inside the viewport.
- **10 Unchanged surfaces.** Acquire dialog in the browser: "Weight 2 (reported). Costs 4.0 / h now (2×), up to 6.0 / h at the highest band." and the price-change sentence; its code is the same as on `main` except that the sentence is now a shared constant with identical text. No file under My usage or Setup inventory changed; `usage/weight.tsx` only gained exports (`WeightTag` untouched); `formatPerHour` output is unchanged; the Settings page changed in one hint string. `Modal` changes are additive (a dialog without `fallbackFocus` behaves as before, except that Shift+Tab from a non-tabbable focused element now stays inside).
- **11 Nothing lost.** Weight, source, attributes, balance and the release-request time are in the panel for every row tried.
- **12 Selectors.** One "Details for" role selector in the e2e file, exact (`integration.spec.ts:249`).
- Cost and holder cells beyond the claimed case, 1280 px: cost above 1,000 (`12,345.678 now · up to 37,037.034`), three decimals, a 40-character holder name, a long reason, an own hold of 11 days with "overdrawn", an admin with four disabled setups: each stays on one line with nothing overlapping. Exception: minor 1.

## Docs (kept parts)

- The quoted strings of task 07's list, items 1 to 17 and 19, match the built page (read in the source and seen in the browser). Task 08's four wording differences are as it says: the middle dot in `1.15 · reported (sent by automation)`, no full stop after "Charging starts: …", "Charging." for a metering hold with no start time, the line name "Release".
- D93(a) to (g), D94(a) to (g) and (k) to (o), D95(1), (5), (7) match what is built. D95(6) quotes UI-16 as "one value when they are equal"; the page shows one value when `now` is not below `at_peak`, which D94(f) explains.
- `docs/design/ledger-labels-onboarding.md:204,372` still carry the old UI-9 text. It is about the chips, so it is left for the redo.

## Screenshots (`tasks/09-b5-e2e/screenshots/`)

| File | Shows what "Done when" 3 asks? | Anything wrong? |
| --- | --- | --- |
| `S1-setups-0x.png` | Yes: 1280 px, five label columns, tags `reported` and `override`, top bar "Lab 2% · 0×", every row `0.0 now · up to X` | No wrapping or overlap |
| `S1-panel-0x.png` | Yes: weight `0.8 · reported (sent by automation)`, attributes, 2%, 0×, `0.0 / h`, `2.4 / h (3×)`, the price-change sentence, holder and labels | Backdrop ends at 900 px and the dialog is cut in the JSON block (minor 3). "Held since 14:29 (<1 min)" beside "Charging since 14:30": the test clock runs ahead of the browser's (task 09's inference, which fits) |
| `S2-setups-2x.png` | Yes: "Lab 25% · 2×", rows `1.6 now · up to 2.4` and so on | None |
| `S2-panel-2x.png` | Yes: 25%, 2×, `1.6 / h`, `2.4 / h (3×)`; 0.8 at 2× and 3× agree with both costs | Same as S1 panel |
| `U20-setups-developer.png`, `U20-setups-admin.png` | Extra shots at 1×; cost and Held by cells on one line, Disabled only in the State column | None |

## Commands and counts

- `./ci.sh`: exit 0. Bazel: "Executed 0 out of 40 tests: 40 tests pass", so all 40 targets came from the cache and none ran in this invocation (`ci.log`).
- `npx vitest run` in `ui/`: 18 files, 276 tests, all pass (`vitest-all.log`). These did run.
- `go test ./internal/api -count=1 -v`: 200 top-level tests pass, 0 fail; the UI-18 test's five subtests, `TestWGT14_…` and `TestUI12_…` pass (`gotest.log`).
- Deliberate breaks: 26, each red, each restored with a checksum (`mut/mutate.py`, `mut/results.txt`).
- `e2e/run.sh both`: exit 0. Bundled: 1 Playwright test passed, 16 steps ok, 0 failed, 774 s. Split: 1 passed, 16 steps ok, 0 failed, 769 s. 32 steps in all (`e2e-both.log`, `e2e/out/bundled`, `e2e/out/split`). It was started before the scope change and left to finish; `both` does not include the integration mode.
- Integration mode: not run by me. Task 09's log `tasks/09-b5-e2e/e2e-integration.log` shows exit 0, 1 Playwright test, 20 steps, with `ok U20` (4 s), `ok S1` (56 s), `ok S2` (61 s) at lines 46 to 48; `run2-steps.json` has all three with status pass and their observed values. None was skipped.

## Tree state

`git diff --stat` before and after is the same (`diffstat.before.txt`, `diffstat.after.txt`). No git state change. The integration stack was never started by me; `e2e/run.sh` tears its own stacks down. `hourglass-demo` was not touched.

## Confidence and gaps

- High for the kept behaviour: unit tests that I saw fail on purpose, plus the built page driven in Chromium.
- **Not done: the run against the real stack** (`e2e/integration-stack.sh up`). The browser work used a mocked API, because port 8080 was held by `e2e/run.sh both`, and the scope change then said to finish with what I had. A script for it is ready and unrun: `browser/stack.cjs` (panel under real acquire, release, disable and delete; `pressure` null before the first poll; a real `now > at_peak` after lowering the bands). Real-server evidence for the panel is therefore task 09's W2, U20, S1 and S2 only.
- Chromium only. Firefox and Safari are not installed here.
- `./ci.sh` proved nothing new by itself, since every target was cached; the direct vitest and Go runs are the evidence.
- I did not rerun the integration mode, so I rely on task 09's log for U20, S1, S2.
- The mock's responses follow the test fixtures and the schema; a field the real server sends differently would not show up here.

## Out of reduced scope (done before the scope change; not part of the verdict)

- Layout, 26 cases (`browser/layout.json`, `browser/shots/layout-*.png`): the claimed five-key case fits at 1280 px for a developer and an admin (table 1152 of 1152, label columns 94 px). In my row mix a 22-character name and seven keys also fit, where D94(h), (i) say they scroll; a 40-character name scrolls. Fits down to 1024 px; scrolls at 768 and 375.
- Breaking the CSS in the page shows the property U20 asserts can fail: without `max-width: 0` the scroller is 2291 of 1152 (`browser/u20break.cjs`). U20 itself was not mutation-tested.
- B2 chip with `Team A (infra)` against the mock: filters, combines with a selector chip and the State filter, replaces and clears per key, never reaches the selector box or a request. One observation: with more than 8 keys, an exact filter stays in force after "Hide labels" with no visible sign of it unless the key is a column.
- Mutations M16 to M20 and M26 (UI-19, UI-20, SET-6) were red.
