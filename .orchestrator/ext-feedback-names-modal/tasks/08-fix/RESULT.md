# 08 Fix: result (2026-11-25)

| Finding | Status |
| --- | --- |
| F1 Acquire shows peak multiplier | Fixed: `setups/index.tsx` uses `data.pricing.peak_multiplier` (no wording when `pricing` is absent, as in the panel). The unused `GET /pressure` fetch on that page was removed. Tests changed to "(3×)": HOLD-1, UI-12 acquire, B2.3 |
| F2 e2e specs | Not mine (next task) |
| F3 reverse drag | Fixed: `Modal.tsx` tracks mouseup target too; new test `F3 a press that starts on the backdrop and is released inside the dialog does not close it`; A2.7-7 covers the other direction |
| F4 docs | Fixed: `docs/ui-notes.md` Details/Acquire/units text plus a "Dialogs (D101)" section; `docs/decisions.md` D101 row |
| F5 accent-insensitive server sort | Left: needs golang.org/x/text (not in go.mod) or a hand-written fold; widens dependencies and licence check. UI Audit dropdown already locale-aware. Local admin is always first, so the two-local-accounts case does not arise |
| F6 spaces-only name | Fixed at the server: `auth.DisplayName` trims and skips blank values; cases added to `TestAUTH9_DisplayNameRule`. UI already guards |
| F7 Escape closes stacked dialogs | Left: no screen stacks two Modals; changing event propagation risks the confirm/Grant behaviour |
| F8 x focus ring | Fixed: `.close:focus-visible` rule removed (product ring applies) |
| F9 Grant panel "burning 4.0 / h" | Fixed: now "burning 4.0 tokens / h" (`admin-users/index.tsx`); no test asserted the old text |
| F10 flaky CSRF test | Fixed: `flipFirst` in `internal/auth/session_test.go` |
| F11 files to keep out of the commit | Orchestrator; list below |
| F12 `inert` on app root | Left: UX "should", not in the C1 cut |

## Keep out of the commit (untracked or unrelated; not deleted)
- `testdata/upgrade-v1.1.0/hourglass.sqlite3-shm`, `testdata/upgrade-v1.1.0/hourglass.sqlite3-wal`
- `.orchestrator/setups-declutter/PLAN.md` (modified, previous job)
- Stray `ci.log`, `gotest.log`, `baseline.sha` under `.orchestrator/integration-api/` (other job)

## Files changed
ui/src/pages/setups/index.tsx, setups.test.tsx; ui/src/components/Modal.tsx, Modal.module.css, components.test.tsx; ui/src/pages/admin-users/index.tsx; internal/auth/session.go, session_test.go, greeting_test.go; docs/ui-notes.md, docs/decisions.md.

## Runs
- `./ci.sh`: exit 0, 40 of 40 bazel tests pass (`ci.log`).
- `go test -count=1 ./...`: exit 0, 35 packages ok; 768 top-level tests passed (`gotest.log`).
- `npx vitest run`: 21 files, 360 tests passed (`ui.log`).

## Confidence and gaps
Browser-only behaviour (real mouse drag, focus ring look) not checked; jsdom event sequences only. D101 date and wording are mine; DECISIONS.md was not edited.
