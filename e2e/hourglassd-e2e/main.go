// SPDX-License-Identifier: MIT

// Command hourglassd-e2e is the end-to-end test build of hourglassd: the same
// extension imports, flags and subcommands as cmd/hourglassd, plus a clock
// whose offset a test can change through a small control endpoint. It is test
// infrastructure only; nothing under cmd/, internal/ or pkg/ imports it.
//
// HOURGLASS_E2E_CONTROL is the control listen address (for example :8091).
// Unset, no control endpoint is served and the clock is the wall clock.
//
//	GET /clock                    {"offset":"0s","now":"..."}
//	PUT /clock {"offset":"720h"}  from then on the server's time is the wall clock plus 720h
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

// Same defaults as cmd/hourglassd.
const (
	defaultPressureSource = "inventory"
	defaultMeteringPolicy = "default"
)

// controlEnv names the control listen address.
const controlEnv = "HOURGLASS_E2E_CONTROL"

var subcommands = map[string]func(args []string) int{
	"check-provider": checkProvider,
	"version": func([]string) int {
		fmt.Println("hourglassd-e2e", version.Version)
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

	clock := newOffsetClock()
	if addr := os.Getenv(controlEnv); addr != "" {
		shutdown, err := serveControl(addr, clock)
		if err != nil {
			fmt.Fprintln(os.Stderr, "hourglassd-e2e:", err)
			return 1
		}
		defer shutdown()
	}
	err := hourglass.Run(ctx, hourglass.Options{
		DefaultPressureSource: defaultPressureSource,
		DefaultMeteringPolicy: defaultMeteringPolicy,
		Args:                  args,
		Stdout:                os.Stdout,
		Stderr:                os.Stderr,
		UI:                    embeddedUI(),
		Clock:                 clock,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "hourglassd-e2e:", err)
		return 1
	}
	return 0
}

// serveControl starts the control endpoint and returns a function that stops it.
func serveControl(addr string, c *offsetClock) (func(), error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("control listen: %w", err)
	}
	srv := &http.Server{Handler: controlHandler(c), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "hourglassd-e2e: control server:", err)
		}
	}()
	fmt.Fprintf(os.Stdout, "{\"msg\":\"e2e clock control listening\",\"addr\":%q}\n", ln.Addr().String())
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}, nil
}

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
		fmt.Fprintln(os.Stderr, "hourglassd-e2e check-provider:", err)
		return 1
	}
	return 0
}

// embeddedUI is the UI built into this binary (the "ui" build tag), or nil.
func embeddedUI() fs.FS {
	if fsys, ok := uiserve.Embedded(); ok {
		return fsys
	}
	return nil
}
