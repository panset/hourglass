# 43 re-verify the balance cache after fix 42b
Job intent: /Users/tejaspanse/projects/hourglass/.orchestrator/hourglass-v1/INTENT.md. Rules: CONTRACT.md beside it. You write no product code. If `.md` files cannot be written, use `.txt` and put the findings in your reply.
## Goal
Verification 42 found that the balance cache could disagree with the ledger (tasks/42-verify-final-fixes/RESULT.md, with tests in `evidence/`). Task 42b fixed it (tasks/42b-fix-cache/RESULT.md). Confirm independently that the cache now always equals the current period's ledger sum, and that the fix introduced nothing new. You did not write either.
## Setup
`git clone /Users/tejaspanse/projects/hourglass <scratch>/verify-43/hourglass`, <scratch> = /private/tmp/claude-501/-Users-tejaspanse-projects/17dec24c-e7ec-455e-9b87-01d2217d5fc9/scratchpad (remove verify-43 first). Work only in the clone. Do not start Docker.
## Checks
1. Copy verification 42's ORIGINAL evidence tests (from tasks/42-verify-final-fixes/evidence/, not the adapted copies in the repo) into the clone and run them: every cache scenario must pass, including the multi-month rollover with a current-month entry and both timezone moves. Report each test's result.
2. Write new scenarios the earlier rounds did not: (a) randomised: 200 seeded-random sequences of 60 operations each drawn from {poll with debits, grant, negative grant, reset, role removed, role restored, clock advance of minutes/days/1-3 months, process "restart" (new service objects over the same database), failed poll}; after every operation assert, for every user with a role, cache == ledger sum of the current period, and that the sum of all ledger amounts per user and period is unchanged by restarts; print the seed of any failure. (b) the rebuild-on-period-change path added in 42b: does it run inside a transaction with the writes it sits next to; can a request observe a half-rebuilt state; can two goroutines trigger it at once; what does it cost per poll at spec scale (it must not reintroduce the performance problem: run the full-size scale test, see CONTRIBUTING.md). (c) startup rebuild with 200 users: time and correctness; with an empty database; with a database from before migration 0003.
3. Acquire gating uses the cache: after each scenario in 1 and 2(a), a user whose ledger sum is above zero can acquire and one at or below zero cannot (enforcement on).
4. `go test -race -count=1 ./...` and `./ci.sh` exit 0.
5. Mutation checks: revert each of the three 42b mechanisms in turn (current-period argument in the rollover; rebuild at end of rollover; startup rebuild) and report which repository tests go red for each. A mechanism whose removal leaves everything green is a finding only if a scenario exists where it alone protects correctness; say which.
## Scope
May write: the clone and /Users/tejaspanse/projects/hourglass/.orchestrator/hourglass-v1/tasks/43-verify-cache-fix/.
## Done when
The result file opens with a verdict (cache correct: yes/no) and a numbered findings list with severity, seed or reproduction, and suggested fix.
