# End-to-end acceptance test

`e2e/run.sh` runs the spec's acceptance path (`docs/SPEC.md`, "Acceptance path") in a real headless browser against the local stacks, in bundled and in split mode. A third mode, `integration`, runs the integration API scenario (see below).

## What the test does

One Playwright test (`e2e/browser/tests/acceptance.spec.ts`, Chromium, one worker) with steps A0 to A9 and the steps A2g, A2f, A2p, A3n, A3m, A7g, A8p and A8c added by the ledger, labels and onboarding change. It drives the UI as three users: alice (Keycloak sign-in), dave (pre-approved, signs in once) and the local admin. In order:

- A0 first run of the admin: forced password change, settings. The Settings page has no display label keys field, and a save sends the stored `display_label_keys` back unchanged (UI-23).
- A1 alice signs in and sees the not-authorized page.
- A2 the admin approves her.
- A2g the top bar greets alice ("Hello, Alice Example") and the local admin ("Hello, admin").
- A2f alice filters the setups through the Label selector box (UI-21): no "Available labels" block or chip; a click in the empty box lists the key `size` with its values; she types `si`, picks `size` (the box reads `size=`) and then `large` (the page asks the API with `size=large` and lists lab-03 and lab-04); with the caret after `size=` she picks `medium` (`size=medium`, lab-01 and lab-02); an empty box and Enter list all six.
- A2p the admin pre-approves dave@example.com on Users; dave, who has never signed in, lands on My usage with 300 tokens and never sees the not-authorized page. Dave exists in `deploy/keycloak/realm-hourglass.json` for this step only.
- A3n (AUTH-9, D100) names: the admin and dave, who are not the holder, see lab-03 held by "Alice Example" on Setups and in the Details panel, with no email anywhere; admin Users leads alice's row with her name and shows the local account (no name) by its username with "(you)".
- A3m (D101) the Details dialog: closes by the x, by a click on the backdrop and by Escape, each time returning focus to the Details control; Tab and Shift+Tab loop through the x; a drag between the dialog and the backdrop (either direction) does not close it; at 1280 x 720, 390 x 844 and 1280 x 360 the dialog fits, the x and footer Close stay in view, only the body scrolls, the page scroll is locked (overflow hidden, restored, same position) and the wheel over the backdrop does not move the page.
- A3 alice reads the label `size` of lab-03 in its details panel (labels are not in the table) and acquires it.
- A4 the pool appears in the simulator and metering starts.
- A5 the pressure moves across a band and the debit rate doubles.
- A6 a third acquire is refused by the hold limit (the UI disables the control; the API call proves the server refuses too).
- A7 release of a deployed setup is accepted but pending ("Releasing", HOLD-6) and completes by itself at the poll after the pool is gone (HOLD-7).
- A7g the admin makes 23 one-token grants through Users, so the month's ledger has more than one page.
- A8 the month rolls over with a negative balance and the debt carries into the new month.
- A8p alice pages the old month's ledger with Older and Newer (20 rows a page), switches the view and the month from page two and lands on a first page.
- A8c the test clock moves 9 more days; after a collector poll compacts them, the old month's debits of lab-03 are one row per day with "N polls", and the polls, the total charged and the closing balance are the same as before.
- (inside A8) after the grant that makes alice overdrawn, the first row of the admin Users table and the first user of `GET /api/v1/admin/users` is the built-in local account.
- A9 the mode check: one origin, with page and API from the same container (bundled) or from the two containers (split).

A failing step is recorded with a screenshot and the run goes on. The only things driven outside the UI are the simulator's control API (pool and pressure) and the clock control (the month jump); the clock control exists only in the test build images.

Each mode takes about 12 to 25 minutes, because the collector polls once a minute. Do not start it within a minute of a real month end.

## The integration scenario (`e2e/run.sh integration`)

`e2e/browser/tests/integration.spec.ts` runs the bundled stack with `provider.type: reported` and `pressure.type: reported` (`e2e/config.integration.yaml`, layered on by `e2e/compose.integration.yaml`) and a webhook subscribed to `hold.acquired`, `hold.releasing`, `hold.released` and `poll.failed`. The customer's service is scripted in `e2e/browser/reporter/reporter.ts`, running inside the test process: a receiver on host port 9100 that verifies `X-Hourglass-Signature` and records the events (the container reaches it as `host.docker.internal`), and a reporter that sends weights (from W2 on), deployments and pressure with a service token on command and every 20 s as a heartbeat. Steps:

- I0 set-up: admin first run, poll interval 1m, alice approved.
- I1 the admin creates a service account with the four original scopes and issues a token in the UI (W1 adds the fifth). A click on the backdrop leaves the Create dialog open with the typed values, and leaves the token-shown-once dialog open.
- I2 the reporter sends pressure and an empty set; a poll succeeds.
- I3 alice acquires lab-03; the receiver gets `hold.acquired`.
- I4 the reporter reports lab-03 deployed; after the next poll the hold is deployed, metering started and the balance falls.
- I5 alice releases while deployed: 202, "Releasing", exactly one `hold.releasing`, still charged.
- I6 the reporter reports the set without lab-03; the next poll releases the hold and `hold.released` arrives.
- I7 with lab-04 held and deployed the reporter goes silent and the test clock moves past `max_age` (10 minutes): the poll fails, `poll.failed` arrives, no debit is written, the admin collector shows the error; a fresh report recovers and the gap is an uncharged outage.
- I8 the service releases lab-04 with a reason: `hold.released` carries `service_account`, the audit log shows `hold.service_release` under the account's name.
- W1 the admin gives the account `weights:write` on the Service accounts page (before: 403; with the setting off: 409 `not_configured`) and turns reported weights on with a maximum of 3 on the Settings page.
- W2 the reporter sends a weight, with attributes, for every setup (weights before deployments); before acquire the Setups row of lab-03 shows the `reported` tag on its cost cell and its details panel (opened from the cost cell) shows the weight with its source and the attributes; the acquire dialog shows the weight, the attributes and the change warning.
- W3 lab-03 is held and deployed; its debit is at the reported weight (1.15) and shows so on My usage; the reporter changes the weight to 1.4 and the next debit follows.
- W4 alice releases; the reporter keeps reporting the weight and reports the teardown; the poll releases the hold and the last debit is at the reported weight.
- W5 with lab-04 held at its reported weight, a report over the maximum is refused (400, naming the entry), changes nothing, and the Settings page shows the refusal.
- W6 the reporter stops sending weights but keeps sending deployments and pressure; the test clock passes `max_age`; the price returns to the rate rule (source `rule`, the stale weight listed as expired), polls succeed, and Setup inventory says why.
- I9 negative checks: the token is anonymous (401) on other routes, a session cookie gets 401 on the state route, a token without a scope gets 403 on every write route including weights, a disabled account gets 401.
- I10 the Service accounts page disables, enables, revokes and deletes a throwaway account; its token follows.
- U20 (UI-15, UI-20, `docs/design/setups-declutter.md` and its amendment) the admin stores five neutral display label keys through the admin API (no page edits them since D96; the Settings page has no field for them), gives every setup five label values of 24 characters in Setup inventory, pins one weight, adds a setup with a 32-character name and disables one; the reporter sends weights again and alice holds a setup. At 1280 x 900, as alice and as the admin: exactly the headers Setup, State, Held by, Cost (tokens / h) and the hidden Actions while the API still returns the five keys, every row of the response with five cells, no label value in the table, no sideways scroll of the table or the page, every name, state, cost button and Held by line one line, each cost cell equal to the response's `now` and `at_peak` with a tag exactly for `reported` and `override`; the details panel, opened from a cost cell and from Details, has the sections Price, Holder, Labels, Raw details and shows the weight, source, pressure, multiplier, both costs and every label of the same response, and returns focus on close. This is the only check of UI-20 in a real browser (Chromium); the unit tests check the mechanism.
- U22 (UI-21, UI-22) the admin gives two setups (one free, one held) a label value with a space and parentheses in Setup inventory. As alice: a click in the empty Label selector box lists exactly the keys of `label_index` with their first values; typing `key-o` lists one key, picking it gives `key-one=` and exactly that key's values, picking one asks the API with the term and lists one row; Clear filters. Typing the new key and `=` offers the value marked "exact match"; picking it adds the token after "Exact match:", leaves the box empty and lists the two setups; State Free leaves one; a contradicting selector leaves "No setups match the filters." with the token still shown; no request carried the value; the token's button removes it and focus returns to the box. An invalid selector (`key-one in ()`) shows the API's own message and the forms line, marks the box invalid and keeps the rows. The same pick of key and exact value, the token's removal, Escape and Enter on typed text are then done with the keyboard alone.
- S1 the admin sets the base multiplier to 0 and adds a 3x band; after the next poll every cost cell reads `0.0 now · up to N` and the panel shows 0×. A save on the Settings page keeps the stored display label keys. Screenshots `S1-setups-0x`, `S1-panel-0x` (the panel shot is of a 1280 x 1100 window, not the full page).
- S2 the reporter sends pressure 0.95; after a poll the smoothed pressure is in the 2x band and the cells and the panel follow. Screenshots `S2-setups-2x`, `S2-panel-2x`.
- S3 screenshots of the selector box for the owner: `S3-list-open` (the list on Label keys in an empty box) and `S4-token-and-selector` (an exact-match token, `size=medium` in the box, the count and Clear filters).

Outside the UI: the reporter and receiver (the customer's service), the test clock, the Keycloak lifetime call of D34, and the admin API for the collector, the audit log and the second, scope-limited account of I9 (D63). U20 to S3 act in the UI only, except that U20 stores the display label keys through the admin API. It needs the same prerequisites and the free port 9100 on the host. It takes about 18 to 26 minutes: the first poll after start-up sleeps up to 5 minutes before the 1 minute interval applies. Artifacts go to `e2e/out/integration/`; `steps.json` also lists the webhooks received.

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
