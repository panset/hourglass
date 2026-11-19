# 09 verify internal/store
Job intent: /Users/tejaspanse/projects/hourglass/.orchestrator/hourglass-v1/INTENT.md. Rules: CONTRACT.md beside it. You write no product code.
## Goal
Independently check `internal/store` against docs/SPEC.md "Data model" and the brief at tasks/08-store/BRIEF.md. You did not write it; try to break it.
## Setup
Clone: `git clone /Users/tejaspanse/projects/hourglass <scratch>/verify-09/hourglass`, <scratch> = /private/tmp/claude-501/-Users-tejaspanse-projects/17dec24c-e7ec-455e-9b87-01d2217d5fc9/scratchpad (remove verify-09 first). Work only in the clone.
## Checks
1. `CGO_ENABLED=0 go test -race -count=1 ./internal/store/...` is not possible (race needs cgo): run `go test -race -count=1 ./internal/store/...` and `CGO_ENABLED=0 go test -count=1 ./internal/store/...`; then `bazel test //internal/store/...`. Report test counts.
2. Column-by-column table: for each of the 15 tables in the spec's data-model table, every listed column exists (open a fresh database with the package, query `sqlite_master` and `PRAGMA table_info`), plus the named keys, unique constraints and indexes. List any column or table in the schema that the spec does not name (besides `outages` and `schema_migrations`) with your view on whether it is justified.
3. Behaviour, with your own throwaway tests in the clone: WAL mode and foreign keys actually on for every pooled connection; ledger UPDATE and DELETE rejected, including through a raw SQL handle; duplicate (hold_id, polled_at) sample is a no-op reported as not inserted; two goroutines creating a hold on one setup: exactly one wins with ErrSetupHeld for the other; duplicate setup name gives ErrConflict; deleting a setup with holds/labels behaves sanely; migrations idempotent on re-open; opening a database with a higher schema version fails; timestamps round-trip as UTC and sort lexically (including sub-second values and a non-UTC input time); a Tx that returns an error or panics rolls back; concurrent readers during a write transaction do not fail with SQLITE_BUSY under 20 goroutines.
4. Balance cache: read how `AppendLedger` maintains `users.balance_mt`. The builder says it sets the cache to the ledger sum of the appended entry's period. Work through the LED-3 rollover sequence (expiry in old period, carryover + allowance in new period) and a late correction written to a past period, and state exactly under which call orders the cache ends up not equal to the current period's sum. Recommend the smallest API change that makes the cache correct regardless of order (for example passing the current period, or a separate explicit cache update).
5. Scale sanity (spec: 200 users, 100 setups, 5-minute polling for 13 months is about 100 holds x 112k polls at most): insert 200 users, 100 setups, 100 active holds and 200k usage samples, then time the per-day usage sum for one user and month, the active-holds list and the setups list; report milliseconds and whether an index is missing (`EXPLAIN QUERY PLAN`).
6. Genericity and SPDX: `./tools/check_core_generic.sh` and `./tools/check_spdx.sh` pass; the package imports only stdlib and the SQLite driver.
7. Mutation spot-checks: three deliberate bugs (e.g. drop the partial unique index, remove a ledger trigger, make sample insert non-idempotent) each turn the package's own tests red.
## Scope
May write: the clone and /Users/tejaspanse/projects/hourglass/.orchestrator/hourglass-v1/tasks/09-verify-store/.
## Done when
RESULT.md opens with a verdict and a numbered findings list (severity, path:line, reproduction). Status `done` whatever the verdict.
