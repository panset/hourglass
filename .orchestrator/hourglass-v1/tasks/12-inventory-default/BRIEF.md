# 12 pressure/inventory and policies/default
Job intent: .orchestrator/hourglass-v1/INTENT.md (read it first). Rules: .orchestrator/hourglass-v1/CONTRACT.md.
## Goal
Two bundled extensions: the `inventory` pressure source (package `pressure/inventory`) and the `default` metering policy (package `policies/default`, Go package name `defaultpolicy`). Spec: docs/SPEC.md "Burn rate", "Lifecycle of a hold" step 2, MET-7, EXT-3, EXT-4.
## Behaviour
- `inventory`: pressure = sum of resolved weights of setups with an active hold / sum of resolved weights of all enabled setups in the Snapshot. Total weight 0 (no setups, or all weight 0) gives 0. Result clamped to [0,1]. Takes no config (empty block or `{}`). Needs no backend and ignores allocations.
- `default` policy config: `{"start_after": "2h", "start_on_allocation": true}`; defaults are exactly those. `start_after` is a Go duration string; `"0"`/`"0s"` means meter from the moment of acquire; the string `"off"` disables the time trigger; negative durations are rejected. `start_on_allocation: false` disables the allocation trigger. Both disabled is accepted (nothing is ever metered) .
- `Decide`: Meter is true if metering has already started for the hold (its metering-started time is set: metering continues until release, MET-7), or the allocation trigger is on and the Snapshot has an allocation for the hold's setup, or the time trigger is on and snapshot time - acquired_at >= start_after. `Weight` is always 0 (use the rate rules). `Reason` says which trigger applied in words a user can read, e.g. "resource reported by provider", "2h0m0s since acquire" formatted compactly as "2h since acquire", "metering already started"; when not metering, Reason explains what it is waiting for.
- With no provider configured there are no allocations, so only the time trigger can fire (EXT-4); test this.
## Scope
May write: `pressure/inventory/`, `policies/default/`, this task folder.
## Context
API: `pkg/hourglass` (see tasks/10-extension-api/RESULT.md). Fakes and conformance suites: `pkg/hourglasstest` (see tasks/11-hourglasstest/RESULT.md). Each extension is its own package, registers itself by name in `init()` through the public registry, and imports only the standard library and `pkg/hourglass` (plus `pkg/hourglasstest` in tests). No `internal/` imports: these packages must be buildable by a third party exactly as a third-party extension would be.
## Build notes
No new dependencies; do not edit go.mod or MODULE.bazel. BUILD files via `bazel run //:gazelle -- <dir>`, then add the SPDX comment as the first line of each generated BUILD.bazel. Another agent is building sibling extension packages at the same time; Bazel commands queue, allow 5 minutes each. Verify with `go test -race ./<dir>/...`, `bazel test //<dir>/...`, `./tools/check_spdx.sh`, `./tools/check_core_generic.sh`.
## Tests
Each extension runs its conformance suite from `pkg/hourglasstest`, plus table-driven tests of its own behaviour named with requirement ids. Factory tests: valid config, empty config, unknown field (rejected), malformed JSON.
## Done when
All checks pass; RESULT.md lists config schema, behaviour table and decisions.
