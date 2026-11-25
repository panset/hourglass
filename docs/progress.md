# Progress

Last updated: 2026-11-24

Status of Hourglass v1 against the spec in [SPEC.md](SPEC.md). Decisions are
in [decisions.md](decisions.md). The task-by-task record (briefs, results,
verification reports) is under `.orchestrator/hourglass-v1/`.

## Summary

All ten milestones are built, verified by agents that did not write the
code, and pushed. The four "done" criteria hold:

| Criterion | Status | Evidence |
|---|---|---|
| From a clean checkout, `bazel build //...` and `bazel test //...` pass, and plain `go build ./...` gives an API-only binary | Yes | Fresh clone of the pushed repository: 254 targets built, 35 of 35 test targets pass; the `go build` binary answers `/healthz` and serves no UI. `./ci.sh` passes without `node_modules`. |
| The spec's acceptance path passes end to end in bundled and split mode | Yes | `e2e/run.sh both`: all eight acceptance steps pass through the real UI in a headless browser in each mode (about 12 minutes per mode). |
| HTTP tests cover every cell of the permissions table, including that a user with no role gets no data from any route | Yes | `internal/api/permissions_test.go`: every v1 cell of the table, and all 30 routes for each of 8 kinds of caller (240 combinations). A deliberately seeded leak turned the suite red. |
| Everything is pushed and this file is current | Yes | `main` equals `origin/main` on `github.com/panset/hourglass`. |

Nothing has been run against a real vCenter. See "First run against a real
vCenter" below.

## Changes to the spec made at the owner's direction (2026-11-20)

These two changes were requested after v1 was complete. They are built,
independently verified and pushed. `./ci.sh` passes on them (35 of 35 Bazel
test targets) and the end-to-end acceptance test passes on them in both
bundled and split mode.

- **D40:** the core no longer has a resource-name template or a per-setup
  override (supersedes SET-4). A provider's allocation belongs to a setup
  when its reported name equals the setup's name. The `vsphere` extension
  takes `pool_name_prefix` and `pool_name_suffix` in its own config; the
  reference scenario uses suffix `_RP`. Migration 0004 drops the column.
  Renaming a setup while it is held is refused.
- **D41:** admins land on Setups after sign-in; My usage moves from `/` to
  `/usage`.

`docs/SPEC.md` is a copy of the source document and still shows the old
text for both; the source needs the same edits.

## The integration API (owner-directed, 2026-11-20)

Built to [design/integration-api.md](design/integration-api.md) (D42 to D63). The task record is under `.orchestrator/integration-api/`. `docs/SPEC.md` is not edited; D42 lists the edits the spec's source needs.

**Finished**

- Service accounts: admin routes and UI page, scoped bearer tokens (`hgsa_<id>_<secret>`, hash stored, shown once), expiry, revoke, audit, a failed-attempt rate limit per client address (SVC-1 to SVC-8).
- `PUT /api/v1/integration/deployments` and `/pressure`, `GET /api/v1/integration/state`, `POST /api/v1/integration/holds/{id}/release`, in `api/openapi.yaml` (INT-1 to INT-4, INT-8, HOLD-8).
- The `reported` provider and pressure source, with `max_age`, and the public `pkg/storedreports` they read through (INT-5 to INT-7). `check-provider` prints report ages.
- Pending release: a user's release of a still-reported setup is accepted, marked Releasing, fires `hold.releasing`, and completes at the first poll where the setup is gone (HOLD-6, HOLD-7, HOLD-9, CMP-1, CMP-2). `still_deployed` is gone (D43, D59).
- UI: Service accounts page, Releasing state, last report times on Settings.
- An end-to-end scenario with a scripted reporter and webhook receiver (`e2e/run.sh integration`, 10 of 10 steps, about 13 minutes) and the acceptance path updated for the new release.
- Documentation: `docs/integrating.md` was run example by example against a live `reported` stack and corrected; `extending.md`, `deploying.md` (config, `max_age`, service accounts, proxy note, upgrade entry for migration 0005), the README and `ui-notes.md` describe what was built. Three verification tasks (after service accounts, after the release changes, after the end-to-end scenario) each found issues that were fixed and re-checked.

**Not finished, not verified, or left as is**

- Nothing in this work has been run against a real vCenter, and the `reported` extensions have only been driven by scripted reporters.
- The rate limit counts failures per connection address and does not read `X-Forwarded-For`, so behind a proxy all callers share one failure budget; valid tokens are never refused (D47).
- Events stay at most once and in memory: a receiver that is down misses `hold.acquired` and `hold.releasing` for good. The state route is how a service catches up (documented in the guide).
- If no poll has succeeded since the process started, a user's release is immediate even when the setup is deployed (D61).
- OIDC users have an empty `username` in event JSON and in the state response; the email is the stable key.
- The integration end-to-end scenario is not part of `bazel test //...` (D63).
- `docs/SPEC.md` and the spec's source still describe the old HOLD-2 and lack the integration requirements (D42).

## Ledger roll-up, labels, greeting, pre-approved users (owner-directed, 2026-11-23)

Built on the branch `ledger-labels-onboarding`, cut from `main` at `d96a950`, to [design/ledger-labels-onboarding.md](design/ledger-labels-onboarding.md) (D69 to D79). The task record is under `.orchestrator/ledger-labels-onboarding/`. Nothing here is merged to `main`, tagged or released: that waits for the owner's go-ahead and version number. `docs/SPEC.md` is not edited; the edits its source needs are listed in D79.

**Built**

- Ledger roll-up: migration `0006` (a `rollup_count` column, a marker table, a delete trigger that allows only the compaction of debit rows, two partial indexes) and a bounded daily compaction step run by the collector: per-poll debits of days earlier than seven days before today become one row per user, hold and day, with the same total, id order and balances (LED-7 to LED-10, LED-15; D70, D71).
- `GET /api/v1/me/ledger`: `rollup=day`, `limit`, `cursor`, `entry_count`, `next_cursor`; without the new parameters it answers as before (LED-11 to LED-14; D72). My usage pages the ledger 20 rows at a time, By day or Every poll (UI-8; D73).
- Label discovery: `label_index` on `GET /api/v1/setups` and "Available labels" chips on Setups (SET-8, UI-9; D74). (History: the chips were replaced in v1.3.0 by the Label selector box's suggestions, D96.)
- Greeting: name fallback claims at sign-in, `display_name` in the session and "Hello, {name}" in the top bar (AUTH-9, UI-10; D75).
- Pre-approved SSO users: migration `0007`, the sign-in step, three admin routes, the optional `preapproved_users_file`, and the Pre-approved users section on Users (PRE-1 to PRE-8, UI-11; D76, D77).
- Compatibility tests against fixtures recorded from the previous release (`testdata/upgrade-d96a950/`) (UPG-1 to UPG-3; D78).
- Independent verification ran after milestones 3, 5 and 7 and its findings were fixed in their own commits.
- Documentation: `docs/deploying.md` (upgrade entry for migrations 0006 and 0007, data retention and disk space, the timezone note, the configuration reference, the "Pre-approving SSO users" section), `docs/ui-notes.md`, `README.md`, `deploy/README.md` (a fourth development user, `dave`) and `CONTRIBUTING.md` (the compaction scale test and the upgrade fixtures).

**Not finished, not verified, or left as is**

- The commands in "Pre-approving SSO users" in `docs/deploying.md` were run against the bundled stack built from this branch and corrected where they failed (the first-run password change, blank lines in a pasted list).
- The new end-to-end steps (A2g, A2f, A2p, A7g, A8p, A8c) have run in a browser: `e2e/run.sh both` passed 16 of 16 steps in bundled mode and 16 of 16 in split mode, and `e2e/run.sh integration` passed 11 of 11, on 2026-11-23.
- The roll-up window (seven days) is fixed. Compacted rows cannot be expanded again, and an older binary refuses the upgraded database; the way back is the backup taken before the upgrade.
- Disk: the file stops growing at the old rate but does not shrink until an offline `VACUUM`.
- The release itself (merge, version number, tag, image workflow) has not been done. No version tag exists yet, and `internal/version` has not been checked for what the released binary reports.
- `docs/SPEC.md` and the spec's source still lack the new requirements and table rows; D79 lists the edits they need.

## Reported weights (owner-directed, 2026-11-23)

Built on the branch `reported-weights`, cut from `main` at `753cc50` (release `v1.1.0`), to [design/reported-weights.md](design/reported-weights.md) (D80 to D88, and the entries after them). The task record is under `.orchestrator/reported-weights/`. Nothing here is merged to `main`, tagged or released: that waits for the owner's go-ahead and version number. `docs/SPEC.md` is not edited; the edits its source needs are listed in D90, the last decision of this work.

**Built**

- A customer's service reports a weight per setup on `PUT /api/v1/integration/weights` under the new scope `weights:write`, stored whole and replaced atomically (migration `0008`; WGT-7 to WGT-11, WGT-21). An invalid entry refuses the whole report with a message that gives its position, counted from 0, and its setup name; the last refusal is kept in memory for admins.
- One resolver (`internal/weights`) gives every reader the same weight and its source: override, usable reported weight, first matching rate rule, default (WGT-2, WGT-3, WGT-12, WGT-14). The collector charges the resolved weight, records the source on every usage sample (`weight_source`) and hands the pressure source and the policy the base weight, so a report cannot move pressure or the multiplier (WGT-1, WGT-4 to WGT-6).
- Admin controls: the `reported_weights` setting (switch, minimum, maximum, `max_age`; off by default; audited; optional on `PUT`) and the per-setup `weight_override` (WGT-17, WGT-18).
- Reading back: the weight fields on the state route, the Setups list, the admin setup views, holds and burn figures, ledger entries (`weight_min`, `weight_max`, `weight_sources`), and the collector view with the last accepted and last refused report (WGT-13, WGT-15, WGT-16, WGT-22, WGT-23).
- UI: weight tag and attributes on Setups and the acquire dialog, weight on holds and ledger debits in My usage, weight and override on Setup inventory, the "Reported weights" section of Settings, and the fifth scope on Service accounts (UI-12 to UI-14).
- Compatibility with `v1.1.0`: fixtures recorded from the tag (`testdata/upgrade-v1.1.0/`) and tests that a database from that release upgrades, charges and answers the same with the feature off, apart from the added fields and the two deliberate changes (WGT-19, WGT-20; D81, D88). The `d96a950` fixtures and their tests are unchanged except for the schema version number (D82) and, in `TestUPG2_...`, the requirement that the added setups, hold and ledger fields are present and are stripped before the unchanged whole-body comparison (D85(l)).
- Documentation: `docs/integrating.md` has a fifth job, "Report weights", written from the code as built; every command and response in it was run against a real `hourglassd` built from this tree (transcript in `.orchestrator/reported-weights/tasks/11-m7c-docs/examples.log`). `docs/deploying.md` has the setting, what tightening the limits does, the override, the `max_age` advice and the upgrade entry for migration `0008`; `docs/extending.md`, the README and `docs/ui-notes.md` are current.

**Not finished, not verified, or left as is**

- The examples in the guide were run against a host process on 127.0.0.1, not the bundled compose stack, and with the local admin only: a charge at a reported weight, and the Setups page and acquire dialog, were not exercised there. Those are covered by the unit tests named after WGT-1, WGT-4, WGT-14 and UI-12 to UI-14 and by the end-to-end integration scenario, whose run belongs to the milestone's verification task.
- The last refused report is kept in memory and is lost at a restart (D20).
- A Go metering policy and a pressure source cannot see a reported weight. A field can be added later without breaking anything.
- The ledger weight fields depend on `usage_samples`, kept 13 months; older entries show the weight as unknown.
- The top-level keys of a weights report are matched without regard to case, as in the other two reports (D86).
- Between the upgrade and the first poll, a hold priced by a Go metering policy shows its rate-rule weight on screens; charges are not affected (D86).
- The release itself (merge, version number, tag, image workflow) has not been done.
- `docs/SPEC.md` and the spec's source still lack the new requirements and table rows.

## A Setups page that leads with cost (owner-directed, 2026-11-24)

Built on the branch `setups-declutter`, cut from `main` at `f1a05c2`, to the change request [design/setups-declutter.md](design/setups-declutter.md) (D93 to D95) and its [amendment](design/setups-declutter.md#amendment-2026-11-24), made by the owner the same day after the first screenshots (D96 to D99), for release `v1.3.0`. A senior UX designer specified the revised page (D99). The task record, with the design plans, their review and the UX specification, is under `.orchestrator/setups-declutter/`. `docs/SPEC.md` is not edited; D98 (which replaces D95) lists the edits its source needs.

**What changes**

- The Setups table's columns are Setup, State, Held by (`You · 3 h`, "overdrawn" in words), Cost (tokens / h) (`2.0 now · up to 6.0`, with the `reported` or `override` tag beside it) and actions. There is no Weight, Holder's tokens, Held for or label column (D96(a)).
- A details panel, opened from the cost cell or "Details", explains the price (Cost now, Up to, Charging, then weight and source, attributes, pressure and multiplier) and shows the holder's balance, every label (the only place labels are shown, D96(b)) and the details JSON. It takes its multipliers from the new additive `pricing` object on `GET /api/v1/setups` and shows no equation with an equals sign (D94(a), (b)).
- The per-key label dropdowns and the "Available labels" chips are gone. The Label selector box is a combobox that suggests label keys, operators and values from `label_index` as you type (D97). A label value with no selector term is suggested after `key=` and, when picked, becomes an exact-match token, applied in the browser, which replaces the first build's dashed chip (D96(d), D97(f) to (h)). A line under the filter row shows the tokens, "{n} setups shown" and "Clear filters" while a filter is in force (D99(f)).
- The "Display label keys" field has left Admin, Settings. The setting stays in the API and the database, unread; the page sends the stored value back unchanged (D96(e), D97(i), (j)). The quick start, the seed and the deploy notes no longer ask an admin to set it.
- Unchanged: the acquire dialog, My usage, every admin page other than Settings (Setup inventory keeps its Weight column), and the API's existing fields.

**Requirements and their tests**

Unit tests are in `ui/src/pages/setups/setups.test.tsx` unless another file is named; end-to-end steps are in `e2e/browser/tests/integration.spec.ts` (`e2e/run.sh integration`) or, for A-steps, `e2e/browser/tests/acceptance.spec.ts` (`e2e/run.sh both`).

| Id | Requirement | Tests |
|---|---|---|
| UI-15 (amended, D96) | The Setups table's columns are Setup, State, Held by, Cost and actions. It has no Weight column, no Holder's tokens column and no label column. | `UI-15 the table's columns are Setup, State, Held by, Cost and actions, whatever display_label_keys holds`; end-to-end U20 (the five headers, as a developer and as an admin) |
| UI-16 | The cost cell shows `cost_per_hour_tokens.now` and `.at_peak` with their unit, one value when they are equal, and the source tag when `weight_source` is `reported` or `override`, and none for `rule` or `default`. | `UI-16 the cost cell shows now and at_peak with the unit, and one value when now is not below at_peak`; `UI-16 the cost cell carries the source tag for reported and override, and none for rule or default`; Go `TestUI18_SetupsListCarriesTheMultipliersOfItsCosts` (case `now > at_peak`); end-to-end S1, S2 |
| UI-17 | The Held by cell shows the holder, how long, and an overdrawn marker in words when `holder.overdrawn` is true. | `UI-17 the Held by cell shows the holder or You, how long, and "overdrawn" in words`; end-to-end U20 |
| UI-18 | The details panel opens from the cost cell and from Details, and shows the weight, its source, its attributes, the current pressure and multiplier, both costs, when charging starts, the holder's balance when held, every label, and the details JSON. It shows the price-change sentence whenever `reported_weights_enabled` is true. | `UI-18 the panel opens from the cost cell and from Details, with focus on Price, and returns focus on close`; `UI-18 the panel shows weight, source, attributes, pressure and multiplier, both costs, charging, the holder, every label and the details JSON`; `UI-18 the panel shows the price-change sentence whenever reported_weights_enabled is true`; `UI-18 the multiplier and pressure in the panel are those of the setups response`; `UI-18 the panel opens without pricing in the response`; `UI-18 the panel does not show a highest-band cost below the cost now`; `UI-18 an open panel follows a refresh; when its setup is no longer listed it keeps one response's values and focus returns to the heading`; Go `TestUI18_SetupsListCarriesTheMultipliersOfItsCosts`; end-to-end W2, U20 |
| UI-19 (amended, D96) | The page has no per-label dropdown and no label chips. Labels are filtered with the selector box (UI-21) and with exact-match tokens (UI-22). The State filter works as before. | `UI-19 the page has no per-label dropdown and no label chips; the State filter and the selector box work`; end-to-end A2f (no "Available labels" heading) |
| UI-20 (amended, D96) | At 1280 px the Setups table does not scroll horizontally. | `UI-20 every column of the Setups table is one line` (mechanism only); end-to-end U20 (the real test, Chromium, with a 32-character setup name) |
| UI-21 (new, D96) | The Label selector box is a combobox that suggests, for the caret's position, label keys, operators, or the values of the key being written, all taken from `label_index`. Picking a suggestion replaces the token under the caret; picking a value after `=`, `==` or `!=` applies the filter. It is operable with the keyboard and by touch, and names itself to assistive technology. A selector typed by hand is sent as typed and an invalid one shows the API's message, as before. | `ui/src/pages/setups/selectorSuggest.test.ts`: `UI-21 suggests keys at the start of a requirement, after a comma and after !`; `UI-21 suggests the values of the key after =, == and !=, and nothing for a key that is not in the index`; `UI-21 suggests operators after a key and a space`; `UI-21 inside an in or notin list suggests values that have a term, are not empty and are not in the list yet`; `UI-21 a comma inside parentheses does not start a new requirement`; `UI-21 matches ignore case; prefix matches come before contains matches, each in the index's order`; `UI-21 renders at most 50 options and reports how many more match`; `UI-21 a pick replaces the whole token under the caret, also from the middle, and puts the caret after it`; `UI-21 a key that is a prefix of another key is completed only when picked`; `UI-21 the empty value is offered as (empty) and leaves key=`. `setups.test.tsx`: `UI-21 the selector box is a combobox: typing opens the list, Arrow Down and Enter pick, and a picked value filters through the API`; `UI-21 Enter with no active option submits the typed text; Escape closes the list and keeps the text; Tab picks nothing`; `UI-21 the list opens on click and on Arrow Down in an empty box, and not on focus alone`; `UI-21 aria-activedescendant follows the active option and the status line announces the count`; `UI-21 with an empty index or no label_index the box is a plain text box with no placeholder`; `UI-21 keys pressed during composition are left to the input method`; `RULE-2 the label selector goes to the API as selector= and a parse error is shown` (unedited); end-to-end A2f, U22 |
| UI-22 (new, D96) | A label value that cannot be written in a selector is suggested after `key=` and, when picked, becomes an exact-match token. The token filters the fetched list in the browser, combines with the selector and the State filter, holds one value per key, never reaches the API, and is removed with its own button. | `selectorSuggest.test.ts`: `UI-22 a value with no term is offered only after = and ==, and its pick removes the requirement from the text`. `setups.test.tsx`: `UI-22 picking a value with no selector term adds an exact-match token, filters the rows in the browser and sends nothing to the API`; `UI-22 a token combines with the selector and the State filter; another value of the same key replaces it; the same value again changes nothing`; `UI-22 removing a token restores the rows and returns focus to the selector box`; `UI-22 a token whose value is no longer in label_index stays shown and removable`; end-to-end U22 |
| UI-23 (new, D96) | The Settings page has no display label keys field. Saving settings sends the stored `display_label_keys` back unchanged. | `ui/src/pages/admin-settings/admin-settings.test.tsx`: `UI-23 Settings has no display label keys field and saves the stored keys unchanged`; `UI-23 every other setting saves as before, with the stored keys alongside`; `UI-23 a server error for display_label_keys, which has no field now, shows in the page`; end-to-end A0 (no "Display label keys" step) |
| UI-9 (replaced by UI-21 and UI-22) | The placeholder is taken from `label_index`. | `selectorSuggest.test.ts`: `UI-9 the placeholder is the first term of the index that is at most 16 characters, after the words Type or pick a label` (moved from `setups.test.tsx`; the UX specification words the placeholder "Type or pick a label, e.g. {term}", the term at most 16 characters, so the test's wording follows the built name); end-to-end A2f (placeholder) |
| UI-12 (amended) | The Setups page shows the source tag on the cost cell and the weight, source and attributes in the details panel; the acquire dialog clause is unchanged. | `UI-12 the cost cell carries a tag for reported and override, and none for rule or default`; `UI-12 the panel shows the weight, its source and the attributes; the row shows none of them`; the `UI-12 the acquire dialog …` tests (unchanged); Go `TestUI12_SetupsListReportsWhetherReportedWeightsAreOn` (unchanged); end-to-end W2 |
| SET-6 (retired, D96) | `display_label_keys` is stored, validated and returned by the API for compatibility. No page reads it and the Settings page does not edit it. | `the populated table shows holder, overdrawn word and costs, and no label column` (was `SET-6 populated …`); Go `TestSET6_SetupsListForDeveloper` and the settings tests (unchanged; they pin the kept API) |
| WGT-14 | Still met: the Setups list's weight and source equal what a poll records, read from the panel. | `UI-12 the panel shows the weight …`; Go `TestWGT14_EveryReaderShowsTheWeightThePollCharges` (unchanged); end-to-end A3 (reads the label from the panel), W2 |

Also: `ui/src/components/components.test.tsx` `Modal keeps Shift+Tab inside when a heading has focus` and `Modal returns focus to the fallback when the opener is gone`; `ui/src/lib/format.test.ts` `formatRate`.

**Not finished, not verified, or left as is**

- UI-20 is checked in a real browser only by the integration scenario, in Chromium. Firefox and Safari were not measured. On a window narrower than the table, the table scrolls sideways inside its own scroller, as every Hourglass table does; stacked rows for phones are proposed for later (D99).
- A free setup has no "Charging starts" line, because the server never sends `metering_description` (D94(e)).
- The top bar and the acquire dialog read `GET /api/v1/pressure` and can differ from the panel for up to one poll (D94(c)).
- A stored `display_label_keys` value stays in the database and in `GET /api/v1/setups`, unread; it can be changed only through the API (D97(j)).
- Filters, tokens included, are lost on reload (D97(h)).
- The wireframes `docs/wireframes/project/Setups.dc.html` and `AdminSettings.dc.html` were not redrawn; `docs/ui-notes.md` says they are behind.

## Milestones

| # | Milestone | Status |
|---|---|---|
| 0 | Repository: license, README stub, .gitignore, spec copy | Done |
| - | Spike: govmomi simulator resource pools and cluster figures | Done (D5 to D8) |
| 1 | Build skeleton: Go module, Bazel bzlmod, rules_go, Gazelle, CI script | Verified |
| 2 | Pure core logic: selectors and rate rules, bands and smoothing, debits, ledger and rollover | Verified |
| 3 | SQLite store and migrations | Verified |
| 4 | Extension API, fakes, conformance suites, `inventory`, `default`, `static`, `webhook` | Verified |
| 5 | Collector and event queue (plus settings, ledger service, hold rules) | Verified |
| 6 | Auth and JSON API, `api/openapi.yaml`, contract test | Verified |
| 7 | React + TypeScript UI | Verified |
| 8 | vSphere provider and pressure source, `check-provider` | Verified |
| 9 | Images, compose stacks, end-to-end acceptance path in both modes | Verified |
| 10 | README quick start, `docs/extending.md`, `docs/deploying.md`, `CONTRIBUTING.md`, `examples/custom` | Verified |

## What verification found and what was done about it

Each milestone was checked by a separate agent working in a clean clone.
The findings that mattered:

- **Store:** ledger rows could be rewritten with `INSERT OR REPLACE`; a
  panic inside a transaction left the write lock held. Both fixed.
- **Metering:** a clock stepping backwards could double-charge; a poll at
  exactly midnight on the 1st could run ahead of the rollover. Both fixed.
- **Security:** an admin could save a resource-name template that hung the
  collector (the template has since been removed, D40); a route registered outside the route table would have escaped
  the permissions tests; rule changes were audited without before and after
  values; deleting every setup would re-apply the seed. All fixed. No
  critical or high findings; a user with no role got no data from any
  route under attack, and planted secrets never appeared in logs, responses
  or the database.
- **Performance:** the admin Users list took about 0.6 s at spec scale
  (200 users, 100 setups, 18 days of 5-minute polls) against the 0.5 s
  limit. Fixed: every route now answers in under 0.5 s on that data (the
  Users list in about 0.15 s) and a poll takes about 0.2 s. A scale test
  guards it.
- **Balance cache:** the first version of that performance fix let the
  cached balance drift after a rollover that closed two or more months.
  Verification caught it before it was pushed. Fixed and re-verified with
  200 randomised operation sequences: the cached balance equalled the
  ledger after every step. The ledger itself was never wrong.
- **UI:** a failed page load left a blank page; side panels did not take
  keyboard focus. Both fixed. The pages were viewed in a real browser at
  two desktop widths with no clipped, overlapping or missing content.

## Not finished, not verified, or left as is

- **Real vCenter:** nothing has been run against one. All vSphere code is
  tested against a hand-written fake and the govmomi simulator only.
- **Stray binary in history:** a compiled 32 MB `simulator` binary was
  committed by mistake in `b349a63` and removed in `74c76ba`. It is still in
  the pushed history; removing it needs a history rewrite (D39).
- **Commit size:** several commits are milestone-sized (thousands of lines)
  rather than small.
- **Admin lockout:** the local admin sign-in limit is per account, so
  anyone who can reach the sign-in route can keep it locked out (D31).
- **Debt and role removal:** debt is not carried for a user who has no role
  at the moment of a rollover (D23).
- **Two processes on one database:** two rollovers running at the same
  instant could write a carry-over twice. The spec is single-instance and
  the process serialises its own rollovers, so this needs two processes on
  one database file, which `docs/deploying.md` says not to do.
- **Reference scenario settings:** the `size` display column is an admin
  setting, set once in Settings after first start; the seed file covers
  setups and rate rules only (D33).
- **Not covered:** mobile screen widths; accessibility beyond keyboard
  focus in dialogs and panels; a browser download quirk on this machine
  (Playwright's Chromium install can hang; `e2e/README.md` has the
  workaround).
- **Out of scope by the spec:** deploying or tearing down anything, the
  manager role and team budgets, notifications, token transfers.

## First run against a real vCenter

Mocks cannot settle these (from the spec, with what was learned):

- [ ] The read-only vCenter role has every privilege the provider needs
      (the built-in Read-only role appears sufficient; inferred, not run).
- [ ] The real resource pool names match `pool_name_prefix` /
      `pool_name_suffix` in the vsphere config. Clusters inside
      host sub-folders are not discovered (D27).
- [ ] The pressure source's cluster numbers agree with what vCenter shows.
      They are read as reservations, not live usage (D6).
- [ ] TLS and certificate settings work against the real endpoint with
      `insecure_skip_verify: false`.
- [ ] A poll finishes well inside the poll interval at real inventory size.

`hourglassd check-provider` supports that run.

## Decisions to review

All decisions are in [decisions.md](decisions.md). The ones marked
**REVIEW** there:

| Decision | In one line |
|---|---|
| D3 | Orchestration files (`.orchestrator/`) are tracked in this public repository. |
| D6 | vSphere "allocation" is read as reservations. |
| D8 | The local stacks use a simulator image built from this repository. |
| D10 | An `outages` table was added to the spec's 15 tables. |
| D11 | Band lower bounds are inclusive (exactly 50% gives 1x). |
| D12 | A gap longer than twice the poll interval is not charged at all. |
| D13 | Label values with spaces or punctuation cannot be named in a rule. |
| D19 | Charging starts at the poll after metering starts; time between the last poll and a release is not charged. |
| D22 | A debit is booked in the month of its poll. |
| D23 | Debt is not carried for a user with no role at rollover. |
| D27 | Clusters inside vCenter folders are not discovered. |
| D28 | Third-party builds import `pkg/hourglass/server` as well as `pkg/hourglass`. |
| D29 | The end-to-end run uses a test main with a movable clock. |
| D30 | Three build- or test-only npm packages with licenses outside the spec's list are allowlisted. |
| D31 | The local admin sign-in limit is per account. |
| D39 | The stray binary stays in history unless you ask for a rewrite. |
| D40 | Owner-directed: resource naming moved out of the core into the provider. |
| D41 | Owner-directed: admins land on Setups; My usage is at `/usage`. |
| D43 | Owner-directed: a user's release of a still-deployed setup is accepted and pending, not refused (`still_deployed` is gone). |
| D60 | `hold.releasing` is queued inside the transaction; three residual cases (failed commit, webhook before the state shows it, owner lookup failure) are covered by idempotent handlers and reconciliation. |
| D47 | Failed service-token attempts are limited per connection address; a valid token is never refused; behind a proxy all callers share one failure budget. |
| D61 | A release made with no successful poll since start-up is immediate even if the setup is deployed; release is decided from the last poll, not the stored report. |

Also worth a look, though not marked: D33 (reference-scenario settings are
not seedable), D34 and D38 (what the end-to-end test does outside the UI),
D35 (the OIDC callback always redirects to `/`), D36 (how the balance
cache is kept), D42 (the integration API and the spec-source edits it needs), D47 (the rate limit behind a proxy), D44 to D46 (the stored-report rules), D48 (service-account
naming and token format), D50 (a deleted account's name can be reused), D51
to D55 (how the core learns about `reported` and how the extension reads the
reports), D57 and D58 (repeat release clicks; the completing poll is still
charged), D60 (`hold.releasing` is queued inside the transaction).
