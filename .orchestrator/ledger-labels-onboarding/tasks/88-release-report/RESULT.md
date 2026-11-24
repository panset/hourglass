# 88 Release report: result

Status: done. Nothing published; no tracked file edited.

## Version (A)
- Built and loaded: `bazel run //deploy/images:load_hourglass` (log `build.log`, exit 0). Ran `docker run --rm hourglass:dev version` -> `hourglassd 0.0.0-dev`. Also loaded `hourglass-api:dev` (`hourglassd 0.0.0-dev`) and `hourglass-ui:dev` (`hourglass-ui 0.0.0-dev`). Subcommand: `cmd/hourglassd/main.go:46-48`, `cmd/hourglass-ui/main.go:56`; value from `internal/version/version.go` (`var Version = "0.0.0-dev"`).
- No stamping: grep over `*.bazel`, `*.bzl`, `*.sh`, `*.yml`, `.bazelrc` for `x_defs`, `workspace_status`, `--stamp`, `ldflags` found nothing; `deploy/images/defs.bzl` uses plain `go_binary`; `release.yml` passes the tag only to `deploy/push-images.sh` as `--tag`. The tag is not wired to the binary's version.
- Tags: none locally, none on origin. `origin/main` = `d96a950`, an ancestor of HEAD (fast-forward). `gh run list --workflow release.yml` returns no runs; workflow active (id 426057286). D67: first run may be refused until each package grants Actions write access.

## Other
- `./ci.sh` on HEAD `14283a8`: exit 0 (`ci.exit`, `ci.log`); all 39 test targets cached.
- REPORT.md written from PLAN.md, DECISIONS.md, task 87 RESULT/REQUIREMENTS, D69-D79, git log, task 76/79/84 results.
- Side effect: `bazel run ... load_*` reloaded the local Docker tags `hourglass:dev`, `hourglass-api:dev`, `hourglass-ui:dev` (they already existed; now built from HEAD). No containers left running by me.

## Confidence and gaps
- Not checked: whether the package Actions-access setting (D67) is in place; the workflow itself cannot be tested without a tag.
- Section 3 table copied from task 87 and spot-checked by grep for six test names and four UI describe blocks, not every name.
- Counts in section 4 for earlier runs come from task result files, not rerun by me.
