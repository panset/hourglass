# Review of the design plan: a Setups page that leads with cost

Date 2026-11-24. Reviewed: `.orchestrator/setups-declutter/tasks/01-design/DESIGN.md` against `main` at `f1a05c2`, the change request `docs/design/setups-declutter.md` and `INTENT.md`. Nothing outside this task folder was written.

Words used here: "is" marks what I read in the code or in the design's own files. "Inference" marks a conclusion that depends on browser behaviour I did not measure. Paths are relative to the repository root.

## Verdict

**Build with the listed changes.** The server analysis is correct, the additive `pricing` object is sound and free, and the panel design meets the request. Three things must change in the design before the build starts, and one owner question should be put with corrected facts. None needs a redesign.

Counts: 3 must fix, 9 should fix, 9 notes.

## Must fix before build

### M1. The new UI crashes against an older API, because `pricing` is read as if it were always there

- **What.** The design makes `pricing` a required field and the panel reads `pricing.pressure`, `pricing.multiplier` and `pricing.peak_multiplier` (DESIGN.md sections 2 and 4). It says nothing about a response that has no `pricing`.
- **Evidence.** Split mode runs the API and the UI as two images that are upgraded separately (`docs/deploying.md:11`, `docs/deploying.md:588-592` gives no order). A render error replaces the whole page with "Something went wrong" (`ui/src/shell/Layout.tsx:19`, `ui/src/components/ErrorBoundary.tsx:33-48`). The page already tolerates a missing newer field: `data?.label_index ?? []` (`ui/src/pages/setups/index.tsx:75`), `data?.reported_weights_enabled` (`index.tsx:358`). Three test fixtures serve a setups response without the newer fields (`ui/src/shell/Gate.test.tsx:127`, `ui/src/pages/login/login.test.tsx:75`, `ui/src/pages/change-password/change-password.test.tsx:52`).
- **Failure.** A split installation pulls `hourglass-ui` v1.3.0 while `hourglass-api` is still v1.2.1 (or rolls the API back). A user clicks a cost cell. `data.pricing` is undefined, the panel throws, and the Setups page is replaced by the error screen. Reload does not help until the API is upgraded.
- **Change.** State in section 4 that the panel reads `data.pricing?.…` and leaves out the "Lab pressure" and "Multiplier now" lines and the `(3×)` when `pricing` is absent; both costs still show. Add a unit test `UI-18 the panel opens without pricing in the response`. Keep the field required in the schema (the server always sends it). The other direction (old bundle, new API) is safe: the old page ignores the field.

### M2. The label-column mechanism squeezes every other column to its narrowest width, and the product's cells can wrap where the prototype's cannot

- **What.** Section 3 says "the other columns keep their natural width". That is true only in the prototype, where every non-label cell is `nowrap`. With percentage widths that add up to 100% on the label columns, the browser gives the other columns their minimum content width. The design saw this once and patched one place: "the cost cell's second line has a fixed width of 12 em so the browser does not squeeze it to one word per line".
- **Evidence.** Prototype: `.link{…white-space:nowrap}`, Held by `td class="nowrap"`, the "requested" line has an inline `white-space:nowrap` (`tasks/01-design/proto/measure.cjs:13,35,37`). Product: `.linkButton` has no `white-space` (`ui/src/pages/setups/setups.module.css:18`); `.reason` is `display:block; max-width:18em` and wraps (`setups.module.css:16`), and it carries "requested <time>" in the actions cell (`ui/src/pages/setups/index.tsx:215-219`); the Held by column is not `nowrap` (`index.tsx:188`). The squeeze to minimum width is the design's own observation; that the product cells then wrap is inference from the CSS, not measured.
- **Failure.** The build follows section 3 as written. In a browser the cost button reads `8.0` / `now ·` / `up to` / `12.0` on four lines, "Sam Lee · 5 d 2 h" breaks after each word, and "requested Nov 18, 17:30" stacks. No named test fails: jsdom has no layout, and the end-to-end check only asserts that the table does not scroll, which a squeezed table satisfies better.
- **Change.** In section 3, replace "keep their natural width" with the rule: every cell that is not a label cell is one line (`white-space: nowrap`) except the two second lines, which have a stated width. List the classes that need it: the cost button, the Held by cell, the "requested" line. Add to step U20 an assertion that the cost button of a row is one line high (its height equals one line box), so the squeeze cannot pass.

### M3. The figure given to the owner in Q2 is wrong, and UI-20 holds in a narrower case than the design states

- **What.** The design says a label column shows "8 to 9 characters" at 1280 px with five keys. Its own screenshot shows 6 characters and an ellipsis, and 3 for wide letters. The ellipsis was not counted.
- **Evidence.** `tasks/01-design/proto/layout-1280-five-keys.png`: `abcdef…`, `10.200…`, `vcente…`, `WWW…`; headers `form-fac…`, `source-o…`. DESIGN.md section 3 ("about 64 px of text (8 to 9 characters)"), Q2 and draft D94(h) repeat the higher figure.
- **Failure.** The customer's columns hold host names, subnets and vCenter names. Values of one column usually share a prefix, so a column reads `10.200…` or `vcente…` on every row. The page that was meant to be less cluttered now has five columns that say nothing without a click. The owner accepted Q2's default on "8 to 9 characters".
- **More that narrows UI-20** (inference from the prototype's widths; the spare width at 1280 px is about 100 px in total):
  - A setup name over about 17 characters scrolls the table (the design says so).
  - For an admin, a disabled setup has a second "Disabled" badge in the Setup cell (`index.tsx:155`, repeated in the State cell at `index.tsx:180-181`), about 75 px. The prototype has no such row. With it, names over about 11 characters scroll.
  - Costs of 10.0 or more, a 12-character holder name and a pending release each take 15 to 40 px.
  - The script ran a "worst" row (`measure.cjs:31,43`) and the result is not in DESIGN.md.
- **Change.**
  1. Correct the figure to "6 characters and an ellipsis" in section 3, Q2 and D94(h), and put the screenshot in front of the owner with Q2 before the table is built.
  2. Offer the owner a third option that costs no row height where it matters: label cells may wrap to two lines (clipped after the second). Every row that carries a source tag is already two lines tall, which is every row for a customer whose connector reports weights. That shows roughly twice the characters. Not measured.
  3. Drop the duplicate "Disabled" badge from the Setup cell; the State column already says it, and the existing test only needs one (`ui/src/pages/setups/setups.test.tsx:320`).
  4. State in D94 the case UI-20 is claimed for: name length, holder name length, cost size, viewer role.

## Should fix

### S1. `now` can be higher than `at_peak`, and the cell then reads "4.0 now · up to 3.0"

- **Evidence.** `l.mult` is the multiplier stored with the newest pressure sample; `l.peakMult` is computed from the settings as they are now (`internal/api/user_views.go:88,101-105`). The sample's multiplier was computed with the bands at poll time (`internal/collector/collector.go:509,542-543`). Settings apply "from the next poll"; the default poll interval is 5 minutes (`docs/deploying.md:190`).
- **Failure.** The lab is at 2×. An admin lowers the top band to 1.5×. Until the next successful poll every row says `4.0 now · up to 3.0` and the panel says "Multiplier now 2×" beside "At the highest band 3.0 / h (1.5×)". The request's constraint is that the explanation must not contradict itself. The design's own screenshot step S1 edits the bands and so passes through this state.
- **Change.** Show one value (`now`) when `now >= at_peak`; it is a comparison of two API fields, as "one value when equal" is. Add the case to the UI-16 test and to the Go test (a band edit between two polls), and one sentence to D94.

### S2. Removing the dropdowns removes the only filter for some label values

- **Evidence.** A label value may hold any characters (`internal/rules/labels.go:45-47`). A value with whitespace or one of `, ( ) = !` has no selector term (`internal/rules/selector.go:103-105,266-281`), so its chip is struck through and does nothing (`ui/src/pages/setups/LabelChips.tsx:47-61,92`) and the selector box cannot express it. The dropdown filters on the exact value in the browser (`index.tsx:72`) and handles all of them.
- **Failure.** A display key such as a team or model name has values like `Team A (infra)` or `2U rack`. Today the dropdown filters by them. After the change nothing does. The request's reason for the decision, "the chips and the selector box already filter by every label", is not true for these values. DESIGN.md section 1 lists the removal under "decisions that are easy".
- **Change.** Do not reopen the decision. Record the loss in D94 and put it to the owner as a question (see the end). The design should not claim that nothing is lost.

### S3. Focus is not returned when the row that opened the panel is gone

- **Evidence.** `Modal` focuses the opener on close with no check that it is still in the page (`ui/src/components/Modal.tsx:33,69`); the side-panel hook does check (`ui/src/hooks/usePanelFocus.ts:53`). The design keeps the panel open when its setup leaves the list, and looks the setup up before the State filter (section 4, "State").
- **Failure.** State filter "Free". The user opens the price of a free setup. On the next poll someone has acquired it, the row is filtered out and its button is unmounted. Close: focus falls to `<body>`, and a keyboard user starts again from the top bar. The same happens when the setup is deleted. The request requires that focus returns on close.
- **Change.** When the opener is not connected, focus a stated fallback in the page (the "All setups" table or the h1, with `tabIndex={-1}`). Add it to the test `UI-18 an open panel follows a refresh…`.

### S4. The "no longer listed" copy can mix two responses

- **Evidence.** Section 4 keeps "the last copy" of the setup in a ref. It does not say that `pricing` and `reported_weights_enabled` are kept with it.
- **Failure.** The setup is deleted while the panel is open and the lab changes band. The panel shows the old weight and costs beside the new multiplier: the contradiction that `pricing` was added to prevent.
- **Change.** Keep the setup, `pricing` and `reported_weights_enabled` of the last response that contained the setup, together.

### S5. The real UI-20 test can pass on a table that does not meet UI-20

- **Evidence.** Step U20 asserts `scrollWidth <= clientWidth` on the integration stack, whose setups are named `lab-01` to `lab-06` and are viewed by a developer (`e2e/browser/tests/integration.spec.ts:597`; `deploy/smoke.sh:99-100`). The Vitest test named UI-20 checks class names only and cannot fail on layout (the design says so). The step is Chromium only (`e2e/browser/playwright.config.ts:30`) and is not part of `./ci.sh` (`ci.sh:14`).
- **Failure.** The assertion is true before the five columns render, true when label cells have collapsed to an ellipsis (the design measured that variant at 29 px), and true for six-character names while a customer with 20-character names scrolls.
- **Change.** In U20, first assert ten column headers; assert each label cell is at least a stated width; view the table as the admin as well, with one disabled setup; include a held row with an overdrawn holder and a not-charging reason. Say in D94 that UI-20 is checked only by the integration run.

### S6. Files and documents the change list misses

| Missing | Why |
| --- | --- |
| `ui/src/pages/admin-shared/fixtures.ts:160-169` | `setupList()` is typed `SetupList`; a required `pricing` fails `tsc` until it is added (`ui/tsconfig.json` includes `src`) |
| `docs/integrating.md:471` | "shows them next to the weight in the Setups list": attributes are in the panel now |
| `README.md:9` | "shows the weight and its source wherever a price is shown": the Setups row shows the price without the weight |
| `docs/ui-notes.md:210,220,222,232` | "Differences" items 4 ("N / h if acquired"), 6 and 14 (the Weight cell on Setups) and the overdrawn line become false; a new item 15 alone leaves them standing |
| `docs/progress.md` | Earlier jobs recorded their state there (`docs/progress.md:75,98-113`) |

Also for the end-to-end steps: Playwright matches a role name as a case-insensitive substring. `getByRole("button", { name: "Details for lab-03" })` matches the Details link and the new cost button, whose name ends "Price details for lab-03", and fails in strict mode. The new steps must pass `exact: true`. The Vitest string form is exact already (`setups.test.tsx:198`).

### S7. A 20-character clip on the setup name makes some rows indistinguishable

- **Evidence.** Section 3 reduces `Clip` from 32 to 20 ch (`index.tsx:154`). The name is the row's identifier.
- **Failure.** Names that share a long prefix and differ in a trailing number (`team-infra-cluster-lab-01`, `…-02`) all read `team-infra-cluster-l…`. The full name is on hover and in the panel title only.
- **Change.** Keep 32 ch unless the owner accepts this with Q2; UI-20 already fails for names over 17 characters, so the shorter clip buys nothing below that length and hides the difference above it.

### S8. "Details" opens on a section whose position on screen is left to the browser

- **Evidence.** Section 4 focuses the "Labels and details" heading and says "focusing a heading scrolls it into view". The dialog is the scroller (`ui/src/components/Modal.module.css:14-15`). Where a focused element lands is implementation-defined; Chromium centres it when it was out of view (inference).
- **Failure.** The user clicks Details to read the JSON, as today. The heading lands mid-dialog or at its lower edge with the JSON still below. No test can see this in jsdom.
- **Change.** Either open both at the top with focus on the Price heading (simplest; the request only says Details "opens the same panel"), or call `scrollIntoView({ block: "start" })` on the heading and say so. See Q6.

### S9. The Go test as worded fails before the first poll

- **Evidence.** The design's test requires `pricing.pressure` to equal `GET /api/v1/pressure`. Before the first poll that route sends `smoothed: 0.0` with `updated_at: null` (`internal/api/handlers_user.go:657-662`) while `pricing.pressure` is null by design.
- **Change.** Word the test: equal after a poll; before one, `pricing.pressure` is null and `/pressure.updated_at` is null. Note in D94 that the top bar reads "Lab 0%" in that state while the panel reads "not measured yet".

## Notes

1. **`pricing` carries no sample time.** When the pressure source fails, no sample is written (`collector.go:482-485`) and the newest one is used however old. The panel will show its pressure and multiplier as current. The top bar does the same today. Adding a time later is another additive field; not needed now.
2. **Policy-priced holds (Q4).** The existing Cost column has the same property, so this is no regression. A separate existing limit: `metering.active` is true once metering has started (`user_views.go:226`), so the panel's "Charging since" also shows for a custom policy that later pauses.
3. **Screen reader.** The badge's `title` and the sibling `sr-only` sentence are the same text and may be read twice; keep the `sr-only` sentence and drop the `title`, or keep both and test with a reader. The middle dot in "8.0 now · up to 12.0" is read differently by different readers; the `sr-only` part could carry the whole phrase in words. Each row has two buttons whose names end "details for <name>".
4. **Panel content.** An empty label value should read "(empty)" as the chips do (`LabelChips.tsx:46`). A 63-character unbroken value or key needs `overflow-wrap: anywhere` in a 520 px dialog. The JSON `pre` scrolls inside a scrolling dialog and cannot be reached by keyboard (`setups.module.css:17`); inside the panel it can lose its own `max-height` so the dialog is the one scroller. A setup may have 32 labels (`internal/rules/labels.go:15`).
5. **Wording.** The row tag says `override`; the panel says "pinned by an admin" and never the word. Put the tag word in the panel line ("override: pinned by an admin") so the two connect. The admin page calls it "Weight override".
6. **Generic.** The prototype's keys include `form-factor`, `group` and `subnet`, which are the customer's. They are not product code, but the U20 step and unit fixtures should use neutral keys.
7. **One display key.** All spare width goes to the single label column; the table will look different from today's even spread. Acceptable; the screenshots for the owner show five keys only.
8. **The v1.1.0 comparer** reports an added object as one path and does not descend into it (`internal/api/upgrade_v110_compat_test.go:219-221`). `addedSetupsList` gains the single entry `"pricing"`. This answers the design's "not verified" item.
9. **Scope.** Showing `(3×)` beside the peak cost is not asked for and is the only reader of `peak_multiplier`. It is cheap and I would keep it. The `Modal` Tab fix and `initialFocus` are needed only because a heading takes focus; if S8 is resolved by focusing the dialog itself, neither is needed (`Modal.tsx:58` already handles the dialog node).

## The design's open questions

| # | Design default | My view |
| --- | --- | --- |
| Q1 | Add `pricing` | Agree. Same `lab` value as the costs, no extra read, three numbers per response. Add M1. |
| Q2 | Accept 8 to 9 characters | Disagree as put. The figure is 6 (M3). The owner should decide on the screenshot, with the two-line option beside it. |
| Q3 | No "Charging starts" line for a free setup | Agree. A describe method is a new extension point and out of scope. The panel still shows the text if a server ever sends it. |
| Q4 | Do not mark policy-priced holds | Agree. No regression, and the bundled policy never sets a weight. Keep D94(d). |
| Q5 | Acquire dialog keeps `/pressure` | Agree; the dialog is unchanged by owner decision. D94(c) should say that the panel and the dialog can show different multipliers for up to one poll. |
| Q6 | Details opens on Labels and details | Acceptable only with a stated scroll position (S8). Opening both at the top is simpler and removes two `Modal` changes. |

## Claims checked and found correct

- Both costs of every row are the resolver's weight times `l.mult` and `l.peakMult`, whatever the state; the hold is not consulted (`internal/api/handlers_user.go:850-858`, `internal/api/user_views.go:124-129`).
- `HourlyCostMT` rounds half away from zero to a milli-token and gives 0 when either factor is 0 or less (`internal/metering/metering.go:116-131,148-150`).
- `metering_description` is always null (`handlers_user.go:887-888`, `api/openapi.yaml:2080-2083`, pinned by `internal/api/user_test.go:387`). `expected_start_at` is always null too (`user_views.go:26`).
- `pricing` would be written from the same `lab` value the costs use: `l.mult`, `l.peakMult`, `l.sample` are loaded once per request (`user_views.go:78-109`). No second read; no added query.
- A hold priced by a policy weight is charged at the policy's weight while the Setups list shows the resolver's (`internal/collector/collector.go:616-623`, `user_views.go:208-215`).
- An equals sign can be false from display rounding. The arithmetic of the three examples holds; I did not re-run the formatters.
- `useApi` keeps old data on a failed request; the page and the top bar fetch pressure separately (`ui/src/hooks/useApi.ts:44-48`, `index.tsx:62,64`, `ui/src/shell/TopBar.tsx:20`).
- The source tag has only a `title` today (`ui/src/pages/usage/weight.tsx:16`).
- `Modal` focus rules, Tab trap and focus return are as cited (`Modal.tsx:37-41,58,69`).
- No URL parameter or stored state holds a filter (`index.tsx:50-53`). No end-to-end step uses a label dropdown or reads the Weight or Holder's tokens column, except W2 (`integration.spec.ts:597-602`).
- The acceptance steps on Setups read the label value, state, "You" and the Acquire button from text content and are not affected by clipping (`e2e/browser/tests/acceptance.spec.ts:398-434,478-497,636-637`).
- The contract test closes schemas, so the handler and `api/openapi.yaml` must change together (`internal/api/contract_test.go:28-34`); the key lists at `user_test.go:380` and `upgrade_compat_test.go:195` are as cited; the schema drift check exists (`ui/BUILD.bazel:115`).
- Setup inventory keeps its own Weight column and `WeightTag` (`ui/src/pages/admin-setups/index.tsx:102-111`); a separate `PriceSourceTag` leaves it and My usage alone.
- Earlier releases added response fields the same way, with a line in the upgrade notes (`docs/deploying.md:615,624`).

## What I could not check

- **Any layout number.** The brief allows focused unit tests and read-only git, so I did not run the prototype. M2, the widths in M3 and S8 rest on reading the prototype, its screenshot and the product's CSS.
- Firefox and Safari (the design did not measure them either).
- Whether steps U20, S1 and S2 run as described. I read that the scenario's pressure stays at 0.02 and its bands are base 1, 2× from 5% (`integration.spec.ts:246-251,334`), which is consistent with the plan.
- The fonts of the machine that runs the end-to-end browser; a wide default font leaves less spare width.
- How a screen reader reads the cost button and the tag.
- The customer's real setup names and label values. The request says the installation was not seen.
- I ran no test. Nothing in the design can be tested before it is built.

## Question only the owner can answer

1. **Label legibility (Q2, corrected).** At 1280 px with five keys a label column shows 6 characters. Accept that, allow label cells two lines, or something else? The prototype screenshot is `tasks/01-design/proto/layout-1280-five-keys.png`.
2. **Filter loss (S2).** With the dropdowns gone, a label value that holds a space or one of `, ( ) = !` cannot be filtered at all. Accept and record it, or should such a chip filter in the browser?
