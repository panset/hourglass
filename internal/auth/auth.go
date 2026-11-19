// SPDX-License-Identifier: MIT

// Package auth implements sign-in and sessions (AUTH-1 to AUTH-8): the local
// admin account, OIDC authorization code flow with PKCE, server-side
// sessions, CSRF tokens, role middleware, CORS (UI-4) and the HTTP handlers
// of the /auth/* routes, GET /api/v1/auth/methods and GET /api/v1/session.
//
// Nothing here is specific to any identity provider.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// Config is the file/env configuration auth needs (spec "Configuration").
type Config struct {
	// OIDC is optional; with an empty IssuerURL only local sign-in exists.
	OIDC OIDCConfig
	// SessionKey signs CSRF tokens and the OIDC flow cookie. Empty means a
	// random key is generated at start (sessions do not survive a restart).
	// When set it must be at least 32 bytes.
	SessionKey string
	// AllowedOrigins is cors.allowed_origins (UI-4); empty disables CORS.
	AllowedOrigins []string
}

// OIDCConfig is the OIDC client registration.
type OIDCConfig struct {
	IssuerURL    string // discovery at IssuerURL + /.well-known/openid-configuration
	ClientID     string
	ClientSecret string // HOURGLASS_OIDC_CLIENT_SECRET; never logged
	RedirectURL  string // public URL of GET /auth/callback
}

// Enabled reports whether OIDC sign-in is configured.
func (c OIDCConfig) Enabled() bool { return c.IssuerURL != "" }

// Emitter receives the user.pending event (satisfied by *events.Dispatcher).
type Emitter interface {
	Emit(hourglass.Event)
}

// Option configures a Service.
type Option func(*Service)

// WithClock replaces time.Now (tests).
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

// WithHTTPClient sets the client used for OIDC discovery, JWKS and the
// token exchange. The default has a 10 second timeout.
func WithHTTPClient(c *http.Client) Option { return func(s *Service) { s.httpClient = c } }

// minSessionKeyLen is the minimum configured session key length in bytes.
const minSessionKeyLen = 32

// Service holds the auth state. Create it with New.
type Service struct {
	cfg        Config
	st         *store.Store
	em         Emitter
	log        *slog.Logger
	now        func() time.Time
	httpClient *http.Client

	csrfKey  []byte // HMAC key for CSRF tokens
	stateKey []byte // HMAC key for the OIDC flow cookie

	limiter   failureLimiter
	localMu   sync.Mutex // serializes local password checks (bounds argon2 memory, makes the limit exact)
	dummyHash string     // verified against for unknown usernames (equal timing)

	oidcMu   sync.Mutex
	provider *oidc.Provider
}

// New builds the Service. It does not contact the OIDC issuer; discovery
// happens on the first OIDC request and is retried until it succeeds.
func New(cfg Config, st *store.Store, em Emitter, log *slog.Logger, opts ...Option) (*Service, error) {
	if st == nil {
		return nil, errors.New("auth: store is required")
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	s := &Service{cfg: cfg, st: st, em: em, log: log, now: time.Now,
		httpClient: &http.Client{Timeout: 10 * time.Second}}
	for _, o := range opts {
		o(s)
	}
	if o := cfg.OIDC; o.Enabled() && (o.ClientID == "" || o.RedirectURL == "") {
		return nil, errors.New("auth: oidc issuer is set but client id or redirect url is missing")
	}
	key := []byte(cfg.SessionKey)
	switch {
	case len(key) == 0:
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		log.Warn("no session key configured; generated a random one, so sessions will not survive a restart")
	case len(key) < minSessionKeyLen:
		return nil, errors.New("auth: session key must be at least 32 bytes")
	}
	// Separate keys per purpose, derived from the one configured secret.
	s.csrfKey = deriveKey(key, "hourglass csrf v1")
	s.stateKey = deriveKey(key, "hourglass oidc flow v1")
	dummy, err := GeneratePassword(24)
	if err != nil {
		return nil, err
	}
	if s.dummyHash, err = HashPassword(dummy); err != nil {
		return nil, err
	}
	return s, nil
}

func deriveKey(key []byte, label string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(label))
	return m.Sum(nil)
}

// Methods is the GET /api/v1/auth/methods body.
type Methods struct {
	OIDC  bool `json:"oidc"`
	Local bool `json:"local"`
}

// Methods reports the sign-in methods: local is always available.
func (s *Service) Methods() Methods { return Methods{OIDC: s.cfg.OIDC.Enabled(), Local: true} }

// User is a signed-in user with roles, as stored in the request context.
type User struct {
	store.User
	Roles []string // sorted; empty means unassigned
}

// HasRole reports whether the user has role r.
func (u *User) HasRole(r string) bool {
	for _, x := range u.Roles {
		if x == r {
			return true
		}
	}
	return false
}

// Authorized reports whether the user may see data: active with a role.
func (u *User) Authorized() bool { return u.Status == store.StatusActive && len(u.Roles) > 0 }

// IsLocalAdmin reports whether u is the permanent local admin account: it
// cannot acquire setups and has no token balance (rules list, AUTH-8).
func IsLocalAdmin(u store.User) bool { return u.AuthSource == store.AuthLocal }

type ctxKey int

const (
	userKey ctxKey = iota
	sessionKey
)

// UserFrom returns the signed-in user, if any.
func UserFrom(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userKey).(*User)
	return u, ok
}

// SessionFrom returns the current session row, if any. Its ID is the stored
// hash, never the cookie value.
func SessionFrom(ctx context.Context) (store.Session, bool) {
	s, ok := ctx.Value(sessionKey).(store.Session)
	return s, ok
}

func withPrincipal(ctx context.Context, u *User, sess store.Session) context.Context {
	return context.WithValue(context.WithValue(ctx, userKey, u), sessionKey, sess)
}
