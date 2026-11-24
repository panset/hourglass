# 07 Independent verification of milestones 4 and 5
Job intent: .orchestrator/reported-weights/INTENT.md (read it first). Contract: .orchestrator/reported-weights/CONTRACT.md (binding; you are read-only apart from temporary mutations you restore).
## Goal
Decide, from the code and the requirements alone, whether WGT-7 to WGT-11, WGT-13 to WGT-16, WGT-18 and WGT-21 to WGT-23 hold and whether their tests would catch a break. You did not write this code. Do not read the RESULT.md, PROGRESS.md or mutation logs under `.orchestrator/reported-weights/tasks/05-*` and `06-*`: the authors' conclusions are not evidence.
## What you are given
- The requirements: docs/design/reported-weights.md (whole; sections A3, B1 to B4, C1, C2, D2, D3, Compatibility, Requirements, Tests) and D80 to the last row of docs/decisions.md (decisions taken; check the code matches them and that none contradicts the design).
- The code: `git diff e65477b..HEAD` (e65477b is the commit before milestone 4) and the tree.
## Do
1. Run `go test -count=1 ./...`, and `./ci.sh`; then `bazel test //... --nocache_test_results` once so nothing is a cache hit. Report tests executed, skipped, and that the WGT-named ones ran under Bazel.
2. Per requirement id: find its tests by name, read tests and code, verdict hold / does not hold / partly with `path:line`; one deliberate break in product code per id, confirm a test named after the id goes red, restore exactly. Report ids whose named tests stay green under a real break.
3. Try to break it over HTTP with your own throwaway test file or a running server (delete it afterwards). At least:
   - Report route: every clause of WGT-8 and B1 (string weight, null, boolean, `1e400`, `-0`, `NaN` bare token, exactly min and max, 10,001 entries, a body over 1 MB, 17 attributes, 64-character value, bad key, unknown field in an entry and at top level, duplicate name, missing/null `weights`, empty list); message always has the position and the name when there is one; nothing stored and the previous report still in force after each refusal; check order scope, off, body, ordering (off with an invalid body and with a stale `observed_at`); stale, equal and future `observed_at`; independence from the deployments report; unknown names; concurrency of two reports; the report never triggers a poll, debit or event; the last refusal is recorded for a 400 only.
   - Permissions: the new route for every principal; an account with every other scope gets 403; `weights:write` alone opens no other route; cookies refused on the route.
   - State route: every B4 field; each `not_applied` value and precedence; feature off; no rule selector, balance, ledger, audit or other settings leak.
   - WGT-14: pick one database state and time and compare every reader yourself, for override, reported, rule, default and policy; then run a poll at that time and compare the sample. Look for any reader that still computes a weight on its own (`rules.Weight` callers outside the resolver, `holdWeight`).
   - WGT-15: ledger fields in every-poll view, day view, after compaction; partial samples; pre-migration samples with null source; non-debits; grant and reset responses must not carry the fields; paging.
   - WGT-16, WGT-18, WGT-21, WGT-22, WGT-23 clause by clause, including PUT setups without `weight_override` leaving it unchanged, `null` clearing, audit snapshots, rename following the name, the override following the setup.
   - Released holds and A3's two deliberate changes: are they the only behaviour changes with the feature off? Compare the touched routes' responses with `testdata/upgrade-v1.1.0/responses.json` (README there) with the feature off and no override: only added fields and the two A3 changes may differ. Report every other difference.
4. Existing tests: `git diff 0d20688..HEAD --stat -- '*_test.go' 'ui/src/**/*.test.*' e2e/` and judge each edit to a pre-existing test: weakened or not, and whether it is an A3 change or an additive change the design requires.
5. Contract: `api/openapi.yaml` against `testdata/upgrade-v1.1.0/openapi.yaml`: nothing removed or renamed, no new `required` request field, new response fields as the design names them; generated `ui/src/api/schema.gen.ts` is current.
6. Rules: `tools/check_core_generic.sh` passes; no first-client names (design section E) outside the design; no logic that remembers a removed weight; docs/SPEC.md and the design untouched; the scale test (`HOURGLASS_SCALE_TEST=1 go test ./internal/api -run NFR_ApiUnder500msAtSpecScale -count=1 -timeout 20m`) passes.
## Scope
May write: this task folder only. Restore every mutation and delete throwaway files; finish with `git status --short` showing nothing of yours.
## Done when
RESULT.md has: findings ranked by severity, each with a concrete failing scenario and `path:line`; the per-requirement table (verdict, tests, mutation, red or green); test counts; what you could not check.
## Long-running steps
`./ci.sh` 30 minutes; uncached Bazel 30 minutes; scale test 20 minutes. Log into this task folder.
## Report
Per CONTRACT.md "Reporting". Reply in at most ten lines with the number of findings by severity.
