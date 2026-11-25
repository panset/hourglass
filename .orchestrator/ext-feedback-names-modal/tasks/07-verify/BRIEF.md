# Verify the build independently
Job intent: .orchestrator/ext-feedback-names-modal/INTENT.md (read it first)
Rules: .orchestrator/ext-feedback-names-modal/tasks/CONTRACT.md (read it, follow it).

## Goal
Decide, with evidence, whether the uncommitted work on branch `ext-feedback-names-modal` meets INTENT.md and the spec, and list what does not. You did not build it; try to break it. Change no source file (temporary mutations must be restored exactly; confirm with `git diff --stat` before and after).

## Context
Spec: `tasks/02-ux-review/UX-REVIEW.md` sections A1-A3 and C1 (the cut), `tasks/01-evaluate/RESULT.md`, `DECISIONS.md`. Builders' claims: `tasks/03-backend/RESULT.md`, `tasks/04-modal/RESULT.md`, `tasks/05-ui-names-users/RESULT.md`, `tasks/06-e2e/RESULT.md`. Treat those claims as things to check, not facts. The change itself: `git diff main` plus untracked files (`git status --short`, ignore `.orchestrator/`).

## Check
1. Every A1, A2, A3 acceptance check: is it implemented, and is there a test that would fail without it? Table: check, code `path:line`, test, verdict.
2. Run `./ci.sh` and the Go and UI test suites yourself; report how many tests executed.
3. Mutation checks, one at a time, restored after each: (a) make a nameless sign-in blank the stored name again; (b) remove the local-account pin from `sortUsers`; (c) make the backdrop click close an opted-out dialog; (d) remove the display-name fallback so an unnamed user renders blank. Each must turn at least one test red; name the test.
4. Break attempts: drag-select from inside a dialog to the backdrop; nested/stacked dialogs and scroll lock restore; Escape while busy; a user whose name is only spaces; two users with the same name; a local account that also has a name; case and unicode in the sort; API backward compatibility (`name` still present, nothing removed from openapi); no email shown to someone who could not see it before; nothing specific to any external team in code, fixtures or docs.
5. Scope: nothing changed outside C1; no stray files; docs match the built behaviour and wording.
Do not re-run the long e2e suites; check `tasks/06-e2e/` logs for exit codes and step counts instead.

## Scope
May write: only `.orchestrator/ext-feedback-names-modal/tasks/07-verify/` (plus temporary, restored mutations). Must not touch git state.

## Done when
RESULT.md starts with a verdict (ready / ready with fixes / not ready) and a findings list: blocker / should-fix / minor, each with evidence and the fix suggested.

## Long-running steps
`./ci.sh` under a minute; UI tests about a minute; Go tests under a minute. Log to files in your task folder.

## Order change
The e2e task has NOT run yet (it runs after you, so your temporary mutations cannot disturb an e2e build). There is no `tasks/06-e2e/RESULT.md`; skip the e2e log check. Instead list, for the e2e agent, any behaviour you could only check by reading code (layout, scroll, real focus) that a browser test should cover.
