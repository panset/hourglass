# 15 (C3) Docs, API descriptions, seed and deploy notes for Revision 3
Common rules: `.orchestrator/setups-declutter/tasks/COMMON-R3.md` (read it first; it names the specs).
## Goal
Implement task **C3** of DESIGN-R3.md sections 5 and 6: every document written for the first build that is now wrong is corrected, quoting UI wording from UX-SPEC.md; D96+ entries are added to `docs/decisions.md` (D93-D95 are amended only as DESIGN-R3 says, never silently rewritten: superseded points are marked superseded with a pointer); `docs/design/setups-declutter.md` gains an "Amendment" section recording the owner's change of 2026-11-24 in the owner's words (INTENT.md Amendment) and the amended / retired / new requirement ids (UI-15, UI-19, UI-20, UI-9, SET-6, UI-21..UI-23), without editing the original owner-decision table; openapi descriptions (descriptions only) and regenerated types; the quick-start, seed and deploy notes that tell admins to set display label keys are removed or corrected; `docs/deploying.md` upgrade note says the setting left the Settings page and a stored value stays, unread. Record the senior UX designer's role and any UX-SPEC decision that overrides the change request, as a decision entry.
## Scope
May write: `api/openapi.yaml` (description text only; no schema change), `ui/src/api/schema.gen.ts` (by the generator), `docs/ui-notes.md`, `docs/decisions.md`, `docs/design/setups-declutter.md`, `docs/design/reported-weights.md`, `docs/design/ledger-labels-onboarding.md`, `docs/integrating.md`, `docs/progress.md`, `docs/deploying.md`, `README.md`, `deploy/README.md`, `deploy/seed.sample.yaml` (only if DESIGN-R3 says so, and note that e2e seeds from it). Must not touch `docs/SPEC.md`, code, tests, e2e.
## Done when
- A grep of the docs you own finds no sentence that says the Setups page has label columns, "Available labels" chips, per-label dropdowns, or an admin display-label-keys control, except in history (superseded decisions, the original change request text) where it is marked as such.
- `docs/progress.md` lists every current requirement id with its test names per DESIGN-R3 section 4.
- The schema drift test (`//ui:api_schema_drift_test`) passes after regeneration.
- RESULT.md lists every doc sentence that quotes UI wording, for the verifier to check against the built page.
## Long-running steps
None over a minute. Do not run `./ci.sh` or e2e.
## Report
Per COMMON-R3.md.
