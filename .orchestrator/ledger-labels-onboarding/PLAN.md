# Plan: ledger-labels-onboarding

Statuses: pending, running, done, verified, blocked, dropped.
Models: haiku (small), sonnet (mid), opus (large). The orchestrator commits
after each milestone once `./ci.sh` passes. Nothing is pushed, merged or tagged
before the owner's go-ahead with a version number.

Tasks are sequential: they share one working tree, `internal/api`,
`api/openapi.yaml` and the store. Later tasks are outlines and are re-planned
when the one before returns. Task numbers continue from the previous job (72).

| # | M | Task | Kind | Model | Depends on | Writes to | Status |
|---|---|---|---|---|---|---|---|
| 73 | 1 | Record D69 in `docs/decisions.md` | - | orchestrator | - | `docs/decisions.md` | done |
| 74 | 2 | Migration 0006, store: `rollup_count`, delete trigger + marker, compaction queries, `DebitMultipliers` filter, before/after reader test, schema-5 migration test (LED-7, 8, 9, 15, UPG-1) | build | opus | 73 | `internal/store/` | verified |
| 75 | 3 | `ledger.Service.CompactDue` and collector: bounded steps, lock release, scale test (LED-10) | build | opus | 74 | `internal/ledger/`, `internal/collector/`, `internal/api/perf_scale*` | verified |
| 76 | 3 | Independent verification of M2-M3 | verify | opus | 75 | (read-only) | done (0 high, 1 medium, 4 low: 76b) |
| 77 | 4 | Ledger API: `rollup`, `limit`, `cursor`, `entry_count`, `next_cursor`, balances; OpenAPI, contract test, generated types (LED-11..14) | build | opus | 76 | `internal/api/`, `internal/store/`, `api/`, `ui/src/api/schema.gen.ts` | verified |
| 78 | 5 | Ledger UI (UI-8) and the acceptance scenario's `readLedger` helper | build | sonnet | 77 | `ui/src/pages/usage/`, `e2e/browser/tests/acceptance.spec.ts` | verified |
| 79 | 5 | Independent verification of M4-M5 | verify | opus | 78 | (read-only) | done (0 high, 0 medium, 6 low: 79b; bundled e2e not run, port 8081 held by the hourglass-tls stack) |
| 80 | 6 | Labels: `label_index`, selector term in `internal/rules`, chips (SET-8, UI-9) | build | sonnet | 79 | `internal/rules/`, `internal/api/`, `api/`, `ui/` | verified |
| 81 | 6 | Greeting: claims, shared display-name function, `display_name`, top bar (AUTH-9, UI-10) | build | sonnet | 80 | `internal/auth/`, `internal/api/`, `api/`, `ui/` | verified |
| 82 | 7 | Pre-approval: store, sign-in step via injected interface, admin routes, file overlay, permissions test (PRE-1..8) | build | opus | 81 | `internal/store/`, `internal/auth/`, `internal/api/`, `internal/config/`, `internal/app/`, `api/`, `deploy/config.sample.yaml` | verified |
| 83 | 7 | Admin > Users pre-approval UI (UI-11) | build | sonnet | 82 | `ui/` | verified |
| 84 | 7 | Independent verification of M6-M7, with a security read of the sign-in path | verify | opus | 83 | (read-only) | done (0 high, 1 medium, 5 low: 84b) |
| 85 | 8 | End-to-end scenarios (paging, chip, greeting, pre-approved first sign-in, compaction via test clock), Keycloak user, UPG-2, UPG-3 | build | sonnet | 84 | `e2e/`, `deploy/keycloak/`, `internal/` tests | verified (85b: e2e both 16+16, integration 11, all passed) |
| 86 | 8 | Docs: `deploying.md` (upgrade, retention, config reference, pre-approval with commands run, timezone, disk), `ui-notes.md`, `progress.md`, `README.md` | build | sonnet | 85 | `docs/`, `README.md` | verified (86a, 86b) |
| 87 | 8 | Final verification: requirement-to-test table, `./ci.sh`, `e2e/run.sh both`, integration scenario, guide commands re-run | verify | opus | 86 | (read-only) | done (1 high harness risk reported, 1 medium and 3 low fixed; e2e reruns skipped by owner) |
| 88 | 9 | Release preparation: what the released binary reports as its version; owner report | write | sonnet | 87 | task folder | done (`tasks/88-release-report/REPORT.md`); waiting for the owner |
| 89 | 9 | After go-ahead: merge to main, push, tag, watch the image workflow | - | orchestrator | owner | git | pending |

Fix tasks after a verification are numbered with a letter (76b, ...).

## Milestones and commits

1. D69 (73)
2. Migration and store (74)
3. Ledger service and collector (75, 76)
4. Ledger API (77)
5. Ledger UI (78, 79)
6. Labels and greeting (80, 81)
7. Pre-approval (82, 83, 84)
8. End-to-end, compatibility, docs (85, 86, 87)
9. Release (88, 89)

## Notes

- Milestone 6 could run beside 2-5 per the design, but one working tree and
  per-milestone `./ci.sh` and commits make sequential the safer choice.
- Decisions go to `docs/decisions.md` (D69 on); orchestration notes to
  `DECISIONS.md` in this folder.
