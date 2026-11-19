# 23 verify auth and the JSON API: result

Status: **done**. Verified against commit `160bdc6` in a scratch clone; no product code changed.

## Verdict

**No CRITICAL or HIGH findings.** The authorization core holds up under attack. Roles are read from the database on every request, and a role-less user got only 401/403 from every data route, live and in tests. AUTH-8 held on every route I tried. CSRF is bound to the session, compared in constant time and header-only. OIDC rejected every token forgery I tried. No sentinel secret appeared in any log, response, error or database file. There are **five MEDIUM findings**:

- an admin-settable template can hang the collector;
- an unauthenticated attacker can keep the local admin locked out for as long as they keep sending requests;
- the permissions suite cannot see routes registered outside the route table;
- the audit API drops before/after for rule changes;
- the seed file is re-applied after all setups are deleted.

The owner's "done" criterion ("HTTP tests cover every cell of the permissions table, including no data for a role-less user") is met for every route in the route table. It is not met for routes registered outside it (finding 3).

## Findings

Severity order. All paths are relative to the repository root, at 160bdc6.

### 1. MEDIUM: resource-name template execution is unbounded (admin-triggered, persistent DoS)
- **Where:** `internal/settings/resourcename.go:37-56` (Execute has no time or size bound); `internal/settings/settings.go:90` (validation only renders the name `sample-setup`).
- **Repro (live, admin session `$C`/`$T`):** PUT `/api/v1/admin/settings` with `"resource_name_template": "{{if eq .SetupName \"sample-setup\"}}x{{else}}{{range 3000000000}}{{end}}{{.SetupName}}{{end}}"` (other keys unchanged).
  - The PUT returns 200 in 3 ms.
  - Admin `GET /api/v1/setups` then hangs (curl `-m 10` timed out). The process sits at 96% CPU.
  - On shutdown the log says `collector did not stop within the shutdown bound`.
  - `{{printf "%0999999d" 1}}` is accepted too, and yields a 1 MB resource name. Inside a `range`, this would take memory without bound.
  - Script: `evidence/live/` (`lib.sh`, `run.sh`); the commands are in this report.
- **Impact:** Text/template has no dangerous functions (`call`, `template` recursion and missing keys are all refused, which is good). But Go 1.22+ `range N` and `printf` widths let one admin setting stall:
  - every poll, so metering stops;
  - every admin setups list;
  - every release where the teardown check applies;
  - or exhaust memory, every time any setup's name is derived.
  - The guard renders only `sample-setup`, so a template can behave for the guard and explode for real names.
- **Fix:** Bound execution:
  - write into a size-capped writer, for example 255 bytes, that errors when full;
  - run with a context or deadline, for example a goroutine with a 50 ms budget, and treat a timeout as an invalid template;
  - optionally reject `range`, `define`, `template` and `block` nodes by walking `t.Tree`;
  - validate against every existing setup on save, not only `sample-setup`.

### 2. MEDIUM: the global sign-in limiter lets anyone lock the local admin out indefinitely
- **Where:** `internal/auth/ratelimit.go:16-51` (one global window); `internal/auth/handlers.go:106-128`.
- **Repro:** `evidence/zz_attack_test.go` `TestZZ_RateLimit_SustainedLockout`.
  - An unauthenticated attacker sends one bad `POST /auth/local` every 3 minutes, which is 20 requests an hour.
  - The real admin, with the right password, was refused (429) in 11 of 12 attempts over 6 hours. The only success was before the attacker's fifth failure.
- **What holds:** Case, whitespace, unknown usernames, `X-Forwarded-For` and `X-Real-IP` do not bypass the limit (`TestZZ_RateLimit_BypassAttempts`). Concurrency is exact: of 40 parallel guesses, exactly 5 were checked (`TestZZ_RateLimit_Concurrency`). Attempts while limited do not extend the window.
- **Impact:** The only break-glass account can be denied for as long as the attacker keeps going. The limiter is shared with `POST /auth/local/password`. Without OIDC it is the only way in.
- **Fix:** Keep a limiter keyed by client (RemoteAddr, never a header unless a trusted proxy is configured) at 5 per 15 minutes. Keep a global limiter only as a much higher backstop.
- **Open question:** AUTH-7 says "rate-limited to 5 failures per 15 minutes" without saying per what. This needs an owner decision; I did not resolve it.

### 3. MEDIUM: the permissions and CSRF suites only see the hand-maintained route table, not the mux
- **Where:**
  - `internal/api/api.go:86-104`: `New` registers from `routes`, but `auth.Register` and any `mux.Handle` call add patterns the tests never enumerate.
  - `internal/api/permissions_test.go:645, 714`: the tests iterate `Routes()`.
- **Repro:**
  - `evidence/mutant-M1-unguarded-route.diff` adds `GET /api/v1/setup-names` on the mux with no guard. With it applied, `go test ./internal/api/` is **green**. Yet `evidence/zz_mutant_probe_test.go` shows an anonymous request gets `200 ["MKR-setup-free","MKR-setup-held"]`.
  - `evidence/mutant-C2-unlisted-route-without-csrf.diff` adds `POST /api/v1/me/holds/release-all` with RequireRole and no CSRF. The suite stays **green**.
- **What the suite does catch:**
  - a leak added to a listed route, including in a response header (M1c);
  - CSRF removed from logout (C1) or from all admin routes (C3);
  - an admin route relabelled public in both the router and the YAML (P1);
  - a new or flipped capability row in the spec table, which the meta-test notices.
  - Every probe is a real HTTP request through the full router; no cell is skipped.
- **Impact:** The "every route, no data for role-less users" guarantee depends on contributors adding new routes only through the table.
- **Fix:** Make the mux unreachable except through the table. Either:
  - wrap registration in a recorder (`register(method, pattern, h)`) used by both `api.New` and `auth.Register`, and have a test assert that the recorded set equals `Routes()` plus the two catch-alls; or
  - have a test that greps `internal/` for `mux.Handle`/`HandleFunc` calls outside the one registration loop.

### 4. MEDIUM: the audit log API shows `before: null, after: null` for every rules change (ADM-3, RULE-4)
- **Where:**
  - `internal/api/handlers_admin.go:772, 781`: `ruleSnapshot` stores a JSON array.
  - `internal/api/admin_system.go:274-283`: `decodeObject` turns any non-object into nil.
- **Repro (live):** PUT `/api/v1/admin/rules`, then GET `/api/v1/admin/audit`. The `rules.replace` entries come back as `"before":null,"after":null`.
  - `sqlite3 hg.db "select before_json from audit_log where action='rules.replace'"` shows the data is stored, for example `[{"position":1,"selector":"size=large","weight":2}]`.
- **Impact:** Admins cannot see what a rule change was. The existing tests did not notice.
- **Fix:** Store `{"rules": [...]}` (the schema says `before`/`after` are objects), or wrap arrays in `decodeObject`. Add a test that reads a rules.replace entry back through the API.

### 5. MEDIUM: SET-7 seed is re-applied after an admin deletes every setup, overwriting rate rules without audit
- **Where:** `internal/app/seed.go:121-134` (the "first start" condition is implemented as "no setups now").
- **Repro:** `evidence/live/seedonce.sh` does the following:
  - starts with a seed and imports `seed-a`;
  - the admin changes the password and `DELETE /api/v1/admin/setups/1` returns 204;
  - restart: setups are `['seed-a']` again and the log shows `seed file imported` a second time.
  - Per the code at seed.go:158-165, the seed's `rate_rules` also replace the admin's current rules (`ReplaceRules`), with no audit row.
- **Impact:** The spec says "The seed is never applied again". Here, admin deletions are silently undone and the admin's rules are replaced on the next restart.
- **Fix:** Record "seed applied" once, for example as a `settings` key or a migration-like marker, and check it instead of the setup count. Optionally write a system audit row when seeding.

### 6. LOW: setup `details` containing invalid UTF-8 is stored and served raw, so `GET /api/v1/setups` becomes invalid JSON
- **Where:** `internal/api/handlers_admin.go:461-469` (RawMessage length check only).
- **Repro:** POST `/api/v1/admin/setups` with body bytes `{"name":"x","labels":{},"details":{"k":"v\xff\xfe"},"enabled":true}`. It returns 201. A later `GET /api/v1/setups` contains the raw `\xff\xfe`, and Python's `json.load` fails with `UnicodeDecodeError`.
- **Impact:** SET-2 says "well-formed JSON", and RFC 8259 requires UTF-8. One bad setup breaks strict clients of the main page for every user with a role. Admin-only input. Names are fine: Go replaces invalid bytes with U+FFFD.
- **Fix:** Reject `!utf8.Valid(body.Details)` with 400 `details`.

### 7. LOW: hourglass-ui forwards paths with dot segments or encoded slashes unchanged
- **Where:** `cmd/hourglass-ui/proxy.go:64, 72-79` (the prefix check is on the decoded path; the raw path is forwarded).
- **Repro:** Run against an echo upstream (`evidence/live/echo.py`):
  - `curl --path-as-is http://ui/api/../etc/passwd` reaches the upstream as `GET /api/../etc/passwd`;
  - the same happens for `/api%2f..%2fsecret` and `/healthz/../admin`.
  - Against hourglassd today this is harmless: Go's mux answers with a 307 to the cleaned path, and that path then lands on the UI server.
- **What holds:** An absolute-form request line still goes to the configured upstream. Incoming `X-Forwarded-For` and `Forwarded` are replaced. Hop-by-hop headers are dropped. `Set-Cookie` and `Location` pass through untouched.
- **Fix:** Reject requests whose `path.Clean(p) != p` or whose `RawPath` contains `%2f`/`%5c`, or forward the cleaned path.

### 8. LOW: the OIDC flow cookie is single-use only in the browser
- **Where:** `internal/auth/oidc.go:61-87, 167` (a stateless signed cookie; the server only asks the browser to clear it).
- **Repro:** `TestZZ_OIDC_FlowCookieReplay`. After a successful callback, the same flow cookie plus a second code issued for the same state signs in again.
- **Impact:** Low. The attacker needs the HttpOnly, Secure cookie and an IdP code bound to its PKCE challenge, and the window is 10 minutes. The brief asks for "single-use".
- **Fix:** Keep a small in-memory set of used `state` values until they expire.

### 9. LOW: the Go code depends on UI routes (UI-7)
- **Where:**
  - `internal/auth/oidc.go:157` redirects to `/login?error=…`;
  - `internal/auth/oidc.go:256-258` redirects to `/` and `/unauthorized`;
  - `internal/uiserve/uiserve.go:28, 92` give `assets/` special cache and 404 handling.
- **Impact:** A replacement UI must keep these paths and this layout. A UI without `assets/` still works, but with weaker caching, and a missing asset gets the index.html fallback.
- **Open question:** The spec's page table fixes `/login` and `/unauthorized`, so this may be intended. The owner should confirm that those two paths are part of the contract.
- **Fix if not:** Make the paths configurable, or make `assets/` a config option.

### 10. LOW: auth JSON responses lack `X-Content-Type-Options: nosniff`
- **Where:** `internal/auth/handlers.go:42-47` (`writeJSON` in auth). Seen live on `GET /api/v1/session` and `GET /api/v1/auth/methods`. The API's `writeJSON` and `apierr` do set it.
- **Impact:** The session body carries IdP-controlled `name`/`email`. Low, because the type is `application/json`.
- **Fix:** Set the header in auth's `writeJSON`, or move it into the router's `noStore` middleware.

### 11. LOW: `cors.allowed_origins` accepts `null` and non-origin strings
- **Where:** `internal/auth/cors.go:17-20`; `internal/config/config.go` `splitList`. No validation.
- **Repro:** With `HOURGLASS_CORS_ALLOWED_ORIGINS="http://dev.example:5173, null"`, `Origin: null` gets `Access-Control-Allow-Origin: null` and `Allow-Credentials: true`.
- **What holds:** With the default (empty) list, no CORS headers are sent. Other origins, suffix tricks, case variants and `*` are not reflected. The preflight is exact.
- **Fix:** Reject `null`, `*` and anything that is not `scheme://host[:port]` at config load.

### 12. LOW: `./ci.sh` fails in a fresh clone
- **Where:** `ci.sh:7` → `tools/check_licenses.sh`, which needs `ui/node_modules`. ci.sh does not install it, and pnpm is not installed on this machine.
- **Repro:** `git clone … && ./ci.sh` exits 1 with `ui: node_modules not installed`. The follow-on messages (unused allow entries, out-of-date `third-party-licenses.md`) are consequences of the empty npm list.
- **What holds:** All the other ci.sh steps pass: SPDX, core-generic, go build, CGO_ENABLED=0 build, vet, gazelle diff, and `bazel build //...` / `bazel test //...` with 35 of 35 tests passing.
- **Fix:** Have ci.sh install the UI dependencies, or document the prerequisite. Alternatively, have the license check read licenses through Bazel's rules_js tree.

### 13-20. INFO
- **13.** PHC parsing is lenient but safe: trailing parameters (`m=…,t=…,p=4,x=1`) parse, and `p` up to 255 is accepted. No input panicked; m and t are bounded. `internal/auth/password.go:85-91`. Fix: an exact `fmt.Sscanf` plus a full-match check, and `p <= 16`.
- **14.** Startup creates the database and local admin, and prints a generated password, before it validates the session key. A start that then fails has used up the one-time print. `internal/app/app.go:171` vs `:199`. Fix: validate the auth config before `EnsureLocalAdmin`.
- **15.** The JSON decoder accepts duplicate keys (last wins) and case-insensitive key matches (`{"Roles":[…]}`). This is Go's default behaviour. `internal/api/helpers.go:54-56`.
- **16.** A wrong method on an existing path (for example `POST /api/v1/me`) answers JSON 404, not 405. `internal/api/api.go:99`. The spec is silent.
- **17.** Releasing a hold id that does not exist gives 404; another user's hold gives 403. That reveals whether a hold id exists, but hold ids are visible to every role user in `GET /api/v1/setups` anyway. `internal/holds/holds.go:295-305`.
- **18.** hourglass-ui keeps a client-supplied `X-Forwarded-Proto` and passes `X-Real-IP` through (`cmd/hourglass-ui/proxy.go:38, 84-92`). The API uses neither today.
- **19.** The no-data test's markers cover setup names, labels, details, user name, ledger reason and audit reason. They do not cover rules, settings, orphans, delivery errors or pressure samples. Today this is defence in depth only, because those routes are role-gated.
- **20.** An IdP user may call themselves "admin", with that name, email or `sub`. They become a separate pending OIDC user with an empty username. The local `admin` row and `POST /auth/local` are unaffected (`TestZZ_OIDC_UserNamedAdminCannotBecomeLocalAdmin`). Lists show them as "admin", but `auth_source` and `is_local_admin` tell them apart.

## Checklist

| # | Item | Result |
|---|---|---|
| 1 | `go vet ./...` | checked-ok (exit 0) |
| 1 | `go test -race -count=1 ./internal/... ./cmd/... ./pkg/...` | checked-ok: 18 packages ok, 357 top-level tests, 828 PASS lines including subtests, 0 skip, 0 fail |
| 1 | `./ci.sh` from a clean clone | finding 12. All other steps were run by hand and pass: bazel 212 targets, 35/35 tests (the bazel run reported 36 actions, so mostly cache hits) |
| 2 | every spec cell asserted by a real request | checked-ok: each capability row × 8 actors is a real HTTP request through the full router. Manager-only rows assert that no team route exists |
| 2 | route list derived from the router | finding 3. The table drives registration, but the mux can gain routes the table does not list |
| 2 | no-data test seeds markers and scans body and headers | checked-ok, with coverage gaps (finding 19). Body and headers are both scanned (M1c caught) |
| 2 | mutants: leaking route, route without CSRF, new spec row | M1, C2 survive (finding 3); M1c, C1, C3, P1, spec row added, spec cell flipped are all caught |
| 3 | contract: rename, type change, extra field, drop route, x-roles | checked-ok: K1-K6 all fail (`evidence/contract-*.diff`), including a nested field seen only in grant responses |
| 4 | session fixation (sign-in, password change) | checked-ok, live and in tests |
| 4 | cookie attributes | checked-ok: `HttpOnly; Secure; SameSite=Lax; Path=/`. The flow cookie is path `/auth/callback`, Max-Age 600 |
| 4 | idle timeout server-side; sign-out server-side | checked-ok: `session.go:117-121`, existing tests; live, an old cookie gets 401 after logout and after password change |
| 4 | CSRF unpredictable, session-bound, constant-time, every non-GET, not query/cookie/other session | checked-ok: HMAC of the session id; `TestZZ_CSRF_*`; live `/auth/logout` and `/auth/local/password` need it |
| 4 | `POST /auth/local` login-CSRF posture | checked-ok: JSON content type required (no simple form post), SameSite=Lax, no CORS by default |
| 4 | OIDC state/nonce/PKCE, ID token signature/iss/aud/exp, `sub` identity, no open redirect | checked-ok: wrong iss, alg none, HS256 signed with the client secret, foreign key, no exp, wrong aud and empty sub are all refused. Every redirect target is a constant |
| 4 | flow cookie single-use and expiring | expiring ok; single-use is finding 8 |
| 4 | OIDC user cannot become the local admin | checked-ok (finding 20 is display only) |
| 4 | tokens never stored or logged | checked-ok: the access and refresh token sentinels are found nowhere |
| 4 | rate limit: bypasses, concurrency, permanent lockout | bypasses and concurrency ok; lockout is finding 2 |
| 4 | timing unknown user vs wrong password | checked-ok: 38.0 ms vs 39.0 ms |
| 4 | argon2id parameters, malformed PHC | checked-ok: t=3, m=64 MiB, p=4; no panic (finding 13 INFO) |
| 4 | forced password change on every route | checked-ok: live, every data and admin route returns 403 `password_change_required` |
| 4 | password minimum | checked-ok (12 runes; 1024-byte maximum) |
| 4 | AUTH-8 through every route; no self-demotion | checked-ok live: role set (any), reject, grant, reset, acquire, and DELETE/PATCH/POST on users/1 (no route) |
| 4 | roles from the DB on every request | checked-ok live: a demoted admin's existing session gets 403 forbidden, then 403 no_role on the next request |
| 5 | IDOR on `{id}` routes | checked-ok: other user's hold 403; 0, -1, huge, non-numeric, `1.5` and `0x1` give 400; max int64 gives 404 (finding 17 INFO). `me/*` take no id |
| 5 | mass assignment | checked-ok: unknown fields are 400 on every API body. `/auth/local` ignores extras (harmless). Finding 15 INFO |
| 5 | body limits: 256 KB details, 50 MB, deep nesting | checked-ok: exactly 262144 bytes gives 201, +1 gives 400, 50 MB gives 400 at 1 MiB, nesting of 10k and 100k gives 400 |
| 5 | invalid UTF-8 | finding 6 |
| 5 | SQL injection in selector, label, name, period, filters | checked-ok: all store queries are parameterized (no concatenated SQL found); probes are stored or refused literally |
| 5 | selector parser DoS | checked-ok: 339 KB selectors take 10 ms; a 1 MB rules PUT takes 18 ms; 20k rules take 224 ms |
| 5 | template setting (injection, loops) | finding 1. No dangerous calls are reachable |
| 5 | stored XSS: JSON content type, nosniff, CSP | checked-ok, except finding 10. The index.html CSP is `script-src 'self'`, with no unsafe-inline or unsafe-eval |
| 5 | error bodies: no stacks, SQL, paths, others' data | checked-ok: the recoverer logs the stack server-side only. Template and type errors echo the admin's own input or Go type names only |
| 5 | `Cache-Control: no-store` | checked-ok on every API response seen, including errors and 404s |
| 5 | CORS off by default; exact origin; no wildcard or reflection | checked-ok, except finding 11 |
| 5 | `/metrics` not on the public listener; `/healthz` | checked-ok: public `/metrics` is 404; metrics are served on the separate listener only; healthz shows status, database state and poll age |
| 5 | unknown `/api/` paths JSON 404; method handling | checked-ok (finding 16 INFO) |
| 6 | secrets in logs, responses, DB, errors | checked-ok: four sentinels plus the token sentinels, after sign-in failures, an IdP error that echoes the secret, five config errors, a failing hook, check-provider and admin views. Zero hits in logs, responses, the DB, WAL or temp DBs |
| 7 | first sign-in pending; pending list contents and order; reject then approve | checked-ok live (oldest first, with name, email and first sign-in time) |
| 7 | allowance on first role, at most once per period | checked-ok live: remove, re-add, then balance is 300 with one allowance entry |
| 7 | remove all roles blocked while holding; not pending again | checked-ok live |
| 7 | local admin bootstrap (env or generated, printed once) | checked-ok (finding 14 INFO) |
| 7 | SET-2, SET-3 boundaries; SET-5 | checked-ok live: 32/33 labels, key 63/64, value 63/64 including multibyte, key grammar; disable and delete of a held setup refused |
| 7 | invalid selector on rules replace changes nothing | checked-ok live |
| 7 | settings validation | checked-ok live (16 cases) |
| 7 | grant/reset/force-release reason required | checked-ok (blank or whitespace gives 400) |
| 7 | every admin action audited with before and after, no secrets | finding 4 (rules). Setup `details` are audited as size and SHA-256 only (by design). No secrets found |
| 7 | changes apply from next poll | checked-ok by reading code: the collector loads settings and rules each poll (`collector.go:163, 295-303`) |
| 8 | unknown extension lists registered names (EXT-2) | checked-ok |
| 8 | no provider and no OIDC works (EXT-4) | checked-ok live (local-only start, inventory pressure) |
| 8 | env overrides | checked-ok (all live runs were configured by env over file) |
| 8 | seed applied once (SET-7) | finding 5 |
| 8 | `ui.serve: false`; plain build serves no UI; fallback never shadows `/api/`, `/auth/`, `/healthz` | checked-ok live (UI-tagged build with a stub dist) |
| 8 | UI-7 grep | finding 9 |
| 9 | proxy prefixes; traversal; absolute URL; hop-by-hop; Set-Cookie and redirects; X-Forwarded-* | finding 7 and finding 18 INFO; the rest are ok |

## Evidence and commands

- **Clone:** `/private/tmp/claude-501/-Users-tejaspanse-projects/17dec24c-e7ec-455e-9b87-01d2217d5fc9/scratchpad/verify-23/` contains:
  - `hourglass/`: mutants run here and reverted with `git checkout`;
  - `ci-clone/`;
  - `logs/`: `test.log`, `ci.log`, `ci3.log`;
  - `live/`.
- **`evidence/`** in this task folder:
  - `zz_attack_test.go`: drop into `internal/auth/` and run `go test -run TestZZ -v ./internal/auth/`;
  - `zz_mutant_probe_test.go`;
  - `mutant-*.diff` and `contract-*.diff`: apply each to a clean tree, then `go test ./internal/api/`;
  - `fakeidp/`: a stdlib OIDC issuer;
  - `live/`: `run.sh` starts the server and IdP with the sentinels, `lib.sh` holds the curl helpers, plus `cfgerr.sh`, `seedonce.sh`, `echo.py` and `configerr.log` (generated throwaway passwords redacted).
- **Live setup:** `go build ./cmd/hourglassd` (plain build), listening on 127.0.0.1:18923, metrics on :18924, fake IdP on :18925, UI proxy on :18926/:18928, all with temp databases. No Docker was used, and none of ports 8080/8081/8090/8091.

## Files changed

Only this task folder: `PROGRESS.md`, `RESULT.md`, `evidence/**`. The main repository is untouched.

## Decisions for docs/decisions.md

None. No product choices were made.

## Open questions for the owner

1. AUTH-7 rate limit: should it be per client or global? (finding 2)
2. UI-7: are the `/login` and `/unauthorized` redirect targets part of the backend contract? (finding 9)
3. SET-7: confirm that "first start" means "once ever", not "whenever the database has no setups" (finding 5).

## Confidence and gaps

- **Verified:** everything marked checked-ok above, by live request or test run.
- **Code reading, not executed:**
  - the rate-rule overwrite on re-seed (finding 5);
  - "changes apply from next poll".
- **The UI was not exercised.** The CSP was checked on a stub `index.html` served by the real handler, not on the real built UI.
- **Time-based checks only through existing fake-clock tests:** the 8-hour idle timeout and the 10-minute flow-cookie expiry.
- **Not attempted:** HTTP request smuggling (out of scope per the brief), and the Keycloak/compose stacks (forbidden by the brief).
- **Partial:** `ci.sh` as a single command (finding 12).
