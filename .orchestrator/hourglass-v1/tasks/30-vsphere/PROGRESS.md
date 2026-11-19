# Progress: 30 providers/vsphere

- 2026-11-19 Read brief, INTENT, CONTRACT, spike result, API and test-support results.
- 2026-11-19 Added govmomi v0.56.2 to go.mod/go.sum (`go get`, `go mod tidy`); `use_repo` entry added to MODULE.bazel.
- 2026-11-19 Wrote client.go (interface), config.go, vsphere.go (provider, pressure, shared client), govmomi.go (adapter).
- 2026-11-19 Layer 2 tests (fake) and layer 3 tests (simulator) written; found and fixed a real leak: fmt prints a struct's unexported fields without calling `secret.String`, so provider/pressure/adapter got String/GoString methods.
- 2026-11-19 Finder path "..." found no clusters in the simulator; switched to "*" (direct children of the host folder only).
- 2026-11-19 Gazelle run, SPDX line added to BUILD.bazel. `go test -race`, `bazel test //providers/vsphere/...`, `go build ./...`, check_core_generic.sh, check_spdx.sh all pass.
- Next: commit on the worktree branch.
