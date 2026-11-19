# Result: 30 providers/vsphere

Status: done. Package `providers/vsphere` registers the `vsphere` provider and the `vsphere` pressure source (EXT-3, MET-1, D6, D7). All logic is written against the small internal `client` interface; `govmomi.go` is the only file importing govmomi.

## Files changed
- providers/vsphere/: client.go (interface and figure types), config.go, vsphere.go (factories, shared client, provider, pressure), govmomi.go (adapter), fake_test.go, vsphere_test.go (layer 2), simulator_test.go (layer 3), readonly_test.go, BUILD.bazel
- go.mod, go.sum (govmomi v0.56.2 and yaml.v3), MODULE.bazel (`use_repo` entry). MODULE.bazel.lock did not change.
- this folder: PROGRESS.md, RESULT.md

## Config schema (both extensions, same block, unknown fields rejected)
| Field | Type | Default | Notes |
|---|---|---|---|
| `url` | string | required | http(s) URL, no embedded credentials; empty path becomes `/sdk` |
| `username` | string | required | |
| `password` | string | env `HOURGLASS_VSPHERE_PASSWORD` when empty | required after fallback; redacted everywhere |
| `insecure_skip_verify` | bool | false | |
| `datacenter` | string | all datacenters | optional |
| `clusters` | []string | every cluster in scope | duplicates dropped |
| `request_timeout` | duration string | `20s` | must be positive; applies per call |

Provider and pressure source built from equal url, username, password, datacenter, insecure and timeout share one client and one session (`sharedClient`, vsphere.go). `clusters` is scope only and does not open a second session. The client connects lazily, so building an extension never touches the network.

## Which assertions live where
| Assertion | Layer 2 fake (vsphere_test.go) | Layer 3 simulator (simulator_test.go) |
|---|---|---|
| Pool listing, root excluded, labels | TestEXT3_ProviderListsPoolsExcludingRoot, ...NestedPoolsListedByOwnName | TestEXT3_SimulatorProviderListsPools (parent, path, cluster, datacenter, root excluded) |
| Nested pools; duplicate names across clusters; scope by cluster | yes | nested (child_RP under lab-01_RP); scope by datacenter and cluster in TestEXT3_SimulatorScope |
| Pool created / destroyed shows next call | yes (fake edited) | yes (real create and Destroy) |
| Non-zero CPU and memory ratios; max-of-three (each wins); multi-cluster max; 0.5/0.75/0.9 crossings; zero-capacity error; clamp; inaccessible datastores; no clusters; missing named cluster | yes (TestMET1_*) | no (D5, D7) |
| Pressure in [0,1] and equals storage ratio from datastore figures within 1e-9; CPU and memory read 0 | no | TestMET1_SimulatorPressureEqualsStorageRatio |
| Errors propagate, context cancellation | yes | timeouts via request_timeout not separately tested |
| Config parsing, env fallback, secret redaction (config, key, provider, pressure, errors) | yes | wrong password does not appear in error |
| TLS: self-signed rejected without `insecure_skip_verify`, accepted with it | no | TestEXT3_SimulatorTLS (provider and pressure) |
| Re-login after session ended; Close idempotent | no | TestEXT3_SimulatorReloginAfterSessionTerminated (both read paths) |
| Shared client, registration | yes | |
| No mutating calls in product code | readonly_test.go (AST scan of govmomi.go and vsphere.go) | |
| `hourglasstest.TestProvider` and `TestPressureSource` | yes (TestEXT5_*) | |

## vSphere privileges the adapter needs (first-run checklist)
The built-in Read-only role (System.Anonymous, System.View, System.Read), propagated, on the in-scope datacenter(s), clusters, their resource pools and datastores. Calls made: SessionManager.Login/Logout, SearchIndex/inventory lookups through the Finder, and PropertyCollector retrieval of cluster `name/resourcePool/datastore`, resource pool `name/resourcePool/runtime`, datastore `name/summary`. Inference: no extra privilege such as Resource.* or Datastore.* is needed; confirm on a real vCenter.

## Decisions for docs/decisions.md
1. The `client` interface returns datastores as a list (name, capacity, free, accessible) and root pools flagged `Root`, rather than pre-summed figures; the logic (accessible filter, sums, root exclusion) then sits in layer-2-testable code. Pools also carry cluster and datacenter. Parent of a first-level pool is the root pool's name, `Resources`.
2. Clusters are found with the Finder pattern `*` per datacenter, so only direct children of the host folder are seen; clusters inside host sub-folders are not found (the `...` pattern found nothing in the simulator). Gap to fix if a lab uses folders.
3. A cluster named in `clusters` that does not exist is an error for the pressure source (fails the poll); the provider lists nothing for it.
4. A datastore with accessible=true but capacity 0 is skipped; a negative or zero root-pool maxUsage is a zero-capacity error. NaN is clamped to 0.
5. Shared sessions live for the process lifetime (the extension interfaces have no Close); the adapter logs out through `Close`, used by tests.
6. Re-login: on a NotAuthenticated fault the adapter opens a fresh SOAP client, logs in and retries the call once; the retry error is returned as is.
7. The config `url` may not contain credentials; `request_timeout` is a Go duration string.
8. `fmt` prints unexported struct fields without calling `String` on them, which leaked the password into `%v` of the provider; provider, pressure source and adapter therefore define `String`/`GoString`.
9. Resource pool walking uses property retrieval (`resourcePool` child references), not container views, so no view is created or destroyed.

## Commands and outcome
- `go test -race -count=1 ./providers/vsphere/...` ok (about 5 s)
- `bazel run //:gazelle -- providers/vsphere` ok; `bazel test //providers/vsphere/...` PASSED (first govmomi build took about 1 minute)
- `go build ./...` ok; `tools/check_core_generic.sh` OK; `tools/check_spdx.sh` OK

## Confidence and gaps
- Never run against a real vCenter; privilege list and the `NotAuthenticated` fault shape on a real server are inference from the simulator and the govmomi API.
- Re-login test uses the simulator's session logout; a server-side expiry should produce the same fault (inference).
- Host sub-folder clusters unsupported (decision 2). Standalone (non-cluster) hosts' pools are ignored by design.
- Provider scope by cluster uses the pool's cluster label, so a missing cluster is silent.
