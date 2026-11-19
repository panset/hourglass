# RESULT 31-check-provider

Status: done, with one scope deviation (item 1 under "Open questions").

## Files changed
- `cmd/hourglassd/main.go`, `BUILD.bazel`: blank-imports `providers/vsphere` and `pkg/hourglass/server`; new `check-provider` subcommand. main still imports `internal/app`, but only for `app.CheckProvider`.
- `internal/app/checkprovider.go` (new, generic, no provider named), `checkprovider_test.go`, `BUILD.bazel`.
- `internal/collector/match.go` (new, additive: `DedupAllocations`, `OrphanNames`, `SetupRef`, `Reconciliation`, `Reconcile`); `collector.go` now calls `DedupAllocations` and `OrphanNames` instead of its own inline loops (behaviour unchanged, collector tests pass).
- `internal/store/store.go`: new `OpenReadOnly` (OUTSIDE the brief's "May write"; see open question 1), `BUILD.bazel` regenerated.
- `pkg/hourglass/server/server.go`, `BUILD.bazel` (new public package, imports `internal/app` for its side effect).
- `pkg/hourglass/run.go`: `Run` wraps `runhook.ErrNoImpl` (still `errors.Is`-able) with the import hint; `doc.go` shows the import and a full main; test `TestEXT5_RunWithoutServerImportNamesThePackage` in `hourglass_test.go`.
- No dependency added; MODULE.bazel not touched by me (gazelle's edits to the root files in `git status` are other agents').

## check-provider
`hourglassd check-provider [-config path]` (also HOURGLASS_CONFIG). Builds only the provider and pressure source through the registry (pressure default `inventory` when unnamed), loads setups: database file exists -> `store.OpenReadOnly` (setups, settings incl. resource-name template and bands, active holds); else the seed file if configured; else none. One `List` and one `Pressure` call (30 s bound each), then prints matched setups (disabled/held noted), enabled setups expected but not reported, orphans (reported, matching a setup with no active hold), the count of distinct allocations matching no setup, raw pressure and the multiplier the current bands give for the raw value (unsmoothed; no history is read). No provider configured: says so and skips matching. Any error (config, unknown extension, provider/pressure error, pressure outside 0..1, bad seed, schema mismatch) exits 1 with a message on stderr. Matching logic is `collector.Reconcile`, built on the same helpers the poll uses.

Sample (real binary, static provider, seed file, no database yet; then the same after the server created the database; exit 0 both times):
```
setups: 3 (no database yet; seed file /tmp/.../seed.yaml)
provider: static
matched setups (2):
  lab-01 -> lab-01
  lab-02 -> lab-02
expected but not reported (1):
  lab-03 -> lab-03
orphans (2):
  lab-01
  lab-02
ignored allocations: 2
pressure source: inventory
raw pressure: 0.0000  multiplier: 0x (current bands, unsmoothed)
```
A malformed allocations file gives `hourglassd check-provider: provider static: static: parse ...: unexpected end of JSON input`, exit 1.

Tests (`internal/app/checkprovider_test.go`, fake provider and fake pressure source registered through the registry; the `static` provider cannot be imported from core tests, `tools/check_core_generic.sh` rule 1, so its behaviour was shown with the real binary above): report content incl. duplicates, disabled and ignored (TestEXT3_CheckProviderReport), database bytes unchanged and writes through the read-only store fail, held setup not an orphan, no database + seed (no database file created), no database no seed, no provider (provider never called), error paths.

## Third-party entry point (EXT-5)
Throwaway module `/private/tmp/claude-501/-Users-tejaspanse-projects/17dec24c-e7ec-455e-9b87-01d2217d5fc9/scratchpad/thirdparty-31/` (`go.mod` with `replace github.com/panset/hourglass => /Users/tejaspanse/projects/hourglass`; main imports `pkg/hourglass`, `pkg/hourglass/server` and its own `toy` extension that registers a pressure source and a metering policy named `toy`). Transcript:
```
$ go build -o thirdparty .            -> BUILD_OK (plain go build)
$ HOURGLASS_LISTEN=127.0.0.1:18931 HOURGLASS_DATABASE=<tmp>/h.db ./thirdparty &
$ curl http://127.0.0.1:18931/healthz
{"status":"ok","database":"ok","last_poll_age_seconds":0}
$ kill -INT; exit=0   (log: "http listening", "poll ok", "shutting down", "stopped")
```
Negative: the same main without the server import builds and prints
`hourglass: the server implementation is not linked into this binary: add the import _ "github.com/panset/hourglass/pkg/hourglass/server" to your main package`, exit 1.

## Commands and outcome
- `go build ./...` ok. `go vet` and `gofmt -l` clean on touched dirs.
- `go test -race -count=1 ./cmd/... ./internal/app/... ./internal/collector/... ./internal/store/... ./pkg/...` all ok.
- `bazel run //:gazelle -- <touched dirs>` ok. `bazel test //cmd/... //internal/app/... //internal/collector/... //internal/store/... //pkg/...`: 5 of 5 pass, build of `//cmd/hourglassd` (with vsphere and govmomi) ok.
- `tools/check_core_generic.sh` OK. `tools/check_spdx.sh` reports only `ui/pnpm-lock.yaml` (another agent's); mine pass.

## Decisions for docs/decisions.md
1. check-provider: multiplier is for the raw pressure with the stored bands (no smoothing history is read); it states "unsmoothed".
2. "Orphans" are as the collector defines them (HOLD-4), listed by resource name; "matched" includes disabled setups (flagged); "expected but not reported" lists enabled setups only.
3. The snapshot given to the pressure source has setups with resource names but weight 0 (rate rules are not resolved), and active holds with only the setup name.
4. A read-only open of a WAL database may create the `-shm` and `-wal` side files (SQLite needs them for any reader); the database file itself is never modified (tested by hash). If the schema version differs from the binary's, check-provider fails and asks to start the server once.
5. The 30 s per-call bound reuses `collector.ExtensionTimeout`.

## Open questions for the owner
1. I added `store.OpenReadOnly` in `internal/store/store.go`, which is outside this brief's write scope; no in-scope way gives a truly read-only open (`store.Open` migrates and sets WAL). Additive, tested; move or reject as you see fit.
2. `cmd/hourglassd` still imports `internal/app` (legal inside the module) for `CheckProvider`; only the Run linking goes through `pkg/hourglass/server`.

## Confidence and gaps
- Never run against a real vCenter; the vsphere provider is only linked into the binary and registered. Pressure smoothing and a hung extension (timeout) are untested. `ui/pnpm-lock.yaml` SPDX failure and root BUILD/MODULE changes belong to other agents.
