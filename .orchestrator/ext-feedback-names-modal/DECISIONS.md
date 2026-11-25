# Decisions: ext-feedback-names-modal

- 2026-11-25. A sign-in that yields no name keeps the stored name instead of overwriting it with "" (amends AUTH-2/AUTH-9, new D100). Why: otherwise a name can vanish and "Held by" falls back to the email, which is the reported symptom. Decided: orchestrator (from task 01); the person can overrule.
- 2026-11-25. Admin Users gets an additive `display_name` on `AdminUser` (name, else username, else email) rather than a UI-only fallback. Why: same rule in one place as `holder.name`; API stays backward compatible. Decided: orchestrator (task 01 recommendation).
- 2026-11-25. Modal backdrop dismiss is opt-out per dialog: dialogs with unsaved input or a shown-once token do not close on backdrop click. Why: task 01 found service-account and force-release dialogs would lose data. Decided: orchestrator; details from the UX review (task 02).
- 2026-11-25. UX-Q1..Q4 taken at the designer's defaults: form dialogs always ignore a backdrop click; Details keeps its footer Close beside the new x; the local admin sub-line reads "Built-in local account"; the Audit Actor dropdown sorts alphabetically. Why: keeps the patch moving; all four are small and reversible. Decided: orchestrator; the person can overrule.
- 2026-11-25. Release cut = UX-REVIEW.md C1 only (three asks + Acquire dialog wording + pressed state). F1-F11 are follow-ups for the person. Why: patch release answering an external report. Version v1.3.1.
- 2026-11-25. Backdrop close needs press and release both on the backdrop (verifier F3; spec A2). D101 in docs/decisions.md records the dialog rule (wording by the fix agent). Decided: orchestrator, following the UX spec.
- 2026-11-25. Order changed: verify -> fix -> e2e, so the verifier's temporary mutations cannot disturb an e2e build.
