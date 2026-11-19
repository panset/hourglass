// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
)

// statusRecorder remembers the status and whether the header was written.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wrote {
		s.status, s.wrote = code, true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wrote {
		s.status, s.wrote = http.StatusOK, true
	}
	return s.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the real writer.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// logRequests logs method, path, status and duration. It never logs bodies,
// cookies, tokens, headers or the query string (the OIDC callback carries a
// code there).
func (a *API) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		a.d.Logger.Info("request", "method", r.Method, "path", r.URL.Path,
			"status", rec.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

// recoverer turns a handler panic into the standard internal error.
func (a *API) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler {
				panic(v)
			}
			// The panic value can echo request data; log the type and stack only.
			a.d.Logger.Error("handler panic", "method", r.Method, "path", r.URL.Path,
				"panic_type", typeName(v), "stack", string(debug.Stack()))
			if !rec.wrote {
				apierr.WriteCode(w, apierr.Internal, "internal error")
			}
		}()
		next.ServeHTTP(rec, r)
	})
}

func typeName(v any) string {
	switch v.(type) {
	case error:
		return "error"
	case string:
		return "string"
	default:
		return "other"
	}
}

// noStore sets Cache-Control: no-store on every API response, errors included.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPIPath(r.URL.Path) {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// trackLocalLogin records last_login_at for the local admin after a
// successful POST /auth/local. internal/auth does not do it for local
// sign-ins (its RESULT, "Confidence and gaps"), and the hook stays here so
// that package is untouched.
func (a *API) trackLocalLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/auth/local" {
			next.ServeHTTP(w, r)
			return
		}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if rec.status != http.StatusOK {
			return
		}
		ctx := context.WithoutCancel(r.Context())
		if err := a.d.Store.TouchLocalAdminLogin(ctx, a.d.Clock.Now()); err != nil {
			a.d.Logger.Error("recording local admin last login failed", "err", err)
		}
	})
}
