# Progress 03-backend (2026-11-25)
- Done: UpsertOIDCLogin keeps stored name on empty sign-in name (store test added).
- Done: AdminUser.display_name (auth.DisplayName via userDisplayName), openapi + schema.gen.ts regenerated (npm run gen:api).
- Done: sortUsers local account first, then overdrawn, display name (case-insens.), id.
- Done: tests in internal/api/display_names_test.go; existing order/key/v1.1.0-compat tests updated; BUILD.bazel entry.
- Done: D100 in docs/decisions.md; AUTH-9/AUTH-2 amendment in docs/design/ledger-labels-onboarding.md.
- ci.sh: only ui:typecheck fails, in ui/src/pages/admin-shared/fixtures.ts (needs display_name; other agent's file).
