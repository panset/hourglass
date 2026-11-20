# Result: 64 the `reported` provider and pressure source

Status: done. `go test -race -count=1 ./...` passes; `tools/check_core_generic.sh` OK; `./ci.sh` exits 0 (37 of 37 Bazel tests; ci.log).

## Wiring mechanism (for the docs task)
1. `pkg/storedreports` is a small public package with no core or extension imports: a read-only `Reader` (`Deployments`, `Pressure`, each already aged with the server's clock), `Publish(Reader)` and `Current()`.
2. `internal/app` publishes an adapter over `internal/reported` when it opens the store (`internal/app/storedreports.go`, called from `app.Run`); `check-provider` publishes one over a read-only store.
3. `providers/reported` registers a provider and a pressure source named `reported`; each call fetches `storedreports.Current()` and reads the last report. Factories get only their config block (`max_age`), so `types.go` is unchanged.
4. A third-party `main` gets both by `import _ ".../providers/reported"`; blank import added to `cmd/hourglassd/main.go` and `e2e/hourglassd-e2e/main.go`.
5. The core never imports the extension. One process-wide reader: two servers in one process share the last published one (tests only).

## Behaviour
As the brief. Errors: `no deployment report received yet`, `no pressure report received yet`, `reported: the <kind> report is <age> old, older than max_age <d>`. `max_age` parsed by the extension with the same bounds as `config.ReportedMaxAge`. `check-provider` prints `deployment report: age ...` / `pressure report: age ...` (or `none received yet`) per selected extension.

## Tests (`providers/reported/reported_test.go`, outside the core)
`TestINT5_NoReportThenStaleThenFresh` (real collector: poll.failed, no usage sample or debit, fresh succeeds), `TestINT5_StaleDeploymentsAloneFailTheProviderOnly`, `TestINT5_ProviderMapsEachDeploymentWithLabels`, `TestINT5_Config`, `TestINT6_ReportsSurviveReopen`, `TestINT7_ReportedSetupWithHoldGetsDeployedAndMeteringStarts`, `TestINT7_ReportedSetupWithNoHoldIsAnOrphan`, `TestEXT5_Conformance` (both suites), `TestCheckProviderPrintsReportAges`.

## Decisions appended: D54 (window), D55 (own max_age parsing, check-provider output).

## Files changed
pkg/storedreports/{storedreports.go,BUILD.bazel}; providers/reported/{reported.go,reported_test.go,BUILD.bazel}; internal/app/{storedreports.go,app.go,checkprovider.go,BUILD.bazel}; cmd/hourglassd/{main.go,BUILD.bazel}; e2e/hourglassd-e2e/{main.go,BUILD.bazel}; docs/decisions.md; this folder.

## Confidence and gaps
- The e2e scenario with a scripted reporter (design Files table) is not part of this task and was not run.
- `check-provider` against a database at an older schema fails at the read-only open, as it already does for the setups read.
- docs/extending.md and docs/deploying.md are left to the docs task.
