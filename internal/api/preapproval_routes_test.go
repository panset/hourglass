// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/store"
)

// The pre-approval admin routes (docs/design/ledger-labels-onboarding.md, D3; PRE-5).

type preItem struct {
	ID        *int64          `json:"id"`
	Email     string          `json:"email"`
	Roles     []string        `json:"roles"`
	Source    string          `json:"source"`
	CreatedAt *string         `json:"created_at"`
	CreatedBy *preapprovalRef `json:"created_by"`
	UsedAt    *string         `json:"used_at"`
	UsedBy    *preapprovalRef `json:"used_by"`
}

type preapprovalRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func listPre(t *testing.T, e *preEnv) []preItem {
	t.Helper()
	rec := e.serve("GET", "/api/v1/admin/preapprovals", &e.adm, false, "")
	if rec.Code != 200 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	var out struct {
		Preapprovals []preItem `json:"preapprovals"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Preapprovals
}

func addBody(emails []string, roles string) string {
	b, _ := json.Marshal(emails)
	return `{"emails":` + string(b) + `,"roles":` + roles + `}`
}

func countPre(t *testing.T, st *store.Store) int {
	t.Helper()
	ps, err := st.ListPreapprovals(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return len(ps)
}

func wantValidation(t *testing.T, rec interface {
	Result() *http.Response
}, body, field, mentions string) {
	t.Helper()
	res := rec.Result()
	var e struct {
		Error struct {
			Code, Message, Field string
		} `json:"error"`
	}
	_ = json.Unmarshal([]byte(body), &e)
	if res.StatusCode != 400 || e.Error.Code != apierr.Validation || e.Error.Field != field || !strings.Contains(e.Error.Message, mentions) {
		t.Fatalf("got %d %s, want 400 validation on %q mentioning %q", res.StatusCode, body, field, mentions)
	}
}

// PRE-5: the list holds both sources, by email, with source, creator and use.
func TestPRE5_ListShowsBothSourcesByEmail(t *testing.T) {
	e := newPreEnv(t,
		PreapprovalEntry{Email: "zed@example.test", Roles: []string{store.RoleDeveloper}},
		PreapprovalEntry{Email: "both@example.test", Roles: []string{store.RoleAdmin}})
	if got := listPre(t, e); len(got) != 2 {
		t.Fatalf("file only: %+v", got)
	}
	e.add(`["developer"]`, "both@example.test", "amy@example.test")
	e.mustSignIn(claimsFor("sub-amy", "amy@example.test"))
	amy, _, _ := e.state("sub-amy")
	got := listPre(t, e)
	want := []struct{ email, source string }{
		{"amy@example.test", "database"}, {"both@example.test", "database"}, {"both@example.test", "config"}, {"zed@example.test", "config"}}
	if len(got) != len(want) {
		t.Fatalf("list %+v", got)
	}
	for i, w := range want {
		if got[i].Email != w.email || got[i].Source != w.source {
			t.Fatalf("entry %d = %s/%s, want %s/%s", i, got[i].Email, got[i].Source, w.email, w.source)
		}
	}
	db, file := got[0], got[3]
	if db.ID == nil || db.CreatedAt == nil || db.CreatedBy == nil || db.CreatedBy.ID != e.adm.user.ID || db.CreatedBy.Name != "User pre-admin" ||
		db.UsedAt == nil || db.UsedBy == nil || db.UsedBy.ID != amy.ID || strings.Join(db.Roles, ",") != "developer" {
		t.Fatalf("database entry %+v", db)
	}
	if got[1].UsedAt != nil || got[1].UsedBy != nil {
		t.Fatalf("unused entry shows a use: %+v", got[1])
	}
	if file.ID != nil || file.CreatedAt != nil || file.CreatedBy != nil || file.UsedAt != nil || file.UsedBy != nil ||
		strings.Join(file.Roles, ",") != "developer" {
		t.Fatalf("file entry %+v", file)
	}
	// A file entry stays unused even after it approved someone.
	e.mustSignIn(claimsFor("sub-zed", "zed@example.test"))
	if f := listPre(t, e)[3]; f.UsedAt != nil || f.UsedBy != nil {
		t.Fatalf("file entry marked used: %+v", f)
	}
}

// PRE-5: add answers the counts; an existing email gets the new roles and
// keeps its id, creator and use record; a repeated email counts once;
// emails are stored trimmed and in lower case; the add is audited.
func TestPRE5_AddCountsUpsertKeepsUseAndIsAudited(t *testing.T) {
	e := newPreEnv(t)
	rec := e.serve("POST", "/api/v1/admin/preapprovals", &e.adm, true,
		addBody([]string{" Lee@Example.test", "lee@example.test", "mo@example.test"}, `["developer"]`))
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"added":2,"updated":0}` {
		t.Fatalf("first add: %d %s", rec.Code, rec.Body)
	}
	e.mustSignIn(claimsFor("sub-lee", "lee@example.test"))
	before, _ := e.st.GetPreapprovalByEmail(context.Background(), "lee@example.test")
	e.clk.Advance(time.Hour)
	other := e.newUser("second-admin", store.StatusActive, store.RoleAdmin)
	rec = e.serve("POST", "/api/v1/admin/preapprovals", &other, true,
		addBody([]string{"LEE@example.test", "nia@example.test"}, `["admin","developer"]`))
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"added":1,"updated":1}` {
		t.Fatalf("second add: %d %s", rec.Code, rec.Body)
	}
	after, _ := e.st.GetPreapprovalByEmail(context.Background(), "lee@example.test")
	if after.ID != before.ID || *after.CreatedBy != *before.CreatedBy || !after.CreatedAt.Equal(before.CreatedAt) ||
		after.UsedAt == nil || !after.UsedAt.Equal(*before.UsedAt) || *after.UsedBy != *before.UsedBy ||
		strings.Join(after.Roles, ",") != "admin,developer" {
		t.Fatalf("upsert: before %+v after %+v", before, after)
	}
	if n := countPre(t, e.st); n != 3 {
		t.Fatalf("%d entries, want 3", n)
	}
	as, _ := e.st.ListAudit(context.Background(), store.AuditFilter{Action: auditPreapprovalAdd})
	if len(as) != 2 || as[0].ActorID == nil || *as[0].ActorID != other.user.ID ||
		as[0].AfterJSON != `{"emails":["lee@example.test","nia@example.test"],"roles":["admin","developer"]}` ||
		as[1].AfterJSON != `{"emails":["lee@example.test","mo@example.test"],"roles":["developer"]}` {
		t.Fatalf("audit %+v", as)
	}
}

// PRE-5: 1 to 1,000 emails; 1,001 and none are refused and store nothing.
func TestPRE5_AddAcceptsOneThousandRefusesOneThousandAndOne(t *testing.T) {
	e := newPreEnv(t)
	mk := func(n int, prefix string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("%s-%04d@example.test", prefix, i)
		}
		return out
	}
	rec := e.serve("POST", "/api/v1/admin/preapprovals", &e.adm, true, addBody(mk(1001, "over"), `["developer"]`))
	wantValidation(t, rec, rec.Body.String(), "emails", "1 to 1000")
	rec = e.serve("POST", "/api/v1/admin/preapprovals", &e.adm, true, addBody([]string{}, `["developer"]`))
	wantValidation(t, rec, rec.Body.String(), "emails", "1 to 1000")
	if n := countPre(t, e.st); n != 0 {
		t.Fatalf("%d entries stored by refused requests", n)
	}
	rec = e.serve("POST", "/api/v1/admin/preapprovals", &e.adm, true, addBody(mk(1000, "ok"), `["developer"]`))
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"added":1000,"updated":0}` {
		t.Fatalf("1000: %d %s", rec.Code, rec.Body)
	}
	if n := countPre(t, e.st); n != 1000 {
		t.Fatalf("%d entries, want 1000", n)
	}
	rec = e.serve("POST", "/api/v1/admin/preapprovals", &e.adm, true, addBody(mk(1, "one"), `["developer"]`))
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"added":1,"updated":0}` {
		t.Fatalf("1: %d %s", rec.Code, rec.Body)
	}
}

// PRE-5: every email rule; one bad email among good ones stores nothing and
// the message names the first bad value; bad or empty roles are refused.
func TestPRE5_AddValidatesEveryEmailAndRoleAllOrNothing(t *testing.T) {
	e := newPreEnv(t)
	long := strings.Repeat("a", 254-len("@example.test")) + "@example.test" // 254 characters
	if len(long) != 254 {
		t.Fatal(len(long))
	}
	rec := e.serve("POST", "/api/v1/admin/preapprovals", &e.adm, true, addBody([]string{"  " + long + "  "}, `["developer"]`))
	if rec.Code != 200 {
		t.Fatalf("254 characters refused: %d %s", rec.Code, rec.Body)
	}
	bad := map[string]string{
		"empty":            "",
		"blank":            "   ",
		"255 characters":   "a" + long,
		"no at":            "user.example.test",
		"two ats":          "a@b@example.test",
		"nothing before @": "@example.test",
		"nothing after @":  "user@",
		"inner space":      "us er@example.test",
		"inner tab":        "user@exa\tmple.test",
		"comma":            "a@example.test,b@example.test",
		"star":             "*@example.test",
		"wildcard domain":  "user@*.example.test",
		"zero-width space": "user@exa\u200Bmple.test",
		"byte order mark":  "\uFEFFuser@example.test",
		"control char":     "us\x01er@example.test",
		"bidi override":    "user@example.\u202Etest",
	}
	for name, email := range bad {
		t.Run(name, func(t *testing.T) {
			before := countPre(t, e.st)
			emails := []string{"good-1@example.test", email, "also-bad no@example.test", "good-2@example.test"}
			rec := e.serve("POST", "/api/v1/admin/preapprovals", &e.adm, true, addBody(emails, `["developer"]`))
			wantValidation(t, rec, rec.Body.String(), "emails", "invalid email")
			var er struct{ Error struct{ Message string } }
			_ = json.Unmarshal(rec.Body.Bytes(), &er)
			if strings.Contains(er.Error.Message, "also-bad") {
				t.Fatalf("message names a later value: %s", er.Error.Message)
			}
			if n := countPre(t, e.st); n != before {
				t.Fatalf("stored %d entries on a refused request", n-before)
			}
		})
	}
	for name, tc := range map[string]struct{ body, field, mentions string }{
		"unknown role":   {addBody([]string{"r@example.test"}, `["manager"]`), "roles", `"manager"`},
		"empty roles":    {addBody([]string{"r@example.test"}, `[]`), "roles", "must not be empty"},
		"duplicate role": {addBody([]string{"r@example.test"}, `["developer","developer"]`), "roles", "duplicate"},
		"no roles":       {`{"emails":["r@example.test"]}`, "roles", "required"},
		"no emails":      {`{"roles":["developer"]}`, "emails", "required"},
		"null emails":    {`{"emails":null,"roles":["developer"]}`, "emails", "required"},
		"unknown field":  {`{"emails":["r@example.test"],"roles":["developer"],"x":1}`, "x", "unknown field"},
	} {
		t.Run(name, func(t *testing.T) {
			rec := e.serve("POST", "/api/v1/admin/preapprovals", &e.adm, true, tc.body)
			wantValidation(t, rec, rec.Body.String(), tc.field, tc.mentions)
			if _, err := e.st.GetPreapprovalByEmail(context.Background(), "r@example.test"); err == nil {
				t.Fatal("stored on a refused request")
			}
		})
	}
}

// PRE-5: delete removes one database entry, is audited, answers 404 for an
// unknown id, and file entries cannot be removed.
func TestPRE5_DeleteRemovesOneDatabaseEntry(t *testing.T) {
	e := newPreEnv(t, PreapprovalEntry{Email: "file@example.test", Roles: []string{store.RoleDeveloper}})
	e.add(`["developer"]`, "olga@example.test", "pat@example.test")
	var id int64
	for _, p := range listPre(t, e) {
		if p.Email == "olga@example.test" {
			id = *p.ID
		}
	}
	rec := e.serve("DELETE", fmt.Sprintf("/api/v1/admin/preapprovals/%d", id), &e.adm, true, "")
	if rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	got := listPre(t, e)
	if len(got) != 2 || got[0].Email != "file@example.test" || got[1].Email != "pat@example.test" {
		t.Fatalf("after delete %+v", got)
	}
	rec = e.serve("DELETE", fmt.Sprintf("/api/v1/admin/preapprovals/%d", id), &e.adm, true, "")
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), `"not_found"`) {
		t.Fatalf("second delete: %d %s", rec.Code, rec.Body)
	}
	as, _ := e.st.ListAudit(context.Background(), store.AuditFilter{Action: auditPreapprovalDel})
	if len(as) != 1 || as[0].Target != fmt.Sprintf("preapproval:%d", id) || as[0].ActorID == nil || *as[0].ActorID != e.adm.user.ID ||
		as[0].BeforeJSON != `{"email":"olga@example.test","roles":["developer"]}` || as[0].AfterJSON != "" {
		t.Fatalf("audit %+v", as)
	}
}

// PRE-5: the three routes are refused for every principal but an admin: an
// anonymous caller, users with no role (pending, rejected, roles removed), a
// developer, the local admin before its password change, and a service
// account with any token. An admin and the local admin are allowed. Nothing
// is stored by a refused request.
func TestPRE5_RoutesRefusedForEveryPrincipalButAnAdmin(t *testing.T) {
	f := newPermFix(t)
	ops := []string{"listPreapprovals", "addPreapprovals", "deletePreapproval"}
	for _, a := range f.actors {
		for _, op := range ops {
			t.Run(a.name+"/"+op, func(t *testing.T) {
				p := f.probe(op, a, modeDefault) // may create the entry to delete
				before := countPre(t, f.st)
				rec := f.send(a, p)
				allowed := a.isAdmin() && !a.mustChange
				if allowed {
					if rec.Code >= 300 {
						t.Fatalf("admin refused: %d %s", rec.Code, rec.Body)
					}
					return
				}
				want := http.StatusForbidden
				if !a.signedIn() {
					want = http.StatusUnauthorized
				}
				if rec.Code != want {
					t.Fatalf("%s: %d %s, want %d", a.name, rec.Code, rec.Body, want)
				}
				if m := permContainsMarker(rec); m != "" {
					t.Fatalf("refusal holds %q", m)
				}
				if strings.Contains(rec.Body.String(), "example.test") {
					t.Fatalf("refusal holds an address: %s", rec.Body)
				}
				switch op {
				case "addPreapprovals":
					if n := countPre(t, f.st); n != before {
						t.Fatalf("refused add stored %d entries", n-before)
					}
				case "deletePreapproval":
					if n := countPre(t, f.st); n != before {
						t.Fatalf("refused delete removed %d entries", before-n)
					}
				}
			})
		}
	}
	// A developer and a service account, spelled out (the brief's list).
	dev, svc := f.actor(actDeveloper), f.actor(actSvcScoped)
	for _, a := range []permActor{dev, svc} {
		if rec := f.send(a, f.probe("listPreapprovals", a, modeDefault)); rec.Code != 401 && rec.Code != 403 {
			t.Errorf("%s listed pre-approvals: %d", a.name, rec.Code)
		}
	}
	// The CSRF token is required on the two changes.
	adm := f.actor(actAdmin)
	for _, op := range []string{"addPreapprovals", "deletePreapproval"} {
		p := f.probe(op, adm, modeDefault)
		rec := f.serve(p.method, p.path, adm.s, false, p.body)
		if rec.Code != 403 || !strings.Contains(rec.Body.String(), `"csrf"`) {
			t.Errorf("%s without CSRF: %d %s", op, rec.Code, rec.Body)
		}
	}
}
