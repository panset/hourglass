# Progress 12-verify-m7

- 2026-11-23 Read BRIEF, INTENT, CONTRACT, design doc. Baseline HEAD 036e018 recorded; working tree status saved to status-before.txt.
- Next: ./ci.sh (log ci.log), then uncached bazel test (bazel.log); in parallel static review.
- 2026-11-23 ./ci.sh exit 0 (ci.log); bazel test //... --nocache_test_results exit 0, 40/40 (bazel.log); Go tests under Bazel with -test.v: 755 top-level + 1138 subtests pass, 0 skip, 80 TestWGT* pass (bazel-verbose.log); vitest verbose 251 pass, 33 UI-12..14 tests (vitest-verbose.log).
- 2026-11-23 Whole-job git checks done: SPEC and design untouched, testdata identical, types.go comment only, migration additive, check_core_generic OK, banned words only in design + .orchestrator.
- 2026-11-23 Mutations in scratch copy (never in repo): 12 UI (all red), Go WGT-19/20 and re-checks of WGT-1/2/3/6/12/13/17/21 (ui-mutations.log, go-mutations.log). Found: no test asserts the value of Setups list reported_weights_enabled.
- 2026-11-23 e2e W1-W6 read against design bullets; steps.json is the earlier run (I0-I10 only), W steps not run.
- Next: docs vs behaviour (section 3), decisions D80-D88 (section 5), pre-existing test edits (section 4), then RESULT.md.
- 2026-11-23 Docs vs behaviour checked against code and 11's examples.log; decisions D80-D88 checked; pre-existing test edits judged (none weakened).
- 2026-11-23 RESULT.md written: 0 high, 1 medium (missing SPEC-edits decision row), 6 low, 4 info; e2e W1-W6 not run. git status shows only this folder's files. Done.
