// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"errors"

	"github.com/panset/hourglass/internal/reported"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/storedreports"
)

// NewStoredReader returns the Reader the server publishes for the "reported"
// extensions, over st and clock. Composition-root tests use it too.
func NewStoredReader(st *store.Store, clock reported.Clock) storedreports.Reader {
	return StoredReader{reported.New(st, clock)}
}

// StoredReader adapts the stored-report service to the public read-only
// window that the "reported" extensions use.
type StoredReader struct{ svc *reported.Service }

func (r StoredReader) Deployments(ctx context.Context) (storedreports.Deployments, error) {
	d, err := r.svc.Deployments(ctx)
	if errors.Is(err, reported.ErrNoReport) {
		return storedreports.Deployments{}, storedreports.ErrNoReport
	}
	if err != nil {
		return storedreports.Deployments{}, err
	}
	out := storedreports.Deployments{ObservedAt: d.ObservedAt, Age: d.Age,
		Deployments: make([]storedreports.Deployment, 0, len(d.Deployments))}
	for _, x := range d.Deployments {
		out.Deployments = append(out.Deployments, storedreports.Deployment{Setup: x.Setup, Labels: x.Labels})
	}
	return out, nil
}

func (r StoredReader) Pressure(ctx context.Context) (storedreports.Pressure, error) {
	p, err := r.svc.Pressure(ctx)
	if errors.Is(err, reported.ErrNoReport) {
		return storedreports.Pressure{}, storedreports.ErrNoReport
	}
	if err != nil {
		return storedreports.Pressure{}, err
	}
	return storedreports.Pressure{Value: p.Value, ObservedAt: p.ObservedAt, Age: p.Age}, nil
}
