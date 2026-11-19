# 03 verify build skeleton: RESULT

Status: **done** (all steps pass). Clone used: `/private/tmp/claude-501/-Users-tejaspanse-projects/17dec24c-e7ec-455e-9b87-01d2217d5fc9/scratchpad/verify-03/hourglass`. Original repo not modified.

## Steps
1. PASS. Fresh `git clone` of the repo into the scratchpad verify-03 directory (clone exit 0).
2. PASS.
   - `go build ./...` exit 0 (log in scratchpad verify-03 go-build.log).
   - `go vet ./...` exit 0 (go-vet.log).
   - `bazel build //...` exit 0, 83 actions, build completed successfully (bazel-build.log).
   - `bazel test //... --test_output=errors` exit 0 (bazel-test.log).
   - `./ci.sh` exit 0 (ci.log). Includes SPDX check, go build, go vet, gazelle diff, bazel build, bazel test.
3. PASS. 1 Bazel test target (`//internal/version:version_test`), 1 executed, 1 passed on the fresh run. The ci.sh run hit the cache (0 executed, 1 passed).
4. PASS. Changed `Version == ""` to `Version != ""` in `internal/version/version_test.go`. `bazel test //...` exit 3, `version_test FAILED`. `git checkout --` restored the file; re-run exit 0, `PASSED`.
5. PASS. Added `internal/version/noheader.go` (no SPDX line) and `git add`ed it. `./tools/check_spdx.sh` exit 1 with `missing SPDX header: internal/version/noheader.go`. Removed it from the index and disk; script exit 0 afterwards.
6. PASS.
   - No WORKSPACE file (only MODULE.bazel).
   - `.bazelversion` is `7.4.1`.
   - `MODULE.bazel.lock` is tracked (`git ls-files`).
   - `git status --short` is empty after `./ci.sh`.
7. Consistent. The Bazel-downloaded Go SDK is go1.24.1 (MODULE.bazel `go_sdk.download(version = "1.24.1")`, confirmed in the output base VERSION file). go.mod says `go 1.24`. The local `go version` is go1.27.4 (darwin/arm64). Local go is newer than the module's `go` directive but builds fine; Bazel builds with its own hermetic 1.24.1 SDK.

## Files changed
None in the original repo. Clone only, with changes reverted.

## Confidence and gaps
- The Bazel test ran with `--test_output=errors`, which prints only failures. The PASS line came from the summary, not verbose output.
- Step 4 used the test-expectation flip as the break. Shell is zsh, and `timeout` is not available, so no timeout wrapper was used. All commands completed well under 10 minutes.
