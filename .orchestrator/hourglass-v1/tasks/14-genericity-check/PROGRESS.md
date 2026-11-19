# Progress: 14 core-genericity check
- Read BRIEF, INTENT, CONTRACT. Read tree layout (no providers/pressure/policies/hooks/examples/deploy/e2e dirs yet).
- Wrote tools/check_core_generic.sh (awk import parser, grep text scan, bash 3.2 safe). bash -n OK.
- Current tree: passes (29 core files, 23 Go files).
- Seeded one violation per rule under internal/zz_genericity_seed/ and zz_genericity_seed/, ran, removed; clean again.
- Added to ci.sh after check_spdx.sh. bash -n OK.
- Remaining: write RESULT.md.
