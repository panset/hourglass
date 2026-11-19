# Intent: Hourglass v1

Date: 2026-11-19. Owner: Tejas Panse.

## Goal (owner's words)

"You are starting development of Hourglass, an open-source token-management
framework for shared lab setups." Build v1 exactly as `docs/SPEC.md` says.

## Sources of truth

- Functional spec: `docs/SPEC.md` (copy of https://claude.ai/artifact/AVPtQZjX1gQvSQBseDGGBc).
- Wireframes: https://claude.ai/artifact/PWCrdA3nDinVkmuAsGdK74 (Design canvas;
  artboards `project/{Login,Unauthorized,Main,Setups,AdminUsers,AdminSetups,AdminSettings}.dc.html`).
  No wireframe for the audit log; build that page from the spec's "Dashboard pages".

## What must stay true (owner's rules, verbatim)

- The spec is the contract. Build what it says, cite requirement ids
  (AUTH-1, HOLD-2, RULE-3, ...) in commit messages and test names, and do
  not add features it does not ask for.
- The core must stay generic. No core package may import or mention
  vCenter, sizes, or any concrete provider, pressure source, policy or
  hook. Everything deployment-specific lives in an extension package, the
  sample config or the seed file.
- For the spec's "Open questions", use the bracketed assumptions. Where
  the spec is silent, choose the simplest option and record it in
  docs/decisions.md. If something would contradict the spec, stop and ask.
- There is no real vCenter on this machine. All vSphere code is built and
  tested against mocks exactly as the spec's "Testing without a vCenter"
  section describes. Nothing in the build, tests or local stack may need
  a real one.
- First technical task after Step 0: a short spike confirming the govmomi
  simulator can create resource pools and what cluster figures it
  reports. Record the result in docs/decisions.md and adjust the test
  layers if needed, without weakening coverage of the vSphere logic.
- Never commit secrets, real credentials or proprietary setup data.
  Every source file starts with an SPDX MIT identifier.
- Work in small commits on main and push after every milestone. Never
  force-push or rewrite pushed history.
- Do not create a remote repository or choose its visibility.

## Done means (verbatim)

- From a clean checkout: "bazel build //..." and "bazel test //..." pass,
  and plain "go build ./..." produces an API-only binary.
- The spec's acceptance path passes end to end in bundled and split mode.
- HTTP tests cover every cell of the permissions table, including that a
  user with no role gets no data from any route.
- Everything is pushed, and docs/progress.md lists what is finished, what
  is not, and every decision you made that I should review.

Keep docs/progress.md current throughout.

## Conventions for every task

- Go files start with `// SPDX-License-Identifier: MIT`; other source
  files use the comment syntax of their language.
- Only the orchestrator commits and pushes. Subagents leave changes in the
  working tree and list the files they changed.
- Subagents never edit `docs/SPEC.md`. A conflict with the spec is reported
  as an open question, not resolved by the agent.
- Local toolchain: go 1.27.4, bazel 7.4.1 (no bazelisk), node 26.7, npm 11,
  docker 27 with compose v2. darwin/arm64.
