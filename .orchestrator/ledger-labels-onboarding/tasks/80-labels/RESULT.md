# RESULT 80-labels: status done

## What was done
- B1: `rules.LabelTerm(key, value) (string, bool)` in `internal/rules/selector.go`, next to the parser. It returns `key=value` (`key=` for empty), and no term when the value has whitespace or one of `, ( ) =  !`, exceeds 63 characters, or the key is not a valid label key. The term is checked against the real parser before it is returned.
- `GET /api/v1/setups` gains `label_index` (`internal/api/handlers_user.go`, `labelIndexBuilder`): built from every row the caller may see before the selector filter runs; keys and values sorted and distinct; `[]` when empty; null `selector` when `LabelTerm` has no term. No route or permission changed.
- `api/openapi.yaml`: `SetupList.label_index` (required), schemas `LabelIndexKey`, `LabelIndexValue`. `ui/src/api/schema.gen.ts` regenerated with `npm run gen:api`.
- B2: `ui/src/pages/setups/LabelChips.tsx` (block, chips, "+N more", collapse), `labelTerms.ts` (split on commas outside parentheses, whole-term toggle/replace/append), wiring and placeholder in `index.tsx`, styles in `setups.module.css`. Numbers 12 values and 8 keys are `VALUES_SHOWN` and `KEYS_BEFORE_COLLAPSE`.
- `docs/ui-notes.md` Setups section describes the block.

## Requirement -> test -> file
| Requirement | Test | File |
| --- | --- | --- |
| SET-8 sorted, distinct, `[]` | TestSET8_LabelIndexIsSortedAndDistinct, TestSET8_LabelIndexEmptyIsEmptyArray | internal/api/setups_label_index_test.go |
| SET-8 ignores `selector` | TestSET8_LabelIndexIgnoresSelector | same |
| SET-8 developer vs admin, disabled-only labels | TestSET8_DeveloperIndexHidesDisabledOnlyLabels | same |
| SET-8 every non-null term parses and matches exactly its setups (odd legal characters, empty value, dotted key); unwritable values null | TestSET8_LabelIndexTermsMatchExactlyTheirSetups, TestSET8_LabelTerm | same; internal/rules/rules_test.go |
| UPG-3 contract (schema vs handler) | existing contract test, now covers `label_index` | internal/api/contract_test.go |
| UI-9 chips for every key/value, names | "lists every key and every value..." | ui/src/pages/setups/setups.test.tsx |
| UI-9 append / replace / toggle off | "appends...", "replaces...", "removes it (toggle)..." | same |
| UI-9 whole-term comparison, parentheses | "terms are compared whole..." | same |
| UI-9 selected state (applied only) | "is selected; typing without applying..." | same |
| UI-9 null-term chip inert, explained | "a value with no term cannot be activated..." | same |
| UI-9 placeholder, omitted when empty | "placeholder is the first non-null term", "empty index..." | same |
| UI-9 "+N more", 12 boundary | "first 12 values...", "exactly 12..." | same |
| UI-9 collapse above 8 keys | "more than 8 keys...collapsed" | same |
| UI-9 accessible name, keyboard | "reachable with Tab and activated with Enter and Space" | same |

## Files changed
internal/rules/selector.go, internal/rules/rules_test.go, internal/api/handlers_user.go, internal/api/setups_label_index_test.go (new), internal/api/user_test.go (exact field list of the setups response gains `label_index`; not weakened), api/openapi.yaml, ui/src/api/schema.gen.ts (generated), ui/src/pages/setups/{index.tsx,LabelChips.tsx (new),labelTerms.ts (new),setups.module.css,setups.test.tsx}, ui/src/pages/admin-shared/fixtures.ts (shared fixture: `label_index: []` so the type-check passes), docs/ui-notes.md, this folder. No BUILD.bazel change was needed (gazelle ran clean, new test file picked up).

## Commands and outcome
- `go test ./internal/rules ./internal/api`: pass. `cd ui && npx vitest run`: 197 pass; `tsc --noEmit`: clean.
- Deliberate breaks, both restored: SET-8 (index built after the selector filter) failed TestSET8_LabelIndexIgnoresSelector; UI-9 (replace rule disabled) failed the replace and whole-term tests.
- `./ci.sh` (log `ci.log`, exit code in `ci.exit`): exit 0, 39 of 39 tests pass.

## Decisions for docs/decisions.md
1. `rules.LabelTerm` returns no term for an invalid label key as well as for unwritable values (keys are validated at storage, so this is defensive), and for values over 63 characters.
2. Chips join terms with "," (no space); the chip's "same key" rule recognises a plain `key=value` or `key==value` term, not `!=`, `in`, `notin`, or bare key. If several same-key equality terms are in the box, activation replaces the first and drops the rest.
3. Unusable (null-term) chips use `aria-disabled` rather than `disabled`, so they stay focusable and their explanation (title, aria-label, note below the block) is reachable by keyboard. An empty value shows as "(empty)".
4. Selected state follows the applied selector (what the server filtered by), toggle logic follows the box text.
5. Block placed directly under the filter row; heading "Available labels", with "Show labels"/"Hide labels" and "+N more"/"Show fewer".

## Confidence and gaps
- The Go/Vitest checks and ci.sh ran here; the chips were not viewed in a browser, so layout and the CSS look are inferred from tokens, not seen. No e2e step was added (out of scope).
- tools/check_core_generic.sh ran inside ci.sh and passed; label examples use site and tier.
