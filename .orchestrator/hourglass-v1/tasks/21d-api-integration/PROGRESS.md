# Progress: 21d API integration fixes

- 2026-11-19 Read brief, INTENT, CONTRACT and the 21a/21b/21c/28 results.
- 2026-11-19 Item 2 done: `collector.PollView.Decisions` (hold id to Decision, cloned); API metering reason reads it (user and admin views); `metering_description` and the policy `description` are null (OpenAPI made nullable, schema.gen.ts regenerated, admin-settings page shows the description only when present).
- 2026-11-19 Item 5 done: `holds.CodeConflict`; releasing a released hold is 409 `conflict`, answered after the permission check (a stranger gets 403, not a hint the hold exists).
- 2026-11-19 Item 4 done: uiserve falls back to index.html for every non-file path, 404 only under `assets/`.
- 2026-11-19 Item 7 done: `api.Runtime.OIDCIssuer`, set by `internal/app` from config.
- 2026-11-19 Items 1 and 3 done: one `backend` test environment behind the router, user and admin test envs; shared session/request/user/JSON helpers; `notImplemented` removed; router tests rewritten against real handlers (seeded setup 1 / hold 1).
- 2026-11-19 Item 6: gazelle run (no BUILD diffs). Running `go test -race -count=1 ./...` then `./ci.sh` (logs gotest.log, ci.log in this folder).
