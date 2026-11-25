# 06 e2e result: done (2026-11-25)

## Runs
- `./ci.sh`: exit 0, 40/40 (ci.log).
- `e2e/run.sh both` run 1 (e2e-both.log): bundled 18/18; split 17/18, A3n failed (test bug: `getByRole("table", {name:"Users"})` also matched "Pre-approved users" once dave had signed in). Fixed with `exact: true` (three places).
- `e2e/run.sh both` run 2 (e2e-both2.log): exit 0, bundled 18/18 (776 s), split 18/18 (772 s). Steps: bundled-steps.json, split-steps.json.
- `e2e/run.sh integration` (e2e-integration.log): exit 0, 22/22, 1495 s (integration-steps.json).
- A partial rerun was stopped when I noticed the spec fix had not applied; its leftover bundled stack was removed (`compose down -v` of the e2e project only). The hourglass-demo stack was not running; nothing to stop.

## Changes (e2e only)
- acceptance.spec.ts: footer Close via `.last()` (two buttons named Close); Acquire text regex now "Cost now X tokens / h, up to Y tokens / h at the highest multiplier (N×)."; `Locator` import; Users table `exact: true`.
- integration.spec.ts: four Close clicks use `.last()` (checkPanel, W2 panel, two token dialogs).
- README.md: step lists for A3n, A3m, A8 Users order, I1 backdrop checks.

## Which step covers which ask
- Ask 1 names: A3n (new). Admin and a second developer (dave) see lab-03 held by "Alice Example" on Setups, no email; Details panel Held by the same; admin Users row leads with the name; the local account (no name) shows its fallback "admin" with "(you)". Gap: no unnamed SSO user exists in the fixtures, so the unnamed fallback in Held by is covered only by the local account on Users and by unit tests.
- Ask 2 modal: A3m (new). Close by x, backdrop, Escape, each with focus back on the Details control; Tab/Shift+Tab loop includes the x; drags in both directions between dialog and backdrop do not close; section 6 checks at 1280x720, 390x844 (width <=358, no sideways scroll) and 1280x360 (body scrolls by wheel, header x and footer Close stay in view, page unmoved), html overflow hidden while open and restored with same scroll position, wheel over backdrop does not move the page. Form dialog: I1 (Create service account stays open with typed values on a backdrop click; token-shown-once dialog stays open).
- Ask 3 order: inside A8, after alice is made overdrawn, the first row of the admin Users table and the first user of GET /api/v1/admin/users is the built-in local account while alice is overdrawn (non-vacuous: without the pin she would sort first).

## Confidence and gaps
- No product bug found. Not checked: the 44px coarse-pointer size of the x, focus ring look, Issue-token dialog focus target (07-verify section 6 item 5), real scrollbar width shift. Screenshots A3m-closed and A3m-390 are in e2e/out/*/screenshots (git-ignored).
