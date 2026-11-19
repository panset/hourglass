# RESULT 20-auth

Status: done. All checks pass.

## Files changed
- New: `internal/api/apierr/{apierr.go, apierr_test.go, BUILD.bazel}`
- New: `internal/auth/{auth.go, password.go, ratelimit.go, session.go, middleware.go, cors.go, localadmin.go, oidc.go, handlers.go, helpers_test.go, local_test.go, session_test.go, oidc_test.go, BUILD.bazel}`
- Modified: `go.mod`, `go.sum` (added `github.com/coreos/go-oidc/v3` v3.21.0, `golang.org/x/oauth2` v0.38.0, `golang.org/x/crypto` v0.59.0; indirect `github.com/go-jose/go-jose/v4` v4.1.4; `golang.org/x/sys` v0.49.0 -> v0.50.0 via `go get`), `MODULE.bazel` (`use_repo` + `com_github_coreos_go_oidc_v3`, `org_golang_x_crypto`, `org_golang_x_oauth2`, written by `bazel mod tidy`). `MODULE.bazel.lock` did not change.
- This folder: `PROGRESS.md`, `RESULT.md`, `bazel.log`.

## Commands run (all exit 0)
- `go test -race -count=1 ./internal/auth/... ./internal/api/...`: ok (auth about 28 s under -race, mostly argon2; 33 test funcs in auth, 2 in apierr)
- `bazel run //:gazelle -- internal/auth internal/api`, `bazel mod tidy`, then `bazel run //:gazelle -- update -mode=diff internal/auth internal/api` (clean)
- `bazel test //internal/auth/... //internal/api/...`: 2/2 PASSED
- `go build ./...`, `go vet` on both packages, `gofmt -l` (clean), `tools/check_spdx.sh` OK, `tools/check_core_generic.sh` OK. (One earlier run flagged `internal/collector/collector_test.go`, another agent's file; the later run was clean.)

## Exported API and how the API task mounts it

`internal/api/apierr`: `Write(w, status, code, message)`, `WriteField(w, status, code, message, field)` (for `validation` with `field`), `WriteCode(w, code, message)`, `Status(code) int`, every `ErrorCode` from api/openapi.yaml as constants (`apierr.NoRole`, `apierr.Forbidden`, ...), types `Body`/`Detail`. Error responses also set `Cache-Control: no-store`.

`internal/auth`:
- `New(cfg Config, st *store.Store, em Emitter, log *slog.Logger, opts ...Option) (*Service, error)`. `Emitter` is `Emit(hourglass.Event)` (`*events.Dispatcher` satisfies it; nil is allowed). Options are `WithClock(func() time.Time)` and `WithHTTPClient(*http.Client)` (used for OIDC; default timeout 10 s).
- `EnsureLocalAdmin(ctx, st, initialPassword, log) (store.User, error)`: call once at startup, before serving.
- `(*Service).Register(mux *http.ServeMux)` mounts `GET /auth/oidc/start`, `GET /auth/callback`, `POST /auth/local`, `POST /auth/local/password`, `POST /auth/logout`, `GET /api/v1/auth/methods`, `GET /api/v1/session` with their own middleware.
- Wrap the root handler: `handler := svc.CORS(svc.Authenticate(mux))`.
- Data routes: `mux.Handle("GET /api/v1/me", svc.RoleRoute(h))` (any role) and `mux.Handle("PUT /api/v1/admin/settings", svc.AdminRoute(h))` (admin). Each chain is RequireRole/RequireAdmin, then RequirePasswordChanged, then RequireCSRF. `/healthz` and static files get neither.
- Building blocks: `Authenticate`, `RequireSession`, `RequireRole`, `RequireAdmin`, `RequireLocalAdmin`, `RequireCSRF`, `RequirePasswordChanged`, the package-level `CORS(origins, next)` and the method `(*Service).CORS(next)`.
- Context helpers: `UserFrom(ctx) (*auth.User, bool)` (`auth.User` embeds `store.User` and adds `Roles []string`, `HasRole`, `Authorized`), `SessionFrom(ctx) (store.Session, bool)`, `IsLocalAdmin(store.User) bool`.
- Also: `(*Service).Methods() Methods{OIDC, Local}`, `(*Service).CSRFToken(sessionID)`, `SessionView` (the openapi `Session` schema), `HashPassword`, `VerifyPassword`, `ValidatePassword`, `GeneratePassword`, constants `SessionCookie`, `CSRFHeader`, `IdleTimeout`, `MinPasswordLength`, `LocalAdminUsername`.

## Config struct expected
```go
auth.Config{
  OIDC: auth.OIDCConfig{IssuerURL, ClientID, ClientSecret, RedirectURL}, // empty IssuerURL = OIDC off
  SessionKey string,       // empty = random per start (warning logged); otherwise >= 32 bytes
  AllowedOrigins []string, // cors.allowed_origins
}
```
The initial admin password (`HOURGLASS_ADMIN_PASSWORD`) goes to `EnsureLocalAdmin`, not into Config.

## Security decisions and reasoning
1. **Passwords** (password.go:55,71): argon2id with t=3, m=64 MiB, p=4, a 32-byte key and a random 16-byte salt (RFC 9106's second recommended setting). The hash is stored as a PHC string. Verification reads the parameters from the stored string, bounds them, and compares in constant time. Inputs are capped at 1024 bytes so a large body cannot be used to burn CPU. The 12-character minimum counts runes, not bytes.
2. **One mutex for local password checks** (handlers.go:106,156): only one argon2 computation runs at a time. Without this, N concurrent attempts would use N×64 MiB of memory, and a parallel burst could all pass the limit check before any failure was recorded, so the limit would not be exact.
3. **Rate limit** (ratelimit.go): a sliding window of 5 failures per 15 minutes, held in memory, counted for the endpoint as a whole. Only one account can sign in locally, so "the account" and "the endpoint" are the same thing, and attempts with other usernames also count. While limited, the endpoint returns 429 `rate_limited` with `Retry-After` and does not check the password. A success does not clear earlier failures. A wrong `current_password` on the change-password route also counts as a failure, so a hijacked session cannot be used to guess the password without limit.
4. **Equal timing for unknown users**: unknown usernames and OIDC users are checked against a dummy hash made at startup, so they cost one argon2 run and return the same 401 body as a wrong password.
5. **Sessions** (session.go): the cookie holds 32 random bytes encoded as base64url. The database stores only the SHA-256 hex of those bytes, so a leaked database cannot be replayed as cookies. The cookie is HttpOnly, Secure, SameSite=Lax, Path=/, and has no Max-Age (it ends with the browser session). Idle expiry is checked on the server with `now >= expires_at`, and an expired row is deleted when it is seen. `last_seen_at`/`expires_at` are updated at most once a minute. On sign-in, the session the request already carried is deleted, which prevents session fixation. Expired rows are pruned whenever a new session is created.
6. **CSRF**: the token is the hex HMAC-SHA256 of the session id, compared in constant time. It is required on every method except GET, HEAD and OPTIONS. **Deviation to review:** the HMAC key is a subkey `HMAC(session key, "hourglass csrf v1")`, not the raw session key (auth.go:117). The OIDC flow cookie uses a second subkey, so one secret never serves two purposes. The security property is unchanged.
7. **Session key**: a configured key shorter than 32 bytes stops startup with an error that does not contain the key. An empty key means a random key is generated and a warning is logged.
8. **OIDC** (oidc.go): `state`, `nonce` and the PKCE verifier (S256) are each 32 random bytes or `oauth2.GenerateVerifier`. They are carried in the `hourglass_oidc` cookie, which is HMAC-signed JSON with an embedded expiry of 10 minutes: HttpOnly, Secure, SameSite=Lax (so the IdP's top-level redirect carries it), Path=/auth/callback, Max-Age 600. The cookie is cleared on every callback, so it can be used once. It is signed but not encrypted. The verifier is readable only by the user's own browser, which is the party PKCE protects. The callback compares state and nonce in constant time. Issuer, audience and expiry are checked by go-oidc using the service clock. The access token and ID token are discarded and never stored. A failure redirects to `/login?error=unauthenticated` (`internal` for server faults). The log records a reason, but token-endpoint errors are reduced to their status and OAuth error code, and response bodies are never logged. Scopes are `openid email profile`.
9. **Lazy discovery**: `New` does not contact the issuer. Discovery runs on the first OIDC request, and a failure is retried on the next request, so the API still starts when the IdP is down.
10. **Error bodies carry no data**: every refusal uses a fixed message. The test `wantErr` checks that the body is exactly `{"error":…}`. In the matrix test, the stand-in data route's payload never appears in a refusal.
11. **Role checks**: `RequireRole` needs status `active` and at least one role. Otherwise the user gets 403 `no_role`, and this applies to pending, rejected, active-with-no-roles, and pending-with-a-role users. `RequireAdmin` returns `no_role` for those users and `forbidden` for a developer. With no session, every guarded route returns 401.
12. **Forced password change**: the guard is on every role or admin route. The three routes registered in `Register` that bypass it are `GET /api/v1/session`, `POST /auth/local/password` and `POST /auth/logout`. **Password change rotates the session**: one transaction deletes all of the user's sessions (including the current one) and inserts a new one. The response sets a new cookie and returns a new `csrf_token` in the `Session` body.
13. **JSON only**: auth POST bodies must have `Content-Type: application/json` and are limited to 8 KiB. This means a cross-site HTML form cannot post to `/auth/local` (login CSRF) without a CORS preflight.
14. **CORS**: allowed origins are matched exactly, with no wildcard. A matching origin gets `Allow-Origin: <origin>`, `Allow-Credentials: true` and `Vary: Origin`. Preflights get methods GET/POST/PUT/DELETE and headers Content-Type and X-CSRF-Token. With an empty list the handler is returned unchanged, so no CORS headers are ever sent.
15. **Logging secrets**: the client secret, session key, cookie values, tokens and passwords are never logged. Tests assert that the client secret, access token and passwords do not appear in the logs. The one deliberate exception is the generated initial admin password, which is logged exactly once, when the account is created (spec rules list).

## Decisions for docs/decisions.md
- **Callback target follows api/openapi.yaml, not the brief.** After sign-in, a user with a role is sent to `/` and a user without one to `/unauthorized`. The brief said always `/`. Both satisfy the spec ("ends with a redirect back to the UI"). The orchestrator can change this in one line (oidc.go, `target`).
- `POST /auth/local/password` errors follow openapi: a wrong current password, a new password that is too short, or a new password equal to the current one each return 400 `validation` with `field`. A non-local session gets 403 `forbidden`.
- `POST /auth/local` returns 200 with a `Session` body. Logout returns 204.
- A configured `HOURGLASS_ADMIN_PASSWORD` shorter than 12 characters stops startup (AUTH-7 minimum). It is checked only when the account is created.
- The CSRF and flow-cookie HMAC keys are derived from the session key (item 6 above). A configured session key must be at least 32 bytes.
- The session and CSRF routes are part of auth: `GET /api/v1/session` is served by this package.

## Open questions
None blocking.

## Confidence and gaps
- Verified by running the tests above against a fake issuer and a real temporary SQLite store. It has not been run against a real IdP.
- `last_login_at` is not updated for local admin sign-ins. The store has no method for it, and `internal/store` is outside this task's scope. OIDC sign-ins do update it, through `UpsertOIDCLogin`.
- The rate limiter lives in memory per process, so a restart resets it. The spec asks for nothing more.
- If the IdP signs with an algorithm other than those listed in discovery (`id_token_signing_alg_values_supported`), go-oidc's defaults apply. This was not tested.
- With `cors.allowed_origins`, SameSite=Lax cookies are sent only between same-site origins (for example `localhost:5173` and `localhost:8080`). A cross-site dev origin would need a different cookie policy, which the spec does not ask for (inference).
