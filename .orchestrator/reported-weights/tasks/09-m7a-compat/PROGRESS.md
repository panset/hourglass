# Progress, task 09 (m7a compatibility tests)

- Read BRIEF, INTENT, CONTRACT, the design, fixture README, D81, task 07 RESULT, the d96a950 tests.
- Decision: replay responses.json in recorded order on a copy of the v1.1.0 database; diff each answer with a generic JSON differ; per route an explicit list of added paths and named changed values (path, was, now, count). Removed fields and unlisted differences fail; a listed difference that does not happen also fails.
- Finding while replaying: the fixture stores the decoded body of `admin_setup_post`, which loses key order; the audit entry hashes the details as sent. The test sends the recorder's exact bytes for that one request (checked to decode to the recorded body). Not a product difference.
- Finding: the app's start runs a first poll (no provider, clock set by the test): it completes the pending release of hold 3 and appends samples/runs. So the app-level test is split: store.Open migration (exact row equality of every old table) and Run (old rows unchanged except hold 3 completion; appends only to named tables).
- Wrote: internal/app/upgrade_v110_test.go (2 tests), internal/api/upgrade_v110_compat_test.go (4 tests; the 5th WGT-20 test in the file group is task 07b's). Wired Bazel: root BUILD.bazel exports responses.json and openapi.yaml, data attributes in internal/app and internal/api BUILD.bazel.
- Break checks done with go test and restored (breaks.log).
- Running ./ci.sh, log ci.log, exit code ci.exit.
- fixed a race in the app-level test (stop only after the first poll's collector run is written); rerunning ci
