# UX specification: the Setups page after the owner's change

Date 2026-11-24. Task 12. For the builders of DESIGN-R3 (tasks C1 to C4) and the UX acceptance pass.

**Authority.** On layout, wording, visual treatment and interaction this document wins over `tasks/01-design/DESIGN-R3.md`. On data sources, files, the grammar being completed, requirement ids and tests, DESIGN-R3 stays the authority. Owner decisions (`DECISIONS.md`, the change request, the Amendment) are not reopened here.

**What each claim rests on.** "Built" means I saw it in the first build's screenshots (`tasks/09-b5-e2e/screenshots/`) or read it in the uncommitted source. "Prototype" means I saw it in a static page I made from the product's real CSS files plus the CSS proposed here (`proto/proto.cjs`, shots in `proto/shots/`, Chromium, 1280 px and 390 px). The prototype has no behaviour: every statement about keys, focus, timing or a screen reader is a specification, not an observation. "Inference" is marked as such.

The reader this page is for: a developer on their first visit whose question is "what will this setup cost me, and can I have it".

---

## 1. Critique

### 1.1 The first build (screenshots), ranked

The two worst faults, the label columns and the "Available labels" block, are already removed by the owner. What follows is what is left once they are gone. `proto/shots/00-as-built-without-labels-1280.png` shows the built cells with those two parts taken away.

| # | Fault | Where seen | Why it matters |
| --- | --- | --- | --- |
| 1 | **The cost, the one number the page is about, is drawn as a blue underlined link.** Every row then has two blue links that open the same panel (the cost and "Details") and a blue button. | All four page shots | A link says "go somewhere"; the number says "read me". Underlined blue digits are harder to read than plain ones, and six of them down a column is the clutter the customer complained of, moved to a new column. |
| 2 | **The source tag makes most rows two lines.** With reported weights on, four of six rows carry a `reported` pill on a second line, right under the blue link and in the same blue. | S1, S2 page shots | The tag is on nearly every row of such an installation, so it carries the least information and takes the most room. On its own line it also reads as part of the link. |
| 3 | **The panel answers the question last.** Price section order is: a sentence about weight and multiplier, Weight, Attributes, Lab pressure, Multiplier now, and only then Cost now. | Both panel shots | The reader clicked a price. The panel should repeat that price first and then explain it. |
| 4 | **The panel changes vocabulary.** The row says `up to 2.4`; the panel says "At the highest band". The row's header says "tokens / h"; the panel says `1.6 / h`. A developer never sees the word "band" anywhere else (it is an admin Settings term). | Panel shots; `SetupPanel.tsx:80-87` | Two names for one thing make the reader check whether they are one thing. |
| 5 | **"Charging: Charging since Nov 24, 14:30".** The line name and its value start with the same word. | Panel shots | Small, but it is the kind of repetition that makes a panel feel machine-made. |
| 6 | **"Labels and details" inside a dialog titled "Details: lab-02".** The last section is two unrelated things (labels, a JSON block) under a heading that repeats the dialog's title. | Panel shots | Labels are now shown nowhere else. They need their own heading so they can be found. |
| 7 | **Three lists, three alignments.** Each list sizes its first column to its own longest name, so the values start at three different x positions (about 143, 85 and 210 px in). | Panel shots; `.panelList` uses `max-content` | Ragged; costs nothing to fix for the first two lists. |
| 8 | **The placeholder is cut mid-word**: `e.g. a-rather-long-label-key-name='`. It is the index's first term, whatever its length, in a 260 px box. | All page shots | The one hint a newcomer gets about what to type is unreadable. |
| 9 | **An error under the selector box lifts the box.** The filter row is bottom-aligned (`.filters { align-items: flex-end }`), and the error text is part of the field, so when it appears the box rises above the State select and the Filter button. | Read in `setups.module.css:6-7`, `Field.tsx`; not in a screenshot (inference from the CSS, confirmed for the proposed fix in the prototype) | Controls that jump when a message appears look broken. |
| 10 | **Holder names are cut at 12 ch** although the table now has room. | `index.tsx:189` | A leftover of the ten-column table. |
| 11 | **The panel shots are full-page shots**: the backdrop stops at 900 px and the dialog is cut in the JSON block. | S1, S2 panel shots; task 10 minor 3 | Not a page fault, but the owner sees it as one. Already in DESIGN-R3; kept in the acceptance checks. |

Confirmed as good and kept: the title row and its summary; the State badges; `2.0 now · up to 6.0` as the form of the cost; the unit in the column header; `You · 3 h`; "overdrawn" as a word on its own line; the dialog's focus and close behaviour; the weight line `0.8 · reported (sent by automation)`.

### 1.2 DESIGN-R3, ranked

DESIGN-R3 is sound on mechanics (caret positions, whole-token replacement, no active option until the user moves into the list, composition, old API). Its gaps are about the newcomer.

| # | Gap | Why it matters |
| --- | --- | --- |
| 1 | **A newcomer who types a value gets nothing.** At the start of a requirement only keys are suggested. Typing `large` suggests nothing, because `large` is a value of `size`. | The owner's sentence is "let users start typing … and we could autofill **values**". A person knows what they want ("a large one") before they know the key it is filed under. |
| 2 | **The key list is a list of bare words.** `owner`, `site`, `size`, `tier` with nothing to say what each holds. | The chips showed keys *with* their values. Without that, picking a key is a guess, and discovering the labels takes one pick per key. |
| 3 | **No way to clear everything, and no sign of what is in force.** State, the selector and the tokens are cleared one by one; nothing says how many setups are left or that a filter is on. | With chips, the pressed chips showed the filter. Now an applied selector and an idle box look the same. |
| 4 | **Tokens "beside the Filter button".** At 1280 px two tokens fit; a third wraps under the State select. Each token repeats the words "exact match". | Tokens are results, not controls of the same rank as the box. They belong on their own line with one lead-in. |
| 5 | **Value options carry the resulting term in muted text** (`sjc` and `site=sjc`). | The box already shows `site=`. Saying it again on every row is noise. |
| 6 | **When nothing matches, the list just does not open.** | Silence is ambiguous: no labels at all, or a typo? One line says which. |
| 7 | **Discovery depends on knowing to click an empty box or press Arrow Down.** | A text box does not look like something that lists things. The State select beside it has an arrow; this box should too. |
| 8 | **The operators are symbols only**: `=`, `!=`, `in (`, `notin (`. | Fine for someone who knows selectors. Two words beside each cost nothing. |
| 9 | **The "N more" line is the last line of a scrolling list**, so it is out of sight until the user scrolls to the end. | It is the line that tells the user to keep typing. |

DESIGN-R3's three open questions are resolved in section 3.9.

---

## 2. The page, top to bottom

Order of regions inside `.page`: title row; filter row; filters-in-force line (only while a filter is in force); alerts; loading line or table. Gaps between regions are `--gap-region` as built.

Hierarchy in one sentence: the setup name and the cost now are the two strong things on a row (weight 600, text colour); the Acquire button is the one filled control; everything else is regular weight or muted.

### 2.1 Title row (as built)

- `h1`: **Setups**
- Right, 14 px muted: `{deployed} of {enabled} setups deployed · {free} free`. Shown once the first response is in.

### 2.2 Filter row

Left to right: State, Label selector box, Filter button. Prototype: `proto/shots/01-page-1280.png`.

| Control | Visible text | Accessible name | Notes |
| --- | --- | --- | --- |
| State select, 180 px | Label **State**. Options: **All states**, **Free**, **Deployed**, **Acquired**, and for an admin **Disabled** | "State" | As built |
| Selector box, 360 px, shrinks on narrow windows | Label **Label selector**. Placeholder: see 3.1 | "Label selector" (role combobox) | Section 3 |
| Button | **Filter** | "Filter" | Submits the box, as built |

Layout rule (changed): the row is **top-aligned**, not bottom-aligned. The Filter button is pushed down by the height of a label line (`calc(var(--fs-13) * 1.45 + var(--gap-label))`) so the three controls share one top and one bottom edge. A message under the box then moves nothing. The form (box and button) does not wrap: at 390 px the box narrows and the button stays beside it (prototype `13-token-390.png`).

Text typed in the box is monospace 13 px (`--font-mono`), as label terms are on My usage (`Chip`) and in the suggestion list. The placeholder is the sans face at 14 px, muted.

### 2.3 Filters-in-force line

A single line under the filter row. **Rendered only when** the State filter is not "All states", or the applied selector is not empty, or there is at least one exact-match token. A typed but not applied selector does not show it. Prototype: `06-token-and-count.png`, `13-token-390.png`.

Left to right, wrapping, 14 px:

1. When there are tokens: the lead-in **Exact match:** (muted), then the tokens (3.6).
2. The count, muted: **1 setup shown** / **{n} setups shown**, where n is the number of table rows. Left out when n is 0 (the table says it).
3. A link-style button **Clear filters** (the existing `.linkButton`).

The count is the number of rows the page is drawing. It is not an API number and nothing is computed from one (UI-1; the same kind of count DESIGN-R3 already makes for options). `summary` cannot be used: it ignores the selector (`handlers_user.go`, comment above `listSetups`).

**Clear filters** sets State to "All states", empties the box, applies the empty selector, removes every token, closes the list, and moves focus to the selector box (the button it was pressed on is gone). Announced as "Filters cleared."

### 2.4 Alerts (as built)

In this order, each only when it applies: the warning that the user cannot acquire (texts as built in `acquireBlock`), the error of a failed acquire or release, the error of a failed list request other than an invalid selector.

### 2.5 Table "All setups"

Columns, fixed by the owner: **Setup**, **State**, **Held by**, **Cost (tokens / h)**, and a blank header announced as "Actions". Default cell padding (the `dense` option goes, as in DESIGN-R3). Every cell is one line except the two second lines named below. Prototype measurement: at 1280 px the table is as wide as its box (1152 px) with a 32-character name, a holder name of 15 characters and a tag on the row; no sideways scroll.

| Column | Content | Treatment |
| --- | --- | --- |
| Setup | The name | Weight 600, cut at 32 ch with the full name in `title` (as built) |
| State | **Free** (green badge), **Deployed** (blue), **Acquired · deploying** (grey), **Disabled** (grey, admin only) | As built |
| Held by | Section 4.2 | Regular; "You" weight 600 |
| Cost (tokens / h) | Section 4.1 | The cost now weight 600; the rest regular or muted |
| Actions | **Details** link on every row (name "Details for {setup}"), then **Release** (your hold), or the amber badge **Releasing** (your hold, release pending), or the primary button **Acquire** (name "Acquire {setup}"; free setup, not the local admin) | As built. Details stays the one blue link on the row |

### 2.6 Empty, loading and error states

| State | What shows |
| --- | --- |
| First load | **Loading…** (`role="status"`), no table. As built |
| No setup exists or is visible | Table with one row: **No setups to show.** As built |
| Filters leave nothing | Table with one row: **No setups match the filters.** The filters-in-force line above it holds the tokens and **Clear filters** (prototype `15-empty-1280.png`) |
| Invalid selector | Section 3.7. The table keeps the rows of the last selector that worked |
| List request failed | Red alert with the API's message; the table keeps the last rows if there are any. As built |
| Refresh in progress | Nothing changes until the response arrives; no spinner. As built |

---

## 3. The selector box with autocomplete

### 3.1 Label, placeholder, discovery

- Label: **Label selector**. The owner's name for it, the name the rate rules table on Settings uses, and the name the tests find.
- Placeholder: **Type or pick a label, e.g. {term}** where {term} is the first term in `label_index` (in the API's order) that has a `selector` and is at most 16 characters long. If no term is that short: **Type or pick a label**. With an empty or missing index: no placeholder, as DESIGN-R3 says. (Prototype: the full placeholder with a 10-character term fits the 360 px box. The built placeholder took the first term at any length and was cut.)
- A **down arrow at the right edge of the box**, the same glyph as the State select's, so the box reads as "this lists things". It is a button outside the Tab order (`tabindex="-1"`), name "Show label suggestions", `aria-expanded` and `aria-controls` as on the input. A click or tap puts focus in the box and opens or closes the list for the caret's position. Not rendered when the index is empty.

How a newcomer finds the labels, in the order they are likely to try: click the box (the list of keys opens, each with a sample of its values); type any part of a key or of a value; click the arrow; press Arrow Down. A screen-reader user hears "Label selector, combo box, collapsed" and opens it with Arrow Down.

### 3.2 The suggestion list: look

Prototype: `03-list-keys-empty-box.png`, `04-list-typed-s.png`, `05-list-values-exact.png`, `08-list-operators.png`, `09-list-many.png`, `12-list-390.png`.

- A white panel under the box, 4 px below it, left edges aligned. At least as wide as the box, at most 520 px or the window width less the page padding. 1 px `--color-control-border`, `--radius-control`, the toast's shadow. `z-index` 30 (below the dialog's 40).
- Height: eight options, then it scrolls; the active option is kept in view.
- **Caption** above each group: 12 px, weight 500, muted. Not an option.
- **Option**: 36 px high (44 px when the pointer is coarse). Left, the text that goes into the box, monospace 13 px. Right, a muted 12 px note, one line, cut with an ellipsis at 26 ch.
- **Active option** (moved to with the arrows): `--color-panel` background **and** a 3 px `--color-accent` bar at its left edge. **Hover**: the background only. Two signals, so the active one is not told by colour alone.
- **"N more" line**: muted 12 px, a rule above it, **pinned to the bottom of the list** so it is visible without scrolling.

### 3.3 What the list holds

Caret positions and whole-token replacement are DESIGN-R3's. The content of each list is:

| Caret position | Caption(s) | Options, left text and right note |
| --- | --- | --- |
| Start of a requirement, nothing typed (`|`, `site=sjc,|`) | **Label keys** | Every key. Note: the key's first three values in the API's order joined by ", ", then ", …" when there are more; an empty value shows as "(empty)". Example: `size` — "extralarge, large, medium" |
| Start of a requirement, something typed (`si|`) | **Label keys**, then **Values** | Keys that match, with the same note. Then, under **Values**, every value of any key that matches what was typed, written as the whole term: `site=sjc`. A value with no selector term is written `owner=Team A (infra)` with the note "exact match". Empty values are not listed here |
| After `!` (`!ti|`) | **Label keys** | Keys that match, with the note. No values |
| After a key and a space (`site |`) | **Match {key}** | `=` — "is"; `!=` — "is not"; `in (` — "is one of"; `notin (` — "is none of" |
| After `=`, `==`, `!=` (`site=|`) | **Values of {key}** | The key's values that match. No note, except: a value with no term carries "exact match" (after `=` and `==` only); the empty value is shown as `(empty)` with the note "empty value" |
| Inside a list (`site in (ams, |`) | **Values of {key}** | As DESIGN-R3: values with a term, not empty, not already in the list. No note |

- Value options do **not** repeat the resulting term (`site=sjc`) beside the value. This overrides DESIGN-R3.
- No counts of setups per value. `label_index` has none, and a count made from the fetched rows would follow the selector while the index does not.
- Matching and order inside a group: DESIGN-R3's (ignore case; starts-with before contains; the API's order inside each). Under **Values**, the match is against the value, not the key.
- Limit: 50 options in all; keys are listed before values. The pinned last line reads **{n} more. Keep typing to narrow.**
- **Nothing matches** while something is typed and the caret is at a key or value position of a known key: the list stays open with one line that is not an option: **No label matches "{typed}".** at a key position, **No value of {key} matches "{typed}".** at a value position. Enter still sends the box as typed. A key that is not in the index (`nosuch=|`) shows no list, as in DESIGN-R3.

### 3.4 What a pick does

| Picked | Box afterwards | Filter applied? | List |
| --- | --- | --- | --- |
| A key | The token becomes `key=` (after `!`: `key`, nothing added) | No | Opens on **Values of {key}** at once |
| An operator | The operator is inserted | No | Opens on the values |
| A value after `=`, `==`, `!=` | The value token is replaced | **Yes** | Closes |
| `(empty)` | `key=` | **Yes** | Closes |
| A whole term under **Values** | The token becomes the API's term, `site=sjc` | **Yes** | Closes |
| An "exact match" value, under **Values** or after `key=` | The requirement leaves the box, with one adjoining comma | No request; the token is added and the rows are filtered in the browser | Closes |
| A list item | The item is replaced; no comma, no `)` | No | Closes; typing `,` opens it again |

Focus stays in the box after every pick, the caret after the inserted text.

### 3.5 Filter button, Enter, apply on pick

- **Apply on pick** for a value, as above. It is the one-action filtering the chips gave.
- **Enter** with no active option, or the **Filter** button: sends the box as typed (trimmed), as today. This is for hand-written selectors and for lists (`in (…)`), which a pick does not apply.
- Typing alone never applies.
- Nice to have: while the trimmed text differs from the applied selector, the Filter button is the `primary` variant and is described (`aria-describedby`, visually hidden) by "The typed selector is not applied yet." Its name stays "Filter".

### 3.6 The exact-match token

Lives in the filters-in-force line (2.3), after the lead-in **Exact match:**, not beside the Filter button. Prototype: `06-token-and-count.png`.

- Look: the product's label chip (`Chip`: monospace 12 px, `--color-panel` background, `--radius-chip`) with a 1 px `--color-control-border`, holding the text **`key=value`** with no spaces around `=`, cut at 40 ch with the full text in `title`; joined to it on the right, a square button at least 28 × 28 px showing **×**.
- The × button's accessible name: **Remove exact-match filter {key}={value}**.
- The tokens are a list (`ul`) named by the lead-in, so a screen reader says "Exact match, list, 1 item".
- The words "exact match" appear once, as the lead-in, not on every token.
- Behaviour is DESIGN-R3's: one value per key, combines with the selector and State, never goes to the API, stays when its value leaves the index, untouched by edits to the box. Removing a token moves focus to the selector box.

### 3.7 An invalid selector

Prototype: `07-error.png`.

- The box gets the red border (`aria-invalid`), as built.
- Line 1 under the box, red 13 px: the API's message as sent, for example **selector: expected a value at position 9**.
- Line 2, muted 13 px: **Forms: key=value, key!=value, key in (a,b), key notin (a,b), key, !key. Commas mean AND. The table still shows the last filter that worked.** The first two sentences are the wording Settings already uses under the rate rules.
- Both lines are in the box's `aria-describedby`. The error is also announced (3.10), because after a pick focus never leaves the box and a description is not read again.
- The error and the help line go when the next request succeeds.

### 3.8 Keyboard and touch

DESIGN-R3's key table stands (typing opens and narrows; Arrow Down and Up open and move; Enter picks the active option or submits; Escape closes the list and never clears the text; Tab closes and picks nothing; composition is left alone; a click on an option keeps focus in the box). Added or stated more exactly:

- **Home and End** move the caret, never the active option.
- **Escape** with the list closed does nothing on this page; with the details panel open it closes the panel, as before.
- **Touch**: an option is 44 px high; a tap picks; a drag scrolls the list and picks nothing; a tap outside closes the list. The arrow button is the way to open the list without the on-screen keyboard having been asked for text first. The list never extends past the window's side padding.
- **Tab order of the whole filter area**: State, the box, Filter, each token's × in order, Clear filters. The arrow button is not a stop.

### 3.9 The engineer's three open questions

| # | Question | Decision | Reason |
| --- | --- | --- | --- |
| R3-Q1 | Apply on pick? | **Yes**, for a value and for a whole term; not for a key, an operator or a list item | One action from "I want a large one" to a filtered table. The Filter button and Enter remain for typed text |
| R3-Q2 | Open on click and Arrow Down in an empty box? | **Yes**, and also from the arrow button. **Not** on focus by Tab | It is the only replacement for seeing the labels. Opening on Tab would drop a panel over the table for everyone passing through with the keyboard |
| R3-Q3 | Stored display keys: leave or clear? | **Leave. Nothing on any page refers to them.** | No user-visible effect either way; R3-1 says the value stays |

### 3.10 Screen-reader wording

Roles and attributes are DESIGN-R3's (combobox, listbox named "Label suggestions", options, `aria-activedescendant`). Each group is a `role="group"` inside the listbox, named by its caption. The no-match line and the "N more" line are not options.

What an option says: its left text, then its note. A key: "size, values: extralarge, large, medium" (the words "values:" are visually hidden). An operator: "=, is". A value: "sjc". An exact value: "Team A (infra), exact match".

One polite status line, visually hidden:

| When | Text |
| --- | --- |
| The list opens or its content changes | **{n} suggestions.** / **1 suggestion.** / **No suggestions.** |
| A response arrives after the selector, the State filter or a token changed | **{n} setups shown.** / **1 setup shown.** / **No setups match the filters.** |
| A token is added | **Exact-match filter {key}={value} added.** |
| A token is removed | **Exact-match filter {key}={value} removed.** |
| The API refuses the selector | **Selector not accepted. {the API's message}** |
| Clear filters | **Filters cleared.** |

The 60-second refresh announces nothing.

---

## 4. The cost cell and the Held by cell

### 4.1 Cost

Wording **confirmed** as built: `2.0 now · up to 6.0`; one value (the cost now) when the cost now is not below the cost at the highest band; the unit in the header only, **Cost (tokens / h)**; no "if acquired"; the tag words `reported` and `override`; the button's accessible name "2.0 tokens per hour now, up to 6.0. Price details for setup-01" and the two hidden tag sentences. `0.0 now · up to 2.4` in a quiet lab reads correctly in the built shot S1 and in the prototype (`02-page-1280-0x.png`).

Visual treatment **changed** (prototype `01-page-1280.png` beside `00-as-built-without-labels-1280.png`):

1. **The cost is a number first and a control second.** The button is in the text colour, not link blue. It keeps a dotted underline in `--color-control-border`, 4 px below the text, as the sign that it can be activated; on hover it turns `--color-accent` with a solid underline; on keyboard focus it gets the product's focus ring. The affordance is the underline, not a colour.
2. **The cost now is weight 600.** The word "now" is regular. "· up to 6.0" is muted. A single value is weight 600.
3. **The source tag sits on the same line**, 8 px after the button, not on a second line. Same badges as today (`reported` blue, `override` grey), so the tag looks the same here as in Setup inventory.
4. **A second line appears only for a hold that is not charging yet**: the policy's reason, muted 12 px, 12 em wide, wrapping, with `overflow-wrap: anywhere` (task 10 minor 1).

Result in the prototype: a row without a not-charging reason is one line of text; the built page had two lines on every tagged row.

### 4.2 Held by

Wording **confirmed**: the holder's name or **You**, a middle dot, the time held (`You · 3 h`, `Tom K. · 40 min`); a second line **overdrawn** in weight 600 amber 12 px when the holder is overdrawn; blank for a free or disabled setup.

Changed:

1. The name is cut at **24 ch**, not 12 (full name in `title`). There is room now.
2. **You** is weight 600, so a developer finds their own holds in the column at a glance.

---

## 5. The details panel

The dialog, its title **Details: {setup}**, the **Close** button, focus on the **Price** heading at open, the notice **This setup is no longer listed. These are the last values loaded.** and the dialog's width (520 px) are as built. Prototype: `10-panel-1280.png`, `14-panel-390.png`.

### 5.1 Order and headings

1. **Price** (h3)
2. **Holder** (h3), only when held
3. **Labels** (h3)
4. **Raw details** (h3)

"Labels and details" becomes two sections. Price stays first because both openers are about the price and focus lands there.

### 5.2 Price

The answer first, then the caveat, then the working.

| Line name | Value | Notes |
| --- | --- | --- |
| **Cost now** | **`1.6` tokens / h** | 16 px; the number weight 600. `formatRate`, then the words "tokens / h" |
| **Up to** | `2.4 tokens / h` then, muted 14 px, **at the highest multiplier (3×)** | 16 px. Without `pricing` (older API): the muted text is **at the highest multiplier**. When the cost now is above this cost: the whole value is replaced by **The multipliers were changed. This value shows after the next poll.** |
| **Charging** | **Since {time}** for a hold that is charging (**Yes** when there is no start time); **Not yet: {reason}** or **Not yet** for a hold that is not; **Starts: {description}** for a free setup when the API sends one (it sends none today); otherwise no line | The line name is not repeated in the value |

Then, when `reported_weights_enabled` is true, in the normal text colour: **Weights are reported by automation, so the price can change while the setup is held.** (the shared sentence, unchanged; it is a caveat on the two figures above, so it sits under them).

Then the explanation, muted 13 px, wording unchanged and with no equals sign: **Cost per hour is the setup's weight times the lab's multiplier, rounded to a thousandth of a token. The multiplier follows lab pressure.**

Then the working, 14 px:

| Line name | Value |
| --- | --- |
| **Weight** | `0.8 · reported (sent by automation)` and the other source wordings, as built |
| **Attributes** | `memory-gb 128 · tier silver`, only when there are any |
| **Lab pressure** | `25%`, or **not measured yet**; only with `pricing` |
| **Multiplier now** | `2×`; only with `pricing` |

The words "band" and "/ h" without "tokens" no longer appear in the panel. The row says "up to", the panel says "Up to".

### 5.3 Holder

As built: **Held by** (`You` or the full name), **Held since** (`Nov 24, 11:29 (3 h)`), **Balance** (`299.713 tokens`, or `−12 tokens, overdrawn`), and when set **Release** (`Release requested {time}. It completes when the setup is torn down.`).

The Price and Holder lists share one first-column width, **10 em**, so their values start at the same x (seen in the prototype, `10-panel-1280.png`).

### 5.4 Labels

This is now the only place a setup's labels are shown.

- A two-column list. Key left, muted; value right, text colour; **both monospace 13 px**, the face labels have in the selector box, the suggestion list, the token and My usage's chips, so a reader can see these are the things the selector takes.
- Order: by key, ascending, plain code-point order (the order of the suggestion list's keys).
- The key column is as wide as the longest key, up to 20 em; longer keys wrap. Values wrap anywhere; nothing is cut and nothing scrolls sideways (prototype: a 28-character key and a 54-character value).
- An empty value reads **(empty)** in the sans face, muted, so it cannot be taken for a value that is the text "(empty)".
- No labels: **No labels.**
- Under the list, muted 13 px, only when there is at least one label: **Use a label in the Label selector box to filter the list.**
- Below 480 px the list is one column: key, then its value under it.
- No per-label "filter by this" button. See "Proposed for later" for why.

### 5.5 Raw details

The JSON block as built (monospace 12 px, bordered, wraps, no scroller of its own, accessible name "Setup details JSON"). When `details` is null or an empty object: the text **None.** instead of the block.

---

## 6. Responsive and accessibility

### 6.1 1280 px

No sideways scroll of the page or the table, for a developer and for an admin, with a 32-character setup name (prototype: table 1152 px in a 1152 px box). Rows are 59 to 65 px high; the buttons set the height, not the text.

### 6.2 Narrow (checked at 390 px in the prototype)

- Title row wraps; the summary goes under or beside the heading.
- State select on its own line; under it the selector box and the Filter button on one line, the box taking the width left.
- The suggestion list is as wide as the box and never wider than the window less the page padding.
- The filters-in-force line wraps: tokens, then count and Clear filters.
- The table scrolls sideways **inside its own scroller**; the page does not. This needs `position: relative` on the shared `.scroller` (task 10 minor 2: the visually hidden texts in the cells otherwise widen the page). At 390 px the Setup and State columns are in view and the cost is one swipe away. That is how every table in Hourglass behaves; a stacked layout for phones is in "Proposed for later".
- The dialog takes the window's width less 16 px each side; the Price and Holder lists use an 8.5 em first column; the Labels list is one column.

### 6.3 Keyboard path and focus order

Top bar, then: State → Label selector box → Filter → each token's × → Clear filters → for each row: the cost button → Details → Release or Acquire.

- In the box: arrows, Enter, Escape and Tab as in 3.8. Focus never moves into the list; the active option is shown by `aria-activedescendant` and the bar-and-background look.
- Cost button or Details, Enter or Space: the panel opens with focus on **Price**. Tab reaches **Close** (the panel body has no control). Escape or Close returns focus to the opener, or to the `h1` when the opener is gone. As built.
- After a control removes itself (a token's ×, Clear filters), focus goes to the selector box.
- Every focusable thing shows the product's 2 px focus ring.

### 6.4 Contrast, and nothing by colour or hover alone

| Thing | How it is told | Contrast |
| --- | --- | --- |
| Muted text (`#5c5c5c`) | | 6.6:1 on white, about 6:1 on the dialog's grey (computed, not measured in the build) |
| The cost can be activated | Dotted underline; pointer and colour change on hover are extra | Underline `#8a8a8a` on white, 3.5:1 (computed) |
| Active option | Bar at the left edge and a background | Bar `#1d4ed8`, well above 3:1 |
| Overdrawn | The word | Amber text `#92400e` on white, above 7:1 |
| Source of the price | The word in the badge, and a hidden sentence | As built |
| Exact-match filter | The lead-in words and the token's text | |
| Invalid selector | Red border, the message in words, `aria-invalid`, an announcement | Red `#991b1b` on white, above 7:1 |
| A filter is in force | The filters-in-force line with its count and Clear filters | |
| Full text of a cut name | `title` (hover) **and** the panel (name in the dialog title; holder's full name in Holder) | |

Nothing on the page is reachable by hover only. `title` attributes repeat text that is available elsewhere.

---

## 7. Admin Settings page

The "Display label keys" field is removed (R3-1). What the Rate rules card looks like afterwards, top to bottom:

1. Heading **Rate rules**
2. Note: **First matching rule sets a setup's weight: tokens per hour at 1×.**
3. One field, alone on its line, 120 px wide: **Default weight**, hint **Used when no rule matches.**
4. The rules table (or **No rules: every setup uses the default weight.**), **Add rule**, and the rest of the card unchanged, ending with the note **Selectors: key=value, key!=value, key in (a,b), key notin (a,b), key, !key; commas mean AND.**

- The field's label, its hint ("Comma separated; shown as columns on Setups. Every label of a setup is in its details, so a short list hides nothing.") and its error slot all go with it. Nothing replaces it: no note saying the setting moved or is hidden.
- No gap is left: the line that held two fields holds one, at the left. The wrapper may stay or go as long as the spacing above the rules table is the card's normal 12 px gap.
- Nowhere on the page do the words "Display label keys", "columns on Setups" or "display label" remain.
- A validation error the API returns for `display_label_keys` (it cannot be caused from the page now, since the value is sent back unchanged) has no field to sit under. If one ever arrives it must show in the page's general save error, not be dropped. Inference: I did not trace how a field error with no field is shown; the builder checks it.

---

## 8. Change list for the builders

Each item is one checkable difference. **must** = the build is not accepted without it; **should** = expected, may be argued; **nice** = if time allows.

### (a) Against the code as built (in addition to DESIGN-R3's removals, which stand)

| # | Level | Change | Where |
| --- | --- | --- | --- |
| A1 | must | Cost button: text colour, dotted underline in `--color-control-border` with 4 px offset; hover turns accent with a solid underline. Not `.linkButton` | `index.tsx` cost cell, `setups.module.css` |
| A2 | must | In the cost button the cost now is weight 600; "now" regular; "· up to X" muted. A single value is weight 600. The visible text and the accessible name are unchanged | same |
| A3 | must | The source tag is on line 1, 8 px after the cost button. The second line (`.costNote`) holds only the not-charging reason, and has `overflow-wrap: anywhere` | same |
| A4 | must | Filter row is top-aligned; the Filter button is offset by one label line; the box-and-button form does not wrap. A message under the box moves no control | `setups.module.css` `.filters` |
| A5 | must | Panel, Price: order is Cost now, Up to, Charging; then the price-change sentence; then the explanation sentence (muted 13 px); then Weight, Attributes, Lab pressure, Multiplier now | `SetupPanel.tsx` |
| A6 | must | Panel: both costs read `{rate} tokens / h`; the second line is named **Up to** and carries "at the highest multiplier (3×)"; the band-change text is "The multipliers were changed. This value shows after the next poll." | `SetupPanel.tsx` |
| A7 | must | Panel: headings **Labels** and **Raw details** replace "Labels and details" | `SetupPanel.tsx` |
| A8 | must | Shared table scroller gets `position: relative`, so at narrow widths only the table scrolls sideways | `Table.module.css` |
| A9 | should | Cost now and Up to values are 16 px with the cost-now number weight 600 | `setups.module.css` |
| A10 | should | Panel, Charging values: "Since {time}", "Yes", "Not yet: {reason}", "Not yet", "Starts: {description}" | `SetupPanel.tsx` |
| A11 | should | Price and Holder lists use a fixed 10 em first column (8.5 em below 480 px) | `setups.module.css` |
| A12 | should | Labels list: monospace 13 px key and value, sorted by key, own grid (key column up to 20 em), "(empty)" in muted sans, one column below 480 px | `SetupPanel.tsx`, CSS |
| A13 | should | Under the labels: "Use a label in the Label selector box to filter the list." | `SetupPanel.tsx` |
| A14 | should | Holder name cut at 24 ch; "You" weight 600 | `index.tsx` holder cell |
| A15 | should | Text typed in the selector box is monospace 13 px; the placeholder is sans 14 px muted | CSS |
| A16 | nice | Raw details shows "None." when `details` is null or `{}` | `SetupPanel.tsx` |

### (b) Against DESIGN-R3

| # | Level | Change | Replaces in DESIGN-R3 |
| --- | --- | --- | --- |
| B1 | must | Placeholder "Type or pick a label, e.g. {term}", the term being the first with a selector that is at most 16 characters; "Type or pick a label" when none is; none with an empty index | "the first non-null term" (`placeholderFrom`); the test `UI-9 the placeholder is …` is reworded to match |
| B2 | must | Every group in the list has a caption: "Label keys", "Values", "Values of {key}", "Match {key}" | A flat list |
| B3 | must | A key option carries a note with the key's first three values, then ", …" | Key text only |
| B4 | must | At the start of a requirement with text typed, values of any key that match are listed under "Values" as whole terms; picking one inserts the API's term and applies; an exact one adds a token | Keys only at that position |
| B5 | must | Value options carry no resulting term; only "exact match" and "empty value" notes | "the value with the resulting term in muted text" |
| B6 | must | Tokens are in the filters-in-force line under the filter row, after one lead-in "Exact match:". Token text is `key=value`, monospace, chip look, with a × button of at least 28 × 28 px | "beside the Filter button"; text `key = value`; "exact match" on each token |
| B7 | must | The filters-in-force line has **Clear filters**, which resets State, the box, the applied selector and the tokens, and moves focus to the box | No clear-all |
| B8 | must | Active option: panel background and a 3 px accent bar; hover: background only. Options 36 px, 44 px for a coarse pointer | `aria-selected` with no stated look |
| B9 | must | List geometry: under the box, at least its width, at most 520 px or the window less the page padding, `z-index` 30 | "sits under the box" |
| B10 | must | Status-line texts of section 3.10, including the announcement of a refused selector and of the row count after a filter change | The four texts listed there |
| B11 | must | Groups are `role="group"` named by their captions; a key option's name includes "values: …" | Flat listbox |
| B12 | should | The count "{n} setups shown" in the filters-in-force line | None |
| B13 | should | When typed text matches nothing: one line, "No label matches "{typed}"." or "No value of {key} matches "{typed}"." | The list does not open |
| B14 | should | The arrow button at the right of the box (outside the Tab order, name "Show label suggestions") | Click or Arrow Down only |
| B15 | should | Operator options carry the words "is", "is not", "is one of", "is none of" | Symbols only |
| B16 | should | "N more. Keep typing to narrow." is pinned to the bottom of the list | Last line of the scrolling list |
| B17 | should | Under the API's error: the muted line "Forms: … Commas mean AND. The table still shows the last filter that worked." | The API's message alone |
| B18 | nice | Filter button is `primary` and described by "The typed selector is not applied yet." while the box differs from the applied selector | None |

Agreed with DESIGN-R3 and not listed: caret positions, whole-token replacement, no active option until the user moves, Enter and Escape and Tab, composition, 50 options, eight visible, behaviour of tokens, the old-API fallback, every removal in its section 2, the Settings round trip, all tests except the wording noted in B1 and the token's place in the page.

Effect on DESIGN-R3's test names (it stays the authority; this is a note): B4 needs one more case in `selectorSuggest.test.ts`; B7 one more in `setups.test.tsx`; the e2e step A2f ("type `si`, pick `size`") still holds because keys are listed before values.

Totals: **must 19** (A1 to A8, B1 to B11), **should 13** (A9 to A15, B12 to B17), **nice 2** (A16, B18).

### Proposed for later (outside this job; not for the builders)

1. **Stacked rows on phones.** Below about 600 px each setup as a block (name and state, holder, cost, actions) instead of a table that scrolls sideways. It is a change to the shared table.
2. **"Filter by this label" in the panel.** A control on each label line that puts the term in the selector box. Left out now because the panel body has no control today, and the dialog gives first focus to the first control in its body (`Modal.tsx`), so adding one would move the opening focus off the Price heading.
3. **Counts in `label_index`** (how many setups carry each value), to show beside values. A new API field.
4. **Filters in the URL**, so a filtered view survives a reload and can be shared. Today every filter is lost on reload.
5. **Suggest `key` and `!key`** ("has this label", "lacks this label") as worded options.
6. **One opener per row.** The cost and "Details" open the same panel. The owner's request names both; with experience one may prove enough.
7. **Fixed column widths**, so the columns do not move when a filter changes the rows (seen in the prototype between `01` and `06`).

---

## 9. Acceptance checks for the UX review of the finished build

Run in Chromium at 1280 × 900 and at 390 × 844, as a developer and as an admin, reported weights on.

**Page**

1. The table's headers are exactly Setup, State, Held by, Cost (tokens / h), and the hidden "Actions". No label column, no "Available labels" heading, no chip.
2. At 1280 px neither the page nor the table scrolls sideways, with a 32-character setup name.
3. A row with a tag and no not-charging reason is one line of text in the cost cell; the tag is beside the cost.
4. The cost is in the text colour with a dotted underline; the cost now is bolder than the rest; "Details" is the only blue link on a row.
5. With the lab at 0× a row reads `0.0 now · up to X`; with equal costs, one value.
6. "You" is bold; a 20-character holder name is not cut; an overdrawn holder has the word "overdrawn".
7. The placeholder is readable in full at 1280 px and begins "Type or pick a label".

**Selector box**

8. Clicking the empty box, clicking its arrow, and Arrow Down each open "Label keys" with a sample of values beside each key. Tabbing into the box does not open it.
9. Typing part of a value that is not part of any key lists it under "Values" as `key=value`; picking it filters the table in one action and the box holds the term.
10. Picking a key gives `key=` and the list "Values of {key}"; picking a value applies.
11. Typing a whole selector by hand and pressing Enter sends exactly that; with the list open and no option active, Enter still submits.
12. Escape closes the list and keeps the text. Tab leaves and picks nothing.
13. Typed text that matches nothing shows the "No label matches" line.
14. With more than 50 matches the "N more" line is visible without scrolling the list.
15. An invalid selector: red border, the API's message, the help line; State, the box and Filter stay on one line and do not move when the message appears or goes.
16. The list is not cut by the table, is not wider than the window at 390 px, and is under the dialog when a panel is open.

**Filters in force**

17. Picking an "exact match" value: the term leaves the box; the token appears after "Exact match:" on the line under the filter row; the rows are filtered; no request carried the value.
18. That line shows the count and "Clear filters" whenever State, the selector or a token is in force, and is absent otherwise.
19. "Clear filters" resets all three and focus is in the selector box. The token's × removes it and focus is in the selector box.
20. With filters that leave nothing: "No setups match the filters." and "Clear filters" is still on screen.

**Panel**

21. Opens from the cost and from Details, focus on "Price"; the first two lines are Cost now and Up to, with "tokens / h"; the word "band" is not in the panel.
22. Sections in order: Price, Holder (held only), Labels, Raw details.
23. Every label of the setup is listed, sorted by key, monospace; a long key and a long value wrap inside the dialog; an empty value reads "(empty)".
24. Values of the Price and Holder lists start at the same x.
25. At 390 px the dialog fits the window and the labels are one column.

**Keyboard and screen reader**

26. Tab order: State, box, Filter, token ×, Clear filters, then per row cost, Details, Release or Acquire. Every stop shows a focus ring.
27. With a screen reader (VoiceOver at least): the box is announced as a combo box named "Label selector"; moving in the list reads each option with its note; the count of suggestions, the count of setups after a filter change, a token added or removed, a refused selector and "Filters cleared" are each announced once.

**Settings**

28. The Rate rules card shows Default weight alone on its line with no gap; the page has no "Display label keys" text; saving another field keeps the stored keys (DESIGN-R3's UI-23).

**The four owner screenshots** (`S1-setups-0x`, `S1-panel-0x`, `S2-setups-2x`, `S2-panel-2x`; 1280 px; the panel shots of the viewport, not the full page)

| Shot | Must show |
| --- | --- |
| S1 page, lab at 0× | Five headers only; nothing between the filter row and the table; the placeholder in full; every cost `0.0 now · up to X` in the text colour with the tag beside it; no row taller than its buttons because of a tag |
| S1 panel | The whole dialog inside the viewport with an even backdrop; Cost now `0.0 tokens / h` first, Up to with "(3×)"; the price-change sentence; Multiplier now `0×`; the Holder section; the Labels section with **all** of the setup's labels (the five keys the scenario sets, and any other), in monospace; Raw details |
| S2 page, higher band | As S1 with costs such as `1.6 now · up to 2.4`; the top bar's multiplier agrees |
| S2 panel | As S1 panel with Cost now above zero and a Multiplier now that agrees with it |

Two more shots are asked for, because the owner's change is the box and the four above cannot show it: **S3**, the page with the list open on "Label keys" in an empty box; **S4**, the page with one exact-match token, a selector in the box, the count and "Clear filters".

---

## 10. Questions for the owner

Work proceeds on the default in each case.

| # | Question | Default |
| --- | --- | --- |
| UX-Q1 | The box keeps the name "Label selector" (your word, and the name on Settings). A newcomer may not know the term; "Filter by label" is plainer. Rename? | Keep "Label selector"; the placeholder says "Type or pick a label" |
| UX-Q2 | Typing a value (for example `large`) offers `size=large` directly, without picking the key first. Wanted? | Yes |
| UX-Q3 | While a filter is in force, one line under the filter row shows the exact-match tokens, "{n} setups shown" and "Clear filters". It is the only new line on the page. Acceptable? | Yes |
| UX-Q4 | The cost is no longer blue: it is a bold number with a dotted underline, and "Details" stays the blue link. Acceptable? | Yes |
| UX-Q5 | On a phone the table scrolls sideways and the cost is off screen until swiped, as on every Hourglass table. Leave for now? | Leave; stacked rows are proposed for later |
