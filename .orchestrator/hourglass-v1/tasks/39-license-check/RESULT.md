# Result: 39 license check
Status: done.

## What was done
- `tools/check_licenses.sh [--write]` (bash wrapper plus embedded node; needs only bash, go, node). Runs in about 0.5 s.
  - Go: `go list -deps -test` over `./...` for GOOS linux and darwin (CGO off, union), then reads LICENSE/COPYING of each module from the module cache and classifies by licence text. Copyleft text (GPL/LGPL/AGPL/MPL/SSPL/BUSL/EPL/CDDL) or unrecognised text fails closed, naming the module.
  - npm: parses `ui/pnpm-lock.yaml` (all 260 packages, prod and dev) and reads each package's `license` field from `ui/node_modules/.pnpm`. The lockfile holds no licence metadata, so without node_modules the script fails and says to run `pnpm install --frozen-lockfile` in `ui/`. `e2e/browser` is covered the same way if it appears (it does not exist now).
  - SPDX expressions: OR passes if any alternative is allowed, AND needs all.
  - `tools/licenses_allow.txt`: `id|name|justification` entries, scoped to one module; an entry that no longer occurs fails the check.
  - `docs/third-party-licenses.md` is generated with `--write`; the check fails if it differs.
- `ci.sh`: one line `tools/check_licenses.sh` after `check_core_generic.sh`.

## Evidence
- Passes on the current tree: `license check ok: 16 Go modules, 260 npm packages`.
- Seeded failure (temporary, restored): react licence set to GPL-3.0-only gave `npm react@18.3.1 (ui): license "GPL-3.0-only" is not permitted`, exit 1; a GPL text in github.com/google/uuid's LICENSE gave `go github.com/google/uuid@v1.6.0: LICENSE contains GPL license text`, exit 1. Both files restored byte-for-byte, check passes again.

## Licence mix
- Go (16): BSD-3-Clause 9, MIT 3, Apache-2.0 3, "Apache-2.0 AND MIT" 1 (gopkg.in/yaml.v3); per-module list in `docs/third-party-licenses.md`.
- npm (260): MIT 235, ISC 10, Apache-2.0 7, BSD-2-Clause 2, BSD-3-Clause 2, MIT-0 1, Python-2.0 1, CC-BY-4.0 1, (MIT OR CC0-1.0) 1 (type-fest, passes via MIT). 66 platform/optional packages not installed here take a sibling's licence (see decisions).
- No MPL, LGPL, GPL, AGPL, SSPL, BUSL, unknown or missing licences found.

## Open questions for the owner
1. `caniuse-lite` 1.0.30001826 is CC-BY-4.0 (attribution data licence, not on the brief's list). It is dev-only (pulled by browserslist via @vitejs/plugin-react / @babel/core, and vite) and not in the bundle. Not removed; I allowlisted it as PROVISIONAL in `tools/licenses_allow.txt`. Decide: accept, or drop the entry so CI fails.
2. `@csstools/color-helpers` is MIT-0 (not on the brief's list; equivalent to 0BSD). Allowlisted. Path: jsdom -> @asamuzakjp/css-color -> @csstools/css-color-parser -> @csstools/color-helpers (test only).
3. `argparse` is Python-2.0 (on the brief's list). Allowlisted. Path: js-yaml dependency chain in the UI toolchain (appears to be via openapi-typescript).

## Decisions for docs/decisions.md
- Allowlist is name-scoped, and stale entries fail.
- Platform-specific optional npm packages not installed on this OS (os/cpu in the lockfile, e.g. @esbuild/*, @rollup/*) take the licence of an installed sibling in the same scope; no sibling means fail. The inventory stays the same on macOS and Linux.
- Go secondary licence files (LICENSE-3RD-PARTY.md in modernc.org/*) are skipped: they list modules that are not built (e.g. an MPL-2.0 golang-lru, absent from the build list). Other secondary files (PATENTS, LICENSE-SQLITE, LICENSE-GO) are only checked for copyleft text; primary LICENSE files are classified strictly.

## Regeneration
Other agents are changing go.mod (kin-openapi) and MODULE.bazel (rules_oci). Once they finish, run `tools/check_licenses.sh --write` and commit `docs/third-party-licenses.md`; the new Go module licences must pass the policy, and the module cache needs `go mod download`. MODULE.bazel (Bazel modules) is not inspected; only Go and npm are covered, as briefed.

## Files changed
tools/check_licenses.sh, tools/licenses_allow.txt, ci.sh, docs/third-party-licenses.md, this folder.

## Confidence and gaps
- Verified: script runs, passes, seeded failures. Appears to: libraries' licence file text matching is heuristic; unusual wording fails closed (manual allowlist needed).
- Not verified on Linux. Go test-only packages with build tags other than linux/darwin are not seen.
- Dependencies of Bazel-only rules (rules_oci, etc.) are not covered.
