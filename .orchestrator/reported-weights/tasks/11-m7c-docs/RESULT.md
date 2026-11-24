# Result: 11 Milestone 7c, documentation

Status: done. `./ci.sh` exit code 0 (`ci.log`, `ci.exit`). Examples run against the real server as a host process (built from this working tree) on 127.0.0.1:18080 with a scratch SQLite file, per the brief's change of plan; the process was stopped by its pid and the scratch directory deleted. `e2e/integration-stack.sh` and compose were not used.

## Files changed
- docs/integrating.md
- docs/deploying.md
- docs/extending.md
- docs/ui-notes.md (item 6 only; task 08 had added the rest)
- docs/progress.md
- README.md
- .orchestrator/reported-weights/tasks/11-m7c-docs/{PROGRESS.md,RESULT.md,examples.log,ci.log,ci.exit}

## Checklist: required statement -> where
Section "Report weights" starts at docs/integrating.md:366.
| Statement | Where |
| --- | --- |
| Reserves, not live utilisation | integrating.md, Report weights rules, first bullet |
| Complete set, one service, every time | rules, second bullet |
| Weights before deployments | rules, third bullet |
| Keep reporting until hold is null; left out is charged at rate-rule price incl. last interval of a release | rules, fourth bullet; lifecycle step 5 |
| Cannot size: leave out, never 0 | rules, fifth bullet |
| Number, not string | rules, sixth bullet |
| Expired report does not stop charging (contrast); check weights.expires_at; alert on 400 | "If you stop reporting" (new paragraph), "What can be refused", Reconcile, checklist |
| not_configured means admin has not turned setting on, not config file | "What can be refused", Errors table (new row) |
| Request, B1 rules, positions from 0, real messages | "The request", "What can be refused" |
| New scope | Step 7 command and scope table; deploying.md service accounts |
| New state fields | "Check that it was taken", Reconcile sample |
| New error cases, checklist lines | Errors table; Verify step 7; Checklist (three new lines, one edited) |
| Setting, four fields | deploying.md "Reported weights" (Settings stored in the database) |
| Tightening the limits (D1) | deploying.md "What tightening the limits does"; integrating.md "What the admin can do" |
| Override | deploying.md "Per-setup override" |
| max_age advice (twice the poll interval; expiry returns to rate rules, charging continues) | deploying.md "Choosing max_age"; integrating.md "If you stop reporting" |
| Upgrade entry 0008 (additive, back up, no down migration, older binary refuses, no config change, off by default, added fields, four shifting fields) | deploying.md "Upgrading to reported weights (migration 0008)" |
| From verification 07: policy hold shows rate-rule weight until first poll; top-level keys case-insensitive | same entry, bullets "Policy holds right after the upgrade" and "Body keys" |
| extending.md: base weight, precedence, policy cannot see it, fixed sentence | extending.md Snapshot section (code comment and rules list) |
| README paragraph | README.md "Reported weights" paragraph |
| ui-notes, progress | docs/ui-notes.md item 14 (task 08) and item 6; docs/progress.md new section |

integrating.md has no link or reference to the design, decisions or source (grep checked). No client-specific name appears in any file I wrote.

## Examples run (all in examples.log)
Sign-in and password change, service account with five scopes and token, state before anything, report while off (409), settings read, turn on without max (400), turn on with max 20, the three reports, state with reported weights, 20 refusals (above max, string, null, boolean, missing weight, negative, 1e400, empty and missing name, repeated, unknown field, attribute not string, bad attribute key, NaN, weights missing/null, entry not an object, future observed_at, stale, not JSON), 403 for an account without the scope and for a weights-only account on state and deployments, weights-only account reporting, override, tightening max (not_applied out_of_bounds, report refused, settings PUT without the key leaves it), restoring, empty list, weight 0, max_age 1m expiry (75 s later source rule, not_applied expired, a poll then succeeded), switching off.

## Findings (differences or things not run)
1. No difference from the design found in the behaviour I ran. Positions count from 0 (matches D82(d)).
2. Not run: a charge at a reported weight (needs a developer user with a hold; the local admin cannot acquire and no OIDC provider was running), the Setups page and acquire dialog, 10,000-entry and 1 MB limits, a 16-attribute limit, negative zero, equal-observed_at replacement, turning the setting off and on again within max_age. Those statements in integrating.md come from the design and the unit tests, not from my run. Verification 12 or the e2e run should cover them.
3. Docs show times of the run (2026-11-24T06:xx); the earlier examples in the file use 2026-11-20. Values are real.
4. The audit-log step in examples.log failed on my quoting (a typo in the transcript, not the server); harmless.
5. The settings object returned by GET is wrapped (`{"settings": ..., "runtime": ...}`); the guide's jq uses `.settings`, and PUT takes the inner object. deploying.md's older sentence "fetch it with GET, change a key, send it back" glosses over this; I did not rewrite it.
6. The weights report's validation message for a repeated or empty name has no `field` other than `weights`; documented as is.

## Decisions for docs/decisions.md
None new. I only documented D80 to D88. D86(b) and (c) are stated in the upgrade notes as the brief asked.

## Confidence and gaps
Verified: every command and response quoted in the Report weights section, against the real server. Inferred (from design and decisions): items in finding 2, and "users see the weight on holds and debits" (UI tests). Not looked at in a browser.
