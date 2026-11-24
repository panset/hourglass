# 06 Milestone 5: every other reader on the resolver, ledger fields, setup input, the collector view — RESULT

Status: **done**. `./ci.sh` exit 0 (`ci.log`, `ci.exit`). `go test ./...` exit 0 (`gotest.log`). `go test -race` on the touched api tests exit 0 (`race.log`). Full-size scale test exit 0 (`scale.log`). UI typecheck and vitest (218 tests) pass with the regenerated types (`ui.log`). `tools/check_core_generic.sh` exit 0.

## What was built

1. **A3, every reader on the resolver.** No reader in `internal/api` computes a weight on its own any more (`rules.Weight` is called only by `internal/weights`; checked by grep).
   - `lab` (user routes and the state route) loads the stored weights report once per request with the settings (`weightReport`, `internal/api/handlers_weights.go:283`; read only when the setting is on) and resolves at the request's time: `lab.resolve` (`internal/api/user_views.go:114`). The state route now uses the same `lab.resolve` and `l.report` (`handlers_integration.go:299`) instead of its own read.
   - **Setups list** and its cost per hour: the resolver's weight and source (`handlers_user.go`, `listSetups`). A rule that cannot be evaluated fails the request, as before.
   - **Hold views, My usage burn**: `lab.holdWeight` (`user_views.go:193`). Active hold: the resolver's weight and source, except when the latest sample's source is `policy`, then that sample's weight and `policy` (`activeWeight`, `:208`; a null source counts as not policy). Released hold: its last sample's weight and source (null when the sample has none), attributes `{}`; no sample: what the setup resolves to now.
   - **Admin setup views** (create/update responses and the admin block of the Setups list): `adminState.resolve` (`admin_views.go:147`); a rule that cannot be evaluated falls back as if no rate rule existed (the default weight, but an override or a usable report still applies).
   - **Admin Users burn**: the same `activeWeight` rule as the hold (`admin_views.go:225`), from the latest samples of the active holds, which `loadAdminState` now reads in one `HoldLastSamples` call.
2. **C2 fields (added only).** `GET /api/v1/setups` top-level `reported_weights_enabled`; `Setup` (list and admin views) `weight_source`, `weight_attributes`; the admin block `reported_weight`, `reported_weight_not_applied`, `weight_override`; `Hold.weight_source` (nullable, may be `policy`) and `weight_attributes`; `burning[].weight_source`; `GET /api/v1/admin/collector` `last_weights_report {observed_at, received_at, service_account, entries, out_of_bounds}` and `last_weights_refusal {at, service_account, message}`, both null while the setting is off (`collectorWeights`, `admin_system.go:241`).
3. **C1 ledger fields.** `LedgerEntry.weight_min`, `weight_max`, `weight_sources` on `GET /api/v1/me/ledger` only (the grant/reset responses use another struct, so the fields are absent there). Store: `DebitWeightAggregates` (`internal/store/apiuser_queries.go:172`): one statement for any number of entries; the windows are bound as one JSON array (`json_each`) and each is a range seek on `UNIQUE (hold_id, polled_at)` (query plan checked: `SEARCH u USING INDEX sqlite_autoindex_usage_samples_1 (hold_id=? AND polled_at>? AND polled_at<?)`). API: `debitWeightWindows` (`handlers_user.go:370`) gives a raw row its one poll `[created_at, created_at + 1 ms)` and a rolled-up row or a merged `rollup=day` entry the hold's local day in the configured zone; `entryWeights` (`:396`) applies the partial-data rule (range null unless the count of samples with a debit above zero equals `entry_count`; sources null when the range is or any sample has no source) and orders sources `policy, override, reported, rule, default`. Only the page's entries are looked up.
4. **D2 `weight_override`** on `POST`/`PUT /api/v1/admin/setups` (`parseSetupInput`, `handlers_admin.go:470`): absent / `null` / a number from 0 to 1e12 (finite, `-0` stored as 0, not bound by the reported limits); anything else is 400 `validation`, field `weight_override`. POST: absent or null means none (`CreateSetup` writes it only when set). PUT: absent leaves it (`UpdateSetup` never touches it), null clears and a number sets it through `SetSetupWeightOverride` in the same transaction (`:582`). The audit snapshot (`setupAudit`, `:493`) carries `weight_override` when the setup has one. `GET /api/v1/admin/rules` is unchanged (counts by selector).
5. **OpenAPI** (`api/openapi.yaml`): the fields above; new schemas `WeightSource` (four values), `HoldWeightSource` (five), `WeightAttributes`; `SetupInput.weight_override` (optional, nullable, 0..1e12); `Setup.weight` and `matched_rule_selector` descriptions now say what they mean after this change. Response fields are listed as required where always present; the three ledger fields are optional (absent on grant/reset, as `entry_count`). `cd ui && npm run gen:api` regenerated `ui/src/api/schema.gen.ts`.
6. **Minimal UI type fixes** (test fixtures only; the generated types made the new required fields mandatory): `ui/src/pages/admin-shared/fixtures.ts` (`setup04`/`setup07` gain `weight_source`, `weight_attributes` and the three admin fields; `setupList` gains `reported_weights_enabled: false`; `emptyCollector` gains the two null fields), `ui/src/pages/setups/setups.test.tsx` (`base` gains `weight_source: "default"`, `weight_attributes: {}`; the setup-06 admin block gains the three fields; `list()` gains `reported_weights_enabled: false`), `ui/src/pages/usage/usage.test.tsx` (the `burning` entry gains `weight_source`, `hold` gains `weight_source` and `weight_attributes`). No page code changed.

## Requirement -> tests (mutation that turned each red: `mutations.log`, every file restored byte for byte, all tests green after)

| Requirement | Tests | Mutations -> red |
| --- | --- | --- |
| WGT-14 | `internal/api/weights_readers_test.go`: `TestWGT14_EveryReaderShowsTheWeightThePollCharges` (five setups priced by override, reported, default, rule, and a rule setup whose hold the policy prices at 4; one developer per setup; at one time T the state route, Setups list as developer and as admin, the admin setup view (PUT response), the active hold, My usage burn and Admin Users burn agree, and the poll at T records the same weight and source on every sample; the policy hold shows 4/`policy` in hold and burn while the setup views show 2/`rule`; a rule edit moves the active hold, burn and list at once and the next poll charges it; hold cost = list cost = burn) | M14a policy exception dropped; M14b list shows the base weight; M14c admin views ignore the report; M14d Admin Users burn ignores the policy; M14e active hold shows its last sample (old behaviour) |
| WGT-15 | `TestWGT15_LedgerEntriesShowTheWeightsOfTheirPolls` (Europe/Berlin; raw rows, the day view and after compaction: a day with 1.15, 1.15, 1.4, rule 2, policy 3 gives 1.15..3 with `policy, reported, rule`; a day with a pruned sample gives nulls; a day with a pre-migration sample gives the range and null sources; non-debits null; absent on grant and reset responses); `internal/store/weights_queries_test.go`: `TestWGT15_StoreDebitWeightAggregates` (windows, debit-0 samples excluded, null sources counted, absent windows, one statement) | M15a partial-data rule dropped; M15b rolled-up rows use their one poll; M15c null sources ignored; M15d sources in store order; M15e samples with no debit counted (api and store tests) |
| WGT-16 | `TestWGT16_AttributesAreForDisplayOnly` (attributes with spaces and punctuation round-trip unchanged in the state, Setups list, admin view and hold; shown under an override; `{}` when out of bounds, off, expired; selectors `tier=platinum` and `memory-gb=512` match nothing, `tier=gold` only the labelled setups; the label index has only label keys and values; `match_count` and `matched_rule_selector` ignore attributes; an attribute `tier=platinum` does not price lab-03 by a rule) | M16a hold attributes dropped; M16b label index fed the attributes |
| WGT-18 | `TestWGT18_WeightOverride` (beats reported and rule; feature on and off; 50 above the max, 0, 1e12, `-0` accepted; `-1`, `-0.5`, `1000000000001`, `1e400`, `"3"`, `true`, `{}`, `[1]` refused with field `weight_override`, nothing stored, no audit; a poll charges it; PUT absent leaves, null clears; POST absent/null none, number set; audit on set, clear and create; PUT without the field answers as before; `match_count` counts overridden setups) | M18a absent clears; M18b upper bound unchecked; M18c override not in the audit |
| WGT-22 | `TestWGT22_CollectorViewShowsTheWeightsReportAndTheLastRefusal` (null before a report; entries incl. an unknown name; `out_of_bounds` with inclusive limits; refusal time, account, message; both null off, back when on); `TestWGT22_TighteningTheMaximumShowsToTheAdmin` (design Tests item end to end through the admin settings route: count shown at once, setup view `out_of_bounds`, next report refused and shown, polls succeed at the rule weight, also after `max_age`) | M22a `out_of_bounds` never counted; M22b shown while off; M22c refusal not shown |
| WGT-23 | `TestWGT23_ReleasedHoldShowsItsLastCharge` (pending release completed by a poll at 1.15 reported; unchanged after a new report, a rule edit and an override while the setup moves on; null source for a pre-migration sample; a released hold without a sample follows the resolver) | M23a released hold follows the resolver; M23b source from elsewhere |

## The two deliberate A3 changes and existing assertions

**No existing assertion broke because of the two deliberate A3 changes** (the active hold's weight following a rule edit at once; Admin Users burn honouring a policy's weight): no existing test pinned either old behaviour. Both new behaviours are asserted by `TestWGT14_...` (M14d, M14e).

Existing assertions changed, all because responses gain the C2/C1 fields (exact key-set or whole-body comparisons); none is weakened, each now also requires the new field to be present:

1. `internal/api/admin_setups_test.go:28,42` (`TestSET1_CreateSetupShapeAndAudit`): setup keys + `weight_source`, `weight_attributes`; admin keys + `reported_weight`, `reported_weight_not_applied`, `weight_override`.
2. `internal/api/user_test.go` (`TestLED2_MeShape` burning keys + `weight_source`; `TestHOLD3_MyHoldsShapeAndStateFilter` hold keys + `weight_source`, `weight_attributes`; `TestLED2_MyLedger` entry keys + `weight_min`, `weight_max`, `weight_sources`; `TestSET6_SetupsListForDeveloper` response keys + `reported_weights_enabled`, setup keys + the two; `TestSET6_SetupsListForAdminHasAdminFieldsAndDisabled` admin keys + the three).
3. `internal/api/admin_system_test.go:293` (`TestADM4_CollectorHealth`): + `last_weights_report`, `last_weights_refusal`.
4. `internal/api/ledger_view_test.go` (`TestLED11_NoNewParameterAnswersAsBefore`, helper `withoutNewFields`): also requires and strips the three ledger weight fields before comparing with the recorded pre-change response; new helpers `ledgerWeightFields`, `isLedgerWeightField`.
5. **`internal/api/upgrade_compat_test.go` (`TestUPG2_...`, a d96a950 compatibility test):** `ledgerLikeRelease` also requires and strips the three ledger fields; the setups comparison strips `reported_weights_enabled` and, through a new `withoutSetupWeightFields`, each setup's two fields and the admin block's three. The fixtures are untouched. The INTENT says to keep the d96a950 tests "passing unchanged"; adding response fields cannot leave a whole-body comparison unchanged, so this edit follows the test's own pattern for added fields (`entry_count`, `label_index`, `display_name`). The orchestrator should confirm this reading.

## Decisions for docs/decisions.md

1. A released hold with a usage sample shows `weight_attributes: {}`: a sample does not keep attributes, and the attributes of today's report may not be those it was charged under. A released hold without a sample shows the resolver's weight, source and attributes now. An active hold shows the resolver's attributes, also when the weight shown is a policy's.
2. In the admin views a rate rule that cannot be evaluated is handled by resolving the setup as if no rate rule existed: the rule part falls back to the default weight (today's effect), while an override or a usable reported weight still applies, and `matched_rule_selector` is null (as today on such an error). The user views and the state route fail the request, as before.
3. `weight_override` appears in the setup's audit snapshot only when the setup has one, so a setup without one is audited exactly as `v1.1.0` did; setting shows as the key appearing in `after`, clearing as it leaving.
4. `weight_override` validation: any JSON value that does not decode to a float64 (string, boolean, object, list, `1e400`) or lies outside 0..1e12 is 400 `validation`, field `weight_override`, message `weight_override must be a number from 0 to 1e12, or null for none`; `-0` is stored as 0; `null` on POST means none.
5. The ledger weight window of a raw debit is its own poll (`created_at` to `created_at + 1 ms`, the stored time precision); of a row rolled up in storage, and of a merged `rollup=day` entry, the hold's local day in the configured time zone (`Deps.Location`, the zone compaction uses). The fields are computed only for the entries of the page returned.
6. `DebitWeightAggregates` binds all windows as one JSON array, so the ledger read stays one statement more than before whatever its size (the scale test counts 11 statements, was 10).
7. `last_weights_report.out_of_bounds` counts every stored entry outside the limits now in force, names that match no setup included ("how many of them the limits now exclude"). `last_weights_refusal` is the last 400 whatever its age; the UI compares its `at` with the report's `received_at`.
8. OpenAPI: `WeightSource` (four values) for setups, `HoldWeightSource` (five, with `policy`) for holds, burn and `weight_sources`; nullable enums are written inline with `null` in the list, as the file already does for `cannot_acquire_reason`.
9. The stored weights report is read by the user routes, admin views and collector view only when the setting is on (as the state route already did), so with the feature off these routes run no new statement for it.

## Files changed

```
api/openapi.yaml
ui/src/api/schema.gen.ts
ui/src/pages/admin-shared/fixtures.ts
ui/src/pages/setups/setups.test.tsx
ui/src/pages/usage/usage.test.tsx
internal/api/BUILD.bazel
internal/api/admin_system.go
internal/api/admin_views.go
internal/api/handlers_admin.go
internal/api/handlers_integration.go
internal/api/handlers_user.go
internal/api/handlers_weights.go
internal/api/user_views.go
internal/api/admin_setups_test.go
internal/api/admin_system_test.go
internal/api/ledger_view_test.go
internal/api/upgrade_compat_test.go
internal/api/user_test.go
internal/api/weights_readers_test.go
internal/store/apiuser_queries.go
internal/store/weights_queries_test.go
```
Task folder: `.orchestrator/reported-weights/tasks/06-m5-readers/` (PROGRESS.md, RESULT.md, ci.log, ci.exit, gotest.log, race.log, scale.log, ui.log, genapi.log, gazelle.log, mutations.log).

## Commands run

- `go test ./internal/api ./internal/store` while iterating; `go test ./...` exit 0 (`gotest.log`).
- Mutation script (21 mutations in the api code, one in the store query; each applied, the id's tests run, the file restored): all RED (`mutations.log`). A first run had two GREEN mutations; one was closed with the rule-edit check in `TestWGT14_...`, the other showed a redundant check in the override validation, which was removed.
- `HOURGLASS_SCALE_TEST=1 go test ./internal/api -run NFR_ApiUnder500msAtSpecScale -v -count=1 -timeout 20m`: PASS (`scale.log`). **Slowest route: `GET /api/v1/admin/users`, worst 150.0 ms** (was about 146 ms in an earlier job's log). `GET /api/v1/me/ledger?period=2026-11` (5184 raw debits, 3.0 MB) worst 124.5 ms, about 57 ms before this change: the weight lookup and the larger body roughly double it, still well under 500 ms. Collector poll over 100 holds 273 ms.
- `go test -race -count=1 ./internal/api -run 'WGT|UPG|LED|SET|HOLD|ADM4|UI2|Contract'`: exit 0.
- `cd ui && npm run gen:api`, `npm run typecheck`, `npx vitest run`: exit 0, 218 tests.
- `bazel run //:gazelle` exit 0 (added the new test file); `tools/check_core_generic.sh` exit 0.
- `./ci.sh`: exit 0 (40 of 40 Bazel tests pass).

## Confidence and gaps

- The full-size scale test runs with reported weights off (its seed does not turn the setting on), so with the feature on the user and admin routes read the stored report once more per request (one statement); this appears negligible but was not measured at full size.
- The ledger weights of a debit with a hold rely on the collector writing the debit at its sample's `polled_at` (as `DebitMultipliers` already does) and on compaction using `Deps.Location`; both appear to hold in `internal/app` wiring (not re-verified there).
- A debit's ledger day for a rolled-up row is the local day of its `created_at`; a ledger debit written by anything other than the collector (none exists today) would show null weights.
- `internal/collector` and `internal/weights` were not touched; no bug was found in them.
- `docs/` (the four fields whose meaning shifts, the override, the collector view) is milestone 7.
