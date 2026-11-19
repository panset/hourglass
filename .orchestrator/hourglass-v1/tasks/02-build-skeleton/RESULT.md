# Result: 02 build skeleton (done)

Pinned: rules_go 0.50.1, gazelle 0.39.1, Go SDK 1.24.1 (go.mod `go 1.24`), Bazel 7.4.1 (.bazelversion).
`bazel test //...` executed 1 of 1 tests (//internal/version:version_test), passed.
`./ci.sh` exits 0 (log: ci.log in this folder); second run also 0.

Files: go.mod, .bazelversion, .bazelrc, MODULE.bazel, MODULE.bazel.lock, BUILD.bazel, ci.sh, tools/check_spdx.sh,
cmd/hourglassd/{main.go,BUILD.bazel}, internal/version/{version.go,version_test.go,BUILD.bazel}.

## Decisions for docs/decisions.md
- Bazel SDK is 1.24.1 while local Go is 1.27.4; go.mod says `go 1.24` so both toolchains build it.
- Gazelle-generated BUILD.bazel files get a hand-added SPDX header line; gazelle preserves it (ci diff check passes).
- check_spdx.sh checks git ls-files plus untracked non-ignored files, so it works before the first commit; excludes docs/, .orchestrator/, *.lock, go.sum.
- ci.sh runs gazelle via `bazel run //:gazelle -- update -mode=diff`.
- .bazelrc imports optional .bazelrc.user (already gitignored). .gitignore untouched.
- go_deps extension wired to go.mod (no deps yet) so later milestones just add requires.

## Confidence and gaps
Verified by running ci.sh. Not checked: clean-checkout run (bazel cache was warm for the second run), Linux.
