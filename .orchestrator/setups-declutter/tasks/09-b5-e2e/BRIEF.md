# 09 (B5) End-to-end steps and owner screenshots
Common rules: `.orchestrator/setups-declutter/tasks/COMMON.md` (read it first; it names the other files to read).
## Goal
Implement task **B5** of DESIGN.md sections 6 ("End-to-end"), 7 and 8: update existing e2e steps that read the old Setups columns, the JSON modal or a label dropdown; add steps **U20** (the real UI-20 test: five display keys, 24-character values, 1280 px, no horizontal scroll, and it must not be able to pass vacuously), **S1** (lab at 0x) and **S2** (a higher band) with their screenshots; then run the integration mode green.
## Context
- The page as built: `tasks/08-b3-page/RESULT.md` (accessible names and strings). The API field: `tasks/05-b1-api/RESULT.md`.
- How e2e runs, its timings and prerequisites: `tasks/02-release-map/RESULT.md`.
- The `hourglass-demo` compose stack (`.orchestrator/reported-weights/demo`) holds ports 8080/8081. The person has agreed that it is stopped before e2e and left stopped, volumes kept: stop it with `docker compose ... stop` (or `down` WITHOUT `-v`); never remove its volumes.
## Scope
May write: `e2e/browser/tests/integration.spec.ts`, other `e2e/browser/tests/*.spec.ts` only where a step reads the old columns/modal/dropdowns, `e2e/README.md`, and, if U20 needs it per DESIGN.md section 8, the D94 note is NOT yours to write: report the exact sentence needed. Must not touch product code; if a step fails because of a product bug, do not fix it: report it with the evidence (step, expected, actual, screenshot path).
## Done when
- `e2e/run.sh integration` exits 0 with U20, S1 and S2 executed (report the number of steps/tests executed).
- Screenshots exist: Setups page and open panel, 1280 px, five label columns, reported weights on, at 0x and at the higher band. Copy them to `.orchestrator/setups-declutter/tasks/09-b5-e2e/screenshots/` and list their absolute paths.
- A grep of `e2e/` finds no remaining step that reads the Weight or Holder's tokens column, the JSON-only modal, or a per-label dropdown; every "Details for …" role selector is exact.
## Long-running steps
`e2e/run.sh integration`: 14-24 min, plus about three poll intervals for the new steps. Run it in the foreground with a 45-minute limit into a log in your task folder; wait once. Do not run `e2e/run.sh both` (the verify task does).
## Report
Per COMMON.md, plus the screenshot paths.
