# Orchestration decisions

- 2026-11-23: `.orchestrator/ledger-labels-onboarding/` is tracked in git (briefs, results, plan; not logs), as the two earlier jobs are. Orchestrator.
- 2026-11-23: Tasks run sequentially in one working tree although the design allows milestone 6 beside 2-5; per-milestone `./ci.sh` and commits stay clean. Orchestrator.
- 2026-11-23: Task 74 changed three existing store tests (schema version 5 -> 6, index and column lists extended, one version-2 fixture switched from `AppendLedger` to raw inserts). Task 76 (verification) is asked to judge whether any of these weakens a test. Orchestrator.
- 2026-11-23: Verification 76 (0 high, 1 medium, 4 low) closed by 76b; D70 and D71 wording corrected in place (unpushed). Not done in 76b: an upgrade test from a database written by the real `d96a950` binary; assigned to task 85. Pre-existing timing flake `TestNFR_LedgerAppendTimeIsLinear` left unedited; to be named in the owner report. Orchestrator.
