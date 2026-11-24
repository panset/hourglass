# 88 Release preparation: the version the released binary reports, and the owner report

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first;
its "Release" section says what the report must contain). Agent contract:
`.orchestrator/hourglass-v1/CONTRACT.md`. Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/88-release-report/`.

## Goal

Two outcomes, in this order:
A. Establish, by building and running it, what the released binary reports as
   its version.
B. Write `REPORT.md` in your task folder: the owner's report before release.

Nothing is published: no merge, no push, no tag, no image push, no `gh`
command that writes. Read-only `git` and `gh` queries are fine.

## A. Version

- Read the design doc's Release section, `docs/deploying.md` ("Publishing the
  images to a registry"), D65 and D67 in `docs/decisions.md`,
  `.github/workflows/release.yml`, `deploy/push-images.sh`, `deploy/images/`
  and `internal/version/`.
- Build the product binary the way the release images are built (the Bazel
  target the image uses; build the image itself and load it locally if that
  is practical, without pushing) and run it to print its version (the flag or
  subcommand it offers, and/or the version field of whatever route reports
  it). Record the exact commands and the exact output.
- Say whether anything in the image build sets the version at link time
  (x_defs, stamping, `--workspace_status_command`) and whether the release
  workflow passes the tag to it. If the binary reports `0.0.0-dev` (or any
  value unrelated to the tag), state that plainly. Do NOT add version
  stamping or change any build file.
- Also report, read-only: whether any `v*` tag exists locally or on `origin`
  (`git tag -l`, `git ls-remote --tags origin`); what `origin/main` is and
  whether the branch is a fast-forward from it
  (`git fetch origin` is allowed; `git merge-base --is-ancestor origin/main HEAD`);
  whether the release workflow has ever run (`gh run list --workflow release.yml`
  if `gh` is signed in; otherwise say it could not be checked) and anything in
  D67 the owner should know before the first run.

## B. REPORT.md

Sources (read these, cite paths): `PLAN.md` and `DECISIONS.md` in the job
folder; `tasks/87-final-verify/RESULT.md` and `REQUIREMENTS.md`;
`docs/decisions.md` D69 to D79; `git log --oneline d96a950..HEAD`; the
verification results of tasks 76, 79, 84.

Sections, in this order, written for the owner (plain sentences, the outcome
first, no process narrative):
1. Decision needed: the go-ahead and the version number; what happens on
   go-ahead (merge to main, push, tag, the image workflow moving `latest`);
   what the binary reports as its version and what that means for the tag.
2. What was built: one short paragraph per area (ledger roll-up and paging,
   label discovery, greeting, pre-approved SSO users), and the commit list.
3. Requirements: a table of every id (LED-7 to LED-15, UI-8 to UI-11, SET-8,
   AUTH-9, PRE-1 to PRE-8, UPG-1 to UPG-3) with its test name(s) and file,
   taken from REQUIREMENTS.md and spot-checked with grep against the tree.
4. Checks run and their results: `./ci.sh` on the final commit, the uncached
   test run, the end-to-end runs (which commit, counts, and that the final
   rerun was skipped at the owner's direction because only documentation
   changed afterwards), the three independent verifications and the final one
   with their finding counts.
5. Decisions recorded: D69 to D79, one line each, with the ones the owner
   should look at first (anything that affects who gets access or what a
   client sees).
6. Not done, or left as is. Include at least: the end-to-end harness risk (F1
   of task 87: `localhost` URLs can reach host processes on 127.0.0.1:8080 and
   8091 when the Docker listener is missing; the gate runs did reach the
   stacks); the UPG-2 fixture gap (F3); the seed loader's strictness gaps; the
   existing timing flake `TestNFR_LedgerAppendTimeIsLinear`; UI lint not run
   if that is still true (check task 87's result); `docs/SPEC.md` not edited
   (D79 lists the edits); the non-string `name` claim behaviour; the design's
   "Open points for the owner" with their defaults as built; version stamping
   if absent; anything else the sources mark as open. Do not soften these.
7. After the go-ahead: the exact steps the orchestrator will take, and how
   success is checked (the image workflow finishing successfully).

Keep it to what the owner needs; no pasted logs. Every factual claim comes
from a source file or a command you ran.

## Scope

May write: your task folder only. Bazel output directories are fine.
Must not touch: any tracked file, git state (no commit, tag, merge, push),
remote registries, containers you did not start (remove any you load or run).

## Long-running steps

Building the image targets with Bazel: up to 10 minutes; run once with an
adequate timeout, output to a log, read the tail.

## Report

Follow CONTRACT.md. One final reply of at most ten lines: the version string
the binary reports, whether the tag is wired to it, tag and workflow state,
and the path to REPORT.md.
