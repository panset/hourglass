# 79b Fix the findings of verification 79: result

Status: done. `./ci.sh` exit 0 (`ci.exit`, `ci.log`; Bazel executed 10 of 39 tests, all pass), `go test ./internal/...` ok, `npx vitest run` 183 pass, `tsc --noEmit` clean.

## Findings and what closed them
| id | Closed by |
| --- | --- |
| F1 | `ui/src/pages/usage/index.tsx`: the fetcher returns the page with the request key it answers (`ledgerKey`); the table and `next_cursor` use data only when its key equals the current key. A failed request for another page shows the existing `<Failed>` alert (not the old table); Older is then disabled (no cursor to push twice), Newer works. A failed poll of the page already shown still keeps the table, as before. Test: `UI-8 a failed request for another page shows the error, not the previous page, and the controls still work`. |
| F2 | Same file: an effect clears the cursors when `me.period` changes while the picker has not set a period. Test: `UI-8 a month rollover while paged on the default month drops the old cursors and shows the first page` (went red with the effect removed). |
| F3 | `internal/api/handlers_user.go`: `limit` must match `^[0-9]+$`, then 1..200. LED-13 validation cases added: `+5`, `1.0`, ` 5`, `5 `, `00`, a 20-digit value (plus the existing `-1`). Note `limit=007` is still accepted as 7 (digits only, as the brief says). |
| F4 | `api/openapi.yaml`: `LedgerViewEntry` removed; `LedgerEntry` has an optional `entry_count` (description: always present on `GET /me/ledger` entries, absent on admin grant and reset responses), and `LedgerResponse.entries` references `LedgerEntry` again. `ui/src/api/schema.gen.ts` regenerated with `npm run gen:api`; UI alias is `S["LedgerEntry"]`; the detail column uses `entry_count ?? 1`. The contract test needed no change: it validates responses against the closed schema, and `entry_count` is still sent on every `/me/ledger` entry, which `uWantKeys` in `user_test.go` (exact key set) and LED-11 golden still prove. No check loosened. |
| F5 | `internal/api/ledger_view_test.go`: `TestLED13_PagingWithMoreActiveHoldsThanLimit` (three active holds across midnight, limit 1 and 2, a poll between every request, both views). |
| F6 | `TestLED14_NewestBalanceInAPastPeriod`: the clock moves into December, `period=2026-11` is walked before and after compaction in both views, and the newest balance is compared with the sum of the period's stored rows from `store.SumLedger`. |

## Files changed
`internal/api/handlers_user.go`, `internal/api/ledger_view_test.go`, `api/openapi.yaml`, `ui/src/api/schema.gen.ts`, `ui/src/pages/usage/index.tsx`, `ui/src/pages/usage/holdApi.ts`, `ui/src/pages/usage/usage.test.tsx`, this folder. No BUILD.bazel change.

## Decisions for docs/decisions.md
- Amend D72 (c): the route's entries use `LedgerEntry` again, with optional `entry_count` documented as always present on `/me/ledger`; `LedgerViewEntry` does not exist.
- D72 (d)/A4: `limit` accepts only decimal digits, then 1 to 200; `+5`, `-1`, `1.0`, ` 5` are 400 validation.
- D73 addition: a Ledger page that failed to load shows the error alert instead of the previous page, with Older disabled; a month rollover on the default month resets the cursors.

## Confidence and gaps
- F5 and F6 tests were not run against a deliberately broken handler by me (they pass now); the F2 test was shown red by removing the fix. The F1 test was not mutation-checked cleanly.
- The browser acceptance scenario (e2e) was not run here (out of scope; blocked earlier by a foreign stack on port 8081). `eslint` is not installed locally (npx tried the network), so lint was not run.
