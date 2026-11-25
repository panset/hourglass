# Shared Modal: close by x, backdrop and Escape; body-only scroll
Job intent: .orchestrator/ext-feedback-names-modal/INTENT.md (read it first)
Rules: .orchestrator/ext-feedback-names-modal/tasks/CONTRACT.md (read it, follow it). Decisions: ../../DECISIONS.md. Build rules: `.orchestrator/setups-declutter/tasks/COMMON.md`.

## Goal
Every dialog built on the shared `Modal` follows the rule in `tasks/02-ux-review/UX-REVIEW.md` section A2, implemented once in the component.

## Context
The spec is UX-REVIEW.md section A2 (all of it, including its acceptance checks and its table of which dialog is which kind) plus B1.1 (nothing closes while a request is running). Code facts are in `tasks/01-evaluate/RESULT.md` section 2. UX-Q1 and UX-Q2 are decided at their defaults: form dialogs always ignore a backdrop click; the Setups Details dialog keeps its footer Close beside the new x.

## Changes
- `ui/src/components/Modal.tsx` + `Modal.module.css`: header with a close (x) button at the top right with an accessible name; backdrop click closes only when the press started AND ended on the backdrop (a text selection dragged out of the dialog must not close it); a prop to opt out of backdrop dismiss; a prop/rule for "busy" that blocks x, backdrop and Escape; header and footer stay put while only the body scrolls; max height per spec; page scroll locked while open and restored on close (also on unmount); existing Escape, focus trap, focus return and aria behaviour kept.
- Every Modal call site under `ui/src/pages/`: set the kind/opt-out per the A2 table. Change props only, not the dialogs' content.
- Tests in `ui/src/components/components.test.tsx` for each A2 acceptance check, and one test per opted-out dialog kind in its page test that a backdrop click leaves it open with its input intact.

## Scope
May write: `ui/src/components/Modal*`, `ui/src/components/components.test.tsx`, `ui/src/components/index.ts` if needed, Modal call sites and their tests under `ui/src/pages/` (modal props and modal tests only). Must not touch: `ui/src/api/schema.gen.ts`, `internal/`, `api/`, `docs/`, `e2e/`, git state. Another agent is editing the Go backend and `schema.gen.ts` at the same time: if `./ci.sh` fails only there, re-run once at the end, then report it rather than fixing it. A later task changes how names are shown on the pages; leave that alone.

## Done when
UI tests pass and you report how many ran; each A2 acceptance check maps to a named test (list the mapping in RESULT.md); `./ci.sh` green (or red only in the other agent's files, stated).

## Long-running steps
`./ci.sh` under a minute; the UI test suite about a minute. Run each once into a log file in your task folder and read the tail.
