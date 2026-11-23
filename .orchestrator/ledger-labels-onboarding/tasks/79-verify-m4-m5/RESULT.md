# 79 Independent verification of milestones 4 and 5: result

## Verdict

**The code passes, with 6 low findings. The end-to-end run is blocked by the environment.**
LED-11, LED-12, LED-13, LED-14, LED-15 (at the API) and UI-8 are met. Each has a test named
after its id, and each test went red under a deliberate break. The API change is backward compatible: the
recorded pre-change fixture really matches the d96a950 handler, `api/openapi.yaml` removes and renames nothing,
and the generated types match. I found no high or medium defect. `./ci.sh` passes.
`e2e/run.sh bundled` could not start its stack, because host port 8081 was already taken by a separate
`hourglass-tls` stack that has been up for 2 days and that I did not start. This change is not the cause.
The acceptance scenario, and with it the changed `readLedger`, has therefore **not** been exercised.

## Findings

| id | severity | file:line | failure scenario | suggested fix |
| --- | --- | --- | --- | --- |
| F1 | low | ui/src/pages/usage/index.tsx:294, :324 | A user clicks Older and that page's request fails (network or 5xx). `useApi` keeps the previous page's data, so the table silently still shows page N with no error. `<Failed>` renders only when `!ledger.data`. Older stays enabled with page N's `next_cursor`, which is the cursor just pushed. A second click pushes the same cursor again. The deps do not change, so nothing is fetched, and the user needs two Newer clicks to get back. | Show `ledger.error` even when stale data exists (an inline alert). Do not push a cursor equal to the last one. |
| F2 | low (inference, not reproduced) | ui/src/pages/usage/index.tsx:71 | A user on the default (current) month, with `period` undefined, is on page 2 or later when the month rolls over. The next 60 s poll sends the old month's cursor to the new default period. All of the new month's ids are higher than the cursor, so the page is empty ("No ledger entries"), while Newer is still enabled. Clicking Newer recovers. | Key the cursors to the resolved period (for example, reset them when `me.data.period` changes while `period` is undefined). |
| F3 | low | internal/api/handlers_user.go:277 | `strconv.Atoi` accepts `limit=+5` and `limit=007`, while the cursor is parsed strictly (no sign, no leading zeros). The spec only demands 400 for "not a number". This is an inconsistency, not a violation. | Optional: apply the same `^[1-9][0-9]*$` check to `limit`. |
| F4 | low | api/openapi.yaml:1722 | `LedgerResponse.entries.items` now references the new `LedgerViewEntry` instead of `LedgerEntry` (D72 c). The JSON is a strict superset and nothing is removed. However, a customer's generated client changes the element type name of `entries`, which can break typed code that names `LedgerEntry` for ledger rows. | Accept and note it in the upgrade notes, or add `entry_count` to `LedgerEntry` itself (optional there) and keep the reference. |
| F5 | low (test gap) | internal/api/ledger_view_test.go:48 | The committed LED-13 world has only one active hold. The design's named edge case (limit smaller than the number of holds that are metering, with each one's today entry moving to the front) is not in the suite, so a regression there would go unnoticed. My scratch test covered it and passed (see below). | Add a third hold that is still active, or meter h2 too, in `newLedgerWorld`, and walk with limit 1 and 2. |
| F6 | low (test gap) | internal/api/ledger_view_test.go:514 | LED-14 "newest entry's balance equals the period's balance" is checked only for the current period (`/api/v1/me` balance). A past period's closing balance is never compared, so a bug that only shows for `period=` of an older month would pass. | Also walk a past period and compare its newest balance with that period's closing ledger sum or the usage response. |

Checked and **not** a defect:
- A merged entry with a null `setup_name` cannot occur, because a setup with holds cannot be deleted (internal/store/setups.go:109).
- A cursor from another view or period is not detected; this is accepted and recorded in D72 a.
- The handler reads the whole period even with `limit`; the design accepts this as bounded.

## Requirement -> test -> break made, went red

All breaks were made in the working tree and restored with `git checkout -- <file>`. `git diff` is empty at the end.
`git status` shows only the orchestrator's staged BRIEF.md, the pre-existing untracked logs of other tasks and this
task folder.

| Requirement | Test(s) | Break made | Red? |
| --- | --- | --- | --- |
| LED-11 | TestLED11_NoNewParameterAnswersAsBefore (golden), TestLED2_MyLedger (keys) | Reason dropped from non-merged entries (`!ve.merged` changed to `ve.merged`) | yes, LED11 |
| LED-12 | TestLED12_DayViewIsTheSameBeforeAndAfterCompaction | (a) The day key uses the UTC day instead of the configured zone. (b) The multiplier is kept on merged entries. | (a) LED12 and LED13; (b) LED12 |
| LED-13 | TestLED13_PagingWalksEveryEntryOnce | (a) Cursor compared with `>` instead of `>=`. (b) Cursor applied to the stored rows before merging. (c) `limit` maximum raised to 1000. | (a) LED13 and LED14; (b) LED13 and LED14; (c) LED13 |
| LED-14 | TestLED14_BalancesHoldAcrossPages | (a) Balance computed over the page only. (b) Re-sort by id removed after merging. | (a) LED13 and LED14; (b) LED12, LED13 and LED14 |
| LED-15 (API) | TestLED15_CompactedRowHasNoMultiplierInDefaultView | The handler guard (multiplier only when count is 1) was removed. TestLED15 stays green because the store filter (milestone 2) already hides it. LED12 catches the guard's removal for merged day entries. | covered by LED12 |
| UI-8 | usage.test.tsx "UI-8 ..." (10 tests) | (a) No cursor reset on a view change. (b) No 20-row slice. (c) Newer enabled on the first page. (d) No cursor reset on a month change. (e) `ledger.reload()` removed after a release. | (a) view test; (b) 20-row test; (c) 4 tests; (d) month test; (e) reload test |

For the 60-second poll, refetching the page shown follows from the structure. `useApi` polls with `fetcherRef.current`, whose closure holds the current cursor. The test "UI-8 the 60-second poll refetches the page being shown" asserts it. I did not construct a break that isolates it.

## A4 in detail

- Code reading (internal/api/handlers_user.go, `ledgerView` and `listMyLedger`): the view is built over the whole period's rows
  and re-sorted by id. The running balances are computed over the full view. Only after that is the page cut, by
  binary search on the view's ids. The cursor is therefore applied after merging, and balances cross page boundaries.
- My scratch test (`internal/api/zz_verify79_scratch_test.go`, run and then deleted) covered the following:
  - setup: America/Los_Angeles, polls across local midnight on a day whose UTC date differs, three holds all
    metering, and a grant in the middle of the day;
  - compaction: nine days later, one compaction step and then full compaction. `rollup=day` was identical (ids included) before, after one step and after full compaction;
  - walks: both views, limit 1 to 4, which is smaller than the number of active holds, with a poll between every page request. No id was returned twice, every unchanged entry was returned once, a walk without polls equals the unpaged response, and the newest balance equals the period balance;
  - unit check: a `ledgerView` group mixing a rolled-up row (count 5) and a raw row of the same hold and day is one entry (count 6, newest id, summed amount), with a grant between them balanced as in A3.

  **Both tests passed.**

## Backward compatibility

- Fixture provenance: in a scratch worktree (`tasks/79-verify-m4-m5/wt-old`, since removed) at HEAD, I replaced
  `internal/api/handlers_user.go` with `git show d96a950:internal/api/handlers_user.go`. That handler is unchanged between d96a950 and 7b06cec.
  I then ran TestLED11 there. The only failures were the missing `next_cursor` and `entry_count` (and the poll-count check that derives from them).
  There was no "differs from the pre-change response" for either the before-compaction fixture (58 entries) or the after-compaction fixture (9 entries).
  The fixture is therefore genuine pre-change output, and the new handler without parameters equals it plus the two fields.
- `git diff d96a950..HEAD -- api/openapi.yaml`: it adds three optional query parameters, adds `LedgerViewEntry`, and adds `next_cursor`
  to `LedgerResponse`. It removes and renames no field, parameter or route (see F4 on the item schema reference).
  The contract test (TestUI2_ContractResponsesMatchSchemas) now requests `rollup=day`, `limit=1`, all three parameters combined, a real cursor, and four 400 cases.
- Generated types: `npm run gen:api` regenerated `ui/src/api/schema.gen.ts` byte for byte (`git diff` empty).

## Existing tests edited

- internal/api/user_test.go: `uWantKeys` lists gained `next_cursor` and `entry_count`. The assertions are not weaker (the exact key set is still checked).
- internal/api/contract_test.go: requests and 400 cases only added.
- ui/src/pages/usage/usage.test.tsx: the fixtures gained the new fields. One URL expectation changed to the new
  request (`?period=2026-10&rollup=day&limit=20`), equally strict. No assertion was removed.
- e2e/browser/tests/acceptance.spec.ts: `readLedger` now selects Every poll and walks every page with Older,
  through the UI. The per-pair balance assertions and the oldest-first return are kept, so no assertion is weaker.
- e2e/browser/tests/integration.spec.ts: untouched (`git diff d96a950..HEAD` is empty).

## A5 (page code)

The page meets every A5 point:
- At most 20 rows: `LEDGER_PAGE_SIZE`, with `limit=20` plus a render slice.
- The default is By day (`rollup=day&limit=20`); Every poll sends `limit=20` with no rollup.
- Newer is disabled when no cursor has been visited, and Older when `next_cursor` is null. Both are also disabled while loading (D73).
- The visited cursors are a stack, and changing the month or the view clears it.
- The 60-second poll and `ledger.reload()` after a release use the current cursor.
- Detail shows "setup · N polls" when `entry_count` is above 1, and Type shows the multiplier only when it is not null.
- The page computes no field (UI-1).

## End-to-end

`e2e/run.sh bundled` was started once (log `e2e.log`, exit code 1 in `e2e.done`). It failed after 8 s at stack start-up with
"Bind for 0.0.0.0:8081 failed: port is already allocated" (e2e/out/bundled/up.log). The port is held by
`hourglass-tls-keycloak-1`, part of a `hourglass-tls` stack that was created 2026-11-20 and has been up for 2 days. That stack also
holds 8990. I did not stop it, because it is outside my scope and not mine. `run.sh` tore its own stack down.
**Outcome: blocked by the environment, not by this change. The acceptance scenario was not exercised.** To unblock, the
orchestrator or owner stops the `hourglass-tls` stack and reruns `e2e/run.sh bundled`.

## Commands run

| Command | Outcome |
| --- | --- |
| `go test ./internal/api -run 'TestLED1[1-5]_\|TestLED2_'` (baseline and under each break) | green at baseline, red under each break |
| `npx vitest run src/pages/usage` (baseline and under each break) | 21 tests pass at baseline, red under each break |
| `npm run gen:api` | no diff |
| `./ci.sh` | exit 0. Bazel reported "Executed 0 out of 39 tests" because all results were cached for identical inputs. |
| `bazel test //... --nocache_test_results` | exit 0, **39 of 39 test targets executed and passed** |
| `e2e/run.sh bundled` | exit 1 after 8 s, port 8081 taken by a foreign stack |

## Files changed

The task folder only: PROGRESS.md, RESULT.md, e2e.log, e2e.done, ci.log, bazel-nocache.log, baseline.sha,
handlers_user.go.orig and schema.gen.ts.orig (copies kept for restoring). The scratch test and the scratch worktree were removed.

## Decisions for docs/decisions.md

None.

## Confidence and gaps

- High confidence on the API: code read, several targeted breaks, the old handler run against the fixture, and my own scratch walk test.
- The UI was verified by reading the code and through vitest only. The real browser path, including `readLedger` against the real stack, is unverified until e2e runs.
- F2 is inference from the code; I did not reproduce it.
- I did not run DeepSeek or Haiku passes; this task was a requirement-driven verification with deliberate breaks, as the brief asked.
