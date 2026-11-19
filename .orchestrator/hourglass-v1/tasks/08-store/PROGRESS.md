# Progress 08-store
- 2026-11-19: Part A done: go.mod go 1.26.0, Bazel Go SDK 1.27.4, rules_go 0.59.0, gazelle 0.47.0, modernc.org/sqlite v1.62.0.
- 2026-11-19: Part B/C done: migrations/0001_init.sql, access layer, tests passing (go test -race, CGO_ENABLED=0, bazel test //internal/...).
- 2026-11-19: ./ci.sh stops at its gazelle diff step only because pkg/hourglasstest (other agent) has no BUILD.bazel yet; remaining ci steps run by hand pass.
