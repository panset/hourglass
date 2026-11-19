// SPDX-License-Identifier: MIT

package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

const (
	testClientID     = "hourglass-test"
	testClientSecret = "test-client-secret-not-real"
	testRedirect     = "https://hourglass.example/auth/callback"
)

// fakeIssuer is a minimal OIDC provider: discovery, JWKS, an authorize
// endpoint and a token endpoint that checks the PKCE verifier and signs
// RS256 ID tokens.
type fakeIssuer struct {
	t   *testing.T
	srv *httptest.Server
	key *rsa.PrivateKey
	now func() time.Time

	mu    sync.Mutex
	codes map[string]authz // code -> request
	// Knobs for the next ID token.
	sub, email, name string
	audience         string        // "" = the client id
	nonceOverride    string        // "" = the requested nonce
	ttl              time.Duration // 0 = 5 minutes
	tokenRequests    int
}

type authz struct {
	challenge, method, nonce, redirect string
}

func newFakeIssuer(t *testing.T, now func() time.Time) *fakeIssuer {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeIssuer{t: t, key: key, now: now, codes: map[string]authz{}, sub: "sub-alice", email: "alice@example.test", name: "Alice"}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", f.discovery)
	mux.HandleFunc("GET /jwks", f.jwks)
	mux.HandleFunc("GET /authorize", f.authorize)
	mux.HandleFunc("POST /token", f.token)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeIssuer) discovery(w http.ResponseWriter, _ *http.Request) {
	u := f.srv.URL
	_ = json.NewEncoder(w).Encode(map[string]any{
		"issuer": u, "authorization_endpoint": u + "/authorize", "token_endpoint": u + "/token",
		"jwks_uri": u + "/jwks", "response_types_supported": []string{"code"},
		"subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func (f *fakeIssuer) jwks(w http.ResponseWriter, _ *http.Request) {
	pub := f.key.PublicKey
	_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "kid": "k1", "use": "sig", "alg": "RS256",
		"n": b64(pub.N.Bytes()), "e": b64(big.NewInt(int64(pub.E)).Bytes()),
	}}})
}

func (f *fakeIssuer) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("client_id") != testClientID || q.Get("response_type") != "code" || q.Get("redirect_uri") != testRedirect {
		http.Error(w, "bad authorize request", http.StatusBadRequest)
		return
	}
	scopes := strings.Fields(q.Get("scope"))
	if strings.Join(scopes, " ") != "openid email profile" {
		http.Error(w, "bad scope "+q.Get("scope"), http.StatusBadRequest)
		return
	}
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("nonce") == "" || q.Get("state") == "" {
		http.Error(w, "missing pkce, nonce or state", http.StatusBadRequest)
		return
	}
	code := b64([]byte(q.Get("state")))[:16] + "code"
	f.mu.Lock()
	f.codes[code] = authz{challenge: q.Get("code_challenge"), method: q.Get("code_challenge_method"), nonce: q.Get("nonce"), redirect: q.Get("redirect_uri")}
	f.mu.Unlock()
	http.Redirect(w, r, testRedirect+"?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
}

func (f *fakeIssuer) tokenError(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func (f *fakeIssuer) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		f.tokenError(w, "invalid_request")
		return
	}
	f.mu.Lock()
	f.tokenRequests++
	a, ok := f.codes[r.PostForm.Get("code")]
	delete(f.codes, r.PostForm.Get("code"))
	f.mu.Unlock()
	id, secret, basic := r.BasicAuth()
	if !basic {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if id != testClientID || secret != testClientSecret {
		f.tokenError(w, "invalid_client")
		return
	}
	if !ok || r.PostForm.Get("grant_type") != "authorization_code" || r.PostForm.Get("redirect_uri") != a.redirect {
		f.tokenError(w, "invalid_grant")
		return
	}
	v := r.PostForm.Get("code_verifier")
	sum := sha256.Sum256([]byte(v))
	if v == "" || a.method != "S256" || b64(sum[:]) != a.challenge {
		f.tokenError(w, "invalid_grant")
		return
	}
	f.mu.Lock()
	aud, nonce, ttl := f.audience, f.nonceOverride, f.ttl
	sub, email, name := f.sub, f.email, f.name
	f.mu.Unlock()
	if aud == "" {
		aud = testClientID
	}
	if nonce == "" {
		nonce = a.nonce
	}
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	now := f.now()
	idt := f.sign(map[string]any{"iss": f.srv.URL, "sub": sub, "aud": aud, "nonce": nonce,
		"iat": now.Unix(), "exp": now.Add(ttl).Unix(), "email": email, "name": name})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at-not-real", "token_type": "Bearer", "expires_in": 300, "id_token": idt})
}

func (f *fakeIssuer) sign(claims map[string]any) string {
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": "k1"})
	c, _ := json.Marshal(claims)
	in := b64(h) + "." + b64(c)
	sum := sha256.Sum256([]byte(in))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, sum[:])
	if err != nil {
		f.t.Fatal(err)
	}
	return in + "." + b64(sig)
}

func (f *fakeIssuer) requests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tokenRequests
}

func (f *fakeIssuer) set(fn func(f *fakeIssuer)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func newOIDCEnv(t *testing.T) (*env, *fakeIssuer) {
	var e *env
	iss := newFakeIssuer(t, func() time.Time { return e.clock.Now() })
	e = newEnv(t, Config{OIDC: OIDCConfig{IssuerURL: iss.srv.URL, ClientID: testClientID,
		ClientSecret: testClientSecret, RedirectURL: testRedirect}})
	return e, iss
}

var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// flowResult is the end of one sign-in attempt.
type flowResult struct {
	location string // callback redirect target
	session  *http.Cookie
}

// startFlow runs GET /auth/oidc/start and the issuer's authorize step,
// returning the flow cookie and the callback URL the issuer redirected to.
func startFlow(t *testing.T, e *env) (*http.Cookie, *url.URL) {
	t.Helper()
	rec := e.do(req{method: "GET", path: "/auth/oidc/start"})
	if rec.Code != http.StatusFound {
		t.Fatalf("start: %d %s", rec.Code, rec.Body.String())
	}
	fc := cookieNamed(rec, flowCookie)
	if fc == nil {
		t.Fatal("no flow cookie")
	}
	res, err := noRedirect.Get(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusFound {
		t.Fatalf("authorize: %d", res.StatusCode)
	}
	cb, err := url.Parse(res.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return fc, cb
}

func callback(t *testing.T, e *env, fc *http.Cookie, cb *url.URL) flowResult {
	t.Helper()
	hr := httptest.NewRequest("GET", "https://hourglass.example"+cb.RequestURI(), nil)
	if fc != nil {
		hr.AddCookie(&http.Cookie{Name: fc.Name, Value: fc.Value})
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, hr)
	if rec.Code != http.StatusFound {
		t.Fatalf("callback: %d %s", rec.Code, rec.Body.String())
	}
	if c := cookieNamed(rec, flowCookie); c == nil || c.MaxAge >= 0 {
		t.Fatal("flow cookie not cleared")
	}
	return flowResult{location: rec.Header().Get("Location"), session: cookieNamed(rec, SessionCookie)}
}

func signIn(t *testing.T, e *env) flowResult {
	t.Helper()
	fc, cb := startFlow(t, e)
	return callback(t, e, fc, cb)
}

func wantFailure(t *testing.T, r flowResult) {
	t.Helper()
	if r.location != "/?auth_error=unauthenticated" || r.session != nil {
		t.Fatalf("want failure redirect, got %q session=%v", r.location, r.session != nil)
	}
}

func TestAUTH1_FullCodeFlow(t *testing.T) {
	e, iss := newOIDCEnv(t)
	rec := e.do(req{method: "GET", path: "/api/v1/auth/methods"})
	if strings.TrimSpace(rec.Body.String()) != `{"oidc":true,"local":true}` {
		t.Fatalf("methods %s", rec.Body.String())
	}
	start := e.do(req{method: "GET", path: "/auth/oidc/start"})
	loc, _ := url.Parse(start.Header().Get("Location"))
	q := loc.Query()
	if !strings.HasPrefix(loc.String(), iss.srv.URL+"/authorize?") || q.Get("code_challenge_method") != "S256" ||
		q.Get("state") == "" || q.Get("nonce") == "" || q.Get("code_challenge") == "" || q.Get("scope") != "openid email profile" {
		t.Fatalf("authorize URL %s", loc)
	}
	fc := cookieNamed(start, flowCookie)
	if !fc.HttpOnly || !fc.Secure || fc.SameSite != http.SameSiteLaxMode || fc.MaxAge != 600 {
		t.Fatalf("flow cookie %+v", fc)
	}

	r := signIn(t, e)
	if r.location != "/" || r.session == nil {
		t.Fatalf("result %+v", r)
	}
	if !r.session.HttpOnly || !r.session.Secure || r.session.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie %+v", r.session)
	}
	v := decodeView(t, e.do(req{method: "GET", path: "/api/v1/session", cookie: r.session.Value}))
	if v.AuthSource != store.AuthOIDC || v.Email != "alice@example.test" || v.Name != "Alice" || len(v.Roles) != 0 || v.Status != store.StatusPending {
		t.Fatalf("view %+v", v)
	}
	// No token is stored or returned, and the secret is never logged.
	logs := e.logs.String()
	if strings.Contains(logs, testClientSecret) || strings.Contains(logs, "at-not-real") {
		t.Fatal("secret in logs")
	}

	// With a role the callback also lands on "/" (UI-7: the UI decides).
	u, _ := e.st.GetUserBySub(e.ctx(), "sub-alice")
	_ = e.st.SetUserStatus(e.ctx(), u.ID, store.StatusActive)
	_ = e.st.SetUserRoles(e.ctx(), u.ID, []string{store.RoleDeveloper})
	if r := signIn(t, e); r.location != "/" || r.session == nil {
		t.Fatalf("second sign-in %+v", r)
	}
}

func TestAUTH2_FirstSignInPendingThenRefresh(t *testing.T) {
	e, iss := newOIDCEnv(t)
	signIn(t, e)
	u, err := e.st.GetUserBySub(e.ctx(), "sub-alice")
	if err != nil {
		t.Fatal(err)
	}
	if roles, _ := e.st.GetUserRoles(e.ctx(), u.ID); u.Status != store.StatusPending || len(roles) != 0 {
		t.Fatalf("user %+v roles %v", u, roles)
	}
	evs := e.em.list()
	if len(evs) != 1 || evs[0].Type != hourglass.EventUserPending || evs[0].User == nil || evs[0].User.ID != u.ID {
		t.Fatalf("events %+v", evs)
	}

	iss.set(func(f *fakeIssuer) { f.email, f.name = "alice@new.example.test", "Alice Renamed" })
	e.clock.Advance(time.Hour)
	signIn(t, e)
	u2, _ := e.st.GetUserBySub(e.ctx(), "sub-alice")
	if u2.ID != u.ID || u2.Email != "alice@new.example.test" || u2.Name != "Alice Renamed" {
		t.Fatalf("not refreshed: %+v", u2)
	}
	if n := len(e.em.list()); n != 1 {
		t.Fatalf("second sign-in emitted: %d events", n)
	}
	if users, _ := e.st.ListUsers(e.ctx(), ""); len(users) != 1 {
		t.Fatalf("%d users", len(users))
	}
}

func TestAUTH1_WrongState(t *testing.T) {
	e, _ := newOIDCEnv(t)
	fc, cb := startFlow(t, e)
	q := cb.Query()
	q.Set("state", "forged-state")
	cb.RawQuery = q.Encode()
	wantFailure(t, callback(t, e, fc, cb))
	// No flow cookie at all.
	_, cb2 := startFlow(t, e)
	wantFailure(t, callback(t, e, nil, cb2))
	// A tampered flow cookie.
	fc3, cb3 := startFlow(t, e)
	fc3.Value = "x" + fc3.Value[1:]
	wantFailure(t, callback(t, e, fc3, cb3))
}

func TestAUTH1_IssuerError(t *testing.T) {
	e, _ := newOIDCEnv(t)
	fc, cb := startFlow(t, e)
	q := cb.Query()
	q.Set("error", "access_denied")
	cb.RawQuery = q.Encode()
	wantFailure(t, callback(t, e, fc, cb))
}

func TestAUTH1_FlowCookieExpires(t *testing.T) {
	e, _ := newOIDCEnv(t)
	fc, cb := startFlow(t, e)
	e.clock.Advance(10 * time.Minute)
	wantFailure(t, callback(t, e, fc, cb))
}

func TestAUTH1_IncorrectOrMissingVerifier(t *testing.T) {
	e, iss := newOIDCEnv(t)
	fc, cb := startFlow(t, e)
	f, ok := e.svc.openFlow(fc.Value)
	if !ok {
		t.Fatal("cannot open own flow cookie")
	}
	f.Verifier = "a-different-verifier-that-is-long-enough-1234567890"
	v, _ := e.svc.signFlow(f)
	fc.Value = v
	before := iss.requests()
	wantFailure(t, callback(t, e, fc, cb))
	if iss.requests() <= before {
		t.Fatal("token endpoint not reached; the verifier was not what failed")
	}

	// Missing verifier: refused before any exchange.
	fc2, cb2 := startFlow(t, e)
	f2, _ := e.svc.openFlow(fc2.Value)
	f2.Verifier = ""
	fc2.Value, _ = e.svc.signFlow(f2)
	wantFailure(t, callback(t, e, fc2, cb2))

	// The fake issuer itself rejects an exchange without a verifier.
	_, cb3 := startFlow(t, e)
	res, err := http.PostForm(iss.srv.URL+"/token", url.Values{"grant_type": {"authorization_code"}, "code": {cb3.Query().Get("code")},
		"redirect_uri": {testRedirect}, "client_id": {testClientID}, "client_secret": {testClientSecret}})
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("token without verifier: %d", res.StatusCode)
	}
	if users, _ := e.st.ListUsers(e.ctx(), ""); len(users) != 0 {
		t.Fatalf("user created on failure")
	}
}

func TestAUTH1_WrongNonce(t *testing.T) {
	e, iss := newOIDCEnv(t)
	iss.set(func(f *fakeIssuer) { f.nonceOverride = "replayed-nonce" })
	wantFailure(t, signIn(t, e))
	if len(e.em.list()) != 0 {
		t.Fatal("event emitted on failure")
	}
}

func TestAUTH1_WrongAudience(t *testing.T) {
	e, iss := newOIDCEnv(t)
	iss.set(func(f *fakeIssuer) { f.audience = "some-other-client" })
	wantFailure(t, signIn(t, e))
}

func TestAUTH1_ExpiredToken(t *testing.T) {
	e, iss := newOIDCEnv(t)
	iss.set(func(f *fakeIssuer) { f.ttl = -time.Minute })
	wantFailure(t, signIn(t, e))
	if users, _ := e.st.ListUsers(e.ctx(), ""); len(users) != 0 {
		t.Fatal("user created from an expired token")
	}
}

func TestAUTH1_SignInReplacesExistingSession(t *testing.T) {
	e, _ := newOIDCEnv(t)
	first := signIn(t, e)
	fc, cb := startFlow(t, e)
	hr := httptest.NewRequest("GET", "https://hourglass.example"+cb.RequestURI(), nil)
	hr.AddCookie(&http.Cookie{Name: fc.Name, Value: fc.Value})
	hr.AddCookie(&http.Cookie{Name: SessionCookie, Value: first.session.Value})
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, hr)
	if c := cookieNamed(rec, SessionCookie); c == nil || c.Value == first.session.Value {
		t.Fatal("session not replaced")
	}
	if r := e.do(req{method: "GET", path: "/api/v1/session", cookie: first.session.Value}); r.Code != http.StatusUnauthorized {
		t.Fatalf("old session still valid: %d", r.Code)
	}
}

// AUTH-1: the flow cookie is single use on the server too. A copy replayed with
// a second code for the same state is refused (task 23 finding 8).
func TestAUTH1_FlowCookieReplayRefused(t *testing.T) {
	e, iss := newOIDCEnv(t)
	fc, cb := startFlow(t, e)
	if first := callback(t, e, fc, cb); first.session == nil {
		t.Fatal("first sign-in failed")
	}
	f, _ := e.svc.openFlow(fc.Value)
	sum := sha256.Sum256([]byte(f.Verifier))
	q := url.Values{"client_id": {testClientID}, "response_type": {"code"}, "redirect_uri": {testRedirect},
		"scope": {"openid email profile"}, "code_challenge_method": {"S256"}, "code_challenge": {b64(sum[:])},
		"nonce": {f.Nonce}, "state": {f.State}}
	res, err := noRedirect.Get(iss.srv.URL + "/authorize?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	cb2, err := url.Parse(res.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	wantFailure(t, callback(t, e, fc, cb2))
	// A fresh flow still works.
	if again := signIn(t, e); again.session == nil {
		t.Fatal("a new flow was refused")
	}
}

// Auth JSON responses carry nosniff like the API's (task 23 finding 10).
func TestAuthJSONResponsesSetNosniff(t *testing.T) {
	e := newEnv(t, Config{})
	for _, p := range []string{"/api/v1/auth/methods"} {
		rec := e.do(req{method: "GET", path: p})
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: no nosniff header: %v", p, rec.Header())
		}
	}
	rec := e.do(req{method: "POST", path: "/auth/local", body: `{"username":"x","password":"y"}`, header: map[string]string{"Content-Type": "application/json"}})
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("auth error body: no nosniff header: %v", rec.Header())
	}
}
