# 15 verify the extension API, test support and bundled extensions
Job intent: /Users/tejaspanse/projects/hourglass/.orchestrator/hourglass-v1/INTENT.md. Rules: CONTRACT.md beside it. You write no product code.
## Goal
Independently check `pkg/hourglass`, `pkg/hourglasstest`, `pressure/inventory`, `policies/default`, `providers/static`, `hooks/webhook` and `tools/check_core_generic.sh` against docs/SPEC.md ("Extension points", EXT-1..7, MET-7, EXT-4) and their briefs (tasks/10, 11, 12, 13, 14 BRIEF.md). You did not write them; try to break them.
## Setup
`git clone /Users/tejaspanse/projects/hourglass <scratch>/verify-15/hourglass`, <scratch> = /private/tmp/claude-501/-Users-tejaspanse-projects/17dec24c-e7ec-455e-9b87-01d2217d5fc9/scratchpad (remove verify-15 first). Work only in the clone.
## Checks
1. `go vet ./...`, `go test -race -count=1 ./pkg/... ./pressure/... ./policies/... ./providers/... ./hooks/...`, `bazel test //pkg/... //pressure/... //policies/... //providers/... //hooks/...`, both tools scripts. Report test counts and wall time per package.
2. Spec fidelity: diff the declarations in `pkg/hourglass` against the Go block in docs/SPEC.md "Extension points" token by token (type names, field names and types, method signatures, the four Register functions). Any difference is a bug. The eleven event names and their order; `Vetoable` true only for `hold.acquiring`.
3. Registry: unknown name error lists registered names (EXT-2); duplicate/nil/empty panics; data race free.
4. Third-party view (EXT-5): in a throwaway module outside the clone with a `replace` to the clone, write a main that imports only `pkg/hourglass`, registers a toy provider, and a `_test.go` that runs the four `hourglasstest` conformance suites against toy implementations. It must compile and pass without importing any `internal/` package. Also confirm none of the six packages import `internal/`.
5. `inventory`: hand-computed cases incl. weights 0, no setups, all held; never outside [0,1]. `default` policy: every combination of the two triggers on/off x allocation present/absent x before/after `start_after` x already metering; `start_after` 0; `"off"`; negative rejected; no provider configured (EXT-4).
6. `static`: malformed file, missing file, file edited between calls, empty array. `webhook`: signature verifies with an independent HMAC computation; type filter; non-2xx error carries status and a body excerpt; the secret and the URL never appear in errors; context deadline honoured; redirects not followed.
7. Event JSON: round-trip; nil sub-objects; `details` bytes preserved exactly (including key order and whitespace-insensitive equality at minimum); time in UTC.
8. Conformance suites catch real violations: write three broken toy extensions (pressure 1.5, hook that panics on nil Setup, provider ignoring ctx) and confirm the suites fail them.
9. `pkg/hourglasstest` test time: its own tests take about 40 s. Identify why and recommend a concrete change to get under 5 s without weakening what is asserted.
10. `tools/check_core_generic.sh`: seed one violation per rule in the clone and confirm failure; confirm a false-positive check (a neutral core file with the word "enlarge" or "default" passes).
11. Mutation spot-checks: one deliberate bug in each of the four extensions turns its own tests red.
## Scope
May write: the clone, a throwaway module beside it, and /Users/tejaspanse/projects/hourglass/.orchestrator/hourglass-v1/tasks/15-verify-extensions/.
## Done when
RESULT.md opens with a verdict and a numbered findings list (severity, path:line, reproduction). Status `done` whatever the verdict.
