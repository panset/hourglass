// SPDX-License-Identifier: MIT

// Package filedir is a ResourceProvider for a lab where a rig's presence is
// a file: one allocation per regular file in a directory, and the file name
// is the resource name. Create the file to "deploy" a rig, delete it to
// "tear it down".
//
// It is the smallest useful provider and shows the three things every
// provider does: parse an opaque config block, honour the context, and
// return one Allocation per deployed resource.
//
// Config:
//
//	provider:
//	  type: filedir
//	  config:
//	    dir: /var/lib/lab/rigs
package filedir

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/panset/hourglass/pkg/hourglass"
)

// Name is the name the config file uses to select this provider.
const Name = "filedir"

// init registers the factory, like a database/sql driver. A main package
// enables the extension with a blank import.
func init() {
	hourglass.RegisterProvider(Name, New)
}

type config struct {
	Dir string `json:"dir"`
}

type provider struct {
	dir string
}

// New is the factory. The config block arrives as JSON whatever the config
// file's own format; unknown keys are rejected so a typo fails at startup
// rather than silently doing nothing.
func New(raw json.RawMessage) (hourglass.ResourceProvider, error) {
	var c config
	if t := bytes.TrimSpace(raw); len(t) == 0 || string(t) == "null" {
		return nil, errors.New(`filedir: config "dir" is required`)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("filedir: invalid config: %w", err)
	}
	if c.Dir == "" {
		return nil, errors.New(`filedir: config "dir" is required`)
	}
	return &provider{dir: c.Dir}, nil
}

func (p *provider) Name() string { return Name }

// List reports one allocation per regular file in the directory. Hidden
// files (a leading dot) and sub-directories are ignored. The result is
// sorted so two calls on an unchanged directory are identical, which the
// conformance suite checks. A missing directory is an error: the core then
// records a gap instead of charging for a lab it cannot see.
func (p *provider) List(ctx context.Context) ([]hourglass.Allocation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		return nil, fmt.Errorf("filedir: read %s: %w", p.dir, err)
	}
	out := make([]hourglass.Allocation, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if name == "" || name[0] == '.' || !e.Type().IsRegular() {
			continue
		}
		out = append(out, hourglass.Allocation{
			ResourceName: name,
			Labels:       map[string]string{"source": Name},
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ResourceName < out[j].ResourceName })
	return out, nil
}
