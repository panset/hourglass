# Task 25: UI scaffold, result (status: done)

## What was done
`ui/` is a React 18 + TypeScript (strict) + Vite single-page app, built, type-checked and tested by Bazel (`rules_js`), with an API client typed from `api/openapi.yaml`, the session-driven shell, the public pages, shared components and placeholders for the six role pages.

## Versions
- Bazel 7.4.1 (local). Bzlmod: `aspect_rules_js` 2.3.8, `aspect_bazel_lib` 2.14.0, `rules_nodejs` 6.7.3 (6.3.0 does not know Node 20.18.0 for darwin_arm64), `bazel_skylib` 1.7.1. Node toolchain pinned to 20.18.0 (hermetic, independent of the local node 26). pnpm pinned 9.15.9 (`pnpm-lock.yaml` created with `npx --yes pnpm@9.15.9 install --lockfile-only`).
- Runtime deps: react 18.3.1, react-dom 18.3.1, react-router-dom 6.28.0 (all MIT). Dev deps: vite 5.4.11, @vitejs/plugin-react 4.3.4, vitest 2.1.8, jsdom 25.0.1, @testing-library/{react 16.1.0, dom 10.4.0, jest-dom 6.6.3, user-event 14.5.2}, typescript 5.6.3, openapi-typescript 7.4.4, @types/react(-dom). All pinned exactly. I did not run a license audit of transitive packages (see gaps).

## Bazel targets (all green: `bazel build //ui/...`, `bazel test //ui/...`)
- `//ui:dist` production build, a directory artifact (`bazel-bin/ui/dist`: `index.html`, `assets/`), for the later embed task.
- `//ui:test` Vitest + Testing Library on jsdom (63 tests, no network).
- `//ui:typecheck` `tsc --noEmit`.
- `//ui:api_schema_drift_test` regenerates types from `//:api/openapi.yaml` (`ui/scripts/gen-api.mjs`) and diffs against `ui/src/api/schema.gen.ts`; verified it fails when the file is edited and passes after restoring.
- `//:api/openapi.yaml` is exported from the root `BUILD.bazel` (`api/` has no BUILD file). If someone adds `api/BUILD.bazel`, point `//ui:openapi_copy` at that label instead.
- `bazel build //...` currently fails only in `//internal/api:api_test` (Go, another task's in-flight edits; not touched here).

## Directory layout
```
ui/
  BUILD.bazel  package.json  pnpm-lock.yaml  tsconfig.json  vite.config.ts  vitest.config.ts  index.html
  scripts/gen-api.mjs                 openapi-typescript driver (npm run gen:api, and the Bazel drift test)
  src/
    main.tsx  App.tsx  routes.tsx     routes.tsx = manifest of the six role pages (lazy)
    api/      client.ts endpoints.ts types.ts schema.gen.ts (generated, checked in)
    hooks/    usePolling.ts useApi.ts
    lib/      format.ts
    components/  Button Badge(+Chip) Table Field(+Input,Select) Modal(+ConfirmDialog) Alert(+Toast) index.ts
    shell/    session.tsx Gate.tsx Layout.tsx TopBar.tsx
    pages/    login unauthorized change-password not-found usage setups admin-users admin-setups admin-settings admin-audit
    styles/tokens.css                 design tokens from docs/ui-notes.md; component CSS is CSS modules
    test/     setup.ts mockApi.ts
```
## How a page task adds its page
Replace `ui/src/pages/<name>/index.tsx` (default export a component; keep `<div className="page"><h1>Title</h1>...`). Put the page's own components, CSS modules, API wrappers and tests in that directory. Route, nav entry, lazy loading and admin gating already exist; nothing shared needs editing. For a brand-new seventh page add one entry to `src/routes.tsx`. Import shared pieces from `../../components`, `../../hooks/useApi`, `../../lib/format`, `../../api/client` (`api.get/post/...`), and types from `../../api/types`.
Run just the UI tests while developing: `cd ui && npx vitest run src/pages/<name>`.

## Shared components (props)
- `Button {variant: "primary"|"secondary"|"destructive", block, ...button props}`; type defaults to "button". `LinkButton` same props on an `<a>`.
- `Badge {tone: "blue"|"green"|"grey"|"amber"}`; `Chip` monospace label chip.
- `Table<T> {label (aria-label), columns: {key, header ("" = action column, announced as actionLabel), render(row), align?}[], rows, rowKey, emptyMessage?, actionLabel?}`.
- `Field {label, hint?, error?, children: (controlProps) => node}` with `Input` and `Select` (spread the control props: `{(p) => <Input {...p} />}`).
- `Modal {open, title, onClose, children, footer?}` (role dialog, aria-modal, focus in/trap/restore, Escape). `ConfirmDialog {open, title, children, confirmLabel, cancelLabel?, destructive?, busy?, error?, onConfirm, onCancel}`.
- `Alert {tone: "info"|"error"|"success"|"warning"}` (error = role alert). `ToastProvider` (already in App) and `useToast().push(message, tone?)`, auto-dismiss after `TOAST_MS`.
- `useApi(fetcher, deps?, {pollMs?, enabled?}) -> {data, error: ApiError, loading, reload}`; `usePolling(fn, intervalMs = 60000, enabled = true)` pauses when hidden, refetches on focus/visible.
- `format.ts`: `formatTokens(v, {fixed?, signed?})`, `formatDuration(seconds)`, `formatDateTime/formatDate(iso, {timeZone?})`, `formatPeriod("2026-11")`, `formatPercent`, `formatMultiplier`, `formatPerHour`. Display only; nothing computes a balance, cost or projection (UI-1).
- API client: `api.get/post/put/patch/delete`, `ApiError {code, message, status, field?}`, CSRF header on non-GET from the session token, 401 -> shell sends the user to /login (opt out with `skipUnauthorizedHandler`), `VITE_API_BASE` switches credentials to `include` (UI-4).

## Behaviour implemented
Shell (UI-5, AUTH-5): session load; none -> /login; `must_change_password` -> change-password screen only; empty roles -> /unauthorized only (any URL); otherwise layout. Top bar per docs/ui-notes.md: name, nav (admin group only for admins, "Users · N pending"), pressure pill `Lab 82% · 2×`, balance (hidden for local admin; /me not called for it), user, Sign out. `/me`, `/pressure` and the pending count poll every 60 s (UI-6). Non-admins on `/admin/*` get the not-found page. Login (AUTH-6), unauthorized (exact sentence, email, sign out), change password (12-char minimum, server errors by field), not-found.

## Run the dev server against a local hourglassd
Start `hourglassd` (API on :8080 by default; see docs/deploying or its config), then:
```
cd ui && npx --yes pnpm@9.15.9 install && npm run dev        # http://localhost:5173
```
Vite proxies `/api`, `/auth`, `/healthz` to `VITE_PROXY_TARGET` (default http://localhost:8080), so the browser sees one origin and the cookie and CSRF token work unchanged. For a direct cross-origin setup instead: `VITE_API_BASE=http://localhost:8080 npm run dev` and list `http://localhost:5173` in `cors.allowed_origins`. Regenerate types after an API change: `cd ui && npm run gen:api`.

## Files changed
New: `ui/**` (not `node_modules`), this folder. Edited: `MODULE.bazel`, `MODULE.bazel.lock` (auto-updated by Bazel), `.bazelignore` (`ui/node_modules`, `node_modules`), `BUILD.bazel` (`# gazelle:exclude ui`, `exports_files(["api/openapi.yaml"])`), `tools/check_spdx.sh` (adds `*.mjs`, `*.html`; skips `node_modules`, `dist`, `ui/pnpm-lock.yaml`). `.gitignore` already covered `node_modules`/`ui/dist`; no change. No Go entries touched.

## Commands run
`npx pnpm@9.15.9 install --lockfile-only` ok; `npx vitest run` 63 pass; `bazel build //ui/...` ok; `bazel test //ui/...` 3/3 pass; `./tools/check_spdx.sh` exit 0; drift test negative check ok; `bazel build //... && bazel test //...` fails only on Go package `//internal/api` (not mine).

## Decisions for docs/decisions.md
1. Node 20.18.0 pinned in Bazel (LTS supported by Vite 5); rules_js 2.3.8 chosen for Bazel 7.4.1 compatibility.
2. Typecheck via `tsc` in a Bazel test rather than rules_ts (fewer dependencies).
3. Change-password screen adds a "Repeat new password" field (client-side match check); spec only requires the 12-character minimum.
4. Local admin: `/me` is not fetched; balance hidden.
5. Admin URLs for non-admins render the not-found page (UI reflects; API enforces).
6. Unauthorized page puts the full fixed sentence in the h1 (wireframe splits it into heading plus line); the spec text is exact.
7. Fonts: stack `Inter, system-ui, ...` with no web font loaded, so Inter is used only where installed.
8. Sign out is a button (it performs a POST), styled as a link in the top bar.

## Confidence and gaps
- Verified: build, tests, typecheck, drift test (positive and negative), SPDX check, dist output.
- Not verified: transitive dependency licenses (direct deps are MIT/Apache-2.0/BSD/ISC from my knowledge; no scan run); behaviour in a real browser or against a real `hourglassd` (all API calls mocked); the Vite dev proxy was not started.
- Appears to be fine but untested: a clean-checkout first fetch needs network once (Bazel repository fetch of Node and npm packages); tests themselves are hermetic.
- The `api/openapi.yaml` export lives in the root BUILD file (see above).
