# 66b Fix the findings of verification 66

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Ground rules, Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/66b-fix-findings/`.

## Goal
Every finding in `.orchestrator/integration-api/tasks/66-verify-reports-release/RESULT.md` is
closed: a test added, code fixed, or a decision recorded, as ruled below. The verifier found
no defect in production behaviour; most of this is missing test coverage.

## Context
Read that RESULT.md in full. Code map: the RESULT.md files of tasks `63-*`, `64-*`, `65-*`
under `.orchestrator/integration-api/tasks/`. Design: `docs/design/integration-api.md`.

## Orchestrator's rulings
1. Medium, INT-5 pressure staleness untested: add a `TestINT5_...` test through the real
   collector that fails when the pressure source's staleness check is removed (stale pressure
   report: poll fails, `poll.failed`, no debit). Also cover "no pressure report yet".
2. Medium, wiring untested: add a test at the composition level (`internal/app` or the
   `cmd`/extension side, respecting the core-generic rule) that fails if `app.Run` stops
   publishing the stored reports to the extensions.
3. Low, race tests do not exercise the double-release and double-event guards (including the
   HOLD-8 store guard against a double service release): add tests that do, under `-race`.
4. Low, `hold.releasing` can arrive after `hold.released`: work out whether this is a real
   ordering violation of the in-order event queue's guarantee (EXT-6/EXT-7 as implemented) or
   only possible across two independent requests. If events for one hold can be delivered out
   of the order in which their state changes committed, fix it with a test. If it cannot be
   fixed without redesigning the queue, do not redesign: record it as a decision marked
   **REVIEW** with the exact scenario.
5. Low, dead `not_implemented` / stub code from D49: remove it (code, test helper maps,
   OpenAPI if present) and append "Superseded: all four routes are implemented" to row D49.
6. Low, no collector test pairing one `reported` extension with a non-reported one: add both
   pairings (`provider: reported` + another pressure source; `pressure: reported` + no
   provider or another provider).
7. The two behaviours the verifier asked to confirm are left as they are: (a) whether a user
   release becomes pending is decided from the last successful poll's view of the setup, not
   from the stored report; (b) after a failed poll a user release follows the same existing
   rule as before this work. Append one decision row, marked **REVIEW**, that states both
   precisely as the code behaves (read the code; do not guess), and make sure a test pins each.
8. For each new test, prove it bites: break the production line it protects, see it fail,
   restore (compare `git diff | shasum` of production files before and after).

## Scope
May write: `providers/reported/`, `internal/` (tests; production code only for rulings 4 and
5), `api/openapi.yaml` and `ui/src/api/schema.gen.ts` only if ruling 5 requires, `BUILD.bazel`
via Gazelle, `docs/decisions.md`, your task folder.
Must not touch: `docs/SPEC.md`, `docs/design/`, `docs/integrating.md`, `ui/src/pages`, `e2e/`,
git state.

## Done when
RESULT.md has one line per finding: fixed (test name, and the break that turned it red) or
closed (decision number). `go test -race -count=1 ./...` and `./ci.sh` pass.

## Long-running steps
`./ci.sh` takes roughly 5 to 15 minutes: run it once at the end, foreground, 30-minute timeout,
output to a log file in your task folder, read only the exit code and tail.

## Report
Follow the Reporting section of the contract.
