# 82 Pre-approved SSO users: store, sign-in step, admin routes, file overlay

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; "the
spec" means `docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/82-preapproval-server/`.

## Goal

The server half of milestone 7: sections D1 to D4 of the design, with tests
named after PRE-1 to PRE-8 that fail when the requirement is broken. The admin
page (D5, UI-11) is the next task; you deliver the API, OpenAPI and generated
types it needs.

This is sign-in and authorisation code for a product customers run: a mistake
here grants access. Read every rule in D1 as a requirement and test each one.

## Context

- Read the design doc sections "What the code does today" (SSO sign-in and
  approval), Non-goals, D1, D2, D3, D4, Compatibility (Sign-in, Hooks,
  Configuration), Requirements (PRE-1 to PRE-8) and the note under "Changes by
  area" about `tools/check_core_generic.sh` (the name of the test identity
  provider must not appear under `internal/`).
- Decisions so far: D69 to D75 in `docs/decisions.md`. D70(a): migration `0006`
  holds the ledger change only, so the `preapprovals` table goes in a new
  migration `0007`. Existing store tests pin the schema version and the table
  list; extend them, do not loosen them.
- `internal/auth` must not import the ledger service: it gets the approval step
  through an interface supplied by `internal/app` (D1, last paragraph).
- Service accounts exist as a separate principal (see
  `internal/api/permissions_test.go`); the permissions test must cover each of
  the three new routes for every principal it knows, including a service
  account, an anonymous caller, a no-role user and a developer.
- CONTRIBUTING.md: generated UI types are regenerated with the documented
  command, never hand-edited.

## What to build

1. Migration `0007` with the `preapprovals` table exactly as D2 gives it, and
   store queries.
2. The sign-in step (D1): the four conditions, the transaction that re-checks
   status and does what `setUserRoles` does (share code with it through a
   transaction-scoped helper if that keeps the two from drifting), the
   `user.preapprove` audit entry (no actor, `before`/`after` in `user.roles`'
   shape, matched sources), first-use marking of an unused database entry,
   `user.approved` after commit, no `user.pending` for a new pre-approved user,
   the first session showing the roles, and failure handling (sign-in succeeds,
   user stays pending, error logged). `email_verified` as a boolean or the
   strings "true"/"false"; absent counts as verified.
3. The three admin routes (D3) with validation, all-or-nothing add, upsert that
   keeps the use record, counts in the answer, audit entries, the route table,
   `api/openapi.yaml`, contract test, generated types.
4. The file overlay (D4): config key and environment variable, loader (JSON by
   extension, else YAML, strict), limits, start-up failure naming the entry,
   redacted config view includes the path, entry count logged (never the
   addresses), never written, never copied to the database;
   `deploy/config.sample.yaml` gets a commented example of the key.
5. Tests named `TestPRE1_` to `TestPRE8_`, covering every rule in D1 and every
   clause of the requirement text, including: local admin never matched; a
   rejected user; an active user with no roles; a pending user from an earlier
   sign-in; a second SSO identity with the same email; deleting an entry
   changes no user; case and whitespace in both the claim and the entry;
   `email_verified` false as boolean and as string, true, absent; empty email;
   an email claim that would match only via another claim (must not match);
   union of roles from both sources and the audit entry's sources; a file entry
   alone; a pre-approval granting `admin`; 1,000 and 1,001 emails; one invalid
   email among valid ones stores nothing; every invalid-email rule; duplicate
   emails within one request (choose and record); an invalid file of each kind
   stops start-up with the entry named; the file is not modified; with no
   entries and no file a new SSO user is pending and gets `user.pending`
   exactly as before; a forced failure of the approval transaction (PRE-8).

## Scope

May write: `internal/store/` (migration `0007`, queries, tests),
`internal/auth/`, `internal/api/`, `internal/config/`, `internal/app/`,
`cmd/` only if wiring requires it (list it), `api/openapi.yaml`,
`ui/src/api/schema.gen.ts` (generated), UI test fixtures that must change to
type-check (list them), `deploy/config.sample.yaml`, `BUILD.bazel` files
Gazelle updates, your task folder.
Must not touch: `docs/` (including `docs/decisions.md`), the design doc,
`e2e/`, `deploy/keycloak/`, the admin Users page (next task), the ledger
compaction code, git state. Do not weaken any existing test. If a rule in D1
cannot be met without contradicting another part of the design, stop and
report.

## Done when

`go test ./internal/...` passes; `go test -race ./internal/auth/ ./internal/api/`
passes; `tools/check_core_generic.sh` passes; `./ci.sh` exits 0 (record
`ci.exit`); one deliberate break each for PRE-2, PRE-3, PRE-5 (permissions) and
PRE-8 went red and was restored; RESULT.md has the requirement -> test -> file
table and describes the request and response shapes of the three routes in
words for the UI task.

## Long-running steps

`./ci.sh`: allow 10 minutes (timeout 600000), output to `ci.log` in your task
folder, read the exit code and tail only.

## Report

Follow CONTRACT.md: PROGRESS.md, RESULT.md with "Decisions for
docs/decisions.md", one final reply of at most ten lines.
