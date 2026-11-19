# Result: 05 metering (done)

Tests: 10 test functions, 80 table cases executed (counted by TestMain). Pass under `go test ./internal/metering/...` and `bazel test //internal/metering/...` (1 of 1). `./tools/check_spdx.sh` exits 0. Log: bazel.log.

Files: internal/metering/{metering.go,metering_test.go,BUILD.bazel} (BUILD has hand-added SPDX header, as in task 02). Stdlib only; no other Hourglass imports.

## Exported API
Consts `MilliPerToken=1000`, `DefaultWindow=15m`; types `Step{From,Multiplier}`, `Bands{Base,Steps}`, `Sample{At,Raw}`;
`DefaultBands()`, `(Bands).Validate() error`, `.Multiplier(p)`, `.Max()`; `ValidPressure(p)`;
`Smooth(samples, now, window) (float64,bool)`; `DebitMT(weight, mult, elapsed) int64`;
`ChargeableElapsed(prev, now, poll) (time.Duration, bool)`; `HourlyCostMT(weight, mult) int64`;
`ProjectedRunOut(balanceMT, burn24hMT, now) (time.Time,bool)`.

## Decisions for docs/decisions.md
- DebitMT returns 0 for NaN/Inf weight or multiplier and clamps to MaxInt64 on overflow.
- ProjectedRunOut returns false (no projection) when the run-out is more than 100 years away, to avoid time.Duration overflow; the caller treats that as "not this month". Burn <= 0 is checked before balance, so balance <= 0 with no burn gives false.
- Multiplier assumes validated (ascending) bands; Max includes Base.
- Validate rejects NaN From values and a non-ascending or equal From.
- Smooth does not filter non-finite Raw values; callers should gate with ValidPressure first.

## Confidence and gaps
Verified by running the tests. Linux and clean-cache Bazel not checked. Spec says "50% to 75%" for 1x; implemented inclusive lower bounds as the brief states.
