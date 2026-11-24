# 13 Release preparation: final runs and the owner's report
Job intent: .orchestrator/reported-weights/INTENT.md (read it first). Contract: .orchestrator/reported-weights/CONTRACT.md (binding).
## Goal
Everything the design's Release section (docs/design/reported-weights.md) asks for before publishing, on the current HEAD of `reported-weights`, and a report the owner can decide from. You publish nothing: no merge, no push, no tag, no image push, no `gh` write, no change to any tracked file outside this task folder.
## Do
1. Record `git rev-parse HEAD` and `git status --short` (tracked files must be clean).
2. Port check first: `lsof -iTCP -sTCP:LISTEN -P | grep -E ':(8080|8081|8990|8091|9100|8443) '`. If anything not started by the e2e stacks listens there, do not stop it; report `blocked` with what you found.
3. Run, each once, logging into this task folder, recording exit code and duration: `./ci.sh`; `bazel test //... --nocache_test_results` (so no result is a cache hit; report tests executed and skipped); `e2e/run.sh both`; `e2e/run.sh integration`. Read `e2e/out/*/steps.json` and list every step with its status.
4. What the released binary reports as its version: build the product image or binary as `docs/deploying.md` ("Images", "Publishing the images to a registry") describes without pushing, and run its `version` command (`hourglassd version`; the UI binary too). Report the exact output. State what `.github/workflows/release.yml` and `deploy/push-images.sh` will do when a `v*` tag is pushed (read them), and whether `git merge --ff-only reported-weights` onto `main` would be a fast-forward (`git merge-base --is-ancestor main HEAD`). Check `gh run list --workflow release.yml --limit 3` (read-only) to report how the v1.1.0 run ended, if `gh` is signed in; if not, say so.
5. Write `.orchestrator/reported-weights/REPORT.md` (not in the task folder) for the owner, in plain prose and tables, containing:
   - What was built, in a short section per design area (A to D), and the commits (`git log --oneline 753cc50..HEAD`).
   - Every requirement WGT-1 to WGT-23 and UI-12 to UI-14 with its test names and files (take the table from `.orchestrator/reported-weights/tasks/12-verify-m7/RESULT.md` and its fix task if any, and confirm each named test exists with grep).
   - The first client's eight acceptance scenarios, each with its test or e2e step.
   - Every decision recorded: D80 to the last row of docs/decisions.md, one line each, with the **REVIEW** ones listed first and explained in a sentence.
   - The verification rounds (tasks 04, 07, 12): what each found and how it was closed.
   - The final runs from step 3 with exit codes, durations, test and step counts.
   - What the binary reports as its version, and the exact release steps that remain after the go-ahead (merge to main, push, annotated tag, push tag, watch the image workflow), with the note that the tag moves `latest`.
   - Anything not done or not verified, plainly: collect every "Confidence and gaps" section of the task RESULT.md files and the open points of the design that the owner should pass on.
## Scope
May write: this task folder, `.orchestrator/reported-weights/REPORT.md`. The e2e runs write under `e2e/out/` (git-ignored; confirm). Nothing else.
## Done when
All four runs exit 0 (or the task reports `blocked`/`partial` with the failing step and log tail, without attempting a product fix), and REPORT.md exists with every section.
## Long-running steps
`./ci.sh` 30 minutes; uncached Bazel 30 minutes; `e2e/run.sh both` about 25 minutes, timeout 60; `e2e/run.sh integration` about 20 minutes, timeout 45.
## Report
Per CONTRACT.md "Reporting".
