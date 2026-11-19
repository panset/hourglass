# 08 internal/store: SQLite store and migrations
Job intent: .orchestrator/hourglass-v1/INTENT.md (read it first). Rules: .orchestrator/hourglass-v1/CONTRACT.md.
## Goal
Package `internal/store`: the single SQLite database of docs/SPEC.md "Data model", with embedded numbered migrations and typed access functions, tested against real temporary database files.
## Part A: toolchain and dependency (you own go.mod, go.sum, MODULE.bazel, MODULE.bazel.lock for this task)
- Raise the Go version: Bazel Go SDK and go.mod `go` directive to a 1.25.x or newer release that the pinned (or a newer, Bazel 7.4.1-compatible) rules_go can download (a later dependency needs Go >= 1.25). Record the versions.
- Add `modernc.org/sqlite` (pure Go, no cgo) via go.mod and Gazelle's `go_deps` extension with `use_repo`. The build must work with cgo disabled.
## Part B: schema
- Migrations as `migrations/NNNN_name.sql`, embedded with `embed`, applied in order at `Open` inside transactions, tracked in a `schema_migrations` table; re-running is a no-op; a database newer than the binary is an error.
- Exactly the 15 tables and columns of the spec's data-model table, with its keys, the unique constraints and indexes it names (users.oidc_sub unique; setups.name unique; setup_labels composite key and index on key+value; holds partial unique index on setup_id where released_at is null; usage_samples unique on hold_id+polled_at; user_roles composite key), sensible NOT NULLs and foreign keys (`PRAGMA foreign_keys=ON`), WAL mode, a busy timeout. Timestamps are UTC ISO 8601 text (RFC 3339, fixed width so they sort); token amounts are integer milli-tokens.
- The ledger is append-only: enforce with triggers that abort UPDATE and DELETE.
- One addition not in the spec's table, already decided: an `outages` table (`id`, `started_at`, `ended_at`, `reason`) for the uncharged gaps of MET-4 / ADM-4.
- Nothing else beyond the spec. If you believe another column or table is unavoidable, do not add it; list it under open questions.
## Part C: access layer
`Open(path string) (*Store, error)`, `Close`, and `(*Store).Tx(ctx, func(*Tx) error) error`; read and write methods available both on Store and inside a Tx. Row structs live in this package (`store.User`, `store.Setup` with its labels map, `store.Rule`, `store.Hold`, `store.UsageSample`, `store.LedgerEntry`, `store.PressureSample`, `store.Orphan`, `store.EventDelivery`, `store.Setting`, `store.AuditEntry`, `store.Session`, `store.CollectorRun`, `store.Outage`). Cover, at minimum:
- users: create OIDC user (pending, no roles), upsert-on-login by `oidc_sub` refreshing email/name/last_login_at, create the local admin row, get by id/sub/username, list with filters (status), set status, set password hash and must_change flag; roles: get, replace set.
- setups: create, update (labels replaced as a set), delete, get, list (optionally enabled only), with labels loaded.
- rules: list by kind in position order, replace the whole ordered list for a kind in one transaction.
- holds: create (the partial unique index must surface as a typed `ErrSetupHeld`), set metering_started_at / deployed_at once (no overwrite), release (released_at, released_by, release_reason), active holds, active count for a user, holds of a user (current and past).
- usage_samples: insert idempotently on (hold_id, polled_at) reporting whether a row was inserted (MET-5); per-day sums of debit for a user and period range; sum of a user's debits since a time; prune older than a cutoff.
- ledger: append an entry and update `users.balance_mt` in the same transaction; list by user and period; `RebuildBalance(userID, period)` recomputing the cache from the ledger (LED-2); sum by user/period/type.
- pressure_samples: insert, list since a time, latest N. orphans: upsert by resource_name (first_seen kept, last_seen and labels updated), delete those not seen in the current poll, list. outages: insert, list. collector_runs: insert start, finish, last successful, latest. event_deliveries: insert, finish with outcome, list recent, prune older than a cutoff. settings: get, put (with updated_by), all. audit_log: append, list newest first with optional actor and action filters and limit/offset. sessions: create, get, touch last_seen/expires, delete, delete expired, delete all for a user.
Sentinel errors: `ErrNotFound`, `ErrSetupHeld`, `ErrConflict` (unique violations such as a duplicate setup name).
The package imports only the standard library and the SQLite driver: no other Hourglass package, and no mention of vCenter, sizes or concrete extensions outside neutral test data.
## Tests
Real database files in `t.TempDir()`. Cover: fresh migrate and re-open; every table round-trips; each constraint named above; ledger triggers reject UPDATE and DELETE; idempotent sample insert (`TestMET5_...`); balance cache equals ledger sum after mixed appends and after RebuildBalance; hold uniqueness under two concurrent creates; a schema test that reads `PRAGMA table_info` and asserts every column named in the spec's data-model table exists.
## Scope
May write: `internal/store/`, go.mod, go.sum, MODULE.bazel, MODULE.bazel.lock, this task folder. Another agent is creating `pkg/hourglass/` at the same time (no dependencies); do not touch it. Regenerate BUILD files only for your package (`bazel run //:gazelle -- internal/store`).
## Done when
`./ci.sh` exits 0; RESULT.md lists versions, the exported API by group, test count, and decisions.
## Long-running steps
Bazel fetching a new Go SDK and the SQLite module graph: allow 15 minutes for the first build.
