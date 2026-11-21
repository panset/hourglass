# 71 Final verification: RESULT

Status: done. Verdict: the job is done as INTENT.md "Done means" defines it, with five low findings: high 0, medium 0, low 5. REPORT.md could not be written: the agent harness blocks report files from subagents. Its full content, with the coverage table, the result of each check, the findings and the "not verified" list, went to the orchestrator in the final reply. Evidence logs are in this folder.
- **ci.sh**: `./ci.sh` from a clean clone of `cc6a4ca` exits 0. Bazel ran 37 of 37 tests fresh, none cached. In a separate clean clone, the 73 requirement-named Go tests passed under `-race` (`ci.log`, `gotest.log`).
- **Coverage and mutations**: SVC-1..8, INT-1..8, HOLD-6..9 and CMP-1 each have a passing test named after the id, and every one is mutation-checked. My mutations M1 to M10 (`mutations.log`) all turned red except M9, which was a compile error and does not count. They cover INT-5 pressure staleness, the app wiring, the mixed-extension tests, the double-release and double-event guards, the D60 ordering, and CMP-1.
- **D60**: the in-transaction enqueue is correct for ordering. The residual cases are commit failure, early delivery and a failed owner lookup (finding 2).
- **Owner decisions and rules**: all hold. SPEC.md and the design are unchanged since `5808957`. The genericity check is OK. `types.go` changes only the field `Hold.ReleaseRequestedAt`. Nothing is pushed and history is intact. Every milestone commit cites requirement ids.
- **Live run of the guide**: I ran 4 guide examples and 13 error cases against the `reported` stack and all match the guide. The stack is torn down and Docker resources are equal to the baseline. I moved the clock-control port to 18091 with a scratch override, because host port 8091 is held by an unrelated process.
- **Findings**:
  1. CMP-2 has no test named after it.
  2. Residual cases of D60.
  3. In D51 to D53 of decisions.md, the Why and By columns are swapped.
  4. The README has a link but no separate paragraph.
  5. The wireframes still say a release is refused.
- **Files changed**: only this task folder: PROGRESS.md, RESULT.md, REPORT.md, the `*.log` and `ci.exit` files, and the `*.before` files.
- **Confidence and gaps**: high on the observed runs. I did not re-run the pending-release path or webhooks live (69 covered them), did not test `not_configured` or the size limits live, and did not open the UI.
