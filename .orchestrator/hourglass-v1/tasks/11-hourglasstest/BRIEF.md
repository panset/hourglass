# 11 pkg/hourglasstest: fakes and conformance suites
Job intent: .orchestrator/hourglass-v1/INTENT.md (read it first). Rules: .orchestrator/hourglass-v1/CONTRACT.md.
## Goal
Public package `pkg/hourglasstest` for extension authors and for Hourglass's own tests: fakes of the four extension interfaces, a fake clock, and one conformance suite per interface. Spec: docs/SPEC.md EXT-5 and "Testing without a vCenter" item 1.
## Context
The API is in `pkg/hourglass`; read `.orchestrator/hourglass-v1/tasks/10-extension-api/RESULT.md` for the exported surface and its design decisions, then the source.
## Contents
- `FakeProvider` (settable allocations and error; counts calls), `FakePressureSource` (settable value/error; records the snapshots it saw), `FakePolicy` (decision per hold via a func or a default; settable error), `FakeHook` (records events in order; per-event-type error; optional blocking/delay controlled by a channel for timeout tests). All safe for concurrent use, with `Name()` settable.
- `Clock`: an interface `Now() time.Time` is NOT defined in pkg/hourglass, so define here a `FakeClock` with `Now`, `Set`, `Advance`, safe for concurrent use; keep it a concrete type with a `Now` method so core code can accept any `interface{ Now() time.Time }`.
- Builders for test data: a `Snapshot` builder (setups with labels/weights, holds, allocations) so tests read clearly.
- Conformance suites, each a function taking `*testing.T` and a constructor, runnable by a third party from their own `_test.go`:
  `TestProvider(t, func() hourglass.ResourceProvider)`: non-empty stable `Name()`; `List` honours context cancellation promptly; repeated calls with no backend change return the same set; no allocation has an empty `ResourceName`; safe to call concurrently.
  `TestPressureSource(t, new func() hourglass.PressureSource, snapshots ...hourglass.Snapshot)`: name; value finite and within [0,1] for each supplied snapshot and for an empty snapshot (or a returned error, never an out-of-range value); context cancellation; does not mutate the snapshot.
  `TestMeteringPolicy(t, new func() hourglass.MeteringPolicy, cases ...)`: name; `Decision.Weight` finite and >= 0; when `Meter` is true `Reason` is non-empty; deterministic for the same input; context cancellation; does not mutate inputs.
  `TestHook(t, new func() hourglass.Hook)`: name; `Handle` accepts every event type in `hourglass.EventTypes()` with nil `User`/`Setup`/`Hold` without panicking; returns within the context deadline; tolerates an unknown event type.
  Each check is a `t.Run` subtest with a clear name so failures tell the author what contract was broken.
- Tests for this package: run every conformance suite against the corresponding fake (must pass), and against deliberately broken implementations defined in the test file (must fail). To observe a failing suite without failing the real test, structure each suite around an internal check function taking a small `interface{ Errorf(...); Helper(); ... }` reporter (or equivalent) so the tests can capture failures.
Standard library plus `pkg/hourglass` only. No mention of vCenter/vSphere, sizes or concrete extensions; neutral labels in examples (`tier=a`).
## Scope
May write: `pkg/hourglasstest/`, this task folder. Do not touch go.mod, MODULE.bazel, `pkg/hourglass/` or `internal/` (another agent is editing the toolchain files and `internal/store/` right now). BUILD file via `bazel run //:gazelle -- pkg/hourglasstest`; add the SPDX comment line at the top of the generated BUILD.bazel by hand. Bazel may queue or fail transiently because of the other agent's toolchain change; if a Bazel failure is not about your package, rely on `go test -race ./pkg/hourglasstest/...` and say so.
## Done when
`go build ./...`, `go vet ./pkg/...`, `go test -race ./pkg/...` pass and your files pass `./tools/check_spdx.sh`; RESULT.md lists the exported API and decisions.
