# 15b fix extension findings: RESULT

Status: done. `go test -race -count=1 ./pkg/... ./pressure/... ./policies/... ./providers/... ./hooks/...` and `bazel test` of the same six targets pass (hourglasstest 2.2 s with -race, 1.3 s under bazel); both tools scripts exit 0.

## Findings
1. FIXED. `tools/check_core_generic.sh` now takes imports from `go list -e` (Imports, TestImports, XTestImports) and locates file:line by searching the package's Go files. Seeded proof, each caught with `path:line: rule1|rule3`: blank `_`, dot, aliased, plain single-line, one-line parenthesised, `( "fmt"; _ "x" )`, in a `_test.go` and in an xtest (rule 1, `internal/zz`); blank, aliased, one-line paren (rule 3, `cmd/zz`). `providers/vsphere/zz` importing govmomi still passes. Seeds removed. Known limit: files excluded by build tags are not seen by `go list`.
2. FIXED. Speed-up patch applied (only the asserted check runs, `t.Parallel()` on the four Broken* tests); no assertion removed, no exported signature changed.
3. FIXED. NaN, +/-Inf or negative weight returns an error, as does a finite total that overflows. Test `TestEXT3_InvalidWeightIsError`.
4. FIXED. Allocation trigger fires on `HoldState.Allocation != nil` OR the snapshot allocation. Test `TestMET7_AllocationEitherSourceSuffices` (neither/snapshot/hold-state/both).
5. FIXED. Body read is `200+len(secret)` bytes, redacted, then cut to 200. Test sweeps every straddle offset.
6. FIXED. Transport errors are "timeout", "canceled", "TLS certificate error", "connection failed" or "request failed"; no URL or host:port. Status path unchanged. Test `TestEXT6_TransportErrorsAreGeneric`.
7. LEFT, as instructed (compaction by encoding/json accepted; semantically unmodified; `pkg/hourglass` out of scope).
8. FIXED by documentation: package doc says an explicit empty `types` list sends nothing.
9. FIXED. Empty or `null` config gives the `"path" is required` message (static and webhook); a file containing `null` is an error. Test `TestEXT3_NullAndEmptyConfigAndFile`.
10. FIXED. With `start_after` 0 and clock skew the reason is "waiting for the acquire time". `-0` still parses as 0 (harmless). Test `TestMET7_ClockSkewReasonIsSensible`.
11. LEFT. Rule 2 is a plain case-insensitive substring by the brief's design; a word boundary would be a spec change and could miss real hits. Whitespace and zero-width variants evading a grep is expected.
