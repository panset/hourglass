# 80 Label discovery: `label_index` and chips on Setups

Job intent: `.orchestrator/ledger-labels-onboarding/INTENT.md` (read it first).
Agent contract: `.orchestrator/hourglass-v1/CONTRACT.md` (read it second; "the
spec" means `docs/design/ledger-labels-onboarding.md`). Your task folder is
`.orchestrator/ledger-labels-onboarding/tasks/80-labels/`.

## Goal

The labels half of milestone 6: section B of the design (B1 API, B2 UI), with
tests named after SET-8 and UI-9 that fail when the requirement is broken.

## Context

Read the design doc sections "What the code does today" (Setups), B1, B2,
Compatibility, Requirements (SET-8, UI-9), and the note under "Changes by area"
about words banned by `tools/check_core_generic.sh` (`size=` in particular: use
`site` or `tier` in label examples). Then `docs/ui-notes.md` and CONTRIBUTING.md
(how the generated UI types are regenerated; never hand-edit them).

## What to build

- B1: the selector term for a label value in `internal/rules`, next to the
  parser (null/absent when the value cannot be written; `key=` for an empty
  value); `label_index` on `GET /api/v1/setups` (sorted keys, sorted distinct
  values, over every setup the caller may see, ignoring the request's
  `selector`, `[]` when no labels); `api/openapi.yaml`, contract test,
  generated types. No route added, no permission changed.
- B2: every bullet. The two numbers (12 values, 8 keys) stay as designed.
- Tests named after SET-8: sorted; ignores `selector`; a developer's index
  holds no label that appears only on a disabled setup while an admin's does;
  every non-null term parses with the real parser and matches exactly the
  setups with that value (include values with odd but legal characters, an
  empty value, and keys that also need care if the grammar restricts keys:
  check what the parser accepts for keys and handle a key that cannot be
  written); a value with whitespace or one of `, ( ) = !` has a null term.
- Tests named after UI-9: chips for every key and value; append, replace
  (same key, `key=value` term), toggle off; whole-term comparison with commas
  inside parentheses (`site in (a,b)` is one term and is not replaced by a
  `site=` chip unless the design says so: follow B2's three rules literally);
  selected state; a null-term chip cannot be activated and explains why;
  placeholder from the index and omitted when the index is empty; "+N more";
  collapsed with more than 8 keys; accessible name and keyboard activation.

## Scope

May write: `internal/rules/`, `internal/api/` (setups handler, its tests,
contract test), `api/openapi.yaml`, `ui/src/api/schema.gen.ts` (generated),
`ui/src/pages/setups/`, shared UI components if needed (list them),
`docs/ui-notes.md`, `BUILD.bazel` files Gazelle updates, your task folder.
Must not touch: `docs/decisions.md`, the design doc, `e2e/`, the ledger code,
git state. Do not weaken any existing test.

## Done when

`go test ./internal/...`, UI tests and type-check pass; `./ci.sh` exits 0
(record `ci.exit`); one deliberate break each for SET-8 and UI-9 went red and
was restored; RESULT.md has the requirement -> test -> file table.

## Long-running steps

`./ci.sh`: allow 10 minutes (timeout 600000), output to `ci.log` in your task
folder, read the exit code and tail only.

## Report

Follow CONTRACT.md: PROGRESS.md, RESULT.md with "Decisions for
docs/decisions.md", one final reply of at most ten lines.
