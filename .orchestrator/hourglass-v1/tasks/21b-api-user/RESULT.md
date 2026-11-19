# RESULT 21b-api-user

Status: done, with one follow-up for the orchestrator (stale 21a tests, below).

## Files changed
- `internal/api/handlers_user.go` (8 handlers), `internal/api/user_views.go` (shapes, per-request lab loader), `internal/api/user_testenv_test.go`, `internal/api/user_test.go`
- `internal/store/apiuser_queries.go`, `internal/store/apiuser_queries_test.go`
- `internal/api/BUILD.bazel`, `internal/store/BUILD.bazel` (gazelle). This folder: PROGRESS.md, RESULT.md.
- Not touched: router.go/routes.go/api.go/helpers.go/handlers_admin.go. My helper names are prefixed (`userCaller`, `nullIfEmpty`, `utcTime`, ...) because the admin task's `admin_common.go` already defines `strPtr`.

## Routes and where each response field comes from
All amounts are milli-tokens / 1000; times are UTC RFC 3339. Every route is behind the role chain; no role gives 403 `no_role`, no session 401, with no data.
- `GET /me`: `period`, `period_ends_at` = `ledger.PeriodOf` / `NextBoundary` (configured tz). `balance_tokens` = `SumLedger(period)` (LED-2; null for the local admin). `granted/carryover/debited` = ledger sums by type for the period. `allowance_tokens` = the user's `allowance` entries this period, else the `default_allowance` setting (0 for the local admin). `burning[]`/`burn_per_hour_tokens` = for each active hold with metering started, `metering.HourlyCostMT(weight, multiplier)`; weight = last usage sample's weight (policy override) else the resolved rate-rule weight; multiplier = latest `pressure_samples` row (else bands base). `projection`/`projected_runout_at` = `metering.ProjectedRunOut(balance, SumDebitsSince(24h))`: `not_burning` when no result, `not_this_month` when at or after `period_ends_at`, else `date`. `hold_count` = `CountActiveHolds`; `hold_limit`, `enforcement` = settings (limit 0 for the local admin); `overdrawn` = balance <= 0.
- `GET /me/holds?state=active|released`: `UserHolds` (newest first), Hold view below; bad `state` is 400 `validation` (field `state`).
- Hold view (also acquire/release responses): row fields from `holds`; `setup_*` from the setup; `weight` as above; `deployed` = active and the collector's last successful poll reports the resource name (`settings.ResourceName`); `metering.active/started_at` from the hold, `metering.reason` = reason of the newest usage sample (null before any), `expected_start_at` always null; `cost_per_hour_tokens` = `{now, at_peak}` at the current multiplier and `Bands.Max()`, null once released; `charged_tokens` = ledger debits with that `hold_id`; `released_by` = actor id and name; `forced` = released by someone other than the holder.
- `GET /me/ledger?period=`: `ListLedger` reversed (newest first). `balance_after_tokens` = running sum of the period's entries. `multiplier` = the usage sample at the debit's (hold, poll time), null if pruned. `setup_name` via the hold, `actor_name` via `actor_id`. `available_periods` = periods with entries, newest first, always including the current one. Bad period: 400 `validation`, field `period`.
- `GET /me/usage?period=`: ledger debits bucketed by local calendar day in the configured timezone, zero-filled; current period lists days up to today, past period the whole month, future period none; `total_debited_tokens` is the whole period.
- `GET /pressure`: latest `pressure_samples` row (raw, smoothed, multiplier, source, updated_at; zeros and null before the first poll, source falls back to `Runtime.PressureSource`), `bands` from settings, `history` = samples of the last 7 days oldest first, `setups_enabled`/`setups_deployed` over enabled setups.
- `GET /setups?selector=`: `ListSetups` (enabled only for non-admins), `rules.Parse` (400 `invalid_selector`, field `selector`, message = parse error), weight and `admin.matched_rule_selector` from `rules.Weight`, holder name/balance from the user rows (cached balance, visible to every role by design), `held_since` = acquired_at, `metering` as above (null unless held), `cost_per_hour_tokens` at the resolved weight, `details` returned unmodified, `admin` = null for non-admins, else `{enabled, resource_name, resource_name_override, last_seen_at (collector LastSeen), matched_rule_selector}`. `summary` counts all enabled setups ignoring the selector. `display_label_keys` from settings. State: `held` wins over `disabled`.
- `POST /setups/{id}/acquire`: `holds.Service.Acquire`; 201 with the Hold. Errors go through `writeError`: forbidden (local admin) 403, not_found 404, setup_disabled/setup_held/insufficient_tokens/hold_limit/hook_veto 409.
- `POST /holds/{id}/release`: optional body `{force, reason}` (empty body is a normal release; unknown fields or bad JSON 400); `holds.Service.Release`; 200 with the Hold. forbidden 403, not_found 404, still_deployed 409, missing force reason 400 `validation` with field `reason` (I add the field; the holds package leaves it blank).

## OpenAPI problems and gaps (OpenAPI not edited)
1. `SetupList.metering_description` ("text from the metering policy") and `MeteringState.reason` for a hold that is not metering yet have no source: `MeteringPolicy` has no describe method, `Deps` carries only `Runtime.MeteringPolicy` (a name), and the policy's reason is only stored in usage samples, which exist only for metering holds. I return "Charging follows the \"<name>\" metering policy." and a null reason until the first sample. To show "Free for 1 h 20 m more" and "deployed or 2 h after acquire" the policy needs a describe/preview hook exposed through `Deps`; needs a decision.
2. `POST /holds/{id}/release`: OpenAPI says an already released hold is 409 `conflict`; `internal/holds` answers 404 `not_found` (HOLD-2 decision 17c). I kept the service's behaviour. Tell me if 409 is wanted.
3. `GET /me/usage`: the OpenAPI text says days run up to today for the current period; the brief says every day of the month. I followed the OpenAPI.
4. `Me.allowance_tokens` for the local admin is 0 (the field is not nullable).

## Decisions for docs/decisions.md
- Current multiplier comes from the newest `pressure_samples` row, not the in-memory poll view, so it survives a restart; no samples means the bands' base.
- Setup `weight`/cost use the rate-rule weight; a hold's weight uses the last sample's weight when there is one.
- `available_periods` always includes the current period.
- Setup state `held` takes precedence over `disabled` for a disabled setup that still has a hold.
- Ledger/usage use ledger entries (kept forever), not `usage_samples` (pruned at 13 months).

## Checks (all run)
- `go test -race -count=1 ./internal/store/...` ok; `./internal/api/...`: all new tests pass. Three older 21a tests fail because they expect 501 stubs and cannot be edited by me: `TestAUTH4_AdminSessionReachesEveryStub`, `TestAUTH4_MiddlewareOrder401Then403ThenPasswordThenCSRF`, `TestNFR_RequestLogHasNoBodyCookieTokenOrQuery` (`router_test.go:80,174-181,338`; they also still expect 501 for admin routes, now implemented by the other agent). They need updating once both tasks are in.
- `go build ./...`, `go vet ./internal/...`, `gofmt -l`, `tools/check_spdx.sh`, `tools/check_core_generic.sh`: clean. Gazelle run on `internal/api` and `internal/store` last.
- `bazel test //internal/api/... //internal/store/...`: store and apierr pass; `//internal/api:api_test` fails only on the three stale tests above.

## Tests (user_test.go; names cite requirement ids)
Anonymous and no-role (401/403, no data) on all 8 routes; no 501; me (shape, projection date/not_this_month/not_burning, local admin, overdrawn); my holds (shape, state filter, released view); ledger (shape, periods, grant actor, bad periods, own entries only); usage (zero fill, past/future, timezone); pressure (before/after polls, counts, 7-day cut); setups (developer, admin, selector filter and invalid selector, summary); acquire (success and every refusal incl. veto message, CSRF); release (owner, forced, every refusal, audit row).

## Confidence and gaps
- Verified by the tests above. Not tested: a hold whose setup has a resource-name template error (returns 500 `internal`); more than 500 holds in one list (store query chunks ids, tested at 502).
