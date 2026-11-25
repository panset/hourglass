# 02 Release map: checks, merge, release (read-only survey, 2026-11-24)

Base: `main` = `origin/main` = `f1a05c2` (`git rev-parse main origin/main`, `git ls-remote origin refs/heads/main`). Nothing was pushed, tagged, built or changed outside this folder.

## 1. Checks

### `./ci.sh` (verified from file: `ci.sh:5-15`, `CONTRIBUTING.md:35`)
Runs in order, stopping at the first failure: `tools/check_spdx.sh`, `tools/check_core_generic.sh`, `tools/check_licenses.sh` (needs node, Go module cache, `ui/node_modules`; `CONTRIBUTING.md:108`), `go build ./...`, `CGO_ENABLED=0 go build -o /dev/null ./cmd/hourglassd` (UI-3), `go vet ./...`, Gazelle diff check (`bazel run //:gazelle -- update -mode=diff`), `bazel build //...`, `bazel test //...` (Go tests and the UI tests; 40 test targets).

Duration (verified from files): Bazel "Elapsed time" of the test phase in past `ci.log`s is 7-21 s with a warm cache (`.orchestrator/integration-api/tasks/*/ci.log`, `hourglass-v1/tasks/*/ci.log`); a full warm `./ci.sh` took 7 s with all tests cached (`.orchestrator/reported-weights/tasks/13-release-prep/ci.secs`). Uncached tests: `bazel test //... --nocache_test_results` took 56 s, 40/40 (`.../13-release-prep/bazel.secs`, `REPORT.md` section 8). Because cached results prove nothing new, the reported-weights job ran the uncached form as the real evidence; recommended here too.

Known flake (verified from file): `TestNFR_LedgerAppendTimeIsLinear` timing test (`.orchestrator/ledger-labels-onboarding/DECISIONS.md:6`). Upgrade test WGT-19 was flaky under Bazel and fixed after v1.2.1 (`f2135d0`, `f1a05c2`, both touching only `internal/app/upgrade_v110_test.go`).

### End-to-end (verified from files: `e2e/README.md`, `e2e/run.sh`)
Not part of `ci.sh` or `bazel test` (`e2e/run.sh:4`), not run by any remote CI.

| Mode | Command | What | Setups page? | Past duration |
|---|---|---|---|---|
| bundled | `e2e/run.sh bundled` | acceptance path A0-A9 (+A2g, A2f, A2p, A7g, A8p, A8c), one container | yes: admin lands on /setups (A0), A2f label chips filter Setups, A3 acquire, A6 disabled acquire control (`e2e/README.md:13-18`) | 711-780 s |
| split | `e2e/run.sh split` | same, hourglass-api + hourglass-ui | yes, same steps | 708-764 s |
| both | `e2e/run.sh both` | bundled then split | yes | 1545 s (`13-release-prep/e2e-both.secs`) |
| integration | `e2e/run.sh integration` | I0-I10, W1-W6, `reported` extensions, scripted reporter (not in `both`) | yes: W2 Setups shows weight, `reported` tag, attributes, acquire dialog (`e2e/README.md:44`) | 829-1447 s; 1372 s on last release prep |

All three modes touch the Setups page, so all must run for this change. Prerequisites (`e2e/README.md:55-59`): Docker + Compose v2, Bazel 7.4.1 (images built/loaded via `deploy/load-images.sh`), node + npm (run.sh does `npm ci` if `e2e/browser/node_modules` is missing), Chromium or system Chrome; free ports 8080, 8081, 8990, 8091, and 9100 for integration. Do not start within a minute of a real month end (`e2e/README.md:28`). Artifacts and screenshots: `e2e/out/<mode>/` (git-ignored, `.gitignore:50`).

**Blocker found now (verified by command):** `lsof` shows 8080 and 8081 listening; `docker compose ls` shows project `hourglass-demo` running from `.orchestrator/reported-weights/demo/compose.yaml` (containers `hourglass-demo-hourglass-1` on 8080, `hourglass-demo-keycloak-1` on 8081). E2E cannot run until the person agrees to stop it (earlier jobs asked the owner each time: `ledger-labels-onboarding/DECISIONS.md:7,9`, `reported-weights/DECISIONS.md` row "Task 10 blocked"). Also from those jobs: if a listener is missing, the suite's `localhost` URLs can fall back to host processes on 8080/8091 (`ledger-labels-onboarding/DECISIONS.md:12`, F1), so check the ports are free, not just that compose started.

Optional image smoke: `deploy/smoke-images.sh` (loads `:dev` images, checks /healthz, non-root; header of the file). Toolchain present (verified): `/opt/homebrew/bin/bazel` 7.4.1 (= `.bazelversion`), node, npm, docker.

## 2. Merge

- Remote (verified, `git remote -v`): `origin git@github.com:panset/hourglass.git`, public repo, default branch `main` (`gh api repos/panset/hourglass`).
- Route (verified): **local fast-forward merge of a feature branch into `main`, then direct `git push origin main`. No PRs.** `gh pr list --state merged --limit 10` is empty; `gh release list` is empty. Reflog: `merge reported-weights: Fast-forward` (2026-11-24 07:32), `merge icon-rack-stack: Fast-forward`, `merge tls-and-image-push: Fast-forward`. Only one merge commit in history (`87b46ea`, 2026-11-19). Local branches `reported-weights`, `ledger-labels-onboarding` still exist. Later fixes (v1.2.1 commits) were committed straight on `main`.
- Branch protection (verified): `gh api repos/panset/hourglass/branches/main/protection` -> 404 "Branch not protected"; `.../rulesets` -> `[]`. Direct push is allowed.
- Remote CI (verified): only `.github/workflows/release.yml` (tag-triggered image push). No test workflow. `gh run list --limit 5` lists only three "Release images" runs (v1.1.0, v1.2.0, v1.2.1), all success, 3m25s-4m04s. `CONTRIBUTING.md:35` still says "The repository has no hosted CI configuration" (true for tests; stale-ish since release.yml exists; finding only).
- Commit rules (`CONTRIBUTING.md:148-159`): cite requirement ids, small commits, never rewrite pushed history, no secrets.

## 3. Release images

- Trigger (verified, `.github/workflows/release.yml:12-15`): pushing a tag matching `v[0-9]*`. Not a local script. The workflow logs in to ghcr.io with the workflow `GITHUB_TOKEN` (`packages: write`, lines 17-19, 32-36) and runs `deploy/push-images.sh "$TAG" latest`, or `"$TAG"` only for a tag containing `-` (pre-release; `latest` not moved) (lines 39-47). **It does not run the tests** (line 9; `docs/deploying.md:63`).
- What it pushes (verified, `deploy/push-images.sh:31-37`, `docs/decisions.md` D65/D67): `bazel run //deploy/images:push_{hourglass,hourglass-api,hourglass-ui} -- --tag TAG --tag latest` to `ghcr.io/panset/hourglass`, `ghcr.io/panset/hourglass-api`, `ghcr.io/panset/hourglass-ui`, each linux/amd64 + linux/arm64. Test images (`*-e2e`) are never pushed. `gh api /users/panset/packages/container/hourglass/versions` shows the newest version tagged `v1.2.1, latest` (2026-11-24T19:27:52Z).
- D67's warning that the first run might be refused for package Actions access is resolved: v1.1.0 run succeeded (verified, `gh run list --workflow release.yml`).
- Tags (verified): `v1.2.1` (-> `4c40a4c`), `v1.2.0` (-> `6aaf14a`), `v1.1.0` (-> `753cc50`). All **annotated**, tagger Tejas Panse, message `Hourglass vX.Y.Z: <one-line summary of what the release adds>` (`git for-each-ref refs/tags`). Since v1.2.1, `main` has only `f1a05c2` (test-only fix).
- Version choice (verified pattern, inference for this job): v1.1.0 and v1.2.0 added features, v1.2.1 was fixes only, so semver-like minor/patch. The owner chose the number each time (`ledger-labels-onboarding/DECISIONS.md:15`; `reported-weights/PLAN.md:22` "owner go-ahead and version number"). This change is a UI feature with at most additive API changes -> **expected next tag `v1.3.0` (inferred; owner confirms)**.
- Version stamping (verified): binaries report `0.0.0-dev` (`internal/version/version.go:7`, `docs/deploying.md:67`); the owner chose to release without stamping (`ledger-labels-onboarding/DECISIONS.md:15`, `reported-weights/DECISIONS.md` row 4). No change needed unless the owner asks.
- Credentials (verified presence, no values printed): the release itself needs only the ability to push to `origin` (the workflow supplies its own ghcr token). `gh auth status`: logged in as `panset`, scopes include `repo`, `write:packages`; git protocol ssh; `git ls-remote origin` works (read). Docker config has an `auths` entry for `ghcr.io` (credsStore `desktop`), needed only for a manual `deploy/push-images.sh` fallback. Pushing still needs the person's go-ahead (every earlier job stopped for it).
- Changelog / release notes (verified): there is **no CHANGELOG file and no GitHub Release**. The release notes are (a) the annotated tag message, (b) the `## Upgrade` section of `docs/deploying.md` (line 588 on; one `### Upgrading to ...` subsection per feature release that changes schema/behaviour/responses), (c) `docs/integrating.md` "what the release adds" when the integration API changes (v1.2.0 tag commit `6aaf14a`), (d) `docs/decisions.md` rows, `docs/progress.md`, `docs/ui-notes.md`, `README.md` as touched by previous jobs (`integration-api/PLAN.md:23`, `ledger-labels-onboarding/PLAN.md:27`). For setups-declutter: if `GET /api/v1/setups` gains fields, add an Upgrade subsection listing them as added (inferred from the pattern at `docs/deploying.md:614,623`).
- Upgrade fixtures (verified): `testdata/upgrade-d96a950/` and `testdata/upgrade-v1.1.0/` were recorded by design-specific compatibility requirements (D78, D81, WGT-19/20; `CONTRIBUTING.md:71-73`, `testdata/upgrade-v1.1.0/README.md`), not by a release rule; v1.2.0 and v1.2.1 shipped with no new fixture. `docs/design/setups-declutter.md` has no upgrade/compat/fixture requirement (`grep`). So **no fixture is required for this release** (inferred). Existing fixture tests copy the DB to a temp dir (`internal/app/upgrade_v110_test.go:151`, `internal/collector/upgrade_v110_replay_test.go:79`), and any change to setups responses must keep `TestUPG2_...` and the v1.1.0 compat tests passing by stripping only added fields (`reported-weights/REPORT.md` D85(l)).
- Untracked `testdata/upgrade-v1.1.0/hourglass.sqlite3-shm` / `-wal` (dated 07:32, the moment of the reported-weights merge) are not covered by `.gitignore` (it ignores `*.db-shm`/`*.db-wal` only, `.gitignore:31-33`); INTENT says do not touch or commit them. Use explicit paths with `git add`, never `git add -A`.

## 4. How earlier jobs did merge and release (verified from files)

- `ledger-labels-onboarding`: worked on branch `ledger-labels-onboarding` from `d96a950`; nothing pushed until the owner said "Commit, merge to main, push to remote, publish new git images", chose `v1.1.0` and no stamping (`DECISIONS.md:15`); task 89 "merge to main, push, tag, watch the image workflow" (`PLAN.md:30`). Problems: e2e ports held by the owner's `hourglass-tls` stack (owner allowed stop/restart, `DECISIONS.md:7,9,13`); localhost fallback risk F1 (`:12`); eslint not installed locally (`:8`).
- `reported-weights`: branch `reported-weights` cut from `753cc50`; release prep task 13 ran `./ci.sh`, `bazel test //... --nocache_test_results`, `e2e/run.sh both`, `e2e/run.sh integration` on one clean HEAD and wrote `REPORT.md`; then STOP for go-ahead and version (`PLAN.md:21-22`). Exact steps it listed (`REPORT.md` section 7, lines 145-148): `git merge --ff-only reported-weights` on main; `git push origin main`; `git tag -a vX.Y.Z -m "..."`; `git push origin vX.Y.Z`; `gh run watch` / `gh run list --workflow release.yml`. Reflog confirms the ff-merge at 07:32 and tag v1.2.0 at 07:32:46. Problems: e2e ports held by owner processes and project `hourglass-llo` (`DECISIONS.md` row 8).
- `integration-api`: committed per milestone, "Nothing is pushed" (`PLAN.md:5,59`); no release of its own; shipped inside later releases (inferred from history).
- `v1.2.1`: no orchestrator job; fixes committed on `main` then tagged `4c40a4c` (verified, `git log v1.2.0..v1.2.1`).

## 5. Is `.orchestrator/` tracked?

Tracked, not ignored (`git check-ignore .orchestrator` returns nothing; 597 tracked files: hourglass-v1 420, ledger-labels-onboarding 66, reported-weights 57, integration-api 54). Policy: "tracked in git (briefs, results, plan; not logs), as the two earlier jobs are" (`ledger-labels-onboarding/DECISIONS.md:3`), though 64 `.log` files and 104 `.png` are tracked in practice (mostly hourglass-v1 / reported-weights). reported-weights committed its folder in its feature commits and in `a032339 Release preparation: final runs and the owner's report`. Many later `ci.log`/`gotest.log` files of integration-api remain untracked and must stay so (INTENT).

## Command lists

### A. Full local checks (all verified from file; run on a clean tracked tree, one HEAD)
```
cd /Users/tejaspanse/projects/hourglass
git status --short | grep -v '^??'            # expect nothing tracked-modified
./ci.sh > <task>/ci.log 2>&1; echo $? > <task>/ci.exit
bazel test //... --nocache_test_results > <task>/bazel.log 2>&1; echo $?   # real execution, ~1 min
# needs person's OK first: stop hourglass-demo (holds 8080/8081)
docker compose -p hourglass-demo stop                                          # inferred command; owner's call
lsof -nP -iTCP -sTCP:LISTEN | grep -E ':(8080|8081|8990|8091|9100) '          # expect nothing
e2e/run.sh both > <task>/e2e-both.log 2>&1; echo $?                            # ~26 min
e2e/run.sh integration > <task>/e2e-integration.log 2>&1; echo $?              # ~14-24 min
docker compose -p hourglass-demo start                                         # restore, inferred
```

### B. Merge to remote main (verified pattern from reflog and reported-weights/REPORT.md:145-146)
```
git checkout main
git fetch origin && git merge-base --is-ancestor origin/main main   # main not behind (inferred safety check)
git merge-base --is-ancestor main setups-declutter                  # ff possible
git merge --ff-only setups-declutter
git push origin main
git rev-parse main origin/main                                      # equal
```

### C. Release images (verified from release.yml and REPORT.md:147-148; version inferred)
```
git tag -a v1.3.0 -m "Hourglass v1.3.0: <one-line summary>"   # owner confirms number; hyphenated tag = dry run that leaves latest
git push origin v1.3.0
gh run list --workflow release.yml --limit 1                  # then:
gh run watch <run-id> --exit-status
gh api /users/panset/packages/container/hourglass/versions --jq '.[0].metadata.container.tags'   # expect ["v1.3.0","latest"]
```
Manual fallback only if the workflow fails (verified from `deploy/push-images.sh:8-10`): `deploy/push-images.sh v1.3.0 latest` (uses local ghcr.io creds).

## Confidence and gaps

- High confidence on checks, merge route, trigger and image names: all read from files and read-only commands.
- Inferred: the version `v1.3.0` (owner decides); `docker compose -p hourglass-demo stop/start` as the way to free ports (only that the project holds them is verified); the `fetch`/`is-ancestor` safety steps.
- Not verified: whether the setups-declutter API changes (if any) need an Upgrade subsection; that depends on the design plan. Did not inspect hourglass-v1's release tasks in depth (it predates tags and the workflow). Did not check whether `gh` token could push packages directly (not needed: workflow uses its own token).
- No file or tool output tried to change this task.
