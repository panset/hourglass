# 01 Evaluate the three asks (read-only, 2026-11-25)

## Summary
1. Names. The server ALREADY sends a display name (name, else username, else email) for the Setups "Held by" cell: `holder.name = userDisplayName(hu)` (`internal/api/handlers_user.go:866`), shown at `ui/src/pages/setups/index.tsx:251` and `SetupPanel.tsx:116`. So "Held by" shows an email only when the holder's STORED name is empty, i.e. the holder has not signed in via SSO since D92 (4c40a4c); the name is refreshed on every SSO sign-in (`internal/store/users.go:84-90`, AUTH-2), not otherwise. Sessions idle out after 8 h (AUTH-3), so most active users refresh within a day. Two real code gaps: (a) `UpsertOIDCLogin` overwrites a stored name with "" when a sign-in yields no name (`users.go:87`), so one failed UserInfo call erases a good name; (b) the admin Users page prints the raw `name` (blank when empty) and uses it in aria-labels and messages (`ui/src/pages/admin-users/index.tsx:47,56,60,81,117,131,138,165,255,301`). Everything else that names a person already uses the shared helper. No API change is needed for any UI place.
2. Modal. `Modal` (`ui/src/components/Modal.tsx`) already has Escape, focus in, Tab trap, focus return, role=dialog, aria-modal, aria-labelledby. It lacks backdrop click, a header close button, scroll lock. The Close button is only at the bottom because `.dialog` is the scroll container (`Modal.module.css:12-23`, `overflow:auto`, `max-height:90vh`) with the footer inside it, so a tall panel scrolls title+body+footer together. Fix: make the dialog a flex column, scroll only the body, keep header (title + x) and footer fixed.
3. Users order. API-only ordering (`sortUsers`, `internal/api/admin_views.go:243-260`): overdrawn first, then lowercase name, then id. UI renders API order; the Table has no sorting and there is no pagination (`ui/src/components/Table.tsx`; `handlers_admin.go:88-103`). The local admin is `auth_source == "local"` (`auth.IsLocalAdmin`, `internal/auth/auth.go:187`; exposed as `is_local_admin`). There is exactly one (AUTH-8; `EnsureLocalAdmin` creates only `admin`, `localadmin.go:26-58`; no schema unique index, convention only). Put the pin in `sortUsers` (one comparator clause). The UI already labels the row "Local account · first user" (`admin-users/index.tsx:256`) though it is not first.
4. Tests: Go (api, store, auth), vitest (setups, admin-users, components Modal, service accounts), e2e acceptance + integration. Full `./ci.sh` about 7 s warm, 56 s uncached tests; e2e `both` about 26 min, `integration` about 14-24 min; ports 8080/8081 were held by a demo stack on 2026-11-24 (release map).
Next free ids: decisions D100 (`docs/decisions.md` last row D99); requirement ids UI-23 is last used in setups-declutter (UI-15..UI-23 per commit 6a18780; verify with grep before numbering, I saw UI-15..UI-22 text) so next about UI-24; AUTH-9 (`docs/design/ledger-labels-onboarding.md:387`) and ADM-1 (`docs/SPEC.md:269`) are the ones to amend. SPEC.md was not edited by prior jobs; they amend via design docs + decisions rows.

## 1. Names: every place a person is identified

Helper: `auth.DisplayName(store.User)` (`internal/auth/session.go:158-168`), wrapped by `userDisplayName` (`internal/api/helpers.go:210`). Greeting uses `session.display_name` (`ui/src/shell/TopBar.tsx:61-63`). There is no UI-side helper; the one client-side fallback is the Audit actor filter, `u.name || u.username || u.email` (`ui/src/pages/admin-audit/index.tsx:68`), which re-implements it.

| Where (UI) | Field | Shows today | Display name already? |
|---|---|---|---|
| Setups table "Held by" (`setups/index.tsx:247-253`) | `holder.name` (`holderJSON`, `handlers_user.go:685-866`) | "You", else display name, clipped 24ch | Yes (server) |
| Setups details panel Holder (`SetupPanel.tsx:111-116`) | `holder.name` | "You" or display name | Yes |
| Admin Setups "Held by" column (`admin-setups/index.tsx:135`), force-release form (`SetupForm.tsx:187`) | `holder.name` | display name | Yes |
| Admin Setups lookup message (`admin-setups/index.tsx:179`) | none | no person | n/a |
| My usage ledger line (`usage/index.tsx:49`) | `actor_name` (`handlers_user.go:537-550`) | display name | Yes |
| My usage holds / released-by (`user_views.go:265` `released_by.name`) | `userRefJSON.Name` | display name | Yes |
| Admin Audit list actor (`admin-audit/index.tsx:106`) | `actor.name` (`admin_system.go:319`) | display name | Yes (348c5f9) |
| Admin Audit actor filter options (`admin-audit/index.tsx:62-68`) | `/admin/users` name/username/email | client fallback | Yes, duplicated logic |
| Admin Users, All users row + pending rows + Grant panel + confirm dialogs + aria-labels (`admin-users/index.tsx:47-60,78-81,117,131,138,165,255,301`) | `AdminUser.name` (`admin_views.go:178-209`, raw `u.Name`) | raw stored name; BLANK when empty; email in sub-line | NO. Gap |
| Admin Users pre-approvals "Added by" / "Used by" (`preapprovals.tsx:68,78`) | `created_by.name` (`preapproval.go:340`) | display name | Yes |
| Top bar greeting | `display_name` | display name | Yes |
| Unauthorized page (`unauthorized/index.tsx:16`) | `session.email` | "Signed in as <email>" to the user themself | n/a (own email; AUTH-5 shape fixed) |
| Service accounts, Settings | account/setup names | no person | n/a |

Non-UI outputs: webhook/event payloads and the integration state route carry `username`, `email`, `name` raw (`pkg/hourglass/event_json.go:41,78`; `handlers_integration.go:318`, scoped to service tokens); machine contracts, keep unchanged. No CSV/export exists (`grep csv` over `internal` finds nothing). Sort keys: Users sorts on `strings.ToLower(u.Name)` (`admin_views.go:237`), so users with an empty name sort FIRST (empty string) among non-overdrawn; should use the display name (inference: affects order only for unnamed users).

Storage: holders are stored by user id (`holds.user_id`), not as email strings; names are looked up at read time, so a refreshed name shows everywhere at once. No migration.

Smallest change: none needed on the API for Held by. For Admin Users, either (a) UI-only: add a tiny `displayName(u)` in the UI (`name || username || email`) and use it in the places listed, or (b) additive API field `display_name` on `AdminUser` (server fills `userDisplayName(u)`; add to `api/openapi.yaml`, regenerate `ui/src/api/schema.gen.ts`), which keeps one source of truth (matches AUTH-9 for Session). Recommend (b), needs `AdminUser` required-list and the contract test updated. Also make `sortName` use the display name.

Privacy: `holder.name` can already be an email when no name or username exists (helpers.go:210) and is sent to every role holder on `GET /api/v1/setups` (existing behaviour; the fallback is the intent's accepted fallback). Emails as such are in admin responses (`AdminUser.email`, pre-approvals) and the user's own session only. Do not add `email` to any non-admin response.

Name lifecycle: created at first SSO sign-in with whatever name the ID token (or, since D92, UserInfo) gives (`oidc.go:270-277`, `users.go:71-78`); refreshed on every SSO sign-in by an unconditional `UPDATE ... name = ?` (`users.go:87`), including to empty. The local admin's name is its username `admin` (`users.go:109-111`). A user with no sign-in since D92 keeps the old empty name until their next sign-in (sessions last 8 h idle, AUTH-3). Recommended hardening (S): in `UpsertOIDCLogin`, do not overwrite a non-empty stored name with an empty one (`name = CASE WHEN ? = '' THEN name ELSE ? END`). This is a behaviour change to AUTH-2 "refreshed on every sign-in": amend AUTH-9 wording and add a decision row. Cannot force a refresh without a sign-in (no token available later).

## 2. Modal

`Modal.tsx`: structure `backdrop > dialog(role=dialog, aria-modal, aria-labelledby, tabIndex -1) > h2, body, footer` (`:79-89`). Has: Escape (document keydown, `:49-53`), initial focus (`:36-44`), Tab/Shift+Tab trap (`:54-72`), focus return with `fallbackFocus` (`:75-76`). Missing: backdrop click (backdrop div has no handler), header x, scroll lock (no `overflow:hidden` on body; page behind scrolls), no `aria-describedby` (optional). CSS: `.dialog {max-height:90vh; overflow:auto; display:flex; column}` (`Modal.module.css:12-23`) so all content, footer included, scrolls as one block; Close sits last. `.backdrop` z-index 40, padding 16px.
Setups details usage: `setups/index.tsx:428-436`, `footer={<Button onClick={closeDetails}>Close</Button>}`, `fallbackFocus={headingRef}`; body `SetupPanel`.
Other users of `Modal` and `ConfirmDialog`:
- `Modal` direct: `admin-service-accounts/index.tsx:65` (AccountDialog create/edit: text input, checkboxes, error state), `:146` (token shown ONCE, `close` clears the token, no recovery), `:174` (issue token form with expiry input).
- `ConfirmDialog` (Modal inside, `Modal.tsx:127`): `setups/index.tsx:438` acquire; `admin-users/index.tsx:163` reset balance; `admin-users/preapprovals.tsx:135` remove; `admin-setups/SetupForm.tsx:214` delete, `:229` force-release (has a reason input); `admin-service-accounts/index.tsx:326,342` delete/revoke.
Risk: backdrop click on the token-shown dialog loses a secret that cannot be shown again; on create/edit/issue/force-release dialogs loses typed input; a stray click while `busy` should not close. Recommendation: add a prop (for example `dismissOnBackdrop`, default true for read-only/confirm dialogs, set false for the three service-account dialogs and force-release with a reason, and ignore backdrop while a request is busy), add x button in header via the shared Modal (also gives all dialogs a top-right close; `ConfirmDialog` keeps Cancel), ignore backdrop click when mousedown started inside the dialog (drag-select out of an input), body-scroll lock, scroll only the body. Escape closes all of them today already, including the token dialog, so the token risk is existing for keyboard but new for pointer.
Existing e2e/test assumptions: `e2e/browser/tests/integration.spec.ts:248-265` closes via the Close button and checks focus return; role/name queries of dialogs (`getByRole("dialog",{name})`) stay valid. An x button named "Close" would make `getByRole("button",{name:"Close"})` ambiguous with the footer Close (strict mode); name it "Close dialog" or drop the footer Close for Details (decision for the UX review).

## 3. Users order

- Server: `sortUsers` (`admin_views.go:243-260`), called at `handlers_admin.go:107`; `status=pending` list oldest first. Active list: overdrawn first, name (lowercase raw `Name`), id. The UI shows `list.users.filter(status !== "pending")` in API order (`admin-users/index.tsx:196-198,236`). Table has no sort or pagination.
- Local admin: `store.AuthLocal` ("local"), `is_local_admin` on `AdminUser` (`admin_views.go:183,208`); role shown "Admin · fixed", "No balance", "Cannot hold setups" (`admin-users/index.tsx:259-281`). Username "admin" (`auth.LocalAdminUsername`), AUTH-8 permanent. One row only: `EnsureLocalAdmin` creates it only if `admin` does not exist, errors if a non-local user holds the name; CHECK on auth_source only (`migrations/0001_init.sql:8`), no unique constraint (inference: single by convention). Fixture `mockApi` has one (`ui/src/test/mockApi.ts:81`).
- Placement: API `sortUsers`, first clause `if x.IsLocalAdmin != y.IsLocalAdmin { return x.IsLocalAdmin }`, before overdrawn (the local admin is never overdrawn). Reason: single source, works with any UI, no pagination to break, Table is not client-sortable, and UI-7 says the UI has no business logic. Also apply to the pending path? Not needed (the local admin is never pending). A UI belt-and-braces stable partition is unnecessary. Update the note at `admin-users/index.tsx:306` ("Overdrawn users sort first") to mention the local account first. Audit actor filter takes the same order (`listActors` uses this endpoint), harmless.

## 4. Tests

Go (run by `bazel test //...`, or `go test ./internal/...`):
- Holder/display names: `internal/api` tests around setups list and usage (grep `userDisplayName`, `display_name`), `internal/api/admin_system_test.go` (audit, 348c5f9), `internal/auth/oidc_test.go`, `userinfo_test.go`, `internal/store` users tests (add: empty name does not erase).
- Users order: `internal/api` admin users tests (search `sortUsers`/`/admin/users`); add one with overdrawn user plus local admin plus name "A".
- Contract: openapi vs `ui/src/api/schema.gen.ts` (UI-2 contract test) if `display_name` is added to `AdminUser`.
UI vitest (`ui/package.json:10`, `vitest run`; also run inside bazel as 40 targets):
- `ui/src/components/components.test.tsx:94-222` Modal and ConfirmDialog (add backdrop, x, scroll-lock, no-dismiss prop, mousedown-inside cases).
- `ui/src/pages/setups/setups.test.tsx` (Held by lines 821-974, 1093; details dialog), `admin-users/admin-users.test.tsx` and `preapprovals.test.tsx`, `admin-service-accounts/admin-service-accounts.test.tsx` (token dialog must not close on backdrop), `admin-setups/admin-setups.test.tsx`, `shell/TopBar.test.tsx` (unchanged), fixtures `ui/src/test/mockApi.ts:45-81` and `admin-shared/fixtures.ts`.
e2e (`e2e/run.sh bundled|split|both|integration`): `e2e/browser/tests/acceptance.spec.ts:516` (details dialog), `:537` (Held by alice), `integration.spec.ts:248-300` (details panel check and Close), `:454-466,729,977-1017` service-account dialogs, `:1125-1134` Held by line box. Add: backdrop close and x on Details; Users first row is the local admin.
Commands and durations (from `.orchestrator/setups-declutter/tasks/02-release-map/RESULT.md`): `./ci.sh` 7 s warm, `bazel test //... --nocache_test_results` about 56 s, `e2e/run.sh both` about 26 min, `integration` about 14-24 min; ports 8080/8081 may be held by the `hourglass-demo` compose stack and need the person's OK to stop. I ran no tests.

## Proposed change list
Issue 1 (names), M, low risk:
- `internal/store/users.go` (UpsertOIDCLogin keep non-empty name) + test. S, low-medium (behaviour change to AUTH-2 wording).
- `internal/api/admin_views.go` (`display_name` on AdminUser, sortName from display name), `api/openapi.yaml`, `ui/src/api/schema.gen.ts`, tests. S.
- `ui/src/pages/admin-users/index.tsx` use `display_name` in rows, aria-labels, messages. S.
- `ui/src/pages/admin-audit/index.tsx:68` use `display_name`. S.
- Docs: decisions D100; amend AUTH-9 (and AUTH-2) in a design doc; `docs/ui-notes.md` Users section.
- No change to Held by code; add a test pinning `holder.name` fallback with an unnamed holder (probably exists, `setups.test.tsx:949`), and tell the person the cause (stale empty names until next sign-in).
Issue 2 (modal), M, medium risk (shared by 9 dialogs):
- `ui/src/components/Modal.tsx`, `Modal.module.css`: header with x, backdrop dismiss with mousedown-origin guard and opt-out prop, scroll lock, body-only scroll, keep focus behaviour. Callers: service-account dialogs opt out of backdrop dismiss (token dialog), force-release with reason. Tests in `components.test.tsx`, setups and service-accounts tests, e2e acceptance step. UX designer decides x label and whether footer Close stays.
Issue 3 (users order), S, low risk:
- `internal/api/admin_views.go` `sortUsers` clause + test; `ui/src/pages/admin-users/index.tsx:306` note; e2e one assertion.

## Confidence and gaps
- Verified by reading: all paths cited. Inferred: single local admin by convention only; users without a name sort first; UI-23/D100 numbering (grep before using). Not verified: running behaviour (no tests, no app run); whether the reporter's remaining emails are all stale-name users (cannot see their data; an IdP that sends no name even from UserInfo would also show email, by design). Did not check every API field in `openapi.yaml` for a second `name` use.
