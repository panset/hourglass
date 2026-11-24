Upgrade fixtures recorded from the release code at commit d96a950 (UPG-1 to UPG-3).

Nothing here is edited by hand.

- `hourglass.sqlite3`: a SQLite database written by the release code (schema version 5, 59 ledger rows, 2 periods, 4 users, 4 setups, 2 holds, a grant in the middle of a local day, one hold released). The local admin's password in it is the test password of the unit tests, not a real credential.
- `responses.json`: what the release code answered on that database (clock 2026-11-12T09:15Z, timezone Europe/Berlin): `GET /api/v1/me/ledger` (default, `period=`, a second user), `GET /api/v1/setups` (developer, admin, `selector=`), `GET /api/v1/session` (developer, admin, user with no role), the first SSO sign-in of a new user (redirect, stored user, session, events, audit), and `facts` (balances, per-period sums, per-hold charges read through the store). `csrf_token` is replaced by `<csrf>`; the `time` of an event is the real wall-clock time and is ignored by the tests. `ledger_dev` equals `before_compaction` in `internal/api/testdata/led11_pre_change_responses.json`.
- `openapi.yaml`: `api/openapi.yaml` of d96a950, byte for byte (`git show d96a950:api/openapi.yaml`).
- `recorder_upg_record_test.go.txt` and `recorder_testenv.patch`: the recorder and the small test-environment change it needs (a database path and an event emitter option). They are text, not compiled.

How it was produced (2026-11-23):

    git worktree add --detach /tmp/old d96a950
    cd /tmp/old
    git apply <repo>/testdata/upgrade-d96a950/recorder_testenv.patch
    cp <repo>/testdata/upgrade-d96a950/recorder_upg_record_test.go.txt internal/api/upg_record_test.go
    RECORD_DIR=/tmp/rec go test ./internal/api -run TestRecordUpgradeFixture -count=1
    # /tmp/rec/release_d96a950.db -> hourglass.sqlite3, /tmp/rec/release_d96a950_responses.json -> responses.json
    git worktree remove --force /tmp/old

The recorder builds the world of `newLedgerWorld` in `internal/api/ledger_view_test.go` (same order, so the ledger ids are the same), then adds a second developer, two more setups and a user with no role, records the responses, closes the store (no WAL file is left), copies the database, and records the SSO sign-in on a copy of it.

The database holds the local admin's password hash and a few hashed, long-expired session ids. The password is the unit tests' own. Never use a real password when recording this fixture again.
