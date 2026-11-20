# 69b Strengthen the end-to-end assertions

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Ground rules, Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/69b-e2e-assertions/`.

## Goal
Close the five low findings in
`.orchestrator/integration-api/tasks/69-verify-ui-e2e/RESULT.md` (read it in full) by adding
assertions to `e2e/browser/tests/integration.spec.ts`. Code map: `tasks/68-e2e/RESULT.md` and
`tasks/67-ui/RESULT.md` (selectors).

## Changes
1. "No second `hold.releasing`": replace the fixed sleep with something deterministic if the
   stack allows it (for example trigger a later event or a poll, wait for that one to arrive
   at the receiver, then assert the count of `hold.releasing` is still one, since delivery is
   in order). If nothing better exists, keep the wait and say why.
2. Assert "Releasing" and the absence of the Release button on the Setups page as well as on
   My usage.
3. Assert the last pressure and deployment report times are visible in the admin UI's
   collector health, not only in API JSON.
4. The service-release audit check also asserts the hold id and the reason.
5. After issuing the token in the UI and closing the dialog: assert the token string is absent
   from `localStorage`, `sessionStorage`, the URL, the page content after a reload, and the
   admin list response. Record every request the admin browser session makes during the
   scenario and assert none goes to `/api/v1/integration/`.
6. Add, if cheap, a UI pass over edit (disable/enable), revoke and delete of a throwaway
   account, asserting that a revoked token then gets 401.
If any new assertion fails because of the product rather than the test, do not change product
code: report it with evidence and status `partial`.

## Scope
May write: `e2e/`, your task folder. Must not touch anything else or git state.

## Done when
`e2e/run.sh integration` passes from a fresh stack with the new assertions; containers and
volumes removed afterwards. (`./ci.sh` is not needed unless you touch a BUILD file or a file
its checks cover; if you add a source file, run `bash tools/check_spdx.sh`.)

## Long-running steps
The integration mode takes about 13 minutes. Foreground, 40-minute timeout, log in your task
folder, read only exit code and tail. Three-waits rule applies.

## Report
Follow the Reporting section of the contract.
