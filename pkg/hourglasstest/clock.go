// SPDX-License-Identifier: MIT

package hourglasstest

import (
	"sync"
	"time"
)

// FakeClock is a settable clock. Core code can accept any
// interface{ Now() time.Time } and be handed a *FakeClock in tests.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

// DefaultTime is the instant a clock made by NewFakeClock(time.Time{}) starts at.
var DefaultTime = time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)

// NewFakeClock returns a clock reading start, or DefaultTime when start is zero.
func NewFakeClock(start time.Time) *FakeClock {
	if start.IsZero() {
		start = DefaultTime
	}
	return &FakeClock{now: start}
}

// Now returns the current fake time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Set moves the clock to t (backwards is allowed).
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	c.now = t
	c.mu.Unlock()
}

// Advance moves the clock forward by d and returns the new time.
func (c *FakeClock) Advance(d time.Duration) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	return c.now
}
