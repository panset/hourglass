// SPDX-License-Identifier: MIT

package auth

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/panset/hourglass/internal/store"
)

// TestAUTH9_StoredNameFallbackOrder walks every step of the claim fallback
// through a real sign-in: the stored name is the first non-empty of name,
// given_name + family_name (trimmed), preferred_username.
func TestAUTH9_StoredNameFallbackOrder(t *testing.T) {
	cases := []struct {
		label                          string
		name, given, family, preferred string
		want                           string
	}{
		{"name wins over everything", "Alice Name", "Given", "Family", "pref", "Alice Name"},
		{"name is stored as sent", "  Alice  ", "Given", "Family", "pref", "  Alice  "},
		{"given and family when no name", "", "Ada", "Lovelace", "pref", "Ada Lovelace"},
		{"whitespace-only name falls through", "   ", "Ada", "Lovelace", "pref", "Ada Lovelace"},
		{"only a given name", "", "Ada", "", "pref", "Ada"},
		{"only a family name", "", "", "Lovelace", "pref", "Lovelace"},
		{"whitespace-only given, real family", "", "  ", "Lovelace", "", "Lovelace"},
		{"given and family both whitespace falls to preferred", "", " ", "\t", "ada.l", "ada.l"},
		{"preferred_username last", "", "", "", "ada.l", "ada.l"},
		{"whitespace-only everything is empty", " ", " ", " ", " ", ""},
		{"nothing sent is empty", "", "", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			e, iss := newOIDCEnv(t)
			iss.set(func(f *fakeIssuer) {
				f.name, f.given, f.family, f.preferred = c.name, c.given, c.family, c.preferred
			})
			signIn(t, e)
			u, err := e.st.GetUserBySub(e.ctx(), "sub-alice")
			if err != nil {
				t.Fatal(err)
			}
			if u.Name != c.want {
				t.Fatalf("stored name %q, want %q", u.Name, c.want)
			}
		})
	}
}

// TestAUTH9_NextSignInFillsEmptyStoredName: a user stored with no name gets
// it from the fallback claims at the next sign-in.
func TestAUTH9_NextSignInFillsEmptyStoredName(t *testing.T) {
	e, iss := newOIDCEnv(t)
	iss.set(func(f *fakeIssuer) { f.name = "" })
	signIn(t, e)
	if u, _ := e.st.GetUserBySub(e.ctx(), "sub-alice"); u.Name != "" {
		t.Fatalf("name %q, want empty", u.Name)
	}
	iss.set(func(f *fakeIssuer) { f.given, f.family = "Ada", "Lovelace" })
	signIn(t, e)
	if u, _ := e.st.GetUserBySub(e.ctx(), "sub-alice"); u.Name != "Ada Lovelace" {
		t.Fatalf("name %q after second sign-in", u.Name)
	}
}

// TestAUTH9_SessionDisplayName: display_name is name, else username, else
// email, and a user stored with an empty name has their email at once.
func TestAUTH9_SessionDisplayName(t *testing.T) {
	e := newEnv(t, Config{})
	get := func(u store.User) SessionView {
		c, _ := e.login(u.ID)
		rec := e.do(req{method: "GET", path: "/api/v1/session", cookie: c})
		if rec.Code != http.StatusOK {
			t.Fatalf("%d", rec.Code)
		}
		return decodeView(t, rec)
	}
	// An OIDC user created with an email and no name: no new sign-in needed.
	u, err := e.st.CreateOIDCUser(e.ctx(), "sub-noname", "noname@example.test", "", e.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if v := get(u); v.Name != "" || v.DisplayName != "noname@example.test" {
		t.Fatalf("empty stored name: name %q display_name %q", v.Name, v.DisplayName)
	}
	named, err := e.st.CreateOIDCUser(e.ctx(), "sub-named", "named@example.test", "Nora Named", e.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if v := get(named); v.Name != "Nora Named" || v.DisplayName != "Nora Named" {
		t.Fatalf("named: name %q display_name %q", v.Name, v.DisplayName)
	}
}

// TestAUTH9_DisplayNameRule pins the one shared function's three steps.
func TestAUTH9_DisplayNameRule(t *testing.T) {
	for _, c := range []struct {
		u    store.User
		want string
	}{
		{store.User{Name: "N", Username: "u", Email: "e@x"}, "N"},
		{store.User{Username: "u", Email: "e@x"}, "u"},
		{store.User{Email: "e@x"}, "e@x"},
		{store.User{}, ""},
	} {
		if got := DisplayName(c.u); got != c.want {
			t.Errorf("DisplayName(%+v) = %q, want %q", c.u, got, c.want)
		}
	}
}

// TestAUTH9_NonStringFallbackClaimsNeverFailSignIn: given_name, family_name
// and preferred_username that are not strings count as empty and never refuse
// a sign-in; commit d96a950 ignored them.
func TestAUTH9_NonStringFallbackClaimsNeverFailSignIn(t *testing.T) {
	cases := []struct {
		label  string
		claims map[string]any
		want   string
	}{
		{"numbers", map[string]any{"given_name": 7, "family_name": 8, "preferred_username": 9}, ""},
		{"objects and arrays", map[string]any{"given_name": map[string]any{"a": 1}, "family_name": []any{"x"}, "preferred_username": true}, ""},
		{"nulls", map[string]any{"given_name": json.RawMessage("null"), "family_name": json.RawMessage("null"), "preferred_username": json.RawMessage("null")}, ""},
		{"string family still used next to a bad given", map[string]any{"given_name": 5, "family_name": "Lovelace"}, "Lovelace"},
		{"bad given and family fall to a string preferred", map[string]any{"given_name": 5, "family_name": []any{}, "preferred_username": "ada.l"}, "ada.l"},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			e, iss := newOIDCEnv(t)
			iss.set(func(f *fakeIssuer) { f.name = ""; f.claims = c.claims })
			r := signIn(t, e)
			if r.location != "/" || r.session == nil {
				t.Fatalf("sign-in refused: %q", r.location)
			}
			u, err := e.st.GetUserBySub(e.ctx(), "sub-alice")
			if err != nil {
				t.Fatal(err)
			}
			if u.Name != c.want {
				t.Fatalf("stored name %q, want %q", u.Name, c.want)
			}
		})
	}
}

// TestAUTH9_NameClaimBehavesAsBefore: a non-string name still fails the
// sign-in, as at d96a950, and null or absent is an empty name.
func TestAUTH9_NameClaimBehavesAsBefore(t *testing.T) {
	for label, v := range map[string]any{"number": 42, "object": map[string]any{"a": 1}, "array": []any{"x"}, "bool": true} {
		t.Run(label, func(t *testing.T) {
			e, iss := newOIDCEnv(t)
			iss.set(func(f *fakeIssuer) { f.claims = map[string]any{"name": v} })
			wantFailure(t, signIn(t, e))
		})
	}
	e, iss := newOIDCEnv(t)
	iss.set(func(f *fakeIssuer) { f.claims = map[string]any{"name": json.RawMessage("null")} })
	if r := signIn(t, e); r.location != "/" || r.session == nil {
		t.Fatalf("null name refused: %q", r.location)
	}
}
