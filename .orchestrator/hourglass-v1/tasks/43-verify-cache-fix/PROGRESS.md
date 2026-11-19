# PROGRESS 43

- 2026-11-19 Read the brief, INTENT, CONTRACT, the 42 RESULT and the 42b RESULT. Cloned the repo at 326cc7d into scratch/verify-43/hourglass.
- 2026-11-19 Check 1: ran the original v42 evidence tests (adopted copy moved aside because helper names collide). 12 of 14 cases pass. RollbackLeavesCache fails on its D-A window check (pre-existing). Timezone backwards fails because the harness restart skips the start-up rebuild; it passes once the restart calls it.
- 2026-11-19 Check 2a: wrote the seeded random test. The first classifier was too narrow (it only accepted the last-opened period in the window); widened it to any period between last-opened and current. 200 seeds × 60 ops, 0 failures.
- 2026-11-19 Full-size scale test: pass.
- 2026-11-19 Check 2b/2c: startup rebuild with 200 users, an empty DB, and a pre-0003 DB built with 99408df in a git worktree; concurrency tests. Found that concurrent RolloverDue double-writes the carry-over (pre-existing; serialised in the product).
- 2026-11-19 Check 4: go test -race exit 0; ci.sh exit 0 (pristine clone, my files removed).
- 2026-11-19 Check 5: mutations M1-M3 plus M4 in a second clone; all reverted.
- 2026-11-19 Wrote RESULT.md and evidence/. Done.
