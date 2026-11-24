# 07 Independent verification of milestones 4 and 5: result

Status: **done**. Baseline `626b85d` (`baseline.sha`). I worked from the design, D80 to D85, the diff `e65477b..HEAD` and the tree. I did not read the RESULT, PROGRESS or mutation files of tasks 05 and 06.

Verdict: every id in scope (WGT-7 to WGT-11, WGT-13 to WGT-16, WGT-18, WGT-21 to WGT-23) **holds**. I applied 38 deliberate breaks in product code, and a test named after the id went red for each one. No high or medium findings. There are four low or informational findings.

## Findings, ranked by severity

No high or medium findings.

1. **Low: the state route can now fail with 500 when the reported-weights feature is off.** This is a v1.1.0 route that read no rule before (D84(g), marked REVIEW).
   - Code: `internal/api/handlers_integration.go:311` calls `l.resolve`. That calls `weights.Resolve` (`internal/weights/weights.go:113`), which always evaluates the rate rules.
   - Scenario I ran: a rate rule whose stored selector is `tier in (`, with `reported_weights.enabled=false`. `GET /api/v1/integration/state` answers `500 internal`. In `v1.1.0` it answered 200.
   - Context: rules are validated when saved, so this needs a damaged database. In that state polls and the Setups list fail too.
   - It is still a new failure mode on an existing integration route with the feature off. It also goes beyond A1, which names only the collector, the user views and the admin views. Owner to confirm or choose the admin-view fallback (resolve without rules).
2. **Low: top-level keys of the weights report match regardless of case; entry keys must match exactly.**
   - Code: `decodeJSON` (`internal/api/helpers.go:51`) uses `encoding/json`, which matches field names without regard to case. Entries are checked by exact key (`internal/api/handlers_weights.go:195`).
   - Scenario I ran: `{"WEIGHTS":[]}` is accepted with 200 and replaces the stored report with an empty set, so every setup falls back. `{"weights":[{"Setup":"lab-01","weight":1}]}` is refused, as designed.
   - A duplicate top-level `weights` key is also silently resolved: the last one wins.
   - This is shared behaviour of every route, so it is not a regression. It is a way to empty the report by mistake. Either document it or decode the top level strictly.
3. **Info: after upgrade and before the first poll, a hold priced by a policy weight shows the rule weight. A3 asks for this.**
   - Code: `internal/api/user_views.go:209`. A sample with a null `weight_source` counts as not `policy`, as A3 says.
   - Scenario I ran: replaying `testdata/upgrade-v1.1.0/responses.json` with the feature off. `me_dev2`, `me_holds_dev2` and `me_holds_dev2_active` show `lab-08` at 1.25 (cost 2.5 and 3.75) where `v1.1.0` showed its policy weight of 3.25 (cost 6.5 and 9.75). `burn_per_hour_tokens` goes from 14.5 to 10.5.
   - The next poll still charges 3.25 and writes a `policy` sample. From then on the screens show 3.25 again.
   - I count this as part of A3's first deliberate change, not a new difference. But "the screen matches the charge" does not hold for that window, and the WGT-20 test of task 09 must classify it. The upgrade notes in `docs/deploying.md` should mention it.
4. **Info: a clock-skew refusal names no field.** A future `observed_at` gets `400 validation` without `field` (`observed_at is more than 2 minutes in the future`), while a malformed one carries `field: observed_at`. This is shared `internal/reported` behaviour and the same for the other reports. No action needed.

Apart from finding 3, the `v1.1.0` replay showed no difference beyond the added fields. My harness caused three artifacts, not the product: a different `csrf_token` because the session id differs, and two `details_sha256` values because I re-marshalled the request body and that reorders its keys. Every other route, status and value matched (`compat-replay.log`).

## Per-requirement table

Each mutation was applied in a scratch copy of the tree, the named tests were run with `go test -count=1 ./internal/api -run TestWGTn_`, and the file was then restored. The full list of 38 is in `mutations.log`.

| Id | Verdict | Code (main) | Tests | Mutation(s) | Result |
| --- | --- | --- | --- | --- | --- |
| WGT-7 | holds | `handlers_weights.go:104-176`, `reported/weights.go:143` | TestWGT7_ReportReplacesTheWholeSet, _ReadersNeverSeeAMixOfTwoReports, _OrderingAndClockSkew, _OrderingIndependentOfTheDeploymentsReport, _ReportSurvivesARestart, _ReportNeverPollsDebitsOrEmits (+ store/reported WGT7 tests) | observed_at ignored; only the first entry stored | red, red |
| WGT-8 | holds | `handlers_weights.go:183-237`, `reported/weights.go:104` | TestWGT8_EachInvalidEntryRefusesTheWholeReport, _FirstInvalidEntryReported, _LimitsInclusiveAndNegativeZero, _BodyRefusals, _EntryAndBodyLimits, _LastRefusalKeptForAdmins | unknown field allowed; name dropped; string weight; refusal not kept; refusal kept for 409; no minimum; max exclusive | all red |
| WGT-9 | holds | `handlers_weights.go:160-174` | TestWGT9_UnknownNameIgnoredAndListed | unknown counted as accepted | red |
| WGT-10 | holds | `handlers_weights.go:86-90` | TestWGT10_OffAnswersNotConfiguredWhateverTheBody | body checked before the switch | red |
| WGT-11 | holds | `routes.go:90`, `auth/service.go:44-51` | TestWGT11_RouteNeedsWeightsWriteAndTheScopeOpensNothingElse, _ScopeAddedLastAndNotGrantedToExistingAccounts, permissions matrix | route scope changed | red |
| WGT-13 | holds | `handlers_weights.go:264-320`, `handlers_integration.go:299-316` | TestWGT13_StateWeightFields | not_applied dropped; times before the first report; selector leaked | all red |
| WGT-14 | holds | `user_views.go:193-215`, `admin_views.go:147-153,225`, `handlers_user.go:850` | TestWGT14_EveryReaderShowsTheWeightThePollCharges | hold ignores policy; Admin Users ignores policy; old last-sample holdWeight; Setups ignores report; admin view ignores override | all red |
| WGT-15 | holds | `handlers_user.go:370-415,502`, `store/apiuser_queries.go:171-218` | TestWGT15_LedgerEntriesShowTheWeightsOfTheirPolls, TestWGT15_StoreDebitWeightAggregates | count check off; null sources ignored; raw window for rolled-up rows; zero debits counted; policy last; page cut; UTC day | all red |
| WGT-16 | holds | `weights/weights.go:146-150`, readers above | TestWGT16_AttributesAreForDisplayOnly | expired shows attributes; list hides attributes | red, red |
| WGT-18 | holds | `handlers_admin.go:466-484,491-500,568-588` | TestWGT18_WeightOverride | absent clears; not audited; bound by 20 | all red |
| WGT-21 | holds | resolver by name; `handlers_admin.go:572,582` | TestWGT21_StoredWeightFollowsTheNameAndTheSwitch | rename clears the override | red |
| WGT-22 | holds | `admin_system.go:239-268` | TestWGT22_CollectorViewShowsTheWeightsReportAndTheLastRefusal, TestWGT22_TighteningTheMaximumShowsToTheAdmin | exclusive max; shown when off; message replaced | all red |
| WGT-23 | holds | `user_views.go:195-197` | TestWGT23_ReleasedHoldShowsItsLastCharge | resolves now; null source shown as rule | red, red |

No id kept its named tests green under a real break.

## Over HTTP (throwaway tests in the scratch copy, deleted with it)

**Report route.** Every refusal below gave 400 `validation`, `field` `weights`, with the position and, when the entry has a name, the name. After each one the stored report was unchanged and the collector view's `last_weights_refusal` carried the same message.
- Refused: a string weight, null, a boolean, `1e400`, `-1e400`, 20.0000001 (above the maximum), -0.0001 (below the minimum), a missing weight, 17 attributes, a 64-character value, a bad key, a non-string value, attributes given as a list, an unknown field in an entry, an entry that is a number or null, a duplicate name (`weights[1] (setup "lab-01"): setup is repeated (first at weights[0])`), an empty, numeric or missing name (position only), and a bad second entry (`weights[1]`).
- Refused without a position: 10,001 entries; a body over 1 MB; `NaN` and `Infinity` (invalid JSON); an unknown top-level field; a missing, null or non-list `weights`.
- Accepted: exactly 0, exactly 20, `-0` (stored as 0), `2e1`, an empty list, null attributes, and 63 multi-byte characters.

**Ordering.**
- With the feature off, a stale, an invalid, a non-JSON and an empty body all get 409 `not_configured`, naming `reported_weights`.
- A stale report with an invalid entry gets 400; the body is checked before ordering. A stale valid report gets 409 `stale_report`.
- An equal `observed_at` replaces the report. Exactly 2 minutes ahead is accepted; 2 minutes and 1 second ahead is 400.
- A 409 never replaces the kept refusal.
- 30 concurrent reports never left a mixed set.
- A report wrote no collector run, ledger row or event.

**Permissions.**
- Over all 44 routes, a token holding only `weights:write` opens nothing but this route and the public routes.
- An account with every other scope gets 403 on the route. No credential and a session cookie (admin or developer) get 401.

**State route.**
- Every B4 field is present, with the right `not_applied` precedence: `out_of_bounds` with an override gives `out_of_bounds`; a usable entry with an override gives `override` and keeps its attributes; expired comes first.
- With the feature off, `reported` and `not_applied` are null and the times are null. Before the first report the times are null.
- Nothing about selectors, rules, balances or audit appears.

**Ledger.** Paging with `limit=3` (26 pages) and `rollup=day&limit=1` gave each entry the same weight fields as the unpaged view.

## Test counts

- `go test -count=1 ./...`: exit 0. 747 top-level tests passed and 1,138 subtests passed, with 0 failed and 0 skipped. 72 of the top-level tests are TestWGT* (`gotest.json.gz`).
- `./ci.sh`: exit 0 (`ci.log`).
- `bazel test //... --nocache_test_results`: exit 0, 40 of 40 targets executed (`bazel.log`).
- Under Bazel with `-test.v`, over the seven packages that hold WGT tests: 72 TestWGT* passed (api 34, collector 14, reported 12, weights 6, store 4, settings 1, inventory 1), the same set as `go test` (`bazel-verbose.log`).
- Scale test (`HOURGLASS_SCALE_TEST=1 ... NFR_ApiUnder500msAtSpecScale`): pass, 73.8 s (`scale.log`).

## Existing tests (`git diff 0d20688..HEAD`)

None is weakened. Every edit to a test that existed before this work is one of these, and each is additive or something the design requires:
- added keys in exact key-set assertions: `admin_setups_test.go`, `admin_system_test.go`, `user_test.go`, `ledger_view_test.go`, `setups.test.tsx`, `usage.test.tsx`;
- the route count 43 becomes 44, with a probe and coverage entry for the new route: `router_test.go`, `permissions_test.go`, `service_accounts_test.go`;
- the setting is turned on in the contract test and the integration test, which also gains three new calls: `contract_test.go`, `integration_reports_test.go`;
- the INT-8 allow-list gains the new keys, with `attributes` treated as free-form like `labels`: `integration_reports_test.go`;
- schema version 7 becomes 8, and the schema column list gains the new columns: store tests and `app/upgrade_test.go`;
- `upgrade_compat_test.go` (UPG-2, d96a950) requires each added field to be present and strips it before the whole-body comparison. Every other comparison is unchanged and the d96a950 fixtures are untouched;
- a `dbPath` test-environment option: `testenv_test.go`, `collector_test.go`.

No edit is an A3 change. D85(j) says the same, and my reading of the diff confirms it.

## Contract and rules

**OpenAPI** (`api/openapi.yaml` against `testdata/upgrade-v1.1.0/openapi.yaml`):
- No path, operation, parameter, response code, schema or property is removed or renamed.
- The only new path is `PUT /api/v1/integration/weights`, with `x-roles: service`.
- Request schemas gain only optional properties: `SetupInput.weight_override` and `Settings.reported_weights`. Neither is in `required`.
- Response schemas gain the fields named in C2 and B4, and `LedgerEntry`'s three fields are optional, as for `entry_count`.
- The only changed property text is the description of `Setup.weight`, `Hold.weight`, `Burning.weight` and `matched_rule_selector`, which matches the "fields whose meaning shifts" list.
- `//ui:api_schema_drift_test` passed under Bazel, so `ui/src/api/schema.gen.ts` is current.

**Rules:**
- `tools/check_core_generic.sh` reports OK.
- `git grep` finds `lab-connector`, `mem-gb` and `disk-gb` only in the design.
- `docs/SPEC.md` and the design are unchanged since `9cabece`.
- `rules.Weight` is called only by the resolver (`internal/weights/weights.go:113`). `holdWeight` and the Admin Users burn go through it, and so do the admin views, the Setups list, the state route and the acquire response (`handlers_user.go:901`).
- Nothing keeps a removed weight.

## Files changed

Only this task folder:
.orchestrator/reported-weights/tasks/07-verify-m5/PROGRESS.md
.orchestrator/reported-weights/tasks/07-verify-m5/RESULT.md
.orchestrator/reported-weights/tasks/07-verify-m5/mutations.log
.orchestrator/reported-weights/tasks/07-verify-m5/compat-replay.log
.orchestrator/reported-weights/tasks/07-verify-m5/baseline.sha
.orchestrator/reported-weights/tasks/07-verify-m5/gotest.json.gz
.orchestrator/reported-weights/tasks/07-verify-m5/gotest.exit
.orchestrator/reported-weights/tasks/07-verify-m5/gotest.stderr
.orchestrator/reported-weights/tasks/07-verify-m5/ci.log
.orchestrator/reported-weights/tasks/07-verify-m5/ci.exit
.orchestrator/reported-weights/tasks/07-verify-m5/bazel.log
.orchestrator/reported-weights/tasks/07-verify-m5/bazel.exit
.orchestrator/reported-weights/tasks/07-verify-m5/bazel-verbose.log
.orchestrator/reported-weights/tasks/07-verify-m5/scale.log
.orchestrator/reported-weights/tasks/07-verify-m5/scale.exit

All mutations and throwaway tests were in a scratch copy outside the repo. A read-only `sqlite3` query left `-shm` and `-wal` files beside `testdata/upgrade-v1.1.0/hourglass.sqlite3`. I deleted them; the fixture itself is byte-identical to git. `git status --short` shows nothing of mine outside this folder.

## Decisions for docs/decisions.md

None taken. Owner questions: finding 1 (state route on an unevaluable rule) and finding 3 (classify it in WGT-20 and in the upgrade notes).

## Confidence and gaps

- I did not exercise a real `hourglassd` process. All HTTP probes used the test environment's real router and handlers in-process.
- I did not replay `poll.json` and `responses_after_polls` myself. M3's `TestWGT19_ReplayV110PollsWithFeatureOff` covers the polls; the post-poll responses are task 09's WGT-20.
- I did not check the UI (milestone 6).
- I did not run a full mutation campaign of the resolver (milestone 3, verified by task 04). WGT-16's attribute rule lives there, and I broke it only at the readers.
- "Every reader equal for one state and time" (WGT-14) I checked through the existing test plus five mutations, not by building a second independent database state.
