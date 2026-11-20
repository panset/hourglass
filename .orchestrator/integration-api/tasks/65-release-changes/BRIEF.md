# 65 Pending release, `hold.releasing`, collector completion, service release

Job intent: `.orchestrator/integration-api/INTENT.md` (read it first).
Contract for task agents: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; follow its
Ground rules, Slow commands and Reporting sections exactly).
Task folder (PROGRESS.md, RESULT.md): `.orchestrator/integration-api/tasks/65-release-changes/`.

## Goal
The release behaviour in the design's "Releasing a deployed setup" section and the route
`POST /api/v1/integration/holds/{id}/release` work as written, with HOLD-6, HOLD-7, HOLD-8,
HOLD-9 and CMP-1 each covered by a test named after the id.

## Context
- Design: `docs/design/integration-api.md`. Read all of it; your sections are "Releasing a
  deployed setup", "`POST /api/v1/integration/holds/{id}/release`", and HOLD-6..9, CMP-1.
  `docs/decisions.md` D43 records the behaviour change.
- Earlier results (read the summaries and API sections): tasks `60-store-reported`,
  `61-service-accounts`, `63-report-state-routes`, `64-reported-extensions` under
  `.orchestrator/integration-api/tasks/`. The store already has `release_requested_at`,
  `released_by_service` and `audit_log.actor_service`. The release route is a 501 stub listed
  in `integrationStubs`; remove it from that map.
- Existing code: `internal/holds/` (release rules, HOLD-2), `internal/collector/`,
  `internal/events/`, `pkg/hourglass` (event types, `Hold`), `hooks/webhook`,
  `internal/api/handlers_user.go` and `handlers_admin.go` (release, force-release).

## Behaviour (all from the design; do not widen it)
- User release, setup reported deployed, `release_requires_teardown` on: 202; set
  `release_requested_at` once; fire `hold.releasing` once. A repeat returns the same 202 and
  fires nothing. Every other situation releases at once exactly as today (200).
- `hold.releasing`: background queue, not vetoable; `user`, `setup`, `hold` as for
  `hold.released`; `data` is `{}`; added to `hourglass.EventTypes()` after `hold.deployed`;
  the webhook hook's `types` filter accepts it. `Hold.ReleaseRequestedAt` in the public type
  and its JSON form (`release_requested_at`, null when unset).
- Pending hold stays active: counts toward the hold limit, the setup stays held, metering
  continues under the policy.
- Completion: on each **successful** poll, after metering, release every hold with
  `release_requested_at` set whose setup is no longer reported. `released_by` is the hold's
  owner; `hold.released` fires with `{"forced": false, "reason": ""}`. A failed poll never
  completes one (HOLD-9). The poll in which the setup disappears is still handled correctly
  for metering (decide per the existing rules for a setup that stops being reported, do not
  invent a new charge rule; say what happens in RESULT.md).
- Service release (`holds:release`): `reason` required, 1 to 500 characters after trimming,
  else 400 `validation`; teardown check skipped; released at once; `released_by` null and the
  account name in `released_by_service`; audit entry `hold.service_release` in the same
  transaction with the account as `actor_service`; `hold.released` with
  `{"forced": false, "reason": "...", "service_account": "<name>"}`; 200 with the released
  hold; 404 `not_found`; 409 `conflict` if already released. It completes a pending release
  immediately, and so does an admin's force-release.
- A pending release cannot be cancelled.
- The existing user-facing hold JSON (setups list, my holds) exposes `release_requested_at` so
  the UI task can show "Releasing". Update `api/openapi.yaml` (202 response on the user release
  route, the new field, the service release schemas), keep the contract test passing with the
  release stub removed, and regenerate `ui/src/api/schema.gen.ts`.
- Existing tests that assert the old 409 `still_deployed` (for example
  `TestHOLD2_ReleaseRefusedWhileDeployed`, and any e2e or UI code path) must be updated to the
  new behaviour; list every one you changed. If the `still_deployed` code becomes unreachable,
  remove it from the code and the OpenAPI file and say so. Do not edit files under `e2e/` or
  `ui/src/pages`: list what there depends on the old behaviour instead (tasks 67 and 68).
- CMP-1: a test that runs a deployment with no service account and no `reported` extension
  (the fake or static provider) through acquire, poll, release and shows behaviour is unchanged
  apart from HOLD-6 and HOLD-7.

## Tests
`TestHOLD6_...` (202, set once, event once, repeat fires nothing), `TestHOLD7_...` (stays
active and is debited while pending; completes at the first successful poll without the
setup; counts toward the hold limit), `TestHOLD8_...` (any active hold, audit row under the
account name, event data, 404, 409, reason validation, completes a pending release),
`TestHOLD9_...` (failed poll leaves it pending; the next successful one completes it),
`TestCMP1_...`. Also: admin force-release completes a pending release; concurrency of a user
release racing a poll does not double-release or double-fire (run under `-race`).

## Scope
May write: `internal/holds/`, `internal/collector/`, `internal/api/`, `internal/events/`,
`internal/store/` (only if a query is missing), `pkg/hourglass/` (the event constant and the
`Hold` field only; not the extension interfaces), `pkg/hourglasstest/` if a fake needs the
field, `hooks/webhook/`, `examples/` if it fails to build, `api/openapi.yaml`,
`ui/src/api/schema.gen.ts` (plus the minimum in `ui/src` needed to keep `npm run typecheck`
and the UI tests green, listed), `BUILD.bazel` via Gazelle, `docs/decisions.md` (append rows
with the next free number), your task folder.
Must not touch: `docs/SPEC.md`, `docs/design/`, `docs/integrating.md`, `e2e/`, git state.

## Done when
The tests above pass; `go test -race -count=1 ./...` and `./ci.sh` pass;
`tools/check_core_generic.sh` passes.

## Long-running steps
`./ci.sh` takes roughly 5 to 15 minutes: run it once at the end, foreground, 30-minute timeout,
output to a log file in your task folder, read only the exit code and tail.

## Report
Follow the Reporting section of the contract.
