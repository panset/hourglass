# 12b Fix the findings of verification 12: result

Status: done. `./ci.sh` exit 0 (`ci.log`, `ci.exit`; 40 of 40 targets pass).

## Findings and what closed them
- Low 1 (no test of `reported_weights_enabled`): new `TestUI12_SetupsListReportsWhetherReportedWeightsAreOn` in `internal/api/weights_readers_test.go`. For a developer and an admin it requires true with the setting on, false off, true on again. Mutation: `handlers_user.go:885` hard-coded to `true` turned it red ("setting off, developer ... = true, want false"; `mutation.log`); restored, `git diff` of that file is empty.
- Low 2 and 3 (`docs/integrating.md`): attributes are described as returned and shown while the entry is usable (not expired, within limits), also under an override; the `attributes` bullet of the state section says the same; the setting-off bullet now says `source` is `override` for a setup with an override, otherwise `rule` or `default`, and `attributes` is `{}`; the override bullets say `reported` still shows the stored weight. Checked against `internal/weights/weights.go:121-150` (override before the switch, attributes set before the override check, `not_applied` override) and D85/WGT-18 tests; no server started.
- Low 4 (`docs/deploying.md:216`): now says Admin, Setup inventory and the state route show `not_applied` `expired`.
- Low 5 (`docs/progress.md`): the sentence now names `TestUPG2_...` and its extension for the added fields (D85(l)); it now points to D90 as the last decision.
- Medium 1 (spec edits row): drafted in `SPEC_EDITS_ROW.md`, one line, no pipes. Body only; the orchestrator adds the other cells. Suggested: date 2026-11-23, why "The spec is generated from a source outside this repository's control and is never edited here; the owner applies these.", who orchestrator.
- Info items: no change (8 extending.md clause, 9, 10, 11 are not in my scope or need none; item 7, D88(c), is the orchestrator's decisions file).

## Files changed
internal/api/weights_readers_test.go
docs/integrating.md
docs/deploying.md
docs/progress.md
.orchestrator/reported-weights/tasks/12b-fix-m7/{BRIEF.md (given), PROGRESS.md, RESULT.md, SPEC_EDITS_ROW.md, mutation.log, ci.log, ci.exit}

## Commands
`go test ./internal/api -run TestUI12_SetupsList` (green, red with mutation, green restored); `./ci.sh` exit 0.

## Decisions for docs/decisions.md
None, except adding SPEC_EDITS_ROW.md as D90 (as the orchestrator planned).

## Requirement -> test
UI-12 / WGT-14 (field value): TestUI12_SetupsListReportsWhetherReportedWeightsAreOn.

## Confidence and gaps
- Spec edits: SPEC.md has no service-accounts scope list, integration routes, report tables or Service accounts page; items 20, 26 and 13 to 14 say so and name where to add. Section names for the rest were taken from SPEC.md headings.
- Item 7 of the verification (D88(c)) not touched: decisions.md is off limits.
- The docs corrections were checked by reading code and tests, not by a new server run.
