# Intent: ext-feedback-names-modal
Date: 2026-11-25

## Goal (the person's words)
Hourglass is an open source project, a generic service to manage/track developer setup usage. An external team consumes it and reported:

1. "The external team recently had surfaced an issue where we were displaying their email ids in the top right 'Hello x@y.com' greeting. This happened because 'name' field was only exposed by making a userinfo endpoint call during OIDC authflow. They report that this is no longer an issue. They see the full name in the greeting section. However, other places in hourglass still show email id instead of the full name. e.g 'Held By' section still shows email-id"
2. "On clicking the details link on Setups, a modal opens up. You can only close it after scrolling to the bottom and hitting 'close'. We should ideally close it by clicking anywhere else on the screen (and optionally have a cross icon on top right of the modal)"
3. "In Users screen for admins, local accounr admin should always be the 1st user."

Steps asked for: 1) Evaluate these asks 2) Get a senior UX designer (sub agent) to review any UX improvements; "Ask for a holistic view and any other enhancements." 3) Build a plan 4) Implement 5) Test 6) Release images.

## Must stay true
- Open source, generic: nothing specific to the external team (no names, domains, IdP specifics) in code, tests, fixtures or docs.
- Existing API stays backward compatible (additive changes only).
- Where a user has no name, fall back sensibly (as the greeting and audit log already do: username or email), never blank.
- Emails must not be exposed to people who could not already see them.
- Commit with explicit `git add` paths only. No PRs: ff-merge to main, push, annotated tag -> release.yml builds images (see `.orchestrator/setups-declutter/tasks/02-release-map/RESULT.md`).

## Done looks like
- Every place in the UI that identifies a person shows the full name (with the fallback), "Held by" included.
- Setups details modal closes on backdrop click, on Escape, and by a close (x) button at the top right; accessible (focus, aria).
- Admin Users screen lists the local admin account first, always, whatever the sort.
- UX designer's must-fix items related to these are in; larger ideas are listed as follow-ups for the person.
- `./ci.sh` green, e2e green, independent verify passed, images released under a new tag.
