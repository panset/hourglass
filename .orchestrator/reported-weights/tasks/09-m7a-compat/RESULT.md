# Result, task 09: compatibility tests against the v1.1.0 fixtures (WGT-19, WGT-20)

Status: done. Tests only; no product code, fixture, existing test, `api/openapi.yaml`, `ui/`, `e2e/` or `docs/` file was touched.
`./ci.sh` exit code 0 (`ci.exit`, `ci.log`; run twice, the second after the race fix below). The new tests ran under Bazel and passed, five runs each, uncached (`bazel-wgt.log`, `bazel-wgt.exit` 0). `tools/check_core_generic.sh` passes.

## Differences found between v1.1.0 and today's code

With reported weights off and no override, replaying every recorded request of `responses.json` in order, and the routes of `poll.json` after the three recorded polls, the only differences are the design's added fields and the two deliberate A3 changes. Nothing is removed, nothing unexpected changed, no status code differs.

Named changes (exact values):
- A3 change 1, right after the upgrade (the known difference from verification 07). The hold of `lab-08` (priced by the policy, 3.25, samples written before the migration have no source) shows the resolver's weight 1.25 and source `default`:
  - `GET /api/v1/me` (`me_dev2`): `burn_per_hour_tokens` 14.5 -> 10.5; `burning[].tokens_per_hour` 6.5 -> 2.5; `burning[].weight` 3.25 -> 1.25.
  - `GET /api/v1/me/holds` and `?state=active` (`me_holds_dev2`, `me_holds_dev2_active`): `holds[].weight` 3.25 -> 1.25; `holds[].cost_per_hour_tokens.now` 6.5 -> 2.5; `.at_peak` 9.75 -> 3.75.
  - After one poll on the new code the hold shows 3.25 and source `policy` again (My holds, My usage burn 14.5, Admin Users burn 14.5). Tested by name (see table).
- A3 change 2, visible after the polls: `GET /api/v1/admin/users` (`admin_users_after_polls`) `users[].burn_per_hour_tokens` of `dev2` 2.5 -> 6.5 (burn honours the policy's weight). Before the first poll there is no difference (`admin_users` identical): the resolver gives 1.25 for lab-08 there, as the rate rules did.

Added fields seen, per route: see "Allowed differences" below. Every other recorded route (the 403/401 answers, `me_other`, `me_admin`, `me_usage_*`, `pressure_dev`, `admin_rules`, `admin_service_accounts`, `admin_users*`, the integration 401s) is identical.

Replay artifact, not a product difference: the fixture stores request bodies decoded, which loses key order, and the setup audit entry hashes `details` as sent. The one request where it matters (`admin_setup_post`, details `{"note":"opaque","n":[1,2]}`) is sent with the recorder's exact bytes (taken from the recorder text; the test checks that it decodes to the recorded body). Without that, the audit `details_sha256` differs.

Application start (WGT-19): the application's first poll runs at start. With the test's config (timezone only, no provider) it completes the pending release of hold 3 (its setup is not deployed in this run) and appends to `usage_samples`, `collector_runs`, `pressure_samples`, and the start adds the `seed_state` row to `settings`. These are poll and seed effects, not migration effects; the migration alone (`store.Open`) rewrites nothing (exact row equality of all 24 old tables, new columns null, new tables empty).

## Allowed differences per route (explicit in the tests: `v110Routes`, `v110RoutesAfterPolls`)

Shorthand: LIST = `reported_weights_enabled`, `setups[].weight_source`, `setups[].weight_attributes`; ADMIN = LIST + `setups[].admin.reported_weight`, `.reported_weight_not_applied`, `.weight_override`; HOLDS = `holds[].weight_source`, `holds[].weight_attributes`; LEDGER = `entries[].weight_min`, `weight_max`, `weight_sources`; SETUP = `weight_source`, `weight_attributes`, `admin.reported_weight`, `admin.reported_weight_not_applied`, `admin.weight_override`; STATE = `weights`, `setups[].weight`. Each listed addition must appear; any other addition, any removal and any other changed value fails.

| Route (recorded name) | Allowed additions | Named changed values |
| --- | --- | --- |
| setups_dev, setups_dev_selector, setups_dev2, (after polls) setups_dev_after_polls | LIST | none |
| setups_admin, setups_admin_after_writes, setups_admin_after_polls | ADMIN | none |
| session_dev | none | `csrf_token` ignored (random per session) |
| me_dev | `burning[].weight_source` | none |
| me_dev2 | `burning[].weight_source` | A3 change 1 (three values above) |
| me_holds_dev | HOLDS | none |
| me_holds_dev2, me_holds_dev2_active | HOLDS | A3 change 1 (three values above) |
| me_holds_dev2_after_polls | HOLDS | none (weight 3.25, source `policy` as before the upgrade) |
| me_ledger_dev, _dev_day, _dev2, _dev2_day, me_ledger_dev2_after_polls | LEDGER | none |
| admin_settings, admin_settings_put | `settings.reported_weights` | none |
| admin_collector, admin_collector_after_polls | `last_weights_report`, `last_weights_refusal` | none |
| integration_state_reporter, integration_state_viewer | STATE | none |
| admin_setup_post, _post_no_rule, _put, _put_held | SETUP | none |
| admin_users_after_polls | none | A3 change 2 (`users[].burn_per_hour_tokens` 2.5 -> 6.5, once) |
| every other recorded route | none | none |

## Requirement to test

| Id | Test | File |
| --- | --- | --- |
| WGT-19 collector polls (unchanged, kept) | `TestWGT19_ReplayV110PollsWithFeatureOff` | `internal/collector/upgrade_v110_replay_test.go` |
| WGT-19 migration 7 -> 8, no row rewritten, new columns null | `TestWGT19_MigrationRewritesNoRowOfV110Database` | `internal/app/upgrade_v110_test.go` |
| WGT-19 real application start, balances, period sums and per-hold charges equal the recorded ones | `TestWGT19_ApplicationStartUpgradesV110DatabaseAndKeepsEveryNumber` | `internal/app/upgrade_v110_test.go` |
| WGT-20 answers of every route of `responses.json`, existing requests without the new fields (settings PUT with eight keys, setup create and update without `weight_override`) | `TestWGT20_ResponsesDifferFromV110OnlyByTheNamedAdditionsAndChanges` | `internal/api/upgrade_v110_compat_test.go` |
| WGT-20 the same after the three polls | `TestWGT20_ResponsesAfterThePollsDifferFromV110OnlyByTheNamedAdditionsAndChanges` | same |
| WGT-20 the named A3 change and the return to the policy weight after one poll | `TestWGT20_PolicyPricedHoldShowsResolverWeightUntilTheFirstPollThenThePolicyWeight` | same |
| WGT-20 contract: nothing removed or renamed; no request gained a required field, parameter or body | `TestWGT20_NoRouteParameterOrFieldOfV110IsRemovedAndNoRequestGainedARequiredField` | same |
| WGT-20 state route with a rule that cannot be evaluated (task 07b, existing) | `TestWGT20_StateAnswersAsBeforeWithARuleThatCannotBeEvaluated_FeatureOff` | existing |

A missing fixture fails (`t.Fatalf`), never skips, in all new tests.

## Break checks (`breaks.log`; each restored, `git diff` shows no product change)

| Id | Break | Result |
| --- | --- | --- |
| WGT-19 | migration appends `UPDATE usage_samples SET weight_source = 'default'` | both app tests red |
| WGT-19 | migration appends `UPDATE setups SET weight_override = 1` | both app tests red |
| WGT-19 | collector debit uses `weight*1.01` | `TestWGT19_ReplayV110PollsWithFeatureOff` red |
| WGT-20 | `burning[].tokens_per_hour` json tag renamed | answers test red |
| WGT-20 | extra field `surprise` in `GET /api/v1/setups` | answers test and after-polls test red |
| WGT-20 | `tokens_per_hour` value + 1 | answers test red |
| WGT-20 | `reported_weights` added to the Settings `required` list | contract test red (names the field) |
| WGT-20 | `default_weight` removed from the contract file | contract test red (3 items named) |
| WGT-20 | A3 rule changed: a sample with no source counts as `policy` | answers test and named-change test red |

## Files changed
- `internal/app/upgrade_v110_test.go` (new)
- `internal/api/upgrade_v110_compat_test.go` (new)
- `internal/app/BUILD.bazel` (srcs, data)
- `internal/api/BUILD.bazel` (srcs, data)
- `BUILD.bazel` (root: exports `responses.json` and `openapi.yaml` of `testdata/upgrade-v1.1.0`, which only `poll.json` and the database had; outside the brief's "may write" list, but needed to wire the fixture, which the brief asks for)
- `.orchestrator/reported-weights/tasks/09-m7a-compat/` (PROGRESS.md, RESULT.md, ci.log, ci.exit, breaks.log, bazel-wgt.log, bazel-wgt.exit)

## Commands
- `go test -count=1 -run 'TestWGT19_|TestWGT20_'` in `internal/app`, `internal/api`, `internal/collector`: pass.
- `./ci.sh`: exit 0.
- `bazel test //internal/api:api_test //internal/app:app_test //internal/collector:collector_test --cache_test_results=no --runs_per_test=5 --test_filter='TestWGT19_|TestWGT20_' --test_arg=-test.v --test_output=all`: exit 0, every named test passed five times.
- `tools/check_core_generic.sh`: exit 0.

## Decisions for docs/decisions.md
1. The WGT-20 test compares by a generic JSON differ with an explicit per-route list of added paths and named value changes (path, was, now, count). Removal is never allowed, a listed difference that does not occur fails, so a list cannot go stale.
2. The first deliberate A3 change is named in the test as "A3 change 1" and its exact values are listed; the return to the policy weight and source `policy` after one poll is its own test.
3. The application-level WGT-19 is two tests: the migration alone (exact rows of every old table) and the real `Run`, whose first poll legitimately changes hold 3 and appends rows; the latter allows exactly that and names the tables that may grow. It waits for the first poll's collector run before stopping.
4. The contract test also checks that no request gained a required field, parameter or body; the optional `reported_weights` object's four required fields and the new route's fields are exempt (the key and the route are new).
5. Root `BUILD.bazel` exports two more fixture files.

## Confidence and gaps
- Verified by running: everything above. The app-level start uses the test's config (timezone only, no provider) and a fake clock at 2026-11-04T12:00Z, as the `d96a950` test does, so compaction does not run; compaction after the upgrade is not re-tested for v1.1.0 data (UPG-1 does it for the older fixture). The recorded app configuration of v1.1.0 (provider `reported` etc.) is not used at app level; the collector-level test and the API tests use the recorded runtime names with fakes.
- Only features off is compared, as the brief asks.
- A race in my first version of the app-level test (stopped before the first poll finished) was found by a Bazel run and fixed; 20 local and 5 Bazel runs pass.
