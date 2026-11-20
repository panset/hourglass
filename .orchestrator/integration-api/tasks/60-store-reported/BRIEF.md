# 60 Store migration 0005, queries, and `internal/reported`

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Ground rules, Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/60-store-reported/`.

## Goal
The data layer for the integration API exists and is tested: migration `0005`, store queries for
every new table and column, and a core package `internal/reported` that owns the stored-report
rules. No HTTP, no auth middleware, no extensions: later tasks build on this.

## Context
- Design: `docs/design/integration-api.md`. Read all of it; your sections are "Data model",
  "Rules shared by both report routes", "The `reported` provider and pressure source"
  (Wiring constraints) and requirements INT-2, INT-3, INT-6.
- `CONTRIBUTING.md`, "Add a migration" and "The core stays generic".
- Existing patterns: `internal/store/` (migrations, repositories, tests that count migrations and
  check the data model), `internal/holds/`, `internal/collector/`.

## What to build
1. `internal/store/migrations/0005_*.sql` with exactly the tables and columns in the design's
   Data model table. `service_tokens` cascade-deletes with its account. Names on holds and audit
   rows are text, not foreign keys.
2. Store queries (repositories in the style already there) for:
   - service accounts: create, get by id/name, list (with each token's id, created, expiry,
     last-used), update description/scopes/enabled, delete;
   - service tokens: insert (hash only), look up by token id joined with its account, revoke,
     touch `last_used_at` (the at-most-once-a-minute rule may live here or in auth; say which);
   - reported pressure: read, replace;
   - reported deployments: read header + set, replace header + set in one transaction (INT-2);
   - holds: read/write `release_requested_at` and `released_by_service` on the existing hold
     model and scans (do not change release behaviour: that is task 65);
   - audit: write and read `actor_service` (with `actor_id` null).
3. `internal/reported`: the service the API handlers will write and the extensions will read.
   It must not import any extension package. It owns:
   - ordering: a report with `observed_at` older than the stored one is refused with a typed
     error (later mapped to 409 `stale_report`) and changes nothing; equal replaces (INT-3);
   - clock skew: `observed_at` more than 2 minutes after now is a typed validation error;
   - `observed_at` defaulting to receipt time; recording `received_at` and the account name;
   - atomic replace of the deployment set with its header (INT-2), including the empty set;
   - reads that return the stored report with its age, so a caller can apply `max_age`
     (`max_age` itself is extension config and is passed in by the caller, not stored);
   - persistence across a reopen of the database (INT-6).
   Take a clock the way the other core packages do. Value-range validation of pressure (INT-1)
   and duplicate-name validation may live here if that is the natural place; say which you chose.

## Scope
May read: the whole repository.
May write: `internal/store/`, `internal/reported/`, their `BUILD.bazel` files (via
`bazel run //:gazelle`), `docs/decisions.md` (append rows from D44 only for choices the design
leaves open), your task folder. If an existing test elsewhere fails only because of the new
columns (e.g. a struct literal or a migration count), fix that test and list it.
Must not touch: `docs/SPEC.md`, `docs/design/`, `docs/integrating.md`, `internal/api/`,
`internal/auth/`, `api/`, `ui/`, git state.

## Done when
- Tests named after the ids exist and pass, at least `TestINT2_...` (a concurrent reader never
  sees a mix of two reports; replace is all-or-nothing), `TestINT3_...` (older refused and
  nothing changes; equal is idempotent), `TestINT6_...` (close and reopen the database file, the
  reports are still there). Plus ordinary tests for every new query, the cascade delete, and the
  migration/data-model checks in `internal/store/store_test.go`.
- `go test -race -count=1 ./internal/...` passes.
- `./ci.sh` passes (SPDX, genericity, Gazelle, bazel build and test).

## Long-running steps
`./ci.sh` runs Bazel and takes roughly 5 to 15 minutes: run it once at the end, in the
foreground, with a 30-minute timeout, output to a log file in your task folder (do not leave the
log in the repo root), and read only the exit code and tail.

## Report
Follow the Reporting section of the contract. In RESULT.md also give: the exported API of
`internal/reported` and of the new store queries (signatures only), so the next task can use
them without rereading the code; and the numbers of any decision rows you appended.
