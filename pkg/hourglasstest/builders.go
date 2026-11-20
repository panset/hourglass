// SPDX-License-Identifier: MIT

package hourglasstest

import (
	"encoding/json"
	"time"

	"github.com/panset/hourglass/pkg/hourglass"
)

// SnapshotBuilder assembles a hourglass.Snapshot. Create one with NewSnapshot.
type SnapshotBuilder struct {
	snap   hourglass.Snapshot
	nextID int64
}

// NewSnapshot starts a snapshot at DefaultTime with ProviderConfigured true.
func NewSnapshot() *SnapshotBuilder {
	return &SnapshotBuilder{snap: hourglass.Snapshot{Time: DefaultTime, ProviderConfigured: true}, nextID: 1}
}

// At sets the snapshot time.
func (b *SnapshotBuilder) At(t time.Time) *SnapshotBuilder {
	b.snap.Time = t
	return b
}

// ProviderConfigured sets whether a resource provider is configured.
func (b *SnapshotBuilder) ProviderConfigured(v bool) *SnapshotBuilder {
	b.snap.ProviderConfigured = v
	return b
}

// SetupOption customises a setup added with SnapshotBuilder.Setup.
type SetupOption func(*hourglass.SetupState)

// Label sets one label on the setup.
func Label(key, value string) SetupOption {
	return func(s *hourglass.SetupState) {
		if s.Labels == nil {
			s.Labels = map[string]string{}
		}
		s.Labels[key] = value
	}
}

// Weight sets the setup's resolved weight.
func Weight(w float64) SetupOption { return func(s *hourglass.SetupState) { s.Weight = w } }

// Details sets the setup's raw details JSON.
func Details(raw string) SetupOption {
	return func(s *hourglass.SetupState) { s.Details = json.RawMessage(raw) }
}

// Setup adds a setup. Weight defaults to 1.
func (b *SnapshotBuilder) Setup(name string, opts ...SetupOption) *SnapshotBuilder {
	s := hourglass.SetupState{Setup: hourglass.Setup{Name: name}, Weight: 1}
	for _, o := range opts {
		o(&s)
	}
	b.snap.Setups = append(b.snap.Setups, s)
	return b
}

// HoldOption customises a hold added with SnapshotBuilder.Hold.
type HoldOption func(*hourglass.Hold)

// Acquired sets the hold's acquisition time.
func Acquired(t time.Time) HoldOption { return func(h *hourglass.Hold) { h.AcquiredAt = t } }

// Metering marks metering as started at t.
func Metering(t time.Time) HoldOption { return func(h *hourglass.Hold) { h.MeteringStartedAt = &t } }

// Deployed marks the hold as deployed at t.
func Deployed(t time.Time) HoldOption { return func(h *hourglass.Hold) { h.DeployedAt = &t } }

// Released marks the hold as released at t.
func Released(t time.Time) HoldOption { return func(h *hourglass.Hold) { h.ReleasedAt = &t } }

// Hold adds a hold on setupName for userID. IDs count up from 1; AcquiredAt
// defaults to the snapshot time at the moment of the call.
func (b *SnapshotBuilder) Hold(setupName string, userID int64, opts ...HoldOption) *SnapshotBuilder {
	h := hourglass.Hold{ID: b.nextID, SetupName: setupName, UserID: userID, AcquiredAt: b.snap.Time}
	b.nextID++
	for _, o := range opts {
		o(&h)
	}
	b.snap.Holds = append(b.snap.Holds, h)
	return b
}

// Allocation adds a reported allocation; labels are key, value pairs.
func (b *SnapshotBuilder) Allocation(resourceName string, labelPairs ...string) *SnapshotBuilder {
	a := hourglass.Allocation{ResourceName: resourceName}
	for i := 0; i+1 < len(labelPairs); i += 2 {
		if a.Labels == nil {
			a.Labels = map[string]string{}
		}
		a.Labels[labelPairs[i]] = labelPairs[i+1]
	}
	b.snap.Allocations = append(b.snap.Allocations, a)
	return b
}

// Build returns an independent copy of the snapshot.
func (b *SnapshotBuilder) Build() hourglass.Snapshot { return cloneSnapshot(b.snap) }

// NewHoldState assembles the HoldState a policy sees for the hold at index
// holdIndex of s.Holds, joining the setup and the matching allocation. The
// user is a developer with the given id. It panics when holdIndex is out of
// range or the hold's setup is not in s.
func NewHoldState(s hourglass.Snapshot, holdIndex int, user hourglass.User) hourglass.HoldState {
	h := s.Holds[holdIndex]
	st, ok := s.SetupByName(h.SetupName)
	if !ok {
		panic("hourglasstest: hold references setup not in snapshot: " + h.SetupName)
	}
	hs := hourglass.HoldState{Hold: h, Setup: st, User: user}
	if a, ok := s.AllocationFor(h.SetupName); ok {
		hs.Allocation = &a
	}
	return cloneHoldState(hs)
}

// --- deep copies, used by fakes and by the "does not mutate" checks ---

func cloneLabels(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}

func cloneSetupState(s hourglass.SetupState) hourglass.SetupState {
	s.Labels = cloneLabels(s.Labels)
	if s.Details != nil {
		s.Details = append(json.RawMessage(nil), s.Details...)
	}
	return s
}

func cloneHold(h hourglass.Hold) hourglass.Hold {
	h.MeteringStartedAt = cloneTime(h.MeteringStartedAt)
	h.DeployedAt = cloneTime(h.DeployedAt)
	h.ReleasedAt = cloneTime(h.ReleasedAt)
	return h
}

func cloneAllocations(in []hourglass.Allocation) []hourglass.Allocation {
	if in == nil {
		return nil
	}
	out := make([]hourglass.Allocation, len(in))
	for i, a := range in {
		out[i] = hourglass.Allocation{ResourceName: a.ResourceName, Labels: cloneLabels(a.Labels)}
	}
	return out
}

func cloneSnapshot(s hourglass.Snapshot) hourglass.Snapshot {
	out := s
	if s.Setups != nil {
		out.Setups = make([]hourglass.SetupState, len(s.Setups))
		for i, st := range s.Setups {
			out.Setups[i] = cloneSetupState(st)
		}
	}
	if s.Holds != nil {
		out.Holds = make([]hourglass.Hold, len(s.Holds))
		for i, h := range s.Holds {
			out.Holds[i] = cloneHold(h)
		}
	}
	out.Allocations = cloneAllocations(s.Allocations)
	return out
}

func cloneHoldState(h hourglass.HoldState) hourglass.HoldState {
	out := h
	out.Hold = cloneHold(h.Hold)
	out.Setup = cloneSetupState(h.Setup)
	if h.User.Roles != nil {
		out.User.Roles = append([]string(nil), h.User.Roles...)
	}
	if h.Allocation != nil {
		a := cloneAllocations([]hourglass.Allocation{*h.Allocation})[0]
		out.Allocation = &a
	}
	return out
}
