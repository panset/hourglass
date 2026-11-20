# Design: integration API (push model)

Status: approved direction, not implemented. Written 2026-11-20 against `main` at `fd948a5`, which includes D40 (resource-name template and override removed) and D41.

This document is the brief for the agent that implements the work. It says what to build and why, and names the decisions the owner already made. Where it leaves a detail open, choose the simplest option that satisfies the numbered requirements in [Requirements](#requirements) and record the choice in `docs/decisions.md`.

## Goal

A customer runs the stock Hourglass container, unmodified, and writes no Go. Their own small service watches their infrastructure and talks to Hourglass over HTTP:

- It **reports** which setups are deployed and how contended the lab is.
- It **reads** setups and holds, to know what to look for and to recover from missed events.
- It **receives** webhooks when a user acquires or asks to release a setup, and deploys or tears down in response.
- It **releases** a hold when its own automation has torn the setup down.

Hourglass stays the system of record for setups, holds and tokens. It still never touches the customer's infrastructure.

## Owner decisions

| Decision | Source |
| --- | --- |
| Push model: the customer's service calls Hourglass. The pull model (Hourglass calls their service) is not being built. | Owner, 2026-11-20 |
| The service authenticates as a **service account** with its own restricted permissions. It never holds the `admin` or `developer` role. | Owner, 2026-11-20 |
| The service may release holds. | Owner, 2026-11-20 |
| A new `hold.releasing` event tells the service that a user asked to release a setup that is still deployed. | Owner, 2026-11-20 |
| Setups are identified by their own name everywhere. There is no resource-name mapping in the core. | D40 |

## Fit with D40

D40 removed the resource-name template and the per-setup override. An allocation now belongs to the setup whose name equals its `ResourceName`, and mapping backend names to setup names is the provider's job. This design relies on that and adds nothing to it:

- Reports name setups by their Hourglass name. The `reported` provider returns that name unchanged as `ResourceName`, so it has no naming config.
- A customer whose backend objects are named differently (a pool `lab-03_RP` for the setup `lab-03`) does the mapping in their own service before reporting. Hourglass never sees the backend name unless they put it in `labels`.
- Renaming a held setup is refused (D40 follow-up, `internal/api/handlers_admin.go`), so a name is stable for the life of a hold. After a free setup is renamed, the old name shows up in the report response's `unknown` list until the service re-reads the state.
- Do not reintroduce a resource name, template or override anywhere: not in the stored reports, the state response, the OpenAPI file or the UI. `tools/check_core_generic.sh` rule 4 enforces part of this.

## Non-goals

- No per-setup "deployed" or "deleted" calls. The service reports the full deployed set each time (see [Why full-set reports](#why-full-set-reports)).
- No out-of-process metering policy. The bundled `default` policy already starts charging when a setup is reported deployed.
- No webhook retries or durable event queue. Delivery stays at most once (EXT-7); the state read covers missed events.
- No API to manage setups, rules, settings, users or tokens from a service account. Those stay admin-only.
- No configuration of webhooks through the API. Hooks stay in the config file.
- No change to the Go extension interfaces in `pkg/hourglass/types.go`.

## How the pieces fit

```
customer service                      Hourglass                         user
----------------                      ---------                         ----
                                      <-- click Acquire ----------------
  <-- webhook hold.acquired ----------
  deploy the setup
  PUT /integration/deployments  ----> stored
  PUT /integration/pressure     ----> stored
                                      poll: reads stored reports,
                                      marks hold deployed, starts metering
                                      <-- click Release (still deployed)
                                      hold marked "releasing"
  <-- webhook hold.releasing ---------
  tear the setup down
  PUT /integration/deployments  ----> stored (setup now absent)
                                      poll: setup gone, release completes
  <-- webhook hold.released ----------
```

The collector keeps its 5-minute poll and all of its current semantics. Two new bundled extensions, a provider and a pressure source both named `reported`, answer the collector from the last stored report instead of calling a backend.

## Service accounts

A service account is a separate kind of principal. It is not a row in `users`, so by construction it has no session, no role, no balance and cannot hold a setup.

### Rules

- An admin creates a service account with a unique name, an optional description and a set of scopes.
- An admin issues one or more tokens for the account. Several live tokens allow rotation without downtime.
- A token is shown exactly once, in the response that creates it. Hourglass stores only a SHA-256 hash. The token carries at least 256 bits of randomness, so a slow password hash is not needed.
- Token format: `hgsa_<token id>_<secret>`. The id selects the row; the secret is compared in constant time.
- A token may carry an optional `expires_at`. Hourglass records `last_used_at` at most once a minute.
- The service sends `Authorization: Bearer <token>`.
- A service token is accepted **only** on routes under `/api/v1/integration/`. On every other route it is ignored and the request is treated as anonymous.
- A session cookie is **not** accepted on `/api/v1/integration/` routes. They need no CSRF token because they never read a cookie.
- Disabling the account or revoking a token takes effect on the next request.
- Failed bearer attempts are rate-limited per client address (reuse the limiter in `internal/auth/ratelimit.go`) and answer 429.
- Tokens never appear in logs, audit entries, error messages or any API response after creation.

### Scopes

| Scope | Allows |
| --- | --- |
| `state:read` | `GET /api/v1/integration/state` |
| `pressure:write` | `PUT /api/v1/integration/pressure` |
| `deployments:write` | `PUT /api/v1/integration/deployments` |
| `holds:release` | `POST /api/v1/integration/holds/{id}/release` |

An account has any subset. A request without the needed scope gets 403 `forbidden`. No scope grants anything outside this table, and no scope can be added that reaches an admin or user route.

### Admin routes for service accounts

All require the `admin` role, the CSRF token, and write an audit entry (ADM-3).

| Method and path | Purpose |
| --- | --- |
| `GET /api/v1/admin/service-accounts` | List accounts with scopes, enabled flag, and each token's id, created, expiry and last-used times |
| `POST /api/v1/admin/service-accounts` | Create: `{name, description, scopes}` |
| `PUT /api/v1/admin/service-accounts/{id}` | Change `description`, `scopes`, `enabled` |
| `DELETE /api/v1/admin/service-accounts/{id}` | Delete the account and its tokens |
| `POST /api/v1/admin/service-accounts/{id}/tokens` | Issue a token: `{expires_at?}`. The response holds the token, once |
| `DELETE /api/v1/admin/service-accounts/{id}/tokens/{tokenId}` | Revoke one token |

The admin UI gets a Service accounts page with the same operations. The token is displayed once with a copy button and a warning that it cannot be shown again.

## Integration routes

All four live under `/api/v1/integration/`, take and return JSON, and use the existing error shape `{"error": {"code": "...", "message": "..."}}`.

### `PUT /api/v1/integration/pressure`

Scope `pressure:write`.

```json
{"value": 0.72, "observed_at": "2026-11-20T10:15:00Z"}
```

- `value` is required, finite, from 0 to 1 inclusive. It is the raw pressure. Hourglass still smooths it over 15 minutes and applies the admin's bands.
- `observed_at` is optional and defaults to the time of receipt.
- Response 200: `{"value": 0.72, "observed_at": "...", "received_at": "...", "expires_at": "..."}`. `expires_at` is `observed_at` plus the configured `max_age`.

### `PUT /api/v1/integration/deployments`

Scope `deployments:write`.

```json
{
  "observed_at": "2026-11-20T10:15:00Z",
  "deployments": [
    {"setup": "lab-03", "labels": {"cluster": "c1"}},
    {"setup": "lab-07"}
  ]
}
```

- The list is the **complete** set of setups deployed right now. A setup that is absent is not deployed. An empty list means nothing is deployed.
- `setup` is the setup's name in Hourglass. `labels` is optional, free-form, and shown to admins exactly like a provider's allocation labels.
- A name that matches no setup is accepted and ignored, as allocations that match no setup are today (HOLD-4). The response lists them so the service can notice a typo.
- A duplicate name in one request is a 400 `validation`.
- Limits: 10,000 entries, 32 labels per entry, 1 MB body.
- Response 200: `{"observed_at": "...", "received_at": "...", "expires_at": "...", "accepted": 2, "unknown": []}`.

### Rules shared by both report routes

- **Ordering.** A report whose `observed_at` is older than the stored one is refused with 409 `stale_report` and changes nothing. An equal `observed_at` replaces the stored report, so a retry is harmless.
- **Clock skew.** An `observed_at` more than 2 minutes in the future is a 400 `validation`.
- **Not configured.** If the running config does not select the `reported` provider (for deployments) or the `reported` pressure source (for pressure), the route answers 409 `not_configured` with a message naming the config key. A report must never be silently dropped.
- **Effect.** A report takes effect at the next poll. It never triggers a poll, a debit or an event by itself.
- **Persistence.** Reports are stored in the database and survive a restart, still subject to `max_age`.

### `GET /api/v1/integration/state`

Scope `state:read`. One call returns everything the service needs to act and to reconcile.

```json
{
  "server_time": "2026-11-20T10:16:02Z",
  "poll": {"interval_seconds": 300, "last_ok_at": "2026-11-20T10:15:00Z"},
  "pressure": {
    "source": "reported",
    "reported": {"value": 0.72, "observed_at": "...", "expires_at": "..."},
    "smoothed": 0.68, "multiplier": 1
  },
  "deployments": {"provider": "reported", "observed_at": "...", "expires_at": "..."},
  "setups": [
    {
      "name": "lab-03",
      "labels": {"size": "large"},
      "details": {"note": "sample"},
      "enabled": true,
      "reported_deployed": true,
      "hold": {
        "id": 7,
        "user": {"id": 2, "username": "alice", "email": "alice@example.com", "name": "Alice Example"},
        "acquired_at": "...", "metering_started_at": "...", "deployed_at": "...",
        "release_requested_at": null
      }
    }
  ]
}
```

- `setups` lists every setup, disabled ones included. `hold` is `null` for a free setup.
- `reported` and the `observed_at` fields are `null` before the first report.
- `reported_deployed` reflects the stored report, not the last poll.
- The response never includes balances, ledger entries, other users, settings or audit data.

### `POST /api/v1/integration/holds/{id}/release`

Scope `holds:release`.

```json
{"reason": "idle for 48 hours, torn down by automation"}
```

- `reason` is required, 1 to 500 characters after trimming.
- The teardown check (HOLD-2) is skipped: the service is the authority on what is deployed.
- The hold is released at once. `released_by` stays null and the account's name is recorded on the hold.
- An audit entry `hold.service_release` is written in the same transaction, with the account name as actor.
- `hold.released` fires with `data` of `{"forced": false, "reason": "...", "service_account": "<name>"}`.
- Response 200 with the released hold. 404 `not_found` for an unknown id, 409 `conflict` if already released.

## The `reported` provider and pressure source

Config:

```yaml
provider:
  type: reported
  config:
    max_age: 10m      # default; minimum 1m
pressure:
  type: reported
  config:
    max_age: 10m
```

| | `reported` provider | `reported` pressure source |
| --- | --- | --- |
| Returns | One `Allocation` per stored deployment, `ResourceName` equal to the setup name | The stored `value` |
| No report yet | Error: `no deployment report received yet` | Error: `no pressure report received yet` |
| Report older than `max_age` | Error naming the report's age | Error naming the report's age |

An error fails the poll, exactly as an unreachable backend does today: `poll.failed` fires, nothing is debited, and a gap longer than twice the poll interval becomes an outage (MET-4). This is the safety property of the whole design: **if the customer's service stops reporting, nobody is charged on old data.**

The two can be used independently. `provider: reported` with `pressure: inventory` is valid, and so is `pressure: reported` with no provider.

### Wiring constraints

- Extension factories receive only their config block, and extensions do not touch the database. The stored reports therefore live in a core package (suggested: `internal/reported`) that the API handlers write and the two extensions read.
- The extensions live in a public package (suggested: `providers/reported`), register under the name `reported`, and are blank-imported by `cmd/hourglassd`. A third-party `main` gets them the same way.
- The core must not import the extension package. `tools/check_core_generic.sh` must still pass: besides imports, it rejects vendor and provider terms and resource naming rules in any file under `internal/`, `pkg/hourglass/` and `pkg/hourglasstest/`, comments and test data included.
- Both extensions must pass the conformance suites in `pkg/hourglasstest`.
- `hourglassd check-provider` must work with them and report the age of each stored report.

## Releasing a deployed setup

Today a user's release is refused with 409 `still_deployed` while the setup is reported deployed and `release_requires_teardown` is on. Nothing tells the customer's automation that the user wants out. The change:

| Situation | Today | New |
| --- | --- | --- |
| Setup not reported deployed, or setting off, or no provider | Released at once | Unchanged |
| Setup reported deployed, setting on | 409 `still_deployed` | 202: the hold is marked `release_requested_at` and stays active; `hold.releasing` fires |

Rules for the pending state:

- `release_requested_at` is set once. A repeated request returns the same 202 and fires no second event.
- `hold.releasing` goes through the background queue and is **not** vetoable. `user`, `setup` and `hold` are set as for `hold.released`; `data` is `{}`. It is added to `hourglass.EventTypes()` after `hold.deployed`, and the webhook hook's `types` filter accepts it.
- The hold remains active: it still counts toward the user's hold limit, the setup stays held, and metering continues under the policy. The user pays until teardown is real, which keeps the incentive intact.
- **Completion.** On each successful poll, after metering, the collector releases every hold that has `release_requested_at` set and whose setup is no longer reported. `released_by` is the hold's owner and `hold.released` fires with `{"forced": false, "reason": ""}`.
- A service account's release call and an admin's force-release both complete a pending release immediately.
- A pending release cannot be cancelled in this version.
- The Setups page shows a pending hold as "Releasing" with the request time, in place of the Release button.

This changes HOLD-2 for every deployment, including ones using the `vsphere` provider: a user who clicks Release and then tears down by hand no longer has to come back and click again. It is an owner-visible behaviour change; record it as a decision marked **REVIEW**.

## Why full-set reports

A service that sends "deployed" and "deleted" messages one at a time has no way to recover from a lost message: a lost "deleted" bills a user indefinitely, and a lost "deployed" lets a setup run free. A full-set report is self-correcting on the next call, is safe to retry, and doubles as the heartbeat that `max_age` checks. A per-setup convenience route can be added later without changing the model.

## Data model

One new migration, `0005`. Follow CONTRIBUTING.md, "Add a migration".

| Table or column | Purpose |
| --- | --- |
| `service_accounts` (`id`, `name` unique, `description`, `scopes_json`, `enabled`, `created_by`, `created_at`) | The principals |
| `service_tokens` (`id`, `account_id`, `token_hash`, `created_at`, `expires_at`, `last_used_at`) | Cascade-deleted with the account |
| `reported_pressure` (single row: `value`, `observed_at`, `received_at`, `service_account`) | Last pressure report |
| `reported_deployments_meta` (single row: `observed_at`, `received_at`, `service_account`) | Last deployment report's header; exists even when the set is empty |
| `reported_deployments` (`setup_name` primary key, `labels_json`) | The deployed set, replaced atomically with the header |
| `holds.release_requested_at` | The pending-release marker |
| `holds.released_by_service` | Account name when a service account released the hold |
| `audit_log.actor_service` | Account name when a service account is the actor; `actor_id` is then null |

Service account names are stored as text on holds and audit rows, not as foreign keys, so deleting an account never rewrites history.

## Changes by area

| Area | Change |
| --- | --- |
| `internal/store` | Migration 0005 and queries for the tables above |
| `internal/auth` | Bearer authentication for service tokens, a service principal in the request context, a `ServiceRoute(scope)` chain, rate limiting of failed attempts |
| `internal/api` | New `AccessService` level in `routes.go`; the four integration routes; the six admin routes; `release` returns 202 for a pending release |
| `internal/reported` (new) | Stored-report read and write, staleness and ordering rules |
| `providers/reported` (new) | The two extensions; blank import in `cmd/hourglassd/main.go` |
| `internal/holds` | Pending release, service release, `hold.releasing` |
| `internal/collector` | Complete pending releases after a successful poll |
| `pkg/hourglass` | `EventHoldReleasing`; `Hold.ReleaseRequestedAt` in the type and its JSON form |
| `hooks/webhook` | Accept the new type in `types` |
| `api/openapi.yaml` | Every new route and field, with `x-roles: service` for integration routes; the contract test must cover them |
| `ui/` | Service accounts admin page; "Releasing" state on Setups; last report times on the collector view; regenerate API types |
| `deploy/`, `e2e/` | A compose variant or e2e scenario that runs with `reported` extensions and a scripted reporter |
| `docs/` | See [Documentation deliverables](#documentation-deliverables) |

## Requirements

New requirement ids continue the spec's scheme. Each needs at least one test that fails when the requirement is broken.

### Service accounts

- **SVC-1** A service token authenticates only on `/api/v1/integration/` routes. Every other route treats the request as anonymous, whatever the token's scopes.
- **SVC-2** A session cookie, including an admin's, does not authenticate on `/api/v1/integration/` routes.
- **SVC-3** Each integration route requires its scope; a valid token without it gets 403 `forbidden` and learns nothing else.
- **SVC-4** A token is returned once at creation and is stored only as a hash. It appears in no log, audit entry or later response.
- **SVC-5** A revoked or expired token, or a token of a disabled or deleted account, is refused on the next request with 401.
- **SVC-6** Creating, changing and deleting accounts and issuing and revoking tokens are admin-only and audited.
- **SVC-7** Failed bearer attempts are rate-limited per client address.
- **SVC-8** The permissions test (`internal/api/permissions_test.go`) covers every new route for every principal: anonymous, unassigned, developer, admin, local admin, and a service account with and without the scope.

### Reports

- **INT-1** `PUT pressure` rejects a missing, non-finite or out-of-range value with 400 `validation` and stores nothing.
- **INT-2** `PUT deployments` replaces the whole stored set atomically. A reader never sees a mix of two reports.
- **INT-3** A report older than the stored one is refused with 409 `stale_report`; an equal one is idempotent.
- **INT-4** A report for an extension that is not configured is refused with 409 `not_configured`.
- **INT-5** The `reported` provider and pressure source fail the poll when there is no report or the report is older than `max_age`. No debit is written for that poll.
- **INT-6** Reports survive a restart.
- **INT-7** A reported setup with an active hold gets `deployed_at` and `hold.deployed` at the next poll; with the `default` policy, metering starts at that poll. A reported setup with no hold is an orphan (HOLD-4).
- **INT-8** `GET state` returns every setup and active hold and nothing about balances, ledger, settings or audit.

### Release

- **HOLD-6** A user's release of a setup that is reported deployed, with `release_requires_teardown` on, returns 202, sets `release_requested_at` once and fires `hold.releasing` once.
- **HOLD-7** A hold with a pending release stays active and metered, and completes at the first successful poll in which its setup is not reported.
- **HOLD-8** A service account with `holds:release` can release any active hold with a reason. The release is audited under the account's name and fires `hold.released` with `service_account` in `data`.
- **HOLD-9** A failed poll never completes a pending release.

### Compatibility

- **CMP-1** A deployment that configures no service account and no `reported` extension behaves as before, except for HOLD-6 and HOLD-7.
- **CMP-2** `./ci.sh` passes: SPDX headers, genericity, licenses, Gazelle, `bazel build //...`, `bazel test //...`.

## Documentation deliverables

1. **`docs/integrating.md`**, the guide for a customer's AI agent or engineer. A draft exists and was written from this design before the code. Run every example in it against the real server, correct whatever differs, and remove its status banner. It must stay self-contained: a reader who has only that file can build a working integration.
2. **Do not edit `docs/SPEC.md`.** It is a copy of the owner's source document (see D40). This design is the contract for the feature. Record it in `docs/decisions.md` as an owner-directed change that supersedes HOLD-2's refusal and extends AUTH-4, and list there the edits the source needs: the requirements above, the new routes in the HTTP API table, the new tables, a service-account column in Roles and permissions, and `hold.releasing` in the event table.
3. `docs/extending.md`: `hold.releasing`, `Hold.ReleaseRequestedAt`, the `reported` rows in Bundled extensions, and a pointer to the integration guide as the no-Go path.
4. `docs/deploying.md`: the `reported` config, service account setup, `max_age`, and an entry under Upgrade for migration 0005 and the changed release behaviour.
5. `README.md`: one paragraph and a link under Documentation.
6. `docs/decisions.md`: one row per decision taken during the work, from D42 on.

## Suggested task order

1. Store migration and queries; `internal/reported`.
2. Service accounts: auth, admin routes, permissions test.
3. Report routes and the state route; OpenAPI and contract test.
4. `providers/reported`, conformance suites, `check-provider`.
5. Pending release, `hold.releasing`, collector completion, service release.
6. UI pages and states.
7. End-to-end scenario with a scripted reporter covering acquire, deploy, release request, teardown and completion, plus the reporter going silent.
8. Documentation, including verifying `docs/integrating.md`.

An independent verification pass should follow steps 2, 5 and 7, as the earlier work in `.orchestrator/` did.

## Open points for the owner

- **Pending release changes HOLD-2 for everyone.** The narrower alternative keeps the 409 refusal, emits `hold.releasing` on each refused attempt, and relies on the service calling release. It needs no schema change and no UI state, but the user is shown a refusal while automation is in fact tearing the setup down.
- **Default `max_age` of 10 minutes** is twice the default poll interval. A customer who lengthens the poll interval should lengthen it to match.
- **The state read exposes holder username, email and name** to any account with `state:read`, because a deployer normally needs to know who the setup is for.
