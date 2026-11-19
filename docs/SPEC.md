# Hourglass: functional specification

Nov 18, 2026 · @Tejas

> *Markdown copy of the source document at <https://claude.ai/artifact/AVPtQZjX1gQvSQBseDGGBc>, exported 2026-11-19 at revision 92. The text is unchanged apart from this note and the placeholder for the one embedded diagram. If this file and the source disagree, the source wins.*

Hourglass is an open-source framework that meters how long each developer holds a shared setup and drains a monthly token balance faster when setups are scarce, so idle setups get released. The core is token management only: what is deployed, how scarcity is measured, when charging starts and what happens on events are all extensions. v1 bundles vSphere extensions as its reference scenario. This draft covers the token model, the dashboard, and everything a development agent needs to build v1.

## Project name

**Hourglass** is the working name: tokens drain while you hold a setup, and the neck narrows when the lab is busy. It is one word, has an obvious logo, and works as a binary name (`hourglass`, `hourglassd`).

| Alternative | Idea behind it |
| --- | --- |
| Tollbooth | Pay to stay on a busy road; congestion pricing |
| Valet | Hand the keys back when you are done |
| Ebb | Balance ebbs with the tide of lab demand |

## Token system

One token buys one weight-1 setup for one hour when the lab is moderately busy. Everything else is a multiple of that.

### What is billed

- **Bill what a setup reserves, not what it consumes.** A held setup blocks a colleague whether it is busy or idle, so the charge depends on which setup you hold and for how long, never on live CPU or memory. This also means billing needs nothing from the infrastructure backend.
- **Setups carry labels; rate rules turn labels into a weight.** A setup is a name, free-form labels such as `size=large` or `site=sjc`, and an opaque details blob. An admin keeps an ordered list of rate rules, each a label selector and a weight; the first rule that matches a setup sets its weight, and unmatched setups get the default weight of 1.
- **The core has no built-in sizes.** Medium, Large and ExtraLarge are just label values in our own deployment (`size=medium` → 1, `size=large` → 2, `size=extralarge` → 4). Another team can price by site, GPU, hardware generation or anything else it labels.
- **Hourglass meters whatever setups an admin registers.** It has no notion of physical or virtual. Our deployment registers only the physical setups.

### Burn rate

Every 5 minutes the collector debits each holder: `weight × multiplier × 5/60` tokens.

The multiplier comes from **lab pressure**: one number from 0% to 100% for the whole lab. The core does not decide how it is measured; a **pressure source** extension reports it. The default `inventory` source needs no backend: weight of held setups divided by weight of all enabled setups. The `vsphere` source reports how full the shared cluster is. Whatever the source, the core averages it over the last 15 minutes so the multiplier does not flap.

| Lab pressure | Multiplier | Meaning |
| --- | --- | --- |
| Below 50% | 0× | Free; nobody is waiting |
| 50% to 75% | 1× | Normal |
| 75% to 90% | 2× | Getting tight |
| 90% and above | 3× | Someone is probably blocked |

Bands and multipliers are admin-configurable. Worked example: a weight-1 setup held overnight for 15 hours costs 0 tokens in a quiet lab, 15 at 1×, and 45 at 3×.

### Lifecycle of a hold

1. **Acquired.** The developer claims a free setup in Hourglass. A `hold.acquiring` event goes to the hooks first, and any hook can veto; then `hold.acquired` fires. Nothing is charged yet.
2. **Metering starts** when the **metering policy** extension says so, and `hold.metering_started` fires. The default policy starts at whichever comes first: the resource provider reports the setup's resource (with vSphere, a resource pool named from the setup), or 2 hours pass since acquiring. Either trigger can be turned off in config, and a custom policy can use any rule.
3. **Deployed.** When the provider first reports the setup, the hold is marked deployed and `hold.deployed` fires. With no provider configured, this state is skipped.
4. **Released.** The developer clicks Release. Release is refused while the provider still reports the setup, so tearing down comes first; an admin can force it. Metering stops, `hold.released` fires, and the setup shows as free.

Hourglass does not deploy anything itself. In our v1 deployment no hook is configured and the developer deploys by hand; automated deployment later is a hook, not a core change (see Extension points).

### Allowance, grants and running out

- **Monthly allowance:** 300 tokens, credited on the 1st. That covers about 200 working hours on a weight-1 setup at 1× with headroom, but not holding it around the clock in a busy month (720 hours).
- **Surplus expires, debt carries over.** Unused tokens are dropped at month end. A negative balance is subtracted from the next allowance: ending at −40 means starting the next month at 260.
- **Grants and resets:** an admin can add tokens to any user with a required reason, or reset a user's balance to the full allowance, which clears carried debt.
- **At zero or below, no new setups.** The developer cannot acquire. Setups they already hold keep metering and push the balance further negative. They are flagged on the Setups page and listed as overdrawn for admins. Hourglass never tears anything down in v1.
- **Two setups at most.** A user can hold 2 setups at once by default; an admin can change the limit.

### Rollout and calibration

- **Shadow mode first.** Run 2 to 4 weeks with metering on and enforcement off. Use the recorded data to set the rate rules, the bands and the allowance before anyone is blocked.
- **The allowance must be tight to matter.** A developer with surplus tokens and no rollover has no reason to release. Set the default so that a typical developer ends the month near zero, and handle exceptions with grants.
- **If the resource provider is unreachable, nobody is charged** for the gap. The gap is recorded and shown to admins.
- **The 1 to 2 hour redeploy is the real competitor.** Tokens only change behaviour when an overnight hold costs more than the developer values that redeploy time. The 3× band is where that happens; faster redeploys would make every band more effective.

## Architecture

Hourglass is a single Go process: it reads the infrastructure through a resource provider plugin, never writes to it, and is the only writer to its SQLite file.

> *[Diagram not reproduced in this Markdown copy: "architecture · one container, two external systems". It shows the single Hourglass container between the resource provider's backend (vCenter) and the OIDC provider. The paragraphs around it describe the same structure.]*

On each poll the collector asks the provider what is deployed, the pressure source how contended the lab is, and the metering policy which holds to charge, then writes samples and debits. The web layer reads the same database, delegates sign-in to the OIDC provider, and keeps one local `admin` account of its own.

The UI is a separate single-page app that talks only to the JSON API. In v1 the Go binary also serves its static files, so there is still one container; the UI can move to its own container later with no backend change (see Frontend).

### Extension points

The core knows nothing about vCenter, about how a setup gets deployed, about how lab pressure is measured, or about when metering should start. Each sits behind a Go interface in a public package, `pkg/hourglass`, registered by name the way `database/sql` drivers are. A team that needs something different writes a package, imports it from its own `main`, and builds its own binary; the core is not forked.

```go
// Setup is what every extension sees of a setup. The core never
// interprets Labels or Details.
type Setup struct {
    Name    string
    Labels  map[string]string
    Details json.RawMessage
}

// ResourceProvider reports what is currently deployed. Optional:
// with none configured, every other extension receives no allocations.
type ResourceProvider interface {
    Name() string
    List(ctx context.Context) ([]Allocation, error)
}

type Allocation struct {
    ResourceName string            // matched to a setup by the name template
    Labels       map[string]string // provider-specific, shown to admins
}

// PressureSource reports how contended the lab is, from 0.0 to 1.0.
type PressureSource interface {
    Name() string
    Pressure(ctx context.Context, s Snapshot) (float64, error)
}

// MeteringPolicy decides, per hold and per poll, whether to charge.
type MeteringPolicy interface {
    Name() string
    Decide(ctx context.Context, h HoldState, s Snapshot) (Decision, error)
}

type Decision struct {
    Meter  bool
    Weight float64 // 0 means use the weight from the rate rules
    Reason string  // shown to the user, e.g. "2 h since acquire"
}

// Hook receives every event. An error returned for a vetoable event
// (hold.acquiring) aborts the action; other errors are logged.
type Hook interface {
    Name() string
    Handle(ctx context.Context, e Event) error
}

type Event struct {
    Type  string // e.g. "hold.acquired"
    Time  time.Time
    User  *User  // nil when not about a user
    Setup *Setup // nil when not about a setup
    Hold  *Hold  // nil when not about a hold
    Data  map[string]any
}

func RegisterProvider(name string, f ProviderFactory)
func RegisterPressureSource(name string, f PressureFactory)
func RegisterMeteringPolicy(name string, f PolicyFactory)
func RegisterHook(name string, f HookFactory)
```

| Event | Fires when | Vetoable |
| --- | --- | --- |
| `hold.acquiring` | A user asks to acquire, after the core checks pass | Yes |
| `hold.acquired` | The hold is created | No |
| `hold.metering_started` | The policy first says to charge | No |
| `hold.deployed` | The provider first reports the setup | No |
| `hold.released` | The hold ends, by the user or an admin | No |
| `balance.low` | A balance first drops below 10% of the allowance | No |
| `balance.overdrawn` | A balance first reaches zero or below | No |
| `user.pending` | A new user signs in for the first time | No |
| `user.approved` | An admin assigns a first role | No |
| `period.closed` | The monthly rollover finishes | No |
| `poll.failed` | A collector run fails | No |

| Ships in v1 | Kind | What it does |
| --- | --- | --- |
| `vsphere` | Provider | Lists resource pools from vCenter with a read-only account |
| `static` | Provider | Reads allocations from a JSON file; used for demos and tests |
| `inventory` | Pressure source | Weight of held setups ÷ weight of enabled setups; the default, needs no backend |
| `vsphere` | Pressure source | Highest of the cluster's CPU, memory and storage allocation ratios |
| `default` | Metering policy | Starts when the provider reports the setup, or a set time after acquire, whichever is first; each trigger can be disabled |
| `webhook` | Hook | POSTs each event as JSON to a URL, so automation can be added without writing Go |

No hook is configured by default. The only defaults are the `inventory` pressure source and the `default` metering policy, neither of which needs a backend.

- The config file selects at most one provider, exactly one pressure source, exactly one metering policy and an ordered list of hooks, and passes each its own opaque config block.
- `Snapshot` carries the enabled setups with their labels and resolved weights, the active holds and the provider's allocations for the current poll, so a pressure source or policy can branch on labels and needs no database access. Types not spelled out above (Snapshot, HoldState, Hold, User, the factories) are plain structs and funcs that mirror the data model.
- Events about a setup carry its labels and `details` JSON, so a deployment hook has the proprietary data it needs.
- A hook may filter by event type in its config; the `webhook` hook takes a list of types and defaults to all.
- A pressure source that fails or returns a value outside 0 to 1 makes the poll fail; nobody is charged for a failed poll.
- Only the vetoable event is delivered inside the user's request; every other event goes through one in-order background queue, so a slow hook never delays a poll. Each call has a 30-second timeout, so a hook that starts a long deployment should enqueue it and return.
- A future auto-deploy is a hook on `hold.acquired`; chat or email notifications are a hook on `balance.low` and `user.pending`. Neither changes the core.

## Roles and permissions

A new user has no role and sees nothing until an admin assigns one, so other teams cannot learn the setups or their usage by signing in. v1 has two roles, `developer` and `admin`, plus the state of having none (**unassigned**). Roles are stored in Hourglass, not in the IdP.

| Capability | Unassigned | Developer | Admin | Manager (future) |
| --- | --- | --- | --- | --- |
| Sign in | Yes | Yes | Yes | Yes |
| See the "not authorized" page only | Yes |  |  |  |
| See own balance, current holds, history | No | Yes | Yes | Yes |
| See all setups, holders and holders' balances | No | Yes | Yes | Yes |
| Acquire and release a setup for self | No | Yes | Yes | Yes |
| See pending access requests; approve or reject | No | No | Yes | No |
| Assign and remove roles | No | No | Yes | No |
| Grant tokens to a user | No | No | Yes | Within own team |
| Force-release another user's setup | No | No | Yes | No |
| Add, edit, delete setups | No | No | Yes | No |
| Edit metering settings | No | No | Yes | No |
| See team usage, move tokens between team members | No | No | No | Yes |

Rules:

- **First OIDC sign-in** creates a user with no role and status `pending`. The user sees one page: "You are not authorized to use Hourglass. Reach out to your admin." Every data route of the API returns 403 and reveals no setup, user or usage data.
- **Pending requests** appear on the admin Users page with name, email and first sign-in time. The admin assigns a role, which approves the request, or rejects it. A rejected user keeps seeing the same page and leaves the pending list; an admin can still find them and assign a role later.
- **The allowance starts with the role.** A user receives the monthly allowance when first given `developer` or `admin`, not at sign-in, and at most once per period.
- **Removing all roles** returns a user to unassigned. It is blocked while the user holds a setup; the admin force-releases first. The user does not reappear as pending.
- **The local `admin` account** is the first user in the system and the only one not from OIDC. It is created at first start with the `admin` role and a password taken from `HOURGLASS_ADMIN_PASSWORD`, or generated and printed once to the log. It must change the password at first sign-in.
- The local `admin` is permanent: it cannot be disabled, deleted, renamed or demoted. It is an operator account with no token balance, and it cannot acquire setups; its UI shows no balance and no Acquire button.
- An admin cannot remove their own `admin` role.
- Roles are a set per user, checked by one middleware, so `manager` can be added later without a schema change.

## Functional requirements

Each requirement has an id the development agent should cite in commits and tests.

### Authentication and sessions

- **AUTH-1** OIDC sign-in uses authorization code flow with PKCE against the configured issuer, handled entirely by the Go service; the UI never sees a token. Keycloak is the test IdP; nothing in the code is Keycloak-specific.
- **AUTH-2** An OIDC user's identity is the `sub` claim; `email` and `name` are stored for display and refreshed on every sign-in.
- **AUTH-3** Sessions are server-side rows referenced by an HttpOnly, Secure, SameSite=Lax cookie, with an 8-hour idle timeout. State-changing requests must carry the session's CSRF token in an `X-CSRF-Token` header.
- **AUTH-4** Every API route requires a session **and at least one role**, except `/healthz`, the `/auth/*` routes, `GET /api/v1/auth/methods` and `GET /api/v1/session`. Admin routes require the `admin` role. Failures return 401 with no session, or 403 with code `no_role` or `forbidden`, and no data. The UI's static files are public and contain no data.
- **AUTH-5** A signed-in user with no role gets an empty role list from `GET /api/v1/session`, and the UI shows only the not-authorized page: the fixed message and a sign-out link. The API enforces the restriction; the UI only reflects it.
- **AUTH-6** The sign-in page offers "Sign in with SSO" and a separate username and password form used only by the local `admin` account.
- **AUTH-7** The local `admin` password is stored as an argon2id hash, must be changed at first sign-in, has a 12-character minimum, and its sign-in is rate-limited to 5 failures per 15 minutes.
- **AUTH-8** The local `admin` account is permanent. No setting, API call or UI action can disable, delete, rename or demote it.

### Setup inventory

- **SET-1** An admin can add, edit, disable and delete setups. A setup has a unique name, a set of labels and a `details` JSON blob. There are no setup kinds or form factors in the core.
- **SET-2** Hourglass stores and returns `details` unmodified and passes it to extensions. It validates only that it is well-formed JSON of at most 256 KB.
- **SET-3** Labels are string key-value pairs, at most 32 per setup. Keys match `[a-z0-9]([a-z0-9._/-]{0,61}[a-z0-9])?`; values are at most 63 characters. The core attaches no meaning to any key.
- **SET-4** The resource name the provider is expected to report is derived through a Go template with `SetupName` and `Labels` available. The default is `{{.SetupName}}`; our deployment uses `{{.SetupName}}_RP`. An admin can override the resource name per setup.
- **SET-5** A setup with an active hold cannot be deleted or disabled. Disabling a free setup removes it from the acquire list and from the `Snapshot`.
- **SET-6** An admin chooses which label keys appear as columns and filters on the Setups page (`display_label_keys`, default none).
- **SET-7** If the database has no setups at first start, setups and rate rules are imported from an optional seed file named in config. The seed is never applied again.

### Rate rules

- **RULE-1** A rate rule is a label selector, a non-negative decimal weight (0 makes matching setups free) and a position. Rules are evaluated top to bottom; the first whose selector matches a setup's labels sets its weight. A setup that matches none gets the default weight (1, admin-editable).
- **RULE-2** Selector syntax is the Kubernetes label-selector subset: `key=value`, `key!=value`, `key in (a,b)`, `key notin (a,b)`, `key` (exists), `!key`, joined by commas meaning AND. It is implemented by a small parser in `internal/rules` with no Kubernetes dependency.
- **RULE-3** An admin can add, edit, reorder and delete rules in the UI. The page shows how many setups each rule currently matches and lists setups that fall through to the default. An invalid selector is rejected on save with the parse error.
- **RULE-4** Rule changes apply from the next poll, are never retroactive, and are audited.
- **RULE-5** The rules engine is generic: `Match(selector, labels)` plus rule tables tagged by kind. v1 has one kind, `rate`. Per-selector hold limits and allowance rules on user labels are later kinds and must not need a schema change.

### Holds

- **HOLD-1** A developer can acquire a free, enabled setup when all of these hold: their balance is above zero (skipped while enforcement is off); they hold fewer than `max_holds_per_user` setups (default 2, admin-configurable); and no `hold.acquiring` hook returns an error. Each setup has at most one active hold.
- **HOLD-2** A user can release their own hold. While the admin setting `release_requires_teardown` is on (the default), release is refused if the provider still reports that setup's resource in the latest successful poll, and the refusal tells them to tear it down first. An admin can force-release any hold, with a reason.
- **HOLD-3** A hold records `acquired_at`, `metering_started_at`, `deployed_at` and `released_at`. Every transition fires the matching event.
- **HOLD-4** A reported allocation whose resource name matches a setup that has no active hold is recorded as an **orphan** and listed for admins. Orphans are charged to nobody. Allocations that match no setup are ignored, since a provider may see many resources that are not Hourglass setups.
- **HOLD-5** Lowering `max_holds_per_user` never releases existing holds; it only blocks new acquisitions.

### Metering

- **MET-1** A collector runs every `poll_interval` (default 5 minutes). Each run builds a `Snapshot`: enabled setups, active holds, and the allocations from the configured `ResourceProvider`, if any. The core never talks to vCenter directly.
- **MET-2** The collector matches allocations to setups by resource name, then asks the `PressureSource` for lab pressure and the `MeteringPolicy` for a decision on every active hold.
- **MET-3** For each hold the policy says to meter, the debit is `weight × multiplier × elapsed hours`, where weight is the policy's override or the weight from the first matching rate rule. One usage sample and one ledger debit per metered hold are written in a single transaction with the pressure sample.
- **MET-4** A debit covers the actual time since the previous successful poll, capped at twice the poll interval. Longer gaps are not charged and are recorded as outages. A failed provider, pressure source or policy call fails the whole poll.
- **MET-5** Debits are idempotent per hold and poll timestamp, so a restart never double-charges.
- **MET-6** With `enforcement` off (shadow mode), debits are still written and balances still fall, but the balance check in HOLD-1 never blocks.
- **MET-7** The `default` policy meters a hold once the provider reports its resource (`start_on_allocation`, default on) or `start_after` has elapsed since acquire (default 2 hours), whichever is first, and keeps metering until release. Setting `start_after` to 0 meters from the moment of acquire.

### Ledger and balances

- **LED-1** The ledger is append-only. Entry types: `allowance`, `grant`, `debit`, `expiry`, `carryover`, `reset`.
- **LED-2** A user's balance is the sum of their ledger entries for the current period. It is cached on the user row and can be rebuilt from the ledger.
- **LED-3** On the 1st of each month at 00:00 in the configured timezone, a job closes the old period and opens the new one for every user who has a role. A positive closing balance is zeroed by an `expiry` entry in the old period. A negative closing balance is copied into the new period as a `carryover` entry. Then the `allowance` entry is written, so the opening balance is the allowance minus any debt.
- **LED-4** A grant requires an amount, a target user and a reason, and records the granting admin. Negative grants are allowed for corrections.
- **LED-5** An admin can reset a user's balance: a `reset` entry brings the current balance to exactly the default allowance, with a required reason. This is how carried debt is cleared.
- **LED-6** Debt has no floor. A user who keeps holding a setup at zero keeps going negative until they release or an admin intervenes.

### Administration

- **ADM-1** An admin can list users, see each balance and month-to-date burn, change roles, grant tokens and reset a balance.
- **ADM-2** An admin can edit metering settings: resource name template, rate rules, display label keys, pressure bands, default allowance, maximum holds per user, teardown before release, poll interval, enforcement on or off. Changes apply from the next poll and are not retroactive.
- **ADM-3** Every admin action is written to an audit log with actor, time, action and before and after values.
- **ADM-4** An admin can see collector health: active provider, pressure source and metering policy, last successful poll, last error, outage history and orphans.
- **ADM-5** An admin can see pending access requests, oldest first, and approve one by assigning a role or reject it. The navigation shows the pending count. There are no notifications in v1; admins check the page.

### Extensibility

- **EXT-1** `ResourceProvider`, `PressureSource`, `MeteringPolicy` and `Hook` are defined in the public package `pkg/hourglass` with a registry. Core packages import only the interfaces, never a concrete implementation.
- **EXT-2** The config file names at most one provider, one pressure source, one metering policy and zero or more hooks, each with an opaque config block handed to its factory. An unknown name fails startup with the list of registered names.
- **EXT-3** v1 ships the `vsphere` and `static` providers, the `inventory` and `vsphere` pressure sources, the `default` metering policy, and the `webhook` hook, each in its own package that the default `main` imports.
- **EXT-4** With no provider configured, Hourglass is fully functional using `inventory` pressure and time-based metering.
- **EXT-5** A third party can build a binary with its own extensions by writing a `main` that imports the core and its packages. The repository includes a worked example and a conformance test suite for each interface.
- **EXT-6** Every event in the event table is delivered to every configured hook, in config order, with a 30-second timeout per call. `hold.acquiring` is delivered inside the user's request, and an error or timeout aborts the acquire and shows the hook's message. Every other event goes through one in-order background queue, so a slow hook never delays a poll or a request; its errors are logged, recorded and shown to admins, and never block.
- **EXT-7** Events are delivered at most once and are not retried in v1. Each delivery is recorded with its outcome. The queue is in memory, so events still pending at shutdown are lost.

### Frontend

- **UI-1** The UI is a single-page app in `//ui`, written in React and TypeScript and built with Vite. It holds no business logic: every number it shows comes from the API.
- **UI-2** The UI's only interface to the backend is the HTTP API described in `api/openapi.yaml`. The Go service contains no HTML templates and renders no pages. The UI's API client types are generated from that file, and a contract test fails CI when the file and the Go handlers disagree.
- **UI-3** One UI build runs in two modes, chosen by config. **Bundled** (the v1 default): the Go binary serves the UI's static files at `/`, with a fallback to `index.html` for client-side routes. **Split**: `ui.serve: false`, and a separate `hourglass-ui` image serves the static files and reverse-proxies `/api`, `/auth` and `/healthz` to the API container.
- **UI-4** In both modes the browser sees one origin, so the session cookie and CSRF token work unchanged. For UI development from another origin, `cors.allowed_origins` enables credentialed CORS for the listed origins only; it is empty by default.
- **UI-5** The UI decides what to render from `GET /api/v1/session` (identity, roles, status, CSRF token). Hiding data from a user is always enforced by the API, never by the UI alone.
- **UI-6** The UI polls the API for live numbers every 60 seconds. There are no websockets in v1.
- **UI-7** Replacing the UI with one written in another framework must need no backend change. Nothing in the Go code may depend on React, on the UI's file layout beyond one static directory, or on its routes.

## Dashboard pages

Two public pages handle sign-in and unapproved users; six pages behind a role share one top bar showing the signed-in user, their balance, and a lab pressure pill with the current multiplier. Admin pages appear in the navigation only for admins. Wireframes for all but the audit log are on the [Hourglass dashboard wireframes](https://claude.ai/artifact/PWCrdA3nDinVkmuAsGdK74) canvas.

| Page | Path | Who | Shows | Actions |
| --- | --- | --- | --- | --- |
| Sign in | `/login` | Anyone | Product name; SSO button; local admin form | Sign in with SSO; sign in as local `admin` |
| Not authorized | `/unauthorized` | Unassigned | "You are not authorized to use Hourglass. Reach out to your admin." and the signed-in email | Sign out |
| My usage | `/` | Developer, admin | Balance, current burn per hour, projected run-out date, lab pressure; current holds; tokens per day this month; ledger history | Release a hold; pick a past month |
| Setups | `/setups` | Developer, admin | Every setup: name, the chosen label columns, weight, state, holder, holder's balance, held for, current cost per hour | Acquire, release own; filter by state and by label; open details JSON |
| Users | `/admin/users` | Admin | Pending access requests; every user: roles, balance, month-to-date burn, active holds; overdrawn users first | Approve or reject a request; change roles; grant tokens (amount + reason); reset a balance |
| Setup inventory | `/admin/setups` | Admin | Setups with labels, weight, resource name, enabled flag, last seen by the provider; orphans | Add, edit (labels and JSON editor), disable, delete; force-release |
| Settings | `/admin/settings` | Admin | Metering settings; collector health; active provider, pressure source, metering policy, hooks and OIDC issuer read-only | Edit template, rate rules, display label keys, bands, allowance, hold limit, enforcement |
| Audit log | `/admin/audit` | Admin | Admin actions, newest first | Filter by actor and action |

Behaviour worth pinning down:

- **Acquire** asks for confirmation and shows the cost per hour at the current multiplier and at the highest band.
- **Release** is refused while the provider still reports the setup: "This setup is still deployed. Tear it down, then release."
- **Projected run-out** is balance divided by the average hourly burn over the last 24 hours; it shows "not this month" when the projection passes month end.
- **Holder's balance** on the Setups page is visible to every user with a role by design, so peers can see who is holding a setup they cannot afford.
- The pages are routes of a single-page app (UI-1); paths in the table are client-side routes, and live numbers refresh every 60 seconds.

## Data model

One SQLite file in WAL mode holds everything. Timestamps are UTC, stored as ISO 8601 text; token amounts are integers in milli-tokens to avoid float drift, and each debit is rounded to the nearest milli-token.

| Table | Key columns | Notes |
| --- | --- | --- |
| `users` | `id`, `auth_source` (`oidc` or `local`), `oidc_sub` (unique, null for local), `username`, `email`, `name`, `status` (`pending`, `active`, `rejected`), `labels_json`, `password_hash`, `must_change_password`, `balance_mt`, `created_at`, `last_login_at` | `balance_mt` is a cache of the ledger; `labels_json` is empty in v1 and reserved for allowance rules; password columns are used only by the local `admin` row |
| `user_roles` | `user_id`, `role` | Composite key; roles are `developer`, `admin`; no rows means unassigned |
| `setups` | `id`, `name` (unique), `resource_name_override`, `details_json`, `enabled` | `details_json` is opaque |
| `setup_labels` | `setup_id`, `key`, `value` | Composite key on `setup_id` + `key`; indexed on `key` + `value` |
| `rules` | `id`, `kind`, `position`, `selector`, `value`, `updated_by`, `updated_at` | `kind` is `rate` in v1 and `value` is the weight (RULE-5) |
| `holds` | `id`, `setup_id`, `user_id`, `acquired_at`, `metering_started_at`, `deployed_at`, `released_at`, `released_by`, `release_reason` | Partial unique index on `setup_id` where `released_at` is null |
| `usage_samples` | `id`, `hold_id`, `polled_at`, `weight`, `rule_selector`, `pressure`, `multiplier`, `debit_mt`, `reason` | Unique on `hold_id` + `polled_at` (MET-5); `rule_selector` is null when the policy overrode the weight or the default applied |
| `ledger` | `id`, `user_id`, `period` (YYYY-MM), `type`, `amount_mt`, `hold_id`, `actor_id`, `reason`, `created_at` | Append-only; no updates or deletes |
| `pressure_samples` | `polled_at`, `source`, `raw`, `smoothed`, `multiplier` | One row per poll, for the pressure chart |
| `orphans` | `resource_name`, `first_seen_at`, `last_seen_at`, `labels_json` | HOLD-4 |
| `event_deliveries` | `id`, `event_type`, `hook`, `payload_json`, `ok`, `error`, `started_at`, `finished_at` | EXT-7; pruned after 90 days |
| `settings` | `key`, `value_json`, `updated_by`, `updated_at` | Admin-editable metering settings, including `display_label_keys` and the default weight |
| `audit_log` | `id`, `actor_id`, `action`, `target`, `before_json`, `after_json`, `created_at` | ADM-3 |
| `sessions` | `id`, `user_id`, `created_at`, `last_seen_at`, `expires_at` | AUTH-3 |
| `collector_runs` | `id`, `started_at`, `finished_at`, `ok`, `error` | ADM-4 |

Schema changes ship as numbered SQL migrations embedded in the binary and applied at startup. `usage_samples` older than 13 months are pruned; the ledger is kept.

## Configuration

Secrets and connection details live in a config file or environment variables; anything an admin tunes lives in the database and is edited in the UI.

| Setting | Where | Example | Notes |
| --- | --- | --- | --- |
| Provider name and its config block | File / env | `provider: {type: vsphere, config: {...}}` | Optional; the block is opaque to the core |
| `vsphere` provider: URL, username, password, TLS verify, datacenter and cluster scope | File / env | `HOURGLASS_VSPHERE_PASSWORD` | Read-only vCenter role is sufficient; shared with the `vsphere` pressure source |
| Pressure source and its config block | File | `pressure: {type: inventory}` | Exactly one |
| Metering policy and its config block | File | `metering: {type: default, config: {start_after: 2h, start_on_allocation: true}}` | Exactly one |
| Hooks, in order, each with a config block | File | `hooks: []` | `webhook` takes a URL and a shared secret |
| OIDC issuer URL, client id, client secret, redirect URL | File / env | `HOURGLASS_OIDC_CLIENT_SECRET` | Discovery via `/.well-known/openid-configuration` |
| Local admin initial password | File / env | `HOURGLASS_ADMIN_PASSWORD` | Read only when the account is first created |
| Listen address, database path, session key, timezone | File / env | `:8080`, `/data/hourglass.db` |  |
| Resource name template | Admin UI | `{{.SetupName}}` | Go template; `SetupName` and Labels are available |
| Setups: name, labels, JSON details | Admin UI |  | SET-1 |
| Rate rules and default weight | Admin UI | size=large → 2 |  |
| Pressure bands and multipliers | Admin UI | 50/75/90% → 0/1/2/3× | Ordered list of thresholds |
| Default monthly allowance | Admin UI | 300 | Applies from the next reset |
| Maximum holds per user | Admin UI | 2 | HOLD-1 |
| Poll interval | Admin UI | 5 min | Minimum 1 minute |
| Enforcement | Admin UI | Off | Off is shadow mode |
| Serve the UI from the Go binary (ui.serve) | File / env | true | False in split mode, where the hourglass-ui container serves it (UI-3) |
| Allowed CORS origins (cors.allowed\_origins) | File / env | empty | Only for UI development from another origin (UI-4) |
| Teardown before release (release\_requires\_teardown) | Admin UI | On | HOLD-2; has no effect without a provider |
| Metrics listen address (metrics.listen) | File / env | empty (off) | Never the public port |

Environment variables override the file. Secrets are never returned by the API or written to logs.

## HTTP API

The JSON API is the only contract between the UI and the backend, and is specified in `api/openapi.yaml`. Data routes live under `/api/v1` and sign-in routes under `/auth`; both are authenticated by the session cookie, and state-changing requests require the CSRF token.

| Method and path | Who | Purpose |
| --- | --- | --- |
| `GET /auth/oidc/start`, `GET /auth/callback` | Public | OIDC flow; ends with a redirect back to the UI |
| `POST /auth/local` | Public | Local `admin` sign-in (AUTH-6, AUTH-7) |
| `POST /auth/local/password` | Local admin | Change password |
| `POST /auth/logout` | Any session | End the session |
| `GET /healthz` | Public | Liveness; reports database and last poll age |
| `GET /api/v1/auth/methods` | Public | Which sign-in methods the sign-in page should show; nothing else |
| `GET /api/v1/session` | Any session | Identity, roles, status, CSRF token; the only data route open to an unassigned user (AUTH-5) |
| `GET /api/v1/me` | Any role | Balance, burn rate, projection, hold count and limit |
| `GET /api/v1/me/holds` | Any role | Current and past holds |
| `GET /api/v1/me/ledger?period=YYYY-MM` | Any role | Ledger entries |
| `GET /api/v1/me/usage?period=YYYY-MM` | Any role | Tokens per day |
| `GET /api/v1/pressure` | Any role | Current pressure, multiplier, bands, 7-day history |
| `GET /api/v1/setups?selector=` | Any role | Setups with labels, weight, state, holder, holder's balance; also returns the display label keys; admins additionally get disabled setups, resource names and last-seen times |
| `POST /api/v1/setups/{id}/acquire` | Any role | HOLD-1 |
| `POST /api/v1/holds/{id}/release` | Owner or admin | HOLD-2 |
| `GET /api/v1/admin/users?status=pending` | Admin | ADM-1, ADM-5 |
| `PUT /api/v1/admin/users/{id}/roles` | Admin | Replace the role set; a non-empty set approves a pending user |
| `POST /api/v1/admin/users/{id}/reject` | Admin | ADM-5 |
| `POST /api/v1/admin/users/{id}/grants` | Admin | LED-4 |
| `POST /api/v1/admin/users/{id}/reset` | Admin | LED-5 |
| `POST`, `PUT`, `DELETE /api/v1/admin/setups[/{id}]` | Admin | SET-1 |
| `GET`, `PUT /api/v1/admin/rules` | Admin | RULE-3; `PUT` replaces the ordered list and returns match counts |
| `GET`, `PUT /api/v1/admin/settings` | Admin | ADM-2 |
| `GET /api/v1/admin/collector` | Admin | ADM-4 |
| `GET /api/v1/admin/audit` | Admin | ADM-3 |

Any other path is not an API route: in bundled mode the Go binary answers it with the UI's `index.html`, and in split mode the UI container does.

Errors use one shape: `{"error": {"code": "insufficient_tokens", "message": "..."}}` with a matching HTTP status.

## Build, packaging and test environment

By default Hourglass ships as one container image holding the Go binary and the built UI, with SQLite on a mounted volume. Split mode uses two images built from the same sources.

- **Backend:** Go. Suggested libraries: `coreos/go-oidc` with `x/oauth2` for sign-in, `x/crypto/argon2` for the local admin password, and a pure-Go SQLite driver (`modernc.org/sqlite`) so Bazel builds need no cgo toolchain. `govmomi` is imported only by the `vsphere` package.
- **Frontend:** React, TypeScript and Vite in `//ui`, with an API client generated from `api/openapi.yaml`. No server-side rendering and no Go templates.
- **Build:** Bazel with bzlmod; `rules_go` and Gazelle for Go, `rules_js` for the UI with a pinned Node toolchain and lockfile. `bazel build //...` and `bazel test //...` must pass from a clean checkout.
- **Building without the UI:** the embedded UI sits behind a Go build tag, so plain `go build ./...` produces an API-only binary. A third party can import the module, or run the API alone, without Node or Bazel.
- **Images:** built by Bazel with `rules_oci`, non-root. `hourglass` is the Go binary with the UI embedded (the v1 default). `hourglass-api` is the same binary without the UI, and `hourglass-ui` is a static file server with the reverse proxy; together they are split mode. A `Dockerfile` is not the source of truth.
- **Repository files:** `LICENSE`, `README.md` with a quick start, `docs/extending.md` covering the four interfaces and the event list, `docs/deploying.md` covering bundled and split modes, and `CONTRIBUTING.md`.

Suggested layout:

```
//api/openapi.yaml     the UI-to-backend contract
//ui                   React + TypeScript single-page app (own package.json)
//cmd/hourglassd       default main: imports core + bundled extensions
//pkg/hourglass        public API: the four interfaces, Event, registry, Run()
//pkg/hourglasstest    fakes and conformance suites for extension authors
//providers/vsphere    vCenter provider and vsphere pressure source (govmomi)
//providers/static     file-backed provider for demos and tests
//pressure/inventory   default pressure source
//policies/default     default metering policy
//hooks/webhook        POSTs events to a URL
//internal/auth        OIDC, local admin, sessions, CSRF, role middleware
//internal/api         JSON handlers only; no templates
//internal/uiserve     optional static serving of the built UI (build tag)
//internal/rules       selector parser, Match(), ordered rule evaluation
//internal/collector   polling loop, snapshot, allocation-to-setup matching
//internal/metering    multiplier bands, smoothing, debit (pure functions)
//internal/ledger      ledger writes, balance, monthly rollover with carryover
//internal/events      event queue, dispatch to hooks, delivery records
//internal/store       SQLite access, migrations
//examples/custom      a third-party main with its own extensions
//deploy               compose files (bundled, split), Keycloak realm, sample config, seed file
```

### v1 reference scenario

v1 is built and accepted against our own case: setups as resource pools on one shared vCenter. The sample config and seed file in `//deploy` are exactly this.

| Piece | Choice in the reference scenario |
| --- | --- |
| Provider | `vsphere`, resource name template `{{.SetupName}}_RP` |
| Pressure source | `vsphere` |
| Metering policy | `default` with `start_on_allocation: true`, `start_after: 2h` |
| Hooks | `none` |
| Setup labels | `size` with values `medium`, `large`, `extralarge` |
| Rate rules | `size=extralarge` → 4, `size=large` → 2, default 1 |
| Display label keys | `size` |
| Limits | Allowance 300, 2 holds per user, bands 50/75/90% |

### Testing without a vCenter

No real vCenter is available on the development machine, so every vSphere code path is built and tested against mocks. Nothing in `bazel test //...` or the local stack may need network access to a real one.

1. **Core against fakes.** `pkg/hourglasstest` provides a fake provider, pressure source, policy, hook and clock. Unit tests cover selector parsing and rule order, band selection and smoothing, debits, the hold limit, month rollover with carryover, and reset.
2. **vSphere logic against a hand-written fake.** The `vsphere` package talks to vCenter through one small internal interface (list resource pools, read cluster capacity and allocation). Pool matching, nested pools and the pressure formula are unit-tested with a fake of that interface.
3. **vSphere wiring against the govmomi simulator.** Integration tests start the simulator in-process, create resource pools named like the seeded setups, and assert the provider lists them and the pressure source returns a value in range. Where the simulator does not model a value, the test asserts only what it does model and layer 2 carries the logic.
4. **End to end on the local stack.** `docker compose up` starts Hourglass, Keycloak with an imported realm (three users, one confidential client) and the simulator as a container. A scripted test drives the real UI in a headless browser through the acceptance path below, once in bundled mode and once in split mode.
5. **Permissions.** HTTP tests cover every cell of the permissions table, including that an unassigned user gets no data from any route.

Acceptance path: a new user signs in and sees the not-authorized page; the local `admin` approves them; they acquire a `size=large` setup; a resource pool is created in the simulator and metering starts at weight 2; pressure is driven across a band and the debit rate changes; a third acquire is refused by the hold limit; release is refused while the pool exists and succeeds after it is destroyed; the month is rolled over with a negative balance and the debt carries.

Mocks cannot confirm the following, so they stay on a checklist for the first run against a real vCenter:

- [ ] The read-only vCenter role has every privilege the provider needs.
- [ ] Real resource pool layout (folders, nesting, naming) matches the template.
- [ ] The pressure source's cluster numbers agree with what vCenter shows.
- [ ] TLS and certificate settings work against the real endpoint.
- [ ] A poll finishes well inside the poll interval at real inventory size.

`hourglassd check-provider` supports that run: it connects with the configured provider and pressure source, prints matched setups, orphans and the current pressure, and writes nothing.

## Non-functional requirements and scope

- **Scale:** up to 200 users, 100 setups and 5-minute polling; any API call answers in under 500 ms on that data.
- **Single instance:** one process owns the database. No high availability in v1; a restart loses at most one poll, which is not charged.
- **Backup:** the database is one file; document an online backup with `VACUUM INTO` on a schedule.
- **Security:** providers only read from the infrastructure; an unassigned user can read nothing; secrets come only from config; all admin actions are audited; cookies and CSRF as in AUTH-3.
- **Observability:** structured JSON logs, and Prometheus metrics (poll duration, poll failures, lab pressure, tokens debited) on a separate listen address, `metrics.listen`, which is off by default. Metrics are never served on the public port, because they would reveal usage to users without a role.
- **Time:** all arithmetic in UTC; the month boundary uses the configured timezone.

Out of scope for v1:

- Deploying, powering off or deleting VMs. The core observes through a provider and never changes the infrastructure; automation belongs in a hook.
- The manager role and team budgets, user labels with allowance rules, per-selector hold limits, and a configurable period length. The rules engine and data model leave room for all four.
- Notifications by email or chat, queues and reservations for busy setups, and token transfers between users.

## License

Hourglass is released under the **MIT License**: anyone may use, copy, modify, fork and redistribute it free of charge, and the software is provided "as is" with no warranty and no liability for the authors.

- The repository root carries the unmodified MIT text in `LICENSE`, with the copyright line `Copyright (c) 2026 Tejas Panse`.
- Every source file starts with `// SPDX-License-Identifier: MIT`.
- CI runs a license check on dependencies and fails on anything that is not permissive (MIT, BSD, Apache-2.0, ISC).
- No proprietary setup data lives in the repository; setup `details`, credentials and realm files for real environments stay in deployment config.

Apache-2.0 is the main alternative: equally free to use and fork, with an explicit patent grant, at the cost of a longer text and a NOTICE file. This is not legal advice.

## Open questions

The earlier questions are settled and folded into the spec. Four smaller ones remain; the draft assumes the answer in brackets.

- [ ] Is React with TypeScript the right choice for the v1 UI, or do you prefer Angular? Only `//ui` changes either way. \[Assumed: React.\]
- [ ] The role is named `developer`, as you specified. It is the one deployment-flavoured word left in the core; would `user` suit a generic framework better? \[Assumed: keep `developer`.\]
- [ ] How should the `vsphere` pressure source measure fullness: the highest of the cluster's CPU, memory and storage allocation ratios, or one of them only? And is it one cluster or several? \[Assumed: highest of the three, across the clusters named in config.\]
- [ ] Are weights of 1, 2 and 4 close to the real size ratio of our Medium, Large and ExtraLarge setups? \[Assumed: yes until shadow mode says otherwise.\]
- [ ] Can a rejected user be approved later? \[Assumed: yes; they leave the pending list but an admin can still assign a role.\]
