// SPDX-License-Identifier: MIT

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/panset/hourglass/internal/config"
	"github.com/panset/hourglass/internal/rules"
	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
)

// maxDetailsBytes is the SET-2 limit on a setup's details.
const maxDetailsBytes = 256 << 10

// seedFile is the seed document (SET-7). YAML and JSON share one decoder:
// YAML is converted to JSON first, then decoded strictly.
type seedFile struct {
	Setups        []seedSetup `json:"setups"`
	RateRules     []seedRule  `json:"rate_rules"`
	DefaultWeight *float64    `json:"default_weight"`
}

type seedSetup struct {
	Name                 string            `json:"name"`
	Labels               map[string]string `json:"labels"`
	Details              json.RawMessage   `json:"details"`
	Enabled              *bool             `json:"enabled"` // default true
	ResourceNameOverride string            `json:"resource_name_override"`
}

type seedRule struct {
	Selector string   `json:"selector"`
	Weight   *float64 `json:"weight"`
}

// SeedError reports a seed file that is not valid; nothing was imported.
type SeedError struct{ Msg string }

func (e *SeedError) Error() string { return "seed: " + e.Msg }

func seedErrf(format string, a ...any) error { return &SeedError{Msg: fmt.Sprintf(format, a...)} }

// ParseSeed reads a seed file (.json is JSON, anything else YAML) and
// validates it the way the admin API validates the same input. Unknown keys
// are errors.
func ParseSeed(path string) (*seedFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, seedErrf("%v", err)
	}
	raw := json.RawMessage(data)
	if strings.ToLower(filepath.Ext(path)) != ".json" {
		if raw, err = config.YAMLToJSON(data); err != nil {
			return nil, seedErrf("%s: %v", path, err)
		}
	}
	var f seedFile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, seedErrf("%s: %v", path, err)
	}
	if err := f.validate(); err != nil {
		return nil, err
	}
	return &f, nil
}

func (f *seedFile) validate() error {
	names := make(map[string]bool, len(f.Setups))
	for i, s := range f.Setups {
		where := fmt.Sprintf("setups[%d]", i)
		if strings.TrimSpace(s.Name) == "" {
			return seedErrf("%s: name is required", where)
		}
		if names[s.Name] {
			return seedErrf("%s: duplicate name %q", where, s.Name)
		}
		names[s.Name] = true
		if err := rules.ValidateLabels(s.Labels); err != nil {
			return seedErrf("%s (%s): %v", where, s.Name, err)
		}
		if len(s.Details) > maxDetailsBytes {
			return seedErrf("%s (%s): details exceed %d bytes", where, s.Name, maxDetailsBytes)
		}
		if len(s.Details) > 0 && !json.Valid(s.Details) {
			return seedErrf("%s (%s): details are not valid JSON", where, s.Name)
		}
		if _, err := settings.ResourceName(settings.Defaults().ResourceNameTemplate, s.Name, s.Labels, s.ResourceNameOverride); err != nil {
			return seedErrf("%s (%s): resource name: %v", where, s.Name, err)
		}
	}
	for i, r := range f.RateRules {
		where := fmt.Sprintf("rate_rules[%d]", i)
		if _, err := rules.Parse(r.Selector); err != nil {
			return seedErrf("%s: selector: %v", where, err)
		}
		if r.Weight == nil || !validWeight(*r.Weight) {
			return seedErrf("%s: weight must be a finite number >= 0", where)
		}
	}
	if f.DefaultWeight != nil && !validWeight(*f.DefaultWeight) {
		return seedErrf("default_weight must be a finite number >= 0")
	}
	return nil
}

func validWeight(w float64) bool { return !math.IsNaN(w) && !math.IsInf(w, 0) && w >= 0 }

// SeedMarkerKey is the settings row that records, durably, that the database
// has been initialised (SET-7): written in the same transaction as the seed,
// or on the first start without a seed, or when an existing database is
// adopted. While it exists the seed is never applied, even when every setup
// has since been deleted. Settings.Load ignores unknown keys, so it is not a
// user-visible setting.
const SeedMarkerKey = "seed_state"

// Values of the marker's "state" field.
const (
	SeedStateApplied  = "applied"           // the seed file was imported
	SeedStateNoSeed   = "no_seed"           // first start without a seed file
	SeedStateExisting = "existing_database" // data was already there and no marker: treated as initialised
)

func seedMarkerValue(state string) string {
	b, _ := json.Marshal(map[string]string{"state": state})
	return string(b)
}

// ImportSeed implements SET-7. On the first start of a database (no marker) it
// imports the seed's setups, rate rules and optional default weight, if path
// is set, and records the marker in the same transaction. A database that
// already holds setups, rules, or users other than the local admin, and has no
// marker, is treated as initialised: the marker is written and the file is not
// read. Once the marker exists the file is never read, whatever the data looks
// like. It reports whether anything was imported. An invalid seed changes
// nothing (no marker either), so the next start can try again.
func ImportSeed(ctx context.Context, st *store.Store, path string, now time.Time) (bool, error) {
	has, err := hasSeedMarker(ctx, st)
	if err != nil || has {
		return false, err
	}
	var f *seedFile
	if path != "" {
		existing, err := initialisedAlready(ctx, st)
		if err != nil {
			return false, err
		}
		if !existing {
			if f, err = ParseSeed(path); err != nil {
				return false, err
			}
		}
	}
	applied := false
	err = st.Tx(ctx, func(tx *store.Tx) error {
		// Re-check inside the write transaction: two starts cannot both seed.
		if _, err := tx.GetSetting(ctx, SeedMarkerKey); err == nil {
			return nil
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		state := SeedStateNoSeed
		if existing, err := initialisedAlready(ctx, tx); err != nil {
			return err
		} else if existing {
			state = SeedStateExisting
			f = nil
		} else if f != nil {
			state = SeedStateApplied
		}
		if f != nil {
			if err := applySeed(ctx, tx, f, now); err != nil {
				return err
			}
			applied = true
		}
		return tx.PutSetting(ctx, SeedMarkerKey, seedMarkerValue(state), nil, now)
	})
	if err != nil {
		var se *SeedError
		if errors.As(err, &se) {
			return false, err
		}
		return false, fmt.Errorf("seed: %w", err)
	}
	return applied, nil
}

func hasSeedMarker(ctx context.Context, st *store.Store) (bool, error) {
	_, err := st.GetSetting(ctx, SeedMarkerKey)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// dataReader is what both the store and a transaction offer.
type dataReader interface {
	ListSetups(ctx context.Context, enabledOnly bool) ([]store.Setup, error)
	ListRules(ctx context.Context, kind string) ([]store.Rule, error)
	ListUsers(ctx context.Context, status string) ([]store.User, error)
}

// initialisedAlready: the database holds setups, rules, or any user other than
// the local admin.
func initialisedAlready(ctx context.Context, q dataReader) (bool, error) {
	ss, err := q.ListSetups(ctx, false)
	if err != nil || len(ss) > 0 {
		return len(ss) > 0, err
	}
	rs, err := q.ListRules(ctx, rules.KindRate)
	if err != nil || len(rs) > 0 {
		return len(rs) > 0, err
	}
	us, err := q.ListUsers(ctx, "")
	if err != nil {
		return false, err
	}
	for _, u := range us {
		if u.AuthSource != store.AuthLocal {
			return true, nil
		}
	}
	return false, nil
}

func applySeed(ctx context.Context, tx *store.Tx, f *seedFile, now time.Time) error {
	for _, s := range f.Setups {
		details := "{}"
		if d := bytes.TrimSpace(s.Details); len(d) > 0 && string(d) != "null" {
			details = string(d)
		}
		enabled := s.Enabled == nil || *s.Enabled
		if _, err := tx.CreateSetup(ctx, store.Setup{Name: s.Name, Labels: s.Labels, DetailsJSON: details,
			Enabled: enabled, ResourceNameOverride: s.ResourceNameOverride}); err != nil {
			return fmt.Errorf("setup %q: %w", s.Name, err)
		}
	}
	if len(f.RateRules) > 0 {
		rs := make([]store.Rule, len(f.RateRules))
		for i, r := range f.RateRules {
			rs[i] = store.Rule{Kind: rules.KindRate, Selector: r.Selector, Value: *r.Weight}
		}
		if err := tx.ReplaceRules(ctx, rules.KindRate, rs, nil, now); err != nil {
			return err
		}
	}
	if f.DefaultWeight != nil {
		v, err := json.Marshal(*f.DefaultWeight)
		if err != nil {
			return err
		}
		if err := tx.PutSetting(ctx, settings.KeyDefaultWeight, string(v), nil, now); err != nil {
			return err
		}
	}
	return nil
}
