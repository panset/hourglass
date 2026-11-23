# Intent: ledger roll-up and paging, label discovery, greeting, pre-approved SSO users

Date: 2026-11-23. Owner: Tejas Panse. Branch: `ledger-labels-onboarding`, cut
from `main` at `d96a950`; the design doc is commit `414b51d`.

## Goal (owner's words)

"Implement, test and release the design in docs/design/ledger-labels-onboarding.md in this repository."

## Sources of truth

- `docs/design/ledger-labels-onboarding.md` is the contract for this job. Its
  Requirements section (LED-7 to LED-15, UI-8 to UI-11, SET-8, AUTH-9, PRE-1 to
  PRE-8, UPG-1 to UPG-3) is the definition of done. Its "Owner decisions" are
  made; do not reopen them.
- `docs/decisions.md`, `CONTRIBUTING.md`, `docs/deploying.md`.
- Task-agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` (reused). Where
  it says "the spec", read "the design doc"; where it points at the v1 INTENT,
  read this file.
- Job state lives under `.orchestrator/ledger-labels-onboarding/`.

## Rules (owner's words)

- Work on the branch ledger-labels-onboarding, which already holds the design doc (414b51d). Do not commit to main.
- Customers run Hourglass today. An upgraded installation must work with no configuration change, and existing API calls must answer as before. Treat the design's Compatibility section as requirements.
- Never edit docs/SPEC.md or the design doc. Record this work in docs/decisions.md from D69 on as an owner-directed change, and list there the edits the spec's source needs.
- Where the design leaves a detail open, choose the simplest option that satisfies the numbered requirements and record it in docs/decisions.md. If something would contradict the design, stop and ask me.
- Every requirement gets at least one test named after its id that fails when the requirement is broken. The permissions test must cover every new route for every principal, including a service account.
- Do not weaken an existing test to make it pass. e2e/browser/tests/integration.spec.ts must keep passing without edits to its ledger call.
- The core stays generic: tools/check_core_generic.sh must pass.
- Follow the design's suggested task order. Run an independent verification task after milestones 3, 5 and 7, each by an agent that did not write the code and is given the code and the requirements, not the author's conclusions. Fix what it finds before moving on.
- ./ci.sh must pass before each milestone is committed. Commit per milestone with the requirement ids in the message. Only the orchestrator commits. Never rewrite pushed history.
- Before the release step, e2e/run.sh both and the integration scenario must pass.

## Release (owner's words)

- Do everything in the design's Release section up to the point of publishing, then stop and give me a report: what was built, every requirement with its test, every decision recorded, what the released binary reports as its version, and anything not done.
- Do not merge to main, push, or create or push a tag until I reply with the go-ahead and the version number. The tag publishes images that customers pull as latest.

## Done means (owner's words)

Every requirement has a passing test named after its id, ./ci.sh passes, the end-to-end runs pass, docs/deploying.md has the upgrade notes and the "Pre-approving SSO users" section with commands that were actually run, docs/progress.md and README.md are current, and after my go-ahead the release is merged, tagged and the image workflow has finished successfully.

## For task agents

- Task agents never commit, push, tag, stash, reset or switch branches. Leave
  changes in the working tree; the orchestrator commits.
- Task agents do not edit `docs/decisions.md`; they list decisions in
  RESULT.md under "Decisions for docs/decisions.md" and the orchestrator
  records them.
- `tools/check_core_generic.sh` bans some words under `internal/` (tests
  included): avoid `size=` in label examples and the name of the test identity
  provider.
