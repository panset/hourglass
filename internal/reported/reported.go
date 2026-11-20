// SPDX-License-Identifier: MIT

// Package reported owns the stored reports of the integration API: the last
// pressure value and the last deployed set that a customer's own service
// pushed (INT-2, INT-3, INT-6). HTTP handlers write through it and the bundled
// "reported" extensions read through it; it imports no extension package.
//
// Each report is stored whole, with its observed_at, received_at and the name
// of the service account that sent it. max_age is extension config: callers
// pass it in and compare it with the Age of what they read; it is never stored.
package reported

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/panset/hourglass/internal/store"
)

// MaxClockSkew is how far observed_at may lie in the future of the receipt time.
const MaxClockSkew = 2 * time.Minute

// ErrNoReport is returned by a read before the first report.
var ErrNoReport = errors.New("reported: no report received yet")

// StaleError is returned when a report's observed_at is older than the stored
// one. Nothing was changed. The API maps it to 409 stale_report.
type StaleError struct {
	Stored   time.Time // observed_at of the stored report
	Reported time.Time // observed_at of the refused report
}

// Error describes the refusal.
func (e *StaleError) Error() string {
	return fmt.Sprintf("reported: observed_at %s is older than the stored report's %s",
		store.FormatTime(e.Reported), store.FormatTime(e.Stored))
}

// ValidationError is a rejected report. Nothing was stored. The API maps it to
// 400 validation.
type ValidationError struct{ Message string }

// Error returns the message.
func (e *ValidationError) Error() string { return e.Message }

// Clock supplies the time; production passes a wall clock, tests a fake.
type Clock interface{ Now() time.Time }

// Pressure is the stored pressure report with its age.
type Pressure struct {
	Value          float64
	ObservedAt     time.Time
	ReceivedAt     time.Time
	ServiceAccount string
	// Age is the time since ObservedAt, measured at read time (never negative).
	Age time.Duration
}

// Deployment is one setup in a deployment report.
type Deployment struct {
	Setup  string
	Labels map[string]string
}

// Deployments is the stored deployment report with its age. Deployments is
// non-nil and empty when the report said nothing is deployed.
type Deployments struct {
	ObservedAt     time.Time
	ReceivedAt     time.Time
	ServiceAccount string
	Deployments    []Deployment
	// Age is the time since ObservedAt, measured at read time (never negative).
	Age time.Duration
}

// Fresh reports whether a report of this age is still within maxAge.
func Fresh(age, maxAge time.Duration) bool { return age <= maxAge }

// ExpiresAt is observed_at plus maxAge.
func ExpiresAt(observedAt time.Time, maxAge time.Duration) time.Time { return observedAt.Add(maxAge) }

// Service reads and writes the stored reports.
type Service struct {
	st    *store.Store
	clock Clock
}

// New returns a Service over st.
func New(st *store.Store, clock Clock) *Service {
	if st == nil || clock == nil {
		panic("reported: Store and Clock are required")
	}
	return &Service{st: st, clock: clock}
}

// resolve applies the observed_at rules: default to now, truncate to the
// store's millisecond precision, refuse more than MaxClockSkew in the future.
func (s *Service) resolve(observedAt *time.Time) (observed, now time.Time, err error) {
	now = s.clock.Now().UTC().Truncate(time.Millisecond)
	if observedAt == nil {
		return now, now, nil
	}
	observed = observedAt.UTC().Truncate(time.Millisecond)
	if observed.After(now.Add(MaxClockSkew)) {
		return observed, now, &ValidationError{Message: "observed_at is more than 2 minutes in the future"}
	}
	return observed, now, nil
}

// SubmitPressure validates and stores a pressure report from account. value
// must be finite and within 0..1 (INT-1); a nil observedAt means the receipt
// time. A report older than the stored one returns *StaleError and changes
// nothing; an equal observed_at replaces it (INT-3).
func (s *Service) SubmitPressure(ctx context.Context, account string, value float64, observedAt *time.Time) (Pressure, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
		return Pressure{}, &ValidationError{Message: "value must be a number from 0 to 1"}
	}
	observed, now, err := s.resolve(observedAt)
	if err != nil {
		return Pressure{}, err
	}
	rec := store.ReportedPressure{Value: value, ObservedAt: observed, ReceivedAt: now, ServiceAccount: account}
	err = s.st.Tx(ctx, func(tx *store.Tx) error {
		cur, err := tx.GetReportedPressure(ctx)
		switch {
		case errors.Is(err, store.ErrNotFound):
		case err != nil:
			return err
		case observed.Before(cur.ObservedAt):
			return &StaleError{Stored: cur.ObservedAt, Reported: observed}
		}
		return tx.ReplaceReportedPressure(ctx, rec)
	})
	if err != nil {
		return Pressure{}, err
	}
	return pressureFrom(rec, now), nil
}

// SubmitDeployments validates and stores the complete deployed set from
// account, replacing the previous set and header atomically (INT-2). An empty
// or nil list means nothing is deployed and is stored as such. A setup name
// that is empty or repeated is a *ValidationError. Ordering and skew are as
// for SubmitPressure. Matching names to setups is the caller's job.
func (s *Service) SubmitDeployments(ctx context.Context, account string, observedAt *time.Time, deployments []Deployment) (Deployments, error) {
	seen := make(map[string]bool, len(deployments))
	set := make([]store.ReportedDeployment, 0, len(deployments))
	for _, d := range deployments {
		if d.Setup == "" {
			return Deployments{}, &ValidationError{Message: "setup must not be empty"}
		}
		if seen[d.Setup] {
			return Deployments{}, &ValidationError{Message: fmt.Sprintf("duplicate setup %q in one report", d.Setup)}
		}
		seen[d.Setup] = true
		set = append(set, store.ReportedDeployment{SetupName: d.Setup, Labels: d.Labels})
	}
	observed, now, err := s.resolve(observedAt)
	if err != nil {
		return Deployments{}, err
	}
	hdr := store.ReportedHeader{ObservedAt: observed, ReceivedAt: now, ServiceAccount: account}
	err = s.st.Tx(ctx, func(tx *store.Tx) error {
		cur, _, err := tx.GetReportedDeployments(ctx)
		switch {
		case errors.Is(err, store.ErrNotFound):
		case err != nil:
			return err
		case observed.Before(cur.ObservedAt):
			return &StaleError{Stored: cur.ObservedAt, Reported: observed}
		}
		return tx.ReplaceReportedDeployments(ctx, hdr, set)
	})
	if err != nil {
		return Deployments{}, err
	}
	return deploymentsFrom(hdr, set, now), nil
}

// Pressure returns the stored pressure report with its age, or ErrNoReport.
func (s *Service) Pressure(ctx context.Context) (Pressure, error) {
	p, err := s.st.GetReportedPressure(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return Pressure{}, ErrNoReport
	}
	if err != nil {
		return Pressure{}, err
	}
	return pressureFrom(p, s.clock.Now()), nil
}

// Deployments returns the stored deployment report with its age, or
// ErrNoReport. The header and the set come from one snapshot.
func (s *Service) Deployments(ctx context.Context) (Deployments, error) {
	h, set, err := s.st.GetReportedDeployments(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return Deployments{}, ErrNoReport
	}
	if err != nil {
		return Deployments{}, err
	}
	return deploymentsFrom(h, set, s.clock.Now()), nil
}

func age(observed, now time.Time) time.Duration {
	if d := now.Sub(observed); d > 0 {
		return d
	}
	return 0
}

func pressureFrom(p store.ReportedPressure, now time.Time) Pressure {
	return Pressure{Value: p.Value, ObservedAt: p.ObservedAt, ReceivedAt: p.ReceivedAt,
		ServiceAccount: p.ServiceAccount, Age: age(p.ObservedAt, now)}
}

func deploymentsFrom(h store.ReportedHeader, set []store.ReportedDeployment, now time.Time) Deployments {
	out := Deployments{ObservedAt: h.ObservedAt, ReceivedAt: h.ReceivedAt, ServiceAccount: h.ServiceAccount,
		Deployments: make([]Deployment, 0, len(set)), Age: age(h.ObservedAt, now)}
	for _, d := range set {
		labels := d.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		out.Deployments = append(out.Deployments, Deployment{Setup: d.SetupName, Labels: labels})
	}
	return out
}
