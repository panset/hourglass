// SPDX-License-Identifier: MIT

package auth

import (
	"encoding/json"
	"errors"
	"math"
	"mime"
	"net/http"
	"strconv"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/store"
)

// maxBodyBytes bounds auth request bodies.
const maxBodyBytes = 8 << 10

// Register mounts the routes this package serves on mux:
//
//	GET  /auth/oidc/start       public (404 when OIDC is not configured)
//	GET  /auth/callback         public (404 when OIDC is not configured)
//	POST /auth/local            public
//	POST /auth/local/password   local admin session + CSRF (exempt from the password guard)
//	POST /auth/logout           any session + CSRF (exempt from the password guard)
//	GET  /api/v1/auth/methods   public
//	GET  /api/v1/session        any session (exempt from the password guard)
//
// The caller wraps the whole mux in s.CORS(s.Authenticate(mux)) so these
// routes see the session.
func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/oidc/start", s.handleOIDCStart)
	mux.HandleFunc("GET /auth/callback", s.handleOIDCCallback)
	mux.HandleFunc("POST /auth/local", s.handleLocalSignIn)
	mux.Handle("POST /auth/local/password", s.RequireLocalAdmin(s.RequireCSRF(http.HandlerFunc(s.handleChangePassword))))
	mux.Handle("POST /auth/logout", s.RequireSession(s.RequireCSRF(http.HandlerFunc(s.handleLogout))))
	mux.HandleFunc("GET /api/v1/auth/methods", s.handleMethods)
	mux.Handle("GET /api/v1/session", s.RequireSession(http.HandlerFunc(s.handleSession)))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decodeBody reads a JSON body. Requiring Content-Type application/json
// means a cross-site form cannot post here without a CORS preflight.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		apierr.Write(w, http.StatusBadRequest, apierr.Validation, "Content-Type must be application/json")
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(v); err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.Validation, "invalid JSON body")
		return false
	}
	return true
}

func (s *Service) internal(w http.ResponseWriter, what string, err error) {
	s.log.Error(what, "err", err)
	apierr.Write(w, http.StatusInternalServerError, apierr.Internal, msgInternal)
}

func (s *Service) handleMethods(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.Methods())
}

// handleSession is GET /api/v1/session (AUTH-5, UI-5).
func (s *Service) handleSession(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFrom(r.Context())
	sess, _ := SessionFrom(r.Context())
	writeJSON(w, http.StatusOK, s.view(u, sess))
}

func (s *Service) rateLimited(w http.ResponseWriter) bool {
	limited, wait := s.limiter.limited(s.now())
	if !limited {
		return false
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
	apierr.Write(w, http.StatusTooManyRequests, apierr.RateLimited, "too many failed attempts")
	return true
}

type localSignInRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleLocalSignIn is POST /auth/local (AUTH-6, AUTH-7). Only the local
// admin row signs in here. Unknown usernames and OIDC users cost one argon2
// verification like a wrong password and get the same 401.
func (s *Service) handleLocalSignIn(w http.ResponseWriter, r *http.Request) {
	var req localSignInRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" {
		apierr.Write(w, http.StatusBadRequest, apierr.Validation, "username and password are required")
		return
	}
	s.localMu.Lock()
	defer s.localMu.Unlock()
	if s.rateLimited(w) {
		return
	}
	u, err := s.st.GetUserByUsername(r.Context(), req.Username)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.internal(w, "loading user failed", err)
		return
	}
	hash, known := s.dummyHash, err == nil && IsLocalAdmin(u) && u.PasswordHash != ""
	if known {
		hash = u.PasswordHash
	}
	ok, verr := VerifyPassword(hash, req.Password)
	if verr != nil {
		s.log.Error("stored password hash is malformed")
	}
	if !known || !ok {
		s.limiter.fail(s.now())
		apierr.Write(w, http.StatusUnauthorized, apierr.Unauthenticated, "invalid username or password")
		return
	}
	sess, err := s.startSession(r.Context(), w, r, u.ID)
	if err != nil {
		s.internal(w, "creating session failed", err)
		return
	}
	pu, err := s.loadUser(r.Context(), u.ID)
	if err != nil {
		s.internal(w, "loading user failed", err)
		return
	}
	writeJSON(w, http.StatusOK, s.view(pu, sess))
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// handleChangePassword is POST /auth/local/password (AUTH-7). It clears
// must_change_password, deletes every session of the user and issues a new
// one, so the response carries a new cookie and CSRF token.
func (s *Service) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFrom(r.Context())
	var req changePasswordRequest
	if !decodeBody(w, r, &req) {
		return
	}
	s.localMu.Lock()
	defer s.localMu.Unlock()
	if s.rateLimited(w) {
		return
	}
	ok, err := VerifyPassword(u.PasswordHash, req.CurrentPassword)
	if err != nil {
		s.internal(w, "stored password hash is malformed", errors.New("malformed hash"))
		return
	}
	if !ok {
		s.limiter.fail(s.now())
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, "current password is wrong", "current_password")
		return
	}
	if err := ValidatePassword(req.NewPassword); err != nil {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, err.Error(), "new_password")
		return
	}
	if req.NewPassword == req.CurrentPassword {
		apierr.WriteField(w, http.StatusBadRequest, apierr.Validation, ErrPasswordUnchanged.Error(), "new_password")
		return
	}
	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		s.internal(w, "hashing password failed", err)
		return
	}
	cookie, id, err := newSessionToken()
	if err != nil {
		s.internal(w, "creating session failed", err)
		return
	}
	now := s.now()
	err = s.st.Tx(r.Context(), func(tx *store.Tx) error {
		if err := tx.SetPassword(r.Context(), u.ID, hash, false); err != nil {
			return err
		}
		if _, err := tx.DeleteUserSessions(r.Context(), u.ID); err != nil {
			return err
		}
		return tx.CreateSession(r.Context(), id, u.ID, now, now.Add(IdleTimeout))
	})
	if err != nil {
		s.internal(w, "changing password failed", err)
		return
	}
	http.SetCookie(w, sessionCookie(cookie))
	sess, err := s.st.GetSession(r.Context(), id)
	if err != nil {
		s.internal(w, "loading session failed", err)
		return
	}
	pu, err := s.loadUser(r.Context(), u.ID)
	if err != nil {
		s.internal(w, "loading user failed", err)
		return
	}
	s.log.Info("local admin password changed", "user_id", u.ID)
	writeJSON(w, http.StatusOK, s.view(pu, sess))
}

// handleLogout is POST /auth/logout: deletes the row, clears the cookie.
func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r.Context())
	if err := s.st.DeleteSession(r.Context(), sess.ID); err != nil {
		s.internal(w, "deleting session failed", err)
		return
	}
	http.SetCookie(w, sessionCookie(""))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}
