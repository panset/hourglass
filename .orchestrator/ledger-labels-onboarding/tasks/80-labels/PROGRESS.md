# PROGRESS 80-labels

- 2026-11-23 Read brief, INTENT, CONTRACT, design B1/B2/Compatibility/SET-8/UI-9.
- 2026-11-23 B1 done: `rules.LabelTerm` (selector.go), `label_index` in listSetups, openapi LabelIndexKey/LabelIndexValue, schema.gen.ts regenerated (npm run gen:api).
- 2026-11-23 Go tests: TestSET8_* in internal/api/setups_label_index_test.go and internal/rules/rules_test.go; updated the exact-key pin in TestSET6_SetupsListForDeveloper (adds label_index).
- 2026-11-23 B2 done: LabelChips.tsx, labelTerms.ts, index.tsx wiring, css; TestUI-9 tests in setups.test.tsx; fixtures.ts gets label_index: [] (type-check).
- 2026-11-23 docs/ui-notes.md Setups section updated.
- Next: deliberate breaks for SET-8 and UI-9, then ./ci.sh.
- 2026-11-23 Breaks done and restored; gazelle run (no BUILD change needed beyond existing); ./ci.sh exit 0 (ci.exit). Wrote RESULT.md.
