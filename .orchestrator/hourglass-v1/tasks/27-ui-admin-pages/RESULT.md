# Task 27: UI admin pages, result (status: done)

## What was done
Replaced the four placeholders with working pages: Users (ADM-1, ADM-5, LED-4, LED-5), Setup inventory (SET-1..SET-5), Settings (ADM-2, ADM-4, RULE-3), Audit log (ADM-3). Each page polls every 60 s via `useApi({pollMs})`, refetches right after a successful action, shows `{error:{code,message}}` messages, and disables the action button while in flight. No shared file, router, package.json or generated schema was edited.

## Files (all new or replaced)
- `ui/src/pages/admin-users/{index.tsx,api.ts,admin-users.test.tsx}`
- `ui/src/pages/admin-setups/{index.tsx,SetupForm.tsx,api.ts,admin-setups.test.tsx}`
- `ui/src/pages/admin-settings/{index.tsx,form.ts,api.ts,admin-settings.test.tsx}`
- `ui/src/pages/admin-audit/{index.tsx,api.ts,admin-audit.test.tsx}`
- `ui/src/pages/admin-shared/{useAction.ts,admin.module.css,fixtures.ts}` (new directory of mine, shared by the four pages; the brief lists only the four page directories, so this is a small stretch of "under your own directory")
- `.orchestrator/hourglass-v1/tasks/27-ui-admin-pages/{PROGRESS.md,RESULT.md}`

## Commands run
`npx vitest run` 135 pass (47 new); `npx tsc --noEmit` clean; `bazel test //ui/...` 3/3 pass; `bazel build //ui/...` ok; `./tools/check_spdx.sh` exit 0.

## Displayed values and their API fields
Users (`GET /api/v1/admin/users`, and `?status=pending`):
- Pending: `name`, `email`, `first_sign_in_at`; order as returned (oldest first).
- Table: `name`, `email`, `roles`, `status` (Rejected badge), `is_local_admin`, `balance_tokens`, `overdrawn` (amber), `month_to_date_burn_tokens`, `active_holds[].setup_name`; order as returned.
- Grant panel: `balance_tokens`, `burn_per_hour_tokens`, `active_holds`, `period_ends_at`, `default_allowance_tokens` ("Reset to N").
- Actions: `PUT .../{id}/roles {roles}`, `POST .../{id}/reject`, `POST .../{id}/grants {amount_tokens, reason}`, `POST .../{id}/reset {reason}`.

Setup inventory (`GET /api/v1/setups`, `GET /api/v1/admin/collector`):
- `name`, `labels`, `weight`, `admin.resource_name`, `admin.resource_name_override` ("(override)"), `admin.last_seen_at`, `holder.name`, `admin.enabled`, `holder.hold_id`, `details`.
- Orphans: `orphans[].resource_name`, `first_seen_at`, `setup.name`.
- Actions: `POST /api/v1/admin/setups`, `PUT/DELETE /api/v1/admin/setups/{id}`, force-release `POST /api/v1/holds/{hold_id}/release {force:true, reason}`. Disable is a `PUT` with the other fields copied from the listing and `enabled:false`.

Settings (`GET /admin/settings`, `/admin/rules`, `/admin/collector`; `PUT /admin/settings`, `PUT /admin/rules`):
- Form fields: every key of `settings` (`resource_name_template`, `default_weight`, `display_label_keys`, `bands.base`, `bands.steps[]`, `default_allowance`, `max_holds_per_user`, `release_requires_teardown`, `poll_interval`, `enforcement`).
- Rules: `rules[].selector`, `weight`, `match_count`; `unmatched_setups[].name`.
- Collector cards: `last_successful_poll_at`, `matched_setups`, `orphan_count`, `outages_this_month.{count,minutes_not_charged}`, `queue_length`; `last_error`, `outages[]`, `failed_deliveries[]`.
- Read-only line: `runtime.provider`, `pressure_source`, `metering_policy.{name,description}`, `hooks`, `oidc_issuer`.

Audit (`GET /admin/audit?actor&action&limit=50&offset`; actor list from `GET /admin/users`):
- `created_at`, `actor.name`, `action`, `target`, `before`, `after`, `total`, `limit`, `offset`.

## Decisions for docs/decisions.md
1. Pending requests use per-row Developer/Admin checkboxes and an Approve button (disabled until one is ticked), per the brief, not the wireframe's single select.
2. The wireframe's stat cards, "Search users" box and "Cannot hold" notes beyond the local admin are not built (spec does not ask).
3. Poll interval is a text field holding the API duration string (`5m`), not a minutes number, because the API field is a duration.
4. Pressure band "From pressure" is entered as the API fraction (0 to 1), not a percent; converting would be client-side arithmetic.
5. Empty setup details are not allowed to be blank: the field must hold valid JSON (new setups start with `{}`; `null` is valid).
6. Match counts in the rules table are those of the last saved rules; new rules show "After saving" and a note appears while rules are edited (counts are never computed client-side).
7. Rules and settings are saved by one "Save changes" button: rules first, then settings, only the dirty parts. If the settings call fails after the rules saved, the page refetches and keeps the settings draft.
8. Audit actor filter is a select filled from `GET /admin/users` (names), sending the user id; action filter is exact text. Filters apply on "Apply filters".
9. Page size is a fixed 50 (the API default).
10. Last-seen and other times use the shared `formatDateTime` (absolute, browser zone), not "2 min ago".

## Confidence and gaps
- Verified: all commands above. Not verified: behaviour in a real browser or against a real `hourglassd`; layout and CSS were not viewed.
- Gaps in the API for these pages (rendered without):
  - No `/session`-independent way to see "first user" for the local admin row; the note "Local account · first user" is fixed text keyed on `is_local_admin`.
  - Setups list has no `Held by` for non-holders and no per-setup "rate rule that matched" display was requested; `matched_rule_selector` is unused.
  - No server-supplied template preview ("setup-04 becomes setup-04_RP"), so the Settings template field has no preview (computing it would be business logic).
  - The audit API has no actor search by name; the actor select is limited to users returned by `/admin/users`.
  - The Users page grant panel cannot show a "New balance" preview (would be a client-side computation).
- Rule edits that add or reorder rows cannot show new match counts until saved (see decision 6).
- Role-dependent elements are tested for the local admin row and the developer not-found route; the shell, not the pages, enforces admin access.
