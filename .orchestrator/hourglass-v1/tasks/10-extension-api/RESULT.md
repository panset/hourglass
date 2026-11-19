# Result: 10 pkg/hourglass public extension API and registry

Status: done.

## Files changed (all new)
- pkg/hourglass/doc.go: package comment (model, registration, example hook, blank import in main)
- pkg/hourglass/types.go: spec interfaces/structs plus mirrored data-model structs and Snapshot helpers
- pkg/hourglass/events.go: event constants, EventTypes, Vetoable
- pkg/hourglass/registry.go: factories and generic registry
- pkg/hourglass/hourglass_test.go: tests
- pkg/hourglass/BUILD.bazel: from gazelle, SPDX line added by hand
- this folder: PROGRESS.md, RESULT.md, bazel-test.log

Imports: stdlib only (context, encoding/json, fmt, sort, strings, sync, time). No internal/ import. No mention of vCenter/vSphere, sizes or concrete extension names (`grep -riE "vcenter|vsphere|size|inventory|webhook|static" pkg/hourglass` gives nothing).

## Exported API

Spec declarations, reproduced verbatim (names, fields, signatures, field comments), types.go:13-69:
```go
type Setup struct { Name string; Labels map[string]string; Details json.RawMessage }
type ResourceProvider interface { Name() string; List(ctx context.Context) ([]Allocation, error) }
type Allocation struct { ResourceName string; Labels map[string]string }
type PressureSource interface { Name() string; Pressure(ctx context.Context, s Snapshot) (float64, error) }
type MeteringPolicy interface { Name() string; Decide(ctx context.Context, h HoldState, s Snapshot) (Decision, error) }
type Decision struct { Meter bool; Weight float64; Reason string }
type Hook interface { Name() string; Handle(ctx context.Context, e Event) error }
type Event struct { Type string; Time time.Time; User *User; Setup *Setup; Hold *Hold; Data map[string]any }
```

Mirrored structs (types.go:72-126):
```go
type User struct { ID int64; Username, Email, Name string; Roles []string }
type Hold struct {
    ID int64; SetupName string; UserID int64
    AcquiredAt time.Time
    MeteringStartedAt, DeployedAt, ReleasedAt *time.Time
}
type SetupState struct { Setup /* embedded */; Weight float64; ResourceName string }
type Snapshot struct {
    Time time.Time; Setups []SetupState; Holds []Hold
    Allocations []Allocation; ProviderConfigured bool
}
type HoldState struct { Hold Hold; Setup SetupState; User User; Allocation *Allocation }
func (s Snapshot) SetupByName(name string) (SetupState, bool)
func (s Snapshot) AllocationFor(setupName string) (Allocation, bool)
```

Events (events.go):
```go
const EventHoldAcquiring = "hold.acquiring" // ... EventHoldAcquired, EventHoldMeteringStarted,
// EventHoldDeployed, EventHoldReleased, EventBalanceLow, EventBalanceOverdrawn,
// EventUserPending, EventUserApproved, EventPeriodClosed, EventPollFailed
func EventTypes() []string          // table order, fresh copy
func Vetoable(eventType string) bool // true only for hold.acquiring
```

Registry (registry.go):
```go
type ProviderFactory func(config json.RawMessage) (ResourceProvider, error)
type PressureFactory func(config json.RawMessage) (PressureSource, error)
type PolicyFactory   func(config json.RawMessage) (MeteringPolicy, error)
type HookFactory     func(config json.RawMessage) (Hook, error)
func RegisterProvider(name string, f ProviderFactory)
func RegisterPressureSource(name string, f PressureFactory)
func RegisterMeteringPolicy(name string, f PolicyFactory)
func RegisterHook(name string, f HookFactory)
func NewProvider(name string, config json.RawMessage) (ResourceProvider, error)
func NewPressureSource(name string, config json.RawMessage) (PressureSource, error)
func NewMeteringPolicy(name string, config json.RawMessage) (MeteringPolicy, error)
func NewHook(name string, config json.RawMessage) (Hook, error)
func Providers() []string; func PressureSources() []string
func MeteringPolicies() []string; func Hooks() []string
```
Unexported test seam: `resetRegistries()` (registry.go:96).

## Decisions for docs/decisions.md
1. **IDs are int64** in User and Hold, matching SQLite integer rowids; no string IDs, so the store can fill them without conversion.
2. **Hold carries SetupName, not a setup id**: extensions identify setups by name everywhere else (Setup, Snapshot), so the internal setup id is not exposed. `released_by` / `release_reason` are not mirrored (brief lists the fields; they are admin bookkeeping no extension needs). Nullable times are `*time.Time`.
3. **User omits status, auth source, labels, balance**: brief lists id, username, email, name, roles; balance is a ledger concern and would invite extensions to rely on a cached value. Roles is `[]string` ("developer", "admin"), empty when unassigned.
4. **SetupState embeds Setup** and adds `Weight float64` (resolved from rate rules or the default weight; float64 to match `Decision.Weight`) and `ResourceName` (expected provider name from the template or override). Embedding lets extensions write `st.Labels[...]` / `st.Name`. HoldState reuses SetupState so a policy sees the resolved weight.
5. **Snapshot.Holds is `[]Hold`, not `[]HoldState`**: keeps the snapshot flat; a pressure source joins via `SetupByName(h.SetupName)`.
6. **Snapshot helpers limited to two**: `SetupByName` and `AllocationFor` (match allocation `ResourceName` to the setup's expected `ResourceName`; exact string match, first match wins; a setup with empty ResourceName never matches). These are the only matching logic an extension would otherwise re-implement. Linear scans: setups and allocations are small.
7. **HoldState.Allocation is `*Allocation`**, nil when not reported or no provider.
8. **Unknown-name error text**: `hourglass: unknown <kind> "<name>" (registered: a, b, c)`, kinds "provider", "pressure source", "metering policy", "hook"; `registered: none` when empty. Factory errors are returned unwrapped so the caller adds context (e.g. which config key).
9. **Panic messages** follow database/sql: "... with empty name", "... with nil factory", "Register called twice for <kind> <name>".
10. **One generic `registry[F]` with an RWMutex** per kind; names are per-kind, so a provider and a pressure source may share a name (the spec's own v1 set does this).
11. **No `Name()` vs registered-name check**: the registry does not call Name() or compare it to the registration name; simplest option, spec silent.
12. **EventTypes returns a fresh slice** each call so callers cannot mutate the canonical list; listings likewise return fresh sorted slices.
13. Comments added beyond the spec block (e.g. "Each call has a 30-second timeout" on Hook, the 0-1 rule on PressureSource) only restate spec text.

## Tests (pkg/hourglass/hourglass_test.go)
TestEXT1_RegisterAndBuild (all four kinds, nil/empty/JSON config, kinds isolated), TestEXT1_ConfigPassedToFactory, TestEXT1_FactoryErrorReturned, TestEXT1_RegisterPanics (empty name, nil factory, duplicate), TestEXT2_UnknownNameListsRegistered (exact text, sorted, "none"), TestEXT1_ListingIsSortedCopy, TestEXT1_ConcurrentRegisterAndLookup (-race), TestEXT6_EventTypesMatchSpecTable (order + vetoable column, hardcoded from the spec table since Bazel's sandbox has no docs/), TestEXT1_SnapshotHelpers. Every test that registers calls `clean(t)` which resets before and after.

## Commands and outcomes
- `go build ./...` ok; `go vet ./pkg/...` ok; `go test -race -count=1 ./pkg/hourglass/...` ok.
- `bazel run //:gazelle -- pkg/hourglass` ok; `bazel test //pkg/hourglass/...` PASSED (log: bazel-test.log).
- `./tools/check_spdx.sh` exit 1, only `missing SPDX header: internal/store/BUILD.bazel`, which is the concurrent agent's file and out of my scope. All pkg/hourglass files pass.

## Confidence and gaps
- Gazelle does not emit the SPDX line; I added it by hand. A future gazelle run appears to preserve leading comments, but if not, the header must be re-added.
- int64 IDs and the exact Snapshot shape are my inference from the data model; the store task (internal/store) may need to confirm its ID type matches.
- No open questions against the spec.
