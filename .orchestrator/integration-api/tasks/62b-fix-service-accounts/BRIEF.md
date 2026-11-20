# 62b Fix the findings of verification 62

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Ground rules, Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/62b-fix-service-accounts/`.

## Goal
Every finding in `.orchestrator/integration-api/tasks/62-verify-service-accounts/RESULT.md` is
either fixed with a test or closed with a recorded decision, as directed below.

## Context
- The findings: the RESULT.md above (read all of it).
- How the code is organised: `.orchestrator/integration-api/tasks/61-service-accounts/RESULT.md`.
- Design: `docs/design/integration-api.md`, "Service accounts" and SVC-1 to SVC-8.

## Orchestrator's rulings
1. **High: a valid token is refused with 429 once its address has too many failures.** Fix:
   a request that presents a valid token (enabled account, not expired or revoked) always
   authenticates, whatever the address's failure count, and does not count as a failure. Only
   failed attempts are limited: once an address is over the limit, a further *failed* attempt
   answers 429 `rate_limited` instead of 401. This is what the design says ("failed bearer
   attempts are rate-limited per client address") and removes the lockout. Keep keying on the
   connection address; do not start trusting forwarded headers. Rewrite decision row D47 in
   `docs/decisions.md` in place to describe the final behaviour (it is uncommitted, so editing
   it is fine), including that behind a reverse proxy all callers share one failure budget but
   no valid caller can be locked out. Add a test under SVC-7 that proves a valid token still
   works from an address that is over the limit, and that a bad token from it gets 429.
   Make sure the over-limit path gives no timing or status signal about whether a token id
   exists.
2. Limiter sweep cost and the race in its check: fix both; `go test -race` must cover the race.
3. Token ids such as `+1` or `01`: accept only the canonical decimal form; anything else is an
   ordinary failed attempt (401). Add a test.
4. Weak SVC-8 test: strengthen it as the verifier suggests, so that a route added to
   `routes.go` without a matrix row for all seven principals fails.
5. Untested cookie-skip safeguard: add the test.
6. Account names reusable after deletion: leave the behaviour (the design stores names as text
   so that deleting an account never rewrites history) and record it as a new decision row with
   the next free number: a re-created name is a new account and old audit rows under that name
   refer to the earlier one.
7. Any other finding in the RESULT.md: fix it if it is a defect against the design; otherwise
   say why not in your RESULT.md.

## Scope
May write: `internal/auth/`, `internal/api/`, `api/openapi.yaml` and `ui/src/api/schema.gen.ts`
only if a finding requires it, `docs/decisions.md`, your task folder.
Must not touch: `docs/SPEC.md`, `docs/design/`, `docs/integrating.md`, git state.

## Done when
Each finding has a line in RESULT.md: fixed (test name) or closed (decision number).
`go test -race -count=1 ./internal/...` and `./ci.sh` pass.

## Long-running steps
`./ci.sh` takes roughly 5 to 15 minutes: run it once at the end, foreground, 30-minute timeout,
output to a log file in your task folder, read only the exit code and tail.

## Report
Follow the Reporting section of the contract.
