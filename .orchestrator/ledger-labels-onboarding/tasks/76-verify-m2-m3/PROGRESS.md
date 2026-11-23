# PROGRESS 76
- 2026-11-23 Read BRIEF, INTENT, CONTRACT, design sections. Next: read diff and decisions D70/D71.
- 2026-11-23 Read the code diff (migration 0006, ledger_compaction.go, service CompactDue, collector compactIfDue, DebitMultipliers). Grepped all ledger readers. Next: read tests, run with -race.
- 2026-11-23 go test -race -count=1 store, ledger, collector: pass (race.log). Next: deliberate breaks for LED-7..10.
- 2026-11-23 Breaks done (LED-7 x2, LED-8 x2, LED-9 x3, LED-15, LED-10 x3): all went red, each restored. api scale test under -race fails only on timing (expected race slowdown). Next: query plan check, ci.sh.
- 2026-11-23 Query plans checked on a scratch DB (sqlite3 CLI, ANALYZE): both search queries use ledger_raw_debit_created, the delete uses ledger_raw_debit_hold. Existing-test edits reviewed. Next: ./ci.sh.
- 2026-11-23 ./ci.sh exit 0 but 0/39 executed (all cached). bazel test //... --nocache_test_results: 38/39 pass, store_test failed on TestNFR_LedgerAppendTimeIsLinear (timing, passes 5/5 alone). Next: rerun uncached once.
- 2026-11-23 Second uncached bazel run 39/39 executed and pass; check_core_generic OK; scale test without race passes. git diff empty. RESULT.md written (pass with findings: 0 high, 1 medium, 4 low). Done.
