# Design: reported weights

Status: approved direction, not implemented. Written 2026-11-23 against `main` at `753cc50` (release `v1.1.0`), checked against the code the same day, and revised after an independent design review.

This document is the brief for the agent that implements, tests and releases the work with the `/orchestrate` skill. It says what to build and why, and names the decisions the owner already made. Where it leaves a detail open, choose the simplest option that satisfies the numbered requirements in [Requirements](#requirements) and record the choice in `docs/decisions.md` from D80 on. If something would contradict this document, stop and ask the owner.

Customers already run Hourglass. Every change here must leave an upgraded installation working with no configuration change, and must leave existing API clients working. [Compatibility](#compatibility) lists what that means for each part.

## Goal

Today a setup's price is static: an admin's rate rules turn its labels into a weight, and the weight changes only when an admin edits a rule or a label. Teams trying Hourglass want the price of a setup to follow what the setup currently reserves, for example its provisioned memory and storage, and to come down when the setup shrinks.

After this work, a customer's own service can **report a weight per setup**. Hourglass applies it within limits the admin sets, shows it everywhere a price is shown, records it on every charge, and falls back to the rate rules whenever no valid report exists. Hourglass still never looks at the customer's infrastructure and never learns what the weight was computed from.

| | Problem | Outcome |
| --- | --- | --- |
| A | A weight can come only from a rate rule or the default. | A service account reports weights; the collector charges `reported weight × multiplier × elapsed hours`. |
| B | The collector, the Setups page and the admin pages each work out a weight on their own, and the pages read rate rules only. | One resolver gives every reader the same weight and says where it came from. |
| C | A charge does not say where its weight came from. | Every usage sample records the source, and the ledger shows the weight and source of each debit. |
| D | Nothing limits a weight that does not come from an admin. | The feature is off until an admin turns it on with a minimum and a maximum; an admin can pin one setup's weight. |

## Owner decisions

Made on 2026-11-23. Do not reopen them.

| Decision | Detail |
| --- | --- |
| The client computes the weight; Hourglass applies it. | Hourglass does not compute a weight from memory, storage or any other raw figure, although the first client offered that. Units and formulas differ per client and do not belong in a generic core. |
| The weight is a field of its own report on its own route. | Not a reserved label, and not a field of the deployments report. The route has its own scope, `weights:write`. |
| A reported weight may describe what a setup currently reserves. | This amends the spec's "bill what a setup reserves, not what it consumes": the reserved amount may now change over time and be reported. It should be provisioned size, not live utilisation; Hourglass cannot check this, and the integration guide says so. |
| A reported weight changes the debit and nothing else. | The snapshot that pressure sources and metering policies receive keeps the admin's weight (override, rate rule or default). A weights reporter therefore cannot move lab pressure or the multiplier, and existing Go extensions see what they see today. |
| A usable reported weight replaces the rate rules for that setup. | This includes a rule of weight 0. The way to exempt a setup is the per-setup override. No rule can switch reports off for a class of setups. |
| A report is accepted or refused as a whole. | One invalid entry refuses the report and stores nothing. |
| A weight outside the admin's limits is refused, never clamped. | A value over the maximum is most likely a reporter bug (a unit mistake). Clamping would charge the top price for a bug; refusing falls back to the admin's own rule price. |
| There is a minimum as well as a maximum. | Default minimum 0. A stolen `weights:write` token could otherwise make every setup free. |
| The switch and the limits are admin settings in the database. | Not the config file. Turning this on changes who sets prices, so it must be audited (ADM-3). |
| An expired or missing weight falls back to the rate rules. | It does not fail the poll. This differs on purpose from the deployments and pressure reports, where a stale report stops charging. |
| A setup left out of the report is charged at the fallback weight, whatever its state. | This includes the last interval of a release when the client stops reporting the setup at teardown. The product does not remember a removed weight. The guide tells clients to keep reporting a setup until its hold is gone. |
| Admins can see a refused report. | The admin collector view shows the last refused weights report and how many stored weights the limits exclude. |
| With the feature off the route answers 409 `not_configured`. | The existing code is reused; the message names the admin setting. |
| Backward compatible. | Off by default. No configuration change is needed after upgrade, and no route, parameter or response field is removed. |

## What the code does today

Read these before planning. Line numbers are as of `753cc50`.

**Where a weight comes from.**

- `rules.Weight` (`internal/rules/rules.go:84`) returns the weight of the first matching rate rule, or the default weight.
- The collector resolves every setup's weight once per poll (`internal/collector/collector.go:387`, "2. Resolve weights") and puts it in `hourglass.SetupState.Weight`, which the pressure source and the metering policy both see.
- A metering policy may replace the weight for one hold: `Decision.Weight` above 0 wins, and 0 means "use the rate rules" (`collector.go:602`, `docs/extending.md`). The bundled `default` policy always returns 0. This is the only way to vary a weight today, it needs Go, and it exists only for held setups.
- `pressure/inventory` computes pressure from `SetupState.Weight`: weight of held setups divided by weight of enabled setups.
- The poll that completes a pending release meters first and then releases, so that poll's interval is charged (D58, `collector.go:539` to `:559`).

**What a charge records.**

- `usage_samples` has `weight` and `rule_selector` (`internal/store/migrations/0001_init.sql`). `rule_selector` is null both when the default applied and when the policy overrode the weight, so the two cannot be told apart.
- A debit's ledger entry shows `multiplier`, found by joining `usage_samples` on hold and poll time (`DebitMultipliers`, `internal/store/apiuser_queries.go:147`). It shows no weight. Rolled-up entries (more than one poll) show no multiplier (LED-15).
- `usage_samples` is pruned after 13 months by a cutoff that is a moment in time, not a day boundary. Per-poll ledger rows are merged to one row per hold and day after 7 days (D69).

**Where a weight is shown.** Each of these works it out separately:

| Reader | Code | Weight used |
| --- | --- | --- |
| Setups list and its cost per hour | `listSetups`, `internal/api/handlers_user.go:764` | Rate rules |
| Admin setup views | `adminState.weight`, `internal/api/admin_views.go:136`, `:300` | Rate rules |
| Admin Users, burn per hour | `internal/api/admin_views.go:210` | Rate rules, even for a hold whose policy overrode the weight |
| A hold, active or released, and My usage burn | `lab.holdWeight`, `internal/api/user_views.go:173` | The hold's latest usage sample, else rate rules |
| `GET /api/v1/integration/state` | `internal/api/handlers_integration.go:230` | None is returned |

`GET /api/v1/me/holds` returns released holds as well as active ones (`handlers_user.go:189`).

**Reports and scopes.**

- `internal/reported` stores the pressure report and the deployments report, each whole, with `observed_at`, `received_at` and the sending account's name. It owns the ordering and clock-skew rules (`resolve`, `SubmitDeployments`). `observed_at` is optional and defaults to the time of receipt.
- The deployments report lists deployed setups only and replaces the stored set atomically (INT-2). Names that match no setup are stored, ignored and returned as `unknown`.
- Scopes are a fixed list in `internal/auth/service.go:38`. `auth.ServiceRoute(scope, handler)` guards each integration route.
- A report is checked in this order: scope, `not_configured`, body, ordering (D53).
- `decodeJSON` (`internal/api/helpers.go:51`) rejects unknown fields anywhere in a body, with a message that carries no position.

**Settings and setups.**

- `PUT /api/v1/admin/settings` requires all eight current keys (`settingsKeys`, `internal/api/handlers_admin.go:813`) and the `Settings` schema rejects unknown fields. `Settings.UnmarshalJSON` (`internal/settings/settings.go:282`) starts from `Defaults()`, so a key that is absent from the body is saved as its default.
- `POST` and `PUT /api/v1/admin/setups` take `name`, `labels`, `details` and `enabled`, all required. Changes are audited with a snapshot of the setup (`setupAudit`). A setup with an active hold cannot be renamed (`handlers_admin.go:531`).
- `GET /api/v1/admin/collector` reports the last pressure and deployments reports (D62).

## Non-goals

- Hourglass computing a weight from reported figures, a pricing formula, or rate rules that match reported attributes. The attributes in a report are for display only.
- A weight per hold or per user. The reported weight belongs to the setup.
- Smoothing or rate-limiting weight changes. Unlike pressure, a reported weight is applied as sent.
- A new Go extension point for weights, or any change to the types in `pkg/hourglass/types.go`. A Go policy keeps `Decision.Weight`, and any client can use the HTTP route. A Go policy cannot see the reported weight in this version.
- Remembering a weight after its setup leaves the report.
- An event or webhook when a weight changes.
- An audit entry per report. Reports arrive every minute; the stored report names its account, and each charge records its weight and source.
- Several services each reporting part of the set. As with deployments, one report is the whole set.
- Changing how the deployments and pressure reports behave.

## A. Resolving a weight

### A1. One resolver

Add one function, in a new core package (suggested: `internal/weights`), that every reader calls. Given a setup, the rate rules, the settings, the stored weights report and a time, it returns:

- the **weight** to apply;
- its **source**: `override`, `reported`, `rule` or `default`;
- the **base weight**: what the weight is without any report, which is the override, else the first matching rate rule, else the default;
- the selector of the first matching rate rule, if any, whatever the source;
- the setup's **reported** weight, when reported weights are on and the stored report has an entry for it, usable or not;
- why that reported weight is **not applied**: `expired`, `out_of_bounds` or `override`, or nothing;
- the entry's **attributes**, when the entry is usable (see below).

The weight is the first of these that exists:

| Order | Source | Applies when |
| --- | --- | --- |
| 1 | `override` | The setup has a weight override ([D2](#d2-per-setup-override)). |
| 2 | `reported` | Reported weights are on, the stored report has an entry for the setup's name, the report's `observed_at` is no older than `max_age` at the given time, and the weight is within the minimum and maximum now in force. |
| 3 | `rule` | A rate rule matches (RULE-1). |
| 4 | `default` | Otherwise. |

Rules:

- An entry is **usable** when reported weights are on, the report is not expired and the weight is within the limits. The limits are inclusive at both ends.
- The limits are checked again at read time, so a stored weight that an admin's new maximum excludes stops applying at once.
- When more than one reason holds, `not_applied` is the first of `expired`, `out_of_bounds`, `override`.
- With reported weights off, the resolver behaves as if no report were stored: no reported weight, no `not_applied`, no attributes.
- Age counts from `observed_at`, never below zero, as for the other reports (D46).
- Attributes are returned for a usable entry. An override does not hide them.
- A usable reported weight replaces the rate rules for that setup, including a rule of weight 0.
- With reported weights off, the result for every setup without an override is exactly what `rules.Weight` returns today.
- A rate rule that cannot be evaluated keeps its current effect in each caller: it fails the poll in the collector and the request in the user views, and the admin views fall back to the default weight.

### A2. The collector

- In step 1 of `poll`, read the stored weights report with everything else. A database error on that read fails the poll like any other step-1 read. A report that is missing, expired or unusable never fails a poll.
- In step 2, call the resolver for every setup at the poll time `T`.
- **The snapshot keeps the base weight.** `SetupState.Weight` is the base weight, so a pressure source and a policy receive what they receive today, plus the override. A weights report changes no pressure value and no multiplier. Update the field's comment in `pkg/hourglass/types.go` to say this; change nothing else in that file.
- **The debit uses the resolved weight.** `Decision.Weight` above 0 still replaces it for that hold and poll, and the sample's source is then `policy`.
- Each usage sample records the source in a new column, `weight_source`. `rule_selector` stays as it is: set only when the source is `rule`.
- The arithmetic is unchanged (MET-3, MET-4, D19): a poll charges the whole time since the previous successful poll at the weight resolved at this poll. There is no pro-rating inside an interval. A weight reported one second before a poll prices the whole interval that poll charges; no earlier sample or ledger row is touched.
- **A metered hold whose setup has no usable reported weight is charged at the fallback weight**, in every state of the hold. Two cases follow from this and must be tested as written, not worked around:
  - A hold metered by the time trigger before its setup is reported is charged at the fallback weight.
  - The poll that completes a pending release charges its interval (D58). If the client has already dropped the setup from its weights report, that last interval is charged at the fallback weight.

### A3. Every other reader

Replace each row of the table in [What the code does today](#what-the-code-does-today) with a call to the resolver at the request's time:

- **Setups list, admin setup views, the state route:** the resolver's weight and source.
- **An active hold, My usage burn, Admin Users burn:** the resolver's weight for the hold's setup. The exception is a hold whose latest usage sample has the source `policy`: it shows that sample's weight and the source `policy`. A sample with no source (written before the migration) counts as not `policy`.
- **A released hold:** the weight and source of its last usage sample, which is what it was last charged at. The source is null when that sample has none. A released hold with no sample shows the weight its setup resolves to now, as today.
- Cost per hour, now and at the highest band, is computed from that weight as today.

Two things change for existing installations even with the feature off, and both are deliberate:

- An active hold's weight used to be its last sample's weight, so after an admin edited a rule it changed at the next poll. It now changes at once, like the Setups list always has.
- Admin Users burn now honours a policy's weight.

What "the screen matches the charge" means: every screen shows the weight the next poll will charge if nothing changes before it, and the ledger shows the weight each past poll did charge. A screen and a later poll can differ only because something changed in between (a new report, an expiry, an admin edit).

## B. The weights report

### B1. `PUT /api/v1/integration/weights`

Scope `weights:write`, a new scope. No other scope allows this route, and this scope allows no other route.

```json
{
  "observed_at": "2026-11-23T10:15:00Z",
  "weights": [
    {"setup": "lab-03", "weight": 1.15, "attributes": {"memory-gb": "512", "storage-gb": "4000", "tier": "gold"}},
    {"setup": "lab-07", "weight": 0.8}
  ]
}
```

- `observed_at` is optional and defaults to the time of receipt, as in the other reports.
- The list is the **complete** set of reported weights, from one writer. A setup that is absent has no reported weight and falls back. An empty list is valid and means no setup has one. A missing or null `weights` is 400 `validation`.
- A weight may be reported for any setup: deployed or not, held or free, enabled or disabled. Whether a setup is deployed plays no part.
- `setup` is the setup's name in Hourglass, a non-empty string, not repeated in one report.
- `weight` is required and must be a JSON number: finite, and within the minimum and maximum in force, both inclusive. A string such as `"1.15"`, `null`, a boolean, and a number too large for a 64-bit float are all refused. Negative zero counts as 0. 0 is a valid weight when the minimum allows it, and makes the setup free.
- `attributes` is optional: at most 16 entries, keys under the label key rule (SET-3), values strings of at most 63 characters. Hourglass stores them, returns them and shows them. It never interprets them, and no selector matches them.
- An entry may hold only these three fields. Anything else in an entry is refused.
- A name that matches no setup is validated like any other entry, stored, ignored, and listed in the response's `unknown`, as for deployments.
- Limits: 10,000 entries and a 1 MB body. The body limit is the one a large report with attributes reaches first, and a report cannot be split across calls.
- Response 200: `{"observed_at": "...", "received_at": "...", "expires_at": "...", "accepted": 2, "unknown": []}`. `expires_at` is `observed_at` plus `max_age`.

### B2. Refusals

Checks run in the order of D53: scope, then whether reported weights are on, then the body, then ordering.

- **Off.** With reported weights off the route answers 409 `not_configured` whatever the body, and the message names the `reported_weights` setting and says an admin turns it on. Nothing is stored. A report is never silently dropped.
- **Invalid entry.** 400 `validation`, with `field` `weights` and a message that always gives the entry's position, gives its setup name when the entry has one that is a non-empty string, and gives the reason. Example: `weights[3] (setup "lab-03"): weight 25 is above the maximum 20`. The first invalid entry is reported. Nothing is stored and the previous report stays in force until it expires.
- To give the position and name for a wrong type, an out-of-range number or an unknown field inside an entry, decode each entry as raw JSON and check it yourself. Do not let the shared decoder fail the whole body for a problem inside one entry.
- A body that is not valid JSON at all, which includes the bare tokens `NaN` and `Infinity`, gets the existing "not valid JSON" 400 and cannot name an entry.
- **Ordering and clock skew** are the rules of the other reports: an `observed_at` older than the stored one is 409 `stale_report`, an equal one replaces it, and more than 2 minutes ahead is 400 `validation`. Reuse `internal/reported`. The weights report's ordering is independent of the deployments report's; the two may carry the same `observed_at` or different ones.
- **The last refusal is kept for admins.** Record the time, the account's name and the message of the most recent report refused with 400. Keeping it in memory is enough; it may be lost at a restart, like the poll view (D20). It is shown through [C2](#c2-user-and-admin-api).

### B3. Storage and effect

- The report is stored whole and replaced atomically, like the deployments report, and survives a restart.
- It takes effect at the next poll and in the next read. It never triggers a poll, a debit or an event.
- **The stored weight follows the name.** A setup created later with a stored unknown name takes that weight. A free setup renamed to a name the report holds takes that name's weight at once, and the weight stored under its old name no longer applies to it. The override ([D2](#d2-per-setup-override)) follows the setup, not the name.
- **Turning the switch off deletes nothing.** The resolver ignores the stored report while the setting is off. If the setting is turned on again while the report is still within `max_age`, it applies again. Raising `max_age` likewise makes a report that had expired usable again.
- **The two reports are separate calls**, so a poll can run between them.
  - At the start of a hold this is harmless: the poll that first sees a setup deployed starts metering and debits nothing (D19).
  - At the end it is not. A client that drops a setup from its weights report at teardown has the last interval charged at the fallback weight ([A2](#a2-the-collector)). The guide tells clients to send weights before deployments, and to keep reporting a setup's weight until its `hold` is null in the state.

### B4. `GET /api/v1/integration/state`

Added fields only. At the top level:

```json
"weights": {"enabled": true, "min": 0, "max": 20, "observed_at": "...", "expires_at": "..."}
```

`max` is null when no maximum is set. `observed_at` and `expires_at` are null when reported weights are off or before the first report, as D52 does for the other reports.

On every setup:

```json
"weight": {"applied": 1.15, "source": "reported", "reported": 1.15, "not_applied": null,
           "attributes": {"memory-gb": "512"}}
```

- `applied` and `source` are the resolver's answer now. A service checks that its report was taken by reading `source: "reported"` and `applied` equal to what it sent.
- `reported` is the stored entry's weight, or null. `not_applied` is null, `expired`, `out_of_bounds` or `override`. With reported weights off both are null for every setup.
- `attributes` is `{}` when there are none or the entry is not usable.
- The rate rule's selector is not returned here: it is admin configuration (INT-8). The route still returns nothing about balances, the ledger, audit data, or settings other than the switch and limits above.

## C. Charges and what users see

### C1. Usage samples and the ledger

- `usage_samples.weight_source` is `override`, `reported`, `rule`, `default` or `policy` for every new sample. Rows written before the migration keep null.
- `LedgerEntry` gains three optional fields, always present on `GET /api/v1/me/ledger` and absent where the schema is shared with the grant and reset responses (as `entry_count` is, D72):
  - `weight_min` and `weight_max`: the lowest and highest weight among the charged polls the entry stands for. For a single poll they are equal.
  - `weight_sources`: the distinct sources among those polls, `policy` first and then in the order of the table in A1.
- They are computed from the usage samples of the same hold whose poll time falls in the entry: the one poll for a raw row, the local day for a merged or rolled-up one. Only samples with a debit above zero count.
- **Partial data shows as unknown.** `weight_min` and `weight_max` are null unless the number of such samples found equals the entry's `entry_count`. `weight_sources` is null when the weights are null or any of those samples has no source. This covers the day of the upgrade, the day the 13-month prune cuts through, and entries whose samples are gone. All three are null for entries that are not debits.
- Unlike `multiplier`, these fields are filled for rolled-up entries, because a range is true of a whole day and a single value is not (LED-15 stays as it is).

### C2. User and admin API

Added fields only.

- `GET /api/v1/setups`: a top-level `reported_weights_enabled` boolean, so the page can warn that prices may change even for a setup that has no reported weight yet.
- `Setup` in that response, and the admin setup view: `weight_source` and `weight_attributes` (an object, `{}` when none). `weight` and `cost_per_hour_tokens` keep their names and now come from the resolver.
- The admin block of a setup: `reported_weight` (number or null), `reported_weight_not_applied` (the `not_applied` value or null) and `weight_override` (number or null).
- `Hold`: `weight_source` (may be `policy`; null for a released hold whose last sample has no source) and `weight_attributes`. The `burning` entries of `GET /api/v1/me`: `weight_source`.
- `GET /api/v1/admin/collector`, null when reported weights are off:
  - `last_weights_report`: `{observed_at, received_at, service_account, entries, out_of_bounds}` or null. `entries` is how many weights the stored report holds; `out_of_bounds` is how many of them the limits now exclude.
  - `last_weights_refusal`: `{at, service_account, message}` or null.

### C3. UI

Every number still comes from the API (UI-1).

- **Setups.** The weight cell shows the weight and, when the source is `reported` or `override`, a small tag with that word. Attributes appear as one secondary line under it, for example `memory-gb 512 · storage-gb 4000 · tier gold`. They are not columns and not filters.
- **Acquire dialog.** Shows the weight, the attributes, and the cost as today. When `reported_weights_enabled` is true it adds one sentence, whatever the setup's current source: weights are reported by automation, and the price can change while the setup is held.
- **My usage.** A hold row shows its weight with the same tag. The burn line keeps its form. A ledger debit shows its weight after the multiplier: one value when `weight_min` equals `weight_max`, otherwise a range such as `1.15 to 1.40`, with the source when it is not `rule` or `default`, and nothing when the weight is unknown.
- **Admin, Setup inventory.** Shows the weight and source, the reported weight when there is one, and in words why a reported weight is not applied. The setup form gains a weight override field that can be cleared.
- **Admin, Settings.** A "Reported weights" section: on or off, minimum, maximum, maximum age, when the last weights report arrived and from which account. It shows a warning when stored weights are outside the limits, with the count, and when a report was refused after the last accepted one, with the refusal's time, account and message. It notes that the maximum age should be at least twice the poll interval. Turning the switch on without a maximum shows the server's validation message.
- **Admin, Service accounts.** The scope list gains `weights:write`.

A weight that changes during a hold needs no special state: the hold shows the current weight and the total charged so far, and the ledger shows what each charge used.

## D. Admin controls

### D1. Settings

One new settings key, `reported_weights`, holding an object:

| Field | Default | Rule |
| --- | --- | --- |
| `enabled` | `false` | Can be true only when `max` is set. |
| `min` | `0` | Finite, at least 0, at most `max` when `max` is set. |
| `max` | `null` | Null, or finite, above 0 and at most 1e12 (the limit for a rule's weight). |
| `max_age` | `10m` | A duration string from `1m` to `24h`. |

- It is read and written through `GET` and `PUT /api/v1/admin/settings` and audited like the other keys (ADM-2, ADM-3), with the whole object as the before and after value.
- **The key is optional on `PUT`.** A body without it leaves the stored value unchanged. It is not added to the schema's `required` list or to `settingsKeys`. Take care with how the handler builds the next settings: today an absent key is saved as its default, which here would switch the feature off and clear the limits.
- **When the key is present, all four fields are required.**
- `GET` always returns the key.
- A change applies from the next poll and the next read, and is not retroactive (ADM-2).
- `max_age` should be at least twice the poll interval, as `docs/deploying.md` advises for the other reports. With a shorter value, polls can alternate between the reported price and the rule price without any error. This is advice in the docs and the page, not a validation rule.

**What tightening the limits does.** This follows from two owner decisions together (refuse as a whole, never clamp), and the docs must say it:

- A stored weight outside the new limits stops applying at once; that setup is charged at its fallback weight.
- Every later report that still contains such a weight is refused whole. After `max_age`, every setup is back on its fallback weight, with polls succeeding.
- The admin sees both on the Settings page ([C3](#c3-ui)): the count of excluded weights at once, and the refusals as they arrive.

### D2. Per-setup override

- A new nullable column, `setups.weight_override`: finite, at least 0, at most 1e12. It is not bound by the reported minimum and maximum; it is the admin's own value.
- It pins the setup's weight above a reported weight and above the rate rules. It works whether reported weights are on or off. It pins the weight, not the cost: the multiplier still applies.
- It belongs to the setup and survives a rename.
- `POST` and `PUT /api/v1/admin/setups` accept an optional `weight_override`. On `PUT`, an absent field leaves the stored value unchanged, `null` clears it, and a number sets it. On `POST`, absent means none.
- It is part of the setup's audit snapshot, so setting and clearing it are audited.
- `GET /api/v1/admin/rules` keeps counting matches by selector alone. A setup priced by an override or a report still counts for the rule it matches.

### D3. Scope

- `weights:write` is added to `auth.Scopes()` after `holds:release`. No existing account gains it. An admin adds it on the Service accounts page or through the existing admin routes.
- The permissions matrix test covers the new route for every principal, including an account that holds every other scope and not this one (SVC-8).

## E. The first client

One team's service, `lab-connector`, will use this first. What it said it will send, and what the guide must tell it:

| Their plan | Our answer |
| --- | --- |
| A report every 60 seconds with the same `observed_at` as the deployments report. | Fine. The two reports are ordered independently. Send weights first, then deployments. |
| A weight with two decimals, written as `"1.15"`. | The weight is a JSON number, `1.15`. A string is refused. |
| Never above 20. | Their admin sets the maximum to 20. The promise is not what enforces it. |
| `mem-gb`, `disk-gb` and `tier` for display. | These are `attributes`. |
| "For each deployed setup" in one sentence and "every setup" in another. | Both work, with two consequences if only deployed setups are reported. A free setup shows its rate-rule price in the list and in the acquire dialog, and the price changes once it is deployed. And a setup dropped from the report at teardown has its last interval charged at the rate-rule price. To avoid both, report every setup that can be sized, or at least keep reporting a setup until its hold is gone. They need to choose. |
| Hourglass may compute the weight from the raw numbers instead. | Declined (owner decision). |
| Invalid values are refused with an error naming the setup. | Yes for every body that is valid JSON. An encoder that writes a bare `NaN` produces invalid JSON, and that error cannot name a setup. |

Their setup names and attribute keys appear in this section only. Nothing in the product, its tests, its fixtures or its shipped docs may name this client, its attributes or its backend; use neutral names such as `lab-03`, `memory-gb` and `tier=gold`. `tools/check_core_generic.sh` enforces part of that for `internal/` and `pkg/`.

## Compatibility

- **Configuration.** No new config file key. An existing config file and environment start the new version unchanged.
- **Database.** Migration `0008` is additive: the two weights report tables, `usage_samples.weight_source` and `setups.weight_override`. No existing row is rewritten. There is no down migration, and an older binary refuses the newer schema; the way back is the backup taken before the upgrade. `docs/deploying.md` must say this under "Upgrade".
- **Off by default.** With `reported_weights.enabled` false and no override set, every charge and every balance is what the current release computes from the same data.
- **API.** No route, parameter or response field is removed or renamed. Responses gain the fields named in this document. Existing requests keep working without the new fields: `PUT /api/v1/admin/settings` without `reported_weights`, and setup create and update without `weight_override`.
- **Fields whose meaning shifts.** These four are deliberate and must be listed in the upgrade notes. With the feature off and no override, the first and last return what they return today.
  - `Setup.weight` was "resolved weight from the rate rules" and becomes "the weight Hourglass applies"; `weight_source` says where it came from.
  - An active `Hold.weight` and the burn figures become the resolver's weight now, not the last sample's ([A3](#a3-every-other-reader)).
  - Admin Users burn honours a policy's weight.
  - `matched_rule_selector` and a rule's `match_count` describe the first matching rule, which is no longer always the rule that sets the weight.
- **Extensions.** No interface, type or contract changes. `SetupState.Weight` is the base weight, so the `inventory` pressure source and every custom extension compute what they compute today, plus the override. The conformance suites need no change.
- **Service accounts.** Existing accounts and tokens are untouched and cannot call the new route.
- **Hooks.** No new event type and no change to any payload.

## Changes by area

| Area | Change |
| --- | --- |
| `internal/store/migrations` | `0008`: `reported_weights_meta` (single row: `observed_at`, `received_at`, `service_account`), `reported_weights` (`setup_name` primary key, `weight`, `attributes_json`), `usage_samples.weight_source`, `setups.weight_override`. |
| `internal/store` | Queries for the two tables; the sample column; the setup column; the per-entry weight aggregates for the ledger. |
| `internal/reported` | Submit and read the weights report with the shared ordering and skew rules; the last refusal. Value limits come from the caller, as `max_age` does. |
| `internal/weights` (new) | The resolver of [A1](#a1-one-resolver), pure and without database access. |
| `internal/settings` | The `reported_weights` key, its validation and JSON form. |
| `internal/collector` | Resolve through the resolver; base weight in the snapshot, resolved weight in the debit; record the source. |
| `internal/auth` | The `weights:write` scope. |
| `internal/api` | The report route; the state fields; every reader in [A3](#a3-every-other-reader); settings and setup input; ledger fields; the collector view; route table and permissions matrix. |
| `pkg/hourglass` | A comment on `SetupState.Weight`. Nothing else. |
| `api/openapi.yaml`, `ui/src/api/schema.gen.ts` | All of the above, with `x-roles: service` for the new route. |
| `ui/src/pages/setups`, `usage`, `admin-setups`, `admin-settings`, `admin-service-accounts` | [C3](#c3-ui). |
| `e2e/` | The integration scenario's scripted reporter also reports weights. |
| `docs` | See [Documentation deliverables](#documentation-deliverables). |

`tools/check_core_generic.sh` bans some words in every file under `internal/` and `pkg/`, tests and test data included: vendor and provider names and `size=` among them.

## Requirements

Every requirement gets at least one test named after its id that fails when the requirement is broken. If an id is already taken when the work starts, use the next free number and say so in `docs/decisions.md`.

### Resolving and charging

- **WGT-1** With reported weights on and a usable reported weight for a setup, a metered hold of that setup is debited `reported weight × multiplier × elapsed hours`, rounded as today. A weight of 1.15 at multiplier 1 for 5 minutes is 96 milli-tokens.
- **WGT-2** The resolver applies, in order: the setup's override, a usable reported weight, the first matching rate rule, the default weight. A usable reported weight wins over a matching rule of any weight, including 0. A policy's `Decision.Weight` above 0 still replaces the result for that hold and poll.
- **WGT-3** When a setup has no usable reported weight (none sent, the report expired, the setup left out of the latest report, the weight outside the limits, or the feature off), its override, rate rule or default weight applies exactly as without the feature, in every state of the hold including the poll that completes a pending release, and the poll does not fail for that reason. A database error reading the stored report does fail the poll.
- **WGT-4** A changed weight is used by the next poll for the whole interval that poll charges, and no usage sample or ledger row written before it changes.
- **WGT-5** Every usage sample written records its weight and one of the five sources; `rule_selector` is set only for the source `rule`.
- **WGT-6** `SetupState.Weight` in the snapshot is the base weight (override, else rate rule, else default) for every enabled setup, whatever is reported. With the `inventory` pressure source, no weights report changes the pressure or the multiplier.

### The report

- **WGT-7** `PUT /api/v1/integration/weights` replaces the whole stored set atomically, applies the ordering and clock-skew rules of the other reports with an optional `observed_at`, and survives a restart.
- **WGT-8** An entry whose weight is missing, not a JSON number, not finite, below the minimum or above the maximum (both inclusive), whose setup name is missing, not a string, empty or repeated, whose attributes break their limits, or which holds an unknown field, refuses the whole report with 400 `validation`. The message always gives the entry's position, and its setup name when it has a non-empty string name. Nothing is stored and the previous report stays in force.
- **WGT-9** A name that matches no setup is ignored, changes no weight, and is listed in `unknown`.
- **WGT-10** With reported weights off the route answers 409 `not_configured` naming the setting and stores nothing, whether or not the body is valid.
- **WGT-11** The route requires `weights:write`. An account with every other scope gets 403 `forbidden`, and `weights:write` alone allows no other route. The permissions matrix covers the route for every principal.
- **WGT-12** A stored weight that the limits now in force exclude is not applied, from the next poll and the next read, without a new report.
- **WGT-21** The stored report follows the name and the switch: a setup created or renamed to a stored name takes that weight; turning the switch off and on again, or raising `max_age`, makes a report that is within `max_age` apply again; an override stays with its setup through a rename.

### Reading back and display

- **WGT-13** `GET /api/v1/integration/state` returns, for every setup, the applied weight, its source, the reported weight and why it is not applied (the first of `expired`, `out_of_bounds`, `override`), and at the top level the switch, the limits and the report's times. With the feature off, `reported` and `not_applied` are null for every setup and the report's times are null. No rate rule text is returned.
- **WGT-14** For one database state and one time, the weight and source in the Setups list, the admin setup views, an active hold, the burn figures of My usage and Admin Users, and the state route are equal to each other and to the weight a poll at that time records, for setups priced by each of `override`, `reported`, `rule` and `default`. For a hold whose latest sample has the source `policy`, the hold and the burn figures show that sample's weight and `policy`, while the setup views show the resolver's.
- **WGT-15** A debit's ledger entry shows the lowest and highest weight and the sources of the polls it stands for, in the every-poll view, the day view and after storage compaction. The weights are null unless a sample is found for every poll of the entry, and the sources are null if any of those samples has none.
- **WGT-16** Attributes are stored and returned unchanged, are shown only for a usable entry, and no selector, rate rule or filter matches them.
- **WGT-22** `GET /api/v1/admin/collector` returns the last accepted weights report with its entry count and the number of stored weights the limits exclude, and the last refused report with its time, account and message; both are null with the feature off.
- **WGT-23** A released hold shows the weight and source of its last usage sample and does not change when its setup's weight changes afterwards.

### Admin controls

- **WGT-17** The `reported_weights` setting validates as in [D1](#d1-settings), needs all four fields when present, cannot be turned on without a maximum, is audited, and is left unchanged by a `PUT` that does not carry it.
- **WGT-18** A setup's weight override pins its weight above a reported weight and the rate rules, with the feature on or off, and is not bound by the reported limits. It is audited. On update, an absent field leaves it unchanged and `null` clears it.

### Frontend

- **UI-12** The Setups page shows each weight with its source tag and its attributes, and the acquire dialog shows the change warning whenever `reported_weights_enabled` is true.
- **UI-13** My usage shows a hold's weight with its source, and a ledger debit's weight as one value, a range, or nothing when unknown.
- **UI-14** Admin pages: Setup inventory shows the reported weight and why it is not applied, and edits and clears the override; Settings edits the four fields and shows the last report, the excluded count and the last refusal; Service accounts offers `weights:write`.

### Compatibility

- **WGT-19** A database created by `v1.1.0`, with holds and ledger data, upgrades by starting the new binary with the same configuration; with reported weights off, every balance and every per-hold charge is the same as before, and a poll over the same data writes the same debits and passes the same snapshot to the pressure source and the policy.
- **WGT-20** With reported weights off and no override, the responses of the existing routes differ from `v1.1.0` only by the added fields and by the two deliberate changes of [A3](#a3-every-other-reader), and the contract test passes: `api/openapi.yaml` and the handlers agree, and no field, parameter or route of the `v1.1.0` contract is removed.

## Tests

Beyond one test per requirement, the first client's eight acceptance scenarios must each be one named test or end-to-end step:

| Their scenario | Covered by |
| --- | --- |
| A reported weight is charged (1.15 for 5 minutes is 96 milli-tokens) | WGT-1 |
| A weight changes during a hold | WGT-4 |
| Falling back when reports stop or go stale | WGT-3 |
| Invalid values | WGT-8 |
| Every screen agrees with the charge | WGT-14 |
| Reading the applied weight back | WGT-13 |
| The feature switched off behaves like today | WGT-19, WGT-20 |
| The permission check | WGT-11 |

Also:

- A resolver table test: every combination of override, reported entry (absent, usable, expired, out of bounds, at each limit exactly), feature on and off, matching rule (including weight 0) and none.
- A collector test for each fallback case of [A2](#a2-the-collector): a hold metered before its setup is reported, and a pending release completed by a poll after the setup left the weights report. Each asserts the fallback weight and its source on the sample.
- A test that tightening the maximum excludes a stored weight at once, refuses the next report whole, shows both to the admin, and leaves polls succeeding.
- Compatibility fixtures recorded from the code at `v1.1.0`, in the way D78 did for `d96a950`: a database with holds and ledger data, and the responses of the routes this work touches. Keep the existing `d96a950` fixtures and their tests passing unchanged.
- End-to-end, in the integration scenario (`e2e/run.sh integration`):
  - An admin turns reported weights on with a maximum and gives the account the scope through the UI.
  - The scripted reporter sends weights for every setup; the Setups page shows the reported weight and attributes before acquire, and the acquire dialog shows the warning.
  - A hold is charged at the reported weight; the reporter changes the weight and the next debit follows.
  - The user releases; the reporter keeps reporting the weight through teardown, and the last debit is at the reported weight.
  - The reporter stops sending weights while still sending deployments and pressure; after `max_age` the price returns to the rate rule with polls still succeeding.
  - A report over the maximum is refused, changes nothing, and appears on the Settings page.
- `./ci.sh` passes before each milestone is committed.

## Documentation deliverables

1. **`docs/integrating.md`.** A fifth job, "Report weights", with the request, the rules of [B1](#b1-put-apiv1integrationweights) and [B2](#b2-refusals), the new scope, the new state fields, the new error cases, and the checklist lines. Run every example against the real server. The file must stay self-contained. It must say:
   - The weight should describe what the setup reserves, not live utilisation.
   - Send the complete set, from one service, every time.
   - Send weights before deployments.
   - Keep reporting a setup's weight until its `hold` is null in the state; a setup left out is charged at the rate-rule price while it is held, including the last interval of a release.
   - If you cannot size a setup, leave it out; never send 0 in its place.
   - A weight is a number, not a string.
   - An expired weights report does not stop charging, unlike the other two reports: prices go back to the rate rules. Check `weights.expires_at` in the state, and alert on a 400.
   - `not_configured` on this route means an admin has not turned the setting on; it is not a config file change.
2. **`docs/deploying.md`.** The setting and its fields, what tightening the limits does, the override, the `max_age` advice, the upgrade entry for migration `0008`, and the four fields whose meaning shifts.
3. **`docs/extending.md`.** `SetupState.Weight` is the base weight and does not include a reported weight; the order of precedence with `Decision.Weight`; a policy cannot see the reported weight.
4. **`README.md`.** One paragraph where it lists features.
5. **`docs/decisions.md`.** D80 records the owner decisions above. One row per decision taken during the work. A last row lists the edits the source of `docs/SPEC.md` needs, as D79 did: the "What is billed" principle, RULE-1, MET-3, the `usage_samples` and `setups` rows, the two new tables, the settings table, the HTTP API table, the Roles and permissions table, and the requirements above. Do not edit `docs/SPEC.md`.
6. **`docs/ui-notes.md`, `docs/progress.md`.** As the earlier work did.

## Suggested task order

Each milestone is committed on its own, with requirement ids in the message. Run an independent verification task after milestones 3, 5 and 7, by an agent that did not write the code and is given the code and the requirements, not the author's conclusions.

1. Record D80 in `docs/decisions.md`. Record the `v1.1.0` compatibility fixtures before any product code changes.
2. Migration `0008`, store queries, `internal/reported` for the weights report, the `reported_weights` setting (WGT-17).
3. The resolver and the collector (WGT-1 to WGT-6, WGT-12).
4. The scope, the report route and the state fields (WGT-7 to WGT-11, WGT-13, WGT-21); OpenAPI and generated types.
5. Every other reader on the resolver, ledger fields, setup input, the collector view (WGT-14 to WGT-16, WGT-18, WGT-22, WGT-23).
6. UI (UI-12 to UI-14).
7. End-to-end scenario, compatibility tests (WGT-19, WGT-20), documentation.
8. Release: see below.

Two tasks must never edit `api/openapi.yaml` or the generated types at the same time.

## Release

- Cut a work branch from `main` at `753cc50`. `main` stays equal to what customers run until the owner approves the release.
- Follow `docs/deploying.md` ("Publishing the images to a registry") and D67: pushing a git tag that starts with `v` and a digit runs `.github/workflows/release.yml`, which pushes the three product images and moves `latest`. That workflow does not run the tests.
- The current release is `v1.1.0`. The next version number is the owner's to name.
- Before asking to release: `./ci.sh` passes on the commit to be tagged; `e2e/run.sh both` and the integration scenario pass; the upgrade notes in `docs/deploying.md` are written; the job report lists every requirement with its test and every decision recorded.
- Merging to `main`, pushing commits and pushing the tag are outward-facing: the tag publishes images that customers pull as `latest`. Stop and get the owner's go-ahead, with the version number, before any of them. Never rewrite pushed history.

## Open points for the owner

None of these blocks the work; each has a default that the implementation follows unless the owner says otherwise.

| Point | Default |
| --- | --- |
| A policy's `Decision.Weight` still beats the admin's override. | Kept, because it is the existing contract for Go extensions and no bundled policy uses it. Say if the override should win. |
| A Go policy or pressure source cannot see the reported weight. | A consequence of keeping the snapshot unchanged. A field can be added later without breaking anything. |
| The default `max_age` is 10 minutes, as for the other reports. | A weights reporter that is down for longer puts prices back on the rate rules while charging continues. An admin can raise it up to 24 hours. |
| A weight of 0 is accepted when the minimum is 0. | Yes; rate rules allow 0 too. An admin who never wants a free setup sets a minimum above 0. |
| A bare `NaN` in a report is invalid JSON, so that one error cannot name the setup. | Left as it is; the client is told (section E). |
| The last refused report is kept in memory. | It is lost at a restart. Say if it should be stored. |
| Weight detail on ledger entries depends on `usage_samples`, kept 13 months. | If retention of `usage_samples` is shortened, the weight on older ledger entries shows as unknown from then on. |
| Reports are not audited one by one. | The stored report names its account and each charge records its weight and source. Say if a log of weight changes per setup is wanted. |
| The first client has not said whether it reports every setup or only deployed ones. | Both work (section E). The owner passes the choice and its two consequences back to them. |
| The version number of the release, and whether the agent merges and pushes or hands over. | The agent stops before merging or pushing and asks. |
