# Intent: reported weights

Date: 2026-11-23. Branch: `reported-weights`, cut from `main` at `753cc50` (release `v1.1.0`).

## Goal (the owner's words)

Implement, test and release the design in `docs/design/reported-weights.md` in this repository.

## Sources of truth

- `docs/design/reported-weights.md` is the contract for this job. Its Requirements section (WGT-1 to WGT-23, UI-12 to UI-14) is the definition of done. Its "Owner decisions" are made; do not reopen them.
- Also: `docs/decisions.md`, `CONTRIBUTING.md`, `docs/deploying.md`, `docs/integrating.md`, `docs/extending.md`.
- Task-agent contract: `.orchestrator/reported-weights/CONTRACT.md` (the hourglass-v1 contract; "the spec" reads "the design doc").

## Must stay true (the owner's rules)

- Customers run v1.1.0 today. An upgraded installation must work with no configuration change, and existing API calls must answer as before. Treat the design's Compatibility section as requirements.
- Never edit `docs/SPEC.md` or the design doc. Record this work in `docs/decisions.md` from D80 on as an owner-directed change, and list there the edits the spec's source needs.
- Where the design leaves a detail open, choose the simplest option that satisfies the numbered requirements and record it in `docs/decisions.md`. If something would contradict the design, stop and ask the owner.
- Every requirement gets at least one test named after its id that fails when the requirement is broken. The permissions test must cover the new route for every principal, including a service account that holds every other scope.
- Record the v1.1.0 compatibility fixtures before any product code changes, the way D78 did for `d96a950`. Keep the existing `d96a950` fixtures and their tests passing unchanged.
- Do not weaken an existing test to make it pass. If an existing test fails because of one of the two deliberate behaviour changes in section A3, change only that assertion and record it in `docs/decisions.md`.
- The design names two fallback cases in section A2 (a hold metered before its setup is reported, and the last interval of a release after the setup left the report). Implement and test them as written. Do not add logic that remembers a removed weight.
- The core stays generic: `tools/check_core_generic.sh` must pass. Nothing in the product, its tests, its fixtures or its shipped docs may name the first client, its attribute keys or its backend; those appear only in section E of the design. Use neutral names (`lab-03`, `memory-gb`, `tier=gold`).
- Follow the design's suggested task order. Two tasks must never edit `api/openapi.yaml` or the generated UI types at the same time.
- Independent verification after milestones 3, 5 and 7, each by an agent that did not write the code and is given the code and the requirements, not the author's conclusions. Fix what it finds before moving on.
- `./ci.sh` must pass before each milestone is committed. Commit per milestone with the requirement ids in the message. Only the orchestrator commits. Task agents never commit, push, tag, stash, reset or switch branches, and do not edit `docs/decisions.md`; they list their decisions in their result for the orchestrator to record. Never rewrite pushed history.
- The working tree holds untracked `.orchestrator` log files from earlier jobs. Leave them alone; never use a broad `git add`.
- Before the release step, `e2e/run.sh both` and `e2e/run.sh integration` must pass.

## Release

- Do everything in the design's Release section up to the point of publishing, then stop and report to the owner: what was built, every requirement with its test, every decision recorded, what the released binary reports as its version, and anything not done or not verified.
- Do not merge to main, push, or create or push a tag until the owner replies with the go-ahead and the version number. The tag publishes images that customers pull as latest.

## Done means

Every requirement has a passing test named after its id; the first client's eight acceptance scenarios in the design's Tests section each map to a passing test or end-to-end step; `./ci.sh` passes; the end-to-end runs pass; `docs/integrating.md` has the "Report weights" job with examples that were actually run against the server; `docs/deploying.md` has the upgrade notes, the setting and the four fields whose meaning shifts; `docs/extending.md`, `docs/progress.md` and `README.md` are current; and after the owner's go-ahead the release is merged, tagged and the image workflow has finished successfully.
