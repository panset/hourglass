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
| 2 | Pure core logic: selectors and rate rules, bands and smoothing, debits, ledger and rollover | in progress |
| 3 | SQLite store and migrations | not started |
| 4 | Extension API, fakes, conformance suites, bundled non-vSphere extensions | not started |
| 5 | Collector and event queue | not started |
| 6 | Auth and JSON API, `api/openapi.yaml`, contract test | not started |
| 7 | React + TypeScript UI | not started |
| 8 | vSphere provider and pressure source | not started |
| 9 | Images, compose stacks, end-to-end acceptance path (bundled and split) | not started |
| 10 | Docs and `examples/custom` | not started |

## Finished and verified

- Step 0: repository pushed to `github.com/panset/hourglass` (public).
- Spike: the govmomi simulator supports resource pools; its allocation figures are static (decisions D5-D8).
- Milestone 1: `bazel build //...`, `bazel test //...`, `go build ./...` and `./ci.sh` pass from a fresh clone; a deliberately broken test and a file without an SPDX header were both caught.
- Wireframe notes for the UI work: `docs/ui-notes.md`.

## Not finished

Milestones 2 to 10.

## Waiting on the owner

Nothing.

## Decisions to review

See the **REVIEW** entries in [decisions.md](decisions.md): D3, D6, D8.
