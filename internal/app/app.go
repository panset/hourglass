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
	"github.com/panset/hourglass/internal/reported"
	"github.com/panset/hourglass/internal/servetls"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/internal/uiserve"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/storedreports"
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

func (x extensions) runtime(oidcIssuer string) api.Runtime {
	rt := api.Runtime{OIDCIssuer: oidcIssuer, PressureSource: x.pressure.Name(), MeteringPolicy: x.policy.Name(), Hooks: []string{}}
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
	// Everything that can be checked without side effects is checked before the
	// database is opened or created, the local admin is created or a password
	// is logged: config.Load validated the session key length, OIDC shape,
	// listen addresses, timezone and CORS origins; extension names come next.
	ext, err := buildExtensions(cfg, opts.DefaultPressureSource, opts.DefaultMeteringPolicy)
	if err != nil {
		return err
	}
	// The certificate and key are read here, so a wrong path or a mismatched
	// pair stops the start before the database is touched (D64).
	tlsCfg, err := servetls.Load(cfg.TLS.Cert, cfg.TLS.Key)
	if err != nil {
		return err
	}
	// The pre-approval file is read once, here, before the database is
	// touched; an invalid file stops the start (D4). Only the count is logged.
	preFile, err := LoadPreapprovedUsers(cfg.PreapprovedUsersFile)
	if err != nil {
		return err
	}
	if cfg.PreapprovedUsersFile != "" {
		log.Info("pre-approved users file read", "file", cfg.PreapprovedUsersFile, "entries", len(preFile))
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
	for _, n := range st.UpgradeNotes() {
		log.Warn(n)
	}
	closeStore := func() {
		if err := st.Close(); err != nil {
			log.Error("closing the database failed", "err", err)
		}
	}
	var clock interface{ Now() time.Time } = systemClock{}
	if opts.Clock != nil {
		clock = opts.Clock
	}
	// The reported extensions read the stored reports through this window; it
	// is published for the life of the server.
	defer storedreports.Publish(StoredReader{reported.New(st, clock)})()
	// The seed comes before the local admin: a bad seed file must not use up the
	// one-time print of a generated admin password.
	if applied, err := ImportSeed(ctx, st, cfg.Seed, clock.Now()); err != nil {
		closeStore()
		return err
	} else if applied {
		log.Info("seed file imported", "seed", cfg.Seed)
	}
	if _, err := auth.EnsureLocalAdmin(ctx, st, cfg.AdminPassword, log); err != nil {
		closeStore()
		return err
	}

	m := newMetrics(clock.Now())
	// The events dispatcher outlives ctx's cancellation until shutdown closes it.
	disp := events.New(ext.hooks, st, events.WithLogger(log), events.WithClock(clock))
	disp.Start(context.WithoutCancel(ctx))
	led := ledger.NewService(st, disp, clock, loc)
	// Self-repair (LED-2): before the first poll and before any request, every
	// balance cache is rebuilt from the ledger for the current period.
	if err := repairBalances(ctx, led, log); err != nil {
		_ = disp.Close(ctx)
		closeStore()
		return err
	}
	coll := collector.New(collector.Deps{
		Store: st, Provider: ext.provider, Pressure: ext.pressure, Policy: ext.policy,
		Events: countingEmitter{next: disp, m: m}, Ledger: led, Clock: clock, Logger: log,
	})
	holdsSvc := holds.NewService(holds.Deps{
		Store: st, Events: disp, Poll: holdsPoll(coll), ProviderConfigured: ext.provider != nil,
		Clock: clock, Logger: log,
	})
	pre := api.NewPreapprover(api.PreapproverDeps{Store: st, Ledger: led, Events: disp, Clock: clock, Logger: log, File: preFile})
	authSvc, err := auth.New(auth.Config{
		OIDC: auth.OIDCConfig{IssuerURL: cfg.OIDC.Issuer, ClientID: cfg.OIDC.ClientID,
			ClientSecret: cfg.OIDC.ClientSecret, RedirectURL: cfg.OIDC.RedirectURL},
		SessionKey: cfg.SessionKey, AllowedOrigins: cfg.CORS.AllowedOrigins,
	}, st, disp, log, auth.WithClock(clock.Now), auth.WithPreapprover(pre))
	if err != nil {
		_ = disp.Close(ctx)
		closeStore()
		return err
	}
	rt := ext.runtime(cfg.OIDC.Issuer)
	rt.ReportedProviderMaxAge, _, _ = config.ReportedMaxAge(cfg.Provider) // validated by config.Validate
	rt.ReportedPressureMaxAge, _, _ = config.ReportedMaxAge(&cfg.Pressure)
	handler := api.New(api.Deps{
		Store: st, Auth: authSvc, Holds: holdsSvc, Ledger: led, Events: disp, Collector: coll,
		Clock: clock, Location: loc, Logger: log, Runtime: rt, Preapprovals: pre,
		Fallback: uiFallback(opts.UI, cfg.UI.Serve, log),
	})

	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	serve := srv.Serve
	if tlsCfg != nil {
		srv.TLSConfig = tlsCfg
		// Failed handshakes (a client that does not trust the certificate) go to the JSON log.
		srv.ErrorLog = slog.NewLogLogger(log.Handler(), slog.LevelWarn)
		serve = func(l net.Listener) error { return srv.ServeTLS(l, "", "") }
	}
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
	go func() { serveErr <- wrapServe("http", serve(ln)) }()
	log.Info("http listening", "addr", ln.Addr().String(), "tls", tlsCfg != nil)
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

// repairBalances rebuilds every user's balance cache at start-up and logs how
// many changed and by how much; any change is logged as a warning, since it
// means a cache was wrong (or the configured timezone moved the period).
func repairBalances(ctx context.Context, led *ledger.Service, log *slog.Logger) error {
	rep, err := led.RepairBalances(ctx)
	if err != nil {
		return fmt.Errorf("rebuilding balance caches: %w", err)
	}
	attrs := []any{"period", led.CurrentPeriod(), "changed", rep.Changed,
		"abs_delta_mt", rep.AbsDeltaMT, "net_delta_mt", rep.NetDeltaMT}
	if rep.Changed > 0 {
		log.Warn("balance caches rebuilt from the ledger: some were wrong", attrs...)
	} else {
		log.Info("balance caches checked against the ledger", attrs...)
	}
	return nil
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
