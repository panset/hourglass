// SPDX-License-Identifier: MIT

// Package inventory is the bundled pressure source that needs no backend:
// the weight of held setups divided by the weight of all enabled setups.
package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/panset/hourglass/pkg/hourglass"
)

// Name is the registration name.
const Name = "inventory"

func init() {
	hourglass.RegisterPressureSource(Name, New)
}

type source struct{}

// New builds the source. It takes no config: an empty block, null or {}.
func New(config json.RawMessage) (hourglass.PressureSource, error) {
	if trimmed := bytes.TrimSpace(config); len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) {
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		dec.DisallowUnknownFields()
		var cfg struct{}
		if err := dec.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("inventory: invalid config (it takes no settings): %w", err)
		}
		if dec.More() {
			return nil, fmt.Errorf("inventory: invalid config: trailing data")
		}
	}
	return source{}, nil
}

func (source) Name() string { return Name }

// Pressure returns held weight over total weight, clamped to [0,1]. A total
// weight of 0 gives 0. Allocations are ignored.
func (source) Pressure(_ context.Context, s hourglass.Snapshot) (float64, error) {
	held := map[string]bool{}
	for _, h := range s.Holds {
		if h.ReleasedAt == nil {
			held[h.SetupName] = true
		}
	}
	var total, heldW float64
	for _, st := range s.Setups {
		if st.Weight <= 0 || st.Weight != st.Weight {
			continue
		}
		total += st.Weight
		if held[st.Name] {
			heldW += st.Weight
		}
	}
	if total <= 0 {
		return 0, nil
	}
	p := heldW / total
	if p < 0 {
		return 0, nil
	}
	if p > 1 {
		return 1, nil
	}
	return p, nil
}
