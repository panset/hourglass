# RESULT 34-compose

Status: done. `deploy/smoke.sh bundled` and `deploy/smoke.sh split` both exit 0 on this machine (docker 27.3.1, darwin/arm64); `./tools/check_spdx.sh` passes (it does not look at `.json`). Both runs removed their containers, networks and volumes. I did not drive a browser login; I confirmed that Hourglass's authorization URL gives Keycloak's login page (HTTP 200, `id="username"` present), which validates the client, redirect URI and PKCE settings.

## Files (all new, under deploy/)
config.sample.yaml, seed.sample.yaml, keycloak/realm-hourglass.json, compose.bundled.yaml, compose.split.yaml, smoke.sh (executable), README.md; this folder (PROGRESS.md, RESULT.md).

## What the stack does
- Issuer `http://keycloak.localhost:8081/realms/hourglass` for browser and container: works on Docker Desktop macOS with `KC_HOSTNAME=http://keycloak.localhost:8081`, `KC_HTTP_PORT=8081`, port 8081:8081 and the network alias `keycloak.localhost`. No other arrangement was needed. Go's resolver in the Hourglass image resolves the alias (`/auth/oidc/start` redirected to the issuer after server-side discovery).
- Keycloak 26.4.0 (pinned), `start-dev --import-realm`; realm `hourglass`, confidential client `hourglass` (PKCE S256, secret `dev-only-not-a-secret`), users alice, bob, carol (`@example.com`).
- Project names `hourglass-bundled` and `hourglass-split`; same host ports 8080, 8081, 8990, 8091 (e2e control, published from the Hourglass or API container).
- Bundled default image `hourglass-e2e:dev` (`HOURGLASS_IMAGE` overrides); split default `hourglass-api-e2e:dev` (`HOURGLASS_API_IMAGE`) and `hourglass-ui:dev` (`HOURGLASS_UI_IMAGE`).
- Config and seed are mounted read-only from the sample files; secrets come from compose env. The vsphere password is `HOURGLASS_VSPHERE_PASSWORD=pass` (simulator dummy).

## Deviations and decisions for docs/decisions.md
1. Healthchecks: the Hourglass, UI and simulator images are distroless (no shell, no curl) and `hourglassd` has no health subcommand, so they cannot carry a container healthcheck. Two sidecars from `curlimages/curl:8.11.1` (pinned) provide it: `deps-ready` (simulator control and Keycloak discovery; Hourglass `depends_on` it with `service_healthy`, so the first poll finds the simulator) and `stack-ready` (`/healthz`, and `/ui-healthz` in split), which makes `up --wait` mean "usable". Keycloak has a bash TCP healthcheck of the discovery document (no curl in its image).
2. Probes address Keycloak as `http://keycloak:8081` because curl treats `*.localhost` as loopback by itself. The smoke check "from inside" uses the probe container with `--resolve` to the alias address; Hourglass's own discovery is proved by the OIDC redirect.
3. Realm `sslRequired: none`, `start-dev` (HTTP), as the brief allows.
4. Config sample sets `database: /data/hourglass.db` and `seed: /etc/hourglass/seed.yaml`; split mode turns the UI off with `HOURGLASS_UI_SERVE=false`.
5. The smoke check changes the admin password (the forced change) before reading `/api/v1/admin/collector`, as the brief says; its throwaway new password is in the script.
6. Seed `details` are `{note: "sample setup"}`; `default_weight: 1` is written explicitly.

## Product issues
None found. Note only: the first poll runs at start (about 1 second after Hourglass starts) and succeeded in both modes.

## Commands run and outcome
- `deploy/load-images.sh`: ok. `docker compose ... up -d --wait` by hand: ok after the probe fix (first try failed because curl resolved `keycloak.localhost` to loopback, see 2).
- `deploy/smoke.sh bundled`: exit 0. `deploy/smoke.sh split`: exit 0. `./tools/check_spdx.sh`: exit 0.

## Transcript tails
### bundled
    == up (bundled)
    == checks
    ok   /healthz is 200
    ok   / returns the UI's HTML
    ok   /api/v1/auth/methods reports OIDC available
    ok   /auth/oidc/start redirects to the issuer
    ok   Keycloak discovery reachable from the host at the issuer URL
    ok   Keycloak discovery reachable inside the compose network under the issuer's host name
    ok   simulator control endpoint answers
    == local admin
    ok   POST /auth/local with the compose password gives 200 and a session cookie
    ok   first sign-in requires a password change
    ok   password change succeeds
    == first poll (up to 5 minutes)
    ok   collector: provider vsphere, pressure source vsphere, last poll OK
    ok   the six seeded setups exist (lab-01 .. lab-06)
    SMOKE OK (bundled)
    == teardown

### split
    == up (split)
    == checks
    ok   /healthz is 200
    ok   / returns the UI's HTML
    ok   /api/v1/auth/methods reports OIDC available
    ok   /auth/oidc/start redirects to the issuer
    ok   Keycloak discovery reachable from the host at the issuer URL
    ok   Keycloak discovery reachable inside the compose network under the issuer's host name
    ok   simulator control endpoint answers
    ok   hourglass-ui /ui-healthz answers
    == local admin
    ok   POST /auth/local with the compose password gives 200 and a session cookie
    ok   first sign-in requires a password change
    ok   password change succeeds
    == first poll (up to 5 minutes)
    ok   collector: provider vsphere, pressure source vsphere, last poll OK
    ok   the six seeded setups exist (lab-01 .. lab-06)
    SMOKE OK (split)
    == teardown

## Confidence and gaps
- Verified by running: both smoke runs, the Keycloak login page for the OIDC start URL, teardown leaving nothing behind.
- Not verified: a full browser login through Keycloak back to Hourglass (callback, token exchange); the plain `hourglass:dev` / `hourglass-api:dev` images with these files (the env override is trivial, inference); amd64 hosts; Keycloak health on a slow machine (retries allow 5 minutes).
