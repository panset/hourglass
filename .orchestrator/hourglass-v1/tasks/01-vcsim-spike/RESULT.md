# 01 vcsim spike: result

Status: done. Evidence: `spike_test.go.txt` (copy of the spike test; run in the scratchpad module `vcsim-spike`, `go test -v -count=1`, PASS, 0.5 s). Model: `simulator.VPX()` with `Cluster = 1` (cluster DC0_C0, 3 hosts, 1 datastore LocalDS_0, 4 VMs).

## 1. Resource pools
- Create under the cluster root pool works (`rp.Create`): `lab-01_RP` (reservation 1000) and a nested `child_RP` under it (reservation 500). Destroy works (`rp.Destroy`), nested first then parent.
- Listing via container view (`ResourcePool`, properties `name`, `parent`, `config.cpuAllocation`) returned: `Resources` (x2: the standalone host's pool, parent ComputeResource, and the cluster root, parent ClusterComputeResource `domain-c28`), `lab-01_RP` parent `ResourcePool:resgroup-27` (cluster root), `child_RP` parent `ResourcePool:resgroup-73` (lab-01_RP). After destroy both are gone from the view.
- Finder: `*/Resources/*` and `*/Resources/lab-01_RP/*` each found 1 pool while they existed and returned "not found" after destroy.
- Simulator behaviour to know (source `simulator/resource_pool.go:468-496`, read, not exercised): destroying a pool with children re-parents the children and moves its VMs to the parent; it does not refuse. So "release refused while the pool exists" is Hourglass logic, not simulator behaviour. Duplicate sibling names are rejected (`resource_pool.go:88`, read only).

## 2. Cluster figures (all stable across two reads)
| Source | Value |
|---|---|
| cluster summary totalCpu / effectiveCpu | 6882 MHz / 6882 |
| totalMemory / effectiveMemory | 12883292160 B / 12883292160 |
| numHosts | 3 |
| `usageSummary` | present but every field 0 (the simulator only allocates an empty struct, `cluster_compute_resource.go:689`) |
| root RP `runtime.cpu` | maxUsage 4121, unreservedForPool 4121, unreservedForVm 4121, reservationUsed 0, overallUsage 0 |
| root RP `runtime.memory` | maxUsage 1007681536, unreservedForPool 1007681536, reservationUsed 0, overallUsage 0 |
| root RP `config` cpu / mem reservation | 4121 / 961 (note: root CPU maxUsage 4121 is below summary totalCpu 6882, so the two do not agree) |
| datastore LocalDS_0 `summary.capacity` / `freeSpace` | 4398046511104 / 4355096838144 (4 TiB, about 1.0% used) |
Capacity values are non-zero and stable. Allocation/usage values (reservationUsed, overallUsage, usageSummary) are zero.

## 3. Do figures react?
No. After creating pools with reservations (1000/500), after powering on all 4 VMs, after creating a VM with 4000 MHz / 4096 MB reservations inside `lab-01_RP` (created and powered on, no error), the cluster summary, root RP runtime figures, root RP config and datastore free space were byte-identical to baseline (see `run.log` lines in the scratchpad). Default VMs report cpuReservation 0 and memoryReservation 0. Child-pool reservations also did not change root `unreservedForPool`. Conclusion: a simulator test cannot drive an allocation ratio across a threshold; that must be done by the layer-2 fake. A simulator test can only assert "returns a number in 0..1 from the modelled figures" (here reservationUsed/maxUsage = 0, storage = about 0.0098).
Possible workaround, not tried: start the simulator server and mutate model objects directly (e.g. through `simulator.Map` with the model's service in-process) to set `Runtime.Cpu.ReservationUsed`. This would test our reading code against values we set, so it adds little over layer 2. Inference.

## 4. Container image
- Image `vmware/vcsim` (tags `latest`, version tag, short commit), defined in `.goreleaser.yml:~139-158` and `Dockerfile.vcsim` of govmomi v0.56.2 (scratch image, entrypoint `/vcsim`, default `-l 0.0.0.0:8989`, port 8989). Verified: `docker manifest inspect vmware/vcsim:latest` returned a manifest from Docker Hub (not pulled; no pull attempted). The goreleaser config builds `--platform=linux/amd64` only, so it runs under emulation on this arm64 Mac (inference from build flags). `ghcr.io/vmware/govmomi/vcsim` is denied, so do not use it.
- Creating pools from outside: the simulator serves the full SOAP API, and `CreateResourcePool` is the same call the in-process test made over HTTP (client connected to `s.URL`, not through shared memory), so any client, including `govc pool.create`, can do it. I did not run govc: it is a separate Go module absent from the module cache and I did not install it. Unverified.
- The container is therefore usable in compose; recommend the layer-4 setup script creates pools through a small Go helper or govc image `vmware/govc`, and pins a version tag, not `latest`.

## 5. Version, license, Go
- govmomi v0.56.2 (fetched by `go mod tidy` 2026-11-19, resolved as latest). License Apache-2.0 (`LICENSE.txt`; Dockerfile header SPDX Apache-2.0). Compatible with an MIT project as a dependency; keep its notice in any distribution of the binary (inference, not legal advice). `go.mod` requires Go 1.25.0; local go 1.27.4 builds it.

## Recommendations

### (a) Ratio definitions, readable by a read-only account (System.Read / View)
- CPU ratio = `ResourcePool(cluster root).runtime.cpu.reservationUsed / runtime.cpu.maxUsage` (allocation view; both MHz). Alternative if the owner prefers demand: `overallUsage / maxUsage`. Cluster `summary.usageSummary.cpuReservationMhz / totalCpuCapacityMhz` is an equivalent, but is empty in the simulator and not guaranteed populated on all vCenter versions (inference).
- Memory ratio = `runtime.memory.reservationUsed / runtime.memory.maxUsage` of the root pool (bytes).
- Storage ratio = over the cluster's `datastore` list: `1 - sum(summary.freeSpace) / sum(summary.capacity)`, skipping datastores with `summary.accessible == false` or capacity 0.
- Pressure = max of the three, clamped to 0..1; across clusters named in config, max across clusters (matches the spec's assumption). Guard maxUsage 0 (return error or skip).

### (b) What layer 3 (simulator) vs layer 2 (fake) carries
| Assertion | Layer 3 simulator | Layer 2 fake |
|---|---|---|
| Provider lists pools by name and parent, incl. nested and `lab-01_RP` style names | Yes | Yes |
| Pool created/destroyed shows up/disappears on next poll | Yes | Yes |
| Name matching against the template, orphans | Yes | Yes |
| Pressure source connects, reads all figures, returns a value in 0..1 | Yes (value is about max(0, 0, 0.0098)) | Yes |
| Storage ratio from real datastore capacity/free | Yes (non-zero, stable) | Yes |
| CPU/memory ratio equals a specific non-zero value | No (always 0) | Yes |
| Ratio crosses a band when pools/VMs are added or powered on | No (figures do not move) | Yes |
| Max-of-three, zero-capacity guard, multi-cluster max | No | Yes |
| "Release refused while pool exists" | Hourglass logic via provider list; simulator does not block destroy | Yes |
| Acceptance path "pressure driven across a band" (layer 4) | Not by the simulator alone; needs a test-only pressure override or a stub endpoint | Yes (layer 1/2) |

### (c) Entry for docs/decisions.md
```
## 2026-11-19 govmomi simulator spike (govmomi v0.56.2, Apache-2.0, Go >= 1.25)
- The simulator creates, nests and destroys resource pools and lists them with name and parent; layer-3 tests can cover provider listing, orphans and name matching.
- It reports static capacity (cluster totals, root pool maxUsage, datastore capacity/free) but never changes allocation or usage figures when pools get reservations or VMs are created or powered on; `usageSummary` is all zeros. CPU/memory ratios therefore read 0 and cannot be driven across a band.
- Decision: layer 3 asserts only wiring and range (0..1, storage ratio non-zero); the pressure formula, thresholds, max-of-three and zero-capacity guard stay in layer-2 fake tests. Layer 4 drives "pressure across a band" through a test-only override of the vsphere pressure reading (owner to review), since the container cannot.
- Ratios: CPU/memory = root pool runtime reservationUsed / maxUsage; storage = 1 - sum(freeSpace)/sum(capacity) over cluster datastores; pressure = max, per cluster then across clusters.
- Container: `vmware/vcsim` (amd64 only; pin a version tag). Pool creation from outside via govc not run here.
```

## Decisions for docs/decisions.md
See (c). The layer-4 override is a proposal that affects the spec's acceptance path ("pressure is driven across a band") and needs an owner decision: spec says the simulator container is used; it cannot move the figures.

## Confidence and gaps
- Verified by running: Q1, Q2, Q3 (single simulator model, VPX, one cluster). Image existence verified by manifest only; not pulled or run. govc pool.create not run. Behaviour of the vcsim binary equals the library (inference).
- Not checked: other model types, `Cluster > 1`, DRS/vSAN datastores, property collector `WaitForUpdates` paging, vCenter-side privileges needed for the properties above.
