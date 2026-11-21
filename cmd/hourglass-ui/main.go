// SPDX-License-Identifier: MIT

// Command hourglass-ui is the split-mode front end (UI-3, UI-4): it serves the
// built UI (embedded under the "ui" build tag, like hourglassd) and
// reverse-proxies /api/, /auth/ and /healthz to the API container, so the
// browser sees one origin and the session cookie and CSRF token work unchanged.
//
//	hourglass-ui [-listen addr] [-api http://host:port] [-tls-cert file -tls-key file]
//
// -api may also be set with HOURGLASS_UI_API, and -listen with
// HOURGLASS_UI_LISTEN (default :8080); a flag wins over its variable. With
// -tls-cert and -tls-key (or HOURGLASS_UI_TLS_CERT and HOURGLASS_UI_TLS_KEY)
// it serves HTTPS itself (D64). The proxy's own health check is /ui-healthz.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/panset/hourglass/internal/servetls"
	"github.com/panset/hourglass/internal/uiserve"
	"github.com/panset/hourglass/internal/version"
)

const (
	apiEnv        = "HOURGLASS_UI_API"
	listenEnv     = "HOURGLASS_UI_LISTEN"
	tlsCertEnv    = "HOURGLASS_UI_TLS_CERT"
	tlsKeyEnv     = "HOURGLASS_UI_TLS_KEY"
	defaultListen = ":8080"
	shutdownBound = 10 * time.Second
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fsys, _ := uiserve.Embedded()
	os.Exit(run(ctx, os.Args[1:], os.Getenv, fsys, os.Stdout, os.Stderr))
}

// run parses the flags, serves until ctx is cancelled and returns the exit code.
func run(ctx context.Context, args []string, getenv func(string) string, ui fs.FS, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "version" {
		fmt.Fprintln(stdout, "hourglass-ui", version.Version)
		return 0
	}
	fl := flag.NewFlagSet("hourglass-ui", flag.ContinueOnError)
	fl.SetOutput(stderr)
	listen := fl.String("listen", "", "address to listen on (or HOURGLASS_UI_LISTEN; default "+defaultListen+")")
	api := fl.String("api", "", "base URL of the API container, e.g. http://hourglass-api:8080 (or HOURGLASS_UI_API)")
	tlsCert := fl.String("tls-cert", "", "PEM certificate file; with -tls-key, serve HTTPS (or "+tlsCertEnv+")")
	tlsKey := fl.String("tls-key", "", "PEM private key file for -tls-cert (or "+tlsKeyEnv+")")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	if fl.NArg() > 0 {
		fmt.Fprintf(stderr, "hourglass-ui: unexpected argument %q\n", fl.Arg(0))
		return 2
	}
	if *listen == "" {
		*listen = getenv(listenEnv)
	}
	if *listen == "" {
		*listen = defaultListen
	}
	if *api == "" {
		*api = getenv(apiEnv)
	}
	if *api == "" {
		fmt.Fprintf(stderr, "hourglass-ui: the API address is required: -api http://host:port or %s\n", apiEnv)
		return 2
	}
	if *tlsCert == "" {
		*tlsCert = getenv(tlsCertEnv)
	}
	if *tlsKey == "" {
		*tlsKey = getenv(tlsKeyEnv)
	}
	tlsCfg, err := servetls.Load(*tlsCert, *tlsKey)
	if errors.Is(err, servetls.ErrOneOfPair) {
		fmt.Fprintf(stderr, "hourglass-ui: %v: -tls-cert and -tls-key, or %s and %s\n", err, tlsCertEnv, tlsKeyEnv)
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, "hourglass-ui:", err)
		return 1
	}
	if ui == nil {
		fmt.Fprintln(stderr, "hourglass-ui: this binary was built without the UI (build tag \"ui\")")
		return 1
	}
	h, err := newHandler(ui, *api)
	if err != nil {
		fmt.Fprintln(stderr, "hourglass-ui:", err)
		return 2
	}
	log := slog.New(slog.NewJSONHandler(stdout, nil))
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintln(stderr, "hourglass-ui: listen:", err)
		return 1
	}
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
	serve := srv.Serve
	if tlsCfg != nil {
		srv.TLSConfig = tlsCfg
		srv.ErrorLog = slog.NewLogLogger(log.Handler(), slog.LevelWarn)
		serve = func(l net.Listener) error { return srv.ServeTLS(l, "", "") }
	}
	errc := make(chan error, 1)
	go func() { errc <- serve(ln) }()
	log.Info("http listening", "addr", ln.Addr().String(), "api", *api, "tls", tlsCfg != nil)
	select {
	case <-ctx.Done():
	case err := <-errc:
		fmt.Fprintln(stderr, "hourglass-ui:", err)
		return 1
	}
	log.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownBound)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		_ = srv.Close()
	}
	log.Info("stopped")
	return 0
}

// parseAPI validates the API base URL: http or https, a host, no path.
func parseAPI(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("api address: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("api address %q must look like http://host:port", raw)
	}
	if u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("api address %q must not have a path, query or fragment", raw)
	}
	return u, nil
}
