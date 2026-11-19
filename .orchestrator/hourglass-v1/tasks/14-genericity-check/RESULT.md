# Result: 14 core-genericity check

Status: done.

## What was done
- Added `tools/check_core_generic.sh`. Core = `internal/`, `pkg/hourglass/`, `pkg/hourglasstest/`.
  - rule1: core `.go` files (tests included) must not import `github.com/panset/hourglass/{providers,pressure,policies,hooks,examples}` or any path containing `vmware/govmomi`. Imports are parsed in-script with awk (import blocks and single imports, `//` lines skipped). `go list` is not used.
  - rule2: core files of any type (grep -I, so binary files are skipped) must not match, case-insensitively: `vcenter|vsphere|govmomi|vcsim|resource pool|resourcepool|extralarge|size=|keycloak`.
  - rule3: `vmware/govmomi` imports outside core are allowed only under `providers/vsphere/`, `deploy/`, `e2e/`.
  - Output `path:line: ruleN: text`, exit 1 on violation; otherwise a one-line OK summary.
  - Missing extension directories are handled (find only runs on existing dirs).
- Added `tools/check_core_generic.sh` to `ci.sh` directly after `tools/check_spdx.sh`.
- Did not run Bazel or `go build`; ran the script and `bash -n` only.

## Evidence
Current tree (`tools/check_core_generic.sh`, exit 0):
`core-genericity: OK (29 core files, 23 Go files checked, no violations)`

Seeded violations (temporary files under `internal/zz_genericity_seed/` and `zz_genericity_seed/`), exit 1:
```
internal/zz_genericity_seed/rule1_seed.go:5: rule1: import "github.com/panset/hourglass/providers/vsphere"
zz_genericity_seed/rule3_seed.go:4: rule3: import "github.com/vmware/govmomi/vim25"
internal/zz_genericity_seed/rule1_seed.go:5: rule2: 	"github.com/panset/hourglass/providers/vsphere"
internal/zz_genericity_seed/rule1_seed.go:9: rule2: var _ = vsphere.X
internal/zz_genericity_seed/rule2_seed.md:1: rule2: Config note: the vCenter size=large default lives here.
core-genericity: 5 violation(s) found
```
The seed directories were then removed, and the script returned to exit 0.
Note: the rule1 seed also trips rule2, because its import path contains "vsphere". That is expected from the rule text.

## Files changed
- `tools/check_core_generic.sh` (new, executable)
- `ci.sh` (one line added)
- `.orchestrator/hourglass-v1/tasks/14-genericity-check/PROGRESS.md`, `RESULT.md`

## Decisions for docs/decisions.md
- Rule 3 allows all of `deploy/` and `e2e/`, not only test files, because the brief says "test-only tooling" but names no file pattern. A non-test helper there is not flagged.
- A core file importing govmomi is reported under rule1 only; rule3 skips core files to avoid duplicate reports.

## Confidence and gaps
- Verified: the script passes on the current tree, and each of the three rules fires on a seeded file. Syntax checked with `bash -n` under bash 3.2.57 on darwin.
- Not verified: Linux behaviour (BSD/GNU awk and grep differences), since only macOS was run. The awk uses only `match`, `RSTART`, `RLENGTH`, `FNR` and `printf`, which are portable.
- Import detection is line-based. It will miss imports written in unusual layouts, such as a block comment containing `import (`. Rule 1 does not follow transitive imports (the brief said direct imports are enough).
- Rule 2 does not scan binary files.
- Not run: `ci.sh` end to end (it needs Bazel, which the brief excludes).
