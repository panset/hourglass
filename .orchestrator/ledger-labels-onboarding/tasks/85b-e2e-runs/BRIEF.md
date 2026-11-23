# 85b Run the end-to-end suites and make them pass

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; "the
spec" means `docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/85b-e2e-runs/`.

## Goal

`e2e/run.sh both` exits 0 and `e2e/run.sh integration` exits 0 on the current
tree, with the new end-to-end steps in place and every pre-existing assertion
as strong as it was.

## Context

- Task 85 wrote the new steps (A2g, A2f, A2p, A7g, A8p, A8c in
  `e2e/browser/tests/acceptance.spec.ts`) and changed nothing else there; an
  earlier task changed `readLedger` to use the Every poll view and walk pages
  with Older. NONE of this has run in a browser. Read
  `.orchestrator/ledger-labels-onboarding/tasks/85-e2e-compat/RESULT.md`, the
  design doc's Tests section (end-to-end bullets) and `e2e/README.md`.
- The owner's `hourglass-tls` compose project has been stopped by the
  orchestrator with the owner's consent so ports 8081 and 8990 are free. Do
  not start, remove or otherwise touch that project (`docker compose -p
  hourglass-tls ...` is off limits); the orchestrator restarts it afterwards.
  Do not touch the `platform-lab` containers either.
- Another agent is editing documentation files (`docs/`, `README.md`,
  `CONTRIBUTING.md`, `deploy/README.md`) at the same time. Do not edit those.

## What to do

1. Run one mode first (`e2e/run.sh bundled`), debug failures, then `split`,
   then `integration`, then a final clean `e2e/run.sh both` and
   `e2e/run.sh integration` on the final tree.
2. For each failure decide: a defect in a test step (fix the step, keeping
   what it asserts), or a defect in the product (make the minimal fix under
   `internal/` or `ui/src/`, add or extend a unit test named after the
   requirement id that would have caught it, and list it prominently in
   RESULT.md). Never weaken or delete a pre-existing assertion; never edit
   `integration.spec.ts`'s ledger call (`debitsOf` and the request it makes).
   If an existing assertion can only pass by being weakened, stop and report:
   that may be a contradiction with the design.
3. If product code changed, `./ci.sh` must exit 0 afterwards.

## Scope

May write: `e2e/` (within the limits above), `deploy/keycloak/`, deploy files
the end-to-end stack needs (list them), minimal product fixes with unit tests,
`BUILD.bazel` files Gazelle updates, your task folder.
Must not touch: `docs/`, `README.md`, `CONTRIBUTING.md`, the design doc, git
state, the `hourglass-tls` and `platform-lab` containers.

## Done when

The final `e2e/run.sh both` and `e2e/run.sh integration` exit 0 on the final
tree; exit codes are recorded in `e2e-both.exit` and `e2e-integration.exit` in
your task folder; RESULT.md gives the number of tests passed per run, every
change made and why, and `./ci.sh`'s exit code (`ci.exit`) on the final tree.
The end-to-end stacks the runs started are torn down.

## Long-running steps

`e2e/run.sh <mode>`: 12 to 25 minutes per mode; `both` up to 50. Each exceeds
one command's 10-minute limit: run it in the background writing to a log and a
done-marker file with the exit code, and block with a single wait loop per
wait (at most three waits per run). Read only exit codes and tails, plus the
failing test's error. `./ci.sh`: about 5 minutes.

## Report

Follow CONTRACT.md: PROGRESS.md, RESULT.md with "Decisions for
docs/decisions.md", one final reply of at most ten lines.
