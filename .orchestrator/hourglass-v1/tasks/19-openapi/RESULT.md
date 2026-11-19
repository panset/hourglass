# Task 19: api/openapi.yaml — RESULT

Status: done. `api/openapi.yaml` (OpenAPI 3.0.3, 30 operations, SPDX first line) written.

## Validation
`npx --yes @redocly/cli@latest lint api/openapi.yaml`: valid, 0 errors, 4 warnings (`operation-2xx-response` on the two 302-only OIDC routes, `operation-4xx-response` on `/healthz` and `/auth/methods`; they cannot meaningfully return a 4xx/2xx, left as is). No package.json added. No Go code.

## Files changed
- `api/openapi.yaml` (new)
- this folder: `PROGRESS.md`, `RESULT.md`

## HTTP API table to operationId
| Spec row | operationId |
| --- | --- |
| GET /auth/oidc/start | oidcStart |
| GET /auth/callback | oidcCallback |
| POST /auth/local | localSignIn |
| POST /auth/local/password | changeLocalPassword |
| POST /auth/logout | logout |
| GET /healthz | getHealth |
| GET /api/v1/auth/methods | getAuthMethods |
| GET /api/v1/session | getSession |
| GET /api/v1/me | getMe |
| GET /api/v1/me/holds | listMyHolds |
| GET /api/v1/me/ledger | listMyLedger |
| GET /api/v1/me/usage | getMyUsage |
| GET /api/v1/pressure | getPressure |
| GET /api/v1/setups | listSetups |
| POST /api/v1/setups/{id}/acquire | acquireSetup |
| POST /api/v1/holds/{id}/release | releaseHold (also admin force-release) |
| GET /api/v1/admin/users | listAdminUsers |
| PUT /api/v1/admin/users/{id}/roles | setUserRoles |
| POST /api/v1/admin/users/{id}/reject | rejectUser |
| POST /api/v1/admin/users/{id}/grants | grantTokens |
| POST /api/v1/admin/users/{id}/reset | resetBalance |
| POST /api/v1/admin/setups | createSetup |
| PUT /api/v1/admin/setups/{id} | updateSetup |
| DELETE /api/v1/admin/setups/{id} | deleteSetup |
| GET /api/v1/admin/rules | getRules |
| PUT /api/v1/admin/rules | replaceRules |
| GET /api/v1/admin/settings | getSettings |
| PUT /api/v1/admin/settings | updateSettings |
| GET /api/v1/admin/collector | getCollector |
| GET /api/v1/admin/audit | listAudit |

## docs/ui-notes.md fields to response fields
Paths are relative to the response body of the named operation. Client-side derivations are marked "client".

### Sign in / Not authorized
| UI element | Source |
| --- | --- |
| SSO button / local form visibility | getAuthMethods `oidc`, `local` |
| SSO link | oidcStart; local form posts to localSignIn |
| Forced password change | getSession `must_change_password`; changeLocalPassword |
| "Signed in as morgan@..." | getSession `email` |
| Sign out | logout |

### Top bar
| UI element | Source |
| --- | --- |
| Lab pressure pill "82% · 2x" | getPressure `smoothed`, `multiplier` |
| Balance "142 tokens" | getMe `balance_tokens` (null for local admin: show none) |
| User name | getSession `name` |
| Admin links visible | getSession `roles` |
| "Users · 2 pending" | listAdminUsers `pending_count` |

### My usage
| UI element | Source |
| --- | --- |
| Month select (Nov/Oct) | listMyLedger / getMyUsage `available_periods`, `period` |
| Tokens left | getMe `balance_tokens` |
| "of 300 allowance + 40 granted" | getMe `allowance_tokens`, `granted_tokens` (and `carryover_tokens`) |
| Burning now "2.0 / hour" | getMe `burn_per_hour_tokens` |
| "setup-04 · weight 1 x 2x" | getMe `burning[].setup_name`, `weight`, `multiplier` |
| Projected run-out "Nov 21" / "not this month" | getMe `projection`, `projected_runout_at` |
| Lab pressure "82% · 2x", bar marker | getPressure `smoothed`, `multiplier`; band widths from `bands.base`, `bands.steps[]` |
| "9 of 11 setups deployed" | getPressure `setups_deployed`, `setups_enabled` |
| "Current holds · 1 of 2 allowed" | getMe `hold_count`, `hold_limit`; listMyHolds?state=active |
| Holds table: Setup, Labels, State, Weight, Cost now, Held for | listMyHolds `holds[].setup_name`, `setup_labels`, `deployed`/`metering`, `weight`, `cost_per_hour_tokens.now`, `acquired_at` |
| Release button | releaseHold with `holds[].id` |
| Overnight 15 h estimate | client: `cost_per_hour_tokens.now` x 15 |
| Tokens per day chart, axis labels, today bar | getMyUsage `days[].date`, `debited_tokens` |
| Ledger: When, Type, Detail, Amount, Balance | listMyLedger `entries[].created_at`, `type` (+`multiplier`), `setup_name`/`actor_name`/`reason`, `amount_tokens`, `balance_after_tokens` |
| "Show all entries, grouped by day" | client grouping of `entries` |

### Setups
| UI element | Source |
| --- | --- |
| "9 of 11 setups deployed · 2 free" | listSetups `summary.deployed`, `summary.enabled`, `summary.free` |
| State filter (Free/Deployed/Acquired) | client from `setups[].state`, `deployed` |
| Label column and filter ("size") | listSetups `display_label_keys`, `setups[].labels` |
| Label selector box | listSetups `selector` query |
| Setup, Held by, Holder's tokens, "overdrawn" | `setups[].name`, `holder.name`, `holder.balance_tokens`, `holder.overdrawn` |
| State badge "Acquired · deploying" | `state`, `deployed` |
| Held for | `held_since` |
| Cost "8.0 / h", "N / h if acquired" | `cost_per_hour_tokens.now` |
| "Free for 1 h 20 m more" | `metering.expected_start_at`, `metering.active` |
| Weight column | `weight` |
| Details link/JSON view | `details` |
| Release only on own hold | `holder.is_you`, `holder.hold_id` |
| Acquire button | acquireSetup |
| Acquire dialog: cost now and at peak | `cost_per_hour_tokens.now`, `.at_peak` |
| "weight 1 (default rule)" | `admin.matched_rule_selector` only for admins; see open questions |
| "Charging starts when deployed or after 2 hours" | listSetups `metering_description` |
| "your 2nd of 2 allowed" | getMe `hold_count`, `hold_limit` |
| "free when the lab is below 50%" | getPressure `bands` |

### Users
| UI element | Source |
| --- | --- |
| Stat cards (Users, admins incl. local, Overdrawn, Debited, Granted, "3 grants") | listAdminUsers `summary.*` |
| Search box | client over `users[]` |
| Pending table: User, email, First signed in | listAdminUsers?status=pending `users[].name`, `email`, `first_sign_in_at` |
| Role select + Approve | setUserRoles |
| Reject | rejectUser |
| All users: User, Role, Tokens left, Burned this month, Holding | `users[].name`, `email`, `roles`, `balance_tokens`, `month_to_date_burn_tokens`, `active_holds[].setup_name` |
| Overdrawn amber / sort first | `overdrawn`; server sorts |
| Local admin row ("Admin · fixed", "No balance", "Cannot hold") | `is_local_admin`, `balance_tokens: null` |
| Role change | setUserRoles |
| Grant panel: balance, "burning 4.0 / hour on setup-02" | `balance_tokens`, `burn_per_hour_tokens`, `active_holds[]` |
| "New balance", "expire Nov 30" | client from `balance_tokens`; `period_ends_at` |
| Grant N tokens | grantTokens |
| "Reset to 300" | resetBalance; `default_allowance_tokens` |

### Setup inventory
| UI element | Source |
| --- | --- |
| Setups table (Setup, Labels, Weight) | listSetups (admin) `name`, `labels`, `weight` |
| Resource name "(override)" | `admin.resource_name`, `admin.resource_name_override` |
| Last seen / "Not deployed" | `admin.last_seen_at` (null) |
| Held by | `holder.name` |
| Status Enabled/Disabled | `admin.enabled`, `state` |
| Orphans: resource name, first seen, matches | getCollector `orphans[].resource_name`, `first_seen_at`, `setup.name` |
| Edit panel: name, labels, override, details, enabled | updateSetup / createSetup `SetupInput` |
| "Blank uses the template: setup-04_RP" | `admin.resource_name` |
| "Weight 1: no rule matches" | `admin.matched_rule_selector` null |
| Save / Add setup / Delete | updateSetup, createSetup, deleteSetup |
| Force-release | releaseHold `{force: true, reason}` with `holder.hold_id` |

### Settings
| UI element | Source |
| --- | --- |
| Last successful poll | getCollector `last_successful_poll_at` |
| Setups matched, "1 orphan" | `matched_setups`, `orphan_count` |
| Outages this month, minutes not charged | `outages_this_month.count`, `.minutes_not_charged`; history in `outages[]` |
| Provider / pressure / policy / hooks / OIDC issuer line | getSettings `runtime.*` (also getCollector `extensions`) |
| Last error, recent runs, failed deliveries, queue | getCollector `last_error`, `recent_runs`, `failed_deliveries`, `queue_length` |
| Enforcement, teardown checkboxes | `settings.enforcement`, `release_requires_teardown` |
| Allowance, max holds, poll interval | `settings.default_allowance`, `max_holds_per_user`, `poll_interval` |
| Template and preview | `settings.resource_name_template`; preview see open questions |
| Pressure bands | `settings.bands` |
| Rate rules: #, selector, weight, Matches, up/down/remove | getRules `rules[].position`, `selector`, `weight`, `match_count`; replaceRules |
| "7 setups match no rule" | getRules `unmatched_setups` (count) |
| Default weight, display label keys | `settings.default_weight`, `settings.display_label_keys` |
| Save changes | updateSettings + replaceRules |

### Audit log (spec only)
| UI element | Source |
| --- | --- |
| Rows, filters | listAudit `entries[].actor`, `action`, `target`, `before`, `after`, `created_at`; query `actor`, `action`, `limit`, `offset`; `total` |

## Decisions for docs/decisions.md
- Session cookie is named `hourglass_session`.
- `POST /auth/local` and the OIDC calls are `x-csrf: false` (no session yet); every other state-changing call is `x-csrf: true`.
- `POST /auth/local/password` returns the refreshed `Session`; `POST /auth/logout` returns 204.
- OIDC failures redirect to `/login?error=<ErrorCode>`; success to `/` or `/unauthorized`.
- `PUT /admin/settings` takes the full `Settings` (all fields required); the GET wraps it as `{settings, runtime}`. `default_weight` lives in Settings and is read-only in the rules response, so the Settings page saves with two calls (rules, settings), not atomically.
- `PUT /admin/rules` takes `{rules:[{selector, weight}]}`, position by array order.
- Grants and resets return `{entry, user}` with 201; reset requires a `reason` (LED-5; ui-notes difference 9).
- Pressure values are fractions 0..1; bands as `{base, steps:[{from, multiplier}]}`, same as Settings.
- Local admin: `balance_tokens: null`, `hold_limit: 0`; acquiring is 403 `forbidden`; grant/reset on it is 409 `local_admin_protected`.
- Release: `still_deployed` 409; non-owner or non-admin force is 403 `forbidden`; already released is 409 `conflict`.
- Extra top-level helpers inside existing routes: `summary` and `metering_description` (setups), `pending_count`/`summary`/`period_ends_at`/`default_allowance_tokens` (admin users), `available_periods` (ledger, usage), `setups_enabled`/`setups_deployed` (pressure).
- `GET /healthz` returns 503 with the same body when the database is down.

## Open questions
1. Outages: the data model has no outage table; the API assumes the backend derives them (from MET-4 skipped gaps and failed runs) or records them somewhere.
2. Metering "expected start" (`metering.expected_start_at`, wireframe "Free for 1 h 20 m more") is not in the spec; it is nullable and only the policy can supply it.
3. Developers see "weight 1 (default rule)" in the acquire dialog in the wireframe, but `matched_rule_selector` is admin-only per the brief; non-admins get only `weight`.
4. Template preview before saving ("setup-04 becomes setup-04_RP") has no route; the UI can show only the saved template's result via `admin.resource_name`.
5. Filtering the audit log by actor needs a user list; the UI can reuse `listAdminUsers`. No list of action names is offered.
6. Users-page Search and the Setups state filter are client-side (no spec route parameter).

## Confidence and gaps
The file lints clean of errors; Go handlers do not exist yet, so conformance (UI-2 contract test) is untested. Field names and error-code-to-operation assignments are my choices where the spec is silent; the 4 lint warnings remain.
