# 86a Documentation (everything that needs no running stack)

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; "the
spec" means `docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/86a-docs/`.

## Goal

The documentation the design asks for, true to the code as built, except the
"Pre-approving SSO users" commands that must be run against the bundled stack
(a later task, 86b, runs them and fills them in).

## Context

- The design doc: "Changes by area" (the `docs` row), Compatibility (every
  bullet names something `docs/deploying.md` must say), D4, D6, Release.
- What was built is recorded in `docs/decisions.md` D69 to D78; read them.
  Where the design and a decision differ in detail, the decision says what the
  code does; check the code when in doubt (`path:line` in RESULT.md for every
  factual claim you add about behaviour).
- Follow the existing style of each document.

## What to write

1. `docs/deploying.md`:
   - "Upgrade": an entry for this release: migrations `0006` and `0007`; no
     configuration change needed; no down migration, an older binary refuses
     the database afterwards, compacted rows cannot be expanded, the way back
     is the backup taken before the upgrade; the first start may take longer
     on a large ledger while the migration builds its indexes; what the
     roll-up does after the first start and how it is paced.
   - "Data retention": the ledger is no longer "never pruned": debits older
     than 7 days are rolled up to one row per user, hold and day; per-poll
     detail stays in `usage_samples` for 13 months; what users see.
   - Disk: the file stops growing at the old rate; SQLite reuses freed pages
     but does not shrink the file; an offline `VACUUM` reclaims the space.
   - The existing note on timezone changes: days are cut in the timezone
     configured when compaction runs, and a compacted day is never cut again.
   - Configuration reference: `preapproved_users_file` /
     `HOURGLASS_PREAPPROVED_USERS_FILE`, with the file format, limits, strict
     decoding, read once at start.
   - A new "Pre-approving SSO users" section per D6: the three ways (admin
     page, file, scripted), and the three statements the guide "must also
     say". For the file and scripted ways, write the steps and put the
     commands in clearly marked blocks beginning with the line
     `<!-- 86b: run and correct -->` so the next task can find, run and
     correct them. Do not present any command as run.
2. `docs/ui-notes.md`: check the notes earlier tasks added (ledger paging,
   label chips, greeting, pre-approved users) are complete and consistent;
   fix gaps.
3. `docs/progress.md` and `README.md` (where it lists features): current.
4. `deploy/README.md`: it lists three development realm users; there is now a
   fourth, `dave`, reserved for the pre-approval end-to-end step.
5. `CONTRIBUTING.md`: mention the compaction scale test beside the existing
   scale test, and the `testdata/upgrade-d96a950/` fixtures and how they are
   reproduced (see D78 and the note inside that folder).
6. Do NOT edit `docs/SPEC.md`, the design doc or `docs/decisions.md`. In
   RESULT.md, under "Edits the spec's source needs", write the complete list
   the design asks for (LED-1; the ledger and users table rows and the new
   `preapprovals` and marker tables; the HTTP API table: new parameters,
   fields and the three routes; the Dashboard pages table; the config
   reference; new requirement ids LED-7 to LED-15, UI-8 to UI-11, SET-8,
   AUTH-9, PRE-1 to PRE-8, UPG-1 to UPG-3; AUTH-2 and AUTH-5 notes), each as
   one sentence saying what the spec text should become, written so the
   orchestrator can paste it into one table cell (no pipe characters, no line
   breaks inside an item).

## Scope

May write: `docs/deploying.md`, `docs/ui-notes.md`, `docs/progress.md`,
`README.md`, `deploy/README.md`, `CONTRIBUTING.md`, your task folder.
Must not touch: anything else; git state. Do not start or stop Docker
containers.

## Done when

The documents are updated; `./ci.sh` exits 0 (record `ci.exit`; docs can trip
its checks); RESULT.md lists each document section changed and the spec-edits
list.

## Long-running steps

`./ci.sh`: allow 10 minutes (timeout 600000), output to `ci.log` in your task
folder, read the exit code and tail only.

## Report

Follow CONTRACT.md. One final reply of at most ten lines.
