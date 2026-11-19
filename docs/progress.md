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
| 4 | Extension API, fakes, conformance suites, bundled non-vSphere extensions | verified, pushed |
| 5 | Collector and event queue | verified, pushed |
| 6 | Auth and JSON API, `api/openapi.yaml`, contract test | built and pushed; permissions-table and contract tests pass; security verification running |
| 7 | React + TypeScript UI | built and pushed (all eight pages, embedded serving); verification against the wireframes running |
| 8 | vSphere provider and pressure source | verified, pushed |
| 9 | Images, compose stacks, end-to-end acceptance path (bundled and split) | images and both stacks built and smoke-tested; browser acceptance test running |
| 10 | Docs and `examples/custom` | `examples/custom` and the license check built; documents being written; final verification pending |

## Finished and verified

- Step 0: repository pushed to `github.com/panset/hourglass` (public).
- Spike: the govmomi simulator supports resource pools; its allocation figures are static (decisions D5-D8).
- Milestone 1: `bazel build //...`, `bazel test //...`, `go build ./...` and `./ci.sh` pass from a fresh clone; a deliberately broken test and a file without an SPDX header were both caught.
- Milestone 2: `internal/rules`, `internal/metering`, `internal/ledger`. An independent check found no bugs against the spec and caught 29 of 33 seeded mutations (2 were equivalent); its findings are fixed. Decisions D11-D17.
- Milestone 3: `internal/store` matches the data model column for column. Verification found three defects (ledger rows replaceable, a panic leaving the write lock held, stale balance cache on out-of-order writes); all fixed with tests. Decision D10.
- Milestone 4: `pkg/hourglass`, `pkg/hourglasstest`, and the `inventory`, `default`, `static` and `webhook` extensions. Verification confirmed the interfaces match the spec's declarations and that a third-party module builds against the public packages alone; its 11 findings are fixed except two accepted ones (event JSON compacts `details`; the term check is a plain substring match).
- Milestone 5: events, settings, ledger service, collector and hold rules. An adversarial check walked the acceptance path at service level with every ledger amount matching hand-computed values; no high-severity findings, and the medium ones (backwards clock, rollover ordering at midnight, false outage on interval change) are fixed with regression tests. Decisions D19-D25.
- Milestone 8: `providers/vsphere`, `hourglassd check-provider`, `pkg/hourglass/server`. Verification confirmed the provider only reads, the tests need nothing beyond localhost, and a main outside this module can call `hourglass.Run`; its low-severity findings are fixed. Decisions D27, D28.
- Wireframe notes for the UI work: `docs/ui-notes.md`.

## Not finished

- Milestone 6 and 7: independent verification (running).
- Milestone 9: the end-to-end acceptance run in both modes (running).
- Milestone 10: documents, final verification from a clean clone.

## Waiting on the owner

Nothing.

## Decisions to review

See the **REVIEW** entries in [decisions.md](decisions.md): D3, D6, D8, D10, D11, D12, D13, D19, D22, D23, D27, D28, D29, D30.
