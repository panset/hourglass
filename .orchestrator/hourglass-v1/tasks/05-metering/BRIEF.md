# 05 internal/metering
Job intent: .orchestrator/hourglass-v1/INTENT.md (read it first). Rules: .orchestrator/hourglass-v1/CONTRACT.md.
## Goal
Package `internal/metering`: pressure bands, smoothing and debit arithmetic as pure functions with unit tests. Spec: docs/SPEC.md "Burn rate", MET-3, MET-4, "Data model" first paragraph (milli-tokens), and "Projected run-out" under Dashboard pages.
## Behaviour
- Bands: `Bands{Base float64; Steps []Step}`, `Step{From float64; Multiplier float64}` meaning "pressure >= From gets Multiplier"; below the first step the multiplier is Base. `DefaultBands()` = base 0, steps 0.50→1, 0.75→2, 0.90→3. `(Bands).Validate()`: steps strictly ascending, each From in (0,1], multipliers and base non-negative and finite, at least one step not required. `(Bands).Multiplier(pressure float64) float64`; lower bounds are inclusive (0.50 → 1, 0.75 → 2, 0.90 → 3, 1.0 → 3). `(Bands).Max() float64` = the highest multiplier (the UI's "at the highest band").
- `ValidPressure(p float64) bool`: finite and within [0,1].
- Smoothing: `Sample{At time.Time; Raw float64}`; `Smooth(samples []Sample, now time.Time, window time.Duration) (float64, bool)` = arithmetic mean of Raw for samples with now-window < At <= now; false when there are none. Default window const 15 minutes. Order of input must not matter.
- Milli-tokens: `const MilliPerToken = 1000`; `DebitMT(weight, multiplier float64, elapsed time.Duration) int64` = weight × multiplier × elapsed hours × 1000, rounded half away from zero to the nearest milli-token; zero or negative inputs give 0. Spec examples must hold: weight 1, 1×, 5 min → 83 mt; weight 1 for 15 h at 0×/1×/3× → 0 / 15000 / 45000 mt.
- `ChargeableElapsed(prev, now time.Time, pollInterval time.Duration) (elapsed time.Duration, outage bool)`: prev zero (no earlier successful poll) → 0,false; gap <= 2×pollInterval → gap,false (exactly 2× is charged); gap > 2× → 0,true (MET-4: longer gaps are not charged and are recorded as outages); now <= prev → 0,false.
- `HourlyCostMT(weight, multiplier float64) int64` (cost per hour shown in the UI).
- `ProjectedRunOut(balanceMT int64, burnLast24hMT int64, now time.Time) (time.Time, bool)`: average hourly burn = burnLast24hMT/24; false when burn <= 0 or balance <= 0... specifically: burn <= 0 → no projection (false); balance <= 0 → now, true. The caller compares with month end.
## Scope
May write: `internal/metering/`, this task folder.
## Build notes
The skeleton from task 02 is in place (see tasks/02-build-skeleton/RESULT.md for versions). Standard library only: add no dependency and do not edit go.mod, MODULE.bazel or root files. Generate your BUILD file with `bazel run //:gazelle -- <your package dir>` and test with both `go test ./<dir>/...` and `bazel test //<dir>/...`. Two other agents are building sibling packages at the same time; Bazel commands in one workspace queue behind each other, so allow up to 5 minutes per Bazel command and never kill the Bazel server. The package must not import any other Hourglass package, and must not mention vCenter, vSphere, sizes (medium/large/...) or any concrete extension outside test data that uses neutral labels (e.g. `tier=a`, `site=x`).
## Tests
Table-driven, named with requirement ids (`TestRULE2_...`). Cover every rule in "Behaviour" including error cases and boundaries. Report the number of test cases executed.
## Done when
Tests pass under go and Bazel, `./tools/check_spdx.sh` passes, RESULT.md lists the exported API and decisions.
