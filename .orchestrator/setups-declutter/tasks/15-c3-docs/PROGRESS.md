# Progress 15 (C3) docs

- 2026-11-24 Read BRIEF, COMMON-R3, INTENT (with Amendment), DECISIONS, UX-SPEC, DESIGN-R3 s.3-6.
- 2026-11-24 README quick start step 5 removed, steps renumbered 5-7; README line 7 reworded (DESIGN-R3 s.5). deploy/README.md sentence removed; deploy/seed.sample.yaml comment block (3 lines, comment only; e2e mounts this file via deploy/compose.*.yaml, data unchanged) removed.
- 2026-11-24 docs/deploying.md: settings row reworded, "One setting is not seedable" removed, v1.3.0 upgrade note gained three bullets (setting left Settings; stored value stays, unread; split mode).
- 2026-11-24 api/openapi.yaml: listSetups description + two property descriptions (text only). `npm run gen:api`; `bazel test //ui:api_schema_drift_test` PASSED (1 test).
- 2026-11-24 ledger-labels-onboarding.md: "Amended by D96" notes at status, B2, dropdown bullet, UI-9 (text kept as history).
- 2026-11-24 setups-declutter.md: status line pointer + new "Amendment, 2026-11-24" section; owner-decision table untouched.
- 2026-11-24 decisions.md: D96-D99 added. Decision: brief says superseded points are "marked superseded with a pointer", DESIGN-R3 says D93-D95 "left as written"; reconciled by prepending a bold marker to the Decision cell of D33, D69, D74, D79, D93, D94, D95 and changing no existing text.
- 2026-11-24 D97 written from DESIGN-R3's draft merged with UX-SPEC where it overrides (values at start of requirement, captions, arrow button, token place); D98 = DESIGN-R3 draft plus "attributes" and `pricing` (the draft dropped them from D95); D99 new: UX designer role, overrides, UX-Q1..Q5 and R3-Q1..Q3 defaults.
- 2026-11-24 ui-notes.md Setups section rewritten to UX-SPEC; Settings line, Differences items 13, 15, spec item 4 updated.
- 2026-11-24 progress.md section rewritten with every current id and DESIGN-R3 s.4 test names (+ two extra UI-23 tests C2 already wrote); line 75 marked history.
- Next: none. Verifier checks quoted wording against the built page (C1 not finished when this was written).
