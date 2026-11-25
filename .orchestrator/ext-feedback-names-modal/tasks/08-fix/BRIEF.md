# Fix the verifier's findings
Job intent: .orchestrator/ext-feedback-names-modal/INTENT.md (read it first)
Rules: .orchestrator/ext-feedback-names-modal/tasks/CONTRACT.md (read it, follow it). Build rules: `.orchestrator/setups-declutter/tasks/COMMON.md`.

## Goal
Every finding in `tasks/07-verify/RESULT.md` is either fixed with a test or listed as deliberately left, with the reason.

## Context
Findings and suggested fixes: `tasks/07-verify/RESULT.md` (F1-F12). Spec: `tasks/02-ux-review/UX-REVIEW.md`. Decisions: `DECISIONS.md`.

## Changes
- F1 (blocker): Acquire dialog must show the peak multiplier where it says "highest multiplier", not the current one. Fix code and the tests that assert the wrong value. If the peak value is not available to the UI from the existing API response, reword per the UX spec B2.3 so the sentence is true with the data there is; do not add API fields.
- F3: a press on the backdrop released inside the dialog (and the reverse) must NOT close it; only press and release both on the backdrop closes. Test both directions.
- F4: docs describe the dialog rule (x, backdrop by kind, Escape, busy) and the new wording; add a decisions row for the modal rule (next free D id; grep first).
- Minors F5-F12: fix each one that is small and safe (a name of only spaces must fall back rather than show blank, at whichever layer it can actually occur; accent-insensitive, case-insensitive server sort if it is a few lines with a test). Leave anything that widens scope and say why.
- NOT yours: F2 (e2e specs) belongs to the next task; stray `-shm`/`-wal` files and the commit contents belong to the orchestrator (do not delete files; just list in RESULT.md the exact untracked paths that must stay out of the commit).

## Scope
May write: `ui/src/`, `internal/`, `docs/`, BUILD.bazel if gazelle needs it. Must not touch: `e2e/`, `api/openapi.yaml` (unless a description is now untrue), git state.

## Done when
`./ci.sh` green; Go and UI suites green with counts reported; RESULT.md has a table F1-F12: fixed (test name) / left (reason).

## Long-running steps
`./ci.sh` under a minute; UI tests about a minute; Go tests under a minute. `TestAUTH3_CSRFRequiredOnPostNotGet` is a known rare flake unrelated to this work: re-run once if it alone fails. Log to files in your task folder.
