// SPDX-License-Identifier: MIT

// Package settings holds the admin-editable metering settings (ADM-2): typed
// values with defaults and validation, one JSON value per key in the settings
// table and a stable API JSON form.
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
	KeyDefaultWeight           = "default_weight"
	KeyDisplayLabelKeys        = "display_label_keys"
	KeyBands                   = "bands"
	KeyDefaultAllowance        = "default_allowance"
	KeyMaxHoldsPerUser         = "max_holds_per_user"
	KeyReleaseRequiresTeardown = "release_requires_teardown"
	KeyPollInterval            = "poll_interval"
	KeyEnforcement             = "enforcement"
	KeyReportedWeights         = "reported_weights"
)

// keyOrder is the fixed order of all keys.
var keyOrder = []string{
	KeyDefaultWeight, KeyDisplayLabelKeys, KeyBands,
	KeyDefaultAllowance, KeyMaxHoldsPerUser, KeyReleaseRequiresTeardown,
	KeyPollInterval, KeyEnforcement, KeyReportedWeights,
}

// MinPollInterval is the smallest allowed poll interval.
const MinPollInterval = time.Minute

// Bounds of the reported_weights setting (docs/design/reported-weights.md, D1).
const (
	// MaxReportedWeight is the largest allowed maximum: the limit for a rate
	// rule's weight.
	MaxReportedWeight = 1e12
	// MinReportedWeightsMaxAge and MaxReportedWeightsMaxAge bound max_age.
	MinReportedWeightsMaxAge = time.Minute
	MaxReportedWeightsMaxAge = 24 * time.Hour
)

// MaxDisplayLabelKeys bounds DisplayLabelKeys (the SET-3 label limit).
const MaxDisplayLabelKeys = rules.MaxLabels

// Settings is the full set of admin-editable metering settings.
type Settings struct {
	DefaultWeight           float64
	DisplayLabelKeys        []string // SET-6
	Bands                   metering.Bands
	DefaultAllowanceMT      int64 // milli-tokens
	MaxHoldsPerUser         int
	ReleaseRequiresTeardown bool
	PollInterval            time.Duration
	Enforcement             bool // false is shadow mode
	ReportedWeights         ReportedWeights
}

// ReportedWeights is the reported_weights setting (design D1): whether a
// service may report weights, the inclusive limits a reported weight must
// lie within, and how long a weights report stays usable.
type ReportedWeights struct {
	Enabled bool
	Min     float64
	Max     *float64 // nil when no maximum is set; Enabled needs one
	MaxAge  time.Duration
}

// Defaults returns the spec defaults.
func Defaults() Settings {
	return Settings{
		DefaultWeight:           1,
		DisplayLabelKeys:        []string{},
		Bands:                   metering.DefaultBands(),
		DefaultAllowanceMT:      300 * metering.MilliPerToken,
		MaxHoldsPerUser:         2,
		ReleaseRequiresTeardown: true,
		PollInterval:            5 * time.Minute,
		Enforcement:             false,
		ReportedWeights:         ReportedWeights{Enabled: false, Min: 0, Max: nil, MaxAge: 10 * time.Minute},
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
	return s.ReportedWeights.validate()
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// validate applies the rules of the D1 table. Errors name the field as
// reported_weights.<field>.
func (r ReportedWeights) validate() error {
	const k = KeyReportedWeights
	if r.Max != nil {
		if m := *r.Max; !finite(m) || m <= 0 || m > MaxReportedWeight {
			return fieldErr(k+".max", "max must be null, or a number above 0 and at most 1e12")
		}
	}
	if !finite(r.Min) || r.Min < 0 {
		return fieldErr(k+".min", "min must be a number of at least 0")
	}
	if r.Max != nil && r.Min > *r.Max {
		return fieldErr(k+".min", "min must not be above max")
	}
	if r.MaxAge < MinReportedWeightsMaxAge || r.MaxAge > MaxReportedWeightsMaxAge {
		return fieldErr(k+".max_age", "max_age must be a duration from 1m to 24h")
	}
	if r.Enabled && r.Max == nil {
		return fieldErr(k+".enabled", "enabled can be true only when max is set: set a maximum weight to turn reported weights on")
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

type reportedWeightsJSON struct {
	Enabled bool     `json:"enabled"`
	Min     float64  `json:"min"`
	Max     *float64 `json:"max"`
	MaxAge  string   `json:"max_age"`
}

// reportedWeightsFields are the fields of the reported_weights object, all
// required when the key is present.
var reportedWeightsFields = []string{"enabled", "min", "max", "max_age"}

// decodeReportedWeights reads the reported_weights object: all four fields
// are required (max may be null), no other field is allowed. Negative zero
// counts as 0. The values are not validated; Validate does that.
func decodeReportedWeights(raw json.RawMessage) (ReportedWeights, error) {
	const k = KeyReportedWeights
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return ReportedWeights{}, fieldErr(k, "must be an object with enabled, min, max and max_age")
	}
	for _, f := range reportedWeightsFields {
		if _, ok := fields[f]; !ok {
			return ReportedWeights{}, fieldErr(k+"."+f, "%s is required", f)
		}
	}
	for f := range fields {
		known := false
		for _, g := range reportedWeightsFields {
			known = known || f == g
		}
		if !known {
			return ReportedWeights{}, fieldErr(k+"."+f, "unknown field %s", f)
		}
	}
	var out ReportedWeights
	if err := strictDecode(fields["enabled"], &out.Enabled); err != nil || string(fields["enabled"]) == "null" {
		return out, fieldErr(k+".enabled", "enabled must be true or false")
	}
	if err := strictDecode(fields["min"], &out.Min); err != nil || string(fields["min"]) == "null" {
		return out, fieldErr(k+".min", "min must be a number")
	}
	if err := strictDecode(fields["max"], &out.Max); err != nil {
		return out, fieldErr(k+".max", "max must be null or a number")
	}
	var age string
	if err := strictDecode(fields["max_age"], &age); err != nil || string(fields["max_age"]) == "null" {
		return out, fieldErr(k+".max_age", "max_age must be a duration string such as \"10m\"")
	}
	d, err := time.ParseDuration(age)
	if err != nil {
		return out, fieldErr(k+".max_age", "max_age must be a duration string such as \"10m\"")
	}
	out.MaxAge = d
	if out.Min == 0 {
		out.Min = 0 // negative zero
	}
	if out.Max != nil && *out.Max == 0 {
		z := 0.0
		out.Max = &z
	}
	return out, nil
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
	case KeyReportedWeights:
		r := s.ReportedWeights
		v = reportedWeightsJSON{Enabled: r.Enabled, Min: r.Min, Max: r.Max, MaxAge: formatDuration(r.MaxAge)}
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
	case KeyReportedWeights:
		var r ReportedWeights
		if r, err = decodeReportedWeights(raw); err == nil {
			s.ReportedWeights = r
		}
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
// A caller for which an absent reported_weights means "unchanged" (PUT
// /api/v1/admin/settings, design D1) checks for the key and copies the stored
// value itself.
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
