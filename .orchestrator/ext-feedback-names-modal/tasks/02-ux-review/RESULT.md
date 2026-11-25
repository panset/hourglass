# Result: 02 Senior UX designer review

Status: done. Deliverable: `.orchestrator/ext-feedback-names-modal/tasks/02-ux-review/UX-REVIEW.md` (sections A, B, C).

## What I did
Read INTENT, CONTRACT, the Setups spec (`.orchestrator/setups-declutter/tasks/12-ux/UX-SPEC.md`), every page in `ui/src/pages`,
the shared components and shell, `hooks/useApi.ts`, `hooks/usePanelFocus.ts`, and the server name and user-sort code. Looked at
the e2e screenshots in `e2e/out/bundled/screenshots/`, `.orchestrator/setups-declutter/tasks/16-c4-e2e/screenshots/` and
`.../10-verify/browser/shots/panel-375.png`. No server started, no e2e run, no source changed.

## Key findings
- Names: the server's `auth.DisplayName` (name > username > email, `internal/auth/session.go:158-167`) already feeds holders,
  actors and the greeting. The admin users list sends the raw `name` (`internal/api/admin_views.go:207`), so Pending and All
  users can show a **blank** bold name and blank `aria-label`s (`ui/src/pages/admin-users/index.tsx:47,51,56,60,255`), and
  `sortUsers` sorts by the raw name (`admin_views.go:236`). "Held by" showing an email appears to come from a blank stored
  name (inference; task 01 owns the data path). The spec lists 15 places a person is shown, each with what it should show.
- Modal: the whole dialog scrolls (`ui/src/components/Modal.module.css:12-24`), the backdrop has no handler (`Modal.tsx:86`),
  there is no × and no scroll lock, and Escape cancels a busy confirmation (`Modal.tsx:52-55`). The spec sets one rule for all
  11 dialogs by kind (read-only, confirm, form/secret), keeps the Details content and Price focus unchanged.
- Users pin: done in `sortUsers`, no new badge, sub-line "Built-in local account", updated note; the Audit Actor dropdown
  reads the same list (`ui/src/pages/admin-audit/api.ts:26`) and should sort alphabetically.
- Holistic: 2 must (busy Escape; aria-pressed has no visual style, `ui/src/components/Button.module.css`), 9 should (notably
  a failed 60 s refresh replaces admin pages and unmounts open edit panels; audit pager loses focus; four spellings of the
  rate unit and "band" still in the Acquire dialog, `ui/src/pages/setups/index.tsx:449-455`), 9 polish.
- Cut: C1 = A1, A2, A3 plus unit wording (B2.3) and the pressed style (B1.2). 11 follow-ups listed; F2, F3, F5 are the next
  smallest.

## Files changed
Created in this task folder only: `UX-REVIEW.md`, `PROGRESS.md`, `RESULT.md`.

## Commands run
Read-only: `ls`, `find`, `cat`, `sed`, `grep`, `wc` over `ui/src`, `internal/`, `e2e/out`, `.orchestrator/setups-declutter`.

## Confidence and gaps
- Not run in a browser: keyboard, focus and screen-reader behaviour are read from code and specified, not observed.
- I did not trace how SSO sign-in stores `name`/`username`; the claim that "Held by" shows an email because the stored name
  is blank is inference (task 01).
- The phone-width top bar finding (B2.8) is inference from the CSS; no screenshot of it exists.
- Line numbers for `SetupForm.tsx` dialogs (214, 227-240) were computed from an offset listing; they appear correct.
- Open questions for the owner: UX-Q1 to UX-Q4 in UX-REVIEW.md section C3.
