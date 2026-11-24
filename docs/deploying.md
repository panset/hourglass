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
| `preapproved_users_file` | `HOURGLASS_PREAPPROVED_USERS_FILE` | empty | Path of the optional [pre-approved users file](#pre-approved-users-file): SSO users listed in it get a role at their first sign-in. Read once at start |
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
- The ledger roll-up (see [Data retention](#data-retention)) also cuts days in the configured zone, and it does so when it runs: a day is cut in the timezone that is configured at that moment, and a compacted day is never cut again. If you change `timezone`, days already rolled up keep the boundaries of the old zone, and only days not yet rolled up follow the new one. The per-poll rows of the last week are not affected until they are compacted.

### Pre-approved users file

`preapproved_users_file` (environment `HOURGLASS_PREAPPROVED_USERS_FILE`) names an optional file that lists SSO users to approve at their first sign-in. It is empty by default, and with no file and no entries added in the admin page, sign-in behaves as before. The path is shown in the logged and redacted configuration; the file holds no secret. The whole guide to using it is under [Pre-approving SSO users](#pre-approving-sso-users). The file itself:

```yaml
users:
  - email: alice@example.com
    roles: [developer]
  - email: bob@example.com        # roles default to [developer]
```

- A path ending in `.json` is read as JSON, anything else as YAML.
- It is decoded strictly, like the seed file, and a little more strictly than the seed: it must be one document, a key may not repeat, and keys must be spelled exactly (`users`, `email`, `roles`; `Email` is an error). An unknown key is an error. An empty file is valid and holds no entries.
- Limits: at most 10,000 entries. Each email follows the rules of the admin route (at most 254 characters after trimming, exactly one `@` with text on both sides, no whitespace, comma or `*`, no control or invisible character). A role set must hold only `developer` and `admin`; an entry with no `roles` key gets `[developer]`, and an empty `roles` list is an error. The same email twice is an error. Emails are compared after trimming and lower-casing ASCII letters.
- The file is read once, at start, before the database is opened. An unreadable or invalid file stops the start with a message that names the entry by position (`users[3]`) and never repeats a valid address. A change takes effect at the next restart; to add someone without a restart, use the admin page.
- The file is never written to and never copied into the database. Remove an entry and restart, and the pre-approval is gone; people it already approved keep their role.
- The log line at start is `pre-approved users file read` with the path and the number of entries, never the addresses.

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
| `reported_weights` | off, minimum `0`, no maximum, `max_age` `10m` | Lets a customer's service report a weight per setup. An object of four fields, see [Reported weights](#reported-weights) |

Rate rules (an ordered list of selector and weight) and the setups themselves are also stored in the database. `PUT /api/v1/admin/settings` takes the whole object: fetch it with `GET`, change a key, and send it back. The one exception is `reported_weights`: a `PUT` that does not carry it leaves the stored value unchanged, so a script written for an earlier release cannot switch the feature off by accident.

### Reported weights

By default a setup's weight, and so its price, comes from the rate rules and changes only when an admin edits a rule or a label. With `reported_weights` on, a service account that holds the `weights:write` scope can report a weight per setup (`PUT /api/v1/integration/weights`, described in [the integration guide](integrating.md#report-weights)), and Hourglass applies it within limits you set. The feature is **off by default**, and it is a setting in the database, not a key of the config file: turning it on changes who sets prices, so it is audited like every other setting (before and after value, the whole object).

| Field | Default | Rule |
| --- | --- | --- |
| `enabled` | `false` | Can be `true` only when `max` is set |
| `min` | `0` | The lowest weight a report may carry. Finite, at least 0, and at most `max` when `max` is set. A minimum above 0 means no setup can be reported free |
| `max` | none | The highest weight a report may carry. Null, or above 0 and at most 1e12. There is no default: you choose the ceiling |
| `max_age` | `10m` | How long a report applies after its `observed_at`. A duration from `1m` to `24h` |

When the key is present in a `PUT`, all four fields are required. `GET` always returns it. Both limits are inclusive. A change applies from the next poll and the next read, and is not retroactive.

What it does, in short:

- A usable reported weight replaces the rate rules for that setup, including a rule of weight 0, and every charge, balance and screen uses it. A setup with no usable reported weight (none sent, left out of the latest report, expired, outside the limits, or the feature off) is priced as before: its override, else its rate rule, else the default weight. A poll never fails for that reason.
- A weight outside the limits is **refused**, never clamped, and a report is accepted or refused whole. The previous report stays in force until it expires.
- A reported weight changes the charge and nothing else. The snapshot that the pressure source and the metering policy receive keeps the admin's weight, so a weights reporter cannot move lab pressure or the multiplier.
- Turning the setting off deletes nothing: the stored report is ignored, and applies again if the setting is turned back on while it is still within `max_age`. Raising `max_age` makes an expired report apply again in the same way.

**Choosing `max_age`.** It should be at least twice the poll interval, and well above the period at which the service reports (every 60 seconds), as for the `reported` provider and pressure source. With a shorter value, polls can alternate between the reported price and the rate-rule price without any error. Unlike those two reports, an expired weights report does not fail polls and does not stop charging: every hold is charged at its rate-rule price until the next report, and Admin, Setup inventory and the state route show that (`not_applied` is `expired`). A weights reporter that is down for longer than `max_age` therefore changes prices without any alarm; raise `max_age` (up to 24 hours) if you would rather keep the last reported weights through a longer outage, and have the service alert on a `400` from the weights route. Admin, Settings shows when the last weights report arrived and from which account.

**What tightening the limits does.** Because a report is refused whole and never clamped:

- A stored weight outside the new limits stops applying at once, from the next poll and the next read; that setup is charged at its rate-rule price.
- Every later report that still contains such a weight is refused whole, with a `400` that names the setup. After `max_age`, every setup is back on its rate-rule price, with polls succeeding.
- You see both on Admin, Settings: the number of stored weights the limits exclude at once, and the last refused report (time, account, message) as it arrives. The same two are in `last_weights_report` and `last_weights_refusal` of `GET /api/v1/admin/collector`; they are null while the feature is off. The last refusal is kept in memory and is lost at a restart.

**Per-setup override.** A setup can carry a weight override (Admin, Setup inventory, "Weight override", or `weight_override` in `POST` and `PUT /api/v1/admin/setups`). It pins that setup's weight above a reported weight and above the rate rules, whether reported weights are on or off, and is not bound by the reported minimum and maximum. It pins the weight, not the cost: the pressure multiplier still applies. Use it to exempt a setup from reports. On `PUT` an absent field leaves it unchanged, `null` clears it, and a number sets it; it belongs to the setup and survives a rename, and setting and clearing it are audited. A metering policy's `Decision.Weight` above 0 still replaces the result for its hold and poll.

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

A user is identified by the `sub` claim. `email` and the name are stored for display and refreshed at every sign-in. The stored name is the first non-empty of the `name` claim, the `given_name` and `family_name` claims joined by a space, and `preferred_username`; if all are empty the name stays empty. The top bar greets the user with "Hello" and their name, else their username, else their email, so a provider that sends no name still gives a greeting. No scope is added: `profile` already covers these claims. The ID token must verify against the issuer's keys and carry the nonce Hourglass sent.

What the first sign-in looks like:

1. The user opens Hourglass and chooses Sign in with SSO. They sign in at your provider.
2. Hourglass creates them with no role and status `pending`, and shows one page: "You are not authorized to use Hourglass. Reach out to your admin." Every data route of the API returns 403 for them.
3. An admin opens Admin, Users. Pending requests are listed oldest first with name, email and first sign-in time, and the navigation shows the count. The admin assigns `developer` or `admin` to approve, or rejects. A rejected user keeps seeing the same page and can be approved later.
4. The user reloads and lands on My usage (`/usage`). The monthly allowance is credited when the first role is assigned, not at sign-in.

If the user's email is on the pre-approved list, steps 2 to 4 do not happen: the user is active with the listed role and the allowance at the end of the first sign-in and lands on My usage. See [Pre-approving SSO users](#pre-approving-sso-users).

Signing out of Hourglass ends only the Hourglass session. It does not sign the user out of the identity provider, so a second sign-in in the same browser can be silent and use the same identity. To test with several users, use separate private windows.

Roles live in Hourglass, not in the identity provider.

## Pre-approving SSO users

By default a new SSO user waits for an admin to give them a role. If you already hold a list of people who should get in, load it and they are approved at their first sign-in instead. A **pre-approval** is an email address and a non-empty set of roles (`developer`, `admin`). It comes from the list in the database (managed in the admin page or with the admin API) or from the optional file; the roles for an email that is in both are the union of the two.

When a pre-approval applies, the user is active with the listed roles and the monthly allowance by the end of that sign-in, and their first page is My usage. They never see the not-authorized page. It applies when all of these hold:

1. The user signs in with SSO (the local admin is never matched) and their status is `pending`: a new user, or one who signed in before and was never approved or rejected.
2. The ID token's `email` claim is not empty. No other claim is used.
3. The `email_verified` claim is not false. It may be a boolean or the string `"true"` or `"false"`; a claim that is absent counts as verified, because several enterprise providers never send it, and any other value counts as not verified.
4. A pre-approval exists for the email, compared after trimming and lower-casing both sides (ASCII letters only).

What it does not do:

- A user who is `active` (even with no roles) or `rejected` is never changed. An admin who removed someone's roles, or rejected them, is not overridden at the next sign-in.
- Deleting an entry, or removing it from the file, changes nobody who was already approved. There is no way to remove access through this list.
- An entry is not consumed. It stays in the list with the date of its first use, and a second SSO identity that later signs in with the same email is approved too.
- Adding an entry for someone who is already pending does not approve them at once. They are approved at their next sign-in, or by an admin in Users, as before.
- If the approval itself fails, the sign-in still succeeds and the user stays pending, as without a pre-approval. The error is logged.

Each approval is written to the audit log as `user.preapprove` with no actor (shown as "system") and the sources that matched (`database`, `config` or both), and emits the existing `user.approved` event with the payload it always has. A pre-approved new user gets no `user.pending` event. Adding and removing entries are audited as `preapproval.add` and `preapproval.delete`.

### Three ways to load the list

**1. The admin page.** Sign in as an admin, open Admin, Users, and find "Pre-approved users". Paste the emails (commas, spaces or new lines between them), choose the role (Developer is the default), and choose Add. Each row shows the email, roles, where the entry came from (Added here or Config file), who added it, and when the person first signed in ("Not yet" until then). Remove deletes an entry that was added here, after a confirmation; rows from the file have no Remove button.

**2. The file.** Write the file in the format shown under [Pre-approved users file](#pre-approved-users-file), make it readable by the user the process runs as (uid 65532 in the images), mount it into the container, set `preapproved_users_file` or `HOURGLASS_PREAPPROVED_USERS_FILE` to its path inside the container, and restart. The entries then appear in Users with the source "Config file" and cannot be removed there. The file is read once at start, so a change to it needs another restart. Check the start-up log for `pre-approved users file read` and the entry count; an invalid file stops the start and the message names the entry.

```sh
cat > preapproved-users.yaml <<'EOF'
users:
  - email: alice@example.com
    roles: [developer]
  - email: bob@example.com
EOF
chmod 644 preapproved-users.yaml            # the container's user (65532) must be able to read it

docker run -d --name hourglass -p 8080:8080 -v hourglass-data:/data \
  -v "$PWD/preapproved-users.yaml:/etc/hourglass/preapproved-users.yaml:ro" \
  -e HOURGLASS_PREAPPROVED_USERS_FILE=/etc/hourglass/preapproved-users.yaml \
  -e HOURGLASS_ADMIN_PASSWORD="choose-a-long-password" \
  -e HOURGLASS_SESSION_KEY="$(head -c 32 /dev/urandom | base64)" \
  hourglass:dev
docker logs hourglass 2>&1 | grep 'pre-approved users file read'
```

**3. Scripted, into the database.** Use this to load a long list without the admin page. It takes three calls with a cookie jar. The local admin must have changed its first-run password first (see [Local admin account](#local-admin-account)); until then the admin routes answer `password_change_required`. The session cookie is `Secure`, so the base URL must be HTTPS or `localhost` (see [Cookies](#cookies)).

1. `POST /auth/local` as the local admin. The cookie jar keeps the session cookie.
2. `GET /api/v1/session` and read `csrf_token`. (A password change replaces the session and the token: read it again afterwards.)
3. `POST /api/v1/admin/preapprovals` with the `X-CSRF-Token` header and the body `{"emails": [...], "roles": [...]}`. It takes 1 to 1,000 emails and one role set for all of them, so a longer list goes in several calls, grouped by role. Any invalid email or role is a 400 `validation` that names the first bad value, and nothing is stored. The answer is `{"added": N, "updated": M}`: an email already in the list gets the new roles and keeps its use record, so the call can be repeated safely. `GET /api/v1/admin/preapprovals` lists every entry from both sources.

On a stack that has just started, the first-run password change comes first. This sets the new password (it must differ from the current one and be at least 12 characters), and the sign-in below then uses it:

```sh
export HOURGLASS=http://localhost:8080      # HTTPS, or localhost: the cookie is Secure

curl -sS -c jar -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"choose-a-long-password"}' $HOURGLASS/auth/local >/dev/null
CSRF=$(curl -sS -b jar $HOURGLASS/api/v1/session | jq -r .csrf_token)
curl -sS -c jar -b jar -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
  -d '{"current_password":"choose-a-long-password","new_password":"choose-another-long-password"}' \
  $HOURGLASS/auth/local/password
```

Then the three calls (`jq` is needed):

```sh
export HOURGLASS=http://localhost:8080      # HTTPS, or localhost: the cookie is Secure

curl -sS -c jar -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"choose-another-long-password"}' $HOURGLASS/auth/local
CSRF=$(curl -sS -b jar $HOURGLASS/api/v1/session | jq -r .csrf_token)

curl -sS -b jar -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
  -d '{"emails":["alice@example.com","bob@example.com"],"roles":["developer"]}' \
  $HOURGLASS/api/v1/admin/preapprovals
curl -sS -b jar $HOURGLASS/api/v1/admin/preapprovals | jq .
```

```sh
# Run after the calls above (it uses their cookie jar and $CSRF).
# From a file with one email per line, 1,000 at a time; blank lines are skipped:
split -l 1000 emails.txt part-
for f in part-*; do
  jq -R 'select(test("\\S"))' "$f" | jq -s '{emails: ., roles: ["developer"]}' |
    curl -sS -b jar -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
      -d @- $HOURGLASS/api/v1/admin/preapprovals
done
```

### What to know before you rely on it

- **Matching is by the `email` claim of the ID token and nothing else.** The identity provider must release that claim, and it should be a provider that controls its users' email addresses: whoever can sign in with an address on the list gets the listed role. Some providers send the address only in another claim unless they are configured otherwise. A user whose provider sends no `email` stays pending and is approved by hand in Users.
- **A pre-approval can grant `admin`.** Anyone who signs in with that address becomes an admin. Check the list before you load it.
- **Editing the SQLite file by hand is not supported.** Writing to the database directly bypasses the audit log. The admin page and the admin API are the supported ways to load the list into the database.

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

`max_age` is a Go duration, default `10m`, minimum `1m`. A report is stale once it is older than `max_age`, counted from the `observed_at` the service sent. While either selected report is missing or stale, every poll fails with an error that names `max_age`, nobody is charged for that time, and the gap is recorded as an outage (see [Roll out in shadow mode](#roll-out-in-shadow-mode) for how outages are shown). You can select just one of the two; `provider` and `pressure` are independent. A report sent to the integration API while its extension is not selected is refused with 409 `not_configured`. The weights report is separate: it does not depend on the config file, and a stale or missing one never fails a poll (see [Reported weights](#reported-weights)).

Match `max_age` to the poll interval and to how often the service reports. The service should report every 60 seconds, the poll runs every `poll_interval` (5 minutes by default, 1 minute at the least), and each poll needs a report younger than `max_age`. So `max_age` should be at least twice the poll interval and well above the reporting period, or one late report fails a poll. The default 10 minutes suits the default 5-minute poll if the service reports every minute. If you lower `poll_interval`, `max_age` can stay at 10 minutes; if you raise it above 5 minutes, raise `max_age` with it. A shorter `max_age` finds a dead reporter sooner but fails polls on short hiccups; a longer one lets old data stand in for longer. Check the result before relying on it with `hourglassd check-provider`, which prints the age of each report.

### Service accounts

A service account is the identity of your service. It is a separate kind of principal, never a user and never an admin or developer, and its token works only under `/api/v1/integration/`.

1. In Admin, Service accounts, create an account. The name is 1 to 64 letters, digits, `.`, `_` or `-`, and cannot be changed. Give it only the scopes the service needs: `deployments:write`, `pressure:write`, `weights:write`, `state:read`, `holds:release`. `weights:write` allows only `PUT /api/v1/integration/weights`, and no other scope allows that route; no existing account gains it when you upgrade. (`GET` and `PUT /api/v1/admin/service-accounts` do the same from a script.)
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

### Upgrading to the ledger roll-up and pre-approved users (migrations 0006 and 0007)

This release adds two migrations. `0006` adds a column to the ledger (`rollup_count`), a one-row marker table, two partial indexes over per-poll debit rows, and replaces the ledger's delete trigger. `0007` adds the `preapprovals` table. No configuration change is needed: the one new setting, `preapproved_users_file`, is optional, and an existing config file and environment start the new version unchanged. Existing API calls answer as before (see below for the one that gets shorter).

- **Back up first, and keep the backup.** There is no down migration. Once `0006` has run, an older binary refuses the database (`database schema version N is newer than this binary`), and rows that the roll-up has compacted cannot be expanded again. The way back is the backup taken before the upgrade, which loses what was written since. See [Backup and restore](#backup-and-restore).
- **The first start can take longer on a large ledger.** Migration `0006` builds its two indexes over every per-poll debit row, so the time grows with the ledger. How long it takes was not measured on a large ledger, so allow for a pause before the server listens. Each migration runs in one transaction, so an interrupted one leaves the database at the old version.
- **The roll-up runs by itself after the first start.** Right after the first poll the collector starts compacting. In each cycle (after the poll and the daily prune) it runs at most 20 steps, each of at most 2,000 rows taken as whole roll-up groups (a single larger group is taken alone), and it holds the collector lock for one step at a time, so a poll and any user request that writes can run between two steps. The time a cycle spends compacting is taken off the wait before the next poll, so a backlog adds nothing to the gap between polls. A cycle therefore replaces up to about 40,000 rows; at the default 5-minute interval, a ledger with a million per-poll rows takes a few hours of cycles to work off (arithmetic from these limits, not a measurement). The slowest step measured 60 ms on 1.26 million debit rows, so one cycle spends about a second and a quarter compacting at most. The log shows `compacted ledger` with `rows` (rows removed) and `groups` whenever a step removed something. A step that fails is logged as an error and retried on the next cycle; it never fails a poll. When nothing is due any more, the check runs once a day with the other pruning. A restart starts the search again from the oldest day, which costs little because the indexes hold only the rows that are still per-poll.
- **What changes for users.** Nothing changes in anyone's balance, per-hold charges, the tokens-per-day chart, month-to-date burn or the admin totals. On My usage the ledger now shows 20 rows at a time, by day by default, with a switch to Every poll. See [Data retention](#data-retention) for what is kept.
- **`GET /api/v1/me/ledger` without the new parameters** still returns one entry per stored row, unpaged, in the same order, with the same fields plus `entry_count` and `next_cursor` (null). After compaction the stored rows of older days are the rolled-up ones, so the answer gets shorter for those days, and the running balance shown against a grant or other entry in the middle of a compacted day no longer includes that day's earlier debits for the same hold. Each row's balance still equals the previous row's balance plus its own amount, and the balance after the last entry of every day is unchanged.
- **Two other responses each gain one field:** `GET /api/v1/setups` returns `label_index` and `GET /api/v1/session` returns `display_name`. Nothing is removed or renamed.
- **Sign-in is unchanged** until you add a pre-approval: with none, a new SSO user is `pending` and gets `user.pending`, as before.

### Upgrading to reported weights (migration 0008)

This release adds one migration, `0008`, and the optional weights report. `0008` is additive: it creates two tables for the stored weights report and adds a nullable `weight_source` column to the usage samples and a nullable `weight_override` column to the setups. No existing row is rewritten. No configuration change is needed: reported weights are **off by default**, there is no new key in the config file, and an existing config file and environment start the new version unchanged. With the feature off and no override set, every charge and every balance is what the previous release computes from the same data.

- **Back up first, and keep the backup.** There is no down migration. Once `0008` has run, an older binary refuses the database (`database schema version N is newer than this binary`). The way back is the backup taken before the upgrade, which loses what was written since. See [Backup and restore](#backup-and-restore).
- **No existing account can use the new route.** `weights:write` is a new scope that no account holds until an admin adds it, and no existing token or scope allows `PUT /api/v1/integration/weights`. With the setting off the route answers 409 `not_configured`.
- **Added response fields. Nothing is removed or renamed.** `GET /api/v1/setups` gains `reported_weights_enabled`, and each setup gains `weight_source` and `weight_attributes` (and, in its admin block, `reported_weight`, `reported_weight_not_applied` and `weight_override`). A hold gains `weight_source` and `weight_attributes`, and the `burning` entries of `GET /api/v1/me` gain `weight_source`. Each ledger entry of `GET /api/v1/me/ledger` gains `weight_min`, `weight_max` and `weight_sources` (null when unknown, for debits whose samples are gone or were written before the upgrade). `GET /api/v1/integration/state` gains a top-level `weights` and a `weight` on every setup. `GET /api/v1/admin/collector` gains `last_weights_report` and `last_weights_refusal` (null while the feature is off). `GET /api/v1/admin/settings` always returns `reported_weights`, and `PUT` accepts it as an optional key. `POST` and `PUT /api/v1/admin/setups` accept an optional `weight_override`. A client that ignores unknown fields needs no change.
- **Four fields change meaning.** These are deliberate. With the feature off and no override, the first and the last return what they return today.
  - `Setup.weight` (`GET /api/v1/setups`, the admin setup views) was "the weight resolved from the rate rules". It is now "the weight Hourglass applies" (an override, a usable reported weight, a rate rule or the default weight, in that order), and `weight_source` says which.
  - An active `Hold.weight` and the burn figures (`burning` in `GET /api/v1/me`, burn per hour on My usage, and the Admin Users burn) were the weight of the hold's last usage sample. They are now the weight Hourglass would charge now, so after an admin edits a rate rule they change at once, as the Setups list always has, and not at the next poll. A hold whose latest sample came from a Go metering policy's `Decision.Weight` still shows that sample's weight, with the source `policy`.
  - The Admin Users burn per hour now honours a metering policy's weight; it used to show the rate-rule weight even for a hold whose policy had replaced it.
  - `matched_rule_selector` and a rule's `match_count` describe the first matching rate rule. That rule is no longer always the one that sets the weight, because an override or a reported weight can replace it.
- **Policy holds right after the upgrade.** A hold whose weight a Go metering policy sets shows its rate-rule weight on screens between the upgrade and the first poll after it, because samples written before the upgrade carry no source. Charges are not affected, and the first poll puts the policy weight back. The bundled `default` policy never sets a weight, so the bundled configuration is not affected.
- **Body keys.** The top-level keys of a weights report (`observed_at`, `weights`) are matched without regard to case, as in the other two reports. The keys inside an entry are strict.
- **Extensions.** No interface, type or contract changes. `SetupState.Weight` is the base weight, so the `inventory` pressure source and every custom extension compute what they computed before. See [Extending Hourglass](extending.md#snapshot-holdstate-and-decision).
- **Turning it on** is a separate, audited step (Admin, Settings, "Reported weights"; a maximum is required). See [Reported weights](#reported-weights).

## Data retention

| Data | Kept |
| --- | --- |
| Usage samples | 13 months |
| Event deliveries | 90 days |
| Expired sessions | Deleted daily and at each sign-in |
| Ledger: debits | One row per poll for the current day and the seven days before it; older days are rolled up to one row per user, hold and day (see below) |
| Ledger: allowance, grant, reset and every other entry | Kept as written, never pruned or changed |
| Audit log, outages, pressure samples, collector runs | Kept (not pruned in v1) |
| Pre-approvals | Kept until an admin removes them. A used entry stays |

Pruning runs once a day, from the collector.

### The ledger roll-up

The ledger is no longer kept forever in full detail. The per-poll debits of a hold are rolled up once their local day is earlier than seven days before today's date, both taken in the configured timezone (so the current day and the seven before it still have a row per poll). All the per-poll debits of one user, hold, month and local day are replaced by one debit row. The new row carries the total, the `id` and time of the newest row it replaced, and the number of rows replaced, which My usage shows as "N polls". Only debits that belong to a hold are rolled up. Grants, resets, allowances, debits with no hold, rows of the last eight days and rows already rolled up are never changed, and a group of a single row is left as it is. Rolling up is final: a rolled-up row is never opened up again, and compacted rows cannot be expanded.

The sums do not change. Per user, month and type, per hold, and per user and day, the totals are the same before and after, and so are balances.

- **What users see.** My usage shows the ledger by day by default, one row per hold and day. The Every poll view shows single debits with their multipliers for the days that have not been rolled up yet, and the rolled-up row, with its poll count and no multiplier, for older days. The ledger can show 20 rows at a time and reach the oldest entry with Older.
- **Per-poll detail is not lost from the database.** The detail of every poll (weight, pressure, multiplier and debit) stays in `usage_samples` for 13 months. It is no longer shown on the ledger.
- **The ledger stays append-only.** The database still refuses every update, every replace, every delete of a non-debit row and every delete made outside the roll-up. The roll-up is the one exception, for the debit rows of one hold and day, and it keeps the same total.

### Disk space

The roll-up stops the database file growing at the old rate: a hold that is held all day wrote 288 rows a day at the default interval, and every day older than a week now keeps one. It does not shrink the file. SQLite keeps the pages the roll-up frees and reuses them for new rows, so the file stays as large as it got until it is compacted, and then grows slowly. To get the space back, take the file offline and run `VACUUM`: stop Hourglass, take a backup (see [Backup and restore](#backup-and-restore)), run the command below on the stopped database, and start Hourglass again. `VACUUM` needs free disk space about the size of the database while it runs, and the time grows with the file. Do it after the backlog has been worked off, or the file will grow again before it has been used.

```sh
sqlite3 /data/hourglass.db "VACUUM"
```

## Known limits

- One instance. One process owns the database; there is no high availability, and running two against one file is not supported.
- TLS is optional and static: one certificate, read at start, no reload and no automatic issuing (ACME). In split mode the UI container cannot reach an API that serves a self-signed certificate.
- The event queue is in memory: events still queued at shutdown are lost, and nothing is retried.
- The local admin password cannot be reset without database access.
- The ledger roll-up age (seven days) is fixed: there is no setting and no way to turn the roll-up off, and compacted rows cannot be expanded again.
- Pre-approval matches the `email` claim only, by whole address (no wildcard or domain entries). The file is read at start only.
- vSphere: clusters in host sub-folders are not discovered, and nothing has been run against a real vCenter.
- Images are built and smoke-tested on arm64. The amd64 `hourglass` image has only been started under emulation; the other amd64 images have not been run.
- The release workflow publishes the images on a `v*` git tag without running the tests. There is no Helm chart and no Dockerfile.

## Local stacks

`deploy/` holds compose files for trying Hourglass with Keycloak and a simulator standing in for vCenter. Every secret in them is a fake development value. Do not use them as a production template. The [README](../README.md) shows how to start one, and `deploy/README.md` lists ports and test users.
