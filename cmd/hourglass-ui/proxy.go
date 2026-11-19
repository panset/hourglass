// SPDX-License-Identifier: MIT

package main

import (
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/uiserve"
)

// proxied are the paths forwarded to the API container (UI-3). Each matches
// itself and anything below it; nothing else is proxied.
var proxied = []string{"/api", "/auth", "/healthz"}

// UIHealthPath is this server's own health check.
const uiHealthPath = "/ui-healthz"

// newHandler serves the UI files from ui and forwards the proxied prefixes to
// the API at apiBase.
func newHandler(ui fs.FS, apiBase string) (http.Handler, error) {
	target, err := parseAPI(apiBase)
	if err != nil {
		return nil, err
	}
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			// The API sees the Host the browser used, so absolute URLs and
			// cookies it builds name the one origin the browser sees (UI-4).
			pr.Out.Host = pr.In.Host
			pr.Out.Header.Set("X-Forwarded-Host", pr.In.Host)
			pr.Out.Header.Set("X-Forwarded-Proto", forwardedProto(pr.In))
			if ip, _, err := net.SplitHostPort(pr.In.RemoteAddr); err == nil {
				pr.Out.Header.Set("X-Forwarded-For", ip)
			}
		},
		// Cookies, Set-Cookie and Location are passed through untouched:
		// ReverseProxy does not rewrite them, and it never follows redirects.
		FlushInterval: -1,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			MaxIdleConns:          32,
			IdleConnTimeout:       60 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			apierr.Write(w, http.StatusBadGateway, apierr.Internal, "the API is not reachable")
		},
	}
	static := uiserve.Handler(ui)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == uiHealthPath:
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write([]byte("{\"status\":\"ok\"}\n"))
		case isProxied(r.URL.Path):
			rp.ServeHTTP(w, r)
		default:
			static.ServeHTTP(w, r)
		}
	}), nil
}

func isProxied(p string) bool {
	for _, pre := range proxied {
		if p == pre || strings.HasPrefix(p, pre+"/") {
			return true
		}
	}
	return false
}

// forwardedProto is the scheme the browser used. A TLS-terminating proxy in
// front of this server states it in X-Forwarded-Proto, which is kept; without
// one it is derived from this connection.
func forwardedProto(r *http.Request) string {
	if p := r.Header.Get("X-Forwarded-Proto"); p == "http" || p == "https" {
		return p
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
