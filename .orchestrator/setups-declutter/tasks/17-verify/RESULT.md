# Result 17 (V2): verification of the Revision 3 build

Date 2026-11-24. Branch `setups-declutter`, uncommitted working tree against `main` at `f1a05c2`. Nothing in the working tree was edited; every deliberate break was made in the private copy `ui-copy/` in this folder. Paths below are relative to the repository root unless they start with a task-folder name (`browser/`, `mut/`, `pages/`), which are under `.orchestrator/setups-declutter/tasks/17-verify/`.

## Verdict

**Ready after the listed fixes.** 0 blockers, 2 should fix before release (both one-line), 8 minor. The end-to-end task can start now; neither fix changes a control, a role or a name. Should-fix 1 changes one table message, so task 16 should assert "No setups match the filters." for a selector that matches nothing only after that fix is in.

## Findings

### Blocker

None.

### Should fix before release

1. **A selector that matches no setup makes the table say "No setups to show." while a filter is plainly in force.**
   - Evidence: `ui/src/pages/setups/index.tsx:414` chooses the message by `data.setups.length === 0`, and the API's list is already filtered by the selector. Seen in Chromium: `browser/r3a.log`, check "P1" (selector `site=nosuch`, with and without a token: cell text "No setups to show."; the status line at the same moment says "No setups match the filters."; "Clear filters" is on screen).
   - Scenario: a developer picks `site=ams`, adds `,size=large` and presses Enter; no setup has both. The table tells them there are no setups at all, the screen-reader line tells them the filters match nothing. UX-SPEC 2.6 gives "No setups match the filters." for this case. The same line of code is on `main`; it matters more now because picking a value applies at once and the page announces the other sentence.
   - Smallest fix: `emptyMessage={inForce ? "No setups match the filters." : "No setups to show."}` (`inForce` is already computed at `index.tsx:150`), plus one assertion in the UI-21 apply test.
2. **`docs/progress.md:143` names a test that does not exist.**
   - Evidence: it cites `UI-9 the placeholder is the first non-null term of the index`; the built test is `UI-9 the placeholder is the first term of the index that is at most 16 characters, after the words Type or pick a label` (`ui/src/pages/setups/selectorSuggest.test.ts`). Every other test name in `docs/progress.md:116-158`, the amendment section of `docs/design/setups-declutter.md` and D96 to D99 exists (script check, 0 other misses).
   - Scenario: someone tracing UI-9 from the requirement table finds no such test.
   - Smallest fix: replace the name in that row.

Not a finding against this build, for the orchestrator: `e2e/README.md:13,51` and the two spec files still describe chips and five display keys. They belong to task 16, which runs after this one.

### Minor

1. **Three picks leave text the API refuses, or remove a neighbouring word.** None of them applies the filter, so nothing wrong is sent by the pick itself. `browser/r3a.log`, notes "1e", "P2", "P3".
   - Caret after a comma and before a space inside a list, `site in (ams,| sjc)`, pick `lon`: `site in (ams,lon sjc)` (refused: "expected , or )"). DESIGN-R3 says a list pick adds no comma, so this is as designed.
   - `site |=sjc`, pick `!=`: `site!==sjc` (refused). Fix: offer no operator when one already follows (`ctx.bare` is false).
   - `site |sjc`, pick `=`: `site=`; the word after the caret is deleted, because a token that starts at the caret counts as "under" it (`selectorSuggest.ts:112-113`).
2. **Settings sends the keys as they were when the admin started editing, not as last loaded.** `form.ts:47,94` with `index.tsx:187,220-222`. In Chromium (`browser/r3b.log`, notes "P16", "P16b"): the admin types a new default weight, a script changes `display_label_keys` through the API, the page polls, the admin saves: the PUT carries the old keys and undoes the script's change (with an audit entry). After such a poll, undoing the edit by hand still leaves "Unsaved changes" and an enabled Save with nothing visibly different. Narrow race; the same last-write-wins holds for every field, but this one is now invisible. Fix: take `display_label_keys` from the current server settings when building the body, and leave it out of the dirty comparison.
3. **An exact-match pick can leave an applied requirement for the same key in force with nothing showing it.** `browser/r3a.log`, note "P4": applied `site=sjc,owner=plain`; the user edits the owner value and picks an exact value; the box reads `site=sjc`, the API filter is still `site=sjc,owner=plain`, the token is added, the table is empty. The Filter button does say the typed selector is not applied. As designed ("the box is not applied by this pick"); pressing Filter resolves it.
4. **Typing past a space in a value closes the list.** `owner=Team A` shows nothing (note "P5"), so two values that differ after their first word cannot be narrowed by typing; they must be picked from the list after `owner=Team`.
5. **Costs above 10,000 tokens per hour scroll the table at 1280 px.** With `12,345.678 now · up to 37,037.034`, a tag on every row and a 24-character holder: scroller 1239 of 1152 (`browser/r3b.log`, note "18"; `browser/shots/18-big-costs-1280.png`). Task 13 reported 32 px; with these rows it is 87 px. Ordinary costs fit.
6. **A setup without a `labels` object crashes the page once a token is in force** (`index.tsx:95` reads `s.labels[t.key]`; the panel guards with `?? {}`). The server always sends an object (`handlers_user.go:856`, `labelsNonNil`), and `main` read `s.labels[k]` the same way, so this needs a broken or foreign API to happen. `browser/r3b.log`, note "P a setup with no labels object".
7. **Two CSS behaviours have no automated test.** Removing `position: relative` from the shared scroller (M34) and `overflow-wrap` from the cost note (M35) leaves all 300 unit tests green. Both hold in Chromium (below); only a browser check would catch a regression.
8. **`./ci.sh` executed nothing new.** Exit 0, "Executed 0 out of 40 tests", all cached from task 13's run on the same inputs. The direct vitest and Go runs below are the evidence that the tests pass.

## Requirements matrix

"Red" = the named test failed when I broke the code in `ui-copy/` (`mut/mutate.py`, `mut/results.txt`: 35 breaks, 33 red, 2 CSS-only survivors).

| Item | Tests | Can it fail? | Verdict |
| --- | --- | --- | --- |
| UI-15 | `UI-15 the table's columns are Setup, State, Held by, Cost and actions, whatever display_label_keys holds` | Red (M12) | Met; five headers in Chromium, developer and admin |
| UI-16 | two `UI-16 …` tests | Red (M29) | Met |
| UI-17 | `UI-17 the Held by cell …` | Red (M30) | Met |
| UI-18 | the `UI-18 …` block; Go `TestUI18_…` (5 subtests) | Red (M31, M32, M33: 12 tests) | Met; opens from the cost cell and Details, focus on Price, focus returns to the opener |
| UI-19 | `UI-19 the page has no per-label dropdown and no label chips; …` | Red (M18) | Met |
| UI-20 | `UI-20 every column of the Setups table is one line` | Red (M17) for the mechanism only | Met at 1280 px in Chromium (32- and 60-character names, 72-character one-word reason, pending release); minor 5 |
| UI-21 | 14 in `selectorSuggest.test.ts`, the `UI-21 …` block in `setups.test.tsx` | Red (M01 to M06, M08, M09, M20 to M25) | Met; minor 1 and 4 |
| UI-22 | `UI-22 …` block | Red (M04, M07, M10, M11, M13 to M16) | Met; minor 3 |
| UI-23 | six `UI-23 …` tests | Red (M26, M27, M28) | Met; minor 2 |
| UI-9 as amended | `UI-9 the placeholder is the first term … at most 16 characters …` | Red (M08) | Met; should-fix 2 (doc names the old test) |
| UI-12 amended, WGT-14 | `UI-12 …` (two), Go `TestUI12_…`, `TestWGT14_…` | Red (M29, M32) | Met |
| SET-6 retired | Go `TestSET6_SetupsListForDeveloper` still passes | not broken | API keeps the field; no page reads it |
| Constraint UI-1 | `UI-1 …` | read | Met: the only arithmetic is counts of options and rows (`SelectorBox.tsx:94`, `index.tsx:110`) |
| Constraint: additive API | contract and upgrade tests | read | Met: against `main` the handler gains `pricing` and its struct; openapi gains `pricing` (required) and description text; nothing removed or renamed |
| Constraint: generic | `tools/check_core_generic.sh` | not broken | Met; no customer key in `ui/src`, e2e specs, `internal`, `api`, `deploy` |
| Constraint: accessible | UI-18, UI-21 ARIA tests | Red (M21, M24) | Met in Chromium; no screen reader used |
| Constraint: nothing lost | UI-12, UI-18 | Red (M31, M32) | Met |
| R3-1 | UI-23 | Red | Met: no control or text; stored keys sent back byte for byte with 0, 5 and 32 keys |
| R3-2 | UI-22 | Red | Met: every exact value picked in turn, no request carried one |
| R3-3 | UI-15, UI-18 | Red | Met |
| UX defaults Q1 to Q5 | | | As built (name kept, values suggested directly, one in-force line, cost not blue, phone layout as other tables) |

## DESIGN-R3 break list, in Chromium

Built UI (`dist-new/`, vite production build of the tree), API mocked with a parser that follows `internal/rules/selector.go`'s grammar (`browser/lib.cjs`). `browser/r3a.cjs`: 113 checks, 112 pass (the failure is should-fix 1). `browser/r3b.cjs`: 67 checks, 67 pass.

| # | Result |
| --- | --- |
| 1 Caret in the middle | Pass for `si|te=sjc`, `site=s|jc,tier=gold`, `site in (a|ms, sjc)`, between two spaces, after a comma touching the next item. Minor 1 for the comma-then-space case |
| 2 Pasting | Pass: nothing applied; Enter sends the text as it stands; a newline becomes a space (the browser's doing); invalid text shows the API's message |
| 3 Composition | Pass with a CDP input method and with synthetic events: no recompute, no pick, no submit; the list follows after the end |
| 4 500 keys, 2,000 values | Pass: 50 options in the DOM, "451 more", "4950 more", "1950 more" right and in view; slowest keystroke to paint 49 ms, no long task; arrows wrap and keep the active option in view |
| 5 Prefix keys | Pass |
| 6 Awkward values | Pass: spaces, commas, parentheses, `=`, `!`, quotes, angle brackets, emoji, 63 characters, empty. Shown as text in the list, token, `title` and panel; no element or script created; a value with a term (`<b>bold</b>`) is inserted as is and accepted |
| 7 Duplicate token | Pass |
| 8 Contradiction | Pass for `!owner` and another `owner=` value when the API still returns rows; should-fix 1 when it returns none |
| 9 Escape | Pass |
| 10 Focus | Pass: after every kind of pick, after removing a token, after Clear filters; Tab picks nothing and lands on Filter |
| 11 ARIA and status | Pass: expanded, controls, activedescendant, group names, every status text; a refresh with the list open changes nothing. No screen reader |
| 12 Old API | Pass: no `label_index`, no `pricing`, empty index. Settings PUT against a mock that requires the key succeeds |
| 13 Free typing | Pass: 13 forms by Enter and by the button reach the API as typed; 5 invalid ones show the message and mark the box |
| 14 Apply on pick, unfinished requirement | Pass: `!, site=sjc` sent, error shown, text kept, last rows kept |
| 15 Touch | Pass at 390 px, coarse pointer: 44 px options, tap picks, drag scrolls and picks nothing, tap outside closes, arrow button toggles |
| 16 Settings | Pass; minor 2. No audit check was possible without a server; an unchanged body gives the server nothing to record (inference from task 14 and the handler) |
| 17 Dead code | Pass: no `shrink`, `ShrinkText`, `dense`, `LabelChips`, `applyChip`, `labelTerms`; `display_label_keys` only in fixtures, tests, `form.ts`, `schema.gen.ts` |
| 18 Layout | Pass at 1280 (developer, admin); list over the table and not cut, z-index 30 under the dialog's 40; page does not scroll sideways at 1024, 768, 390. Table scrolls inside its scroller at 1024 (1142 of 960), as documented. Minor 5 |
| 19 Kept parts | Pass (M29 to M33 red; panel focus checks) |

## Shared table, page by page (step 4)

Done by a helper agent writing only under `pages/` (`pages/results.json`, `pages/shots/`); both builds, same mocks, fixed clock. I did not repeat its measurements.

- My usage, Users and pre-approvals, Audit log, Service accounts, Setup inventory at 1280 px: every table, header and cell box identical to `main`; 0 pixels differ.
- Same pages at 390 px: boxes identical; the only change is that the page no longer scrolls sideways (page width 686, 709, 468, 1023 on `main`, 390 to 392 now). Every table scrolls to its last column inside its scroller.
- Settings: differs only where the field was removed; the gap from Default weight to the rules table is 12 px in both builds. Its own Rate rules table (`index.tsx:414-425`): no sideways page scroll at 1280, 768, 390, 375 in either build. At 320 px the page is 30 px too wide in both builds because of its hidden "Actions" header; unchanged by this work.
- My usage is 2 px too wide at 390 in both builds, from two sections unrelated to tables.

## Unchanged surfaces (step 5)

- No file under `ui/src/pages/usage` (except added exports in `weight.tsx`), `admin-setups`, `admin-users`, `admin-audit`, `admin-service-accounts` differs from `main`. `Table.tsx` and `components/index.ts` are identical to `main`. The acquire dialog's text in `setups/index.tsx:437-451` is the wording of `main`.
- API against `main`: `pricing` and description text only (matrix).

## Docs against the build (step 6)

- Quoted UI strings of `tasks/15-c3-docs/RESULT.md` in `docs/ui-notes.md:63-102,187,258`: checked against the page source by script and by reading the misses; all are in the build or are examples. D96 to D99 match what is built, including the UX overrides in D99.
- Test names: should-fix 2.
- No doc describes label columns, chips or the Settings field as current, except `e2e/README.md:13,51` (task 16).

## Commands and counts

| Command | Outcome |
| --- | --- |
| `./ci.sh` | exit 0; 40 targets pass, 0 executed, 40 cached (`ci.log`) |
| `npx vitest run` in `ui/` | exit 0; 19 files, 300 of 300 (`vitest-all.log`) |
| `go test ./internal/api -count=1 -v` | exit 0; 200 top-level tests, 234 subtests, 0 fail (`gotest.log`) |
| `python3 mut/mutate.py` (in `ui-copy/`) | 35 breaks, 33 red, 2 survive (CSS); copy restored and identical to the tree |
| `node browser/r3a.cjs`, `node browser/r3b.cjs` | 180 checks, 179 pass |
| Helper's `pages/run.cjs`, `pages/extra.cjs` | 7 pages, 2 builds, 2 to 5 widths |

Tree state: the hash of `git diff` plus untracked product files is the same before and after (`treehash.before.txt`, `treehash.after.txt`); `git status` the same. No git state change, no docker, no e2e. Two containers of another project (`platform-lab-*`) were already running and were not touched.

## Confidence and gaps

- High for the selector box, tokens, Settings round trip and kept parts: unit tests seen to fail on purpose, plus the production build driven in Chromium.
- The API was a mock. Its selector parser is mine, written from DESIGN-R3's grammar table; a difference from the real parser (for example which characters a bare value may hold) would not show here. The real stack is task 16.
- Chromium only. No screen reader, no real touch device, no real input method (CDP composition and synthetic events instead).
- The page-by-page comparison is the helper's; I read its report and numbers, not every screenshot.
- Not checked: the audit log after a Settings save, the wireframes, `docs/SPEC.md` (not edited by design, D98).
