# RESULT 17c-holds

Status: done.

## Files changed
internal/holds/{holds.go, holds_test.go, BUILD.bazel}; this folder (PROGRESS.md, RESULT.md, bazel-test.log). Nothing outside these.

## Exported API (package holds)
- `NewService(Deps) *Service`; `Deps{Store *store.Store, Events Dispatcher, Poll func() PollView, ProviderConfigured bool, Clock Clock, Logger *slog.Logger}`.
- Interfaces: `PollView{OK() bool; Reported(resourceName string) bool}`, `Dispatcher{Veto(ctx, hourglass.Event) error; Emit(hourglass.Event)}` (`*events.Dispatcher` satisfies it), `Clock{Now()}`.
- `(*Service).Acquire(ctx, userID, setupID int64) (Hold, error)`; `(*Service).Release(ctx, actorID, holdID int64, ReleaseOpts{Force bool, Reason string}) (Hold, error)`; `Hold = store.Hold`.
- `*Error{Code, Message}`, `CodeOf(err) string`; `MessageStillDeployed`; audit action `AuditForceRelease` = "hold.force_release".

## Error codes (same strings as api/openapi.yaml ErrorCode)
forbidden, not_found, setup_disabled, setup_held, insufficient_tokens, hold_limit, hook_veto (message = the hook's message), still_deployed, validation.
Acquire order: forbidden, not_found, setup_disabled, setup_held, insufficient_tokens, hold_limit, hook_veto, then create (ErrSetupHeld -> setup_held).

## Decisions for docs/decisions.md
1. Acquire/Release take user ids, not user objects; the service loads roles and auth source itself, so a stale session cannot grant rights (the brief's `user` read as "identified user").
2. Veto runs outside any DB transaction (a hook may take 30 s). After it, the hold-limit count is re-checked inside the write transaction that creates the hold, so concurrent acquires by one user cannot exceed the limit; the partial unique index decides races for one setup. A veto therefore sees a hold-limit-correct state but the limit can still refuse after the veto in a race (code hold_limit).
3. The `hold.acquiring` event carries a Hold with ID 0 and AcquiredAt = the time the hold will get (it does not exist yet); `hold.acquired` carries the stored hold.
4. Balance check uses the cached `users.balance_mt` (<= 0 blocks while enforcement is on).
5. Release authorisation: no role -> forbidden; owner may release own; Force requires admin (an owner who is not admin cannot pass Force); an admin who is not the owner must pass Force, otherwise forbidden. An admin releasing their own hold without Force follows the normal teardown rule; with Force it is audited like any force-release.
6. Release order: forbidden (no role), not_found, forbidden (not owner / force without admin), validation (blank reason with Force), still_deployed.
7. Stored `release_reason` is the reason as given; the blank check and the audit row use the trimmed reason. `released_by` is always the actor.
8. Force-release audit: action `hold.force_release`, target `hold:<id>`, after_json `{hold_id, setup, user_id, reason}`, no before_json; written in the same transaction as the release.
9. `hold.released` carries the hold owner as User (not the actor) and Data `{"forced": bool, "reason": string}` (reason "" when none given).
10. Still-deployed check uses `settings.ResourceName` with the current template, labels and override; it applies only when the setting is on, ProviderConfigured is true, Poll() is non-nil and OK().
11. A Veto error that is not an `*events.VetoError` (e.g. cancelled context) is returned unwrapped, not as hook_veto.
12. Events are emitted after commit; emit failures cannot affect the result. If the owner cannot be loaded after release, the event is skipped and logged.

## Checks (all run, all pass)
- `go test -race -count=3 ./internal/holds/...` ok; go vet and gofmt clean.
- `bazel test //internal/holds/...` PASSED (log: bazel-test.log).
- `./tools/check_spdx.sh` exit 0; `./tools/check_core_generic.sh` OK.
- Tests (names cite HOLD-1/2/3/5, MET-6): all refusals in order, shadow mode, limit 2/third refused, lowered limit keeps holds, veto message and nothing created, veto timeout (real events.Dispatcher + FakeHook gate), 8 concurrent acquires of one setup (exactly one wins), concurrent acquires by one user respect the limit, local admin refused, release refused while reported / allowed after / allowed when no provider, setting off, no poll, failed poll, override resource name, force-release rules and audit, event order and content.

## Confidence and gaps
- Appears fine under -race; the real-collector PollView is untested here (only a fake).
- No store changes were needed. Not tested: the hold.released event path when the owner row is missing (cannot happen with foreign keys).
- The openapi lists hook_veto etc. as 409 and forbidden as 403; mapping to HTTP is left to the API layer.
