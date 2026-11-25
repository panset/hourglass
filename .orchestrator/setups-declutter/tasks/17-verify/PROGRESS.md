# Progress 17 verify

- 2026-11-24 read brief, INTENT, DECISIONS, DESIGN-R3, results 05/10/13/14/15. Tree snapshot saved (diffstat.before.txt, treehash.before.txt). Next: ci.sh, vitest, go test.
- 2026-11-24 ci.sh exit 0 (0 of 40 targets executed, all cached); vitest 19 files 300/300; go test ./internal/api 200 top-level pass.
- 2026-11-24 private copy ui-copy/ + builds dist-new, dist-main made in task folder. 35 deliberate breaks run in ui-copy only: 33 red, 2 CSS-only survive (browser checks cover them). Copy restored, identical to tree.
- 2026-11-24 page-by-page table regression (step 4) handed to a helper agent writing only under pages/. Next: break list 1-19 in Chromium (browser/).
- 2026-11-24 browser break list done: r3a 113 checks (1 fail = should-fix 1), r3b 67 checks (all pass). Helper report on table pages in (no difference outside Setups/Settings).
- 2026-11-24 steps 5-7 done (API diff, dead code, docs vs build, generic, UI-1). Tree hash and status same as at start.
- 2026-11-24 RESULT.md written. Verdict: ready after the listed fixes; 0 blocker, 2 should fix, 8 minor. Done.
