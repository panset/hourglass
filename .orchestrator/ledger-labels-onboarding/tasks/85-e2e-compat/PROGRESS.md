# Progress: 85 e2e-compat

- 2026-11-23 Read brief, design (Tests, Compatibility, UPG), e2e README. Checked ports: e2e/run.sh and compose files have fixed host ports (8080, 8081, 8990, 8091, 9100) and no environment variable or option to change them. Owner's hourglass-tls project holds 8081, 8443, 8990; I do not touch it and do not edit compose mappings.
- 2026-11-23 Made a worktree of d96a950 under old-d96a950; wrote a recorder test there; produced the release database and the golden responses; fixtures committed-ready in testdata/upgrade-d96a950 (README there says how).
- 2026-11-23 Wrote TestUPG1 (internal/app/upgrade_test.go), TestUPG2 and TestUPG3 (internal/api/upgrade_compat_test.go); all pass.
- Next: end-to-end steps in e2e/browser/tests and the Keycloak user, type-check, deliberate breaks, ci.sh, remove the worktree.
- 2026-11-23 Wrote the end-to-end steps (A2g, A2f, A2p, A7g, A8p, A8c) in acceptance.spec.ts, user dave in the realm file, e2e/README.md; type-check and `playwright --list` pass. Not run: no stack could be started (see RESULT.md).
- 2026-11-23 Deliberate breaks: UPG-2 (a renamed JSON tag) and UPG-3 (a removed openapi property) went red and were restored. Worktree d96a950 removed. ./ci.sh exit 0 (ci.exit).
- Done. Status partial: e2e/run.sh both and integration not run.
