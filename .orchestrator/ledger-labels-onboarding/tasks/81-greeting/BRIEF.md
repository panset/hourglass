# 81 Greeting: name claims, `display_name`, top bar

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; "the
spec" means `docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/81-greeting/`.

## Goal

The greeting half of milestone 6: section C of the design (C1 server, C2 UI),
with tests named after AUTH-9 and UI-10 that fail when the requirement is
broken.

## Context

Read the design doc sections "What the code does today" (Name in the top bar),
C1, C2, Compatibility, Requirements (AUTH-9, UI-10), and the note under
"Changes by area" about words banned by `tools/check_core_generic.sh` (the name
of the test identity provider must not appear under `internal/`). Then
`docs/ui-notes.md` and CONTRIBUTING.md (generated UI types are regenerated with
the documented command, never hand-edited).

## What to build

- C1: the three extra ID token claims and the stored-name rule (first non-empty
  of `name`; `given_name` + space + `family_name`, trimmed; `preferred_username`;
  else empty as today). No scope added. `display_name` on
  `GET /api/v1/session`: name, else username, else email, by ONE shared
  function used by both the session and `userDisplayName` in the API (mind the
  import direction between `internal/auth` and `internal/api`; put the function
  where both can use it without a cycle). `name` keeps its meaning.
- The two AUTH-5 tests (`TestAUTH5_SessionOfUnassignedUser`,
  `TestAUTH5_SessionOfUserWithNoRoleIsOwnIdentityOnly`): add `display_name` to
  their field lists and change nothing else in them. Update the `SessionView`
  comment and the `Session` schema description, `api/openapi.yaml`, the
  contract test and the generated types.
- C2: "Hello, {display_name}" in `ui/src/shell/TopBar.tsx` for SSO users and
  the local admin; ellipsis at a fixed maximum width with the full text in
  `title`; no greeting at all when `display_name` is empty.
- Tests named after AUTH-9: each step of the claim fallback (including
  whitespace-only claims, only a given name, only a family name); a user whose
  stored name is empty gets their email as `display_name` with no new sign-in;
  the session and the rest of the API use the same function (a test that fails
  if the two drift); a no-role user's session holds `display_name` and nothing
  else new. Tests named after UI-10: greeting for an SSO user and for the local
  admin, no greeting when empty, `title` carries the full name.
- Existing behaviour that must not change: what is stored when the provider
  does send `name`; every other session field.

## Scope

May write: `internal/auth/`, `internal/api/` (helpers, session-related tests,
contract test), a small shared package under `internal/` if needed to avoid an
import cycle (name it in RESULT.md), `api/openapi.yaml`,
`ui/src/api/schema.gen.ts` (generated), `ui/src/shell/`, UI test fixtures that
must gain the field to type-check (list them), `docs/ui-notes.md`,
`BUILD.bazel` files Gazelle updates, your task folder.
Must not touch: `docs/decisions.md`, the design doc, `e2e/`, the ledger and
setups code, git state. Do not weaken any existing test.

## Done when

`go test ./internal/...`, UI tests and type-check pass; `./ci.sh` exits 0
(record `ci.exit`); one deliberate break each for AUTH-9 and UI-10 went red and
was restored; RESULT.md has the requirement -> test -> file table.

## Long-running steps

`./ci.sh`: allow 10 minutes (timeout 600000), output to `ci.log` in your task
folder, read the exit code and tail only.

## Report

Follow CONTRACT.md: PROGRESS.md, RESULT.md with "Decisions for
docs/decisions.md", one final reply of at most ten lines.
