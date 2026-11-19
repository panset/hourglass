// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/pkg/hourglass"
	"golang.org/x/oauth2"
)

// OIDC flow cookie: state, nonce and PKCE verifier, authenticated with a key
// derived from the session key. No server-side table.
const (
	flowCookie     = "hourglass_oidc"
	flowCookiePath = "/auth/callback"
	flowLifetime   = 10 * time.Minute
)

var oidcScopes = []string{oidc.ScopeOpenID, "email", "profile"}

type flowState struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Expires  int64  `json:"e"` // unix seconds
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Service) signFlow(f flowState) (string, error) {
	payload, err := json.Marshal(f)
	if err != nil {
		return "", err
	}
	p := base64.RawURLEncoding.EncodeToString(payload)
	m := hmac.New(sha256.New, s.stateKey)
	m.Write([]byte(p))
	return p + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil)), nil
}

func (s *Service) openFlow(v string) (flowState, bool) {
	p, sig, ok := strings.Cut(v, ".")
	if !ok {
		return flowState{}, false
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return flowState{}, false
	}
	m := hmac.New(sha256.New, s.stateKey)
	m.Write([]byte(p))
	if !hmac.Equal(got, m.Sum(nil)) {
		return flowState{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(p)
	if err != nil {
		return flowState{}, false
	}
	var f flowState
	if err := json.Unmarshal(payload, &f); err != nil || f.State == "" || f.Nonce == "" || f.Verifier == "" {
		return flowState{}, false
	}
	if !s.now().Before(time.Unix(f.Expires, 0)) {
		return flowState{}, false
	}
	return f, true
}

func flowCookieWith(value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: flowCookie, Value: value, Path: flowCookiePath, MaxAge: maxAge,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
}

func (s *Service) clientCtx(ctx context.Context) context.Context {
	return oidc.ClientContext(ctx, s.httpClient)
}

// oidcProvider returns the discovered provider, discovering on first use.
// A failed discovery is not cached, so the next request retries.
func (s *Service) oidcProvider() (*oidc.Provider, error) {
	s.oidcMu.Lock()
	defer s.oidcMu.Unlock()
	if s.provider != nil {
		return s.provider, nil
	}
	// Background context: the provider keeps it for later JWKS fetches.
	p, err := oidc.NewProvider(s.clientCtx(context.Background()), s.cfg.OIDC.IssuerURL)
	if err != nil {
		return nil, err
	}
	s.provider = p
	return p, nil
}

func (s *Service) oauthConfig(p *oidc.Provider) *oauth2.Config {
	o := s.cfg.OIDC
	return &oauth2.Config{ClientID: o.ClientID, ClientSecret: o.ClientSecret, RedirectURL: o.RedirectURL,
		Endpoint: p.Endpoint(), Scopes: oidcScopes}
}

// handleOIDCStart is GET /auth/oidc/start (AUTH-1).
func (s *Service) handleOIDCStart(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.OIDC.Enabled() {
		apierr.Write(w, http.StatusNotFound, apierr.NotFound, "not found")
		return
	}
	p, err := s.oidcProvider()
	if err != nil {
		s.log.Error("oidc discovery failed", "err", err)
		apierr.Write(w, http.StatusInternalServerError, apierr.Internal, msgInternal)
		return
	}
	state, err1 := randomToken()
	nonce, err2 := randomToken()
	if err := errors.Join(err1, err2); err != nil {
		s.log.Error("random generation failed", "err", err)
		apierr.Write(w, http.StatusInternalServerError, apierr.Internal, msgInternal)
		return
	}
	f := flowState{State: state, Nonce: nonce, Verifier: oauth2.GenerateVerifier(),
		Expires: s.now().Add(flowLifetime).Unix()}
	v, err := s.signFlow(f)
	if err != nil {
		s.log.Error("signing oidc flow failed", "err", err)
		apierr.Write(w, http.StatusInternalServerError, apierr.Internal, msgInternal)
		return
	}
	http.SetCookie(w, flowCookieWith(v, int(flowLifetime/time.Second)))
	w.Header().Set("Cache-Control", "no-store")
	target := s.oauthConfig(p).AuthCodeURL(f.State, oidc.Nonce(f.Nonce), oauth2.S256ChallengeOption(f.Verifier))
	http.Redirect(w, r, target, http.StatusFound)
}

// callbackError redirects to the sign-in page with only an error code.
func callbackError(w http.ResponseWriter, r *http.Request, code string) {
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/login?error="+url.QueryEscape(code), http.StatusFound)
}

// handleOIDCCallback is GET /auth/callback (AUTH-1, AUTH-2).
func (s *Service) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.OIDC.Enabled() {
		apierr.Write(w, http.StatusNotFound, apierr.NotFound, "not found")
		return
	}
	// The flow cookie is single use: clear it whatever happens next.
	http.SetCookie(w, flowCookieWith("", -1))
	fail := func(reason string, err error) {
		s.log.Warn("oidc sign-in failed", "reason", reason, "err", err)
		callbackError(w, r, apierr.Unauthenticated)
	}
	c, err := r.Cookie(flowCookie)
	if err != nil {
		fail("missing flow cookie", nil)
		return
	}
	f, ok := s.openFlow(c.Value)
	if !ok {
		fail("invalid or expired flow cookie", nil)
		return
	}
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		fail("issuer returned an error", errors.New(truncate(e, 64)))
		return
	}
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(f.State)) != 1 {
		fail("state mismatch", nil)
		return
	}
	code := q.Get("code")
	if code == "" {
		fail("missing code", nil)
		return
	}
	p, err := s.oidcProvider()
	if err != nil {
		s.log.Error("oidc discovery failed", "err", err)
		callbackError(w, r, apierr.Internal)
		return
	}
	ctx := s.clientCtx(r.Context())
	tok, err := s.oauthConfig(p).Exchange(ctx, code, oauth2.VerifierOption(f.Verifier))
	if err != nil {
		fail("code exchange failed", exchangeErr(err))
		return
	}
	raw, _ := tok.Extra("id_token").(string)
	if raw == "" {
		fail("no id token in token response", nil)
		return
	}
	idt, err := p.Verifier(&oidc.Config{ClientID: s.cfg.OIDC.ClientID, Now: s.now}).Verify(ctx, raw)
	if err != nil {
		fail("id token verification failed", err)
		return
	}
	if subtle.ConstantTimeCompare([]byte(idt.Nonce), []byte(f.Nonce)) != 1 {
		fail("nonce mismatch", nil)
		return
	}
	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := idt.Claims(&claims); err != nil {
		fail("bad id token claims", err)
		return
	}
	if idt.Subject == "" {
		fail("empty subject", nil)
		return
	}
	now := s.now()
	su, created, err := s.st.UpsertOIDCLogin(r.Context(), idt.Subject, claims.Email, claims.Name, now)
	if err != nil {
		s.log.Error("storing oidc user failed", "err", err)
		callbackError(w, r, apierr.Internal)
		return
	}
	if created && s.em != nil {
		s.em.Emit(hourglass.Event{Type: hourglass.EventUserPending, Time: now,
			User: &hourglass.User{ID: su.ID, Username: su.Username, Email: su.Email, Name: su.Name, Roles: []string{}}})
	}
	if _, err := s.startSession(r.Context(), w, r, su.ID); err != nil {
		s.log.Error("creating session failed", "err", err)
		callbackError(w, r, apierr.Internal)
		return
	}
	u, err := s.loadUser(r.Context(), su.ID)
	if err != nil {
		s.log.Error("loading user failed", "err", err)
		callbackError(w, r, apierr.Internal)
		return
	}
	target := "/"
	if !u.Authorized() {
		target = "/unauthorized"
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, target, http.StatusFound)
}

// exchangeErr reduces a token endpoint error to its OAuth error code and
// HTTP status, dropping the response body.
func exchangeErr(err error) error {
	var re *oauth2.RetrieveError
	if errors.As(err, &re) {
		status := 0
		if re.Response != nil {
			status = re.Response.StatusCode
		}
		return errors.New("token endpoint: status " + http.StatusText(status) + ", error " + truncate(re.ErrorCode, 64))
	}
	return errors.New("token endpoint request failed")
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
