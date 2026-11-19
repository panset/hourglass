# 09b fix store verification findings
Job intent: .orchestrator/hourglass-v1/INTENT.md. Rules: .orchestrator/hourglass-v1/CONTRACT.md. Findings and reproductions: .orchestrator/hourglass-v1/tasks/09-verify-store/RESULT.md and its evidence/ folder.
## Changes (exactly these)
1. Ledger append-only (LED-1): make it impossible to rewrite a ledger row through `INSERT OR REPLACE` (or any conflict-resolution clause) as well as UPDATE and DELETE. Do it in a new migration `0002_...sql` (do not edit 0001), e.g. a BEFORE INSERT trigger that aborts when a row with NEW.id already exists. Test `TestLED1_...` using a raw SQL handle.
2. `Store.Tx`: a panic inside the callback must roll back, release the connection and re-panic; test that a later write succeeds promptly.
3. Balance cache (LED-2): `AppendLedger` (and its Tx form) takes the current period explicitly and sets `users.balance_mt` to the ledger sum of that current period, whatever period the appended entry belongs to. Update `RebuildBalance` consistently. Tests: an expiry written after the new allowance, and a late grant to a past period, both leave the cache equal to the current period's sum. Update existing callers/tests.
4. Keep timestamp precision as is but document it in a comment (millisecond, UTC, fixed width).
No other changes. `internal/store` only; no other package calls these functions yet.
## Scope
May write: `internal/store/`, this task folder. Verify with `go test -race -count=1 ./internal/store/...`, `CGO_ENABLED=0 go test ./internal/store/...`, `bazel test //internal/store/...`, `go build ./...`, both tools scripts.
## Done when
All pass; RESULT.md lists the changed signatures exactly, since the next tasks call them.
