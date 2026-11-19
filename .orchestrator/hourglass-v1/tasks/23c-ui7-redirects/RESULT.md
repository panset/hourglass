# 23c RESULT: backend no longer depends on UI routes (UI-7). Status: done

## What changed
- `internal/auth/oidc.go`: callback success always redirects to `/` (the unauthorized branch and the now-useless loadUser call are gone); every failure redirects to `/?auth_error=<code>`. A grep of non-test Go in internal/, cmd/, pkg/ found no other UI route.
- `internal/auth/oidc_test.go`: expectations updated (`/?auth_error=unauthenticated`, `/` for the no-role user).
- `internal/api/registration_test.go`: new `TestUI7_NoGoCodeNamesAUIRoute` (AST scan of string literals, so comments are ignored) over internal/, cmd/, pkg/ for the seven UI routes.
- `api/openapi.yaml`: `/auth/callback` description updated. `schema.gen.ts` regenerated: no change (descriptions not emitted; drift test passes).
- `ui/src/shell/Gate.tsx`: anonymous visitor at `/?auth_error=x` is sent to `/login?auth_error=x`. `ui/src/pages/login/index.tsx` reads `auth_error` (old `error` removed), same messages. Tests: `login.test.tsx` updated, `Gate.test.tsx` gained a UI-7 forwarding test.
- e2e acceptance.spec.ts, docs/deploying.md, docs/extending.md: no change needed (no reliance on old redirects).

## Commands
- `go test -race -count=1 ./...` exit 0; `./ci.sh` exit 0 (35 bazel tests pass); vitest login+shell pass.

## Decisions for docs/decisions.md
| D26 amendment | 2026-11-19 | After OIDC sign-in the backend always redirects to `/`; failures go to `/?auth_error=<code>`. The UI shell decides sign-in vs not-authorized from `GET /api/v1/session` and forwards `auth_error` to the sign-in page. D26's "`/unauthorized` for a user without a role" no longer applies. | UI-7 | agent |

## Confidence and gaps
- Under Bazel the UI-7 scan only sees the packages listed in `internal/api/BUILD.bazel` data (same limit as the existing registration scan); under `go test` it sees everything. Widening needs go_sources deps in other BUILD files (outside my scope).
- The `docs/decisions.md` D26 text still states the old behaviour; the orchestrator should amend it. e2e was not run (Docker not allowed).
