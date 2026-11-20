# Progress 67

- 2026-11-20 Read brief, intent, contract, results of 61 and 65.
- 2026-11-20 Collector: added `last_pressure_report` / `last_deployments_report` to GET /api/v1/admin/collector (internal/api/admin_system.go), OpenAPI, D62, Go test, contract seeding.
- 2026-11-20 OpenAPI: `release_requested_at` required on Hold and Holder; regenerated types; fixtures updated.
- 2026-11-20 UI: Service accounts page and route; Setups and My usage Releasing state, 202 handling; Settings collector report tiles; tests.
- 2026-11-20 `cd ui && npm run typecheck && npm test` pass; `./ci.sh` exit 0 (ci.log, ci.exit). Afterwards reworded one hint on the Settings page (release_requires_teardown); ui typecheck and tests re-run, pass.
