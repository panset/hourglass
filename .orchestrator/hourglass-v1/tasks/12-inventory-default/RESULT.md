# Result: 12 pressure/inventory and policies/default

Status: done.

## Files (new)
pressure/inventory/{inventory.go,inventory_test.go,BUILD.bazel}; policies/default/{default.go,default_test.go,BUILD.bazel} (package `defaultpolicy`); this folder: RESULT.md, bazel-test.log. Imports: stdlib and pkg/hourglass (pkg/hourglasstest in tests only). Each registers in init() (`inventory` pressure source, `default` metering policy).

## Config schema
- inventory: no settings. nil, empty, `null`, `{}` accepted; any field, malformed JSON, non-object or trailing data rejected.
- default: `{"start_after": "2h", "start_on_allocation": true}`, defaults exactly those. start_after: Go duration; `"0"`/`"0s"` meter from acquire; `"off"` disables time trigger; negative, empty or unparseable rejected. Unknown fields and wrong types rejected. Both triggers off is accepted.

## Behaviour
| Case | Result |
| --- | --- |
| inventory: active-hold setup weight / enabled setup weight | clamped [0,1]; total 0 gives 0; released holds, holds on setups absent from snapshot ignored; a setup with several holds counted once; allocations ignored |
| default: MeteringStartedAt set | Meter, "metering already started" |
| allocation trigger on and Snapshot.AllocationFor(setup) found | Meter, "resource reported by provider" |
| time trigger on and Time-AcquiredAt >= start_after | Meter, "2h since acquire" (or "metering from acquire" when start_after is 0) |
| otherwise | no meter, "waiting for the provider to report the resource or 2h since acquire" (parts omitted per disabled trigger; "not metering: no trigger is enabled" when both off) |
Weight is always 0. Tests: TestMET7_*, TestEXT4_NoProviderOnlyTimeTrigger, TestEXT3_Pressure, TestEXT1_Factory, plus hourglasstest conformance suites.

## Decisions for docs/decisions.md
1. "Active hold" = ReleasedAt nil. Setups with weight <= 0 or NaN count as 0 in both numerator and denominator.
2. Allocation trigger uses `Snapshot.AllocationFor` (matches the setup's ResourceName).
3. Reason wording as in the table; "metering from acquire" is used when start_after is 0 so the text does not read "0s since acquire".
4. Strict config parsing (DisallowUnknownFields, trailing data rejected); `null` config treated as empty.

## Commands
go vet, `go test -race -count=1 ./pressure/... ./policies/...` pass; gazelle run, SPDX line added by hand; `bazel test //pressure/... //policies/...` PASSED (bazel-test.log); check_spdx.sh and check_core_generic.sh exit 0.

## Confidence and gaps
Reason strings are my wording of the brief's examples. Not tested against the real collector (not built yet). No spec conflicts.
