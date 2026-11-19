# 18 verify metering: RESULT

Status: done. Verified commit e7bd862 (HEAD of a fresh clone at
`/private/tmp/claude-501/-Users-tejaspanse-projects/17dec24c-e7ec-455e-9b87-01d2217d5fc9/scratchpad/verify-18/hourglass`).

## Verdict

**Pass with findings: no HIGH, 3 MEDIUM, 6 LOW.** The acceptance path runs end to end at package level. The ledger
amounts match hand-computed milli-token values exactly. At every step the ledger debits equal the usage-sample debits
(checked per user and per hold), and the balance cache equals the ledger sum for the current period. Idempotence,
atomicity, refusal order, concurrency and the teardown matrix all hold.

Four adversarial tests fail and are reported below. One of them is a real double charge, which happens when the clock
steps backwards.

## Findings

1. **MEDIUM: a backwards clock step double-charges.** Cause: `internal/collector/collector.go:287` takes "previous
   successful poll" as the newest ok run *strictly before T* (`internal/store/collector_queries.go:17`). After polls at
   10:05 and 10:10, a clock stepped back to 10:07 makes the previous poll 10:05. The collector then charges 10:05 to
   10:07 a second time (67 mt for a weight-2 hold). The usage-sample key (hold, 10:07) is new, so MET-5 idempotence
   does not catch it.
   - Repro: `TestMET4_ClockBackwardsChargesNothing` (evidence/money_test.go.txt). It gets samples `[0 167 67 167]`
     and charges 67 mt again.
   - Fix: use the newest successful run regardless of T. If `T <= prev`, charge nothing. Keep the same-instant case:
     at `T == prev` the elapsed time is 0, and the sample already exists.
   - Collector decision 1 in 17-collector/RESULT.md is the root of this.
2. **MEDIUM: debt escapes through role removal across a rollover.** `ListRoleHolders` (`internal/ledger/service.go:290`)
   skips a user who has no role at rollover, so their negative balance is never carried. When a role is given back
   later, `EnsureFirstAllowanceTx` (`service.go:257`) finds no allowance in the new period and writes the full amount.
   - Repro: `TestLED3_DebtSurvivesRoleRemovalAcrossRollover`. The user goes -40, roles are removed, the month rolls
     over, the role is restored, and the balance comes back as 300 instead of 260.
   - The spec says debt carries over and only a reset clears it (LED-5), but it is silent on role removal. Open
     question for the owner.
   - Fix if debt should survive: when the first allowance is written in a period, also carry the user's last non-empty
     period's negative closing balance. The alternative is to keep role-less users with a non-zero balance in the
     rollover.
3. **MEDIUM (spec reading): a poll at 00:00 before the rollover leaves the cache negative and blocks acquire.**
   - `Run` calls `RolloverDue` and then `RunOnce`, and each reads the clock separately (`collector.go:140`, `:145`).
     The same happens when `RunOnce` is called on its own.
   - If the boundary falls between the two reads, the debit lands in the new period first
     (`internal/ledger/service.go:122`). The cache becomes `-167`, and with enforcement on, Acquire refuses with
     `insufficient_tokens` (`internal/holds/holds.go:199`). This lasts until the next cycle's rollover, up to one poll
     interval later.
   - Nothing is lost or double-counted: after the rollover, Nov = 0 and Dec = 300000 - 167.
   - The debit also covers 23:55 to 00:00 of the old month but is booked in the new one. That follows the "debit
     belongs to the poll's period" choice, and the owner may want it the other way.
   - Repro: `TestLED3_PollAtBoundaryBeforeRollover`.
   - Fix: run `RolloverDue` inside `RunOnce` with the same T, or have `DebitTx` run the pending rollover first.
     Either way, take the debit period from `prev`/T consistently.
4. **LOW: shortening `poll_interval` turns the next poll into an uncharged outage.** `collector.go:418` judges the gap
   against the *new* interval, but the wait already scheduled uses the old one. Changing 5m to 1m gives a 5-minute gap,
   which is more than 2 × 1m. That 5 minutes is not charged for anyone, and a false outage row is shown to admins.
   - Repro: `TestADM2_PollIntervalShortened`.
   - Fix: cap at 2 × max(interval at previous poll, current interval), or remember the interval that was used to
     schedule the wait.
5. **LOW: the collector's own tests do not cover the in-transaction "released meanwhile" check.** Mutation M10 (deleting
   the `cur.ReleasedAt != nil` guard at `collector.go:478`) survives `internal/collector` tests.
   - My `TestMET3_ReleasedDuringPollNotCharged` kills it. It uses a policy that releases the hold during `Decide`.
   - Fix: add that test to the package.
6. **LOW: the holds limit race test is nondeterministic.** Mutation M4 (dropping the in-transaction limit re-check at
   `holds.go:228-234`) was caught in 6 of 9 runs by `TestHOLD1_ConcurrentAcquireRespectsLimit`
   (`internal/holds/holds_test.go:328`).
   - Fix: use a gated veto hook so that two acquires are both past the pre-check before either creates its hold.
7. **LOW: the clock rule is broken in settings.** `internal/settings/settings.go:336` has `var now = time.Now`, which
   `Save` uses to stamp `updated_at`. It is not injectable from production code.
   - Fix: take a time argument or a clock in `Save`.
   - The only other `time.Now` hit is `internal/events/events.go:36`, the default clock, which is acceptable.
8. **LOW: the force-release audit row has no `before_json`** (`internal/holds/holds.go:319`). ADM-3 asks for before and
   after values.
   - Fix: write the hold's pre-release state, for example `{"released_at":null,...}`.
9. **LOW (wiring): `collector.PollView` does not satisfy `holds.PollView`.** The collector has an `OK` field
   (`collector.go:67`) where holds expects an `OK()` method (`holds.go:64`). The API wiring therefore needs an adapter;
   my harness has one (`pollAdapter`).
   - Fix: add an `OK()`-style method, or rename the field, so the two can be wired directly.

Observations (not defects):

- With two collector instances 1 s apart, the second charges only that 1 s (1 mt), so there is no double charge.
- A setup disabled under a hold by bypassing SET-5 at store level is still charged and leaves the Snapshot. A delete
  under a hold is refused by the foreign key.

## Checks

1. Build and tests:
   - `go vet ./...`: exit 0.
   - `go test -race -count=1 ./internal/...`: all ok.
   - `bazel test //internal/...`: 9/9 pass.
   - `tools/check_spdx.sh`: 0.
   - `tools/check_core_generic.sh`: OK (60 files).
   - Test counts:

     | Package | Top-level tests | Including subtests |
     | --- | --- | --- |
     | events | 15 | 15 |
     | settings | 8 | 8 |
     | ledger | 28 | 96 |
     | collector | 21 | 26 |
     | holds | 17 | 21 |

     The ledger counts include the pure `ledger_test.go`.
2. End-to-end path (`TestE2E_AcceptancePath`): passes. It uses the real store, events dispatcher with a FakeHook and
   the store as recorder, ledger service, collector, holds service, real `default` policy, fake provider, fake pressure
   and fake clock. Every amount was checked by hand:
   - The allowance is written once even when the role is assigned twice.
   - No samples are written before the provider reports.
   - At the poll where the provider reports: metering starts, the hold is deployed and a 0 debit is recorded.
   - Debits in mt: `167, 167, 167, 333`. That is 15-minute smoothing of 0.6/0.6/0.8 → 0.667, then 0.733, then 0.8 →
     2×.
   - A third acquire gets `hold_limit`.
   - Release is refused while the setup is reported. Once it is not, the hold pays one more 2× poll (333) before the
     release succeeds.
   - The balance is driven to −40 tokens and the month rolls over: balance 260000, carryover −40000.
   - The event sequence and payloads (user, setup labels and details, hold) are as expected.
   - The delivery-record count equals the number of hook calls, and one outage row is written.
3. Money safety: every invariant above holds at every poll.
   - Same-instant dual collectors, a restart between polls (charges the 2 min gap = 67), a policy error on the last
     hold (no rows written, run recorded failed, `poll.failed`), a closed DB, release between and during polls, rule
     change forward-only, policy override (250), and weight-0 (sample only): all pass.
   - Fail → good at exactly 2× the interval is charged in full. Fail, fail → good writes an outage row covering 10:15
     to 10:30, with no charge.
   - Two failures: finding 1 (clock backwards) and finding 4 (poll interval shortened).
4. Rollover: all pass except findings 2 and 3.
   - The New York boundary is exact: 23:59:59.999 closes nothing and 00:00 local closes November.
   - Idempotent.
   - Down across two boundaries: debt carries and expires correctly.
   - A user approved during a pending rollover ends with exactly one allowance.
   - An allowance change applies at the next rollover.
   - The local admin and role-less users have no ledger rows.
5. Events:
   - Each transition fires once, after commit and in order; mutation M3 is killed.
   - `balance.low` and `balance.overdrawn` fire exactly once across ten debits.
   - `poll.failed` fires.
   - A slow, gated hook does not delay polls: 3 polls took under 1 s.
   - The veto message reaches the caller verbatim.
   - Delivery records are written.
6. Holds:
   - The refusal order is as specified.
   - In shadow mode a zero balance passes and the next refusal is the limit.
   - 20 goroutines × 3 setups (5 rounds) never produce more than 2 holds, and every error is a coded refusal.
   - Two users racing for one setup (10 rounds): exactly one wins, and the other gets `setup_held`.
   - Force-release rules and the audit row are correct.
   - The 16-cell teardown matrix refuses only when provider, setting, poll-OK and reported are all true.
7. Settings: the boundaries hold.
   - Poll interval 59s is rejected and 60s accepted.
   - Bands that are unsorted, duplicate, start at 0, start above 1 or have a negative multiplier are rejected; a band
     from 1 is accepted.
   - A negative allowance is rejected and 0 is accepted.
   - Bad, unknown-field and empty templates are rejected.
   - The JSON round trip is stable, unknown fields are rejected, and Save writes only the 3 changed keys.
8. Genericity and SPDX: clean. `time.Now`: see finding 7.
9. Mutations (`evidence/mutations.txt`): 13 run.
   - Killed by the packages' own tests: M1 (skip idempotence), M2 (charge on metering-start poll), M3 (emit before
     commit), M5 (off-by-one on the 2× cap), M6b (carry positive balances), M7, M8, M9, M11, M12, M13.
   - M4 is killed only intermittently (finding 6).
   - M10 survives the package tests (finding 5).
   - M6 at the pure `Rollover` switch was an equivalent mutant (the `> 0` case wins first) and is discarded.
   - M1 was re-run after a build-failing first form.
10. Decisions review: `decisions-rows.md`.
    - All five RESULT.md decision sections are consistent with the spec, except collector decision 1 (finding 1) and
      holds decision 8 (finding 8).
    - Ledger decision 3, combined with D21, explains finding 3's window.

## Files changed

- `/Users/tejaspanse/projects/hourglass/.orchestrator/hourglass-v1/tasks/18-verify-metering/`:
  - `RESULT.md`, `PROGRESS.md`, `decisions-rows.md`
  - `evidence/*.go.txt`: harness, e2e, money, rollover, holds_events and settings tests
  - `evidence/mutations.txt`, `evidence/mutate.py.txt`, `evidence/zverify-run.txt`
  - `vet.log`, `gotest.log`, `bazel.log`
- Clone only: `internal/zverify/` (the same tests). Nothing was changed in the real repository.

## Confidence and gaps

- High confidence in findings 1–4 and in the e2e amounts: they are reproduced by tests.
- Not tested:
  - `Run`'s loop timing with a real `After`, and the daily prune.
  - The events package's `Close` under load.
  - The API layer's role and approval wiring. I mimicked it as `SetUserRoles` + `EnsureFirstAllowanceTx` in one
    transaction.
- Finding 2's severity depends on how the owner reads the spec.
- The adversarial tests need the `policies/default` package and are not in a Bazel BUILD; run them with
  `go test ./internal/zverify/` in the clone.
