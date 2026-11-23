# Local stacks

`docker compose up` brings up a complete local Hourglass with Keycloak and the vCenter simulator. Nothing needs a real vCenter. Development only: every secret in these files is a fake value (`dev-only-...`).

## Load the images
Needs Bazel and Docker. `deploy/load-images.sh` builds and loads `hourglass`, `hourglass-api`, `hourglass-ui`, `hourglass-e2e`, `hourglass-api-e2e` and `hourglass-vcsim` as `<name>:dev`; name the ones you need to load only those (the bundled stack runs `hourglass` and `hourglass-vcsim`, the split stack `hourglass-api`, `hourglass-ui` and `hourglass-vcsim`). Keycloak (`quay.io/keycloak/keycloak:26.4.0`) and a small curl image used for readiness probes are pulled by compose.

## Start and stop
Both stacks use the same host ports, so run one at a time (they have different project names).

    docker compose -f deploy/compose.bundled.yaml up -d --wait     # one container serves API and UI
    docker compose -f deploy/compose.split.yaml up -d --wait       # hourglass-api + hourglass-ui
    docker compose -f deploy/compose.<mode>.yaml down -v           # stop and delete the data volume

To run the bundled stack over HTTPS with a self-signed certificate (Hourglass serves TLS itself on port 8443, D64), make a development certificate and add the override file. It is a separate project, `hourglass-tls`, with its own data volume:

    deploy/make-dev-cert.sh                                        # writes deploy/certs/, which git ignores
    docker compose -f deploy/compose.bundled.yaml -f deploy/compose.tls.yaml up -d --wait
    curl --cacert deploy/certs/tls.crt https://localhost:8443/healthz
    docker compose -p hourglass-tls down                           # stop; add -v to delete its data

Open https://localhost:8443. The browser warns until it trusts `deploy/certs/tls.crt`. The Keycloak realm allows the redirect URL `https://localhost:8443/auth/callback` as well as the plain one.

Add `down` without `-v` to keep the database. The stacks run the product images (`hourglass:dev`; `hourglass-api:dev` and `hourglass-ui:dev`) and publish no clock control port. The end-to-end test (`e2e/run.sh`, see `e2e/README.md`) layers `e2e/compose.<mode>.yaml` over these files to swap in the test builds `hourglass-e2e:dev` and `hourglass-api-e2e:dev`, which add the clock control endpoint on port 8091.

`deploy/smoke.sh <bundled|split>` loads the product images (and the simulator), starts the stack, checks it, and tears it down. It takes about 2 to 5 minutes.

## Ports on the host
| Port | What |
|---|---|
| 8080 | Hourglass (the UI container in split mode). Open http://localhost:8080 |
| 8443 | Hourglass over HTTPS, only with `compose.tls.yaml`. Open https://localhost:8443 |
| 8081 | Keycloak, http://keycloak.localhost:8081 |
| 8990 | Simulator control API (`/pools`, `/pressure`, `/healthz`) |
| 8091 | End-to-end clock control (`GET/PUT /clock`); published only by the `e2e/` override files, never by these stacks |

## One issuer URL
The browser runs on the host; Hourglass runs in a container. The OIDC issuer is `http://keycloak.localhost:8081/realms/hourglass` for both: browsers resolve `*.localhost` to loopback, and Keycloak has the network alias `keycloak.localhost` and listens on 8081 inside and outside, so containers reach it under the same name. The redirect URL is `http://localhost:8080/auth/callback` (the session cookie is `Secure`; browsers accept that on `http://localhost`). `curl` also treats `*.localhost` as loopback.

## Signing in
- Local admin: username `admin`, password `dev-only-not-a-secret` (from `HOURGLASS_ADMIN_PASSWORD` in the compose file; read only when the account is first created). Hourglass makes you change it at first sign-in.
- Keycloak users (realm `hourglass`, client `hourglass`): `alice` / `dev-only-alice`, `bob` / `dev-only-bob`, `carol` / `dev-only-carol`, and `dave` / `dev-only-dave`. A new OIDC user lands on the not-authorized page until the admin gives them a role. `dave` (`dave@example.com`) is reserved for the pre-approval end-to-end step: the admin pre-approves his address and he signs in for the first time with a role already in place. No other scenario signs him in, so keep him unused if you want to repeat that step by hand.
- Keycloak admin console: http://keycloak.localhost:8081, `admin` / `dev-only-not-a-secret`.

## First-run admin steps
The seed (`seed.sample.yaml`) creates `lab-01` to `lab-06` and the rate rules once. One setting of the reference scenario is not seedable; set it in Admin > Settings (the end-to-end test does it through the API): display label key `size`. The pools are named `<setup>_RP`; `pool_name_suffix: _RP` in the provider block of `config.sample.yaml` makes the provider report them under the setup names.

## Files
`config.sample.yaml` (the reference scenario's config), `seed.sample.yaml`, `keycloak/realm-hourglass.json` (imported with `--import-realm`), `compose.bundled.yaml`, `compose.split.yaml`, `compose.tls.yaml` and `make-dev-cert.sh` (the HTTPS variant), `push-images.sh`, `smoke.sh`; the end-to-end overrides are `../e2e/compose.bundled.yaml` and `../e2e/compose.split.yaml`. Distroless product images have no shell, so readiness is probed by two small curl containers (`deps-ready`, `stack-ready`) in the compose files.
