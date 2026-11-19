// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"strings"
)

// CORS enables credentialed CORS for exactly the listed origins (UI-4).
// With an empty list it returns next unchanged, so no CORS header is ever
// sent. Origins are compared exactly (scheme://host[:port], no wildcard).
func CORS(allowedOrigins []string, next http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		return next
	}
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[strings.TrimRight(o, "/")] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		if origin == "" || !allowed[origin] {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
			h.Set("Access-Control-Allow-Headers", "Content-Type, "+CSRFHeader)
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CORS is the package CORS using cors.allowed_origins from the Config.
func (s *Service) CORS(next http.Handler) http.Handler { return CORS(s.cfg.AllowedOrigins, next) }
