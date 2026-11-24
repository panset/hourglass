// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"testing"
)

// signInStored signs in and returns the stored user; the sign-in must succeed.
func signInStored(t *testing.T, e *env) (name, email string, session *http.Cookie) {
	t.Helper()
	r := signIn(t, e)
	if r.location != "/" || r.session == nil {
		t.Fatalf("sign-in refused: %q", r.location)
	}
	u, err := e.st.GetUserBySub(e.ctx(), "sub-alice")
	if err != nil {
		t.Fatal(err)
	}
	return u.Name, u.Email, r.session
}

// TestAUTH9_UserInfoNameWhenIDTokenHasNone: a provider that keeps the profile
// claims out of the ID token and serves them from UserInfo still gives a
// stored name, by the same fallback order, and the greeting shows it.
func TestAUTH9_UserInfoNameWhenIDTokenHasNone(t *testing.T) {
	cases := []struct {
		label    string
		userinfo map[string]any
		want     string
	}{
		{"name", map[string]any{"sub": "sub-alice", "name": "Alice Userinfo", "given_name": "G", "family_name": "F"}, "Alice Userinfo"},
		{"given and family when no name", map[string]any{"sub": "sub-alice", "given_name": "Ada", "family_name": "Lovelace", "preferred_username": "ada.l"}, "Ada Lovelace"},
		{"whitespace-only name falls through", map[string]any{"sub": "sub-alice", "name": "  ", "preferred_username": "ada.l"}, "ada.l"},
		{"no name claims at all is empty", map[string]any{"sub": "sub-alice"}, ""},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			e, iss := newOIDCEnv(t)
			iss.set(func(f *fakeIssuer) { f.name = ""; f.userinfo = c.userinfo })
			name, _, session := signInStored(t, e)
			if name != c.want {
				t.Fatalf("stored name %q, want %q", name, c.want)
			}
			if n := iss.userinfoCalls(); n != 1 {
				t.Fatalf("userinfo requests %d, want 1", n)
			}
			if c.want == "" {
				return
			}
			rec := e.do(req{method: "GET", path: "/api/v1/session", cookie: session.Value})
			if v := decodeView(t, rec); v.DisplayName != c.want {
				t.Fatalf("display_name %q, want %q", v.DisplayName, c.want)
			}
		})
	}
}

// TestAUTH9_UserInfoNotCalledWhenIDTokenHasAName: any name from the ID token,
// by any step of the fallback, is used and no UserInfo request is made.
func TestAUTH9_UserInfoNotCalledWhenIDTokenHasAName(t *testing.T) {
	for label, setup := range map[string]func(f *fakeIssuer){
		"name":               func(f *fakeIssuer) {},
		"given and family":   func(f *fakeIssuer) { f.name, f.given, f.family = "", "Ada", "Lovelace" },
		"preferred_username": func(f *fakeIssuer) { f.name, f.preferred = "", "ada.l" },
	} {
		t.Run(label, func(t *testing.T) {
			e, iss := newOIDCEnv(t)
			iss.set(func(f *fakeIssuer) {
				f.userinfo = map[string]any{"sub": "sub-alice", "name": "From Userinfo"}
				setup(f)
			})
			name, _, _ := signInStored(t, e)
			if name == "" || name == "From Userinfo" {
				t.Fatalf("stored name %q, want the ID token's", name)
			}
			if n := iss.userinfoCalls(); n != 0 {
				t.Fatalf("userinfo requests %d, want 0", n)
			}
		})
	}
}

// TestAUTH9_NoUserInfoEndpointMakesNoRequest: a provider whose discovery
// document names no UserInfo endpoint is signed in as before.
func TestAUTH9_NoUserInfoEndpointMakesNoRequest(t *testing.T) {
	e, iss := newOIDCEnv(t)
	iss.set(func(f *fakeIssuer) { f.name = "" })
	if name, _, _ := signInStored(t, e); name != "" {
		t.Fatalf("stored name %q, want empty", name)
	}
	if n := iss.userinfoCalls(); n != 0 {
		t.Fatalf("userinfo requests %d, want 0", n)
	}
}

// TestAUTH9_UserInfoNeverFailsSignIn: an error, a response for another
// subject, a response with no subject and claims that are not strings all
// leave the name empty and the sign-in succeeds.
func TestAUTH9_UserInfoNeverFailsSignIn(t *testing.T) {
	cases := []struct {
		label    string
		userinfo map[string]any
		status   int
	}{
		{"server error", map[string]any{"sub": "sub-alice", "name": "Alice"}, http.StatusInternalServerError},
		{"unauthorized", map[string]any{"sub": "sub-alice", "name": "Alice"}, http.StatusUnauthorized},
		{"another subject", map[string]any{"sub": "sub-mallory", "name": "Mallory"}, 0},
		{"no subject", map[string]any{"name": "Nobody"}, 0},
		{"non-string claims", map[string]any{"sub": "sub-alice", "name": 42, "given_name": []any{"x"}, "family_name": map[string]any{"a": 1}, "preferred_username": true}, 0},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			e, iss := newOIDCEnv(t)
			iss.set(func(f *fakeIssuer) { f.name = ""; f.userinfo = c.userinfo; f.userinfoStatus = c.status })
			if name, _, _ := signInStored(t, e); name != "" {
				t.Fatalf("stored name %q, want empty", name)
			}
			if n := iss.userinfoCalls(); n != 1 {
				t.Fatalf("userinfo requests %d, want 1", n)
			}
		})
	}
}

// TestAUTH9_UserInfoGivesOnlyTheName: the email stays the ID token's, so a
// UserInfo response cannot change who a pre-approval matches.
func TestAUTH9_UserInfoGivesOnlyTheName(t *testing.T) {
	e, iss := newOIDCEnv(t)
	iss.set(func(f *fakeIssuer) {
		f.name = ""
		f.userinfo = map[string]any{"sub": "sub-alice", "name": "Alice Userinfo", "email": "other@example.test", "email_verified": true}
	})
	name, email, _ := signInStored(t, e)
	if name != "Alice Userinfo" || email != "alice@example.test" {
		t.Fatalf("stored name %q email %q", name, email)
	}
}

// TestAUTH9_UserInfoNameRefreshedAtNextSignIn: a user stored with no name
// before the provider's UserInfo was read gets the name at the next sign-in.
func TestAUTH9_UserInfoNameRefreshedAtNextSignIn(t *testing.T) {
	e, iss := newOIDCEnv(t)
	iss.set(func(f *fakeIssuer) { f.name = ""; f.userinfo = map[string]any{"sub": "sub-alice"} })
	if name, _, _ := signInStored(t, e); name != "" {
		t.Fatalf("stored name %q, want empty", name)
	}
	iss.set(func(f *fakeIssuer) { f.userinfo = map[string]any{"sub": "sub-alice", "name": "Alice Userinfo"} })
	if name, _, _ := signInStored(t, e); name != "Alice Userinfo" {
		t.Fatalf("stored name %q after the second sign-in", name)
	}
}
