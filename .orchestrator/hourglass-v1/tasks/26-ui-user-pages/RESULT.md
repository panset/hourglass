# Task 26: UI pages My usage and Setups (status: partial, one external blocker)

## What was done
Replaced the placeholders with the My usage page (`ui/src/pages/usage/`) and the Setups page (`ui/src/pages/setups/`). Hand-drawn SVG bars and a pressure band bar, no chart library. Tests: `usage/usage.test.tsx` (10), `setups/setups.test.tsx` (15), API mocked at fetch with typed fixtures.

## Displayed values and their API fields
My usage (GET /api/v1/me, /pressure, /me/holds?state=active, /me/usage?period, /me/ledger?period)
- Tokens left: `me.balance_tokens`; sub-line `allowance_tokens`, `granted_tokens`, `carryover_tokens` (only when nonzero); "Overdrawn" badge from `me.overdrawn`.
- Burning now: `burn_per_hour_tokens`; sub-line `burning[].setup_name, weight, multiplier`.
- Projected run-out: `projection` + `projected_runout_at` ("not this month" for `not_this_month`, "not burning" for `not_burning`).
- Lab pressure: `pressure.smoothed`, `multiplier`, `bands.base/steps` (segments and aria label), `setups_deployed` / `setups_enabled`.
- Current holds heading: `me.hold_count`, `me.hold_limit`. Rows: `hold.setup_name, setup_labels, deployed, weight, cost_per_hour_tokens.now, acquired_at` (elapsed time is formatted from it), Release posts `/api/v1/holds/{hold.id}/release`.
- Tokens per day: `usage.days[].date, debited_tokens`, `total_debited_tokens`; today bar = last day when the selected period equals `me.period`.
- Month picker: union of `me.period`, `usage.available_periods`, `ledger.available_periods`.
- Ledger: `created_at, type, multiplier, setup_name, actor_name, reason, amount_tokens, balance_after_tokens`.

Setups (GET /api/v1/setups[?selector=], /me, /pressure)
- Setup `name`; one column per `display_label_keys` (`labels[key]`); `weight`; State from `state` + `deployed`; Held by `holder.name` / `is_you`; Holder's tokens `holder.balance_tokens` with `holder.overdrawn` flag ("−12 overdrawn"); Held for from `held_since`; Cost from `cost_per_hour_tokens.now` ("N / h if acquired" for free setups; for held-not-yet-metering `metering.expected_start_at` / `metering.reason`).
- Header "9 of 11 setups deployed · 2 free": `summary.deployed/enabled/free`.
- Acquire dialog: `cost_per_hour_tokens.now`, `.at_peak`, `weight`, `pressure.multiplier`, `metering_description`. Details: `setup.details` pretty-printed.
- Disabled setups (admins only, from the API): `state === "disabled"` shown with a Disabled badge; the State filter has a Disabled option for admins only.
- Release: `holder.hold_id`. Acquire disabled reasons from `me.enforcement`, `me.overdrawn`, `me.hold_count`, `me.hold_limit`.

## Gaps and notes
- The API has no explicit "cannot acquire" flag. The page compares `me.hold_count >= me.hold_limit` and uses `me.enforcement && me.overdrawn`; this repeats HOLD-1 in the UI (the API still enforces and its error is shown). A `can_acquire` / `cannot_acquire_reason` field on `/me` would remove it.
- "Held for" and "Free for … more" are the difference between an API timestamp and the browser clock (display only).
- Not built (not in spec): the wireframe's "Holding setup-04 overnight (15 h) costs 30 tokens" estimate (would be client arithmetic), "Show all entries, grouped by day" link, "your 2nd of 2 allowed" text in the Acquire dialog.
- Wireframe has Details only on others' holds; the spec lists Details as an action, so every row has it.

## Decisions for docs/decisions.md
1. Local admin on My usage: `/me`, holds, usage and ledger are not fetched; the page shows the pressure card and a one-line note.
2. Label selector box applies on Enter or a "Filter" button (not per keystroke); an `invalid_selector` message shows under the box and the previous list stays visible.
3. Release on My usage has no confirm dialog (spec asks for none); the API message shows inline on refusal.
4. Pages render defensively when optional arrays are absent (the shell tests mount these pages with minimal `/me` and `/pressure` mocks).

## Files changed
New: `ui/src/pages/usage/{index.tsx,BarChart.tsx,PressureBar.tsx,holdApi.ts,usage.module.css,usage.test.tsx}`, `ui/src/pages/setups/{index.tsx,setups.module.css,setups.test.tsx}`, this folder (`PROGRESS.md`, `RESULT.md`, `bazel-test.log`, `bazel-build.log`).

## Commands
- `bazel test //ui/...`: `//ui:test` PASS, `//ui:api_schema_drift_test` PASS, `//ui:typecheck` FAIL. All typecheck errors are in `ui/src/pages/index.tsx` and `ui/src/pages/form.ts`, stray files from another task (broken relative imports, apparently meant for an admin page directory). None are in my files.
- `bazel build //ui/...` ok. `./tools/check_spdx.sh` exit 0.

## Confidence and gaps
Verified by tests: states, endpoints, bodies, CSRF headers, role differences. Not verified: appearance in a real browser, layout against the wireframes, a real hourglassd. Timestamps use the browser time zone; tests do not assert times.
