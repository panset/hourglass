# Progress: 14 (C2) Settings page

- 2026-11-24 Read BRIEF, COMMON-R3, INTENT, DECISIONS, UX-SPEC section 7 and item 28, DESIGN-R3 C2/UI-23.
- 2026-11-24 form.ts: `labelKeys: string` replaced by `displayLabelKeys: string[]`, kept as loaded and sent back as is (no text conversion). Why: PUT requires the key (D97 i).
- 2026-11-24 index.tsx: removed the "Display label keys" Field (label, hint, error slot). Kept the `.row` wrapper around Default weight alone (UX-SPEC 7 allows it; one flex child leaves no gap). No CSS change needed.
- 2026-11-24 Checked the unowned point: a save error with field `display_label_keys` already renders in the page's general save alert (`errorText` appends the field name, index.tsx:292); nothing drops it. Added a test.
- 2026-11-24 Tests added (UI-23 block, 6 cases); vitest 30/30, tsc, check_spdx, check_core_generic pass. Done; next: none.
