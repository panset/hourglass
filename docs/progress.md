# Progress

Last updated: 2026-11-19

Status of Hourglass v1 against the spec in [SPEC.md](SPEC.md). The detailed
task table lives in `.orchestrator/hourglass-v1/PLAN.md`; decisions are in
[decisions.md](decisions.md).

## Milestones

| # | Milestone | Status |
|---|---|---|
| 0 | Repository: license, README stub, .gitignore, spec copy | done, pushed |
| - | Spike: govmomi simulator resource pools and cluster figures | done; see decisions D5-D9 |
| 1 | Build skeleton (Go module, Bazel bzlmod, rules_go, Gazelle, CI script) | verified from a fresh clone, pushed |
| 2 | Pure core logic: selectors and rate rules, bands and smoothing, debits, ledger and rollover | verified, pushed |
| 3 | SQLite store and migrations | verified, pushed |
| 4 | Extension API, fakes, conformance suites, bundled non-vSphere extensions | built; independent verification running |
| 5 | Collector and event queue | in progress: event dispatcher and settings built; ledger service, collector and hold rules next |
| 6 | Auth and JSON API, `api/openapi.yaml`, contract test | in progress: OpenAPI contract and auth |
| 7 | React + TypeScript UI | not started |
| 8 | vSphere provider and pressure source | not started |
| 9 | Images, compose stacks, end-to-end acceptance path (bundled and split) | not started |
| 10 | Docs and `examples/custom` | not started |

## Finished and verified

- Step 0: repository pushed to `github.com/panset/hourglass` (public).
- Spike: the govmomi simulator supports resource pools; its allocation figures are static (decisions D5-D8).
- Milestone 1: `bazel build //...`, `bazel test //...`, `go build ./...` and `./ci.sh` pass from a fresh clone; a deliberately broken test and a file without an SPDX header were both caught.
- Milestone 2: `internal/rules`, `internal/metering`, `internal/ledger`. An independent check found no bugs against the spec and caught 29 of 33 seeded mutations (2 were equivalent); its findings are fixed. Decisions D11-D17.
- Milestone 3: `internal/store` matches the data model column for column. Verification found three defects (ledger rows replaceable, a panic leaving the write lock held, stale balance cache on out-of-order writes); all fixed with tests. Decision D10.
- Wireframe notes for the UI work: `docs/ui-notes.md`.

## Not finished

Milestones 4 to 10.

## Waiting on the owner

Nothing.

## Decisions to review

See the **REVIEW** entries in [decisions.md](decisions.md): D3, D6, D8, D10, D11, D12, D13, D19.
