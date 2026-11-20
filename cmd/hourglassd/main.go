// SPDX-License-Identifier: MIT

// Command hourglassd is the Hourglass server. It links the bundled
// extensions by blank import (EXT-3, EXT-5: a third-party main does the same
// with its own packages) and hands control to hourglass.Run.
//
//	hourglassd [-config file]     run the server
//	hourglassd check-provider [-config file]
//	                              list and report what the provider and pressure source see; writes nothing
//	hourglassd version            print the version
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/panset/hourglass/hooks/webhook"
	"github.com/panset/hourglass/internal/app"
	"github.com/panset/hourglass/internal/uiserve"
	"github.com/panset/hourglass/internal/version"
	"github.com/panset/hourglass/pkg/hourglass"
	_ "github.com/panset/hourglass/pkg/hourglass/server" // implements hourglass.Run (EXT-5)
	_ "github.com/panset/hourglass/policies/default"
	_ "github.com/panset/hourglass/pressure/inventory"
	_ "github.com/panset/hourglass/providers/reported"
	_ "github.com/panset/hourglass/providers/static"
	_ "github.com/panset/hourglass/providers/vsphere"
)

// Defaults used when the config does not name a pressure source or policy.
const (
	defaultPressureSource = "inventory"
	defaultMeteringPolicy = "default"
)

// subcommands are run instead of the server when named as the first
// argument.
var subcommands = map[string]func(args []string) int{
	"check-provider": checkProvider,
	"version": func([]string) int {
		fmt.Println("hourglassd", version.Version)
		return 0
	},
}

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) > 0 {
		if cmd, ok := subcommands[args[0]]; ok {
			return cmd(args[1:])
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := hourglass.Run(ctx, hourglass.Options{
		DefaultPressureSource: defaultPressureSource,
		DefaultMeteringPolicy: defaultMeteringPolicy,
		Args:                  args,
		Stdout:                os.Stdout,
		Stderr:                os.Stderr,
		UI:                    embeddedUI(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "hourglassd:", err)
		return 1
	}
	return 0
}

// checkProvider is `hourglassd check-provider` (EXT-3 first-run check).
func checkProvider(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := app.CheckProvider(ctx, app.CheckOptions{
		Args:                  args,
		DefaultPressureSource: defaultPressureSource,
		Stdout:                os.Stdout,
		Stderr:                os.Stderr,
	})
	if errors.Is(err, flag.ErrHelp) {
		return 0 // -h: the usage was printed
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "hourglassd check-provider:", err)
		return 1
	}
	return 0
}

// embeddedUI is the UI built into this binary (the "ui" build tag), or nil.
// hourglass.Run serves it only when ui.serve is true and logs which case
// applies.
func embeddedUI() fs.FS {
	if fsys, ok := uiserve.Embedded(); ok {
		return fsys
	}
	return nil
}
