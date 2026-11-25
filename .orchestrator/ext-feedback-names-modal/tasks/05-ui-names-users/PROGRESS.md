# Progress 05-ui-names-users (2026-11-25)
- Read brief, intent, contract, UX-REVIEW A1/A3/B1.2/B2.3/C1, task 01/03/04 results.
- Decision: no UI helper existed (greeting uses session.display_name), so added ui/src/lib/person.ts.
- Done: helper, Users page (pending, All users, panel, toasts, labels), Audit actor filter and column, Setups and Setup inventory Held by and panel, Acquire wording, Burning now and holds units, pressed state CSS, fixtures display_name.
- Done: tests (person.test.ts, admin-users/names.test.tsx, additions to setups, admin-setups, admin-audit tests), docs/ui-notes.md.
- Final: vitest 359 passed (ui6.log); ./ci.sh exit 0 (ci.log).
