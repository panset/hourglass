# 23 verify auth and the JSON API: progress

- 2026-11-19 Read BRIEF, INTENT, CONTRACT, docs/SPEC.md, docs/decisions.md. Cloned to scratch `verify-23/hourglass` (HEAD 160bdc6); main repo working tree was clean.
- 2026-11-19 `go vet ./...` exit 0; `go test -race -count=1 ./internal/... ./cmd/... ./pkg/...` exit 0 (18 packages ok, 357 top-level tests, 828 PASS lines incl. subtests, 0 skip, 0 fail).
- 2026-11-19 `./ci.sh` in a second fresh clone: first run used a `timeout` wrapper that does not exist on macOS (my error); second run exit 1 at `tools/check_licenses.sh` (needs `ui/node_modules`; pnpm not installed). Decision: run the remaining ci.sh steps by hand (`logs/ci-steps.sh`) and report.
- 2026-11-19 Mutation runs (check 2, 3): M1, C2 survive; M1c, C1, C3, P1, spec-row add/flip, K1-K6 all fail the suite. Diffs in evidence/.
- 2026-11-19 Auth attack tests (evidence/zz_attack_test.go) run green: forgeries refused; flow cookie replay accepted; sustained lockout confirmed.
- 2026-11-19 Live server (plain go build, 127.0.0.1:18923, fake IdP :18925, metrics :18924, temp DB, sentinel secrets): business rules, AUTH-8, mass assignment, body limits, SQLi, selector DoS, template DoS (confirmed hang), CORS, headers, secrets grep (no sentinel anywhere), config errors, check-provider, SET-7 (re-seed confirmed), UI-3 fallback, proxy (dot segments forwarded).
- 2026-11-19 ci-steps (ci.sh minus license check) exit 0: bazel 212 targets, 35/35 tests. Live servers stopped. RESULT.md written; status done.
