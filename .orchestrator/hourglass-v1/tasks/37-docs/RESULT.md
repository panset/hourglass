# RESULT 37-docs

Status: done. Four documents written; every command in them was run by me except those listed under "Not run".

## Files changed
- `README.md` (rewritten: summary, token model, status, two quick starts, build and test table, layout, links, out-of-scope, license)
- `docs/extending.md` (new), `docs/deploying.md` (new), `CONTRIBUTING.md` (new)
- This folder: `PROGRESS.md`, `RESULT.md`, `doc-examples/` (the throwaway module's source, kept as `*.go.txt` so Gazelle and the genericity check ignore it)
- Nothing else touched. `docs/SPEC.md`, `docs/progress.md`, `docs/decisions.md` unchanged.

## What was run, and the outcome (scratch dir under the session scratchpad, port 18180 and 18181 to 18190, temp databases)
- `go build ./cmd/hourglassd`, `CGO_ENABLED=0 go build ./cmd/hourglassd`, `go build ./...`, `go vet ./...`: ok.
- `go test -count=1 ./...`: all ok. `go test -race -count=1 ./internal/... ./pkg/... ./providers/... ./pressure/... ./policies/... ./hooks/... ./examples/...`: all ok.
- `bazel build //...` and `bazel test //...`: ok (35 of 35 tests pass). `bazel build //cmd/hourglassd:hourglassd` binary run with `ui.serve` true (`/` and `/setups` HTML, unknown `/api/` path JSON 404) and with `HOURGLASS_UI_SERVE=false` (`/` 404).
- `./ci.sh`: exit 1 at the license check, not because of my files: (a) `e2e/browser` has a `package.json` but no `pnpm-lock.yaml` (task 35 work in progress), (b) `docs/third-party-licenses.md` is stale. I then ran the remaining steps one by one: SPDX, genericity, `go build`, no-cgo build, `go vet`, `bazel build`, `bazel test` pass; the Gazelle diff fails because of `.orchestrator/hourglass-v1/tasks/23-verify-auth-api/evidence/zz_mutant_probe_test.go` (Gazelle wants a BUILD file there).
- README quick start 2, executed literally from the extracted code blocks (port changed to 18180): build, config, seed, start, `/healthz`, local sign-in (`must_change_password: true`), password change with CSRF, `/api/v1/setups`, `check-provider` with the `static` provider. All as documented.
- Local admin: generated password logged once (WARN, field `password`); a short `HOURGLASS_ADMIN_PASSWORD` is refused (`password must be at least 12 characters`); a short session key is refused (`at least 32 bytes`).
- `PUT /api/v1/admin/settings` (full object required; partial body gives `default_weight is required`), audit entry written.
- Backup: `sqlite3 -readonly db ".timeout 10000" "VACUUM INTO 'x-$(date ...).db'"` against a live server, `PRAGMA integrity_check` on the copy prints `ok`; a second run onto an existing name fails (documented).
- Metrics listener on a second port: the four metric names as documented; `/metrics` on the public port is 404.
- Webhook: ran `hourglassd` with a `webhook` hook (types `[poll.failed]`, secret) and a local receiver; captured a real `poll.failed` delivery with `X-Hourglass-Event` and `X-Hourglass-Signature`; Python HMAC check matched. That payload is quoted in extending.md. The `hold.acquired` payload there is `json.Marshal` of an `hourglass.Event` with made-up values.
- Throwaway module (`doc-examples/`, replace directive to this repo): provider, pressure source, policy, hook and `main` compile; the four conformance suites pass; the binary starts with all four selected in config, answers `/healthz`, and an unknown type fails with the registered names.
- vSphere path without Docker: built `e2e/simulator`, ran it on 18989/18990, ran `hourglassd` with `deploy/config.sample.yaml` (URLs changed) and `deploy/seed.sample.yaml`: poll ok with the `vsphere` provider and pressure source; `check-provider` showed all six setups "expected but not reported" until the template is set to `{{.SetupName}}_RP` (see finding 3). Simulator control commands used in the README (`POST /pools`, `PUT /pressure`, `DELETE /pools/<name>`) work.
- `cd ui && npm run gen:api` (no diff), `npm run typecheck`, `npm test` (135 pass).
- `go test` of `examples/custom` passes.

## Not run
- Docker: `deploy/load-images.sh`, `docker compose ... up/down`, the plain `docker run` example in deploying.md (taken from `deploy/README.md` and the RESULT files of tasks 33 and 34; another agent holds the ports). The README quick-start steps in the UI (Settings, Users, acquire as alice) are from `deploy/README.md` and task 34; I did not drive a browser or a Keycloak login.
- `bazel run //deploy/images:load_hourglass`.
- `pnpm install --frozen-lockfile` and `npm run dev` (pnpm is not installed here).
- The `examples/custom/README.md` curl walkthrough (linked, not repeated).
- A hook veto over HTTP (needs an OIDC user); statements about it come from `internal/holds/holds.go` and `internal/events/events.go`.
- Private-CA trust for vCenter, and the vSphere privilege table, are inference. Plain-HTTP cookie behaviour in browsers is inference from `Secure`.

## Findings in the product (awkward, undocumented or inconsistent)
1. A 32 MB Mach-O executable named `simulator` is tracked at the repository root (commit b349a63). Almost certainly an accidental `go build ./e2e/simulator`. It bloats history and should be removed and ignored.
2. `./ci.sh` is red in the live tree: the Gazelle check sees a Go file under `.orchestrator/`; add `# gazelle:exclude .orchestrator` (and the same in `.bazelignore`) or rename the file. The license check needs `e2e/browser` handled and `docs/third-party-licenses.md` regenerated.
3. The resource name template and display label keys cannot be seeded, so the sample config plus sample seed with the `vsphere` provider shows every setup as "expected but not reported" until an admin edits Settings. Seed support for the template would remove a first-run trap.
4. `PUT /api/v1/admin/settings` requires every key; a one-key update is rejected. Documented as get, modify, put.
5. `check-provider` lives in `internal/app`, so a third-party `main` cannot offer it. `check-provider -h` and every binary built on `Run` print "Usage of hourglassd".
6. Startup order: with a bad `HOURGLASS_SESSION_KEY` the database is created, the local admin is created and a generated password is logged, and only then the key is rejected and the process exits 1.
7. `pressure_samples`, `collector_runs`, `outages` and `audit_log` are never pruned (about 288 rows a day each for the first two). Documented as "kept".
8. The event queue has no size limit, and a hook that ignores its context keeps running after its 30-second timeout (the call is abandoned, not stopped).
9. Hourglass sign-out does not end the identity provider session and the authorization request has no `prompt`, so a second user in the same browser silently signs in as the first.
10. The `vsphere` provider silently lists nothing when `datacenter` or `clusters` names something missing (the pressure source errors). From task 32; documented in deploying.md.
11. From task 38's list and still true: no config-decoding helper in `pkg/hourglass`, `Run` cannot report its bound address, `Decision.Weight == 0` cannot mean "free". Documented, not fixed. `pkg/hourglass/doc.go` still lacks the sentence "use `Snapshot.Time`, not the wall clock".
12. `hold.acquiring` carries a hold with id 0 (D25); hold events never carry `Data`; `hold.released` always carries `forced` and `reason`. Now in the event table. Calling all four conformance suites from one test function gives subtest names with `#01` suffixes; docs show one function per suite.
13. There is no way to reset the local admin password short of editing the database. Documented as a limit.
14. The compose default images are the `*-e2e` test builds with a clock control endpoint on 8091. Documented in `deploy/README.md`; docs/deploying.md says never to deploy the `*-e2e` images.

## Decisions for docs/decisions.md
- Docs name Go 1.26 as the minimum (what `go.mod` says), not the 1.27.4 used by the local toolchain.
- The no-Docker quick start uses curl against the API, because plain `go build` has no UI.
- Example sources for extending.md are kept in this task folder as `*.go.txt`, not in the repository tree, so CI tooling ignores them. If the owner wants them compile-checked in CI, move them under `examples/`.

## Confidence and gaps
- Everything above marked run was run in this session. Statements about code behaviour cite the code I read (`internal/config/config.go`, `internal/auth/*`, `internal/events/events.go`, `internal/collector/collector.go`, `internal/app/metrics.go`, `hooks/webhook/webhook.go`, `providers/vsphere/config.go`); the privilege table and private-CA note are inference.
- docs/progress.md still says documents are being written; the orchestrator updates it.
- `e2e/run.sh` does not exist yet; CONTRIBUTING.md says so and points to `deploy/smoke.sh`.
