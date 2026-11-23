# 79b Fix the findings of verification 79

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` ("the spec" means
`docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/79b-fix-findings/`.

## Goal

Close the six low findings in
`.orchestrator/ledger-labels-onboarding/tasks/79-verify-m4-m5/RESULT.md`.

## Context

Read that RESULT.md in full (file:line, failure scenario and suggested fix per
finding), and design sections A4, A5 and Compatibility. D72 and D73 in
`docs/decisions.md` record the choices so far.

## What to do

1. UI: a failed page request must not silently keep showing the previous page
   as if it were the requested one. Show the page's existing error state for
   the ledger (follow how the page reports other failed loads) and keep the
   controls usable. Test it.
2. UI: when the shown month changes by rollover (not by the picker), the old
   month's cursors must be dropped and the first page shown. Test it under a
   UI-8 name.
3. API: `limit` accepts only decimal digits (`^[0-9]+$`) with a value from 1 to
   200; "+5", "-1", "1.0", " 5" are 400 `validation`. Add the cases to the
   LED-13 validation test.
4. OpenAPI schema name. Customers generate clients from `api/openapi.yaml`, and
   the `/me/ledger` entries changed schema name from `LedgerEntry` to
   `LedgerViewEntry`. Orchestrator's direction: go back to one schema named
   `LedgerEntry` for this route, with `entry_count` added as an optional
   property whose description says it is always present on `GET /me/ledger`
   entries and absent on the admin grant and reset responses. No route's JSON
   changes. Remove `LedgerViewEntry`, regenerate `ui/src/api/schema.gen.ts`
   with the documented command, and adjust the UI alias and the contract test
   (the contract test must still prove the handler always sends `entry_count`
   on `/me/ledger`; do not loosen any other check). If the contract test's
   design makes an optional property impossible without loosening other
   checks, stop and report the options instead.
5. Tests: add a committed LED-13 test that pages with `limit` smaller than the
   number of the user's active holds while polls write between requests, and
   extend the LED-14 newest-balance check to a past period as well as the
   current one.

For any finding you decide not to fix, say why.

## Scope

May write: `internal/api/`, `api/openapi.yaml`, `ui/src/api/schema.gen.ts`
(generated), `ui/src/pages/usage/`, `BUILD.bazel` files Gazelle updates, your
task folder.
Must not touch: `docs/`, `e2e/`, `internal/store/`, git state. Do not weaken
any test.

## Done when

`go test ./internal/...` passes, UI tests and type-check pass, `./ci.sh` exits
0 with the changed tests executed (record `ci.exit`). RESULT.md lists each
finding and what closed it, and "Decisions for docs/decisions.md".

## Long-running steps

`./ci.sh`: allow 10 minutes (timeout 600000), output to `ci.log` in your task
folder, read the exit code and tail only.

## Report

Follow CONTRACT.md. One final reply of at most ten lines.
