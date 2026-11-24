# 86b Run and correct the commands in "Pre-approving SSO users"

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; "the
spec" means `docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/86b-guide-commands/`.

## Goal

Every command in the "Pre-approving SSO users" section of `docs/deploying.md`
has been run against the bundled stack built from the current tree, corrected
until it works as written, and the `<!-- 86b: run and correct -->` markers are
gone. Done means the guide's commands were actually run, with evidence.

## Context

- Design section D6 says what the section must contain; D76 and D77 in
  `docs/decisions.md` say what the server does.
- The three marked blocks are near lines 255, 280 and 294 of
  `docs/deploying.md`.
- `docs/deploying.md` and `deploy/README.md` say how to start the bundled
  stack locally from the product images (`deploy/smoke.sh bundled` shows the
  sequence). The session cookie is `Secure`: use `localhost` or HTTPS as the
  guide's "Cookies" section explains. The local admin must change its
  first-run password before it can call admin routes.
- The owner's `hourglass-tls` compose project is stopped on purpose so its
  ports are free: do not start, remove or touch it, nor the `platform-lab`
  containers. Use a compose project of your own and tear it down (containers
  and volumes of YOUR project only) when finished. A host process may be
  listening on localhost:8080; make sure your requests reach your stack (check
  what answers, for example by the version or a value only your stack has) and
  say how you made sure.

## What to do

1. Build the images from the current tree and start the bundled stack.
2. Scripted way: run the three calls exactly as the guide gives them (cookie
   jar, `POST /auth/local`, `GET /api/v1/session` for `csrf_token`,
   `POST /api/v1/admin/preapprovals` with `X-CSRF-Token`), correct the guide
   until copy-paste works, then confirm with `GET /api/v1/admin/preapprovals`.
   Use example addresses under `example.com` only; never a real address or a
   real password in the guide (follow how the guide writes passwords
   elsewhere).
3. File way: write the YAML as the guide gives it, mount it, set the key (and
   separately the environment variable), restart, confirm the entries appear
   with source `config`; confirm an invalid file stops start-up with the
   message the guide describes.
4. Admin-page way: confirm the steps match the page (read the UI code or drive
   the page; no screenshot needed).
5. End to end once: sign in as the development realm user `dave` (reserved for
   pre-approval; see `deploy/README.md`) after pre-approving that user's
   email, and confirm through the API that the user is active with the role.
   If driving the browser sign-in is impractical from the shell, say so and
   point at the end-to-end step that covers it (A8p in
   `e2e/browser/tests/acceptance.spec.ts`).
6. Update `docs/progress.md`: it says the new end-to-end steps had not run in
   a browser; they now have (`e2e/run.sh both`: 16 of 16 bundled, 16 of 16
   split; integration: 11 of 11, on 2026-11-23).
7. If a command fails because the product is wrong, not the guide, stop and
   report it; do not change product code.

## Scope

May write: `docs/deploying.md` (that section, and other places only where a
command you ran proves the text wrong: list each), `docs/progress.md`, your
task folder (keep a transcript of the commands and their trimmed output in
`commands.log`, with passwords and cookies replaced by placeholders).
Must not touch: product code, `e2e/`, `docs/decisions.md`, the design doc, git
state, containers you did not start.

## Done when

No `86b:` marker remains in `docs/deploying.md`; RESULT.md lists each command
with "ran, worked as written" or what was corrected; your stack is torn down;
`./ci.sh` exits 0 (record `ci.exit`).

## Long-running steps

Building the images and starting the stack: a few minutes to 15 (see
`deploy/smoke.sh`); `./ci.sh`: about 5 minutes. Run each once with an adequate
timeout or one blocking wait; read exit codes and tails only.

## Report

Follow CONTRACT.md. One final reply of at most ten lines.
