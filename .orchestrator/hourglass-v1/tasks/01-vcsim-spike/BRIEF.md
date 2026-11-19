# 01 govmomi simulator spike
Job intent: .orchestrator/hourglass-v1/INTENT.md (read it first). Rules: .orchestrator/hourglass-v1/CONTRACT.md.
## Goal
Establish, by running code, what the govmomi in-process simulator (`github.com/vmware/govmomi/simulator`) supports for the two things Hourglass's `vsphere` extension needs, so the test layers in docs/SPEC.md "Testing without a vCenter" can be confirmed or adjusted.
## Questions to answer with evidence
1. Can a test create and destroy resource pools (including nested ones, and ones named like `lab-01_RP`) under a cluster's root pool, and does listing pools (property collector / container view / finder) return them with name and parent?
2. For a ClusterComputeResource, which CPU, memory and storage capacity and allocation/usage figures does the simulator report, and are they non-zero and stable? Check at least: cluster `summary` (totalCpu, totalMemory, effectiveCpu, effectiveMemory, and `usageSummary` if present), the root resource pool's `runtime.cpu` / `runtime.memory` (maxUsage, reservationUsed, overallUsage, unreservedForPool), per-pool `config.cpuAllocation/memoryAllocation` reservations, and datastore `summary.capacity` / `freeSpace` for the cluster's datastores.
3. Do those figures change when pools are created with reservations, or when VMs are added / powered on? I.e. can a test drive the "allocation ratio" across a threshold through the simulator, or only by a fake?
4. Is there an official container image for the simulator (vcsim) usable in docker compose, and can resource pools be created in it from outside (govc)? State image name and how you verified (do not need to pull it if Docker pulls are slow; say so).
5. Which govmomi version did you test, its license, and its Go version requirement.
## Scope
May write: this task folder, and a throwaway Go module under /private/tmp/claude-501/-Users-tejaspanse-projects/17dec24c-e7ec-455e-9b87-01d2217d5fc9/scratchpad/vcsim-spike/. Must not touch anything else in the repo. Copy the final spike test file into this task folder as `spike_test.go.txt`.
## Done when
RESULT.md answers 1-5 with the observed values, and ends with (a) a recommended definition of each of the three ratios (CPU, memory, storage) in terms of concrete vSphere properties that a read-only account can read, (b) a table: which assertions the layer-3 simulator tests can make vs. which must be carried by the layer-2 fake, and (c) a ready-to-paste entry for docs/decisions.md (5-10 lines).
## Long-running steps
`go mod tidy` / first `go test` download govmomi: allow 5 minutes.
