# Result: 83 Admin > Users, "Pre-approved users" (D5, UI-11)

Status: done.

## Requirement -> test -> file

All tests in `ui/src/pages/admin-users/preapprovals.test.tsx`; code in `ui/src/pages/admin-users/preapprovals.tsx`.

| UI-11 part | Test |
| --- | --- |
| Columns, explanation line, both sources shown | `UI-11 shows the columns, the explanation and both sources` |
| "Not yet" versus date (and who signed in) | `UI-11 shows Not yet until used, then the date and who signed in` |
| No Remove on a file row | `UI-11 a file row has no Remove button, a database row has one` |
| Same email in both sources = two rows | `UI-11 an email in both sources appears as two rows` |
| Empty list | `UI-11 shows an empty message for an empty list` |
| Remove confirms, DELETE with id, refetch | `UI-11 asks for confirmation, then calls DELETE with the id and refetches`; `UI-11 Cancel sends nothing` |
| Split on commas/spaces/new lines/mixed, empty pieces dropped | `UI-11 splitEmails on ...` (5 cases) |
| POST body, default role, clear text, refetch | `UI-11 posts the split list with the default role developer, clears the text and refetches`; `UI-11 sends the chosen role` |
| Add disabled with no emails | `UI-11 Add is disabled with no email in the text` |
| 400 keeps text, shows message by the text area | `UI-11 a 400 shows the server's message next to the text area and keeps the text` |

## Files changed
- New: `ui/src/pages/admin-users/preapprovals.tsx`, `preapprovals.module.css`, `preapprovals.test.tsx`.
- Changed: `ui/src/pages/admin-users/api.ts` (list, add, delete calls), `index.tsx` (renders the section), `admin-users.test.tsx` (the shared `routes()` helper also serves an empty preapprovals list; no assertion changed), `docs/ui-notes.md`.
- No shared helpers or fixtures touched.

## Commands run
- `npm run typecheck`: clean. `npx vitest run src/pages/admin-users`: 35 pass.
- Deliberate break: Remove shown on file rows; the file-row test failed; restored byte for byte; 35 pass.
- `./ci.sh > ci.log`: exit 0 (`ci.exit` = 0), 39 bazel tests pass including `//ui:test` and `//ui:typecheck`.

## Decisions for docs/decisions.md
1. The section loads its own list, so a failure there shows its own error and Retry without hiding the rest of the page; no polling.
2. The Add button is disabled while the split text holds no email (the page does no other validation).
3. Role choice is a single select (Developer default, Admin) sent as a one-element roles array.
4. "Signed in" shows the date plus the signed-in user's name beneath; "Added by" shows `created_by.name`, or a dash for a file row.
5. The 400 text shown is `error.message` alone (without the "(field)" suffix other forms add); remove errors appear inside the confirmation dialog.
6. Duplicates in pasted text are sent as typed; the server counts them once.
7. The text area is a plain `textarea` styled in a local CSS module, because `components/Field.tsx` has no text-area control and components were out of scope.

## Confidence and gaps
- Verified by the tests and CI above. Not checked in a real browser (no visual check of layout).
- Date text in tests asserts "Nov 12", which depends on the test machine's zone only near midnight UTC; the fixture time is mid-day UTC.
