# Result 04-modal: done (ci.sh red only in the other agent's files)

## What changed
- ui/src/components/Modal.tsx, Modal.module.css: header (title + x, aria-label "Close", disabled while busy), body is the only scroller (max-height min(90vh, 100dvh-32px), overscroll contain), footer fixed; toast shadow; props `dismissOnBackdrop` (default true) and `busy`; backdrop closes only when mousedown and click both hit the backdrop; busy blocks x, backdrop, Escape; Escape ignored while `isComposing`; html overflow hidden + scrollbar-width padding, ref-counted across dialogs, restored to prior values on close and on unmount. Focus order/initial focus code unchanged (the x is in the header, outside the body query, so Price/Cancel/token field still take focus). ConfirmDialog forwards `busy` and new `dismissOnBackdrop`.
- Call sites: admin-service-accounts/index.tsx (create/edit: dismissOnBackdrop=false + busy; issue token: same; token shown once: false), admin-setups/SetupForm.tsx (force-release: false). Details, Acquire and the other confirmations use the default (backdrop closes).
- Not done (spec "should"): `inert` on app root (A2.4), overflow-dependent rule (rules always on).
- Existing tests updated: bare "Close" queries now pick the footer Close (two buttons named Close); Tab-order test includes the x; one setups test checked "no x in dialog", now checks the body only.

## A2.7 check to test
- 1/2 layout: components.test.tsx "A2.7-1/2 ..." (structure only; sizes at 1280x720 / 390x844 are CSS and need the e2e or a browser, not checked in jsdom)
- 3: components "A2.7-3 ..."; setups.test "A2.7-3 A2.7-5 Details closes on backdrop and x"
- 4: components "A2.7-4 ..."; existing Escape tests
- 5: components "A2.7-5 ..." (name Close, Shift+Tab reaches x); setups test above
- 6: components "A2.7-6" x2 (Price, Cancel)
- 7: components "A2.7-7 ..." (mousedown inside, click on backdrop)
- 8: components "A2.7-8" x2 (lock, exact restore, unmount, two dialogs). Wheel/scroll position/no layout shift not checkable in jsdom.
- 9: components "A2.7-9"; admin-service-accounts.test "A2.7-9 Create service account keeps the typed name"
- 10: admin-service-accounts.test "A2.7-10 Token shown once ..."
- 11: components "A2.7-11" x3 (backdrop = Cancel with focus return; busy blocks Escape/x/backdrop); setups.test "A2.7-11 Acquire ..."
- 12: admin-setups.test "A2.7-12 ..."; components "A2.7-12"
- Also: edit and issue-token dialogs ignore backdrop; confirmations without a field close on backdrop (service accounts, admin setups); IME Escape.

## Commands
- `npx vitest run` in ui/: 19 files, 330 tests passed (log ui4.log).
- `./ci.sh`: exit 1 at the gazelle step: internal/api/BUILD.bazel lacks `display_names_test.go` (the backend agent's file). Run twice (ci.log). Not touched. UI steps after it did not run via ci.sh; `tsc --noEmit` shows only `display_name` missing in ui/src/pages/admin-shared/fixtures.ts (schema.gen.ts is ahead of fixtures; a later task).

## Confidence and gaps
- Layout, dvh sizing, 44px coarse targets, scroll lock against a real scrollbar and wheel behaviour are inference from CSS, not verified in a browser. e2e not run (out of scope).
