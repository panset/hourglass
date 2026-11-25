// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/panset/hourglass/internal/store"
)

// Session parameters (AUTH-3).
const (
	SessionCookie   = "hourglass_session"
	CSRFHeader      = "X-CSRF-Token"
	IdleTimeout     = 8 * time.Hour
	touchInterval   = time.Minute
	sessionTokenLen = 32
)

// newSessionToken returns the cookie value (base64url of 32 random bytes)
// and the stored id (SHA-256 hex of those bytes).
func newSessionToken() (cookie, id string, err error) {
	b := make([]byte, sessionTokenLen)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), hashToken(b), nil
}

func hashToken(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// sessionIDFromCookie maps a cookie value to the stored id; ok is false for
// a value that is not 32 base64url bytes.
func sessionIDFromCookie(v string) (string, bool) {
	b, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil || len(b) != sessionTokenLen {
		return "", false
	}
	return hashToken(b), true
}

// CSRFToken is hex HMAC-SHA256(session key, session id) (AUTH-3).
func (s *Service) CSRFToken(sessionID string) string {
	m := hmac.New(sha256.New, s.csrfKey)
	m.Write([]byte(sessionID))
	return hex.EncodeToString(m.Sum(nil))
}

func (s *Service) checkCSRF(sessionID, got string) bool {
	want := s.CSRFToken(sessionID)
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// sessionCookie builds the session cookie; an empty value clears it.
func sessionCookie(value string) *http.Cookie {
	c := &http.Cookie{Name: SessionCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
	if value == "" {
		c.MaxAge = -1
	}
	return c
}

// startSession creates a session row for userID and sets the cookie. Any
// session the request already carried is deleted first (no fixation).
func (s *Service) startSession(ctx context.Context, w http.ResponseWriter, r *http.Request, userID int64) (store.Session, error) {
	if c, err := r.Cookie(SessionCookie); err == nil {
		if old, ok := sessionIDFromCookie(c.Value); ok {
			_ = s.st.DeleteSession(ctx, old)
		}
	}
	cookie, id, err := newSessionToken()
	if err != nil {
		return store.Session{}, err
	}
	now := s.now()
	// Opportunistic cleanup keeps abandoned rows from piling up.
	_, _ = s.st.DeleteExpiredSessions(ctx, now)
	if err := s.st.CreateSession(ctx, id, userID, now, now.Add(IdleTimeout)); err != nil {
		return store.Session{}, err
	}
	http.SetCookie(w, sessionCookie(cookie))
	return s.st.GetSession(ctx, id)
}

// loadSession resolves the request's cookie to a live session and user. It
// returns ok=false for no cookie, an unknown or expired session; errors are
// only for store failures.
func (s *Service) loadSession(ctx context.Context, r *http.Request) (*User, store.Session, bool, error) {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return nil, store.Session{}, false, nil
	}
	id, ok := sessionIDFromCookie(c.Value)
	if !ok {
		return nil, store.Session{}, false, nil
	}
	sess, err := s.st.GetSession(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, store.Session{}, false, nil
	}
	if err != nil {
		return nil, store.Session{}, false, err
	}
	now := s.now()
	if !now.Before(sess.ExpiresAt) {
		_ = s.st.DeleteSession(ctx, id)
		return nil, store.Session{}, false, nil
	}
	u, err := s.loadUser(ctx, sess.UserID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, store.Session{}, false, nil
	}
	if err != nil {
		return nil, store.Session{}, false, err
	}
	if now.Sub(sess.LastSeenAt) >= touchInterval {
		exp := now.Add(IdleTimeout)
		if err := s.st.TouchSession(ctx, id, now, exp); err != nil && !errors.Is(err, store.ErrNotFound) {
			return nil, store.Session{}, false, err
		}
		sess.LastSeenAt, sess.ExpiresAt = now, exp
	}
	return u, sess, true, nil
}

func (s *Service) loadUser(ctx context.Context, id int64) (*User, error) {
	su, err := s.st.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	roles, err := s.st.GetUserRoles(ctx, id)
	if err != nil {
		return nil, err
	}
	if roles == nil {
		roles = []string{}
	}
	return &User{User: su, Roles: roles}, nil
}

// DisplayName is the name shown for a user: name, else username, else email.
// It is the one function behind the session's display_name and the user names
// the rest of the API shows (AUTH-9), so the two cannot drift. It lives here
// because internal/api already imports this package and not the reverse.
func DisplayName(u store.User) string {
	// A value of only spaces counts as no value, so the result is never blank
	// while any of the three has text.
	for _, v := range []string{u.Name, u.Username, u.Email} {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

// SessionView is the Session schema of api/openapi.yaml: for a user with no
// role it is exactly these fields and nothing else (AUTH-5, AUTH-9). The one
// field beyond the identity is display_name, derived only from name, username
// and email, which this response already carries.
type SessionView struct {
	UserID             int64    `json:"user_id"`
	Username           string   `json:"username"`
	Email              string   `json:"email"`
	Name               string   `json:"name"`
	DisplayName        string   `json:"display_name"`
	AuthSource         string   `json:"auth_source"`
	Status             string   `json:"status"`
	Roles              []string `json:"roles"`
	CSRFToken          string   `json:"csrf_token"`
	MustChangePassword bool     `json:"must_change_password"`
	IsLocalAdmin       bool     `json:"is_local_admin"`
}

func (s *Service) view(u *User, sess store.Session) SessionView {
	roles := u.Roles
	if roles == nil {
		roles = []string{}
	}
	return SessionView{UserID: u.ID, Username: u.Username, Email: u.Email, Name: u.Name,
		DisplayName: DisplayName(u.User), AuthSource: u.AuthSource, Status: u.Status, Roles: roles, CSRFToken: s.CSRFToken(sess.ID),
		MustChangePassword: u.MustChangePassword, IsLocalAdmin: IsLocalAdmin(u.User)}
}
