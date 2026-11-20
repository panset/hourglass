# 62c Verify the fixes to service-account authentication

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/62c-verify-fixes/`.

## Goal
An independent check that the seven findings in
`.orchestrator/integration-api/tasks/62-verify-service-accounts/RESULT.md` are really closed and
that the fixes broke nothing. You did not write the fixes. Do **not** read
`tasks/62b-fix-service-accounts/` (the fixer's own account).

## Expected behaviour (rulings the fixes had to meet)
1. A valid token (enabled account, not expired or revoked) always authenticates, whatever the
   failure count of its address, and is not counted as a failure. Once an address is over the
   limit, a further failed attempt answers 429 `rate_limited` instead of 401. The over-limit
   path gives no status or obvious timing signal about whether a token id exists. Keyed on the
   connection address; forwarded headers are not trusted.
2. The limiter has no check-then-record race and no unbounded sweep cost or unbounded memory.
3. Token ids are accepted only in canonical decimal form (`+1`, `01` are failed attempts).
4. The SVC-8 permissions test fails when a new route lacks a row for all seven principals or
   has the wrong access level.
5. The cookie-skip safeguard on integration routes has a test.
6. Reuse of a deleted account's name is allowed and recorded as a decision in
   `docs/decisions.md`; D47 there describes behaviour 1.

## How
Read the code under `internal/auth/` and `internal/api/` and the tests. Run
`go test -race -count=1 ./internal/auth/... ./internal/api/...` and report how many tests ran.
For items 1, 3 and 4 make one small deliberate break each, confirm a named test goes red, and
restore exactly (compare `git diff | shasum` before and after). Try to find a new way to lock
out a valid caller or to bypass the limiter. Check SVC-1 to SVC-7 still each have a passing
test. Do not run `./ci.sh` (it was run after the fixes).

## Scope
May write: only your task folder; temporary breaks must be restored. Must not touch git state.

## Done when
RESULT.md lists each of the six items as closed or not, with `path:line` evidence, plus any new
finding with severity and a failure scenario.

## Report
Follow the Reporting section of the contract.
