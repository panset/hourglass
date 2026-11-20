# 69 Verification of UI and end-to-end scenario

## What ran
- `e2e/run.sh integration`: exit 0, 1 Playwright test, steps I0-I9 all ok, 829 s (log e2e-integration.log).
- `e2e/run.sh bundled`: exit 0, 1 test, A0-A9 all ok, 713 s (e2e-bundled.log).
- `cd ui && npm run typecheck && npm test`: exit 0, 16 files, 170 tests passed (ui.log).
- `./ci.sh`: exit 0, 37 of 37 Bazel tests pass, all reported (cached) (ci.log).
- Cleanup: no new containers, volumes or networks vs. the baseline (*.before files); no new hourglass images (the six images in images.before existed before the run: the run is not strictly from-cold on images, but run.sh rebuilds and reloads them through Bazel).

## Findings
1. low: I5 proves "no second hold.releasing" with a fixed 5 s sleep after a repeated release (integration.spec.ts ~l.368). A late duplicate would be missed. Fix: also compare the event count after the next poll (the test already waits a poll later; re-assert there).
2. low: The "Releasing replaces Release" check is asserted in e2e only on My usage (I5). Setups page is covered by the unit test setups.test.tsx (HOLD-6, two cases) only. Fix: add the same check on the Setups row in I5.
3. low: The collector view's last report times are asserted in e2e only through the admin API JSON (I2); the Settings page rendering is covered only by a unit test (admin-settings.test.tsx). Fix: assert "Last pressure report" text in the Settings page in I2.
4. low: The audit assertion in I8 checks action and actor name only, not the hold id or reason in the entry. A row with the wrong target would pass. Fix: assert the entry's target/detail.
5. low: "Admin UI makes no request to /api/v1/integration/" and "token not in storage, URL or after reload" are not asserted by any test. Source inspection: no `/api/v1/integration` string in ui/src outside schema.gen.ts, no localStorage/sessionStorage use in ui/src, the token is held in IssueDialog state only (index.tsx), and the list response never contains the secret (I1 asserts this). Fix: in I1, record page requests and assert none match /api/v1/integration/, reload and check storage and location.

No high or medium findings. Assertions judged sound: hold.releasing exactly once (event count plus repeated request), still charged (charged_tokens grows by a poll while active), silent phase (poll fails with max_age error, poll.failed received, debit count and charged_tokens unchanged, outage listed, recovery gap not charged), webhook signature (receiver recomputes HMAC over raw body, a mismatch returns 401 and counts in badSignatures, failing the test; delivery would also time out), service release (200, hold.released with service_account and reason, audit actor, 409 on repeat). The scenario covers acquire, deploy, release request, teardown, completion and silence.

## Counts: high 0, medium 0, low 5.

## Could not check
- Token absence from browser storage/URL/after reload and the no-integration-requests property were not exercised live in a browser (inferred from source and unit tests).
- UI create/edit/delete/revoke flows exercised live only as create+issue in I1; the rest via unit tests (170 passing).
- ci.sh results were all cached, so it did not re-run the Go tests.
