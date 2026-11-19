// SPDX-License-Identifier: MIT

// Package busiestsite is a PressureSource that measures contention per
// site and reports the worst one. A lab spread over several sites is
// "busy" when any one site is nearly full, even if the others sit idle,
// because a developer usually needs a rig at a particular site.
//
// It shows how a pressure source branches on the labels carried by the
// Snapshot: setups are grouped by the value of a label (default "site"),
// and no database access is needed.
//
// Config (all optional):
//
//	pressure:
//	  type: busiest-site
//	  config:
//	    label: site
package busiestsite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/panset/hourglass/pkg/hourglass"
)

// Name is the name the config file uses to select this source.
const Name = "busiest-site"

func init() {
	hourglass.RegisterPressureSource(Name, New)
}

type source struct {
	label string
}

// New is the factory. The only setting is the label key to group by.
func New(raw json.RawMessage) (hourglass.PressureSource, error) {
	c := struct {
		Label string `json:"label"`
	}{Label: "site"}
	if t := bytes.TrimSpace(raw); len(t) > 0 && string(t) != "null" {
		dec := json.NewDecoder(bytes.NewReader(t))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&c); err != nil {
			return nil, fmt.Errorf("busiest-site: invalid config: %w", err)
		}
	}
	if c.Label == "" {
		return nil, fmt.Errorf("busiest-site: config label must not be empty")
	}
	return source{label: c.Label}, nil
}

func (source) Name() string { return Name }

// Pressure returns the highest held-weight fraction over all sites: for each
// group of setups sharing a label value, the weight of the held ones divided
// by the weight of all of them. Setups without the label, and setups of
// weight 0, are ignored. With nothing to measure the answer is 0.
func (s source) Pressure(ctx context.Context, snap hourglass.Snapshot) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	held := map[string]bool{}
	for _, h := range snap.Holds {
		if h.ReleasedAt == nil {
			held[h.SetupName] = true
		}
	}
	type sum struct{ total, held float64 }
	sites := map[string]*sum{}
	for _, st := range snap.Setups {
		site, ok := st.Labels[s.label] // the label this deployment groups by
		if !ok {
			continue
		}
		if math.IsNaN(st.Weight) || math.IsInf(st.Weight, 0) || st.Weight < 0 {
			return 0, fmt.Errorf("busiest-site: setup %q has an invalid weight %v", st.Name, st.Weight)
		}
		if st.Weight == 0 {
			continue
		}
		g := sites[site]
		if g == nil {
			g = &sum{}
			sites[site] = g
		}
		g.total += st.Weight
		if held[st.Name] {
			g.held += st.Weight
		}
	}
	worst := 0.0
	for _, g := range sites {
		if f := g.held / g.total; f > worst {
			worst = f
		}
	}
	// Returning a value outside 0..1 would fail the poll; clamp defensively.
	return math.Min(worst, 1), nil
}
