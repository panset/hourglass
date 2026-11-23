// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/store"
)

// TestAUTH9_SessionAndAPIShareOneDisplayNameFunction fails if the session's
// display_name and the name the rest of the API shows for a user drift: for
// users whose stored name, username and email vary, the session's field must
// equal userDisplayName and auth.DisplayName of the stored row.
func TestAUTH9_SessionAndAPIShareOneDisplayNameFunction(t *testing.T) {
	b := newBackend(t, backendOpts{})
	ctx := context.Background()
	users := map[string]store.User{}

	named, err := b.st.CreateOIDCUser(ctx, "sub-named", "named@example.test", "Nora Named", b.clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	users["name set"] = named
	// A stored name of "" is what a provider without a name claim leaves.
	if _, _, err := b.st.UpsertOIDCLogin(ctx, "sub-blank", "blank@example.test", "", b.clk.Now()); err != nil {
		t.Fatal(err)
	}
	blank, err := b.st.GetUserBySub(ctx, "sub-blank")
	if err != nil {
		t.Fatal(err)
	}
	users["name empty"] = blank
	users["local admin"] = b.admin

	want := map[string]string{"name set": "Nora Named", "name empty": "blank@example.test", "local admin": b.admin.Username}
	for label, u := range users {
		s := b.session(u)
		rec := b.serve("GET", "/api/v1/session", &s, false, "")
		if rec.Code != 200 {
			t.Fatalf("%s: %d", label, rec.Code)
		}
		var got struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.DisplayName != want[label] {
			t.Errorf("%s: session display_name %q, want %q", label, got.DisplayName, want[label])
		}
		if got.DisplayName != userDisplayName(u) || got.DisplayName != auth.DisplayName(u) {
			t.Errorf("%s: session %q, userDisplayName %q, auth.DisplayName %q", label, got.DisplayName, userDisplayName(u), auth.DisplayName(u))
		}
		if got.Name != u.Name {
			t.Errorf("%s: name %q changed meaning, stored %q", label, got.Name, u.Name)
		}
	}
	// The API's helper and the session's function agree for every shape.
	for _, u := range []store.User{{Name: "N", Username: "u", Email: "e"}, {Username: "u", Email: "e"}, {Email: "e"}, {}} {
		if userDisplayName(u) != auth.DisplayName(u) {
			t.Errorf("userDisplayName(%+v) = %q, auth.DisplayName = %q", u, userDisplayName(u), auth.DisplayName(u))
		}
	}
}

// TestAUTH9_NoRoleSessionHoldsDisplayNameAndNothingElseNew: the session of a
// user with no role is the AUTH-5 field list plus display_name, and display_name
// is derived from fields already in the response.
func TestAUTH9_NoRoleSessionHoldsDisplayNameAndNothingElseNew(t *testing.T) {
	b := newBackend(t, backendOpts{})
	u, err := b.st.CreateOIDCUser(context.Background(), "sub-norole", "norole@example.test", "", b.clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := b.session(u)
	rec := b.serve("GET", "/api/v1/session", &s, false, "")
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"user_id": true, "username": true, "email": true, "name": true, "display_name": true,
		"auth_source": true, "status": true, "roles": true, "csrf_token": true, "must_change_password": true, "is_local_admin": true}
	for k := range m {
		if !want[k] {
			t.Errorf("unexpected session field %q", k)
		}
	}
	for k := range want {
		if _, ok := m[k]; !ok {
			t.Errorf("missing session field %q", k)
		}
	}
	if m["display_name"] != "norole@example.test" {
		t.Errorf("display_name %v, want the email (stored name is empty)", m["display_name"])
	}
}
