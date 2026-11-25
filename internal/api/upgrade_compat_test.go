// SPDX-License-Identifier: MIT

package api

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/panset/hourglass/internal/store"
)

// Compatibility with the release at d96a950 (UPG-2, UPG-3). The fixtures in
// testdata/upgrade-d96a950 were written by that release's code; see the
// README there for how.

const upgradeDir = "testdata/upgrade-d96a950"

func readUpgradeGolden(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(findUp(t, upgradeDir+"/responses.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g map[string]any
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

// upgradeWorld is the new code on a copy of the release's database: the
// same clock, zone and sessions as the recorder used, and an issuer for SSO.
func upgradeWorld(t *testing.T) (*preEnv, map[string]any) {
	t.Helper()
	g := readUpgradeGolden(t)
	now, err := time.Parse(time.RFC3339Nano, g["now"].(string))
	if err != nil {
		t.Fatal(err)
	}
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	iss := newPreIssuer(t)
	ev := &recEmitter{}
	b := newBackend(t, backendOpts{start: now, loc: berlin, oidcIssuer: iss.srv.URL, emitter: ev,
		dbCopy: findUp(t, upgradeDir+"/hourglass.sqlite3")})
	return &preEnv{backend: b, iss: iss, ev: ev}, g
}

func (e *preEnv) sessionFor(id int64) *sess {
	e.t.Helper()
	u, err := e.st.GetUser(uBg, id)
	if err != nil {
		e.t.Fatal(err)
	}
	s := e.session(u)
	return &s
}

func (e *preEnv) getJSON(s *sess, path string) map[string]any {
	e.t.Helper()
	rec := e.serve("GET", path, s, false, "")
	if rec.Code != 200 {
		e.t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body)
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		e.t.Fatal(err)
	}
	return m
}

// without removes the added fields from a copy of m, requires each to be
// present, and returns the copy. CSRF tokens are random per session.
func without(t *testing.T, what string, m map[string]any, added ...string) map[string]any {
	t.Helper()
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	for _, k := range added {
		if _, ok := out[k]; !ok {
			t.Errorf("%s: the added field %q is missing", what, k)
		}
		delete(out, k)
	}
	if _, ok := out["csrf_token"]; ok {
		out["csrf_token"] = "<csrf>"
	}
	return out
}

func sameAsRelease(t *testing.T, what string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		g, _ := json.MarshalIndent(got, "", " ")
		w, _ := json.MarshalIndent(want, "", " ")
		t.Errorf("%s differs from the release at d96a950 by more than the added fields\n got: %s\nwant: %s", what, g, w)
	}
}

// ledgerLikeRelease strips entry_count (which must be 1: nothing is
// compacted) and next_cursor (which must be null).
func ledgerLikeRelease(t *testing.T, what string, m map[string]any) map[string]any {
	t.Helper()
	if nc, ok := m["next_cursor"]; !ok || nc != nil {
		t.Errorf("%s: next_cursor = %v (present %v), want null", what, nc, ok)
	}
	out := without(t, what, m)
	delete(out, "next_cursor")
	var es []any
	for _, x := range m["entries"].([]any) {
		e := x.(map[string]any)
		if e["entry_count"] != float64(1) {
			t.Errorf("%s: entry %v has entry_count %v, want 1", what, e["id"], e["entry_count"])
		}
		for _, k := range ledgerWeightFields { // added by reported weights (C1)
			if _, ok := e[k]; !ok {
				t.Errorf("%s: entry %v lacks the added field %s", what, e["id"], k)
			}
		}
		c := map[string]any{}
		for k, v := range e {
			if k != "entry_count" && !isLedgerWeightField(k) {
				c[k] = v
			}
		}
		es = append(es, c)
	}
	out["entries"] = es
	return out
}

// withoutSetupWeightFields removes from a copy of a Setups list the fields
// reported weights add to each setup and its admin block (design C2),
// requiring each to be present.
func withoutSetupWeightFields(t *testing.T, what string, m map[string]any) map[string]any {
	t.Helper()
	list := []any{}
	for _, x := range m["setups"].([]any) {
		s := without(t, what+" setup", x.(map[string]any), "weight_source", "weight_attributes")
		if adm, ok := s["admin"].(map[string]any); ok {
			s["admin"] = without(t, what+" admin", adm, "reported_weight", "reported_weight_not_applied", "weight_override")
		}
		list = append(list, s)
	}
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	out["setups"] = list
	return out
}

// UPG-2: with no pre-approvals and no new request parameters, the ledger,
// setups and session responses and the SSO sign-in of a new user differ from
// the release only by the added fields (entry_count, next_cursor,
// label_index, display_name).
func TestUPG2_ResponsesAndSignInDifferFromTheReleaseOnlyByAddedFields(t *testing.T) {
	e, g := upgradeWorld(t)
	ids := g["facts"].(map[string]any)["user_ids"].(map[string]any)
	id := func(name string) int64 { return int64(ids[name].(float64)) }
	dev, other, nor, adm := e.sessionFor(id("dev")), e.sessionFor(id("other")), e.sessionFor(id("norole")), e.sessionFor(id("admin"))

	t.Run("ledger", func(t *testing.T) {
		for _, c := range []struct {
			key, path string
			s         *sess
		}{
			{"ledger_dev", "/api/v1/me/ledger", dev},
			{"ledger_dev_period", "/api/v1/me/ledger?period=2026-11", dev},
			{"ledger_other", "/api/v1/me/ledger", other},
		} {
			sameAsRelease(t, c.path, ledgerLikeRelease(t, c.path, e.getJSON(c.s, c.path)), g[c.key])
		}
	})
	t.Run("setups", func(t *testing.T) {
		for _, c := range []struct {
			key, path string
			s         *sess
		}{
			{"setups_dev", "/api/v1/setups", dev},
			{"setups_dev_filtered", "/api/v1/setups?selector=tier%3Da", dev},
			{"setups_admin", "/api/v1/setups", adm},
		} {
			sameAsRelease(t, c.key, withoutSetupWeightFields(t, c.key,
				without(t, c.key, e.getJSON(c.s, c.path), "label_index", "reported_weights_enabled", "pricing")), g[c.key])
		}
	})
	t.Run("session", func(t *testing.T) {
		for _, c := range []struct {
			key string
			s   *sess
		}{{"session_dev", dev}, {"session_admin", adm}, {"session_norole", nor}} {
			sameAsRelease(t, c.key, without(t, c.key, e.getJSON(c.s, "/api/v1/session"), "display_name"), g[c.key])
		}
	})
	t.Run("sso sign-in of a new user", func(t *testing.T) {
		want := g["signin_new_user"].(map[string]any)
		loc, s := e.signIn(map[string]any{"sub": "sub-new", "email": "new.user@example.test", "name": "New User"})
		if loc != want["location"] || (s != nil) != want["session_cookie_set"] {
			t.Errorf("redirect %q session %v, release: %v %v", loc, s != nil, want["location"], want["session_cookie_set"])
		}
		u, err := e.st.GetUserBySub(uBg, "sub-new")
		if err != nil {
			t.Fatal(err)
		}
		roles, _ := e.st.GetUserRoles(uBg, u.ID)
		got := map[string]any{"id": float64(u.ID), "status": u.Status, "name": u.Name, "email": u.Email,
			"balance_mt": float64(u.BalanceMT), "roles": float64(len(roles))}
		wu := want["user"].(map[string]any)
		wantUser := map[string]any{"id": wu["id"], "status": wu["status"], "name": wu["name"], "email": wu["email"],
			"balance_mt": wu["balance_mt"], "roles": float64(0)}
		if len(roles) != 0 {
			wantUser["roles"] = float64(len(roles))
		}
		sameAsRelease(t, "the stored user", got, wantUser)
		if s != nil {
			sameAsRelease(t, "the first session", without(t, "session", e.getJSON(s, "/api/v1/session"), "display_name"), want["session"])
		}
		var evs []any
		for _, ev := range e.ev.evs {
			b, _ := json.Marshal(ev)
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			delete(m, "time") // wall-clock time in the release's recording
			evs = append(evs, m)
		}
		wantEvs := []any{}
		for _, x := range want["events"].([]any) {
			m := x.(map[string]any)
			delete(m, "time")
			wantEvs = append(wantEvs, m)
		}
		if evs == nil {
			evs = []any{}
		}
		sameAsRelease(t, "the events of the sign-in", evs, wantEvs)
		au, err := e.st.ListAudit(uBg, store.AuditFilter{})
		if err != nil {
			t.Fatal(err)
		}
		var auditGot []any
		for i := len(au) - 1; i >= 0; i-- {
			a := au[i]
			var actor any
			if a.ActorID != nil {
				actor = float64(*a.ActorID)
			}
			auditGot = append(auditGot, map[string]any{"action": a.Action, "target": a.Target, "actor_id": actor,
				"actor_service": a.ActorService, "before": a.BeforeJSON, "after": a.AfterJSON,
				"created_at": a.CreatedAt.UTC().Format(time.RFC3339Nano)})
		}
		sameAsRelease(t, "the audit log after the sign-in", auditGot, want["audit"])
	})
}

// contractShape lists every route, parameter, request field and response
// field of an OpenAPI document as strings, following $ref, allOf, oneOf and
// anyOf (the union of the branches).
func contractShape(t *testing.T, path string) map[string]bool {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	out := map[string]bool{}
	var walk func(prefix string, ref *openapi3.SchemaRef, seen map[*openapi3.Schema]bool)
	walk = func(prefix string, ref *openapi3.SchemaRef, seen map[*openapi3.Schema]bool) {
		if ref == nil || ref.Value == nil || seen[ref.Value] {
			return
		}
		s := ref.Value
		seen = copySeen(seen)
		seen[s] = true
		for name, p := range s.Properties {
			out[prefix+"."+name] = true
			walk(prefix+"."+name, p, seen)
		}
		if s.Items != nil {
			walk(prefix+"[]", s.Items, seen)
		}
		if s.AdditionalProperties.Schema != nil {
			walk(prefix+"{}", s.AdditionalProperties.Schema, seen)
		}
		for _, list := range []openapi3.SchemaRefs{s.AllOf, s.OneOf, s.AnyOf} {
			for _, b := range list {
				walk(prefix, b, seen)
			}
		}
	}
	for p, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			key := method + " " + p
			out["route "+key] = true
			for _, pr := range append(append(openapi3.Parameters{}, item.Parameters...), op.Parameters...) {
				out["param "+key+" "+pr.Value.In+":"+pr.Value.Name] = true
			}
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				for mt, c := range op.RequestBody.Value.Content {
					walk("request "+key+" "+mt, c.Schema, map[*openapi3.Schema]bool{})
				}
			}
			for code, r := range op.Responses.Map() {
				out["response "+key+" "+code] = true
				if r.Value == nil {
					continue
				}
				for mt, c := range r.Value.Content {
					walk("response "+key+" "+code+" "+mt, c.Schema, map[*openapi3.Schema]bool{})
				}
			}
		}
	}
	return out
}

func copySeen(m map[*openapi3.Schema]bool) map[*openapi3.Schema]bool {
	c := make(map[*openapi3.Schema]bool, len(m)+1)
	for k, v := range m {
		c[k] = v
	}
	return c
}

// UPG-3: no route, parameter, request field, response code or response field
// of the release's api/openapi.yaml (d96a950) is missing from today's file.
// The contract tests (TestUI2_ContractRoutesMatch and
// TestUI2_ContractResponsesMatchSchemas) check that the file and the handlers
// agree; this test checks that nothing was removed or renamed.
func TestUPG3_NoRouteParameterOrFieldOfTheReleaseIsRemoved(t *testing.T) {
	old := contractShape(t, findUp(t, upgradeDir+"/openapi.yaml"))
	cur := contractShape(t, findUp(t, "api/openapi.yaml"))
	if len(old) < 300 {
		t.Fatalf("only %d items found in the release's contract; the walk is not reading it", len(old))
	}
	var missing []string
	for k := range old {
		if !cur[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d items of the release's api/openapi.yaml are not in today's file:\n%s", len(missing), strings.Join(missing, "\n"))
	}
	added := 0
	for k := range cur {
		if !old[k] {
			added++
		}
	}
	if added == 0 {
		t.Error("today's contract has nothing the release's lacks; the comparison reads the same file twice")
	}
}
