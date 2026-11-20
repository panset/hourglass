# Result: 68 end-to-end scenario with the `reported` extensions and a scripted reporter

Status: done. No product bug found.

## What was built
- `e2e/config.integration.yaml`: `provider` and `pressure` of type `reported` (`max_age: 10m`), metering `default`, a webhook (fake secret `dev-only-webhook-secret`) for `hold.acquired`, `hold.releasing`, `hold.released`, `poll.failed`, pointing at `host.docker.internal:9100`.
- `e2e/compose.integration.yaml`: override over `deploy/compose.bundled.yaml` + `e2e/compose.bundled.yaml` (swaps the config, adds `host.docker.internal`).
- `e2e/browser/reporter/reporter.ts`: the scripted customer service, in the Playwright process: `Receiver` (verifies `X-Hourglass-Signature`, records events, 401 on a bad signature) and `Reporter` (full-set deployments plus pressure with the token, on command and a 20 s heartbeat; can go silent). No new dependency.
- `e2e/browser/tests/integration.spec.ts`: steps I0 to I9 = brief items a to i (I0 is set-up). Time is moved with the test clock (+11 min, past `max_age`), no real waiting.
- `e2e/run.sh integration` (new mode; not in `both`), `e2e/README.md` (what it does, ~13 to 15 min, port 9100), `e2e/browser/tsconfig.json` (includes `reporter`).
- `e2e/integration-stack.sh up|token|down`: brings the same stack up and leaves it running.
- `e2e/browser/tests/acceptance.spec.ts` step A7: now proves the release of a deployed setup is accepted and pending ("Release requested for lab-03.", row shows "Releasing", no Release button, still there after reload), and completes by itself at the poll after the pool is destroyed (Free on Setups, holds 1 of 2). The old refusal text and the second click are gone; the step still proves HOLD-2's teardown rule, now as D43.
- `docs/decisions.md`: D63 (what the scenario does outside the UI).

## Outside the UI (D63)
The reporter and receiver, the test clock, the Keycloak lifetime call (as in the acceptance run), and the admin API for the collector, the audit log and the second account of I9 (scope `state:read` only). Account creation and token issue run through the UI in I1.

## Things learned about the product (not bugs)
- The default bands give multiplier 0 while the lab is idle, so nothing is charged; I0 sets base multiplier 1 as the acceptance run does. OIDC users have an empty `username` in webhook and state JSON; the scenario compares the email.
- The first poll after start-up sleeps the default 5 minutes before the 1 minute interval applies, so I2 takes about 5 minutes.
- Poll that fails after the clock jump: error names `max_age`; recovery poll records an outage (`duration_minutes` >= 10, `minutes_not_charged`), and no debit above 0.1 token appears.

## Commands and outcomes (logs in this folder)
- `e2e/run.sh integration`: runs 1 and 2 failed on test bugs only (fixed); run 3 (`integration3.log`) and run 4 (`integration4.log`) both passed 10 of 10 steps from a cold start, about 13 min each. The spec was not changed between runs 3 and 4.
- `e2e/run.sh bundled` (`bundled.log`, exit 0, A0 to A9 pass), `e2e/run.sh split` (`split.log`, exit 0, 713 s).
- `./ci.sh` (`ci.log`, `ci.exit` 0, 37 of 37 tests).
- `e2e/integration-stack.sh up`, a curl of state and pressure with the printed token, `token`, `down`: worked.
- After all runs: no hourglass containers or volumes remain.

## Bringing the `reported` stack up and getting a token (for the docs task)
    e2e/integration-stack.sh up      # images are loaded if missing; prints "token: hgsa_..."
    e2e/integration-stack.sh token   # another token for the same account
    e2e/integration-stack.sh down    # remove containers and volume
App http://localhost:8080. Admin login: user `admin`, password `dev-only-e2e-admin-password` (the script changes the initial `dev-only-not-a-secret` itself). Service account `docs-example` has all four scopes. Clock control on http://localhost:8091 (`PUT /clock {"offset":"11m"}`). The webhook points at host port 9100; nothing listens there unless you start one, so deliveries fail quietly (at most once). The stack has `max_age: 10m` and the default 5 minute poll until an admin sets 1m in Settings; the first poll after start fails (no report yet) and that is expected. Base multiplier is 0 until an admin sets it (Settings), so no debits are written by default.

## Files changed
e2e/config.integration.yaml, e2e/compose.integration.yaml, e2e/integration-stack.sh, e2e/run.sh, e2e/README.md, e2e/browser/reporter/reporter.ts, e2e/browser/tests/integration.spec.ts, e2e/browser/tests/acceptance.spec.ts, e2e/browser/tsconfig.json, docs/decisions.md, this folder.

## Confidence and gaps
- Not covered: the `hold.releasing` receipt after a service release of a pending hold; `vsphere` provider with the new release behaviour beyond the acceptance run's A7.
- The scenario waits on collector runs through the admin API (`recent_runs`); it appears robust to timing because it only accepts runs that start after the report was sent.
- `e2e/out/` is git-ignored; artifacts of the last run (split) remain there.
