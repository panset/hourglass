// SPDX-License-Identifier: MIT

// Command labd is a complete Hourglass server for a lab of GPU rigs, built
// the way a third party would build one: it imports the public packages of
// the framework, adds its own extensions, and never touches internal/.
//
// A custom binary needs three kinds of import:
//
//  1. pkg/hourglass, for Run and Options.
//  2. pkg/hourglass/server, for its side effect only. It links the
//     implementation behind hourglass.Run; without it Run returns an
//     error that says so.
//  3. Blank imports of every extension the config file may name. Each
//     registers itself by name in init(), like a database/sql driver.
//
// Usage:
//
//	labd [-config file]
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/panset/hourglass/pkg/hourglass"
	_ "github.com/panset/hourglass/pkg/hourglass/server" // links hourglass.Run

	// This lab's own extensions, one of each kind.
	_ "github.com/panset/hourglass/examples/custom/busiestsite"
	_ "github.com/panset/hourglass/examples/custom/businesshours"
	_ "github.com/panset/hourglass/examples/custom/filedir"
	_ "github.com/panset/hourglass/examples/custom/logfile"

	// Bundled extensions this binary also offers. The config may name any
	// of them; nothing is enabled just by being linked in.
	_ "github.com/panset/hourglass/hooks/webhook"
	_ "github.com/panset/hourglass/policies/default"
	_ "github.com/panset/hourglass/pressure/inventory"
	_ "github.com/panset/hourglass/providers/static"
)

// Used when the config file names no pressure source or metering policy.
// The core names no extension of its own, so the caller must supply these.
const (
	defaultPressureSource = "inventory"
	defaultMeteringPolicy = "default"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "labd:", err)
		stop()
		os.Exit(1)
	}
}

// run is main without the process plumbing, so a test can start the same
// wiring in-process. It blocks until ctx is cancelled.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return hourglass.Run(ctx, hourglass.Options{
		DefaultPressureSource: defaultPressureSource,
		DefaultMeteringPolicy: defaultMeteringPolicy,
		Args:                  args, // -config is read from here
		Stdout:                stdout,
		Stderr:                stderr,
		// UI is left nil: labd is API-only. A team that wants the
		// dashboard serves the UI container beside it (split mode).
	})
}
