# 84 Independent verification of milestones 6 and 7 (labels, greeting, pre-approved SSO users)

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` ("the spec" means
`docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/84-verify-m6-m7/`.

## Goal

Decide, from the code and the requirements alone, whether SET-8, UI-9, AUTH-9,
UI-10, PRE-1 to PRE-8 and UI-11 are met and tested, with a security read of
the sign-in and pre-approval path; list every defect with a severity (high /
medium / low) and a concrete failure scenario.

You did not write this code. Do NOT read the authors' PROGRESS.md or RESULT.md
under `.orchestrator/ledger-labels-onboarding/tasks/80-*` to `83-*`.

## Inputs

- Requirements: the design doc, sections B, C, D (all of D1 to D5), Non-goals,
  Compatibility, Requirements. Every rule in D1 is a requirement.
- Code: `git diff 36a93b0..HEAD` (four commits: labels, greeting, pre-approval
  server, pre-approval UI). Decisions recorded: D74, D75, D76 in
  `docs/decisions.md`; judge whether any of them contradicts the design.

## What to check

1. Each requirement has at least one test named after its id that would fail
   if the requirement were broken. Make one deliberate break each for SET-8,
   UI-9, AUTH-9, UI-10, PRE-1, PRE-2, PRE-3, PRE-4, PRE-5, PRE-6, PRE-7, PRE-8
   and UI-11, confirm red, restore (`git diff` empty at the end; say so).
2. Security read of pre-approval. Try to find any way a person gets a role
   they should not: a path where the approval runs for a non-pending, rejected,
   active or local user; a race between two sign-ins or between a sign-in and
   an admin's rejection (the status re-check inside the transaction);
   `email_verified` handling for every JSON type; matching on anything other
   than the `email` claim; normalisation mismatches between stored entries,
   file entries and the claim (Unicode case folding, internal whitespace,
   trailing dots); the file loader's strictness and limits; an entry granting
   `admin`; what happens when the approval transaction fails halfway (roles set
   but allowance not, audit missing); whether the first session after approval
   really carries the roles, and whether a stale pending session elsewhere
   gains them correctly; events (`user.approved` once, no `user.pending` for a
   pre-approved new user, `user.pending` unchanged otherwise).
3. The three admin routes: admin role and CSRF required exactly like the other
   admin routes; every principal in the permissions test, including a service
   account, a developer, a no-role user and an anonymous caller. The author
   noted that the general permissions test takes its expectations from the
   route table: confirm a wrong access level in the table for one of the new
   routes is caught by some test (make that break). Validation is all or
   nothing; audit entries hold what D3 says; no email addresses are logged at
   start-up; the redacted config view shows the path.
4. `setUserRoles` was refactored to share `grantRolesTx`: confirm its behaviour
   and its existing tests are unchanged (compare with `git show 36a93b0:internal/api/handlers_admin.go`).
5. Compatibility: with no entries and no file, the SSO sign-in of a new user is
   as at `d96a950` (status, events, audit, session) apart from `display_name`;
   `GET /api/v1/setups` and `GET /api/v1/session` differ only by the added
   field; `api/openapi.yaml` removes or renames nothing
   (`git diff d96a950..HEAD -- api/openapi.yaml`); regenerated UI types match.
6. Labels (B1, B2) and greeting (C1, C2) against every bullet. D74(b) says a
   chip, when the box holds several equality terms for its key, replaces the
   first and drops the others: does that contradict B2? The AUTH-5 tests and
   the router test that pin the session field list: was anything loosened
   beyond adding `display_name`?
7. Existing tests edited in these four commits
   (`git diff 36a93b0..HEAD --stat -- '*_test.go' '*.test.tsx'`): is any
   assertion weaker than before?
8. `tools/check_core_generic.sh` passes; `./ci.sh` passes with tests actually
   executed (use an uncached run and report the count); run
   `go test -race ./internal/auth/ ./internal/api/ ./internal/store/`.

## Scope

May read: the whole repository except the four task folders named above.
May write: your task folder only. Temporary breaks must be restored.
Must not touch: git state beyond restoring your own breaks.

## Done when

RESULT.md starts with a verdict and a findings table (id, severity, file:line,
failure scenario, suggested fix), then the requirement -> test -> "break made,
went red" table, then the security read's conclusions.

## Long-running steps

`./ci.sh`: about 5 minutes, allow 10 (timeout 600000), output to `ci.log` in
your task folder; read exit codes and tails only.

## Report

Follow CONTRACT.md. One final reply of at most ten lines: verdict, counts of
high / medium / low findings, path to RESULT.md.
