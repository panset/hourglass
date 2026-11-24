# Result: 85b e2e-runs

Status: **done**. Final `e2e/run.sh both` exit 0 (`e2e-both.exit`) and `e2e/run.sh integration` exit 0 (`e2e-integration.exit`) on the final tree. All stacks torn down (only `platform-lab` containers remain).

## Tests passed per run (final tree)
- bundled: 1 Playwright test, all 16 steps pass (A0 to A9 plus A2g, A2f, A2p, A7g, A8p, A8c); `steps-bundled.json`.
- split: 1 test, all 16 steps pass; `steps-split.json`.
- integration: 1 test, all 11 steps pass; `steps-integration.json`.

## Product defects
None. No product code changed; `./ci.sh` was not required (no `ci.exit`; last green run is task 85's, exit 0, with no product or unit-test change since).

## Changes made (both in `e2e/browser/tests/acceptance.spec.ts`; no assertion weakened or removed)
1. A7 (pre-existing step), flaky in split: after the pool was deleted, `untilHome` counted lab-03's rows in "Current holds" right after the ledger rendered, before the holds section had rendered, saw 0 and moved on; the next assertion ("1 of 2 allowed") then failed (screenshot showed lab-03 Releasing, 2 of 2). Fix: the poll callback first waits for the "Current holds · N of 2 allowed" heading, then counts rows.
2. `openUsage` (used by A8, A8p, A8c), flaky in split: after the 9-day clock move the SPA redirects to the sign-in page only after the first API answer, so the helper's immediate check saw neither /login nor an SSO link and skipped the SSO sign-in, and the "My usage" heading never appeared (A8c failed). Fix: wait for either the "My usage" heading or the "Sign in with SSO" link, then sign in if the link is there.
Other files: none (task 85's `deploy/keycloak/realm-hourglass.json` and `e2e/README.md` unchanged). Runs that failed on the way: first split run (A7), first final `both` (split half, A8c); each fixed and then re-run in full, plus the final `both` and `integration`.

## Runs, exit codes
bundled 0 (712 s); split 1 (A7) then 0; integration 0 (830 s); final both 0 (bundled 711 s, split 771 s); final integration 0 (832 s). Logs: `*.log`; `steps-split-failed.json` is the failed split run before fix 2.

## Decisions for docs/decisions.md
- E2E helpers wait for a page to have rendered (holds heading; heading or SSO link) before reading counts or deciding about sign-in, rather than reading right after the ledger table appears.

## Confidence and gaps
- The first bundled run passed without changes; the A7 and A8c races are timing-dependent and were fixed by reasoning from the failure screenshots (inference), then confirmed by two full passing split runs and one full `both`.
- Owner processes `hd` (port 8091) and `hourglassd` (port 8080) listen on IPv4 localhost; the Docker stacks bind the wildcard. Runs reached the container stacks (A0's forced password change and Keycloak users worked), but those host processes were not touched and the owner may want to stop them before the next run.
- `hourglass-tls` was not started or touched by me (it was stopped by the orchestrator); it needs restarting by the orchestrator.
