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
   - "Burning now": `2.0 / hour`, "setup-04 · weight 1 × 2×".
   - "Projected run-out": `Nov 21`, "at the last 24 hours' rate".
   - "Lab pressure": `82% · 2×`, then a segmented bar (12 px high, grey border, 6 px radius) with widths 50% white (free), 25% `#E5E7EB` (1x), 15% `#9CA3AF` (2x), 10% `#4B5563` (3x), and a 3 px amber marker (`#B45309`) at 82%. The bar has an accessible label: "Pressure bands: free below 50 percent, 1x to 75, 2x to 90, 3x above; currently 82 percent". Sub-line "9 of 11 setups deployed".
3. Section "Current holds · 1 of 2 allowed" (h2, 16 px). Table "Current holds", columns: Setup, Labels, State, Weight, Cost now, Held for, (blank action column).
   - Sample row: **setup-04**; label chips `size=medium` `site=sjc`; state badge "Deployed"; weight 1; "2.0 tokens / hour"; "2 d 4 h"; button "Release".
   - Note (13 px, muted): "Charging continues until you release. If the setup is still deployed, release is accepted but pending: the hold shows Releasing, keeps charging, and ends at the first poll where the setup is no longer reported. Holding setup-04 overnight (15 h) at the current multiplier costs 30 tokens."
4. Two columns (auto-fit, min 360 px, 24 px gap):
   - "Tokens per day": bordered card with a placeholder bar chart (18 grey `#9CA3AF` bars, 150 px tall, baseline rule; the last bar, today, is blue `#1D4ED8`; zero days are 2 px stubs). Axis labels "Nov 1" and "Nov 18". Accessible label: bar chart of tokens debited per day this month, today highlighted.
   - "Ledger": table "Ledger", columns When, Type, Detail, Amount, Balance. Sample rows:
     - Nov 18, 17:30 | Debit · 2× | setup-04 | −0.17 | 142.0
     - Nov 18, 17:25 | Debit · 2× | setup-04 | −0.17 | 142.2
     - Nov 12, 10:02 | Grant | Riley Admin: release week | +40.00 | 213.5
     - Nov 1, 00:00 | Allowance | (blank) | +300.00 | 300.0
     - Link below: "Show all entries, grouped by day". (Built as the view switch below; see the next bullets.)
     - Ledger paging (UI-8, design A5): above the table a two-button switch "By day" (default, requests `rollup=day&limit=20`) and "Every poll" (requests `limit=20`), the pressed one has `aria-pressed="true"`; the table shows at most 20 rows; below it the buttons "Newer" (disabled on the first page) and "Older" (disabled when `next_cursor` is null). Visited cursors are kept in the page; changing the month or the view drops them and shows the first page. The 60-second poll and the reload after a release refetch the page being shown. The Type cell shows the multiplier only when it is not null; the Detail cell shows "setup-name · N polls" when `entry_count` is above 1.

States shown: Deployed hold. Not shown: empty state, an overdrawn balance on this page, an error for a refused release (text is in the spec).

## Setups (`/setups`)

Top bar, "Setups" active (Developer view: no admin links).

1. Title row: h1 "Setups"; right "9 of 11 setups deployed · 2 free" (14 px, muted).
2. Filter row (wrap, 12 px gap, bottom aligned):
   - "State" select, 180 px: All states, Free, Deployed, Acquired.
   - "size" select, 160 px: Any, medium, large, extralarge.
   - "Label selector" text input, 260 px, placeholder from `label_index`: "e.g. " plus the first non-null `selector` term of the index (for example "e.g. site=sjc"). With an empty index the placeholder is omitted.
   - "Available labels" block under the selector box (UI-9, design B2). One row per key of `label_index` (the API sorts keys and values), each value a chip: a button with the accessible name "Filter by site=sjc", reachable with Tab and activated with Enter or Space. Activating a chip puts the term the API sent (`label_index[].values[].selector`) into the selector box and applies the filter: if the box already holds exactly that term, the term is removed (the chip toggles); else if the box holds another plain `key=value` term for the same key, that term is replaced; else the term is appended, joined with a comma. The page splits the box on commas outside parentheses and compares whole terms, so `site in (a,b)` is one term and is never replaced by a `site=` chip. A chip whose term is in the applied selector (not just typed) is selected (`aria-pressed="true"`). A value with a null `selector` is a struck-through chip with `aria-disabled="true"`, a title and a note below the block saying it cannot be used in a selector; activating it does nothing. A key shows its first 12 values and a "+N more" button for the rest (then "Show fewer"). With more than 8 keys the block starts collapsed behind a "Show labels" button (`aria-expanded`). With an empty index the block says "No setup has labels." The two numbers (12 values, 8 keys) are `VALUES_SHOWN` and `KEYS_BEFORE_COLLAPSE` in `ui/src/pages/setups/LabelChips.tsx`.
3. Table "All setups", columns: Setup, size, State, Held by, Holder's tokens, Held for, Cost, (blank action). Rows:

   | Setup | size | State badge | Held by | Holder's tokens | Held for | Cost | Action |
   | --- | --- | --- | --- | --- | --- | --- | --- |
   | setup-01 | extralarge | Deployed | Sam Lee | 61 | 5 d 2 h | 8.0 / h | link "Details" |
   | setup-02 | large | Deployed | Priya N. | "−12 overdrawn" (amber, bold) | 9 d 1 h | 4.0 / h | link "Details" |
   | setup-03 | large | "Acquired · deploying" | Tom K. | 230 | 40 min | "Free for 1 h 20 m more" | link "Details" |
   | setup-04 | medium | Deployed | You | 142 | 2 d 4 h | 2.0 / h | button "Release" |
   | setup-05 | medium | Free | (blank) | (blank) | (blank) | "2.0 / h if acquired" | primary button "Acquire" |
   | setup-06 | medium | Free | (blank) | (blank) | (blank) | "2.0 / h if acquired" | primary button "Acquire" |

   The Release button appears only on your own hold, Acquire only on free setups, "Details" on others' holds.
4. Note: "The size column and filter appear because an admin listed size as a display label. The core has no built-in columns for labels."
5. Acquire confirmation, drawn as an inline panel (grey `#F4F4F4`, bordered, 8 px radius, max 520 px) with the note "Shown as a dialog after clicking Acquire." Contents:
   - h2 "Acquire setup-05?"
   - Body: "size=medium · weight 1 (default rule). Costs 2.0 tokens / hour now (2×), up to 3.0 at peak, free when the lab is below 50%. Charging starts when the setup is deployed or after 2 hours, whichever is first. This will be your 2nd of 2 allowed setups."
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
     - admin, second line "Local account · first user" | badge "Admin · fixed" instead of a select | "No balance" | (blank) | "Cannot hold setups" | no Grant button
   - Note: "Overdrawn users sort first. An admin cannot remove their own admin role."
   - Grant panel (aside "Grant tokens", grey, bordered; opens when Grant is clicked): h2 "Grant tokens to Priya N."; sub-line "Balance −12 · burning 4.0 / hour on setup-02"; label "Amount" (text, `100`); label "Reason (required)" (text, "Release week, needs a large setup for soak test"); live line "New balance: 88. Unused tokens expire Nov 30; debt carries into December."; buttons primary "Grant 100 tokens", secondary "Reset to 300", secondary "Cancel".
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
     - Two inputs: "Default weight" `1` (hint "7 setups match no rule"); "Display label keys" `size` (hint "Shown as columns and filters on Setups").
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

### Reusable components

- Stat card: bordered 8 px card, caption, large value, sub-line.
- Pill / badge: 12 px, 1 px border and text in one color (blue Deployed, green Free/Enabled, grey Acquired/Disabled/fixed role, amber pressure pill).
- Label chip: monospace `key=value`, grey fill and border.
- Buttons: primary (blue fill, white text), secondary (white fill, grey border), destructive (secondary with red text and border); all 44 px high.
- Data table: full width, left-aligned, 12 px muted header with bottom rule, 14 px rows with light row rules, wrapped in a horizontal scroller, aria-label per table, trailing blank-header action column.
- Side panel (aside): grey `#F4F4F4`, bordered, 8 px radius, 20 px padding, used for Acquire confirmation, Grant tokens, Edit setup.
- Form field: 13 px label (600 weight when it is a panel field) above a 44 px input; hint text 12 to 13 px muted below.
- Pressure band bar and bar-chart placeholder (My usage).
- Overdrawn marker: amber bold negative number (also "overdrawn" text on Setups).

All pages use accessible markup in the wireframes: labels tied to inputs, aria-label on tables and image-like charts, aria-current on the active nav link.

## Differences from the spec

Wireframe shows, spec does not ask for (or says differently):
1. Users page stat cards (Users, Overdrawn, Debited this month, Granted this month) and a "Search users" box. The spec's Users row lists neither.
2. Settings "Collector health" is shown as three cards (last poll, setups matched, outages this month with minutes not charged). The spec only says "collector health".
3. Setup inventory shows a "Held by" column and a Status (Enabled/Disabled) column; the spec lists labels, weight, resource name, enabled flag, last seen.
4. Setups page has a "Details" link for other holders' setups and a "Cost" cell with "Free for 1 h 20 m more" or "N / h if acquired". The spec says "open details JSON", which the wireframe does not show as JSON; a grace-period text is also not in the spec.
5. The wireframe nav shows pending count inside "Users · 2 pending" and an "Audit log" link without a page.
6. The Setups table shows weight only through the cost figure, not as its own column; the spec lists "weight" among the columns.
7. Settings has no hold-limit-specific section; "Max setups per developer" sits under "Allowance and timing" (the spec calls it hold limit). It also has "Poll interval (minutes)" and a "Require teardown before release" checkbox next to "Block acquiring at zero tokens"; the spec lists enforcement and the others elsewhere, and these may need cross-checking against the settings keys.
8. Local admin row on Users ("Admin · fixed", "No balance", "Cannot hold setups") and the rule "An admin cannot remove their own admin role" are shown as notes; check these against AUTH requirements.
9. Grant panel has "Reset to 300" with a reason requirement; the spec says "reset a balance" without requiring a reason.
10. My usage: "Show all entries, grouped by day" link, "Holding setup-04 overnight (15 h)" estimate, and a Month select with November/October. The spec mentions "pick a past month" and ledger history but not the overnight estimate.
11. Acquire confirmation text includes the "2 hours" grace and "your 2nd of 2 allowed setups"; the spec asks for cost at current multiplier and at the highest band.
12. Pressure is drawn as a four-band bar with a marker; the spec only says "lab pressure".

Spec asks for, wireframe does not show:
1. Audit log page (`/admin/audit`), including actor and action filters. Build from the spec.
2. The Releasing state (a release requested while the setup is still deployed: a "Releasing" badge, no Release button, and a message such as "Release requested for lab-03.") and any error or empty states.
3. Projected run-out "not this month" variant.
4. Setups page: the "weight" column and a details JSON view.
5. Setup inventory: a distinct "disable" action (only an Enabled checkbox in the Edit panel); Add setup form (the panel is shown as edit only).
6. Role-change confirmations and a "forced password change" screen after first local sign-in (mentioned only in the Sign in footnote).
7. The 60-second live refresh has no visible indicator.
