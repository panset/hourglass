# Result, task 38: examples/custom

Status: done.

## What was built (all under examples/custom/)
- `filedir/` provider `filedir`: one allocation per regular file in `dir` (dot-files, subdirs skipped, sorted; missing dir is an error).
- `busiestsite/` pressure source `busiest-site`: max over groups of (held weight / total weight), grouped by a label (default `site`).
- `businesshours/` policy `business-hours`: meters only inside timezone/start/end/days; `gpu=true` setups get `Decision.Weight = gpu_weight`; `Reason` always set.
- `logfile/` hook `logfile`: JSON line per event with an `events` filter; vetoes `hold.acquiring` when the setup label `maintenance=true` (veto independent of the filter).
- `cmd/labd/main.go`: imports pkg/hourglass, pkg/hourglass/server, the four extensions and bundled webhook, default policy, inventory, static; `run()` wraps `hourglass.Run` so a test can call it.
- `config.yaml`, `seed.yaml` (4 rigs, 2 sites, one rate rule), `README.md`, `data/` (rigs dir, .gitignore), BUILD.bazel files (gazelle plus SPDX lines, test `data` deps).
- No `internal/` import anywhere under examples/.

## Tests
Each extension: its hourglasstest conformance suite plus behaviour tests (names `TestEXT5_*`). `cmd/labd/main_test.go` starts `run()` (so `hourglass.Run`) with the example config and seed copied to a temp dir, port 0, env-supplied secrets; checks `/healthz` database ok, waits for a successful poll, shuts down cleanly. A second test checks an unknown extension name fails listing registered names.

## Commands run
- `go build ./examples/...`: ok. `go vet ./examples/...`: ok.
- `go test -race -count=1 ./examples/...`: 5 packages pass.
- `bazel run //:gazelle -- examples`, then `bazel test //examples/...`: 5/5 pass (7 s).
- `tools/check_core_generic.sh`: OK. `tools/check_spdx.sh`: OK.

## Awkward points in the public API (feedback for docs)
1. Config parsing boilerplate: every factory repeats "trim, treat empty/null as absent, DisallowUnknownFields, reject trailing data". A helper such as `hourglass.DecodeConfig(raw, &dst)` would remove about ten lines per extension.
2. `hourglass.Run` has no way to learn the bound address when `listen` uses port 0; the test had to parse the JSON log line `http listening`. A callback or channel in Options would help embedders and tests.
3. `Decision.Weight == 0` means "no override", so a policy cannot override to a free (0) weight. Documented in the type, but surprising.
4. `Snapshot.Time` is the only clock a policy should use; this is not stated in doc.go. Worth one sentence.
5. A third-party main must import `pkg/hourglass/server` and choose default pressure/policy names itself (D28, task 32 finding 5); doc.go covers it, but forgetting either is only found at runtime.
6. Hook veto: the error text is shown to the user, but the doc does not say what happens to the hook's own logging for a vetoed acquire; the example logs nothing for it.
7. `Event.Setup.Details` and `Data` are free-form; the example logs labels and Data only. A doc note on which `Data` keys each event sets would help hook authors.
8. `hourglasstest.TestHook` calls Handle with nil User/Setup/Hold, so every hook must nil-guard; stated in the check name only.

## Decisions for docs/decisions.md
- Example config and seed use `timezone: UTC` so no tzdata is needed.
- `busiest-site` and `business-hours` take no windows spanning midnight (start must be before end); simplest option.
- `labd` serves no UI (API-only); bundled extensions offered: webhook, default, inventory, static (not vsphere, to avoid govmomi in the example).
- The `logfile` maintenance veto applies even when `hold.acquiring` is excluded from the `events` filter.

## Confidence and gaps
Verified by running: builds, tests, bazel, both scripts. Not verified: the manual curl walkthrough in the README (acquire of rig-b2 refused over HTTP end to end); the hook veto is tested at the Hook level only. The README run steps appear correct from the config and test but were not executed by hand.
