# 13 providers/static, hooks/webhook, and the Event JSON form
Job intent: .orchestrator/hourglass-v1/INTENT.md (read it first). Rules: .orchestrator/hourglass-v1/CONTRACT.md.
## Goal
Two bundled extensions, the `static` provider (package `providers/static`) and the `webhook` hook (package `hooks/webhook`), plus one stable JSON encoding of `hourglass.Event`. Spec: docs/SPEC.md "Extension points" tables and bullets, EXT-3, EXT-6, Configuration row "Hooks".
## Part A: Event JSON (in pkg/hourglass)
Add `pkg/hourglass/event_json.go` (+ test): `func (e Event) MarshalJSON() ([]byte, error)` and matching `UnmarshalJSON`, producing a documented snake_case object: `type`, `time` (RFC 3339 UTC), `user` (`id`, `username`, `email`, `name`, `roles`) or null, `setup` (`name`, `labels`, `details` passed through unmodified) or null, `hold` (`id`, `setup`, `user_id`, `acquired_at`, `metering_started_at`, `deployed_at`, `released_at`, nulls where unset) or null, `data` (object, `{}` when empty). Do not change any existing declaration or add struct tags to the spec's types; adapt field names to what the structs actually contain. Round-trip test. This is the form the webhook sends and the core will later store as delivery payloads.
## Part B: static provider
Config `{"path": "/path/allocations.json"}` (required). The file is a JSON array of `{"resource_name": "...", "labels": {"k": "v"}}`. It is re-read on every `List` so a demo can edit it while running. A missing or malformed file, or an entry with an empty resource_name, is an error (which fails the poll); an empty array is valid. Honour context cancellation.
## Part C: webhook hook
Config `{"url": "...", "secret": "...", "types": ["hold.acquired", ...]}`: url required (http or https); secret optional; types optional, default all; unknown event types in `types` are rejected at construction. `Handle` skips events not in `types`; otherwise POSTs the Event JSON with `Content-Type: application/json`, header `X-Hourglass-Event: <type>`, and, when a secret is set, `X-Hourglass-Signature: sha256=<hex HMAC-SHA256 of the body with the secret>`. Any non-2xx status or transport error is returned as an error that includes the status code and at most the first 200 bytes of the response body (so a vetoing endpoint's message can be shown for `hold.acquiring`). It uses the context for cancellation and sets no timeout of its own beyond a safety default (the core applies 30 s). Redirects are not followed. The secret must never appear in errors or logs.
Tests with `httptest.Server`: payload and headers, signature verifies, type filter, non-2xx, slow server with context deadline, secret absent from error text.
## Scope
May write: `providers/static/`, `hooks/webhook/`, `pkg/hourglass/event_json.go`, `pkg/hourglass/event_json_test.go`, this task folder. (After adding files in pkg/hourglass, run gazelle for that directory too and keep its SPDX line.)
## Context
API: `pkg/hourglass` (see tasks/10-extension-api/RESULT.md). Fakes and conformance suites: `pkg/hourglasstest` (see tasks/11-hourglasstest/RESULT.md). Each extension is its own package, registers itself by name in `init()` through the public registry, and imports only the standard library and `pkg/hourglass` (plus `pkg/hourglasstest` in tests). No `internal/` imports: these packages must be buildable by a third party exactly as a third-party extension would be.
## Build notes
No new dependencies; do not edit go.mod or MODULE.bazel. BUILD files via `bazel run //:gazelle -- <dir>`, then add the SPDX comment as the first line of each generated BUILD.bazel. Another agent is building sibling extension packages at the same time; Bazel commands queue, allow 5 minutes each. Verify with `go test -race ./<dir>/...`, `bazel test //<dir>/...`, `./tools/check_spdx.sh`, `./tools/check_core_generic.sh`.
## Tests
Each extension runs its conformance suite from `pkg/hourglasstest`, plus table-driven tests of its own behaviour named with requirement ids. Factory tests: valid config, empty config, unknown field (rejected), malformed JSON.
## Done when
All checks pass; RESULT.md documents the JSON form, both config schemas, and decisions.
