# Intent: integration API (push model)

Date: 2026-11-20. Owner: Tejas Panse. Base: `main` at `5808957`.

## Goal (owner's words)

"Implement the Hourglass integration API (push model) in this repository."

## Sources of truth

- `docs/design/integration-api.md`: the contract for this job. Its
  Requirements section (SVC-*, INT-*, HOLD-6 to HOLD-9, CMP-*) is the
  definition of done.
- `docs/integrating.md`: a draft customer guide written before the code.
  Making it true is part of the job.
- `docs/decisions.md` from D40 on, `CONTRIBUTING.md`, `docs/extending.md`.
- Task-agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` (reused). Where
  it says "the spec", read "the design doc" for this job; where it points at
  the v1 INTENT, read this file as well.

## Owner decisions, already made; do not reopen them

- Push model only: the customer's service calls Hourglass. No pull model.
- Service accounts are a separate principal with scopes. They never hold the
  admin or developer role, and their tokens work only under
  /api/v1/integration/.
- A service account with the holds:release scope may release holds.
- A user's release of a still-deployed setup returns 202, marks the hold as
  releasing, fires the new hold.releasing event, and completes at the first
  successful poll where the setup is no longer reported.
- GET /api/v1/integration/state includes the holder's username, email and name.
- Setups are identified by their own name everywhere (D40). Do not reintroduce
  a resource name, template or override.

## Rules (verbatim)

- Never edit docs/SPEC.md. Record this work in docs/decisions.md from D42 on as
  an owner-directed change, and list there the edits the spec's source needs.
- The core stays generic: tools/check_core_generic.sh must pass.
- Where the design leaves a detail open, choose the simplest option that
  satisfies the numbered requirements and record it in docs/decisions.md. If
  something would contradict the design, stop and ask me.
- Every requirement gets at least one test named after its id. The permissions
  test must cover every new route for every principal, including a service
  account with and without the scope.
- Follow the design's suggested task order. Run an independent verification
  task after service accounts, after the release changes, and after the
  end-to-end scenario, each by an agent that did not write the code and is
  given the code and requirements, not the author's conclusions.
- ./ci.sh must pass before each milestone is committed. Commit per milestone
  with the requirement ids in the message. Never rewrite pushed history. Do
  not push; I will review and push.
- The design doc and the guide have uncommitted edits in the working tree.
  Commit them as the first step, unchanged, before any code. (Done: 5808957.)

## Done means

- Every requirement in the design's Requirements section has at least one test
  named after its id that fails when the requirement is broken.
- `./ci.sh` passes.
- Every example in docs/integrating.md has been run against the running
  server, whatever differed is corrected, and its draft banner is removed.
- docs/extending.md, docs/deploying.md (including an Upgrade entry for the
  migration and the changed release behaviour), README.md and docs/progress.md
  are updated.
- A short report to the owner: what was built, which requirements are verified
  and how, every decision recorded, and anything that is not done.

## Conventions for every task

- Only the orchestrator commits. Nobody pushes. Subagents leave changes in the
  working tree and list the files they changed.
- Subagents never edit `docs/SPEC.md` or `docs/design/integration-api.md`. A
  conflict with the design is reported as an open question, not resolved.
- Decisions: D42 (the owner-directed change and the spec-source edits) and D43
  (pending release supersedes HOLD-2's refusal, REVIEW) are written by the
  orchestrator. A task agent that makes a choice the design leaves open
  appends a row to `docs/decisions.md` with the next free number (D44 on),
  dated 2026-11-20, "By" = agent, and lists the numbers in its RESULT.md.
- Test names cite the requirement id: `TestSVC1_...`, `TestINT3_...`,
  `TestHOLD6_...`, `TestCMP1_...`.
- SPDX header on every source file. `tools/check_core_generic.sh` must pass:
  nothing under `internal/`, `pkg/hourglass/`, `pkg/hourglasstest/` may import
  an extension package or use the banned terms (see CONTRIBUTING.md).
- Local toolchain: go 1.27.4, bazel 7.4.1, node 26, pnpm 9.15.9, docker 27
  with compose v2. darwin/arm64. `./ci.sh` takes roughly 5 to 15 minutes.
