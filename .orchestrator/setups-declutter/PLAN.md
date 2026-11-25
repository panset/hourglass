# Plan: setups-declutter

Models: Fable/Opus only for this job (person's instruction). Branch: `setups-declutter`. Spec: `tasks/01-design/DESIGN.md` (Revision 2) + DECISIONS.md. Common build rules: `tasks/COMMON.md`.

| # | Task | Kind | Model | Depends on | Writes to | Status |
|---|---|---|---|---|---|---|
| 01 | Evaluate the ask, write the design plan | design | fable | - | `tasks/01-design/` | done |
| 02 | Map test / merge / release procedure | explore | opus | - | `tasks/02-release-map/` | done |
| 03 | Review the design plan | review | fable (high) | 01 | `tasks/03-design-review/` | done (build with changes) |
| 04 | Revise DESIGN.md with review findings | design | fable | 03 | `tasks/01-design/` | done |
| 05 | B1 additive `pricing` on GET /api/v1/setups | build | opus | 04 | `internal/api/*`, `api/openapi.yaml`, `ui/src/api/schema.gen.ts`, admin-shared fixtures, `docs/deploying.md` | done |
| 06 | B2 shared UI pieces (Table, Modal, format) | build | opus | 04 | `ui/src/components/{Table,Modal}*`, `ui/src/lib/format*` | done |
| 07 | B4 docs | build | opus | 04 | `docs/*` (not SPEC, not deploying), `README.md` | done |
| 08 | B3 Setups page (incl. Q-B = B2 chip filter) | build | fable | 05, 06 | `ui/src/pages/setups/*`, `usage/weight.tsx`, `admin-settings/index.tsx` | done |
| 09 | B5 e2e steps U20/S1/S2 + owner screenshots; stop demo stack; `e2e/run.sh integration` | build | opus | 08 | `e2e/browser/tests/integration.spec.ts`, `e2e/README.md` | done (integration e2e green, 20 steps) |
| 10 | Verify (break list of DESIGN.md s.7; `./ci.sh`; `e2e/run.sh both`; doc wording vs page) | verify | fable | 05-09 | `tasks/10-verify/` | running |
| 11 | Fix findings from 10, re-verify | build | opus | 10 | as needed | pending |
| 12 | Commit (explicit paths), `/deepseek-review --base main` triage | orchestrator | - | 11 | git | pending |
| 13 | ff-merge to main, push origin main | orchestrator | - | 12 | git remote | done (300 UI tests, ci green) |
| 14 | Tag v1.3.0, push tag, watch release.yml, confirm images | orchestrator | - | 13 | git remote, ghcr | done |

Notes:
- From 02: merge = `git merge --ff-only setups-declutter` on main + `git push origin main` (no PRs, no protection, no remote test CI). Release = annotated tag pushed -> `.github/workflows/release.yml` builds ghcr.io/panset/{hourglass,hourglass-api,hourglass-ui}, ~4 min. Checks: `./ci.sh` (<1 min), `e2e/run.sh both` (~26 min), `e2e/run.sh integration` (14-24 min). The `hourglass-demo` compose stack holds ports 8080/8081; the person agreed it is stopped before e2e and left stopped. Commit with explicit `git add` paths only.
- Fix-up list for task 11 (collected): `docs/design/ledger-labels-onboarding.md:204,372` UI-9 text needs the B2 amendment (from 07); 20 quoted UI strings in tasks/07-b4-docs/RESULT.md to check against the built page.

## Revision 3 (owner change of 2026-11-24; see INTENT.md Amendment, DECISIONS.md R3-1..R3-3)

Specs: `tasks/01-design/DESIGN-R3.md` (data, files, tests, break list) + `tasks/12-ux/UX-SPEC.md` (layout, wording, interaction; wins on UX). Old rows 11-14 above are replaced by the rows below.

| # | Task | Kind | Model | Depends on | Writes to | Status |
|---|---|---|---|---|---|---|
| 10 | Verify first build, reduced scope (parts that stay) | verify | fable | - | `tasks/10-verify/` | done (kept parts ready; 0 blocker, 6 minor) |
| 11 | DESIGN-R3.md (design agent resumed) | design | fable | - | `tasks/01-design/` | done |
| 12 | Senior UX designer: UX-SPEC.md | design | fable | 11 | `tasks/12-ux/` | done (19 must, 13 should) |
| 13 | C1 Setups page: combobox, token, no label columns/chips, shared clean-up, + fixes from 10 | build | fable | 10, 12 | `ui/src/pages/setups/*`, `ui/src/components/{Table*,index.ts,components.test.tsx}` | done (300 UI tests, ci green) |
| 14 | C2 Settings page without display keys (PUT sends loaded array back) | build | opus | 12 | `ui/src/pages/admin-settings/*` | done |
| 15 | C3 docs + openapi descriptions + seed/deploy notes | build | opus | 12 | docs, `api/openapi.yaml` (descriptions), `schema.gen.ts`, `README.md`, `deploy/*` | done |
| 16 | C4 e2e (acceptance A0/A2f/A3, integration U20/U22/S1/S2), screenshots, `e2e/run.sh both` + `integration` | build | opus | 13, 14 | `e2e/browser/tests/*`, `e2e/README.md` | running |
| 17 | V2 verify (R3 break list, kept-parts matrix, ci, mutation checks) | verify | fable | 13-16 | `tasks/17-verify/` | done (0 blocker, 2 should-fix, 8 minor) |
| 18 | UX acceptance pass by the UX designer on the finished build | verify | fable | 16 | `tasks/12-ux/` | done (accept; 1 should-fix, 5 polish) |
| 19 | Fix findings from 17/18, re-verify | build | opus | 17, 18 | as needed | done (307 UI tests, ci green) |
| 20 | Commit (explicit paths), `/deepseek-review --base main` triage | orchestrator | - | 19 | git | pending |
| 21 | ff-merge to main, push origin main | orchestrator | - | 20 | git remote | pending |
| 22 | Tag v1.3.0, push tag, watch release.yml, confirm images | orchestrator | - | 21 | git remote, ghcr | pending |

## PAUSED by the person, 2026-11-24 ("please pause the work. will resume in sometime")
State at pause: tasks 01-15, 17, 18, 19 done. Task 16 (e2e + owner screenshots) was mid-run; its agent was told to let the running command finish, record state in `tasks/16-c4-e2e/PROGRESS.md` / `RESULT.md` and start nothing new. Nothing is committed; all work is uncommitted on branch `setups-declutter`. Demo stack stopped.
On resume: read `tasks/16-c4-e2e/RESULT.md`; finish whatever of task 16 remains (both e2e modes green, screenshots); then 20 commit with explicit paths + `/deepseek-review --base main` triage; 21 ff-merge to main and push; 22 tag v1.3.0, push, watch release.yml. Do not start any of these until the person says to resume.

RESUMED by the person, 2026-11-24 ("resume the work"). Task 16 agent told to complete its brief in full.
