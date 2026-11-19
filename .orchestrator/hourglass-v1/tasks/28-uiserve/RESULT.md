# RESULT 28-uiserve

Status: done, with one caveat: `./ci.sh` could not be run to the end in the live tree because `//internal/api/BUILD.bazel` is stale (other agents added `admin_*.go`, `user_views.go` and new imports; their gazelle run is pending), so `bazel build //...` fails there. Everything of mine was verified in a scratch copy of the tree in which `bazel run //:gazelle -- update` had regenerated that one file: `bazel test //cmd/hourglassd/... //internal/uiserve/... //internal/app/...` passed 4 of 4 (gazelle left my BUILD files unchanged). `go build ./...`, `go test ./internal/uiserve ./internal/app`, `tools/check_spdx.sh`, `tools/check_core_generic.sh` pass in the live tree.

## What was built
- `internal/uiserve/uiserve.go`: `Handler(fsys fs.FS) http.Handler`. Knows only "index.html at the root" and "assets/ is fingerprinted"; no framework or route knowledge (UI-7).
- `embed_ui.go` (`//go:build ui`, `//go:embed all:dist`, `Embedded() (fs.FS, bool)`, rooted at dist, false if no index.html) and `embed_noui.go` (`!ui`, returns `nil, false`).
- `internal/app/app.go`: `uiFallback(opts.UI, cfg.UI.Serve, log)` sets `api.Deps.Fallback`. Bundled = `ui.serve` true and a UI linked in. Otherwise `nil` (nothing at `/`) and one log line at start: "not serving the UI: ui.serve is false (split mode)" or "... built without it (API-only)"; bundled logs "serving the bundled UI at /".
- `cmd/hourglassd/main.go`: passes `uiserve.Embedded()` as `Options.UI` (nil when absent).
- `ci.sh`: added `CGO_ENABLED=0 go build -o /dev/null ./cmd/hourglassd` after `go build ./...`.
- `ui/BUILD.bazel`: only `//ui:dist` visibility (to `//internal/uiserve`).

## Build tag
`ui`. Plain `go build ./...` has no tag, needs no Node/Bazel/dist files and yields the API-only binary (run by hand: `/` 404, `/healthz` 200, log says API-only). `go build -tags ui` needs `internal/uiserve/dist/` to hold the build output (copy `bazel-bin/ui/dist`); not checked in.

## Bazel targets
- `//cmd/hourglassd:hourglassd` = bundled (`gotags = ["ui"]`, the `//go:embed` input is `//ui:dist`). `//cmd/hourglassd:hourglassd-api` = same lib, no tag, no UI build. `go_binary.gotags` sets rules_go's `//go/config:tags` for the whole dependency tree, so one `go_library` `//internal/uiserve` serves both; its `embedsrcs` is `select({":ui_tag": [":dist"], default: []})`.
- `//internal/uiserve:dist` = `copy_directory(//ui:dist -> dist)` (aspect_bazel_lib), a tree artifact in `embedsrcs`. This works with rules_go, so the tar/zip fallback was NOT needed.
- Tests: `//internal/uiserve:uiserve_test` (MapFS handler tests), `//internal/uiserve:uiserve_ui_test` (`gotags = ["ui"]`: embedded FS has index.html and serves it, fallback works), `//cmd/hourglassd:ui_modes_test` (sh_test, runs both binaries: API-only `/` 404 + `/healthz` 200; bundled `/` HTML, `/setups` falls back, unknown `/api/` and `/auth/` paths stay 404 non-HTML, an `/assets/*.js` named by index.html is served; `HOURGLASS_UI_SERVE=false` on the bundled binary gives `/` 404). The sh_test is tagged `local` (binds loopback ports). `internal/uiserve/BUILD.bazel` carries `# gazelle:ignore` (hand-written); in cmd the `gotags` line has `# keep` and gazelle's diff for cmd was clean.
- `internal/app`: gazelle added the `//internal/uiserve` dep; `TestUI3_UIFallbackModes` added.

## CSP
`default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'`. Built `index.html` (bazel-bin/ui/dist/index.html) has exactly one external module script and one external stylesheet, no inline script or style, so no `'unsafe-inline'`/`'unsafe-eval'`. `data:` images are allowed for bundler-inlined small assets; `connect-src 'self'` is the same-origin API (UI-4). Sent on `index.html` responses (also the fallback), with `X-Frame-Options: DENY` and `Referrer-Policy: same-origin`. Inference: React sets style via CSSOM, which CSP does not block; if a page ever needs a `style=""` attribute in the markup, relax `style-src` narrowly.

## Caching and routing policy
`/assets/*` (anything under the assets dir): `public, max-age=31536000, immutable`. `index.html` (and the fallback) and other root files: `no-cache`. All responses: `X-Content-Type-Options: nosniff`. Content types from the extension (`mime.TypeByExtension`). GET and HEAD only (405 otherwise). `/api/`, `/auth/`, `/healthz` (and anything under) return a plain 404 from this handler, never HTML (in the full server the router matches API paths first). Path is `path.Clean("/"+p)` and served through `fs.FS` (rejects `..`); backslash and NUL are 404; directories are never listed (they fall back to index.html as non-files).

## Decisions for docs/decisions.md
1. A missing path under `assets/` or with a file extension is a 404, not index.html, so a stale script never receives HTML. Consequence: a client route containing a dot in its last segment would 404.
2. `cmd` always passes the embedded FS; `internal/app` decides whether to serve (it owns the config and the log), giving the one start-up log line.
3. Extra headers beyond the brief: `X-Frame-Options`, `Referrer-Policy` on index only.

## Files changed
New: `internal/uiserve/{uiserve.go,embed_ui.go,embed_noui.go,uiserve_test.go,embed_ui_test.go,embed_noui_test.go,BUILD.bazel}`, `cmd/hourglassd/ui_modes_test.sh`. Edited: `cmd/hourglassd/{main.go,BUILD.bazel}`, `internal/app/{app.go,app_test.go,BUILD.bazel}`, `ui/BUILD.bazel`, `ci.sh`, this folder.

## Confidence and gaps
- Not run: the full `./ci.sh` and `bazel build //...` in the live tree (stale `internal/api` BUILD, see top). Once the api agents run gazelle, it should pass; my targets passed against a regenerated copy.
- Not tested in a real browser; the CSP is derived from reading the built `index.html`, not from a browser console.
- `bazel-bin/ui/dist` shown at the time had other page chunks; the CSP check used index.html only.
