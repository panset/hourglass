// SPDX-License-Identifier: MIT

// Package static is a ResourceProvider that reads allocations from a JSON
// file, for demos and tests. Register it by importing the package:
//
//	import _ "github.com/panset/hourglass/providers/static"
//
// Config:
//
//	{"path": "/path/allocations.json"}
//
// The file is a JSON array of {"resource_name": "...", "labels": {"k": "v"}}.
// It is re-read on every List, so it can be edited while the system runs.
package static

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/panset/hourglass/pkg/hourglass"
)

// Name is the registered name of the provider.
const Name = "static"

func init() {
	hourglass.RegisterProvider(Name, New)
}

type config struct {
	Path string `json:"path"`
}

type entry struct {
	ResourceName string            `json:"resource_name"`
	Labels       map[string]string `json:"labels"`
}

type provider struct {
	path string
}

// New is the factory registered under "static".
func New(raw json.RawMessage) (hourglass.ResourceProvider, error) {
	var c config
	if t := bytes.TrimSpace(raw); len(t) == 0 || string(t) == "null" {
		return nil, errors.New(`static: config "path" is required`)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("static: invalid config: %w", err)
	}
	if dec.More() {
		return nil, errors.New("static: invalid config: trailing data")
	}
	if c.Path == "" {
		return nil, errors.New(`static: config "path" is required`)
	}
	return &provider{path: c.Path}, nil
}

func (p *provider) Name() string { return Name }

// List re-reads the file. A missing or malformed file, or an entry with an
// empty resource_name, is an error.
func (p *provider) List(ctx context.Context) ([]hourglass.Allocation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p.path)
	if err != nil {
		return nil, fmt.Errorf("static: read allocations: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var entries []entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("static: parse %s: %w", p.path, err)
	}
	if entries == nil {
		return nil, fmt.Errorf("static: parse %s: want a JSON array, got null", p.path)
	}
	out := make([]hourglass.Allocation, 0, len(entries))
	for i, e := range entries {
		if e.ResourceName == "" {
			return nil, fmt.Errorf("static: entry %d in %s has an empty resource_name", i, p.path)
		}
		out = append(out, hourglass.Allocation{ResourceName: e.ResourceName, Labels: e.Labels})
	}
	return out, nil
}
