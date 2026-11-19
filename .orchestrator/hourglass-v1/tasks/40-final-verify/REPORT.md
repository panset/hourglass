# Hourglass v1: final verification report

Date: 2026-11-19. Verified commit: `05aaa3a`, from a fresh clone of
`git@github.com:panset/hourglass.git`. Written by the verification agent;
saved here by the orchestrator, lightly condensed. Evidence beside this
file: `e2e-bundled.log`, `e2e-split.log`, `perf.log`,
`zz_perf_verify_test.go.txt`, `screens/`.

**Bottom line at that commit: not finished.** Three of the four "done"
criteria passed; the fourth failed because `docs/progress.md` was stale,
and one spec requirement failed when measured (an admin call at about 0.6 s
at spec scale against a 0.5 s limit). Task 41 addresses the findings.

## 1. Verdict per "done" criterion

1. **Clean-checkout build, test and API-only binary: yes.** `bazel build //...`
   built 254 targets; `bazel test //...` passed 35 of 35 targets, each with
   real tests; `CGO_ENABLED=0 go build ./...` passed; the `go build` binary
   answered `/healthz` 200, `/api/v1/auth/methods` 200 and `/` 404 and logged
   that it was built API-only; `./ci.sh` exited 0 without `ui/node_modules`.
2. **Acceptance path in both modes: yes.** `e2e/run.sh bundled` exit 0 in
   716 s; `e2e/run.sh split` exit 0 in 722 s; steps A0 to A9 pass in both.
3. **Permissions table covered by HTTP tests: yes.** Every v1 cell has a
   real HTTP assertion through the full router with 8 actors; 30 routes x 8
   actors = 240 combinations; 151 passing tests and subtests, none skipped.
   A seeded setup-name leak on `/healthz` turned 4 tests red; reverting it
   turned them green.
4. **Everything pushed, progress.md current: no.** Everything was pushed,
   but `docs/progress.md` still described the paused state.

## 2. Not finished or not verified at that commit

- `docs/progress.md` stale. The 500 ms requirement fails for
  `/admin/users` (H1). A 32 MB binary remains in pushed history.
- Left on purpose for the owner: D31 and D23.
- Not verified: anything needing a real vCenter; accessibility beyond
  panel focus; mobile widths (out of scope); metrics output (only that
  `/metrics` is 404 on the public port and served from a separate listener).

## 3. Findings

### High

**H1. The 500 ms requirement fails for the admin Users list.** Data set:
200 users, 100 setups, 100 active holds, 18 days of 5-minute polls (518,400
usage samples and ledger debits), measured in-process through the real
router. `GET /admin/users` 614-673 ms; with `status=pending` 612-739 ms.
Every other route is under 500 ms; `GET /setups` is closest at about
310 ms (worst 416 ms). `GET /me/ledger` returns a 2.3 MB body. One
collector poll took 2.8 s. Likely cause: every debit re-sums the user's
whole period ledger (`internal/store/ledger.go`, `RebuildBalance`), and the
admin list aggregates the ledger for every user on each request.

### Medium

- **M1.** `docs/progress.md` stale.
- **M2.** Several commits are milestone-sized (`b349a63` 6,111 lines,
  `7a86e41` 4,906, `5209dbc` 3,697, `2e4d9ff` 1,881). History is otherwise
  linear and plausible: 75 commits, one identity, one ordinary merge,
  requirement ids cited.
- **M3.** A 32 MB `simulator` binary (blob `db0b5153`) was added in
  `b349a63` and removed in `74c76ba`; it is the only blob over 1 MB in
  history.

### Low

- **L1.** Stray tracked file `internal/api/user_test.go-E`.
- **L2.** AUTH-8, RULE-5 and UI-2 have tests but none named with the id.
- **L3.** The acceptance test makes direct API calls in A1 and A6 that D34
  does not list; in A6 the UI disables Acquire and shows the hold-limit
  message, and the 409 itself is observed through the API call.
- **L4.** Dead branch in `readLedger` in the acceptance spec.
- **L5.** The compose stacks default to the `*-e2e` images with the clock
  control port.
- **L6.** On a fresh clone `e2e/run.sh` runs the Playwright browser install,
  which hangs on this machine; its message points to a missing README.
- **L7.** The reference scenario's template and display key are set by hand
  after first start (D33).
- **L8.** Held setups that are not yet metering show an empty Cost cell.
- **L9.** Small misalignments on Settings (band inputs) and the Audit log
  filters; the Users page has no stat cards or search box (in the
  wireframe, not required by the spec).
- **L10.** Tracked `.orchestrator` files contain local filesystem paths. No
  secrets.

### Checked and fine

- **Secrets:** all of `git log -p` scanned: no keys, tokens, real hosts,
  IPs or personal email addresses. Development credentials are obvious
  fakes (`dev-only-...`, `user`/`pass` for the simulator) and labelled so.
- **License:** `LICENSE` is byte-identical to the MIT text with the right
  copyright line; SPDX check passes; dependency license check passes
  (25 Go modules, 267 npm packages; exceptions in D30).
- **Core generic:** `check_core_generic.sh` passes; an independent grep
  finds only the word "inventory" in comments and `_RP` as a template value
  in a few core tests.
- **No vCenter needed:** every URL is local or a reserved example domain;
  the end-to-end simulator moves the real figures the unchanged `vsphere`
  pressure source reads.
- **Spec structure:** all 11 events fire; all 15 tables exist plus
  `outages`; every Configuration row exists; the route table matches the
  spec's 30 HTTP API rows exactly; all Dashboard page routes exist.
- **Features not in the spec:** no extra API routes. Small extras:
  `/ui-healthz` in the UI container; `state` on `/me/holds`, `limit` and
  `offset` on `/admin/audit`; session fields `is_local_admin` and
  `must_change_password`; Change password and Not found pages; event queue
  and failed-delivery panels on Settings; the `outages` table (D10).
- **Acceptance spec review:** each of the eight steps is asserted through
  the UI against values the run produces (for example A5: the pill moves
  from 1x to 2x and the debit ratio is 2.03; A8: carry-over -100.267,
  allowance +300, balance 199.733).
- **Real pages:** 35 screenshots at 1280x800 and 1440x900 as admin and as a
  developer. No horizontal overflow, no wrapped headers, time headers show
  the zone, panels take focus. Apart from L8 and L9 nothing clipped,
  overlapping, missing or showing raw errors.

## 4. Requirement coverage

Every requirement id in the spec (AUTH-1..8, SET-1..7, RULE-1..5,
HOLD-1..5, MET-1..7, LED-1..6, ADM-1..5, EXT-1..7, UI-1..7) has an
implementation and tests; none is missing. Three (AUTH-8, RULE-5, UI-2) had
no test named with the id (L2).

## 5. Decisions for the owner to review

Marked REVIEW: D3, D6, D8, D10, D11, D12, D13, D19, D22, D23, D27, D28,
D29, D30, D31. Also worth a look: D33, D34, D35, and whether to rewrite
history to drop the 32 MB binary.

## 6. Only a real vCenter can settle

- [ ] The read-only role has every privilege the provider needs.
- [ ] The real pool layout matches `{{.SetupName}}_RP`; clusters in
      sub-folders are not discovered (D27).
- [ ] The cluster numbers agree with vCenter; they are read as
      reservations (D6).
- [ ] TLS and certificates work with `insecure_skip_verify` false.
- [ ] A poll finishes well inside the interval at real inventory size.

## Open items accepted from earlier verifications

Selector values (D13); millisecond timestamps; event JSON compacts
`details`; debt escapes through role removal (D23); per-account sign-in
limit (D31); narrow-screen overflow; top bar and page both poll the same
endpoints; `check-provider` may create SQLite side files next to a WAL
database; vCenter sessions are not logged out at shutdown.
