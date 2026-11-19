// SPDX-License-Identifier: MIT

package hourglass

import (
	"context"
	"io"
	"io/fs"

	"github.com/panset/hourglass/internal/app/runhook"
)

// Options configure Run.
type Options struct {
	// ConfigPath is the YAML config file. The -config flag in Args, then the
	// HOURGLASS_CONFIG variable, are consulted when it is empty (the flag
	// wins over this field). No path at all means defaults plus environment.
	ConfigPath string
	// DefaultPressureSource and DefaultMeteringPolicy name the extensions
	// used when the config does not name one. The core names no concrete
	// extension; the caller of Run supplies these.
	DefaultPressureSource string
	DefaultMeteringPolicy string
	// Args are the command-line arguments after the program name (flags such
	// as -config). Subcommands are handled by the caller before Run.
	Args []string
	// Stdout receives the JSON log; Stderr receives flag and fatal errors.
	// Nil means os.Stdout and os.Stderr.
	Stdout, Stderr io.Writer
	// UI is the static file tree served at / in bundled mode. Nil means the
	// binary serves no UI (API-only).
	UI fs.FS
}

// Run starts the server and blocks until ctx is cancelled, then shuts down
// in order (HTTP, collector, events, store) within 10 seconds. It returns
// nil after a clean shutdown.
//
// Run is implemented by internal/app, which imports this package for the
// extension types, so the implementation is reached through
// internal/app/runhook: a binary must link internal/app in (the bundled
// cmd/hourglassd does, with a blank import). A binary that does not gets
// runhook.ErrNoImpl.
func Run(ctx context.Context, opts Options) error {
	return runhook.Call(ctx, runhook.Options{
		ConfigPath:            opts.ConfigPath,
		DefaultPressureSource: opts.DefaultPressureSource,
		DefaultMeteringPolicy: opts.DefaultMeteringPolicy,
		Args:                  opts.Args,
		Stdout:                opts.Stdout,
		Stderr:                opts.Stderr,
		UI:                    opts.UI,
	})
}
