# 05 Milestone 4: the scope, the report route and the state fields (WGT-7 to WGT-11, WGT-13, WGT-21)
Job intent: .orchestrator/reported-weights/INTENT.md (read it first). Contract: .orchestrator/reported-weights/CONTRACT.md (binding).
## Goal
Design sections B1, B2, B3, B4 and D3 of docs/design/reported-weights.md, implemented and tested over HTTP, with `api/openapi.yaml` and the generated UI types updated. You are the only task editing those two files now.
## Context
Committed so far: milestone 2 (storage, `internal/reported` weights report, the setting) and milestone 3 (resolver `internal/weights`, collector). Read the exported APIs and decisions in `.orchestrator/reported-weights/tasks/02-m2-storage-settings/RESULT.md` and `.../03-m3-resolver-collector/RESULT.md`, the fix results beside them if present (`04b-*`), and D82 onward in docs/decisions.md (entry positions count from 0; `reported.EntryMessage` is the one message format). Test-layout pointers: `.orchestrator/reported-weights/tasks/00-digest/RESULT.md`.
## Build
1. `internal/auth`: scope `weights:write`, added to `Scopes()` after `holds:release` (D3). No existing account gains it.
2. `PUT /api/v1/integration/weights` (B1, B2, B3): every bullet. In particular: check order scope, feature on, body, ordering (D53); off answers 409 `not_configured` whatever the body, with a message that names the `reported_weights` setting and says an admin turns it on; entries decoded as raw JSON and checked one by one so that a wrong type, an out-of-range number, a number too large for a float64 or an unknown field inside an entry gives 400 `validation` with `field` `weights` and the position, the setup name when it is a non-empty string, and the reason; the first invalid entry is reported; body not valid JSON gets the existing "not valid JSON" 400; missing or null `weights` is 400; limits 10,000 entries and 1 MB; response `{observed_at, received_at, expires_at, accepted, unknown}`; the last 400 refusal recorded for admins (time, account, message); nothing stored on any refusal; never triggers a poll, debit or event.
3. `GET /api/v1/integration/state` (B4): the top-level `weights` object and the `weight` object on every setup, from the resolver. Added fields only. No rate rule text.
4. Route table, OpenAPI (`x-roles: service`, the new scope, schemas, error responses), `cd ui && npm run gen:api`. The UI scope checkbox list and pages are milestone 6: only make the smallest fix if generated types break the UI build, and list it.
5. Tests that pin the list of exactly four scopes may be extended to five; that is the additive requirement D3, not a weakened test. List each such edit.
## Tests (named after ids; each must fail when the requirement is broken: break the code once per id, see red, restore)
- WGT-7: whole-set atomic replace over HTTP; optional `observed_at`; stale 409 `stale_report`, equal replaces, more than 2 minutes ahead 400; independent of the deployments report's `observed_at`; survives a restart (reopen the database).
- WGT-8: one test case per clause of the requirement (weight missing, string `"1.15"`, null, boolean, `1e400`, below min, above max, exactly min and max accepted, negative zero accepted as 0; name missing, not a string, empty, repeated; attributes: 17 entries, bad key, value of 64 characters, non-string value; unknown field in an entry), each asserting the position, the name when there is one, that nothing was stored and that the previous report is still in force. Bare `NaN` gets the not-valid-JSON answer.
- WGT-9: unknown name ignored, listed in `unknown`, changes no weight.
- WGT-10: off answers 409 `not_configured` naming the setting, for a valid and for an invalid body, and stores nothing.
- WGT-11: the permissions matrix covers the route for every principal (every user kind, no credentials, a service account with every other scope and not this one gets 403 `forbidden`, an account with only `weights:write` is refused on every other route). Extend `TestSVC3_...` and the matrix (`permExtraCover` or a `capMatrix` row) and add `TestWGT11_...`.
- WGT-13: every field of B4 for setups priced by override, reported, rule and default, each `not_applied` value and their precedence, feature off (reported and not_applied null for every setup, report times null), no rule selector anywhere in the response.
- WGT-21: a setup created later with a stored unknown name takes that weight; a free setup renamed to a stored name takes it at once and the old name's weight no longer applies to it; switch off then on within `max_age` applies again; raising `max_age` revives an expired report; an override stays with its setup through a rename (set the override through the store; the HTTP input is milestone 5).
- The design's Tests item: tightening the maximum excludes a stored weight at once, refuses the next report whole, and polls keep succeeding (the admin-view half is milestone 5; say so).
- The contract test must pass with the new route validated on a real 200 response.
## Scope
May write: `internal/auth/**`, `internal/api/**`, `internal/reported/**` and `internal/store/**` only where the route needs it, `api/openapi.yaml`, `ui/src/api/schema.gen.ts`, minimal `ui/src` fixes, BUILD.bazel via Gazelle, this task folder. Must not touch: the other readers of design A3 (setups list, admin views, holds, burn, ledger: milestone 5), `internal/collector`, `internal/weights` (report a resolver bug instead of patching around it; fix it only if it is a plain bug and say so), `docs/`, `testdata/`, `e2e/`.
## Done when
`./ci.sh` exits 0; RESULT.md has the requirement-to-test table with the mutation per id, the list of existing assertions you changed and why, and your decisions.
## Long-running steps
`./ci.sh`: 30 minute timeout, log into this task folder.
## Report
Per CONTRACT.md "Reporting".
