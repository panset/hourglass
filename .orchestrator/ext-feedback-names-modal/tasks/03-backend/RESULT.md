# 03 Backend result (2026-11-25)
## Changed
- internal/store/users.go: UpsertOIDCLogin `name = CASE WHEN ? = '' THEN name ELSE ? END`; test TestAUTH2_SignInWithoutNameKeepsStoredName (internal/store/store_test.go).
- internal/api/admin_views.go: `display_name` on adminUserView (userDisplayName), sortName from display name, sortUsers pins local account first.
- api/openapi.yaml (AdminUser.display_name, required), ui/src/api/schema.gen.ts (regenerated with `npm run gen:api`).
- Tests: internal/api/display_names_test.go (new; order, display_name fallback, holder named/unnamed for developer and admin, name kept after nameless sign-in), admin_users_test.go (order, key list), admin_responses_test.go (order check ignores the local account), upgrade_v110_compat_test.go (display_name allowed addition on the admin users routes), internal/api/BUILD.bazel.
- Docs: D100 in docs/decisions.md; AUTH-9 (amended by D100, covers AUTH-2) in docs/design/ledger-labels-onboarding.md.
## Checks
- Other person-naming responses (holder.name, actor_name, released_by, audit actor, preapproval created_by) already use userDisplayName; the holder test pins the stored-name and unnamed-holder cases at handler level.
- `go test ./internal/...`: exit 0, 617 top-level tests passed (gotest.log).
- `./ci.sh`: all Go targets and ui:test pass; only //ui:typecheck fails: ui/src/pages/admin-shared/fixtures.ts:17 lacks the now-required `display_name` (not my file; the UI agent must add it). See ci.log.
## Confidence and gaps
- DisplayName does not trim whitespace-only names (existing helper reused as told). internal/auth/oidc_test.go is not gofmt-clean (pre-existing, untouched).
