# Setting up Hourglass and integrating your infrastructure

This guide is for the engineer or AI agent who installs Hourglass in a customer environment and connects the existing lab stack to it. It is self-contained: you do not need Hourglass's source, Bazel or any Go. You run the published Hourglass container and build one small service of your own.

It has three parts. Do them in order.

1. [Part 1: Set up Hourglass](#part-1-set-up-hourglass). Run the container over HTTPS, with its data on a volume.
2. [Part 2: Build the integration service](#part-2-build-the-integration-service). Report deployments and pressure, optionally report weights, react to events, reconcile.
3. [Part 3: Verify and go live](#part-3-verify-and-go-live).

[Deploying Hourglass](deploying.md) is the full operator reference (every setting, OIDC, backup, upgrade). You need it only for something this guide does not cover.

## If you are an AI agent

- **Work one step at a time.** Each step in Part 1 ends with a check and the output to expect. If a check fails, stop, read the container log (`docker compose logs hourglass`), and fix the cause. Do not go on with a failed check.
- **Ask the person you work for; do not guess.** [What to find out first](#what-to-find-out-first) lists what only they can decide.
- **Never print a secret.** Passwords, the session key, the webhook secret and service tokens go straight into a secret store or a file with mode `0600`. Do not echo them, log them, commit them, or paste them into a chat or a ticket. A service token is shown once: capture it to a file, never to the terminal.
- **Do not make these changes without being asked:** turning `enforcement` on, deleting the data volume (`docker volume rm`, `docker compose down -v`), force-releasing a user's hold, or changing roles and balances.
- **Hourglass never touches the infrastructure.** Deploying and tearing down is your service's job, and it is the only part that can damage something. Build it to do nothing when it is unsure.
- The examples use `curl`, `jq` and `openssl`. `$HOURGLASS` is the base URL, for example `https://hourglass.example.com`, with no trailing slash.

## What to find out first

| Question | Why it matters | If they have no preference |
| --- | --- | --- |
| Which image version? | The published image is `ghcr.io/panset/hourglass` | Use `latest`, the newest release |
| What host name will users type? | It goes in the certificate and in the OIDC redirect URL | Ask |
| Is there a certificate, or should you make a self-signed one? | Users must reach Hourglass over HTTPS | Self-signed, then give them the certificate to trust |
| How do users sign in? | Hourglass users come from an OIDC identity provider (Keycloak, Entra ID, Okta and the like). Without one, only the local `admin` can sign in | Ask. You need the issuer URL, a client id and a client secret |
| What are the setups called? | Your service reports setups by exactly these names | Use the names people already use for the environments |
| What does "deployed" mean, and how is lab pressure measured? | You implement both in Part 2 | Deployed: the environment's resources exist. Pressure: the highest of CPU, memory and storage use, 0 to 1 |
| Where does your service run, and at what URL can Hourglass reach it? | Hourglass posts events to it | Next to Hourglass, on the same Docker network |
| Where do backups go? | The database is one file on one volume | Ask. Off the host |

## What Hourglass does and what you do

| Hourglass | Your service |
| --- | --- |
| Keeps the list of setups, who holds each one, and every user's token balance | Knows what is really deployed in your infrastructure |
| Charges holders every poll (5 minutes by default), faster when the lab is contended | Measures how contended the lab is |
| Tells you when a user acquires a setup or asks to release one | Deploys and tears down in response |
| Never touches your infrastructure | Never touches Hourglass's database |

A **setup** is one shared lab environment, identified by a name such as `lab-03`. A **hold** is one user holding one setup. Your service always refers to a setup by its Hourglass name.

## The five jobs

| Job | Call | How often |
| --- | --- | --- |
| Report what is deployed | `PUT /api/v1/integration/deployments` | Every 60 seconds, and right after you deploy or tear down |
| Report lab pressure | `PUT /api/v1/integration/pressure` | Every 60 seconds |
| Report weights (optional) | `PUT /api/v1/integration/weights` | Every 60 seconds, if the admin has turned reported weights on |
| React to events | Receive webhooks from Hourglass | As they arrive |
| Reconcile | `GET /api/v1/integration/state` | At startup and every 5 minutes |

Only the first two are required to make metering work. Report weights when the price of a setup should follow what the setup currently reserves. Add the others when you want Hourglass to drive deployment and teardown.

**Reporting more often than every 60 seconds gains nothing.** Hourglass reads your latest report once per poll. Reporting less often than every few minutes is dangerous: see [If you stop reporting](#if-you-stop-reporting).

## Part 1: Set up Hourglass

You need a Linux host with Docker (amd64 or arm64) and port 443 open to users. Hourglass is one container, one port and one volume. All state is one SQLite file in `/data`.

### Step 1: Make a directory and the secrets

```sh
mkdir -p hourglass/certs hourglass/ca && cd hourglass
umask 077
printf 'HOURGLASS_ADMIN_PASSWORD=%s\nHOURGLASS_SESSION_KEY=%s\n' \
  "$(openssl rand -hex 16)" "$(openssl rand -hex 32)" > hourglass.env
openssl rand -hex 32 > webhook.secret
```

`hourglass.env` and `webhook.secret` are secrets. Keep them out of version control. The admin password is read only on the first start; the session key must stay the same across restarts or everyone is signed out.

### Step 2: Get a certificate

If the customer gave you a certificate and key, copy them to `certs/tls.crt` (leaf first, then intermediates) and `certs/tls.key`. Otherwise make a self-signed one. Replace the host name; it must be the name users type:

```sh
openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
  -keyout certs/tls.key -out certs/tls.crt -subj "/CN=hourglass.example.com" \
  -addext "subjectAltName=DNS:hourglass.example.com"
chmod 755 certs && chmod 644 certs/tls.crt
sudo chown 65532:65532 certs/tls.key && sudo chmod 600 certs/tls.key
```

The container runs as uid 65532 and must be able to read the key. A key it cannot read stops the start with `permission denied`.

### Step 3: Write the config

`config.yaml`. Put the contents of `webhook.secret` where shown, and your service's URL:

```yaml
listen: ":8443"
database: /data/hourglass.db
timezone: UTC                  # the month boundary for allowances uses this zone
seed: /etc/hourglass/seed.yaml
tls:
  cert: /certs/tls.crt
  key: /certs/tls.key
provider:
  type: reported               # your service reports what is deployed
  config:
    max_age: 10m
pressure:
  type: reported               # your service reports lab pressure
  config:
    max_age: 10m
metering:
  type: default
  config:
    start_on_allocation: true  # start charging when you report the setup deployed
    start_after: 2h            # or 2 hours after acquire, whichever comes first
hooks:
  - type: webhook
    config:
      url: https://lab-automation.example.com/hourglass
      secret: PASTE-THE-CONTENTS-OF-webhook.secret
      types: [hold.acquired, hold.releasing, hold.released, poll.failed]
```

The file now holds the webhook secret and uid 65532 must read it: `sudo chown 65532:65532 config.yaml && sudo chmod 600 config.yaml`. An unknown key anywhere in the file stops the start, so do not add keys that this guide or [the reference](deploying.md#configuration-reference) does not name.

`seed.yaml` creates the first setups and rate rules (`chmod 644 seed.yaml`; it holds no secret). It is read once, on the first start of an empty database, and never again:

```yaml
setups:
  - name: lab-01
    labels: {size: medium}
    details: {}
  - name: lab-02
    labels: {size: large}
    details: {}
rate_rules:
  - {selector: "size=large", weight: 2}   # a large setup costs twice as much per hour
```

### Step 4: Start it

`compose.yaml`:

```yaml
services:
  hourglass:
    image: ghcr.io/panset/hourglass:latest      # the newest release
    restart: unless-stopped
    env_file: hourglass.env
    environment:
      HOURGLASS_CONFIG: /etc/hourglass/config.yaml
      SSL_CERT_DIR: /extra-ca:/etc/ssl/certs    # trust the certificates in ./ca, see below
    ports:
      - "443:8443"
    volumes:
      - data:/data
      - ./config.yaml:/etc/hourglass/config.yaml:ro
      - ./seed.yaml:/etc/hourglass/seed.yaml:ro
      - ./certs:/certs:ro
      - ./ca:/extra-ca:ro
volumes:
  data:
```

```sh
docker compose pull && docker compose up -d
```

`latest` moves with each release, and Docker does not fetch it again by itself: `docker compose pull` is what brings a newer one. So the image changes only when you run that command.

**Trusting private certificates on outgoing calls.** Hourglass makes HTTPS calls of its own: to your webhook receiver and to the OIDC issuer. It checks their certificates against the public authorities in the image. If either uses a self-signed certificate or a private authority, put that certificate (PEM) in `./ca`, readable by everyone. `SSL_CERT_DIR` above adds that directory to the trusted set. Without it, webhook deliveries fail with a certificate error in the log.

**The data.** The named volume `data` is the whole state of Hourglass. It survives `docker compose down`, restarts and image upgrades. It does not survive `docker compose down -v`, `docker volume rm`, or the loss of the host: see [Back up the database](#back-up-the-database).

### Step 5: Check that it is up

```sh
export HOURGLASS=https://hourglass.example.com
export CURL_CA_BUNDLE=$PWD/certs/tls.crt     # self-signed only: makes curl trust it
curl -sS $HOURGLASS/healthz
```

Expect `{"status":"ok","database":"ok","last_poll_age_seconds":null}`. `null` is right at this point. Then check the log:

```sh
docker compose logs hourglass | grep -E '"msg":"(http listening|poll failed)"'
```

Expect an `http listening` line with `"tls":true`, and a `poll failed` line that says `no deployment report received yet`. That failure is expected: nothing has reported yet, and nobody is charged while polls fail. If your webhook receiver is already running, it receives `poll.failed`.

If the host name does not resolve yet, add `--resolve hourglass.example.com:443:<host address>` to the `curl` commands.

### Step 6: Set the admin password

The first start created the local account `admin` with the password from `hourglass.env`. It must be changed before the API answers anything else (`403 password_change_required`). Choose the new password with the customer, at least 12 characters, and store it in their secret store.

```sh
. ./hourglass.env                                  # the initial password; NEW_PASSWORD is yours to set
curl -sS -c cookies.txt -o /dev/null -w '%{http_code}\n' -H 'Content-Type: application/json' \
  -d "$(jq -n --arg p "$HOURGLASS_ADMIN_PASSWORD" '{username:"admin",password:$p}')" \
  $HOURGLASS/auth/local                            # 200

CSRF=$(curl -sS -b cookies.txt $HOURGLASS/api/v1/session | jq -r .csrf_token)
curl -sS -b cookies.txt -c cookies.txt -o /dev/null -w '%{http_code}\n' \
  -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
  -d "$(jq -n --arg c "$HOURGLASS_ADMIN_PASSWORD" --arg n "$NEW_PASSWORD" '{current_password:$c,new_password:$n}')" \
  $HOURGLASS/auth/local/password                   # 200

CSRF=$(curl -sS -b cookies.txt $HOURGLASS/api/v1/session | jq -r .csrf_token)   # it changed: read it again
```

- Every admin call that changes something needs the session cookie and the `X-CSRF-Token` header. The token changes when the password changes.
- Five wrong passwords in 15 minutes lock sign-in for the account (`429 rate_limited`). Do not retry a failing sign-in in a loop.
- `cookies.txt` is a live admin session. Delete it when you finish Part 1.
- The same steps work in a browser at `$HOURGLASS`.

### Step 7: Create the service account and its token

```sh
ACCOUNT=$(curl -sS -b cookies.txt -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
  -d '{"name":"lab-automation","scopes":["deployments:write","pressure:write","weights:write","state:read","holds:release"]}' \
  $HOURGLASS/api/v1/admin/service-accounts | jq -r .id)

(umask 077; curl -sS -b cookies.txt -H "X-CSRF-Token: $CSRF" -X POST \
  $HOURGLASS/api/v1/admin/service-accounts/$ACCOUNT/tokens | jq -r .token > service.token)
```

| Scope | Needed for |
| --- | --- |
| `deployments:write` | Reporting what is deployed |
| `pressure:write` | Reporting lab pressure |
| `weights:write` | Reporting weights. Only the weights route accepts it, and no other scope allows that route. Leave it out if you do not report weights |
| `state:read` | Reading setups and holds |
| `holds:release` | Releasing a hold after your automation tears a setup down |

- Give the account only the scopes your service uses.
- The token (`hgsa_`, the token id, then 64 hex characters) appears only in that one response. Hourglass keeps a hash and cannot show it again. If you lose it, issue another.
- An account can hold several tokens, so you can rotate without a gap: issue a new one, switch the service, then revoke the old one (Admin, Service accounts).
- A name that already exists is `409 conflict`: list the accounts with `GET /api/v1/admin/service-accounts` and use its id.

To add a setup later without the UI:

```sh
curl -sS -b cookies.txt -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
  -d '{"name":"lab-03","labels":{"size":"large"},"details":{},"enabled":true}' \
  $HOURGLASS/api/v1/admin/setups                   # 201; all four fields are required
```

### Step 8: Check the integration API

```sh
export HOURGLASS_TOKEN=$(cat service.token)
curl -sS $HOURGLASS/api/v1/integration/state -H "Authorization: Bearer $HOURGLASS_TOKEN" \
  | jq '{provider: .deployments.provider, pressure: .pressure.source, setups: [.setups[].name]}'
```

Expect `"provider": "reported"`, `"pressure": "reported"` and your setup names. Hourglass is now ready for your service.

### Step 9: Connect sign-in

Skip this only if the customer wants nothing but the local admin for now. Users cannot sign in without it.

1. In the identity provider, register a confidential client that uses the authorization code flow, with the redirect URI `https://hourglass.example.com/auth/callback` (the host name from step 2).
2. Add to `hourglass.env`, then run `docker compose up -d` again:

   ```sh
   HOURGLASS_OIDC_ISSUER=https://idp.example.com/realms/lab
   HOURGLASS_OIDC_CLIENT_ID=hourglass
   HOURGLASS_OIDC_CLIENT_SECRET=...
   HOURGLASS_OIDC_REDIRECT_URL=https://hourglass.example.com/auth/callback
   ```

3. Check: `curl -sS -o /dev/null -w '%{http_code}\n' $HOURGLASS/auth/oidc/start` prints `302`. A `500` means Hourglass could not read the issuer's discovery document: check the URL, and `./ca` if the issuer has a private certificate.
4. A person who signs in for the first time has no role and sees a not-authorized page. An admin gives them the `developer` role in Admin, Users. Roles live in Hourglass, not in the identity provider.

### Back up the database

Hourglass takes no backups of its own. Do not copy the file while the container runs. Take a consistent copy with `VACUUM INTO`, on a schedule, and move the copies off the host. The Hourglass image has no shell, so run `sqlite3` in a helper container that mounts the same volume. Any image that has `sqlite3` will do; use one the customer already trusts:

```sh
docker run --rm -v hourglass_data:/data -v /backups:/backups <image with sqlite3> \
  sqlite3 -readonly /data/hourglass.db ".timeout 10000" \
  "VACUUM INTO '/backups/hourglass-$(date +%Y%m%d-%H%M%S).db'"
```

The volume name is the compose project name, an underscore, then `data` (`docker volume ls` shows it). The target file must not exist. Check a copy with `sqlite3 FILE "PRAGMA integrity_check"`, which prints `ok`. Take a backup before every image upgrade: there are no down migrations, so the backup is the only way back.

## Part 2: Build the integration service

## What Part 1 gave you

- `$HOURGLASS`, the base URL, and (with a self-signed certificate) the certificate file your HTTP client must trust. In `curl` that is `--cacert` or `CURL_CA_BUNDLE`; in Python `requests` it is `verify="tls.crt"`; in Node it is `NODE_EXTRA_CA_CERTS`. Do not switch certificate checking off instead.
- A service token, in `service.token`. Move it to the secret store your service reads.
- The webhook secret, in `webhook.secret`. Your receiver needs it to verify signatures.
- Setups whose names your service will report.

If someone else installed Hourglass, ask them for these four things. The config your integration depends on is in [step 3](#step-3-write-the-config): the `reported` provider and pressure source, and a `webhook` hook that points at your receiver.

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

## Report weights

By default the price of a setup comes from the admin's rate rules and changes only when an admin edits a rule or a label. If the price should follow what a setup currently reserves, for example its provisioned memory and storage, and come down when the setup shrinks, your service can report a **weight** for each setup. Your service works the weight out; Hourglass applies it, within limits the admin sets, and never learns what it was computed from. This job is optional. Without it, nothing in this guide changes.

A charge is the weight times the pressure multiplier times the hours elapsed. A reported weight changes the weight and nothing else: it cannot move lab pressure or the multiplier.

### An admin turns it on first

The feature is off until an admin turns it on, and the route refuses every report until then. The switch and its limits are settings in Hourglass (Admin, Settings, "Reported weights"), not keys of the config file. They are changed through the admin API like this, signed in as in [step 6](#step-6-set-the-admin-password):

```sh
curl -sS -b cookies.txt -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' -X PUT \
  -d "$(curl -sS -b cookies.txt $HOURGLASS/api/v1/admin/settings \
        | jq '.settings | .reported_weights = {enabled: true, min: 0, max: 20, max_age: "10m"}')" \
  $HOURGLASS/api/v1/admin/settings | jq -c .settings.reported_weights
```

```json
{"enabled":true,"min":0,"max":20,"max_age":"10m"}
```

| Field | Default | Meaning |
| --- | --- | --- |
| `enabled` | `false` | The switch. It can be turned on only when `max` is set |
| `min` | `0` | The lowest weight a report may carry, inclusive |
| `max` | none | The highest weight a report may carry, inclusive. The admin chooses it: an upper bound that catches a unit mistake in your service |
| `max_age` | `10m` | How long a weights report applies after its `observed_at`. From `1m` to `24h` |

Turning the switch on without a `max` is refused: `enabled can be true only when max is set: set a maximum weight to turn reported weights on`. A change applies from the next poll and is not retroactive. Ask the admin for a `max_age` of at least twice the poll interval (see [If you stop reporting](#if-you-stop-reporting)).

### The request

```sh
curl -sS -X PUT "$HOURGLASS/api/v1/integration/weights" \
  -H "Authorization: Bearer $HOURGLASS_TOKEN" -H 'Content-Type: application/json' \
  -d '{
        "weights": [
          {"setup": "lab-03", "weight": 1.15, "attributes": {"memory-gb": "512", "storage-gb": "4000", "tier": "gold"}},
          {"setup": "lab-07", "weight": 0.8}
        ]
      }'
```

Response:

```json
{"observed_at": "2026-11-24T06:19:04.255Z", "received_at": "2026-11-24T06:19:04.255Z", "expires_at": "2026-11-24T06:29:04.255Z", "accepted": 2, "unknown": []}
```

`observed_at` is optional, in RFC 3339 UTC, and works as in the other two reports (see below). `expires_at` is `observed_at` plus the admin's `max_age`. `unknown` lists names that match no setup in Hourglass: they are checked like any other entry, stored and ignored, and a setup created later with that name takes the weight. A report with `{"setup": "lab-99", ...}` answers `"accepted": 2, "unknown": ["lab-99"]`.

Rules:

- **The weight should describe what the setup reserves, not live utilisation.** Report its provisioned size: the memory and storage it holds, not how busy it is. A weight that follows CPU load would make a user pay more for using the setup they hold. Hourglass cannot check this; it is yours to get right.
- **Send the complete set, from one service, every time.** The list replaces the stored report whole. A setup you leave out has no reported weight and falls back to the admin's rate rules. An empty list, `{"weights": []}`, is valid and means no setup has one. A missing or `null` `weights` is a `400`. Do not have two services each report part of the set: the last report wins and the other's setups fall back.
- **Send weights before deployments.** The two reports are separate calls and a poll can run between them. See the next rule for why the order matters at the end of a hold.
- **Keep reporting a setup's weight until its `hold` is `null` in the state.** A setup left out of the report is charged at the rate-rule price while it is held, including the last interval of a release: when a user releases, Hourglass charges the interval up to the poll that sees the setup gone, and if your service already dropped the setup from its weights report at teardown, that interval is charged at the rate-rule price. Stop reporting the weight only after the hold has ended.
- **If you cannot size a setup, leave it out. Never send 0 in its place.** A weight of 0 is valid, when the admin's minimum allows it, and means the setup is free; a setup you could not size is not free. Leaving it out sends it to the rate rules.
- **A weight is a number, not a string.** `1.15` is accepted; `"1.15"` is refused, as are `null`, `true`, and a number too large for a 64-bit float. The weight must be finite and between the admin's `min` and `max`, both inclusive. `-0` counts as 0. A value above `max` is refused whole, not clamped: the most likely cause is a bug in your service, and Hourglass does not charge the top price for a bug.
- **A report is accepted or refused as a whole.** One invalid entry refuses the report, nothing is stored, and the previous report stays in force until it expires.
- A weight may be reported for any setup, whether it is deployed or not, held or free, enabled or disabled. If you report only deployed setups, a free setup shows its rate-rule price in the Setups list and in the acquire dialog, and the price changes once it is deployed. To avoid that, report every setup you can size.
- `setup` is the Hourglass setup name, exactly, a non-empty string, not repeated in one report.
- An entry holds only `setup`, `weight` and `attributes`. Any other field in an entry is refused. (The top-level keys of the body are matched without regard to case, as in the other two reports; the keys inside an entry are strict.)
- `attributes` is optional: at most 16 entries, keys of 1 to 63 lowercase letters, digits, `.`, `_`, `/` or `-` that start and end with a letter or digit, and string values of at most 63 characters. Hourglass stores them, returns them and shows them next to the weight in the Setups list and the acquire dialog. It never interprets them, and no rate rule or filter matches them. They are shown only while the entry applies.
- A usable reported weight replaces the rate rules for that setup, even a rule of weight 0. The way to exempt a setup is the admin's per-setup weight override, which pins the setup's weight above both a report and the rate rules.
- **Ordering and clock skew** are the rules of the other reports: an `observed_at` older than the one Hourglass holds is `409 stale_report`, an equal one replaces it, and one more than 2 minutes ahead of Hourglass's clock is `400 validation`. Without `observed_at`, Hourglass uses the time it received the report. The weights report is ordered separately from the deployments report, so the two may carry the same `observed_at` or different ones.
- At most 10,000 entries and a 1 MB body. A report cannot be split across calls, so a large report with many attributes reaches the body limit first.

What Hourglass does with it:

- The report takes effect at the next poll and in every screen and API answer from then on. It never triggers a poll, a debit or an event.
- A poll charges the whole time since the previous poll at the weight it resolves at that moment. A weight reported one second before a poll prices that whole interval, and no earlier charge changes.
- Every charge records its weight and where it came from (`override`, `reported`, `rule`, `default` or, for a Go metering policy's weight, `policy`). Users see the weight on their holds and on each debit.
- A setup without a usable reported weight is charged at the weight it would have had without this feature, in every state of the hold, and the poll does not fail for that reason.

### What can be refused

Checks run in this order: your token's scope, then whether reported weights are on, then the body, then the ordering.

- **`409 not_configured`**: `reported weights are off: an admin turns them on with the reported_weights setting; the report was not stored`. An admin has not turned the setting on. This is not a change to the config file, and there is nothing to fix in your service: ask the admin. It is answered whatever the body, and nothing is stored.
- **`400 validation`** with `field` `weights`. The message always gives the entry's position, counted from 0, its setup name when the entry has a non-empty one, and the reason. The first invalid entry is reported. Real examples, each of which refused the whole report:

  ```
  weights[3] (setup "lab-03"): weight 25 is above the maximum 20
  weights[0] (setup "lab-03"): weight must be a JSON number, not a string
  weights[0] (setup "lab-03"): weight is required
  weights[0] (setup "lab-03"): weight -1 is below the minimum 0
  weights[0]: setup must be a non-empty string
  weights[1] (setup "lab-03"): setup is repeated (first at weights[0])
  weights[0] (setup "lab-03"): unknown field "memory" (an entry holds only setup, weight and attributes)
  weights[0] (setup "lab-03"): attributes "memory-gb" must be a string
  weights is required (an empty list means no setup has a reported weight)
  ```

  A body that is not valid JSON at all gets `request body is not valid JSON` and cannot name an entry. This includes a bare `NaN` or `Infinity`, which some JSON encoders write for an invalid number: encode your numbers so that this cannot happen, and check them before you send.
- **`409 stale_report`** and `400 validation` for `observed_at`, as for the other reports.
- **`403 forbidden`** if the token lacks `weights:write`. An account that holds every other scope is refused too.

**Alert on every `400` from this route.** It means your service sent a value Hourglass does not accept, and, unlike the other two reports, nothing stops charging to tell you: the previous report stays in force until `max_age` and then the prices quietly go back to the rate rules. An admin sees the last refused report, with its time, your account's name and the message, on the Settings page and in the collector view.

### Check that it was taken

The state route ([Reconcile](#reconcile)) tells your service what Hourglass applies now, so it can check that its report was taken. Read the setup's `weight`: `source` is `reported` and `applied` equals what you sent.

```sh
curl -sS "$HOURGLASS/api/v1/integration/state" -H "Authorization: Bearer $HOURGLASS_TOKEN" \
  | jq '{weights, setups: [.setups[] | {name, weight}]}'
```

```json
{
  "weights": {"enabled": true, "min": 0, "max": 20, "observed_at": "2026-11-24T06:18:46.000Z", "expires_at": "2026-11-24T06:28:46.000Z"},
  "setups": [
    {"name": "lab-03", "weight": {"applied": 1.15, "source": "reported", "reported": 1.15, "not_applied": null,
                                  "attributes": {"memory-gb": "512", "storage-gb": "4000", "tier": "gold"}}},
    {"name": "lab-07", "weight": {"applied": 0.8, "source": "reported", "reported": 0.8, "not_applied": null, "attributes": {}}},
    {"name": "lab-09", "weight": {"applied": 1, "source": "default", "reported": null, "not_applied": null, "attributes": {}}}
  ]
}
```

- `applied` and `source` are what Hourglass applies right now and what the next poll charges if nothing changes before it. `source` is `override`, `reported`, `rule` or `default`.
- `reported` is the weight in the stored report for the setup, or `null`. `not_applied` says why a reported weight is not applied: `expired` (the report is older than `max_age`), `out_of_bounds` (the admin's limits now exclude it) or `override` (the admin pinned this setup's weight). It is `null` when the weight applies or there is none.
- `attributes` is `{}` when there are none or the entry is not applied.
- At the top level, `weights` holds the switch, the limits (`max` is `null` when none is set), and the stored report's `observed_at` and `expires_at`. The two times are `null` before the first report and while the setting is off.
- With the setting off, `reported` and `not_applied` are `null` for every setup and `source` is `rule` or `default`.
- The rate rule that matched is not returned: it is admin configuration.

### What the admin can do to your weights

- **Tighten the limits.** A stored weight outside the new `min` or `max` stops applying at once, and that setup is charged at its rate-rule price (`not_applied` is `out_of_bounds`). Every later report that still holds such a weight is refused whole, so your service sees `400` until it reports weights inside the limits; after `max_age` every setup is back on the rate rules, with polls still succeeding. Real run, with the maximum lowered from 20 to 1 while `lab-03` was reported at 1.15: `lab-03` went to `{"applied": 2, "source": "rule", "reported": 1.15, "not_applied": "out_of_bounds"}`, and the same report was refused with `weights[0] (setup "lab-03"): weight 1.15 is above the maximum 1`.
- **Pin one setup.** A weight override beats a report. The state shows the override as `source` and `override` as `not_applied` for the setup's reported weight.
- **Turn the setting off.** Reports are then refused with `409 not_configured`. Nothing stored is deleted: if it is turned on again while the report is still within `max_age`, it applies again. Raising `max_age` makes an expired report apply again in the same way.

## If you stop reporting

Each report expires `max_age` after its `observed_at` (10 minutes by default; the response's `expires_at` tells you exactly when). Once either of those two reports that the config selects has expired (or before the first one has arrived):

- Every poll fails until a fresh report arrives, with an error that names `max_age`. `poll.failed` is sent.
- **Nobody is charged** for that time. Once reports resume, Hourglass records the gap as an outage and shows it to admins.
- No hold is marked deployed or released by a poll.

This is deliberate: stale data never bills anyone. It also means an outage of your service stops metering for the whole lab, so run it with a restart policy and alert on a failed report.

**The weights report is the exception.** An expired or missing weights report does not stop charging and does not fail a poll. The prices go back to the admin's rate rules (the state shows `not_applied: "expired"` and `source: "rule"` or `"default"`), polls keep succeeding, and every hold keeps being charged, at the rate-rule price. Nothing tells you this happened except the state, so check `weights.expires_at` in it, and alert on a `400` from the weights route. A real run with `max_age` of `1m`: 75 seconds after the last report, the state showed `lab-03` at `{"applied": 2, "source": "rule", "reported": 1.15, "not_applied": "expired"}`, and the next poll succeeded.

Keep the admin's `max_age` for weights at least twice the poll interval (so at least 10 minutes with the default 5-minute poll). With a shorter value, polls can alternate between the reported price and the rate-rule price without any error.

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

The response lists every setup, disabled ones included, with its labels, details, enabled flag and current hold (`null` when free), plus the reports Hourglass currently holds from you and when they expire, and the weight Hourglass applies to each setup. Trimmed example:

```json
{
  "server_time": "2026-11-20T10:16:02Z",
  "poll": {"interval_seconds": 300, "last_ok_at": "2026-11-20T10:15:00Z"},
  "pressure": {"source": "reported", "reported": {"value": 0.72, "observed_at": "...", "expires_at": "..."},
               "smoothed": 0.68, "multiplier": 1},
  "deployments": {"provider": "reported", "observed_at": "...", "expires_at": "..."},
  "weights": {"enabled": true, "min": 0, "max": 20, "observed_at": "...", "expires_at": "..."},
  "setups": [
    {"name": "lab-03", "labels": {"size": "large"}, "details": {"note": "sample"}, "enabled": true,
     "reported_deployed": true,
     "weight": {"applied": 1.15, "source": "reported", "reported": 1.15, "not_applied": null, "attributes": {"memory-gb": "512"}},
     "hold": {"id": 7, "user": {"id": 2, "username": "", "email": "alice@example.com", "name": "Alice Example"},
              "acquired_at": "...", "metering_started_at": "...", "deployed_at": "...", "release_requested_at": null}}
  ]
}
```

- `reported_deployed` says whether your latest stored report lists the setup. It is not what the last poll acted on: `hold.deployed_at` and `release_requested_at` are.
- `poll.last_ok_at` is `null` until the first successful poll after Hourglass starts. `pressure.smoothed` is `null` until then too. `pressure.reported`, `deployments.observed_at` and `deployments.expires_at` are `null` before your first report (and when that extension is not `reported`).
- `weights` and each setup's `weight` are described under [Report weights](#check-that-it-was-taken); `weights.observed_at` and `weights.expires_at` are `null` before the first weights report and while the setting is off.
- There are no balances, ledger entries, other users, audit data or settings in it, apart from the weights switch and limits.

At startup and every 5 minutes, compare it with your infrastructure and correct the difference:

| In Hourglass | In your infrastructure | Action |
| --- | --- | --- |
| Held, `release_requested_at` is null | Not deployed, no deployment running | Deploy it (you missed `hold.acquired`) |
| Held, `release_requested_at` is set | Deployed | Tear it down (you missed `hold.releasing`) |
| Free | Deployed | Tear it down, or leave it and let admins see the orphan, according to your policy |
| Held | Deployed | Nothing |

Also check `pressure.reported.expires_at` and `deployments.expires_at`. If either is null or in the past, your reporting loop is not working. If you report weights, check `weights.expires_at` too, and that each setup you reported shows `source: "reported"` with `applied` equal to what you sent. Unlike the other two, an expired weights report stops no charging: prices quietly go back to the rate rules.

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
5. You tear the setup down and send a deployments report without it. If you report weights, keep the setup in your weights report until the hold is gone from the state.
6. At the next poll Hourglass releases the hold, stops charging, and sends `hold.released`. That last poll's interval is still charged, because the hold was metered until the poll saw the setup gone.

Charging starts and stops on poll boundaries, so allow up to one poll interval between your report and its effect. A user's second click on Release changes nothing, and a pending release cannot be cancelled.

## Errors

Every error has the shape `{"error": {"code": "...", "message": "..."}}`.

| Status | Code | Meaning | What to do |
| --- | --- | --- | --- |
| 401 | `unauthenticated` | Missing, wrong, expired or revoked token, a disabled or deleted account, or a user's cookie instead of a token | Stop and alert. Do not retry in a loop |
| 403 | `forbidden` | The token lacks the scope for this route (`weights:write` for the weights route) | Ask the admin to add the scope |
| 404 | `not_found` | No such hold | Re-read the state |
| 409 | `stale_report` | Hourglass already holds a report with a later `observed_at` (an equal one is accepted) | Drop this report; send a fresh one on the next cycle |
| 409 | `not_configured` | The config does not select the `reported` provider or pressure source | Fix the Hourglass config; the message names the key |
| 409 | `not_configured` | On the weights route: reported weights are off. The message names the `reported_weights` setting | Ask an admin to turn it on (Admin, Settings). It is not a config file change |
| 409 | `conflict` | The hold is already released | Treat as success |
| 400 | `validation` | Bad body: not valid JSON, over 1 MB; `value` missing, not a number or outside 0 to 1; `deployments` missing or `null` (an empty list is fine); an empty or repeated setup name; more than 10,000 deployments or 32 labels in one; `observed_at` more than 2 minutes ahead; `reason` missing or over 500 characters. On the weights route: `weights` missing or `null`; an entry whose `weight` is missing, not a JSON number, below the admin's minimum or above the maximum, whose `setup` is empty, missing or repeated, whose `attributes` break their limits, or that holds a field other than `setup`, `weight` and `attributes`; more than 10,000 entries. For an entry, `field` is `weights` and `message` gives its position (from 0) and its setup name. `message` says which, and `field` names it for most | Fix the request. Do not retry unchanged |
| 429 | `rate_limited` | Too many failed authentication attempts from your address (5 in 15 minutes). The `Retry-After` header says how many seconds to wait. A request with a valid token is never refused for this | Back off, then check the token |
| 5xx | | Hourglass is unhealthy or restarting | Retry on the next cycle |

For network errors and 5xx, do not build a retry queue. Reports are full snapshots, so the next scheduled report supersedes anything you failed to send.

## Part 3: Verify and go live

### Verify end to end

Do this with a real person's account that has the `developer` role. The local admin cannot acquire a setup.

1. Start your service. Within a minute, the state response shows both reports with an `expires_at` in the future:

   ```sh
   curl -sS $HOURGLASS/api/v1/integration/state -H "Authorization: Bearer $HOURGLASS_TOKEN" \
     | jq '{deployments: .deployments.expires_at, pressure: .pressure.reported.expires_at, last_ok: .poll.last_ok_at}'
   ```

2. Wait for one poll (5 minutes by default). `last_ok` is no longer `null`, and `curl -sS $HOURGLASS/healthz` shows a number for `last_poll_age_seconds`.
3. Have the person acquire a setup. Your receiver gets `hold.acquired` with a valid signature, and your service deploys the setup.
4. After your next report and one poll, the hold in the state response has `deployed_at` and `metering_started_at` set.
5. Have the person release it. Your receiver gets `hold.releasing`; your service tears the setup down and reports without it. After one poll the hold is gone from the state response and you receive `hold.released`.
6. Stop your service for longer than `max_age`. Polls fail, `poll.failed` arrives, and your alert fires. Start it again and polls recover.
7. If you report weights: ask the admin to turn reported weights on with a maximum. Send a report and read the state back: each reported setup shows `source: "reported"` and `applied` equal to what you sent. Send a weight above the maximum and see the `400` with the setup's name (your alert fires and the previous weights stay in force). Stop only the weights report for longer than `max_age`: the state shows `not_applied: "expired"` and the rate-rule price, and polls keep succeeding.

### Go live

- **Start in shadow mode.** A new Hourglass has `enforcement` off: balances fall but nobody is blocked from acquiring. Leave it off for 2 to 4 weeks while the customer checks the rates, the pressure bands and the monthly allowance against real use. Turning it on is the customer's decision, in Admin, Settings.
- **Schedule the backup** from [Back up the database](#back-up-the-database) and test one restore.
- **Hand over the secrets:** the admin password, `hourglass.env`, the webhook secret and the certificate. Delete `cookies.txt` and `service.token` once the token is in the secret store.
- **Give users the certificate** if it is self-signed, or their browsers warn on every visit.
- **Renewing the certificate** means replacing the two files and restarting the container. Hourglass reads them only at start.
- **Upgrading** means taking a backup, then running `docker compose pull && docker compose up -d` to move to the newest release. Read the [upgrade notes](deploying.md#upgrade) first: a newer release can migrate the database, and then only the backup takes you back.

## Checklist

- [ ] Hourglass answers on HTTPS, its data is on a named volume, and a backup runs on a schedule.
- [ ] The admin password was changed and stored; `cookies.txt` and `service.token` are deleted.
- [ ] The token comes from a secret store and is never logged.
- [ ] The service checks Hourglass's certificate; verification is not switched off.
- [ ] Both reports are sent every 60 seconds from a loop that survives a single failure.
- [ ] A failed infrastructure query skips the report; it never sends an empty list or a zero.
- [ ] The deployments report always contains the full set.
- [ ] If you report weights: the weights report always contains the complete set from one service, is sent before the deployments report, and keeps a setup in it until its `hold` is `null` in the state.
- [ ] Weights are JSON numbers that describe what a setup reserves, not live utilisation. A setup you cannot size is left out, never sent as 0.
- [ ] An admin turned reported weights on with a minimum and a maximum. The service reads back `weights.expires_at` and each setup's `source` and `applied`, and alerts on a `400` from the weights route: an expired weights report stops no charging, it only puts prices back on the rate rules.
- [ ] Clocks are synchronised (NTP). `observed_at` more than 2 minutes ahead of Hourglass is rejected.
- [ ] The webhook receiver verifies the signature, replies within 30 seconds, and is idempotent per hold id.
- [ ] Reconciliation runs at startup and every 5 minutes.
- [ ] There is an alert on a failed report, on a 401, and on `poll.failed`.
- [ ] `enforcement` is still off unless the customer asked for it.
- [ ] The service account has only the scopes the service uses (`weights:write` only if it reports weights).
