# Orchestration decisions

- 2026-11-23: `.orchestrator/ledger-labels-onboarding/` is tracked in git (briefs, results, plan; not logs), as the two earlier jobs are. Orchestrator.
- 2026-11-23: Tasks run sequentially in one working tree although the design allows milestone 6 beside 2-5; per-milestone `./ci.sh` and commits stay clean. Orchestrator.
- 2026-11-23: Task 74 changed three existing store tests (schema version 5 -> 6, index and column lists extended, one version-2 fixture switched from `AppendLedger` to raw inserts). Task 76 (verification) is asked to judge whether any of these weakens a test. Orchestrator.
