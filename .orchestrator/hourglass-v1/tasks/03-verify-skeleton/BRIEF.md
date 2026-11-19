# 03 verify build skeleton
Rules: /Users/tejaspanse/projects/hourglass/.orchestrator/hourglass-v1/CONTRACT.md (reporting and slow-command sections apply; you write no product code).
## Goal
Independently confirm that the build skeleton works from a clean checkout. You did not build it; do not trust its RESULT.md.
## Steps
1. `git clone /Users/tejaspanse/projects/hourglass /private/tmp/claude-501/-Users-tejaspanse-projects/17dec24c-e7ec-455e-9b87-01d2217d5fc9/scratchpad/verify-03/hourglass` (delete that verify-03 directory first if it exists). Work only in the clone.
2. In the clone run, each with output to a log and reading exit code + tail: `go build ./...`; `go vet ./...`; `bazel build //...`; `bazel test //... --test_output=errors`; then `./ci.sh`.
3. Report how many Bazel test targets executed and passed (must be at least 1; zero tests is a failure).
4. Break check: in the clone, change the expectation in internal/version's test so it must fail, confirm `bazel test //...` goes red, then `git checkout` the file and confirm green again.
5. SPDX check: add a Go file without the SPDX header in the clone (`git add` it, do not commit), confirm `./tools/check_spdx.sh` fails naming it, then remove it.
6. Confirm there is no WORKSPACE file, `.bazelversion` says 7.4.1, MODULE.bazel.lock is committed, and `git status` in the clone is clean after a full `./ci.sh` (the build must not modify tracked files).
7. Note the Go SDK version Bazel uses versus the `go` directive in go.mod and the local `go version`, and whether they are consistent.
## Scope
May write: only the clone and this task folder (/Users/tejaspanse/projects/hourglass/.orchestrator/hourglass-v1/tasks/03-verify-skeleton). Do not modify the original repo.
## Long-running steps
First Bazel build in a new workspace path may take up to 10 minutes.
## Done when
RESULT.md has a pass/fail line per step with evidence. Overall status `done` only if every step passes; otherwise `partial` with what failed.
