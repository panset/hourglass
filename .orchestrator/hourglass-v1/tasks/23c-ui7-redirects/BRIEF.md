# 23c backend must not depend on the UI's routes (UI-7)
Job intent: .orchestrator/hourglass-v1/INTENT.md. Rules: .orchestrator/hourglass-v1/CONTRACT.md.
## Problem
docs/SPEC.md UI-7: "Nothing in the Go code may depend on React, on the UI's file layout beyond one static directory, or on its routes." Today the OIDC callback in `internal/auth` redirects to the UI's client-side routes (`/login?error=...`, `/unauthorized`, `/`), and other Go code may name UI routes too.
## Change
- After OIDC sign-in the backend always redirects to `/` (the spec: "ends with a redirect back to the UI"). On failure it redirects to `/?auth_error=<code>`. No other UI path appears anywhere in non-test Go code: grep `internal/`, `cmd/`, `pkg/` for `/login`, `/unauthorized`, `/setups`, `/admin/` used as UI paths (API paths under `/api/v1/` are of course fine) and remove any dependence.
- UI: the shell already decides what to show from `GET /api/v1/session` (no session -> sign-in page, no roles -> not-authorized page). Make the sign-in page read `auth_error` from the query string of `/` (the shell forwards it when it routes an anonymous visitor to the sign-in page) and show the same messages it showed for `?error=` before; remove the old parameter.
- Update `api/openapi.yaml` descriptions for the two OIDC routes accordingly, regenerate `ui/src/api/schema.gen.ts` if the YAML change affects it, and update `e2e/browser/tests/acceptance.spec.ts` only if it relied on the old redirect targets (URL assertions should follow the UI's own routing, which is unchanged for the user).
- Add a Go test (source scan, alongside the existing registration scan) that fails if non-test Go code under `internal/`, `cmd/` or `pkg/` contains a UI route literal from a small list (`/login`, `/unauthorized`, `/setups`, `/admin/users`, `/admin/setups`, `/admin/settings`, `/admin/audit`) outside comments, naming UI-7.
- Record in RESULT.md a row for docs/decisions.md.
## Scope
May write: `internal/auth/`, `internal/api/` (tests), `api/openapi.yaml`, `ui/src/` (shell and login page and their tests; generated schema via the generator only), `e2e/browser/tests/acceptance.spec.ts` (only if needed), `docs/deploying.md` / `docs/extending.md` (only if they describe the old redirects), this task folder. You are the only agent working in the repository.
## Done when
`go test -race -count=1 ./...` and `./ci.sh` exit 0 (allow 30 minutes). Do not start Docker stacks. RESULT.md lists what changed.
