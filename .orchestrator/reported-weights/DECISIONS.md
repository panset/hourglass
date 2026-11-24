# Orchestrator decisions (reported-weights)

| Date | Decision | Why | By |
|---|---|---|---|
| 2026-11-23 | No plan checkpoint with the owner before starting: the owner's request fixes the task order, the commit policy and the one stop (before merge, push and tag). | Explicit instructions in the request. | orchestrator |
| 2026-11-23 | The orchestrator read the design, `docs/decisions.md` and `CONTRIBUTING.md` itself; `docs/deploying.md`, `docs/integrating.md` and `docs/extending.md` are digested by task 00 and read in full by every task agent that touches them. | Keeps the orchestrator's context for the plan. | orchestrator |
| 2026-11-23 | Assertions that pin the list of exactly four scopes (UI scope list test, e2e I1, store tests) are extended to five when `weights:write` lands. This is the additive requirement of design D3, not a weakened test; recorded in docs/decisions.md with milestone 4. | Raised by task 00. | orchestrator |
| 2026-11-23 | The binary reports `0.0.0-dev` (nothing stamps `internal/version`; v1.1.0 shipped that way by the owner's choice). The release report says so and asks nothing more unless the owner wants stamping. | Task 00, section 3. | orchestrator |
