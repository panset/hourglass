# Progress
- 2026-11-20 Read brief, intent, contract, design. 
- 2026-11-20 Wrote migration 0005, store queries (service_accounts.go, reported.go, hold and audit columns), internal/reported, tests.
- 2026-11-20 Fixed existing tests for migration count and the v2 upgrade test (raw hold insert). Decisions D44-D46 appended.
- 2026-11-20 Ran gazelle. Next: go test -race ./internal/..., then ./ci.sh once.
- 2026-11-20 Added missing SPDX header to internal/reported/BUILD.bazel; ./ci.sh exit 0 (ci.log). Done.
