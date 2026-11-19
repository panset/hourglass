# 14 core-genericity check
Job intent: .orchestrator/hourglass-v1/INTENT.md (read it first). Rules: .orchestrator/hourglass-v1/CONTRACT.md.
## Goal
`tools/check_core_generic.sh`, a script run by `ci.sh` that fails when a core package stops being generic (INTENT "The core must stay generic"; spec EXT-1, EXT-3: govmomi is imported only by the vsphere package).
## Rules to enforce
Core = every Go package under `internal/`, `pkg/hourglass/` and `pkg/hourglasstest/`. Extension space = `providers/`, `pressure/`, `policies/`, `hooks/`, plus `cmd/`, `examples/`, `deploy/`, `e2e/`, `docs/` (not checked by rule 1 and 2).
1. No core `.go` file (tests included) imports a path under `github.com/panset/hourglass/{providers,pressure,policies,hooks,examples}` or any module matching `vmware/govmomi`. Use `go list -deps`-free text parsing of import blocks or `go list -f '{{.ImportPath}}: {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./internal/... ./pkg/...` (direct imports are enough).
2. No core file of any type contains, case-insensitively, any of: `vcenter`, `vsphere`, `govmomi`, `vcsim`, `resource pool`, `resourcepool`, `extralarge`, `size=`, `keycloak`.
3. `vmware/govmomi` is imported only by packages under `providers/vsphere/` and by test-only tooling under `deploy/` or `e2e/` (anywhere else in the repo is a failure).
Print each violation as `path:line: rule: text` and exit 1; exit 0 with a one-line summary otherwise. Must work on macOS (BSD grep/sed, bash 3.2) and Linux, and must pass when the extension directories do not exist yet.
## Also
- Add the script to `ci.sh` right after the SPDX check.
- Prove it works: temporarily add violating files for each of the three rules, show the script fails naming them, remove them. Record the output in RESULT.md.
## Scope
May write: `tools/check_core_generic.sh`, `ci.sh`, this task folder. Other agents are adding Go packages right now; do not touch their files, and do not run Bazel (run the script and `bash -n` only).
## Done when
The script passes on the current tree and fails on each seeded violation.
