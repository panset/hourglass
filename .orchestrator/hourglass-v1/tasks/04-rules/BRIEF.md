# 04 internal/rules
Job intent: .orchestrator/hourglass-v1/INTENT.md (read it first). Rules: .orchestrator/hourglass-v1/CONTRACT.md.
## Goal
Package `internal/rules`: label validation, the selector parser, `Match`, and ordered rule evaluation, as pure functions with unit tests. Spec: docs/SPEC.md RULE-1, RULE-2, RULE-5, SET-3.
## Behaviour
- `Parse(selector string) (Selector, error)`; `(Selector).Matches(labels map[string]string) bool`; `Match(selector string, labels map[string]string) (bool, error)`.
- Syntax (RULE-2), the Kubernetes label-selector subset with Kubernetes semantics: `key=value` (also accept `==`), `key!=value` (true when the key is absent), `key in (a,b)`, `key notin (a,b)` (true when absent), `key` (exists), `!key`, joined by commas meaning AND. Whitespace around tokens is ignored. An empty selector matches everything. Keys and values in a selector must satisfy the SET-3 grammar; parse errors are human-readable and say where the problem is (RULE-3 shows them to the admin). No Kubernetes dependency; hand-written parser.
- `ValidateLabels(map[string]string) error` per SET-3: at most 32 labels; keys match `[a-z0-9]([a-z0-9._/-]{0,61}[a-z0-9])?`; values at most 63 characters (empty allowed, any characters; record this as a decision).
- `Rule{ID int64; Kind string; Position int; Selector string; Value string}` and `Evaluate(rules []Rule, kind string, labels map[string]string) (*Rule, error)`: considers only rules of that kind, in ascending Position (stable for ties), returns the first match or nil. Value stays an opaque string here so later kinds need no change (RULE-5).
- Kind `rate`: const `KindRate = "rate"`; `ParseWeight(string) (float64, error)` accepts a non-negative finite decimal (0 allowed) and rejects everything else; `Weight(rules []Rule, labels map[string]string, defaultWeight float64) (weight float64, selector string, matched bool, err error)` returns the first matching rate rule's weight and its selector, or the default with matched=false.
- `MatchCounts(rules []Rule, kind string, labelSets []map[string]string) (perRule map[int64]int, unmatched []int)`: for RULE-3, how many label sets each rule wins (first-match semantics, not merely "selector matches") and the indexes that fall through.
## Scope
May write: `internal/rules/`, this task folder.
## Build notes
The skeleton from task 02 is in place (see tasks/02-build-skeleton/RESULT.md for versions). Standard library only: add no dependency and do not edit go.mod, MODULE.bazel or root files. Generate your BUILD file with `bazel run //:gazelle -- <your package dir>` and test with both `go test ./<dir>/...` and `bazel test //<dir>/...`. Two other agents are building sibling packages at the same time; Bazel commands in one workspace queue behind each other, so allow up to 5 minutes per Bazel command and never kill the Bazel server. The package must not import any other Hourglass package, and must not mention vCenter, vSphere, sizes (medium/large/...) or any concrete extension outside test data that uses neutral labels (e.g. `tier=a`, `site=x`).
## Tests
Table-driven, named with requirement ids (`TestRULE2_...`). Cover every rule in "Behaviour" including error cases and boundaries. Report the number of test cases executed.
## Done when
Tests pass under go and Bazel, `./tools/check_spdx.sh` passes, RESULT.md lists the exported API and decisions.
