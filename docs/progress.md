# Progress

Last updated: 2026-11-19

Status of Hourglass v1 against the spec in [SPEC.md](SPEC.md). The detailed
task table lives in `.orchestrator/hourglass-v1/PLAN.md`; decisions are in
[decisions.md](decisions.md).

## Milestones

| # | Milestone | Status |
|---|---|---|
| 0 | Repository: license, README stub, .gitignore, spec copy | committed locally; **push waiting on the remote URL** |
| - | Spike: govmomi simulator resource pools and cluster figures | not started |
| 1 | Build skeleton (Go module, Bazel bzlmod, rules_go, Gazelle, CI script) | not started |
| 2 | Pure core logic: selectors and rate rules, bands and smoothing, debits, ledger and rollover | not started |
| 3 | SQLite store and migrations | not started |
| 4 | Extension API, fakes, conformance suites, bundled non-vSphere extensions | not started |
| 5 | Collector and event queue | not started |
| 6 | Auth and JSON API, `api/openapi.yaml`, contract test | not started |
| 7 | React + TypeScript UI | not started |
| 8 | vSphere provider and pressure source | not started |
| 9 | Images, compose stacks, end-to-end acceptance path (bundled and split) | not started |
| 10 | Docs and `examples/custom` | not started |

## Finished and verified

Nothing yet.

## Not finished

Everything above.

## Waiting on the owner

- Git remote URL for the first push (the placeholder in the brief was not filled in).
- Approval of the plan before milestone 1.

## Decisions to review

See the **REVIEW** entries in [decisions.md](decisions.md): D3.
