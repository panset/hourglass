# Result: 08 Milestone 6, the UI (UI-12, UI-13, UI-14)

Status: done. `cd ui && npm run typecheck && npm test` pass (18 files, 251 tests). `./ci.sh` exit code 0 (`ci.log`, `ci.exit` in this folder). No API gap found: every field C3 needs is in `schema.gen.ts`. Go code, openapi.yaml, schema.gen.ts, e2e and BUILD.bazel untouched (BUILD globs `src/**`).

## Files changed
- ui/src/pages/usage/weight.tsx (new: WeightTag, attributesLine, ledgerWeight, notAppliedText)
- ui/src/pages/setups/index.tsx
- ui/src/pages/usage/index.tsx
- ui/src/pages/admin-setups/index.tsx
- ui/src/pages/admin-setups/SetupForm.tsx
- ui/src/pages/admin-settings/index.tsx
- ui/src/pages/admin-settings/form.ts
- ui/src/pages/admin-service-accounts/api.ts
- ui/src/pages/setups/setups.test.tsx
- ui/src/pages/usage/usage.test.tsx
- ui/src/pages/admin-setups/admin-setups.test.tsx
- ui/src/pages/admin-settings/admin-settings.test.tsx
- ui/src/pages/admin-service-accounts/admin-service-accounts.test.tsx
- docs/ui-notes.md
- .orchestrator/reported-weights/tasks/08-m6-ui/{PROGRESS.md,RESULT.md,ci.log,ci.exit}

## Existing assertions extended (design D3, additive)
- admin-service-accounts.test.tsx, "creates an account with the checked scopes and refetches": `getAllByRole("checkbox")).toHaveLength(4)` -> `5`. The only one. No other test was changed or weakened.

## Requirement to test
| C3 bullet | Tests (all names begin `UI-12`, `UI-13`, `UI-14`) |
| --- | --- |
| Setups tag per source, attributes line | setups.test: "UI-12 the weight cell carries a tag for reported and override, and none for rule or default"; "UI-12 attributes are one secondary line, not columns, and absent when empty" |
| Acquire dialog: weight, attributes, cost, sentence present (any source) and absent | setups.test: "UI-12 the acquire dialog shows the change warning ... (reported / rule / default / override)"; "UI-12 ... no change warning when reported_weights_enabled is false" |
| My usage hold tag (reported, override, policy, none for rule/default/null) | usage.test: "UI-13 a hold row shows its weight with a tag ..." |
| Burn line unchanged | usage.test: "UI-13 the burn line keeps its form" |
| Ledger single value / range / source / unknown | usage.test: "UI-13 a ledger debit shows one weight ...", "... a range ... with the source ...", "... the source for a single reported or policy weight", "... no weight when it is unknown (null), and a grant shows none" |
| Setup inventory weight, source, reported weight, not-applied words (3) | admin-setups.test: "UI-14 shows the weight and its source tag, the reported weight, and the attributes"; "UI-14 says in words why a reported weight is not applied (expired / out_of_bounds / override)" |
| Override set, clear, untouched, invalid, disable button (request bodies asserted) | admin-setups.test: "UI-14 setting the override sends the number", "... clearing the override sends null", "... an untouched override is left out of the PUT body", "... not a number is refused", "... the disable button does not touch the override" |
| Settings fields, advice note, defaults | admin-settings.test: "UI-14 shows the switch, minimum, maximum and maximum age, and the advice ..."; "UI-14 shows the defaults when the server sends no key" |
| Settings save body; other settings unchanged | "UI-14 saving a change sends all four fields; blank maximum is null"; "UI-14 a save that changes only other settings does not send reported_weights"; existing "ADM-2 saves settings ..." still passes unchanged |
| Server validation message | "UI-14 turning the switch on without a maximum shows the server's validation message" |
| Last report time and account; none received | "UI-14 shows when the last weights report arrived and from which account, or that none did"; "UI-14 says None received, with no warnings ..." |
| Out-of-bounds warning with count | "UI-14 warns with the count when stored weights are outside the limits" |
| Refusal warning (time, account, message); older refusal hidden | "UI-14 warns with the time, account and message of a refusal ..."; "UI-14 does not show a refusal that is older than the last accepted report" |
| Fifth scope | admin-service-accounts.test: "UI-14 the scope list offers weights:write and sends it with the others" |

## User-visible strings added
- Tags: `reported`, `override`, `policy` (title "Weight source: <source>").
- Acquire dialog: "Weights are reported by automation, so the price can change while the setup is held."; "(reported)" / "(override)" after the weight; the attributes line.
- Ledger Type cell: "weight 1.15", "weight 1.15 to 1.40", " (reported, ...)" suffix.
- Setup inventory: "Reported weight N"; "Not applied: the report is older than the maximum age." / "Not applied: the weight is outside the minimum and maximum." / "Not applied: the weight override takes its place."; field "Weight override" with hint "Pins this setup's weight above a reported weight and the rate rules. Leave blank for none; clear it to remove it."; error "Enter a number, or leave it blank to clear the override."
- Settings: heading "Reported weights"; "Accept weights reported by a service account"; "On: a usable reported weight replaces the rate rules for that setup. Off: reports are refused and prices come from the rules."; "Minimum weight"; "Maximum weight" (hint "Required before the switch can be on."); "Maximum age" (hint "A duration from 1m to 24h, such as 10m."); "The maximum age should be at least twice the poll interval (X). With a shorter value, polls can alternate between the reported price and the rule price without any error."; "Last weights report", "None received", "received from A; observed T; N weights"; "N stored weights are outside the minimum and maximum and not applied. Those setups are charged at their rate rule price, and a report that still holds such a weight is refused."; "A weights report was refused after the last accepted one, at T, from A: message".
- Service accounts: scope `weights:write`, hint "report a weight per setup".

## Decisions for docs/decisions.md
1. A ledger debit's weight is written in the Type cell, after the multiplier ("Debit · 2× · weight 1.15"), as C3 says "after the multiplier"; a range uses two decimals ("1.15 to 1.40"), a single value the usual trimmed format. Sources `rule` and `default` are not named; `override`, `reported`, `policy` are, in parentheses.
2. The tag shows for `reported` and `override` on Setups and Setup inventory, and also for `policy` on holds (C3's My usage bullet says "the same tag"; a hold's source may be `policy`).
3. The settings PUT carries `reported_weights` only when one of its four fields was edited, then all four together (D1); a blank maximum is sent as `null`. An unchanged section is omitted, so the other settings are sent exactly as before and the stored value is never reset.
4. The weight override field sends nothing when untouched, `null` when emptied, a number otherwise; non-numbers are refused on the page. The Enable/Disable button and the existing full-replacement PUT never send it.
5. A refusal is shown only when it is later than the last accepted report's `received_at` (C3: "refused after the last accepted one"); the page compares the two API timestamps and does no other arithmetic.
6. The Settings page now fetches the collector once and shares it between "Collector health" and the new card (was fetched inside the health component).
7. The Setups Weight column keeps its place; attributes and the reported/override tag live in that cell.

## Confidence and gaps
- Verified: typecheck, 251 unit tests, ci.sh exit 0 (most Go targets cached, no Go change by me).
- Not verified in a browser or by e2e; styling reuses existing classes (`reason`, `sub`, `stat`, `card`) and was not looked at. The server's field name for the "no maximum" error is assumed to start with `reported_weights` (settings.go comment); the message also appears in the top alert with its field either way.
- Admin Setup inventory shows the "Reported weight" line only when `reported_weight` is not null, per "the reported weight when there is one".
