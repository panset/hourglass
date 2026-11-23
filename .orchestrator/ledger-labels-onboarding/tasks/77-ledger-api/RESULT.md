# 77 Ledger API: roll-up view, paging, balances: result

Status: done. Date: 2026-11-23.

## What was built

`GET /api/v1/me/ledger` now takes `rollup`, `limit` and `cursor` and returns `next_cursor` and a per-entry `entry_count`, as design A4 describes.

- `parseLedgerQuery` (`internal/api/handlers_user.go:264`) checks the three parameters. Every bad value is 400 `validation` with `field` set to the parameter's name. A parameter that is present with an empty value (`limit=`) counts as given, and is therefore invalid.
- `ledgerView` (`internal/api/handlers_user.go:316`) builds the view in Go from the period's rows (`ListLedger`, id order).
  - In the `rollup=day` view, every `debit` row that has a hold, raw or rolled up, is merged with the other debit rows of the same hold and the same local day in `a.d.Location`.
  - A merged entry sits at the position of its newest row and takes that row's id and `created_at`.
  - `entry_count` is the sum of `max(rollup_count, 1)` over the merged rows.
  - The running balance is computed over the whole view, so it holds on every page.
- `listMyLedger` (`internal/api/handlers_user.go:383`) cuts the page from the view after merging:
  - it keeps the entries whose id is below the cursor (binary search over the id-ordered view) and takes the newest `limit` of them;
  - `next_cursor` is the id of the oldest entry on the page, and only when older entries exist;
  - it looks up setup and actor names only for the page's entries;
  - `multiplier` is reported only when `entry_count == 1`;
  - `actor_name` and `reason` are null for an entry that merged more than one row.
- No store change was needed. `ListLedger`, `DebitMultipliers` and `RollupCount` already provide everything.
- `api/openapi.yaml` has the three parameters (`rollup` enum `[day]`, `limit` 1–200 integer, `cursor` string), `next_cursor` (nullable, required) on `LedgerResponse`, and a new schema `LedgerViewEntry` (the `LedgerEntry` fields plus a required `entry_count` ≥ 1).
  - `LedgerEntry` itself is unchanged, because the admin grant/reset response (`entry`) also uses it, and the contract test closes schemas.
  - The operation description documents LED-11 to LED-14.
- `ui/src/api/schema.gen.ts` was regenerated with `cd ui && npm run gen:api`.

## Requirement -> test -> file

| Requirement | Test | File |
| --- | --- | --- |
| LED-11 | `TestLED11_NoNewParameterAnswersAsBefore` | `internal/api/ledger_view_test.go:222` |
| LED-12 | `TestLED12_DayViewIsTheSameBeforeAndAfterCompaction` | `internal/api/ledger_view_test.go:350` |
| LED-13 | `TestLED13_PagingWalksEveryEntryOnce` | `internal/api/ledger_view_test.go:408` |
| LED-14 | `TestLED14_BalancesHoldAcrossPages` | `internal/api/ledger_view_test.go:508` |
| LED-15 (API level) | `TestLED15_CompactedRowHasNoMultiplierInDefaultView` | `internal/api/ledger_view_test.go:285` |
| UPG-3 (contract for the new parameters and fields) | `TestUI2_ContractResponsesMatchSchemas` (extended) | `internal/api/contract_test.go` |

All five LED tests share one scenario, `newLedgerWorld` (`ledger_view_test.go`):

- The configured zone is Europe/Berlin, and two holds meter at 2x. Their polls at 21:00Z–23:15Z on 2026-11-02 fall on two local days.
- An admin grant lands in the middle of the local Nov 2.
- One hold is released on Nov 3.
- The clock then jumps to 2026-11-12, where the other hold keeps metering (today's entry).
- Compaction runs through `ledger.Service.CompactDue`, the real path.

How each test checks its requirement:

- **LED-11.** The test compares against `internal/api/testdata/led11_pre_change_responses.json`. I recorded that file from the handler as it was before this task, on the same scenario, both before and after compaction. A throwaway test produced it and was then deleted.
  - The test removes `entry_count` and `next_cursor` from the new response and requires deep equality with the recording. That covers ids, order, every field and every balance.
  - It also asserts that `next_cursor` is null, that `entry_count` is 1 (or the stored `rollup_count` after compaction), that there is one entry per stored row, and that the polls are preserved in total.
- **LED-12.** The expected day view is computed independently from the raw default view. The whole `rollup=day` response must be deep-equal before and after compaction.
  - The test also checks that merged entries have a null `multiplier`, `actor_name` and `reason`.
  - It checks that a single raw debit in the day view keeps its multiplier (a second, small environment).
- **LED-13.** For both views, both before and after compaction:
  - Walking with no new polls at limits 1, 2, 3, N, N+1 and 200 gives exactly the unpaged entries.
  - At limit = N, `next_cursor` is null.
  - Walking at limits 1 and 2 with a poll between every request returns no id twice and no (hold, local day) twice in the day view, and returns every unchanged entry exactly once.
  - Seventeen bad queries are 400 `validation` with the right `field`: cursor without limit, malformed cursor, limit 0, 201, text, empty, and so on.
- **LED-14.** For both views, both before and after compaction, the test walks the pages at limits 1, 2 and 5. It requires that each entry's balance equals the next older entry's balance plus its own amount (for the oldest entry, its own amount), and that the newest entry's balance equals the period balance from `GET /api/v1/me`. The grant in the middle of a day is part of the data.

## Deliberate breaks (each made, run, then restored; the file was confirmed byte-identical with `cmp`)

| Break in `handlers_user.go` | Named test red | Also red |
| --- | --- | --- |
| LED-11: merge debit rows even without `rollup` | TestLED11 | TestLED12 |
| LED-12: a merged entry keeps its oldest row's id instead of its newest | TestLED12 | none |
| LED-13: apply the cursor to stored rows before merging | TestLED13 | TestLED14 |
| LED-14: balance computed over the page only | TestLED14 | TestLED13 (its page concatenation no longer equals the unpaged response) |

## Example response shapes (in words)

1. **No parameters, after compaction** (the scenario above):
   - The top level is `period`, `available_periods`, `entries` and `next_cursor: null`.
   - There are 9 entries, newest first: three raw debits of today, each with multiplier 2 and `entry_count` 1. Then four rolled-up debits (one per hold and local day) with `entry_count` 4, 3, 23 and 23 and a null multiplier. Then the grant, with `actor_name` and `reason`. Then the allowance.
   - The grant's balance is 350, not 350 minus the earlier debits of that day, which is the A3 effect. Every closing balance is unchanged.
2. **`rollup=day`, before or after compaction (identical):**
   - There are 7 entries. First is today's merged entry for hold 1 (the id of its newest poll, `entry_count` 3, multiplier null).
   - Then the four day entries for Nov 3 and Nov 2 (local), then the grant and the allowance.
   - Each balance is the one below it plus its amount, and the newest balance equals the period balance.
3. **`rollup=day&limit=2`:** two entries, the two newest of the view above, and `next_cursor` set to a decimal string (the id of the second entry). Following it with `&cursor=…` returns the next two older entries. The last page has `next_cursor: null`.

## Files changed

- `internal/api/handlers_user.go`: the parameters, the view, paging and `entry_count`.
- `internal/api/ledger_view_test.go` (new): the scenario and the LED-11 to LED-15 tests.
- `internal/api/testdata/led11_pre_change_responses.json` (new): the responses recorded from the pre-change handler.
- `internal/api/user_test.go`: in `TestLED2_MyLedger`, the exact-field lists gained `next_cursor` and `entry_count`. These are the additive fields; no assertion was relaxed. Without this change the existing test fails on the additive fields, because it checks the exact key set.
- `internal/api/contract_test.go`: calls with `rollup=day`, `limit=1`, `rollup=day&limit=2&period=…`, a real cursor walk, and four new 400 cases. Nothing was removed.
- `internal/api/BUILD.bazel`: the new test source (Gazelle) and the testdata file in `data` (added by hand; Gazelle's diff is clean).
- `api/openapi.yaml`: the parameters, `next_cursor`, the `LedgerViewEntry` schema and the description.
- `ui/src/api/schema.gen.ts`: generated.
- `ui/src/pages/usage/usage.test.tsx`: a minimal fixture fix so the generated types type-check (`entry_count: 1` on the two entries, `next_cursor: null`). The ledger page itself is not touched (task 78).

## Commands run

- `go test ./internal/...`: all packages ok.
- `go test ./internal/api -run 'TestLED1[1-5]_'`: ok, and red under each break as listed above.
- `bazel run //:gazelle -- update -mode=diff`: clean.
- `cd ui && npm run gen:api`, `npx tsc --noEmit -p tsconfig.json` (exit 0), `npx vitest run src/pages/usage` (11 passed).
- `./ci.sh`: exit 0 (`ci.exit`; log in `ci.log`).
- `tools/check_core_generic.sh`, `tools/check_spdx.sh`: OK.

## Decisions for docs/decisions.md

1. **Cursor encoding.** The cursor is the decimal id of the last (oldest) entry of the page, as a string, and it means "entries of this view with id < cursor". It must match `^[1-9][0-9]{0,18}$` and fit an int64; anything else is 400 `validation` (`field: cursor`). It is documented as opaque. It does not encode `period` or `rollup`. A cursor used with another view is not detected; it still means "ids below this".
2. **Where merging happens.** Merging is in Go, in the handler (`ledgerView`), over the period's rows from `ListLedger`, grouped by the local day in the configured `Location`. No new store query was added. Paging and balances are computed over the merged view, never over stored rows.
3. **Separate schema.** The response entries use a new OpenAPI schema `LedgerViewEntry` (`LedgerEntry` plus `entry_count`). `LedgerEntry` is left as it is because the admin grant/reset response shares it, and adding a required field there would change that route's contract. The UI's `LedgerEntry` alias in `ui/src/pages/usage/holdApi.ts` still points at `LedgerEntry`; task 78 should switch it to `LedgerViewEntry` to read `entry_count`.
4. **Empty parameter values.** `rollup=`, `limit=` and `cursor=` count as given and are 400 `validation`. This differs from the audit route, where `limit=` counts as absent; I chose the strict reading of "not a number is 400".
5. **Which rows merge.** Only `debit` rows that have a hold are merged in the day view. A merged entry (more than one row) has null `actor_name`, `reason` and `multiplier`. A day with a single row (raw or rolled up) is that row unchanged, apart from `entry_count`.
6. **Multiplier guard in the handler.** The handler shows `multiplier` only when `entry_count == 1`. This guards LED-15 in the API on top of the `DebitMultipliers` filter in the store.

## Confidence and gaps

- I am confident about the behaviour I verified: the tests, the deliberate breaks and a green `ci.sh`.
- The LED-11 recording comes from the pre-change handler on the current store, which already had compaction (milestone 2). It is therefore "the handler before this task", not the `d96a950` release. The handler code was unchanged since that release, so it appears equivalent (inference).
- I did not run `e2e/` (out of scope). `integration.spec.ts` calls the route without parameters, and LED-11 shows that response is unchanged apart from the added fields. I infer, but have not verified, that the spec still passes.
- Performance: the existing scale test (`perf_compaction_scale_test.go`, the default view) still passes. The paged views were not timed separately. They read the same rows and do O(n) work in Go.
