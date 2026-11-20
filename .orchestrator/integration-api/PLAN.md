# Plan: integration API (push model)

Statuses: pending, running, done, verified, blocked, dropped.
Models: haiku (small), sonnet (mid), opus (large). The orchestrator commits
after each milestone once `./ci.sh` passes. Nothing is pushed.

Tasks are sequential: they share `internal/api`, `api/openapi.yaml` and the
store. Later tasks are outlines and are re-planned when the one before returns.

| # | M | Task | Kind | Model | Depends on | Writes to | Status |
|---|---|---|---|---|---|---|---|
| 59 | 0 | Commit the owner's edits to the design doc and guide, unchanged | - | orchestrator | - | - | done (5808957) |
| 60 | 1 | Migration 0005, store queries, `internal/reported` (ordering, staleness, atomic replace) (INT-2,3,6 at store level) | build | sonnet | 59 | `internal/store/`, `internal/reported/` | verified |
| 61 | 2 | Service accounts: bearer auth, service principal, `ServiceRoute(scope)`, failed-attempt rate limit, `AccessService`, six admin routes, OpenAPI, permissions test (SVC-1..8) | build | opus | 60 | `internal/auth/`, `internal/api/`, `api/openapi.yaml`, `ui/src/api/schema.gen.ts` | verified |
| 62 | 2 | Independent verification of 60-61, with a security read of the token path | verify | opus | 61 | (read-only) | done (findings: 62b) |
| 63 | 3 | Report routes and state route; OpenAPI and contract test (INT-1..4, INT-8) | build | sonnet | 62 | `internal/api/`, `internal/reported/`, `api/`, config | verified |
| 64 | 4 | `providers/reported` provider and pressure source, conformance suites, `check-provider`, blank import (INT-5,6,7) | build | sonnet | 63 | `providers/reported/`, `cmd/`, collector tests | verified |
| 65 | 5 | Pending release, `hold.releasing`, collector completion, service release route (HOLD-6..9, CMP-1) | build | sonnet | 64 | `internal/holds/`, `internal/collector/`, `internal/api/`, `pkg/hourglass/`, `hooks/webhook/`, `api/` | verified |
| 66 | 5 | Independent verification of 63-65 | verify | opus | 65 | (read-only) | done (0 high, 2 medium, 4 low: 66b) |
| 67 | 6 | UI: Service accounts page, "Releasing" state, last report times on the collector view, regenerated types | build | sonnet | 66 | `ui/` | verified |
| 68 | 7 | End-to-end scenario with `reported` extensions and a scripted reporter: acquire, deploy, release request, teardown, completion, reporter going silent | build | sonnet | 67 | `deploy/`, `e2e/` | verified |
| 69 | 7 | Independent verification of 67-68 by running the scenario from scratch | verify | sonnet | 68 | (read-only) | done (0 high, 0 medium, 5 low: 69b) |
| 70 | 8 | Docs: run every example in `docs/integrating.md` against the running server, correct it, remove the banner; `extending.md`, `deploying.md` (Upgrade entry), `README.md`, `progress.md` | build | sonnet | 69 | `docs/`, `README.md` | done (verified in 71) |
| 71 | 8 | Final verification: requirement-id coverage table, `./ci.sh`, guide examples re-run | verify | opus | 70 | (read-only) | running |
| 72 | 8 | Owner report | write | sonnet | 71 | task folder | pending |

Fix tasks after a verification are numbered with a letter (62b, 66b, ...).

## Milestones and commits

1. Store and `internal/reported` (60)
2. Service accounts (61, 62)
3. Report and state routes (63)
4. `reported` extensions (64)
5. Release changes (65, 66)
6. UI (67)
7. End-to-end (68, 69)
8. Docs and final verification (70, 71)

## Notes

- `docs/decisions.md`: D42 and D43 by the orchestrator in milestone 1; agents
  append from D44.
- The end-to-end test needs Docker and stays outside `bazel test //...`.

## Added during the work

| # | Task | Model | Status |
|---|---|---|---|
| 62 | Verification found 1 high (valid token locked out by the failure limiter), 6 low | opus | done |
| 62b | Fix the findings of 62 | sonnet | verified |
| 62c | Verify the fixes of 62b | sonnet | done: all closed |
| 66b | Fix the findings of 66 (missing tests, dead stub code, event order); new tests re-checked in 71 | sonnet | done |
| 69b | Strengthen the e2e assertions per the five low findings of 69 | sonnet | done (re-checked in 71) |
