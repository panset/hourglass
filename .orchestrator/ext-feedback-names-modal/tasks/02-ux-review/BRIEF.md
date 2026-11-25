# 02 Senior UX designer review
Job intent: .orchestrator/ext-feedback-names-modal/INTENT.md (read it first)
Rules: .orchestrator/ext-feedback-names-modal/tasks/CONTRACT.md (read it, follow it)

## Role
You are a senior UX designer reviewing Hourglass, an open source web app teams use to see and manage developer setups (who holds what, cost, usage) with an admin area. An external team that consumes it reported the three issues in INTENT.md. Read-only: change no source file.

## Goal
Write `UX-REVIEW.md` in your task folder: a spec for the three reported issues, plus a holistic view of the product's UX with other enhancements worth making, ranked.

## Context
- UI is React/TS in `ui/src` (pages in `ui/src/pages`, shared pieces in `ui/src/components`). The Setups page was redesigned in the last release; its spec is `.orchestrator/setups-declutter/tasks/12-ux/UX-SPEC.md`: stay consistent with it and with the existing visual language; do not redesign what was just shipped.
- Screenshots of the current build may exist under `e2e/` (look for png files from the last e2e run) and `docs/`; use them if present. Otherwise work from the code.
- A separate agent is mapping the code facts (task 01); you do not need to trace API fields.

## What to cover
A. The three asks, each as a precise spec a builder can follow:
   1. People's identity: how a person should be shown everywhere (full name; the fallback order when there is no name; whether/where email appears as secondary text or tooltip; truncation; "you" marking; how to tell two people with the same name apart; sorting and filtering by the shown value). Walk every page and list where people are shown and what each should show.
   2. Details modal on Setups: closing (backdrop click, Escape, x button at top right), sticky header/footer vs scrolling body, max height, focus handling and return, scroll lock, mobile width, what happens to modals that contain forms with unsaved input (backdrop click there). State the rule for all modals, not just this one.
   3. Admin Users: local admin account pinned first. How it is marked (badge?), how the pin behaves when the admin sorts or filters, and any other ordering rule for the rest.
B. Holistic pass: walk every page and the shared components as a first-time user and as a daily user. Report inconsistencies and friction (wording, empty/loading/error states, tables, forms, feedback after actions, keyboard and screen-reader access, contrast, responsive behaviour, navigation). Give each finding: where (`path:line` or page), what is wrong, what to do, severity (must / should / polish), size (S = under an hour, M = half a day, L = more).
C. A short recommended cut: which findings ship with this fix release (small, low risk, related to the asks), and which are follow-ups for the owner to decide. Keep the release cut honest: this is a patch release answering an external team's report.

## Scope
May read: the whole repo. May write: only `.orchestrator/ext-feedback-names-modal/tasks/02-ux-review/`. Must not touch: any source, git state.

## Done when
`UX-REVIEW.md` exists with sections A, B, C; every A item is testable ("Escape closes the modal and focus returns to the Details link"); RESULT.md gives the summary and the path.

## Long-running steps
None. Do not start servers or run e2e.
