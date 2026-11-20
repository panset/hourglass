# 70 Documentation: make the guide true, update the rest

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Ground rules, Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/70-docs/`.

## Goal
`docs/integrating.md` is verified against the running server and its draft banner is gone;
`docs/extending.md`, `docs/deploying.md`, `README.md` and `docs/progress.md` describe the
integration API as built.

## Context
- Design: `docs/design/integration-api.md`, "Documentation deliverables" (items 1, 3, 4, 5).
- `docs/decisions.md` D42 onward: everything decided during the work. The docs must agree with
  it (for example D47 on rate limiting, D43 on release, D51-D55 on config and wiring, D58 on
  what is charged in the poll where a setup disappears, D60, D61).
- How to bring up a stack with the `reported` extensions and get a service token:
  `.orchestrator/integration-api/tasks/68-e2e/RESULT.md`.
- Wiring description for extending.md: `tasks/64-reported-extensions/RESULT.md`.

## Part 1: `docs/integrating.md`
Bring the `reported` stack up and run **every** example in the guide against it, exactly as
written (with `$HOURGLASS` and `$HOURGLASS_TOKEN` set): both PUT reports, the state read, the
service release, each row of the Errors table that can be provoked (401, 403, 404, 409
`stale_report`, 409 `not_configured` on a stack configured without `reported`, 409 `conflict`,
400 `validation` for each named cause, 429), the config snippet (start a server with exactly
that YAML, adjusted only for the URL and secret), the webhook body (capture a real
`hold.acquired` delivery and compare every field and header, including
`X-Hourglass-Signature` checked with the guide's Python snippet and `X-Hourglass-Event`), and
the lifecycle steps 1 to 6 in order with real polls. Keep a table in RESULT.md: example,
command run, expected per the guide, actual, verdict.
Wherever the guide and the server differ, the server (and the design) wins: correct the guide.
If the *server* differs from the *design*, do not edit either: report it as a finding.
Also check each factual sentence that is not an example (poll cadence, default bands, 30
seconds, 200 bytes, `max_age`, 2-minute skew, at-most-once delivery, what admins see) against
the code or the existing docs, and fix what is wrong. Then remove the status banner. The guide
must stay self-contained: a reader with only that file can build a working integration. Keep
its structure and voice; change only what is untrue or missing.

## Part 2: the other documents
- `docs/extending.md`: `hold.releasing` in the events table and JSON form;
  `Hold.ReleaseRequestedAt`; `reported` rows in Bundled extensions; how an extension can read
  stored reports only if that is public API (`pkg/storedreports`); a pointer to the
  integration guide as the no-Go path; remove any text that still says a release is refused
  with `still_deployed`.
- `docs/deploying.md`: the `reported` config and `max_age` (and matching it to the poll
  interval); service account setup and token rotation; the rate-limit note for deployments
  behind a reverse proxy (D47); an entry under Upgrade for migration 0005 (no down migration;
  back up first, as the existing entries say) and the changed release behaviour (D43).
- `README.md`: one paragraph and a link under Documentation.
- `docs/progress.md`: a section for this work in the file's existing style: what is finished,
  what is not, and add every decision from D42 on that is marked **REVIEW** to the "Decisions
  to review" table, plus a line for the others worth a look.
- Anything else under `docs/`, `deploy/README.md` or `e2e/README.md` that still describes the
  old `still_deployed` refusal: fix it. Do not touch `docs/SPEC.md` or `docs/design/`.

## Scope
May write: `docs/ui-notes.md`, `docs/integrating.md`, `docs/extending.md`, `docs/deploying.md`, `docs/progress.md`,
`README.md`, `deploy/README.md`, `e2e/README.md`, your task folder. `docs/decisions.md` only to
append a row if you must make a documentation-level choice.
Must not touch: `docs/SPEC.md`, `docs/design/`, any code, git state.

## Done when
The RESULT.md table shows every guide example run with verdict "matches" after corrections;
the banner is removed; the four other documents are updated; `./ci.sh` passes; the stack you
started is torn down.

## Long-running steps
Bringing the stack up takes several minutes (image build); `./ci.sh` 5 to 15. Foreground,
40-minute timeout, log to your task folder, read only exit code and tail.

## Report
Follow the Reporting section of the contract. List server-versus-design differences, if any,
as open questions.
- `docs/ui-notes.md` still describes the old release refusal: update it (reported by task 67).
- Task 68 observed that users who sign in through OIDC have an empty `username` in event JSON
  (the e2e scenario compares the email instead). Check what `username` holds for OIDC and for
  local users in both the webhook body and the state response, and make the guide say what a
  reader will really see and which field to key on. If the state response omits a field the
  design's example shows (username, email, name), that is a server-versus-design finding.
- Stack helper: `e2e/integration-stack.sh up | token | down` (see task 68's RESULT.md).
