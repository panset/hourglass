# 09 Re-verify: result (2026-11-25)

## Verdict: fixes hold. No blocking problem.

Read-only; no e2e, no ci.sh. `npx vitest run`: 21 files, 360 tests pass (ui.log). `go test ./internal/auth/...`: ok (go.log).

- F1: `setups/index.tsx:453` uses `data.pricing.peak_multiplier`, same as `SetupPanel.tsx:73`. Missing pricing gives no wording (no "undefined×", no crash). Current = peak and one band just print the same number. `getPressure` has no other use on the page (grep). Tests now expect "(3×)".
- F3: `Modal.tsx:~125-146` needs mousedown and mouseup on the backdrop, then click, and resets both flags on each click. Drag out and drag in both stay open. Touch and pen go through synthesized mouse events, so they behave the same. A right-click gives no `click`, and the next mousedown resets the flags. A plain backdrop click still closes (A2.7-3 tests). Minor, no fix needed: a `click` with no mousedown or mouseup (some assistive tech or voice control) will not close on the backdrop; Escape and the x still work.
- F6: `auth.DisplayName` (`session.go:159-168`) trims and skips blank. Its callers (session `display_name`, holder, released_by, audit actor, preapproval, admin users) all go through `userDisplayName`, and none relies on an untrimmed value. New cases were added to `TestAUTH9_DisplayNameRule`, none removed.
- F4/F8/F9/F10: docs strings match the code ("tokens / h", "at the highest multiplier (3×)", "Built-in local account"). D101 (a) to (e) matches `Modal.tsx` and the UI behaviour. The `.close:focus-visible` rule is gone. `admin-users/index.tsx:143` reads "tokens / h". `flipFirst` in `session_test.go` is correct.
- F5, F7, F12 left: reasons are sound for a patch release. F5 needs a new dependency and does not arise with one local account. F7 and F12 are not reachable or not required.
- Tests: the removed or changed assertions are all intended. The Acquire text changed, "Close" scoping changed because of the x, and the order test now puts admin first. The one loosening is `admin_responses_test.go:176`, where `&& a.Username != "admin"` exempts the local account from the overdrawn-order check. That is correct now that it is always first. Nothing else was weakened.

## Suggestions (optional)
- `docs/decisions.md` D101: none required.

## Confidence and gaps
jsdom only; real-browser drag and focus ring not checked. Did not re-read every changed test line, only the removed ones.
