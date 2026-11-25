# Progress: 02-ux-review
- 2026-11-25 Read BRIEF, INTENT, CONTRACT. Next: read Setups UX-SPEC, then walk ui/src pages and shared components, then e2e screenshots.
- 2026-11-25 Read Setups UX-SPEC (setups-declutter task 12) in full; decided to keep its details-panel content, focus-on-Price and wording untouched, and change only the dialog frame.
- 2026-11-25 Read shared components (Modal, Table, Alert, Field, Button, Badge), shell (TopBar, Gate, Layout, session), hooks (useApi, usePanelFocus) and every page. Listed each place a person is shown.
- 2026-11-25 Checked the server name rule: auth.DisplayName (name > username > email) feeds holder, actor, preapproval and session names; AdminUser.name is the raw name and sortUsers sorts by the raw name (admin_views.go:207,236). Decision: the identity spec asks for one display value from the API and a UI fallback for older APIs.
- 2026-11-25 Looked at the e2e screenshots (e2e/out/bundled, setups-declutter 16-c4-e2e, 10-verify panel-375). Next: write UX-REVIEW.md, then RESULT.md.
- 2026-11-25 Wrote UX-REVIEW.md (A1 names with 15-place table, A2 modal rule by kind, A3 pin, B 20 findings, C cut) and RESULT.md. Done.
