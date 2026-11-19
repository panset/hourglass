# Contributing to Hourglass

Thank you for helping. This page covers what you need installed, how to build and test, the rules CI enforces and why, and how to make the common changes.

## Before you change behaviour

[docs/SPEC.md](docs/SPEC.md) is the contract. Build what it says and cite its requirement ids (`AUTH-1`, `HOLD-2`, `RULE-3`) in commit messages and test names. Do not add a feature it does not ask for. Where it is silent, choose the simplest option and record the choice in [docs/decisions.md](docs/decisions.md): a numbered row with the date, the decision and why. If a change would contradict the spec, stop and raise it instead of working around it.

## Prerequisites

| Tool | Version | Needed for |
| --- | --- | --- |
| Go | 1.26 or newer (`go 1.26.0` in `go.mod`; Bazel downloads 1.27.4 for its own builds) | Everything |
| Bazel | 7.4.1, pinned in `.bazelversion` (no Bazelisk needed) | `bazel build //...`, `bazel test //...`, the UI build, the images |
| Node | Bazel downloads Node 20.18.0 and pnpm 9.15.9 for its own builds. For `tools/check_licenses.sh` and the UI scripts you need `node` on your `PATH` (the project has been run with Node 26) | The license check, UI scripts |
| pnpm | 9.15.9 | `pnpm install --frozen-lockfile` in `ui/` |
| Docker | 27 with Compose v2 | Only the images and the local stacks |
| `sqlite3` | any recent | Only the backup command in [docs/deploying.md](docs/deploying.md) |

The Go dependencies are pinned in `go.mod` and `go.sum`. Bazel module versions are pinned in `MODULE.bazel` and `MODULE.bazel.lock` (rules_go 0.59.0, Gazelle 0.47.0, rules_oci 2.2.6).

## Build and test

```sh
go build ./...          # API-only binary, no Node or Bazel needed
go vet ./...
go test ./...
go test -race -count=1 ./internal/...    # one package tree, with the race detector

bazel build //...
bazel test //...
./ci.sh                  # everything CI runs, in order
```

`./ci.sh` runs: the SPDX check, the core-genericity check, the license check, `go build ./...`, a `CGO_ENABLED=0` build of `./cmd/hourglassd` (the no-tag path must build with no Node, Bazel or UI files), `go vet ./...`, the Gazelle check, `bazel build //...` and `bazel test //...`. It stops at the first failure. The repository has no hosted CI configuration; `./ci.sh` is the CI script, so run it from your CI system.

For the UI, install dependencies once and then use the scripts in `ui/package.json`:

```sh
cd ui
pnpm install --frozen-lockfile
npm run typecheck
npm test
npm run dev        # Vite dev server
```

For UI development from another origin, set `HOURGLASS_CORS_ALLOWED_ORIGINS` on the server to the dev server's origin.

The end-to-end test through a real browser is `e2e/run.sh <bundled|split|both>` (about 12 to 25 minutes per mode, needs Docker; see `e2e/README.md`). `deploy/smoke.sh bundled` and `deploy/smoke.sh split` are the quick check: they start a local stack from the product images, check it, and tear it down (about 2 to 5 minutes).

### Scale test

The spec requires every API call to answer in under 500 ms with up to 200 users, 100 setups and 5-minute polling. `TestNFR_ApiUnder500msAtSpecScale` in `internal/api` checks it. In a normal `go test` it runs on a reduced data set of the same shape at two sizes and fails if any GET route runs more SQL statements on the larger one (a per-row query) or answers slowly. The full-size run seeds 200 users, 100 setups, 100 active holds and 18 days of 5-minute polls (about half a million usage samples and ledger debits, about a minute to seed), requires every GET route under 500 ms, and logs each route's time and one collector poll:

```sh
HOURGLASS_SCALE_TEST=1 go test ./internal/api -run NFR_ApiUnder500msAtSpecScale -v -count=1 -timeout 20m
```

Run it after changing a store query, a list handler or the ledger. `go test -race` runs the reduced version only.

## Rules CI enforces

| Rule | Check | Why |
| --- | --- | --- |
| Every source file starts with an SPDX MIT line | `tools/check_spdx.sh` | The project is MIT and every file says so |
| The core stays generic | `tools/check_core_generic.sh` | Anything specific to one deployment must live in an extension, the sample config or the seed file, so other teams can use the core unchanged |
| Dependencies are permissively licensed | `tools/check_licenses.sh` | The spec allows only permissive licenses |
| Gazelle's `BUILD.bazel` files are current | `bazel run //:gazelle -- update -mode=diff` in `ci.sh` | `bazel build //...` must work from a clean checkout |
| Generated UI API types match `api/openapi.yaml` | Bazel test `//ui:api_schema_drift_test` | The UI's only interface to the backend is that file |
| API routes, roles and CSRF match `api/openapi.yaml`, and every permissions-table cell holds | Go tests in `internal/api` (`contract_test.go`, `permissions_test.go`) | A user with no role must get no data from any route |

### SPDX headers

The first five lines of each source file must contain `SPDX-License-Identifier: MIT`, in the file's comment syntax (`//` for Go and TypeScript, `#` for shell, YAML and Bazel, `--` for SQL). The check covers `*.go`, `*.sh`, `*.bzl`, `BUILD.bazel`, `MODULE.bazel`, `*.ts`, `*.tsx`, `*.js`, `*.mjs`, `*.css`, `*.html`, `*.sql`, `*.yaml` and `*.yml` that git tracks or could add. It skips `docs/`, `.orchestrator/`, lockfiles, `go.sum`, `node_modules` and `dist`. It needs bash and git.

### The core stays generic

The core is `internal/`, `pkg/hourglass/` and `pkg/hourglasstest/`. The check fails when:

1. a core Go file, tests included, imports an extension package (`providers`, `pressure`, `policies`, `hooks` or `examples` under this module) or `govmomi`;
2. any core file of any type contains one of these terms, ignoring case: `vcenter`, `vsphere`, `govmomi`, `vcsim`, `resource pool`, `resourcepool`, `extralarge`, `size=`, `keycloak`. Comments and test data count;
3. a file outside `providers/vsphere/`, `deploy/` and `e2e/` imports `govmomi`.

The matching is a plain substring search, so even a comment that mentions a term fails. Rephrase it, or move the code to an extension. Wiring lives in the composition roots (`cmd/hourglassd`, `e2e/`), which are not core.

### Dependency licenses

The base policy allows MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0 and ISC. Anything else needs a line in `tools/licenses_allow.txt`:

```
SPDX-id|exact module or package name|one-line justification
```

An entry applies only to that id on that name, and an entry that no longer occurs fails the check. Today it holds three build or test-only npm packages. The script also fails when `docs/third-party-licenses.md` is out of date: regenerate it with `tools/check_licenses.sh --write` and commit the result. It needs `node`, the Go modules in the module cache (`go mod download`) and `ui/node_modules`.

### Gazelle

Run `bazel run //:gazelle` after you add, remove or rename Go files or change imports, and commit the changed `BUILD.bazel` files. The `ui/` package is excluded and its `BUILD.bazel` is written by hand. Do not leave `.go` files in directories you do not want Bazel to see: Gazelle would generate rules for them and the check would fail. When you add a Go dependency, run `go get` and add the repository to the `use_repo(go_deps, ...)` list in `MODULE.bazel`.

### Generated UI types

`ui/src/api/schema.gen.ts` is generated from `api/openapi.yaml`. After you edit the OpenAPI file:

```sh
cd ui && npm run gen:api
```

Commit the result. The drift test fails when the checked-in file is stale.

## Add an API route

OpenAPI first, then Go. The contract and permissions tests tell you what is missing.

1. Describe the operation in `api/openapi.yaml`: path, method, `operationId`, request and response schemas, error responses, `x-roles` (`public`, `session`, `role`, `admin` or `local-admin`) and `x-csrf` (true for state-changing calls).
2. Add the route to the table in `internal/api/routes.go` with the same method, path, access level and operation id.
3. Write the handler in `internal/api` (`handlers_user.go` or `handlers_admin.go`). Handlers decode the request, call a service in the core and write JSON. Business rules belong in the services (`holds`, `ledger`, `settings`), not in the handler. Return errors through the shared helpers so they use the standard `{"error": {"code", "message"}}` shape.
4. Run `go test ./internal/api/...`. The contract test fails if the route table and the OpenAPI file disagree on routes, roles or CSRF, or if a real response does not validate against its schema. The permissions test sends every route as every kind of user, so it fails if your route leaks data to a user with no role.
5. Regenerate the UI types (above) and use them in the UI.
6. Add a test named for the requirement, for example `TestHOLD2_ReleaseRefusedWhileDeployed`.

A new capability row in the spec's permissions table also needs a row in the permissions matrix test.

## Add a migration

Schema changes are numbered SQL files in `internal/store/migrations/`, embedded in the binary and applied at start, each in a transaction.

1. Create the next file: `0003_short_name.sql`. The name is a four-digit number, an underscore and a name. The loader reads the number as an integer and applies migrations in numeric order, so keep numbers unique and increasing.
2. Start the file with `-- SPDX-License-Identifier: MIT`.
3. **Never edit a migration that has shipped.** Databases that already applied it would not change. Write a new migration instead.
4. Run `bazel run //:gazelle` so the file is listed in `embedsrcs` in `internal/store/BUILD.bazel`.
5. Update the store tests that count migrations (`internal/store/store_test.go`) and, if you added or changed columns, the data-model checks there.
6. Remember there are no down migrations. A database newer than the binary is refused at start.

## Commit messages

Short imperative subject, with the requirement ids it implements in parentheses at the end. A body explains why, wrapped at about 72 characters. For example:

```
Add permissions-table tests and the OpenAPI contract test (AUTH-4, AUTH-5, AUTH-8, UI-2)

Every cell of the spec's permissions table and every route for every
actor is exercised over HTTP.
```

Keep commits small and focused. Do not rewrite history that has been pushed. Never commit secrets, real credentials or proprietary setup data: samples use obviously fake values such as `dev-only-not-a-secret`.

## Where decisions are recorded

- [docs/decisions.md](docs/decisions.md): choices made where the spec is silent, and spike results. Mark ones the owner should look at as **REVIEW**.
- [docs/progress.md](docs/progress.md): what is finished and what is not.
- `.orchestrator/`: task briefs and results from the project's build, kept for reference.

## License of contributions

Hourglass is released under the [MIT License](LICENSE). By contributing you agree that your contribution is licensed under the same terms.
