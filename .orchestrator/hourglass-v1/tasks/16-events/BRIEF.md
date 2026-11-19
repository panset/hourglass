# 16 internal/events: dispatch to hooks
Job intent: .orchestrator/hourglass-v1/INTENT.md (read it first). Rules: .orchestrator/hourglass-v1/CONTRACT.md.
## Goal
Package `internal/events`: delivers `hourglass.Event`s to the configured hooks. Spec: docs/SPEC.md "Extension points" bullets, the event table, EXT-6, EXT-7, data-model row `event_deliveries`.
## Behaviour
- `New(hooks []hourglass.Hook, recorder Recorder, opts...) *Dispatcher`; hooks are in config order. `Recorder` is a small interface you define that `*store.Store` satisfies through its existing event-delivery methods (read internal/store; write a thin adapter in this package if signatures differ). Options: per-call timeout (default 30 s), clock, logger (`log/slog`).
- `Veto(ctx, e Event) error`: for vetoable events only (`hourglass.Vetoable`), called inside the user's request. Calls each hook in order with a per-call timeout derived from ctx; the first error or timeout stops delivery and is returned as a typed `*VetoError{Hook, Err}` whose message is the hook's message (a timeout reads "hook <name> timed out"). Hooks after the vetoing one are not called. Every call made is recorded.
- `Emit(e Event)`: for all other events. Never blocks the caller and never returns an error: appends to one unbounded in-memory FIFO queue drained by a single goroutine, so events are delivered strictly in emit order, each to every hook in config order, each call with its own 30 s timeout. Errors, timeouts and panics in a hook are recovered, logged and recorded, and never stop later hooks or later events. No retries: at most once.
- Calling `Emit` with a vetoable type or `Veto` with a non-vetoable type is a programming error: return/log an error, do not deliver.
- Each delivery (event, hook) writes one record: event type, hook name, payload JSON (`json.Marshal(event)`; another task is adding a stable `MarshalJSON` to Event, so just marshal the value), ok, error text, started and finished times.
- With zero hooks everything is a cheap no-op and nothing is recorded.
- `Start(ctx)` / `Close(ctx) error`: Close stops accepting, lets the in-flight call finish (bounded by ctx), and drops what is still queued (EXT-7: pending events at shutdown are lost), reporting how many were dropped in a log line. `Flush(ctx)` waits until the queue is empty (for tests and for the collector's tests).
- `Pending() int` for health reporting.
## Tests (with pkg/hourglasstest fakes; names cite EXT-6 / EXT-7)
Order across events and across hooks; a slow hook does not block `Emit` (measure) nor reorder; timeout on a hook that ignores ctx (use a short configured timeout) is recorded and the next hook still runs; panic recovery; veto stops at first error and returns its message; veto timeout; records written with correct ok/error; no retries; Close drops pending; zero hooks; run under -race. Keep the whole test package under 5 seconds.
## Scope
May write: `internal/events/`, this task folder.
## Build notes
No new dependencies; do not edit go.mod or MODULE.bazel. BUILD file via `bazel run //:gazelle -- <dir>`, then add the SPDX comment as the first line of the generated BUILD.bazel. Other agents are building other packages at the same time; Bazel commands queue, allow 5 minutes each. Verify with `go test -race ./<dir>/...`, `bazel test //<dir>/...`, `./tools/check_spdx.sh`, `./tools/check_core_generic.sh`. This is a core package: it may import `pkg/hourglass`, `pkg/hourglasstest` (tests only), `internal/store` and the pure packages, never a concrete extension, and must not mention vCenter/vSphere, sizes or concrete extension names. Do not change `internal/store` or `pkg/hourglass`; if you need something they lack, depend on a small interface you define and list the gap in RESULT.md.
## Done when
All checks pass; RESULT.md lists the exported API and decisions.
