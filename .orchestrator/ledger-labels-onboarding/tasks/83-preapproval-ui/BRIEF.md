# 83 Admin > Users: the "Pre-approved users" section

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; "the
spec" means `docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/83-preapproval-ui/`.

## Goal

Section D5 of the design (UI-11), with tests named after UI-11 that fail when
the requirement is broken.

## Context

- Read the design doc sections D3, D5 and Requirements (UI-11), then
  `docs/ui-notes.md` and CONTRIBUTING.md.
- The server is built and committed. The three routes' request and response
  shapes are described in
  `.orchestrator/ledger-labels-onboarding/tasks/82-preapproval-server/RESULT.md`
  and in `api/openapi.yaml`; the generated types in `ui/src/api/schema.gen.ts`
  are current. D76 in `docs/decisions.md` records the choices (for example
  `created_by` and `used_by` are `{id, name}` objects or null, and the 400
  names the first bad email).
- The Users page is `ui/src/pages/admin-users/`. Follow how the other admin
  pages do tables, confirmations, forms and server errors.

## What to build

Every bullet of D5: the table (Email, Roles, Source shown as "Added here" or
"Config file", Added by, Signed in as the date or "Not yet"), Remove with a
confirmation on database rows only, the Add form (text area, role choice
defaulting to developer, Add button; split on commas, whitespace and new
lines; the server validates; a 400 shows the server's message next to the text
area and keeps the text), and the one-line explanation above the table. The UI
formats fields and computes none (UI-1). After a successful add or remove the
list is refetched; after a successful add the text area is cleared.

Tests named after UI-11: the table's columns and both sources; no Remove on a
file row; Remove asks for confirmation and calls DELETE with the id; the split
of pasted text (commas, spaces, new lines, mixed, empty pieces dropped) and the
POST body; the default role; a 400 keeps the text and shows the message; an
email in both sources appears as two rows; "Not yet" versus a date; the empty
list.

## Scope

May write: `ui/src/pages/admin-users/`, shared UI test helpers or fixtures if
needed (list them), `docs/ui-notes.md`, your task folder.
Must not touch: `internal/`, `api/`, `ui/src/api/schema.gen.ts`, `e2e/`,
`docs/decisions.md`, git state. Do not weaken any existing test.

## Done when

UI tests and type-check pass; `./ci.sh` exits 0 (record `ci.exit`); one
deliberate break made a UI-11 test go red and was restored; RESULT.md has the
requirement -> test -> file table.

## Long-running steps

`./ci.sh`: allow 10 minutes (timeout 600000), output to `ci.log` in your task
folder, read the exit code and tail only.

## Report

Follow CONTRACT.md: PROGRESS.md, RESULT.md with "Decisions for
docs/decisions.md", one final reply of at most ten lines.
