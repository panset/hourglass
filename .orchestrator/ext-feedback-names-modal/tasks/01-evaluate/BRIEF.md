# 01 Evaluate the three asks
Job intent: .orchestrator/ext-feedback-names-modal/INTENT.md (read it first)
Rules: .orchestrator/ext-feedback-names-modal/tasks/CONTRACT.md (read it, follow it)

## Goal
For each of the three reported issues, establish the root cause and the exact change needed, so a builder can start without exploring. Read-only: change no source file.

## Context
- Repo: Go backend (`internal/`, `cmd/`, `api/openapi.yaml`), React/TS UI (`ui/src`), e2e in `e2e/`.
- Recent related commits: 4c40a4c (SSO name from the UserInfo endpoint, AUTH-9, D92), 348c5f9 (audit log shows an actor with no name by username or email, ADM-3), 6a18780 (Setups page details panel/modal).
- `docs/` holds SPEC and design docs with requirement ids (AUTH-9, ADM-3, UI-xx, Dxx); note which ids/doc sections each change would touch and the next free ids.

## Questions to answer
1. Names. List EVERY place the UI (and any API response, CSV/export, notification, webhook, log shown to users) identifies a person: page, component `path:line`, which API field feeds it, and what it shows today (name / username / email / id). "Held by" on Setups is the reported one; find the rest (setups list, details modal, usage, ledger, reservations/holds, queue, audit log, admin users, service accounts, history, tooltips, filters/autocomplete options, sort keys). For each: does the API already carry a display name there? If not, where is the value stored (is the holder stored as an email/username string or a user id?), and what is the smallest additive API change to carry a display name. Is there an existing shared helper for "name, else username, else email" (the greeting and audit log use something): where, and who does not use it. Note any privacy rule: who may see whose email today.
   Also: a user who has not logged in since the AUTH-9 fix has no stored name. Say what is stored when, and whether the name refreshes at each login.
2. Modal. The shared Modal component and the Setups details usage: how it closes today, whether backdrop click / Escape / a header close button / focus trap / focus return / scroll lock / aria exist, why the close button is only at the bottom (is the whole modal scrolling instead of the body?), and every other user of the Modal component (so a shared fix is checked against all of them, especially forms where a backdrop click could lose input).
3. Users order. How the admin Users screen orders rows today (server and client), how the local admin account is identified (a flag, auth source = local, a fixed username, bootstrap config?), whether there can be more than one local account, and where a "local admin first" pin best goes (API order vs UI sort) given the table's sortable columns and any pagination.
4. Tests. Which unit tests, UI tests and e2e steps cover these areas today and would need to change or be added; the commands to run them and how long they take (see `.orchestrator/setups-declutter/tasks/02-release-map/RESULT.md`).

## Scope
May read: the whole repo. May write: only `.orchestrator/ext-feedback-names-modal/tasks/01-evaluate/`. Must not touch: any source, git state.

## Done when
RESULT.md answers 1-4 with `path:line` evidence, starts with a summary of at most 25 lines, and ends with a proposed change list per issue (files, size S/M/L, risk).

## Long-running steps
None expected. Do not run the e2e suites.
