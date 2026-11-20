# Integrating your infrastructure with Hourglass

This guide is for the engineer or AI agent connecting an existing lab stack to Hourglass. It is self-contained: you do not need to read Hourglass's source or write any Go. You run the stock Hourglass container and build one small service of your own.

## What Hourglass does and what you do

| Hourglass | Your service |
| --- | --- |
| Keeps the list of setups, who holds each one, and every user's token balance | Knows what is really deployed in your infrastructure |
| Charges holders every poll (5 minutes by default), faster when the lab is contended | Measures how contended the lab is |
| Tells you when a user acquires a setup or asks to release one | Deploys and tears down in response |
| Never touches your infrastructure | Never touches Hourglass's database |

A **setup** is one shared lab environment, identified by a name such as `lab-03`. A **hold** is one user holding one setup. Your service always refers to a setup by its Hourglass name.

## The four jobs

| Job | Call | How often |
| --- | --- | --- |
| Report what is deployed | `PUT /api/v1/integration/deployments` | Every 60 seconds, and right after you deploy or tear down |
| Report lab pressure | `PUT /api/v1/integration/pressure` | Every 60 seconds |
| React to events | Receive webhooks from Hourglass | As they arrive |
| Reconcile | `GET /api/v1/integration/state` | At startup and every 5 minutes |

Only the first two are required to make metering work. Add the others when you want Hourglass to drive deployment and teardown.

**Reporting more often than every 60 seconds gains nothing.** Hourglass reads your latest report once per poll. Reporting less often than every few minutes is dangerous: see [If you stop reporting](#if-you-stop-reporting).

## One-time setup by the Hourglass admin

1. In the Hourglass config file, select the `reported` extensions and point a webhook at your service:

   ```yaml
   provider:
     type: reported
     config:
       max_age: 10m
   pressure:
     type: reported
     config:
       max_age: 10m
   metering:
     type: default
     config:
       start_on_allocation: true   # start charging when you report the setup deployed
       start_after: 2h             # or 2 hours after acquire, whichever comes first
   hooks:
     - type: webhook
       config:
         url: https://lab-automation.example.com/hourglass
         secret: a-long-shared-secret
         types: [hold.acquired, hold.releasing, hold.released]   # add poll.failed to be alerted; see "React to events"
   ```

   Until your service has sent its first deployments report and its first pressure report, every poll fails with `no deployment report received yet` (or `no pressure report received yet`). That is expected at first start.

2. In the admin UI, open Service accounts, create an account for your service, and give it only the scopes it needs:

   | Scope | Needed for |
   | --- | --- |
   | `deployments:write` | Reporting what is deployed |
   | `pressure:write` | Reporting lab pressure |
   | `state:read` | Reading setups and holds |
   | `holds:release` | Releasing a hold after your automation tears a setup down |

3. Issue a token for the account (`hgsa_` followed by the account id and a long random part). It is shown once. Store it in your secret manager and never in source control or logs. An account can hold several tokens, so you can rotate without a gap: issue a new one, switch the service, revoke the old one.

4. Create the setups in the admin UI (or the seed file) with the names your service will report.

## Authentication

Send the token on every call:

```
Authorization: Bearer hgsa_...
Content-Type: application/json
```

A service token works only on `/api/v1/integration/` routes. It cannot read balances, change settings or act as a user, and a signed-in user's cookie does not work on these routes either. No cookie and no CSRF token are involved.

## Report what is deployed

```sh
curl -sS -X PUT "$HOURGLASS/api/v1/integration/deployments" \
  -H "Authorization: Bearer $HOURGLASS_TOKEN" -H 'Content-Type: application/json' \
  -d '{
        "observed_at": "2026-11-20T10:15:00Z",
        "deployments": [
          {"setup": "lab-03", "labels": {"cluster": "c1"}},
          {"setup": "lab-07"}
        ]
      }'
```

Rules:

- **Send the complete set every time.** Any setup you leave out is treated as not deployed. Do not send only what changed.
- **Send an empty list when nothing is deployed.** `{"deployments": []}` is a valid report. Sending nothing at all is not the same thing.
- **If you cannot see your infrastructure, do not report.** Skip the call and let the report age. An empty list sent because your vCenter query failed tells Hourglass everything was torn down.
- `setup` is the Hourglass setup name, exactly. Hourglass has no name mapping of its own. If your backend names things differently (a resource pool `lab-03_RP` for the setup `lab-03`), translate in your service before reporting. You may put the backend name in `labels` so admins can see it.
- You decide what "deployed" means for a setup, for example that its resource pool exists in vCenter.
- `labels` is optional and free-form. Admins see them next to an orphan on the collector page.
- `observed_at` is when you took the measurement, in RFC 3339 UTC. If you leave it out, Hourglass uses the time it received the report. A report older than the one Hourglass already holds is refused (`stale_report`); one with the same `observed_at` is accepted. More than 2 minutes ahead of Hourglass's clock is refused.
- At most 10,000 deployments, 32 labels per deployment and a 1 MB body.

Response:

```json
{"observed_at": "...", "received_at": "...", "expires_at": "...", "accepted": 2, "unknown": []}
```

`received_at` is when Hourglass got the report and `expires_at` is `observed_at` plus `max_age`. `unknown` lists names that match no setup in Hourglass. They are ignored. A non-empty `unknown` usually means a typo or a setup that an admin has not created yet.

What Hourglass does with it, at its next poll:

- A deployed setup that someone holds is marked deployed, and (with the default policy) charging starts.
- A deployed setup that nobody holds is shown to admins as an orphan, with your labels, on the collector page. Nobody is charged.
- A held setup that is no longer deployed, and whose holder asked to release it, is released.

## Report lab pressure

```sh
curl -sS -X PUT "$HOURGLASS/api/v1/integration/pressure" \
  -H "Authorization: Bearer $HOURGLASS_TOKEN" -H 'Content-Type: application/json' \
  -d '{"value": 0.72, "observed_at": "2026-11-20T10:15:00Z"}'
```

- `value` is a number from 0 to 1: 0 means the lab is empty, 1 means it is full. How you compute it is yours to decide. A common choice is the highest of CPU, memory and storage utilisation across the clusters that host setups.
- **Send the raw, current value.** Do not average it. Hourglass averages over 15 minutes and maps the result to a multiplier through bands the admin sets (by default: free below 0.50, 1x from 0.50, 2x from 0.75, 3x from 0.90).
- If you cannot measure pressure, skip the call. Do not send 0.

Response (`observed_at` and `expires_at` as for deployments):

```json
{"value": 0.72, "observed_at": "...", "received_at": "...", "expires_at": "..."}
```

## If you stop reporting

Each report expires `max_age` after its `observed_at` (10 minutes by default; the response's `expires_at` tells you exactly when). Once either report that the config selects has expired (or before the first one has arrived):

- Every poll fails until a fresh report arrives, with an error that names `max_age`. `poll.failed` is sent.
- **Nobody is charged** for that time. Once reports resume, Hourglass records the gap as an outage and shows it to admins.
- No hold is marked deployed or released by a poll.

This is deliberate: stale data never bills anyone. It also means an outage of your service stops metering for the whole lab, so run it with a restart policy and alert on a failed report.

## React to events

Hourglass sends each event as an HTTP `POST` with a JSON body to the URL in the config.

```json
{
  "type": "hold.acquired",
  "time": "2026-11-20T10:16:00.123Z",
  "user": {"id": 2, "username": "alice", "email": "alice@example.com", "name": "Alice Example", "roles": ["developer"]},
  "setup": {"name": "lab-03", "labels": {"size": "large"}, "details": {"note": "sample"}},
  "hold": {"id": 7, "setup": "lab-03", "user_id": 2, "acquired_at": "2026-11-20T10:16:00Z",
           "metering_started_at": null, "deployed_at": null, "release_requested_at": null, "released_at": null},
  "data": {}
}
```

Times are RFC 3339 in UTC; `time` may carry fractional seconds of any length, so use a real parser. `hold.released` has `data` of `{"forced": false, "reason": "..."}`, plus `"service_account": "<name>"` when a service account released the hold; `poll.failed` has `data.error`; other events have `{}`.

**Who is the user?** `user.id` is stable and is the thing to key on. `username` is the local account name for the local admin and **empty for everyone who signs in through OIDC**, which is nearly all users, so do not rely on it. `email` and `name` come from the identity provider (`email` can be empty if the provider sends none). The state response carries the same four fields under `hold.user`, with the same rule.

| Event | Meaning | What to do |
| --- | --- | --- |
| `hold.acquiring` | A user is about to acquire. Sent inside their request, and only if you add it to `types` | Optional. Reply 2xx to allow. Reply non-2xx to refuse; the first 200 bytes of your reply body are shown to the user |
| `hold.acquired` | The hold exists | Start deploying the setup for `user` |
| `hold.releasing` | The user asked to release a setup that is still deployed | Start tearing the setup down |
| `hold.released` | The hold has ended: the user released it, an admin forced it, a service account released it, or a poll completed a pending release | Make sure the setup is torn down; free anything you kept for it |
| `hold.deployed`, `hold.metering_started` | Hourglass saw your report and began charging | Nothing required. Add them to `types` if you want them |
| `poll.failed` | A poll failed; `data.error` says why | Alert if it mentions a missing or expired report |

Requirements for your receiver:

- **Reply within 30 seconds.** Queue the deployment or teardown and return 2xx at once. Do not do the work inside the request.
- **Verify the signature** when a secret is configured. The `X-Hourglass-Signature` header is `sha256=` followed by the hex HMAC-SHA256 of the raw request body, keyed with the secret. Compute it over the bytes you received, before parsing:

  ```python
  import hashlib, hmac
  # secret and body are bytes
  want = "sha256=" + hmac.new(secret, body, hashlib.sha256).hexdigest()
  ok = hmac.compare_digest(want, request.headers["X-Hourglass-Signature"])
  ```

- **Make every handler idempotent.** Key your work on `hold.id`.
- **Do not rely on receiving every event, or on their order across holds.** Each is delivered at most once and never retried. If your receiver is down, the event is gone. Reconciliation, below, is what makes the integration correct.
- The `X-Hourglass-Event` header carries the event type. The body's `Content-Type` is `application/json`.
- `hold.releasing` is sent only when the setup was still reported deployed at the last poll. Right after Hourglass restarts, before its first successful poll, a user's release is immediate and you get only `hold.released`. Treat the state response as the authority.

## Reconcile

```sh
curl -sS "$HOURGLASS/api/v1/integration/state" -H "Authorization: Bearer $HOURGLASS_TOKEN"
```

The response lists every setup, disabled ones included, with its labels, details, enabled flag and current hold (`null` when free), plus the reports Hourglass currently holds from you and when they expire. Trimmed example:

```json
{
  "server_time": "2026-11-20T10:16:02Z",
  "poll": {"interval_seconds": 300, "last_ok_at": "2026-11-20T10:15:00Z"},
  "pressure": {"source": "reported", "reported": {"value": 0.72, "observed_at": "...", "expires_at": "..."},
               "smoothed": 0.68, "multiplier": 1},
  "deployments": {"provider": "reported", "observed_at": "...", "expires_at": "..."},
  "setups": [
    {"name": "lab-03", "labels": {"size": "large"}, "details": {"note": "sample"}, "enabled": true,
     "reported_deployed": true,
     "hold": {"id": 7, "user": {"id": 2, "username": "", "email": "alice@example.com", "name": "Alice Example"},
              "acquired_at": "...", "metering_started_at": "...", "deployed_at": "...", "release_requested_at": null}}
  ]
}
```

- `reported_deployed` says whether your latest stored report lists the setup. It is not what the last poll acted on: `hold.deployed_at` and `release_requested_at` are.
- `poll.last_ok_at` is `null` until the first successful poll after Hourglass starts. `pressure.smoothed` is `null` until then too. `pressure.reported`, `deployments.observed_at` and `deployments.expires_at` are `null` before your first report (and when that extension is not `reported`).
- There are no balances, ledger entries, other users, settings or audit data in it.

At startup and every 5 minutes, compare it with your infrastructure and correct the difference:

| In Hourglass | In your infrastructure | Action |
| --- | --- | --- |
| Held, `release_requested_at` is null | Not deployed, no deployment running | Deploy it (you missed `hold.acquired`) |
| Held, `release_requested_at` is set | Deployed | Tear it down (you missed `hold.releasing`) |
| Free | Deployed | Tear it down, or leave it and let admins see the orphan, according to your policy |
| Held | Deployed | Nothing |

Also check `pressure.reported.expires_at` and `deployments.expires_at`. If either is null or in the past, your reporting loop is not working.

Use `hold.id` from the state response for the release call below.

## Release a hold yourself

Use this when your automation ends a setup's life: an idle timeout, a failed deployment, a maintenance window.

```sh
curl -sS -X POST "$HOURGLASS/api/v1/integration/holds/7/release" \
  -H "Authorization: Bearer $HOURGLASS_TOKEN" -H 'Content-Type: application/json' \
  -d '{"reason": "deployment failed: datastore full"}'
```

- `reason` is required. The user and the admins see it, so write it for a person.
- The hold ends immediately and charging stops. Hourglass does not check whether the setup is still deployed: tear it down first, or the next report shows it as an orphan. This also ends a hold whose release the user already requested.
- `reason` is 1 to 500 characters. The user sees it on the released hold.
- The response is the released hold as the web UI shows it to its holder (`200`, with `released_at` and `release_reason` set). `hold.released` is sent with your account's name in `data.service_account`.
- Releasing a hold that has already ended is `409 conflict`: treat it as success.
- The call is audited under your service account's name.

You do **not** need this call for the normal release path. When a user asks to release and you tear the setup down, your next deployments report no longer lists it, and Hourglass completes the release at its next poll.

## The full lifecycle

1. A user clicks Acquire. Hourglass sends `hold.acquired`.
2. You deploy the setup. This can take as long as it takes; the default policy does not charge until you report it deployed or 2 hours pass.
3. Your next deployments report includes the setup. At the next poll Hourglass marks the hold deployed and starts charging.
4. The user clicks Release. The setup is still deployed, so Hourglass marks the hold as releasing, keeps charging, and sends `hold.releasing`.
5. You tear the setup down and send a deployments report without it.
6. At the next poll Hourglass releases the hold, stops charging, and sends `hold.released`. That last poll's interval is still charged, because the hold was metered until the poll saw the setup gone.

Charging starts and stops on poll boundaries, so allow up to one poll interval between your report and its effect. A user's second click on Release changes nothing, and a pending release cannot be cancelled.

## Errors

Every error has the shape `{"error": {"code": "...", "message": "..."}}`.

| Status | Code | Meaning | What to do |
| --- | --- | --- | --- |
| 401 | `unauthenticated` | Missing, wrong, expired or revoked token, a disabled or deleted account, or a user's cookie instead of a token | Stop and alert. Do not retry in a loop |
| 403 | `forbidden` | The token lacks the scope for this route | Ask the admin to add the scope |
| 404 | `not_found` | No such hold | Re-read the state |
| 409 | `stale_report` | Hourglass already holds a report with a later `observed_at` (an equal one is accepted) | Drop this report; send a fresh one on the next cycle |
| 409 | `not_configured` | The config does not select the `reported` provider or pressure source | Fix the Hourglass config; the message names the key |
| 409 | `conflict` | The hold is already released | Treat as success |
| 400 | `validation` | Bad body: not valid JSON, over 1 MB; `value` missing, not a number or outside 0 to 1; `deployments` missing or `null` (an empty list is fine); an empty or repeated setup name; more than 10,000 deployments or 32 labels in one; `observed_at` more than 2 minutes ahead; `reason` missing or over 500 characters. `message` says which, and `field` names it for most | Fix the request. Do not retry unchanged |
| 429 | `rate_limited` | Too many failed authentication attempts from your address (5 in 15 minutes). The `Retry-After` header says how many seconds to wait. A request with a valid token is never refused for this | Back off, then check the token |
| 5xx | | Hourglass is unhealthy or restarting | Retry on the next cycle |

For network errors and 5xx, do not build a retry queue. Reports are full snapshots, so the next scheduled report supersedes anything you failed to send.

## Checklist

- [ ] The token comes from a secret store and is never logged.
- [ ] Both reports are sent every 60 seconds from a loop that survives a single failure.
- [ ] A failed infrastructure query skips the report; it never sends an empty list or a zero.
- [ ] The deployments report always contains the full set.
- [ ] Clocks are synchronised (NTP). `observed_at` more than 2 minutes ahead of Hourglass is rejected.
- [ ] The webhook receiver verifies the signature, replies within 30 seconds, and is idempotent per hold id.
- [ ] Reconciliation runs at startup and every 5 minutes.
- [ ] There is an alert on a failed report, on a 401, and on `poll.failed`.
- [ ] The service account has only the scopes the service uses.
