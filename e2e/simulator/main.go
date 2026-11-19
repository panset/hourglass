// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := Config{Log: log}
	var ctl string
	flag.StringVar(&cfg.SOAPAddr, "soap-listen", env("VCSIM_SOAP_LISTEN", "0.0.0.0:8989"), "simulated vCenter listen address (TLS, self-signed)")
	flag.StringVar(&ctl, "control-listen", env("VCSIM_CONTROL_LISTEN", "0.0.0.0:8990"), "control API listen address (plain HTTP)")
	flag.StringVar(&cfg.Username, "username", env("VCSIM_USERNAME", "user"), "simulated vCenter username (dummy value)")
	flag.StringVar(&cfg.Password, "password", env("VCSIM_PASSWORD", "pass"), "simulated vCenter password (dummy value)")
	flag.IntVar(&cfg.Hosts, "hosts", 3, "hosts in the single cluster")
	flag.Parse()

	sim, err := NewSim(cfg)
	if err != nil {
		log.Error("start simulator", "err", err.Error())
		os.Exit(1)
	}
	defer sim.Close()
	srv := &http.Server{Addr: ctl, Handler: sim.Handler(), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("started", "soap", cfg.SOAPAddr, "control", ctl, "hosts", cfg.Hosts)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	select {
	case s := <-sig:
		log.Info("shutting down", "signal", s.String())
	case err := <-errc:
		log.Error("control server", "err", err.Error())
		sim.Close()
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Info("stopped")
}
