// SPDX-License-Identifier: MIT

// Verification task 23: attack tests against internal/auth. Not product code.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/store"
)

// attackIssuer is fakeIssuer with a token endpoint whose ID token the test
// builds itself (any claims, any header, any key).
func newAttackOIDCEnv(t *testing.T, mint func(f *fakeIssuer, nonce string) string) (*env, *fakeIssuer) {
	var e *env
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	f := &fakeIssuer{t: t, key: key, now: func() time.Time { return e.clock.Now() }, codes: map[string]authz{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", f.discovery)
	mux.HandleFunc("GET /jwks", f.jwks)
	mux.HandleFunc("GET /authorize", f.authorize)
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		a := f.codes[r.PostForm.Get("code")]
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at-not-real", "token_type": "Bearer", "id_token": mint(f, a.nonce)})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	e = newEnv(t, Config{OIDC: OIDCConfig{IssuerURL: f.srv.URL, ClientID: testClientID, ClientSecret: testClientSecret, RedirectURL: testRedirect}})
	return e, f
}

func claims(f *fakeIssuer, nonce string) map[string]any {
	now := f.now()
	return map[string]any{"iss": f.srv.URL, "sub": "sub-mallory", "aud": testClientID, "nonce": nonce,
		"iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "email": "admin@example.test", "name": "admin"}
}

func rawJWT(header, body map[string]any, sig string) string {
	h, _ := json.Marshal(header)
	c, _ := json.Marshal(body)
	return b64(h) + "." + b64(c) + "." + sig
}

func TestZZ_OIDC_TokenForgeries(t *testing.T) {
	cases := map[string]func(f *fakeIssuer, nonce string) string{
		"wrong issuer": func(f *fakeIssuer, n string) string {
			c := claims(f, n)
			c["iss"] = "https://evil.example"
			return f.sign(c)
		},
		"alg none": func(f *fakeIssuer, n string) string {
			return rawJWT(map[string]any{"alg": "none", "typ": "JWT"}, claims(f, n), "")
		},
		"HS256 with client secret": func(f *fakeIssuer, n string) string {
			h, _ := json.Marshal(map[string]any{"alg": "HS256", "typ": "JWT", "kid": "k1"})
			c, _ := json.Marshal(claims(f, n))
			in := b64(h) + "." + b64(c)
			m := hmac.New(sha256.New, []byte(testClientSecret))
			m.Write([]byte(in))
			return in + "." + b64(m.Sum(nil))
		},
		"signed by another key": func(f *fakeIssuer, n string) string {
			other, _ := rsa.GenerateKey(rand.Reader, 2048)
			g := *f
			g.key = other
			return g.sign(claims(f, n))
		},
		"no exp": func(f *fakeIssuer, n string) string {
			c := claims(f, n)
			delete(c, "exp")
			return f.sign(c)
		},
		"aud array without client": func(f *fakeIssuer, n string) string {
			c := claims(f, n)
			c["aud"] = []string{"other-client"}
			return f.sign(c)
		},
		"empty sub": func(f *fakeIssuer, n string) string {
			c := claims(f, n)
			c["sub"] = ""
			return f.sign(c)
		},
	}
	for name, mint := range cases {
		t.Run(name, func(t *testing.T) {
			e, _ := newAttackOIDCEnv(t, mint)
			r := signIn(t, e)
			if r.location != "/login?error=unauthenticated" || r.session != nil {
				t.Fatalf("FORGERY ACCEPTED: %q session=%v", r.location, r.session != nil)
			}
			if users, _ := e.st.ListUsers(e.ctx(), ""); len(users) != 0 {
				t.Fatal("user created")
			}
		})
	}
}

// An IdP user called "admin" with any email is a separate pending user; the
// local admin is untouched and stays the only one POST /auth/local accepts.
func TestZZ_OIDC_UserNamedAdminCannotBecomeLocalAdmin(t *testing.T) {
	e, _ := newAttackOIDCEnv(t, func(f *fakeIssuer, n string) string {
		c := claims(f, n)
		c["preferred_username"] = "admin"
		c["sub"] = "admin"
		return f.sign(c)
	})
	adm := e.localAdmin()
	r := signIn(t, e)
	if r.location != "/unauthorized" || r.session == nil {
		t.Fatalf("got %+v", r)
	}
	v := decodeView(t, e.do(req{method: "GET", path: "/api/v1/session", cookie: r.session.Value}))
	if v.UserID == adm.ID || v.IsLocalAdmin || len(v.Roles) != 0 || v.Username != "" {
		t.Fatalf("collided with local admin: %+v", v)
	}
	u, err := e.st.GetUserByUsername(e.ctx(), "admin")
	if err != nil || u.ID != adm.ID || u.AuthSource != store.AuthLocal {
		t.Fatalf("username admin resolves to %+v %v", u, err)
	}
}

// The flow cookie is cleared on the callback but not tracked server-side: a
// copy can be replayed with a second code for the same state within 10 min.
func TestZZ_OIDC_FlowCookieReplay(t *testing.T) {
	e, iss := newOIDCEnv(t)
	fc, cb := startFlow(t, e)
	first := callback(t, e, fc, cb)
	if first.session == nil {
		t.Fatal("first sign-in failed")
	}
	// Ask the issuer for another code with the same state/challenge.
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
	cb2, _ := url.Parse(res.Header.Get("Location"))
	second := callback(t, e, fc, cb2)
	t.Logf("replayed flow cookie accepted: %v (location %q)", second.session != nil, second.location)
}

func TestZZ_CSRF_QueryStringAndForeignSessionAndCase(t *testing.T) {
	e := newEnv(t, Config{})
	a := e.user("sub-a", store.StatusActive, store.RoleAdmin)
	b := e.user("sub-b", store.StatusActive, store.RoleAdmin)
	ca, ta := e.login(a.ID)
	_, tb := e.login(b.ID)
	// Token in the query string only.
	wantErr(t, e.do(req{method: "PUT", path: "/api/v1/admin/settings?X-CSRF-Token=" + ta + "&csrf_token=" + ta, cookie: ca}), http.StatusForbidden, apierr.CSRF)
	// Another session's token.
	wantErr(t, e.do(req{method: "PUT", path: "/api/v1/admin/settings", cookie: ca, csrf: tb}), http.StatusForbidden, apierr.CSRF)
	// Upper-cased hex of the right token.
	wantErr(t, e.do(req{method: "PUT", path: "/api/v1/admin/settings", cookie: ca, csrf: strings.ToUpper(ta)}), http.StatusForbidden, apierr.CSRF)
	// Token in a cookie named like the header.
	hr := httptest.NewRequest("PUT", "https://hourglass.example/api/v1/admin/settings", nil)
	hr.AddCookie(&http.Cookie{Name: SessionCookie, Value: ca})
	hr.AddCookie(&http.Cookie{Name: "X-CSRF-Token", Value: ta})
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, hr)
	wantErr(t, rec, http.StatusForbidden, apierr.CSRF)
	// Logout without the token keeps the session.
	wantErr(t, e.do(req{method: "POST", path: "/auth/logout", cookie: ca}), http.StatusForbidden, apierr.CSRF)
	if r := e.do(req{method: "GET", path: "/api/v1/session", cookie: ca}); r.Code != http.StatusOK {
		t.Fatalf("session lost: %d", r.Code)
	}
	// Methods other than GET/HEAD/OPTIONS: PATCH and a made-up verb need it too.
	for _, m := range []string{"PATCH", "PROPFIND", "DELETE"} {
		rec := e.do(req{method: m, path: "/api/v1/admin/settings", cookie: ca})
		if rec.Code == http.StatusOK {
			t.Fatalf("%s without CSRF reached the handler", m)
		}
	}
}

func TestZZ_RateLimit_BypassAttempts(t *testing.T) {
	e := newEnv(t, Config{})
	e.localAdmin()
	bad := func(user string, hdr map[string]string) int {
		return e.do(req{method: "POST", path: "/auth/local", body: map[string]string{"username": user, "password": "wrong-password-x"}, header: hdr}).Code
	}
	codes := []int{
		bad("admin", nil), bad("ADMIN", nil), bad(" admin", nil), bad("nobody", nil),
		bad("admin", map[string]string{"X-Forwarded-For": "10.9.9.9"}),
	}
	for _, c := range codes {
		if c != http.StatusUnauthorized {
			t.Fatalf("early codes %v", codes)
		}
	}
	// Sixth attempt, any variant: limited.
	for _, u := range []string{"admin", "Admin", "x", "admin\t"} {
		if c := bad(u, map[string]string{"X-Forwarded-For": "1.2.3.4", "X-Real-IP": "1.2.3.5"}); c != http.StatusTooManyRequests {
			t.Fatalf("%q after 5 failures: %d", u, c)
		}
	}
	// Right password is refused too while limited (global lockout).
	if c := e.do(req{method: "POST", path: "/auth/local", body: map[string]string{"username": "admin", "password": adminPW}}).Code; c != http.StatusTooManyRequests {
		t.Fatalf("right password while limited: %d", c)
	}
	// Attempts while limited do not extend the window.
	e.clock.Advance(15*time.Minute + time.Second)
	if c := e.do(req{method: "POST", path: "/auth/local", body: map[string]string{"username": "admin", "password": adminPW}}).Code; c != http.StatusOK {
		t.Fatalf("after window: %d", c)
	}
}

func TestZZ_RateLimit_Concurrency(t *testing.T) {
	e := newEnv(t, Config{})
	e.localAdmin()
	var wg sync.WaitGroup
	var mu sync.Mutex
	verified := 0
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := e.do(req{method: "POST", path: "/auth/local", body: map[string]string{"username": "admin", "password": "wrong-password-x"}}).Code
			if c == http.StatusUnauthorized {
				mu.Lock()
				verified++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if verified != 5 {
		t.Fatalf("%d of 40 concurrent guesses were checked, want exactly 5", verified)
	}
}

// Sustained lockout: one attacker request whenever a slot frees keeps the
// right password refused indefinitely (the limiter is global, not per client).
func TestZZ_RateLimit_SustainedLockout(t *testing.T) {
	e := newEnv(t, Config{})
	e.localAdmin()
	good := map[string]string{"username": "admin", "password": adminPW}
	refused := 0
	for minute := 0; minute < 6*60; minute++ { // six hours
		if minute%3 == 0 { // attacker: 5 guesses per 15 minutes, spread out
			e.do(req{method: "POST", path: "/auth/local", body: map[string]string{"username": "x", "password": "y"}})
		}
		if minute%30 == 1 { // the real admin tries every 30 minutes
			if e.do(req{method: "POST", path: "/auth/local", body: good}).Code == http.StatusTooManyRequests {
				refused++
			}
		}
		e.clock.Advance(time.Minute)
	}
	t.Logf("admin refused %d of 12 attempts over 6 hours by an unauthenticated attacker sending 20 requests/hour", refused)
	if refused == 12 {
		t.Log("FINDING: permanent lockout of the local admin")
	}
}

func TestZZ_PHCMalformedNeverPanics(t *testing.T) {
	inputs := []string{"", "$", "$$$$$", "$argon2id$v=19$m=65536,t=3,p=4$$", "$argon2id$v=19$m=65536,t=3,p=0$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA",
		"$argon2id$v=19$m=4294967295,t=3,p=4$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA", "$argon2id$v=19$m=65536,t=99999,p=4$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA",
		"$argon2id$v=18$m=65536,t=3,p=4$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA", "$argon2i$v=19$m=65536,t=3,p=4$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA",
		"$argon2id$v=19$m=65536,t=3,p=255$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA", "$argon2id$v=19$m=-1,t=3,p=4$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA",
		"$argon2id$v=19$m=65536,t=3,p=4$!!!$aGFzaGhhc2hoYXNoaGFzaA", "$argon2id$v=19$m=65536,t=3,p=4,x=1$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA"}
	for _, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on %q: %v", in, r)
				}
			}()
			start := time.Now()
			ok, err := VerifyPassword(in, "pw")
			t.Logf("%-90q ok=%v err=%v %s", in, ok, err, time.Since(start).Round(time.Millisecond))
		}()
	}
}

func TestZZ_TimingUnknownVsWrong(t *testing.T) {
	e := newEnv(t, Config{})
	e.localAdmin()
	measure := func(user string) time.Duration {
		var total time.Duration
		for range 4 {
			e.clock.Advance(16 * time.Minute) // stay under the limit
			s := time.Now()
			e.do(req{method: "POST", path: "/auth/local", body: map[string]string{"username": user, "password": "wrong-password-x"}})
			total += time.Since(s)
		}
		return total / 4
	}
	k, u := measure("admin"), measure("no-such-user")
	t.Logf("wrong password %s, unknown user %s", k, u)
}
