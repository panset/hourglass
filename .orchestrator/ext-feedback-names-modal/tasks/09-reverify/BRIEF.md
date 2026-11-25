# Re-verify the fixes
Job intent: .orchestrator/ext-feedback-names-modal/INTENT.md (read it first)
Rules: .orchestrator/ext-feedback-names-modal/tasks/CONTRACT.md (read it, follow it).

## Goal
Confirm, independently, that the fixes claimed in `tasks/08-fix/RESULT.md` for the findings in `tasks/07-verify/RESULT.md` are real, tested and introduced nothing new. Strictly read-only: change NO file outside your task folder, make no temporary mutations (an e2e run is using the tree), run no e2e and no `./ci.sh`.

## Check
- F1: the Acquire dialog's peak multiplier comes from the right field, and is right when current = peak, when there is one band, and when pricing data is missing (no "undefined×", no crash). The removed /pressure fetch was truly unused on that page.
- F3: press/release logic in `ui/src/components/Modal.tsx`: both directions, touch/pointer events, right-click, and that a normal backdrop click still closes.
- F6: `auth.DisplayName` trimming: every caller still behaves (greeting, holder name, audit actor); no test weakened to pass.
- F4/F8/F9/F10: docs match the code's actual strings; D101 text is accurate.
- F5, F7, F12 left: are the reasons sound for a patch release?
- Any test that was deleted or loosened by the fix agent (compare with `git diff main -- '*test*'`).
You may run `cd ui && npx vitest run` and `go test ./internal/auth/...` once each, logging to your task folder.

## Done when
RESULT.md starts with a verdict (fixes hold / problems) and lists any problem with `path:line` and a suggested fix.
