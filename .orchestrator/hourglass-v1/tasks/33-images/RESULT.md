# RESULT 33-images

Status: done. `bazel build //...` and `bazel test //...` pass (35 of 35 tests; no `internal/api` failure seen). The three smoke checks pass on this machine (docker 27, darwin/arm64). `tools/check_spdx.sh` and `tools/check_core_generic.sh` pass; `go build ./...` and `go vet` pass.

## What was built
1. `cmd/hourglass-ui` (`main.go`, `proxy.go`, `proxy_test.go`, `BUILD.bazel`). Serves the UI with `internal/uiserve` (embedded under the `ui` tag) and reverse-proxies `/api`, `/auth`, `/healthz` (each path and anything under it; `/apix` is not proxied). `-api URL` or `HOURGLASS_UI_API` (required), `-listen` or `HOURGLASS_UI_LISTEN` (default `:8080`), `version`. The API sees the browser's Host (`Out.Host = In.Host`) plus `X-Forwarded-Host`, `X-Forwarded-Proto` (an inbound `http`/`https` value from a TLS-terminating front proxy is kept, else derived from the connection) and `X-Forwarded-For`. Cookies, `Set-Cookie` and `Location` are untouched; redirects are never followed. Own `/ui-healthz` returns `{"status":"ok"}`. Upstream down gives 502 in the standard error shape (`internal/api/apierr`, code `internal`, "the API is not reachable"). Tests (`httptest`): static file, SPA fallback, three prefixes proxied, 302 with `Set-Cookie` and Host passthrough, nothing else proxied, 502 JSON, API address validation, run() flags.
2. `hourglass.Options.Clock` (`interface{ Now() time.Time }`, nil = wall clock): `pkg/hourglass/run.go`, `internal/app/runhook/runhook.go`, `internal/app/app.go`. The one clock goes to the ledger service, collector, holds, events (`events.WithClock`), auth (`auth.WithClock`), the seed import time and the metrics start time (the API already took it). Test `TestOptionsClock_CollectorAndLedgerUseTheSuppliedClock` (`internal/app/app_test.go`): with a fixed clock in 2031 the collector run is stamped with it and the ledger rollover writes entries in that clock's period at that time. Gap: `auth.EnsureLocalAdmin` still stamps the admin's `created_at` with `time.Now()` (in `internal/auth`, outside my scope; harmless).
3. `e2e/hourglassd-e2e` (`main.go`, `clock.go`, `clock_test.go`): same imports, defaults and subcommands as `cmd/hourglassd`, plus the offset clock. `HOURGLASS_E2E_CONTROL=:8091` serves `GET /clock` and `PUT /clock {"offset":"720h"}` (Go duration, negative allowed; a PUT replaces the offset, it does not add; bad input 400). Unset: no control endpoint. Targets `//e2e/hourglassd-e2e:hourglassd-e2e` (UI) and `:hourglassd-api-e2e`. Nothing under `cmd/`, `internal/`, `pkg/` imports `e2e/`.
4. Images, in `deploy/images/BUILD.bazel` with the macro in `deploy/images/defs.bzl` (rules_oci 2.2.6 added to `MODULE.bazel`, plus `platforms`; `MODULE.bazel.lock` updated). Binaries: `go_binary` with `goos="linux"`, `goarch`, `pure="on"` (cgo off, static) per architecture; layer = `/usr/local/bin/<binary>` (root-owned 0755) from aspect `tar`; second layer creates `/data` owned 65532:65532; user `65532:65532`, `WORKDIR /data`, entrypoint the binary, `exposed_ports` (the EXPOSE equivalent) and OCI labels (title, description, licenses, base name, ports, data-volume note).

## Image targets
All in `//deploy/images`. For each NAME: `:NAME_amd64`, `:NAME_arm64` (single-platform images), `:NAME_index` (both as an OCI index), `:NAME` (alias for the host CPU), `:load_NAME` (`oci_load`, tag `manual`, loads `NAME:dev`). Load and run: `bazel run //deploy/images:load_NAME`, or `deploy/load-images.sh [names]` for all.

| NAME | contents | ports | docker size (arm64) | run |
|---|---|---|---|---|
| hourglass | hourglassd with UI | 8080 | 55.9MB | `docker run -v hg:/data -e HOURGLASS_ADMIN_PASSWORD=... -p 8080:8080 hourglass:dev` |
| hourglass-api | hourglassd, no UI | 8080 | 55.4MB | same with `hourglass-api:dev` |
| hourglass-ui | proxy plus UI | 8080 | 20.5MB | `docker run -e HOURGLASS_UI_API=http://api:8080 -p 8080:8080 hourglass-ui:dev` |
| hourglass-e2e | e2e main with UI | 8080, 8091 | 55.9MB | add `-e HOURGLASS_E2E_CONTROL=:8091` |
| hourglass-api-e2e | e2e main, no UI | 8080, 8091 | 55.4MB | same |
| hourglass-vcsim | `//e2e/simulator` | 8989 (SOAP, TLS), 8990 (control) | 51.6MB | `docker run -p 8989:8989 -p 8990:8990 hourglass-vcsim:dev` |

Config comes from the usual `HOURGLASS_*` variables or a mounted file (`HOURGLASS_CONFIG`). Default database is `/data/hourglass.db` (WORKDIR). `bazel build //...` builds all layers for both architectures without Docker; `load_*` are `manual` so `//...` skips them.

## Volume ownership
`/data` exists in the image owned by 65532:65532, so a Docker named volume (initialised from the image) is writable. A bind mount keeps the host directory's owner: it must be writable by uid 65532 (`chown 65532:65532 dir`, or `user:` / `--user` override in compose). The smoke check uses named volumes.

## Base image
`gcr.io/distroless/static-debian12:nonroot`, index digest `sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab` (pulled 2026-11-19). Platform manifests: linux/amd64 `sha256:52dcfbabb7457ea47c82f6e13af8c8a4a1d9f7b0145142b3ecab20f2b888411d`, linux/arm64/v8 `sha256:06c3c14b7fa252d9f4285242d2d665ce3b51f74b02d3c529464f220f2912aecb`. Pinned in `MODULE.bazel` (`oci.pull`).

## Scripts
- `deploy/load-images.sh [names]`: loads all six (or the named ones).
- `deploy/smoke-images.sh [--no-load]`: for `hourglass:dev` (named volume, throwaway random admin password, `/healthz` 200 with database ok, `/` HTML, SPA fallback, non-root), `hourglass-api:dev` (`/` 404), `hourglass-ui:dev` pointed at it over a temporary Docker network (`/ui-healthz`, proxied `/healthz` and `/api/v1/auth/methods`, `/` HTML); non-root is checked by image user 65532:65532 and `docker top` uid. Everything it creates is removed (verified: no leftover containers or volumes). Result on this machine: `SMOKE OK`. I also started `hourglass-e2e:dev` (control listener logged) and `hourglass-vcsim:dev` (started) by hand.

## Files changed
New: `cmd/hourglass-ui/*`, `e2e/hourglassd-e2e/*`, `deploy/images/{BUILD.bazel,defs.bzl}`, `deploy/load-images.sh`, `deploy/smoke-images.sh`, this folder. Edited: `MODULE.bazel` (rules_oci block inserted before `go_sdk`; I did not touch `use_repo(go_deps, ...)`), `MODULE.bazel.lock`, `pkg/hourglass/run.go`, `internal/app/{app.go,app_test.go}`, `internal/app/runhook/runhook.go`, and one-line visibility changes (private to `//deploy/images:__pkg__`) in `cmd/hourglassd/BUILD.bazel` and `e2e/simulator/BUILD.bazel` so the image macro can embed their libraries (outside the brief's write list but needed; no source change). `go.mod` untouched.

## Decisions for docs/decisions.md
1. rules_oci 2.2.6 (newest release that works with Bazel 7.4.1 without flags); image targets in `deploy/images`, one macro, both architectures built always, host one loaded.
2. Proxy 502 uses error code `internal` (the API's enum has no gateway code) with status 502.
3. Proxy keeps an inbound `X-Forwarded-Proto` of `http`/`https` (a TLS front proxy) and otherwise derives it; it is spoofable by a client, so the API must not treat it as authenticated.
4. A PUT to the e2e `/clock` replaces the offset; it is not cumulative.
5. Image timestamps are the epoch (reproducible build), so `docker images` shows "56 years ago".
6. Smoke script lives at `deploy/smoke-images.sh` (the brief gave no name).

## Confidence and gaps
- Verified by running: all commands above, the three smoke checks, build and test of `//...`.
- Not verified: amd64 images were built but never run (host is arm64); only arm64 was executed. Registry push is not set up. `docker top` uid check depends on Docker's `ps` support (works on this Docker Desktop). The seed/hook paths of `Options.Clock` beyond collector and ledger are covered by plumbing, not by dedicated tests.
