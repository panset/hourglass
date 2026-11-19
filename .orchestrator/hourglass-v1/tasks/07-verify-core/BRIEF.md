# 07 verify core logic (rules, metering, ledger)
Job intent: /Users/tejaspanse/projects/hourglass/.orchestrator/hourglass-v1/INTENT.md. Rules: CONTRACT.md beside it (reporting and slow-command sections; you write no product code).
## Goal
Independently check `internal/rules`, `internal/metering` and `internal/ledger` against docs/SPEC.md and their briefs (tasks/04-rules, 05-metering, 06-ledger BRIEF.md). You did not write them. Try to break them.
## Setup
`git clone /Users/tejaspanse/projects/hourglass <scratch>/verify-07/hourglass` where <scratch> = /private/tmp/claude-501/-Users-tejaspanse-projects/17dec24c-e7ec-455e-9b87-01d2217d5fc9/scratchpad (remove verify-07 first if present). Work only in the clone; other agents are changing the original.
## Checks
1. `./ci.sh` in the clone exits 0; report tests executed per package (`go test -v ./internal/... | grep -c '^=== RUN'` or similar). Zero is a failure.
2. Spec conformance, by reading code and by writing your own extra throwaway tests in the clone (do not copy them back): RULE-1, RULE-2 (each of the six selector forms, AND, whitespace, absent-key semantics of `!=` and `notin`, malformed input such as `a in (`, `=b`, `a=b,,c=d`, trailing comma, uppercase keys), RULE-5, SET-3 (32/33 labels, 63/64-char keys and values, key regex edges), burn-rate table boundaries (0.4999, 0.5, 0.75, 0.9, 1.0), smoothing window edges, MET-3 rounding, MET-4 (exactly 2x interval vs just over), the spec's worked examples (15 h at 0x/1x/3x = 0/15/45 tokens; -40 closing with 300 allowance opens at 260), LED-3 ordering and periods of entries, LED-5, month boundary in a non-UTC timezone and across DST and year end, `balance.low` / `balance.overdrawn` crossings firing only on first crossing.
3. Mutation spot-checks: in the clone make one deliberate bug per package (e.g. `>=` to `>` in the band lookup, drop the carryover entry, make `notin` false for absent keys), confirm the package's own tests go red, restore. A mutation that stays green is a finding.
4. Genericity: `grep -rniE 'vcenter|vsphere|govmomi|medium|large|webhook|keycloak'` over the three packages must find nothing outside neutral test data; none may import another Hourglass package or a non-stdlib module.
5. Every .go file starts with the SPDX line; test names cite requirement ids.
6. Read the "Decisions for docs/decisions.md" section of each of the three RESULT.md files. For each, say whether it is consistent with the spec. Write `decisions-rows.md` in your task folder: one Markdown table row per decision worth recording, in the form `| 2026-11-19 | <decision> | <why> | agent |`, at most 12 rows, flagging with **REVIEW** only those the owner would plausibly disagree with.
## Scope
May write: the clone, and /Users/tejaspanse/projects/hourglass/.orchestrator/hourglass-v1/tasks/07-verify-core/.
## Done when
RESULT.md opens with a verdict (pass / pass with findings / fail) and a numbered findings list, each with severity (bug / gap in tests / nit), `path:line`, and a failing input. Status `done` whatever the verdict.
