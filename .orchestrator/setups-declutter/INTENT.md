# Intent: setups-declutter

Date: 2026-11-24. Base: `main` at `f1a05c2`.

## Goal (the person's words)

"You are the lead developer for Hourglass. UX team has requested a change docs/design/setups-declutter.md

1) Please evaluate the ask and come up with a design plan
2) Have a fable subagent (high effort) to review the design plan. Look for corner cases, a fit with other areas of Hourglass etc
3) Implement
4) Test
5) Merge to main (remote)
6) Release images."

The change request itself is `docs/design/setups-declutter.md` (currently untracked; it is committed with the change). It is the source of truth for requirements UI-15..UI-20, the amended UI-12 / SET-6 / WGT-14, the owner decisions (not to be reopened), constraints, "What else changes" and "Done when".

## Must stay true

- Owner decisions in the change request are final.
- UI-1: the page computes no number. Generic: no customer label key / attribute / extension name in code or test assertions beyond fixtures.
- Any API change is additive (openapi, generated types, contract test updated; nothing removed or renamed).
- Acquire dialog, My usage, every admin page, and the API's existing fields are unchanged.
- Out of scope: column chooser / saved views / per-user settings, sorting/paging/grouping, changes to weight/multiplier/cost computation, a limit on display label keys.
- Subagents for this job run on Fable or Opus only (person's instruction). No Sonnet/Haiku.
- Unrelated untracked files (`testdata/upgrade-v1.1.0/*.sqlite3-shm|-wal`, other `.orchestrator/` jobs) are not touched or committed.
- The person's global rule: code review of a diff goes through `/deepseek-review`, not read on the main model. (Independent verify agents running tests and trying to break the build are fine.)

## Done looks like

1. UI-15..UI-20 and amended UI-12 tests pass; `./ci.sh` passes; e2e modes touching the Setups page pass.
2. Screenshots (1280 px, five label columns, reported weights on; lab at 0x and at a higher band; page and open panel) exist for the owner.
3. Docs in "What else changes" updated, D93+ decisions recorded.
4. Merged to `main` on the remote.
5. Images released by the repo's release process.

## Amendment, 2026-11-24 (person, after seeing the first screenshots)

"1) In the 'Setups' page, lets remove the label columns. Its looks very much cluttered
2) Lets get rid of the Available labels as well. We should instead just let users start typing in the 'Label Selector' checkbox and we could autofill values."

This overrides the change request where they conflict. Follow-up answers are R3-1..R3-3 in DECISIONS.md: the admin "display label keys" setting leaves the Settings page (API field and stored value stay, unread); unselectable label values are suggested by the autocomplete and matched exactly in the browser as a removable token; labels are visible in the details panel only. Table columns: Setup, State, Held by, Cost, actions. The change request doc and requirement ids are amended to match.
