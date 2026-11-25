# 07 Verify: result (2026-11-25)

## Verdict: ready with fixes

One wrong-text defect on the Setups page should be fixed before release (F1). The e2e specs will fail as they stand (F2). That is expected, since the e2e task runs next, but it has to be fixed there. Everything else is should-fix or minor. All four mutations turned tests red. CI, Go and UI suites are green.

## Findings

### Blocker
- **F1 The Acquire dialog shows the current multiplier but labels it as the highest.** `ui/src/pages/setups/index.tsx:453-454` renders "up to {at_peak} tokens / h at the highest multiplier" followed by `(${formatMultiplier(pressure.data.multiplier)})`. That is the multiplier *now*, from `GET /pressure`, not the peak. In the test fixture (`setups.test.tsx:82`, `pricing: {multiplier: 2, peak_multiplier: 3}`) the dialog reads "(2×)" while the details panel reads "(3×)" (`SetupPanel.tsx:73` uses `pricing.peak_multiplier`). The tests (`setups.test.tsx`, HOLD-1 and the B2.3 test, about diff lines 345/438) assert the wrong "(2×)", so they lock in the bug. UX-REVIEW B2.3 asks for "({peak})". **Fix:** use `data?.pricing ? \` (${formatMultiplier(data.pricing.peak_multiplier)})\` : ""`, and change the three test expectations to "(3×)". This is one line on the page the report is about, so it is a blocker for the release text, not for the architecture.

### Should-fix
- **F2 The e2e specs are not updated and will fail** (for task 06):
  - Every dialog now has two buttons named "Close": the × (`aria-label="Close"`) and the footer Close. A strict-mode `getByRole("button", {name: "Close"})` without `exact` hits both at `e2e/browser/tests/integration.spec.ts:470` and `:987` (token dialog: × plus footer Close).
  - `exact: true` does not help, because both names are exactly "Close": `acceptance.spec.ts:519`, `integration.spec.ts:301` and `:778`.
  - `acceptance.spec.ts:526` expects the old text `/Costs [\d.,]+ \/ h/`.
  - **Fix:** scope to the footer, or take `.last()` or `.nth()`; update the Acquire text regex.
- **F3 A reverse drag closes the dialog.** If you press on the backdrop, drag into the dialog and release there, the browser fires `click` on the common ancestor (the backdrop), so `e.target === e.currentTarget && pressed` holds and the dialog closes (`Modal.tsx:130-141`). A temporary jsdom test (mouseDown backdrop, mouseUp dialog, click backdrop) gave `onClose` calls = 1 (`break-ui.log`). A2.3 rule 1 says press **and** release both on the backdrop. For opted-out dialogs nothing happens, so no data is lost. **Fix:** record `onMouseUp`/`onPointerUp` target === currentTarget as well, and require both.
- **F4 The docs do not describe the new dialog behaviour, and some wording is out of date.**
  - `docs/ui-notes.md:96` still says the Details dialog has "a Close button ... Escape or Close". It does not mention the ×, the backdrop click, the per-kind rule (read-only and confirm close on the backdrop; form and secret dialogs do not), the busy rule or the scroll lock.
  - `docs/decisions.md` has no row for the modal rule; DECISIONS.md lists it as a decision. Only D100 (names and order) was added.
  - `ui-notes.md:104` (Acquire body "Costs 2.0 tokens / hour now (2×) ... at peak") and `:10` (section heading "unchanged by D93 and D96") no longer match the built text. Neither do `:40` ("2.0 / hour") and `:44` ("tokens / hour").
  - **Fix:** add a D101 row (modal kinds, ×, busy, scroll lock, Acquire wording, pressed state) and a short "Dialogs" rule in ui-notes; update the Acquire, Burning now and holds sample strings, or mark them as wireframe-only.

### Minor
- **F5 The server sort is a byte order, not a locale order.**
  - `admin_views.go:238` uses `strings.ToLower(display_name)`, so "émile" sorts after "Zoe" on Users, while the Audit dropdown (`personOptions`, `localeCompare` with base sensitivity) puts it before "Eve". UX A1.1-8 says "locale compare".
  - Fix: `golang.org/x/text/collate` or a documented ASCII limitation.
  - If two local accounts ever existed, they would sort by name rather than by id (A3.1-3; theoretical, since only one is ever created).
- **F6 A value of only spaces can still render blank.**
  - `displayNameOf("   ")` returns "" (temporary test, `break-ui.log`), and `personOptions` gives a blank option for a person whose values are all spaces.
  - `auth.DisplayName` (`internal/auth/session.go:158-167`) does not trim.
  - This cannot happen through sign-in today (`storedName`, `oidc.go:413-421`, rejects names of only spaces; the local admin is "admin"), so it is not reachable now.
  - Fix: trim in `auth.DisplayName`, so the server never sends a value of only spaces.
- **F7 Escape closes every open dialog at once.** Each Modal listens on `document` and calls `stopPropagation` (not `stopImmediatePropagation`), so one Escape closes both of two stacked dialogs (temporary test: a=1, b=1). No screen stacks two Modals today: the Grant panel's listener is on its own node and the confirm dialog is portalled. The scroll-lock ref-count across stacked dialogs restores correctly (checked).
- **F8 The × focus ring differs from the product ring.** The × uses `outline: 2px solid var(--color-text)` (`Modal.module.css:60-63`); the product ring is `--color-accent` (`styles/tokens.css:92-95`). A2.2 asks for "the product focus ring". Fix: delete the `.close:focus-visible` rule.
- **F9 The Grant panel still reads "burning 4.0 / h".** It uses `formatPerHour` (`admin-users/index.tsx:143`), not "tokens / h" (B2.3: one form). The builder flagged it as outside the C1 list.
- **F10 There is an existing flaky Go test, not caused by this change.** `TestAUTH3_CSRFRequiredOnPostNotGet` (`internal/auth/session_test.go:90`) builds a wrong token as `"00"+csrf[2:]`, which equals the real token whenever it starts with "00" (1 in 256). It failed once in my first full run and passed 5 of 5 alone and in the second full run. Fix: flip a character instead of overwriting it.
- **F11 Files to keep out of the commit.**
  - `testdata/upgrade-v1.1.0/hourglass.sqlite3-shm` and `-wal` are untracked and dated Nov 24, so they predate this job.
  - `.orchestrator/setups-declutter/PLAN.md` was modified: it has a closing note for the previous job, not part of this change.
- **F12 `inert` on the app root is not set** (A2.4 "should", F11 in UX-REVIEW). The divider rules are always shown (allowed by A2.1). Both are as the builder reported.

## 1. Acceptance checks

| Check | Code | Test | Verdict |
| --- | --- | --- | --- |
| A1-1 named holder, no email | `setups/index.tsx:252`, `SetupPanel.tsx:117` (`displayNameOf`); server `handlers_user.go:866` | setups.test "A1-1 P2 P3 ..."; Go `TestAUTH9_SetupsHolderShowsStoredNameOrFallback` | pass |
| A1-2 fallback to username or email, never blank | `lib/person.ts:17-19`; `admin_views.go:208` `DisplayName: userDisplayName(u)` | person.test x4, names.test (unnamed, neither, older API), setups.test A1-2, admin-audit A1-7; Go `TestAUTH9_AdminUsersDisplayNameAndLocalFirst` | pass (spaces-only gap F6) |
| A1-3 pending user non-blank, "Approve {name}" | `admin-users/index.tsx:49,56,60` | names.test "A1-3 P6 P7" x3 | pass |
| A1-4 sort by display name | `admin_views.go:238` | Go `TestAUTH9_AdminUsersDisplayNameAndLocalFirst` (carl@ under C); names.test keeps server order | pass (byte order, F5) |
| A1-5 "(you)" and "You" | `admin-users/index.tsx:260`; `admin-setups/index.tsx:136`; setups as built | names.test "(you)"; setups.test A1-5; admin-setups "You" | pass |
| A1-6 cut at 24/32/40 with `title` | `Clip` uses in setups, admin-users, admin-setups | setups.test A1-6, names.test x2, admin-setups P4 | pass (visual cut only in a browser) |
| A1-7 duplicates "name · email" in the Actor filter | `person.ts:31-44`, `admin-audit/index.tsx:67` | admin-audit "A1-7 two people named Sam Lee" | pass |
| A1-8 no company-specific data | fixtures use `example.com`/`.test` | grep of all added lines: no other domain, no IdP or team names; `check_core_generic.sh` green | pass |
| A2.7-1/2 header, body scroll, footer; sizes | `Modal.module.css:13-86`, `Modal.tsx:143-156` | components "A2.7-1/2" (structure only) | structure pass; sizes need e2e |
| A2.7-3 backdrop closes Details, focus back | `Modal.tsx:130-141` | components A2.7-3; setups "A2.7-3 A2.7-5" | pass (reverse drag F3) |
| A2.7-4 Escape, focus back, fallback h1 | `Modal.tsx:96-99,123-124` | components A2.7-4; setups.test:1137,1152 | pass |
| A2.7-5 × named Close, Shift+Tab reaches it | `Modal.tsx:148` | components A2.7-5; setups A2.7-5 | pass |
| A2.7-6 focus on Price or Cancel, not × | `Modal.tsx:89-93` (body first, then data-autofocus) | components A2.7-6 x2; setups.test:914 | pass |
| A2.7-7 drag-select out does not close | `Modal.tsx:132-140` | components A2.7-7; mutation-like temporary test | pass |
| A2.7-8 scroll lock, exact restore, no shift | `Modal.tsx:32-49,77-81` | components A2.7-8 x2 | lock and restore pass; wheel and layout shift need e2e |
| A2.7-9 Create service account keeps text | `admin-service-accounts/index.tsx:69` | components A2.7-9; svc test A2.7-9 | pass |
| A2.7-10 Token dialog ignores backdrop | `admin-service-accounts/index.tsx:152` | svc test A2.7-10 | pass |
| A2.7-11 Acquire: backdrop = Cancel; busy blocks all | `Modal.tsx:98,140,148`; ConfirmDialog forwards `busy` | components A2.7-11 x3; setups A2.7-11 | pass |
| A2.7-12 Force-release keeps the reason | `SetupForm.tsx:233` | admin-setups A2.7-12; components A2.7-12 | pass |
| A3-1 local, then overdrawn, then name | `admin_views.go:254-256` | Go `TestADM1_ListUsersShapeOrderAndSummary`, `TestAUTH9_*LocalFirst*` x2; names.test A3-1 | pass |
| A3-2 sub-line, no new badge, note, "—" | `admin-users/index.tsx:263,291,317` | names.test A3-2 x2 | pass |
| A3-3 API local first; pending unchanged | `sortUsers` pending branch untouched | Go tests above | pass |
| A3-4 Actor dropdown alphabetical, no pin | `personOptions` | admin-audit "A1-7 ... not pinned (A3-4)" | pass |
| B2.3 Acquire wording and units | `setups/index.tsx:453`, `usage/index.tsx:171,258` | setups B2.3, HOLD-1, usage.test | **fail on the multiplier value (F1)** |
| B1.2 pressed state | `Button.module.css:29-33` | CSS only, not testable in vitest (css:false) | needs e2e or screenshot |

## 2. Test runs (logs in this folder)
- `./ci.sh`: exit 0; 40 of 40 bazel tests passed (cached) (`ci.log`, `ci.exit`).
- `bazel test //... --nocache_test_results`: exit 0, all 40 executed and passed (`bazel-nocache.log`).
- `go test -count=1 -v ./...`, run 1: exit 1. Of 768 top-level tests, 767 passed and one failed, the flake F10 (`gotest.log`). Run 2: exit 0, all 768 passed, 35 packages ok (`gotest2.log`).
- `npx vitest run`: exit 0; 21 files and 359 tests passed (`ui.log`).

## 3. Mutations (each restored with `cp` from a backup and confirmed with `cmp`)
| Mutation | Red tests |
| --- | --- |
| (a) `users.go` back to `name = ?` | store `TestAUTH2_SignInWithoutNameKeepsStoredName`; api `TestAUTH9_SetupsHolderShowsStoredNameOrFallback` (`mut-a.log`) |
| (b) remove the `IsLocalAdmin` clause from `sortUsers` | `TestADM1_ListUsersShapeOrderAndSummary`, `TestAUTH9_AdminUsersDisplayNameAndLocalFirst`, `TestAUTH9_LocalAdminFirstWithNameBeforeAdmin` (`mut-b.log`) |
| (c) drop `dismissOnBackdrop &&` in `Modal.tsx` | 6 red: components A2.7-9 and A2.7-12; service accounts A2.7-9, Edit/Issue, A2.7-10; admin-setups A2.7-12 (`mut-c.log`) |
| (d1) `person.ts` `displayName` without the username/email fallback | 6 red: person.test x3; admin-audit "actor with no name by their email" and A1-7; names.test "older API with no display_name" (`mut-d1.log`) |
| (d2) server `DisplayName: u.Name` | `TestAUTH9_AdminUsersDisplayNameAndLocalFirst` (`mut-d2.log`) |

## 4. Break attempts
- **Drag-select from inside the dialog to the backdrop:** stays open (pass). The reverse drag closes it (F3).
- **Stacked dialogs:**
  - Scroll lock is ref-counted and restores the exact earlier `overflow` after the last close (pass; temporary test).
  - One Escape closes both dialogs (F7, not reachable today).
- **Escape while busy:** it does nothing, the backdrop does nothing, and the × is disabled (pass).
- **A name of only spaces:** the UI trims it for admin users; a holder or actor of only spaces would show blank (F6, not reachable).
- **Two users with the same name:** the Actor filter adds the email to each (pass). The server tie-breaks by id ("Same Name" then "same name" by id; Go test).
- **A local account that also has a name:** pinned whatever the name (`TestAUTH9_LocalAdminFirstWithNameBeforeAdmin` covers a user named "Aaron" sorting before "admin").
- **Case and unicode:** case is handled on both server and UI. Accents are ordered differently by the server and the UI (F5).
- **API compatibility:**
  - `name` is unchanged.
  - The openapi diff only adds `display_name` (property plus required entry); no line was removed apart from the old required list.
  - The v1.1.0 upgrade test allows the addition on the 4 admin_users routes and passes.
- **Email exposure:**
  - No email was added on Setups, My usage or the shell. Grep of added lines shows only test names containing "email".
  - The admin Setup inventory shows the API's `holder.name` (no new field).
  - "(you)" uses the session `user_id`.
- **Generic project:** no external team, IdP or company names in any added line; fixtures use example.com/.test; `check_core_generic.sh` passed.

## 5. Scope
- All source changes are within C1: A1 P2-P4, P6-P9, P12; A2; A3; B2.3; B1.2. Not done: `inert` (should) and the overflow-dependent rule (allowed).
- Outside the change: `.orchestrator/setups-declutter/PLAN.md` modified, and two untracked SQLite sidecar files from before this job (F11).
- The docs match the names and order behaviour (D100, AUTH-9 amendment, ui-notes Users and "Showing a person"). They do not cover the dialog behaviour or the new Acquire and units wording (F4).

## 6. For the e2e agent: what only a browser can check
1. At 1280×720, the Details dialog for a setup with 7 labels has its title, × and footer Close in view. The body scrolls by wheel and by Page Down; the dialog itself does not scroll.
2. At 390×844, the dialog is 358 px wide, the × and Close are visible, and there is no horizontal page scroll. The `100dvh - 32px` cap holds with the browser bars shown.
3. Scroll lock:
   - The wheel over the backdrop does not scroll the Setups table or the page.
   - After closing, the scroll position is unchanged and the page did not shift sideways: the scrollbar-width padding is applied while open and removed after.
4. A real mouse:
   - A drag-select in Raw details released on the backdrop keeps the dialog open.
   - Press on the backdrop and release inside: current code closes the dialog (F3); confirm, and decide.
5. Real focus:
   - Details opens on the Price heading.
   - Shift+Tab from the footer Close reaches the ×.
   - Focus returns to the opener after a backdrop click, Escape and the ×.
   - The Issue token dialog swaps to "Token for …" inside the same Modal instance, so the open effect does not re-run. Check where focus lands; A2.4 wants the token field. This was not checked and may be an existing gap.
6. Visuals:
   - The × hit area is 32 px, and 44 px under `pointer: coarse` (mobile emulation).
   - The focus ring on the ×: F8.
   - The dialog shadow and the header and footer rules.
   - The pressed state of By day / Every poll (B1.2) is visible without hover.
7. On Users:
   - The Clip of the name plus "(you)" sits on one line and is aligned.
   - The ellipsis at 32 and 40 characters, with the full value on hover.
   - The first row is the built-in local account.
8. Update the specs listed in F2 (two buttons named "Close", the Acquire text).

## Commands run
`git diff --stat main` before and after (identical, `diffstat.before`), `git status --short`, `./ci.sh`, `bazel test //... --nocache_test_results`, `go test -count=1 -v ./...` x2, a single-test loop x5, `npx vitest run`, five mutation runs, and one temporary vitest file (`ui/src/components/zz-verify-tmp.test.tsx`, deleted after the run; output in `break-ui.log`). No git writes.

## Confidence and gaps
- Verified by running: the test counts, the mutations, and the break cases tested in jsdom (reverse drag, stacked Escape, scroll-lock restore, busy, spaces-only, sort order).
- Inferred from the code: real browser click targeting for the reverse drag (UI Events spec: the click goes to the common ancestor), and all layout, scroll and focus behaviour in a real browser.
- I did not run e2e (per the brief) and did not open the app. I did not check every docs page beyond ui-notes, decisions and the AUTH-9 design doc.
