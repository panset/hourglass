# Result: 85 e2e-compat

Status: **partial**. The UPG tests, the end-to-end steps, the type-check and `./ci.sh` are done. `e2e/run.sh both`, `e2e/run.sh integration` and any browser run of the new steps were NOT done.

## Why the end-to-end runs were not done

The owner's compose project `hourglass-tls` is running (containers `hourglass-tls-keycloak-1` on 8081, `hourglass-tls-vcsim-1` on 8990, `hourglass-tls-hourglass-1` on 8443). The end-to-end stacks need host ports 8080, 8081, 8990, 8091 (and 9100 for integration). 8081 and 8990 are taken. `e2e/run.sh` and `deploy/compose.*.yaml`, `e2e/compose.*.yaml` have no environment variable or option for other host ports (ports are literal, and the issuer URL `keycloak.localhost:8081` is baked into the app config and the specs). I did not stop that project and did not edit port mappings. To finish: stop `hourglass-tls` (or accept a port option), then run `e2e/run.sh bundled`, `split`, `integration`.

Consequence: the new steps have never run in a browser, and neither has the existing `readLedger` (it uses the Every poll view and Older). Expect to debug them; the brief said so. They compile (`tsc --noEmit`, `playwright test --list`).

## Requirement -> test -> file

| Requirement | Test | File |
| --- | --- | --- |
| UPG-1 (database written by d96a950; app start migrates; balances, period sums, per-hold charges same after start and after compaction) | `TestUPG1_ReleaseDatabaseUpgradesAtStartAndKeepsEveryNumber` | `internal/app/upgrade_test.go` |
| UPG-1 (store level, schema 5 built by migrations) | `TestUPG1_Schema5LedgerUpgradesAndCompacts` (existing) | `internal/store/ledger_compaction_test.go` |
| UPG-2 (ledger, setups, session, SSO sign-in of a new user differ only by added fields) | `TestUPG2_ResponsesAndSignInDifferFromTheReleaseOnlyByAddedFields` (subtests ledger, setups, session, sso sign-in) | `internal/api/upgrade_compat_test.go` |
| UPG-2 (ledger, earlier) | `TestLED11_NoNewParameterAnswersAsBefore` (existing) | `internal/api/ledger_view_test.go` |
| UPG-3 (no route, parameter, request field, response code or response field of d96a950's `api/openapi.yaml` removed) | `TestUPG3_NoRouteParameterOrFieldOfTheReleaseIsRemoved` | `internal/api/upgrade_compat_test.go` |
| UPG-3 (file and handlers agree) | `TestUI2_ContractRoutesMatch`, `TestUI2_ContractResponsesMatchSchemas` (existing) | `internal/api/contract_test.go` |

Fixtures: `testdata/upgrade-d96a950/` (README there says exactly how they were produced: a worktree of d96a950, a recorder test plus a small test-env patch, both kept as `.txt`/`.patch`): `hourglass.sqlite3` (written by the release; 213 KB; `.db` is git-ignored so it is `.sqlite3`), `responses.json` (what the release answered on a copy of it, plus the recorded numbers), `openapi.yaml` (byte for byte from d96a950). The ledger response in `responses.json` equals `before_compaction` of `internal/api/testdata/led11_pre_change_responses.json`. The old database contains the local admin's password hash of the unit-test password (not a credential). Wall-clock `time` of the recorded event is ignored by the test (the release stamped it with real time).

Deliberate breaks, both red then restored: UPG-2 (JSON tag `setup_name` of a ledger-adjacent field renamed in `handlers_user.go`; failed in `/api/v1/me/ledger` comparisons) and UPG-3 (`actor_name` removed from the ledger entry schema in `api/openapi.yaml`; failed naming the 3 missing items). The tree is clean of both.

## End-to-end steps added (`e2e/browser/tests/acceptance.spec.ts`, in the one acceptance test; ids as in `steps.json`)

- A2g "the top bar greets an SSO user and the local admin by name" (UI-10): "Hello, Alice Example", "Hello, admin".
- A2f "a developer filters the setups with a label chip" (UI-9, SET-8): chip adds (size=large, 2 setups), replaces (size=medium), removes (all 6); asserts the API request carries the term.
- A2p "a pre-approved user signs in for the first time and lands on My usage with a balance" (PRE-1): admin adds dave@example.com on Users; dave signs in with SSO in a third browser context, lands on /usage with 300 tokens, never sees the not-authorized page; the list then shows the entry used.
- A7g "the admin tops up the user with 23 small grants..." (set-up, through the Users page UI): so the old month's ledger exceeds one page.
- A8p "a developer pages the ledger with Older and Newer and switches the view" (UI-8, LED-13, LED-14): page 1 has 20 rows and Newer disabled, Older, balance continuity across pages, Newer returns to the same page, switching view or month from page two returns to a first page.
- A8c "compaction through the real stack: a day with debits, more than 8 days later" (LED-7, LED-8, LED-15): reads the old month in the Every poll view, moves the test clock 9 more days, waits (up to 5 min) for a poll to compact, then requires one row per hold and day with "N polls", the same poll total, same total charged, same closing balance, no multiplier on rolled-up rows.
No existing assertion was changed; `integration.spec.ts` is untouched. Files: `e2e/browser/tests/acceptance.spec.ts`, `e2e/README.md`, `deploy/keycloak/realm-hourglass.json` (new user `dave`, password `dev-only-dave`; the only deploy file needed; `deploy/README.md` still lists three users).

## Product code changed

None.

## Files changed

`BUILD.bazel` (exports_files for the fixtures), `internal/api/BUILD.bazel`, `internal/app/BUILD.bazel`, `internal/api/testenv_test.go` (`dbCopy` option), `internal/api/upgrade_compat_test.go` (new), `internal/app/upgrade_test.go` (new), `testdata/upgrade-d96a950/*` (new), `e2e/browser/tests/acceptance.spec.ts`, `e2e/README.md`, `deploy/keycloak/realm-hourglass.json`, this folder.

## Commands and outcomes

- `./ci.sh`: exit 0 (`ci.exit`); 39 of 39 Bazel tests pass (api_test, app_test included).
- `npx tsc --noEmit` in `e2e/browser`: 0. `npx playwright test acceptance --list`: parses.
- `e2e/run.sh both`, `e2e/run.sh integration`: not run (ports). Passed-test counts: none to report.

## Decisions for docs/decisions.md

- The UPG tests read fixtures recorded from d96a950 (database, responses, contract) kept under `testdata/upgrade-d96a950/`, with the recorder committed as text, so they can be reproduced; UPG-1 starts the real `app.Run` on a copy of the database at a clock two days after the data (nothing compacted at start) and then compacts at a later date.
- UPG-3 is its own test that compares the release's contract with today's by route, parameter, request field, response code and response field (existence only); the existing contract tests keep checking file against handlers.
- New end-to-end steps live in the acceptance scenario; extra ledger rows come from admin grants made through the Users page, not the API; the compaction step moves the test clock 9 days beyond the month jump so it holds whatever the calendar date.
- New realm user `dave` is reserved for the pre-approval step.

## Confidence and gaps

- Verified: the Go tests and `./ci.sh`, the deliberate breaks, the type-check.
- Not verified: everything that needs the stack: every new step, the existing `readLedger`, selectors (for example the greeting text, `Month` option order, table row order), timing of compaction (appears to run each poll cycle per `internal/collector/collector.go`), whether session expiry after the 9-day move is handled by `openUsage` (appears so, by analogy with A8), and the timezone assumption that all lab-03 debits fall on at most two local days.
- The UPG-2 sign-in check compares events, stored user, session and audit; a pre-approved sign-in is covered by the existing PRE tests.
