# Decisions

Choices made where the spec is silent, plus spike results. Newest last.
Each entry: date, decision, why, who decided. Entries marked **REVIEW** are
the ones the project owner should look at.

| # | Date | Decision | Why | By |
|---|---|---|---|---|
| D1 | 2026-11-19 | Spec open questions use the bracketed assumptions: React; role named `developer`; `vsphere` pressure = highest of CPU, memory and storage allocation ratios across the clusters named in config; weights 1/2/4; a rejected user can be approved later. | Instructed by the owner. | owner |
| D2 | 2026-11-19 | `docs/SPEC.md` is a Markdown export of the source doc at revision 92. The one embedded architecture diagram is replaced by a text note. | The Markdown export cannot carry the drawing; the surrounding prose says the same thing. | agent |
| D3 | 2026-11-19 | Orchestration state (intent, plan, task briefs and results) is tracked in the repo under `.orchestrator/`; `docs/progress.md` is the human-readable status. **REVIEW**: say so if you would rather have `.orchestrator/` git-ignored. | The owner asked for state to be tracked in files in the repo. | agent |
