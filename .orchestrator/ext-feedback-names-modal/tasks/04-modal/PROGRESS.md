# Progress 04-modal
- 2026-11-25 Read brief, UX-REVIEW A2/B1.1, Modal code and call sites.
- 2026-11-25 Rewrote Modal.tsx/Modal.module.css (header + x, body-only scroll, backdrop press guard, dismissOnBackdrop, busy, IME guard, ref-counted scroll lock). Decision: always-on 1px rules (spec allows) instead of measuring overflow.
- 2026-11-25 Call sites: three service-account dialogs and force-release pass dismissOnBackdrop={false}; AccountDialog and Issue token pass busy.
- 2026-11-25 Tests added/updated; full vitest 330 passed. ci.sh red only at gazelle diff in internal/api/BUILD.bazel (other agent's display_names_test.go).
