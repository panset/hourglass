# Task 74 progress

- 2026-11-23 Read BRIEF, INTENT, CONTRACT, design doc (Ledger, A1-A3, Compatibility, Requirements, Tests) and the store code (ledger.go, apiuser/apiadmin/ledgersvc queries, store.go, migration tests).
- 2026-11-23 Found existing tests that pin schema version 5, the ledger index list and the table/column list; they need the new version, index, column and table added (not weakened).
- 2026-11-23 Design chosen: day-walk cursor. Find = MIN(created_at) of raw due debits at/after a cursor (partial index), then GROUP BY user, hold, period over that local day's [start, end) in SQL; the step compacts one group under a SAVEPOINT so it is atomic even inside a caller's transaction. Failure injection in tests through a temporary trigger.
- Next: migration 0006, store code, tests.
- 2026-11-23 Migration 0006, store compaction (ledger_compaction.go), DebitMultipliers filter, tests written; go test ./internal/... passes. Deliberate breaks done (LED-7 x2, LED-9 x2, LED-15). Step timing: worst 2000-row step 46 ms on a 518k-row ledger. Next: ./ci.sh.
- 2026-11-23 ./ci.sh exit 0 (ci.log). RESULT.md written. Task complete.
