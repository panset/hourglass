// SPDX-License-Identifier: MIT

// Package settings holds the admin-editable metering settings (ADM-2): typed
// values with defaults and validation, one JSON value per key in the settings
// table, a stable API JSON form, and the resource-name template (SET-4).
package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/panset/hourglass/internal/metering"
	"github.com/panset/hourglass/internal/rules"
	"github.com/panset/hourglass/internal/store"
)

// Setting keys as stored in the settings table and used in the API JSON.
const (
	KeyResourceNameTemplate    = "resource_name_template"
	KeyDefaultWeight           = "default_weight"
	KeyDisplayLabelKeys        = "display_label_keys"
	KeyBands                   = "bands"
	KeyDefaultAllowance        = "default_allowance"
	KeyMaxHoldsPerUser         = "max_holds_per_user"
	KeyReleaseRequiresTeardown = "release_requires_teardown"
	KeyPollInterval            = "poll_interval"
	KeyEnforcement             = "enforcement"
)

// keyOrder is the fixed order of all keys.
var keyOrder = []string{
	KeyResourceNameTemplate, KeyDefaultWeight, KeyDisplayLabelKeys, KeyBands,
	KeyDefaultAllowance, KeyMaxHoldsPerUser, KeyReleaseRequiresTeardown,
	KeyPollInterval, KeyEnforcement,
}

// MinPollInterval is the smallest allowed poll interval.
const MinPollInterval = time.Minute

// MaxDisplayLabelKeys bounds DisplayLabelKeys (the SET-3 label limit).
const MaxDisplayLabelKeys = rules.MaxLabels

// Settings is the full set of admin-editable metering settings.
type Settings struct {
	ResourceNameTemplate    string // SET-4
	DefaultWeight           float64
	DisplayLabelKeys        []string // SET-6
	Bands                   metering.Bands
	DefaultAllowanceMT      int64 // milli-tokens
	MaxHoldsPerUser         int
	ReleaseRequiresTeardown bool
	PollInterval            time.Duration
	Enforcement             bool // false is shadow mode
}

// Defaults returns the spec defaults.
func Defaults() Settings {
	return Settings{
		ResourceNameTemplate:    "{{.SetupName}}",
		DefaultWeight:           1,
		DisplayLabelKeys:        []string{},
		Bands:                   metering.DefaultBands(),
		DefaultAllowanceMT:      300 * metering.MilliPerToken,
		MaxHoldsPerUser:         2,
		ReleaseRequiresTeardown: true,
		PollInterval:            5 * time.Minute,
		Enforcement:             false,
	}
}

// FieldError addresses a validation problem to one field (JSON key name).
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

func fieldErr(field, format string, args ...any) error {
	return &FieldError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// Validate returns a *FieldError for the first problem found, or nil.
func (s Settings) Validate() error {
	if _, err := ResourceName(s.ResourceNameTemplate, sampleSetup, nil, ""); err != nil {
		return fieldErr(KeyResourceNameTemplate, "%v", err)
	}
	if math.IsNaN(s.DefaultWeight) || math.IsInf(s.DefaultWeight, 0) || s.DefaultWeight < 0 {
		return fieldErr(KeyDefaultWeight, "must be finite and >= 0")
	}
	if len(s.DisplayLabelKeys) > MaxDisplayLabelKeys {
		return fieldErr(KeyDisplayLabelKeys, "too many keys: %d (at most %d)", len(s.DisplayLabelKeys), MaxDisplayLabelKeys)
	}
	seen := make(map[string]bool, len(s.DisplayLabelKeys))
	for _, k := range s.DisplayLabelKeys {
		if err := rules.ValidateLabels(map[string]string{k: ""}); err != nil {
			return fieldErr(KeyDisplayLabelKeys, "%v", err)
		}
		if seen[k] {
			return fieldErr(KeyDisplayLabelKeys, "duplicate key %q", k)
		}
		seen[k] = true
	}
	if err := s.Bands.Validate(); err != nil {
		return fieldErr(KeyBands, "%v", err)
	}
	if s.DefaultAllowanceMT < 0 {
		return fieldErr(KeyDefaultAllowance, "must be >= 0")
	}
	if s.MaxHoldsPerUser < 0 {
		return fieldErr(KeyMaxHoldsPerUser, "must be >= 0")
	}
	if s.PollInterval < MinPollInterval {
		return fieldErr(KeyPollInterval, "must be at least %s", MinPollInterval)
	}
	return nil
}

// ---- JSON encoding of individual values (shared by the API and the store) ----

type stepJSON struct {
	From       float64 `json:"from"`
	Multiplier float64 `json:"multiplier"`
}

type bandsJSON struct {
	Base  float64    `json:"base"`
	Steps []stepJSON `json:"steps"`
}

func formatDuration(d time.Duration) string {
	s := d.String()
	if n := len(s); n > 2 && s[n-2:] == "0s" && s[n-3] == 'm' {
		s = s[:n-2]
	}
	if n := len(s); n > 2 && s[n-2:] == "0m" && s[n-3] == 'h' {
		s = s[:n-2]
	}
	return s
}

// encodeKey returns the JSON value of one key.
func (s Settings) encodeKey(key string) (json.RawMessage, error) {
	var v any
	switch key {
	case KeyResourceNameTemplate:
		v = s.ResourceNameTemplate
	case KeyDefaultWeight:
		v = s.DefaultWeight
	case KeyDisplayLabelKeys:
		keys := s.DisplayLabelKeys
		if keys == nil {
			keys = []string{}
		}
		v = keys
	case KeyBands:
		b := bandsJSON{Base: s.Bands.Base, Steps: make([]stepJSON, len(s.Bands.Steps))}
		for i, st := range s.Bands.Steps {
			b.Steps[i] = stepJSON{From: st.From, Multiplier: st.Multiplier}
		}
		v = b
	case KeyDefaultAllowance:
		v = float64(s.DefaultAllowanceMT) / metering.MilliPerToken
	case KeyMaxHoldsPerUser:
		v = s.MaxHoldsPerUser
	case KeyReleaseRequiresTeardown:
		v = s.ReleaseRequiresTeardown
	case KeyPollInterval:
		v = formatDuration(s.PollInterval)
	case KeyEnforcement:
		v = s.Enforcement
	default:
		return nil, fmt.Errorf("unknown setting %q", key)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("setting %q: %w", key, err)
	}
	return b, nil
}

func strictDecode(raw json.RawMessage, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data")
	}
	return nil
}

// decodeKey sets one key of s from its JSON value.
func (s *Settings) decodeKey(key string, raw json.RawMessage) error {
	var err error
	switch key {
	case KeyResourceNameTemplate:
		err = strictDecode(raw, &s.ResourceNameTemplate)
	case KeyDefaultWeight:
		err = strictDecode(raw, &s.DefaultWeight)
	case KeyDisplayLabelKeys:
		var keys []string
		if err = strictDecode(raw, &keys); err == nil {
			if keys == nil {
				keys = []string{}
			}
			s.DisplayLabelKeys = keys
		}
	case KeyBands:
		var b bandsJSON
		if err = strictDecode(raw, &b); err == nil {
			s.Bands = metering.Bands{Base: b.Base}
			for _, st := range b.Steps {
				s.Bands.Steps = append(s.Bands.Steps, metering.Step{From: st.From, Multiplier: st.Multiplier})
			}
		}
	case KeyDefaultAllowance:
		var tokens float64
		if err = strictDecode(raw, &tokens); err == nil {
			mt := math.Round(tokens * metering.MilliPerToken)
			if math.IsNaN(mt) || math.Abs(mt) > 1<<53 {
				return fieldErr(key, "out of range")
			}
			s.DefaultAllowanceMT = int64(mt)
		}
	case KeyMaxHoldsPerUser:
		err = strictDecode(raw, &s.MaxHoldsPerUser)
	case KeyReleaseRequiresTeardown:
		err = strictDecode(raw, &s.ReleaseRequiresTeardown)
	case KeyPollInterval:
		var str string
		if err = strictDecode(raw, &str); err == nil {
			var d time.Duration
			if d, err = time.ParseDuration(str); err == nil {
				s.PollInterval = d
			}
		}
	case KeyEnforcement:
		err = strictDecode(raw, &s.Enforcement)
	default:
		return fmt.Errorf("unknown setting %q", key)
	}
	if err != nil {
		var fe *FieldError
		if errors.As(err, &fe) {
			return err
		}
		return fieldErr(key, "%v", err)
	}
	return nil
}

func isKnownKey(key string) bool {
	for _, k := range keyOrder {
		if k == key {
			return true
		}
	}
	return false
}

// MarshalJSON writes the stable snake_case API form.
func (s Settings) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keyOrder {
		v, err := s.encodeKey(k)
		if err != nil {
			return nil, err
		}
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(v)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// UnmarshalJSON reads the API form. Unknown fields are rejected; fields that
// are absent take their default value (the result does not depend on the
// receiver's previous contents). The result is not validated; call Validate.
func (s *Settings) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	out := Defaults()
	for _, k := range keyOrder {
		raw, ok := m[k]
		if !ok {
			continue
		}
		if err := out.decodeKey(k, raw); err != nil {
			return err
		}
	}
	for k := range m {
		if !isKnownKey(k) {
			return fieldErr(k, "unknown field")
		}
	}
	*s = out
	return nil
}

// ---- storage ----

// Reader is satisfied by the store's AllSettings method.
type Reader interface {
	AllSettings(ctx context.Context) ([]store.Setting, error)
}

// Writer is satisfied by the store's PutSetting method.
type Writer interface {
	PutSetting(ctx context.Context, key, valueJSON string, updatedBy *int64, now time.Time) error
}

// Change is one setting that Save wrote; Before and After are JSON values (ADM-3).
type Change struct {
	Key    string
	Before string
	After  string
}

// now stamps updated_at for Save; SaveAt takes the time explicitly.
var now = time.Now

// Load reads every stored setting. Missing keys take defaults; unknown keys
// are ignored. A stored value that cannot be decoded is an error.
func Load(ctx context.Context, r Reader) (Settings, error) {
	rows, err := r.AllSettings(ctx)
	if err != nil {
		return Settings{}, err
	}
	s := Defaults()
	for _, row := range rows {
		if !isKnownKey(row.Key) {
			continue
		}
		if err := s.decodeKey(row.Key, json.RawMessage(row.ValueJSON)); err != nil {
			return Settings{}, fmt.Errorf("stored setting %q: %w", row.Key, err)
		}
	}
	// A stored template that breaks the SET-4 restrictions (for example one
	// written before they existed) fails the load, hence the poll, with a clear error.
	if err := ValidateTemplate(s.ResourceNameTemplate); err != nil {
		return Settings{}, fmt.Errorf("stored setting %q: %w", KeyResourceNameTemplate, err)
	}
	return s, nil
}

// Save validates next and writes only the keys whose JSON value differs from
// old, attributing them to actorID. It returns the changes for the audit log.
func Save(ctx context.Context, w Writer, old, next Settings, actorID int64) ([]Change, error) {
	return SaveAt(ctx, w, old, next, actorID, now())
}

// SaveAt is Save with the updated_at timestamp supplied by the caller, so
// production code can pass its injected clock.
func SaveAt(ctx context.Context, w Writer, old, next Settings, actorID int64, at time.Time) ([]Change, error) {
	if err := next.Validate(); err != nil {
		return nil, err
	}
	var changes []Change
	t := at
	for _, k := range keyOrder {
		before, err := old.encodeKey(k)
		if err != nil {
			return changes, err
		}
		after, err := next.encodeKey(k)
		if err != nil {
			return changes, err
		}
		if bytes.Equal(before, after) {
			continue
		}
		if err := w.PutSetting(ctx, k, string(after), &actorID, t); err != nil {
			return changes, fmt.Errorf("save %q: %w", k, err)
		}
		changes = append(changes, Change{Key: k, Before: string(before), After: string(after)})
	}
	return changes, nil
}
