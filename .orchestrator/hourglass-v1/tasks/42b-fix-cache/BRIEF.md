# 42b fix the balance-cache regressions found by verification 42
Job intent: .orchestrator/hourglass-v1/INTENT.md. Rules: .orchestrator/hourglass-v1/CONTRACT.md (if `.md` files cannot be written, use `.txt` and put the result in your reply). Findings with failing tests: .orchestrator/hourglass-v1/tasks/42-verify-final-fixes/RESULT.md and its `evidence/` folder.
## Changes
1. HIGH: a rollover that closes two or more months for a user who already has an entry in the current month leaves the cache holding two allowances. Apply the verifier's fix (the rollover passes the true current period when appending) and adopt its failing test as a permanent regression test named with LED-2 / LED-3. Then go further so this class of bug cannot leave a wrong balance in place: after every rollover transaction, set each affected user's cache from the ledger sum of the current period inside the same transaction (one grouped statement, not per-user scans), and assert in the test that cache equals ledger sum after every step.
2. Self-repair at startup: when the application starts (before the first poll and before serving requests), rebuild every user's balance cache from the ledger for the current period in one grouped statement, logging how many caches changed and by how much in total (a non-zero change is logged at warning level). This also repairs the timezone-moved-backwards case (finding 2). Document in `docs/deploying.md`, in two sentences, that changing the configured timezone across a month boundary can move entries between months and should be done just after a rollover.
3. Users with no role have no allowance and are skipped by the rollover: `GET /api/v1/admin/users` returns their balance and month-to-date burn as null (update `api/openapi.yaml`, the generated UI schema, the handler, the contract/permissions tests and the Users page, which shows a dash), instead of a stale cached number.
4. The default-size scale test also covers `GET /admin/users?status=pending` and fails if a per-user query is reintroduced there (prove with the mutation, restore).
5. `readLedger` in `e2e/browser/tests/acceptance.spec.ts` asserts the row order it relies on (newest first) instead of assuming it.
6. The audit filter's example text uses a real action name from the audit vocabulary.
7. Adopt every other test in the verifier's `evidence/` folder that is not already covered, as permanent tests.
## Scope
`internal/ledger/`, `internal/store/` (new migration only if truly needed; never edit an old one), `internal/app/`, `internal/api/`, `api/openapi.yaml`, `ui/src` (generated schema via the generator; Users page; audit page text), `e2e/browser/tests/acceptance.spec.ts`, `docs/deploying.md`, this task folder. You are the only agent running; Docker and the stack ports are yours. No new dependencies.
## Verification
`go test -race -count=1 ./...`; `./ci.sh` exit 0; the full-size `TestNFR_ApiUnder500msAtSpecScale` (see CONTRIBUTING.md); `e2e/run.sh both` (about 25 minutes; 60-minute limit; log to a file, read the tail). All must pass.
## Done when
All pass; the result file maps each finding to its fix and test, and gives rows for docs/decisions.md.
