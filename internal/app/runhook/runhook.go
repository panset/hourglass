// SPDX-License-Identifier: MIT

// Package runhook breaks the import cycle between pkg/hourglass (the public
// package that offers Run) and internal/app (which implements it and imports
// pkg/hourglass for the extension types). It imports nothing from the module:
// pkg/hourglass calls Impl, internal/app sets it from its init function.
package runhook

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"time"
)

// Options mirrors hourglass.Options field for field.
type Options struct {
	ConfigPath            string
	DefaultPressureSource string
	DefaultMeteringPolicy string
	Args                  []string
	Stdout, Stderr        io.Writer
	UI                    fs.FS
	Clock                 interface{ Now() time.Time }
}

// ErrNoImpl is returned by Call when internal/app was not linked in.
var ErrNoImpl = errors.New("hourglass: the server implementation is not linked into this binary")

// Impl is set by internal/app's init.
var Impl func(ctx context.Context, opts Options) error

// Call runs Impl, or fails with ErrNoImpl.
func Call(ctx context.Context, opts Options) error {
	if Impl == nil {
		return ErrNoImpl
	}
	return Impl(ctx, opts)
}
