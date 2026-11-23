# 76b Fix the findings of verification 76

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` ("the spec" means
`docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/76b-fix-findings/`.

## Goal

Close every finding (1 medium, 4 low) in
`.orchestrator/ledger-labels-onboarding/tasks/76-verify-m2-m3/RESULT.md`.

## Context

Read that RESULT.md in full; it gives file:line, a failure scenario and a
suggested fix per finding. The requirements are in the design doc (A1, LED-7 to
LED-10). D70 and D71 in `docs/decisions.md` record the choices made so far.

## What to do

- Medium (LED-10, "a backlog of any size delays no poll past the poll
  interval"): add a `TestLED10_` test that drives the collector's own loop, the
  way production runs, with a backlog larger than one cycle, and fails if the
  time between polls grows past the interval. Keep the design's shape: a fixed
  number of steps per cycle. Do not add a setting. If you conclude a code change
  is needed to guarantee the bound, make the smallest one and explain it.
- Low, unbounded walk over old one-row groups: bound it so one step's search
  cost does not grow with history, and show it in the scale test.
- Low, timing assertions that fail under `-race` or load: make them robust
  without making them meaningless (for example skip wall-clock bounds under the
  race detector, or measure with generous margins). If the "known timing flake"
  the verifier saw in an uncached run is in a test that existed before this job
  (check with `git log`), do not edit that test: name it in RESULT.md instead.
- Low, test-coverage gaps: add the missing cases.
- Low, wording of D70(b) and (c): do NOT edit `docs/decisions.md`. Put the
  exact corrected sentences in RESULT.md under "Decisions for docs/decisions.md".
- For any finding you decide not to fix, say why.

## Scope

May write: `internal/store/`, `internal/ledger/`, `internal/collector/`,
`internal/api/perf_compaction_scale_test.go`, `BUILD.bazel` files Gazelle
updates, your task folder.
Must not touch: `docs/`, `api/`, `ui/`, `e2e/`, git state. Do not weaken or
delete any test that existed before this job (before commit 4e8686e).

## Done when

`go test ./internal/...` passes, `go test -race ./internal/ledger/ ./internal/collector/ ./internal/store/`
passes, and `./ci.sh` exits 0 with the changed tests actually executed
(record the exit code in `ci.exit`). RESULT.md lists each finding and what
closed it.

## Long-running steps

`./ci.sh`: allow 10 minutes (timeout 600000), output to `ci.log` in your task
folder, read the exit code and tail only.

## Report

Follow CONTRACT.md. One final reply of at most ten lines.
