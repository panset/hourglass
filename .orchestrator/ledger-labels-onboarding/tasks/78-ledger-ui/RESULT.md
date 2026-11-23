# Result, task 78: Ledger UI (UI-8) and readLedger

Status: done. `./ci.sh` exit 0 (`ci.exit`, `ci.log`); UI vitest 181 passed; `tsc --noEmit` clean for `ui/` and `e2e/browser`.
The e2e suite was not run (task 85).

## What was built
- `ui/src/pages/usage/holdApi.ts`: `LedgerEntry` now aliases `LedgerViewEntry`; `getMyLedgerPage(period, view, cursor)` builds `?[period=][&rollup=day]&limit=20[&cursor=]`; `LEDGER_PAGE_SIZE`, `LedgerView`.
- `ui/src/pages/usage/index.tsx`: By day / Every poll switch (buttons with `aria-pressed`), Newer / Older, cursor stack in state (page shown = last cursor), cursors dropped on month or view change, table rendered with at most 20 rows, Detail "setup · N polls" when `entry_count` > 1. Poll and release reload use the same fetcher, so they refetch the page shown. Newer and Older are also disabled while a page is loading (avoids pushing the same cursor twice on a double click).
- `ui/src/pages/usage/usage.module.css`: styles for switch and pager. No shared component added.
- `docs/ui-notes.md`: ledger paging described.
- `e2e/browser/tests/acceptance.spec.ts`: `readLedger` switches to Every poll and walks Older; helpers `ledgerPageRows`, `ledgerClick`. Row parsing, the newest-first balance check across the joined pages, and every consuming assertion are unchanged.

## Requirement -> test -> file (all in `ui/src/pages/usage/usage.test.tsx`)
| Requirement | Test |
| --- | --- |
| UI-8 at most 20 rows | "UI-8 shows at most 20 rows even if the API returned more" |
| UI-8 default and switched request | "UI-8 By day is the default: rollup=day&limit=20; Every poll sends limit=20 with no rollup" |
| UI-8 Older / Newer | "UI-8 Older follows next_cursor and Newer returns to the page before" |
| UI-8 disabled at the ends | "UI-8 Newer is disabled on the first page and Older is disabled when next_cursor is null", "UI-8 a single page has both Newer and Older disabled" |
| UI-8 month change -> first page | "UI-8 changing the month returns to the first page and drops the visited cursors" |
| UI-8 view change -> first page | "UI-8 changing the view returns to the first page and drops the visited cursors" |
| A5 poll / release refetch current page | "UI-8 the 60-second poll refetches the page being shown, not the first", "UI-8 the reload after a release refetches the page being shown" |
| A5 Detail / Type cells | "UI-8 Detail shows the poll count only when entry_count is above 1; Type shows the multiplier only when not null" |

Existing test changed: "month picker reloads usage and ledger" now expects `/api/v1/me/ledger?period=2026-10&rollup=day&limit=20` (the request the requirement defines); not weakened.

## Deliberate break
Removed the cursor reset from the month handler: "changing the month returns to the first page..." went red; restored, all green (ci run after).

## Compaction question
`readLedger` is called in A4 (twice, via `untilHome`), A5, the grant step, and after the month jump. Everything before the jump runs on the real clock with debits from the same day, so nothing is old enough to be compacted. After the jump it reads the new month (the poll waits until the Month select shows the new period), whose rows are all newer than the jump. No existing assertion reads compacted days. Inference: the old month is compacted later but is never read by the scenario.

## Decisions for docs/decisions.md
- Newer/Older are disabled while a page is loading, in addition to the stated disabled states.
- The table slices entries to 20 defensively ("never more"), display only.
- `readLedger` always returns to By day, then Every poll, so the walk starts on page 1 regardless of earlier state; it waits for the page's response and row count, then two animation frames, before reading rows.

## Confidence and gaps
- The e2e walk (`ledgerClick` timing) is type-checked only, not executed; the two-animation-frame wait is a heuristic for the React commit after the response.
- Lint: no separate lint target found beyond `tsc` and `./ci.sh`.
