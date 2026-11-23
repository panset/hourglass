# Result: 86a docs

Status: **done**. `./ci.sh` exited 0 (`ci.exit` = 0, `ci.log`; 39 of 39 Bazel tests pass). The three command blocks in the new "Pre-approving SSO users" section were written from the code and have NOT been run; each starts with `<!-- 86b: run and correct -->` (find them with `grep -n '86b: run' docs/deploying.md`: lines 255, 280, 294).

## Documents and sections changed

`docs/deploying.md`
- Configuration reference, settings table: new row `preapproved_users_file` / `HOURGLASS_PREAPPROVED_USERS_FILE`.
- Configuration reference, Rules: new bullet on the roll-up and timezone (days cut in the zone configured when compaction runs; a compacted day is never cut again).
- New subsection "Pre-approved users file": format, `.json` or YAML, strict decoding, limits, read once at start, never written, log line.
- "Sign-in: OIDC": stored name fallback (`name`, then `given_name` with `family_name`, then `preferred_username`), the greeting, and a pointer to pre-approval after the first-sign-in steps.
- New section "Pre-approving SSO users" (before "Local admin account"): rules for when a pre-approval applies and what it does not do, audit and events, the three ways (admin page; file, with a marked block; scripted, with two marked blocks), and the three statements the design requires (email claim only, can grant admin, no hand-editing of SQLite).
- "Upgrade": new entry "Upgrading to the ledger roll-up and pre-approved users (migrations 0006 and 0007)": no config change, no down migration, older binary refuses the database, compacted rows cannot be expanded, backup is the way back, slower first start, how the roll-up is paced after the first start, what users see, the ledger API and running-balance effect, the two added fields, sign-in unchanged.
- "Data retention": table updated (ledger no longer "never pruned"), new subsection "The ledger roll-up", new subsection "Disk space" (file stops growing at the old rate, no shrink, offline `VACUUM`).
- "Known limits": two bullets (fixed seven-day window; email-claim-only matching and file read at start).

`docs/ui-notes.md`: My usage ledger sample rows (a rolled-up row), D73 behaviours (Newer and Older disabled while loading, error state, month rollover, 20-row cap); Setups "Hide labels"; "Differences from the spec" item 13 listing the four post-wireframe features. The earlier tasks' notes for ledger paging, chips, greeting and pre-approved users were otherwise complete and matched the code.

`docs/progress.md`: date, new section "Ledger roll-up, labels, greeting, pre-approved users" (built, not finished).

`README.md`: a paragraph on the new features (before Status), `dave` reserved in the quick start, the guides list entry for `deploying.md`.

`deploy/README.md`: "Signing in" lists `dave` / `dev-only-dave` as the fourth realm user and says why he is reserved.

`CONTRIBUTING.md`: new "Compaction scale test" (name, CI size, `HOURGLASS_SCALE_TEST=1` full size, command) and "Upgrade fixtures" (what `testdata/upgrade-d96a950/` holds, that nothing is hand-edited, how it is reproduced, pointing to the README in that folder, D78).

## Evidence for factual claims (path:line)

- Migration 0006 contents (column, marker table, trigger, two partial indexes): `internal/store/migrations/0006_ledger_rollup.sql:5-34`. 0007 (`preapprovals`): `0007_preapprovals.sql:6-14`.
- Day is due when earlier than today minus 7 in the configured zone: `internal/store/ledger_compaction.go:61-69`. Step bounds 2,000 rows and 60 days: `ledger_compaction.go:23,30`.
- At most 20 steps per cycle, order poll, prune, compaction; time deducted from the wait: `internal/collector/collector.go:36-40,170-197`. Pending set by the first cycle and then daily: `collector.go:199-207`. One step under the lock, log line `compacted ledger`, failure retried: `collector.go:233-262`. Search position in memory and restart: D71(e), `collector.go:129-132`. Step 60 ms on 1.26 million rows: D71 (measured by the task that built it, not by me).
- Config key and env var: `internal/config/config.go:105,175,395` (shown in the redacted view). Sample comment: `deploy/config.sample.yaml:15-21`.
- File loader: `.json` vs YAML, strict keys, one document, 10,000 limit, email and role rules, duplicate email, default roles, error naming `users[i]`: `internal/app/preapproved.go:19,45-172`. Read at start before the database, count logged: `internal/app/app.go:165-174`. Email rules: `internal/api/preapproval.go:84-108`; ASCII-only folding: `preapproval.go:57-62`.
- Matching conditions (SSO user, pending, non-empty email, `email_verified`; status re-checked in the transaction; local admin excluded): `internal/auth/oidc.go:299-312`, `internal/api/preapproval.go:220-236`. Claims read: `internal/auth/oidc.go:255-260`; name fallback: `oidc.go:360`; `DisplayName`: `internal/auth/session.go:154-158`.
- Audit action names, 1,000 emails per POST, 254 characters: `internal/api/preapproval.go:30-40`. `user.approved` emitted for pre-approval, no `user.pending`: D76(f),(n); `preapproval.go:135-153`. Routes and response shapes `{preapprovals}` and `{added, updated}`: `internal/api/routes.go:66-68`, `api/openapi.yaml:697-790,2098-2110`.
- Local sign-in body, session CSRF token and the password-change rule: `internal/auth/handlers.go:95-150`; README quick start lines 88-97 use the same calls. The Secure cookie and `localhost` exception are from the existing "Cookies" section.
- UI texts in `ui-notes.md`: `ui/src/pages/setups/LabelChips.tsx:9-36,54-85`, `ui/src/pages/usage/index.tsx:40-43,307-337`, `ui/src/shell/TopBar.tsx:62-63`, `ui/src/pages/admin-users/preapprovals.tsx:47-142`.
- Compaction scale test name and sizes: `internal/api/perf_compaction_scale_test.go:5-17,66`. Fixtures and how they are made: `testdata/upgrade-d96a950/README.md`. `dave`: `deploy/keycloak/realm-hourglass.json:59-65`, `e2e/README.md:14`.
- `VACUUM` command: ran `sqlite3 FILE "VACUUM"` and `PRAGMA integrity_check` on a copy of the d96a950 fixture database in the scratchpad; both succeeded. That is a schema 5 database, not a compacted one.

## Commands run

- `sqlite3` VACUUM and integrity check on a copy of the fixture (above): ok.
- `./ci.sh > ci.log 2>&1`: exit 0.

## Edits the spec's source needs

Each item is one sentence for one table cell: no pipe characters, no line breaks.

1. LED-1 (Ledger and balances): The ledger is append-only, except that the roll-up may replace the debit rows of one hold and day with a single debit row of the same total.
2. Data model, `ledger` row, columns: add `rollup_count` (integer, null for a raw row, otherwise the number of per-poll debit rows a rolled-up row replaced).
3. Data model, `ledger` row, notes: Append-only; no updates, no replaces, no deletes of non-debit rows, and no deletes outside the roll-up, which replaces one hold and day of debit rows with one row of the same total that keeps the newest row's id and time.
4. Data model, new `ledger_compaction` row: the one-row marker table, column `one` (integer primary key, must be 1), inserted and deleted inside each compaction step so it is never committed, which lets the delete trigger allow debit deletes only during compaction.
5. Data model, new `preapprovals` row: `id`, `email` (unique, trimmed, ASCII lower case), `roles_json` (non-empty, from developer and admin), `created_by` (user id or null), `created_at`, `used_at` (first use), `used_by` (user id or null); entries are never consumed and file entries are never stored here.
6. Data model, `users` row notes: `name` is the first non-empty of the `name` claim, the `given_name` and `family_name` claims joined by a space, and `preferred_username` (AUTH-9), and `status` can become `active` at an SSO sign-in when a pre-approval matches (PRE-1).
7. HTTP API table, `GET /api/v1/me/ledger`: change the row to `GET /api/v1/me/ledger?period=YYYY-MM&rollup=day&limit=N&cursor=C`, any role, ledger entries; `rollup=day` merges debits of one hold and local day, `limit` (1 to 200) pages, `cursor` is the opaque `next_cursor`, bad values are 400 `validation`, and each entry has `entry_count` and the response has `next_cursor` (LED-11 to LED-15).
8. HTTP API table, `GET /api/v1/setups?selector=`: add that the response also returns `label_index`, every label key with its sorted distinct values and a ready-made selector term or null, over every setup the caller may see and ignoring `selector` (SET-8).
9. HTTP API table, `GET /api/v1/session`: add that the response carries `display_name` (name, else username, else email) for every session, including a user with no role (AUTH-9).
10. HTTP API table, new row: `GET /api/v1/admin/preapprovals`, admin, lists pre-approvals from the database and the file by email with source, creator and first use (PRE-5, PRE-6).
11. HTTP API table, new row: `POST /api/v1/admin/preapprovals`, admin, adds 1 to 1,000 emails with one role set, all or nothing, answers the counts added and updated, audited as `preapproval.add` (PRE-5).
12. HTTP API table, new row: `DELETE /api/v1/admin/preapprovals/{id}`, admin, removes one database entry (204 or 404 `not_found`), changes no user, audited as `preapproval.delete` (PRE-5).
13. Dashboard pages table, My usage row, Shows: replace "ledger history" with "ledger, 20 rows at a time, by day or every poll"; Actions: add "switch the ledger view; page with Newer and Older" (UI-8).
14. Dashboard pages table, Setups row, Shows: add "every label key and value as chips"; Actions: add "click a label chip to add, replace or remove its selector term" (UI-9).
15. Dashboard pages table, Users row, Shows: add "the pre-approved list with source and first use"; Actions: add "pre-approve emails with a role; remove an entry added here" (UI-11).
16. Dashboard pages text on the top bar: add that the top bar greets the signed-in user with "Hello" and their display name, cut with an ellipsis at a fixed width, and shows no greeting when the name is empty (UI-10).
17. Configuration table, new row: `preapproved_users_file` (`HOURGLASS_PREAPPROVED_USERS_FILE`), file or environment, example `/etc/hourglass/preapproved-users.yaml`, optional read-only list of SSO users approved at first sign-in, read once at start, an invalid file stops the start.
18. Requirement LED-7: Raw debit rows of the same user, hold, period and local day whose day ended more than 7 days ago are replaced by one debit row with their total, the id and created_at of the newest of them, and the count of rows replaced; other types, rows without a hold, rows of the last 7 days and rows already rolled up are never changed.
19. Requirement LED-8: Compaction changes no sum and no order: totals per user, period and type, per hold, and per user and local day are equal before and after, `users.balance_mt` is untouched, and every ledger read returns the same result apart from the merged rows.
20. Requirement LED-9: The ledger still refuses every update, every replace, every delete of a non-debit row and every delete made outside compaction, and no compaction step, successful or failed, leaves the marker behind.
21. Requirement LED-10: Compaction runs in bounded steps that release the collector lock between them, a backlog of any size delays no poll past the poll interval, a failed step fails no poll, and running it when nothing is due changes nothing.
22. Requirement LED-11: `GET /api/v1/me/ledger` without `rollup`, `limit` or `cursor` returns one entry per stored row, unpaged, newest first in the same order as before, with the earlier fields plus `entry_count` and `next_cursor`.
23. Requirement LED-12: With `rollup=day`, debit rows of the same hold and local day are one entry, identical including its id whether or not storage has compacted them, with `entry_count` set and `multiplier` null when it stands for more than one poll.
24. Requirement LED-13: With `limit`, at most `limit` entries are returned and `next_cursor` leads to the next older page; following cursors returns no entry twice and every unchanged entry exactly once even while polls write; bad `rollup`, `limit` or `cursor` values are 400 `validation`.
25. Requirement LED-14: In every view and on every page, an entry's `balance_after_tokens` equals the next older entry's balance plus its own amount, and the newest entry's balance equals the period's balance.
26. Requirement LED-15: A rolled-up row never reports a multiplier taken from a single poll.
27. Requirement UI-8: The Ledger on My usage shows at most 20 rows, by day by default with a switch to every poll, with Newer and Older controls disabled at the ends; changing the month or the view returns to the first page.
28. Requirement UI-9: The Setups page lists every key and value of `label_index` as chips; activating a chip adds, replaces or removes its term and filters; a value with no term cannot be activated; the placeholder is taken from the index.
29. Requirement UI-10: The top bar shows "Hello, {display_name}" for SSO users and the local admin, and no greeting when the name is empty.
30. Requirement UI-11: Admin > Users shows the pre-approval list with its source and use, adds pasted emails with a role, and removes database entries.
31. Requirement SET-8: `GET /api/v1/setups` returns `label_index` over every setup the caller may see, sorted, ignoring `selector`; a developer's index holds no label found only on a disabled setup; every non-null term parses and matches exactly the setups with that value, and a value the grammar cannot express has a null term.
32. Requirement AUTH-9: The stored name of an SSO user is the first non-empty of `name`, `given_name` with `family_name`, and `preferred_username`; `GET /api/v1/session` returns `display_name` as name, else username, else email, by the function the rest of the API uses, and a no-role user's session gets that field and nothing else new.
33. Requirement PRE-1: A new or pending SSO user whose email matches a pre-approval is, by the end of that sign-in, active with the entry's roles and the period's allowance, and their first session shows those roles.
34. Requirement PRE-2: Matching uses only the `email` claim, ignores case and surrounding whitespace, needs a non-empty email, and never matches when `email_verified` is false as a boolean or a string.
35. Requirement PRE-3: A rejected user, an active user and the local admin are never changed by a pre-approval, and deleting an entry changes no existing user.
36. Requirement PRE-4: A pre-approved sign-in writes one `user.preapprove` audit entry with no actor and the matched sources, emits `user.approved`, emits no `user.pending`, and marks an unused database entry as used.
37. Requirement PRE-5: The admin routes list, add (1 to 1,000 emails, all or nothing, with validation) and delete pre-approvals, are audited, and are refused for every principal except an admin.
38. Requirement PRE-6: `preapproved_users_file` is optional; a valid file's entries apply as an overlay and appear in the list with source `config`; an invalid file stops startup and names the problem; the file is never written.
39. Requirement PRE-7: The roles for an email in both sources are the union.
40. Requirement PRE-8: If the approval step fails, the sign-in succeeds and the user stays pending.
41. Requirement UPG-1: A database created by the release at `d96a950`, with ledger data, upgrades by starting the new binary with the same configuration, and every balance and per-hold charge is the same afterwards.
42. Requirement UPG-2: With no pre-approvals and no new request parameters, the responses of `GET /api/v1/me/ledger`, `GET /api/v1/setups` and `GET /api/v1/session`, and the SSO sign-in of a new user, differ from the release at `d96a950` only by the added fields.
43. Requirement UPG-3: The contract test passes: `api/openapi.yaml` and the handlers agree, and no field, parameter or route of the earlier contract is removed.
44. AUTH-2: An OIDC user's identity is the `sub` claim; `email` is stored for display, and the name is the first non-empty of `name`, `given_name` with `family_name`, and `preferred_username`, both refreshed on every sign-in; no scope is added.
45. AUTH-5: Add that the session of a user with no role also carries `display_name`, derived only from `name`, `username` and `email` that the response already holds, so nothing else about the user is revealed.
46. Roles and permissions table: add that the three pre-approval routes need the `admin` role and the CSRF token, and no service-account scope grants them.

## Decisions for docs/decisions.md

None new. One documentation choice for the orchestrator to know: the "Pre-approving SSO users" section sits before "Local admin account", and the commands carry the marker comment (no end marker) so 86b can find them.

## Confidence and gaps

- The three marked command blocks are unrun. Details most likely to need correction in 86b: the `docker run` example uses `hourglass:dev` and a file readable by uid 65532 (a bind-mounted file keeps the host owner and mode); the `jq` and `split` loop; whether the local admin's password change is needed in the bundled stack before step 3 (written so, from the middleware).
- The statement that the first start "can take longer" is an inference from the migration building indexes over all debit rows; its duration was not measured. The "few hours to work off a million rows" figure is arithmetic from the step limits (20 steps of 2,000 rows per cycle), not a measurement; the 60 ms per step figure is quoted from D71.
- `docs/extending.md` (event table, line 469) still says `user.approved` is emitted when an admin assigns a first role. It is also emitted for a pre-approved sign-in. It is outside this task's scope, so I did not edit it; `docs/progress.md` lists it as open. `docs/integrating.md` was not checked for hook-event wording.
- `docs/progress.md` says the new end-to-end steps had not run in a browser, as the 85-e2e-compat result stated; if that has changed since, the line needs updating.
- Item 46 of the spec list goes beyond the design's own list (it names the permissions table); drop it if the spec has no such row for admin routes.
- I did not check `internal/version` or the release workflow; those belong to the release step.
