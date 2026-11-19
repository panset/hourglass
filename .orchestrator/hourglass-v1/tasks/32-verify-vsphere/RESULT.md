# 32 verify vSphere extension, check-provider and the third-party entry point

Status: done. Clone at `/private/tmp/claude-501/-Users-tejaspanse-projects/17dec24c-e7ec-455e-9b87-01d2217d5fc9/scratchpad/verify-32/hourglass` (committed state). No product code was written.

## Verdict
PASS with minor findings. No blocker or high finding. Read-only guarantee holds by inspection; Layer 2 and 3 cover D7; my 4 extra suites (pressure, pools, simulator, secrets) all pass; 11 of 12 mutations turn tests red (the 12th is a near-equivalent mutant, finding 4); check-provider and the third-party entry point behave as specified. The findings are small deviations and gaps.

## Findings (severity, location, reproduction, fix)
1. LOW, `internal/app/checkprovider.go:121` (`store.OpenReadOnly`, `internal/store/store.go:113`). check-provider is not strictly "writes nothing" on a WAL database: after it runs, empty `hg.db-wal` and 32 KiB `hg.db-shm` appear beside the DB (db file hash and mtime unchanged, all 16 tables' row counts unchanged, integrity_check ok). Repro: `rm hg.db-shm hg.db-wal; hourglassd check-provider -config c.yaml; ls hg.db*`. Harmless on a normal directory; but if the database directory is read-only (chmod 555) it fails: `store: open .../hg.db read-only: attempt to write a readonly database (1544)`. Fix: document it, or open with `immutable=1` when the server is known stopped (unsafe if running); at least mention in the first-run checklist.
2. LOW, `providers/vsphere/vsphere.go` / `govmomi.go:160-170`. A typo in `datacenter` or `clusters` makes the provider list zero pools silently (no error), while the pressure source errors ("cluster not found" / "no clusters found"). Effect: every setup shows "expected but not reported" with no hint why. `TestEXT3_SimulatorScope` asserts the silent behaviour. Fix: have the provider also error on a configured cluster/datacenter that is not found (mirrors the pressure source), or have check-provider say so.
3. LOW, `providers/vsphere/vsphere.go:131-145`, `config.go:46`. Password redaction holds for every public path (config, provider, pressure source, connKey, client pointer, maps, JSON, all errors tested), but printing the unexported `provider` / `pressureSource` struct BY VALUE (`fmt.Sprintf("%+v", *p)`) or the adapter's value fields leaks the password, because fmt cannot call `secret.String` through an unexported field. Unreachable from outside the package (types are unexported, factories return pointers with String methods), so only a future in-package debug print could trigger it. Fix: give `config` a pointer-safe design (store the password in a struct with a `Format` method) or add a test that fails on `%+v` of values.
4. LOW (test gap), `providers/vsphere/vsphere.go:236` `clamp`. Mutation "remove the lower/NaN clamp" stays green: no test uses a negative ratio. Negative values are only reachable if vCenter reports a negative `reservationUsed`; NaN is unreachable (zero capacity errors first). Fix: add a case with negative `ReservationUsed` expecting 0.
5. LOW (test quality), `providers/vsphere/simulator_test.go:262-264`. The "no credentials in TLS error" assertion is `Contains("pass") && Contains("user:pass")`, which would not catch most leaks. My own check (`TestV_WrongPwAndTLSErrs`) confirms the real error is clean (`Post "https://127.0.0.1:PORT/sdk": tls: failed to verify certificate: x509: certificate signed by unknown authority`). Fix: assert the actual password value is absent.
6. INFO, `providers/vsphere/govmomi.go:196` vs `:240`. `ResourcePools` silently skips a cluster with no root pool; `Clusters` errors. Inconsistent but harmless (every real cluster has a root pool).
7. INFO, `providers/vsphere/vsphere.go`. Shared client sessions are never logged out at shutdown (no extension Close hook exists); the vCenter session lingers until its idle timeout. Acceptable; note for operators.
8. INFO, third-party main. A main importing only `pkg/hourglass`, `.../server` and its own pressure source fails at start with `unknown metering policy "default" (registered: none)` when `DefaultMeteringPolicy: "default"` is passed, because the default policy lives in the public package `policies/default`. Works once `_ ".../policies/default"` is added (no `internal` needed). The EXT-5 docs/worked example should show that import. Also SPEC EXT-5 mentions "a worked example" in the repository: none exists in the clone (`ls` shows no `examples/`); may belong to another task.
9. INFO, `hourglassd check-provider -h` prints "Usage of hourglassd:" and exits 1; cosmetic.
10. INFO. `bash tools/check_spdx.sh` passes; `sh tools/check_spdx.sh` fails with a syntax error (the script is bash; not a defect). Orphans in check-provider output list resources that are also under "matched" when the setup has no hold (matches HOLD-4: matching a setup with no active hold).

## 1. Test runs
- `go test -race -count=1 ./providers/vsphere/... ./internal/app/... ./pkg/...`: all ok (vsphere 31 top-level tests; 181 passing test/subtest lines across the four packages with tests). `go vet` clean.
- `bazel test //providers/vsphere/... //internal/app/... //pkg/...`: 4 of 4 pass (`//internal/app:app_test`, `//pkg/hourglass:hourglass_test`, `//pkg/hourglasstest:hourglasstest_test`, `//providers/vsphere:vsphere_test`); fresh-clone build including govmomi worked, no failures outside scope.
- `tools/check_core_generic.sh`: OK (108 core files); `tools/check_spdx.sh`: OK (with bash).

## 2. Read-only guarantee and privilege table (the privilege column is INFERENCE from the vSphere docs and the calls; no real vCenter was available)
No non-test file calls anything that creates, reconfigures, moves, powers or destroys (grep plus `TestEXT3_NoMutatingCalls`, an AST check on `govmomi.go` and `vsphere.go` that bans Create/Destroy/Delete/Remove/Rename/Reconfigure/Power/Move/... call prefixes; it is selector-prefix based so it would miss a mutating call through a function value, none exists). The only side effects are SessionManager Login/Logout.

| Adapter call (`providers/vsphere/govmomi.go`) | vSphere API | Privilege |
|---|---|---|
| `soap.NewClient`, `vim25.NewClient` | RetrieveServiceContent | none (anonymous) |
| `session.Manager.Login` / `Logout` | SessionManager.Login / Logout | none (any valid user) |
| `find.Finder.Datacenter` / `DatacenterList` | Folder childEntity traversal via PropertyCollector, SearchIndex path lookup | System.View on root folder (propagated) |
| `find.ClusterComputeResourceList("*")` | Folder childEntity of each datacenter's host folder | System.View |
| `RetrieveOne ClusterComputeResource {name, resourcePool, datastore}` | PropertyCollector.RetrieveProperties | System.Read on each in-scope cluster |
| `Retrieve ResourcePool {name, resourcePool}` (walk children) | PropertyCollector | System.Read on each pool |
| `RetrieveOne ResourcePool {runtime}` (root pool cpu/memory reservationUsed, maxUsage) | PropertyCollector | System.Read on the cluster root pool |
| `Retrieve Datastore {name, summary}` | PropertyCollector | System.Read on each datastore |
Conclusion: the built-in "Read-only" role (System.Anonymous, System.View, System.Read) granted at the vCenter root or datacenter with "propagate to children" should suffice. No Datastore.Browse, Resource.*, VirtualMachine.* or Global.* privilege is used. First-run checklist item 1 should still confirm that the `runtime` property of the root pool is readable with that role.

## 3. No real vCenter, no real credentials
- Ran the compiled vsphere test binary under `sandbox-exec` with `(deny network*)` except localhost (confirmed the sandbox blocks the outside: `curl example.com` fails to resolve), proxies unset, `HOURGLASS_VSPHERE_PASSWORD=sentinel-SECRET-xyz`: 31/31 pass, sentinel never appears in output. The only `os.Getenv` is `config.go:108`; tests that touch the variable use `t.Setenv` (`vsphere_test.go:332,379,388`), so the ambient value is overridden. Bazel's darwin sandbox run also passes. All connections are to the in-process simulator at 127.0.0.1.

## 4. Layer 2 vs D7
Every D7 item has a test (table): non-zero CPU and memory ratios `TestMET1_PressureNonZeroCPUAndMemory`; max-of-three each wins `..MaxOfThreeEachWins`; multi-cluster `..MultiClusterMax`; band crossing `..CrossesBands`; zero capacity `..ZeroCapacityIsError`; clamp `..Clamp`; inaccessible `..InaccessibleDatastores`; no clusters `..NoClustersIsError`; pools/root/nested/duplicates/scope `TestEXT3_Provider*`. My extra cases (`extra_vsphere_test.go.txt`, TestV_Pressure/TestV_Pools: 16 pressure cases and the scope/unknown-cluster/pool cases above, including exact 0.5/0.75/0.9, ratio>1, zero capacity cpu/mem, one zero cluster poisoning only when in scope, all datastores inaccessible, no datastores, a nested pool literally named "Resources" still listed) all PASS.

## 5. Layer 3 (my own TestV_Sim, TestV_WrongPwAndTLSErrs)
lab-01_RP, lab-02_RP and nested_RP listed with correct parent/path labels; destroying lab-01_RP removes it on the next List and the simulator (like vSphere) re-parents the nested pool to the root, which the provider reports correctly (parent "Resources"); pressure from the simulator is in (0,1] and the existing test asserts equality with the datastore ratio within 1e-9 (passes; mutations 7/8 turn it red); self-signed TLS is refused without `insecure_skip_verify` and accepted with it, for both extensions; after the session is logged out behind the adapter's back, the shared client logs in once more for both List and Pressure.

## 6. Secrets (TestV_Secrets)
Sentinel password never appears in `%v %+v %#v %s` of config, &config, connKey, `[]connKey`, provider, pressure source, the adapter client, a map holding them, the package `shared` map, or `json.Marshal(config)`; nor in errors for unreachable host (`Post "https://127.0.0.1:1/sdk": ... connection refused`), bad URLs (`https://%zz`, `ftp://x`, userinfo URL, junk), wrong JSON type for password, unknown field, wrong password against the simulator, TLS failure. Exception: finding 3. Through `hourglassd check-provider` with vsphere config (password in file, or via the env var) the sentinel appeared 0 times in output.

## 7. Genericity
`govmomi` appears only under `providers/vsphere/` plus `go.mod`, `go.sum`, `MODULE.bazel` and the check script. No core package mentions vSphere; the only references outside the package are the blank import in `cmd/hourglassd/main.go:28` and its BUILD file (composition root, expected), docs, and the wireframe HTML. `internal/app/checkprovider.go` is generic (registry lookups, `collector.Reconcile`, no concrete names).

## 8. check-provider (built `CGO_ENABLED=0`, static provider, temp dir)
- No database + seed: prints `setups: 2 (no database yet; seed file ...)`, `provider: static`, matched (1), expected but not reported (1), orphans (1), ignored allocations (2), pressure source, `raw pressure: 0.0000  multiplier: 0x (current bands, unsmoothed)`; exit 0; no file created. No database and no seed: `setups: 0 (no database and no seed file)`, exit 0, no `none.db` created.
- With database (created by a real server run): same sections, `(database ...)`; db sha256, size and mtime identical before/after; every table's row count identical; a live server running at the same time also works. Side files: finding 1.
- Errors, all exit 1: unknown provider `provider: hourglass: unknown provider "nope" (registered: static, vsphere)`; unknown pressure source lists `inventory, vsphere`; unknown YAML key; missing config file; missing seed; missing allocations file; vsphere unreachable; vsphere URL with credentials.

## 9. Third-party entry point (EXT-5)
Throwaway module `.../verify-32/third` (`thirdparty_main.go.txt` here) with `replace github.com/panset/hourglass => ../hourglass`; imports only `pkg/hourglass`, `pkg/hourglass/server`, its toy pressure source (plus `policies/default`, see finding 8). `CGO_ENABLED=0 go build` OK (21 MB). Started: `/healthz` returned `{"status":"ok","database":"ok",...}` 200; SIGINT gave exit 0 and "stopped". Removing the server import: builds, and `Run` returns `hourglass: the server implementation is not linked into this binary: add the import _ "github.com/panset/hourglass/pkg/hourglass/server" to your main package`. `import _ ".../internal/app"` from the throwaway module is refused (`use of internal package ... not allowed`) and the main does not need it. (`go list -deps` shows `internal/*` linked transitively, which Go allows.)

## 10. Mutation spot-checks in `providers/vsphere` (each reverted; files confirmed identical afterwards)
RED: min instead of max across cpu/memory (8 tests); min across clusters (10); include root pool (6); no upper clamp (1); ignore cluster scope (4); count inaccessible datastores (1); zero capacity not an error (3); storage min (9); storage ratio inverted (9); adapter drops child pool references (4); no re-login (1); unknown cluster not an error (1). GREEN: removing only the lower/NaN clamp (finding 4).

## Decisions for docs/decisions.md
None new.

## Confidence and gaps
Privilege table is inference (no vCenter). Hold/`[held]` display and multiplier-with-pressure in check-provider were not exercised with real holds. I did not test behaviour on an older-schema database or a database written by a different binary version. Layer 4 (compose simulator, D8) is out of scope.

## Files
Written: this RESULT.md, PROGRESS.md, `extra_vsphere_test.go.txt`, `thirdparty_main.go.txt` in this folder; everything else only in the scratch clone.
