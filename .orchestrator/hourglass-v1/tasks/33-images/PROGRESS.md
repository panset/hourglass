# PROGRESS 33-images
- 2026-11-19 Options.Clock added (pkg/hourglass/run.go, runhook, internal/app); test TestOptionsClock_CollectorAndLedgerUseTheSuppliedClock passes.
- 2026-11-19 e2e/hourglassd-e2e built (offset clock, HOURGLASS_E2E_CONTROL); test passes.
- 2026-11-19 cmd/hourglass-ui built (proxy, /ui-healthz); httptest tests pass.
- 2026-11-19 Decision: rules_oci 2.2.6 (Bazel 7.4.1 compatible), base pinned by index digest; images built with rules_go goos/goarch/pure binaries, one oci_image per arch, alias by host CPU.
- 2026-11-19 deploy/images, deploy/load-images.sh, deploy/smoke-images.sh done; smoke passes on darwin/arm64 Docker.
- 2026-11-19 bazel build //... and bazel test //... pass (35 tests); check_spdx and check_core_generic pass.
