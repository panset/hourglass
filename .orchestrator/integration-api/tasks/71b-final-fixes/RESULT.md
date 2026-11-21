# 71b result: done
- CMP-2: `tools/ci_test.go` (`TestCMP2_CIRunsEveryRequiredStep`), `tools/BUILD.bazel`, root `BUILD.bazel` exports `ci.sh`. Bites: see mutation.log (license step removed, failed, restored).
- docs/decisions.md: D51 to D53 columns fixed, D53 Why written; every row D42 to D63 has five cells and agent/owner in By (script check).
- D60 amended and marked REVIEW with cases (a) to (c), each checked against internal/holds/holds.go requestRelease (emit inside Tx; async dispatcher; ownerErr skips emit). Added to docs/progress.md table.
- README.md: paragraph under Documentation.
- `./ci.sh` exit 0 (ci.log, ci.exit): 38 Bazel tests pass, //tools:tools_test executed, the rest cached.
## Confidence and gaps
Case (b) is inferred from the code structure (emit inside the transaction, delivery asynchronous), not observed.
