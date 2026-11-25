# 06 (B2) Shared UI pieces
Common rules: `.orchestrator/setups-declutter/tasks/COMMON.md` (read it first; it names the other files to read).
## Goal
Implement task **B2** of DESIGN.md section 7: the shared component and formatting changes the Setups page will build on (table layout mechanism where label columns give way and other columns stay on one line, Q-A = single-line clipped labels; the Modal/panel changes including focus return with a fallback when the opener is gone; format helpers), exactly as DESIGN.md sections 3, 4 and 6 ("UI") specify for these files.
## Scope
May write: `ui/src/components/Table.tsx`, `ui/src/components/Table.module.css`, `ui/src/components/Modal.tsx`, `ui/src/components/components.test.tsx`, `ui/src/lib/format.ts`, `ui/src/lib/format.test.ts`. Must not touch `ui/src/pages/**` or anything else. Every other page that uses `Table` and `Modal` must look and behave as before unless it opts in: new behaviour is behind props/classes that default to today's behaviour.
## Done when
- The unit tests DESIGN.md names for these files exist and pass; the existing tests for these components and for `format` pass.
- The UI type check and lint pass for your files. (Another agent is regenerating `ui/src/api/schema.gen.ts` and a third is not yet on the Setups page: a failure in `ui/src/pages/**` or the generated types is not yours; note it.)
## Long-running steps
UI unit tests: well under a minute. Do not run `./ci.sh` or e2e.
## Report
Per COMMON.md. In RESULT.md list the exact props/classes/functions you added and their signatures, for the Setups-page builder.
