# Progress: 06 (B2) shared UI pieces

- 2026-11-24 Read BRIEF, COMMON, INTENT, DECISIONS (Q-A one line), change request, DESIGN.md Rev 2 sections 3, 4, 6, 7; read Table, Modal, format and their tests.
- 2026-11-24 Decision: label-cell markup hook is an exported `ShrinkText` component in Table.tsx (DESIGN 3 item 5 asks for `<span class="shrinkText" title>`; a component keeps the CSS-module class private). Q-A one line = default `.shrinkText` rule.
- 2026-11-24 Edited Table.tsx/.module.css (shrink, dense, ShrinkText), Modal.tsx (Shift+Tab, isConnected + fallbackFocus), format.ts (formatRate); added tests.
- 2026-11-24 vitest components+lib 41 passed; mutation check: named Modal tests fail without the fix; full UI suite 259 passed; tsc exit 0.
- 2026-11-24 RESULT.md written. Done.
