# Result: 13 static provider, webhook hook, Event JSON

Status: done.

## Event JSON (pkg/hourglass/event_json.go)
`Event.MarshalJSON` / `(*Event).UnmarshalJSON`; no existing declaration or tag changed (private mirror structs).
```
{"type": "...", "time": "RFC3339 UTC",
 "user":  {"id","username","email","name","roles"} | null,
 "setup": {"name","labels","details"} | null,      // details passed through unmodified
 "hold":  {"id","setup","user_id","acquired_at","metering_started_at","deployed_at","released_at"} | null,
 "data":  {}}                                      // always an object
```
Field mapping: Hold.SetupName -> `setup` (the struct has a name, not an id). Unset nullable hold times are null. Nil labels/roles/details encode as null.
Round trip: absent/null user, setup, hold -> nil; `{}` data -> nil map; null details -> nil. Data numbers decode as float64 (standard encoding/json).
Tests: TestEXT6_EventJSONRoundTrip, TestEXT6_EventJSONShape.

## static provider (providers/static), name `static`
Config `{"path": "..."}`; path required; unknown fields, malformed JSON, empty config rejected. File is a JSON array of `{"resource_name","labels"}`, re-read on every List. Missing/malformed file, non-array, empty or missing resource_name -> error; `[]` valid (returns empty, non-nil slice). Context checked before and after the read. Duplicate resource names are allowed (spec silent).

## webhook hook (hooks/webhook), name `webhook`
Config `{"url","secret","types"}`; url required, absolute http/https with host; types default all, `[]` is treated as "none" (nil means all) , unknown types rejected listing known ones. POST body = Event JSON, headers Content-Type, X-Hourglass-Event, and X-Hourglass-Signature `sha256=<hex hmac>` when secret set. Non-2xx (including 3xx; redirects not followed) or transport error -> error with status and first 200 bytes of body (trimmed). Client timeout 60 s safety default; context governs.
Secret safety: errors never include the secret; response snippet has any echoed secret replaced by `[redacted]`; `*url.Error` is unwrapped and URLs are not echoed in config errors, since URLs may carry credentials. Nothing is logged.

## Decisions for docs/decisions.md
1. `types: []` (explicit empty list) means no events; omitted means all. Spec says "defaults to all" only when absent.
2. Hook errors drop the URL (possible userinfo credentials) at the cost of less context.
3. Echoed secret in response bodies is redacted.
4. Event JSON uses `setup` (name) in `hold`, since Hold has SetupName only.
5. static allows duplicate resource names.

## Files changed
pkg/hourglass/event_json.go, event_json_test.go, BUILD.bazel (gazelle, SPDX kept); providers/static/{static.go,static_test.go,BUILD.bazel}; hooks/webhook/{webhook.go,webhook_test.go,BUILD.bazel}; this folder (PROGRESS.md, RESULT.md, bazel-test.log).

## Commands
`go vet` and `go test -race -count=1 ./providers/... ./hooks/... ./pkg/hourglass/...` pass; `bazel test //providers/static/... //hooks/webhook/... //pkg/hourglass/...` 3/3 PASSED (bazel-test.log); `./tools/check_spdx.sh` clean; `./tools/check_core_generic.sh` OK. Both extensions run the hourglasstest conformance suites (TestEXT5_Conformance) and import only stdlib + pkg/hourglass.

## Confidence and gaps
Gazelle preserved the SPDX line in pkg/hourglass/BUILD.bazel; new BUILD files got it by hand. Not tested against a real remote endpoint beyond httptest. No spec conflicts.
