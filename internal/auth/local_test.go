// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/store"
)

const adminPW = "initial-password-123"

func (e *env) localAdmin() store.User {
	e.t.Helper()
	u, err := EnsureLocalAdmin(e.ctx(), e.st, adminPW, nil)
	if err != nil {
		e.t.Fatal(err)
	}
	return u
}

func TestAUTH7_PasswordRules(t *testing.T) {
	for pw, want := range map[string]error{
		"":                        ErrPasswordTooShort,
		"elevenchars":             ErrPasswordTooShort,
		"twelve-chars":            nil,
		"ünïcödé-twlv":            nil, // 12 runes, more bytes
		strings.Repeat("a", 1025): ErrPasswordTooLong,
	} {
		if got := ValidatePassword(pw); !errors.Is(got, want) {
			t.Errorf("ValidatePassword(len %d) = %v, want %v", len(pw), got, want)
		}
	}
}

func TestAUTH7_Argon2idPHCHash(t *testing.T) {
	h1, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	h2, _ := HashPassword("correct horse battery")
	if !regexp.MustCompile(`^\$argon2id\$v=19\$m=65536,t=3,p=4\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$`).MatchString(h1) {
		t.Fatalf("not a PHC argon2id string: %s", h1)
	}
	if h1 == h2 {
		t.Fatal("salt is not random")
	}
	if ok, err := VerifyPassword(h1, "correct horse battery"); !ok || err != nil {
		t.Fatalf("verify = %v, %v", ok, err)
	}
	if ok, _ := VerifyPassword(h1, "correct horse batterY"); ok {
		t.Fatal("wrong password verified")
	}
	if _, err := VerifyPassword("$argon2i$v=19$m=65536,t=3,p=4$abc$def", "x"); err == nil {
		t.Fatal("malformed hash accepted")
	}
}

func TestAUTH7_GeneratedPassword(t *testing.T) {
	p, err := GeneratePassword(24)
	if err != nil || len(p) != 24 || !regexp.MustCompile(`^[A-Za-z0-9]{24}$`).MatchString(p) {
		t.Fatalf("GeneratePassword = %q, %v", p, err)
	}
}

func TestAUTH7_LocalAdminCreatedOnceWithGivenPassword(t *testing.T) {
	st := newStore(t)
	logs := &syncBuffer{}
	log := slog.New(slog.NewTextHandler(logs, nil))
	u, err := EnsureLocalAdmin(t.Context(), st, adminPW, log)
	if err != nil {
		t.Fatal(err)
	}
	if u.AuthSource != store.AuthLocal || u.Username != "admin" || u.Status != store.StatusActive || !u.MustChangePassword {
		t.Fatalf("admin row %+v", u)
	}
	if roles, _ := st.GetUserRoles(t.Context(), u.ID); len(roles) != 1 || roles[0] != store.RoleAdmin {
		t.Fatalf("roles %v", roles)
	}
	if ok, _ := VerifyPassword(u.PasswordHash, adminPW); !ok {
		t.Fatal("password not taken from the argument")
	}
	if strings.Contains(logs.String(), adminPW) {
		t.Fatal("configured password logged")
	}
	// Second start: the row exists, the supplied password is ignored.
	u2, err := EnsureLocalAdmin(t.Context(), st, "another-password-456", log)
	if err != nil || u2.ID != u.ID || u2.PasswordHash != u.PasswordHash {
		t.Fatalf("second call changed the admin: %v", err)
	}
	if users, _ := st.ListUsers(t.Context(), ""); len(users) != 1 {
		t.Fatalf("%d users", len(users))
	}
	if !IsLocalAdmin(u) {
		t.Fatal("IsLocalAdmin false")
	}
}

func TestAUTH7_LocalAdminGeneratedPasswordLoggedOnce(t *testing.T) {
	st := newStore(t)
	logs := &syncBuffer{}
	log := slog.New(slog.NewTextHandler(logs, nil))
	u, err := EnsureLocalAdmin(t.Context(), st, "", log)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`password=([A-Za-z0-9]{24})\b`).FindAllStringSubmatch(logs.String(), -1)
	if len(m) != 1 {
		t.Fatalf("generated password logged %d times: %s", len(m), logs.String())
	}
	if ok, _ := VerifyPassword(u.PasswordHash, m[0][1]); !ok {
		t.Fatal("logged password does not match the hash")
	}
	if _, err := EnsureLocalAdmin(t.Context(), st, "", log); err != nil {
		t.Fatal(err)
	}
	if n := len(regexp.MustCompile(`password=`).FindAllString(logs.String(), -1)); n != 1 {
		t.Fatalf("password logged %d times after restart", n)
	}
}

func TestAUTH7_LocalAdminShortInitialPasswordRejected(t *testing.T) {
	st := newStore(t)
	_, err := EnsureLocalAdmin(t.Context(), st, "short", nil)
	if err == nil || strings.Contains(err.Error(), "short\"") {
		t.Fatalf("err = %v", err)
	}
}

func TestAUTH6_LocalSignIn(t *testing.T) {
	e := newEnv(t, Config{})
	admin := e.localAdmin()
	rec := e.do(req{method: "POST", path: "/auth/local", body: map[string]string{"username": "admin", "password": adminPW}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	v := decodeView(t, rec)
	if v.UserID != admin.ID || !v.IsLocalAdmin || !v.MustChangePassword || v.CSRFToken == "" || len(v.Roles) != 1 {
		t.Fatalf("view %+v", v)
	}
	if c := cookieNamed(rec, SessionCookie); c == nil || c.Value == "" {
		t.Fatal("no session cookie")
	}
	if strings.Contains(rec.Body.String(), "argon2") {
		t.Fatal("hash leaked")
	}
}

func TestAUTH6_LocalSignInWrongAndUnknownSameError(t *testing.T) {
	e := newEnv(t, Config{})
	e.localAdmin()
	wrong := e.do(req{method: "POST", path: "/auth/local", body: map[string]string{"username": "admin", "password": "not-the-password"}})
	unknown := e.do(req{method: "POST", path: "/auth/local", body: map[string]string{"username": "nobody", "password": "not-the-password"}})
	wantErr(t, wrong, http.StatusUnauthorized, apierr.Unauthenticated)
	wantErr(t, unknown, http.StatusUnauthorized, apierr.Unauthenticated)
	if wrong.Body.String() != unknown.Body.String() {
		t.Fatalf("bodies differ: %s vs %s", wrong.Body, unknown.Body)
	}
	if cookieNamed(wrong, SessionCookie) != nil || cookieNamed(unknown, SessionCookie) != nil {
		t.Fatal("cookie set on failure")
	}
	if strings.Contains(e.logs.String(), "not-the-password") {
		t.Fatal("password logged")
	}
}

func TestAUTH6_LocalSignInRefusesOIDCUser(t *testing.T) {
	e := newEnv(t, Config{})
	e.localAdmin()
	u := e.user("sub-1", store.StatusActive, store.RoleAdmin)
	// OIDC users have no username or password; try their identifiers.
	for _, name := range []string{"sub-1", u.Email, u.Name} {
		rec := e.do(req{method: "POST", path: "/auth/local", body: map[string]string{"username": name, "password": adminPW}})
		wantErr(t, rec, http.StatusUnauthorized, apierr.Unauthenticated)
	}
}

func TestAUTH7_RateLimitFiveFailuresPer15Minutes(t *testing.T) {
	e := newEnv(t, Config{})
	e.localAdmin()
	bad := map[string]string{"username": "admin", "password": "wrong-password-x"}
	good := map[string]string{"username": "admin", "password": adminPW}
	// A success inside the window does not clear earlier failures.
	for i := range 5 {
		wantErr(t, e.do(req{method: "POST", path: "/auth/local", body: bad}), http.StatusUnauthorized, apierr.Unauthenticated)
		e.clock.Advance(time.Minute)
		if i == 1 {
			if rec := e.do(req{method: "POST", path: "/auth/local", body: good}); rec.Code != http.StatusOK {
				t.Fatalf("success between failures: %d", rec.Code)
			}
		}
	}
	// Now limited: even the right password gets 429 without being checked.
	rec := e.do(req{method: "POST", path: "/auth/local", body: good})
	wantErr(t, rec, http.StatusTooManyRequests, apierr.RateLimited)
	ra, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	// First failure at t0, now t0+5m: 10 more minutes.
	if err != nil || ra != 600 {
		t.Fatalf("Retry-After %q", rec.Header().Get("Retry-After"))
	}
	// Unknown usernames are limited too (one local account).
	wantErr(t, e.do(req{method: "POST", path: "/auth/local", body: map[string]string{"username": "x", "password": "y"}}),
		http.StatusTooManyRequests, apierr.RateLimited)
	e.clock.Advance(10*time.Minute - time.Second)
	wantErr(t, e.do(req{method: "POST", path: "/auth/local", body: good}), http.StatusTooManyRequests, apierr.RateLimited)
	e.clock.Advance(time.Second)
	if rec := e.do(req{method: "POST", path: "/auth/local", body: good}); rec.Code != http.StatusOK {
		t.Fatalf("after window: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAUTH7_ForcedPasswordChange(t *testing.T) {
	e := newEnv(t, Config{})
	e.localAdmin()
	rec := e.do(req{method: "POST", path: "/auth/local", body: map[string]string{"username": "admin", "password": adminPW}})
	cookie := cookieNamed(rec, SessionCookie).Value
	csrf := decodeView(t, rec).CSRFToken

	// Guard: data routes refused; session, password change and logout allowed.
	wantErr(t, e.do(req{method: "GET", path: "/api/v1/me", cookie: cookie}), http.StatusForbidden, apierr.PasswordChangeRequired)
	wantErr(t, e.do(req{method: "GET", path: "/api/v1/admin/users", cookie: cookie}), http.StatusForbidden, apierr.PasswordChangeRequired)
	wantErr(t, e.do(req{method: "PUT", path: "/api/v1/admin/settings", cookie: cookie, csrf: csrf}), http.StatusForbidden, apierr.PasswordChangeRequired)
	if r := e.do(req{method: "GET", path: "/api/v1/session", cookie: cookie}); r.Code != http.StatusOK {
		t.Fatalf("session: %d", r.Code)
	}

	// A second session of the same user, to be dropped by the change.
	other := e.do(req{method: "POST", path: "/auth/local", body: map[string]string{"username": "admin", "password": adminPW}})
	otherCookie := cookieNamed(other, SessionCookie).Value

	chg := func(cur, nw string) *http.Response {
		return e.do(req{method: "POST", path: "/auth/local/password", cookie: cookie, csrf: csrf,
			body: map[string]string{"current_password": cur, "new_password": nw}}).Result()
	}
	// Without CSRF.
	wantErr(t, e.do(req{method: "POST", path: "/auth/local/password", cookie: cookie,
		body: map[string]string{"current_password": adminPW, "new_password": "brand-new-password"}}), http.StatusForbidden, apierr.CSRF)
	for _, c := range []struct{ cur, nw, field string }{
		{"wrong-current-pw", "brand-new-password", "current_password"},
		{adminPW, "short", "new_password"},
		{adminPW, adminPW, "new_password"},
	} {
		rec := e.do(req{method: "POST", path: "/auth/local/password", cookie: cookie, csrf: csrf,
			body: map[string]string{"current_password": c.cur, "new_password": c.nw}})
		wantErr(t, rec, http.StatusBadRequest, apierr.Validation)
		if !strings.Contains(rec.Body.String(), `"field":"`+c.field+`"`) {
			t.Fatalf("field missing: %s", rec.Body.String())
		}
	}
	res := chg(adminPW, "brand-new-password")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("change: %d", res.StatusCode)
	}
	var newCookie string
	for _, c := range res.Cookies() {
		if c.Name == SessionCookie {
			newCookie = c.Value
		}
	}
	if newCookie == "" || newCookie == cookie {
		t.Fatal("session not rotated")
	}
	// Old and other sessions are gone; the new one works and the guard is lifted.
	wantErr(t, e.do(req{method: "GET", path: "/api/v1/session", cookie: cookie}), http.StatusUnauthorized, apierr.Unauthenticated)
	wantErr(t, e.do(req{method: "GET", path: "/api/v1/session", cookie: otherCookie}), http.StatusUnauthorized, apierr.Unauthenticated)
	if r := e.do(req{method: "GET", path: "/api/v1/admin/users", cookie: newCookie}); r.Code != http.StatusOK {
		t.Fatalf("after change: %d %s", r.Code, r.Body.String())
	}
	if r := e.do(req{method: "POST", path: "/auth/local", body: map[string]string{"username": "admin", "password": "brand-new-password"}}); r.Code != http.StatusOK || decodeView(t, r).MustChangePassword {
		t.Fatalf("sign in with new password: %d", r.Code)
	}
	if strings.Contains(e.logs.String(), "brand-new-password") || strings.Contains(e.logs.String(), adminPW) {
		t.Fatal("password logged")
	}
}

func TestAUTH7_PasswordChangeOnlyForLocalAdmin(t *testing.T) {
	e := newEnv(t, Config{})
	u := e.user("sub-1", store.StatusActive, store.RoleAdmin)
	c, csrf := e.login(u.ID)
	wantErr(t, e.do(req{method: "POST", path: "/auth/local/password", cookie: c, csrf: csrf,
		body: map[string]string{"current_password": "x", "new_password": "brand-new-password"}}), http.StatusForbidden, apierr.Forbidden)
	wantErr(t, e.do(req{method: "POST", path: "/auth/local/password",
		body: map[string]string{"current_password": "x", "new_password": "brand-new-password"}}), http.StatusUnauthorized, apierr.Unauthenticated)
}

func TestAUTH6_LocalSignInRequiresJSON(t *testing.T) {
	e := newEnv(t, Config{})
	e.localAdmin()
	rec := e.do(req{method: "POST", path: "/auth/local", body: `{"username":"admin","password":"` + adminPW + `"}`,
		header: map[string]string{"Content-Type": "text/plain"}})
	wantErr(t, rec, http.StatusBadRequest, apierr.Validation)
	if cookieNamed(rec, SessionCookie) != nil {
		t.Fatal("signed in from a non-JSON body")
	}
}
