// SPDX-License-Identifier: MIT

package hourglass

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"time"

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
	// Clock supplies the current time to every component that needs one (the
	// collector, the ledger, holds, sessions, event records). Nil means the
	// wall clock.
	Clock interface{ Now() time.Time }
}

// Run starts the server and blocks until ctx is cancelled, then shuts down
// in order (HTTP, collector, events, store) within 10 seconds. It returns
// nil after a clean shutdown.
//
// The implementation lives in internal/app, which imports this package for
// the extension types, so it is reached through internal/app/runhook. A main
// links it by importing github.com/panset/hourglass/pkg/hourglass/server for
// its side effect (cmd/hourglassd does). Without that import Run returns an
// error saying so.
func Run(ctx context.Context, opts Options) error {
	err := runhook.Call(ctx, runhook.Options{
		ConfigPath:            opts.ConfigPath,
		DefaultPressureSource: opts.DefaultPressureSource,
		DefaultMeteringPolicy: opts.DefaultMeteringPolicy,
		Args:                  opts.Args,
		Stdout:                opts.Stdout,
		Stderr:                opts.Stderr,
		UI:                    opts.UI,
		Clock:                 opts.Clock,
	})
	if errors.Is(err, runhook.ErrNoImpl) {
		return fmt.Errorf("%w: add the import _ %q to your main package", err, serverImportPath)
	}
	return err
}

// serverImportPath is the package a third-party main imports to link Run.
const serverImportPath = "github.com/panset/hourglass/pkg/hourglass/server"
