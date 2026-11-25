# Design: ledger roll-up and paging, label discovery, greeting, pre-approved SSO users

Status: approved direction, not implemented. Written 2026-11-23 against `main` at `d96a950` and checked against the code the same day.

**Amended by D96 (2026-11-24).** The label chips of section B2 and requirement UI-9 are superseded: the Setups page has no "Available labels" block and no label columns, and the Label selector box suggests label keys and values as the user types. UI-9 is replaced by UI-21 and UI-22 of [setups-declutter.md](setups-declutter.md#amendment-2026-11-24). SET-8 (`label_index`) is unchanged. The text below is kept as the history of this design.

This document is the brief for the agent that implements, tests and releases the work with the `/orchestrate` skill. It says what to build and why, and names the decisions the owner already made. Where it leaves a detail open, choose the simplest option that satisfies the numbered requirements in [Requirements](#requirements) and record the choice in `docs/decisions.md` from D69 on. If something would contradict this document, stop and ask the owner.

Customers already run Hourglass. Every change here must leave an upgraded installation working with no configuration change, and must leave existing API clients working. [Compatibility](#compatibility) lists what that means for each part.

## Goal

Four problems reported from use:

| | Problem | Outcome |
| --- | --- | --- |
| A | The Ledger on My usage is unbounded: one row per hold per poll, all printed. | Old debits are rolled up to one row per setup per day in storage, the API pages, and the page shows 20 rows at a time. |
| B | A developer sees the placeholder `e.g. site=sjc, gpu=true` in the Label selector box and cannot tell which labels exist. | The Setups page lists every label key and value as clickable chips. |
| C | An SSO user sees no name in the top bar. | The top bar says "Hello, {name}" and always has a name to show. |
| D | A new SSO user must sign in, wait for an admin to assign a role, and only then can work. Customers already hold a list of users who should get in. | An admin pre-approves emails, in the admin page or in a file. A listed user is active with a role at first sign-in. |

## Owner decisions

Made on 2026-11-23. Do not reopen them.

| Decision | Detail |
| --- | --- |
| Ledger roll-up compacts storage. | A daily job replaces the per-poll debits of days that closed more than 7 days ago with one row per user, hold and day. The 7 days are fixed, not a setting. |
| Label discovery is chips on the Setups page. | No read-only Setup inventory page for developers. |
| Pre-approval lives in the database and, optionally, in a file. | The database list is managed from Admin > Users and an admin API. A file named in the config is a read-only overlay. |
| Backward compatible. | No configuration change is needed after upgrade, and existing API calls answer as before. |

## What the code does today

Read these before planning. Line numbers are as of `d96a950`.

**Ledger.**

- The collector writes one `debit` ledger row per metered hold per poll, with `created_at` equal to the poll time (`internal/ledger/service.go` `DebitTx`). At the default 5-minute interval that is 288 rows per hold per day.
- The ledger is append-only by three triggers: `ledger_no_update` and `ledger_no_delete` (`internal/store/migrations/0001_init.sql`) and `ledger_no_replace` (`0002`). It is never pruned (`docs/deploying.md`, "Data retention"). No other table has a foreign key to `ledger`.
- `usage_samples` holds the same per-poll detail (weight, pressure, multiplier, debit) and is pruned after 13 months by `Collector.pruneIfDue` (`internal/collector/collector.go`). A restart never double-charges because of its unique key on hold and poll time (MET-5), not because of anything in the ledger.
- `GET /api/v1/me/ledger` (`listMyLedger`, `internal/api/handlers_user.go:265`) loads every row of the period with `ListLedger` (ordered by `id`), computes a running balance in Go and returns everything, newest first.
- Several reads depend on the ledger's `id` order: `ListLedger`, `BalanceThrough` (`id <= ?`, used for the balance shown after a grant or reset), `LatestLedgerEntry`, and the "is this the user's first entry of the period" check in `AppendLedger` (`id < ?`).
- `DebitMultipliers` (`internal/store/apiuser_queries.go:145`) finds a debit's multiplier by joining `usage_samples` on `hold_id` and `polled_at = created_at`.
- These reads sum the ledger and must give the same answers after a roll-up: `SumLedger`, `RebuildBalance`, `RebuildBalances`, `HoldCharges`, `DebitEntries`, `LedgerPeriods`, `CountLedger`, `LatestAllowancePeriod`, `BalanceThrough`, and the admin totals in `internal/store/apiadmin_queries.go`.
- `Collector.runMu` serialises polls and prunes. A poll delayed past twice the poll interval is not charged and is recorded as an outage (MET-4), so nothing that holds this lock may run long. SQLite also has a single writer, so a long write transaction makes user requests that write (acquire, release, grant) wait.
- Two end-to-end tests read the ledger. `e2e/browser/tests/acceptance.spec.ts` (`readLedger`) reads every row of the Ledger table in the page and asserts single debits with their multiplier ("Debit · 2×"). `e2e/browser/tests/integration.spec.ts` (`debitsOf`) calls `GET /api/v1/me/ledger` with no parameters and expects one entry per debit: it is an existing API client and must keep passing unchanged.

**Setups.**

- `GET /api/v1/setups` (`listSetups`, `internal/api/handlers_user.go:507`) already returns every label of every setup to any user with a role. Developers see enabled setups; admins also see disabled ones.
- The Setups page (`ui/src/pages/setups/index.tsx`) builds columns and dropdowns only for the admin-chosen `display_label_keys`. The free-text selector has a fixed placeholder.
- The selector grammar is in `internal/rules/selector.go`: `key=value`, `key!=value`, `key in (a,b)`, `key notin (a,b)`, `key`, `!key`, joined by commas. A value in a selector cannot contain whitespace or any of `, ( ) = !`. A stored label value may contain any characters and may be empty (`internal/rules/labels.go`), so some stored values cannot be written in a selector. An empty value can: `key=`.

**Name in the top bar.**

- `ui/src/shell/TopBar.tsx` prints `session.name` with no greeting.
- `session.name` is `users.name`, which for an SSO user is the ID token's `name` claim and nothing else (`internal/auth/oidc.go`, the `claims` struct; `internal/auth/session.go` `view`). If the identity provider sends no `name` claim, the top bar shows nothing. This is the likely cause of the report; it was not confirmed against the customer's provider.
- Everywhere else the API shows a user through `userDisplayName` (`internal/api/helpers.go:207`), which falls back from name to username to email. The session is the one place without the fallback.
- Two tests pin the exact field list of a no-role user's session: `TestAUTH5_SessionOfUnassignedUser` (`internal/auth/session_test.go`) and `TestAUTH5_SessionOfUserWithNoRoleIsOwnIdentityOnly` (`internal/api/permissions_test.go`).

**SSO sign-in and approval.**

- A user row is created at the first SSO sign-in, keyed by the `sub` claim, with status `pending` and no roles (`UpsertOIDCLogin`, `internal/store/users.go:84`). `user.pending` is emitted for a new row. `users.email` is refreshed at every sign-in and is not unique.
- An admin approves by assigning roles: `setUserRoles` (`internal/api/handlers_admin.go:118`) sets the roles, sets status `active`, calls `Ledger.EnsureFirstAllowanceTx`, writes the `user.roles` audit entry and emits `user.approved`, all in one transaction apart from the event.
- The config has no list of users. The seed file holds setups and rate rules only.

## Non-goals

- No change to how debits are computed or when they are written. Polls keep writing one debit row per metered hold.
- No setting for the roll-up age, and no way to turn the roll-up off.
- No roll-up of `usage_samples`, the audit log or any other table.
- No minimum and maximum multiplier on a rolled-up row. A row that stands for more than one poll shows no multiplier.
- No read-only Setup inventory page for developers.
- No wildcard or domain pre-approval (`*@example.com`). Entries are whole email addresses.
- No group or claim based role mapping from the identity provider, and no matching on a claim other than `email`.
- No removal of access through the pre-approval list. Deleting an entry never changes a user who already has a role.
- No service-account scope for pre-approvals. They stay admin-only, like every other user management route.
- No direct SQL procedure for customers. Writing to the SQLite file by hand bypasses the audit log; the admin API is the supported way to load the list into the database.

## A. Ledger

### A1. Roll-up in storage

A **raw** debit is a `debit` ledger row whose `rollup_count` (a new column) is null. Every row written by a poll is raw.

A **roll-up group** is every raw debit with the same `user_id`, `hold_id`, `period` and local calendar day of `created_at`, the day taken in the configured timezone. Rows with no `hold_id`, rows of any other type and rows that are already rolled up are never grouped.

A group is **due** when its day ended more than 7 days ago: with `today` the current local date, the group's day is earlier than `today - 7`.

Compacting a due group that has more than one row replaces its rows with one row:

| Column | Value |
| --- | --- |
| `id`, `created_at` | Those of the group's newest row (the highest `id`). |
| `user_id`, `hold_id`, `period` | The group's. |
| `type` | `debit` |
| `amount_mt` | The sum of the group's amounts. |
| `actor_id`, `reason` | Null and empty. |
| `rollup_count` | The number of rows replaced. |

The row keeps the newest row's `id` on purpose. The ledger's `id` order is what every running balance and the reads listed above rely on; keeping the id leaves that order, and every one of those reads, as they are. Updates are forbidden by trigger, so the implementation deletes the group's rows and inserts the new row with that explicit id, in one transaction. `ledger_no_replace` allows the insert because the id no longer exists once the group is deleted.

A group with one row is left alone and a rolled-up row is final, which makes the job idempotent.

Rules:

- The deletes and the insert of one group happen in one transaction. A crash leaves a group either untouched or compacted, never half done.
- Compaction does not go through `AppendLedger` and does not touch `users.balance_mt`: the sum of a user's entries in every period is unchanged, so the cache stays right.
- `internal/ledger.Service` stays the only writer of the ledger. Add the operation there (for example `CompactDue`) and have the collector call it, as it calls `RolloverDue`.
- Neither the collector lock nor SQLite's write lock may be held long. Compaction works in bounded steps: one transaction takes whole groups up to a fixed number of rows (a single larger group is taken alone); choose a bound that keeps a step well under one second on the scale test's data. The collector releases `runMu` between steps, and one collector cycle does at most a fixed number of steps. A backlog is worked off over following cycles. The first start after upgrade on a database with months of history is the case to design for. Once nothing is due, the check runs once a day with the other pruning.
- Finding the next due groups must not scan the whole ledger on every step. Add an index that serves it (for example a partial index over raw debits by `created_at`) or walk by hold, and show the cost with the scale test.
- A failed step is logged and retried on a later cycle. It never fails a poll.
- Log the number of rows removed when it is above zero, as the prune steps do.

### A2. The append-only rule

Migration `0006` changes the delete trigger and adds the column. The update and replace triggers stay as they are.

- `ALTER TABLE ledger ADD COLUMN rollup_count INTEGER` (null for a raw row).
- Replace `ledger_no_delete` with a trigger that still aborts every delete, except a delete of a `debit` row made while compaction is in progress. SQLite has no session variables, so use a marker table: the compaction transaction inserts a row into a one-row table (for example `ledger_compaction`) at its start and deletes it before commit, and the trigger aborts unless the marker exists and `OLD.type = 'debit'`. The marker is never committed, so no other connection ever sees it.
- Nothing else in the code may insert into the marker table. Tests: a plain `DELETE FROM ledger` still fails for a debit row and for a non-debit row; a delete of a non-debit row fails even with the marker present; the marker table is empty after every compaction step, including a failed one.

LED-1 becomes: the ledger is append-only, except that the roll-up may replace the debit rows of one hold and day with a single debit row of the same total.

### A3. Reads that must change

- `DebitMultipliers` must skip rows with a non-null `rollup_count`. Without that, a rolled-up row would match the usage sample of its last poll and show that single poll's multiplier for the whole day.
- `ledgerCols`, `scanLedger` and `store.LedgerEntry` gain the column.
- Nothing else should need to change, because ids and sums are preserved. Prove it: one test builds a ledger, records the result of every read listed under [What the code does today](#what-the-code-does-today), compacts, and compares.

One visible effect is inherent to merging. A running balance is the sum of the entries up to a row in id order, and a rolled-up row sits where the day's last debit was. So after a day is compacted, the balance shown against a grant or other entry in the middle of that day no longer includes that day's earlier debits for the same hold. Each row's balance still equals the previous row's balance plus its own amount, and the balance after the last entry of every day is unchanged.

### A4. API: `GET /api/v1/me/ledger`

Three optional query parameters and two response fields are added. A request without the new parameters is answered as it is today.

| Parameter | Meaning |
| --- | --- |
| `rollup` | `day` merges debit rows into one entry per hold and local day, whether or not storage has compacted them yet. Absent: one entry per stored row, as today. Any other value is 400 `validation`. |
| `limit` | Page size, 1 to 200. Absent: no paging, every entry is returned, as today. Out of range or not a number is 400 `validation`. |
| `cursor` | The `next_cursor` of the previous page. Requires `limit`; without it, or malformed, is 400 `validation`. A cursor is meant for the `period` and `rollup` it came from. |

Response changes, both additive:

- `next_cursor`: a string when older entries exist beyond this page, otherwise `null`. Always `null` when `limit` is absent.
- Each entry gains `entry_count`: the number of polls the entry stands for. 1 for a raw row or a non-debit row, the stored `rollup_count` for a compacted row, and the sum over the merged rows in the `rollup=day` view.

An entry in the `rollup=day` view that merges several rows has:

| Field | Value |
| --- | --- |
| `id`, `created_at` | Those of the newest merged row. This is the row compaction keeps, so an entry's id does not change when its day is compacted. |
| `amount_tokens` | The sum. |
| `multiplier` | Null when `entry_count` is above 1. |
| `setup_name`, `hold_id` | The hold's. |
| `actor_name`, `reason` | Null. |

Ordering is newest first, by descending `id`, as today. The view merges every debit row of the same hold and local day, raw or rolled up.

The cursor is opaque to clients. It means "entries of this view whose id is lower than this position", and it is applied to the entries after merging, never to stored rows before merging. This matters for today's entry of an active hold, whose id moves forward at every poll: filtering stored rows first would return the older part of that entry a second time on the next page.

What a walk through the pages guarantees while polls keep writing: no entry is returned twice, and an entry that does not change during the walk is returned exactly once. A merged entry that gains polls during the walk moves to the front; a walk that had not reached it yet does not return it. Only today's entries of active holds can do that, they are the newest entries, and the first page holds them unless `limit` is smaller than the number of holds the user has.

`balance_after_tokens` is the running sum over the entries of the view in id order, from the oldest entry of the period up to and including that entry. With `rollup=day` a merged entry counts once, at its position. So on any page each row's balance equals the balance of the row below it plus its own amount, and the newest row's balance is the period's balance. This holds across pages: the handler needs the sum of everything older than the page, not only the page's rows.

Grouping by local day needs the configured timezone, which SQLite cannot compute for a named zone. Group in Go over the period's rows, or pass day boundaries to SQL. After compaction a period holds at most about a week of per-poll rows plus one row per hold and day, so reading a period is bounded.

`available_periods` is unchanged. Update `api/openapi.yaml`, the contract test and the generated UI types together.

### A5. UI: My usage

- The Ledger table shows 20 rows per page, never more, so the page needs no long scroll for the ledger.
- A two-way switch above the table: **By day** (the default; requests `rollup=day&limit=20`) and **Every poll** (requests `limit=20` with no `rollup`). Every poll keeps what the page shows today, single debits with their multiplier, for the days storage has not compacted; older days show their rolled-up rows in either view.
- Below the table: **Newer** and **Older** buttons. Older is disabled when `next_cursor` is null; Newer is disabled on the first page. The page keeps the cursors it has visited so Newer works without a server-side "previous" cursor.
- Changing the month or the view returns to the first page. The 60-second poll and the reload after a release refetch the page being shown.
- The Type cell shows the multiplier only when it is not null, as today. When `entry_count` is above 1 the Detail cell shows the setup name followed by the count, for example "lab-06 · 288 polls". The UI formats fields; it does not compute them (UI-1).

## B. Label discovery on Setups

### B1. API

`GET /api/v1/setups` gains `label_index`, sorted by key, values sorted and distinct:

```json
"label_index": [
  { "key": "site", "values": [
      { "value": "ams", "selector": "site=ams" },
      { "value": "sjc", "selector": "site=sjc" } ] }
]
```

- It covers every setup the caller may see (enabled setups for a developer, all setups for an admin) and ignores the request's `selector`, like `summary`. It is `[]` when no setup has a label.
- `selector` is the ready-made term that selects exactly the setups with that value, or `null` when the value cannot be written in a selector (it contains whitespace or one of `, ( ) = !`). An empty value gives `key=`. Build the term in `internal/rules`, next to the parser, so the grammar lives in one place. Test that every non-null term parses and matches exactly the setups that carry that value.
- No route is added and no permission changes.

### B2. UI

> Amended by D96: the chips described in this section were removed in v1.3.0. UI-9 is replaced by UI-21 and UI-22 of [setups-declutter.md](setups-declutter.md#amendment-2026-11-24). The bullets below are history.

- Under the Label selector box, an "Available labels" block lists each key from `label_index` with its values as chips. A key shows its first 12 values and a "+N more" control for the rest. With more than 8 keys the block starts collapsed behind a "Show labels" control. Record the two numbers if you change them.
- A chip is a button with an accessible name such as "Filter by site=sjc", and it works with the keyboard. Activating it puts its `selector` term into the selector box and applies the filter:
  - if the box already holds exactly that term, the term is removed (the chip toggles);
  - if the box holds another `key=value` term for the same key, that term is replaced;
  - otherwise the term is appended.
- To do this the page splits the box's text on commas outside parentheses and compares whole terms. It takes the terms from the API and does not build or interpret selector grammar itself.
- A chip whose term is in the applied selector is shown as selected.
- A value whose `selector` is null is shown as a chip that cannot be activated, with a short explanation that it cannot be used in a selector.
- The placeholder is the first non-null term of `label_index` ("e.g. site=sjc"). With an empty index the placeholder is omitted and the block says no setup has labels.
- The existing dropdowns for `display_label_keys` stay as they are.
- (Amended by D96: the dropdowns went with D93 and the chips with D96; the selector box's suggestions replace them.)

## C. Greeting

### C1. Server

- At SSO sign-in read three more ID token claims: `given_name`, `family_name`, `preferred_username`. The name stored in `users.name` is the first non-empty of: `name`; `given_name` and `family_name` joined by a space and trimmed; `preferred_username`. If all are empty the stored name is empty, as today. AUTH-2 is otherwise unchanged; no scope is added (`openid email profile` already covers these claims).
- `GET /api/v1/session` gains `display_name`: name, else username, else email, the rule `userDisplayName` already applies. Use one shared function for the session and the API so the two cannot drift. `name` stays in the response with its current meaning.
- `display_name` is part of the session of every user, including one with no role. It is derived only from `name`, `username` and `email`, which that response already carries, so AUTH-5 still holds. Add the field to the two AUTH-5 tests' field lists, to the `SessionView` comment and to the `Session` schema description; do not loosen those tests in any other way.
- A user whose stored name is empty gets their email as `display_name` at once, with no new sign-in. The stored name is filled from the fallback claims at their next sign-in.

### C2. UI

- The top bar shows "Hello, {display_name}" where it shows the name today, for SSO users and for the local admin.
- A long name is cut with an ellipsis at a fixed maximum width and carries the full text in its `title`.
- If `display_name` is empty (a provider that sends no name, username or email), show no greeting rather than "Hello, ".

## D. Pre-approved SSO users

### D1. Behaviour

A **pre-approval** is an email address and a non-empty set of roles (`developer`, `admin`). It comes from the database list or from the file. The effective roles for an email are the union of both sources.

At the end of a successful SSO sign-in, after the user row is created or refreshed, a pre-approval is applied when all of these hold:

1. The user's status is `pending` (a new user, or one who signed in earlier and was never approved or rejected).
2. The ID token's `email` claim is not empty. No other claim is used for matching.
3. The ID token's `email_verified` claim is not false. The claim may be a boolean or the strings `"true"` / `"false"`. An absent claim counts as verified, because several enterprise providers never send it.
4. A pre-approval exists for the email, compared after trimming and lower-casing both sides.

Applying it does what an admin's approval does, in one transaction: check the status again inside the transaction, set the roles, set status `active`, call `Ledger.EnsureFirstAllowanceTx`, and write an audit entry `user.preapprove` with no actor (shown as "system"), `before` and `after` in the shape `user.roles` uses, plus the sources that matched (`database`, `config` or both). For a database entry that has not been used yet, record when it was used and by which user. After commit emit `user.approved`. A new user who is pre-approved does not get `user.pending`.

The user then lands in the product with their role and allowance, without seeing the not-authorized page.

Rules:

- Only SSO users are affected. The local admin is never matched (AUTH-8).
- A `rejected` user is never matched. An admin's rejection stands until an admin changes it.
- A user who is `active` is never matched, even with no roles. An admin who removed a user's roles is not overridden at the next sign-in.
- Deleting an entry, or removing it from the file, changes nothing for a user who was already approved.
- An entry is not consumed. It stays in the list, marked with its first use. If a second SSO identity later signs in with the same email, it is approved too and the audit log shows each approval.
- Adding an entry for someone who is already pending takes effect at their next sign-in. The admin can also approve them directly, as today.
- If the approval transaction fails, the sign-in still succeeds and the user stays `pending`. Log the error.
- With no database entries and no file, sign-in behaves exactly as today.

`internal/auth` does not import the ledger service today. Give it the approval step through an interface supplied by `internal/app`, and keep `tools/check_core_generic.sh` passing.

### D2. Database list

Add to migration `0006` (or a separate `0007`; choose and record):

```sql
CREATE TABLE preapprovals (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    email       TEXT NOT NULL UNIQUE,      -- trimmed, lower case
    roles_json  TEXT NOT NULL,             -- non-empty, from developer and admin
    created_by  INTEGER REFERENCES users (id),
    created_at  TEXT NOT NULL,
    used_at     TEXT,                      -- first use
    used_by     INTEGER REFERENCES users (id)
);
```

### D3. Admin routes

All three need the `admin` role and the CSRF token like every other admin route.

| Route | Purpose |
| --- | --- |
| `GET /api/v1/admin/preapprovals` | Every entry from both sources, by email. Each has `id` (null for a file entry), `email`, `roles`, `source` (`database` or `config`), `created_at` and `created_by` (null for a file entry), `used_at` and `used_by` (null until used; always null for a file entry). An email in both sources appears twice, once per source. |
| `POST /api/v1/admin/preapprovals` | Body `{ "emails": [...], "roles": [...] }`, 1 to 1,000 emails. Adds each email; an email already in the database list gets the new roles and keeps its use record. Answers the counts added and updated. Any invalid email or role is 400 `validation` naming the first bad value, and nothing is stored. Audited as `preapproval.add` with the emails and roles. |
| `DELETE /api/v1/admin/preapprovals/{id}` | Removes one database entry. 204, or 404 `not_found`. Audited as `preapproval.delete`. File entries have no id and cannot be removed here. |

An email is valid when it is at most 254 characters after trimming, has exactly one `@` with text on both sides, and contains no whitespace, comma or `*`. Roles are validated as `setUserRoles` validates them and must not be empty.

Add the routes to `routes`, to `api/openapi.yaml`, to the permissions test for every principal (including a service account) and to the generated UI types.

### D4. File overlay

A new optional config key, `preapproved_users_file` (environment: `HOURGLASS_PREAPPROVED_USERS_FILE`), names a file:

```yaml
users:
  - email: alice@example.com
    roles: [developer]
  - email: bob@example.com        # roles default to [developer]
```

- A `.json` file is read as JSON, anything else as YAML, decoded strictly, like the seed file.
- It is read once at start. An unreadable or invalid file stops startup with a message naming the entry, as an invalid seed does. A change to the file takes effect at the next restart; the admin page is the way to add someone without a restart.
- Limits: 10,000 entries; the same email and role rules as the admin route; a repeated email is an error.
- The file is never written to and never copied into the database. Removing an entry from the file and restarting removes the pre-approval.
- The key is empty by default. It holds no secret; include the path in the redacted config view. Log the number of entries at start, never the addresses.

### D5. UI: Admin > Users

A "Pre-approved users" section on the Users page:

- A table: Email, Roles, Source (`Added here` or `Config file`), Added by, Signed in (the date, or "Not yet"). Database rows have a Remove button with a confirmation; file rows do not.
- An Add form: a text area for emails, a role choice defaulting to developer, and an Add button. The page splits the text on commas, whitespace and new lines and sends the list; the server validates. A 400 shows the server's message next to the text area and keeps the text.
- A one-line explanation above the table: listed people get the chosen role the first time they sign in with SSO.

### D6. Loading a customer's existing list

Write these steps into `docs/deploying.md` under a new "Pre-approving SSO users" heading, with commands that were run against the bundled stack.

1. **Admin page.** Sign in as an admin, open Users, paste the emails into "Pre-approved users", pick the role, Add.
2. **File.** Write the YAML above, mount it into the container, set `preapproved_users_file` (or the environment variable), restart.
3. **Scripted, into the database.** Three calls with a cookie jar: `POST /auth/local` as the local admin; `GET /api/v1/session` to read `csrf_token`; `POST /api/v1/admin/preapprovals` with the `X-CSRF-Token` header and the emails. Give a working `curl` example. The session cookie is `Secure`, so the example must use HTTPS or `localhost` (see "Cookies" in the same guide), and the local admin must have changed its first-run password before it can call admin routes.

The guide must also say:

- Matching is by the `email` claim of the ID token and nothing else. The identity provider must release that claim, and it should be a provider that controls its users' email addresses. Some providers send the address only in another claim unless configured otherwise; such a user stays pending and is approved by hand.
- A pre-approval can grant `admin`.
- Editing the SQLite file by hand is not supported.

## Compatibility

- **Configuration.** One new optional key. An existing config file and environment start the new version unchanged.
- **Database.** Migration `0006` (and `0007` if split) is additive except for the trigger change. There is no down migration. Once it has run, an older binary refuses the database, and compacted rows cannot be expanded again; the way back is the backup taken before the upgrade. `docs/deploying.md` must say this under "Upgrade", and that the first start may take longer on a large ledger if the migration builds an index.
- **API.** No route, parameter or response field is removed or renamed. `GET /api/v1/me/ledger` with no new parameter returns one entry per stored row, unpaged, in the same order, with the fields it has today plus `entry_count` and `next_cursor: null`. After compaction the stored rows of days older than a week are the rolled-up ones, so that response gets shorter for those days and their running balances follow [A3](#a3-reads-that-must-change); totals and closing balances do not change. `GET /api/v1/setups` and `GET /api/v1/session` each gain one field.
- **Numbers.** Balances, per-hold charges, the tokens-per-day chart, month-to-date burn and the admin totals are the same before and after compaction of the same data.
- **What users can see.** The Every poll view keeps single debits and their multipliers visible in the page for the last week. Older per-poll detail remains in `usage_samples` for 13 months but is no longer shown in the ledger.
- **Sign-in.** With no pre-approvals, a new SSO user is `pending` and gets `user.pending`, as today.
- **Hooks.** No new event type. `user.approved` is also emitted for a pre-approved sign-in, with the payload it has today.
- **Timezone.** Days are cut in the timezone configured when compaction runs, and a compacted day is never cut again. Add this to the existing note on timezone changes in `docs/deploying.md`.
- **Disk.** SQLite reuses freed pages but does not shrink the file. Say in `docs/deploying.md` that the file stops growing at the old rate, and that an offline `VACUUM` reclaims the space.

## Changes by area

| Area | Change |
| --- | --- |
| `internal/store/migrations` | `0006`: `ledger.rollup_count`, the new delete trigger and marker table, an index for finding due rows if used, `preapprovals`. |
| `internal/store` | Compaction queries; `DebitMultipliers` filter; `LedgerEntry` column; pre-approval queries; a transaction-scoped approval helper if it helps share code with `setUserRoles`. |
| `internal/ledger` | The compaction operation and its bounded step. |
| `internal/collector` | Calls compaction between polls without holding the lock long. |
| `internal/rules` | The selector term for a label value. |
| `internal/api` | Ledger parameters, roll-up view and paging; `label_index`; shared display-name function; pre-approval routes; route table. |
| `internal/auth` | Extra claims; `display_name`; `email_verified`; the pre-approval step through an injected interface. |
| `internal/config`, `internal/app` | `preapproved_users_file`, its loader and wiring. |
| `api/openapi.yaml`, `ui/src/api/schema.gen.ts` | All of the above. |
| `ui/src/pages/usage`, `setups`, `admin-users`, `ui/src/shell/TopBar.tsx` | A5, B2, D5, C2. |
| `e2e/browser/tests`, `deploy/keycloak` | See [Tests](#tests). |
| `docs` | `decisions.md` from D69; `deploying.md` (upgrade, data retention, configuration reference, pre-approval, timezone note); `ui-notes.md`; `progress.md`; `README.md` where it lists features; `deploy/config.sample.yaml` (a commented example of the new key). `docs/SPEC.md` is not edited: list in `decisions.md` the edits its source needs (LED-1, the ledger and users table rows, the HTTP API table, the Dashboard pages table). |

`tools/check_core_generic.sh` bans some words in every file under `internal/`, tests included. Two are easy to hit in this work: `size=` (use another key such as `site` or `tier` in label examples) and the name of the test identity provider.

## Requirements

Every requirement gets at least one test named after its id that fails when the requirement is broken. If an id is already taken when the work starts, use the next free number and say so in `decisions.md`.

### Ledger

- **LED-7** Raw debit rows of the same user, hold, period and local day whose day ended more than 7 days ago are replaced by one debit row with their total, the `id` and `created_at` of the newest of them, and the count of rows replaced. Rows of other types, rows without a hold, rows of the last 7 days and rows already rolled up are never changed.
- **LED-8** Compaction changes no sum and no order: for every user, period and type, for every hold, and for every user and local day, the total before equals the total after; `users.balance_mt` is untouched; and every ledger read the code has today returns the same result, apart from the merged rows themselves.
- **LED-9** The ledger still refuses every update, every replace, every delete of a non-debit row, and every delete made outside compaction. No compaction step, successful or failed, leaves the marker behind.
- **LED-10** Compaction runs in bounded steps that release the collector lock between them; a backlog of any size delays no poll past the poll interval, and a failed step fails no poll. Running it again when nothing is due changes nothing.
- **LED-11** `GET /api/v1/me/ledger` without `rollup`, `limit` or `cursor` returns one entry per stored row, unpaged, newest first in the same order as today, with today's fields.
- **LED-12** With `rollup=day`, debit rows of the same hold and local day are one entry, identical (including its `id`) whether or not storage has compacted them, with `entry_count` set and `multiplier` null when it stands for more than one poll.
- **LED-13** With `limit`, at most `limit` entries are returned and `next_cursor` leads to the next older page. Following cursors to the end returns no entry twice and returns every entry that did not change during the walk exactly once, even when polls write new debits between requests. Bad `rollup`, `limit` or `cursor` values are 400 `validation`.
- **LED-14** In every view and on every page, an entry's `balance_after_tokens` equals the next older entry's balance plus its own amount, and the newest entry's balance equals the period's balance.
- **LED-15** A rolled-up row never reports a multiplier taken from a single poll.

### Frontend

- **UI-8** The Ledger on My usage shows at most 20 rows, by day by default with a switch to every poll, with Newer and Older controls that are disabled at the ends; changing the month or the view returns to the first page.
- **UI-9** (Replaced. Amended by D96: UI-9 is replaced by UI-21 and UI-22 of [setups-declutter.md](setups-declutter.md#amendment-2026-11-24); its D94(j) amendment is superseded too. Original text, kept as history:) The Setups page lists every key and value of `label_index` as chips; activating a chip adds, replaces or removes its term and filters; a value with no term cannot be activated; the placeholder is taken from the index.
- **UI-10** The top bar shows "Hello, {display_name}" for SSO users and the local admin, and no greeting when the name is empty.
- **UI-11** Admin > Users shows the pre-approval list with its source and use, adds pasted emails with a role, and removes database entries.

### Setups

- **SET-8** `GET /api/v1/setups` returns `label_index` over every setup the caller may see, sorted, ignoring `selector`; a developer's index holds no label that appears only on a disabled setup; every non-null `selector` term parses and matches exactly the setups with that value, and a value the grammar cannot express has a null term.

### Sign-in

- **AUTH-9** The stored name of an SSO user is the first non-empty of `name`, `given_name` with `family_name`, and `preferred_username`. `GET /api/v1/session` returns `display_name` as name, else username, else email, by the same function the rest of the API uses, and a no-role user's session holds that field and nothing else new.
- **AUTH-9 (amended by D100)** A sign-in that yields no name keeps the stored name; only a non-empty name replaces it. This amends AUTH-2's "refreshed on every sign-in": email and last sign-in time are refreshed every time, the name only when the provider gives one. `GET /api/v1/admin/users` adds `display_name` to each user (the same function; never blank) beside the unchanged `name`, and lists the local account first, always, then overdrawn users, then everyone by display name ignoring case, then id.

### Pre-approval

- **PRE-1** A new or pending SSO user whose email matches a pre-approval is, by the end of that sign-in, active with the entry's roles and the period's allowance, and their first session shows those roles.
- **PRE-2** Matching uses only the `email` claim, ignores case and surrounding whitespace, needs a non-empty email, and never matches when `email_verified` is false (boolean or string).
- **PRE-3** A rejected user, an active user and the local admin are never changed by a pre-approval; deleting an entry changes no existing user.
- **PRE-4** A pre-approved sign-in writes one `user.preapprove` audit entry with no actor and the matched sources, emits `user.approved`, emits no `user.pending`, and marks an unused database entry as used.
- **PRE-5** The admin routes list, add (1 to 1,000 emails, all or nothing, with validation) and delete pre-approvals, are audited, and are refused for every principal except an admin.
- **PRE-6** `preapproved_users_file` is optional; a valid file's entries apply as an overlay and appear in the list with source `config`; an invalid file stops startup and names the problem; the file is never written.
- **PRE-7** Roles for an email in both sources are the union.
- **PRE-8** If the approval step fails, the sign-in succeeds and the user stays pending.

### Compatibility

- **UPG-1** A database created by the current release (`d96a950`), with ledger data, upgrades by starting the new binary with the same configuration, and every balance and per-hold charge is the same afterwards.
- **UPG-2** With no pre-approvals and no new request parameters, the responses of `GET /api/v1/me/ledger`, `GET /api/v1/setups` and `GET /api/v1/session`, and the SSO sign-in of a new user, differ from the current release only by the added fields.
- **UPG-3** The contract test passes: `api/openapi.yaml` and the handlers agree, and no existing field, parameter or route is removed.

## Tests

Beyond one test per requirement:

- A store test that builds a ledger across two periods, three holds and a grant in the middle of a day, snapshots every reader's result, compacts, and compares.
- A scale test beside `internal/api/perf_scale_test.go`: a year of 5-minute debits for several holds compacts within the step bound, finding due groups does not scan the ledger, and a ledger page stays fast before and after.
- A migration test: a database at schema 5 with data, opened by the new store, passes UPG-1.
- End-to-end, in `e2e/browser/tests`:
  - The acceptance scenario asserts single debits and their multipliers through the page. Keep those assertions as strong as they are: have `readLedger` use the Every poll view and walk the pages with Older. Do not replace them with API calls.
  - `integration.spec.ts` reads the ledger API with no parameters. It must pass without being edited for that call.
  - New steps: a developer pages the ledger and switches the view; a developer filters setups with a chip; an SSO user sees the greeting; a pre-approved user who has never signed in lands on My usage with a balance at first sign-in. The last one needs a user in `deploy/keycloak/realm-hourglass.json` that no other scenario signs in; add one.
  - Compaction through the real stack: the test clock can be moved (the month jump already does it). Add a step that moves it more than 8 days past a day with debits and checks that the day's ledger shows one row per hold with its poll count and that the period's closing balance did not change.
- `./ci.sh` passes before each milestone is committed.

## Suggested task order

Each milestone is committed on its own, with requirement ids in the message. Run an independent verification task after milestones 3, 5 and 7, by an agent that did not write the code and is given the code and the requirements, not the author's conclusions.

1. Record D69 (the owner decisions above) in `docs/decisions.md`. This document is already committed on the work branch; do not change it.
2. Migration and store: `rollup_count`, trigger, compaction queries, `DebitMultipliers` filter, the before-and-after reader test (LED-7 to LED-9, LED-15, UPG-1).
3. Ledger service and collector: bounded compaction (LED-10), scale test.
4. Ledger API: roll-up view, paging, balances (LED-11 to LED-14), OpenAPI and generated types.
5. Ledger UI (UI-8) and the acceptance scenario's ledger helper.
6. Labels: `label_index` and chips (SET-8, UI-9). Greeting: claims, `display_name`, top bar (AUTH-9, UI-10). These are small and independent of each other and of the ledger work; they can run in parallel with milestones 2 to 5 as long as two tasks never edit `api/openapi.yaml` or the generated types at the same time.
7. Pre-approval: store and sign-in step (PRE-1 to PRE-4, PRE-8), admin routes (PRE-5), file overlay (PRE-6, PRE-7), admin UI (UI-11).
8. End-to-end scenarios, compatibility tests (UPG-2, UPG-3), documentation.
9. Release: see below.

## Release

- The work happens on the branch `ledger-labels-onboarding`, cut from `main` at `d96a950`. `main` stays equal to what customers run until the owner approves the release.
- Follow `docs/deploying.md` ("Publishing the images to a registry") and D67: pushing a git tag that starts with `v` and a digit runs `.github/workflows/release.yml`, which pushes the three product images and moves `latest`. That workflow does not run the tests.
- No version tag exists locally or on `origin` as of 2026-11-23, so this is the first tagged release. The version number is the owner's to name.
- `internal/version` says the version is overridden at link time, but nothing under `deploy/images` was seen to set it. Check what the released binary reports. If it reports `0.0.0-dev`, tell the owner; do not add version stamping unasked.
- Before asking to release: `./ci.sh` passes on the commit to be tagged; `e2e/run.sh both` and the integration scenario pass; the upgrade notes in `docs/deploying.md` are written; the job report lists every requirement with its test and every decision recorded.
- Merging to `main`, pushing commits and pushing the tag are outward-facing: the tag publishes images that customers pull as `latest`. Stop and get the owner's go-ahead, with the version number, before any of them. Never rewrite pushed history.

## Open points for the owner

None of these blocks the work; each has a default that the implementation follows unless the owner says otherwise.

| Point | Default |
| --- | --- |
| The version number of the release, and whether the agent merges and pushes or hands over for the owner to do it. | The agent stops before merging or pushing and asks. |
| The ledger page has a By day / Every poll switch, added after the design choices were approved so that single debits and their multipliers stay visible. | Built, defaulting to By day. Say if the page should show only the daily view. |
| A pre-approval may grant `admin`. | Allowed, audited. Say if it should be limited to `developer`. |
| The file is read at start only. | A change needs a restart; the admin page covers the no-restart case. Say if the file should be re-read when it changes. |
| An absent `email_verified` claim counts as verified. | Yes, because several enterprise providers do not send it. Say if a strict mode is wanted. |
| Matching uses the `email` claim only. | A provider that does not release `email` leaves its users pending. Say if another claim should be accepted. |
| Adding an entry for a user who is already pending does not approve them at once. | They are approved at their next sign-in, or by an admin as today. |
| An entry is not consumed, so a second SSO identity with the same email is approved too. | Kept, with each approval audited. Say if an entry should work once only. |
| The cause of the missing name was inferred from the code, not confirmed against the customer's identity provider. | The fallback chain and `display_name` are built regardless. If the name is still missing afterwards, check which claims the provider sends. |
