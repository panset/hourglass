# PROGRESS 37-docs

- 2026-11-19: read brief, INTENT, CONTRACT, SPEC, RESULT files of 21a, 28, 30, 31, 32, 33, 34, 38, and the code behind config, auth, events, collector, metrics, webhook, bundled extensions.
- 2026-11-19: ran by hand (scratch dir, port 18180): go build of hourglassd, API-only run with static provider, local admin sign-in and forced password change, settings PUT, VACUUM INTO backup, check-provider, metrics listener, webhook capture of a real poll.failed delivery, a throwaway third-party module (four extensions, conformance suites, main) built and started.
- 2026-11-19: started ./ci.sh in the background (log in scratchpad).
- Next: write docs/extending.md, docs/deploying.md, README.md, CONTRIBUTING.md; then RESULT.md.
- 2026-11-19: wrote docs/extending.md, docs/deploying.md, README.md, CONTRIBUTING.md; ran README quick start 2 literally, link and anchor check clean; go test, race tests, bazel build and test pass; ./ci.sh red for reasons outside this task (see RESULT.md).
- 2026-11-19: wrote RESULT.md. Done.
