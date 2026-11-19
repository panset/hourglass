# 32b fix vSphere verification findings
Job intent: .orchestrator/hourglass-v1/INTENT.md. Rules: .orchestrator/hourglass-v1/CONTRACT.md. Findings: .orchestrator/hourglass-v1/tasks/32-verify-vsphere/RESULT.md (and `extra_vsphere_test.go.txt` beside it: adopt its useful cases as regression tests).
## Changes (providers/vsphere only)
1. The provider returns an error (failing the poll) when a configured `datacenter` or any name in `clusters` does not exist, the same way the pressure source does; an existing cluster with zero pools is still a valid empty result. Tests at layer 2 and layer 3.
2. Add the missing lower-clamp test so that mutation goes red; strengthen the TLS-error test's assertion that no credential appears in the error.
3. Give the provider and pressure-source structs (and the config type) `String()`/`GoString()`/`LogValue` implementations, or keep the password out of the struct after login, so that formatting them with `%v` or `%+v` can never print the password. Test.
No other changes. Scope: `providers/vsphere/`, this task folder. Verify: `go test -race -count=1 ./providers/vsphere/...`, `bazel test //providers/vsphere/...` (allow 10 minutes; commands queue), both tools scripts.
## Done when
All pass; RESULT.md maps each finding to its fix.
