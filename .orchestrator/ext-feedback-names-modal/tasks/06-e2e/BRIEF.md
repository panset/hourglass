# e2e for the three asks, then the full check
Job intent: .orchestrator/ext-feedback-names-modal/INTENT.md (read it first)
Rules: .orchestrator/ext-feedback-names-modal/tasks/CONTRACT.md (read it, follow it; the waiting rules matter here). Build rules: `.orchestrator/setups-declutter/tasks/COMMON.md`.

## Goal
The browser e2e suites prove the three asks on a real build, and all suites are green.

## Context
- What was built: `tasks/03-backend/RESULT.md`, `tasks/04-modal/RESULT.md`, `tasks/05-ui-names-users/RESULT.md`. Spec: `tasks/02-ux-review/UX-REVIEW.md` sections A1-A3 (acceptance checks).
- How e2e runs, timings, ports: `.orchestrator/setups-declutter/tasks/02-release-map/RESULT.md` and `e2e/README.md`. Previous job's e2e notes: `.orchestrator/setups-declutter/tasks/16-c4-e2e/RESULT.md`.
- The `hourglass-demo` compose stack may hold ports 8080/8081. If it is running, stop it (`docker compose stop`, not `down`, no volume removal), say so in RESULT.md, and leave it stopped.

## Changes
Add or adjust steps in `e2e/browser/tests/*` (and `e2e/README.md` step lists):
1. Names: a person who signed in through SSO with a name is shown by name in "Held by" on Setups and on admin Users; an account with no name shows its fallback, not a blank. Use whatever identities the e2e IdP fixture already has; generic names only.
2. Modal: open Details on Setups; close by x; reopen, close by clicking the backdrop; reopen, close by Escape; focus returns to the Details control; a form dialog stays open on a backdrop click.
3. Users: the local admin row is the first row of the admin Users table.
Existing steps that the Modal or names change broke get fixed in the test only if the new behaviour is the specified one; if a product bug shows, do not paper over it: report it.

## Scope
May write: `e2e/`. Must not touch: `ui/src`, `internal/`, `api/`, `docs/`, git state. If a product fix is needed, stop and report `partial` with the evidence.

## Done when
`./ci.sh` green; `e2e/run.sh both` green; `e2e/run.sh integration` green; RESULT.md gives the step counts per suite and which new steps cover which ask.

## Long-running steps
`./ci.sh` under a minute. `e2e/run.sh both` about 26 minutes; `e2e/run.sh integration` 14-24 minutes. Run each in the foreground with a 45-minute timeout into a log file in your task folder; read the exit code and the tail. Run them one after the other, not together (shared ports).

## Added after verify and fix
- Also read `tasks/07-verify/RESULT.md` finding F2 (existing specs that break: two buttons now named "Close", the old Acquire dialog text) and its section 6 (things only a browser can check: layout, body-only scroll with the footer visible, real focus, scroll lock). Cover section 6 in the new modal steps where it is cheap.
- Read `tasks/08-fix/RESULT.md`: the Acquire dialog wording changed again (peak multiplier), and backdrop close needs press and release both on the backdrop.
- A read-only re-verifier may run the unit suites while you work; it changes no files.
