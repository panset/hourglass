# Result: 11 pkg/hourglasstest

Status: done.

## Files (all new, pkg/hourglasstest/)
doc.go, clock.go, builders.go, fakes.go, conformance.go, hourglasstest_test.go, BUILD.bazel (gazelle + SPDX line by hand).
Imports: stdlib and pkg/hourglass only.

## Exported API
- Clock: `FakeClock` (`NewFakeClock(start)`, `Now`, `Set`, `Advance`), `DefaultTime`. Concrete type, satisfies `interface{ Now() time.Time }`.
- Fakes (all mutex-guarded, `SetName`, `Calls`): `FakeProvider` (`SetAllocations`, `SetError`), `FakePressureSource` (`SetValue`, `SetError`, `Snapshots`), `FakePolicy` (`SetDefault`, `SetFunc`, `SetError`), `FakeHook` (`SetError(eventType, err)`, `SetGate(<-chan struct{})`, `Events`, `EventsOfType`). Constructors `NewFake*`.
- Builders: `NewSnapshot()` -> `SnapshotBuilder` (`At`, `ProviderConfigured`, `Setup`, `Hold`, `Allocation`, `Build`); options `Label`, `Weight`, `Resource`, `Details`, `Acquired`, `Metering`, `Deployed`, `Released`; `NewHoldState(snapshot, holdIndex, user)`.
- Suites: `TestProvider(t, newFn)`, `TestPressureSource(t, newFn, snapshots...)`, `TestMeteringPolicy(t, newFn, cases ...PolicyCase)` with `PolicyCase` and `NewPolicyCase`, `TestHook(t, newFn)`. Subtests are named after the contract, e.g. `Weight_is_finite_and_not_negative`.

## Decisions for docs/decisions.md
1. Each suite is a list of `check{name, fn(reporter)}`; `reporter` is `Helper()` + `Errorf`. Panics inside a check become failures. The package tests run the lists against a collector to prove broken implementations fail.
2. Cancellation checks call with an already-cancelled context and require a return within 1s (error or value both fine). Hook checks use a 500ms deadline plus 500ms grace.
3. Pressure and policy suites always add an empty snapshot; with no supplied inputs a small default is used. An error return is always accepted in place of a value.
4. Fakes return `ctx.Err()` for a done context; FakePolicy's default decision is {Meter true, Weight 1, "default"}; FakePressureSource does not clamp its value so callers can be tested against bad values.
5. FakeHook records the event before blocking on its gate and returns `ctx.Err()` if the context ends first.
6. Fake inputs and outputs are deep-copied, so tests cannot alias fake state.
7. The suites cannot detect a hook that panics only in a goroutine it spawns, nor mutation of data a provider hands out later; checks are black-box.

## Commands
`go build ./...`, `go vet ./pkg/...`, `go test -race -count=1 ./pkg/...` pass (hourglasstest ~41s because broken-implementation tests deliberately sleep to trigger timeouts). `bazel test //pkg/hourglasstest/...` PASSED. `./tools/check_spdx.sh` printed nothing (clean).

## Confidence and gaps
Test runtime of ~40s is inference-tuned only by shortening bounds; could be cut with an injectable timeout if wanted. Test names do not carry EXT ids beyond EXT5. No spec conflicts.
