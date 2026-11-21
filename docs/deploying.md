# Deploying Hourglass

This page is for the person who runs Hourglass: choosing a mode, building and loading images, configuring, connecting a login provider, connecting vCenter, rolling out, and keeping it running.

Hourglass is v1 and has been tested against a simulator, not a real vCenter. Run the [first-run checklist](#first-run-checklist-against-a-real-vcenter) before you rely on it. It is a single process with one SQLite file: there is no high availability.

## Bundled and split mode

| | Bundled (default) | Split |
| --- | --- | --- |
| Containers | `hourglass`: one process serves the API and the UI | `hourglass-api` (API only) and `hourglass-ui` (static files and a reverse proxy) |
| Config | `ui.serve: true` (the default) | `ui.serve: false` on the API (`HOURGLASS_UI_SERVE=false`), and `HOURGLASS_UI_API` on the UI container |
| What the browser talks to | The one container | The UI container only. It forwards `/api`, `/auth` and `/healthz` to the API |
| Choose it when | You want the simplest thing: one container, one port, one volume | You want the UI served by something separate from the process that holds the database, or you want to replace the UI without touching the API |

Both modes give the browser one origin, so the session cookie and CSRF token work the same way. You can move from bundled to split later by changing images and config; the database does not change.

In split mode:

- The API container must not be reachable from users except through the UI container. Anything that reaches the API port can use the API directly.
- `hourglass-ui` takes `-api URL` or `HOURGLASS_UI_API` (required, for example `http://hourglass-api:8080`), and `-listen ADDR` or `HOURGLASS_UI_LISTEN` (default `:8080`). A flag wins over its variable.
- To serve HTTPS from the UI container, give it `-tls-cert FILE` and `-tls-key FILE`, or `HOURGLASS_UI_TLS_CERT` and `HOURGLASS_UI_TLS_KEY` (see [TLS and cookies](#tls-and-cookies)). Leave the API container on plain HTTP inside the private network: the proxy trusts only public certificate authorities, so it cannot reach an API that serves a self-signed certificate.
- It proxies exactly `/api`, `/auth` and `/healthz` (each path and anything under it). Everything else is a static file or the single-page app's `index.html`.
- Its own health check is `/ui-healthz`. If the API is down it answers 502 with the standard JSON error shape.
- Set the OIDC redirect URL to the public URL of the UI container (`https://hourglass.example.com/auth/callback`).

A binary built with plain `go build ./...` has no UI. It is an API-only binary, equivalent to the `hourglass-api` image.

## Images

Images are built by Bazel with `rules_oci`; there is no Dockerfile. Each is built for `linux/amd64` and `linux/arm64`, on the `gcr.io/distroless/static-debian12:nonroot` base (pinned by digest in `MODULE.bazel`), and runs as user 65532 with `/data` as the working directory.

| Image | Contains | Port |
| --- | --- | --- |
| `hourglass` | `hourglassd` with the UI (bundled mode) | 8080 |
| `hourglass-api` | `hourglassd` without the UI | 8080 |
| `hourglass-ui` | Static UI and the reverse proxy | 8080 |
| `hourglass-e2e`, `hourglass-api-e2e` | Test builds with a clock control endpoint on 8091. Never deploy these | 8080, 8091 |
| `hourglass-vcsim` | A simulator that stands in for vCenter in the local stacks. Never deploy this | 8989, 8990 |

Build and load them into your local Docker daemon as `<name>:dev` (needs Bazel 7.4.1 and a running Docker):

```sh
deploy/load-images.sh                  # all six
deploy/load-images.sh hourglass        # only the named ones
bazel run //deploy/images:load_hourglass   # the same, one image
```

`bazel build //...` builds every image layer for both architectures without Docker.

### Publishing the images to a registry

The three product images each have a push target, which publishes both architectures as one multi-platform image. The test images have none. The default registry is `ghcr.io/panset` (`REGISTRY` in `deploy/images/BUILD.bazel`), giving `ghcr.io/panset/hourglass`, `ghcr.io/panset/hourglass-api` and `ghcr.io/panset/hourglass-ui`.

```sh
docker login ghcr.io                     # once; a token with the write:packages scope
deploy/push-images.sh v1.0.0             # all three, tagged v1.0.0
deploy/push-images.sh v1.0.0 latest      # several tags
bazel run //deploy/images:push_hourglass -- --tag v1.0.0   # one image
```

- Always name a tag. Without `--tag`, a push target publishes by digest only.
- A release does this for you: pushing a git tag such as `v1.2.0` runs `.github/workflows/release.yml`, which pushes all three images tagged `v1.2.0` and `latest`. A tag with a hyphen (`v1.2.0-rc1`) is a pre-release: it is pushed under its own name and `latest` does not move. The workflow does not run the tests, so run `./ci.sh` before you tag.
- To publish somewhere else, set `HOURGLASS_REGISTRY=registry.example.com/team` for the script, or pass `--repository registry.example.com/team/hourglass` to one target.
- The push reads credentials from the Docker config file. It does not need a running Docker daemon.
- On GitHub, a new package is private until you make it public in the package's settings.
- The binary reports its version from `internal/version`, not from the image tag.

Someone with only Docker can then run a published image, with no Bazel and no source tree:

```sh
docker run -d --name hourglass -p 8080:8080 -v hourglass-data:/data \
  -e HOURGLASS_ADMIN_PASSWORD="choose-a-long-password" \
  -e HOURGLASS_SESSION_KEY="$(head -c 32 /dev/urandom | base64)" \
  ghcr.io/panset/hourglass:v1.0.0
```

Run a product image on its own:

```sh
docker run -d --name hourglass -p 8080:8080 -v hourglass-data:/data \
  -e HOURGLASS_ADMIN_PASSWORD="choose-a-long-password" \
  -e HOURGLASS_SESSION_KEY="$(head -c 32 /dev/urandom | base64)" \
  hourglass:dev
```

Notes:

- The images have no shell and no `curl`. Probe `/healthz` from outside.
- `/data` exists in the image owned by 65532:65532, so a named volume works. A bind mount keeps the host directory's owner: make it writable by uid 65532 (`chown 65532:65532 dir`).
- Images carry the epoch as their build time, so `docker images` shows them as very old. That is the reproducible build, not a mistake.
- Mount a config file and set `HOURGLASS_CONFIG` to its path, or use only `HOURGLASS_*` variables. Without a provider, the metering policy and pressure source defaults apply.

## Configuration reference

Configuration is a YAML file plus environment variables. The file comes from `-config PATH`, else `HOURGLASS_CONFIG`; with neither, you get the defaults plus the environment. A named file that is missing is an error, and so is an unknown key anywhere in it. An environment variable overrides the file. An empty variable counts as unset, so an empty value in a compose file does not blank a setting from the file.

### Settings in the file or environment

| File key | Environment variable | Default | Meaning |
| --- | --- | --- | --- |
| `listen` | `HOURGLASS_LISTEN` | `:8080` | Address of the public listener. Plain HTTP unless `tls.cert` and `tls.key` are set |
| `tls.cert` | `HOURGLASS_TLS_CERT` | empty | Path of a PEM certificate file. With `tls.key`, the public listener serves HTTPS. See [TLS and cookies](#tls-and-cookies) |
| `tls.key` | `HOURGLASS_TLS_KEY` | empty | Path of the certificate's PEM private key file. Set both or neither |
| `database` | `HOURGLASS_DATABASE` | `hourglass.db` | Path of the SQLite file. The images set the working directory to `/data` |
| `session_key` | `HOURGLASS_SESSION_KEY` | empty | Signs CSRF tokens and the OIDC handshake cookie. At least 32 bytes. Empty means a random key at every start, so sessions do not survive a restart (a warning is logged). Always set it in production |
| `timezone` | `HOURGLASS_TIMEZONE` | `UTC` | IANA name. The month boundary (allowance, rollover) uses it. All stored times are UTC |
| `admin_password` | `HOURGLASS_ADMIN_PASSWORD` | empty | Initial password of the local `admin`. Read only when that account is first created. At least 12 characters. Empty: a password is generated and logged once |
| `oidc.issuer` | `HOURGLASS_OIDC_ISSUER` | empty | Issuer URL. Empty turns OIDC off and leaves only the local admin |
| `oidc.client_id` | `HOURGLASS_OIDC_CLIENT_ID` | empty | Required when `issuer` is set |
| `oidc.client_secret` | `HOURGLASS_OIDC_CLIENT_SECRET` | empty | The client secret |
| `oidc.redirect_url` | `HOURGLASS_OIDC_REDIRECT_URL` | empty | Required when `issuer` is set. `https://<public host>/auth/callback` |
| `provider` | none | none | `{type, config}`. Optional. See [Extensions](#extensions-in-the-config-file) |
| `pressure` | none | the binary's default (`inventory`) | `{type, config}` |
| `metering` | none | the binary's default (`default`) | `{type, config}` |
| `hooks` | none | `[]` | List of `{type, config}`, in delivery order |
| `seed` | none | empty | Path of the [seed file](#seed-file) |
| `ui.serve` | `HOURGLASS_UI_SERVE` | `true` | Serve the bundled UI at `/`. `false` for split mode. A boolean |
| `cors.allowed_origins` | `HOURGLASS_CORS_ALLOWED_ORIGINS` | empty | Comma-separated origins allowed credentialed CORS. Only for UI development from another origin |
| `metrics.listen` | `HOURGLASS_METRICS_LISTEN` | empty (off) | Address of the metrics listener. Must differ from `listen` |
| none | `HOURGLASS_CONFIG` | none | Path of the config file, when `-config` is not given |
| `provider.config.password` | `HOURGLASS_VSPHERE_PASSWORD` | none | The vSphere password, when the block has none |

Rules:

- Keep secrets out of the file. Use the environment, or a file readable only by the service account.
- Hourglass logs its configuration at start with `session_key`, `admin_password`, `oidc.client_secret` and every extension `config` block replaced by `[redacted]`.
- `provider`, `pressure`, `metering` and each hook allow only `type` and `config`.
- A `provider` block with a `config` but no `type` is an error.
- Changing `timezone` across a month boundary can move ledger entries between months, because each entry's month is taken from the configured zone. Change it just after a rollover, early in a month, so that no month boundary lies between the old and the new zone's time.

### Extensions in the config file

```yaml
provider:
  type: vsphere
  config:
    url: https://vcenter.example.com/sdk
    username: hourglass-ro
    # password: from HOURGLASS_VSPHERE_PASSWORD
pressure:
  type: vsphere
  config:
    url: https://vcenter.example.com/sdk
    username: hourglass-ro
metering:
  type: default
  config:
    start_on_allocation: true
    start_after: 2h
hooks: []
```

An unknown `type` fails startup and lists the registered names. Extensions are checked before the database is opened, so a typo does not leave a new database file behind. Config keys of each bundled extension are in [Extending Hourglass](extending.md#bundled-extensions).

With no `provider`, Hourglass is fully functional with `inventory` pressure and time-based metering.

### Settings stored in the database

An admin edits these in the UI (Admin, Settings), or with `GET` and `PUT /api/v1/admin/settings`. They apply from the next poll and are never retroactive. Every change is written to the audit log.

| Key | Default | Meaning |
| --- | --- | --- |
| `default_weight` | `1` | Weight of a setup that no rate rule matches |
| `display_label_keys` | `[]` | Label keys shown as columns and filters on the Setups page |
| `bands` | base `0`; from 0.50 give 1, from 0.75 give 2, from 0.90 give 3 | Pressure bands and multipliers. Lower bounds are inclusive |
| `default_allowance` | `300` | Tokens credited at the start of each month and when a user is first given a role |
| `max_holds_per_user` | `2` | Lowering it never releases existing holds |
| `release_requires_teardown` | `true` | A user's release of a setup the provider still reports is accepted but pending (the hold shows Releasing and keeps charging); it completes at the first poll where the setup is no longer reported. No effect without a provider |
| `poll_interval` | `5m` | Minimum `1m` |
| `enforcement` | `false` | `false` is shadow mode: balances fall but nobody is blocked |

Rate rules (an ordered list of selector and weight) and the setups themselves are also stored in the database. `PUT /api/v1/admin/settings` takes the whole object: fetch it with `GET`, change a key, and send it back.

## Sign-in: OIDC

Hourglass works with any OIDC provider that supports discovery and the authorization code flow. Nothing in it is specific to one provider.

1. Register a **confidential** client with your provider.
2. Set the redirect URI to `https://<public host>/auth/callback` (in split mode, the host of the UI container).
3. Allow the scopes `openid`, `email` and `profile`. Hourglass uses PKCE (`S256`) as well as the client secret.
4. Set the issuer, client id, client secret and redirect URL:

   ```sh
   HOURGLASS_OIDC_ISSUER=https://idp.example.com/realms/lab
   HOURGLASS_OIDC_CLIENT_ID=hourglass
   HOURGLASS_OIDC_CLIENT_SECRET=...
   HOURGLASS_OIDC_REDIRECT_URL=https://hourglass.example.com/auth/callback
   ```

5. Hourglass finds the endpoints through `<issuer>/.well-known/openid-configuration`. It contacts the issuer on the first sign-in attempt, not at start-up, so a wrong issuer does not stop the process. Check with a browser, or run `curl -si https://<host>/auth/oidc/start`: a `302` to your provider means discovery worked, and a `500` means it did not (see the log).

A user is identified by the `sub` claim. `email` and `name` are stored for display and refreshed at every sign-in. The ID token must verify against the issuer's keys and carry the nonce Hourglass sent.

What the first sign-in looks like:

1. The user opens Hourglass and chooses Sign in with SSO. They sign in at your provider.
2. Hourglass creates them with no role and status `pending`, and shows one page: "You are not authorized to use Hourglass. Reach out to your admin." Every data route of the API returns 403 for them.
3. An admin opens Admin, Users. Pending requests are listed oldest first with name, email and first sign-in time, and the navigation shows the count. The admin assigns `developer` or `admin` to approve, or rejects. A rejected user keeps seeing the same page and can be approved later.
4. The user reloads and lands on My usage (`/usage`). The monthly allowance is credited when the first role is assigned, not at sign-in.

Signing out of Hourglass ends only the Hourglass session. It does not sign the user out of the identity provider, so a second sign-in in the same browser can be silent and use the same identity. To test with several users, use separate private windows.

Roles live in Hourglass, not in the identity provider.

## Local admin account

The first start creates a local account, `admin`, with the `admin` role. It is the only account that does not come from OIDC. It is the operator account: it has no token balance and cannot acquire setups. It cannot be disabled, deleted, renamed or demoted.

- **Where the password comes from.** `HOURGLASS_ADMIN_PASSWORD` (or `admin_password`) when set, at least 12 characters. Otherwise Hourglass generates 24 characters and writes them once to the log as a `WARN` line with the field `password`. That is the one deliberate secret in the logs. Read it from the first start's log and do not keep the log.
- **Forced change.** The account must change its password at first sign-in. Until it does, the API's data routes answer `password_change_required`. The minimum is 12 characters.
- **Later starts.** The environment variable is ignored once the account exists. To change a lost password you must act on the database; there is no reset command in v1.
- **Rate limit.** After 5 failed sign-ins within 15 minutes, further attempts are refused (HTTP 429, `rate_limited`) until the oldest failure is 15 minutes old. The counter is in memory, belongs to the account rather than to a client address, and resets when the process restarts.
- **When OIDC is off** (no issuer), this is the only way in.

The password is stored as an argon2id hash.

## TLS and cookies

Users must reach Hourglass over HTTPS. There are two ways:

- **Hourglass serves HTTPS itself.** Give it a certificate and key, as below. This is the simplest way to run one container with a self-signed certificate.
- **Something in front terminates TLS.** Leave the two settings empty; Hourglass listens on plain HTTP behind a reverse proxy or load balancer.

### Serving HTTPS with your own certificate

Set `HOURGLASS_TLS_CERT` and `HOURGLASS_TLS_KEY` (or `tls.cert` and `tls.key`) to the paths of a PEM certificate and its private key. Both or neither: one alone stops the start. For a self-signed certificate:

```sh
openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
  -keyout tls.key -out tls.crt -subj "/CN=hourglass.example.com" \
  -addext "subjectAltName=DNS:hourglass.example.com"
chown 65532:65532 tls.key        # the container runs as uid 65532; keep the key 0600

docker run -d --name hourglass -p 443:8443 -v hourglass-data:/data \
  -v "$PWD/tls.crt:/certs/tls.crt:ro" -v "$PWD/tls.key:/certs/tls.key:ro" \
  -e HOURGLASS_LISTEN=:8443 \
  -e HOURGLASS_TLS_CERT=/certs/tls.crt -e HOURGLASS_TLS_KEY=/certs/tls.key \
  -e HOURGLASS_ADMIN_PASSWORD="choose-a-long-password" \
  -e HOURGLASS_SESSION_KEY="$(head -c 32 /dev/urandom | base64)" \
  ghcr.io/panset/hourglass:v1.0.0

curl --cacert tls.crt https://hourglass.example.com/healthz
```

- The certificate's subject alternative name must be the host name (or IP address) that users type. Browsers ignore the common name.
- With a certificate signed by an authority, put the leaf first in the certificate file, then the intermediates.
- The key file must be readable by uid 65532. A key that only root can read stops the start with `permission denied`.
- The image runs without root, so it cannot listen on a port below 1024 inside the container. Listen on a high port and publish it as 443.
- The files are read once, at start. Restart the container after you renew the certificate.
- A wrong path, or a key that does not belong to the certificate, stops the start before the database is opened. The start-up log line `http listening` carries `"tls":true`.
- The listener then serves only HTTPS, with TLS 1.2 as the minimum; plain HTTP on that port gets `400`. There is no redirect from port 80.
- Everything that calls Hourglass must trust a self-signed certificate: each browser (or it warns on every visit), every script that uses the integration API, and any health check (`curl --cacert`).
- The metrics listener (`metrics.listen`) stays plain HTTP.
- Set `oidc.redirect_url` to the `https://` address.

In split mode the same two settings exist for the UI container, as `HOURGLASS_UI_TLS_CERT` and `HOURGLASS_UI_TLS_KEY`; put the certificate there and not on the API container.

### Cookies

The session cookie (`hourglass_session`) is always `Secure`, `HttpOnly` and `SameSite=Lax`, with an 8-hour idle timeout. Browsers send a `Secure` cookie only over HTTPS, with one exception: they accept it on `http://localhost`. So:

- Over a plain-HTTP address that is not `localhost`, sign-in appears to succeed and the next request is unauthenticated, because the browser drops the cookie. Use HTTPS.
- On `localhost`, plain HTTP works. The local stacks use it.

A reverse proxy in front must pass `/` through unchanged, including the `Set-Cookie` header and `Location` redirects, and must not rewrite the `Host`-dependent paths. Hourglass does not read `X-Forwarded-*` headers. Set `oidc.redirect_url` to the public HTTPS URL, because that is the only place the public address is configured.

State-changing requests must carry the session's CSRF token in an `X-CSRF-Token` header. The UI does this; a script gets the token from `GET /api/v1/session`.

## The vSphere provider

The `vsphere` provider and the `vsphere` pressure source read from one vCenter with a read-only account. They never create, change, move or delete anything. Configuration keys are in [Extending Hourglass](extending.md#bundled-extensions). A reference setup, with setups as resource pools named `<setup>_RP`:

```yaml
provider:
  type: vsphere
  config:
    url: https://vcenter.example.com/sdk
    username: hourglass-ro@vsphere.local
    clusters: [prod-cluster-1]
    pool_name_suffix: _RP      # pool lab-06_RP is reported as setup lab-06
pressure:
  type: vsphere
  config:
    url: https://vcenter.example.com/sdk
    username: hourglass-ro@vsphere.local
    clusters: [prod-cluster-1]
```

Set the password with `HOURGLASS_VSPHERE_PASSWORD`. Hourglass matches a reported allocation only to the setup with exactly its name, so the provider maps pool names to setup names: with `pool_name_suffix: _RP` it reports only pools ending in `_RP`, with the suffix stripped (`pool_name_prefix` works the same way at the start of the name). The pool's own name stays in the allocation's `pool` label. Leave `insecure_skip_verify` at `false` against a real vCenter. Hourglass uses Go's default trust store, so a vCenter signed by a private certificate authority needs that authority trusted where Hourglass runs (this is untested). Give both blocks the same connection values so that they share one vCenter session; the naming keys do not matter to the pressure source, which accepts and ignores them.

### Read-only role and privileges

Create a role from vCenter's built-in Read-only role and assign it to the Hourglass account, with "propagate to children", on the datacenter or vCenter root that holds the clusters you list. The adapter makes these calls:

| Adapter call | vSphere API | Privilege |
| --- | --- | --- |
| Connect | RetrieveServiceContent | none |
| Sign in and out | SessionManager.Login and Logout | none (any valid user) |
| Find datacenters and clusters | Folder `childEntity` traversal, SearchIndex lookups | `System.View` on the root folder, propagated |
| Read a cluster's name, resource pools and datastores | PropertyCollector | `System.Read` on each cluster in scope |
| Walk resource pools | PropertyCollector | `System.Read` on each pool |
| Read the cluster root pool's CPU and memory runtime | PropertyCollector | `System.Read` on the root pool |
| Read each datastore's name and summary | PropertyCollector | `System.Read` on each datastore |

The built-in Read-only role (`System.Anonymous`, `System.View`, `System.Read`) covers these. This table comes from reading the code and the vSphere documentation. No real vCenter has confirmed it: that is checklist item 1 below.

### What the pressure source measures

For each cluster in scope, the highest of three ratios: the root resource pool's CPU `reservationUsed` over `maxUsage`, the same for memory, and one minus free over total space across the cluster's accessible datastores. The result is the highest across the clusters. This reads "allocation" as reservations, not live usage. A cluster with zero capacity is an error and fails the poll.

### Limits you should know about

- Clusters are found only as direct children of a datacenter's host folder. A cluster inside a host sub-folder is not discovered.
- If `datacenter` or `clusters` names something that does not exist, both the provider and the pressure source fail the poll with an error naming it (`datacenter "x" not found`, `cluster "y" not found`); the provider no longer lists nothing. Run `check-provider` after any change to those keys.
- A standalone host's pools (not in a cluster) are ignored.
- The vCenter session is never logged out at shutdown. It ends at vCenter's idle timeout.

### First-run checklist against a real vCenter

Mocks cannot confirm these. Work through them before enforcement is turned on, with `hourglassd check-provider -config your.yaml` (see [Check a provider](extending.md#check-a-provider)), which writes nothing.

- [ ] The read-only vCenter role has every privilege the provider needs. Confirm in particular that the `runtime` property of each cluster's root resource pool is readable with that role.
- [ ] The real resource pool layout (folders, nesting, naming) matches the setup names after `pool_name_prefix` and `pool_name_suffix` are stripped. The `matched setups` and `enabled setups not reported` sections tell you.
- [ ] The pressure source's cluster numbers agree with what vCenter shows. Compare `raw pressure` with the cluster's reservation and datastore figures.
- [ ] TLS and certificate settings work against the real endpoint, with `insecure_skip_verify` left `false`.
- [ ] A poll finishes well inside the poll interval at real inventory size. The collector page shows each run's start and finish, and `hourglass_poll_duration_seconds` records the last one.

`check-provider` opens an existing database read-only. That can leave empty `-wal` and `-shm` files beside it, and fails if the database directory is read-only. If the schema is older or newer than the binary, it fails and asks you to start the server once.

## Connecting your infrastructure with the integration API

If your lab is not vCenter, or you would rather not give Hourglass credentials to it, your own service can tell Hourglass what is deployed and how contended the lab is. Hourglass never reads your infrastructure; the service calls Hourglass over HTTP. [Setting up Hourglass and integrating your infrastructure](integrating.md) is the guide to give the engineer or AI agent who installs Hourglass and builds that service. This section is what you, the operator, set up.

### Config

Select the `reported` provider and pressure source. They take one key, `max_age`:

```yaml
provider:
  type: reported
  config:
    max_age: 10m
pressure:
  type: reported
  config:
    max_age: 10m
```

`max_age` is a Go duration, default `10m`, minimum `1m`. A report is stale once it is older than `max_age`, counted from the `observed_at` the service sent. While either selected report is missing or stale, every poll fails with an error that names `max_age`, nobody is charged for that time, and the gap is recorded as an outage (see [Roll out in shadow mode](#roll-out-in-shadow-mode) for how outages are shown). You can select just one of the two; `provider` and `pressure` are independent. A report sent to the integration API while its extension is not selected is refused with 409 `not_configured`.

Match `max_age` to the poll interval and to how often the service reports. The service should report every 60 seconds, the poll runs every `poll_interval` (5 minutes by default, 1 minute at the least), and each poll needs a report younger than `max_age`. So `max_age` should be at least twice the poll interval and well above the reporting period, or one late report fails a poll. The default 10 minutes suits the default 5-minute poll if the service reports every minute. If you lower `poll_interval`, `max_age` can stay at 10 minutes; if you raise it above 5 minutes, raise `max_age` with it. A shorter `max_age` finds a dead reporter sooner but fails polls on short hiccups; a longer one lets old data stand in for longer. Check the result before relying on it with `hourglassd check-provider`, which prints the age of each report.

### Service accounts

A service account is the identity of your service. It is a separate kind of principal, never a user and never an admin or developer, and its token works only under `/api/v1/integration/`.

1. In Admin, Service accounts, create an account. The name is 1 to 64 letters, digits, `.`, `_` or `-`, and cannot be changed. Give it only the scopes the service needs: `deployments:write`, `pressure:write`, `state:read`, `holds:release`. (`GET` and `PUT /api/v1/admin/service-accounts` do the same from a script.)
2. Issue a token. It looks like `hgsa_<id>_<64 hex characters>` and is shown once. Only its hash is stored. Put it in the service's secret store.
3. To rotate: issue a second token for the same account, switch the service to it, check that the first token's last-used time in the UI stops moving, then revoke the first. An account can have several live tokens at once, so there is no gap. Tokens can have an expiry; give them one if your secret manager can rotate them.
4. To cut a service off at once, disable the account (every token stops working) or revoke the one token.

Every create, change, token issue and revoke is audited, and so is each hold a service releases, under the account's name. A token that is revoked, expired or belongs to a disabled or deleted account answers 401.

### Behind a reverse proxy

Failed token attempts are rate-limited per client address: 5 failures in 15 minutes, then 429 `rate_limited` with `Retry-After` for further bad tokens from that address. Hourglass uses the address of the TCP connection and does not read `X-Forwarded-For`. Behind a reverse proxy every caller therefore shares the proxy's address and one failure budget. Nobody who presents a valid token is ever refused by the limit, so the service keeps working while someone else's bad requests use up the budget; they just see 429 instead of 401. Do not try to restore client addresses with a forwarding header: Hourglass would trust whatever the client sends. If you want the service to bypass the proxy, let it reach Hourglass directly on a private address.

### Releasing and the teardown setting

See [Upgrade](#upgrade) for what changed. With `release_requires_teardown` on (the default) and a provider selected, a user's release of a setup the last poll still reported as deployed is accepted and pending. With it off, or with no provider, release is immediate.

## Seed file

A seed file creates the first setups and rate rules, once. Name it in `seed:`. It is imported only on the first start of a database, which is recorded in a marker (the `seed_state` row of the settings table, written in the same transaction); once the marker exists it is never applied again, even if every setup is later deleted, and Hourglass does not read the file, so edit setups and rules in the UI. A database that already has setups, rules or users other than the local admin and no marker is treated as initialised.

```yaml
default_weight: 1            # optional
setups:
  - name: lab-01
    labels: {size: medium}
    details: {note: "free-form JSON"}
    enabled: true            # optional, default true
rate_rules:
  - {selector: "size=large", weight: 2}
```

A `.json` file is read as JSON, anything else as YAML. Unknown keys are rejected; a seed written for an earlier version that still has a per-setup `resource_name_override` fails with a message saying the key is no longer supported. The seed is validated like the admin API: unique non-empty names, labels within the limits (at most 32 per setup, 63-character values), `details` valid JSON of at most 256 KB, a valid selector, finite non-negative weights. An invalid seed stops startup and writes nothing. `deploy/seed.sample.yaml` is the reference scenario's seed.

One setting is not seedable and an admin sets it on first run: the display label keys.

## Roll out in shadow mode

A fresh Hourglass has `enforcement` off. Metering is on, debits are written and balances fall, but the balance check never blocks an acquire. Run like this for 2 to 4 weeks:

1. Register the setups, the rate rules and the provider. Run `check-provider` and settle the checklist.
2. Let people use the lab. Check the collector page for failed polls and outages.
3. Calibrate against the recorded data: the rate rules, the bands, the default allowance and `start_after`. Set the allowance tight enough that a typical user ends the month near zero; use grants for exceptions.
4. In Admin, Settings, turn `enforcement` on. From the next poll, a user at zero or below cannot acquire. Holds they already have keep metering; Hourglass never tears anything down.

If the provider is unreachable, nobody is charged for the gap and the gap is recorded as an outage. A gap longer than twice the poll interval is not charged at all.

## Backup and restore

The database is one SQLite file in WAL mode, and the Hourglass process is its only writer. Do not copy the file while the process runs. Take a consistent online copy with `VACUUM INTO`, which reads the live database and writes a new file:

```sh
sqlite3 -readonly /data/hourglass.db ".timeout 10000" \
  "VACUUM INTO '/backups/hourglass-$(date +%Y%m%d-%H%M%S).db'"
```

The target file must not exist, so the timestamp is part of the name. The copy is a single file with no `-wal` or `-shm`. Check it with `sqlite3 FILE "PRAGMA integrity_check"`, which prints `ok`. Run it from cron or a systemd timer, and keep copies off the host. In a container, run `sqlite3` where the file is visible: on the host for a bind mount, or in a helper container that mounts the same volume (the product images have no shell).

To restore, stop Hourglass, replace the database file with the copy, delete any `hourglass.db-wal` and `hourglass.db-shm` beside it, and start again.

## Metrics

Set `metrics.listen` to a separate address, for example `127.0.0.1:9090`. It is off by default. Metrics are never served on the public port: they would reveal usage to people with no role. Keep this listener on an internal network.

```sh
curl -s http://127.0.0.1:9090/metrics
```

| Metric | Type | Meaning |
| --- | --- | --- |
| `hourglass_poll_duration_seconds` | gauge | Duration of the most recent finished poll |
| `hourglass_poll_failures_total` | counter | Polls that failed since the process started |
| `hourglass_lab_pressure{kind="raw"}` and `{kind="smoothed"}` | gauge | Pressure of the last successful poll, 0 to 1. Absent until a poll succeeds |
| `hourglass_tokens_debited_total` | counter | Tokens debited since the process started |

Counters reset at restart. The format is Prometheus text exposition 0.0.4.

## Logs and health

Logs are JSON, one object per line, on standard output. They carry `time`, `level`, `msg` and fields, including one `request` line per HTTP request with `method`, `path` (no query string), `status` and `duration_ms`. Secrets are never logged, with one exception: the generated local admin password, once.

`GET /healthz` is public and returns, for example, `{"status":"ok","database":"ok","last_poll_age_seconds":1}`. It answers 503 only when the database query fails. A stale poll does not change the status; watch `last_poll_age_seconds` or `hourglass_poll_failures_total`.

## Upgrade

Schema changes ship as numbered SQL migrations embedded in the binary and applied at start, each in a transaction. To upgrade, back up, stop, start the new binary, and check `/healthz`.

- There are no down migrations. To go back, restore the backup taken before the upgrade.
- A database whose schema is newer than the binary is refused at start (`database schema version N is newer than this binary`).
- A restart loses at most one poll, which is not charged.

### Upgrading from a build with the resource name template

Earlier builds had a resource name template setting and a per-setup resource name override. Migration 0004 removes both. A setup now matches the provider's allocation with exactly its own name. Before upgrading, set `pool_name_prefix` / `pool_name_suffix` in the `vsphere` provider block (or the equivalent in your own provider) so that what the provider reports equals your setup names. For example, a template of `{{.SetupName}}_RP` becomes `pool_name_suffix: _RP`. Otherwise setups stop matching what the provider reports: nothing deploys and a release is never pending. When the migration removes a non-default template or any override, the server logs one warning at start with the number of setups that had overrides (a template is shown at most 100 characters).

### Upgrading to the integration API (migration 0005)

Migration 0005 adds the service account, token and report tables and three columns (`holds.release_requested_at`, `holds.released_by_service`, `audit_log.actor_service`). It has no down migration. As with every upgrade, take a backup first and keep it: once 0005 has run, an older binary refuses the database (`database schema version N is newer than this binary`), and going back means restoring that backup. No configuration change is needed. Nothing changes for an installation that does not create a service account or select `reported`, except the release behaviour below.

**Release behaviour changed.** A user's release of a setup that the provider still reports (with `release_requires_teardown` on) used to be refused with 409 `still_deployed`; the user had to tear the setup down, wait a poll and click again. Now the first click is accepted (202): the hold is marked Releasing, keeps metering, fires the new `hold.releasing` event, and ends at the first successful poll in which the setup is no longer reported. That poll's interval is still charged. A pending release cannot be cancelled, and clicking again changes nothing. This applies to every provider, `vsphere` included. The `still_deployed` error code no longer exists, so scripts that matched it must stop. If no poll has succeeded since the process started, the release is immediate, as before. Hooks that treated `hold.released` as the only end-of-hold signal keep working; `hold.releasing` is new and optional.

## Data retention

| Data | Kept |
| --- | --- |
| Usage samples | 13 months |
| Event deliveries | 90 days |
| Expired sessions | Deleted daily and at each sign-in |
| Ledger, audit log, outages, pressure samples, collector runs | Kept (not pruned in v1) |

Pruning runs once a day, from the collector.

## Known limits

- One instance. One process owns the database; there is no high availability, and running two against one file is not supported.
- TLS is optional and static: one certificate, read at start, no reload and no automatic issuing (ACME). In split mode the UI container cannot reach an API that serves a self-signed certificate.
- The event queue is in memory: events still queued at shutdown are lost, and nothing is retried.
- The local admin password cannot be reset without database access.
- vSphere: clusters in host sub-folders are not discovered, and nothing has been run against a real vCenter.
- Images are built and smoke-tested on arm64. The amd64 `hourglass` image has only been started under emulation; the other amd64 images have not been run.
- The release workflow publishes the images on a `v*` git tag without running the tests. There is no Helm chart and no Dockerfile.

## Local stacks

`deploy/` holds compose files for trying Hourglass with Keycloak and a simulator standing in for vCenter. Every secret in them is a fake development value. Do not use them as a production template. The [README](../README.md) shows how to start one, and `deploy/README.md` lists ports and test users.
