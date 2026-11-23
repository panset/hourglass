// SPDX-License-Identifier: MIT

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/api"
	"github.com/panset/hourglass/internal/app/runhook"
	"github.com/panset/hourglass/internal/store"
)

// The pre-approval file overlay (docs/design/ledger-labels-onboarding.md, D4; PRE-6).

func writeFile(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o400); err != nil { // read-only: the server must never write it
		t.Fatal(err)
	}
	return p
}

func TestPRE6_FileLoaderReadsYAMLAndJSON(t *testing.T) {
	y := writeFile(t, "users.yaml", `# pre-approved users
users:
  - email: alice@example.test
    roles: [developer]
  - email: "  Bob@Example.TEST "        # roles default to [developer]
  - email: carol@example.test
    roles: [admin, developer]
`)
	got, err := LoadPreapprovedUsers(y)
	if err != nil {
		t.Fatal(err)
	}
	want := "alice@example.test:developer bob@example.test:developer carol@example.test:admin,developer"
	if s := entriesString(got); s != want {
		t.Fatalf("yaml entries %q, want %q", s, want)
	}
	j := writeFile(t, "users.JSON", `{"users":[{"email":"dan@example.test","roles":["admin"]},{"email":"eve@example.test"}]}`)
	got, err = LoadPreapprovedUsers(j)
	if err != nil {
		t.Fatal(err)
	}
	if s := entriesString(got); s != "dan@example.test:admin eve@example.test:developer" {
		t.Fatalf("json entries %q", s)
	}
	// Empty documents are a file with no entries.
	for _, body := range []string{"", "users: []\n", "# nothing yet\n"} {
		if got, err := LoadPreapprovedUsers(writeFile(t, "empty.yaml", body)); err != nil || len(got) != 0 {
			t.Fatalf("%q: %v %v", body, got, err)
		}
	}
	// No path, no file.
	if got, err := LoadPreapprovedUsers(""); err != nil || got != nil {
		t.Fatalf("no path: %v %v", got, err)
	}
}

func entriesString(es []api.PreapprovalEntry) string {
	parts := []string{}
	for _, x := range es {
		parts = append(parts, x.Email+":"+strings.Join(x.Roles, ","))
	}
	return strings.Join(parts, " ")
}

// PRE-6: an invalid file of each kind is an error naming the entry, and
// never repeats a valid address.
func TestPRE6_InvalidFileIsAnErrorNamingTheEntry(t *testing.T) {
	many := &strings.Builder{}
	many.WriteString("users:\n")
	for i := 0; i <= MaxPreapprovedFileEntries; i++ {
		fmt.Fprintf(many, "  - email: u%05d@example.test\n", i)
	}
	cases := []struct {
		name, file, body string
		mentions         []string
	}{
		{"missing file", "", "", []string{"no such file"}},
		{"not YAML", "u.yaml", "users: [\n", []string{"u.yaml"}},
		{"not JSON", "u.json", `{"users": [`, []string{"u.json"}},
		{"YAML read as JSON by extension", "u.json", "users:\n  - email: a@example.test\n", []string{"u.json"}},
		{"unknown top key", "u.yaml", "users: []\ngroups: []\n", []string{`unknown field "groups"`}},
		{"unknown entry key", "u.yaml", "users:\n  - email: a@example.test\n    role: admin\n", []string{`unknown field "role"`}},
		{"not a list", "u.yaml", "users: a@example.test\n", []string{"u.yaml"}},
		{"missing email", "u.yaml", "users:\n  - email: a@example.test\n  - roles: [admin]\n", []string{"users[1]", "email is required"}},
		{"invalid email", "u.yaml", "users:\n  - email: a@example.test\n  - email: '*@example.test'\n", []string{"users[1]", "invalid email"}},
		{"email with space", "u.yaml", "users:\n  - email: 'x y@example.test'\n", []string{"users[0]", "whitespace"}},
		{"email with comma", "u.yaml", "users:\n  - email: 'c@example.test,d@example.test'\n", []string{"users[0]", "invalid email"}},
		{"two ats", "u.yaml", "users:\n  - email: 'c@d@example.test'\n", []string{"users[0]", `exactly one "@"`}},
		{"too long", "u.yaml", "users:\n  - email: " + strings.Repeat("a", 250) + "@example.test\n", []string{"users[0]", "254"}},
		{"zero-width space", "u.yaml", "users:\n  - email: \"x\\u200By@example.test\"\n", []string{"users[0]", "invisible"}},
		{"byte order mark inside", "u.yaml", "users:\n  - email: \"x@exa\\uFEFFmple.test\"\n", []string{"users[0]", "invisible"}},
		{"control character", "u.yaml", "users:\n  - email: \"x\\x01y@example.test\"\n", []string{"users[0]", "invisible"}},
		{"second YAML document", "u.yaml", "users:\n  - email: p@example.test\n---\nusers:\n  - email: q@example.test\n", []string{"u.yaml", "more than one"}},
		{"YAML duplicate key", "u.yaml", "users:\n  - email: p@example.test\n    email: q@example.test\n", []string{"u.yaml"}},
		{"YAML duplicate top key", "u.yaml", "users: []\nusers: []\n", []string{"u.yaml"}},
		{"JSON duplicate entry key", "u.json", `{"users":[{"email":"p@example.test","email":"q@example.test"}]}`, []string{"u.json", "duplicate key"}},
		{"JSON duplicate top key", "u.json", `{"users":[],"users":[]}`, []string{"u.json", "duplicate key"}},
		{"JSON key in another case", "u.json", `{"users":[{"EMAIL":"p@example.test"}]}`, []string{"u.json", `unknown field "EMAIL"`}},
		{"JSON top key in another case", "u.json", `{"Users":[{"email":"p@example.test"}]}`, []string{"u.json", `unknown field "Users"`}},
		{"YAML key in another case", "u.yaml", "users:\n  - Email: p@example.test\n", []string{"u.yaml", `unknown field "Email"`}},
		{"unknown role", "u.yaml", "users:\n  - email: a@example.test\n    roles: [manager]\n", []string{"users[0]", `"manager"`}},
		{"empty roles", "u.yaml", "users:\n  - email: a@example.test\n    roles: []\n", []string{"users[0]", "must not be empty"}},
		{"duplicate role", "u.yaml", "users:\n  - email: a@example.test\n    roles: [admin, admin]\n", []string{"users[0]", "duplicate"}},
		{"repeated email", "u.yaml", "users:\n  - email: a@example.test\n  - email: b@example.test\n  - email: ' A@EXAMPLE.test'\n",
			[]string{"users[2]", "repeats the email of users[0]"}},
		{"over the limit", "u.yaml", many.String(), []string{"10001 entries", "at most 10000"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "missing.yaml")
			if c.file != "" {
				path = writeFile(t, c.file, c.body)
			}
			_, err := LoadPreapprovedUsers(path)
			var pe *PreapprovedFileError
			if !errors.As(err, &pe) {
				t.Fatalf("err = %v, want a PreapprovedFileError", err)
			}
			msg := err.Error()
			if !strings.HasPrefix(msg, "preapproved_users_file: ") {
				t.Errorf("message %q does not name the setting", msg)
			}
			for _, m := range c.mentions {
				if !strings.Contains(msg, m) {
					t.Errorf("message %q does not mention %q", msg, m)
				}
			}
			if strings.Contains(msg, "a@example.test") || strings.Contains(msg, "b@example.test") {
				t.Errorf("message %q repeats a valid address", msg)
			}
		})
	}
	// 10,000 entries are accepted.
	ok := &strings.Builder{}
	ok.WriteString("users:\n")
	for i := 0; i < MaxPreapprovedFileEntries; i++ {
		fmt.Fprintf(ok, "  - email: u%05d@example.test\n", i)
	}
	if got, err := LoadPreapprovedUsers(writeFile(t, "max.yaml", ok.String())); err != nil || len(got) != MaxPreapprovedFileEntries {
		t.Fatalf("10000 entries: %d %v", len(got), err)
	}
}

// PRE-6: an invalid file stops the start before the database is created, and
// the error names the entry.
func TestPRE6_InvalidFileStopsStartup(t *testing.T) {
	registerFakes()
	bad := writeFile(t, "users.yaml", "users:\n  - email: ok@example.test\n  - email: not-an-email\n")
	cfg, db := writeConfig(t, "preapproved_users_file: "+bad+"\n")
	err := Run(context.Background(), runhook.Options{ConfigPath: cfg, DefaultPressureSource: testPressure,
		DefaultMeteringPolicy: testPolicy, Stdout: io.Discard, Stderr: io.Discard})
	var pe *PreapprovedFileError
	if !errors.As(err, &pe) || !strings.Contains(err.Error(), "users[1]") {
		t.Fatalf("err = %v, want a PreapprovedFileError naming users[1]", err)
	}
	if _, statErr := os.Stat(db); !errors.Is(statErr, os.ErrNotExist) {
		t.Error("the database was created despite an invalid pre-approval file")
	}
	// The environment variable names the file too.
	t.Setenv("HOURGLASS_PREAPPROVED_USERS_FILE", filepath.Join(t.TempDir(), "absent.yaml"))
	cfg2, _ := writeConfig(t, "")
	err = Run(context.Background(), runhook.Options{ConfigPath: cfg2, DefaultPressureSource: testPressure,
		DefaultMeteringPolicy: testPolicy, Stdout: io.Discard, Stderr: io.Discard})
	if !errors.As(err, &pe) || !strings.Contains(err.Error(), "absent.yaml") {
		t.Fatalf("env: err = %v", err)
	}
}

// PRE-6: a valid file is an overlay: its entries appear in the list with
// source config; the count is logged, never the addresses; the redacted
// config shows the path; the file is never written and never copied to the
// database.
func TestPRE6_ValidFileIsAReadOnlyOverlay(t *testing.T) {
	body := "users:\n  - email: Alice@Example.test\n    roles: [admin]\n  - email: bob@example.test\n"
	file := writeFile(t, "users.yaml", body)
	before, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	cfg, db := writeConfig(t, "preapproved_users_file: "+file+"\n")
	r := start(t, cfg, db)
	line := r.out.logLine(t, "pre-approved users file read")
	if line["entries"] != float64(2) || line["file"] != file {
		t.Fatalf("log line %v", line)
	}
	starting := r.out.logLine(t, "starting")
	if c, _ := starting["config"].(map[string]any); c == nil || c["preapproved_users_file"] != file {
		t.Fatalf("redacted config %v lacks the path", starting["config"])
	}
	if out := strings.ToLower(r.out.String()); strings.Contains(out, "alice@example.test") || strings.Contains(out, "bob@example.test") {
		t.Fatalf("an address was logged: %s", r.out.String())
	}

	// The list through the API, as the local admin after the password change.
	cookie := localAdminCookie(t, r.base)
	req, _ := http.NewRequest("GET", r.base+"/api/v1/admin/preapprovals", nil)
	req.Header.Set("Cookie", "hourglass_session="+cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var list struct {
		Preapprovals []struct {
			ID     *int64   `json:"id"`
			Email  string   `json:"email"`
			Roles  []string `json:"roles"`
			Source string   `json:"source"`
		} `json:"preapprovals"`
	}
	if err := json.Unmarshal(b, &list); err != nil || resp.StatusCode != 200 {
		t.Fatalf("list: %d %s", resp.StatusCode, b)
	}
	if len(list.Preapprovals) != 2 || list.Preapprovals[0].Email != "alice@example.test" || list.Preapprovals[0].Source != "config" ||
		list.Preapprovals[0].ID != nil || strings.Join(list.Preapprovals[0].Roles, ",") != "admin" ||
		list.Preapprovals[1].Email != "bob@example.test" || strings.Join(list.Preapprovals[1].Roles, ",") != "developer" {
		t.Fatalf("list %s", b)
	}
	if err := r.stop(); err != nil {
		t.Fatal(err)
	}

	after, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	now, _ := os.ReadFile(file)
	if !bytes.Equal(now, []byte(body)) || !after.ModTime().Equal(before.ModTime()) || after.Mode() != before.Mode() {
		t.Fatal("the pre-approval file was modified")
	}
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if ps, err := st.ListPreapprovals(context.Background()); err != nil || len(ps) != 0 {
		t.Fatalf("file entries copied to the database: %v %v", ps, err)
	}
}

// PRE-6: the key is optional; an existing config without it starts and logs
// nothing about a file.
func TestPRE6_NoFileStartsAsBefore(t *testing.T) {
	cfg, db := writeConfig(t, "")
	r := start(t, cfg, db)
	time.Sleep(20 * time.Millisecond)
	if strings.Contains(r.out.String(), "pre-approved users file") {
		t.Fatal("a file was reported without the key")
	}
	if err := r.stop(); err != nil {
		t.Fatal(err)
	}
}

// localAdminCookie signs in as the local admin, changes the first-run
// password, and returns the session cookie value of the new session.
func localAdminCookie(t *testing.T, base string) string {
	t.Helper()
	call := func(method, path, cookie, csrf, body string) (*http.Response, []byte) {
		req, _ := http.NewRequest(method, base+path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if cookie != "" {
			req.Header.Set("Cookie", "hourglass_session="+cookie)
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp, b
	}
	cookieOf := func(resp *http.Response) string {
		for _, c := range resp.Cookies() {
			if c.Name == "hourglass_session" {
				return c.Value
			}
		}
		t.Fatalf("no session cookie: %v", resp.Header)
		return ""
	}
	resp, b := call("POST", "/auth/local", "", "", `{"username":"admin","password":"`+adminPW+`"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("local sign-in: %d %s", resp.StatusCode, b)
	}
	c := cookieOf(resp)
	_, b = call("GET", "/api/v1/session", c, "", "")
	var s struct {
		CSRF string `json:"csrf_token"`
	}
	_ = json.Unmarshal(b, &s)
	resp, b = call("POST", "/auth/local/password", c, s.CSRF, `{"current_password":"`+adminPW+`","new_password":"a changed long admin password"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("password change: %d %s", resp.StatusCode, b)
	}
	return cookieOf(resp)
}
