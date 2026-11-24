# 00 Digest for planning
Job intent: .orchestrator/reported-weights/INTENT.md (read it first). Contract: .orchestrator/reported-weights/CONTRACT.md.
## Goal
A short planning digest (RESULT.md, at most 120 lines) so the orchestrator can brief build tasks without reading the sources. Read-only task: write only inside this task folder.
## Answer these, with path:line evidence
1. `docs/deploying.md`, `docs/integrating.md`, `docs/extending.md`: section outline of each (heading list), and where exactly the design's Documentation deliverables 1 to 3 (docs/design/reported-weights.md) will have to land (which sections exist: "Upgrade", the four existing integration "jobs", checklist, `max_age` advice, `Decision.Weight` text). Note how the integrating examples were run for the earlier work (D66) and anything that says how to start a local server to run examples.
2. How D78's fixtures were recorded: read `testdata/upgrade-d96a950/README*`, the recorder files, and the tests that consume them (names, files). What a v1.1.0 recorder must do differently.
3. Version and release mechanics: `internal/version` (what the binary reports now, how v1.1.0 set it: see `git log -5 --stat -- internal/version`), `.github/workflows/release.yml`, `deploy/push-images.sh`, "Publishing the images to a registry" in docs/deploying.md. List the exact steps the v1.1.0 release took (see `.orchestrator/ledger-labels-onboarding/PLAN.md` and its release task RESULT).
4. Test layout facts a builder needs: where the permissions matrix and contract tests live and how a service route is added to them; the test env helper for `internal/api` and for `internal/collector`; how the e2e integration scenario is organised (`e2e/` files, the scripted reporter); how long `./ci.sh` and each e2e mode took in the last job (look in `.orchestrator/*/tasks/*/RESULT.md`, do not run them).
5. Is any id WGT-1..23 or UI-12..14 already used anywhere in the repo (grep tests and docs, excluding docs/design/reported-weights.md)? What is the highest existing UI-n id and D-number?
6. Anything in those docs or the code that appears to contradict the design doc. Report as findings; do not resolve.
## Scope
May read: whole repo. May write: `.orchestrator/reported-weights/tasks/00-digest/` only. Do not run ci.sh, bazel or e2e.
## Report
Per CONTRACT.md "Reporting".
