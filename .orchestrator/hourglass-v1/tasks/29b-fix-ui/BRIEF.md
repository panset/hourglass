# 29b fix UI verification findings
Job intent: .orchestrator/hourglass-v1/INTENT.md. Rules: .orchestrator/hourglass-v1/CONTRACT.md. Findings with screenshots: .orchestrator/hourglass-v1/tasks/29-verify-ui/RESULT.md. Scaffold conventions: tasks/25-ui-scaffold/RESULT.md.
## Changes
1. Error boundary around the routed pages and around lazy chunk loading: a failed chunk load or a render error shows a message with a Reload button instead of a blank page. Test.
2. Side panels and dialogs (Grant, Reset, Edit setup, role change, any other the finding lists) move focus into the panel when opened, trap it while open, close on Escape and return focus to the control that opened them. Tests.
3. A network failure on the first `GET /api/v1/session` shows a "Cannot reach the server" state with Retry, not the sign-in page (only a 401 means "not signed in").
4. Remove client-side business computations flagged under UI-1: any "Free for X more" style countdown derived in the browser is replaced by the policy reason text the API returns (or nothing when null). "Held for" may stay as formatting of the elapsed time since the API's timestamp. The 12-character hint on the password form may stay as a hint, but the API's error is what decides.
5. The Setups and Setup inventory layout problems listed as cramped or clipped at desktop widths (1280 and 1440): fix so no column content is clipped; long names and many labels wrap or truncate with a title. Mobile widths are out of scope.
6. Timestamps: display all times in the browser's local zone with the zone abbreviation shown once per table or as a tooltip giving the UTC value; day labels in the usage chart use the dates the API returns unchanged.
7. Every other LOW finding where the fix is local and obvious; for any you leave, say why.
## Scope
May write: `ui/src/` (not the generated schema), this task folder. No API or Go changes; no new dependencies. Keep every label that the end-to-end test might click unchanged unless a finding requires otherwise, and list any label or role you changed in RESULT.md under "Changed selectors".
## Done when
`bazel test //ui/...` and `bazel build //ui/...` pass (allow 15 minutes), `./tools/check_spdx.sh` passes. RESULT.md maps each finding to fixed / left with reason.
