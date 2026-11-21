# 71b Close the low findings of the final verification

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Ground rules, Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/71b-final-fixes/`.

## Goal
Four small corrections, from `.orchestrator/integration-api/tasks/71-final-verify/RESULT.md`
(read it). No behaviour change to the product.

1. **CMP-2 has no test named after it.** Add `TestCMP2_...`: a Go test, outside the core
   packages (for example in `tools/` or next to `ci.sh`, whichever fits Bazel and Gazelle with
   least fuss; it must run under both `go test ./...` and `bazel test //...`), that reads
   `ci.sh` and fails if any step CMP-2 names is missing: the SPDX check, the genericity check,
   the license check, the Gazelle check, `bazel build //...`, `bazel test //...`. Prove it
   bites by deleting one step from `ci.sh`, seeing it fail, and restoring.
2. **`docs/decisions.md` rows D51 to D53** have the Why and By columns swapped and D53 has no
   Why. Fix the columns; write D53's Why from what the row decides (read the code it
   describes if needed). Check every row from D42 to the end has five cells in the header's
   order and "agent" or "owner" in By.
3. **D60 residual cases.** Do not change the code. Amend row D60 so it states plainly, and is
   marked **REVIEW**: (a) `hold.releasing` is queued inside the transaction, so if the commit
   then fails the event is delivered for a release request that was not stored, and the
   user's retry delivers it again; (b) the webhook can reach the receiver before a state read
   shows `release_requested_at`; (c) if looking up the hold's owner for the event fails, the
   event is not sent although the request is stored. Verify each of the three against
   `internal/holds/holds.go` before writing it; write only what the code does. Add one
   sentence each on why the guide's advice (idempotent handlers keyed on hold id, plus
   reconciliation from the state read) covers it. Add D60 to the "Decisions to review" table
   in `docs/progress.md`.
4. **README.md**: the design asks for "one paragraph and a link under Documentation". There is
   only the link line. Add one short paragraph in the README's existing voice saying what the
   integration API is for (a customer's own service reports deployments and pressure and
   receives webhooks; no Go required), pointing at `docs/integrating.md`.

Leave `docs/wireframes/` alone (saved artboards of the owner's source).

## Scope
May write: the new test file and its `BUILD.bazel` (via `bazel run //:gazelle`),
`docs/decisions.md`, `docs/progress.md`, `README.md`, your task folder.
Must not touch: product code, `ci.sh` (except the temporary break, restored), `docs/SPEC.md`,
`docs/design/`, git state.

## Done when
`./ci.sh` passes with the new test executed (say how many Bazel tests ran).

## Long-running steps
`./ci.sh`: 5 to 15 minutes, once, foreground, 30-minute timeout, log in your task folder.

## Report
Follow the Reporting section of the contract.

## Resume note
An earlier agent on this task was stopped partway. The working tree holds its partial,
unreviewed edits: `tools/ci_test.go`, `tools/BUILD.bazel`, a change to the root `BUILD.bazel`
(`git diff BUILD.bazel`), and possibly a PROGRESS.md here. Inspect them first; keep, fix or
redo them as needed (check `ci.sh` is exactly as committed: `git diff ci.sh` must be empty).
Items 2, 3 and 4 had not been started. Then do everything in this brief.
