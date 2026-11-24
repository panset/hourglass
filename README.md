# Hourglass

Hourglass is an open-source token-management framework for shared lab setups. It meters how long each developer holds a shared setup and drains a monthly token balance faster when setups are scarce, so idle setups get released. The core handles tokens only: what is deployed, how scarcity is measured, when charging starts and what happens on events are extensions you can replace without forking. Version 1 bundles vSphere extensions as its reference scenario.

**How tokens work.** An admin gives each setup a weight through label-based rate rules (for example `size=large` is 2). Every 5 minutes, each holder is debited `weight x multiplier x 5/60` tokens. The multiplier comes from lab pressure, a number from 0% to 100% that a pressure source reports and the core averages over 15 minutes: below 50% is free (0x), 50% to 75% is 1x, 75% to 90% is 2x, 90% and above is 3x. Bands and multipliers are admin-editable. Each user gets 300 tokens on the 1st of the month; surplus expires and debt carries over. At zero or below you cannot acquire new setups, but nothing you hold is torn down.

**Ledger, labels and sign-in.** The ledger on My usage pages 20 rows at a time, by day or every poll; storage rolls the per-poll debits of days older than a week up to one row per hold and day, with the same totals. The Setups page lists every label as a clickable chip that fills the selector. The top bar greets the signed-in user by name. An admin can pre-approve SSO users by email, in Admin, Users or with a file, so that a listed person gets their role at the first sign-in instead of waiting; [docs/deploying.md](docs/deploying.md#pre-approving-sso-users) explains how to load a list.

**Reported weights.** A customer's service can report a weight per setup, so that the price of a setup follows what the setup currently reserves and comes down when it shrinks. It is off until an admin turns it on with a minimum and a maximum. Hourglass applies a valid report within those limits, records on every charge where the weight came from, shows the weight and its source wherever a price is shown, and falls back to the rate rules, without failing a poll, whenever no valid report exists. An admin can pin one setup's weight. See [docs/integrating.md](docs/integrating.md#report-weights) and [docs/deploying.md](docs/deploying.md#reported-weights).

**Status: v1, tested against a simulator.** All vSphere code is built and tested against mocks and the govmomi simulator. It has not been run against a real vCenter, and the first run needs the [checklist](docs/deploying.md#first-run-checklist-against-a-real-vcenter). Hourglass runs as one process with one SQLite file and has no high availability.

## Quick start with Docker and Bazel

This starts Hourglass, Keycloak as the login provider, and a simulator standing in for vCenter. You need Docker with Compose v2 and Bazel 7.4.1. Every secret in the local stack is a fake development value.

1. Build and load the images the bundled stack runs into Docker (the first run takes a while):

   ```sh
   deploy/load-images.sh hourglass hourglass-vcsim
   ```

2. Start the bundled stack: one container serves the API and the UI.

   ```sh
   docker compose -f deploy/compose.bundled.yaml up -d --wait
   ```

3. Open <http://localhost:8080>.
4. Sign in as the local admin: username `admin`, password `dev-only-not-a-secret`. That password comes from `HOURGLASS_ADMIN_PASSWORD` in the compose file and is read only when the account is first created. Hourglass makes you choose a new password of at least 12 characters before it shows anything else. Without that variable, Hourglass generates a password and prints it once in its log.
5. Open Admin, Settings and set the one thing the seed file cannot set: the display label keys to `size`.
6. Sign in as a realm user. Open a private window at <http://localhost:8080>, choose Sign in with SSO and use `alice` with password `dev-only-alice` (`bob` and `carol` work too; `dave` is kept for the pre-approval end-to-end test, so leave him unused). Alice sees "You are not authorized to use Hourglass." That is correct: new users have no role. Use a private window because signing out of Hourglass does not end the Keycloak session.
7. Approve her. In the admin window open Admin, Users. Alice is listed under pending requests. Give her the `developer` role.
8. Acquire a setup. In Alice's window reload, open Setups and acquire `lab-03`. Metering starts when the provider reports the setup's resource pool, or after 2 hours. To start it now, create the pool in the simulator and wait for the next poll (polls run every 5 minutes). If Alice releases while the pool exists, the hold is marked Releasing and stays metered until the pool is gone, so delete the pool and wait for another poll to finish the release:

   ```sh
   curl -X POST localhost:8990/pools -H 'Content-Type: application/json' -d '{"name":"lab-03_RP"}'
   curl -X DELETE localhost:8990/pools/lab-03_RP
   ```

Stop the stack and delete its data with `docker compose -f deploy/compose.bundled.yaml down -v`. `deploy/compose.split.yaml` runs the same scenario as two containers (API and UI). `deploy/README.md` lists ports and test users.

## Quick start with no Docker and no Bazel

Plain `go build` produces an API-only binary: no UI, no Node, no Bazel. You need Go 1.26 or newer (`go.mod` says 1.26.0).

1. Build it and make a working directory:

   ```sh
   mkdir demo
   go build -o demo/hourglassd ./cmd/hourglassd
   cd demo
   ```

2. Write a minimal config. It uses the `static` provider, which reads allocations from a JSON file, and a seed file with two setups.

   ```sh
   cat > hourglass.yaml <<'EOF'
   listen: 127.0.0.1:8080
   database: hourglass.db
   provider:
     type: static
     config:
       path: allocations.json
   seed: seed.yaml
   EOF
   echo '[]' > allocations.json
   cat > seed.yaml <<'EOF'
   setups:
     - name: lab-01
       labels: {size: medium}
     - name: lab-02
       labels: {size: large}
   rate_rules:
     - {selector: "size=large", weight: 2}
   EOF
   ```

3. Start it. With no `oidc` block, the local admin is the only way in.

   ```sh
   export HOURGLASS_ADMIN_PASSWORD="choose-a-long-local-password"
   export HOURGLASS_SESSION_KEY="$(head -c 32 /dev/urandom | base64)"
   ./hourglassd -config hourglass.yaml
   ```

4. In a second terminal, check it and sign in. The session cookie is `Secure`; `curl` accepts it on `localhost`.

   ```sh
   curl -s localhost:8080/healthz
   curl -s -c jar -H 'Content-Type: application/json' \
     -d '{"username":"admin","password":"choose-a-long-local-password"}' localhost:8080/auth/local
   ```

   The reply has `"must_change_password":true`. Change the password, sending the CSRF token from the session:

   ```sh
   CSRF=$(curl -s -b jar localhost:8080/api/v1/session | python3 -c 'import sys,json; print(json.load(sys.stdin)["csrf_token"])')
   curl -s -b jar -c jar -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" \
     -d '{"current_password":"choose-a-long-local-password","new_password":"a-new-long-password"}' \
     localhost:8080/auth/local/password
   curl -s -b jar localhost:8080/api/v1/setups
   ```

5. Make the provider report a resource, then see what Hourglass sees without writing anything:

   ```sh
   echo '[{"resource_name":"lab-01"},{"resource_name":"unrelated"}]' > allocations.json
   ./hourglassd check-provider -config hourglass.yaml
   ```

To see the dashboard, use the Bazel-built bundled binary (`bazel build //cmd/hourglassd:hourglassd`) or the split-mode UI image. A binary built with plain `go build` serves the API only.

## Build and test

| Command | What it does |
| --- | --- |
| `go build ./...` | Builds everything. `hourglassd` has no UI. No Node, no Bazel |
| `go test ./...` | Runs the Go tests |
| `bazel build //...` | Builds every binary, the UI and all images for both architectures |
| `deploy/push-images.sh TAG` | Pushes the three product images to the registry (`ghcr.io/panset` by default) |
| `bazel test //...` | Runs every test, including the UI tests and the API contract test |
| `./ci.sh` | Everything CI runs: header and genericity checks, the dependency license check, Go build and vet, the Gazelle check, `bazel build //...`, `bazel test //...` |
| `bazel build //cmd/hourglassd:hourglassd` | The bundled binary with the UI embedded (build tag `ui`) |
| `bazel build //cmd/hourglassd:hourglassd-api` | The same without the UI |

The prerequisites and what CI enforces are in [CONTRIBUTING.md](CONTRIBUTING.md).

## Repository layout

| Path | What is there |
| --- | --- |
| `api/openapi.yaml` | The contract between the UI and the backend |
| `ui/` | React and TypeScript single-page app, built with Vite |
| `cmd/hourglassd` | The server: the core plus the bundled extensions |
| `cmd/hourglass-ui` | The split-mode UI server and reverse proxy |
| `pkg/hourglass` | The public extension API: the four interfaces, `Event`, the registry, `Run` |
| `pkg/hourglass/server` | Imported by a custom `main` to link the server behind `Run` |
| `pkg/hourglasstest` | Fakes and conformance suites for extension authors |
| `providers/vsphere`, `providers/static` | Resource providers; `vsphere` also holds the vsphere pressure source |
| `pressure/inventory` | The default pressure source |
| `policies/default` | The default metering policy |
| `hooks/webhook` | Posts events as JSON to a URL |
| `internal/` | The core: `auth`, `api`, `rules`, `collector`, `metering`, `ledger`, `holds`, `events`, `settings`, `store`, `uiserve`, `config`, `app` |
| `examples/custom` | A third-party binary with its own four extensions |
| `deploy/` | Compose files, Keycloak realm, sample config and seed, image targets |
| `e2e/` | The simulator container and the end-to-end test main |
| `tools/` | CI checks: SPDX headers, core genericity, dependency licenses |
| `docs/` | Specification, decisions, progress, and the guides below |

## Documentation

The integration API is for a lab that already has its own tooling. A customer's own service reports what is deployed and how contended the lab is over a token-authenticated HTTP API, and receives webhooks when a hold is acquired or released, with no Go required. [docs/integrating.md](docs/integrating.md) walks through it.

- [docs/deploying.md](docs/deploying.md): bundled and split mode, images, configuration reference, OIDC, pre-approving SSO users, vSphere, backup, metrics, upgrade, data retention.
- [docs/integrating.md](docs/integrating.md): connect an existing lab stack without writing Go. Your service reports what is deployed and how contended the lab is over a token-authenticated HTTP API, and receives webhooks for acquire and release.
- [docs/extending.md](docs/extending.md): the four interfaces, events, hooks, building your own binary.
- [CONTRIBUTING.md](CONTRIBUTING.md): building, testing and the rules CI enforces.
- [docs/SPEC.md](docs/SPEC.md): the functional specification. [docs/decisions.md](docs/decisions.md) records choices where the spec is silent, and [docs/progress.md](docs/progress.md) records what is finished.

Out of scope for v1: deploying or deleting VMs (automation belongs in a hook), the manager role and team budgets, user labels with allowance rules, per-selector hold limits, a configurable period length, notifications, queues and reservations, and token transfers between users.

## License

MIT. See [LICENSE](LICENSE).
