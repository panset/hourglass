# 02 Milestone 2: storage, `internal/reported` weights, the `reported_weights` setting — RESULT

Status: **done**. `./ci.sh` exit 0 (`ci.log`, `ci.exit`). `go test ./...` exit 0 (`gotest.log`). UI typecheck and vitest pass with the regenerated types and no fixture change (`ui.log`).

## What was built

1. **Migration `0008_reported_weights.sql`** (`internal/store/migrations/0008_reported_weights.sql`): tables `reported_weights_meta` (single row: `observed_at`, `received_at`, `service_account`, shaped like `reported_deployments_meta`, D46) and `reported_weights` (`setup_name` TEXT PRIMARY KEY, `weight` REAL NOT NULL, `attributes_json` TEXT NOT NULL DEFAULT '{}'), `ALTER TABLE usage_samples ADD COLUMN weight_source TEXT`, `ALTER TABLE setups ADD COLUMN weight_override REAL`. Additive only. Gazelle added it to `embedsrcs`.
2. **Store** (`internal/store`): weights report queries (`reported.go:159`, `:209`), `UsageSample.WeightSource` written and read (`usage.go:25`), `LastSample.WeightSource` read by `HoldLastSamples` (`apiuser_queries.go:64`), `Setup.WeightOverride` read by `GetSetup`/`ListSetups`, written by `CreateSetup` only when non-nil, changed by `SetSetupWeightOverride` (`setups.go:20`, `:131`). No ledger aggregates. Every existing caller still writes a null `weight_source` and never sets an override.
3. **`internal/reported`** (`weights.go`): submit and read the weights report with the shared `resolve` (default to receipt time, millisecond truncation, more than 2 minutes ahead refused), ordering against the stored **weights** report only, atomic replace in one transaction; entry validation by `WeightChecker`; entry error text by `EntryMessage`; the in-memory last refusal on the `Service`.
4. **`internal/settings`**: key `reported_weights` (`settings.go:32`), type `ReportedWeights` (`:74`), defaults `false, 0, null, "10m"`, validation per D1 (`:145`), strict decoding with all four fields required (`:193`), JSON form `{"enabled","min","max","max_age"}`. Audited through the existing `SaveAt` change list, so the before and after values are the whole object.
5. **`PUT /api/v1/admin/settings`** (`internal/api/handlers_admin.go:842`, `:855`): if the body has no `reported_weights`, the stored value is copied into `next` inside the transaction before `SaveAt`, so an absent key leaves it unchanged and writes no audit for it. Not added to `settingsKeys`. `GET` returns the key because it is in `keyOrder`.
6. **`api/openapi.yaml`**: `Settings.reported_weights` → new schema `ReportedWeightsSetting` (all four fields required inside it, `additionalProperties: false`). Not in `Settings.required`. `ui/src/api/schema.gen.ts` regenerated (`npm run gen:api`); `reported_weights?` is optional in the TS type, so no UI change was needed.

## Exported API added (for briefing milestones 3 and 4)

`internal/store` (methods on `*Store` and `*Tx` via `queries`):
```go
type ReportedWeight struct { SetupName string; Weight float64; Attributes map[string]string }
func (q *queries) GetReportedWeights(ctx context.Context) (ReportedHeader, []ReportedWeight, error) // ErrNotFound before the first report; set ordered by name; Attributes never nil
func (q *queries) ReplaceReportedWeights(ctx context.Context, h ReportedHeader, set []ReportedWeight) error // atomic; duplicate name -> ErrConflict, nothing changed
func (q *queries) SetSetupWeightOverride(ctx context.Context, id int64, w *float64) error // nil clears; unknown id -> ErrNotFound; no validation
// New fields: UsageSample.WeightSource string ("" = null); LastSample.WeightSource string; Setup.WeightOverride *float64
// CreateSetup writes Setup.WeightOverride when non-nil; UpdateSetup never touches it.
```

`internal/reported`:
```go
const MaxWeightEntries = 10000; const MaxWeightAttributes = 16; const MaxAttributeValueLength = 63
type WeightLimits struct { Min, Max float64 }            // inclusive; from the caller (the setting)
type Weight struct { Setup string; Weight float64; Attributes map[string]string }
type Weights struct { ObservedAt, ReceivedAt time.Time; ServiceAccount string; Weights []Weight /* by name, non-nil */; Age time.Duration }
type EntryError struct { Index int; Setup string; Reason string } // Error() = EntryMessage(...); Unwrap() -> *ValidationError (so classify maps it to 400 validation)
func EntryMessage(index int, setup, reason string) string     // `weights[3] (setup "lab-03"): weight 25 is above the maximum 20`; name omitted when ""
type WeightChecker struct{ ... }
func NewWeightChecker(limits WeightLimits) *WeightChecker
func (c *WeightChecker) Check(index int, w Weight) error      // *EntryError or nil; tracks duplicates across calls
func (s *Service) SubmitWeights(ctx context.Context, account string, observedAt *time.Time, weights []Weight, limits WeightLimits) (Weights, error)
    // errors: *EntryError (first invalid entry), *ValidationError (> MaxWeightEntries, or observed_at > 2 min ahead), *StaleError
func (s *Service) Weights(ctx context.Context) (Weights, error) // ErrNoReport before the first
type Refusal struct { At time.Time; ServiceAccount string; Message string }
func (s *Service) RecordWeightsRefusal(r Refusal)
func (s *Service) LastWeightsRefusal() (Refusal, bool)
// Existing: Fresh(age, maxAge), ExpiresAt(observedAt, maxAge) apply to Weights.Age / ObservedAt.
```

`internal/settings`:
```go
const KeyReportedWeights = "reported_weights"
const MaxReportedWeight = 1e12; const MinReportedWeightsMaxAge = time.Minute; const MaxReportedWeightsMaxAge = 24 * time.Hour
type ReportedWeights struct { Enabled bool; Min float64; Max *float64 /* nil = no maximum */; MaxAge time.Duration }
// Settings.ReportedWeights ReportedWeights; Defaults() sets {false, 0, nil, 10m}
```

Notes for milestone 4: the handler decodes each entry as raw JSON. For a wrong type or an unknown field in entry i it should return `&reported.EntryError{Index: i, Setup: <name if a non-empty string>, Reason: ...}` (or use `EntryMessage`). For the order "first invalid entry", it can run `NewWeightChecker(limits).Check(i, w)` right after decoding each entry; `SubmitWeights` checks again (cheap, same result). `classify` (`internal/api/helpers.go:157`) maps it to 400 `validation` with an empty `field`: milestone 4 must write `field: "weights"` itself (or extend `classify`), since `ValidationError` has no field. The handler must also call `RecordWeightsRefusal` for each 400, using `a.reported` (the API's one `reported.Service`, `internal/api/api.go:118`), and the collector view reads `LastWeightsRefusal` from the same instance. Limits to pass: `WeightLimits{Min: cfg.ReportedWeights.Min, Max: *cfg.ReportedWeights.Max}` (Max is non-nil whenever Enabled is true; check Enabled first for 409).

## Decisions for docs/decisions.md

1. Entry positions in refusal messages count from 0 (`weights[0]` is the first entry), as JSON array indexes do and as the existing `rules[%d].weight` field does.
2. The entry-message function lives in `internal/reported` (`EntryMessage`, with the `EntryError` type and `WeightChecker`), so the package that validates what it stores and the milestone-4 handler that validates types produce one format.
3. The last refusal is kept on `reported.Service` (mutex-guarded), because the API owns exactly one `Service` that both the route handler and the collector view use; it is lost at restart (B2, D20).
4. Refusal reasons: `setup must be a non-empty string`; `setup is repeated (first at weights[N])`; `weight must be a finite number`; `weight X is below the minimum M`; `weight X is above the maximum M` (numbers in Go's shortest `g` form); `attributes has N entries (at most 16)`; `attributes: <label rule message>` (keys use the SET-3 label key rule via `rules.ValidateLabels`, values at most 63 characters). More than 10,000 entries is a plain validation error without a position (`weights has N entries (at most 10000)`), checked before any entry.
5. `UpdateSetup` never writes `weight_override`; a new `SetSetupWeightOverride` does. `CreateSetup` writes the override in a separate statement only when it is non-nil, so a setup without one is inserted with the exact pre-0008 statement (the older-schema upgrade tests call `CreateSetup` on a version-2 database and pass unchanged). This keeps every existing caller's behaviour and gives milestone 5 "absent leaves unchanged" for free.
6. Settings validation errors for the new key carry `field` `reported_weights.<field>` (`reported_weights.enabled`, `.min`, `.max`, `.max_age`, or the unknown field's name), and `reported_weights` alone when the value is not an object (`null` included: the key is present, so it must be a full object). Messages start with the field name, e.g. `enabled can be true only when max is set: set a maximum weight to turn reported weights on`.
7. Rule order in validation: `max`, then `min` (finite, at least 0, at most `max`), then `max_age`, then `enabled` needs `max`. `min` may be set while `max` is null (the D1 table only bounds `min` by `max` "when `max` is set").
8. `max_age` accepts any Go duration string within `1m`..`24h` inclusive and is returned in the short form of `poll_interval` (`60s` → `1m`, `1440m` → `24h`). `-0` for `min` or `max` is stored as `0`; JSON numbers too large for a float64 (`1e400`) are refused as "must be a number".
9. In the OpenAPI file the object is a separate schema `ReportedWeightsSetting`, referenced from `Settings.reported_weights` and not listed in `Settings.required`; `max` is described with `exclusiveMinimum: true, minimum: 0, maximum: 1000000000000`.
10. **Existing-test assertions changed (not A3 changes; both are consequences of the design itself):**
    - Migration-count assertions 7 → 8: `internal/store/store_test.go` (two in `TestMigrateFreshAndReopen`/`TestNFR_Migration0003...`), `internal/store/integration_queries_test.go:363`, `internal/store/ledger_compaction_test.go:967`, `internal/store/migrate0004_test.go:44,65`, and **`internal/app/upgrade_test.go:164-165`** (`TestUPG1_...`, a d96a950 upgrade test; it asserts the schema version after the start, which a new migration necessarily changes). The commit that added 0007 (`873fb3d`) did the same for its counts. `internal/app` is outside this brief's May-write list: the orchestrator should accept or redo this one-number edit.
    - `TestSchemaColumnsMatchSpec` lists the two new tables and two new columns (CONTRIBUTING "Add a migration" step 5).
    - `TestADM2_GetSettingsShapeAndRuntime` (`internal/api/admin_system_test.go:159`) checks the exact key set of `settings`; `reported_weights` was added to its list, because D1 requires `GET` to always return the key.

## Requirement -> tests

| Requirement | Tests |
| --- | --- |
| WGT-17 (D1 table, all four fields, on needs max, audit, PUT without the key, GET always returns it) | `internal/api/settings_reported_weights_test.go`: `TestWGT17_GetAlwaysReturnsTheKeyWithDefaults`, `TestWGT17_EveryRuleOfTheD1Table` (12 accept rows incl. every bound, 21 reject rows each with its field, each also asserting nothing stored and no audit), `TestWGT17_OnWithoutMaximumMessage`, `TestWGT17_AllFourFieldsRequiredWhenPresent`, `TestWGT17_ChangeAuditedWithWholeObjects`, `TestWGT17_PutWithoutTheKeyLeavesItUnchanged`, `TestWGT17_StoredValueLoads`; `internal/settings/settings_test.go`: `TestWGT17_ReportedWeightsValidateAndDefaults` (non-finite values, defaults, JSON form, save/load) |
| WGT-7 (storage part: whole atomic replace, ordering and skew, optional observed_at, survives restart) | `internal/store/weights_queries_test.go`: `TestWGT7_StoreReplacesWeightsReportWhole`, `TestWGT7_StoreWeightsReplaceIsAtomicForReaders`, `TestWGT7_StoreWeightsReportSurvivesReopen`; `internal/reported/weights_test.go`: `TestWGT7_WeightsNoReportYet`, `TestWGT7_WeightsReplaceWholeSetAndDefaultObservedAt`, `TestWGT7_WeightsOrderingAndClockSkew` (stale / equal / exactly 2 min / 2 min + 1 ms), `TestWGT7_WeightsOrderingIndependentOfDeployments`, `TestWGT7_AttributesStoredUnchanged`, `TestWGT7_WeightsSurviveReopen` |
| WGT-8 (package part: each entry rule, position and name, whole report refused, previous kept) | `TestWGT8_EachEntryRuleRefusesWholeReport` (11 rules, inclusive limits, 16 x 63-char attributes, weight 0), `TestWGT8_FirstInvalidEntryReported`, `TestWGT8_EntryMessageForm`, `TestWGT8_TooManyEntriesRefused`, `TestWGT8_NegativeZeroStoredAsZero`, `TestWGT8_UnknownNamesStored` |
| B2 last refusal | `TestWeightsLastRefusal` |
| Migration 0008 (additive, no row rewritten) | `TestNFR_Migration0008UpgradesVersion7DatabaseWithoutRewritingRows` (version-7 database with users, setups, labels, a hold, samples with and without selector, ledger, deployments report, settings; every old column of every row compared before and after; new columns null) |
| New columns | `TestUsageSampleWeightSourceRoundTrip`, `TestSetupWeightOverrideColumn`, `TestSchemaColumnsMatchSpec` |

Mutation check: replacing the "keep stored value" line in the handler makes `TestWGT17_PutWithoutTheKeyLeavesItUnchanged` fail (run once, reverted).

## Files changed

```
api/openapi.yaml
ui/src/api/schema.gen.ts
internal/api/BUILD.bazel
internal/api/admin_system_test.go
internal/api/handlers_admin.go
internal/api/settings_reported_weights_test.go
internal/app/upgrade_test.go
internal/reported/BUILD.bazel
internal/reported/reported.go
internal/reported/weights.go
internal/reported/weights_test.go
internal/settings/settings.go
internal/settings/settings_test.go
internal/store/BUILD.bazel
internal/store/apiuser_queries.go
internal/store/integration_queries_test.go
internal/store/ledger_compaction_test.go
internal/store/migrate0004_test.go
internal/store/migrations/0008_reported_weights.sql
internal/store/reported.go
internal/store/setups.go
internal/store/store.go
internal/store/store_test.go
internal/store/usage.go
internal/store/weights_queries_test.go
```
Task-folder files (logs, PROGRESS.md, RESULT.md) are under `.orchestrator/reported-weights/tasks/02-m2-storage-settings/`.

## Commands run

- `go test ./internal/store/ ./internal/reported/ ./internal/settings/ ./internal/api/` while iterating: pass.
- `cd ui && npm run gen:api` (exit 0, `genapi.log`); `npm run typecheck && npx vitest run` (exit 0, 218 tests, `ui.log`).
- `bazel run //:gazelle` (exit 0, `gazelle.log`).
- `go test ./...` (exit 0, `gotest.log`).
- `./ci.sh` (exit 0, `ci.log`; 39 Bazel tests pass).

## Confidence and gaps

- The "PUT with only the eight keys answers as v1.1.0 did" check compares against the in-process sent body and runtime block, not against `testdata/upgrade-v1.1.0/responses.json`: wiring that fixture into Bazel needs the root `BUILD.bazel` exports, outside this brief. The fixture-based comparison is milestone 7's WGT-20.
- `internal/app/upgrade_test.go` was edited outside the May-write list (one expected number); see decision 10.
- Nothing yet reads the setting or the stored report for pricing, writes a non-null `weight_source`, or exposes the override; those are milestones 3 to 5. The new fields appear to be harmless to all current callers (all tests pass unchanged apart from the listed count/key-list assertions).
- `TestNFR_LedgerAppendTimeIsLinear` (known flake) did not fail in these runs.
