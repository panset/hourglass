# UX review: names, the details modal, the local admin pin, and a holistic pass

Date 2026-11-25. Task 02, written as a senior UX designer. Read-only: no source file was changed.

**Basis.** The UI source in `ui/src` (every page, the shared components, the shell), the server name rule in
`internal/auth/session.go` and `internal/api/admin_views.go`, the Setups spec
`.orchestrator/setups-declutter/tasks/12-ux/UX-SPEC.md` (called "the Setups spec" below), and screenshots:
`e2e/out/bundled/screenshots/*.png` (last e2e run, 1280 px), `.orchestrator/setups-declutter/tasks/16-c4-e2e/screenshots/`
and `.orchestrator/setups-declutter/tasks/10-verify/browser/shots/panel-375.png`. I did not run the app. Statements about
keys, focus and screen readers are specifications, or readings of the code, not observations. "Inference" is marked as such.

**What stays fixed.** The Setups page and its panel content were just shipped. Nothing here changes the Setups table, the
selector box, the panel's sections, wording or the focus on the **Price** heading. Section A2 changes only the dialog frame
(how it closes and scrolls), which the Setups spec left "as built".

Severity: **must** (the release is not accepted without it), **should** (expected, may be argued), **polish**.
Size: **S** under an hour, **M** about half a day, **L** more.

---

## A. The three asks

### A1. How a person is shown

#### A1.1 The rule (one display value, everywhere)

1. **Display name** = the user's full name if it is not blank after trimming; else the username; else the email. It is never
   blank, never "—", never "Unknown". This is the rule the server already applies in `auth.DisplayName`
   (`internal/auth/session.go:158-167`) for the greeting, holders, actors, preapproval "Added by" and ledger actors.
2. **One source.** The UI shows the API's display value; it does not build a name of its own. Where the API today sends only
   the raw `name` (the admin users list, `AdminUser.name` = raw `u.Name`, `internal/api/admin_views.go:207`), the API should
   add a `display_name` field (additive), and the UI uses `display_name ?? (name.trim() || username || email)` so an older
   API still never yields a blank. The audit actor list already does this fallback by hand (`admin-audit/index.tsx:68`); it
   moves to the same helper.
3. **A blank stored name cannot be fixed in the UI.** The report says "Held by" shows the email. The holder name is already
   `DisplayName` (`internal/api/handlers_user.go:866`), so the email shows because the stored `name` is blank and the
   username is blank or the email (inference: I did not trace how an SSO sign-in stores `name` and `username`; task 01 owns
   that). The UI spec below is only half the fix: the stored name has to be filled from the ID token or the UserInfo
   endpoint at sign-in, as the greeting already is.
4. **Email as secondary text** only where the viewer may already see emails: admin pages. It is never added as a tooltip,
   sub-line or `title` on a page a developer sees (Setups, My usage). A developer sees an email only in the case they
   already do today: the user has neither name nor username, so the email *is* the display name.
5. **The viewer is "You".** In a cell that names one person in a sentence-like slot (Held by, holder in the panel), the
   viewer is shown as **You**, weight 600, as the Setups spec has it. In an admin list of people (Users, Audit actor), the
   viewer's own row shows their display name followed by a muted **(you)**, so the list still reads as names.
6. **Truncation.** In table cells a name is one line, cut with an ellipsis at **24 ch** on Setups (as built) and **32 ch**
   on admin tables, with the full display name in `title` (the shared `Clip`). An email sub-line is cut at **40 ch** with
   the full email in `title`. The greeting keeps its 240 px cut and `title` (`shell/TopBar.module.css:53-58`). Accessible
   names (`aria-label`) always carry the full, uncut display name.
7. **Two people with the same display name.**
   - Admin pages: the email sub-line already tells them apart in Users and Pending. In the Audit **Actor** dropdown, when two
     or more users share a display name, each of those options reads `{display name} · {email}`.
   - Developer pages: no disambiguation (no email may be added). Accepted for this release; see follow-up F9.
8. **Sorting and filtering use the shown value.** Lists of people sort by display name, case-insensitive (locale compare),
   then by id. Today `sortUsers` sorts by the raw name (`admin_views.go:236`, `sortName = strings.ToLower(u.Name)`), so every
   user without a name sorts as "" and lands at the top of the non-overdrawn block. Any text filter added later matches the
   display name, the username and (admin pages only) the email.
9. **Non-people stay as they are.** Service accounts show their account name; system events show **System** (capital S,
   today lowercase "system", `internal/api/admin_system.go:333`); a service-account actor gets a muted suffix
   **(service account)** in the Audit Actor column. Polish, not in this cut (see B-list).

#### A1.2 Every place a person is shown, and what it shows

| # | Page and place | Code | Today | Should show | Level |
| --- | --- | --- | --- | --- | --- |
| P1 | Top bar greeting | `shell/TopBar.tsx:61-65` | `Hello, {display_name}` | Unchanged (the external team confirms it works) | — |
| P2 | Setups, **Held by** cell | `pages/setups/index.tsx:251` | `holder.name` (display name, email when the stored name is blank) | **You** or display name, cut at 24 ch, `title` = full name; ` · {time held}` as built. Correct once the stored name is filled (A1.1 item 3) | must (via the data fix) |
| P3 | Setups details panel, Holder > **Held by** | `pages/setups/SetupPanel.tsx:116` | `You` or `holder.name` | **You** or the full display name (not cut; wraps) | must (via the data fix) |
| P4 | Setup inventory, **Held by** column | `pages/admin-setups/index.tsx:135` | `holder.name` | **You** when the holder is the viewer, else the display name, `Clip` 32 ch. No email (the API sends none here; not worth adding now) | should |
| P5 | My usage, Ledger **Detail** of a grant or reset | `pages/usage/index.tsx:49` | `{actor_name}: {reason}` | Display name of the admin who acted (API already uses `DisplayName`, `handlers_user.go:547`). If `actor_name` is null the Detail is the reason alone, as today | — (check only) |
| P6 | Users, **Pending access requests** row | `pages/admin-users/index.tsx:47-48` | raw `user.name` in bold, email under it | Display name in bold, email under it. A pending SSO user is exactly the user most likely to have no name, so today the bold line can be **blank** | must |
| P7 | Users, accessible names in a pending row | `admin-users/index.tsx:51,56,60` | "Roles for {name}", "Approve {name}", "Reject {name}" | Same texts with the display name; never "Approve " with nothing after it | must |
| P8 | Users, **All users** > User cell | `admin-users/index.tsx:255-256` | raw `u.name`, email or "Local account · first user" under it | Display name (+ muted **(you)** on the viewer's row), `Clip` 32 ch; under it the email (`Clip` 40 ch) or, for the local admin, **Built-in local account** (see A3) | must |
| P9 | Users, row actions and texts | `admin-users/index.tsx:78,81,117,131,138,165,301` | raw `user.name` in "Save roles for", "Grant to", panel heading "Grant tokens to {name}", toast "Granted … to {name}.", "Reset {name}'s balance?" | Display name in all of them | must |
| P10 | Users, Pre-approved users > **Added by** | `admin-users/preapprovals.tsx:68` | `created_by.name` (display name), "—" for config-file rows | Unchanged | — |
| P11 | Users, Pre-approved users > **Signed in** sub-line | `preapprovals.tsx:78` | `used_by.name` (display name) | Unchanged. The Email column already holds the email | — |
| P12 | Audit log, **Actor** filter options | `pages/admin-audit/index.tsx:66-70` | `name \|\| username \|\| email`, in the users-list order (overdrawn first) | Display name; sorted by display name, case-insensitive; `{name} · {email}` for duplicates (A1.1 item 7) | should |
| P13 | Audit log, **Actor** column | `admin-audit/index.tsx:106` | `actor.name` (display name; "system"; service account name) | Display name; **(you)** on the viewer's own entries is polish, not this cut | — |
| P14 | Not-authorized page | `pages/unauthorized/index.tsx:16` | "Signed in as {email}" | Keep the email: the person needs to know *which* account they used. Polish: "Signed in as {display name} ({email})" when the two differ | polish |
| P15 | Acquire, Release, Force-release toasts | `setups/index.tsx:181,199`, `admin-setups/SetupForm.tsx:104` | Name the setup, not a person | Unchanged | — |

People are not shown anywhere else: My usage holds and cards, Settings and Service accounts name setups or accounts only.

#### A1.3 Acceptance checks (A1)

1. With a user whose stored name is "Ada Lovelace" holding a setup, a developer's Setups **Held by** reads `Ada Lovelace · {time}`, and the panel's **Held by** reads `Ada Lovelace`. No email appears in either, nor in a `title`.
2. With a user who has no name but a username, every place in the table above shows the username; with neither, the email. No cell, heading, toast or `aria-label` is blank or ends in a space.
3. A pending SSO user with no name shows a non-blank bold line in Pending access requests; the Approve button's name is "Approve {display name}".
4. In All users, users sort by display name ignoring case (after the pinned row and the overdrawn block, see A3): a nameless user `zed@…` sorts under Z, not first.
5. The viewer's own row in All users shows "(you)" after the name; on Setups the viewer's holds read **You**.
6. A 40-character name is cut with an ellipsis in table cells, and hovering shows the full name; the panel shows it in full.
7. In the Audit Actor filter, two users both named "Sam Lee" appear as `Sam Lee · {email1}` and `Sam Lee · {email2}`.
8. Nothing specific to any one company appears in fixtures or tests: example names and `example.com` addresses only.

---

### A2. The details modal, and the rule for every modal

The fault: the whole dialog scrolls (`components/Modal.module.css:12-24`, `max-height: 90vh; overflow: auto` on the dialog), so the
footer **Close** is below the fold on any screen shorter than the panel (the 1280 × 1100 shot fits; the 375 px shot
`panel-375.png` is cut mid-panel with no button in view). There is no backdrop click (`Modal.tsx:86`, the backdrop has no
handler) and no × button.

The fix is in the shared `Modal` (`components/Modal.tsx`), so every dialog gets it; no page builds its own.

#### A2.1 Layout

- The dialog is three parts in a column: **header** (title + ×), **body** (the only part that scrolls), **footer** (when given).
- Header and footer never scroll out of view. The body is `overflow-y: auto` with `min-height: 0`; the dialog itself does
  not scroll.
- Max height: `min(90vh, 100dvh - 32px)` (the `dvh` term keeps it inside a phone's visible area with the browser bars shown).
  Max width 520 px as built. Below 552 px the dialog is the window width less 16 px each side (the backdrop's 16 px padding,
  as built).
- When the body overflows, a 1 px `--color-row-rule` rule separates it from the header and from the footer, so the reader
  sees that there is more. When it does not overflow, no rule (should; an always-on rule is acceptable if simpler).
- Header: the title (h2, as built) on the left, wrapping if long; the × on the right, top-aligned with the title's first line.
- Give the dialog the toast's shadow (it has none today and sits on a grey backdrop with a grey background) (polish).

#### A2.2 The × button

- A real `<button type="button">` in the header after the title in DOM order, showing **×** (the same glyph as the toast's
  Dismiss, `components/Alert.tsx:69`), at least 32 × 32 px (44 × 44 when `pointer: coarse`), muted colour, text colour on
  hover, the product focus ring.
- Accessible name **Close**. It is on every dialog, including confirmations, where it does what Cancel does.
- It is never the first focus unless the dialog has nothing else to focus (see A2.4).
- While the dialog is busy (a request running; `ConfirmDialog busy`, or a form dialog's `act.busy`) the × is disabled.

#### A2.3 Ways to close, per kind of dialog

A dialog is one of three kinds. The kind decides what a **backdrop click** does; Escape, × and Cancel/Close always mean "close".

| Kind | Dialogs in Hourglass today | Backdrop click | Escape | × / Cancel / Close |
| --- | --- | --- | --- | --- |
| **Read-only** | Setups **Details** (`setups/index.tsx:428-436`) | Closes | Closes | Close |
| **Confirm** (no field) | Acquire (`setups/index.tsx:438`), Reset balance (`admin-users/index.tsx:163`), Remove pre-approval (`preapprovals.tsx:135`), Delete setup (`admin-setups/SetupForm.tsx:214`), Delete service account, Revoke token (`admin-service-accounts/index.tsx:326,342`) | Closes, same as Cancel | Same as Cancel | Same as Cancel |
| **Form or secret** | Create/Edit service account (`admin-service-accounts/index.tsx:65`), Issue token (`:174`), **Token shown once** (`:146`), Force-release with its Reason field (`admin-setups/SetupForm.tsx:227-240`) | **Does nothing** | Closes (discards), as today | Closes (discards), as today |

Rules that apply to all three:

1. A backdrop click is a press **and** release both on the backdrop. A drag that starts inside the dialog (selecting text in
   the JSON block, say) and ends on the backdrop does not close it.
2. While busy, no way of closing works: backdrop, Escape and × do nothing, and Cancel is disabled (as built). Today Escape
   still cancels a busy confirmation (`Modal.tsx:52-55`), which can hide the outcome of the request.
3. Escape does nothing while an IME composition is in progress (`event.isComposing`), and a native `<select>` that is open
   takes the first Escape (browser default; nothing to build).
4. Why a form dialog ignores the backdrop even when nothing is typed: the rule is per kind, so a person learns it once, and
   a stray click beside a half-filled form never loses it. The Token-shown dialog is in this group because closing it loses
   a secret that cannot be shown again.
5. A page passes the kind to `Modal` (for example `dismissOnBackdrop` default true, set false by form dialogs). `ConfirmDialog`
   is "confirm" except where the caller puts a field in it (Force-release), which passes false.

#### A2.4 Focus

- On open, as built (`Modal.tsx:45-49`): the first field in the body; else the element marked `data-autofocus` (the
  **Price** heading on Details; **Cancel** on confirmations); else the first focusable element. Adding the × must not change
  this: Details still opens on **Price**, a confirmation still on **Cancel**, the token dialog on the token field.
- Tab cycles inside the dialog (as built). Order: ×, then the body's controls, then the footer. From the Price heading Tab
  goes to the next control in DOM order after it (the footer **Close**), Shift+Tab from there reaches the ×.
- On close by any route (backdrop, Escape, ×, Close, Cancel), focus returns to the element that opened the dialog; on
  Setups, when that row is gone, to the `h1` (as built, `fallbackFocus`, `Modal.tsx:79-80`).
- The page behind is inert to the keyboard (as built by the trap). Should: also set `inert` on the app root while open, so a
  screen reader's virtual cursor cannot wander behind the dialog; `aria-modal="true"` stays.

#### A2.5 Scroll lock

- While any dialog is open the page behind does not scroll (`overflow: hidden` on `html`), with the scrollbar's width added
  as right padding so the page does not shift sideways. On close both are restored to their earlier values (not to "").
- Touch: scrolling inside the body does not scroll the page behind once the body reaches its end (`overscroll-behavior: contain`
  on the body).

#### A2.6 Setups Details specifically

- Keeps its footer **Close** (now always visible). The × is added by the shared component. Two controls named "Close" do the
  same thing; tests should scope by role and position (`within(dialog)`, the header's button) rather than a bare name query.
- Content, order, wording and the Price focus are unchanged (Setups spec section 5).

#### A2.7 Acceptance checks (A2)

1. At 1280 × 720 open Details on a setup with 7 labels: the title, the × and the footer **Close** are all in view without
   scrolling; the body scrolls by wheel and by keyboard (Page Down with focus in the dialog).
2. At 390 × 844 the dialog is 358 px wide, the × and **Close** are in view, and nothing scrolls sideways.
3. Clicking the dark backdrop closes Details, and focus is on the **Details** button (or the cost button) that opened it.
4. Pressing Escape closes Details and focus returns to the same opener; if the row has been filtered away meanwhile, focus is on the **Setups** heading.
5. Clicking × closes Details; × has the accessible name "Close" and is reachable with Shift+Tab from the footer Close.
6. Opening Details still puts focus on the **Price** heading, not on the ×.
7. Selecting text in Raw details and releasing the mouse over the backdrop does not close the dialog.
8. While the dialog is open, the wheel over the backdrop does not scroll the Setups table; after close the page is at the scroll position it had, and its layout did not shift.
9. In **Create service account**, typing a name and clicking the backdrop leaves the dialog open with the text intact; Escape closes it.
10. In the **Token shown once** dialog, clicking the backdrop does nothing.
11. In **Acquire {setup}?**, clicking the backdrop closes it like Cancel and focus returns to that row's Acquire button; while the request runs, Escape, × and backdrop do nothing.
12. In **Force-release** with a reason typed, a backdrop click leaves the dialog and the reason as they were.

---

### A3. Users: the local admin account first

#### A3.1 Order

1. The local admin account (`is_local_admin`) is the first row of **All users**, always.
2. Then the existing order: overdrawn users first, then by display name ignoring case (A1.1 item 8, a change from the raw
   name), then id.
3. If more than one local account ever exists, they are all pinned, by id.
4. The order is made by the API (`sortUsers`, `internal/api/admin_views.go:243-260`), so every API client gets it, and the UI
   does not re-sort. **Pending access requests** is a separate list (oldest first) and is not affected; the local admin is
   never pending.
5. The Audit **Actor** dropdown does not inherit this order: it sorts by display name (P12). Inference: it reads the same
   `/api/v1/admin/users` list (`admin-audit/api.ts:26`), so without that change the pin would show up there too.

#### A3.2 How it is marked

- No new badge. The row already has two marks: the badge **Admin · fixed** in Role, and the sub-line under the name. A third
  would be noise.
- The sub-line changes from "Local account · first user" to **Built-in local account**. "first user" now reads as a claim
  about the sort order, which it is not; the built-in account is what the login page and My usage call it
  ("The built-in administrator has no balance…", `usage/index.tsx:212`).
- The note under the table (`admin-users/index.tsx:309`) changes from "Overdrawn users sort first." to **The built-in local
  account is listed first, then overdrawn users, then everyone by name. An admin cannot remove their own admin role.**
- Its empty cells read the same way: **Tokens left** "No balance" (as built), **Burned this month** "—" (today blank),
  **Holding** "Cannot hold setups" (as built) (polish, S).

#### A3.3 If sorting or filtering is added later

- Sorting by any column, in either direction, leaves the pinned row first; the sort applies to the rows below it.
- A filter is a question, the pin is only an order: a filter that the local account does not match hides it.
- The pinned row is not "selected" or highlighted; it looks like every other row.

#### A3.4 Acceptance checks (A3)

1. With users "Aaron", "admin" (local) and an overdrawn "Zoe", the order is admin, Zoe, Aaron.
2. The local admin's sub-line reads "Built-in local account"; no new badge was added; the note under the table names the order.
3. `GET /api/v1/admin/users` returns the local admin first; `?status=pending` is unchanged.
4. The Audit Actor dropdown lists users alphabetically, not with the local admin pinned.

---

## B. Holistic pass

Walked as a first-time developer (sign in, My usage, Setups, acquire, release) and as a daily admin (Users, Setup inventory,
Settings, Service accounts, Audit log). Items already covered in A are not repeated.

### B1. Must

| # | Where | What is wrong | What to do | Size |
| --- | --- | --- | --- | --- |
| B1.1 | `components/Modal.tsx:52-55` with `ConfirmDialog busy` | Escape cancels a confirmation while its request is running; the dialog goes, the outcome lands with nobody looking | Part of A2.3 rule 2: no close route while busy | S |
| B1.2 | `components/Button.module.css` (no `[aria-pressed]` style); used by My usage **By day / Every poll** (`usage/index.tsx:322-329`) | The pressed state is given to screen readers only; sighted users cannot tell which view is on (in the e2e shots the two buttons look alike except on hover) | Style `[aria-pressed="true"]`: `--color-panel` background, 1 px `--color-text` border, weight 600. Not by colour alone: the border weight and the bold text carry it | S |

### B2. Should

| # | Where | What is wrong | What to do | Size |
| --- | --- | --- | --- | --- |
| B2.1 | `admin-users/index.tsx:196-206`, `admin-setups/index.tsx:63-72`, `admin-settings/index.tsx:199-215` | One failed 60-second refresh replaces the whole page with an error, although `useApi` keeps the last data (`hooks/useApi.ts`, the catch sets `error` only). On Setup inventory this unmounts an open **Edit setup** panel and loses what was typed; on Users it closes an open Grant panel | Full-page error only when there is no data yet. With data: keep the page, show the error Alert under the title ("Could not refresh: {message}. Showing the last values loaded."), clear it on the next success | M |
| B2.2 | `admin-audit/index.tsx:84` with `useApi.ts` (a dependency change sets `loading` true) | **Next** / **Previous** or **Apply filters** replace the table with "Loading audit log…"; the button that was pressed disappears and keyboard focus falls to the page body | Show the Loading line only when there is no data yet; keep the table (and the buttons) while the next page loads, disable the pager meanwhile | S |
| B2.3 | Units: `setups` "tokens / h"; My usage holds "4.0 tokens / hour" (`usage/index.tsx:258`); Burning now "4.0 / h" (`formatPerHour`, `usage/index.tsx:171`); Acquire dialog "… / h now (2×), up to … / h at the highest band." (`setups/index.tsx:449-455`) | Four spellings of one unit, and the Acquire dialog still says "band", which the Setups spec removed from the panel (spec 5.2) | One form, the Setups spec's: **tokens / h**. Acquire body: **Cost now {now} tokens / h, up to {at_peak} tokens / h at the highest multiplier ({peak}).** then the weight line. Burning now card: `4.0 tokens / h` | S |
| B2.4 | `admin-users/index.tsx:274` | Overdrawn balance is told by amber bold text only (colour plus weight); Setups and My usage use the word "overdrawn" | Add the word: `−12 · overdrawn` (or the amber **Overdrawn** badge My usage uses) | S |
| B2.5 | `hooks/usePanelFocus.ts:28-31` (Grant panel, Edit setup panel) | Escape closes a side panel and discards what was typed, with no warning | Escape closes only when the panel's fields are unchanged; otherwise it does nothing (the Cancel button discards). Same principle as A2.3 | S |
| B2.6 | Admin success messages: inline Alert at the top of the page (`admin-users/index.tsx:215`, `admin-setups/index.tsx:80`); Setups and My usage use toasts | Below 1500 px the side panel sits under the table; after Grant or Save the confirmation appears at the page top, out of view | Use the shared toast (`useToast`) for action results on admin pages, as Setups does | M |
| B2.7 | `admin-audit/index.tsx:66-70` | Actor options come in the users-list order (overdrawn first, soon local admin first) | Covered by P12; listed so it is not lost | S |
| B2.8 | Top bar at phone width (`shell/TopBar.module.css`) | Inference (no phone shot of an admin top bar): 2 + 5 links plus pill, balance, greeting and Sign out wrap into four or more lines above every page | Follow-up: a "Menu" disclosure under 600 px. Not in this release | L |
| B2.9 | `admin-settings/index.tsx` (the "Unsaved changes" marker exists) | Leaving the page with unsaved changes loses them silently | Follow-up: a "Discard unsaved changes?" confirm on navigation | M |

### B3. Polish

| # | Where | What is wrong | What to do | Size |
| --- | --- | --- | --- | --- |
| B3.1 | `pages/not-found/index.tsx:9` | "Back to My usage" for an admin, whose home is Setups (Gate, D41) | Link to the role's home: **Back to Setups** for an admin | S |
| B3.2 | Pagers: Ledger **Newer / Older** (`usage/index.tsx:345-355`), Audit **Previous / Next** with "1 to 50 of N" (`admin-audit/index.tsx:112-122`) | Two vocabularies for one control | Keep **Newer / Older** for time-ordered lists (both are); add the "1 to 50 of N" count where the API gives a total | S |
| B3.3 | My usage holds table **Labels** column (`usage/index.tsx:228-238`) | Setups moved labels into the panel; My usage still shows chips in a column | Owner decision; leave for now | S |
| B3.4 | `internal/api/admin_system.go:331-333`, Audit Actor column | "system" lower-case; a service account looks like a person | **System**; service account name + muted "(service account)" | S |
| B3.5 | `admin-users/preapprovals.tsx:29-33` | **Add** gives no confirmation beyond the box emptying | Toast "Pre-approved {n} email(s)." | S |
| B3.6 | `admin-setups/index.tsx:25` | A failed Enable/Disable shows a red Alert inside the table cell, making the row tall | Use the toast for the error, or an Alert above the table naming the setup | S |
| B3.7 | `components/Modal.module.css:12-24` | No shadow; grey dialog on a grey backdrop | The toast's shadow (A2.1) | S |
| B3.8 | `pages/unauthorized/index.tsx:16` | Email only | P14 | S |
| B3.9 | `usage/index.tsx:289,319` | Inline `style` on two h2s repeats what `h2` already sets | Remove (no visible change) | S |

### B4. Confirmed as good (keep)

Accessible names on every row action ("Acquire lab-01", "Grant to …"); Alert roles (errors `alert`, others `status`);
`Time` with UTC in the tooltip and the zone in column headers; the focus ring on every control (`styles/tokens.css:93-96`);
the opener-focus return in `Modal` and the side panels; the Settings "Unsaved changes" marker; the one-time token warning;
the pending count in the Users nav link.

---

## C. Recommended cut

This is a patch release answering an external report. The cut is the three asks plus the small, low-risk items that touch
the same code.

### C1. Ship with this release

| Item | Why it belongs | Size |
| --- | --- | --- |
| A1: P2, P3 (via the stored-name fix, task 01), P6, P7, P8, P9; `display_name` on `AdminUser` (additive) with the UI fallback; sort by display name | The first ask | M (UI S, API S, sign-in name fix per task 01) |
| A1: P4 ("You" in Setup inventory), P12 (Actor dropdown: display name, sorted, duplicates) | Same helper, same pages, small | S |
| A2 entire: shared Modal layout, ×, backdrop by kind, busy rule (= B1.1), focus unchanged, scroll lock | The second ask; it is one component, so all dialogs get it at once | M |
| A3 entire: pin in `sortUsers`, sub-line, note, "—" in Burned this month | The third ask | S |
| B2.3 units and "band" in the Acquire dialog | One-line text changes on the page the report is about | S |
| B1.2 pressed state of By day / Every poll | CSS only, an accessibility defect | S |

### C2. Follow-ups for the owner to decide

| # | Item | Size |
| --- | --- | --- |
| F1 | B2.1 keep the page on a failed refresh (admin pages) | M |
| F2 | B2.2 audit pager keeps the table while loading | S |
| F3 | B2.5 Escape in side panels does not discard typed input | S |
| F4 | B2.6 toasts for admin action results | M |
| F5 | B2.4 the word "overdrawn" in Users | S |
| F6 | B2.8 top bar menu on phones | L |
| F7 | B2.9 unsaved-changes guard on Settings | M |
| F8 | B3 polish items | S each |
| F9 | Telling apart two developers with the same name on Setups without showing an email (for example the username when it is not an email) | M, needs a decision on what a developer may see |
| F10 | A "discard changes?" confirm on Escape in form dialogs, instead of discarding | M |
| F11 | `inert` on the app root while a dialog is open (A2.4 should) if not done with A2 | S |

F2, F3 and F5 are small and low risk; if the release has room after C1, they are the next three to take.

### C3. Questions for the owner

| # | Question | Default |
| --- | --- | --- |
| UX-Q1 | Form dialogs ignore a backdrop click even when nothing is typed (one rule per kind). Acceptable, or should a pristine form close? | Ignore always |
| UX-Q2 | Details keeps its footer **Close** next to the new ×. Drop the footer button instead? | Keep both |
| UX-Q3 | The local admin's sub-line becomes "Built-in local account". OK? | Yes |
| UX-Q4 | The Audit Actor dropdown sorts alphabetically rather than inheriting the pin. OK? | Yes |
