# Reported weights: report for the owner before release

Branch `reported-weights`, HEAD `a5a3d086fffed1c84447ce765f5546ce482d284e` (cut from `main` at `753cc50`, release `v1.1.0`). Nothing has been merged, pushed or tagged. This report was written after a final round of runs on that exact HEAD. Every tracked file was clean when those runs started and when they ended; the only untracked files are old `.orchestrator` logs.

## 1. Result in brief

- All four final runs passed: `./ci.sh`, the uncached `bazel test //...`, `e2e/run.sh both` and `e2e/run.sh integration` (section 8).
- Every requirement (WGT-1 to WGT-23, UI-12 to UI-14) has at least one passing test named after it, and the first client's eight acceptance scenarios each map to a passing test (section 4).
- The binary reports its version as `0.0.0-dev`, not as the tag. This is how `v1.1.0` behaved too (section 9). Decide whether that matters before you tag.
- Fast-forwarding `main` to this branch is possible (`main` is an ancestor of HEAD).
- The release steps that remain, and what the tag does to `latest`, are in section 9. They wait for your go-ahead and the version number.

## 2. What was built

The design is `docs/design/reported-weights.md`. In plain terms: a customer's own service can now tell Hourglass what a setup currently reserves, as a number called a weight. Hourglass charges that weight instead of the one the rate rules give, as long as the report is fresh and within limits an admin sets. When there is no usable report, it charges exactly as before.

**A. Charging and the resolver (design area A).**
- One place, `internal/weights`, decides the weight of a setup. The order is: an admin's override, then a usable reported weight, then the rate rules (even a rule of weight 0), then the default. A Go metering policy that sets its own weight still wins for that hold and poll, as before.
- The collector charges through it. A change of weight prices the next poll's whole interval; earlier samples and ledger rows never change. Every usage sample now records the weight and its source (`override`, `reported`, `rule`, `default` or `policy`).
- If the report is missing, expired, outside the limits, or the feature is off, the charge is as it was in v1.1.0, and polling does not fail. Two fallbacks the design names are implemented and tested as written: a hold metered before its setup was reported, and the last interval of a release after the setup left the report. No logic remembers a removed weight.
- The base weight is what the snapshot given to pressure sources and policies carries, so a report never moves inventory pressure or the price multiplier.

**B. The report route and storage (area B).**
- `PUT /api/v1/integration/weights`, under a new scope `weights:write`, replaces the whole set of weights atomically, with the same ordering and clock-skew rules as the other two reports. Any invalid entry refuses the whole report with a 400 that names the entry. Unknown setup names are stored and listed back, not an error. With the setting off the route answers 409 `not_configured`.
- Migration `0008` adds two tables (the stored report and its metadata) and two nullable columns (a sample's weight source and a setup's weight override). It rewrites no existing row and has no down migration.
- The admin setting `reported_weights` has `enabled`, `min`, `max` and `max_age`. It cannot be on without a maximum. It is audited, and a settings save that omits the key leaves it unchanged. Limits are checked when a weight is read, so tightening the maximum takes effect at the next poll.
- `GET /api/v1/integration/state` shows the applied weight, its source, the reported weight and why a report is not applied.

**C. What users and admins see (area C).**
- Every reader (Setups list, holds, My usage, admin views, state route, collector view) uses the same resolver, so every screen agrees with the charge. A ledger debit shows the lowest and highest weight and the sources of the polls it sums up. A released hold shows its last charge's weight.
- Admins can set or clear a per-setup weight override. The collector view shows the last report and the last refusal.
- UI: Setups shows the weight with a source tag and the attributes, and the acquire dialog warns that the price can change; My usage shows weights on holds and debits; Setup inventory shows the reported weight and why it is not applied, and edits the override; Settings has the four fields; Service accounts offers `weights:write`.

**D. Compatibility, tests, docs and end-to-end (area D).**
- The v1.1.0 fixtures (a database, responses, and three recorded polls) were recorded from the tag before any product change, in `testdata/upgrade-v1.1.0/`. Tests prove an upgraded v1.1.0 database keeps every number, and that with the feature off every response differs from v1.1.0 only by added fields and the two deliberate changes the design names.
- Docs: the "Report weights" job in `docs/integrating.md` (its examples were run against a real server), the setting, upgrade notes and the four fields whose meaning shifts in `docs/deploying.md`, plus `docs/extending.md`, `docs/progress.md`, `docs/ui-notes.md` and `README.md`.
- `e2e/run.sh integration` has six new steps (W1 to W6) that run the whole story through a browser.

### Commits (`git log --oneline 753cc50..HEAD`)

- a5a3d08 End-to-end scenario for reported weights: report, charge, change, release, refusal, expiry (WGT-1 to WGT-12, WGT-17, WGT-21, WGT-22, UI-12 to UI-14)
- f35e3f4 Close the final verification's findings; list the spec edits (UI-12, WGT-13, WGT-20)
- 036e018 Document reported weights: the Report weights job, the setting, the upgrade notes (WGT-1 to WGT-23)
- f066462 Test compatibility with v1.1.0: same charges, same responses but for added fields (WGT-19, WGT-20)
- a420208 Show weights, their source and the reported-weights controls in the UI (UI-12, UI-13, UI-14)
- 33cd261 State route answers with a rate rule that cannot be evaluated (WGT-13, WGT-20)
- 626b85d Put every reader on the resolver; ledger weights, setup override, collector view (WGT-14, WGT-15, WGT-16, WGT-18, WGT-22, WGT-23)
- dd124b7 Add the weights report route, the weights:write scope and the state fields (WGT-7, WGT-8, WGT-9, WGT-10, WGT-11, WGT-13, WGT-21)
- e65477b Run the v1.1.0 poll replay in CI and close three test gaps (WGT-2, WGT-5, WGT-17, WGT-19)
- 1cb441e Resolve weights in one place and charge through it (WGT-1, WGT-2, WGT-3, WGT-4, WGT-5, WGT-6, WGT-12)
- 35b377d Add migration 0008, the stored weights report and the reported_weights setting (WGT-7, WGT-8, WGT-17)
- 0d20688 Record D80 and the v1.1.0 compatibility fixtures (WGT-19, WGT-20)
- 9cabece Design: reported weights

## 3. Requirements and their tests

Each test name below was confirmed to exist by grep on this HEAD (78 distinct `TestWGT*` functions, plus `TestUI12_SetupsListReportsWhetherReportedWeightsAreOn`; every WGT id has at least one). The table comes from the independent final verification (task 12) and its fix (12b). That verification broke the code deliberately 25 times (12 in the UI, 13 in Go) and a test named after the requirement went red each time, except one gap that 12b closed with the UI-12 test named above. I did not repeat those break-and-restore trials; I re-ran all the tests.

"Task 12" is the final independent review; "12b" is the fix of its findings.

| Requirement | What it requires | Tests | File |
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
| UI-12 (added by 12b) | The setups list field `reported_weights_enabled` follows the setting, since the acquire warning depends on it | TestUI12_SetupsListReportsWhetherReportedWeightsAreOn | internal/api/weights_readers_test.go |

### The first client's eight acceptance scenarios

| Scenario | Passing test | End-to-end step |
| --- | --- | --- |
| A reported weight is charged (1.15 for 5 minutes is 96 milli-tokens) | TestWGT1_ReportedWeightIsCharged | W3 |
| A weight changes during a hold | TestWGT4_ChangedWeightPricesTheNextIntervalOnly | W3 |
| Falling back when reports stop or go stale | TestWGT3_EachFallbackCauseChargesAsWithoutTheFeature, _HoldMeteredBeforeItsSetupIsReported, _PendingReleaseCompletedAfterSetupLeftTheReport | W6 |
| Invalid values | TestWGT8_EachInvalidEntryRefusesTheWholeReport and 11 more WGT-8 tests | W5 |
| Every screen agrees with the charge | TestWGT14_EveryReaderShowsTheWeightThePollCharges | W2, W3 |
| Reading the applied weight back | TestWGT13_StateWeightFields | W2, W6 |
| The feature switched off behaves like today | the three TestWGT19 and five TestWGT20 tests | none (the end-to-end run has the feature on) |
| The permission check | TestWGT11_RouteNeedsWeightsWriteAndTheScopeOpensNothingElse, _ScopeAddedLastAndNotGrantedToExistingAccounts | W1, I9 |

## 4. Decisions recorded in `docs/decisions.md` (D80 to D91)

### Marked REVIEW (existing tests that had to change, listed first)

These are the places where a test that already existed was edited. None weakens a check; each is a consequence of something that cannot stay as written. Please read them.

- **D82(j)**: every assertion of the number of migrations or the schema version moves from 7 to 8, including `TestUPG1_ReleaseDatabaseUpgradesAtStartAndKeepsEveryNumber`, one of the tests of the older `d96a950` upgrade fixture. That is the only change to that test; the fixture and every balance comparison are untouched. A new migration cannot leave the number as it was.
- **D85(l)**: `TestUPG2_ResponsesAndSignInDifferFromTheReleaseOnlyByAddedFields`, also a `d96a950` test, compares whole response bodies with that release's and already stripped the fields added since. It now also requires and strips the fields this work adds to the ledger and setups responses. Every other comparison in it is unchanged. (A whole-body comparison cannot stay as written when a response gains a field.)
- **D86(c)**: a hold whose weight a Go metering policy sets has samples with no source in a database written by v1.1.0. Between the upgrade and the first poll after it, that hold and its burn figures show the resolver's weight instead of the policy's; no charge is affected, and the first poll records the source `policy`. The upgrade notes say so.

Also marked REVIEW in the file: **D84(g)**, which made the state route answer 500 when a stored rate rule cannot be evaluated (a v1.1.0 route that never did). The verification flagged it and **D86(a)** superseded it: the state route now always answers 200. D86(c) is not marked REVIEW in the file's text, but it is the kind of item you asked to see first, so it is listed above.

### All entries, one line each

- **D80**: The change is owner-directed. The client computes the weight and Hourglass applies it within limits; the weight has its own route and scope; "bill what a setup reserves" is amended to allow a reported reservation; `docs/SPEC.md` and the design are not edited.
- **D81**: The v1.1.0 compatibility fixtures live in `testdata/upgrade-v1.1.0/`, recorded from the tag before any product change, as D78 did for `d96a950`.
- **D82**: Storage and settings details: migration 0008 is additive; the report is validated in `internal/reported`; entry positions count from 0; the last refusal is kept in memory only; the setting's validation rules and error fields; the REVIEW items (j) and (k) above.
- **D83**: Resolver and collector details: the resolver takes the settings, rules and its own report type; it always evaluates the rate rules, so a broken rule still fails a poll for a setup with an override; attributes are never null.
- **D84**: Route details: the 409 message, the switch is checked before the body, every 400 is kept as the last refusal, the new entry-refusal wording, and the state fields. D84(g), which made the state route fail on a broken rate rule, is superseded by D86(a).
- **D85**: Reader details: a released hold's attributes are `{}`; admin views fall back to the default weight on a broken rule; ledger weight fields; the override; the collector view; the REVIEW item (l) above.
- **D86**: Rulings on the first round of verification: the state route never fails because of a broken rate rule; top-level report keys are matched without regard to case; the policy-hold window after an upgrade (REVIEW, above).
- **D87**: Page choices: a debit's weight sits in the Type cell ("Debit · 2× · weight 1.15", a range as "1.15 to 1.40"); the source tag shows for `reported`, `override` and, on a hold, `policy`; Settings sends `reported_weights` only when one of its fields was edited; the override field sends nothing, `null` or a number.
- **D88**: How WGT-19 and WGT-20 are tested against the v1.1.0 fixtures: three upgrade tests, and a response comparison with an explicit allowed list per route.
- **D89**: Rulings on the final verification: the WGT-20 list names both deliberate changes (the policy hold's weight 1.25 before its first poll, and Admin Users burn 6.5 instead of 2.5); the setups-list flag test; `docs/extending.md` says a free setup is `Meter: false` or an override of 0; end-to-end scenario choices.
- **D90**: The edits that the source of `docs/SPEC.md` needs for this work, as a numbered list. SPEC.md itself was not edited. These are for you to apply.
- **D91**: The first end-to-end run of the weights steps: four wrong expectations in the new test steps were corrected in the test, with no product change (setting path, a page reload after a report, the By day ledger row shown as a range, and the "last error" check on the collector route).

## 5. Verification rounds

| Round | What it checked | Found | Closed by |
| --- | --- | --- | --- |
| Task 04 (after the resolver and collector) | WGT-1 to WGT-6, WGT-12, WGT-17, with 30 deliberate breaks | No product bug. Medium: the v1.1.0 replay test was skipped under Bazel, so `./ci.sh` never ran it. Lows: no test that a policy weight beats an override; a missing check of the rule selector under `policy`; no test for `min: -0` | 04b: fixture files exported to Bazel and the skip turned into a failure; three new tests and a comment fix; each break re-run and red |
| Task 07 (after the route and readers) | WGT-7 to WGT-11, WGT-13 to WGT-16, WGT-18, WGT-21 to WGT-23, with 38 breaks | No high or medium. Low: the state route could answer 500 on a stored rate rule that cannot be evaluated, which v1.1.0 never did | 07b: the state route resolves like the admin views and always answers 200 (D86(a)) |
| Task 12 (final, whole job) | Every requirement, the UI, the docs against behaviour, the whole-job rules | No high findings. One medium: the decision listing the spec edits was missing. Lows: no test of the value of `reported_weights_enabled`; four wrong statements in the docs; an understated sentence in `progress.md`; D88(c) incomplete. It could not run the end-to-end steps | 12b and the orchestrator: D90 (spec edits), the new test, the doc corrections, D89 (completing D88(c)); the end-to-end run then passed (task 10, and again here) |

## 6. Docs

Present on this HEAD: the "Report weights" job in `docs/integrating.md`, the setting, the upgrade notes (migration 0008, the policy-hold window) and the four fields whose meaning shifts in `docs/deploying.md`, `docs/extending.md`, `docs/progress.md`, `docs/ui-notes.md` and `README.md`.

## 7. Release steps that remain (after your go-ahead and the version number)

1. `git merge --ff-only reported-weights` on `main`. It is a fast-forward: `git merge-base --is-ancestor main HEAD` succeeds.
2. `git push origin main`.
3. `git tag -a vX.Y.Z -m "..."` on that commit, then `git push origin vX.Y.Z`.
4. Watch the image workflow (`gh run watch`, or `gh run list --workflow release.yml`).

What the tag does: `.github/workflows/release.yml` runs on any pushed tag matching `v[0-9]*`. It does not run the tests. It logs in to ghcr.io and runs `deploy/push-images.sh TAG latest`, which pushes `ghcr.io/panset/hourglass`, `hourglass-api` and `hourglass-ui`, each as one multi-platform image (linux/amd64 and linux/arm64), tagged with the version and `latest`. **The tag therefore moves `latest`, which customers pull.** A tag with a hyphen (for example `v1.2.0-rc1`) is a pre-release: it is pushed under its own name and `latest` does not move. If you want a dry run first, push a hyphenated tag.

The v1.1.0 run of this workflow (checked with read-only `gh run list --workflow release.yml --limit 3`) completed with status success in 4 minutes 4 seconds. It was the only run listed.

## 8. The final runs on this HEAD

All run once, from a clean tracked tree, with the e2e ports free beforehand. Logs are in `.orchestrator/reported-weights/tasks/13-release-prep/`.

| Run | Exit code | Duration | Counts |
| --- | --- | --- | --- |
| `./ci.sh` | 0 | 7 s | 40 of 40 test targets pass; all 40 were served from Bazel's cache ("Executed 0 out of 40"), so this run shows nothing new by itself; the uncached run below is the real execution |
| `bazel test //... --nocache_test_results` | 0 | 56 s | 40 of 40 targets executed, 40 passed, 0 cached, 0 skipped targets. The final verification counted, from a verbose run of the same code, 755 top-level Go tests and 1,138 subtests, 0 failed, 0 skipped, and 251 UI tests. I did not recount those |
| `e2e/run.sh both` | 0 | 1545 s (bundled 780 s, split 764 s) | 16 of 16 steps pass in each mode: A0, A1, A2, A2g, A2f, A2p, A3, A4, A5, A6, A7, A7g, A8, A8p, A8c, A9 |
| `e2e/run.sh integration` | 0 | 1371 s | 17 of 17 steps pass: I0 to I8, W1 to W6, I9, I10 |

`e2e/out/` is git-ignored (confirmed in `.gitignore`).

## 9. What the released binary reports as its version

Built with Bazel without pushing anything (`bazel build //cmd/hourglassd:hourglassd //cmd/hourglassd:hourglassd-api //cmd/hourglass-ui:hourglass-ui`) and run with `version`. These are the binaries that go into the images; I did not load or run the image itself.

```
hourglassd 0.0.0-dev      (hourglassd, the bundled image's binary)
hourglassd 0.0.0-dev      (hourglassd-api, the API-only image's binary)
hourglass-ui 0.0.0-dev
```

The string comes from `internal/version/version.go` (`var Version = "0.0.0-dev"`). Nothing in the Bazel build or the release workflow stamps it from the tag, and `docs/deploying.md` says so ("The binary reports its version from `internal/version`, not from the image tag"). A binary from a `v1.2.0` image will therefore say `0.0.0-dev`. This was already true of v1.1.0.

## 10. Not done or not verified

**Gaps in this release step.**
- Each end-to-end mode has passed exactly as many times in this step as I ran it: once each (`both` covers bundled and split once each, `integration` once). Task 10 reported one passing run of the weights steps after fixing four wrong expectations (D91), so the whole `integration` mode has two passing runs in total, and timing-bound waits have not been exercised across many repetitions. The `e2e/` run is not repeated by CI.
- `./ci.sh` served every test from cache; the uncached Bazel run is the execution evidence. I did not rerun the `go test` outside Bazel, and I did not recount tests (counts quoted come from task 12).
- I did not run the product image itself, only the binaries built from the same targets.
- The scale test (the performance requirement) was run only with reported weights off. With the feature on, each user and admin request reads the stored report one more time; this looks negligible but was not measured at full size.
- I did not look at the UI in a real browser by hand. The end-to-end browser steps do exercise it.

**Documentation checks (nothing changed).**
- A page that is already loaded needs a refetch to show a changed report. The end-to-end test hit this (D91(b): the test reloads Setups after a report). ``grep` for refetch, reload, refresh, "already loaded", "loaded page" and "open page" in `docs/integrating.md` and `docs/ui-notes.md` finds no sentence saying so for reports, and its UI-12 to UI-14 paragraphs (`docs/ui-notes.md` lines 231 to 236) say nothing about it. `docs/ui-notes.md` does say that the 60-second poll and the reload after a release refetch the ledger page being shown (line 55), which is about the ledger and not the report. `docs/deploying.md` mentions reloading only for sign-in and TLS. So this is **not documented**.
- The By day ledger view shows a weight range ("1.15 to 1.40") for a debit whose polls had different weights. This is documented in two places: `docs/ui-notes.md` line 233 (the Type cell reads `Debit · 2× · weight 1.15 to 1.40`, "a range, two decimals", one value when `weight_min` equals `weight_max`) and `docs/deploying.md` line 624 (the added ledger fields `weight_min`, `weight_max`, `weight_sources`, null when unknown). `docs/integrating.md` does not mention it, which fits: it is a guide for the reporting service, not for the ledger. What is not stated anywhere I found is that a By day row sums several polls, so a range there is expected rather than an error; the By day rule is in `docs/ui-notes.md` line 55 and is not tied to the range.
- The binary's version string is not stamped from the tag (section 9).

**Gaps the build tasks reported (their own "Confidence and gaps" sections).**
- Fixtures (task 01): compaction of old debit rows is not exercised in the v1.1.0 fixture, and a hold before its first debit is not in it. The fixture was recorded with the `reported` provider and pressure source names.
- Storage (02): the check that a PUT of only the eight old settings keys answers as v1.1.0 compares against the in-process body, not the fixture; the fixture comparison is WGT-20's.
- Resolver (03): a stale comment on `Decision.Weight` in `pkg/hourglass/types.go` ("0 means use the weight from the rate rules") is slightly imprecise; the brief allowed changing only the `SetupState.Weight` comment, and `docs/extending.md` states the precedence.
- Route (05): the restart test reopens the same SQLite file in a new backend rather than starting a separate process.
- Readers (06): a ledger debit written by anything other than the collector would show null weights (none exists today); a rolled-up row's day is the local day of its creation time.
- UI (08): styling reuses existing classes and was not looked at by hand; the Settings page assumes the server's field name for the no-maximum error starts with `reported_weights`.
- Compatibility tests (09): only the feature-off case is compared; compaction after an upgrade is not re-tested for v1.1.0 data (the older fixture does it); the recorded v1.1.0 app configuration is not used at application level.
- End-to-end (10): one passing run when written; `bundled` and the acceptance spec were not touched by it.
- Docs (11): the examples were run against a real server; the sentence that users see weights on holds and debits is inferred from UI tests.
- Verification rounds (04, 07, 12): none started a real `hourglassd`; HTTP probes used the in-process router. Task 12 re-broke only one or two points per requirement on the final code and did not repeat the full campaigns of 04 and 07.
- Task 12 informational items left as they are: the end-to-end steps W3 and W4 check that a debit's amount is above 0 but not its arithmetic (covered by `TestWGT1_...`); W6 moves the test clock 11 minutes and could in principle time out, not mis-assert; a ledger range narrower than 0.005 shows the same number twice.

**Open points of the design for you to decide or pass on** (each has a default that was implemented; section "Open points for the owner" of the design):
- A Go policy's `Decision.Weight` still beats an admin's override. Say if the override should win.
- A Go policy or pressure source cannot see the reported weight.
- The default `max_age` is 10 minutes (an admin can raise it to 24 hours). A reporter that is down longer puts prices back on the rate rules while charging continues.
- A weight of 0 is accepted when the minimum is 0. An admin who never wants a free setup sets a minimum above 0.
- A bare `NaN` in a report is invalid JSON, so that one error cannot name the setup; the client is told in the docs.
- The last refused report is kept in memory and is lost at a restart. Say if it should be stored.
- The weight on older ledger entries depends on `usage_samples`, kept 13 months; shortening that retention makes older weights show as unknown.
- Reports are not audited one by one; each charge records its weight and source and the stored report names its account. Say if a log of weight changes per setup is wanted.
- Pass to the first client the choice of reporting every setup or only deployed ones, with its two consequences (design section E).
- Apply the edits listed in D90 to the source of `docs/SPEC.md`, which this work did not touch.
- The version number of the release, and whether the agent merges and pushes or hands over (the INTENT says it hands over).
