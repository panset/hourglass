# Change request: a Setups page that leads with cost

Status: designed and built on the branch `setups-declutter` (2026-11-24, from `main` at `f1a05c2`), for release `v1.3.0`; the decisions taken are D93 to D95 in `docs/decisions.md`. **Amended the same day by the owner after the first screenshots: no label columns and no label chips; see [Amendment, 2026-11-24](#amendment-2026-11-24) (D96 to D99), which wins where it conflicts with the text above it.** Written 2026-11-24 against `main` at `4c40a4c`, from a reading of `ui/src/pages/setups/index.tsx` and the documents named below. The page was not run and the customer's installation was not seen.

Corrections found while designing (the owner decisions below are unchanged; D94 has the detail):

- "Clipped as the setup name already is (`Clip`)" cannot meet UI-20: a fixed-width clip per column is too wide for five columns. Label columns instead share the width the others leave and cut with an ellipsis. Measured in Chromium at 1280 px with five keys, a label shows about 6 characters, and UI-20 holds for setup names up to 21 characters (D94(h), (i)). The owner chose one line, clipped.
- "The chips and the selector box already filter by every label" is not true for a value that holds whitespace or one of `, ( ) = !`: it has no selector term, and only the removed dropdown filtered it. The owner chose that such a chip filters by exact match in the browser; UI-9 is amended (D94(j)).
- `metering_description` is always null today, so a free setup has no "when charging starts" line (D94(e)).
- `cost_per_hour_tokens.now` can be above `.at_peak` between a band edit and the next poll; the row then shows one value (D94(f)).
- The equation is not shown with an equals sign, and `GET /api/v1/setups` gains the additive `pricing` object with the multipliers and pressure its costs were computed with (D94(a), (b)).

This document is the brief for the agent that designs and implements the change. It says what must be true afterwards and why, and names the decisions the owner already made. Layout, wording and visual treatment are yours to design within the requirements in [Requirements](#requirements). Where a detail is open, choose the simplest option that satisfies them and record it in `docs/decisions.md` from D93 on.

## Why

A customer told us the Setups page (`/setups`) is too cluttered. They see a weight and a cost on every row and cannot tell which one matters. What they care about is what a setup costs. They suggested that how the cost follows from the weight and lab pressure be explained on click.

They are right about the problem. The page today has these faults:

| | Fault today | Where |
| --- | --- | --- |
| A | Three token numbers per row with no unit: Weight (`2`), Holder's tokens (`212`), Cost (`2.0 / h`). At a 1× multiplier the weight and the cost are the same number. | `columns` in `index.tsx` |
| B | The weight cell is two lines tall when a setup has attributes (`memory-gb 512 · storage-gb 4000`), so every such row is. | `weight` column |
| C | Nothing on the page says that cost is weight × the pressure multiplier. Only the acquire dialog hints at it. | `costCell`, `ConfirmDialog` |
| D | "Details" opens raw JSON. There is no place where a price is explained. | `Modal` |
| E | Three controls filter by label: one dropdown per display label key, the selector box, and the label chips. | `filters`, `LabelChips` |
| F | Held by, Holder's tokens and Held for are three columns about one fact: who has it. | `columns` |

This customer shows five label columns (`form-factor`, `fdqn-source`, `group`, `subnet`, `vcenter`). Their table is 13 columns wide and their filter row holds a state dropdown, five label dropdowns, the selector box and chips for every key. After this change the same installation has 10 columns and no label dropdowns.

Hourglass is a generic product. Nothing here may be specific to this customer: no label key, attribute name or pressure source is named in code, and there is no per-customer switch.

## Owner decisions

Made on 2026-11-24. Do not reopen them.

| Decision | Detail |
| --- | --- |
| Weight is not a column on the Setups page. | It is an input to the price, not something a developer acts on. It moves into the details panel. |
| The row shows a cost range, not the current cost alone. | In a quiet lab the multiplier is 0× and every setup reads `0.0 / h`, so a large setup looks like a small one. The row shows the cost now and the cost at the highest band. Both are already in the API (`cost_per_hour_tokens.now`, `.at_peak`). This is where we did not follow the customer's wording ("only the cost"). |
| The source of the weight stays visible on the row. | A price that comes from a `reported` weight can change during a hold, and an `override` is an admin's pin. Customers whose connectors report weights need to see this without a click. The tag moves from the weight cell to the cost cell. |
| The calculation is explained on click, in the details panel. | Not on hover: it must work with a keyboard and on touch. |
| No setting to bring the weight column back. | One default for every customer. Revisit only if another customer asks for it. |
| Holder's balance is not a column. | An overdrawn holder is marked beside their name. The figure moves into the details panel. |
| Held by and Held for are one column. | For example `You · 3 h`. |
| The per-key label dropdowns are removed. | The chips and the selector box already filter by every label. The State dropdown stays. |

## What to build

### The table

Columns, in order: Setup, one per display label key, State, Held by, Cost, actions.

- **Cost.** Shows the cost per hour now and at the highest band, for example `2.0 now · up to 6.0`. When the two are equal, one value. The unit (tokens per hour) is stated in the header or the cell; the designer chooses. "if acquired" is not repeated on free rows. A held setup that is not metering keeps the policy's reason text as it is today. When `weight_source` is `reported` or `override` the cell carries the tag that the weight cell carries today, with an accessible name that says the price comes from a reported or pinned weight.
- **Held by.** The holder's name or "You", then how long. An overdrawn holder is marked in words, not by colour alone.
- **Label columns.** Still chosen by the admin (SET-6). A long value is clipped with the full value available, as the setup name already is (`Clip`), so that five label columns do not force the table to scroll sideways at 1280 px.
- Activating the cost cell opens the details panel on its price section. "Details" in the actions column opens the same panel.

### The details panel

Replaces the JSON-only modal. Three sections:

1. **Price.** The weight; where it comes from, in words (`rule`, `default`, `reported`, `override`); the attributes line when there is one; the current lab pressure and multiplier; the cost now and at the highest band; when charging starts (`metering_description`, or the hold's `metering.reason`); and, when `reported_weights_enabled` is true, the sentence the acquire dialog already shows. One plain sentence says that cost is weight × the multiplier and that the multiplier follows lab pressure.
2. **Holder**, when held: name, held since, balance, overdrawn or not, release requested or not.
3. **Labels and details.** Every label of the setup, not only the display keys, then the details JSON as today.

Because every label is now one click away, an admin can show fewer label columns without hiding anything. Say so in the Settings help text for display label keys if there is one; do not add a limit.

### Filters

State dropdown, selector box, chips. The `labelFilters` state and the per-key `Select`s go.

### Unchanged

The acquire dialog (it already shows weight, source, attributes, both costs and the multiplier). My usage. Every admin page: Setup inventory keeps its weight column, because admins set weights. The API's existing fields.

## Constraints

- **UI-1 holds.** The page computes no number. Weight, multiplier, both costs and the pressure are API fields and are only formatted.
- **The explanation must not contradict itself.** Setups and pressure are two requests on the same 60-second poll, so the multiplier on screen can be one poll behind the cost. Before showing `weight × multiplier = cost` as an equation, check that it holds for every case the API can return: a held setup whose hold is priced by a `policy` weight, a setup that is not metering, a weight of 0. If it does not, either list the values without an equals sign, or add the multiplier that the cost was computed with to the `GET /api/v1/setups` response. An API change must be additive, with `api/openapi.yaml`, the generated types and the contract test updated; nothing is removed or renamed.
- **Generic.** No label key, attribute name or extension name in code or in tests' assertions beyond fixtures.
- **Accessible.** The panel opens from the keyboard, returns focus on close, and the cost cell's control has a name such as "Price details for setup-01". No information is carried by colour or by hover alone.
- **Nothing a user could see is lost.** Weight, source, attributes and the holder's balance are all still on the page, one click away.

## Requirements

New ids. Each needs a test named after it.

- **UI-15** The Setups table has no Weight column and no Holder's tokens column. Its columns are Setup, the display label keys, State, Held by, Cost and actions.
- **UI-16** The cost cell shows `cost_per_hour_tokens.now` and `.at_peak` with their unit, one value when they are equal, and the source tag when `weight_source` is `reported` or `override`, and none for `rule` or `default`.
- **UI-17** The Held by cell shows the holder, how long, and an overdrawn marker in words when `holder.overdrawn` is true.
- **UI-18** The details panel opens from the cost cell and from Details, and shows the weight, its source, its attributes, the current pressure and multiplier, both costs, when charging starts, the holder's balance when held, every label, and the details JSON. It shows the price-change sentence whenever `reported_weights_enabled` is true.
- **UI-19** The page has no per-label dropdown. The State filter, the selector box and the chips (UI-9) work as before.
- **UI-20** With five display label keys and values of up to 24 characters, the table does not scroll horizontally at 1280 px.

Amended:

- **UI-12** (in `docs/design/reported-weights.md`) becomes: the Setups page shows the source tag on the cost cell and the weight, source and attributes in the details panel; the acquire dialog clause is unchanged.
- **SET-6** is still met: the display label keys are columns, and the chips filter by them.
- **WGT-14** is about values, not placement. The Setups list's weight and source must still equal what a poll records; a test that reads them from the table reads them from the panel instead.

## What else changes

- `docs/ui-notes.md`, section "Setups": columns, the cost cell, the panel, the filter row.
- `docs/wireframes/project/Setups.dc.html`: update the artboard, or note in `ui-notes.md` that it is behind.
- `docs/SPEC.md` is a copy and is not edited. Record in `docs/decisions.md` the edit its source needs: the Setups row of the "Dashboard pages" table lists weight and the holder's balance as shown; they are now in the details panel.
- `ui/src/pages/setups/setups.test.tsx`: the SET-6 header assertion and the UI-12 block assert the old columns.
- `e2e/browser`: check for steps that read the Weight or Holder's tokens column or use a label dropdown.
- A `docs/decisions.md` entry, owner-directed, for this change request.

## Out of scope

- A column chooser, saved views, or any per-user or per-customer display setting.
- Sorting, paging or grouping the table.
- Changes to how a weight, a multiplier or a cost is computed.
- A limit on the number of display label keys.

## Done when

1. UI-15 to UI-20 pass, and the amended UI-12 tests pass.
2. `./ci.sh` passes, and the end-to-end modes that touch the Setups page pass.
3. Screenshots of the page and the open panel, at 1280 px with five label columns and with reported weights on, are attached for the owner: one with the lab at 0× and one at a higher band.
4. The documents in [What else changes](#what-else-changes) are updated.

## Amendment, 2026-11-24

Made by the owner on 2026-11-24, after seeing the first build's screenshots. It overrides everything above where the two conflict; the sections above, the owner-decision table included, are left as written and are history where this section says so. The decisions are D96 (owner), D97 (details), D98 (edits for the source of `docs/SPEC.md`, replacing D95) and D99 (the UX designer's specification and the owner questions taken on its defaults) in `docs/decisions.md`.

### The owner's words

> 1) In the 'Setups' page, lets remove the label columns. Its looks very much cluttered
> 2) Lets get rid of the Available labels as well. We should instead just let users start typing in the 'Label Selector' checkbox and we could autofill values.

Follow-up answers from the owner:

- **R3-1.** The admin "display label keys" setting is removed from the admin Settings page. The stored value and the API field stay (the API changes only additively), but nothing reads them.
- **R3-2.** A value the selector cannot express is still suggested by the autocomplete and matched exactly in the browser, shown as a removable filter token.
- **R3-3.** Labels are shown in the details panel only. The table is Setup, State, Held by, Cost and actions.

The owner later asked for a senior UX designer to drive the revision. Its specification decides layout, wording and interaction (D99).

### What it overrides above

- **"The table", columns and the "Label columns" bullet.** There are no label columns. The columns are Setup, State, Held by, Cost (tokens / h) and actions. The "Label columns" bullet and the one-line clipped label cells (the owner's earlier answer Q-A, D93(i)) are retired.
- **"Filters".** It now says: State dropdown and the Label selector box, which suggests label keys and values as the user types. The "Available labels" chips are gone. A value with no selector term is suggested after `key=` and, when picked, becomes an exact-match token; this replaces the chip form of the owner's earlier answer Q-B (D93(j)).
- **The Settings help-text sentence** ("Because every label is now one click away, an admin can show fewer label columns …"). It no longer applies: the Settings page has no display label keys field, so there is no help text.
- **"The details panel", section 3, "Labels and details".** It is still the place where every label is shown, and now the only one. The designer made it two sections, **Labels** and **Raw details** (D99).
- **"Why".** After this change the installation described there has 5 columns, not 10, and no label dropdowns or chips.
- **UI-20's wording.** "With five display label keys …" no longer applies: there are no label columns. The amended text is below.
- **"SET-6 is still met".** It is not. SET-6 is retired (below).
- **The owner decision "The per-key label dropdowns are removed."** It stands. Its reason ("The chips and the selector box already filter by every label") now reads: the selector box and its suggestions, with exact-match tokens, filter by every label.
- **The corrections at the top** about label columns that give way and chips that filter by exact match are history.
- **"Done when", item 3.** The screenshots are taken without label columns (there are none). Two more show the change itself: the suggestion list open in an empty box, and an exact-match token in force.

### Requirements, amended

- **UI-15** The Setups table's columns are Setup, State, Held by, Cost and actions. It has no Weight column, no Holder's tokens column and no label column.
- **UI-19** The page has no per-label dropdown and no label chips. Labels are filtered with the selector box (UI-21) and with exact-match tokens (UI-22). The State filter works as before.
- **UI-20** At 1280 px the Setups table does not scroll horizontally.
- **UI-9** (in `docs/design/ledger-labels-onboarding.md`) is replaced by UI-21 and UI-22: the Setups page offers every key and value of `label_index` as suggestions in the Label selector box, and the placeholder is taken from the index. SET-8 (`label_index`) is unchanged.
- **SET-6** is retired: `display_label_keys` is stored, validated and returned by the API for compatibility. No page reads it, and the Settings page does not edit it.
- **UI-18** is unchanged and now carries R3-3 alone: every label of a setup is in the panel.
- **UI-12**, **UI-16**, **UI-17** and **WGT-14** are unchanged.

### Requirements, new

- **UI-21** The Label selector box is a combobox that suggests, for the caret's position, label keys, operators, or the values of the key being written, all taken from `label_index`. Picking a suggestion replaces the token under the caret; picking a value after `=`, `==` or `!=` applies the filter. It is operable with the keyboard and by touch, and names itself to assistive technology. A selector typed by hand is sent as typed, and an invalid one shows the API's message, as before.
- **UI-22** A label value that cannot be written in a selector is suggested after `key=` and, when picked, becomes an exact-match token. The token filters the fetched list in the browser, combines with the selector and the State filter, holds one value per key, never reaches the API, and is removed with its own button.
- **UI-23** The Settings page has no display label keys field. Saving settings sends the stored `display_label_keys` back unchanged.

### Unchanged by the amendment

The requirements on the cost cell, the Held by cell and the details panel (UI-16, UI-17, UI-18; the designer changed their look and some of their wording, D99), the `pricing` object on `GET /api/v1/setups`, the acquire dialog, My usage, every admin page other than Settings, and the API: nothing is removed, renamed or added in this revision. The out-of-scope list above still holds.
