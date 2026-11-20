# 69b Strengthen the end-to-end assertions

Status: done. `e2e/run.sh integration` exit 0, 11 of 11 steps (I0-I10), 832 s, from a fresh stack (integration.log, integration.exit). No containers or volumes remain. No product bug found.

## Changes (all in e2e/browser/tests/integration.spec.ts)
1. No second hold.releasing: fixed sleep removed; the count is re-asserted in I6 after hold.released arrives (single in-order queue, internal/events/events.go).
2. I5 also checks the Setups row: Releasing shown, no Release button.
3. I2 checks Settings "Last pressure report" and "Last deployment report" tiles show "received from lab-automation; observed" and not "None received".
4. I8 finds the audit entry by target hold:<id> and asserts actor, after.hold_id, after.setup, after.reason.
5. I1: after Close and after reload, the token secret is absent from localStorage, sessionStorage, URL, cookie, page HTML and text; list response check kept. All admin browser requests are recorded; none go to /api/v1/integration/ (asserted in I9 and I10).
6. New I10: UI create, issue, disable (401), enable (200), revoke (401), delete of account "throwaway".

## Files changed
e2e/browser/tests/integration.spec.ts, this folder. tsc on e2e/browser passes.

## Confidence and gaps
- Ordering argument for item 1 relies on the dispatcher being one in-order queue (read in source, not tested).
- ./ci.sh and SPDX check not run (no BUILD or new source file).
