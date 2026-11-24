# 04 Independent verification of milestones 2 and 3
Job intent: .orchestrator/reported-weights/INTENT.md (read it first). Contract: .orchestrator/reported-weights/CONTRACT.md (binding; you are read-only apart from temporary mutations you restore).
## Goal
Decide, from the code and the requirements alone, whether WGT-1 to WGT-6, WGT-12 and WGT-17 hold and whether their tests would catch a break. You did not write this code. Do not read the authors' RESULT.md or PROGRESS.md files under `.orchestrator/reported-weights/tasks/02-*` and `03-*`: their conclusions are not evidence.
## What you are given
- The requirements: docs/design/reported-weights.md (whole document; sections A1, A2, B2 ordering, B3, D1, Compatibility, Requirements, Tests).
- The code: `git diff 0d20688..HEAD` (0d20688 is the commit before product code changed) and the tree.
## Do
1. Run `go test -count=1 ./...` and `./ci.sh` yourself; report how many tests executed and that the WGT-named ones ran.
2. For each requirement id above: find its tests by name, read them and the code, and state hold / does not hold / partly, with `path:line`. For each, make one small deliberate break in product code, confirm a test named after the id goes red, and restore the file exactly (confirm with `git diff --stat` equal to before your change). Report any id whose named tests stay green under a real break.
3. Try to break it. At least: every A1 rule one by one; limits exactly at min and max; negative zero; `not_applied` precedence; feature off with a stored report and with an override; override 0; rule weight 0 under a usable report; report expiring exactly at `max_age`; `observed_at` in the future (age never below zero); the two A2 fallback cases; `Decision.Weight` with override and with reported; `rule_selector` under each source; any path where `SetupState.Weight` could carry a reported weight; any logic that remembers a removed weight (forbidden); a poll failing because of a weights report that is missing, expired or unusable (forbidden); settings `PUT` without the key resetting it; a `PUT` with the key and three fields; `enabled` true with `max` null; audit before/after; migration 0008 rewriting any existing row.
4. Compatibility with v1.1.0 at this point: with the feature off and no override, does a poll write the same samples and debits and pass the same snapshot as `testdata/upgrade-v1.1.0/poll.json` records? Say how you checked. Do the d96a950 tests pass unchanged, and was any existing test edited (`git diff 0d20688..HEAD --stat -- '*_test.go' 'ui/src/**/*.test.*'`)? Judge each edit: weakened or not.
5. Rules of the job: `tools/check_core_generic.sh` passes; nothing names the first client (design section E names) outside the design doc; `pkg/hourglass/types.go` changed only in the one comment; docs/SPEC.md and the design are untouched.
## Scope
May write: this task folder only. Temporary mutations to product code must be restored; finish with `git status --short` showing no change you made.
## Done when
RESULT.md has: a findings list ranked by severity, each with a concrete failing scenario and `path:line`; the per-requirement table (verdict, tests, mutation, red or green); test counts; what you could not check.
## Long-running steps
`./ci.sh`: 30 minute timeout, log into this task folder.
## Report
Per CONTRACT.md "Reporting". Reply in at most ten lines with the number of findings by severity.
