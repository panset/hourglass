# 07b result: state route never fails on a rate rule that cannot be evaluated
Status: done. `./ci.sh` exit 0 (ci2.log). First run exit 3 with only `//ui:test` failing (ui work by another agent); the rerun passed.

## What changed
- `internal/api/admin_views.go`: new `resolveLenient` (the admin views' fallback, moved out of `adminState.resolve`, which now calls it). No second fallback.
- `internal/api/handlers_integration.go`: the state route calls `resolveLenient(s, l.rules, l.set, report, l.now)` instead of `l.resolve`; a bad rule gives the default weight for the rule part, an override or usable reported weight still applies, 200 always.
- `internal/api/weights_report_test.go`: tests below (gofmt also applied to this file).
User routes keep `l.resolve` and still fail the request (D84/D85(b)).

## Files changed
internal/api/admin_views.go
internal/api/handlers_integration.go
internal/api/weights_report_test.go

## Tests (both seen red with 500 first, green now)
| id | test |
| --- | --- |
| WGT-13 | TestWGT13_StateAnswersWithARuleThatCannotBeEvaluated (feature on: reported, override, default; expired report) |
| WGT-20 | TestWGT20_StateAnswersAsBeforeWithARuleThatCannotBeEvaluated_FeatureOff |
No existing test asserted the 500, so none was replaced.

## Decisions for docs/decisions.md
D84(g) is superseded: the state route handles an unevaluable rate rule as the admin views do (D85(b)); only the user views fail the request. Update the D85(b) sentence "the user views and the state route fail" and the comment/REVIEW mark accordingly.

## Confidence and gaps
`go test ./internal/...` passes. The doc comment on `lab.resolve` (user_views.go) still says it fails the request, which stays true for user views.
