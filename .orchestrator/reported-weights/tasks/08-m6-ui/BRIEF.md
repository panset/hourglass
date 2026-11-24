# 08 Milestone 6: the UI (UI-12, UI-13, UI-14)
Job intent: .orchestrator/reported-weights/INTENT.md (read it first). Contract: .orchestrator/reported-weights/CONTRACT.md (binding).
## Goal
Design section C3 of docs/design/reported-weights.md, every bullet, on the five page areas it names, with tests named after UI-12, UI-13 and UI-14. Every number comes from the API (UI-1): the UI computes no weight, cost or range itself beyond formatting.
## Context
The API is complete and committed (milestones 2 to 5, verified). The types are in `ui/src/api/schema.gen.ts`: do not edit it or `api/openapi.yaml`. If a field C3 needs is missing from the API, stop and report it; do not work around it. Read docs/ui-notes.md for the page conventions and how the earlier work (UI-8 to UI-11, D73, D74, D77) built and tested pages; follow the existing components and styles; no new dependency.
## Build
- Setups (`ui/src/pages/setups`): weight cell with a small tag when the source is `reported` or `override`; attributes as one secondary line (`memory-gb 512 · storage-gb 4000 · tier gold`), not columns, not filters. Acquire dialog: weight, attributes, cost as today, plus one sentence whenever `reported_weights_enabled` is true, whatever the setup's source: weights are reported by automation and the price can change while the setup is held.
- My usage (`ui/src/pages/usage`): hold row weight with the same tag (a hold's source may be `policy`, or null for an old released hold); burn line unchanged in form; a ledger debit shows its weight after the multiplier: one value when `weight_min` equals `weight_max`, otherwise a range such as `1.15 to 1.40`, with the source when it is not `rule` or `default`, and nothing when the weight is unknown (null).
- Admin, Setup inventory (`admin-setups`): weight and source, the reported weight when there is one, in words why a reported weight is not applied (`expired`, `out_of_bounds`, `override`); the setup form gains a weight override field that can be cleared (send `null` to clear, omit or keep when untouched, per design D2).
- Admin, Settings (`admin-settings`): a "Reported weights" section: on/off, minimum, maximum, maximum age; when the last weights report arrived and from which account; a warning when stored weights are outside the limits, with the count; a warning when a report was refused after the last accepted one, with the refusal's time, account and message; the note that the maximum age should be at least twice the poll interval; turning the switch on without a maximum shows the server's validation message. The page must keep sending the other settings as it does today.
- Admin, Service accounts: the scope list gains `weights:write`.
- docs/ui-notes.md: add what the earlier work added for its pages.
## Tests
Component tests in the style of the existing page tests, named with the ids (`UI-12 ...`, as the existing `UI-11` tests are named): each C3 bullet has an assertion; cover source tags for each source, attributes line, the dialog sentence present and absent, single value / range / unknown ledger weight, each not-applied wording, set and clear override (assert the request body), settings save body and each warning, the server's validation message shown, the fifth scope. Existing assertions that pin exactly four scopes may be extended to five (additive, design D3); list them. Do not weaken any other existing test.
## Scope
May write: `ui/src/**` except `ui/src/api/schema.gen.ts`, `docs/ui-notes.md`, `ui/BUILD.bazel` only if a new file must be listed, this task folder. Must not touch: Go code, `api/openapi.yaml`, `e2e/`, other docs.
## Done when
`cd ui && npm run typecheck && npm test` pass and `./ci.sh` exits 0; RESULT.md has the requirement-to-test table, the list of user-visible strings added, and your decisions.
## Long-running steps
`./ci.sh`: 30 minute timeout, log into this task folder.
## Report
Per CONTRACT.md "Reporting".
## Note
Another agent is editing `internal/api` Go files at the same time (a small fix). Do not touch Go code. If `./ci.sh` fails only in a Go target you did not touch, rerun it once after a few minutes and report what you saw.
