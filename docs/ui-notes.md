<!-- Source: Hourglass dashboard wireframes canvas, https://claude.ai/artifact/PWCrdA3nDinVkmuAsGdK74 (version 1795099020-534d). Raw artboards saved unchanged under docs/wireframes/project/. -->
# Dashboard UI notes

A written description of the seven wireframe artboards, enough to build the UI without opening the canvas. Routes come from the "Dashboard pages" section of `docs/SPEC.md`. All people, setups and numbers in the wireframes are sample values. The wireframes are low-fidelity: monochrome plus a blue accent, system-style forms, no icons. Each artboard is 1280 px wide.

Canvas layout (canvas.json): three groups. "Developer pages" holds My usage and Setups. "Admin pages" holds Users and grants, Setup inventory, Settings. "Sign-in and access" holds Sign in and Not authorized. Artboard heights range 760 to 1280 px.

## Sign in (`/login`)

Public page, no top bar. Single centered column, max width 440 px, 96 px top padding, 24 px gaps, white background.

1. Heading "Hourglass" (30 px, 600).
2. Line "Sign in to see your setups and tokens." (14 px, muted).
3. Primary link-button "Sign in with SSO" (full width, 44 px high, blue fill, white text). In the wireframe it navigates to the Not authorized page.
4. Divider with centered label "Local administrator" (12 px, muted; rule lines on both sides).
5. Local form:
   - Label "Username", text input, prefilled with `admin`.
   - Label "Password", password input, empty.
   - Secondary button "Sign in as admin" (white, grey border).
6. Footnote (12 px, muted): "The local form is only for the built-in admin account. Everyone else uses SSO. First local sign-in forces a password change."

## Not authorized (`/unauthorized`)

Public page for any signed-in user with no role. Same centered 440 px column, no top bar or navigation.

1. Small wordmark "Hourglass" (18 px, 600).
2. Heading "You are not authorized to use Hourglass" (26 px, 600).
3. "Reach out to your admin." (16 px).
4. "Signed in as morgan@example.com" (14 px, muted).
5. Secondary button "Sign out" (full width). In the wireframe it returns to Sign in.
6. Annotation (12 px, muted): "Shown to every signed-in user with no role, including rejected requests. No navigation, setup names, user names or usage appear on this page."

## My usage (`/`)

Top bar (see Shared), "My usage" active. Content: max width 1216 px, 32 px padding, 28 px between regions.

1. Title row: h1 "My usage" (26 px) left; right side a label "Month" and a select with options "November 2026" (selected) and "October 2026".
2. Four stat cards in an auto-fit grid (min 220 px, 16 px gap). Each card has a 12 px muted caption, a 28 px/600 value, and a 13 px muted sub-line.
   - "Tokens left": `142`, "of 300 allowance + 40 granted".
   - "Burning now": `2.0 tokens / h`, "setup-04 · weight 1 × 2×".
   - "Projected run-out": `Nov 21`, "at the last 24 hours' rate".
   - "Lab pressure": `82% · 2×`, then a segmented bar (12 px high, grey border, 6 px radius) with widths 50% white (free), 25% `#E5E7EB` (1x), 15% `#9CA3AF` (2x), 10% `#4B5563` (3x), and a 3 px amber marker (`#B45309`) at 82%. The bar has an accessible label: "Pressure bands: free below 50 percent, 1x to 75, 2x to 90, 3x above; currently 82 percent". Sub-line "9 of 11 setups deployed".
3. Section "Current holds · 1 of 2 allowed" (h2, 16 px). Table "Current holds", columns: Setup, Labels, State, Weight, Cost now, Held for, (blank action column).
   - Sample row: **setup-04**; label chips `size=medium` `site=sjc`; state badge "Deployed"; weight 1; "2.0 tokens / h"; "2 d 4 h"; button "Release".
   - Note (13 px, muted): "Charging continues until you release. If the setup is still deployed, release is accepted but pending: the hold shows Releasing, keeps charging, and ends at the first poll where the setup is no longer reported. Holding setup-04 overnight (15 h) at the current multiplier costs 30 tokens."
4. Two columns (auto-fit, min 360 px, 24 px gap):
   - "Tokens per day": bordered card with a placeholder bar chart (18 grey `#9CA3AF` bars, 150 px tall, baseline rule; the last bar, today, is blue `#1D4ED8`; zero days are 2 px stubs). Axis labels "Nov 1" and "Nov 18". Accessible label: bar chart of tokens debited per day this month, today highlighted.
   - "Ledger": table "Ledger", columns When, Type, Detail, Amount, Balance. Sample rows:
     - Nov 18, 17:30 | Debit · 2× | setup-04 | −0.17 | 142.0
     - Nov 18, 17:25 | Debit · 2× | setup-04 | −0.17 | 142.2
     - Nov 12, 10:02 | Grant | Riley Admin: release week | +40.00 | 213.5
     - Nov 1, 00:00 | Allowance | (blank) | +300.00 | 300.0
     - A day older than a week that storage has rolled up is one row per hold, with no multiplier in the Type cell (just "Debit") and the poll count in the Detail cell, for example Nov 3, 23:55 | Debit | lab-06 · 288 polls | −82.30 | 61.2. In the By day view the days that are not rolled up yet are merged the same way; in Every poll they show one row per poll.
     - Link below: "Show all entries, grouped by day". (Built as the view switch below; see the next bullets.)
     - Ledger paging (UI-8, design A5): above the table a two-button switch "By day" (default, requests `rollup=day&limit=20`) and "Every poll" (requests `limit=20`), the pressed one has `aria-pressed="true"`; the table shows at most 20 rows; below it the buttons "Newer" (disabled on the first page) and "Older" (disabled when `next_cursor` is null). Visited cursors are kept in the page; changing the month or the view drops them and shows the first page. The 60-second poll and the reload after a release refetch the page being shown. The Type cell shows the multiplier only when it is not null; the Detail cell shows "setup-name · N polls" when `entry_count` is above 1. Newer and Older are also disabled while a page is loading; a failed page request shows the ledger's error state; a month change by rollover, not only by the picker, returns to the first page; the table never renders more than 20 rows even if a response held more (D73).

States shown: Deployed hold. Not shown: empty state, an overdrawn balance on this page, an error for a refused release (text is in the spec).

## Setups (`/setups`)

Top bar, "Setups" active (Developer view: no admin links).

This section describes the page as changed by D93 (owner-directed, `docs/design/setups-declutter.md`, UI-15 to UI-20) and by D96 (the owner's amendment of the same day, UI-21 to UI-23), built to the UX designer's specification (D99): the page leads with cost, the weight and the holder's balance are in a details panel, there are no label columns, no per-key label dropdowns and no "Available labels" chips, and the Label selector box suggests label keys and values as you type. Labels are shown in the details panel only. The wireframe artboard `docs/wireframes/project/Setups.dc.html` is behind: it still draws the older table and filter row (see "Differences", item 15). Every number on the page is an API field and is only formatted (UI-1); the one count the page shows, "{n} setups shown", is the number of rows it draws.

1. Title row: h1 "Setups"; right "9 of 11 setups deployed · 2 free" (14 px, muted).
2. Filter row, top aligned (State, then the selector box and the Filter button, which do not wrap apart; the Filter button is offset by one label line so the three controls share a top and a bottom edge, and a message under the box moves no control):
   - "State" select, 180 px: All states, Free, Deployed, Acquired, and for an admin Disabled.
   - There is no select per label key and there are no label chips (UI-19).
   - "Label selector" box, 360 px (it narrows on a narrow window), a combobox (UI-21). Typed text is monospace 13 px. Placeholder "Type or pick a label, e.g. site=sjc": the term is the first one in `label_index` that has a `selector` and is at most 16 characters; "Type or pick a label" when none is that short; no placeholder with an empty or missing index. A down arrow at the right edge, a button named "Show label suggestions" that is not a Tab stop, opens or closes the list; it is not shown with an empty index.
   - "Filter" button: sends the box as typed (trimmed), as Enter does.
3. Suggestion list (UI-21, D97). A listbox named "Label suggestions" under the box, at least as wide as it and at most 520 px, eight options tall, then it scrolls. It opens when you type, click in the box, click the arrow or press Arrow Down; focus by Tab alone does not open it. What it holds follows the caret, in captioned groups:
   - At the start of a requirement with nothing typed: "Label keys", each key with a note of its first three values ("extralarge, large, medium", then ", …" when there are more).
   - At the start of a requirement with text typed: "Label keys" that match, then "Values": every value of any key that matches, written as the whole term (`size=large`).
   - After `!`: "Label keys". After a key and a space: "Match {key}", with `=` "is", `!=` "is not", `in (` "is one of", `notin (` "is none of". After `=`, `==` or `!=`, or inside an `in (` list: "Values of {key}"; the empty value is offered as "(empty)" with the note "empty value".
   - Matching ignores case; values and keys that start with the typed text come first. At most 50 options; a line pinned to the bottom reads "{n} more. Keep typing to narrow." Typed text that matches nothing shows "No label matches "{typed}"." or "No value of {key} matches "{typed}".".
   - A pick replaces the whole token under the caret, and focus stays in the box. Picking a key gives `key=` and opens its values; picking a value, or a whole term under "Values", applies the filter at once; picking an operator or a list item does not. No option is active until you move into the list with the arrows, so Enter on typed text submits it as before. Escape closes the list and never clears the text; Tab closes it and picks nothing. The active option has a grey background and a 3 px blue bar at its left edge. A visually hidden status line announces "{n} suggestions.", "{n} setups shown." after a filter change, a token added or removed, a refused selector and "Filters cleared."
4. Exact-match token (UI-22, D97(f) to (h)). A value with no selector term (it holds whitespace or one of `, ( ) = !`) is offered after `key=` and `key==`, and under "Values", with the note "exact match". Picking it takes the requirement out of the box and adds a token; no request carries the value, and the rows are filtered in the browser by exact match. One value per key: another value of the same key replaces the token, the same value again changes nothing. A token whose value is no longer in `label_index` stays shown and in force until removed.
5. Filters-in-force line, under the filter row, shown only while the State is not "All states", an applied selector is not empty, or a token is in force. Left to right: "Exact match:" and the tokens (each a chip reading `key=value`, cut at 40 ch, with a × button named "Remove exact-match filter key=value"); the count "1 setup shown" or "{n} setups shown" (left out when no row is shown); the link-style button "Clear filters", which sets State to All states, empties the box, applies the empty selector and removes every token. After a token's × or "Clear filters", focus goes to the selector box.
6. An invalid selector: the box gets a red border, the API's message is under it (for example "selector: expected a value at position 9"), and under that a muted line: "Forms: key=value, key!=value, key in (a,b), key notin (a,b), key, !key. Commas mean AND. The table still shows the last filter that worked."
7. Table "All setups" (UI-15), columns in order: Setup, State, Held by, "Cost (tokens / h)", (blank action header, announced as "Actions"). There is no Weight column, no Holder's tokens column and no label column, and no setting brings them back (D93(a), (e), (f); D96(a)). Every column is one line, and at 1280 px the table does not scroll sideways (UI-20); on a narrow window it scrolls inside its own scroller, not the page. Sample rows, with the lab at 82% and 2×, the highest band 3×:

   | Setup | State badge | Held by | Cost (tokens / h) | Action |
   | --- | --- | --- | --- | --- |
   | setup-01 | Deployed | Sam Lee · 5 d 2 h | 8.0 now · up to 12.0 | link "Details" |
   | setup-02 | Deployed | Priya N. · 9 d 1 h, second line "overdrawn" | 4.0 now · up to 6.0 and the tag `override` | link "Details" |
   | setup-03 | "Acquired · deploying" | Tom K. · 40 min | 4.0 now · up to 6.0, second line "Free for 1 h 20 m more" | link "Details" |
   | setup-04 | Deployed | You · 2 d 4 h | 2.0 now · up to 3.0 | link "Details", button "Release" |
   | setup-05 | Free | (blank) | 2.0 now · up to 3.0 and the tag `reported` | link "Details", primary button "Acquire" |
   | setup-06 | Free | (blank) | 2.0 now · up to 3.0 | link "Details", primary button "Acquire" |

   - **Setup:** the name in bold, cut at 32 ch with the full name in its `title`. An admin's disabled setup says "Disabled" in the State column only.
   - **Held by** (UI-17): the holder's name, or "You" in bold, then how long, joined by a middle dot (`You · 3 h`); the name is cut at 24 ch with the full name in its `title` and in the panel. When `holder.overdrawn` is true a second line reads "overdrawn" in bold amber, in words and not by colour alone. The balance itself is in the details panel. Free and disabled rows are blank.
   - **Cost (tokens / h)** (UI-16): the unit is in the header. The cell is a button in the text colour with a dotted underline (blue with a solid underline on hover): `cost_per_hour_tokens.now` and `.at_peak` as `2.0 now · up to 6.0`, the cost now in bold and "· up to 6.0" muted, or one value (the `now` value) when `now` is not below `at_peak` (one band, a weight of 0, or a band edit before the next poll), so it never reads "4.0 now · up to 3.0". With the lab at 0× it reads `0.0 now · up to 6.0`. Free rows carry no "if acquired". The button's accessible name is in words, for example "2.0 tokens per hour now, up to 6.0. Price details for setup-01". On the same line, after the button: the source tag, `reported` (blue) or `override` (grey), with the hidden sentence "The price comes from a reported weight." or "The price comes from a weight pinned by an admin." (nothing for `rule` and `default`). A second line appears only for a held setup that is not metering: the policy's reason text as before (for example "Free for 1 h 20 m more"), muted, wrapping.
   - **Action:** a "Details" link on every row (accessible name "Details for setup-01"), the only blue link on the row; then Release on your own hold, the "Releasing" badge on your own hold with a pending release (the time of the request is in the panel), or Acquire on a free setup.
   - Activating the cost button or "Details" opens the details panel (item 8). With filters that leave nothing the table says "No setups match the filters."
8. Details panel (UI-18, D94(k) to (m), D99(c), (d)). A dialog titled "Details: setup-01" with a × at the top right and a Close button in the footer (D101). Both openers open it at its top with focus on the "Price" heading; Escape, a click on the backdrop, the × or Close returns focus to the control that opened it, or to the page heading when that control is no longer on the page (for example when a filter has hidden the row). It follows the 60-second refresh. If its setup leaves the list (deleted, disabled for a developer, dropped by a filter), it stays open with the values of the last response that held the setup and shows "This setup is no longer listed. These are the last values loaded." Sections, each an h3:
   - **Price.** First the answer: Cost now, `4.0 tokens / h`; Up to, `6.0 tokens / h` and, muted, "at the highest multiplier (3×)" ("at the highest multiplier" against an older API without `pricing`). When the cost now is above the cost at the highest band, the Up to value is "The multipliers were changed. This value shows after the next poll." Charging: "Since " and a time for a metering hold ("Yes" with no start time), "Not yet: " and the reason (or "Not yet") for a hold that is not metering, "Starts: " and `metering_description` for a free setup, with no line when that is null, which it always is today (D94(e)). Then, when `reported_weights_enabled` is true, "Weights are reported by automation, so the price can change while the setup is held." Then, muted: "Cost per hour is the setup's weight times the lab's multiplier, rounded to a thousandth of a token. The multiplier follows lab pressure." Then the working: Weight, for example `1.15`, with its source in words: `rule (set by a rate rule)`, `default (the default weight)`, `reported (sent by automation)`, `override (pinned by an admin)`, or the word as sent for any other value; Attributes, `memory-gb 512 · storage-gb 4000`, only when there are any; Lab pressure, `82%`, or "not measured yet" before the first poll; Multiplier now, `2×`. The pressure and both multipliers come from the setups response's `pricing` object, not from `GET /api/v1/pressure`, so they are the values the costs were computed with; against an older API without `pricing` the pressure and multiplier lines are left out and both costs still show. There is no equation with an equals sign (D94(a)), and the word "band" is not in the panel.
   - **Holder**, only when the setup is held: Held by ("You" or the full name); Held since, a time then `(5 d 2 h)`; Balance, `61 tokens` or `−12 tokens, overdrawn` (the server marks a balance of 0 or less as overdrawn); and, when set, Release, "Release requested " and a time, then ". It completes when the setup is torn down." for any holder. The Price and Holder lists share one first-column width, so their values line up.
   - **Labels.** The only place a setup's labels are shown (D96(b)): every label, key and value in monospace, sorted by key; an empty value reads "(empty)"; "No labels." when there are none. Under the list: "Use a label in the Label selector box to filter the list." Long keys and values wrap; below 480 px the list is one column.
   - **Raw details.** The details JSON, read only, in a block labelled "Setup details JSON" that wraps long lines and does not scroll on its own; "None." when there are no details. The `admin` block is not shown; Setup inventory has it.
9. Note: labels are shown in the details panel only (D96(b)). They are found through the selector box's suggestions and filtered with the selector and exact-match tokens.
10. Acquire confirmation (wording by D101; the wireframe strings below are samples), drawn as an inline panel (grey `#F4F4F4`, bordered, 8 px radius, max 520 px) with the note "Shown as a dialog after clicking Acquire." Contents:
   - h2 "Acquire setup-05?"
   - Body: "size=medium · weight 1 (default rule). Cost now 2.0 tokens / h, up to 3.0 tokens / h at the highest multiplier (3×), free when the lab is below 50%. Charging starts when the setup is deployed or after 2 hours, whichever is first. This will be your 2nd of 2 allowed setups."
   - Buttons: primary "Acquire", secondary "Cancel".

## Users and grants (`/admin/users`)

Top bar (admin), "Users · 2 pending" active, showing "300 tokens" and "Riley Admin".

1. Title row: h1 "Users"; right a search input (260 px), aria-label "Search users", placeholder "Search name or email".
2. Four stat cards:
   - "Users": 24, "3 admins incl. local".
   - "Overdrawn": 1, "blocked from acquiring".
   - "Debited this month": 3,410, "tokens, all users".
   - "Granted this month": 240, "3 grants".
3. Section "Pending access requests · 2". Table "Pending access requests", columns: User, First signed in, Assign role, (blank), (blank). Rows:
   - **Morgan Q.** / morgan@example.com (second line 12 px muted) | Nov 18, 09:14 | role select | "Approve" | "Reject"
   - **Alex R.** / alex@other-team.example.com | Nov 17, 16:40 | role select | "Approve" | "Reject"
   - The role select options: "Choose role", "Developer", "Admin". Approve is primary (blue), Reject secondary.
   - Note: "New sign-ins have no role and see only the not-authorized page until approved here. Approving credits the monthly allowance. Rejected users drop off this list."
4. Two-column area (wrap): left "All users" (flexible, min 560 px), right the grant panel (min 300 px).
   - Table "Users", columns: User, Role, Tokens left, Burned this month, Holding, (blank action). Each row: bold name over muted email; a role select with options "Unassigned", "Developer", "Admin"; tokens; burned; holding; button "Grant". Rows:
     - Priya N. (priya@example.com) | Developer | −12 (amber, bold) | 312 | setup-02
     - Sam Lee (sam@example.com) | Developer | 61 | 239 | setup-01
     - Dana Dev (dana@example.com) | Developer | 142 | 198 | setup-04
     - Tom K. (tom@example.com) | Developer | 230 | 70 | setup-03
     - Riley Admin (riley@example.com) | Admin | 300 | 0 | (blank)
     - admin (the built-in local account), second line "Built-in local account" | badge "Admin · fixed" instead of a select | "No balance" | "—" | "Cannot hold setups" | no Grant button
   - Order (built, not in the wireframe): the API sends the local account first, then overdrawn users, then everyone by display name ignoring case; the page keeps that order and does not sort. The local account is not marked beyond its sub-line and its "Admin · fixed" badge.
   - Names: the User cell shows the person's display name (cut at 32 characters, full name in the tooltip) with the email under it (cut at 40); the signed-in admin's own row adds a muted "(you)". Pending rows, row actions, the grant panel heading, toasts, confirmations and accessible names use the same display name.
   - Note: "The built-in local account is listed first, then overdrawn users, then everyone by name. An admin cannot remove their own admin role."
   - Grant panel (aside "Grant tokens", grey, bordered; opens when Grant is clicked): h2 "Grant tokens to Priya N."; sub-line "Balance −12 · burning 4.0 tokens / h on setup-02"; label "Amount" (text, `100`); label "Reason (required)" (text, "Release week, needs a large setup for soak test"); live line "New balance: 88. Unused tokens expire Nov 30; debt carries into December."; buttons primary "Grant 100 tokens", secondary "Reset to 300", secondary "Cancel".
   - Panel note: "Opens when Grant is clicked. Negative amounts correct mistakes. Reset sets the balance to the full allowance and clears carried debt; it needs a reason too. Both are written to the audit log."

### Pre-approved users (UI-11, design D5)

A section below the two-column area, h2 "Pre-approved users", loaded on its own from `GET /api/v1/admin/preapprovals` (a failure there does not hide the rest of the page; it shows its own error and Retry).

- One-line note above the table: "Listed people get the chosen role the first time they sign in with SSO."
- Table "Pre-approved users", columns: Email, Roles ("Developer, Admin"), Source ("Added here" for `database`, "Config file" for `config`), Added by (`created_by.name`, "—" for a file row), Signed in (the date with the signed-in user's name beneath, or "Not yet"), (blank action). An email in both sources shows two rows. Empty list: "No pre-approved users."
- Database rows have a Remove button (aria-label "Remove <email>"); a confirmation dialog "Remove <email>?" with Cancel and a red Remove, then `DELETE /api/v1/admin/preapprovals/{id}`. File rows (id null) have no button.
- Add form: text area "Emails to pre-approve" (hint: separate with commas, spaces or new lines), select "Role" (Developer default, Admin), primary "Add" (disabled while the text holds no email). The page splits the text on commas, whitespace and new lines, drops empty pieces and posts `{emails, roles: [role]}`; it does no other validation. A 400 shows the server's `error.message` under the text area and keeps the text. After a successful add the text area is cleared; after an add or a remove the list is refetched.

## Setup inventory (`/admin/setups`)

Top bar (admin), "Setup inventory" active.

1. Title row: h1 "Setup inventory"; primary button "Add setup".
2. Two-column layout: left list (min 560 px), right edit panel (min 340 px).
3. Table "Setups", columns: Setup, Labels, Weight, Resource name, Last seen by provider, Held by, Status, (blank). Rows:

   | Setup | Labels | Weight | Resource name | Last seen | Held by | Status | Action |
   | --- | --- | --- | --- | --- | --- | --- | --- |
   | setup-01 | size=extralarge, site=sjc | 4 | setup-01_RP | 2 min ago | Sam Lee | Enabled | Edit |
   | setup-04 | size=medium, site=sjc | 1 | setup-04_RP | 2 min ago | Dana Dev | Enabled | Edit |
   | setup-05 | size=medium, site=sjc | 1 | setup-05_RP | Not deployed | (blank) | Enabled | Edit |
   | setup-07 | size=large, site=rtp | 2 | lab7-pool (override) | Not deployed | (blank) | Disabled | Edit |

   Status is a pill: "Enabled" green, "Disabled" grey. Labels are monospace chips. An overridden resource name is suffixed "(override)".
4. Section "Orphans". Table "Orphans", columns: Resource name reported, First seen, Matches. Row: setup-09_RP | Nov 16, 08:10 | "setup-09 exists but nobody holds it". Note: "Something is deployed that nobody holds. Orphans are charged to nobody."
5. Edit panel (aside "Edit setup", shown here for setup-04): h2 "Edit setup-04";
   - "Name" text input `setup-04`.
   - Fieldset "Labels": rows of key input (aria "Label key N") and value input (aria "Label value N") plus a "Remove" button: size=medium, site=sjc. Button "Add label". Hint: "Free-form key and value. Weight 1: no rate rule matches, so the default applies."
   - "Resource name override" text input (empty); hint "Blank uses the template: setup-04_RP".
   - "Details (JSON, stored as-is, passed to hooks)" textarea, 7 rows. Sample content: an object with keys `rack` and `mgmt_ip` (both placeholders `[YOUR FIELD]`) and `notes` = "opaque to Hourglass". Validation hint: "Valid JSON".
   - Checkbox (checked) "Enabled (counts toward lab pressure, can be acquired)".
   - Buttons: primary "Save", "Force-release", "Delete" (red text and border `#991B1B`).
   - Note: "Delete is blocked while the setup is held."

## Settings (`/admin/settings`)

Top bar (admin), "Settings" active.

1. Title row: h1 "Settings"; right the note "Changes apply from the next poll, never retroactively" and primary button "Save changes".
2. Section "Collector health": three cards.
   - "Last successful poll": `2 min ago`, "Nov 18, 17:30".
   - "Setups matched": `9`, "1 orphan".
   - "Outages this month": `1`, "35 min, not charged".
   - Read-only line below: "Provider: vsphere · Pressure source: vsphere · Metering policy: default (deployed or 2 h after acquire) · Hooks: none · OIDC issuer: [YOUR ISSUER URL]. Extensions, connection settings and secrets come from the config file and are read-only here."
3. Two columns (each min about 320 to 380 px).
   - Left:
     - "Enforcement and release": checkbox "Block acquiring at zero tokens" (unchecked), note "Off is shadow mode: usage is metered and balances fall, nobody is blocked."; checkbox "Require teardown before release" (checked), note "A release of a setup the provider still reports is accepted but pending: the hold keeps charging and is released at the first poll where the setup is gone. No effect without a provider."
     - "Allowance and timing": three inputs: "Monthly allowance (tokens)" `300`; "Max setups per developer" `2`; "Poll interval (minutes)" `5`.
     - "Resource name template": label "Template", value `{{.SetupName}}_RP`, hint "Preview: setup-04 becomes setup-04_RP".
   - Right:
     - "Pressure bands": rows, each a pair "From pressure" and "Multiplier": 0% / 0×, 50% / 1×, 75% / 2×, 90% / 3×. Button "Add band".
     - "Rate rules": intro "First matching rule sets a setup's weight: tokens per hour at 1×." Table "Rate rules", columns: #, Label selector, Weight, Matches, (blank). Rows: 1 | `size=extralarge` | 4 | "1 setup"; 2 | `size=large` | 2 | "3 setups". Selector and weight are inputs. Row actions: "Up", "Down", "Remove". Button "Add rule".
     - "Default weight" `1` (hint "7 setups match no rule"; built: "Used when no rule matches."), alone on its line. The wireframe's "Display label keys" input (`size`, hint "Shown as columns and filters on Setups") is not built (D96): the Settings page has no display label keys field, and saving sends the stored value back unchanged (UI-23). The artboard `docs/wireframes/project/AdminSettings.dc.html` is behind.
     - Footnote: "Selectors: key=value, key!=value, key in (a,b), key notin (a,b), key, !key; commas mean AND. A metering policy extension can override the weight per hold."

## Shared

### Top bar (all role pages: My usage, Setups, and the four admin pages)

Header strip, 12 px by 32 px padding, bottom border `#BDBDBD`, wraps. Left to right:

1. Wordmark "Hourglass" (18 px, 600).
2. Main nav (aria-label "Main", 20 px gaps): "My usage", "Setups". Active item is 600 weight, `#1A1A1A`, with a 2 px bottom border (aria-current="page"). Inactive items are plain text without underline.
3. Admins only: after a divider (1 px left border and 12 px padding) a small muted label "Admin" followed by "Users · 2 pending" (the pending count is part of the link text), "Setup inventory", "Settings", "Audit log". In the wireframes the Audit log link goes nowhere (`#audit`).
4. Right group: lab pressure pill "Lab 82% · 2×" (13 px, amber border `#B45309`, text `#92400E`, fully rounded); balance in bold, "142 tokens" for the developer and "300 tokens" for the admin; greeting "Hello, {display_name}" ("Hello, Dana Dev", "Hello, Riley Admin"; UI-10), where `display_name` is the session's name, else username, else email; it is cut with an ellipsis at a fixed maximum width (240 px) with the full text in its `title`, and nothing is shown when `display_name` is empty; link "Sign out".

Developer view shows only the first two nav items. The Sign in and Not authorized pages have no top bar.

### Design tokens (as used)

Colors:
- `#1A1A1A` text and active nav; `#5C5C5C` muted text, table headers, captions; `#FFFFFF` page background and secondary button fill.
- `#1D4ED8` accent: primary button fill, links, Deployed badge, today bar. `#1E3A8A` link hover.
- `#8A8A8A` control borders (inputs, selects, secondary buttons); `#BDBDBD` section and card borders, header rule, table header rule; `#E6E6E6` table row rule; `#D0D0D0` chip border; `#F4F4F4` chip and panel background.
- `#166534` green: Free and Enabled pills. `#5C5C5C` grey pill: Acquired, Disabled, "Admin · fixed".
- `#B45309` and `#92400E` amber: pressure pill border and text, pressure marker, overdrawn text (`#92400E`, 600 weight).
- `#991B1B` red: Delete button text and border only.
- `#9CA3AF`, `#E5E7EB`, `#4B5563` greys for chart bars and pressure bands.

Type: Inter (weights 400, 500, 600; fallback system-ui, sans-serif). Sizes: 12 (captions, table headers, pills, chips), 13 (labels, notes, nav pill), 14 (body, tables, controls, nav), 16 (section h2), 18 (wordmark), 26 (page h1), 28 (stat values), 30 (Sign in h1). Chips use monospace `code`.

Spacing: page max width 1216 px with 32 px padding; vertical rhythm 28 px between page regions; 24 px between columns; 16 px between cards; 12 px between controls; 6 px between a label and its input; card padding 20 px; table cell padding 10 px by 12 px; control min-height 44 px with 12 px horizontal padding (16 px for buttons).

Radii: 6 px controls and buttons; 8 px cards and panels; 4 px chips; 999 px pills.

### Showing a person (people-display rule)
- One rule everywhere: the full name, else the username, else the email; never blank. The shared helper is `ui/src/lib/person.ts` (`displayName`, `displayNameOf`, `personOptions`). A value of only spaces counts as no value.
- The UI shows the API's display value (`display_name` on session and admin users; `holder.name`, `actor.name` and similar elsewhere) and does not build a name of its own. The helper only guards against an empty value.
- Where one person is named in a sentence-like cell (Held by, the panel's holder), the viewer reads "You". In an admin list of people the viewer's row shows the display name and a muted "(you)".
- An email is shown only on admin pages that already list it (Users, pre-approvals) and never as a tooltip or sub-line on a page a developer sees.
- Table cells cut a name to one line (24 characters on Setups, 32 on admin tables) with the full name in the tooltip; the panel shows it in full.
- Lists of people (the Audit actor filter) sort by display name ignoring case; two people with the same display name read "{name} · {email}" there.

### Reusable components

- Stat card: bordered 8 px card, caption, large value, sub-line.
- Pill / badge: 12 px, 1 px border and text in one color (blue Deployed, green Free/Enabled, grey Acquired/Disabled/fixed role, amber pressure pill).
- Label chip: monospace `key=value`, grey fill and border.
- Buttons: primary (blue fill, white text), secondary (white fill, grey border), destructive (secondary with red text and border); all 44 px high.
- Data table: full width, left-aligned, 12 px muted header with bottom rule, 14 px rows with light row rules, wrapped in a horizontal scroller, aria-label per table, trailing blank-header action column.
- Side panel (aside): grey `#F4F4F4`, bordered, 8 px radius, 20 px padding, used for Acquire confirmation, Grant tokens, Edit setup.
- Form field: 13 px label (600 weight when it is a panel field) above a 44 px input; hint text 12 to 13 px muted below.
- Pressure band bar and bar-chart placeholder (My usage).
- Overdrawn marker: amber bold negative number. On Setups it is the word "overdrawn" under the holder's name (D93(f)); the negative balance is in the details panel.

All pages use accessible markup in the wireframes: labels tied to inputs, aria-label on tables and image-like charts, aria-current on the active nav link.

## Differences from the spec

Wireframe shows, spec does not ask for (or says differently):
1. Users page stat cards (Users, Overdrawn, Debited this month, Granted this month) and a "Search users" box. The spec's Users row lists neither.
2. Settings "Collector health" is shown as three cards (last poll, setups matched, outages this month with minutes not charged). The spec only says "collector health".
3. Setup inventory shows a "Held by" column and a Status (Enabled/Disabled) column; the spec lists labels, weight, resource name, enabled flag, last seen.
4. Setups page has a "Details" link and a "Cost" cell with a grace-period text such as "Free for 1 h 20 m more"; the grace-period text is not in the spec. The spec says "open details JSON", which the wireframe does not show as JSON. Since D93 "Details" is on every row and opens the details panel, which ends with the JSON, and the cost cell reads `2.0 now · up to 6.0` with no "N / h if acquired".
5. The wireframe nav shows pending count inside "Users · 2 pending" and an "Audit log" link without a page.
6. The Setups table shows weight only through the cost figure, not as its own column; the spec lists "weight" among the columns. (Reported weights added a Weight cell on Setups, item 14; since D93 the weight is again not a column, and it is shown with its source and attributes in the details panel, item 15.)
7. Settings has no hold-limit-specific section; "Max setups per developer" sits under "Allowance and timing" (the spec calls it hold limit). It also has "Poll interval (minutes)" and a "Require teardown before release" checkbox next to "Block acquiring at zero tokens"; the spec lists enforcement and the others elsewhere, and these may need cross-checking against the settings keys.
8. Local admin row on Users ("Admin · fixed", "No balance", "Cannot hold setups") and the rule "An admin cannot remove their own admin role" are shown as notes; check these against AUTH requirements.
9. Grant panel has "Reset to 300" with a reason requirement; the spec says "reset a balance" without requiring a reason.
10. My usage: "Show all entries, grouped by day" link, "Holding setup-04 overnight (15 h)" estimate, and a Month select with November/October. The spec mentions "pick a past month" and ledger history but not the overnight estimate.
11. Acquire confirmation text includes the "2 hours" grace and "your 2nd of 2 allowed setups"; the spec asks for cost at current multiplier and at the highest band.
12. Pressure is drawn as a four-band bar with a marker; the spec only says "lab pressure".
13. Added after the wireframes (owner-directed, design `docs/design/ledger-labels-onboarding.md`, D69 to D77): the ledger's By day and Every poll switch with Newer and Older paging (UI-8); the "Available labels" chips on Setups (UI-9; replaced by the selector box's suggestions, item 15); the "Hello, {display_name}" greeting in the top bar (UI-10); the "Pre-approved users" section on Users (UI-11). None is in the wireframe images; the descriptions above are written from the built pages.

14. Added after the wireframes (owner-directed, design `docs/design/reported-weights.md`, C3; UI-12 to UI-14). Every number is an API field; the helpers are in `ui/src/pages/usage/weight.tsx`.
    - Setups: as amended by D93 (UI-12 amended), the cost cell carries a small tag with the source word when it is `reported` or `override` (nothing for `rule` and `default`), and the details panel shows the weight, its source in words and one line of attributes, `memory-gb 512 · storage-gb 4000 · tier gold` (not columns, not filters). Until D93 the weight, tag and attributes were a Weight cell on the row. The Acquire dialog adds the attributes and, whenever `reported_weights_enabled` is true and whatever the setup's source, the sentence "Weights are reported by automation, so the price can change while the setup is held."
    - My usage: a hold row's Weight carries the same tag, and also for `policy`; null shows no tag. The burn line is unchanged. A ledger debit's Type cell reads `Debit · 2× · weight 1.15` (one value when `weight_min` equals `weight_max`), `Debit · 2× · weight 1.15 to 1.40` (a range, two decimals), with ` (reported)` and so on when a source is not `rule` or `default`, and nothing after the multiplier when the weight is null.
    - Setup inventory: the Weight cell adds the source tag, the line "Reported weight N" with, when the API gives a reason, "Not applied: the report is older than the maximum age." (`expired`), "... the weight is outside the minimum and maximum." (`out_of_bounds`) or "... the weight override takes its place." (`override`), and the attributes line. The setup form has a "Weight override" field: untouched is left out of the PUT body, a number sets it, a blank field sends `null` to clear it. The Enable/Disable button never sends it.
    - Settings: a "Reported weights" card (switch, minimum, maximum, maximum age, the advice that the maximum age should be at least twice the poll interval, "Last weights report" with account and times, a warning with the count of stored weights outside the limits, a warning for a refusal newer than the last accepted report). `reported_weights` goes in the PUT body only when one of its four fields changed, all four together, a blank maximum as `null`; the server's validation message is shown in the card. The collector is fetched once for the page.
    - Service accounts: the scope list has five scopes; `weights:write` is the fifth.

15. Changed after the wireframes (owner-directed, change request `docs/design/setups-declutter.md` and its amendment, D93 to D99; UI-15 to UI-23, UI-12 amended, UI-9 replaced, SET-6 retired). The Setups page leads with cost: its columns are Setup, State, Held by, Cost (tokens / h) and actions, with no Weight, Holder's tokens, Held for or label column; Held by reads `You · 3 h` with "overdrawn" in words; the cost cell reads `2.0 now · up to 6.0` as a text-coloured button with the source tag beside it; a details panel opened from the cost cell or "Details" holds the price (Cost now, Up to, Charging, then the weight, its source, the attributes, the pressure and multiplier), the holder's balance, every label and the details JSON; the per-key label dropdowns and the "Available labels" chips are gone, and the Label selector box suggests label keys and values as you type, with a value that has no selector term becoming an exact-match token on a filters-in-force line that also holds the row count and "Clear filters". The Setups section above is written from the design, not from the wireframe. **The artboards `docs/wireframes/project/Setups.dc.html` and `AdminSettings.dc.html` were not redrawn and are behind**: the first still shows the size column and dropdown, the Holder's tokens and Held for columns, and the "if acquired" cost text; the second still shows the "Display label keys" input. The Acquire dialog, My usage and the admin pages other than Settings are unchanged; Setup inventory keeps its Weight column.

Spec asks for, wireframe does not show:
1. Audit log page (`/admin/audit`), including actor and action filters. Build from the spec.
2. The Releasing state (a release requested while the setup is still deployed: a "Releasing" badge, no Release button, and a message such as "Release requested for lab-03.") and any error or empty states.
3. Projected run-out "not this month" variant.
4. Setups page: the "weight" column and a details JSON view. Since D93 the weight is not a column by owner decision; it and the details JSON are in the details panel (D98 lists the spec-source edit; it replaces D95).
5. Setup inventory: a distinct "disable" action (only an Enabled checkbox in the Edit panel); Add setup form (the panel is shown as edit only).
6. Role-change confirmations and a "forced password change" screen after first local sign-in (mentioned only in the Sign in footnote).
7. The 60-second live refresh has no visible indicator.

## Dialogs (D101)

Every dialog is one `Modal`: a header with the title and a × (accessible name "Close", 32 px, 44 px on a coarse pointer), a body that scrolls, and a footer, so the title and the buttons are always in view. The page behind does not scroll while a dialog is open and does not shift sideways.
- Close with the ×, with Escape, or, by kind, with a click on the backdrop. A read-only dialog (Details) and a confirmation (Acquire, Release) close on the backdrop like Cancel. A dialog with a form or a shown-once secret (Create or edit service account, Force release with its reason, Issue token) ignores the backdrop, so typed text and a token are never lost; the × and Escape still close it.
- A click closes only when the press and the release are both on the backdrop. A text selection dragged out of the dialog, or a press on the backdrop released inside it, does not close it.
- While a request runs (busy), Escape, the backdrop and the × do nothing.
- Focus goes to the first field, else to the safe button, and returns to the control that opened the dialog.
