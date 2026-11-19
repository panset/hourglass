# PROGRESS 21c-api-admin

- 2026-11-19 Read brief, INTENT, CONTRACT, 21a RESULT, openapi admin section, SPEC roles table and ADM/SET/RULE/LED requirements.
- 2026-11-19 Added `internal/store/apiadmin_queries.go` (+ test): ledger stats, period totals, audit count, collector/outage/delivery lists, hold counts.
- 2026-11-19 Wrote `internal/api/handlers_admin.go` (14 handlers) and `admin_common.go`, `admin_views.go`, `admin_system.go`.
- 2026-11-19 Decision: `details` is spliced into the response as raw bytes (encoding/json would compact it and escape `<>&`, breaking SET-2).
- 2026-11-19 Decision: delete/disable of a held setup answers `has_active_hold` (OpenAPI), not `setup_held` (brief).
- 2026-11-19 Wrote tests `admin_*_test.go`; all pass under `-race`.
- 2026-11-19 Ran gazelle on internal/api and internal/store; started `bazel test //internal/api/... //internal/store/...`.
