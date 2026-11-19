# PROGRESS 22-permissions-contract

- 2026-11-19 read brief, INTENT, CONTRACT, 21d RESULT. Added kin-openapi v0.133.0 to go.mod/go.sum (go get).
- 2026-11-19 additional item 1 done: `holds.AcquireBlock` (+ test), `can_acquire`/`cannot_acquire_reason` on GET /me and openapi, schema.gen.ts regenerated, `ui/src/pages/setups/index.tsx` uses the fields, setups/usage tests updated and green (25 tests), `tsc` clean.
- 2026-11-19 additional item 2 done: every API timestamp goes through `ts`/`tsPtr` (helpers.go, RFC 3339 UTC, ms, Z); user views now hold strings; existing user tests updated to the `.000Z` form.
- Decision: a fresh backend costs ~0.9 s under -race (argon2 for the local admin), so the permission tests share one backend per test function and probes create their own targets (new setup, user, hold) instead of fresh backends.
- Next: internal/api/permissions_test.go, then contract_test.go, BUILD files, MODULE.bazel use_repo, bite proofs, ci.sh.
- 2026-11-19 permissions_test.go written; passes; mutation (RoleRoute weakened) fails 160+ checks, restored. Next: contract_test.go.
- 2026-11-19 contract_test.go written; bites (renamed field, removed route, RFC3339 w/o ms) all fail as expected and were restored. YAML fixes: callback 400/500 -> 404, changeLocalPassword +429, timestamps doc; holds audit JSON uses ms format. Next: bazel test //internal/api/... then ci.sh, RESULT.md.
