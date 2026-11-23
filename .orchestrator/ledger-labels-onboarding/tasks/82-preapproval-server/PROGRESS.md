# Progress: 82 pre-approval server

- 2026-11-23 Read BRIEF, INTENT, CONTRACT, design D1-D4, decisions D69-D75, the sign-in code (`internal/auth/oidc.go`), `setUserRoles`, store, config, app wiring, test environments.
- Decision: the approval step is implemented in `internal/api` (`preapproval.go`, type `Preapprover`), which already has the store, the ledger service, the audit helper and `setUserRoles`; it shares a transaction-scoped helper `grantRolesTx` with `setUserRoles`. `internal/auth` defines the `Preapprover` interface and takes it through `auth.WithPreapprover`; `internal/app` builds it and passes it to both auth and the API.
- Decision: the file loader lives in `internal/app` beside the seed loader and uses the API's exported email/role validators, so file and route rules cannot drift.
- Next: migration 0007 and store queries.
- 2026-11-23 Done: migration 0007 + store queries (`internal/store/preapprovals.go`); schema version pins 6 -> 7 and `preapprovals` added to the table spec in store tests.
- 2026-11-23 Done: `auth.Preapprover` interface, `auth.WithPreapprover`, the gate in the callback (`applyPreapproval`, `emailVerified`); `api.Preapprover` with `grantRolesTx` shared with `setUserRoles`; three routes, route table, OpenAPI; permissions/contract/router tests extended (route count 40 -> 43, probes, marker `MKR-preapproval`, contract scenario).
- 2026-11-23 Done: config key + env var + redacted view; `internal/app/preapproved.go` loader; wiring in `app.Run`.
- Decision: email_verified: absent/null/true/"true" (any case) verified; false, "false" and any other value not verified (conservative reading of "not false").
- Decision: a new user whose approval step fails gets `user.pending` (they are pending).
- 2026-11-23 Done: auth-side tests `internal/auth/preapproval_test.go` pass.
- Next: api-side PRE tests with a small fake issuer, app tests for the file, generated types, sample config, gazelle, ci.
- 2026-11-23 Done: api PRE tests (sign-in with a small fake issuer, routes), app file tests, config and store tests; UI types regenerated (`npm run gen:api`, typecheck clean); sample config; Gazelle updated four BUILD files.
- 2026-11-23 Done: `go test ./internal/...` exit 0; `go test -race -count=1 ./internal/auth/ ./internal/api/` ok; `tools/check_core_generic.sh` OK.
- 2026-11-23 Done: deliberate breaks went red and were restored: PRE-2 (emailVerified always true), PRE-3 (gate status check removed; step's auth-source check replaced), PRE-5 (list route AccessRole), PRE-8 (failed step reported as approved).
- Next: `./ci.sh` to `ci.log`, then finish RESULT.md.
- 2026-11-23 Done: `./ci.sh` exit 0 (`ci.exit`, `ci.log`). RESULT.md written. Task complete.
