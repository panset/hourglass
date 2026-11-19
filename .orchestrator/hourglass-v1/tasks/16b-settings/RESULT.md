# 16b internal/settings: RESULT

Status: done.

## Files
internal/settings/{settings.go, resourcename.go, settings_test.go, BUILD.bazel}; tasks/16b-settings/{PROGRESS,RESULT}.md.

## Exported API
- `Settings` (fields per brief), `Defaults()`, `(Settings).Validate() error` -> `*FieldError{Field, Message}` (Field is the JSON key).
- `Load(ctx, Reader) (Settings, error)`, `Save(ctx, Writer, old, new Settings, actorID int64) ([]Change, error)`, `Change{Key, Before, After}` (JSON values).
- `Reader` = `AllSettings(ctx) ([]store.Setting, error)`; `Writer` = `PutSetting(ctx, key, valueJSON string, updatedBy *int64, now time.Time) error` (compile-time asserted against `*store.Store`).
- `ResourceName(tmpl, setupName, labels, override) (string, error)`; parsed templates cached in a sync.Map.
- Key constants `Key*`, `MinPollInterval`, `MaxDisplayLabelKeys`.

## JSON form (API and per-key storage use the same value encoding)
Keys: resource_name_template (string), default_weight (number), display_label_keys ([]string), bands ({"base":0,"steps":[{"from":0.5,"multiplier":1},...]}), default_allowance (tokens, decimals ok; stored as milli-tokens, rounded), max_holds_per_user (int), release_requires_teardown (bool), poll_interval ("5m" written; "5m0s" accepted), enforcement (bool). Unknown fields rejected (also inside bands/steps).

## Decisions for docs/decisions.md
- Settings.UnmarshalJSON: absent fields take defaults (not the receiver's old value); result is not validated, caller runs Validate.
- Store rows hold the same per-key JSON as the API; Load fails on a stored value that cannot be decoded (unknown keys ignored).
- Save compares per-key JSON and writes with time.Now(); changes are in fixed key order.
- Template validation uses a sample setup name with no labels, so a template that is only a label reference is rejected as "empty".
- A nil DisplayLabelKeys is encoded as [].
- ResourceName with an override skips template parsing entirely.

## Checks
go vet, `go test -race ./internal/settings/...` pass; `bazel test //internal/settings/...` pass; check_core_generic.sh OK. check_spdx.sh exits 1 only for `internal/events/BUILD.bazel` (another agent's package, not mine).

## Confidence and gaps
No gaps in store interfaces. Not tested against a real SQLite store beyond the compile-time interface assertion.
