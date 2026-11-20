// SPDX-License-Identifier: MIT

// Package storedreports is the read-only window through which the bundled
// "reported" extensions see the reports the server has stored. An extension
// factory receives only its config block and extensions never touch the
// database, so the server publishes a Reader here when it starts and the
// extensions fetch it with Current each time they are called. The package
// imports nothing from the core and nothing from any extension.
//
// A server in the same process as another (tests) shares one Reader: the last
// Publish wins, and the function it returns puts the previous one back.
package storedreports

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrNoReport is returned by a Reader before the first report of that kind.
var ErrNoReport = errors.New("no report received yet")

// Deployment is one setup in the stored deployment report.
type Deployment struct {
	Setup  string
	Labels map[string]string
}

// Deployments is the stored deployment report. Age is measured from
// ObservedAt with the server's clock at the moment of the read.
type Deployments struct {
	ObservedAt  time.Time
	Age         time.Duration
	Deployments []Deployment
}

// Pressure is the stored pressure report, aged as Deployments is.
type Pressure struct {
	Value      float64
	ObservedAt time.Time
	Age        time.Duration
}

// Reader reads the last stored reports. Both methods return ErrNoReport
// (possibly wrapped) before the first report.
type Reader interface {
	Deployments(ctx context.Context) (Deployments, error)
	Pressure(ctx context.Context) (Pressure, error)
}

var (
	mu      sync.RWMutex
	current Reader
)

// Publish makes r the Reader that Current returns and gives back a function
// that restores the previous one. A nil r unpublishes.
func Publish(r Reader) (restore func()) {
	mu.Lock()
	prev := current
	current = r
	mu.Unlock()
	return func() {
		mu.Lock()
		current = prev
		mu.Unlock()
	}
}

// Current returns the published Reader, or nil when no server has published one.
func Current() Reader {
	mu.RLock()
	defer mu.RUnlock()
	return current
}
