// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"

	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// The contract test (UI-2): api/openapi.yaml and the Go handlers must agree.
//
//   - TestUI2_ContractRoutesMatch: the (method, path) set, the access level
//     (x-roles) and where CSRF is enforced (x-csrf) equal Routes() and the
//     router's behavior.
//   - TestUI2_ContractResponsesMatchSchemas: real responses of every operation,
//     from a world with data, validate against the operation's schemas, which
//     are treated as closed (an unknown field fails) unless they declare
//     additionalProperties. Request bodies the test sends validate too.
//   - Every date-time of every response is RFC 3339 UTC with milliseconds.

// strictDateTime is the one timestamp format of the API.
var strictDateTime = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$`)

// dateTimesSeen counts the date-time values the validator has checked.
var dateTimesSeen int

func init() {
	openapi3.DefineStringFormatValidator("date-time", openapi3.NewCallbackValidator(func(s string) error {
		dateTimesSeen++
		if !strictDateTime.MatchString(s) {
			return fmt.Errorf("%q is not RFC 3339 UTC with milliseconds and a Z suffix (2006-01-02T15:04:05.000Z)", s)
		}
		if _, err := time.Parse(time.RFC3339Nano, s); err != nil {
			return err
		}
		return nil
	}))
}

// loadContract loads api/openapi.yaml, validates the document and closes
// every object schema that has properties and does not declare
// additionalProperties.
func loadContract(t *testing.T) *openapi3.T {
	t.Helper()
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(findUp(t, "api/openapi.yaml"))
	if err != nil {
		t.Fatalf("api/openapi.yaml does not load: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("api/openapi.yaml is not a valid OpenAPI document: %v", err)
	}
	closeSchemas(doc)
	return doc
}

func closeSchemas(doc *openapi3.T) {
	seen := map[*openapi3.Schema]bool{}
	var walk func(ref *openapi3.SchemaRef, inAllOf bool)
	walk = func(ref *openapi3.SchemaRef, inAllOf bool) {
		if ref == nil || ref.Value == nil || seen[ref.Value] {
			return
		}
		s := ref.Value
		seen[s] = true
		// A branch of an allOf with several object branches cannot be closed on
		// its own: it would reject the fields of its siblings.
		if len(s.Properties) > 0 && s.AdditionalProperties.Has == nil && s.AdditionalProperties.Schema == nil && !inAllOf {
			closed := false
			s.AdditionalProperties.Has = &closed
		}
		for _, p := range s.Properties {
			walk(p, false)
		}
		walk(s.Items, false)
		walk(s.AdditionalProperties.Schema, false)
		for _, l := range [][]*openapi3.SchemaRef{s.AnyOf, s.OneOf} {
			for _, c := range l {
				walk(c, false)
			}
		}
		for _, c := range s.AllOf {
			walk(c, len(s.AllOf) > 1)
		}
	}
	for _, ref := range doc.Components.Schemas {
		walk(ref, false)
	}
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			for _, p := range op.Parameters {
				walk(p.Value.Schema, false)
			}
			if op.RequestBody != nil {
				for _, mt := range op.RequestBody.Value.Content {
					walk(mt.Schema, false)
				}
			}
			for _, r := range op.Responses.Map() {
				for _, mt := range r.Value.Content {
					walk(mt.Schema, false)
				}
			}
		}
	}
}

// extString reads an operation extension as a string ("" when absent).
func extString(op *openapi3.Operation, name string) string {
	v, ok := op.Extensions[name]
	if !ok {
		return ""
	}
	raw, _ := json.Marshal(v)
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.Trim(string(raw), `"`)
}

func TestContract_TheDocumentLoadsAndTimestampsAreStrict(t *testing.T) {
	doc := loadContract(t)
	if doc.Paths.Len() == 0 {
		t.Fatal("no paths")
	}
	// Response schemas are closed: a field the schema does not name fails.
	errSchema := doc.Components.Schemas["Error"].Value
	if err := errSchema.VisitJSON(map[string]any{"error": map[string]any{"code": "internal", "message": "x"}}); err != nil {
		t.Errorf("the plain error object does not validate: %v", err)
	}
	if err := errSchema.VisitJSON(map[string]any{"error": map[string]any{"code": "internal", "message": "x", "extra": 1}}); err == nil {
		t.Error("an unknown field in the error object validates; schemas must be closed")
	}
	// The format checker itself bites: the other RFC 3339 spellings fail.
	schema := openapi3.NewStringSchema().WithFormat("date-time")
	for s, ok := range map[string]bool{
		"2026-11-19T10:00:00.000Z":       true,
		"2026-11-19T10:00:00Z":           false, // no milliseconds
		"2026-11-19T10:00:00.123456789Z": false, // nanoseconds
		"2026-11-19T10:00:00.000+00:00":  false, // not the Z suffix
		"2026-11-19T10:00:00.000+02:00":  false,
		"2026-11-19 10:00:00.000Z":       false,
	} {
		err := schema.VisitJSON(s)
		if (err == nil) != ok {
			t.Errorf("date-time %q: accepted=%v, want %v (%v)", s, err == nil, ok, err)
		}
	}
}

// ---- route parity ----

func TestUI2_ContractRoutesMatch(t *testing.T) {
	doc := loadContract(t)
	type key struct{ method, path string }
	inYAML := map[key]*openapi3.Operation{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			inYAML[key{method, path}] = op
		}
	}
	inGo := map[key]Route{}
	for _, r := range Routes() {
		inGo[key{r.Method, r.Pattern}] = r
	}
	for k, r := range inGo {
		op, ok := inYAML[k]
		if !ok {
			t.Errorf("route %s %s (%s) is in the handlers but not in api/openapi.yaml", k.method, k.path, r.OperationID)
			continue
		}
		if op.OperationID != r.OperationID {
			t.Errorf("%s %s: operationId %q in the YAML, %q in the router", k.method, k.path, op.OperationID, r.OperationID)
		}
		if roles := extString(op, "x-roles"); roles != string(r.Access) {
			t.Errorf("%s %s: x-roles %q in the YAML, access level %q in the router", k.method, k.path, roles, r.Access)
		}
		csrf := extString(op, "x-csrf") == "true"
		if csrf != r.StateChanging() {
			t.Errorf("%s %s: x-csrf = %v in the YAML, the router enforces CSRF = %v", k.method, k.path, csrf, r.StateChanging())
		}
	}
	for k, op := range inYAML {
		if _, ok := inGo[k]; !ok {
			t.Errorf("api/openapi.yaml documents %s %s (%s) but no handler serves it", k.method, k.path, op.OperationID)
		}
	}
	if len(inYAML) != len(inGo) {
		t.Errorf("%d operations in the YAML, %d routes", len(inYAML), len(inGo))
	}
}

// x-csrf is the truth about the running router: with the session's valid
// cookie and no token, exactly the operations that declare x-csrf: true answer
// 403 csrf.
func TestContract_CSRFDeclarationMatchesBehavior(t *testing.T) {
	doc := loadContract(t)
	f := newPermFix(t)
	adm, local := f.actor(actAdmin), f.actor(actLocal)
	checked := 0
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			var r Route
			for _, c := range Routes() {
				if c.Method == method && c.Pattern == path {
					r = c
				}
			}
			a := adm
			if r.Access == AccessLocalAdmin {
				a = local
			}
			p := f.probe(r.OperationID, a, modeDefault)
			if a.localAdmin {
				f.setMustChange(false)
			}
			s := a.s
			if p.as != nil {
				s = p.as
			}
			rec := f.serve(p.method, p.path, s, false, p.body)
			isCSRF := rec.Code == http.StatusForbidden && permErrCode(rec) == "csrf"
			if want := extString(op, "x-csrf") == "true"; isCSRF != want {
				t.Errorf("%s %s: x-csrf = %v but a request without the token got %d %s", method, path, want, rec.Code, rec.Body)
			}
			checked++
		}
	}
	if checked != len(Routes()) {
		t.Errorf("checked %d operations, want %d", checked, len(Routes()))
	}
}

// ---- response validation ----

// contract sends requests through a backend and validates them and their
// responses against the document.
type contract struct {
	t      *testing.T
	doc    *openapi3.T
	router routers.Router
	// per operationId: validated success (<400) and error (>=400) responses
	ok, bad map[string]int
}

func newContract(t *testing.T) *contract {
	t.Helper()
	doc := loadContract(t)
	rt, err := legacy.NewRouter(doc)
	if err != nil {
		t.Fatal(err)
	}
	return &contract{t: t, doc: doc, router: rt, ok: map[string]int{}, bad: map[string]int{}}
}

// call sends one request to b. s may be nil; csrf sends the session's token.
// validRequest says the request is meant to be valid: its parameters and body
// are validated against the operation too. It returns the response.
func (c *contract) call(b *backend, validRequest bool, method, path string, s *sess, csrf bool, body string) *httptest.ResponseRecorder {
	c.t.Helper()
	newReq := func() *http.Request {
		var rd io.Reader
		if body != "" {
			rd = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, rd)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if s != nil {
			req.Header.Set("Cookie", auth.SessionCookie+"="+s.cookie)
			if csrf {
				req.Header.Set(auth.CSRFHeader, s.csrf)
			}
		}
		return req
	}
	vreq := newReq()
	route, params, err := c.router.FindRoute(vreq)
	if err != nil {
		c.t.Errorf("%s %s: no such operation in api/openapi.yaml: %v", method, path, err)
		return nil
	}
	rvi := &openapi3filter.RequestValidationInput{Request: vreq, PathParams: params, Route: route,
		Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}}
	op := route.Operation.OperationID
	if validRequest {
		if err := openapi3filter.ValidateRequest(context.Background(), rvi); err != nil {
			c.t.Errorf("%s (%s %s): the request does not match the contract: %v", op, method, path, err)
		}
	}
	rec := httptest.NewRecorder()
	b.h.ServeHTTP(rec, newReq())

	rvi.Request = newReq()
	in := &openapi3filter.ResponseValidationInput{RequestValidationInput: rvi, Status: rec.Code, Header: rec.Header(),
		Options: &openapi3filter.Options{IncludeResponseStatus: true}}
	in.SetBodyBytes(rec.Body.Bytes())
	if err := openapi3filter.ValidateResponse(context.Background(), in); err != nil {
		c.t.Errorf("%s (%s %s) answered %d and the response does not match the contract: %v\nbody: %s", op, method, path, rec.Code, err, rec.Body)
		return rec
	}
	if rec.Code >= 400 {
		c.bad[op]++
		// Every JSON error is also the Error schema, whatever the operation documents.
		if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") && op != "getHealth" {
			var v any
			if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
				c.t.Errorf("%s: error body is not JSON: %s", op, rec.Body)
			} else if err := c.doc.Components.Schemas["Error"].Value.VisitJSON(v); err != nil {
				c.t.Errorf("%s: error body is not the Error schema: %v: %s", op, err, rec.Body)
			}
		}
	} else {
		c.ok[op]++
	}
	return rec
}

// callBearer sends one request with a service token and no session, and
// validates the response like call.
func (c *contract) callBearer(b *backend, method, path, token, body string) *httptest.ResponseRecorder {
	c.t.Helper()
	newReq := func() *http.Request {
		var rd io.Reader
		if body != "" {
			rd = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, rd)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return req
	}
	vreq := newReq()
	route, params, err := c.router.FindRoute(vreq)
	if err != nil {
		c.t.Errorf("%s %s: no such operation in api/openapi.yaml: %v", method, path, err)
		return nil
	}
	rec := httptest.NewRecorder()
	b.h.ServeHTTP(rec, newReq())
	op := route.Operation.OperationID
	rvi := &openapi3filter.RequestValidationInput{Request: newReq(), PathParams: params, Route: route,
		Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}}
	in := &openapi3filter.ResponseValidationInput{RequestValidationInput: rvi, Status: rec.Code, Header: rec.Header(),
		Options: &openapi3filter.Options{IncludeResponseStatus: true}}
	in.SetBodyBytes(rec.Body.Bytes())
	if err := openapi3filter.ValidateResponse(context.Background(), in); err != nil {
		c.t.Errorf("%s (%s %s) answered %d and the response does not match the contract: %v\nbody: %s", op, method, path, rec.Code, err, rec.Body)
		return rec
	}
	if rec.Code >= 400 {
		c.bad[op]++
	} else {
		c.ok[op]++
	}
	return rec
}

// world is a backend with data in every part of the API.
type world struct {
	*backend
	c                    *contract
	devA, devB, adm      sess
	local                sess
	pending, rejected    sess
	noRole               sess
	setupA, setupB, free int64
	holdA, holdB         int64
	idp                  *httptest.Server
}

func (w *world) must(status int, method, path string, s *sess, body string) map[string]any {
	w.t.Helper()
	rec := w.c.call(w.backend, true, method, path, s, true, body)
	if rec == nil {
		w.t.FailNow()
	}
	if rec.Code != status {
		w.t.Fatalf("%s %s: %d, want %d: %s", method, path, rec.Code, status, rec.Body)
	}
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	return m
}

func (w *world) get(path string, s *sess) *httptest.ResponseRecorder {
	w.t.Helper()
	return w.c.call(w.backend, true, "GET", path, s, false, "")
}

func idpServer(t *testing.T) *httptest.Server {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": srv.URL, "authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint": srv.URL + "/token", "jwks_uri": srv.URL + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

const contractSettingsBody = `{"default_weight":1,"display_label_keys":["tier","site"],` +
	`"bands":{"base":0,"steps":[{"from":0.5,"multiplier":1},{"from":0.75,"multiplier":2},{"from":0.9,"multiplier":3}]},"default_allowance":300,` +
	`"max_holds_per_user":2,"release_requires_teardown":true,"poll_interval":"5m","enforcement":true}`

func newWorld(t *testing.T) *world {
	t.Helper()
	idp := idpServer(t)
	b := newBackend(t, backendOpts{start: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC), startEvents: true, oidcIssuer: idp.URL,
		runtime: Runtime{Provider: "fake-provider", PressureSource: "fake-pressure", MeteringPolicy: "fake-policy",
			Hooks: []string{"fake-hook"}, OIDCIssuer: idp.URL,
			ReportedProviderMaxAge: 10 * time.Minute, ReportedPressureMaxAge: 10 * time.Minute},
		preFile: []PreapprovalEntry{{Email: "both@example.test", Roles: []string{store.RoleAdmin}},
			{Email: "file-only@example.test", Roles: []string{store.RoleDeveloper}}}})
	b.policy.SetDefault(hourglass.Decision{Meter: true, Reason: "lab is busy"})
	w := &world{backend: b, c: newContract(t), idp: idp}
	ctx := context.Background()

	w.devA = b.newUser("dev-a", store.StatusActive, store.RoleDeveloper)
	w.devB = b.newUser("dev-b", store.StatusActive, store.RoleDeveloper)
	w.adm = b.newUser("boss", store.StatusActive, store.RoleAdmin)
	w.local = b.session(b.admin)
	w.pending = b.newUser("pending", store.StatusPending)
	w.rejected = b.newUser("rejected", store.StatusRejected)
	w.noRole = b.newUser("no-role", store.StatusActive)

	// Settings, rate rules and setups through the admin API.
	w.must(200, "PUT", "/api/v1/admin/settings", &w.adm, contractSettingsBody)
	w.must(200, "PUT", "/api/v1/admin/rules", &w.adm, `{"rules":[{"selector":"tier=a","weight":2},{"selector":"","weight":1}]}`)
	mk := func(name string, enabled bool, labels string) int64 {
		m := w.must(201, "POST", "/api/v1/admin/setups", &w.adm, fmt.Sprintf(
			`{"name":%q,"labels":%s,"details":{"console":"https://example.test/%s","n":[1,2]},"enabled":%t}`, name, labels, name, enabled))
		return int64(m["id"].(float64))
	}
	w.setupA = mk("lab-01", true, `{"tier":"a","site":"x"}`)
	w.setupB = mk("lab-02", true, `{"tier":"b","site":"y"}`)
	w.free = mk("lab-03", true, `{"tier":"b"}`)
	mk("lab-04", false, `{}`)

	// Holds through the API; the provider reports two of the three enabled setups
	// and a resource no setup matches (an orphan).
	b.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-01", Labels: map[string]string{"site": "x"}},
		hourglass.Allocation{ResourceName: "lab-02"}, hourglass.Allocation{ResourceName: "lab-03", Labels: map[string]string{"site": "z"}},
		hourglass.Allocation{ResourceName: "unrelated"})
	w.holdA = int64(w.must(201, "POST", fmt.Sprintf("/api/v1/setups/%d/acquire", w.setupA), &w.devA, "")["id"].(float64))
	w.holdB = int64(w.must(201, "POST", fmt.Sprintf("/api/v1/setups/%d/acquire", w.setupB), &w.devB, "")["id"].(float64))

	// Polls: debits at two pressure levels, a failed poll, an outage, a failed delivery.
	poll := func(v float64) {
		b.pressure.SetValue(v)
		if err := b.col.RunOnce(ctx); err != nil {
			t.Fatalf("poll: %v", err)
		}
	}
	poll(0.6)
	for _, v := range []float64{0.8, 0.95, 0.7} {
		b.clk.Advance(5 * time.Minute)
		poll(v)
	}
	b.clk.Advance(5 * time.Minute)
	b.pressure.SetError(fmt.Errorf("pressure source down"))
	if err := b.col.RunOnce(ctx); err == nil {
		t.Fatal("poll should have failed")
	}
	b.pressure.SetError(nil)
	b.clk.Advance(5 * time.Minute)
	poll(0.8)
	if _, err := b.st.InsertOutage(ctx, store.Outage{StartedAt: b.clk.Now().Add(-3 * time.Hour), EndedAt: b.clk.Now().Add(-2 * time.Hour), Reason: "gap"}); err != nil {
		t.Fatal(err)
	}
	id, err := b.st.InsertEventDelivery(ctx, "hold.acquired", "fake-hook", `{}`, b.clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := b.st.FinishEventDelivery(ctx, id, b.clk.Now(), false, "hook failed"); err != nil {
		t.Fatal(err)
	}

	// Admin actions: grants, reset, roles, reject, setup edits, a forced release.
	uid := func(s sess) string { return fmt.Sprint(s.user.ID) }
	w.must(201, "POST", "/api/v1/admin/users/"+uid(w.devA)+"/grants", &w.adm, `{"amount_tokens":25.5,"reason":"extra for the demo"}`)
	w.must(201, "POST", "/api/v1/admin/users/"+uid(w.devB)+"/grants", &w.adm, `{"amount_tokens":-10,"reason":"correction"}`)
	p2, p3 := b.newUser("pending-2", store.StatusPending), b.newUser("pending-3", store.StatusPending)
	w.must(200, "PUT", "/api/v1/admin/users/"+uid(p2)+"/roles", &w.adm, `{"roles":["developer"]}`)
	w.must(200, "POST", "/api/v1/admin/users/"+uid(p3)+"/reject", &w.adm, "")
	w.must(201, "POST", "/api/v1/admin/users/"+uid(p2)+"/reset", &w.adm, `{"reason":"month start"}`)
	// A developer whose roles are removed: the admin list has null balance and
	// month-to-date burn for them (LED-3), which the schema must allow.
	demoted := b.newUser("demoted", store.StatusActive, store.RoleDeveloper)
	w.must(200, "PUT", "/api/v1/admin/users/"+uid(demoted)+"/roles", &w.adm, `{"roles":[]}`)
	extra := mk("lab-05", true, `{"tier":"a"}`)
	w.must(200, "PUT", fmt.Sprintf("/api/v1/admin/setups/%d", extra), &w.adm,
		`{"name":"lab-05","labels":{"tier":"a"},"details":{"edited":true},"enabled":true}`)
	w.must(204, "DELETE", fmt.Sprintf("/api/v1/admin/setups/%d", extra), &w.adm, "")
	// devB's hold is force-released; devB then acquires again so there is a released and an active hold.
	w.must(200, "POST", fmt.Sprintf("/api/v1/holds/%d/release", w.holdB), &w.adm, `{"force":true,"reason":"maintenance window"}`)
	// devA releases nothing yet; a second setup is acquired by devA and released by them (a released, own hold).
	h2 := int64(w.must(201, "POST", fmt.Sprintf("/api/v1/setups/%d/acquire", w.free), &w.devA, "")["id"].(float64))
	b.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-01", Labels: map[string]string{"site": "x"}}, hourglass.Allocation{ResourceName: "unrelated"})
	b.clk.Advance(5 * time.Minute)
	poll(0.8)
	w.must(200, "POST", fmt.Sprintf("/api/v1/holds/%d/release", h2), &w.devA, "")
	b.clk.Advance(5 * time.Minute)
	poll(0.8)
	if err := b.ev.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	return w
}

func (w *world) status(rec *httptest.ResponseRecorder, want int) {
	w.t.Helper()
	if rec == nil {
		w.t.FailNow()
	}
	if rec.Code != want {
		w.t.Fatalf("status %d, want %d: %s", rec.Code, want, rec.Body)
	}
}

func TestUI2_ContractResponsesMatchSchemas(t *testing.T) {
	w := newWorld(t)
	c := w.c
	dateTimesSeen = 0

	// Success responses, with data, as each kind of user.
	for _, s := range []*sess{&w.devA, &w.devB, &w.adm} {
		w.status(w.get("/api/v1/me", s), 200)
		w.status(w.get("/api/v1/me/holds", s), 200)
		w.status(w.get("/api/v1/me/holds?state=active", s), 200)
		w.status(w.get("/api/v1/me/holds?state=released", s), 200)
		w.status(w.get("/api/v1/me/ledger", s), 200)
		w.status(w.get("/api/v1/me/ledger?period=2026-11", s), 200)
		w.status(w.get("/api/v1/me/ledger?rollup=day", s), 200)
		w.status(w.get("/api/v1/me/ledger?limit=1", s), 200)
		w.status(w.get("/api/v1/me/ledger?rollup=day&limit=2&period=2026-11", s), 200)
		w.status(w.get("/api/v1/me/usage", s), 200)
		w.status(w.get("/api/v1/pressure", s), 200)
		w.status(w.get("/api/v1/setups", s), 200)
		w.status(w.get("/api/v1/setups?selector=tier%3Da", s), 200)
		w.status(w.get("/api/v1/session", s), 200)
	}
	// A ledger page with a real cursor (LED-13).
	if nc, _ := w.must(200, "GET", "/api/v1/me/ledger?limit=1", &w.devA, "")["next_cursor"].(string); nc != "" {
		w.status(w.get("/api/v1/me/ledger?limit=1&cursor="+nc, &w.devA), 200)
	} else {
		t.Error("ledger?limit=1: no next_cursor for a ledger with more than one entry")
	}
	// The local admin: no balance, no holds.
	for _, p := range []string{"/api/v1/me", "/api/v1/me/holds", "/api/v1/me/ledger", "/api/v1/me/usage", "/api/v1/session", "/api/v1/setups", "/api/v1/pressure"} {
		w.status(w.get(p, &w.local), 200)
	}
	// Roles-less users get the session and nothing else.
	for _, s := range []*sess{&w.pending, &w.rejected, &w.noRole} {
		w.status(w.get("/api/v1/session", s), 200)
	}
	// Stored reports, so the collector answer carries both report objects.
	{
		at := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC) // older than any report the later steps send
		if err := w.st.ReplaceReportedPressure(context.Background(), store.ReportedPressure{Value: 0.4, ObservedAt: at, ReceivedAt: at, ServiceAccount: "reporter"}); err != nil {
			t.Fatal(err)
		}
		if err := w.st.ReplaceReportedDeployments(context.Background(), store.ReportedHeader{ObservedAt: at, ReceivedAt: at, ServiceAccount: "reporter"}, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"/api/v1/admin/users", "/api/v1/admin/users?status=pending", "/api/v1/admin/rules", "/api/v1/admin/settings",
		"/api/v1/admin/collector", "/api/v1/admin/audit", "/api/v1/admin/audit?action=settings.update", "/api/v1/admin/audit?limit=2"} {
		w.status(w.get(p, &w.adm), 200)
	}
	w.status(w.get("/api/v1/auth/methods", nil), 200)
	w.status(w.get("/healthz", nil), 200)

	// The public sign-in calls.
	w.status(c.call(w.backend, true, "GET", "/auth/oidc/start", nil, false, ""), 302)
	w.status(c.call(w.backend, true, "GET", "/auth/callback?error=access_denied", nil, false, ""), 302)
	w.status(c.call(w.backend, true, "GET", "/auth/callback", nil, false, ""), 302) // every failure is a redirect
	w.status(c.call(w.backend, true, "POST", "/auth/local", nil, false, `{"username":"admin","password":"wrong password here"}`), 401)
	w.status(c.call(w.backend, true, "POST", "/auth/local", nil, false, `{"username":"","password":""}`), 400)
	rec := c.call(w.backend, true, "POST", "/auth/local", nil, false, `{"username":"admin","password":"`+testAdminPassword+`"}`)
	w.status(rec, 200)
	// Without OIDC configured the start route is a documented 404.
	plain := newBackend(t, backendOpts{})
	w.status(c.call(plain, true, "GET", "/auth/oidc/start", nil, false, ""), 404)
	w.status(c.call(plain, true, "GET", "/auth/callback", nil, false, ""), 404)
	// An unreachable database is the documented 503 of /healthz.
	sick := newBackend(t, backendOpts{})
	sick.st.Close()
	w.status(c.call(sick, true, "GET", "/healthz", nil, false, ""), 503)

	// Errors: every operation refuses an anonymous caller in the documented way,
	// and the usual domain refusals are documented too.
	for _, r := range Routes() {
		if r.Access == AccessPublic {
			continue
		}
		path := strings.ReplaceAll(r.Pattern, "{id}", fmt.Sprint(w.holdA))
		body := ""
		if r.Method == "POST" || r.Method == "PUT" {
			body = "{}"
		}
		w.status(c.call(w.backend, false, r.Method, path, nil, false, body), 401)
	}
	for _, r := range Routes() {
		if r.Access != AccessAdmin && r.Access != AccessRole {
			continue
		}
		path := strings.ReplaceAll(r.Pattern, "{id}", fmt.Sprint(w.holdA))
		w.status(c.call(w.backend, false, r.Method, path, &w.noRole, true, ""), 403)
	}
	pending := w.newPending()
	for _, tc := range []struct {
		status int
		method string
		path   string
		as     *sess
		body   string
	}{
		{400, "PUT", "/api/v1/admin/users/" + fmt.Sprint(pending.user.ID) + "/roles", &w.adm, `{"roles":["manager"]}`},
		{404, "PUT", "/api/v1/admin/users/99999/roles", &w.adm, `{"roles":["developer"]}`},
		{409, "PUT", "/api/v1/admin/users/" + fmt.Sprint(w.admin.ID) + "/roles", &w.adm, `{"roles":[]}`},
		{409, "PUT", "/api/v1/admin/users/" + fmt.Sprint(w.adm.user.ID) + "/roles", &w.adm, `{"roles":["developer"]}`},
		{409, "POST", "/api/v1/admin/users/" + fmt.Sprint(w.devA.user.ID) + "/reject", &w.adm, ""},
		{400, "POST", "/api/v1/admin/users/" + fmt.Sprint(w.devA.user.ID) + "/grants", &w.adm, `{"amount_tokens":1}`},
		{404, "POST", "/api/v1/admin/users/99999/grants", &w.adm, `{"amount_tokens":1,"reason":"x"}`},
		{400, "POST", "/api/v1/admin/users/" + fmt.Sprint(w.devA.user.ID) + "/reset", &w.adm, `{}`},
		{400, "POST", "/api/v1/admin/setups", &w.adm, `{"name":"","labels":{},"details":{},"enabled":true}`},
		{409, "POST", "/api/v1/admin/setups", &w.adm, `{"name":"lab-01","labels":{},"details":{},"enabled":true}`},
		{404, "PUT", "/api/v1/admin/setups/99999", &w.adm, `{"name":"zz","labels":{},"details":{},"enabled":true}`},
		{409, "DELETE", fmt.Sprintf("/api/v1/admin/setups/%d", w.setupA), &w.adm, ""},
		{404, "DELETE", "/api/v1/admin/setups/99999", &w.adm, ""},
		{400, "PUT", "/api/v1/admin/rules", &w.adm, `{"rules":[{"selector":"tier in (a","weight":2}]}`},
		{400, "PUT", "/api/v1/admin/settings", &w.adm, `{}`},
		{400, "GET", "/api/v1/admin/audit?limit=0", &w.adm, ""},
		{400, "GET", "/api/v1/me/holds?state=bogus", &w.devA, ""},
		{400, "GET", "/api/v1/me/ledger?period=nope", &w.devA, ""},
		{400, "GET", "/api/v1/me/ledger?rollup=week", &w.devA, ""},
		{400, "GET", "/api/v1/me/ledger?limit=0", &w.devA, ""},
		{400, "GET", "/api/v1/me/ledger?cursor=5", &w.devA, ""},
		{400, "GET", "/api/v1/me/ledger?limit=5&cursor=abc", &w.devA, ""},
		{404, "POST", "/api/v1/setups/99999/acquire", &w.devA, ""},
		{409, "POST", fmt.Sprintf("/api/v1/setups/%d/acquire", w.setupA), &w.devB, ""},
		{409, "POST", fmt.Sprintf("/api/v1/setups/%d/acquire", w.setupA+3), &w.devB, ""},
		{403, "POST", fmt.Sprintf("/api/v1/setups/%d/acquire", w.free), &w.local, ""},
		{403, "POST", fmt.Sprintf("/api/v1/holds/%d/release", w.holdA), &w.devB, ""},
		{404, "POST", "/api/v1/holds/99999/release", &w.devB, ""},
		{409, "POST", fmt.Sprintf("/api/v1/holds/%d/release", w.holdB), &w.devB, ""},
		{202, "POST", fmt.Sprintf("/api/v1/holds/%d/release", w.holdA), &w.devA, ""}, // still deployed: pending (HOLD-6)
		{403, "POST", "/auth/local/password", &w.devA, `{"current_password":"a","new_password":"b"}`},
		{400, "POST", "/auth/local/password", &w.local, `{"current_password":"wrong","new_password":"another long password"}`},
	} {
		rec := c.call(w.backend, false, tc.method, tc.path, tc.as, true, tc.body)
		w.status(rec, tc.status)
	}
	// Pre-approvals (PRE-5, PRE-6): add, update, a used entry, the list with
	// both sources, the refusals and a delete.
	w.must(200, "POST", "/api/v1/admin/preapprovals", &w.adm, `{"emails":["new@example.test"," Both@Example.test "],"roles":["developer"]}`)
	w.must(200, "POST", "/api/v1/admin/preapprovals", &w.adm, `{"emails":["new@example.test"],"roles":["admin","developer"]}`)
	{
		ctx := context.Background()
		p, err := w.st.GetPreapprovalByEmail(ctx, "both@example.test")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.st.MarkPreapprovalUsed(ctx, p.ID, w.devA.user.ID, w.clk.Now()); err != nil {
			t.Fatal(err)
		}
	}
	list := w.must(200, "GET", "/api/v1/admin/preapprovals", &w.adm, "")
	if n := len(list["preapprovals"].([]any)); n != 4 {
		t.Errorf("pre-approval list holds %d entries, want 4 (two database, two file)", n)
	}
	newID := ""
	for _, e := range list["preapprovals"].([]any) {
		if m := e.(map[string]any); m["email"] == "new@example.test" {
			newID = fmt.Sprint(int64(m["id"].(float64)))
		}
	}
	for _, tc := range []struct {
		status       int
		method, path string
		body         string
	}{
		{400, "POST", "/api/v1/admin/preapprovals", `{"emails":["not-an-email"],"roles":["developer"]}`},
		{400, "POST", "/api/v1/admin/preapprovals", `{"emails":["a@example.test"],"roles":[]}`},
		{400, "POST", "/api/v1/admin/preapprovals", `{"emails":[],"roles":["developer"]}`},
		{404, "DELETE", "/api/v1/admin/preapprovals/99999", ""},
		{400, "DELETE", "/api/v1/admin/preapprovals/x", ""},
	} {
		w.status(c.call(w.backend, false, tc.method, tc.path, &w.adm, true, tc.body), tc.status)
	}
	w.must(204, "DELETE", "/api/v1/admin/preapprovals/"+newID, &w.adm, "")
	// Service accounts: the admin routes with data, and their refusals.
	acct := w.must(201, "POST", "/api/v1/admin/service-accounts", &w.adm,
		`{"name":"deployer","description":"lab automation","scopes":["state:read","deployments:write"]}`)
	aid := fmt.Sprint(int64(acct["id"].(float64)))
	w.must(201, "POST", "/api/v1/admin/service-accounts/"+aid+"/tokens", &w.adm, `{"expires_at":"2027-01-01T00:00:00.000Z"}`)
	second := w.must(201, "POST", "/api/v1/admin/service-accounts/"+aid+"/tokens", &w.adm, "")
	w.must(200, "PUT", "/api/v1/admin/service-accounts/"+aid, &w.adm, `{"description":"","scopes":["state:read"],"enabled":false}`)
	w.must(200, "PUT", "/api/v1/admin/service-accounts/"+aid, &w.adm, `{"description":"lab automation","scopes":["state:read"],"enabled":true}`)
	// A use records last_used_at, so the listing has every timestamp filled.
	tok := second["token"].(string)
	w.status(c.callBearer(w.backend, "GET", "/api/v1/integration/state", tok, ""), 200) // before any report
	w.status(w.get("/api/v1/admin/service-accounts", &w.adm), 200)
	for _, tc := range []struct {
		status       int
		method, path string
		body         string
	}{
		{400, "POST", "/api/v1/admin/service-accounts", `{"name":"x","scopes":["admin"]}`},
		{400, "POST", "/api/v1/admin/service-accounts", `{"name":"","scopes":[]}`},
		{409, "POST", "/api/v1/admin/service-accounts", `{"name":"deployer","scopes":[]}`},
		{400, "PUT", "/api/v1/admin/service-accounts/" + aid, `{"description":"","scopes":["nope"],"enabled":true}`},
		{404, "PUT", "/api/v1/admin/service-accounts/99999", `{"description":"","scopes":[],"enabled":true}`},
		{400, "POST", "/api/v1/admin/service-accounts/" + aid + "/tokens", `{"expires_at":"2020-01-01T00:00:00.000Z"}`},
		{404, "POST", "/api/v1/admin/service-accounts/99999/tokens", ""},
		{404, "DELETE", "/api/v1/admin/service-accounts/" + aid + "/tokens/99999", ""},
		{400, "DELETE", "/api/v1/admin/service-accounts/" + aid + "/tokens/x", ""},
	} {
		w.status(c.call(w.backend, false, tc.method, tc.path, &w.adm, true, tc.body), tc.status)
	}
	w.must(204, "DELETE", "/api/v1/admin/service-accounts/"+aid+"/tokens/"+fmt.Sprint(int64(second["id"].(float64))), &w.adm, "")
	w.must(204, "DELETE", "/api/v1/admin/service-accounts/"+aid, &w.adm, "")
	w.status(c.call(w.backend, false, "DELETE", "/api/v1/admin/service-accounts/"+aid, &w.adm, true, ""), 404)
	// Integration routes: a token without the scope (403) and a refused token (401).
	weak := w.mintServiceAccount("weak")
	for _, r := range Routes() {
		if r.Access != AccessService {
			continue
		}
		path := strings.ReplaceAll(r.Pattern, "{id}", fmt.Sprint(w.holdA))
		w.status(c.callBearer(w.backend, r.Method, path, weak.token, "{}"), 403)
		w.status(c.callBearer(w.backend, r.Method, path, tok, "{}"), 401)
	}
	// Integration routes with a token that has every scope: the two reports,
	// their refusals, and the state with holds, a report and a free setup.
	rep := w.mintServiceAccount("reporter", allScopes()...)
	for _, tc := range []struct {
		status       int
		method, path string
		body         string
	}{
		{200, "PUT", "/api/v1/integration/pressure", `{"value":0.4,"observed_at":"2026-11-19T09:00:00Z"}`},
		{200, "PUT", "/api/v1/integration/deployments", `{"observed_at":"2026-11-19T09:00:00Z","deployments":[{"setup":"` + setupNameOf(t, w.backend, w.setupA) + `","labels":{"cluster":"c1"}},{"setup":"no-such-setup"}]}`},
		{400, "PUT", "/api/v1/integration/pressure", `{"value":2}`},
		{400, "PUT", "/api/v1/integration/deployments", `{"deployments":[{"setup":"a"},{"setup":"a"}]}`},
		{409, "PUT", "/api/v1/integration/pressure", `{"value":0.4,"observed_at":"2026-11-19T08:00:00Z"}`},
		{409, "PUT", "/api/v1/integration/deployments", `{"observed_at":"2026-11-19T08:00:00Z","deployments":[]}`},
		{200, "GET", "/api/v1/integration/state", ""}, // holdA is pending here: release_requested_at is set
		{400, "POST", fmt.Sprintf("/api/v1/integration/holds/%d/release", w.holdA), `{"reason":"  "}`},
		{404, "POST", "/api/v1/integration/holds/99999/release", `{"reason":"gone"}`},
		{200, "POST", fmt.Sprintf("/api/v1/integration/holds/%d/release", w.holdA), `{"reason":"torn down by automation"}`},
		{409, "POST", fmt.Sprintf("/api/v1/integration/holds/%d/release", w.holdA), `{"reason":"again"}`},
	} {
		w.status(c.callBearer(w.backend, tc.method, tc.path, rep.token, tc.body), tc.status)
	}

	// The password change and the sign-out come last: they end sessions.
	lm := w.newLocalMustChange()
	w.status(c.call(w.backend, false, "GET", "/api/v1/me", &lm, false, ""), 403) // password_change_required
	w.status(c.call(w.backend, true, "POST", "/auth/local/password", &lm, true,
		`{"current_password":"`+testAdminPassword+`","new_password":"a brand new long password"}`), 200)
	out := w.session(w.adm.user)
	w.status(c.call(w.backend, true, "POST", "/auth/logout", &out, true, ""), 204)
	// 429: five wrong passwords (the limit is shared with the password change).
	for i := 0; i < 6; i++ {
		rec = c.call(w.backend, false, "POST", "/auth/local", nil, false, `{"username":"admin","password":"nope nope nope"}`)
	}
	w.status(rec, 429)

	// Coverage: every operation has a validated success and, when it documents
	// one, a validated error.
	for _, r := range Routes() {
		op := w.opByID(r.OperationID)
		if c.ok[r.OperationID] == 0 {
			t.Errorf("%s: no success response was validated", r.OperationID)
		}
		documentsError := false
		for status := range op.Responses.Map() {
			if strings.HasPrefix(status, "4") || strings.HasPrefix(status, "5") {
				documentsError = true
			}
		}
		switch {
		case !documentsError:
			t.Errorf("%s documents no error response", r.OperationID)
		case c.bad[r.OperationID] == 0 && errorNotProvokable[r.OperationID] == "":
			t.Errorf("%s: no error response was validated", r.OperationID)
		}
	}
	if dateTimesSeen < 50 {
		t.Errorf("only %d date-time values were checked; the world should produce many more", dateTimesSeen)
	}
}

// errorNotProvokable lists operations whose only documented error is the 500
// every route can answer, which a handler without I/O cannot produce.
var errorNotProvokable = map[string]string{
	"getAuthMethods": "documents only 500 internal and does no I/O that can fail",
}

func (w *world) opByID(id string) *openapi3.Operation {
	for _, item := range w.c.doc.Paths.Map() {
		for _, op := range item.Operations() {
			if op.OperationID == id {
				return op
			}
		}
	}
	w.t.Fatalf("no operation %q in the YAML", id)
	return nil
}

func (w *world) newLocalMustChange() sess {
	w.t.Helper()
	w.setMustChangeFlag(true)
	return w.session(w.admin)
}

func (w *world) setMustChangeFlag(v bool) {
	w.t.Helper()
	if err := w.st.SetPassword(context.Background(), w.admin.ID, w.admin.PasswordHash, v); err != nil {
		w.t.Fatal(err)
	}
}

func (w *world) newPending() sess { return w.newUser("pending-extra", store.StatusPending) }

// Every response of the world carries timestamps in the one format; the check
// is the format validator above. This test names the rule and proves it on a
// raw response of each family of routes (user, admin, auth).
func TestContract_EveryTimestampIsRFC3339UTCWithMilliseconds(t *testing.T) {
	w := newWorld(t)
	var found []string
	scan := func(path string, s *sess) {
		rec := w.get(path, s)
		w.status(rec, 200)
		var walk func(k string, v any)
		walk = func(k string, v any) {
			switch x := v.(type) {
			case map[string]any:
				for kk, vv := range x {
					walk(kk, vv)
				}
			case []any:
				for _, vv := range x {
					walk(k, vv)
				}
			case string:
				if strings.HasSuffix(k, "_at") || k == "at" {
					found = append(found, x)
					if !strictDateTime.MatchString(x) {
						t.Errorf("%s: %s = %q is not RFC 3339 UTC with milliseconds", path, k, x)
					}
				}
			}
		}
		var v any
		_ = json.Unmarshal(rec.Body.Bytes(), &v)
		walk("", v)
	}
	scan("/api/v1/me", &w.devA)
	scan("/api/v1/me/holds", &w.devA)
	scan("/api/v1/me/ledger", &w.devA)
	scan("/api/v1/me/usage", &w.devA)
	scan("/api/v1/pressure", &w.devA)
	scan("/api/v1/setups", &w.devA)
	scan("/api/v1/admin/users", &w.adm)
	scan("/api/v1/admin/collector", &w.adm)
	scan("/api/v1/admin/audit", &w.adm)
	sort.Strings(found)
	if len(found) < 30 {
		t.Errorf("only %d timestamps found across the routes", len(found))
	}
}

// setupNameOf is the name of a setup, read from the store.
func setupNameOf(t *testing.T, b *backend, id int64) string {
	t.Helper()
	list, err := b.st.ListSetups(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.ID == id {
			return s.Name
		}
	}
	t.Fatalf("no setup %d", id)
	return ""
}
