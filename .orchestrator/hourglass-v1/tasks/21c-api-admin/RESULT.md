# RESULT 21c-api-admin

Status: done, with three 21a router tests that now fail by design (see Checks). No admin route answers 501.

## Routes (all in `internal/api/handlers_admin.go`)
| Route | Behaviour |
| --- | --- |
| `GET /admin/users?status=` | All users with roles, status, balance (null for local admin and never-funded users), month-to-date burn, burn per hour (rate-rule weight x current multiplier over metering holds), active holds; `pending_count`, `summary`, `period`, `period_ends_at`, `default_allowance_tokens`. Default order: overdrawn first, then name (case-insensitive), then id. `status=pending` is oldest first. Bad status: 400 `validation` field `status`. |
| `PUT /admin/users/{id}/roles` | One transaction: replace roles; non-empty set on a pending or rejected user sets status active, credits the allowance through `ledger.Service.EnsureFirstAllowanceTx` (at most once per period), and emits `user.approved` after commit (only on the pending/rejected to active transition). Empty set: user stays active, unassigned, not pending; refused with `has_active_hold` while they hold. Errors: `local_admin_protected`, `self_demote`, `last_admin` (cannot be reached while the local admin exists), `not_found`, `validation` (unknown or duplicate role, missing `roles`). Audit `user.roles`. |
| `POST /admin/users/{id}/reject` | Pending only (else `conflict`); local admin is `local_admin_protected`. Audit `user.reject`. |
| `POST /admin/users/{id}/grants` | 201 `{entry, user}`. Through `ledger.Service.Grant` (reason required, negative allowed, zero is `validation` field `amount_tokens`; no role is `conflict`; local admin `local_admin_protected`). |
| `POST /admin/users/{id}/reset` | 201 `{entry, user}` through `ledger.Service.Reset`. |
| `POST /admin/setups`, `PUT`, `DELETE /admin/setups/{id}` | SetupInput validation: name (trimmed, non-empty, at most 200 chars), labels (SET-3), `details` well-formed JSON at most 256 KB stored and returned byte for byte, enabled, derivable resource name (SET-4). Duplicate name `conflict`. Held setup cannot be disabled or deleted: `has_active_hold`. Delete of a setup that ever had a hold: 409 `conflict` "...disable it instead" (the schema keeps holds, so the delete cannot succeed). Replies are the admin Setup view; delete is 204. |
| `GET`/`PUT /admin/rules` | Ordered rules with first-match counts over enabled setups, default weight, fall-through setups. PUT replaces the list; invalid selector is 400 `invalid_selector`, field `rules[N].selector`, message is the parser error with its position; whole request rejected. |
| `GET`/`PUT /admin/settings` | `{settings, runtime}`. PUT takes the full object (all nine keys required, unknown rejected), validates through `internal/settings` (400 `validation` + `field`), audits only changed keys. |
| `GET /admin/collector` | Extension names, last successful poll (start time of the run, as `/healthz`), last error (newest failed run), matched setups, orphans (with the matching setup), outages (this month count and minutes, newest 50), 20 recent runs, 20 recent failed deliveries, queue length. |
| `GET /admin/audit` | Newest first, `actor`, `action`, `limit` (1..200, default 50), `offset`, `total`. Out-of-range values are 400 `validation`. |

## Audit action vocabulary
`user.roles`, `user.reject`, `ledger.grant` and `ledger.reset` (written by `internal/ledger` in its own transaction), `setup.create`, `setup.update`, `setup.delete`, `rules.replace`, `settings.update`, `hold.force_release` (written by `internal/holds`). Targets: `user:N`, `setup:N`, `rules:rate`, `settings`, `hold:N`. Setup rows record name, labels, enabled, override, `details_bytes` and `details_sha256` (never the details). Audit rows are written in the same transaction as the change.

## OpenAPI problems and deviations
1. Brief says a held setup gives `setup_held`; the OpenAPI (and SET-5 text in it) says `has_active_hold` for disable and delete. I followed the OpenAPI.
2. `Runtime` in `api.Deps` has no OIDC issuer or metering description, so `runtime.oidc_issuer` is always null and `runtime.metering_policy.description` is "". Needs two fields added to `api.Runtime` and `app.go` (outside my scope).
3. `SetupInput.resource_name_override` is listed as required; I accept it absent (treated as blank). `name`, `labels`, `details`, `enabled` are required.
4. `Outage` is described as "or a failed-provider gap"; only the `outages` table (uncharged gaps) is listed. Failed polls show in `recent_runs`/`last_error`.
5. Setup `metering.reason` comes from the newest usage sample of the hold (null before the first sample); `expected_start_at` is always null as decided.
6. `AuditEntry.actor` for a system row is `{id: 0, name: "system"}`.

## Decisions for docs/decisions.md
- "First role ever" for `user.approved` = status moves from pending/rejected to active. A user returned to unassigned and re-assigned does not get the event.
- `overdrawn` is true only for a user with a role and balance <= 0; `user_count` counts active users (including the local admin); `granted_this_month_tokens` is the net of grant entries (negative grants reduce it) and `grant_count_this_month` counts every grant entry.
- The grant/reset reply finds its ledger entry as the newest entry of that type by the acting admin; two simultaneous grants to the same user by the same admin could swap entries in the reply (the ledger itself is correct).
- Match counts and "falls through" consider enabled setups only.
- `details` is written to the response by splicing the stored bytes (encoding/json would compact them and escape `<`, `>`, `&`). Other handlers returning `details` through `json.RawMessage` do not keep SET-2 byte for byte.

## Files
New: `internal/store/apiadmin_queries.go`, `apiadmin_queries_test.go`; `internal/api/admin_common.go`, `admin_views.go`, `admin_system.go`, `admin_testenv_test.go`, `admin_users_test.go`, `admin_setups_test.go`, `admin_system_test.go`. Rewritten: `internal/api/handlers_admin.go`. Regenerated: `internal/api/BUILD.bazel`, `internal/store/BUILD.bazel`. This folder: PROGRESS.md, RESULT.md.

## Checks
- `go test -race -count=1 ./internal/store/...` passes. `./internal/api/...` passes except three tests in 21a's `router_test.go` that still expect admin (and user) stubs to answer 501: `TestAUTH4_AdminSessionReachesEveryStub`, `TestAUTH4_MiddlewareOrder401Then403ThenPasswordThenCSRF` (lines ~174-181), `TestNFR_RequestLogHasNoBodyCookieTokenOrQuery` (acquires with a Deps that has no holds service). They need updating once both handler tasks land; I did not edit `router_test.go`.
- `go build ./...` and `go vet ./internal/...` clean; `tools/check_spdx.sh` (run with bash) and `tools/check_core_generic.sh` pass.
- Bazel: see the line at the end of this file.

## Confidence and gaps
- Tests use the real router, store, ledger, holds, collector and event dispatcher; only extensions and the clock are fakes. Not tested: concurrent admin actions, a rollover pending at the moment of approval (the ledger service defers the allowance to the rollover then), `last_admin` (unreachable).
- Burn per hour uses the rate-rule weight, not a policy weight override.
- Bazel: `bazel test //internal/api/... //internal/store/...`: store and apierr pass; `//internal/api:api_test` fails only on the same three 21a router tests listed above (all other tests pass).
