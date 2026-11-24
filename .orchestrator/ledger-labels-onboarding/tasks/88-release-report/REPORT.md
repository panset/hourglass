# Release report: ledger roll-up and paging, label discovery, greeting, pre-approved SSO users

Branch `ledger-labels-onboarding`, HEAD `14283a888c56041250387bec784b50907c0d61ca`, cut from `main` at `d96a950`. Nothing has been merged, pushed or tagged.

## 1. Decision needed

**Say go, and name the version number.** On your go-ahead the orchestrator will fast-forward `main` to this branch, push `main`, create the tag you name and push it. The tag push starts `.github/workflows/release.yml`, which builds and pushes `ghcr.io/panset/hourglass`, `hourglass-api` and `hourglass-ui` (linux/amd64 and linux/arm64) under the tag and under `latest`. Customers who pull `latest` get this release from that moment. A tag with a hyphen (for example `v1.1.0-rc1`) is a pre-release: it is pushed under its own name and `latest` does not move. The workflow does not run tests; `./ci.sh` passed on this commit (section 4).

**The binary will not report the tag.** I built the `hourglass` image from this tree, loaded it into local Docker and ran it:

    bazel run //deploy/images:load_hourglass
    docker run --rm hourglass:dev version        ->  hourglassd 0.0.0-dev

`hourglass-api:dev` prints `hourglassd 0.0.0-dev` and `hourglass-ui:dev` prints `hourglass-ui 0.0.0-dev` the same way. The value comes from `var Version = "0.0.0-dev"` in `internal/version/version.go`, whose comment says "overridden at link time when releasing". Nothing does that. No `x_defs`, `--stamp`, `--workspace_status_command` or `ldflags` appears in any `*.bazel`, `*.bzl`, `.bazelrc`, shell script or workflow file; `deploy/images/defs.bzl` builds each binary with plain `go_binary` (no `x_defs`); `release.yml` passes the tag only to `deploy/push-images.sh` as an image tag, and the script passes it only as `--tag`. So a `v1.1.0` image will print `0.0.0-dev`, and `docs/deploying.md` (line 67) and D67 already say the binary reports the version in `internal/version`, not the tag. I did not add stamping. If you want the binary to report the tag, that is a separate change to the build (a build file and the workflow) and needs a commit before the tag is pushed; say so and it can be done first.

Other state before the first run (read-only checks, 2026-11-23):

- Tags: no `v*` tag locally (`git tag -l` empty) and none on `origin` (`git ls-remote --tags origin` empty). This is the first tagged release; the number is yours.
- `origin/main` is `d96a950`, the branch's base. `git merge-base --is-ancestor origin/main HEAD` succeeds, so the merge is a fast-forward and no history is rewritten.
- The workflow has never run: `gh run list --workflow release.yml` returns no runs (signed in as `panset`; `gh workflow list` shows "Release images" active, id 426057286). The file is already on `origin/main`.
- D67 warns that the first run may be refused until each package (`hourglass`, `hourglass-api`, `hourglass-ui`) grants this repository's Actions write access (package settings, Manage Actions access), because the packages were first pushed with a personal token. This cannot be checked from here; please confirm it or expect the first run to fail at the push and be rerun after you fix it. D65 also marks `ghcr.io/panset` as an item for your review. A new package is private until made public (`docs/deploying.md`, "Publishing the images to a registry").

## 2. What was built

**Ledger roll-up and paging (LED-7 to LED-15, UI-8, D70 to D73).** A daily job in the collector replaces the per-poll debit rows of days closed more than 7 days ago with one row per user, hold and day (same total, newest id and time, a count in the new `rollup_count` column). Migration 0006 adds the column, a one-row marker table, two partial indexes and a replacement delete trigger; the ledger stays append-only apart from this step. It works in bounded steps (at most 20 steps of at most 2,000 rows per collector cycle), releases the lock between steps and does not delay polls. `GET /api/v1/me/ledger` answers as before with no new parameter; `rollup=day`, `limit` and `cursor` give the day view and paging with running balances that stay consistent across pages, and a rolled-up row never shows a single poll multiplier. My usage shows 20 rows at a time, By day by default with an Every poll switch.

**Label discovery (SET-8, UI-9, D74).** `GET /api/v1/setups` carries `label_index` (distinct label keys and values of visible setups, sorted). The Setups page shows them as chips that add, replace or remove a selector term.

**Greeting (AUTH-9, UI-10, D75).** A user's display name is the stored name, else the given and family name claims, else the preferred username, else the email, through one shared function. The session has a new `display_name` field and the top bar shows "Hello, ...". A `name` claim behaves as before.

**Pre-approved SSO users (PRE-1 to PRE-8, UI-11, D76, D77).** Migration 0007 adds `preapprovals`. A pending SSO user whose verified `email` claim matches an entry becomes active with the entry's roles and allowance at that sign-in. Entries come from the database (admin routes and the Admin > Users section) and optionally a read-only file named by the new `preapproved_users_file` setting. Rejected users, active users and the local admin are never changed; a failed approval leaves the user pending and sign-in still succeeds. Each approval is audited.

**Compatibility (UPG-1 to UPG-3, D78).** Fixtures recorded from the `d96a950` code check that its database upgrades at start with unchanged balances and hold charges, that the ledger, setups, session and sign-in responses differ only by added fields, and that no route, parameter or field was removed. `docs/deploying.md` has the upgrade notes, data retention and "Pre-approving SSO users" (commands run, task 86b and re-run in 87); `docs/progress.md` and `README.md` are current.

Commits since `d96a950` (oldest first):

- 414b51d Design: ledger roll-up and paging, label discovery, greeting, pre-approved SSO users
- 4e8686e Record the owner decisions for ledger roll-up, label discovery, greeting and pre-approved SSO users (D69)
- 6ba2458 Ledger roll-up in storage: migration 0006, compaction queries, multiplier filter (LED-7, LED-8, LED-9, LED-15, UPG-1; D70)
- 2ea4295 Ledger roll-up in the collector: bounded compaction steps and the scale test (LED-10; D71)
- 7b06cec Ledger roll-up: fixes from the independent verification of milestones 2 and 3 (LED-7, LED-10)
- 8ea80bf Ledger API: day roll-up view, paging and running balances across pages (LED-11, LED-12, LED-13, LED-14, LED-15; D72)
- 031821f My usage: the ledger pages 20 rows at a time, by day or every poll (UI-8; D73)
- 36a93b0 Ledger API and page: fixes from the independent verification of milestones 4 and 5 (LED-13, LED-14, UI-8)
- 2b8edab Setups: label_index in the API and label chips on the page (SET-8, UI-9; D74)
- de38d61 Greeting: name fallback claims, display_name in the session, Hello in the top bar (AUTH-9, UI-10; D75)
- 873fb3d Pre-approved SSO users: migration 0007, sign-in step, admin routes, file overlay (PRE-1 to PRE-8; D76)
- b69e395 Admin > Users: the Pre-approved users section (UI-11)
- 7cc2f07 Pre-approval and greeting: fixes from the independent verification of milestones 6 and 7 (PRE-2, PRE-3, PRE-5, PRE-6, AUTH-9; D77)
- 9425bc1 Compatibility tests from the previous release's database and responses; new end-to-end steps (UPG-1, UPG-2, UPG-3; D78)
- 45b1d65 Docs: upgrade notes, data retention, pre-approving SSO users, configuration reference; spec edits listed (D79)
- b1872cc End-to-end: two acceptance steps wait for the page before reading it
- 4232458 Docs: the pre-approval commands as run against the bundled stack; progress is current
- 5b0d35b Orchestrator: brief for the final verification
- 14283a8 Final verification: progress page current, event comment, test addresses under example.test

## 3. Requirements

Each id has at least one test named after it. The table is from `.orchestrator/ledger-labels-onboarding/tasks/87-final-verify/REQUIREMENTS.md`; I spot-checked it against the tree with grep (the test function names `TestLED12_DayViewIs...`, `TestLED10_ScaleYear...`, `TestSET8_LabelTerm`, `TestAUTH9_DisplayNameRule`, `TestPRE7_UnionOfBoth...`, `TestUPG3_NoRoute...` exist in the files named, and the UI-8 to UI-11 describe blocks exist in the four UI test files).

| id | requirement (short) | tests | file |
| --- | --- | --- | --- |
| LED-7 | Due raw debits of a user/hold/period/local day become one row (total, newest id and time, count); others untouched | `TestLED7_CompactsDueRawDebitsOnly`, `TestLED7_CutoffIsLocalDateMinusSeven`, `TestLED7_DueLedgerGroupsBoundedWholeGroups`, `TestLED7_DueSearchUsesRawDebitIndexes`, `TestLED7_SpringForwardDayIsOneGroup`, `TestLED7_TimezoneChangeLeavesRolledUpRowsAlone`; e2e A8c | `internal/store/ledger_compaction_test.go` |
| LED-8 | Compaction changes no sum, order, balance cache or ledger read | `TestLED8_EveryReaderSameAfterCompaction`; e2e A8c | `internal/store/ledger_compaction_test.go` |
| LED-9 | Ledger refuses updates, replaces, non-debit deletes and deletes outside compaction; marker never left | `TestLED9_AppendOnlyExceptCompaction`, `TestLED9_MarkerEmptyAfterEveryStep` | `internal/store/ledger_compaction_test.go` |
| LED-10 | Bounded steps, lock released between steps, no poll delayed or failed, idempotent | `TestLED10_ScaleYearOfDebitsCompactsInBoundedSteps`<br>`TestLED10_BacklogFinishesOverLaterCycles`, `TestLED10_CompactionDoesNotStretchThePollInterval`, `TestLED10_FailedStepFailsNoPollAndIsRetried`, `TestLED10_IdleCheckRunsOnceADay`, `TestLED10_LongCompactionNeverWaitsPastTheInterval`, `TestLED10_NoCompactionKeepsTheFullInterval`, `TestLED10_StepsReleaseTheLockBetweenThem`<br>`TestLED10_CompactDueAgainWhenNothingDueChangesNothing`, `TestLED10_CompactDueFailedStepChangesNothingAndIsRetried`, `TestLED10_CompactDueStepsAreBounded`<br>`TestLED10_DueSearchStepIsBoundedByDays` | `internal/api/perf_compaction_scale_test.go`<br>`internal/collector/compact_led10_test.go`<br>`internal/ledger/compact_test.go`<br>`internal/store/ledger_compaction_test.go` |
| LED-11 | /me/ledger without new parameters answers as before | `TestLED11_NoNewParameterAnswersAsBefore`; e2e integration.spec.ts debitsOf (unchanged) | `internal/api/ledger_view_test.go` |
| LED-12 | rollup=day merges a hold-day, same id before/after compaction, entry_count, null multiplier | `TestLED12_DayViewIsTheSameBeforeAndAfterCompaction` | `internal/api/ledger_view_test.go` |
| LED-13 | limit/cursor paging: no duplicates, unchanged entries once, 400 on bad values | `TestLED13_PagingWalksEveryEntryOnce`, `TestLED13_PagingWithMoreActiveHoldsThanLimit`; e2e A8p | `internal/api/ledger_view_test.go` |
| LED-14 | Running balance consistent on every page; newest = period balance | `TestLED14_BalancesHoldAcrossPages`, `TestLED14_NewestBalanceInAPastPeriod`; e2e A8p | `internal/api/ledger_view_test.go` |
| LED-15 | A rolled-up row never shows a single poll multiplier | `TestLED15_CompactedRowHasNoMultiplierInDefaultView`<br>`TestLED15_RolledUpRowHasNoMultiplier` | `internal/api/ledger_view_test.go`<br>`internal/store/ledger_compaction_test.go` |
| UI-8 | Ledger: max 20 rows, By day default / Every poll, Newer/Older disabled at ends, month/view change resets | 12 tests "UI-8 ..." (describe "UI-8 Ledger paging"); e2e step A8p | `ui/src/pages/usage/usage.test.tsx` |
| UI-9 | Setups label chips: add/replace/remove term, disabled null term, placeholder from index | 14 tests "UI-9 ..."; e2e step A2f | `ui/src/pages/setups/setups.test.tsx` |
| UI-10 | Top bar "Hello, {display_name}", none when empty | 5 tests in describe "UI-10 greeting in the top bar"; e2e step A2g | `ui/src/shell/TopBar.test.tsx` |
| UI-11 | Admin > Users pre-approval list with source/use, add pasted emails, remove database entries | 16 tests under "UI-11 ..." describes; e2e step A2p (PRE-1 through the UI) | `ui/src/pages/admin-users/preapprovals.test.tsx` |
| SET-8 | label_index over visible setups, sorted, ignores selector, terms parse and match exactly | `TestSET8_DeveloperIndexHidesDisabledOnlyLabels`, `TestSET8_LabelIndexEmptyIsEmptyArray`, `TestSET8_LabelIndexIgnoresSelector`, `TestSET8_LabelIndexIsSortedAndDistinct`, `TestSET8_LabelIndexTermsMatchExactlyTheirSetups`<br>`TestSET8_LabelTerm`; e2e A2f | `internal/api/setups_label_index_test.go`<br>`internal/rules/rules_test.go` |
| AUTH-9 | Stored-name fallback claims; session display_name via shared function; no-role session adds only that field | `TestAUTH9_NoRoleSessionHoldsDisplayNameAndNothingElseNew`, `TestAUTH9_SessionAndAPIShareOneDisplayNameFunction`<br>`TestAUTH9_DisplayNameRule`, `TestAUTH9_NameClaimBehavesAsBefore`, `TestAUTH9_NextSignInFillsEmptyStoredName`, `TestAUTH9_NonStringFallbackClaimsNeverFailSignIn`, `TestAUTH9_SessionDisplayName`, `TestAUTH9_StoredNameFallbackOrder` | `internal/api/greeting_test.go`<br>`internal/auth/greeting_test.go` |
| PRE-1 | Matching pending SSO user is active with roles and allowance at that sign-in | `TestPRE1_NewUserIsActiveWithRolesAndAllowanceAtFirstSignIn`, `TestPRE1_PendingUserFromAnEarlierSignInIsApprovedAtTheNextSignIn`; e2e A2p | `internal/api/preapproval_signin_test.go` |
| PRE-2 | Email claim only, case/whitespace-insensitive (ASCII), non-empty, email_verified false never matches | `TestPRE2_ClaimWithInvisibleCharacterNeverMatches`, `TestPRE2_FoldsASCIICaseOnlyLookAlikesNeverMatch`, `TestPRE2_MatchIgnoresCaseAndSurroundingWhitespace`, `TestPRE2_NoMatchWithoutAVerifiedNonEmptyEmailClaim`<br>`TestPRE2_EmailVerifiedClaimGatesTheStep`, `TestPRE2_EmailVerifiedParsing`, `TestPRE2_OnlyANonEmptyEmailClaimIsUsed` | `internal/api/preapproval_signin_test.go`<br>`internal/auth/preapproval_test.go` |
| PRE-3 | Rejected, active and local admin never changed; delete changes no user | `TestPRE3_ApplyPreapprovalRechecksTheUserInsideItsTransaction`, `TestPRE3_ConcurrentApplyPreapprovalApprovesOnce`<br>`TestPRE3_RejectedActiveAndLocalAdminAreNeverChanged`<br>`TestPRE3_OnlyAPendingSSOUserReachesTheStep` | `internal/api/preapproval_recheck_test.go`<br>`internal/api/preapproval_signin_test.go`<br>`internal/auth/preapproval_test.go` |
| PRE-4 | One user.preapprove audit (no actor, sources), user.approved, no user.pending, first use marked | `TestPRE4_AuditEventsAndFirstUse`, `TestPRE4_NoEntriesNoFileSignInAsBefore`<br>`TestPRE4_ApprovedNewUserGetsNoPendingEvent` | `internal/api/preapproval_signin_test.go`<br>`internal/auth/preapproval_test.go` |
| PRE-5 | Admin list/add/delete routes, 1-1,000 all-or-nothing, audited, admin only | `TestPRE5_AddAcceptsOneThousandRefusesOneThousandAndOne`, `TestPRE5_AddCountsUpsertKeepsUseAndIsAudited`, `TestPRE5_AddValidatesEveryEmailAndRoleAllOrNothing`, `TestPRE5_DeleteRemovesOneDatabaseEntry`, `TestPRE5_ListShowsBothSourcesByEmail`, `TestPRE5_RoutesRefusedForEveryPrincipalButAnAdmin`<br>`TestPRE5_PreapprovalQueries` | `internal/api/preapproval_routes_test.go`<br>`internal/store/preapprovals_test.go` |
| PRE-6 | Optional file overlay, source config, invalid file stops start, never written | `TestPRE6_FileLoaderReadsYAMLAndJSON`, `TestPRE6_InvalidFileIsAnErrorNamingTheEntry`, `TestPRE6_InvalidFileStopsStartup`, `TestPRE6_NoFileStartsAsBefore`, `TestPRE6_ValidFileIsAReadOnlyOverlay`<br>`TestPRE6_NoStepSignInAsBefore`<br>`TestPRE6_PreapprovedUsersFileKey` | `internal/app/preapproved_test.go`<br>`internal/auth/preapproval_test.go`<br>`internal/config/config_test.go` |
| PRE-7 | Roles in both sources are the union | `TestPRE7_UnionOfBothSources` | `internal/api/preapproval_signin_test.go` |
| PRE-8 | Failed approval: sign-in succeeds, user stays pending | `TestPRE8_FailedApprovalTransactionLeavesTheUserPending`<br>`TestPRE8_FailedStepStillSignsInAndLeavesUserPending` | `internal/api/preapproval_signin_test.go`<br>`internal/auth/preapproval_test.go` |
| UPG-1 | d96a950 database upgrades at start; balances and hold charges unchanged | `TestUPG1_ReleaseDatabaseUpgradesAtStartAndKeepsEveryNumber`<br>`TestUPG1_Schema5LedgerUpgradesAndCompacts` | `internal/app/upgrade_test.go`<br>`internal/store/ledger_compaction_test.go` |
| UPG-2 | Ledger/setups/session responses and new SSO sign-in differ only by added fields | `TestUPG2_ResponsesAndSignInDifferFromTheReleaseOnlyByAddedFields` | `internal/api/upgrade_compat_test.go` |
| UPG-3 | Nothing of the d96a950 contract removed; contract test passes | `TestUPG3_NoRouteParameterOrFieldOfTheReleaseIsRemoved` | `internal/api/upgrade_compat_test.go` |

## 4. Checks run and their results

- `./ci.sh` on the final commit `14283a8`: exit 0 (run by me for this report, `ci.exit`, `ci.log` here). All 39 Bazel test targets pass; they were served from cache, so this run executed 0 of 39. Includes the SPDX, core-generic and licence checks, `go build`, `go vet`, gazelle diff and `bazel build //...`.
- Uncached run: `bazel test //... --nocache_test_results` in task 87 at `5b0d35b`: exit 0, 39 of 39 executed and passed. Also in 87: `go test -count=1 -v ./...` on an exact copy of HEAD (all 80 named Go tests pass), `npx vitest run` (218 of 218; 47 carry UI-8 to UI-11) and the UI typecheck. Commits after `5b0d35b` (`14283a8`) change only `docs/progress.md`, a comment in `pkg/hourglass/events.go`, a fixture README and test email addresses in a UI test.
- End-to-end (task 85b, commit `b1872cc`, the gate): `e2e/run.sh both` exit 0 with bundled 16 of 16 steps and split 16 of 16; integration scenario exit 0, 11 of 11 steps (`e2e/browser/tests/integration.spec.ts` unchanged since `d96a950`). The final full rerun at HEAD was skipped at your direction because only documentation changed after `b1872cc` (task 87 confirmed with `git diff b1872cc..HEAD --stat`: docs and orchestrator files only; the later `14283a8` adds only the comment and test-address edits noted above).
- Independent verifications by agents that did not write the code, each followed by a fix task: tasks 76 (milestones 2 and 3): 0 high, 1 medium, 4 low, fixed in 76b. Task 79 (milestones 4 and 5): 0 high, 0 medium, 6 low, fixed in 79b; its bundled e2e run was blocked by port 8081 held by your `hourglass-tls` stack. Task 84 (milestones 6 and 7, with a security read of the sign-in path): 0 high, 1 medium, 5 low, fixed in 84b. Final verification, task 87: 1 high (harness, F1), 1 medium, 4 low; the medium and three low (F2, F4, F5, F6) were fixed in `14283a8`, F1 and F3 were not (section 6). Its verdict was "not ready, narrowly" pending F1 and F2; F2 is fixed and F1 is yours to accept.
- Every deliberate break in task 87 turned a named test red except one (F3). A Haiku secret check over `d96a950..HEAD` found no live credential.

## 5. Decisions recorded (`docs/decisions.md`)

Look first at the ones that decide who gets access or what a client sees: D69(c), D76(d), D77(a), D77(b), D72(c) and D75(c).

- D69: owner-directed change: roll-up compacts the ledger after 7 days (fixed), chips not an inventory page, pre-approval in the database plus an optional read-only file, fully backward compatible. Amends LED-1.
- D70: storage of the roll-up: migration 0006 holds the ledger change, one-row marker table, two partial raw-debit indexes, 60-day search window, 2,000-row steps.
- D71: collector: at most 20 steps per cycle, order poll then prune then compaction, failed step retried and never fails a poll, scale test sizes.
- D72: ledger paging: the cursor is an id string (opaque, not tied to a view), merge in Go, one `LedgerEntry` schema with optional `entry_count` (a client's generated type name does not change), strict parsing of `limit`, `cursor` and `rollup`. **Affects what a client sees.**
- D73: My usage page choices (buttons disabled while loading, 20 rows never exceeded, helper starts on page one).
- D74: label chips: when no term is built, same-key replacement rule, 12 values and 8 keys shown.
- D75: greeting: one `auth.DisplayName`, `display_name` required in `Session` (**a client's session type gains a field**), 240 px cut.
- D76: pre-approval server details: `email_verified` absent counts as verified, a matched user gets `user.approved` and no `user.pending`, one transaction per pending sign-in. **Affects who gets access.**
- D77: hardening: ASCII-only case folding, control characters refused, the file must be one YAML document with exact keys, non-string `given_name`, `family_name` and `preferred_username` count as empty while a non-string `name` still fails sign-in as before. **Affects who gets access.**
- D78: how UPG-1 to UPG-3 are tested; new e2e steps; development realm user `dave`.
- D79: the edits `docs/SPEC.md`'s source needs; `docs/SPEC.md` was not edited.

## 6. Not done, or left as is

- **End-to-end harness risk (task 87 F1, high, not fixed).** The suite uses `http://localhost:8080` (app) and `http://localhost:8091` (test clock). Your `hourglass` and `hd` processes listen on `127.0.0.1:8080` and `127.0.0.1:8091`, Docker publishes the stacks on the IPv6 wildcard, and Node tries `::1` first and then `127.0.0.1`. If the Docker listener is missing or refuses (a container crash, an unpublished port, a step before the stack is up), test traffic, including test credentials and clock changes, silently goes to your host processes. Task 87 proved the fallback with its own listener. The gate runs did reach the stacks (container logs show requests only this branch answers, and the 9-day clock move reached the container), so the gate stands. Fixing it changes the harness after the gate, so it was left for your decision; stop `hourglass` and `hd` before the next e2e run.
- **UPG-2 fixture gap (F3, low).** Every recorded session has a non-empty `name`, so a change that put the display name in the session's `name` would pass UPG-2. The AUTH-9 tests catch that break (they went red), so the requirement is covered; the UPG-2 fixture is not complete.
- **Seed loader strictness (left alone, D77(c)).** The pre-approval file rejects a second YAML document and repeated keys; the seed loader, which has the same gaps (a second document is dropped, duplicate keys), was not changed.
- **Timing flake `TestNFR_LedgerAppendTimeIsLinear` (pre-existing).** It failed once under parallel load in task 76 (212 ms to 476 ms) and passed five of five alone; it appends debits outside the new indexes, so it appears unrelated (inference). An occasional red `./ci.sh` on a loaded machine is possible. Left unedited.
- **UI lint not run.** `ui/package.json` has no lint script (task 87); the UI typecheck passes instead.
- **`docs/SPEC.md` not edited.** D79 lists the edits its source needs.
- **Non-string `name` claim.** A `name` claim that is not a string still fails the sign-in, exactly as in the previous release (D77(d)). Kept for compatibility.
- **Version stamping absent.** The binary reports `0.0.0-dev` whatever the tag (section 1).
- **First start on a large ledger can pause.** Migration 0006 builds two indexes over every per-poll debit row; the time was not measured on a large ledger (`docs/deploying.md`). There is no down migration; an older binary refuses an upgraded database and compacted rows cannot be expanded. The way back is the pre-upgrade backup.
- **Timing under load.** Task 76 M1: the collector has no time budget for compaction; a backlog on slow storage lengthens the cycle by up to 20 steps. The time spent compacting is deducted from the wait before the next poll (D71(a)), which addressed it.
- **Bundled e2e at HEAD and the full `e2e/run.sh both` rerun were not done** (your direction). Bundled-mode container evidence is from task 87's partial run through step A5, not from 85b's bundled run, whose container log was overwritten.
- **Open points for you (design, with the defaults as built):**
  - Version number and who merges and pushes: stopped, awaiting you.
  - My usage By day / Every poll switch: built, By day is the default.
  - A pre-approval may grant `admin`: allowed and audited.
  - The file is read at start only: a change needs a restart.
  - An absent `email_verified` counts as verified.
  - Matching uses the `email` claim only; a provider that does not release it leaves its users pending.
  - Adding an entry for an already pending user does not approve them until their next sign-in.
  - An entry is not consumed, so a second identity with the same email is approved too (each approval is audited).
  - The cause of the missing name was inferred from the code and not confirmed against the customer's identity provider; if it is still missing, check which claims the provider sends.
- Task 87's other findings (F4, F5, F6) were fixed in `14283a8`.

## 7. After the go-ahead

1. Orchestrator confirms the tree is clean, HEAD is the commit reported here, and `origin/main` is still `d96a950`.
2. Fast-forward merge of `ledger-labels-onboarding` into `main` (`git merge --ff-only`), then `git push origin main`.
3. Create the tag you name on that commit (annotated) and `git push origin <tag>`. History is never rewritten.
4. Watch the "Release images" workflow with `gh run list --workflow release.yml` and `gh run watch`. Success means the run finishes with conclusion success; then `gh`/`docker manifest inspect` of `ghcr.io/panset/hourglass:<tag>` and `:latest` (not for a pre-release) confirms both platforms are there.
5. If the run fails (see the D67 package-access note), stop and report the failing step; do not move or delete the tag without your word.
6. Report back with the tag, the run link, the digests and the version the image reports (`docker run --rm ghcr.io/panset/hourglass:<tag> version`, expected `hourglassd 0.0.0-dev` unless stamping is added first).
