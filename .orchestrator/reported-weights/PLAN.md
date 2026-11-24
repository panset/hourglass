# Plan: reported weights

Milestones follow the design's "Suggested task order". Build tasks run one at a time (they share
`internal/api`, the store and `api/openapi.yaml`). Only the orchestrator commits.

| # | Task | Kind | Model | Depends on | Writes to | Status |
|---|---|---|---|---|---|---|
| 00 | Digest of deploying/integrating/extending docs, how D78 fixtures were recorded, version and release mechanics | explore | sonnet | - | (read-only) | done |
| 01 | M1: record v1.1.0 compatibility fixtures (`testdata/upgrade-v1.1.0/`) before product code changes | build | sonnet | - | `testdata/upgrade-v1.1.0/` | done |
| 02 | M2: migration 0008, store queries, `internal/reported` weights report, `reported_weights` setting (WGT-17) | build | opus | 01 | store, reported, settings, settings handler, openapi | done |
| 03 | M3: resolver `internal/weights` and collector (WGT-1..6, WGT-12) | build | opus | 02 | weights, collector, pkg/hourglass comment | done |
| 04 | Verify M3 (independent) | verify | opus | 03 | (read-only) | verified (4 test gaps, fixed in 04b) |
| 05 | M4: scope, report route, state fields (WGT-7..11, 13, 21); OpenAPI + generated types | build | opus | 04 | auth, api, openapi, schema.gen.ts | done |
| 06 | M5: other readers, ledger fields, setup input, collector view (WGT-14..16, 18, 22, 23) | build | opus | 05 | api, store, openapi, schema.gen.ts | done |
| 07 | Verify M4+M5 (independent) | verify | opus | 06 | (read-only) | verified (2 low; 07b fixes the state route) |
| 08 | M6: UI (UI-12..14) | build | sonnet | 07 | `ui/src`, docs/ui-notes.md | done (browser check comes with e2e, task 10) |
| 09 | M7a: compatibility tests (WGT-19, WGT-20) against v1.1.0 fixtures | build | sonnet | 08 | internal tests | done |
| 10 | M7b: e2e integration scenario with weights | build | sonnet | 08 | `e2e/` | blocked (ports 8080, 8091, 8081, 8990, 8443 held by the owner; steps W1-W6 written, not run) |
| 11 | M7c: documentation (integrating with run examples, deploying, extending, README, ui-notes, progress) | build | sonnet | 09, 10 | docs, README | done (examples run on a host process; charge, UI and size limits not run by this task) |
| 12 | Verify M7 and the whole (independent, requirement-by-requirement, mutation checks) | verify | opus | 11 | (read-only) | verified (1 medium, 6 low; 12b fixes; e2e not run) |
| 13 | Release preparation: ci.sh, `e2e/run.sh both`, `e2e/run.sh integration`, report | build | sonnet | 12 | REPORT.md | pending |
| 14 | STOP: owner go-ahead and version number, then merge, tag, push, watch workflow | release | orchestrator | 13 | git | pending |

Fix tasks (04b, 07b, 12b) are added when a verification finds something.

## Notes
- Decisions go to `docs/decisions.md` from D80 on, written by the orchestrator from each RESULT.md.
- 09 and 10 may run side by side (disjoint paths, neither touches openapi).
