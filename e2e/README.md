# End-to-end acceptance test

`e2e/run.sh` runs the spec's acceptance path (`docs/SPEC.md`, "Acceptance path") in a real headless browser against the local stacks, in bundled and in split mode. A third mode, `integration`, runs the integration API scenario (see below).

## What the test does

One Playwright test (`e2e/browser/tests/acceptance.spec.ts`, Chromium, one worker) with steps A0 to A9 and the steps A2g, A2f, A2p, A7g, A8p and A8c added by the ledger, labels and onboarding change. It drives the UI as three users: alice (Keycloak sign-in), dave (pre-approved, signs in once) and the local admin. In order:

- A0 first run of the admin: forced password change, settings.
- A1 alice signs in and sees the not-authorized page.
- A2 the admin approves her.
- A2g the top bar greets alice ("Hello, Alice Example") and the local admin ("Hello, admin").
- A2f alice filters the setups with the label chips: add, replace and remove a term (the Setups page asks the API with the term).
- A2p the admin pre-approves dave@example.com on Users; dave, who has never signed in, lands on My usage with 300 tokens and never sees the not-authorized page. Dave exists in `deploy/keycloak/realm-hourglass.json` for this step only.
- A3 alice acquires a setup.
- A4 the pool appears in the simulator and metering starts.
- A5 the pressure moves across a band and the debit rate doubles.
- A6 a third acquire is refused by the hold limit (the UI disables the control; the API call proves the server refuses too).
- A7 release of a deployed setup is accepted but pending ("Releasing", HOLD-6) and completes by itself at the poll after the pool is gone (HOLD-7).
- A7g the admin makes 23 one-token grants through Users, so the month's ledger has more than one page.
- A8 the month rolls over with a negative balance and the debt carries into the new month.
- A8p alice pages the old month's ledger with Older and Newer (20 rows a page), switches the view and the month from page two and lands on a first page.
- A8c the test clock moves 9 more days; after a collector poll compacts them, the old month's debits of lab-03 are one row per day with "N polls", and the polls, the total charged and the closing balance are the same as before.
- A9 the mode check: one origin, with page and API from the same container (bundled) or from the two containers (split).

A failing step is recorded with a screenshot and the run goes on. The only things driven outside the UI are the simulator's control API (pool and pressure) and the clock control (the month jump); the clock control exists only in the test build images.

Each mode takes about 12 to 25 minutes, because the collector polls once a minute. Do not start it within a minute of a real month end.

## The integration scenario (`e2e/run.sh integration`)

`e2e/browser/tests/integration.spec.ts` runs the bundled stack with `provider.type: reported` and `pressure.type: reported` (`e2e/config.integration.yaml`, layered on by `e2e/compose.integration.yaml`) and a webhook subscribed to `hold.acquired`, `hold.releasing`, `hold.released` and `poll.failed`. The customer's service is scripted in `e2e/browser/reporter/reporter.ts`, running inside the test process: a receiver on host port 9100 that verifies `X-Hourglass-Signature` and records the events (the container reaches it as `host.docker.internal`), and a reporter that sends weights (from W2 on), deployments and pressure with a service token on command and every 20 s as a heartbeat. Steps:

- I0 set-up: admin first run, poll interval 1m, alice approved.
- I1 the admin creates a service account with the four original scopes and issues a token in the UI (W1 adds the fifth).
- I2 the reporter sends pressure and an empty set; a poll succeeds.
- I3 alice acquires lab-03; the receiver gets `hold.acquired`.
- I4 the reporter reports lab-03 deployed; after the next poll the hold is deployed, metering started and the balance falls.
- I5 alice releases while deployed: 202, "Releasing", exactly one `hold.releasing`, still charged.
- I6 the reporter reports the set without lab-03; the next poll releases the hold and `hold.released` arrives.
- I7 with lab-04 held and deployed the reporter goes silent and the test clock moves past `max_age` (10 minutes): the poll fails, `poll.failed` arrives, no debit is written, the admin collector shows the error; a fresh report recovers and the gap is an uncharged outage.
- I8 the service releases lab-04 with a reason: `hold.released` carries `service_account`, the audit log shows `hold.service_release` under the account's name.
- W1 the admin gives the account `weights:write` on the Service accounts page (before: 403; with the setting off: 409 `not_configured`) and turns reported weights on with a maximum of 3 on the Settings page.
- W2 the reporter sends a weight, with attributes, for every setup (weights before deployments); the Setups page shows lab-03's weight, the `reported` tag and the attributes before acquire, and the acquire dialog shows the weight, the attributes and the change warning.
- W3 lab-03 is held and deployed; its debit is at the reported weight (1.15) and shows so on My usage; the reporter changes the weight to 1.4 and the next debit follows.
- W4 alice releases; the reporter keeps reporting the weight and reports the teardown; the poll releases the hold and the last debit is at the reported weight.
- W5 with lab-04 held at its reported weight, a report over the maximum is refused (400, naming the entry), changes nothing, and the Settings page shows the refusal.
- W6 the reporter stops sending weights but keeps sending deployments and pressure; the test clock passes `max_age`; the price returns to the rate rule (source `rule`, the stale weight listed as expired), polls succeed, and Setup inventory says why.
- I9 negative checks: the token is anonymous (401) on other routes, a session cookie gets 401 on the state route, a token without a scope gets 403 on every write route including weights, a disabled account gets 401.

Outside the UI: the reporter and receiver (the customer's service), the test clock, the Keycloak lifetime call of D34, and the admin API for the collector, the audit log and the second, scope-limited account of I9 (D63). It needs the same prerequisites and the free port 9100 on the host. It takes about 15 to 20 minutes: the first poll after start-up sleeps up to 5 minutes before the 1 minute interval applies. Artifacts go to `e2e/out/integration/`; `steps.json` also lists the webhooks received.

To bring the same stack up by hand and keep it running, see `e2e/integration-stack.sh`.

## Prerequisites

- Docker with Compose v2, Bazel 7.4.1 (the images are built and loaded through it), node and npm.
- Free ports 8080 (app), 8081 (Keycloak), 8990 (simulator control) and 8091 (clock control), and 9100 for the integration scenario's webhook receiver. Stop any other local stack first.
- A browser: `run.sh` installs the npm packages itself when `e2e/browser/node_modules` is missing (`npm ci`), uses Playwright's Chromium if it launches, otherwise system Google Chrome (the `chrome` channel), and downloads Chromium only when neither works.

## How to run

    e2e/run.sh bundled     # one container serves API and UI
    e2e/run.sh split       # hourglass-api + hourglass-ui
    e2e/run.sh both        # bundled, then split
    e2e/run.sh integration # the integration API scenario (not part of `both`)

The exit status is the test's. Output is long; send it to a file and read the tail:

    e2e/run.sh both > /tmp/e2e.log 2>&1; echo $?; tail -40 /tmp/e2e.log

The script loads the images (`deploy/load-images.sh`), starts `deploy/compose.<mode>.yaml` with the override `e2e/compose.<mode>.yaml` (the `*-e2e` test builds and the clock control port), runs the test and always tears the stack down, volume included. The product compose files do not contain the test builds or the control port; the stacks started by hand or by `deploy/smoke.sh` are the product images.

## Known problem: Playwright's Chromium download can hang

`npx playwright install chromium` can finish downloading and then stall while extracting (seen with node 26 on macOS). `run.sh` only runs it when neither Chromium nor system Chrome launches, so a machine that already has a browser never calls it. If it hangs on yours:

1. Stop it (Ctrl-C).
2. Install Google Chrome and run again; `run.sh` picks it up through the `chrome` channel. Or
3. Download the Chromium, headless-shell and ffmpeg zips that `playwright install` printed, unzip them into the Playwright cache (`~/Library/Caches/ms-playwright` on macOS, `~/.cache/ms-playwright` on Linux) and create an empty `INSTALLATION_COMPLETE` file in each unpacked folder.

## Where the artifacts go

`e2e/out/<mode>/` (git-ignored), recreated on every run: `playwright.log` (the console output), `steps.json` (each step's status, timing, what was asserted and observed), `report.json`, `playwright-report/` (HTML report; open `index.html`), `screenshots/`, `test-results/` (traces of failures), `containers.log`, `up.log`, `down.log`, `load-images.log` and `seconds.txt`.
