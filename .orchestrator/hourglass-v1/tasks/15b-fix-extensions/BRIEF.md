# 15b fix extension verification findings
Job intent: .orchestrator/hourglass-v1/INTENT.md. Rules: .orchestrator/hourglass-v1/CONTRACT.md. Findings with reproductions: .orchestrator/hourglass-v1/tasks/15-verify-extensions/RESULT.md (read all 11) and `hourglasstest-speedup.patch` beside it.
## Changes
1. `tools/check_core_generic.sh`: catch every import form (blank `_`, dot, aliased, single-line `import "x"`, one-line parenthesised) for rules 1 and 3. Simplest robust approach: take imports from `go list -f` (Imports, TestImports, XTestImports) per package rather than text parsing, keeping `path:` output useful. Re-run the seeded-violation proof for each form and record it.
2. `pkg/hourglasstest`: apply the speed-up (run only the asserted check against each broken implementation, `t.Parallel()` where safe) so the package's tests take under 5 s with `-race`, with no assertion removed.
3. `pressure/inventory`: non-finite or negative weights in the snapshot are treated as an error return (never NaN, never out of range). Test.
4. `policies/default`: the allocation trigger fires when `HoldState.Allocation` is set OR the Snapshot has an allocation for the hold's setup (either is sufficient; the collector fills both). Test both.
5. `hooks/webhook`: errors never contain any part of the secret (redact before truncating to 200 bytes, and redact partial overlaps at the cut) nor the URL or its host:port (replace transport error text with a generic class such as "connection failed", "timeout", keeping the status code path unchanged). Tests.
6. Fix the remaining LOW findings in RESULT.md where the fix is local and obvious; for any you leave, say why in your RESULT.md. Do not change the Event JSON's handling of `details` (compaction by encoding/json is accepted; it is semantically unmodified).
## Scope
May write: `tools/check_core_generic.sh`, `pkg/hourglasstest/`, `pressure/inventory/`, `policies/default/`, `hooks/webhook/`, `providers/static/` (only if a finding needs it), this task folder. Do not change exported signatures in `pkg/hourglasstest` (other agents are compiling against it right now) and do not touch `pkg/hourglass`, `internal/`, go.mod or MODULE.bazel.
## Done when
`go test -race -count=1 ./pkg/... ./pressure/... ./policies/... ./providers/... ./hooks/...`, the matching `bazel test`, and both tools scripts pass. RESULT.md maps each of the 11 findings to fixed / left with reason.
