# Progress: 50 remove resource-name template and override
- 2026-11-20 Read brief, INTENT, CONTRACT, D40. Grepped every use.
- 2026-11-20 Core edits done: settings (template + sandbox removed), store (migration 0004 drops column, deletes stored key; upgrade test), pkg/hourglass (SetupState.ResourceName removed, AllocationFor by setup name), collector/holds/api/app (match by exact setup name), seed rejects resource_name_override, check-provider prints setup names.
- 2026-11-20 vsphere pool_name_prefix/suffix + layer 2/3 tests; pressure source accepts and ignores the keys (shared parser; simpler than per-kind parsing). Sample gives the suffix only in the provider block.
- 2026-11-20 UI (settings field, setup form override, inventory column, schema regen), e2e A0, docs, examples, check_core_generic rule4.
- Next: go test -race, ci.sh, scale test, upgrade test from the previous commit's database, RESULT.md.
- 2026-11-20 go test -race exit 0; upgrade test with old/new binaries passed; RESULT.md drafted. Next: ci.sh result, then scale test.
- 2026-11-20 ci.sh exit 0; full-size scale test PASS. Done.
