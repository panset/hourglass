Compatibility fixtures recorded from the release code of `v1.1.0` (commit `753cc50`), for WGT-19 and WGT-20 of `docs/design/reported-weights.md`. They are recorded the way `testdata/upgrade-d96a950/` was (D78).

Nothing here is edited by hand. The recording is deterministic: the clock, the ids and the secrets are fixed, and a second run writes identical files (checked with `cmp`).

- `hourglass.sqlite3`: a SQLite database written by the release code (schema version 7, no WAL file). The clock at the end is 2026-11-12T09:25:00Z (the time of the last poll and of the stored reports), timezone Europe/Berlin. Rows per table are in `responses.json` under `facts.rows_per_table` (121 ledger rows, 157 usage samples, 34 polls, 8 setups, 6 users, 5 holds, 3 rate rules, 2 service accounts). Contents:
  - Settings: `default_weight` 1.25, `display_label_keys` `["tier","zone"]`, `max_holds_per_user` 4. The other five are not stored, so they read as defaults.
  - Rate rules in order: `tier=gold` 2.5, `memory-gb=64` 4, `tier=free` 0.
  - Setups `lab-01` (gold and 64 GB, first rule), `lab-02` (weight 0), `lab-03` (second rule), `lab-04` (default weight), `lab-05` (disabled), `lab-06` and `lab-07` (never held), `lab-08` (a policy weight of 3.25).
  - Users: the local admin, `dev`, `dev2`, `other` (no activity), `norole` (active, no role), `ops` (developer and admin). Email addresses end in `@example.test`.
  - Holds: `dev` holds `lab-01` and `lab-02`; `dev2` holds `lab-03` (release requested, still deployed: a pending release), `lab-08`, and had `lab-04` (released). All are metering.
  - Ledger: the allowance of each user, one grant to `dev`, debits of every poll over three local days (two days in early November and a day after a gap of nine days; the gap is one recorded outage).
  - Two service accounts, `reporter` (all four scopes of v1.1.0) and `viewer` (`state:read`), each with one token; the stored deployments report (four setups and one unknown name, `lab-99`) and pressure report (0.8), both sent at the recorded time.
  - No sessions, no pending events (the outbox is emptied) and no audit entries beyond the recorder's own grant.
- `responses.json`: what the release code answered on a copy of that database, in a fresh process (no poll had run), at the recorded time. `responses` is a list of `{name, principal, request {method, path, body}, status, response}`; `facts` holds balances, per-period sums, per-hold charges, the ids of users, setups, holds and service accounts, and the rows per table. Reads come first, then the writes (settings PUT with the eight current keys, setup POST and PUT without any new field, a refused write by a developer), then the lists again. The routes are listed in `RESULT.md` of the task and in the `name` fields.
- `poll.json`: three polls of the release code over another copy, at 09:30, 09:35 and 09:40Z on 2026-11-12 (five minutes apart, the first charged for five minutes after the recorded last poll). For each poll: the fake pressure value and the allocations the fake provider reports, the balances before and after, the exact `Snapshot` passed to the pressure source, every call of the metering policy (hold state and snapshot as passed, and the decision), the raw and smoothed pressure and the multiplier, every usage sample and every ledger row the poll wrote, and the state of the holds. The third poll no longer reports `lab-03`, so its pending release completes. `responses_after_polls` are routes answered in the same process after the three polls.
- `openapi.yaml`: `api/openapi.yaml` of `v1.1.0`, byte for byte (`git show v1.1.0:api/openapi.yaml`).
- `recorder_upg_v110_record_test.go.txt` and `recorder_testenv.patch`: the recorder and the small test-environment change it needs (a database path option). They are text, not compiled.

The passwords and tokens in it are not real. The local admin's password hash is that of the unit tests' own password; the service tokens' secrets are derived from fixed labels in the recorder (`recSecret`) and the database stores only their hashes. They open nothing but this fixture.

How it was produced (2026-11-23):

    git worktree add --detach /tmp/old v1.1.0
    cd /tmp/old
    git apply <repo>/testdata/upgrade-v1.1.0/recorder_testenv.patch
    cp <repo>/testdata/upgrade-v1.1.0/recorder_upg_v110_record_test.go.txt internal/api/upg_record_test.go
    RECORD_DIR=/tmp/rec go test ./internal/api -run TestRecordUpgradeFixtureV110 -count=1
    # /tmp/rec/{hourglass.sqlite3,responses.json,poll.json} -> this folder
    git show v1.1.0:api/openapi.yaml > <repo>/testdata/upgrade-v1.1.0/openapi.yaml
    cd <repo> && git worktree remove --force /tmp/old

Run it twice into two directories and `diff -r` them: they are identical.

What the recorder does to make the files reproducible: it replaces the local admin's creation time and password hash (wall clock and a random salt), the migrations' `applied_at`, and the service tokens' `last_used_at` (the auth service stamps the wall clock); it empties the event outbox and the sessions table; it re-inserts the setup labels in key order (the store inserts them in map order); and it runs `VACUUM`. It does not touch the ledger, holds, usage samples, rules, settings, setups, users, reports or audit rows.

Loading it in a test (the environment of `internal/api/testenv_test.go` at this tree): `newBackend(t, backendOpts{start: <now>, loc: Europe/Berlin, dbCopy: "../../testdata/upgrade-v1.1.0/hourglass.sqlite3", runtime: Runtime{Provider: "reported", PressureSource: "reported", MeteringPolicy: "fake-policy", ReportedProviderMaxAge: 10 * time.Minute, ReportedPressureMaxAge: 15 * time.Minute}})`, with the fake provider and pressure source named `reported` and the policy set as `recPolicy` in the recorder (a weight of 3.25 for the hold of `lab-08`, a meter decision for every other). `now` is `responses.json` `now`. Sessions are made with the test's own session helper; the service tokens are `reporter` and `viewer` from the recorder's `recSecret`, or tests mint their own. Replay the polls of `poll.json` in order: advance the clock, set the fake pressure value and allocations, run one poll, and compare the pressure source's snapshot, the policy calls, the samples, the ledger rows and the balances.
