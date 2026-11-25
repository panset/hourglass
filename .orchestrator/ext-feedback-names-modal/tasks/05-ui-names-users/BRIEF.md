# UI: display names everywhere, Users page, two small wording/state fixes
Job intent: .orchestrator/ext-feedback-names-modal/INTENT.md (read it first)
Rules: .orchestrator/ext-feedback-names-modal/tasks/CONTRACT.md (read it, follow it). Decisions: ../../DECISIONS.md. Build rules: `.orchestrator/setups-declutter/tasks/COMMON.md`.

## Goal
Every place the UI shows a person follows `tasks/02-ux-review/UX-REVIEW.md` section A1, the admin Users page follows A3, and the two small C1 extras are in.

## Context
- Spec: UX-REVIEW.md A1 (items P2, P3, P4, P6, P7, P8, P9, P12 as listed in C1), A3 (UI part: sub-line "Built-in local account", the note, "—" in Burned this month, anything else A3 says), B2.3 (units and the word "band" in the Acquire dialog), B1.2 (visible pressed state of By day / Every poll). Section C1 is the cut: do nothing outside it.
- Code facts: `tasks/01-evaluate/RESULT.md` sections 1 and 3.
- Already built, read their RESULT.md first: `tasks/03-backend/RESULT.md` (API now sends `display_name` on admin users, pins the local account first and sorts by display name; `schema.gen.ts` regenerated) and `tasks/04-modal/RESULT.md` (shared Modal changed; do not undo it).
- UX-Q3 and UX-Q4 are decided at their defaults (see DECISIONS.md).

## Changes
- One shared helper for how a person is shown (reuse the existing one the greeting / audit log use if there is one; extend it rather than adding a second), used by every place in the A1 list. No place renders a blank where a person should be.
- Admin Users and Audit pages use `display_name`; the UI must not re-sort the users list in a way that moves the local account off the first row.
- Tests: for each A1 item and each A3 acceptance check, a UI test (named person, unnamed person falling back, the local admin row first with its sub-line). List the mapping in RESULT.md.
- `docs/ui-notes.md`: update the Users section and the people-display rule. Keep it generic.

## Scope
May write: `ui/src/pages/*`, `ui/src/lib/*`, `ui/src/components/*` only if the helper lives there (not `Modal*`), their tests, `docs/ui-notes.md`. Must not touch: `internal/`, `api/`, `ui/src/api/schema.gen.ts`, `e2e/`, git state.

## Done when
UI tests pass (report how many ran); `./ci.sh` green; grep shows no remaining place that prints a raw `.email` or raw `.name` for a person outside the helper, or each remaining one is listed in RESULT.md with the reason.

## Long-running steps
`./ci.sh` under a minute; UI tests about a minute. Run each once into a log file in your task folder and read the tail.

## Added after tasks 03/04 reported
- `./ci.sh` is red right now at `//ui:typecheck`: `ui/src/pages/admin-shared/fixtures.ts:17` (and any other mock of an admin user) needs the now-required `display_name`. Fixing that is part of this task.
- A name that is only spaces must count as no name in the UI helper (fall back to username/email). The backend helper `auth.DisplayName` does not trim; note in RESULT.md whether the UI can ever receive such a value, do not change Go code.
- The server order for users is: local account, then overdrawn users, then display name. The UI keeps the server order.
