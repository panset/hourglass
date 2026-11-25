# Progress 05-b1-api
- 2026-11-24 Read BRIEF, COMMON, INTENT, DECISIONS, change request, DESIGN rev 2. Next: read handlers_user.go listSetups and tests.
- 2026-11-24 Added pricingJSON + l.pricing() in handlers_user.go; openapi pricing (required) + listSetups description; pinned-key lists in user_test/upgrade_compat/upgrade_v110; new setups_pricing_test.go (TestUI18_) + BUILD entry; regenerated schema.gen.ts via npm run gen:api; pricing in fixtures.ts and setups.test.tsx list(); deploying.md upgrade note + display_label_keys row. go test of UI18/UI12/WGT14 pass. Next: bazel test //internal/api/... and tsc.
- 2026-11-24 bazel test //internal/api/... PASS (2 targets); go test ./internal/api 200 top-level tests pass, 0 fail; tsc exit 0; //ui:api_schema_drift_test PASS. Wrote RESULT.md. Done.
