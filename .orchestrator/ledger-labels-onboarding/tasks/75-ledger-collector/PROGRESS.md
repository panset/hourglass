# Task 75 progress

- 2026-11-23 Read BRIEF, INTENT, CONTRACT, design doc (A1, A2, A3, Compatibility, Requirements, Tests, task order), task 74 RESULT, `internal/ledger/service.go`, `internal/collector/collector.go`, `internal/store/ledger_compaction.go`, `internal/api/perf_scale_test.go`.
- Decision: `ledger.Service.CompactDue(ctx)` runs one step (DueLedgerGroups + CompactLedgerGroup in one `st.Tx`), keeps the resume position in memory, and resets it to the start once a search finds nothing more due (so the next daily check also catches late raw rows behind the cursor).
- Decision: the collector sets a "compaction pending" flag in `pruneIfDue` (daily cadence, and the first cycle after start), then runs up to a fixed number of steps per cycle after the prune, taking and releasing `runMu` around each step. A failed step leaves the flag set (retried next cycle) and ends that cycle's compaction.
- Next: implement service op, collector wiring, tests, scale test.
- 2026-11-23 Implemented `ledger.Service.CompactDue` (+ `CompactionStep`), collector `compactIfDue`/`compactStep`, `Ledger` interface gains `CompactDue`. Tests: `internal/ledger/compact_test.go` (3 TestLED10_), `internal/collector/compact_led10_test.go` (4 TestLED10_), `internal/api/perf_compaction_scale_test.go` (scale). All pass.
- 2026-11-23 Scale: reduced (3 holds x 1 year, 315,789 debits) 179 steps, worst 27.8 ms; full (12 holds, 1,263,156 debits) 716 steps, median 17.0 ms, worst 59.8 ms. Decision: CompactStepsPerCycle = 20 (was 10 in a first draft): worst cycle about 1.2 s at the full-scale worst step, far under the 1-minute minimum poll interval.
- 2026-11-23 Deliberate breaks done (lock held across steps; failure clears pending; budget ignored; pending never set; step bound ignored): each made a TestLED10_ test red; restored, green.
- Next: go test ./internal/..., gazelle, ./ci.sh, RESULT.md.
- 2026-11-23 Gazelle added the three test files; go test ./internal/... pass; ./ci.sh exit 0 (ci.exit); -race -count=3 on LED10 tests pass. RESULT.md written. Task done.
