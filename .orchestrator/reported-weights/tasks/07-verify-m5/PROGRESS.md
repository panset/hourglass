# 07 progress
- 2026-11-23 Read BRIEF, INTENT, CONTRACT, design, D80-D85. Next: background chain go test -> ci.sh -> bazel uncached -> scale test; mutations in a scratch copy of the tree.
- 2026-11-23 go test -count=1 ./... exit 0 (747 top-level tests pass, 1138 subtests, 0 fail, 0 skip; 72 TestWGT*). ./ci.sh exit 0. bazel test //... --nocache_test_results exit 0 (40/40 targets executed). Scale test exit 0 (73.8 s). Bazel -test.v on the 7 packages with WGT tests: 72 TestWGT* PASS, same as go test.
- 2026-11-23 HTTP probes (throwaway test in a scratch copy of the tree, never in the repo): every B1/B2 clause answered as designed; one low finding (top-level keys match case-insensitively). Next: mutations in the scratch copy.
- 2026-11-23 38 mutations over the 15 ids, all red (mutations.log). v1.1.0 response replay (compat-replay.log): only added fields plus one A3-#1 difference. OpenAPI diff, existing-test edits, rules checked. Next: RESULT.md.
- 2026-11-23 RESULT.md written; 0 high, 0 medium, 2 low, 2 info. Done.
