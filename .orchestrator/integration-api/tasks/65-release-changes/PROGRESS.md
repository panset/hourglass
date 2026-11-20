# Progress, task 65

- 2026-11-20 read intent, contract, design, tasks 63 and 64 results; store already had the columns and queries.
- 2026-11-20 pkg/hourglass: EventHoldReleasing after hold.deployed, Hold.ReleaseRequestedAt and its JSON form.
- 2026-11-20 internal/holds: pending release (Pending, requestRelease), ReleaseByService, still_deployed removed.
- 2026-11-20 internal/collector: completeRelease after metering inside the poll transaction.
- 2026-11-20 internal/api: 202 on user release, service release handler, release_requested_at on Hold and Holder, stub map emptied, audit actor shows the service account (D56).
- 2026-11-20 openapi.yaml and schema.gen.ts updated; decisions D56 to D59 appended.
- 2026-11-20 tests added and old still_deployed tests updated; go test -race ./... green.
- next: ./ci.sh, then RESULT.md.
