# 38 examples/custom: a worked third-party build
Job intent: .orchestrator/hourglass-v1/INTENT.md (read it first). Rules: .orchestrator/hourglass-v1/CONTRACT.md.
## Goal
`examples/custom`: the worked example EXT-5 requires: a `main` such as another team would write, with its own extensions, building its own binary without forking the core. Spec: docs/SPEC.md "Extension points", EXT-5, suggested layout. Decision D28 in docs/decisions.md.
## Context
`pkg/hourglass` (doc.go, types, registry, run.go), `pkg/hourglass/server`, `pkg/hourglasstest` (conformance suites), the bundled extensions as style reference (`pressure/inventory`, `policies/default`, `providers/static`, `hooks/webhook`), tasks/31-check-provider/RESULT.md, tasks/32-verify-vsphere/RESULT.md finding 5 (a third-party main must import a metering policy and a pressure source itself).
## Contents
- A deliberately non-vSphere scenario to show the framework is generic, e.g. a lab of GPU rigs: one custom extension of EACH of the four kinds, each small, readable and commented for a reader learning the API:
  - provider `filedir`: one allocation per file in a directory (the file name is the resource name);
  - pressure source `busiest-site`: pressure = the highest held-weight fraction among groups of setups sharing a `site` label (shows branching on labels from the Snapshot);
  - metering policy `business-hours`: meters only inside configured hours and applies a weight override for setups labelled `gpu=true` (shows `Decision.Weight` and `Reason`);
  - hook `logfile`: appends each event as a JSON line to a file, with an event-type filter (shows vetoing `hold.acquiring` when a label says the setup is under maintenance).
  Each in its own package under `examples/custom/`, registered by name in `init()`, with config parsed from its opaque block.
- `examples/custom/cmd/labd/main.go`: imports `pkg/hourglass`, `pkg/hourglass/server`, the four example extensions, and whichever bundled extensions it also wants to offer; calls `hourglass.Run`. No `internal/` import anywhere under `examples/`.
- `examples/custom/config.yaml` and a seed file using these extensions, and `examples/custom/README.md` walking through: what each extension does, how to build (`go build ./examples/custom/cmd/labd`), how to run it locally with no OIDC (local admin only), and how to run the conformance suites.
- Tests: each extension runs its `pkg/hourglasstest` conformance suite plus a few behaviour tests; one test starts `labd`'s wiring in-process (through `hourglass.Run` with a temp database and the example config, on a free port) and checks `/healthz`, proving the third-party path works end to end.
- Must be part of the main module (so `go build ./...` and Bazel cover it) but written exactly as an external module would write it. `tools/check_core_generic.sh` must still pass.
## Scope
May write: `examples/custom/`, this task folder. BUILD via `bazel run //:gazelle -- examples` plus SPDX first lines. No new dependencies. Other agents are editing `internal/api`, `cmd/`, `internal/app`, `deploy/`, MODULE.bazel concurrently; Bazel commands queue and may fail transiently outside your package.
## Done when
`go build ./examples/...`, `go test -race -count=1 ./examples/...`, `bazel test //examples/...` (allow 15 minutes), both tools scripts pass. RESULT.md summarises the example and anything in the public API that was awkward to use (useful feedback for the docs).
