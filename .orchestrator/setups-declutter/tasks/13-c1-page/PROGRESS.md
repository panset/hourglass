# Progress: 13 (C1)

- 2026-11-24 Read brief, COMMON-R3, INTENT, DECISIONS, UX-SPEC, DESIGN-R3, verify minors, and the page sources.
- 2026-11-24 Decision: the page owns the one status line (rendered once data is in, so the kept "loading state" test still finds one status); SelectorBox reports through onAnnounce.
- 2026-11-24 Decision: with an empty or missing label_index the box has no combobox role and no list at all (brief: "plain text box"); DESIGN-R3 would still show operators.
- 2026-11-24 Wrote selectorSuggest.ts. Next: SelectorBox.tsx, index.tsx, SetupPanel.tsx, CSS, Table clean-up, tests.
- 2026-11-24 Wrote SelectorBox.tsx; reworked index.tsx (no label columns or chips, tokens, filters-in-force line, status line, cost and holder cells), SetupPanel.tsx (UX-SPEC 5), setups.module.css; deleted LabelChips.tsx and labelTerms.ts.
- 2026-11-24 Shared clean-up: Table.tsx back to its form on main (no shrink, dense, ShrinkText), Table.module.css keeps only `position: relative` on the scroller, index.ts export and the shrink test removed.
- 2026-11-24 Decision: option names are set with aria-label ("size, values: a, b"), not hidden spans, so the name is the same in every engine.
- 2026-11-24 Decision: weight below 0.0005 (minor 4): new formatWeight in lib/format.ts prints the API value unrounded when three decimals would print 0; used on the panel's Weight line only (acquire dialog and My usage untouched).
- 2026-11-24 Tests: selectorSuggest.test.ts (14), setups.test.tsx rewritten where the page changed (59). UI suite 300 of 300, tsc clean.
- 2026-11-24 Mutation check: 10 mutations, all red (mutation.log). Files restored (cmp).
- 2026-11-24 Browser check in Chromium with a mocked API at 1280 (developer, admin), 768, 390, 375: 60 checks, 0 failed (browser/check.cjs, check.log, shots).
- Next: ./ci.sh once, RESULT.md.
- 2026-11-24 ./ci.sh exit 0 (40 Bazel tests). RESULT.md written. Done.
- 2026-11-24 Fix round (task 19): empty message by filter in force; operators only where nothing follows the caret; list pick adds a comma before a following item; 'Selector in force' on the in-force line; labels read defensively; Filter primary only with the list closed; narrow placeholder; sticky caption; docs/progress.md UI-9 test name. 7 new tests, each red with its fix reverted. UI suite 307 of 307, tsc 0, browser check 70 of 70, ./ci.sh exit 0. Left: typing past a space, 10,000+ costs, Settings minor, CSS-only tests.
