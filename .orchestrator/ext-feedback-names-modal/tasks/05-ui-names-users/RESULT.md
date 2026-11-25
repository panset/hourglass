# Result 05-ui-names-users: done

## What changed
- New helper `ui/src/lib/person.ts`: `displayName(person)` (display_name, else name, else username, else email; whitespace-only counts as empty, ends trimmed), `displayNameOf(apiValue)` for holder/actor values, `personOptions(users)` (sorted by display name, case-insensitive, then id; duplicates read `name · email`). No second helper added; the greeting uses the session's `display_name` directly and is untouched.
- `pages/admin-users/index.tsx`: pending rows (P6, P7), All users cell (P8: display name Clip 32, email Clip 40, "(you)" from `useSession().user_id`), row actions, panel heading, toasts, reset dialog, Role group labels (P9). Local row sub-line "Built-in local account", "—" in Burned this month, new note text (A3). No re-sorting: server order kept.
- `pages/admin-audit/index.tsx`: Actor filter uses `personOptions` (P12, sorted, duplicates with email); Actor column via `displayNameOf`.
- `pages/setups/index.tsx`, `SetupPanel.tsx`: Held by via `displayNameOf` (P2, P3); Acquire dialog now "Cost now X tokens / h, up to Y tokens / h at the highest multiplier (2×)." (B2.3).
- `pages/admin-setups/index.tsx`: Held by shows You for the viewer, else display name in Clip 32 (P4).
- `pages/usage/index.tsx`: Burning now "2.0 tokens / h", holds "tokens / h" (B2.3).
- `components/Button.module.css`: `.button[aria-pressed="true"]` style (panel background, text-colour border, weight 600) (B1.2). Outside the brief's listed paths only in that this is a components file; not Modal.
- `pages/admin-shared/fixtures.ts`: `display_name` on every admin user mock (fixes the typecheck).
- `docs/ui-notes.md`: Users section and new "Showing a person" rule.

## Test mapping (359 UI tests ran, all passed; before this task 330)
- A1-1 / P2, P3 named holder, no email: setups.test "A1-1 P2 P3 ..."
- A1-2 unnamed fallback: setups.test "A1-2 P2 P3 ..."; admin-users names.test "an unnamed pending user", "a user with neither name nor username", "an older API with no display_name"; person.test fallbacks; admin-audit "two people named Sam Lee ... falls back" and the existing no-name test
- A1-3 pending SSO user, Approve name: names.test "A1-3 P6 P7" x3
- A1-4 sort by display name: server order kept (names.test "keeps the server order"); Actor filter sort in admin-audit "A1-7 lists people by display name"; personOptions in person.test. The sort itself is the server's (task 03 tests).
- A1-5 (you) / You: names.test "(you)"; setups.test "A1-5"; admin-setups "shows the viewer's own hold as You"
- A1-6 40-char cut + title: setups.test "A1-6", names.test (name 32, email 40), admin-setups (32)
- A1-7 duplicate names in Actor filter: admin-audit "A1-7 two people named Sam Lee"
- A1-8 no company names: fixtures use example.com only
- P9 actions/toast/dialog: names.test "A1 P9"
- A3-1 order (local first, overdrawn, name): names.test "A3-1 A3-3" and admin-users.test populated page (local row first)
- A3-2 sub-line, single badge, "—", note: names.test "A3-2" x2
- A3-3 API order: backend (task 03); A3-4 Actor dropdown alphabetical, not pinned: admin-audit "A1-7 lists people ... not pinned"
- B2.3 wording: setups.test "B2.3" and HOLD-1, usage.test; B1.2 pressed state: CSS only, NOT unit-tested (vitest runs with css:false); aria-pressed toggling tests exist.

## Whitespace-only name
The UI can not receive one from the sign-in path: `storedName` (internal/auth/oidc.go) treats whitespace-only claims as empty, and the local admin's name is "admin". A name with surrounding spaces ("  Ada ") can reach `display_name`; the helper trims it for admin lists, `holder.name`/`actor.name` are shown as sent (Clip does not trim; visually harmless). `auth.DisplayName` does not trim, left alone as told.

## Raw `.email` / `.name` left outside the helper (reasons)
- unauthorized/index.tsx:16 `session.email`: the person's own email, "which account did I use" (UX P14 keeps it).
- admin-users/index.tsx:50 and :263 `user.email` / `u.email`: admin-only secondary line (A1.1 item 4).
- preapprovals.tsx:65,92,137 `p.email`: it is the email entry itself. :68,:78 `created_by.name` / `used_by.name`: server display values, spec says unchanged (P10, P11).
- service-account, setup and metering-policy `.name`: not people.

## Commands
- `npx tsc --noEmit` clean; `npx vitest run` 21 files, 359 passed (ui6.log); `./ci.sh` exit 0, all 40 tests (ci.log, ci.exit).

## Confidence and gaps
- Existing admin-users and preapprovals tests render the page without a session provider; I mock `useSession` in those two test files and in names.test (fallback to the real hook). 
- Pressed-state look and truncation widths are CSS, not checked in a browser.
- Admin grant panel still prints "burning 4.0 / h" (not in the C1 unit list).
- Audit Actor column does not show "(you)", "System" or "(service account)" (polish, outside C1).
