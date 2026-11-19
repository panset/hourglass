# Result: 16 internal/events (done)

## Exported API (internal/events/events.go)
- `New(hooks []hourglass.Hook, rec Recorder, opts ...Option) *Dispatcher`
- `Recorder` interface: `InsertEventDelivery`, `FinishEventDelivery` (same signatures as `*store.Store`; no adapter needed; rec may be nil)
- `Clock` interface (`Now()`); options `WithTimeout`, `WithClock`, `WithLogger`; `DefaultTimeout` = 30 s
- `(*Dispatcher).Veto(ctx, Event) error`, `Emit(Event)`, `Start(ctx)`, `Close(ctx) error`, `Flush(ctx) error`, `Pending() int`
- `*VetoError{Hook, Err}`: Error() is the hook's message; Unwrap returns Err

## Decisions for docs/decisions.md
- Hooks run in a goroutine so a hook that ignores ctx still times out; the abandoned goroutine is left to finish on its own (leak bounded by the hook's behaviour).
- Timeout message is "hook <name> timed out" for both Veto and Emit.
- Pending() counts queued plus the in-flight event; Flush waits for both.
- Emit before Start queues; Emit after Close is logged and dropped. Close stops after the in-flight call (remaining hooks of that event are skipped), counts and logs dropped queued events; if ctx ends first the call is cancelled and ctx error returned.
- Records: row inserted at call start, updated at finish; recording failures are logged only. Record writes use context.WithoutCancel so a cancelled request still records.
- Panics are recovered and recorded as error "hook <name> panicked: <value>".
- Veto after Close still works (synchronous).

## Files
internal/events/{events.go, events_test.go, BUILD.bazel}; this folder.

## Evidence
go test -race -count=3 ./internal/events/... ok (~2.4 s); bazel test //internal/events/... PASSED; check_spdx.sh and check_core_generic.sh OK; gofmt clean.

## Confidence and gaps
No gaps in store/hourglass. Event.MarshalJSON is whatever pkg/hourglass currently provides (event_json.go exists). Test of EXT-7 store round-trip uses a real temp sqlite store.
