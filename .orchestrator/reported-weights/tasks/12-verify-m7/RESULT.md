# 12 Independent verification of milestones 6 and 7 and of the whole job: result

Status: **done**. Baseline `036e018` (`baseline.sha`), plus the uncommitted e2e changes in the working tree. I worked from the design, INTENT, CONTRACT, D80 to D88, the code and the tests. I did not read the RESULT, PROGRESS or mutation logs of the build tasks. I read the RESULT files of 04 and 07 and the 11 examples log, which the brief allows.

Verdict: every id from WGT-1 to WGT-23 and from UI-12 to UI-14 has a named test. Each of those tests runs under Bazel and passes, and none is skipped. Every deliberate break I made (12 in the UI, 13 in Go) turned a test named after the id red, except one that names no id (finding 2). The compatibility rules of the job hold. There are **no high findings**: 1 medium, 6 low and 4 informational. The end-to-end steps W1 to W6 were **not run**, because the brief says the owner's processes hold the ports. I reviewed them by reading.

## Findings, ranked by severity

1. **Medium: the decision that lists the edits `docs/SPEC.md` needs is missing.**
   - Design deliverable 5 asks for "a last row [that] lists the edits the source of `docs/SPEC.md` needs, as D79 did".
   - D80 (`docs/decisions.md:88`) says "the edits ... are listed in a later entry when the work is complete". `docs/progress.md:93` says they "are listed in the last decision of this work".
   - The last row is D88 (`docs/decisions.md:96`), which is about the compatibility tests. No row lists the edits for SPEC.
   - Scenario: the release report names "every decision recorded", but the spec-source edits the owner needs are recorded nowhere. `progress.md` points to a row that does not exist.
   - Fix: the orchestrator adds D89 with the list the design names: "What is billed", RULE-1, MET-3, the `usage_samples` and `setups` rows, the two new tables, the settings table, the HTTP API table, the Roles table and the requirements.
2. **Low: no automated test checks the value of `reported_weights_enabled` on `GET /api/v1/setups`.**
   - This field is what UI-12's acquire warning depends on.
   - Code: `internal/api/handlers_user.go:885`. Tests check only that the key exists (`internal/api/user_test.go:380`). WGT-20 checks that added fields are present but not their values (`upgrade_v110_compat_test.go:422-427`, by design, D88(b)).
   - Break I made: I hard-coded `"reported_weights_enabled": true`. All of `go test ./internal/api` still passed (`go-mutations.log`).
   - Scenario: a regression makes the field always `false`. The acquire dialog then never warns that the price can change, and no Go or UI test notices. UI tests use fixtures.
   - Only e2e W2 would catch it (`integration.spec.ts`, "Weights are reported by automation..."), and W2 has not run.
   - Fix: one assertion in a WGT-14 or WGT-17 test that the field follows the setting.
3. **Low (docs): `docs/integrating.md:491` says "With the setting off ... `source` is `rule` or `default`".**
   - An override applies with the feature off too (design D2; `internal/weights/weights.go:121-123` runs before the switch check at `:126`; `TestWGT18_WeightOverride`). With the setting off, `source` can be `override`.
   - `docs/deploying.md` states this correctly. The guide contradicts it.
4. **Low (docs): `docs/integrating.md:429` ("They are shown only while the entry applies") and `:489` ("`attributes` is `{}` when ... the entry is not applied") are wrong for an override.**
   - Under an override the entry does not apply (`not_applied: "override"`), but its attributes are returned and shown (design A1, "An override does not hide them"; `weights.go:144-150`).
   - Better wording: "shown while the entry is usable (not expired and within the limits)".
5. **Low (docs): `docs/deploying.md:216` says an expired weights report shows on "the Setups page and the state route ... (`not_applied` is `expired`)".**
   - The user Setups page shows no `not_applied`. Only Admin, Setup inventory and the state route do (`ui/src/pages/admin-setups/index.tsx`, `notAppliedText`).
   - It should say "Setup inventory".
6. **Low (docs): `docs/progress.md:102` says "The `d96a950` fixtures and their tests are unchanged except for the schema version number (D82)".**
   - `TestUPG2_ResponsesAndSignInDifferFromTheReleaseOnlyByAddedFields` (`internal/api/upgrade_compat_test.go`) was also changed: it requires and strips the added setups and ledger fields, as recorded in D85(l), REVIEW.
   - The edit is not a weakening (see "Existing tests"), but the progress sentence understates it.
7. **Low (decisions): D88(c) (`docs/decisions.md:96`) says "The only value change listed is the first deliberate change of design A3".**
   - The list after the polls also names the second change: Admin Users burn goes from 2.5 to 6.5 (`internal/api/upgrade_v110_compat_test.go:394-396`).
   - The code is right; the record is incomplete.
8. **Info (docs): `docs/extending.md:156` says "To make a setup free ... give it a rate rule of weight 0".**
   - With reported weights on, a usable reported weight replaces any rule, including one of weight 0 (owner decision). The way to pin a setup free is a weight override of 0.
   - Worth a clause.
9. **Info (docs): the guide's examples are real but come from different moments.**
   - The request example's response (`observed_at` 06:19:04.255Z) comes from the "weights:write alone" run (`examples.log:328`).
   - The "Check that it was taken" state shows `observed_at` 06:18:46.000Z, from the earlier run that carried an explicit `observed_at` (`examples.log:123-174`).
   - Each matches the log exactly. Read one after the other, they look inconsistent. No action is needed.
10. **Info (e2e): W3 and W4 check a debit's `weight_min`, `weight_max` and `weight_sources`, and that `amount_tokens` is above 0, but not the amount itself.**
    - The arithmetic is covered by `TestWGT1_ReportedWeightIsCharged` (96, 192 and 288 milli-tokens).
11. **Info (e2e): W6 moves the test clock 11 minutes.**
    - The first poll after the jump may fail because the deployments and pressure reports are stale until `rep.send()`. It also sees a gap of more than twice the interval, which is not charged.
    - `debitAfter` waits up to 4 minutes for a later debit, so this appears to be safe, as in I7. A flake here would show as a timeout, not as a wrong weight.

## Requirement table

Tests ran under Bazel (`bazel test //... --nocache_test_results`, and the Go targets again with `-test.v`, `bazel-verbose.log`). Every test listed passed, and none was skipped. UI tests ran under `//ui:test` and with vitest verbose (`vitest-verbose.log`).

| Requirement | What it requires (one line) | Tests | File |
| --- | --- | --- | --- |
| WGT-1 | A usable reported weight is debited weight × multiplier × hours (1.15 for 5 min = 96 mT) | TestWGT1_ReportedWeightIsCharged | internal/collector/weights_wgt_test.go:88 |
| WGT-2 | Order: override, usable reported, rule (even 0), default; policy weight above 0 replaces it | TestWGT2_ResolverTable, _OverrideBeatsReportedAndKeepsAttributes, _ReportedBeatsRuleOfWeightZeroAndOfHigherWeight, _ReportedZeroWithMinimumZero, _BadRuleIsAnError; collector: _OrderOverrideReportedRuleDefault, _ReportedBeatsRuleOfWeightZeroAndOfHigherWeight, _PolicyWeightReplacesResolvedWeightForThatHoldAndPoll, _PolicyWeightBeatsOverrideAndReportedWeight | internal/weights/weights_test.go:75-206; internal/collector/weights_wgt_test.go:114-169 |
| WGT-3 | No usable report: fallback as without the feature in every hold state, poll does not fail; a DB error does | TestWGT3_EachFallbackCauseChargesAsWithoutTheFeature, _DatabaseErrorReadingTheReportFailsThePoll, _HoldMeteredBeforeItsSetupIsReported, _PendingReleaseCompletedAfterSetupLeftTheReport | internal/collector/weights_wgt_test.go:190-303 |
| WGT-4 | A changed weight prices the next poll's whole interval; earlier samples and ledger rows unchanged | TestWGT4_ChangedWeightPricesTheNextIntervalOnly | internal/collector/weights_wgt_test.go:345 |
| WGT-5 | Every sample records weight and one of five sources; `rule_selector` only for `rule` | TestWGT5_EverySampleHasOneOfFiveSourcesAndSelectorOnlyForRule | internal/collector/weights_wgt_test.go:378 |
| WGT-6 | The snapshot carries the base weight; a report never moves inventory pressure or the multiplier | TestWGT6_SnapshotCarriesTheBaseWeight, TestWGT6_InventoryPressureIgnoresReportedWeights | internal/collector/weights_wgt_test.go:429; pressure/inventory/wgt6_collector_test.go:80 |
| WGT-7 | PUT replaces the whole set atomically, with ordering and skew rules and optional `observed_at`; survives a restart | TestWGT7_ReportReplacesTheWholeSet, _ReadersNeverSeeAMixOfTwoReports, _OrderingAndClockSkew, _OrderingIndependentOfTheDeploymentsReport, _ReportSurvivesARestart, _ReportNeverPollsDebitsOrEmits; store: _StoreReplacesWeightsReportWhole, _StoreWeightsReplaceIsAtomicForReaders, _StoreWeightsReportSurvivesReopen; reported: _WeightsNoReportYet, _WeightsReplaceWholeSetAndDefaultObservedAt, _WeightsOrderingAndClockSkew, _WeightsOrderingIndependentOfDeployments, _AttributesStoredUnchanged, _WeightsSurviveReopen | internal/api/weights_report_test.go:190-355; internal/store/weights_queries_test.go:18-138; internal/reported/weights_test.go:27-299 |
| WGT-8 | Any invalid entry refuses the whole report with 400, giving position and name; nothing stored | TestWGT8_EachInvalidEntryRefusesTheWholeReport, _FirstInvalidEntryReported (api and reported), _LimitsInclusiveAndNegativeZero, _BodyRefusals, _EntryAndBodyLimits, _LastRefusalKeptForAdmins, _EachEntryRuleRefusesWholeReport, _EntryMessageForm, _TooManyEntriesRefused, _NegativeZeroStoredAsZero, _UnknownNamesStored | internal/api/weights_report_test.go:384-559; internal/reported/weights_test.go:151-271 |
| WGT-9 | An unknown name is ignored, changes no weight, and is listed in `unknown` | TestWGT9_UnknownNameIgnoredAndListed | internal/api/weights_report_test.go:609 |
| WGT-10 | Off: 409 `not_configured` naming the setting, whatever the body; nothing stored | TestWGT10_OffAnswersNotConfiguredWhateverTheBody | internal/api/weights_report_test.go:633 |
| WGT-11 | The route needs `weights:write`; every other scope gets 403; the scope opens nothing else; full matrix | TestWGT11_RouteNeedsWeightsWriteAndTheScopeOpensNothingElse, _ScopeAddedLastAndNotGrantedToExistingAccounts; permissions matrix | internal/api/weights_report_test.go:668-707; internal/api/permissions_test.go |
| WGT-12 | A stored weight the limits now exclude stops applying from the next poll or read | TestWGT12_LimitsCheckedAtReadTime, _LoweredMaximumChargesTheFallbackAtTheNextPoll, _TighteningTheMaximumOverHTTP | internal/weights/weights_test.go:191; internal/collector/weights_wgt_test.go:491; internal/api/weights_report_test.go:886 |
| WGT-13 | The state route gives applied, source, reported, not_applied and the top-level switch, limits and times; nulls when off; no rule text | TestWGT13_StateWeightFields, _StateAnswersWithARuleThatCannotBeEvaluated | internal/api/weights_report_test.go:735, :950 |
| WGT-14 | Every reader shows the weight and source a poll records; a policy-priced hold shows `policy` | TestWGT14_EveryReaderShowsTheWeightThePollCharges | internal/api/weights_readers_test.go:182 |
| WGT-15 | A ledger debit shows the min, max and sources of its polls (raw, day, compacted); null when partial | TestWGT15_LedgerEntriesShowTheWeightsOfTheirPolls, TestWGT15_StoreDebitWeightAggregates | internal/api/weights_readers_test.go:451; internal/store/weights_queries_test.go:332 |
| WGT-16 | Attributes are stored and returned unchanged, shown only for a usable entry, matched by nothing | TestWGT16_AttributesAreForDisplayOnly | internal/api/weights_readers_test.go:545 |
| WGT-17 | The setting validates per D1, needs all four fields, cannot be on without a max, is audited, and is kept when absent | TestWGT17_ReportedWeightsValidateAndDefaults, _GetAlwaysReturnsTheKeyWithDefaults, _EveryRuleOfTheD1Table, _OnWithoutMaximumMessage, _AllFourFieldsRequiredWhenPresent, _ChangeAuditedWithWholeObjects, _PutWithoutTheKeyLeavesItUnchanged, _StoredValueLoads, _NegativeZeroMinIsStoredAndReturnedAsZero | internal/settings/settings_test.go:229; internal/api/settings_reported_weights_test.go:39-265 |
| WGT-18 | The override pins the weight above report and rules, on or off, outside the limits, audited; absent keeps it, null clears it | TestWGT18_WeightOverride | internal/api/weights_readers_test.go:650 |
| WGT-19 | A v1.1.0 database upgrades with the same config; with the feature off, balances, charges, debits and snapshots are the same | TestWGT19_MigrationRewritesNoRowOfV110Database, _ApplicationStartUpgradesV110DatabaseAndKeepsEveryNumber, _ReplayV110PollsWithFeatureOff | internal/app/upgrade_v110_test.go:302, :344; internal/collector/upgrade_v110_replay_test.go:61 |
| WGT-20 | With the feature off, responses differ from v1.1.0 only by added fields and the two A3 changes; contract kept | TestWGT20_ResponsesDifferFromV110OnlyByTheNamedAdditionsAndChanges, _ResponsesAfterThePollsDifferFromV110OnlyByTheNamedAdditionsAndChanges, _PolicyPricedHoldShowsResolverWeightUntilTheFirstPollThenThePolicyWeight, _NoRouteParameterOrFieldOfV110IsRemovedAndNoRequestGainedARequiredField, _StateAnswersAsBeforeWithARuleThatCannotBeEvaluated_FeatureOff | internal/api/upgrade_v110_compat_test.go:478-691; internal/api/weights_report_test.go:972 |
| WGT-21 | The report follows the name and the switch (create, rename, off and on, `max_age` raised); the override follows the setup | TestWGT21_StoredWeightFollowsTheNameAndTheSwitch | internal/api/weights_report_test.go:826 |
| WGT-22 | The collector view gives the last report (entries, out_of_bounds) and the last refusal; null when off | TestWGT22_CollectorViewShowsTheWeightsReportAndTheLastRefusal, _TighteningTheMaximumShowsToTheAdmin | internal/api/weights_readers_test.go:836, :883 |
| WGT-23 | A released hold shows its last sample's weight and source, and later changes do not move it | TestWGT23_ReleasedHoldShowsItsLastCharge | internal/api/weights_readers_test.go:936 |
| UI-12 | Setups shows the weight with its source tag and attributes; the acquire dialog warns whenever enabled | "UI-12 the weight cell carries a tag...", "UI-12 attributes are one secondary line...", "UI-12 the acquire dialog shows the change warning ... (reported, rule, default, override)", "UI-12 ... has no change warning when ... false" | ui/src/pages/setups/setups.test.tsx:510-560 |
| UI-13 | My usage shows a hold's weight with its source, and a debit's weight as one value, a range or nothing | "UI-13 a hold row shows its weight with a tag...", "the burn line keeps its form", "a ledger debit shows one weight when min equals max", "... a range ...", "... the source for a single reported or policy weight", "... no weight when it is unknown (null)..." | ui/src/pages/usage/usage.test.tsx:470-524 |
| UI-14 | Setup inventory shows the reported weight and why it is not applied, and edits and clears the override; Settings has the four fields, last report, count and refusal; Service accounts offers the scope | admin-setups: 9 tests (:219-288); admin-settings: 10 tests (:228-349); admin-service-accounts: "UI-14 the scope list offers weights:write..." (:206) | ui/src/pages/admin-setups/admin-setups.test.tsx; ui/src/pages/admin-settings/admin-settings.test.tsx; ui/src/pages/admin-service-accounts/admin-service-accounts.test.tsx |

## Deliberate breaks (scratch copy outside the repo; each restored before the next)

UI (`ui-mutations.log`). Every one turned a test named after its id red:
- **UI-12:** the warning is never shown (4 red); no tag for `reported` (red); the attributes line is dropped (red).
- **UI-13:** always a range (2 red); the hold tag is dropped (red); an unknown weight shows as 0 (red).
- **UI-14:** clearing the override sends nothing instead of `null` (red); the not-applied words are dropped (3 red); Settings always sends `reported_weights` (red); the out-of-bounds warning is hidden (red); the refusal warning is hidden (red); `weights:write` is removed from the scopes (red).

Go (`go-mutations.log`):
- **WGT-19:**
  - Migration 0008 also runs `UPDATE usage_samples SET rule_selector = NULL`: red in `_MigrationRewritesNoRowOfV110Database` and `_ApplicationStart...`.
  - The debit uses weight × 1.01: red in `_ReplayV110PollsWithFeatureOff`. The application-start test stayed green for this one; the replay is the guard.
  - The snapshot weight is base + 0.5: red in `_ReplayV110PollsWithFeatureOff`.
- **WGT-20:**
  - The JSON tag `burn_per_hour_tokens` is renamed: red in 2 tests.
  - An unexpected `extra_field` is added to the Setups list: red ("a difference ... the list for this route does not allow: added extra_field").
  - `reported_weights_enabled` is hard-coded: green, see finding 2.
- **Re-checks on the final code:**
  - WGT-1/2: the reported weight is never applied: red in WGT1 and 4 WGT2 tests.
  - WGT-3: expiry is ignored: red.
  - WGT-12: the maximum is not checked: red in 3 packages.
  - WGT-6: the snapshot uses the resolved weight: red in both WGT6 tests.
  - WGT-17: a PUT without the key resets it: red.
  - WGT-21/13: the switch is ignored in both the resolver and `weightReport`: WGT13, WGT16, WGT18, WGT21 and the resolver table go red.
  - Ignoring the switch in the resolver alone is caught only by the collector (WGT-3), because the API also checks the switch before it reads the report (`handlers_weights.go:284`). The guard is double; this is not a gap.

On WGT-19 and WGT-20:
- The allowed differences are an explicit list per route (`upgrade_v110_compat_test.go:296-398`).
- Any unlisted added, changed or removed field fails. So does a listed difference that does not occur, or one that occurs a different number of times.
- A status change fails too, and so does a recorded request without a list.
- Added fields must be present, but their values are not compared (finding 2).

## UI depth (design C3 against the code)

- **Setups:** the weight, the tag for `reported` and `override` (`WeightTag`, `ui/src/pages/usage/weight.tsx`), and the attributes as one block line (`.reason` is `display: block`, `setups.module.css:16`), not columns. **Holds.**
- **Acquire dialog:** the weight with its source in parentheses, the attributes, the cost as before, and the warning gated only on `data.reported_weights_enabled` (`setups/index.tsx`). **Holds.**
- **My usage:**
  - The hold row tag covers `reported`, `override` and `policy`, and the burn line is unchanged.
  - The ledger shows "Debit · 2× · weight 1.15" or "1.15 to 1.40", with the source when it is not `rule` or `default`, and nothing when the weight is null. **Holds.**
  - A range uses two decimals, so a range narrower than 0.005 shows the same number twice. This is cosmetic.
- **Setup inventory:** the weight, the tag, "Reported weight N" with the reason in words, and the attributes. The form has a clearable override: untouched sends nothing, blank sends `null`, a number sets it, and Enable/Disable never send it. **Holds.**
- **Settings:**
  - The four fields, the advice with the server's poll interval, the last report's time, account and entry count, and the out-of-bounds count warning.
  - The refusal is shown only when it is newer than the last accepted report. This is a timestamp comparison, D87(e).
  - The server's message is shown for on-without-max.
  - `reported_weights` is sent only when one of its four fields changed (`toSettings(form, reportedDirty)`), so other saves never reset it by omission. **Holds.**
- **Service accounts:** `weights:write` is the fifth scope. **Holds.**
- **Arithmetic:** the UI does no weight, cost or range arithmetic. The added lines in `ui/src` hold only formatting, comparisons and string joins.

## End-to-end (not run)

`e2e/out/integration/steps.json` comes from an earlier run (16:17, 779 s): I0 to I10 pass, and it has no W steps. W1 to W6 are uncommitted in `e2e/browser/tests/integration.spec.ts`. Against the six bullets of the design's Tests section:

| Design bullet | Step | Assertion that would fail if broken |
| --- | --- | --- |
| Admin turns it on with a max and gives the scope, in the UI | W1 | 403 before the grant; the account holds exactly the five scopes after the UI save; 409 `not_configured` naming the setting; settings saved as `{enabled:true,min:0,max:3,max_age:"10m"}`; an empty report gets 200 |
| Weights for every setup; Setups shows weight and attributes before acquire; the dialog warns | W2 | The state shows every setup with `source: "reported"` and lab-03's attributes; the Setups row shows 1.15, the `reported` tag, the attributes and Free; the dialog shows the warning text, "Weight 1.15 (reported)" and the attributes |
| Charged at the reported weight; a change moves the next debit | W3 | The first debit has `weight_min`/`max` 1.15 and source `reported`; the hold shows 1.15 with its tag; after the change the hold shows 1.4 before any poll; the next debit is 1.4 and the earlier one still 1.15 |
| Release; the weight is kept through teardown; the last debit is at the reported weight | W4 | 202 and Releasing; the weights PUT still 200; a new debit by the completing poll at 1.4 `reported`; the released hold shows 1.4 `reported` |
| Weights stop, deployments and pressure go on; after `max_age` the rule price applies and polls succeed | W6 | The state shows lab-04 at `{applied:2, source:"rule", reported:0.9, not_applied:"expired"}` with attributes `{}`; the next poll is ok; a debit at 2 `rule`; `last_error` null; Setup inventory gives the expired words |
| An over-max report is refused, changes nothing, and appears on Settings | W5 | 400 `validation` `weights` naming entry 0 and lab-03; state `observed_at` unchanged; lab-04 still 0.9; the collector refusal names the account; the Settings warning text; the next debit is still 0.9 |

Every bullet has a step with real assertions, not only a screenshot. `e2e/README.md` lists W1 to W6 to match. `integration-stack.sh` gives the docs account the five scopes. I9 adds the weights route to the 403 checks.

## The first client's eight acceptance scenarios

| Scenario | Named test (passing) | e2e step (not run) |
| --- | --- | --- |
| A reported weight is charged (1.15 for 5 min = 96 mT) | TestWGT1_ReportedWeightIsCharged (asserts 96, 192, 288) | W3 |
| A weight changes during a hold | TestWGT4_ChangedWeightPricesTheNextIntervalOnly | W3 |
| Falling back when reports stop or go stale | TestWGT3_EachFallbackCauseChargesAsWithoutTheFeature, _HoldMeteredBeforeItsSetupIsReported, _PendingReleaseCompletedAfterSetupLeftTheReport | W6 |
| Invalid values | TestWGT8_EachInvalidEntryRefusesTheWholeReport and 11 more WGT8 tests | W5 |
| Every screen agrees with the charge | TestWGT14_EveryReaderShowsTheWeightThePollCharges | W2, W3 |
| Reading the applied weight back | TestWGT13_StateWeightFields | W2, W6 |
| The feature switched off behaves like today | the 3 TestWGT19 and 5 TestWGT20 tests | (not covered) |
| The permission check | TestWGT11_RouteNeedsWeightsWriteAndTheScopeOpensNothingElse, _ScopeAddedLastAndNotGrantedToExistingAccounts | W1, I9 |

Each scenario maps to a passing named test.

## Docs against behaviour

**`docs/integrating.md`**
- The "Report weights" job is present and self-contained: no reference to the design, decisions, D-numbers or source.
- All eight "It must say" points are present: reserves, not utilisation; complete set from one service; weights before deployments; keep reporting until `hold` is null, including the last interval; leave out rather than send 0; a number, not a string; an expired report does not stop charging, so check `weights.expires_at` and alert on a 400; `not_configured` is an admin setting.
- The scope, state fields, error rows and checklist lines are present.
- These match `examples.log` exactly: every quoted refusal message, the 409 message, the on-without-max message (`settings.go:162`), the 403 body, the state shapes, the override and tightening runs, the empty list, weight 0, the 1m `max_age` expiry and the poll succeeding, and switch off.
- The key rule text matches the regex in the log, `[a-z0-9]([a-z0-9._/-]{0,61}[a-z0-9])?`.
- The "5-minute default poll" matches `settings.go:90`.
- Mismatches: findings 3, 4 and 9.

**`docs/deploying.md`**
- Present: the setting and its fields, the PUT exception, tightening, the override, the `max_age` advice, the 0008 upgrade entry (additive, no down migration, with the message `database schema version N is newer than this binary`, which matches `store.go:266`), the four shifted fields, the policy-hold window (D86(c)) and the body keys.
- Mismatch: finding 5.

**`docs/extending.md`**
- `SetupState.Weight` is the base weight, the precedence is right, and a policy cannot see the report. Finding 8.

**`README.md`**: one feature paragraph. Correct.

**`docs/ui-notes.md`**: item 14 matches the code, including the texts of `notAppliedText`.

**`docs/progress.md`**: findings 1 and 6.

## Whole-job rules

- `docs/SPEC.md` is untouched since `753cc50`.
- `git diff 9cabece..HEAD -- docs/design/` is empty.
- `git diff 0d20688..HEAD -- testdata/` is empty. `testdata/` has 3 commits, and `upgrade-d96a950` is unchanged since `753cc50`.
- `pkg/hourglass/types.go` changed only in the `SetupState.Weight` comment.
- No `internal/config` or `cmd` change, so no config key was added.
- Migration 0008 only creates two tables and adds two nullable columns.
- OpenAPI removes nothing: the removed lines are `required` lists that grew, the scope enum that grew, and the description of `Setup.weight`.
- One route is added (`routes.go`) and one scope is appended.
- `tools/check_core_generic.sh` reports OK.
- The banned words (`lab-connector`, `mem-gb`, `disk-gb`) appear only in `docs/design/reported-weights.md` and `.orchestrator/` (CONTRACT, briefs, and the 04 and 07 results).
- No logic remembers a removed weight: the resolver reads only the current stored report, and the fallback cases are tested (WGT-3).

**Existing tests** (`git diff 0d20688..HEAD`, 18 Go and 5 UI test files modified). None is weakened. Every edit is one of these:
- added keys in exact key-set checks, which now also require the new keys;
- migration count or schema version 7 becoming 8;
- the schema column list gaining the new columns;
- the route count 43 becoming 44, with a probe and coverage entry;
- the setting turned on in the contract test and the integration test, which also gain new calls;
- the INT-8 allow-list gaining the new keys, with `attributes` free-form like `labels`;
- a `dbPath` test option;
- the UI scope checkbox count 4 becoming 5, and the UI fixtures gaining the new fields.

The two `d96a950` tests:
- D82(j): `TestUPG1_ReleaseDatabaseUpgradesAtStartAndKeepsEveryNumber` changes only 7 to 8, which a new migration cannot avoid.
- D85(l): `TestUPG2_...` now requires each added field (`reported_weights_enabled`, the setup and admin weight fields, the ledger weight fields) and strips it before the unchanged whole-body comparison.
- Neither loosens any comparison of a field that existed in `d96a950`. Both pass.

## docs/decisions.md D80 to D88

- I checked these against the code: D82(d, e, g, j), D83(a, b, d, e), D84(a, b, c, e), D85(a, b, c, f, i, j, l), D86(a, b, c) and D87(a to g). They match the code, and none contradicts the design.
- D86(a), the state route answering 200 on a broken rule, goes beyond A1's list. A1 does not name the state route, so this does not contradict it.
- D88(c) is incomplete: finding 7.
- Missing from the record: the SPEC-edits row (finding 1).
- I found no other unrecorded choice in the code I read. One small one is recorded only implicitly: the ledger range uses two decimals (D87(a) says this).

## Test counts

- `./ci.sh`: exit 0 (`ci.log`, `ci.exit`).
- `bazel test //... --nocache_test_results`: exit 0. 40 of 40 targets executed and passed (`bazel.log`).
- Go targets under Bazel with `-test.v`, uncached: 755 top-level tests and 1,138 subtests passed, 0 failed, 0 skipped. 80 of them are `TestWGT*`, and all 80 passed (`bazel-verbose.log`). The conditional tzdata skips did not fire.
- UI (vitest verbose, scratch copy): 18 files, 251 tests passed, 0 skipped, 33 named UI-12 to UI-14 (`vitest-verbose.log`). `//ui:test` under Bazel: 251 passed.
- e2e: not run (ports held by the owner's processes, per the brief).

## Commands run

- `./ci.sh`: exit 0.
- `bazel test //... --nocache_test_results --test_summary=detailed`: exit 0.
- `bazel test <all go_test targets> --nocache_test_results --test_arg=-test.v`: exit 0. My first attempt failed to parse its target list under zsh and ran nothing; I reran it under bash.
- `npx vitest run --reporter=verbose` in a scratch copy: exit 0.
- `go test -count=1 <pkgs> -run <pattern>` for each mutation in the scratch copy.
- `bash tools/check_core_generic.sh`: OK.
- git `diff`, `log` and `grep`, read-only.

## Files changed

Only this task folder, all new:
.orchestrator/reported-weights/tasks/12-verify-m7/PROGRESS.md
.orchestrator/reported-weights/tasks/12-verify-m7/RESULT.md
.orchestrator/reported-weights/tasks/12-verify-m7/baseline.sha
.orchestrator/reported-weights/tasks/12-verify-m7/status-before.txt
.orchestrator/reported-weights/tasks/12-verify-m7/ci.log
.orchestrator/reported-weights/tasks/12-verify-m7/ci.exit
.orchestrator/reported-weights/tasks/12-verify-m7/bazel.log
.orchestrator/reported-weights/tasks/12-verify-m7/bazel.exit
.orchestrator/reported-weights/tasks/12-verify-m7/bazel-verbose-run.log
.orchestrator/reported-weights/tasks/12-verify-m7/bazel-verbose.log
.orchestrator/reported-weights/tasks/12-verify-m7/vitest-verbose.log
.orchestrator/reported-weights/tasks/12-verify-m7/ui-mutations.log
.orchestrator/reported-weights/tasks/12-verify-m7/go-mutations.log

All mutations were in a scratch copy outside the repo. `git status --short` shows nothing of mine outside this folder; it matches `status-before.txt` apart from these files. I started no container or compose stack, and stopped no process.

## Decisions for docs/decisions.md

None taken. For the orchestrator:
- Add the SPEC-edits row (finding 1).
- Optionally correct D88(c) (finding 7).

## Confidence and gaps

- I did not run e2e W1 to W6. Their assertions are read, not executed, and selectors or timing could still fail on a real run (finding 11).
- I did not start a real `hourglassd`. I checked the doc examples against `examples.log` and the code, not by re-running them.
- I did not repeat 07's full HTTP probe campaign or 04's resolver campaign. On the final code I re-broke only one or two points per id for WGT-1, 2, 3, 6, 12, 13, 17 and 21.
- The scale test (NFR) was not run.
- `bazel-testlogs` were read right after my verbose run. They reflect the tree at `036e018` plus the uncommitted e2e files, which are not Go or UI tests.
