# Progress: 08 (B3) Setups page

- 2026-11-24 Read BRIEF, COMMON, INTENT, DECISIONS (Q-A one line, Q-B = B2), change request, DESIGN.md Rev 2, RESULT.md of 05/06/07, and the page sources.
- 2026-11-24 Decision: export `ShrinkText` from `ui/src/components/index.ts` (brief allows it) rather than a deep import.
- 2026-11-24 Decision: an exact-match chip filter is applied only while its key/value is still in `label_index` with a null selector, so a filter whose chip has gone cannot hide rows with no control left to clear it.
- Next: write weight.tsx helpers, SetupPanel.tsx, index.tsx, LabelChips.tsx, CSS, then tests, then typecheck + vitest, then ./ci.sh once.
- 2026-11-24 Done: weight.tsx (PRICE_CHANGE_SENTENCE, PriceSourceTag, sourceWords), SetupPanel.tsx (new), index.tsx, LabelChips.tsx (B2 exact chips), setups.module.css, admin-settings hint, `ShrinkText` export in components/index.ts.
- 2026-11-24 Done: tests in setups.test.tsx (56 in file). Mutation check: six broken behaviours made 8 tests fail; restored.
- 2026-11-24 Done: tsc exit 0; whole UI suite 18 files / 276 tests pass (tsc.log, vitest-all.log).
- Next: ./ci.sh once (ci.log), then RESULT.md.
- 2026-11-24 Done: ./ci.sh exit 0 (40 targets pass). RESULT.md written. Task complete.
