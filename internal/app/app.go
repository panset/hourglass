// SPDX-License-Identifier: MIT

// Package app wires the process together: configuration, logging, the store,
// the extensions, the events dispatcher, the collector, the HTTP server and
// the optional metrics listener, and shuts them down in order. pkg/hourglass
// offers it as hourglass.Run through internal/app/runhook.
package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	iofs "io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/panset/hourglass/internal/api"
	"github.com/panset/hourglass/internal/app/runhook"
	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/collector"
	"github.com/panset/hourglass/internal/config"
	"github.com/panset/hourglass/internal/events"
	"github.com/panset/hourglass/internal/holds"
	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/internal/uiserve"
	"github.com/panset/hourglass/pkg/hourglass"
)

func init() { runhook.Impl = Run }

// ShutdownTimeout bounds the whole orderly shutdown.
const ShutdownTimeout = 10 * time.Second

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// extensions are the built extension instances.
type extensions struct {
	provider hourglass.ResourceProvider // nil when none is configured
	pressure hourglass.PressureSource
	policy   hourglass.MeteringPolicy
	hooks    []hourglass.Hook
}

// buildExtensions creates the configured extensions through the registry. An
// unknown name fails with the registry's error, which lists the registered
// names (EXT-2). Config blocks are handed over untouched.
func buildExtensions(cfg config.Config, defPressure, defPolicy string) (extensions, error) {
	var x extensions
	var err error
	if cfg.HasProvider() {
		if x.provider, err = hourglass.NewProvider(cfg.Provider.Type, cfg.Provider.Config); err != nil {
			return x, fmt.Errorf("provider: %w", err)
		}
	}
	pt := cfg.Pressure.Type
	if pt == "" {
		pt = defPressure
	}
	if x.pressure, err = hourglass.NewPressureSource(pt, cfg.Pressure.Config); err != nil {
		return x, fmt.Errorf("pressure: %w", err)
	}
	mt := cfg.Metering.Type
	if mt == "" {
		mt = defPolicy
	}
	if x.policy, err = hourglass.NewMeteringPolicy(mt, cfg.Metering.Config); err != nil {
		return x, fmt.Errorf("metering: %w", err)
	}
	for i, h := range cfg.Hooks {
		hook, err := hourglass.NewHook(h.Type, h.Config)
		if err != nil {
			return x, fmt.Errorf("hooks[%d]: %w", i, err)
		}
		x.hooks = append(x.hooks, hook)
	}
	return x, nil
}

func (x extensions) runtime() api.Runtime {
	rt := api.Runtime{PressureSource: x.pressure.Name(), MeteringPolicy: x.policy.Name(), Hooks: []string{}}
	if x.provider != nil {
		rt.Provider = x.provider.Name()
	}
	for _, h := range x.hooks {
		rt.Hooks = append(rt.Hooks, h.Name())
	}
	return rt
}

// pollView adapts collector.PollView to holds.PollView (the collector's
// OK is a field, the holds interface wants a method).
type pollView struct{ v collector.PollView }

func (p pollView) OK() bool                  { return p.v.OK }
func (p pollView) Reported(name string) bool { return p.v.Reported(name) }
func holdsPoll(c *collector.Collector) func() holds.PollView {
	return func() holds.PollView {
		v := c.LastPoll()
		if !v.OK {
			return nil
		}
		return pollView{v}
	}
}

// countingEmitter forwards events and counts poll.failed for the metrics.
type countingEmitter struct {
	next interface{ Emit(hourglass.Event) }
	m    *metrics
}

func (c countingEmitter) Emit(e hourglass.Event) {
	if e.Type == hourglass.EventPollFailed {
		c.m.pollFailures.Add(1)
	}
	c.next.Emit(e)
}

// Run is hourglass.Run. See pkg/hourglass.Options for the contract.
func Run(ctx context.Context, opts runhook.Options) error {
	stdout, stderr := opts.Stdout, opts.Stderr
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	path, err := configPath(opts, stderr)
	if err != nil {
		return err
	}
	cfg, err := config.Load(path, os.Getenv)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(stdout, nil))
	loc, err := cfg.Location()
	if err != nil {
		return err
	}
	// Extensions first: a misconfigured name must not leave a new database behind.
	ext, err := buildExtensions(cfg, opts.DefaultPressureSource, opts.DefaultMeteringPolicy)
	if err != nil {
		return err
	}
	if cfg.OIDC.Issuer == "" {
		log.Info("OIDC is not configured; only the local admin can sign in")
	}
	if !cfg.HasProvider() {
		log.Info("no resource provider configured; time-based metering with inventory pressure (EXT-4)")
	}
	log.Info("starting", "config", cfg.Redacted(), "config_file", path)

	st, err := store.Open(cfg.Database)
	if err != nil {
		return err
	}
	closeStore := func() {
		if err := st.Close(); err != nil {
			log.Error("closing the database failed", "err", err)
		}
	}
	if _, err := auth.EnsureLocalAdmin(ctx, st, cfg.AdminPassword, log); err != nil {
		closeStore()
		return err
	}
	if applied, err := ImportSeed(ctx, st, cfg.Seed, time.Now()); err != nil {
		closeStore()
		return err
	} else if applied {
		log.Info("seed file imported", "seed", cfg.Seed)
	}

	m := newMetrics(time.Now())
	clock := systemClock{}
	// The events dispatcher outlives ctx's cancellation until shutdown closes it.
	disp := events.New(ext.hooks, st, events.WithLogger(log))
	disp.Start(context.WithoutCancel(ctx))
	led := ledger.NewService(st, disp, clock, loc)
	coll := collector.New(collector.Deps{
		Store: st, Provider: ext.provider, Pressure: ext.pressure, Policy: ext.policy,
		Events: countingEmitter{next: disp, m: m}, Ledger: led, Clock: clock, Logger: log,
	})
	holdsSvc := holds.NewService(holds.Deps{
		Store: st, Events: disp, Poll: holdsPoll(coll), ProviderConfigured: ext.provider != nil,
		Clock: clock, Logger: log,
	})
	authSvc, err := auth.New(auth.Config{
		OIDC: auth.OIDCConfig{IssuerURL: cfg.OIDC.Issuer, ClientID: cfg.OIDC.ClientID,
			ClientSecret: cfg.OIDC.ClientSecret, RedirectURL: cfg.OIDC.RedirectURL},
		SessionKey: cfg.SessionKey, AllowedOrigins: cfg.CORS.AllowedOrigins,
	}, st, disp, log)
	if err != nil {
		_ = disp.Close(ctx)
		closeStore()
		return err
	}
	handler := api.New(api.Deps{
		Store: st, Auth: authSvc, Holds: holdsSvc, Ledger: led, Events: disp, Collector: coll,
		Clock: clock, Location: loc, Logger: log, Runtime: ext.runtime(),
		Fallback: uiFallback(opts.UI, cfg.UI.Serve, log),
	})

	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		_ = disp.Close(ctx)
		closeStore()
		return fmt.Errorf("listen: %w", err)
	}
	var msrv *http.Server
	var mln net.Listener
	if cfg.Metrics.Listen != "" {
		if mln, err = net.Listen("tcp", cfg.Metrics.Listen); err != nil {
			_ = ln.Close()
			_ = disp.Close(ctx)
			closeStore()
			return fmt.Errorf("metrics listen: %w", err)
		}
		msrv = &http.Server{Handler: m.handler(st, coll, log), ReadHeaderTimeout: 10 * time.Second}
	}

	// The collector runs on its own context so shutdown can stop it after HTTP.
	collCtx, stopColl := context.WithCancel(context.WithoutCancel(ctx))
	collDone := make(chan struct{})
	go func() {
		defer close(collDone)
		if err := coll.Run(collCtx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("collector stopped", "err", err)
		}
	}()
	serveErr := make(chan error, 2)
	go func() { serveErr <- wrapServe("http", srv.Serve(ln)) }()
	log.Info("http listening", "addr", ln.Addr().String())
	if msrv != nil {
		go func() { serveErr <- wrapServe("metrics", msrv.Serve(mln)) }()
		log.Info("metrics listening", "addr", mln.Addr().String())
	}

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-serveErr:
	}

	// Orderly shutdown: HTTP, collector, events, store; 10 seconds in total.
	log.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		log.Error("http shutdown", "err", err)
		_ = srv.Close()
	}
	if msrv != nil {
		if err := msrv.Shutdown(sctx); err != nil {
			log.Error("metrics shutdown", "err", err)
			_ = msrv.Close()
		}
	}
	stopColl()
	select {
	case <-collDone:
	case <-sctx.Done():
		log.Error("collector did not stop within the shutdown bound")
	}
	if err := disp.Close(sctx); err != nil {
		log.Error("events shutdown", "err", err)
	}
	closeStore()
	log.Info("stopped")
	return runErr
}

func wrapServe(name string, err error) error {
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("%s server: %w", name, err)
}

// configPath resolves the config file: the -config flag in Args, else
// Options.ConfigPath, else HOURGLASS_CONFIG; empty means no file.
func configPath(opts runhook.Options, stderr io.Writer) (string, error) {
	fs := flag.NewFlagSet("hourglassd", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flagPath := fs.String("config", "", "path of the YAML config file (or set HOURGLASS_CONFIG)")
	if err := fs.Parse(opts.Args); err != nil {
		return "", err
	}
	if fs.NArg() > 0 {
		return "", fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	switch {
	case *flagPath != "":
		return *flagPath, nil
	case opts.ConfigPath != "":
		return opts.ConfigPath, nil
	default:
		return os.Getenv("HOURGLASS_CONFIG"), nil
	}
}

// uiFallback is the handler for non-API paths (UI-3). It serves the embedded
// UI only in bundled mode (ui.serve true and a UI linked in); otherwise it
// returns nil, nothing is served at /, and the log says why once.
func uiFallback(ui iofs.FS, serve bool, log *slog.Logger) http.Handler {
	switch {
	case !serve:
		log.Info("not serving the UI: ui.serve is false (split mode)")
	case ui == nil:
		log.Info("not serving the UI: this binary was built without it (API-only)")
	default:
		log.Info("serving the bundled UI at /")
		return uiserve.Handler(ui)
	}
	return nil
}
