# RESULT 35-e2e

Status: done. The browser acceptance test passes in both modes, each through `e2e/run.sh <mode>` (exit 0, stack and volume removed afterwards). I did not re-run the single `e2e/run.sh both` command end to end (about 24 min); it only calls the two runs below in sequence. `tools/check_spdx.sh` passes.

## Files
New: `e2e/browser/{package.json,package-lock.json,tsconfig.json,playwright.config.ts,tests/acceptance.spec.ts}`, `e2e/run.sh` (executable). Edited: `.bazelignore` (`e2e/browser`), `.gitignore` (`/e2e/out/`; node_modules and Playwright artifacts were already ignored). This folder: PROGRESS.md, RESULT.md, bundled-steps.json, split-steps.json (per-step observed values), bundled.log, split.log. No product code, no `ui/src` test ids.

## How it works
Playwright 1.49.1, Chromium only, one test with steps A0..A9 (a failing step is recorded with a screenshot and the run goes on; a trace of both contexts is kept on failure). Two browser contexts: alice (Keycloak login form) and the local admin (admin form, forced password change). Outside the UI only: simulator control API (pool create/delete, pressure), e2e clock control, and one Keycloak setup call (see Decisions). Waits are page reloads every 10 s until the collector's effect shows, never fixed sleeps. `e2e/run.sh` loads images, `up -d --wait`, runs the test, writes `e2e/out/<mode>/` (playwright-report, report.json, steps.json, screenshots, containers.log, logs) and always runs `down -v`.

## Step table (both modes gave the same values; bundled run 713 s, split run 714 s including image load and stack start)
| Step | Result | Asserted | Observed |
|---|---|---|---|
| A0 first run (admin) | pass | admin form sign-in, forced password change, Settings: template `{{.SetupName}}_RP`, label key `size`, poll 1m, enforcement ON, bands base 1x, 2x from 0.05; saved and reloaded | |
| A1 not authorized | pass | exact sentence, email shown, no nav/header, no setup names or balances in page text; `GET /api/v1/setups` 403 `no_role` | url `/unauthorized` |
| A2 approve | pass | alice in Pending requests with her email; Approve; she lands on My usage with 300 (card and top bar) | 300 |
| A3 acquire | pass | dialog shows weight 2 and cost per hour; lab-03 held by "You", "Acquired · deploying", nothing metering; admin Users page lists lab-03 under her | dialog "Costs 0.0 / h now (0x), up to 4.0 / h at the highest band" (the multiplier was still the default 0x until the first poll after the settings change) |
| A4 pool + metering | pass | after pool create and a poll: Deployed, weight 2, Burning now "lab-03 weight 2 x 1x"; each of the first two debits within 25 percent of 2 x 1 x 60 s = 0.0333; balance = 300 + sum of listed debits | debits -0.033, -0.033; balance 299.934; 414 s (first poll after start sleeps the default 5 min before the 1-minute interval applies) |
| A5 pressure band | pass | pill 1x -> 2x after cpu=1.0 (smoothed 1% -> 21%, computed against the 5% threshold); two consecutive 2x debits vs last two 1x debits ratio within 1.75..2.25 | 0.033, 0.033 then 0.067, 0.067; ratio 2.03; 121 s |
| A6 hold limit | pass | lab-04 acquired; Setups shows "You already hold 2 of 2 allowed setups." and Acquire disabled; API acquire of lab-01: 409 `hold_limit` "You already hold the maximum of 2 setups." | |
| A7 release | pass | Release refused with "This setup is still deployed. Tear it down, then release."; after pool delete and one poll Release works, lab-03 Free, holds 1 of 2 | 61 s |
| A8 rollover | pass | admin grants a negative amount with a reason (balance -100.267, Overdrawn); clock jumped to 2026-12-01 03:00 UTC; after one poll the new month shows Allowance +300 and Carryover -100.267, balance 199.733 = 300 + carry-over + later entries | 61 s |
| A9 mode check | pass | bundled: `/ui-healthz` is not JSON, `/healthz` ok. split: `/ui-healthz` is `{"status":"ok"}` from the UI container while `/healthz` is proxied to the API container; every browser request went to `http://localhost:8080` (plus Keycloak's login page) | |

## Decisions for docs/decisions.md
1. Bands for the test: base 1x, one band from 5% at 2x (baseline simulator pressure is about 1%; smoothed pressure after one poll at 100% is at least 1/15 = 6.7% because the window holds at most 15 samples). Pressure driven: cpu 1.0.
2. Keycloak token lifetimes: the month jump moves Hourglass's clock 12 days ahead while Keycloak keeps real time, so the sign-in after the jump failed with "Single sign-on did not complete" (Keycloak's 5-minute ID token is long expired on Hourglass's clock; Hourglass checks it against its clock). The test therefore raises the `hourglass` realm's access token and SSO session lifetimes to 40 days through Keycloak's admin API at the start (a third outside-the-UI action, identity-provider setup only). This is a test-environment effect of clock control, not a product bug as far as I can tell. Sessions also expire under the jumped clock, so the test signs in again.
3. Hold-limit refusal is shown in two places: the UI disables Acquire and says "You already hold 2 of 2 allowed setups."; the server message is "You already hold the maximum of 2 setups." (checked by an API call from the browser session, since the UI never sends the request).
4. Nav pending count only refreshes every 60 s, so the test reloads before checking it.

## Environment problem
`npx playwright install chromium` hung on this machine (node 26.7): downloads finish, extraction stalls. I downloaded the chromium, headless-shell and ffmpeg zips and unzipped them into `~/Library/Caches/ms-playwright` by hand and created the INSTALLATION_COMPLETE markers. `run.sh` only runs the install when Chromium does not already launch, so on this machine it does not call it. On a fresh machine the install may stall the same way (inference).

## Confidence and gaps
- Verified by running both modes; screenshots and steps.json in `e2e/out/<mode>/` (git-ignored).
- Not verified: `run.sh both` as one command; amd64; the 15-minute smoothing under a different poll history; running while the real date is within 12 days of a month end is fine (target is computed from the clock), but a real month change during a run would confuse A8.
- A3's "Costs 0.0 / h now (0x)" is the pre-poll default band multiplier; not a defect in my view but worth a look.
