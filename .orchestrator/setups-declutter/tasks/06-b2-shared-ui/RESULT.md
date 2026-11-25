# Result: 06 (B2) shared UI pieces

Status: **done**. 2026-11-24.

## What was done

DESIGN.md Rev 2 task B2 (sections 3 "Layout mechanism", 4 "Component", 6 "UI"): `shrink` and `dense` on `Table`, a `ShrinkText` cell helper, the Q-A one-line rule, the Modal Shift+Tab fix and focus return with a fallback, and `formatRate`. Every addition is opt-in; a table or modal that passes none of the new props renders the same markup as before (asserted in the Table test, and the existing 259-test suite passes).

## API for the Setups-page builder (B3)

All in `ui/src/components/Table.tsx` unless noted.

- `Column<T>.shrink?: boolean` (`Table.tsx:19`). The column gives way: `th` and `td` get class `styles.shrink` (`max-width: 0; min-width: 6ch; overflow: hidden; text-overflow: ellipsis; white-space: nowrap`, selector `.table .shrink`, `Table.module.css:52`). Each shrink `th` gets `style.width = (100 / number of shrink columns)%` and `title` = its header (`Table.tsx:46-60`). Give every other column `nowrap: true` (existing option), or those columns get squeezed.
- `TableProps<T>.dense?: boolean`, default `false` (`Table.tsx:33,44`). Adds `styles.dense` to the `<table>`; cells get 8 px side padding instead of 12 px (`.table.dense th, .table.dense td`, `Table.module.css:43-47`).
- `export function ShrinkText({ text }: { text: string })` (`Table.tsx:102`). Renders `<span class={styles.shrinkText} title={text}>{text}</span>`: one line, ellipsis, full value as tooltip (`Table.module.css:62`). Use it as the content of every shrink cell. It is **not** re-exported from `ui/src/components/index.ts` (that file is outside my scope): import it with `import { ShrinkText } from "../../components/Table"`, or have the orchestrator add `ShrinkText` to line 16 of `index.ts`.
- Q-A switch: the comment at `Table.module.css:60-61` says which one rule changes for two lines.
- `ModalProps.fallbackFocus?: RefObject<HTMLElement>` (`ui/src/components/Modal.tsx:22`). On close, focus goes to the opener when `opener.isConnected`, otherwise to `fallbackFocus.current` (`Modal.tsx:79-80`). Setups passes a ref to its `h1` with `tabIndex={-1}`. Without the prop: opener only, as before.
- Modal Shift+Tab: from the first item, the dialog node, or any focused element not in the Tab order (for example the Price `h3` with `tabIndex={-1}` and `data-autofocus`), Shift+Tab wraps to the last focusable item (`Modal.tsx:66-70`). No prop. Initial focus is unchanged: a control in the body first, then `[data-autofocus]`, so the panel body must hold no button/link/field for the heading to get focus.
- `export function formatRate(tokensPerHour: number): string` (`ui/src/lib/format.ts:98`): 1 to 3 decimals, no unit (`2` -> `"2.0"`, `0.125` -> `"0.125"`). `formatPerHour` now returns `` `${formatRate(x)} / h` `` (`format.ts:104`); output unchanged.

## Tests (named as DESIGN.md section 6)

- `ui/src/components/components.test.tsx:62` `Table marks shrink columns and dense tables` (also asserts a table without the props has no class, style or title on `th`/`td`).
- `components.test.tsx:226` `Modal keeps Shift+Tab inside when a heading has focus`.
- `components.test.tsx:235` `Modal returns focus to the fallback when the opener is gone`.
- Extra: `:245` opener still present wins over the fallback; `:253` no fallback, opener gone: focus is not moved (today's behaviour).
- `ui/src/lib/format.test.ts:71` `formatRate` (two cases, including that `formatPerHour` is `formatRate` + ` / h`).
- Mutation check: with the Shift+Tab and fallback lines reverted, the two named Modal tests fail (2 failed, 17 passed); restored afterwards.

## Commands and outcome (from `ui/`)

- `npx vitest run src/components src/lib`: 3 files, 41 tests passed (`vitest.log`).
- `npx vitest run` (whole UI suite, including other agents' current edits): 18 files, 259 tests passed (`vitest-all.log`).
- `npx tsc --noEmit -p tsconfig.json` (the UI type check; the repo has no separate linter for the UI): exit 0 (`tsc.log`). This ran against the working tree as it was, including another agent's in-progress `schema.gen.ts`, `fixtures.ts` and `setups.test.tsx`; no failure there.
- No `./ci.sh`, no e2e, no git state changes.

## Files changed

`ui/src/components/Table.tsx`, `ui/src/components/Table.module.css`, `ui/src/components/Modal.tsx`, `ui/src/components/components.test.tsx`, `ui/src/lib/format.ts`, `ui/src/lib/format.test.ts`. Plus this folder's `PROGRESS.md`, `RESULT.md` and logs.

## Deviations from DESIGN.md

- DESIGN section 3 item 5 asks for the label cell content `<span class="shrinkText" title={value}>`. Because CSS-module class names are private to `Table.module.css`, this is an exported `ShrinkText` component instead; same markup. Needs either a deep import or a one-word addition to `components/index.ts` (outside scope; reported above).
- The Shift+Tab fix wraps for any focused element outside the focusable list, not only a heading; for existing dialogs this only changes a case that previously let focus leave the dialog.

## Confidence and gaps

- High for the mechanism and the tests. jsdom does no layout, so the CSS (that labels give way at 1280 px, that `dense` padding applies over `.table th`) is not checked here; by inference from specificity (`.table.dense td` 0,2,1 over `.table td` 0,1,1; `.table .shrink` 0,2,0 over 0,1,1). The real check is B5's U20 step.
- The shrink header text is clipped by the `th` itself (`text-overflow` on the cell); a header longer than its column shows an ellipsis with the key as `title`. Inference, not measured.
