// SPDX-License-Identifier: MIT

package hourglass

import (
	"context"
	"encoding/json"
	"time"
)

// Setup is what every extension sees of a setup. The core never
// interprets Labels or Details.
type Setup struct {
	Name    string
	Labels  map[string]string
	Details json.RawMessage
}

// ResourceProvider reports what is currently deployed. Optional:
// with none configured, every other extension receives no allocations.
type ResourceProvider interface {
	Name() string
	List(ctx context.Context) ([]Allocation, error)
}

// Allocation is one deployed resource as reported by a ResourceProvider.
type Allocation struct {
	ResourceName string            // belongs to the setup with exactly this name
	Labels       map[string]string // provider-specific, shown to admins
}

// PressureSource reports how contended the lab is, from 0.0 to 1.0.
// An error or a value outside 0 to 1 makes the poll fail.
type PressureSource interface {
	Name() string
	Pressure(ctx context.Context, s Snapshot) (float64, error)
}

// MeteringPolicy decides, per hold and per poll, whether to charge.
type MeteringPolicy interface {
	Name() string
	Decide(ctx context.Context, h HoldState, s Snapshot) (Decision, error)
}

// Decision is a MeteringPolicy's verdict for one hold in one poll.
type Decision struct {
	Meter  bool
	Weight float64 // 0 means use the weight from the rate rules
	Reason string  // shown to the user, e.g. "2 h since acquire"
}

// Hook receives every event. An error returned for a vetoable event
// (hold.acquiring) aborts the action; other errors are logged.
// Each call has a 30-second timeout.
type Hook interface {
	Name() string
	Handle(ctx context.Context, e Event) error
}

// Event is delivered to every configured hook. Type is one of the Event*
// constants.
type Event struct {
	Type  string // e.g. "hold.acquired"
	Time  time.Time
	User  *User  // nil when not about a user
	Setup *Setup // nil when not about a setup
	Hold  *Hold  // nil when not about a hold
	Data  map[string]any
}

// User mirrors a row of the users table together with its roles.
type User struct {
	ID       int64
	Username string
	Email    string
	Name     string
	Roles    []string // "developer", "admin"; empty means unassigned
}

// Hold mirrors a row of the holds table. Nil times have not happened yet.
type Hold struct {
	ID                int64
	SetupName         string
	UserID            int64
	AcquiredAt        time.Time
	MeteringStartedAt *time.Time
	DeployedAt        *time.Time
	ReleasedAt        *time.Time
}

// SetupState is an enabled setup as the core resolves it for one poll.
type SetupState struct {
	Setup
	// Weight is the weight resolved from the rate rules, or the default
	// weight when no rule matches.
	Weight float64
}

// Snapshot is the read-only view of the lab for one poll. Extensions need
// no database access.
type Snapshot struct {
	// Time is when the poll started.
	Time time.Time
	// Setups lists the enabled setups.
	Setups []SetupState
	// Holds lists the active (unreleased) holds.
	Holds []Hold
	// Allocations is what the provider reported for this poll; empty when
	// ProviderConfigured is false.
	Allocations []Allocation
	// ProviderConfigured reports whether a ResourceProvider is configured.
	ProviderConfigured bool
}

// HoldState is one active hold with everything a MeteringPolicy needs.
type HoldState struct {
	Hold  Hold
	Setup SetupState
	User  User
	// Allocation is the provider's allocation for the setup, or nil when
	// the provider does not report it or none is configured.
	Allocation *Allocation
}

// SetupByName returns the enabled setup with the given name.
func (s Snapshot) SetupByName(name string) (SetupState, bool) {
	for _, st := range s.Setups {
		if st.Name == name {
			return st, true
		}
	}
	return SetupState{}, false
}

// AllocationFor returns the allocation whose ResourceName equals the name
// of the named setup (D40). It reports false when the setup is not enabled
// or the provider does not report it.
func (s Snapshot) AllocationFor(setupName string) (Allocation, bool) {
	if _, ok := s.SetupByName(setupName); !ok {
		return Allocation{}, false
	}
	for _, a := range s.Allocations {
		if a.ResourceName == setupName {
			return a, true
		}
	}
	return Allocation{}, false
}
