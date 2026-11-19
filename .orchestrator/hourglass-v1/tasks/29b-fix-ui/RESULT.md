# 29b fix UI: RESULT
Done. `bazel test //ui/...`, `bazel build //ui/...` and `./tools/check_spdx.sh` pass. Only `ui/src` (not schema.gen.ts) changed.

## Findings (task 29 numbering)
1. Fixed. `components/ErrorBoundary` around the app and around the routed pages (keyed by path, so navigating away clears it); message plus Reload. Tests: ErrorBoundary.test.tsx, Gate.test.tsx (render error, lazy chunk failure).
2. Fixed. `hooks/usePanelFocus` for the Grant panel and Edit/Add setup panel (focus in, Tab trapped, Escape closes, focus returns to opener); Modal/ConfirmDialog already trapped, Escape and returned focus. Tests in admin-users and admin-setups. Role change is inline checkboxes, no panel.
3. Fixed. LOW session.tsx: only 401 means signed out; other errors show "Cannot reach the server." with Retry (Gate.tsx). Test added.
4. Fixed. "Free for X more" replaced by the API `metering.reason` (empty when null). "Held for" and the 12-character hint stay (API error decides). Test added.
5. Fixed at desktop widths. Header and name cells no-wrap, long names clipped with title (`Clip`), time cells no-wrap; the side panel stacks under the table below 1500 px so nothing is cut off. Not checked in a browser (no stack started).
6. Narrow screens: left, mobile is out of scope.
7. Fixed (see 3).
8. Left: shared polling store is not a local fix.
9. Partly: ConfirmDialog now focuses Cancel first (safe button). The missing "Nth of M" sentence is not in the spec: left.
10, 11: left, spec-defensible differences from the wireframe. 12: left, Go-side, out of scope. 13, 14: info.
3 (timestamps). Fixed: all times local with the zone abbreviation in each table header and a UTC tooltip (`Time`); chart day labels use the API date unchanged (`formatDay`).

## Changed selectors
- Ledger column header "When" is now "When (<zone>)", likewise Time / Started / When / Last seen by provider / First seen headers in audit, settings, inventory. The e2e spec does not select by these headers, so no edit to it.
- Acquire dialog: initial focus is now Cancel instead of Acquire; the spec clicks Acquire by name, unaffected.
- `Setups` cost cell no longer shows "Free for ..." / "Not charging yet".
