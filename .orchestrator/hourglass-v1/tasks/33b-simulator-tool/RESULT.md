# 33b e2e simulator tool: result

Status: done. `e2e/simulator` builds `hourglass-vcsim`: the govmomi simulator in-process (TLS, self-signed cert, 1 datacenter, 1 cluster, N hosts, no standalone host) plus a plain-HTTP JSON control API.

## How the figures are set
Mutating the simulator's objects works (the spike's untried idea). `PUT /pressure` calls `simulator.Registry.AtomicUpdate` on the cluster root pool with `runtime.cpu.reservationUsed` and `runtime.memory.reservationUsed` = round(fraction * maxUsage), and, for each cluster datastore, `Context.Update` of `summary.freeSpace` (plus `Info.FreeSpace`) = round(capacity * (1 - fraction)). maxUsage and capacity are read first through the property collector. All pool and pressure reads/writes the control API makes go through a real SOAP login to its own server (sim.go `connect`); only the mutation itself touches the simulator's object map (sim.go `SetPressure`). `maxUsage` stays as the model has it (CPU 4121 MHz, memory about 1 GB), so a CPU fraction reads back within about 1e-4 of the requested value, not exact.

## Flags / env
| Flag | Env | Default |
|---|---|---|
| `-soap-listen` | VCSIM_SOAP_LISTEN | 0.0.0.0:8989 (TLS) |
| `-control-listen` | VCSIM_CONTROL_LISTEN | 0.0.0.0:8990 (HTTP) |
| `-username` | VCSIM_USERNAME | user |
| `-password` | VCSIM_PASSWORD | pass |
| `-hosts` | | 3 |
SIGTERM or SIGINT shuts down gracefully. Logs are one JSON object per line (slog) on stdout. Dependencies: govmomi and the standard library.

## Control API
```
curl localhost:8990/healthz                                   # {"status":"ok"}
curl localhost:8990/pools                                     # {"pools":["lab-01_RP"]}  (direct children of the cluster root pool)
curl -XPOST localhost:8990/pools -d '{"name":"lab-01_RP"}'    # 201 {"created":true}; 200 {"created":false} if it exists
curl -XDELETE localhost:8990/pools/lab-01_RP                  # 204; 404 if absent
curl -XPUT localhost:8990/pressure -d '{"cpu":0.8,"memory":0.2,"storage":0.1}'   # omitted fields unchanged; returns the readback
curl localhost:8990/pressure                                  # {"cpu":0.80005,"memory":0.2,"storage":0.1}
```
Fractions outside 0..1 return 400; unknown JSON fields return 400. The provider config for the stack: `url: https://<host>:8989/sdk`, the username and password above, `insecure_skip_verify: true`.

## Tests (e2e/simulator/sim_test.go)
Start the tool in-process, build the REAL `vsphere` provider and pressure source via `hourglass.NewProvider/NewPressureSource("vsphere", cfg)`: pools created through the control API are listed (create idempotent), deleted pools disappear; cpu 0.8 gives pressure 0.8; storage 0.95 then wins; setting memory and storage back down returns 0.8; repeated reads stable; out-of-range rejected.

## Files
e2e/simulator/{main.go, sim.go, control.go, sim_test.go, BUILD.bazel}; this folder. No go.mod change.

## Commands
- `go test -race -count=1 ./e2e/simulator/...` ok; `go build ./e2e/...` ok; `bazel test //e2e/simulator/...` PASSED; `tools/check_core_generic.sh` OK; `tools/check_spdx.sh` OK.
- Manual smoke of the built binary: POST pool, PUT pressure, SIGTERM exits 0 with "stopped" log.

## Decisions for docs/decisions.md
- DELETE of an unknown pool returns 404 (not silent success). Pools are only managed directly under the cluster root pool.
- Baseline storage fraction is about 0.0065 (model datastore), not 0.
- Bazel binary target is `//e2e/simulator:hourglass-vcsim`; there is no container image or compose wiring here.

## Confidence and gaps
- Not run in a container or on linux/amd64; WaitForUpdates subscribers are notified via Update but not tested. Multi-cluster models are not supported (the first cluster is used).
