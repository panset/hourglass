// SPDX-License-Identifier: MIT

package api

import (
	"strings"
	"testing"

	"github.com/panset/hourglass/internal/store"
)

// signIn creates or refreshes an SSO user the way a sign-in does, then gives
// them the developer role and an allowance.
func (e *adEnv) signIn(sub, email, name string) store.User {
	e.t.Helper()
	u, _, err := e.st.UpsertOIDCLogin(uBg, sub, email, name, e.clk.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.st.SetUserStatus(uBg, u.ID, store.StatusActive); err != nil {
		e.t.Fatal(err)
	}
	if err := e.st.SetUserRoles(uBg, u.ID, []string{store.RoleDeveloper}); err != nil {
		e.t.Fatal(err)
	}
	if err := e.st.Tx(uBg, func(tx *store.Tx) error { return e.led.EnsureFirstAllowanceTx(tx, u.ID, e.clk.Now()) }); err != nil {
		e.t.Fatal(err)
	}
	u, _ = e.st.GetUser(uBg, u.ID)
	return u
}

// TestAUTH9_AdminUsersDisplayNameAndLocalFirst: display_name is name, else
// username, else email; the local account is first whatever the names are;
// the rest sort by display name ignoring case, ties by id (D100).
func TestAUTH9_AdminUsersDisplayNameAndLocalFirst(t *testing.T) {
	e := newAdEnv(t)
	e.signIn("s-zoe", "zoe@example.test", "Zoe Z")
	e.signIn("s-abe", "abe@example.test", "abe lower")
	e.signIn("s-bea", "bea@example.test", "Bea")
	unnamed := e.signIn("s-un", "carl@example.test", "") // sorts as its email
	e.signIn("s-dup1", "d1@example.test", "Same Name")
	e.signIn("s-dup2", "d2@example.test", "same name")
	over := e.signIn("s-over", "over@example.test", "Zzz Overdrawn")
	if _, err := e.led.Grant(uBg, e.admin.ID, over.ID, -400_000, "correction"); err != nil {
		t.Fatal(err)
	}

	body := uWantOK(t, e.get("/api/v1/admin/users"), 200)
	var got []string
	for _, u := range adArr(t, body, "users") {
		m := adObjOf(t, u)
		dn, _ := m["display_name"].(string)
		if dn == "" {
			t.Errorf("blank display_name: %v", m)
		}
		got = append(got, dn)
		switch m["email"] {
		case unnamed.Email:
			if dn != "carl@example.test" || m["name"] != "" {
				t.Errorf("unnamed user: display_name %q name %q", dn, m["name"])
			}
		case "bea@example.test":
			if dn != "Bea" || m["name"] != "Bea" {
				t.Errorf("named user: display_name %q name %q", dn, m["name"])
			}
		}
		if m["is_local_admin"] == true && dn != "admin" {
			t.Errorf("local admin display_name %q", dn)
		}
	}
	want := "admin,Zzz Overdrawn,abe lower,Bea,carl@example.test,Same Name,same name,Zoe Z"
	if strings.Join(got, ",") != want {
		t.Errorf("order = %v, want %s", got, want)
	}
}

// TestAUTH9_LocalAdminFirstWithNameBeforeAdmin: a user whose name sorts before
// "admin" never displaces the local account.
func TestAUTH9_LocalAdminFirstWithNameBeforeAdmin(t *testing.T) {
	e := newAdEnv(t)
	e.signIn("s-a", "a@example.test", "Aaron")
	e.signIn("s-0", "0@example.test", "")
	users := adArr(t, uWantOK(t, e.get("/api/v1/admin/users"), 200), "users")
	if first := adObjOf(t, users[0]); first["is_local_admin"] != true {
		t.Errorf("first row is %v, want the local account", first)
	}
}

// TestAUTH9_SetupsHolderShowsStoredNameOrFallback: the "Held by" name is the
// holder's stored name when there is one, else the email (nothing blank).
func TestAUTH9_SetupsHolderShowsStoredNameOrFallback(t *testing.T) {
	e := newAdEnv(t)
	named := e.signIn("s-h1", "named@example.test", "Hana Holder")
	unnamed := e.signIn("s-h2", "unnamed@example.test", "")
	viewer := e.dev("viewer")
	s1 := e.setup("lab-1", true, nil)
	s2 := e.setup("lab-2", true, nil)
	e.hold(s1, named.ID)
	e.hold(s2, unnamed.ID)

	holders := func(s *adSess) map[string]string {
		out := map[string]string{}
		rec := e.do("GET", "/api/v1/setups", s, false, "")
		for _, x := range adArr(t, uWantOK(t, rec, 200), "setups") {
			m := adObjOf(t, x)
			if h, ok := m["holder"].(map[string]any); ok {
				out[m["name"].(string)], _ = h["name"].(string)
			}
		}
		return out
	}
	for label, s := range map[string]*adSess{"developer": &viewer, "admin": &e.adminS} {
		got := holders(s)
		if got["lab-1"] != "Hana Holder" || got["lab-2"] != "unnamed@example.test" {
			t.Errorf("%s: holder names %v", label, got)
		}
	}

	// A later sign-in that yields no name keeps the stored one (D100).
	e.signIn("s-h1", "named@example.test", "")
	if got := holders(&viewer); got["lab-1"] != "Hana Holder" {
		t.Errorf("name lost after a sign-in without a name: %v", got)
	}
}
