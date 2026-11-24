# 07b The state route never fails on a rate rule that cannot be evaluated
Job intent: .orchestrator/reported-weights/INTENT.md (read it first). Contract: .orchestrator/reported-weights/CONTRACT.md (binding).
## Goal
Verification 07 (`.orchestrator/reported-weights/tasks/07-verify-m5/RESULT.md`, first finding) found that `GET /api/v1/integration/state` answers 500 when a stored rate rule cannot be evaluated; `v1.1.0` answered 200 because the route read no rule. Orchestrator ruling (supersedes D84(g)): the design says such a rule "keeps its current effect in each caller", and this caller's current effect is to answer. So the state route handles the error as the admin views do (D85(b)): resolve the setup as if no rate rule existed (default weight for the rule part; an override or a usable reported weight still applies) and answer 200. Reuse the admin views' helper; do not write a second fallback.
## Tests
- A test named after WGT-13 (and one named WGT-20 or in its spirit: "state answers as before with a rule that cannot be evaluated, feature off"): store an unparseable rule directly through the store, call the state route with the feature off and on, expect 200 with the default weight / the reported weight / the override. See it red against the current code first.
- Replace whatever existing test asserts the 500, if one does (it was written in milestone 4, not a pre-existing test); say which.
## Scope
May write: `internal/api/handlers_integration.go`, the shared helper's file if it must be exported within the package, `internal/api/*_test.go`, this task folder. Another agent is working in `ui/src` at the same time: do not touch `ui/`, `api/openapi.yaml` or the generated types. If `./ci.sh` fails only in a UI target, rerun it once after a few minutes and report what you saw.
## Done when
`go test ./internal/...` passes and `./ci.sh` exits 0; RESULT.md lists files changed and the tests.
## Long-running steps
`./ci.sh`: 30 minute timeout, log into this task folder.
## Report
Per CONTRACT.md "Reporting".
