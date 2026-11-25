# Plan: ext-feedback-names-modal

Branch: `ext-feedback-names-modal`. Release procedure and check timings: `.orchestrator/setups-declutter/tasks/02-release-map/RESULT.md`. Build rules: `.orchestrator/setups-declutter/tasks/COMMON.md`.

| # | Task | Kind | Model | Depends on | Writes to | Status |
|---|---|---|---|---|---|---|
| 01 | Evaluate the three asks: root cause, every place a person is shown, modal, users order | explore | sonnet | - | `tasks/01-evaluate/` | done |
| 02 | Senior UX designer: review the three asks + holistic pass, other enhancements | design | opus | - | `tasks/02-ux-review/` | done |
| 03 | Re-plan build tasks from 01 + 02 | orchestrator | - | 01, 02 | PLAN.md | done |
| 04 | Backend: keep stored name on a nameless sign-in; `display_name` on AdminUser; local admin pinned first, rest by display name; openapi + schema.gen.ts; Go tests; docs (D100, AUTH-2/AUTH-9) | build | sonnet | 03 | `internal/`, `api/openapi.yaml`, `ui/src/api/schema.gen.ts`, `docs/` | done (617 Go tests) |
| 05 | Shared Modal: x button, backdrop dismiss by kind, body-only scroll with fixed header/footer, scroll lock, busy rule; caller opt-outs; UI tests | build | sonnet | 03 | `ui/src/components/Modal*`, `components.test.tsx`, modal call sites in `ui/src/pages/*` (props only) | done (330 UI tests; inert on app root not done = F11) |
| 06 | UI names + Users page: display name everywhere (P2-P9, P4, P12), local admin row marking/note, Acquire dialog wording (B2.3), pressed state (B1.2); UI tests; ui-notes | build | sonnet | 04, 05 | `ui/src/pages/*`, `ui/src/lib/*`, `docs/ui-notes.md` | done (359 UI tests, ci green) |
| 07 | e2e: add/adjust steps for the three asks; `./ci.sh`; `e2e/run.sh both` + `integration` | build | sonnet | 09 | `e2e/` | done (ci 40/40; bundled 18/18, split 18/18, integration 22/22) |
| 08 | Verify independently (UX-REVIEW A checks, INTENT, mutation checks, ci); runs BEFORE e2e so its mutations cannot disturb an e2e build | verify | opus | 06 | `tasks/07-verify/` | done (ready with fixes: 1 blocker, 3 should-fix, 8 minor) |
| 09 | Fix findings from 08 (F1, F3, F4 + cheap minors), runs before e2e | build | sonnet | 08 | as needed | done (768 Go, 360 UI tests, ci green; F5, F7, F12 left with reasons) |
| 09b | Re-verify the fixes of 09 (read-only, no mutations; runs beside e2e) | verify | sonnet | 09 | `tasks/09-reverify/` | done (fixes hold) |
| 10 | Commit (explicit paths), ff-merge to main, push | orchestrator | - | 09 | git remote | pending |
| 11 | Tag v1.3.1, push tag, watch release.yml, confirm images | orchestrator | - | 10 | git remote, ghcr | pending |

Notes:
- Specs: `tasks/01-evaluate/RESULT.md` (code facts, change list) and `tasks/02-ux-review/UX-REVIEW.md` (sections A1-A3 are the spec; C1 is the release cut; UX wins on wording and interaction). UX-Q1..Q4 taken at the designer's defaults (see DECISIONS.md).
- Not in this release: follow-ups F1-F11 in UX-REVIEW.md C2; reported to the person at the end.
- 04 and 05 run side by side (no overlapping paths). Task folders on disk are numbered one lower than this table from 04 on (03-backend = task 04, ... 07-verify = task 08).
