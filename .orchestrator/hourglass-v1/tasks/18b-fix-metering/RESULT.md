# 18b fix metering: RESULT

Status: done. `go test -race -count=1` on collector, settings, holds, ledger: ok. `bazel test` on the same four: 4/4 pass.
`tools/check_spdx.sh` 0, `tools/check_core_generic.sh` OK. All exported API changes are additive.

| Finding (task 18) | Fix | Test |
| --- | --- | --- |
| 1 clock backwards double charge | `collector.poll` takes the newest successful poll of any time (`LastSuccessfulRunBefore(t+100y)`); `T <= prev` charges nothing, warning logged when `T < prev` | collector `TestMET4_ClockBackwardsChargesNothing` |
| 2/3 poll at the boundary before rollover | `RunOnce` runs `Ledger.RolloverDue` before reading T; removed from `Run` (it calls `RunOnce`) | collector `TestLED3_PollAtMidnightOpensPeriodFirst` (cache == 2026-12 ledger sum, acquire allowed) |
| 4 shortened interval false outage | cap = 2 x max(current, interval of the previous successful poll, kept in memory in `Collector.lastInterval`; zero after restart) | `TestADM2_PollIntervalShortenedNoFalseOutage`, `TestADM2_PollIntervalAfterRestartUsesCurrentSetting`; `TestRULE4_ChangesApplyFromNextPollNotRetroactive` updated (it encoded the old behaviour) |
| 5 M10 survives | none needed | collector `TestMET3_ReleasedDuringPollNotCharged` (verified: fails with the guard removed) |
| 6 M4 flaky | none needed | holds `TestHOLD1_LimitRecheckedInTransactionDeterministic` (gated veto; fails every run with the re-check removed) |
| 7 settings clock | new `settings.SaveAt(ctx, w, old, next, actor, at)`; `Save` delegates with `now()` (package var kept as the compat default, so `Save` keeps its signature) | settings `TestSaveAtUsesSuppliedTime` |
| 8 force-release `before_json` | holds writes the pre-release hold (ids, acquired/metering/deployed, `released_at: null`) | assertion added to `TestHOLD2_ForceReleaseNeedsAdminAndReasonAndIsAudited` |
| 9 PollView wiring | Go cannot have field `OK` and method `OK()` on one type, so additive `collector.HoldsView` (embeds `PollView`, has `OK()`) and `PollView.ForHolds()` | compile-time assertions plus `TestHOLD2_PollViewForHoldsAdapter` in collector tests |
| decision: role-less users skipped | no code change | ledger `TestLED3_RolelessUserSkippedAtRollover_DebtNotCarried` |

Notes: collector test BUILD gained `//internal/holds` dep. The store was not touched (out of scope), hence the far-future bound for "newest successful poll".
