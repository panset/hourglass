# 07-verify progress
- 2026-11-25 Read BRIEF, INTENT, CONTRACT, DECISIONS, UX-REVIEW, builder RESULTs. Baseline diffstat saved (diffstat.before: 33 files, +739/-78). Next: read diff, run ci/tests.
- 2026-11-25 ./ci.sh exit 0 (40/40, cached). go test -count=1 ./...: run 1 one FAIL TestAUTH3_CSRFRequiredOnPostNotGet (pre-existing 1/256 flake: "00"+csrf[2:] equals csrf when it starts with 00; passed 5/5 alone); run 2 exit 0, 768 top-level PASS. vitest 359/359. bazel --nocache_test_results 40/40.
- Code read: found Acquire dialog shows current multiplier labelled "highest multiplier"; reverse drag (press backdrop, release in dialog) closes. Next: mutations.
- 2026-11-25 Mutations a, b, c, d1, d2 each red, each restored (cmp). Break attempts via a temporary vitest file (deleted). git diff --stat main identical to baseline after all mutations.
- 2026-11-25 RESULT.md written: ready with fixes (F1 Acquire dialog peak multiplier; F2 e2e specs; F3 reverse drag; F4 docs). Done.
