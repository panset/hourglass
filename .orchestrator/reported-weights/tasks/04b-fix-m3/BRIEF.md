# 04b Fix the findings of verification 04
Job intent: .orchestrator/reported-weights/INTENT.md (read it first). Contract: .orchestrator/reported-weights/CONTRACT.md (binding).
## Goal
Close the four findings in `.orchestrator/reported-weights/tasks/04-verify-m3/RESULT.md` (read its findings list). All are test gaps; no product behaviour changes.
1. Medium: `TestWGT19_ReplayV110PollsWithFeatureOff` (`internal/collector/upgrade_v110_replay_test.go`) skips under Bazel, so `./ci.sh` never runs it. Wire `testdata/upgrade-v1.1.0` into Bazel the way `testdata/upgrade-d96a950` is wired (root `BUILD.bazel` exports, a `data` attribute on the collector test, kept stable under Gazelle with whatever directive the d96a950 wiring uses), and make the test fail, not skip, when the fixture is missing. Show it runs under `bazel test` uncached (`--nocache_test_results` on that target) and passes.
2. Low: add a test named after WGT-2 that a policy's `Decision.Weight` above 0 beats a setup's override (and a usable reported weight) for that hold and poll, with source `policy`. Break `collector.go` once to see it red, restore.
3. Low: add to a WGT-5 test that a policy-weighted sample of a rule-priced setup has a null `rule_selector`. See it red once.
4. Low: the `min: -0` settings test cannot fail (-0 equals 0 as floats). Make it assert what is stored and returned in a way that distinguishes negative zero (the JSON text, or `math.Signbit`). See it red once.
Also: the comment at `internal/store/usage.go:23-24` is stale after milestone 3 (it describes `rule_selector`/weight source as before); bring it up to date. Do not touch `pkg/hourglass/types.go`.
## Scope
May write: `internal/collector/*_test.go`, `internal/collector/BUILD.bazel`, root `BUILD.bazel`, `internal/api/settings_reported_weights_test.go`, `internal/settings/settings_test.go`, the one comment in `internal/store/usage.go`, this task folder. Nothing else; no product code change apart from that comment. Never edit files under `testdata/`.
## Done when
`./ci.sh` exits 0 and the replay test is shown executed (not skipped) under Bazel; RESULT.md lists each finding with the test and the mutation that turned it red.
## Long-running steps
`./ci.sh`: 30 minute timeout, log into this task folder.
## Report
Per CONTRACT.md "Reporting".
