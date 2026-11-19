# Local stacks

`docker compose up` brings up a complete local Hourglass with Keycloak and the vCenter simulator. Nothing needs a real vCenter. Development only: every secret in these files is a fake value (`dev-only-...`).

## Load the images
Needs Bazel and Docker. `deploy/load-images.sh` builds and loads `hourglass`, `hourglass-api`, `hourglass-ui`, `hourglass-e2e`, `hourglass-api-e2e` and `hourglass-vcsim` as `<name>:dev`. Keycloak (`quay.io/keycloak/keycloak:26.4.0`) and a small curl image used for readiness probes are pulled by compose.

## Start and stop
Both stacks use the same host ports, so run one at a time (they have different project names).

    docker compose -f deploy/compose.bundled.yaml up -d --wait     # one container serves API and UI
    docker compose -f deploy/compose.split.yaml up -d --wait       # hourglass-api + hourglass-ui
    docker compose -f deploy/compose.<mode>.yaml down -v           # stop and delete the data volume

Add `down` without `-v` to keep the database. By default the stacks run the end-to-end images (`hourglass-e2e:dev`, `hourglass-api-e2e:dev`), which add a clock control endpoint on port 8091. For the plain product images set `HOURGLASS_IMAGE=hourglass:dev` (bundled) or `HOURGLASS_API_IMAGE=hourglass-api:dev` (split).

`deploy/smoke.sh <bundled|split>` loads the images, starts the stack, checks it, and tears it down. It takes about 2 to 5 minutes.

## Ports on the host
| Port | What |
|---|---|
| 8080 | Hourglass (the UI container in split mode). Open http://localhost:8080 |
| 8081 | Keycloak, http://keycloak.localhost:8081 |
| 8990 | Simulator control API (`/pools`, `/pressure`, `/healthz`) |
| 8091 | End-to-end clock control (`GET/PUT /clock`), e2e images only |

## One issuer URL
The browser runs on the host; Hourglass runs in a container. The OIDC issuer is `http://keycloak.localhost:8081/realms/hourglass` for both: browsers resolve `*.localhost` to loopback, and Keycloak has the network alias `keycloak.localhost` and listens on 8081 inside and outside, so containers reach it under the same name. The redirect URL is `http://localhost:8080/auth/callback` (the session cookie is `Secure`; browsers accept that on `http://localhost`). `curl` also treats `*.localhost` as loopback.

## Signing in
- Local admin: username `admin`, password `dev-only-not-a-secret` (from `HOURGLASS_ADMIN_PASSWORD` in the compose file; read only when the account is first created). Hourglass makes you change it at first sign-in.
- Keycloak users (realm `hourglass`, client `hourglass`): `alice` / `dev-only-alice`, `bob` / `dev-only-bob`, `carol` / `dev-only-carol`. A new OIDC user lands on the not-authorized page until the admin gives them a role.
- Keycloak admin console: http://keycloak.localhost:8081, `admin` / `dev-only-not-a-secret`.

## First-run admin steps
The seed (`seed.sample.yaml`) creates `lab-01` to `lab-06` and the rate rules once. Two settings of the reference scenario are not seedable; set them in Admin > Settings (the end-to-end test does it through the API): resource name template `{{.SetupName}}_RP`, display label key `size`.

## Files
`config.sample.yaml` (the reference scenario's config), `seed.sample.yaml`, `keycloak/realm-hourglass.json` (imported with `--import-realm`), `compose.bundled.yaml`, `compose.split.yaml`, `smoke.sh`. Distroless product images have no shell, so readiness is probed by two small curl containers (`deps-ready`, `stack-ready`) in the compose files.
