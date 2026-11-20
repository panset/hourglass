# Result: 67 UI (service accounts, Releasing, last report times)

Status: done. `cd ui && npm run typecheck && npm test` pass (170 tests); `./ci.sh` exits 0 (37 of 37 Bazel tests; ci.log, ci.exit). After ci.sh one hint text on Settings was reworded; typecheck and ui tests were re-run and pass.

## What was built
- **Service accounts page** `ui/src/pages/admin-service-accounts/` (`index.tsx`, `api.ts`, test), route `/admin/service-accounts` in `ui/src/routes.tsx` (admin-only: hidden for developers, not-found by URL). Create, edit, delete (confirm), issue token (optional expiry), revoke (confirm). The issued token is in the issue dialog's state only; it is not stored, not in the URL, not logged, and gone on Close.
- **Setups and My usage**: a hold with `release_requested_at` shows an amber "Releasing" badge and "requested <time>" instead of the Release button. A 202 (hold still active with the marker) gives the toast "Release requested for X. It completes when the setup is torn down." instead of "Released X."; no error. The old `still_deployed` mocks and the "release is refused" note on My usage are replaced. Settings hint for the teardown setting reworded.
- **Collector view** (Settings page, "Collector health"): two tiles, "Last pressure report" and "Last deployment report": received time, sending account, observed time; "None received" when null.
- **Backend addition (D62)**: `GET /api/v1/admin/collector` gains `last_pressure_report` and `last_deployments_report` (`{observed_at, received_at, service_account}` or null; null before the first report and when the extension is not `reported`). `internal/api/admin_system.go`, `api/openapi.yaml` (schema `ReportedReportTimes`), `ui/src/api/schema.gen.ts` regenerated. No new route, so no permissions-matrix row.
- **OpenAPI**: `release_requested_at` is `required` and nullable on `Hold` and `Holder`; fixtures updated.

## Tests
- `admin-service-accounts.test.tsx` (10): list, empty and error states, create, name error, edit, delete confirm, token shown once (value, warning, copy, gone after Close, not in DOM, no storage writes), expiry handling, issue error, revoke confirm. `Gate.test.tsx`: link for admins, not-found for developers.
- `setups.test.tsx`, `usage.test.tsx`: 200 release, 202 moves to Releasing without error or Released toast, Releasing renders with no Release button. `admin-settings.test.tsx`: report tiles and None received.
- Go: `TestINT7_AdminCollectorShowsReportTimes` (null before a report, filled after, null without the reported extensions); `TestUI2_ContractResponsesMatchSchemas` now stores both reports so the non-null shape is validated and also checks `release_requested_at` is present on every Hold.

## Selectors for an end-to-end browser test
- Open Service accounts: top bar link role `link` name "Service accounts" (under the Admin group), or `/admin/service-accounts`; heading "Service accounts".
- Create: button "Create service account"; dialog (role `dialog`, name "Create service account"): textbox "Name", textbox "Description", checkboxes labelled "state:read ...", "pressure:write ...", "deployments:write ...", "holds:release ..." (name starts with the scope); button "Create account"; success text "Created <name>."; the account appears as `listitem` named "Service account <name>".
- Issue and read a token: button "Issue token for <name>"; dialog "Issue token for <name>": optional textbox "Expires (optional)" (datetime-local), button "Issue token". Then dialog "Token for <name>": read-only textbox "Token" (use `inputValue()`; it starts with `hgsa_`), button "Copy token", button "Close", warning text "This is the only time the token is shown". Tokens table: `table` named "Tokens of <name>"; button "Revoke token <id> of <name>" then "Revoke token" in the confirm dialog.
- Releasing on Setups: in the row of the held setup (table "All setups"), text "Releasing" and "requested <time>" replace the button "Release"; the My usage "Current holds" table does the same.
- Collector times: Settings page, text "Last pressure report" and "Last deployment report".

## Decisions for docs/decisions.md
D62 appended (collector fields; `release_requested_at` required).

## Files changed
`api/openapi.yaml`, `docs/decisions.md`, `internal/api/admin_system.go`, `internal/api/admin_system_test.go`, `internal/api/contract_test.go`, `internal/api/integration_reports_test.go`, `ui/src/api/schema.gen.ts`, `ui/src/routes.tsx`, `ui/src/pages/admin-service-accounts/*` (new), `ui/src/pages/admin-settings/{index.tsx,admin-settings.test.tsx}`, `ui/src/pages/admin-shared/fixtures.ts`, `ui/src/pages/setups/{index.tsx,setups.module.css,setups.test.tsx}`, `ui/src/pages/usage/{index.tsx,holdApi.ts,usage.module.css,usage.test.tsx}`, `ui/src/shell/Gate.test.tsx`, this folder.

## Confidence and gaps
- The Releasing marker on Setups appears only on the viewer's own hold (where the Release button was); other people's holds are unchanged.
- `docs/ui-notes.md` still describes the old refusal; it is outside my scope. `e2e/browser` step A7 still expects the old refusal (task 68).
- The unauthenticated 202 detection uses the response body (`active` and `release_requested_at`), because the client does not expose the status code. Appears equivalent to the status.
