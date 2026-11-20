# Result: 63 report routes and the state route

Status: done. `go test -race -count=1 ./internal/...` passes; `tools/check_core_generic.sh` OK; `./ci.sh` exits 0 (36 of 36 Bazel tests; ci.log).

## What was built
- `internal/api/handlers_integration.go`: `PUT pressure`, `PUT deployments`, `GET state` replace their stubs. `POST holds/{id}/release` is still the 501 stub (task 65).
- Checks run in the order scope, not_configured (409, names `pressure.type` or `provider.type`, nothing stored), body validation, ordering (D53). Limits: 10,000 entries, 32 labels, 1 MB body, duplicate name 400. `unknown` and `accepted` come from one setups read. The stored report records the calling account's name.
- State reads in a fixed number of queries (settings, rates/pressure/poll view, setups, active holds, users); the scale test now times it with a token.
- `internal/api/helpers.go` `classify` maps `reported.StaleError` to 409 `stale_report` and `reported.ValidationError` to 400.
- OpenAPI schemas already matched; the state description now says what is null when the extension is not `reported`. `ui/src/api/schema.gen.ts` regenerated.

## How the API learns the `reported` configuration and max_age (for task 64)
`config.ReportedMaxAge(*Extension)` (`internal/config/config.go`) returns (max_age, selected, err): selected when the block's `type` is the string `reported`; `max_age` is read from the opaque config block as a Go duration string, default 10m, minimum 1m (below the minimum or unparseable fails `config.Validate`). `internal/app/app.go` stores the result for `provider` and `pressure` in `api.Runtime.ReportedProviderMaxAge` and `ReportedPressureMaxAge` (0 means not configured). The core imports no extension package. The extensions in `providers/reported` must parse `max_age` with the same default and minimum (D51).

## Decisions appended to docs/decisions.md
D51 (how the config is read), D52 (what state holds when the extension is not `reported`: nulls; reported_deployed false; smoothed and multiplier from the latest sample), D53 (check order, required `deployments`, limits, unknown/accepted).

## Tests
- New `internal/api/integration_reports_test.go`: `TestINT1_PressureValueValidated`, `TestINT2_DeploymentsReplaceWholeSet`, `TestINT2_ReadersNeverSeeAMixOfTwoReports`, `TestINT3_OlderReportRefusedEqualReplaces`, `TestINT3_ReportDoesNotPollDebitOrEmit`, `TestINT4_NotConfiguredRefusedAndNothingStored`, `TestINT8_StateHasEverySetupAndHoldAndNothingElse` (walks every JSON key against an allow-list), `TestINT8_StateWithoutReportedExtensions`. `TestReportedMaxAge` in `internal/config`.
- Changed: `permissions_test.go` (fixture configures reported; three ops out of `integrationStubs`), `contract_test.go` (reported config, success and 400/409 calls for the three routes), `perf_scale_test.go`, `service_accounts_test.go` (state now answers 200, not 501; `reachedHandler`).

## Files changed
api/openapi.yaml; docs/decisions.md; ui/src/api/schema.gen.ts; internal/api/{api.go,handlers_integration.go,helpers.go,BUILD.bazel,integration_reports_test.go,contract_test.go,perf_scale_test.go,permissions_test.go,service_accounts_test.go}; internal/app/app.go; internal/config/{config.go,config_test.go}; this folder.

## Confidence and gaps
- The INT-2 reader test is a race test over SQLite snapshot isolation; it passed under -race but is not a proof.
- `config.Validate` now rejects a bad `reported` max_age at load; before task 64 lands, a config selecting `reported` still fails later at extension lookup.
- Test OIDC users have an empty username, so the state test asserts `username` is present but empty.
