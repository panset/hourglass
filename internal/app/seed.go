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

// ImportSeed implements SET-7: when the database has no setups and path is
// set, it imports the seed's setups, rate rules and optional default weight
// in one transaction. It reports whether anything was imported. Once any
// setup exists the file is not even read. An invalid seed changes nothing.
func ImportSeed(ctx context.Context, st *store.Store, path string, now time.Time) (bool, error) {
	if path == "" {
		return false, nil
	}
	existing, err := st.ListSetups(ctx, false)
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		return false, nil
	}
	f, err := ParseSeed(path)
	if err != nil {
		return false, err
	}
	applied := false
	err = st.Tx(ctx, func(tx *store.Tx) error {
		// Re-check inside the write transaction: two starts cannot both seed.
		cur, err := tx.ListSetups(ctx, false)
		if err != nil {
			return err
		}
		if len(cur) > 0 {
			return nil
		}
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
		applied = true
		return nil
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
