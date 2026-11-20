// SPDX-License-Identifier: MIT

// Package reported holds two bundled extensions, a ResourceProvider and a
// PressureSource, both named "reported". They answer the collector from the
// last report the customer's own service pushed to the integration API,
// instead of calling a backend. Register them by importing the package:
//
//	import _ "github.com/panset/hourglass/providers/reported"
//
// Config, the same for both:
//
//	{"max_age": "10m"}   // default 10m, minimum 1m
//
// A missing report, or one whose observed_at is older than max_age, is an
// error, so the poll fails and nobody is charged on old data. The reports are
// read through pkg/storedreports, which the server populates.
package reported

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/storedreports"
)

// Name is the registration name of both extensions.
const Name = "reported"

// Bounds of max_age.
const (
	DefaultMaxAge = 10 * time.Minute
	MinMaxAge     = time.Minute
)

func init() {
	hourglass.RegisterProvider(Name, NewProvider)
	hourglass.RegisterPressureSource(Name, NewPressureSource)
}

func parseMaxAge(config json.RawMessage) (time.Duration, error) {
	trimmed := bytes.TrimSpace(config)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return DefaultMaxAge, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var cfg struct {
		MaxAge *string `json:"max_age"`
	}
	if err := dec.Decode(&cfg); err != nil {
		return 0, fmt.Errorf("reported: invalid config: %w", err)
	}
	if dec.More() {
		return 0, errors.New("reported: invalid config: trailing data")
	}
	d := DefaultMaxAge
	if cfg.MaxAge != nil {
		var err error
		if d, err = time.ParseDuration(*cfg.MaxAge); err != nil {
			return 0, fmt.Errorf("reported: max_age %q is not a duration (for example 10m)", *cfg.MaxAge)
		}
	}
	if d < MinMaxAge {
		return 0, fmt.Errorf("reported: max_age must be at least %s", MinMaxAge)
	}
	return d, nil
}

type provider struct{ maxAge time.Duration }
type pressure struct{ maxAge time.Duration }

// NewProvider builds the provider.
func NewProvider(config json.RawMessage) (hourglass.ResourceProvider, error) {
	d, err := parseMaxAge(config)
	if err != nil {
		return nil, err
	}
	return provider{maxAge: d}, nil
}

// NewPressureSource builds the pressure source.
func NewPressureSource(config json.RawMessage) (hourglass.PressureSource, error) {
	d, err := parseMaxAge(config)
	if err != nil {
		return nil, err
	}
	return pressure{maxAge: d}, nil
}

func (provider) Name() string { return Name }
func (pressure) Name() string { return Name }

func staleErr(what string, age, maxAge time.Duration) error {
	return fmt.Errorf("reported: the %s report is %s old, older than max_age %s", what, age.Round(time.Second), maxAge)
}

// List returns one allocation per stored deployment.
func (p provider) List(ctx context.Context) ([]hourglass.Allocation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r := storedreports.Current()
	if r == nil {
		return nil, errors.New("reported: the server publishes no stored reports")
	}
	d, err := r.Deployments(ctx)
	if errors.Is(err, storedreports.ErrNoReport) {
		return nil, errors.New("no deployment report received yet")
	}
	if err != nil {
		return nil, fmt.Errorf("reported: reading the deployment report: %w", err)
	}
	if d.Age > p.maxAge {
		return nil, staleErr("deployment", d.Age, p.maxAge)
	}
	out := make([]hourglass.Allocation, 0, len(d.Deployments))
	for _, x := range d.Deployments {
		labels := make(map[string]string, len(x.Labels))
		for k, v := range x.Labels {
			labels[k] = v
		}
		out = append(out, hourglass.Allocation{ResourceName: x.Setup, Labels: labels})
	}
	return out, nil
}

// Pressure returns the stored value.
func (p pressure) Pressure(ctx context.Context, _ hourglass.Snapshot) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	r := storedreports.Current()
	if r == nil {
		return 0, errors.New("reported: the server publishes no stored reports")
	}
	v, err := r.Pressure(ctx)
	if errors.Is(err, storedreports.ErrNoReport) {
		return 0, errors.New("no pressure report received yet")
	}
	if err != nil {
		return 0, fmt.Errorf("reported: reading the pressure report: %w", err)
	}
	if v.Age > p.maxAge {
		return 0, staleErr("pressure", v.Age, p.maxAge)
	}
	return v.Value, nil
}
