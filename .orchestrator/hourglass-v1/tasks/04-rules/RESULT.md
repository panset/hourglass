# Result: 04 internal/rules (done)

Tests: `go test ./internal/rules/` and `bazel test //internal/rules/...` pass. 137 test cases executed (135 subtests + 2 top-level checks); `./tools/check_spdx.sh` passes.

Files: internal/rules/{labels.go,selector.go,rules.go,rules_test.go,BUILD.bazel}. Stdlib only, no other Hourglass imports.

## Exported API
- `ValidateLabels(map[string]string) error`; consts `MaxLabels=32`, `MaxValueLength=63`
- `Parse(string) (Selector, error)`, `(Selector).Matches(labels) bool`, `Match(selector, labels) (bool, error)`
- `Rule{ID, Kind, Position, Selector, Value}`, `Evaluate(rules, kind, labels) (*Rule, error)`
- `KindRate`, `ParseWeight(string) (float64, error)`, `Weight(rules, labels, default) (weight, selector, matched, err)`
- `MatchCounts(rules, kind, labelSets) (perRule map[int64]int, unmatched []int)`

## Decisions for docs/decisions.md
- Label values: any characters, empty allowed, max 63 characters counted in runes (SET-3 says "characters").
- Selector values are unquoted tokens: may not contain whitespace or `, ( ) = !`; may be empty for `=`/`!=` only; list elements in `in`/`notin` must be non-empty. Selector values are limited to 63 chars.
- `==` accepted as `=`; `in(a)` without a space accepted; a key may be named `in`/`notin`.
- Parse errors read `selector: <problem> at position N` (1-based byte column).
- `Evaluate` returns an error naming the rule when a considered rule has an unparseable selector; rules of other kinds are never parsed.
- `ParseWeight` accepts only digits with an optional single dot (`0`, `0.5`, `.5`, `3.`); rejects signs (including `-0`), exponents, hex, inf/nan, whitespace, overflow.
- `Weight` returns an error (with default weight) if the matching rule's Value is not a valid weight, rather than skipping it.
- `MatchCounts`: perRule has an entry (possibly 0) for every rule of the kind; a rule with an unparseable selector never matches; `unmatched` is an empty non-nil slice when none.

## Confidence and gaps
Verified by running both test runners. Not checked: Windows/Linux; fuzzing of the parser.
