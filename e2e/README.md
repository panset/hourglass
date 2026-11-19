# End-to-end acceptance test

`e2e/run.sh` runs the spec's acceptance path (`docs/SPEC.md`, "Acceptance path") in a real headless browser against the local stacks, in bundled and in split mode.

## What the test does

One Playwright test (`e2e/browser/tests/acceptance.spec.ts`, Chromium, one worker) with steps A0 to A9. It drives the UI as two users: alice (Keycloak sign-in) and the local admin. In order:

- A0 first run of the admin: forced password change, settings.
- A1 alice signs in and sees the not-authorized page.
- A2 the admin approves her.
- A3 alice acquires a setup.
- A4 the pool appears in the simulator and metering starts.
- A5 the pressure moves across a band and the debit rate doubles.
- A6 a third acquire is refused by the hold limit (the UI disables the control; the API call proves the server refuses too).
- A7 release is refused while the pool exists and succeeds once it is gone.
- A8 the month rolls over with a negative balance and the debt carries into the new month.
- A9 the mode check: one origin, with page and API from the same container (bundled) or from the two containers (split).

A failing step is recorded with a screenshot and the run goes on. The only things driven outside the UI are the simulator's control API (pool and pressure) and the clock control (the month jump); the clock control exists only in the test build images.

Each mode takes about 12 to 25 minutes, because the collector polls once a minute. Do not start it within a minute of a real month end.

## Prerequisites

- Docker with Compose v2, Bazel 7.4.1 (the images are built and loaded through it), node and npm.
- Free ports 8080 (app), 8081 (Keycloak), 8990 (simulator control) and 8091 (clock control). Stop any other local stack first.
- A browser: `run.sh` installs the npm packages itself when `e2e/browser/node_modules` is missing (`npm ci`), uses Playwright's Chromium if it launches, otherwise system Google Chrome (the `chrome` channel), and downloads Chromium only when neither works.

## How to run

    e2e/run.sh bundled     # one container serves API and UI
    e2e/run.sh split       # hourglass-api + hourglass-ui
    e2e/run.sh both        # bundled, then split

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
