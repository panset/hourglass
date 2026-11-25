# Backend: names and users order
Job intent: .orchestrator/ext-feedback-names-modal/INTENT.md (read it first)
Rules: .orchestrator/ext-feedback-names-modal/tasks/CONTRACT.md (read it, follow it). Decisions: ../../DECISIONS.md. Build rules: `.orchestrator/setups-declutter/tasks/COMMON.md`.

## Goal
The API always gives the UI a non-blank display name for a person, never loses a stored name, and lists the local admin account first.

## Context
Read `tasks/01-evaluate/RESULT.md` (sections 1, 3, 4 and the change list) and `tasks/02-ux-review/UX-REVIEW.md` sections A1 and A3 (the parts about the API, sorting and the pin). Do not re-explore what they already established.

## Changes
1. `UpsertOIDCLogin` (`internal/store/users.go`): a sign-in that yields an empty name keeps the stored name; a non-empty name still replaces it. Test both, plus first sign-in with no name.
2. `AdminUser` gets additive `display_name` (name, else username, else email; reuse the existing fallback helper that feeds `holder.name`, do not write a second rule). Existing `name` field unchanged. Update `api/openapi.yaml` and regenerate `ui/src/api/schema.gen.ts` with the repo's generator (find the command; do not hand-edit if a generator exists).
3. `sortUsers` (`internal/api/admin_views.go`): the local account (`auth_source=local`) first, always; the rest by display name, case-insensitive, with a stable tie-break. If the UX spec A3 states an ordering rule for the rest, follow it.
4. Check every other API response that names a person (task 01 section 1 lists them) and confirm with a test that an unnamed person comes out as username/email, not blank. In particular add or confirm a handler-level test that a holder WITH a stored name is returned by name in the setups response (the reported "Held by" symptom), and one for an unnamed holder.
5. Docs: add D100 (grep to confirm it is free) and amend the AUTH-2 / AUTH-9 wording where they live; keep the docs generic (no team names).

## Scope
May write: `internal/`, `api/openapi.yaml`, `ui/src/api/schema.gen.ts`, `docs/` (not `docs/ui-notes.md`), BUILD.bazel files if gazelle requires. Must not touch: any other file under `ui/`, `e2e/`, git state. Another agent is editing `ui/src/components` and `ui/src/pages` at the same time: if `./ci.sh` fails only in UI files you did not touch, re-run once at the end, then report it rather than fixing it.

## Done when
Go tests for the changed packages pass and you report how many ran; `./ci.sh` is green (or red only in the other agent's UI files, stated); openapi and schema.gen.ts agree.

## Long-running steps
`./ci.sh` takes under a minute; Go package tests seconds. Run each once into a log file in your task folder and read the tail.
