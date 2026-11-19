# 23b fix findings from the auth/API verification and the docs pass: result

Status: done. `go test -race -count=1 ./...` exit 0 (gotest.log), `./ci.sh` exit 0 in the working tree (ci.log) and in a scratch clone of HEAD with this diff applied and no `ui/node_modules`, no pnpm (35 of 35 Bazel tests, license check ok). A changed lockfile makes the plain license check fail (tried).

## Findings

| Finding | Result | Where / test |
|---|---|---|
| 1 MEDIUM template hang | fixed: tree walk allows only text, `.SetupName`, `.Labels`/`.Labels.key`, `index`, `print`, `printf` (plain verbs, constant format), `if`, `with .Labels`; rejects define/template/block/range/variables/other functions; 1 KB template cap, 253-char rendered cap, same check on `settings.Load` | `internal/settings/resourcename.go`, `TestSET4_Template*` (incl. the verifier's hanging template), `TestSET4_BadStoredTemplateFailsLoad` |
| 2 MEDIUM global sign-in limiter | left by instruction: scope stays per account (AUTH-7), owner reviews | none |
| 3 MEDIUM permissions suite sees only the table | fixed: `registrar` is the only caller of `Handle` in `internal/api`; auth routes arrive through it; source scan fails on any other `.Handle(`/`.HandleFunc(`/`http.Handle`/`NewServeMux(`/`http.ServeMux` in non-test files of internal/api, internal/app, internal/auth, cmd/ (allow list: the registrar lines and `internal/app/metrics.go`, the separate metrics listener). M1-style mutants (`reg.mux.HandleFunc` and `reg.register` outside the table) both turn the suite red | `registration_test.go`, `TestAUTH4_*`; Bazel `go_sources` filegroups feed the scan |
| 4 MEDIUM audit before/after null | fixed: rules audit rows are objects `{"rules":[...]}`; `decodeObject` wraps any non-object as `{"value":...}`. Other actions checked: user.roles, user.reject, setup.*, settings.update (changed keys only), ledger.*, hold force release all carry before/after; setup `details` stay size+SHA-256 by design | `TestADM3_RulesAuditShowsBeforeAndAfterLists`, `TestADM3_EveryAuditActionCarriesBeforeAndAfter` |
| 5 MEDIUM seed re-applied | fixed: marker row `seed_state` in `settings` (state applied / no_seed / existing_database), written in the seed transaction; existing data without marker is initialised; invalid seed writes no marker | `internal/app/seed.go`, `TestSET7_*` (4 new) |
| 6 LOW invalid UTF-8 details | fixed (400 `details`) | `TestSET2_DetailsWithInvalidUTF8AreRefused` |
| 7 LOW hourglass-ui dot segments | fixed: 400 on `.`/`..` segments, `%2f`, `%5c`, backslash in proxied paths | `TestUI3_UnnormalisedProxiedPathsAreRefused` |
| 8 LOW flow cookie replay | fixed: server remembers used states until expiry | `TestAUTH1_FlowCookieReplayRefused` |
| 9 LOW Go depends on UI routes | left: needs an owner decision (are `/login`, `/unauthorized` contract?) |
| 10 LOW nosniff on auth JSON | fixed | `TestAuthJSONResponsesSetNosniff` |
| 11 LOW CORS origins | fixed at config load: only `http(s)://host[:port]`, no `null`, `*`, path | `TestCFG_WholeConfigurationValidatedAtLoad` |
| 12 LOW ci.sh in fresh clone | fixed: see item 8 below | scratch clone run |
| INFO 14 startup order | fixed with item 5 | `TestStartupOrder_BadConfigurationLeavesNoDatabaseAndLogsNoPassword` |
| INFO 13, 15-20 | left (INFO, not in scope of "every LOW") | |
| docs 2 (gazelle sees .orchestrator) | already excluded in BUILD.bazel; Gazelle diff is clean | ci.sh |
| docs 6 startup order | fixed (item 5) | |
| docs 10 vsphere lists nothing | fixed in docs/deploying.md (provider errors on unknown datacenter/cluster) | |
| docs 11 Snapshot.Time sentence | added to `pkg/hourglass/doc.go` | |
| docs 2 (license check, e2e/browser) | fixed (item 8) | |

## Other items
- Item 5: `config.Validate` now checks session key length (>= 32 bytes when set), OIDC shape (issuer needs client id and redirect URL, absolute http(s) URLs), listen and metrics addresses (host:port, numeric port) and CORS origins; extensions and timezone were already checked first. The seed is imported before the local admin is created, so a bad seed no longer uses up the generated-password print. Address-in-use still surfaces only at `net.Listen`.
- Item 7: deploying.md also documents the seed-once marker and the template restrictions in one line each.
- Item 8: `tools/check_licenses.sh` now runs `tools/licensecheck.go` (Go, build tag `ignore`, needs only `go`); no node. Data file `tools/npm_licenses.json` holds each lockfile's SHA-256 and the npm inventory (`ui/pnpm-lock.yaml` via `ui/node_modules`, `e2e/browser/package-lock.json` from its own license fields). `--write` regenerated it and `docs/third-party-licenses.md` (25 Go modules incl. kin-openapi, 267 npm packages incl. e2e/browser).

## Decisions for docs/decisions.md
- SET-4 template: `if`, and `with .Labels`, are allowed on top of the brief's list; `lower`/`upper` are not offered, so not allowed.
- SET-7 marker lives in the `settings` table (key `seed_state`, ignored by `settings.Load`), not a migration.
- Config now rejects session keys shorter than 32 bytes at load (was at auth start).

## Open questions
Rate-limit scope (finding 2) and UI paths in the backend (finding 9), as before.

## Files changed
internal/{settings,api,app,auth,config}, cmd/hourglass-ui, BUILD.bazel files of internal/api, internal/api/apierr, internal/app, internal/app/runhook, internal/auth, internal/settings, cmd/hourglassd, cmd/hourglass-ui, pkg/hourglass/doc.go, docs/deploying.md, docs/third-party-licenses.md, tools/check_licenses.sh, tools/licensecheck.go, tools/npm_licenses.json. `api/openapi.yaml` unchanged. No ui/ file touched.

## Confidence and gaps
Scratch clone is HEAD plus this diff, not a commit. `//ui` targets passed on both runs.
