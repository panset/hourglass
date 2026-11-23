# Progress, task 78
- 2026-11-23 Read brief, intent, contract, design A4/A5/UI-8.
- 2026-11-23 holdApi.ts: LedgerEntry -> LedgerViewEntry; getMyLedgerPage (period, rollup=day, limit=20, cursor).
- 2026-11-23 usage/index.tsx: view switch, Newer/Older with client-side cursor stack, reset on month/view change, poll and release reload use current page, poll-count detail.
- 2026-11-23 usage.test.tsx: 10 UI-8 tests; month test query string updated to the new request.
- 2026-11-23 Deliberate break (month change no longer drops cursors) turned the month test red; restored.
- 2026-11-23 acceptance.spec.ts readLedger: Every poll + Older walk; tsc clean.
- 2026-11-23 docs/ui-notes.md updated. ./ci.sh exit 0 (ci.exit).
