// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/store"
)

// Service accounts (docs/design/integration-api.md, "Service accounts",
// SVC-1 to SVC-7). A service account is a principal of its own: not a user,
// no session, no role. It authenticates with a bearer token only on routes
// under IntegrationPrefix, and only those routes read a bearer token; they
// never read the session cookie.

// IntegrationPrefix is the path prefix of every route a service token may
// call. A session cookie is not read under it (SVC-2); a bearer token is not
// read anywhere else (SVC-1).
const IntegrationPrefix = "/api/v1/integration/"

// IsIntegrationPath reports whether a request path is under IntegrationPrefix.
func IsIntegrationPath(p string) bool { return strings.HasPrefix(p, IntegrationPrefix) }

// Scopes of a service account (the design's Scopes table).
const (
	ScopeStateRead        = "state:read"
	ScopePressureWrite    = "pressure:write"
	ScopeDeploymentsWrite = "deployments:write"
	ScopeHoldsRelease     = "holds:release"
	// ScopeWeightsWrite allows the weights report only
	// (docs/design/reported-weights.md, D3).
	ScopeWeightsWrite = "weights:write"
)

// Scopes returns every valid scope, in the design's order.
func Scopes() []string {
	return []string{ScopeStateRead, ScopePressureWrite, ScopeDeploymentsWrite, ScopeHoldsRelease, ScopeWeightsWrite}
}

// ValidScope reports whether s is one of Scopes().
func ValidScope(s string) bool {
	for _, x := range Scopes() {
		if x == s {
			return true
		}
	}
	return false
}

// Service token format: hgsa_<token id>_<secret>, where the secret is the
// lower-case hex of 32 random bytes (256 bits). Only SHA-256(secret), as hex,
// is stored.
const (
	ServiceTokenPrefix = "hgsa_"
	serviceSecretBytes = 32
)

// NewServiceSecret returns a fresh secret and the hash to store for it.
func NewServiceSecret() (secret, hash string, err error) {
	b := make([]byte, serviceSecretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	secret = hex.EncodeToString(b)
	return secret, HashServiceSecret(secret), nil
}

// HashServiceSecret is the stored form of a secret: hex SHA-256.
func HashServiceSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// FormatServiceToken builds the token a client sends from the row id and the
// secret. It is shown once, in the response that issues it (SVC-4).
func FormatServiceToken(tokenID int64, secret string) string {
	return ServiceTokenPrefix + strconv.FormatInt(tokenID, 10) + "_" + secret
}

// parseServiceToken splits a token into its id and secret; ok is false for
// anything that is not the documented shape.
func parseServiceToken(tok string) (id int64, secret string, ok bool) {
	rest, found := strings.CutPrefix(tok, ServiceTokenPrefix)
	if !found {
		return 0, "", false
	}
	idPart, secret, found := strings.Cut(rest, "_")
	if !found || len(secret) != 2*serviceSecretBytes {
		return 0, "", false
	}
	// Only the canonical decimal form: no sign, no leading zero, so a token
	// has exactly one spelling.
	if idPart == "" || idPart[0] == '0' || strings.Trim(idPart, "0123456789") != "" {
		return 0, "", false
	}
	id, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || id <= 0 {
		return 0, "", false
	}
	return id, secret, true
}

// ServicePrincipal is an authenticated service account, as stored in the
// request context of an integration route.
type ServicePrincipal struct {
	AccountID   int64
	AccountName string
	Scopes      []string
	TokenID     int64
}

// HasScope reports whether the principal holds scope s.
func (p *ServicePrincipal) HasScope(s string) bool {
	for _, x := range p.Scopes {
		if x == s {
			return true
		}
	}
	return false
}

// ServiceFrom returns the service principal of an integration request, if any.
func ServiceFrom(ctx context.Context) (*ServicePrincipal, bool) {
	p, ok := ctx.Value(serviceKey).(*ServicePrincipal)
	return p, ok
}

// Fixed messages of the bearer refusals: they say nothing about why a token
// was refused or which scope is missing (SVC-3, SVC-4).
const (
	msgServiceUnauthenticated = "a valid service token is required"
	msgServiceForbidden       = "not allowed"
	msgServiceRateLimited     = "too many failed attempts"
)

// errBadToken is any refusal of a presented token: unknown, wrong secret,
// expired, revoked, deleted or disabled. They are not told apart.
var errBadToken = errors.New("auth: invalid service token")

// dummyServiceHash is compared against when the token id matches no row, so
// that an unknown id costs the same comparison as a wrong secret.
var dummyServiceHash = HashServiceSecret(strings.Repeat("0", 2*serviceSecretBytes))

// verifyServiceToken resolves a token to a principal (SVC-5: every check runs
// on every request, against the database). Store failures are returned as
// they are; every refusal is errBadToken.
func (s *Service) verifyServiceToken(ctx context.Context, tok string) (*ServicePrincipal, error) {
	id, secret, ok := parseServiceToken(tok)
	if !ok {
		return nil, errBadToken
	}
	t, a, err := s.st.GetServiceTokenWithAccount(ctx, id)
	want := t.TokenHash
	switch {
	case errors.Is(err, store.ErrNotFound):
		want = dummyServiceHash
	case err != nil:
		return nil, err
	}
	got := HashServiceSecret(secret)
	match := subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
	if err != nil || !match || !a.Enabled {
		return nil, errBadToken
	}
	now := s.now()
	if t.ExpiresAt != nil && !now.Before(*t.ExpiresAt) {
		return nil, errBadToken
	}
	// last_used_at at most once a minute; the store applies the rule (D44).
	if _, err := s.st.TouchServiceToken(ctx, t.ID, now); err != nil {
		s.log.Error("recording service token use failed", "token_id", t.ID, "err", err)
	}
	scopes := append([]string(nil), a.Scopes...)
	return &ServicePrincipal{AccountID: a.ID, AccountName: a.Name, Scopes: scopes, TokenID: t.ID}, nil
}

// bearerToken returns the token of an "Authorization: Bearer <token>" header;
// present is false when the request carries no bearer credentials at all.
func bearerToken(r *http.Request) (tok string, present bool) {
	h := r.Header.Get("Authorization")
	scheme, rest, found := strings.Cut(h, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// clientAddr is the address failed bearer attempts are counted against: the
// host of the connection's remote address. Forwarding headers are not
// trusted, since any client can set them (D47).
func clientAddr(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// RequireService authenticates a service token and puts the principal in the
// context, or answers 401 unauthenticated. A token that verifies always
// authenticates, whatever the address's failure count, and is not a failure.
// Only failed attempts are limited, like local sign-in (AUTH-7's 5 per 15
// minutes) and per client address: once an address is over the limit a
// further failed attempt answers 429 rate_limited instead of 401 (SVC-7, D47).
// Every request does the same lookup and comparison before the limiter is
// consulted, so the limit says nothing about whether a token id exists. It
// never reads the session cookie (SVC-2).
func (s *Service) RequireService(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok, present := bearerToken(r)
		if !present {
			apierr.Write(w, http.StatusUnauthorized, apierr.Unauthenticated, msgServiceUnauthenticated)
			return
		}
		p, err := s.verifyServiceToken(r.Context(), tok)
		switch {
		case errors.Is(err, errBadToken):
			if limited, wait := s.bearerLimiter.attempt(clientAddr(r), s.now()); limited {
				secs := max(int(math.Ceil(wait.Seconds())), 1)
				w.Header().Set("Retry-After", strconv.Itoa(secs))
				apierr.Write(w, http.StatusTooManyRequests, apierr.RateLimited, msgServiceRateLimited)
				return
			}
			apierr.Write(w, http.StatusUnauthorized, apierr.Unauthenticated, msgServiceUnauthenticated)
			return
		case err != nil:
			s.log.Error("verifying service token failed", "err", err)
			apierr.Write(w, http.StatusInternalServerError, apierr.Internal, msgInternal)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), serviceKey, p)))
	})
}

// RequireScope answers 403 forbidden, with a fixed message, when the service
// principal lacks scope (SVC-3). It must run after RequireService.
func (s *Service) RequireScope(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := ServiceFrom(r.Context())
		if !ok {
			apierr.Write(w, http.StatusUnauthorized, apierr.Unauthenticated, msgServiceUnauthenticated)
			return
		}
		if !p.HasScope(scope) {
			apierr.Write(w, http.StatusForbidden, apierr.Forbidden, msgServiceForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ServiceRoute is the chain of an integration route: bearer token, then the
// scope. There is no CSRF check: the route never reads a cookie (SVC-2).
// scope must be one of Scopes().
func (s *Service) ServiceRoute(scope string, h http.Handler) http.Handler {
	if !ValidScope(scope) {
		panic("auth: unknown service scope " + scope)
	}
	return s.RequireService(s.RequireScope(scope, h))
}

// addrLimiter counts failed bearer attempts per client address (SVC-7) in the
// same window as local sign-in. One mutex covers the check and the recording,
// so concurrent failures cannot exceed the limit. Stale addresses are swept
// at most once a minute, and the map is capped: when it is full of live
// addresses a failure from a new address is refused like an over-limit one.
// The zero value is ready to use.
type addrLimiter struct {
	mu        sync.Mutex
	by        map[string][]time.Time
	nextSweep time.Time
}

const (
	// maxTrackedAddrs bounds the memory of the limiter.
	maxTrackedAddrs = 10000
	// sweepEvery is the least time between two sweeps of the whole map.
	sweepEvery = time.Minute
)

// attempt records one failed attempt from addr, unless the address is already
// over the limit; limited then reports it and how long until the oldest
// counted failure leaves the window.
func (l *addrLimiter) attempt(addr string, now time.Time) (limited bool, wait time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.by == nil {
		l.by = map[string][]time.Time{}
	}
	if !now.Before(l.nextSweep) {
		l.sweep(now)
	}
	fs, known := l.by[addr]
	if !known && len(l.by) >= maxTrackedAddrs {
		return true, sweepEvery
	}
	cut := now.Add(-failureWindow)
	i := 0
	for i < len(fs) && !fs[i].After(cut) {
		i++
	}
	fs = fs[i:]
	if len(fs) >= maxFailures {
		l.by[addr] = fs
		return true, fs[len(fs)-maxFailures].Add(failureWindow).Sub(now)
	}
	l.by[addr] = append(fs, now)
	return false, 0
}

// sweep drops addresses whose failures have all left the window. Caller holds mu.
func (l *addrLimiter) sweep(now time.Time) {
	l.nextSweep = now.Add(sweepEvery)
	cut := now.Add(-failureWindow)
	for k, fs := range l.by {
		if len(fs) == 0 || !fs[len(fs)-1].After(cut) {
			delete(l.by, k)
		}
	}
}
