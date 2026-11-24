# 03 progress

- 2026-11-23 Read brief, intent, contract, design (in full), CONTRIBUTING, M2 RESULT, digest, collector.go, rules.go, reported/weights.go, store weights queries.
- 2026-11-23 Plan: `internal/weights` takes settings.Settings, []rules.Rule, *weights.Report (own pure type, built from reported.Weights or store rows), time; collector reads `st.GetReportedWeights` (exists, no store addition needed). Next: write resolver + table test.
- 2026-11-23 Resolver, collector change, types.go comment, tests (collector, inventory WGT-6, v1.1.0 replay) written; 14 mutations each turn a named test red (mutations.log). Next: gazelle, go test ./..., ci.sh.
- 2026-11-23 gazelle ok (SPDX line added to new BUILD), go test ./... exit 0, ./ci.sh exit 0. RESULT.md written. Done.
