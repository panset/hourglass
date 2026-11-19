# Progress: 10 extension API

- 2026-11-19 Read BRIEF, INTENT, CONTRACT and SPEC "Extension points", EXT-1/EXT-2, data model.
- 2026-11-19 Wrote pkg/hourglass: doc.go, types.go, events.go, registry.go, hourglass_test.go. Decisions recorded in RESULT.md.
- 2026-11-19 go build ./..., go vet ./pkg/..., go test -race ./pkg/hourglass/... pass.
- 2026-11-19 Generated BUILD.bazel with gazelle, added SPDX header (gazelle does not write one); bazel test //pkg/hourglass/... passes.
- 2026-11-19 check_spdx.sh fails only on internal/store/BUILD.bazel (other agent's scope, not touched).
- 2026-11-19 Wrote RESULT.md. Done.
