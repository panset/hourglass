# 64 The `reported` provider and pressure source

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Ground rules, Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/64-reported-extensions/`.

## Goal
A stock `hourglassd` configured with `provider.type: reported` and/or `pressure.type: reported`
polls from the last stored report, with INT-5, INT-6 and INT-7 each covered by a test named
after the id.

## Context
- Design: `docs/design/integration-api.md`. Read all of it; your section is "The `reported`
  provider and pressure source" including Wiring constraints, plus INT-5, INT-6, INT-7.
- `.orchestrator/integration-api/tasks/60-store-reported/RESULT.md` (`internal/reported` API)
  and `.orchestrator/integration-api/tasks/63-report-state-routes/RESULT.md` (how the API
  learns the `reported` config and `max_age`; stay consistent with it).
- `docs/extending.md` (registering an extension, conformance suites, "Check a provider"),
  existing extensions `providers/static`, `pressure/inventory`, and `cmd/hourglassd`.

## The hard part: wiring
Extension factories receive only their config block and extensions do not touch the database,
yet these two must read reports that the core stores. The core (`internal/`, `pkg/hourglass/`,
`pkg/hourglasstest/`) must not import `providers/reported`, and the design forbids changing the
Go extension interfaces in `pkg/hourglass/types.go`. A third-party `main` must get the
extensions by blank import alone, like the bundled ones. Find the simplest mechanism that
meets all of that (for example a small read-only "stored reports" accessor that the core
publishes through a public package and the extension reads when it is called), keep it
generic, and record it in `docs/decisions.md`. If you conclude it cannot be done without
changing `pkg/hourglass/types.go` or breaking a wiring constraint, stop and report `blocked`
with the options: do not pick one.

## Behaviour
- Provider: one `Allocation` per stored deployment, `ResourceName` equal to the setup name
  unchanged, labels passed through. No naming config (D40).
- Pressure source: the stored value.
- No report yet: error `no deployment report received yet` / `no pressure report received yet`.
- Report older than `max_age` (default 10m, minimum 1m, validated at config load): error
  naming the report's age. Age is measured from `observed_at` with the server's clock (the
  test clock in tests and e2e).
- An error fails the poll like an unreachable backend: `poll.failed`, no debit (INT-5).
- The two work independently (`provider: reported` + `pressure: inventory`, and
  `pressure: reported` with no provider).
- Both pass the conformance suites in `pkg/hourglasstest`.
- `hourglassd check-provider` works with them and prints the age of each stored report.
- Blank import in `cmd/hourglassd/main.go` (and the e2e test main if it lists extensions).

## Tests
- `TestINT5_...`: through the real collector: no report, then a stale report: the poll fails,
  `poll.failed` fires and no debit row is written; a fresh report then succeeds.
- `TestINT6_...`: reports written, database closed and reopened, the extensions still return
  them (subject to `max_age`).
- `TestINT7_...`: through the real collector with the `default` policy: a reported setup with
  an active hold gets `deployed_at` and `hold.deployed` at the next poll and metering starts at
  that poll; a reported setup with no hold is an orphan.
  Tests that need both the core and the extension must live outside the core packages (the
  extension package or a composition-root test), since the core cannot import the extension.

## Scope
May write: `providers/reported/`, `cmd/`, `e2e/` test main only if it lists extensions,
`internal/reported/`, a new public package under `pkg/` if the wiring needs one, wiring in
`internal/app/` or `pkg/hourglass/server`, `BUILD.bazel` via Gazelle, `MODULE.bazel` only if
unavoidable, `docs/decisions.md` (append rows with the next free number), your task folder.
Must not touch: `pkg/hourglass/types.go` interfaces, `docs/SPEC.md`, `docs/design/`,
`docs/integrating.md`, `internal/holds/`, `ui/`, git state.

## Done when
The three tests and the conformance suites pass; `tools/check_core_generic.sh` passes;
`go test -race -count=1 ./...` and `./ci.sh` pass.

## Long-running steps
`./ci.sh` takes roughly 5 to 15 minutes: run it once at the end, foreground, 30-minute timeout,
output to a log file in your task folder, read only the exit code and tail.

## Report
Follow the Reporting section of the contract. In RESULT.md describe the wiring mechanism in
five lines, for the docs task.
