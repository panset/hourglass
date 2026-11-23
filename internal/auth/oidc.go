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
	"github.com/panset/hourglass/internal/store"
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

// useFlow marks a flow as used and reports whether it was fresh. The signed
// cookie is stateless, so single use (AUTH-1) needs the server to remember the
// state until the cookie would have expired anyway.
func (s *Service) useFlow(f flowState) bool {
	now := s.now()
	s.usedMu.Lock()
	defer s.usedMu.Unlock()
	if s.usedFlows == nil {
		s.usedFlows = map[string]time.Time{}
	}
	for k, exp := range s.usedFlows {
		if !now.Before(exp) {
			delete(s.usedFlows, k)
		}
	}
	if _, used := s.usedFlows[f.State]; used {
		return false
	}
	s.usedFlows[f.State] = time.Unix(f.Expires, 0)
	return true
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

// callbackError redirects back to the UI root with only an error code; the
// UI decides what to show (UI-7).
func callbackError(w http.ResponseWriter, r *http.Request, code string) {
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/?auth_error="+url.QueryEscape(code), http.StatusFound)
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
	if !s.useFlow(f) {
		fail("flow cookie already used", nil)
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
		Name  string `json:"name"` // a non-string fails sign-in, as at d96a950
		// The fallback claims never fail a sign-in: a value that is not a
		// string counts as empty (AUTH-9).
		GivenName         json.RawMessage `json:"given_name"`
		FamilyName        json.RawMessage `json:"family_name"`
		PreferredUsername json.RawMessage `json:"preferred_username"`
		// EmailVerified is a boolean or the string "true" / "false"; absent
		// counts as verified (D1).
		EmailVerified json.RawMessage `json:"email_verified"`
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
	su, created, err := s.st.UpsertOIDCLogin(r.Context(), idt.Subject, claims.Email,
		storedName(claims.Name, claimString(claims.GivenName), claimString(claims.FamilyName), claimString(claims.PreferredUsername)), now)
	if err != nil {
		s.log.Error("storing oidc user failed", "err", err)
		callbackError(w, r, apierr.Internal)
		return
	}
	// A pre-approved user is approved here and gets user.approved instead of
	// user.pending. A failed approval leaves the user pending and the sign-in
	// goes on (PRE-8).
	approved := s.applyPreapproval(r.Context(), su, claims.Email, claims.EmailVerified)
	if created && !approved && s.em != nil {
		s.em.Emit(hourglass.Event{Type: hourglass.EventUserPending, Time: now,
			User: &hourglass.User{ID: su.ID, Username: su.Username, Email: su.Email, Name: su.Name, Roles: []string{}}})
	}
	if _, err := s.startSession(r.Context(), w, r, su.ID); err != nil {
		s.log.Error("creating session failed", "err", err)
		callbackError(w, r, apierr.Internal)
		return
	}
	// UI-7: always back to the UI root; the shell reads GET /api/v1/session
	// and shows the not-authorized page for a user without a role.
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/", http.StatusFound)
}

// applyPreapproval runs the pre-approval step (D1) for a signed-in SSO user
// and reports whether it approved them. It applies only to a pending SSO user
// whose email claim is not empty and whose email_verified claim is not false;
// the step itself checks the status again in its transaction and looks the
// email up. An error is logged, never returned: the sign-in succeeds and the
// user stays pending.
func (s *Service) applyPreapproval(ctx context.Context, su store.User, email string, verified json.RawMessage) bool {
	if s.pre == nil || su.AuthSource != store.AuthOIDC || su.Status != store.StatusPending {
		return false
	}
	if strings.TrimSpace(email) == "" || !emailVerified(verified) {
		return false
	}
	ok, err := s.pre.ApplyPreapproval(ctx, su.ID, email)
	if err != nil {
		s.log.Error("applying a pre-approval failed; the user stays pending", "user_id", su.ID, "err", err)
		return false
	}
	return ok
}

// emailVerified reads the email_verified claim (D1): absent or null is
// verified, because several enterprise providers never send it; true and the
// string "true" are verified; false, the string "false" and any other value
// are not.
func emailVerified(raw json.RawMessage) bool {
	v := strings.TrimSpace(string(raw))
	if v == "" || v == "null" {
		return true
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return strings.EqualFold(strings.TrimSpace(str), "true")
	}
	return false
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

// storedName is the name kept in users.name for an SSO user (AUTH-9): the
// first non-empty of name; given_name and family_name joined by a space and
// trimmed; preferred_username. Whitespace-only claims count as empty. All
// empty gives an empty name.
// claimString is a claim's value when it is a JSON string, else "".
func claimString(raw json.RawMessage) string {
	var s string
	if len(raw) == 0 || json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

func storedName(name, given, family, preferred string) string {
	if strings.TrimSpace(name) != "" {
		return name // stored exactly as sent, as before
	}
	if n := strings.TrimSpace(strings.TrimSpace(given) + " " + strings.TrimSpace(family)); n != "" {
		return n
	}
	return strings.TrimSpace(preferred)
}
