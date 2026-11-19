# 32b fix vSphere verification findings

Status: done. Changes only in `providers/vsphere/` and this folder.

## Finding -> fix
- Finding 2 (silent empty list on unknown datacenter/cluster): `vsphere.go` `provider.List` now returns `vsphere: cluster %q not found` when a configured cluster has no pool entry (root pools count, so a cluster with only its root is a valid empty result). `govmomi.go` `clusters()` returns `datacenter %q not found` when a configured datacenter is missing. Tests: layer 2 `TestEXT3_ProviderUnknownClusterIsError` (vsphere_test.go); layer 3 `TestEXT3_SimulatorUnknownScopeIsError` (simulator_test.go, provider and pressure source, plus the valid-empty case). `TestEXT3_SimulatorScope` no longer asserts the silent behaviour.
- Finding 4 (lower clamp untested): `TestMET1_LowerClampOnRatios` tests `reservationRatio`, `storageRatio`, `clamp(NaN)` and `clusterPressure` directly. Reason it was green before: the pressure loop starts `highest` at 0, which hides negatives, so the clamp is only observable on the helpers.
- Finding 5 (weak TLS assertion): now fails if the error contains the password "pass" or "user:".
- Finding 3 (password via by-value print): `provider` and `pressureSource` String/GoString now use value receivers (cover `*p` as well as `p`), plus `LogValue` on both and on `config` (and `GoString` on `config`). Test `TestEXT3_PasswordNeverPrintedByValueOrLog` covers %v %+v %#v %s and slog text/JSON for values and pointers.

## Commands
- `go test -race -count=1 ./providers/vsphere/...`: ok. `bazel test //providers/vsphere/...`: passed. `tools/check_core_generic.sh` and `tools/check_spdx.sh` (bash): OK.

## Confidence and gaps
- I did not re-run the mutation of removing the lower clamp; the new test targets the helper that mutation changes (inference that it now goes red).
- A real cluster with no root pool (not seen in practice) would now be reported "not found" by the provider.
