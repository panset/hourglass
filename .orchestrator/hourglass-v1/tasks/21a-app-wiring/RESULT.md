# RESULT 21a-app-wiring

Status: done. `./ci.sh` exited 0 (log: ci.log; 20 of 20 Bazel tests pass, including the new `//internal/config`, `//internal/app`, `//internal/api`). `go test -race` passes on `internal/app`, `internal/api`, `internal/config`, `pkg/hourglass`. `tools/check_spdx.sh` and `tools/check_core_generic.sh` pass. Ran the built binary by hand (below).

## Files changed
- New: `internal/config/{config.go,config_test.go,BUILD.bazel}`
- New: `internal/app/{app.go,seed.go,metrics.go,app_test.go,seed_test.go,metrics_test.go,BUILD.bazel}`, `internal/app/runhook/{runhook.go,BUILD.bazel}`
- New: `internal/api/{api.go,routes.go,middleware.go,health.go,helpers.go,handlers_user.go,handlers_admin.go,router_test.go,helpers_test.go,testenv_test.go,BUILD.bazel}`
- New: `internal/store/{app_queries.go,app_queries_test.go}`; BUILD regenerated
- New: `pkg/hourglass/run.go` (BUILD regenerated)
- Rewritten: `cmd/hourglassd/main.go` (BUILD regenerated)
- `go.mod`, `go.sum`, `MODULE.bazel` (added `gopkg.in/yaml.v3` v3.0.1 only; `MODULE.bazel.lock` unchanged)
- This folder: PROGRESS.md, RESULT.md, ci.log

## Config schema (`internal/config`)
YAML file (path: `-config` flag, else `Options.ConfigPath`, else `HOURGLASS_CONFIG`; none = defaults + env). A named file that is missing is an error. Unknown keys anywhere (also inside `provider`/`pressure`/`metering`/`hooks` entries, which allow only `type` and `config`) are errors.
```yaml
listen: ":8080"            # HOURGLASS_LISTEN
database: hourglass.db     # HOURGLASS_DATABASE
session_key: ""            # HOURGLASS_SESSION_KEY (>= 32 bytes; empty = random per start, warning logged)
timezone: UTC              # HOURGLASS_TIMEZONE (IANA name)
admin_password: ""         # HOURGLASS_ADMIN_PASSWORD (only read when the local admin is first created)
oidc: {issuer, client_id, client_secret, redirect_url}   # HOURGLASS_OIDC_ISSUER / _CLIENT_ID / _CLIENT_SECRET / _REDIRECT_URL
provider: {type, config}   # optional
pressure: {type, config}   # empty type = the default passed to Run
metering: {type, config}   # empty type = the default passed to Run
hooks: [{type, config}]    # in order
seed: ""                   # path of the seed file (SET-7)
ui: {serve: true}          # HOURGLASS_UI_SERVE
cors: {allowed_origins: []}# HOURGLASS_CORS_ALLOWED_ORIGINS (comma separated)
metrics: {listen: ""}      # HOURGLASS_METRICS_LISTEN; off when empty; must differ from `listen`
```
Env overrides the file; an empty env value counts as unset. Each `config` block is converted from YAML to JSON (`json.RawMessage`) and given to the factory untouched. `Config.Redacted()` returns a map for logging: `session_key`, `admin_password`, `oidc.client_secret` become `"[redacted]"` when set, and every extension `config` block is `"[redacted]"` (it may hold secrets). `config.YAMLToJSON` is exported (used for the seed). `Run` logs the redacted view at start.

## Seed (SET-7, `internal/app/seed.go`)
`ImportSeed(ctx, st, path, now)`: no-op for an empty path; if any setup exists the file is not read at all; otherwise one `store.Tx` creates setups, replaces the `rate` rules, and writes `default_weight`, re-checking "no setups" inside the transaction. Format (`.json` = JSON, else YAML; both decoded strictly, unknown keys rejected):
```yaml
default_weight: 1.5            # optional
setups:  [{name, labels: {k: v}, details: {...}, enabled: true (default), resource_name_override: ""}]
rate_rules: [{selector: "tier=large", weight: 2}]   # list order = position
```
Validation as the admin API will: non-empty unique names, labels via `rules.ValidateLabels` (SET-3), details valid JSON <= 256 KB (SET-2), resource name derivable (SET-4), selector via `rules.Parse`, weights finite and >= 0. Invalid input returns `*SeedError`; Run then fails startup and nothing is written.

## `hourglass.Run` and the import cycle
`pkg/hourglass/run.go` defines `Options` and `Run`; `Run` calls `internal/app/runhook.Call`. `runhook` imports nothing from the module (it has its own mirror of Options and a `var Impl`). `internal/app` sets `runhook.Impl = Run` in `init()` and imports `pkg/hourglass` freely. So the graph is `pkg/hourglass -> internal/app/runhook <- internal/app -> pkg/hourglass`, no cycle. A binary must link `internal/app` (cmd/hourglassd does with a blank import); otherwise `Run` returns `runhook.ErrNoImpl`.
Run order: flags and config; JSON slog logger (stdout); build extensions through the registry (unknown name fails with the registry's list; done before the store is opened so a bad name leaves no database); open store; `EnsureLocalAdmin`; seed; events dispatcher; ledger service; collector; holds service; auth; router; listeners; collector goroutine; serve until ctx is cancelled. Shutdown (one 10-second bound): HTTP (and metrics) shutdown, stop collector and wait, events `Close`, store `Close`. A server error also triggers the same shutdown and is returned.

## `api.Deps` (what handlers receive, as `a.d` on `*API`)
```go
type Deps struct {
    Store *store.Store; Auth *auth.Service; Holds *holds.Service; Ledger *ledger.Service
    Events *events.Dispatcher; Collector *collector.Collector   // Collector.LastPoll() = in-memory poll view
    Clock Clock; Location *time.Location; Logger *slog.Logger
    Runtime Runtime   // {Provider, PressureSource, MeteringPolicy string; Hooks []string}: names for the settings/collector "runtime" part
    Fallback http.Handler  // non-API paths (UI later); nil = plain 404
}
```
Only `Store`, `Auth` are required by `New`; the rest may be nil in tests. Handlers run behind `auth.RoleRoute` or `auth.AdminRoute` (role/admin check, forced-password guard, CSRF), so `auth.UserFrom(ctx)` is a role-holding active user. Note `holds.Service` takes the user id; use `auth.UserFrom`.

## Router
`api.Routes()` (copy of the table in `routes.go`: Method, Pattern, Access, OperationID; the 30 operations of openapi.yaml; `Route.StateChanging()`) is the single source for registration and for the future contract test. `New(Deps)` mounts auth's routes via `(*auth.Service).Register`, every other route behind `guard(access)`, JSON `not_found` catch-alls for `/api/` and `/auth/` (any method, so a wrong method on a known path is also JSON `not_found`), and `Fallback` or a plain 404 on `/`. Outer to inner: request log, recoverer, `Cache-Control: no-store` (paths `/healthz`, `/api/`, `/auth/`), CORS, `Authenticate`, local-login hook, mux. The log carries method, path (no query), status, duration only.

## Helpers (`internal/api/helpers.go`)
`writeJSON`, `decodeJSON(w,r,v) bool` (JSON content type, 1 MiB `MaxBodyBytes`, unknown fields rejected with `field`, single document), `pathID`, `periodParam(w,r,current)`, `tokens(mt) float64`, `milliFromTokens(float64) (int64, error)` (rounds, rejects NaN/Inf/>1e12), `notImplemented(w)`, `(*API).writeError(w,r,err)` / `classify(err)`: `*holds.Error` code as is; `*events.VetoError` -> `hook_veto`; `*settings.FieldError` -> `validation`+field; `ledger.ErrLocalAdmin` -> `local_admin_protected`; `ledger.ErrNoRole` -> `conflict`; `ErrReasonRequired` -> `validation`/reason; `ErrZeroAmount` -> `validation`/amount_tokens; `store.ErrNotFound` -> `not_found`; `ErrSetupHeld` -> `setup_held`; `ErrConflict` -> `conflict`; anything else is logged and returned as `internal` with a fixed message. Status always comes from `apierr.Status`.

## Which stub belongs to which later task
- `internal/api/handlers_user.go` (user-data task): `getMe`, `listMyHolds`, `listMyLedger`, `getMyUsage`, `getPressure`, `listSetups`, `acquireSetup`, `releaseHold`.
- `internal/api/handlers_admin.go` (admin task): `listAdminUsers`, `setUserRoles`, `rejectUser`, `grantTokens`, `resetBalance`, `createSetup`, `updateSetup`, `deleteSetup`, `getRules`, `replaceRules`, `getSettings`, `updateSettings`, `getCollector`, `listAudit`.
Stubs answer 501 `not_implemented`. Neither task edits `api.go`/`routes.go`; if a task needs a new test helper it adds its own `_test.go`. New dependencies reach handlers through `Deps` (edit `api.go` and `app.go` only for that).
Implemented here: `getHealth`; served by `internal/auth`: `oidcStart`, `oidcCallback`, `localSignIn`, `changeLocalPassword`, `logout`, `getAuthMethods`, `getSession`.

## Decisions for docs/decisions.md
1. `GET /api/v1/auth/methods` and `GET /api/v1/session` come from `internal/auth` (its `Register`), not reimplemented; the router test checks their shapes.
2. `/healthz`: 200 `{"status":"ok","database":"ok","last_poll_age_seconds":N|null}`; the age is from the last successful `collector_runs` row (survives restarts); 503 with `degraded`/`error` only when the database query fails. A stale poll does not change the status.
3. 501 `not_implemented` is not in the openapi `ErrorCode` enum nor in `apierr` (out of scope); the stubs write the literal. The contract test will flag them until replaced.
4. Last login of the local admin: new store method `TouchLocalAdminLogin` (in `app_queries.go`), called from a wrapper in `internal/api/middleware.go` after a 200 on `POST /auth/local`. `internal/auth` is unchanged (no one-line hook needed).
5. Metrics: poll duration = latest finished `collector_runs` row; failures = count of `poll.failed` events seen since start (a wrapper around the collector's emitter); pressure raw/smoothed from `Collector.LastPoll()`; tokens debited = sum of `usage_samples.debit_mt` since process start (new store method `SumAllDebitsSince`, same file, with a test). All are `hourglass_*` names; counters reset at restart. `metrics.listen` equal to `listen` is a config error.
6. Logs go to `Options.Stdout`; flag errors to `Stderr`. The one deliberate secret in the log is auth's generated admin password at creation (spec).
7. Config precedence for the path: `-config` flag, then `Options.ConfigPath`, then `HOURGLASS_CONFIG`.
8. Extensions are validated before the database is opened.
9. `ui.serve` is parsed and logged but nothing serves the UI yet (later task); `Options.UI` is accepted and unused; `Deps.Fallback` is the hook.
10. Wrong method on a known API path answers JSON `not_found` (404), not 405.
11. Seed: `default_weight` is stored as the `default_weight` setting; rules as kind `rate`.

## Evidence of the manual run
Built `go build ./cmd/hourglassd`, ran with `HOURGLASS_LISTEN=127.0.0.1:18765 HOURGLASS_DATABASE=<tmp>/h.db HOURGLASS_METRICS_LISTEN=127.0.0.1:18766`, no config file: JSON log lines; admin created with a generated password; `GET /healthz` -> 200 `{"status":"ok","database":"ok","last_poll_age_seconds":2}`; `/api/v1/auth/methods` -> `{"oidc":false,"local":true}`; unknown `/api/v1/nothing` -> 404 JSON; `/metrics` on the second port showed the four metrics; SIGTERM -> "shutting down", "stopped", exit 0. `hourglassd version` prints `hourglassd 0.0.0-dev`.

## Open questions for the owner
1. EXT-5: a third-party `main` outside this module cannot import `internal/app`, so with the function-variable arrangement `hourglass.Run` would return `ErrNoImpl` for it. A public package that blank-imports `internal/app` (for example `pkg/hourglass/serve`) fixes it; I did not add one (outside my scope). Needs a decision before the EXT-5 example is written.
2. A sibling agent's git worktree at `.claude/worktrees/agent-a766a8a7d1be52821` (task 30) contains a full copy of the tree. While it exists, `bazel run //:gazelle` reports "matches multiple rules" for every import and `bazel build //...` would also see it. I ran gazelle and `./ci.sh` with a temporary `.bazelignore` containing `.claude` (outside my scope) and removed it afterwards. The repo should carry `.bazelignore` with `.claude` (or `# gazelle:exclude .claude`); ci.sh may be red until then or until that worktree is gone.
3. Contract test, later: it should compare `api.Routes()` with the 30 operations (method, path, `x-roles`, `x-csrf`).

## Confidence and gaps
- Verified by tests and the manual run above. Not tested: OIDC through the full router (auth's own tests cover it), a hook or provider that blocks shutdown beyond 10 s, behaviour with the UI fallback in a real browser.
- The router test "every route answers something other than 404" uses an unreachable OIDC issuer so the OIDC routes exist; those two routes answer 404 by design when OIDC is not configured (auth's behaviour).
- Handler panics log the stack and the panic type, never the panic value.
