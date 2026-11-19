// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/store"
)

// Fixed error messages: error responses carry no data (AUTH-4).
const (
	msgUnauthenticated = "sign in required"
	msgNoRole          = "you have no role"
	msgForbidden       = "admin role required"
	msgCSRF            = "missing or invalid CSRF token"
	msgPasswordChange  = "password change required"
	msgInternal        = "internal error"
)

// Authenticate loads the session and the user with roles into the request
// context. A request without a valid session continues anonymous.
func (s *Service) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, sess, ok, err := s.loadSession(r.Context(), r)
		if err != nil {
			s.log.Error("loading session failed", "err", err)
			apierr.Write(w, http.StatusInternalServerError, apierr.Internal, msgInternal)
			return
		}
		if ok {
			r = r.WithContext(withPrincipal(r.Context(), u, sess))
		}
		next.ServeHTTP(w, r)
	})
}

// RequireSession answers 401 unauthenticated without a session.
func (s *Service) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFrom(r.Context()); !ok {
			apierr.Write(w, http.StatusUnauthorized, apierr.Unauthenticated, msgUnauthenticated)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole requires a session and at least one role on an active user;
// otherwise 401 unauthenticated or 403 no_role (AUTH-4).
func (s *Service) RequireRole(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFrom(r.Context())
		if !ok {
			apierr.Write(w, http.StatusUnauthorized, apierr.Unauthenticated, msgUnauthenticated)
			return
		}
		if !u.Authorized() {
			apierr.Write(w, http.StatusForbidden, apierr.NoRole, msgNoRole)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin requires the admin role: 401 without a session, 403 no_role
// for a user with no role (or not active), 403 forbidden otherwise.
func (s *Service) RequireAdmin(next http.Handler) http.Handler {
	return s.RequireRole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := UserFrom(r.Context())
		if !u.HasRole(store.RoleAdmin) {
			apierr.Write(w, http.StatusForbidden, apierr.Forbidden, msgForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// RequireLocalAdmin requires the local admin's session (401 / 403 forbidden).
func (s *Service) RequireLocalAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFrom(r.Context())
		if !ok {
			apierr.Write(w, http.StatusUnauthorized, apierr.Unauthenticated, msgUnauthenticated)
			return
		}
		if !IsLocalAdmin(u.User) {
			apierr.Write(w, http.StatusForbidden, apierr.Forbidden, msgForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// safeMethod reports methods that never need the CSRF header.
func safeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// RequireCSRF checks X-CSRF-Token on every request whose method is not
// GET, HEAD or OPTIONS: 401 without a session, 403 csrf when missing or wrong.
func (s *Service) RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if safeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		sess, ok := SessionFrom(r.Context())
		if !ok {
			apierr.Write(w, http.StatusUnauthorized, apierr.Unauthenticated, msgUnauthenticated)
			return
		}
		if !s.checkCSRF(sess.ID, r.Header.Get(CSRFHeader)) {
			apierr.Write(w, http.StatusForbidden, apierr.CSRF, msgCSRF)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequirePasswordChanged answers 403 password_change_required while the
// local admin still has must_change_password set. Mount it on every route
// except GET /api/v1/session, POST /auth/local/password and POST
// /auth/logout (the public routes need no guard: they show no data).
func (s *Service) RequirePasswordChanged(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, ok := UserFrom(r.Context()); ok && IsLocalAdmin(u.User) && u.MustChangePassword {
			apierr.Write(w, http.StatusForbidden, apierr.PasswordChangeRequired, msgPasswordChange)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RoleRoute is the chain for an "Any role" data route: role, password
// guard, CSRF. The request must already have passed Authenticate.
func (s *Service) RoleRoute(h http.Handler) http.Handler {
	return s.RequireRole(s.RequirePasswordChanged(s.RequireCSRF(h)))
}

// AdminRoute is the chain for an admin route: admin role, password guard,
// CSRF. The request must already have passed Authenticate.
func (s *Service) AdminRoute(h http.Handler) http.Handler {
	return s.RequireAdmin(s.RequirePasswordChanged(s.RequireCSRF(h)))
}
