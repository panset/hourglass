// SPDX-License-Identifier: MIT

package hourglass

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ProviderFactory builds a ResourceProvider from its opaque config block.
// The block is JSON whatever the config file's format, and is nil or empty
// when the config file gives none.
type ProviderFactory func(config json.RawMessage) (ResourceProvider, error)

// PressureFactory builds a PressureSource from its opaque config block.
type PressureFactory func(config json.RawMessage) (PressureSource, error)

// PolicyFactory builds a MeteringPolicy from its opaque config block.
type PolicyFactory func(config json.RawMessage) (MeteringPolicy, error)

// HookFactory builds a Hook from its opaque config block.
type HookFactory func(config json.RawMessage) (Hook, error)

// registry is a named set of factories of one kind, safe for concurrent use.
type registry[F any] struct {
	kind string // used in panic and error messages, e.g. "provider"
	mu   sync.RWMutex
	m    map[string]F
}

func (r *registry[F]) register(name string, f F, isNil bool) {
	if name == "" {
		panic("hourglass: Register " + r.kind + " with empty name")
	}
	if isNil {
		panic("hourglass: Register " + r.kind + " " + name + " with nil factory")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.m[name]; dup {
		panic("hourglass: Register called twice for " + r.kind + " " + name)
	}
	if r.m == nil {
		r.m = make(map[string]F)
	}
	r.m[name] = f
}

func (r *registry[F]) lookup(name string) (F, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.m[name]
	if !ok {
		names := r.namesLocked()
		list := "none"
		if len(names) > 0 {
			list = strings.Join(names, ", ")
		}
		return f, fmt.Errorf("hourglass: unknown %s %q (registered: %s)", r.kind, name, list)
	}
	return f, nil
}

func (r *registry[F]) names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.namesLocked()
}

func (r *registry[F]) namesLocked() []string {
	names := make([]string, 0, len(r.m))
	for n := range r.m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (r *registry[F]) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m = nil
}

var (
	providers = &registry[ProviderFactory]{kind: "provider"}
	pressures = &registry[PressureFactory]{kind: "pressure source"}
	policies  = &registry[PolicyFactory]{kind: "metering policy"}
	hooks     = &registry[HookFactory]{kind: "hook"}
)

// resetRegistries clears every registry. For tests only.
func resetRegistries() {
	providers.reset()
	pressures.reset()
	policies.reset()
	hooks.reset()
}

// RegisterProvider makes a ResourceProvider available under name. It is
// meant to be called from an init function and panics if name is empty, f
// is nil, or name is already registered.
func RegisterProvider(name string, f ProviderFactory) {
	providers.register(name, f, f == nil)
}

// RegisterPressureSource makes a PressureSource available under name. It
// panics if name is empty, f is nil, or name is already registered.
func RegisterPressureSource(name string, f PressureFactory) {
	pressures.register(name, f, f == nil)
}

// RegisterMeteringPolicy makes a MeteringPolicy available under name. It
// panics if name is empty, f is nil, or name is already registered.
func RegisterMeteringPolicy(name string, f PolicyFactory) {
	policies.register(name, f, f == nil)
}

// RegisterHook makes a Hook available under name. It panics if name is
// empty, f is nil, or name is already registered.
func RegisterHook(name string, f HookFactory) {
	hooks.register(name, f, f == nil)
}

// NewProvider builds the provider registered under name with config. An
// unknown name returns an error listing the registered names, sorted
// (EXT-2). Errors from the factory are returned unchanged.
func NewProvider(name string, config json.RawMessage) (ResourceProvider, error) {
	f, err := providers.lookup(name)
	if err != nil {
		return nil, err
	}
	return f(config)
}

// NewPressureSource builds the pressure source registered under name, as
// NewProvider does.
func NewPressureSource(name string, config json.RawMessage) (PressureSource, error) {
	f, err := pressures.lookup(name)
	if err != nil {
		return nil, err
	}
	return f(config)
}

// NewMeteringPolicy builds the metering policy registered under name, as
// NewProvider does.
func NewMeteringPolicy(name string, config json.RawMessage) (MeteringPolicy, error) {
	f, err := policies.lookup(name)
	if err != nil {
		return nil, err
	}
	return f(config)
}

// NewHook builds the hook registered under name, as NewProvider does.
func NewHook(name string, config json.RawMessage) (Hook, error) {
	f, err := hooks.lookup(name)
	if err != nil {
		return nil, err
	}
	return f(config)
}

// Providers returns the registered provider names, sorted.
func Providers() []string { return providers.names() }

// PressureSources returns the registered pressure source names, sorted.
func PressureSources() []string { return pressures.names() }

// MeteringPolicies returns the registered metering policy names, sorted.
func MeteringPolicies() []string { return policies.names() }

// Hooks returns the registered hook names, sorted.
func Hooks() []string { return hooks.names() }
