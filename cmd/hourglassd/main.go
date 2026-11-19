// SPDX-License-Identifier: MIT

// Command hourglassd is the Hourglass server. It links the bundled
// extensions by blank import (EXT-3, EXT-5: a third-party main does the same
// with its own packages) and hands control to hourglass.Run.
//
//	hourglassd [-config file]     run the server
//	hourglassd version            print the version
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/panset/hourglass/hooks/webhook"
	_ "github.com/panset/hourglass/internal/app" // implements hourglass.Run
	"github.com/panset/hourglass/internal/version"
	"github.com/panset/hourglass/pkg/hourglass"
	_ "github.com/panset/hourglass/policies/default"
	_ "github.com/panset/hourglass/pressure/inventory"
	_ "github.com/panset/hourglass/providers/static"
)

// Defaults used when the config does not name a pressure source or policy.
const (
	defaultPressureSource = "inventory"
	defaultMeteringPolicy = "default"
)

// subcommands are run instead of the server when named as the first
// argument. A later task adds check-provider here.
var subcommands = map[string]func(args []string) int{
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
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "hourglassd:", err)
		return 1
	}
	return 0
}
