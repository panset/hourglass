# Progress: 63 report routes and state route
- 2026-11-20: read brief, design, results of 60 and 61.
- 2026-11-20: added `config.ReportedMaxAge` and `api.Runtime.Reported*MaxAge`, wired in `internal/app` (D51).
- 2026-11-20: implemented reportPressure, reportDeployments, getIntegrationState; mapped reported errors in `classify`.
- 2026-11-20: removed the three ops from `integrationStubs`; scale test times state with a token; contract test validates the three routes.
- 2026-11-20: TestINT1/2/3/4/8 added; `go test -race ./internal/...` passes; ci.sh exit 0 (ci.log).
