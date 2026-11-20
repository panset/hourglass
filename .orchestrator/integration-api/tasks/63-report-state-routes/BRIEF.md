# 63 Report routes and the state route

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Ground rules, Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/63-report-state-routes/`.

## Goal
`PUT /api/v1/integration/pressure`, `PUT /api/v1/integration/deployments` and
`GET /api/v1/integration/state` behave exactly as the design says, replacing their 501 stubs,
with requirements INT-1, INT-2, INT-3, INT-4 and INT-8 each covered over HTTP by a test named
after the id. `POST .../holds/{id}/release` stays a stub (task 65).

## Context
- Design: `docs/design/integration-api.md`. Read all of it; your sections are "Integration
  routes" (the two PUTs, the shared rules, the state route) and INT-1..4, INT-8.
- `.orchestrator/integration-api/tasks/60-store-reported/RESULT.md`: the `internal/reported`
  and store API.
- `.orchestrator/integration-api/tasks/61-service-accounts/RESULT.md`: how a handler gets the
  service principal, how tests mint tokens, and the `integrationStubs` map and scale-test skip
  you must remove entries from as you replace each stub.
- `CONTRIBUTING.md`, "Add an API route".

## Details to get right
- Limits on deployments: 10,000 entries, 32 labels per entry, 1 MB body; duplicate name is 400
  `validation`; unknown names are accepted, ignored and listed in `unknown`; `accepted` counts
  the names that match a setup.
- `expires_at` in responses is `observed_at` plus the configured `max_age` of the matching
  `reported` extension. The `providers/reported` package does not exist yet (task 64) and the
  core must never import it. The API needs to know, from the loaded configuration alone,
  whether `provider.type` / `pressure.type` is `reported` and what `max_age` is (default 10m,
  minimum 1m). Find the simplest generic way (for example reading the config block of the
  selected extension by its type name at wiring time) and record the choice in
  `docs/decisions.md`. Note that the literal word `reported` in core code is not a banned term,
  but check `tools/check_core_generic.sh` to be sure.
- Not configured: 409 `not_configured` with a message naming the config key (`provider.type`
  or `pressure.type`); nothing is stored (INT-4).
- 409 `stale_report`; equal `observed_at` replaces; more than 2 minutes in the future is 400.
- A report never triggers a poll, a debit or an event.
- State: every setup, disabled included; `hold` null when free; `hold.user` has id, username,
  email, name (owner decision); `reported_deployed` reflects the stored report, not the last
  poll; `reported` and `observed_at` fields null before the first report; `poll`, `pressure`
  (`source`, `reported`, `smoothed`, `multiplier`) and `deployments` blocks as in the design's
  example. Decide and record what `pressure.reported` and `deployments.observed_at` hold when
  the corresponding extension is not `reported` (simplest: null). INT-8's test must assert the
  response contains no balance, ledger, settings, audit or other-user data (walk the JSON keys
  against an allow-list). No per-row queries: the scale test must pass with state included.
- The stored report records the calling account's name.
- OpenAPI: make the schemas match the real responses; the contract test must validate the
  three routes' success responses; regenerate `ui/src/api/schema.gen.ts`.

## Scope
May write: `internal/api/`, `internal/reported/`, `internal/store/` (only if a query is
missing), `internal/config/`, `internal/app/` and other wiring, `api/openapi.yaml`,
`ui/src/api/schema.gen.ts`, `BUILD.bazel` via Gazelle, `docs/decisions.md` (append rows with
the next free number), your task folder.
Must not touch: `docs/SPEC.md`, `docs/design/`, `docs/integrating.md`, `internal/holds/`,
`internal/collector/`, `providers/`, other UI files, git state.

## Done when
`TestINT1_`, `TestINT2_`, `TestINT3_`, `TestINT4_`, `TestINT8_` HTTP tests exist and pass;
the permissions and contract tests pass with the three stubs removed;
`go test -race -count=1 ./internal/...` and `./ci.sh` pass.

## Long-running steps
`./ci.sh` takes roughly 5 to 15 minutes: run it once at the end, foreground, 30-minute timeout,
output to a log file in your task folder, read only the exit code and tail.

## Report
Follow the Reporting section of the contract. In RESULT.md also say how the API learns the
`reported` configuration and `max_age`, so task 64 can keep the extension consistent with it.
