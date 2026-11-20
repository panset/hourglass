# examples/custom: a third-party Hourglass build

This is the worked example for requirement EXT-5. It is a lab of GPU rigs
that builds its own binary, `labd`, with four extensions of its own, one of
each kind, without forking the core. Everything here imports only
`pkg/hourglass`, `pkg/hourglass/server`, `pkg/hourglasstest` and the
bundled extension packages, exactly as a program in another repository
would. Nothing under `examples/` imports `internal/` (the repository's
`tools/check_core_generic.sh` guards the other direction).

| Package | Kind | What it does |
| --- | --- | --- |
| `filedir` | provider | One allocation per file in a directory; the file name is the resource name. |
| `busiestsite` | pressure source, `busiest-site` | Pressure is the highest held-weight fraction among groups of setups sharing a `site` label. |
| `businesshours` | metering policy, `business-hours` | Meters only inside configured hours; setups labelled `gpu=true` get a weight override. |
| `logfile` | hook | Appends each event as a JSON line, with an event-type filter, and vetoes `hold.acquiring` for setups labelled `maintenance=true`. |
| `cmd/labd` | main | Imports the above plus bundled extensions and calls `hourglass.Run`. |

## How a build fits together

1. An extension package defines a type that implements one interface from
   `pkg/hourglass` and calls `hourglass.Register...` from `init()`, passing a
   factory. The factory receives the extension's config block as JSON, however
   the config file wrote it, and must accept an empty block if it has no
   required settings.
2. The `main` blank-imports every extension the config may name, plus
   `pkg/hourglass/server`, which links the implementation behind
   `hourglass.Run`. See `cmd/labd/main.go`.
3. `config.yaml` selects extensions by `type:` (the registered name) and gives
   each its `config:` block. An unknown name fails startup with the list of
   registered names.

Read the packages in this order; each is short and commented for a reader
learning the API: `filedir`, `busiestsite`, `businesshours`, `logfile`.

## What each extension does

**filedir.** `provider: {type: filedir, config: {dir: data/rigs}}`. `List`
returns one `Allocation` per regular file (dot-files and directories are
skipped), sorted. Creating `data/rigs/rig-a1` "deploys" the setup `rig-a1`; the
core matches an allocation only to the setup with exactly its name. A provider
whose backend names differ from setup names maps them itself in `List`, for
example by returning `strings.TrimSuffix(file, ".rig")` as the `ResourceName`
so that the file `rig-a1.rig` reports setup `rig-a1`. A missing directory is an
error, so the core records a gap instead of charging for a lab it cannot see.

**busiest-site.** `pressure: {type: busiest-site, config: {label: site}}`.
From the `Snapshot` it groups setups by the value of the label, divides held
weight by total weight in each group, and returns the maximum. A lab with one
full site and one empty site reports 1.0, not 0.5. Setups without the label
and setups of weight 0 are ignored.

**business-hours.** Config keys: `timezone`, `start`, `end`, `days`,
`gpu_weight`. Outside the window it returns `Meter: false` with a reason.
Inside it returns `Meter: true`; for a setup labelled `gpu=true` it also sets
`Decision.Weight` to `gpu_weight`, otherwise it leaves `Weight` at 0 so the
rate rules apply. The `Reason` string is what users see on the dashboard.

**logfile.** Config keys: `path`, `events` (optional list of event types).
Observer role: each event in the list becomes one JSON line. Gatekeeper role:
`hold.acquiring` for a setup whose `maintenance` label is `true` returns an
error, which aborts the acquire and shows the message to the user. The veto
does not depend on the `events` filter.

## Build

```sh
go build ./examples/custom/cmd/labd
```

Bazel builds it too: `bazel build //examples/custom/cmd/labd`.

## Run locally with no OIDC

There is no `oidc:` block in `config.yaml`, so the only way in is the local
`admin` account. The session key and the admin password come from the
environment, never from the file. Run from this directory so the relative
paths in the config resolve:

```sh
cd examples/custom
export HOURGLASS_SESSION_KEY="$(head -c 32 /dev/urandom | base64)"
export HOURGLASS_ADMIN_PASSWORD="choose-a-long-local-password"
go run ./cmd/labd -config config.yaml
```

On first start the seed file creates four rigs and one rate rule. `labd` is
API-only (no UI is embedded); to see the dashboard, serve the UI container
beside it. Try it with the API:

```sh
curl -s localhost:8080/healthz
touch data/rigs/rig-a1                       # "deploy" rig-a1; the next poll sees it
curl -s -c jar -H 'Content-Type: application/json' \
  -d "{\"username\":\"admin\",\"password\":\"$HOURGLASS_ADMIN_PASSWORD\"}" \
  localhost:8080/auth/local
curl -s -b jar localhost:8080/api/v1/setups
cat data/events.jsonl                        # events as JSON lines
```

`rig-b2` carries `maintenance=true` in the seed, so acquiring it is refused
by the `logfile` hook.

## Tests and conformance suites

```sh
go test -race -count=1 ./examples/...
bazel test //examples/...
```

Each extension runs the matching `pkg/hourglasstest` conformance suite
(`TestProvider`, `TestPressureSource`, `TestMeteringPolicy`, `TestHook`) plus a
few behaviour tests; the snapshots and policy cases passed to the suites are
built with `hourglasstest.NewSnapshot()`. `cmd/labd/main_test.go` starts the
binary's wiring in-process through `hourglass.Run` with the example config, a
temporary database and a free port, and checks `/healthz`.

To run the suites against your own extension, from its `_test.go`:

```go
func TestConformance(t *testing.T) {
	hourglasstest.TestProvider(t, func() hourglass.ResourceProvider { return newMine(t) })
}
```
