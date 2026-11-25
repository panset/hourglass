# Result: 09 (B5) End-to-end steps and owner screenshots

Status: **done**. 2026-11-24.

## What was done

- **W2 updated** (`e2e/browser/tests/integration.spec.ts:745-760`): the row keeps `reported` (exact) and "Free". The weight `1.15 · reported (sent by automation)` and the attributes line are now read from the details panel, opened with `getByRole("button", { name: /Price details for lab-03$/ })` (UI-12 as amended, WGT-14). The old `row toContainText("1.15")` and attributes-on-row checks are gone.
- **Helpers** (`integration.spec.ts:212-371`): `ddOf` (`:220`), `loadSetups` (`:223`, returns the `/api/v1/setups` response the page itself received, so shown and compared values are one response), `checkPanel` (`:245`, opens from the cost cell or from Details with `exact: true`; checks focus on the Price h3; compares weight, source words, pressure, multiplier, both costs, peak multiplier, every label, the price-change sentence, the holder, the JSON block; closes with Escape or Close; checks focus is back on the opener), `measureSetups` (`:294`, measures the table in the browser and checks every row's cost text against `now`/`at_peak` and the tag against `weight_source`).
- **U20** (`:1013`): all set-up in the UI. The admin sets five neutral display keys (`key-one`, `key-two-longer`, `k3`, `fourth-key`, `a-rather-long-label-key-name`) on Settings, gives lab-01..lab-06 five values of exactly 24 characters each (one set of wide capitals) in Setup inventory, pins lab-05's weight to 3 (an `override` row), adds `setup-name-of-21chars` (the longest name the UI-20 claim covers, D94(i)), and disables lab-06. The reporter sends weights again; alice acquires lab-02. At 1280 x 900, as alice (6 rows) and as the admin (7 rows, with the disabled one): ten headers in order; table scroller and page `scrollWidth <= clientWidth`; every label cell has the shrink class, is >= 60 px, one line, actually cut, and its title and text equal the API value; every cost text is one line fragment and every cost button and Held by first line one line box; a held row and a tagged row exist. Then the panel from the cost cell (lab-02) and from Details (lab-05) for each role. It cannot pass vacuously: header list, row counts, cell counts, the held/tagged/disabled rows and the "cut" check all must hold.
- **S1** (`:1104`): base multiplier 0 and an added band (0.5 -> 3x) on Settings; lab-02 reported deployed; waits until the API shows multiplier 0 and lab-02 deployed. Every cost cell reads `0.0 now · up to X` (X = at_peak), no sideways scroll; panel of lab-02: `0×`, `0.0 / h`, `2.4 / h (3×)`, "Charging since". Shots `S1-setups-0x`, `S1-panel-0x`.
- **S2** (`:1141`): reporter sends 0.95; waits until `pricing.multiplier` is 2 (smoothed pressure 0.2525 in run 2). Every cell reads `N now · up to M` with the response's values; panel `2×`, `25%`. Shots `S2-setups-2x`, `S2-panel-2x`.
- Header comment of the spec lists U20, S1, S2. `e2e/README.md`: W2 bullet rewritten; bullets for I10 (was missing), U20, S1, S2; duration 18-26 min; "U20, S1 and S2 act in the UI only".

## Commands and outcome

- `npx tsc --noEmit -p tsconfig.json` in `e2e/browser`: exit 0.
- `e2e/run.sh integration`, run 1: exit 1 after 1491 s. Every step passed, U20/S1/S2 included, except I9 and I10: 401 expected, **429** received. The cause was outside the product: the hourglass-demo host connector (`.orchestrator/reported-weights/demo/lab-connector.py`, pid 25143) was still running after the compose stop. It called `localhost:8080/api/v1/integration/state` every 15 s with the demo's token, so the e2e stack rate-limited failed auth from the host. `containers.log` shows `state 429` every 15 s from start-up, and `connector.log` shows `state=429`. I killed that process (the demo's own `stop.sh` does the same; no volume or demo file touched). Run-1 artifacts: `run1-steps.json`, `run1-e2e-integration.log`.
- `e2e/run.sh integration`, run 2: **exit 0 after 1490 s; 1 Playwright test passed, 20 steps executed (I0-I10, W1-W6, U20, S1, S2), 0 failed** (`e2e-integration.log`, `e2e-integration.exit`, `run2-steps.json`).
- U20 measured: table 1152 px = scroller 1152/1152 for both roles; label cells 81 px (developer), 87 px (admin).

## Grep check (done-when 3)

`grep -nE "Holder's tokens|columnheader|Setup details JSON|selectOption|combobox|Details for" e2e/browser/tests/*.ts`: no step reads a Weight or Holder's tokens column or a label dropdown. The only `selectOption` calls are the Month selects in `acceptance.spec.ts:728,770,782`. The JSON block is read only inside the new panel (`:281`). The one "Details for" selector is exact (`:249`). `acceptance.spec.ts` needed no edit (A2f/A3 read text that stays).

## Files changed

`e2e/browser/tests/integration.spec.ts`, `e2e/README.md`. Task folder: `PROGRESS.md`, `RESULT.md`, logs, `run*-steps.json`, `screenshots/`.

## Screenshots (1280 px, five label columns, reported weights on)

`screenshots/S1-setups-0x.png`, `S1-panel-0x.png`, `S2-setups-2x.png`, `S2-panel-2x.png`, plus `U20-setups-developer.png`, `U20-setups-admin.png`.

## Deviations from DESIGN.md

1. S1 adds a third band (0.5 -> 3x) besides base 0. Without it the 2x band would also be the peak, and S2's cells would show one value, not a range.
2. U20 also adds a 21-character setup and a weight override, so the claimed edge case and the `override` tag are covered.
3. Labels were set in the UI, so no D63/D94 sentence is needed.

## Findings (not product bugs)

- Environment: the demo connector process outlives `docker compose stop` and breaks I9/I10 with 429. Anyone who stops the demo before e2e must also stop the connector (`demo/stop.sh` without `--purge` does both).
- The panel screenshots are full-page, so the fixed backdrop covers only the first 900 px, and the dialog is cut at the viewport bottom (it scrolls). This comes from `fullPage: true`, not the page.
- "Held since … (<1 min)" stays the same across polls in the screenshots. Inference: the e2e test clock runs about 22 min ahead of the browser clock (I7/W6 jumps), so the hold starts "in the future" for the browser.

## Confidence and gaps

- High: run 2 is green with all new steps executed. The measurements are in `run2-steps.json`.
- Chromium only. Firefox and Safari were not measured (D94(i) says so).
- U20 checks one-line layout for cost and Held by, not for State or Setup name cells. Those are `nowrap` by mechanism, and the screenshots show them on one line.
- The hourglass-demo stack and its connector are left stopped, volumes kept.
