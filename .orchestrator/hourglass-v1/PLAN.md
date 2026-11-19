# Plan: Hourglass v1

Statuses: pending, running, done, verified, blocked, dropped.
Models: haiku (small), sonnet (mid), opus (large). The orchestrator commits
after each verified task and pushes after each milestone.

Tasks for milestones 3 onward are outlines; each milestone is re-planned in
detail when the one before it is verified.

| # | M | Task | Kind | Model | Depends on | Writes to | Status |
|---|---|---|---|---|---|---|---|
| 00 | 0 | Repo, license, README stub, .gitignore, SPEC.md, first push | build | orchestrator | - | repo root, `docs/` | verified (pushed 1b36a72) |
| 01 | - | Spike: govmomi simulator creates resource pools; which cluster CPU/memory/storage figures it reports | research | sonnet | 00 | task folder, scratch module outside the repo | verified |
| 02 | 1 | Build skeleton: go.mod, MODULE.bazel (bzlmod, rules_go, Gazelle), .bazelversion, `ci.sh`, SPDX header check, stub `cmd/hourglassd` | build | sonnet | 00 | repo root, `cmd/`, `tools/` | verified |
| 03 | 1 | Verify 02 from a fresh clone: `bazel build //...`, `bazel test //...`, `go build ./...` | verify | haiku | 02 | (read-only) | done |
| 04 | 2 | `internal/rules`: selector parser, `Match`, ordered rule evaluation by kind (RULE-1,2,5) | build | sonnet | 03 | `internal/rules/` | running |
| 05 | 2 | `internal/metering`: bands, 15-minute smoothing, debit in milli-tokens with gap cap (MET-3,4) | build | sonnet | 03 | `internal/metering/` | done |
| 06 | 2 | `internal/ledger` pure logic: entry types, balance, rollover with expiry and carry-over, reset, grant (LED-1..6) | build | sonnet | 03 | `internal/ledger/` | done |
| 07 | 2 | Verify 04-06: run tests, count them, mutation spot-checks, spec worked examples | verify | sonnet | 04,05,06 | (read-only) | pending |
| 08 | 3 | `internal/store`: SQLite (modernc), WAL, embedded numbered migrations for all 15 tables, repositories | build | sonnet | 07 | `internal/store/` | pending |
| 09 | 3 | Verify 08 against the data-model table, column by column | verify | sonnet | 08 | (read-only) | pending |
| 10 | 4 | `pkg/hourglass`: four interfaces, Setup/Allocation/Decision/Event, Snapshot/HoldState, registry with unknown-name error (EXT-1,2) | build | opus | 07 | `pkg/hourglass/` | pending |
| 11 | 4 | `pkg/hourglasstest`: fake provider, pressure source, policy, hook, clock; conformance suite per interface (EXT-5) | build | sonnet | 10 | `pkg/hourglasstest/` | pending |
| 12 | 4 | `pressure/inventory` and `policies/default` (MET-7) | build | sonnet | 11 | `pressure/`, `policies/` | pending |
| 13 | 4 | `providers/static` and `hooks/webhook` | build | sonnet | 11 | `providers/static/`, `hooks/` | pending |
| 14 | 4 | Core-genericity check: script failing CI if a core package imports or mentions vCenter, sizes or a concrete extension | build | haiku | 10 | `tools/` | pending |
| 15 | 4 | Verify 10-14 | verify | sonnet | 12,13,14 | (read-only) | pending |
| 16 | 5 | `internal/events`: in-order queue, veto path, 30 s timeout, delivery records (EXT-6,7) | build | sonnet | 09,15 | `internal/events/` | pending |
| 17 | 5 | `internal/collector`: snapshot, matching, orphans, outages, idempotent debits in one transaction, hold lifecycle events (MET-1..6, HOLD-3,4) | build | opus | 16 | `internal/collector/` | pending |
| 18 | 5 | Verify 16-17, including restart and failed-poll cases | verify | opus | 17 | (read-only) | pending |
| 19 | 6 | `api/openapi.yaml` for every route in the spec's HTTP API table | build | sonnet | 15 | `api/` | pending |
| 20 | 6 | `internal/auth`: local admin (argon2id, forced change, rate limit), OIDC code flow with PKCE, sessions, CSRF, role middleware (AUTH-1..8) | build | opus | 09 | `internal/auth/` | pending |
| 21 | 6 | `internal/api`: handlers for holds, setups, me, pressure, admin; settings, audit, seed import; `pkg/hourglass.Run`, config loading, `hourglassd` wiring | build | sonnet (split into 2-3 at re-plan) | 18,19,20 | `internal/api/`, `cmd/`, `pkg/hourglass/` | pending |
| 22 | 6 | Permissions-table HTTP tests (every cell, unassigned gets no data) and the OpenAPI contract test | build | sonnet | 21 | `internal/api/` tests | pending |
| 23 | 6 | Verify 19-22; security read of auth | verify | opus | 22 | (read-only) | pending |
| 24 | 7 | Extract the seven wireframe artboards into `docs/ui-notes.md` (layout, columns, labels per page) | explore | sonnet | 00 | `docs/ui-notes.md` | done |
| 25 | 7 | `//ui` scaffold: Vite, React, TypeScript, rules_js with pinned Node and lockfile, generated API client, app shell and top bar | build | sonnet | 19,24 | `ui/` | pending |
| 26 | 7 | UI pages: Sign in, Not authorized, My usage, Setups | build | sonnet | 25 | `ui/src/pages/` (user) | pending |
| 27 | 7 | UI pages: Users, Setup inventory, Settings, Audit log | build | sonnet | 25 | `ui/src/pages/admin/` | pending |
| 28 | 7 | `internal/uiserve` behind a build tag, index.html fallback, bundled vs split, CORS (UI-3,4,7) | build | sonnet | 25 | `internal/uiserve/` | pending |
| 29 | 7 | Verify 25-28 against wireframes and UI-1..7 | verify | sonnet | 26,27,28 | (read-only) | pending |
| 30 | 8 | `providers/vsphere`: internal client interface, pool matching, nested pools, pressure formula, unit tests on a hand-written fake (layer 2) | build | sonnet | 01,15 | `providers/vsphere/` | pending |
| 31 | 8 | govmomi client, in-process simulator integration tests (layer 3), `hourglassd check-provider` | build | sonnet | 30 | `providers/vsphere/`, `cmd/` | pending |
| 32 | 8 | Verify 30-31; confirm govmomi is imported only by the vsphere package | verify | sonnet | 31 | (read-only) | pending |
| 33 | 9 | rules_oci images: `hourglass`, `hourglass-api`, `hourglass-ui`, non-root | build | sonnet | 29 | `deploy/`, BUILD files | pending |
| 34 | 9 | Compose stacks (bundled, split), Keycloak realm with three test users, simulator container, sample config, seed file | build | sonnet | 33,32 | `deploy/` | pending |
| 35 | 9 | End-to-end acceptance script in a headless browser, bundled and split | build | sonnet | 34 | `e2e/` | pending |
| 36 | 9 | Verify 33-35 by running both stacks from scratch | verify | sonnet | 35 | (read-only) | pending |
| 37 | 10 | README quick start, `docs/extending.md`, `docs/deploying.md`, `CONTRIBUTING.md` | build | sonnet | 36 | `README.md`, `docs/` | pending |
| 38 | 10 | `examples/custom` third-party main with its own extensions | build | sonnet | 15 | `examples/custom/` | pending |
| 39 | 10 | Dependency license check in CI; Prometheus metrics listener and JSON logs if not already done | build | sonnet | 36 | `tools/`, `internal/` | pending |
| 40 | 10 | Final verification against "Done means", from a clean clone, with a requirement-id coverage table | verify | opus | 37,38,39 | (read-only) | pending |

## Notes

- Parallel groups: 04/05/06; 12/13/14; 19/20; 24 alongside anything; 26/27/28;
  37/38/39. Tasks in a group write to disjoint directories. Shared files
  (`go.mod`, `MODULE.bazel`, lockfiles) are touched by one task at a time:
  when parallel tasks need a new dependency, the orchestrator adds it first.
- Milestone 8 does not depend on 5-7 and can move earlier if the spike says
  the test layers need changing.
- The end-to-end test needs Docker, so it is tagged `manual` in Bazel and run
  by its own script; `bazel test //...` stays hermetic and network-free.
- Every milestone ends with: verify task green, `docs/progress.md` updated,
  commit(s) citing requirement ids, push.
