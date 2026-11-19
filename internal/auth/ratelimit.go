// SPDX-License-Identifier: MIT

package auth

import (
	"sync"
	"time"
)

// Local admin sign-in limit (AUTH-7): 5 failures per 15 minutes.
const (
	maxFailures   = 5
	failureWindow = 15 * time.Minute
)

// failureLimiter is an in-memory sliding-window failure counter. A success
// does not clear failures already inside the window.
type failureLimiter struct {
	mu       sync.Mutex
	failures []time.Time
}

// prune drops failures that left the window. Caller holds mu.
func (l *failureLimiter) prune(now time.Time) {
	cut := now.Add(-failureWindow)
	i := 0
	for i < len(l.failures) && !l.failures[i].After(cut) {
		i++
	}
	l.failures = l.failures[i:]
}

// limited reports whether another attempt is refused now and, if so, how
// long until the oldest failure leaves the window.
func (l *failureLimiter) limited(now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)
	if len(l.failures) < maxFailures {
		return false, 0
	}
	return true, l.failures[len(l.failures)-maxFailures].Add(failureWindow).Sub(now)
}

// fail records one failure.
func (l *failureLimiter) fail(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)
	l.failures = append(l.failures, now)
}
