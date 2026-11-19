# 02 build skeleton
Job intent: .orchestrator/hourglass-v1/INTENT.md (read it first). Rules: .orchestrator/hourglass-v1/CONTRACT.md.
## Goal
An empty but shippable build skeleton for the repo at /Users/tejaspanse/projects/hourglass: `bazel build //...`, `bazel test //...` and plain `go build ./...` all pass.
## Requirements
- Go module `github.com/panset/hourglass` (go.mod). Local Go is 1.27.4.
- Bazel with bzlmod only (no WORKSPACE): `MODULE.bazel` with rules_go and Gazelle, `.bazelversion` = 7.4.1 (installed version; no bazelisk), `.bazelrc` with sensible defaults, root `BUILD.bazel` with the gazelle target (`bazel run //:gazelle`) and prefix directive. Commit `MODULE.bazel.lock`. Pick rules_go/gazelle versions that work with Bazel 7.4.1 and a Go SDK the rules can download; pin the SDK version in MODULE.bazel and keep go.mod's `go` directive compatible with it.
- `cmd/hourglassd/main.go`: a stub that prints a version string and exits 0; `internal/version` package with a `Version` variable and one trivial unit test, so `bazel test //...` runs at least one test.
- `tools/check_spdx.sh`: fails listing any tracked source file (`*.go`, `*.sh`, `*.bzl`, `BUILD.bazel`, `MODULE.bazel`, `*.ts`, `*.tsx`, `*.js`, `*.css`, `*.sql`, `*.yaml`/`*.yml`) lacking an `SPDX-License-Identifier: MIT` line in its first 5 lines. Generated/lock files and `docs/`, `.orchestrator/` are excluded. Make every file you create pass it.
- `ci.sh` at the repo root: runs, in order, the SPDX check, `go build ./...`, `go vet ./...`, a gazelle-is-up-to-date check (gazelle in diff mode), `bazel build //...`, `bazel test //...`. Exits non-zero on the first failure.
- Do not add anything for the UI, images or other later milestones.
## Scope
May write: repo root build files, `cmd/`, `internal/version/`, `tools/`, this task folder. Must not touch: `docs/`, `LICENSE`, `README.md`, other `.orchestrator/` files. `.gitignore` may be extended only if needed.
## Done when
`./ci.sh` exits 0, and RESULT.md records the pinned versions (rules_go, gazelle, Go SDK), how many tests `bazel test //...` executed, and any decision made.
## Long-running steps
First `bazel build //...` downloads the Go SDK and rules: allow 10 minutes. Send output to a log.
