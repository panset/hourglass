# PROGRESS 20-auth
- 2026-11-19 Read BRIEF, INTENT, CONTRACT, spec AUTH-1..8, openapi auth routes, store/events results.
- Next: add deps (go-oidc, oauth2, x/crypto), write internal/api/apierr and internal/auth.
- 2026-11-19 Added deps via go get + go mod tidy: go-oidc/v3 v3.21.0, oauth2 v0.38.0, x/crypto v0.59.0 (go-jose/v4 indirect; x/sys bumped to v0.50.0).
- 2026-11-19 Wrote internal/api/apierr (+test) and internal/auth: auth.go, password.go, ratelimit.go, session.go, middleware.go, cors.go, localadmin.go, oidc.go, handlers.go. go build/vet OK.
- Decision: callback redirects to "/" with a role, "/unauthorized" without (api/openapi.yaml oidcCallback), not always "/" as the brief says; both satisfy the spec.
- Decision: password change rotates the session (delete all user sessions, new cookie + CSRF token) in one tx.
- Next: tests (fake OIDC issuer etc.), then gazelle/MODULE.bazel, bazel test, tool scripts.
- 2026-11-19 Tests written (32 test funcs); go test -race passes (~29 s). Next: gazelle + MODULE.bazel use_repo, bazel test.
- 2026-11-19 Gazelle + bazel mod tidy done; bazel test 2/2 pass; go build, spdx, genericity OK. Added JSON content-type requirement + test. RESULT.md written. Done.
