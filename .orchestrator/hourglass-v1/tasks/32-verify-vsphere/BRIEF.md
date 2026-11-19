# 32 verify vSphere extension, check-provider and the third-party entry point
Job intent: /Users/tejaspanse/projects/hourglass/.orchestrator/hourglass-v1/INTENT.md. Rules: CONTRACT.md beside it. You write no product code.
## Goal
Independently check `providers/vsphere`, `hourglassd check-provider` and `pkg/hourglass/server` against docs/SPEC.md ("Testing without a vCenter", EXT-3, EXT-5, the Configuration rows for vsphere), docs/decisions.md D5-D8, D27, D28, and the briefs tasks/30-vsphere and tasks/31-check-provider. You did not write them.
## Setup
`git clone /Users/tejaspanse/projects/hourglass <scratch>/verify-32/hourglass`, <scratch> = /private/tmp/claude-501/-Users-tejaspanse-projects/17dec24c-e7ec-455e-9b87-01d2217d5fc9/scratchpad (remove verify-32 first). Work only in the clone. Note: the clone is of committed state only; `ui/` and some API handler files are still being written in the original and are absent or stubs here; ignore them, and if `bazel build //...` fails because of something outside the packages under review, test the specific targets instead.
## Checks
1. `go test -race -count=1 ./providers/vsphere/... ./internal/app/... ./pkg/...`; `bazel test //providers/vsphere/... //internal/app/... //pkg/...`; both tools scripts. Counts.
2. Read-only guarantee: grep the vsphere package (non-test files) for any govmomi call that creates, reconfigures, moves, powers or destroys anything, and for any use of write-capable methods; list every govmomi API call the adapter makes and the vSphere privilege each needs. The provider must work with a read-only role (inference is acceptable, say so).
3. No real vCenter needed: run the vsphere tests with the network disabled as far as you can (`bazel test` sandbox, or unset proxy and confirm only loopback connections via the test code); confirm tests do not read real credentials from the environment (set `HOURGLASS_VSPHERE_PASSWORD=sentinel` and confirm no test depends on or leaks it).
4. Layer 2 completeness against D7: for each logic item in D7's list find the test; write your own extra cases for the pressure formula (three ratios each winning; multi-cluster; threshold crossings at 0.5/0.75/0.9; zero capacity; all datastores inaccessible; ratio > 1 clamped) and for pool matching (nested, duplicate names across clusters, root pool excluded, cluster scoping, unknown cluster). Report any that fail.
5. Layer 3: your own simulator test creating pools `lab-01_RP`, `lab-02_RP` and a nested pool, asserting the provider output; destroy one and re-list; pressure in range and equal to the simulator's storage ratio; self-signed TLS refused unless `insecure_skip_verify`; behaviour after the session is invalidated.
6. Secrets: the password never appears in any error, log line or `%v`/`%+v` of the config or the extension values; try wrong password, unreachable host, bad URL.
7. Genericity: govmomi imported only under `providers/vsphere/`; no core file mentions vSphere; `internal/app/checkprovider.go` is generic.
8. `check-provider`: build `hourglassd`, run it with the `static` provider and a seed file in a temp dir; confirm the output sections, exit codes (0 ok; non-zero on provider error, bad config, unknown extension name with the registered names listed), and that it writes nothing: hash the database file and the directory listing before and after (note whether `-shm`/`-wal` side files appear and whether that is acceptable), and confirm no rows change. Also run it with no database present.
9. Third-party entry point (EXT-5): in a throwaway module outside the clone with a `replace`, a main importing only `pkg/hourglass`, `pkg/hourglass/server` and its own toy pressure source; `go build` with `CGO_ENABLED=0`; start it, hit `/healthz`, stop it. Then remove the `server` import and confirm `Run` returns the instructive error. Confirm the throwaway module cannot import `internal/...` (expected) and does not need to.
10. Mutation spot-checks: four deliberate bugs in `providers/vsphere` (e.g. min instead of max across ratios, include the root pool, skip clamp, ignore cluster scope) turn its tests red.
## Scope
May write: the clone, throwaway modules beside it, and /Users/tejaspanse/projects/hourglass/.orchestrator/hourglass-v1/tasks/32-verify-vsphere/.
## Long-running steps
Bazel in a fresh clone path compiling govmomi: allow 20 minutes.
## Done when
RESULT.md opens with a verdict and a numbered findings list (severity, path:line, reproduction, suggested fix), plus the privilege table for the first-run checklist. Status `done` whatever the verdict.
