# Result 05 (B1): additive `pricing` on GET /api/v1/setups

Status: **done**.

## What was done
- Server: `pricingJSON` and `(*lab).pricing()` (`internal/api/handlers_user.go:893-909`); `"pricing": l.pricing()` in the `listSetups` response (`handlers_user.go:889`). Values come from the same `lab` (one `loadLab` per request) that `l.cost` uses: `multiplier` = `l.mult`, `peak_multiplier` = `l.peakMult`, `pressure` = `l.sample.Smoothed` or null when `l.sample` is nil. No extra store read.
- OpenAPI: `SetupList.pricing` added and required (`api/openapi.yaml:2053`, `2085-2104`); description says it is new in v1.3.0 and clients talking to older servers should treat it as optional. `listSetups` description updated (`openapi.yaml:~421`).
- Generated types: `ui/src/api/schema.gen.ts` regenerated with `npm run gen:api` (not by hand).
- Fixtures: `pricing: { multiplier: 2, peak_multiplier: 3, pressure: 0.8 }` in `ui/src/pages/admin-shared/fixtures.ts:168` and in the `list()` fixture of `ui/src/pages/setups/setups.test.tsx:80` (only that line). Values match the fixtures' costs (weight 2 -> now 4, at_peak 6).
- Contract/compat: pinned key list (`internal/api/user_test.go:380`), field stripped before comparing to the release (`upgrade_compat_test.go:195`), `addedSetupsList` gains `"pricing"` (`upgrade_v110_compat_test.go:297-298`; `addedSetupsAdmin` inherits it). `contract_test.go` unchanged; it validates `pricing` against the closed schema.
- New test `internal/api/setups_pricing_test.go`: `TestUI18_SetupsListCarriesTheMultipliersOfItsCosts` with the five DESIGN.md cases as subtests (lines 96, 123, 142, 159, 165): before the first poll (pressure null, /pressure updated_at null, multiplier = bands' value at 0, here 0.5); after a poll pricing equals /pressure multiplier and smoothed at the same fake time; every setup's now/at_peak = `tokens(HourlyCostMT(weight, mult|peak))` exactly, with weights 0, 1.15, 1.23456789 (reported) and a policy-priced hold (resolver weight 2, policy weight 4); developer and admin get identical `pricing`; after lowering the top band before the next poll, multiplier stays 2.5, peak becomes 1.25, `now > at_peak`, and the cost identity still holds. Added to `internal/api/BUILD.bazel` srcs (explicit list, gazelle-style).
- Docs: `docs/deploying.md` new "Upgrading to v1.3.0 (the Setups page leads with cost)" subsection (lines ~635-640); the `display_label_keys` settings row (line 185) now says "shown as columns" and that the panel and chips cover every label.

## Commands and outcomes
- `go test ./internal/api -run 'TestUI18_|TestUI12_|TestWGT14_' -v`: 3 pass.
- `bazel test //internal/api/...`: exit 0, 2 test targets pass (`bazel.log`).
- `go test ./internal/api -count=1 -v`: 200 top-level tests pass, 0 fail.
- `npm run typecheck` in `ui/`: exit 0 (`tsc.log`).
- `bazel test //ui:api_schema_drift_test`: pass (`drift.log`).
- Not run: `./ci.sh`, e2e, vitest (per brief).

## Deviations from DESIGN.md
- `openapi.yaml` listSetups description: DESIGN said "only the state filter is applied client-side". Under owner decision Q-B = B2 a chip with no selector term also filters in the browser, so the text says the page applies its state filter and the exact match of a label value with no selector term client-side.
- Test uses its own bands (base 0.5, multipliers 1.25/2.5/3.75) so the pre-poll multiplier is non-zero and the multipliers have decimals; DESIGN did not fix values.

## Confidence and gaps
- High for the API, tests and types: all ran green.
- The deploying.md note says the new UI reads `pricing` as optional; that is B3's job (inference from DESIGN M1), not verified here.
- `docs/deploying.md:185` wording mentions the details panel, which B3 builds.
