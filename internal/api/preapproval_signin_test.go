// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// Pre-approved SSO sign-in through the real router, auth service, store and
// ledger, with a small OIDC issuer (docs/design/ledger-labels-onboarding.md,
// D1; PRE-1 to PRE-4, PRE-7, PRE-8).

// preIssuer is a minimal OIDC issuer: discovery, JWKS and a token endpoint
// that signs an ID token with the nonce of the flow and the claims set by the
// test. It does not check the code or the PKCE verifier; internal/auth's own
// tests cover the flow.
type preIssuer struct {
	t   *testing.T
	srv *httptest.Server
	key *rsa.PrivateKey

	mu     sync.Mutex
	nonce  string
	claims map[string]any
}

func newPreIssuer(t *testing.T) *preIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &preIssuer{t: t, key: key}
	enc := base64.RawURLEncoding.EncodeToString
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		u := p.srv.URL
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": u, "authorization_endpoint": u + "/authorize",
			"token_endpoint": u + "/token", "jwks_uri": u + "/jwks", "response_types_supported": []string{"code"},
			"subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"}})
	})
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "RSA", "kid": "k1", "use": "sig",
			"alg": "RS256", "n": enc(key.PublicKey.N.Bytes()), "e": enc(big.NewInt(int64(key.PublicKey.E)).Bytes())}}})
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, _ *http.Request) {
		p.mu.Lock()
		now := time.Now()
		c := map[string]any{"iss": p.srv.URL, "aud": "cid", "nonce": p.nonce, "iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix()}
		for k, v := range p.claims {
			c[k] = v
		}
		p.mu.Unlock()
		h, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": "k1"})
		cb, _ := json.Marshal(c)
		in := enc(h) + "." + enc(cb)
		sum := sha256.Sum256([]byte(in))
		sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
		if err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at-not-real", "token_type": "Bearer",
			"expires_in": 300, "id_token": in + "." + enc(sig)})
	})
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	return p
}

// recEmitter records events; it stands in for the dispatcher in sign-in.
type recEmitter struct {
	mu  sync.Mutex
	evs []hourglass.Event
}

func (r *recEmitter) Emit(e hourglass.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evs = append(r.evs, e)
}

func (r *recEmitter) types() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []string{}
	for _, e := range r.evs {
		out = append(out, e.Type)
	}
	return out
}

func (r *recEmitter) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evs = nil
}

type preEnv struct {
	*backend
	iss *preIssuer
	ev  *recEmitter
	adm sess
}

func newPreEnv(t *testing.T, file ...PreapprovalEntry) *preEnv {
	t.Helper()
	iss := newPreIssuer(t)
	ev := &recEmitter{}
	b := newBackend(t, backendOpts{start: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC), oidcIssuer: iss.srv.URL,
		emitter: ev, preFile: file})
	return &preEnv{backend: b, iss: iss, ev: ev, adm: b.newUser("pre-admin", store.StatusActive, store.RoleAdmin)}
}

// claimsFor are the ID token claims of an SSO user with this subject and email.
func claimsFor(sub, email string) map[string]any {
	return map[string]any{"sub": sub, "email": email, "name": "User " + sub}
}

// signIn runs one SSO sign-in with the given ID token claims and returns the
// callback's redirect target and the session (nil when none was set).
func (e *preEnv) signIn(claims map[string]any) (string, *sess) {
	e.t.Helper()
	start := httptest.NewRecorder()
	e.h.ServeHTTP(start, httptest.NewRequest("GET", "http://app.test/auth/oidc/start", nil))
	if start.Code != http.StatusFound {
		e.t.Fatalf("start: %d %s", start.Code, start.Body)
	}
	loc, err := url.Parse(start.Header().Get("Location"))
	if err != nil {
		e.t.Fatal(err)
	}
	var flow *http.Cookie
	for _, c := range start.Result().Cookies() {
		if c.Name == "hourglass_oidc" {
			flow = c
		}
	}
	if flow == nil {
		e.t.Fatal("no flow cookie")
	}
	e.iss.mu.Lock()
	e.iss.nonce, e.iss.claims = loc.Query().Get("nonce"), claims
	e.iss.mu.Unlock()
	req := httptest.NewRequest("GET", "http://app.test/auth/callback?code=c1&state="+url.QueryEscape(loc.Query().Get("state")), nil)
	req.AddCookie(&http.Cookie{Name: flow.Name, Value: flow.Value})
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		e.t.Fatalf("callback: %d %s", rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookie && c.Value != "" {
			u, err := e.st.GetUserBySub(context.Background(), claims["sub"].(string))
			if err != nil {
				e.t.Fatal(err)
			}
			return rec.Header().Get("Location"), &sess{user: u, cookie: c.Value}
		}
	}
	return rec.Header().Get("Location"), nil
}

// mustSignIn signs in and requires success: back to "/" with a session.
func (e *preEnv) mustSignIn(claims map[string]any) sess {
	e.t.Helper()
	loc, s := e.signIn(claims)
	if loc != "/" || s == nil {
		e.t.Fatalf("sign-in: redirect %q, session %v", loc, s != nil)
	}
	return *s
}

// state is a user's stored status and roles, and their allowance entries this period.
func (e *preEnv) state(sub string) (store.User, []string, int) {
	e.t.Helper()
	ctx := context.Background()
	u, err := e.st.GetUserBySub(ctx, sub)
	if err != nil {
		e.t.Fatal(err)
	}
	roles, err := e.st.GetUserRoles(ctx, u.ID)
	if err != nil {
		e.t.Fatal(err)
	}
	n, err := e.st.CountLedger(ctx, u.ID, e.led.CurrentPeriod(), string(ledger.TypeAllowance))
	if err != nil {
		e.t.Fatal(err)
	}
	return u, roles, n
}

func (e *preEnv) wantPending(sub string) {
	e.t.Helper()
	u, roles, n := e.state(sub)
	if u.Status != store.StatusPending || len(roles) != 0 || n != 0 || u.BalanceMT != 0 {
		e.t.Fatalf("%s: status %s roles %v allowances %d balance %d, want pending with nothing", sub, u.Status, roles, n, u.BalanceMT)
	}
}

func (e *preEnv) wantActive(sub string, roles ...string) store.User {
	e.t.Helper()
	u, got, n := e.state(sub)
	if u.Status != store.StatusActive || strings.Join(got, ",") != strings.Join(roles, ",") || n != 1 || u.BalanceMT != 300_000 {
		e.t.Fatalf("%s: status %s roles %v allowances %d balance %d, want active with %v and the 300-token allowance",
			sub, u.Status, got, n, u.BalanceMT, roles)
	}
	return u
}

// add pre-approves emails with roles through the admin API.
func (e *preEnv) add(roles string, emails ...string) {
	e.t.Helper()
	b, _ := json.Marshal(emails)
	rec := e.serve("POST", "/api/v1/admin/preapprovals", &e.adm, true, `{"emails":`+string(b)+`,"roles":`+roles+`}`)
	if rec.Code != 200 {
		e.t.Fatalf("add: %d %s", rec.Code, rec.Body)
	}
}

// preAudits returns the user.preapprove audit entries, oldest first.
func (e *preEnv) preAudits() []store.AuditEntry {
	e.t.Helper()
	es, err := e.st.ListAudit(context.Background(), store.AuditFilter{Action: auditUserPreapprove})
	if err != nil {
		e.t.Fatal(err)
	}
	for i, j := 0, len(es)-1; i < j; i, j = i+1, j-1 {
		es[i], es[j] = es[j], es[i]
	}
	return es
}

func sessionView(t *testing.T, b *backend, s *sess) map[string]any {
	t.Helper()
	rec := b.serve("GET", "/api/v1/session", s, false, "")
	if rec.Code != 200 {
		t.Fatalf("session: %d %s", rec.Code, rec.Body)
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// PRE-1: a new SSO user whose email matches is, by the end of the first
// sign-in, active with the entry's roles and the period's allowance; the
// first session shows the roles and the user reaches a role route.
func TestPRE1_NewUserIsActiveWithRolesAndAllowanceAtFirstSignIn(t *testing.T) {
	e := newPreEnv(t)
	e.add(`["developer"]`, "alice@example.test")
	s := e.mustSignIn(claimsFor("sub-alice", "alice@example.test"))
	e.wantActive("sub-alice", store.RoleDeveloper)
	v := sessionView(t, e.backend, &s)
	if v["status"] != store.StatusActive || len(v["roles"].([]any)) != 1 || v["roles"].([]any)[0] != store.RoleDeveloper {
		t.Fatalf("first session %v, want active with [developer]", v)
	}
	rec := e.serve("GET", "/api/v1/me", &s, false, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"balance_tokens":300`) {
		t.Fatalf("GET /me: %d %s", rec.Code, rec.Body)
	}
}

// PRE-1: a user who signed in before the entry existed stays pending until
// the next sign-in, which approves them.
func TestPRE1_PendingUserFromAnEarlierSignInIsApprovedAtTheNextSignIn(t *testing.T) {
	e := newPreEnv(t)
	e.mustSignIn(claimsFor("sub-bob", "bob@example.test"))
	e.wantPending("sub-bob")
	e.add(`["developer"]`, "bob@example.test")
	e.wantPending("sub-bob") // adding an entry approves nobody at once
	s := e.mustSignIn(claimsFor("sub-bob", "bob@example.test"))
	e.wantActive("sub-bob", store.RoleDeveloper)
	if v := sessionView(t, e.backend, &s); v["status"] != store.StatusActive {
		t.Fatalf("session %v", v)
	}
}

// PRE-2: case and surrounding whitespace are ignored on both sides.
func TestPRE2_MatchIgnoresCaseAndSurroundingWhitespace(t *testing.T) {
	e := newPreEnv(t, PreapprovalEntry{Email: "file.user@example.test", Roles: []string{store.RoleDeveloper}})
	e.add(`["developer"]`, "  Carol.Smith@Example.TEST\t")
	e.mustSignIn(claimsFor("sub-carol", " CAROL.smith@example.test  "))
	e.wantActive("sub-carol", store.RoleDeveloper)
	e.mustSignIn(claimsFor("sub-file", "File.User@EXAMPLE.test "))
	e.wantActive("sub-file", store.RoleDeveloper)
}

// PRE-2: no match without a verified, non-empty email claim; an address in
// another claim never matches; a different address does not match.
func TestPRE2_NoMatchWithoutAVerifiedNonEmptyEmailClaim(t *testing.T) {
	e := newPreEnv(t, PreapprovalEntry{Email: "dana@example.test", Roles: []string{store.RoleDeveloper}})
	e.add(`["developer"]`, "dana@example.test")
	cases := []struct {
		name   string
		claims map[string]any
		match  bool
	}{
		{"verified false", map[string]any{"email": "dana@example.test", "email_verified": false}, false},
		{"verified string false", map[string]any{"email": "dana@example.test", "email_verified": "false"}, false},
		{"empty email", map[string]any{"email": "", "preferred_username": "dana@example.test"}, false},
		{"no email claim, address in other claims", map[string]any{"preferred_username": "dana@example.test",
			"upn": "dana@example.test", "name": "dana@example.test"}, false},
		{"other address", map[string]any{"email": "dana.other@example.test", "preferred_username": "dana@example.test"}, false},
		{"verified true", map[string]any{"email": "dana@example.test", "email_verified": true}, true},
		{"verified string true", map[string]any{"email": "dana@example.test", "email_verified": "true"}, true},
		{"verified absent", map[string]any{"email": "dana@example.test"}, true},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sub := "sub-dana-" + string(rune('a'+i))
			c.claims["sub"] = sub
			e.mustSignIn(c.claims)
			if c.match {
				e.wantActive(sub, store.RoleDeveloper)
			} else {
				e.wantPending(sub)
			}
		})
	}
}

// PRE-3: a rejected user, an active user with no roles and the local admin
// are never changed; deleting an entry changes no user.
func TestPRE3_RejectedActiveAndLocalAdminAreNeverChanged(t *testing.T) {
	e := newPreEnv(t, PreapprovalEntry{Email: "erin@example.test", Roles: []string{store.RoleAdmin}})
	e.add(`["developer"]`, "erin@example.test", "frank@example.test", "admin@example.test")
	ctx := context.Background()

	// Rejected by an admin after an earlier sign-in.
	e.mustSignIn(claimsFor("sub-erin", "erin@example.test"))
	e.wantActive("sub-erin", store.RoleAdmin, store.RoleDeveloper)
	rej, err := e.st.CreateOIDCUser(ctx, "sub-rejected", "erin@example.test", "Rejected", e.clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	if rec := e.serve("POST", "/api/v1/admin/users/"+itoa(rej.ID)+"/reject", &e.adm, true, ""); rec.Code != 200 {
		t.Fatalf("reject: %d %s", rec.Code, rec.Body)
	}
	e.mustSignIn(claimsFor("sub-rejected", "erin@example.test"))
	if u, roles, n := e.state("sub-rejected"); u.Status != store.StatusRejected || len(roles) != 0 || n != 0 {
		t.Fatalf("rejected user changed: %s %v %d", u.Status, roles, n)
	}

	// Active with no roles: an admin removed them; the next sign-in must not restore them.
	e.mustSignIn(claimsFor("sub-frank", "frank@example.test"))
	frank := e.wantActive("sub-frank", store.RoleDeveloper)
	if rec := e.serve("PUT", "/api/v1/admin/users/"+itoa(frank.ID)+"/roles", &e.adm, true, `{"roles":[]}`); rec.Code != 200 {
		t.Fatalf("remove roles: %d %s", rec.Code, rec.Body)
	}
	e.mustSignIn(claimsFor("sub-frank", "frank@example.test"))
	if u, roles, _ := e.state("sub-frank"); u.Status != store.StatusActive || len(roles) != 0 {
		t.Fatalf("active user with no roles changed: %s %v", u.Status, roles)
	}

	// The local admin: never matched, even called directly with a listed email.
	approved, err := e.pre.ApplyPreapproval(ctx, e.admin.ID, "admin@example.test")
	if err != nil || approved {
		t.Fatalf("local admin: approved %v err %v", approved, err)
	}
	if roles, _ := e.st.GetUserRoles(ctx, e.admin.ID); strings.Join(roles, ",") != store.RoleAdmin {
		t.Fatalf("local admin roles %v", roles)
	}

	// Deleting entries, from the database and (by restarting without it) from
	// the file, changes no user that was approved.
	list := e.serve("GET", "/api/v1/admin/preapprovals", &e.adm, false, "")
	var out struct {
		Preapprovals []struct {
			ID    *int64 `json:"id"`
			Email string `json:"email"`
		} `json:"preapprovals"`
	}
	_ = json.Unmarshal(list.Body.Bytes(), &out)
	for _, p := range out.Preapprovals {
		if p.ID != nil {
			if rec := e.serve("DELETE", "/api/v1/admin/preapprovals/"+itoa(*p.ID), &e.adm, true, ""); rec.Code != 204 {
				t.Fatalf("delete: %d %s", rec.Code, rec.Body)
			}
		}
	}
	e.wantActive("sub-erin", store.RoleAdmin, store.RoleDeveloper)
	if u, roles, _ := e.state("sub-frank"); u.Status != store.StatusActive || len(roles) != 0 {
		t.Fatalf("frank changed by delete: %s %v", u.Status, roles)
	}
	e.mustSignIn(claimsFor("sub-erin", "erin@example.test"))
	e.wantActive("sub-erin", store.RoleAdmin, store.RoleDeveloper)
	// A restart without the file entry (a step built without it) leaves the
	// approved user as they are.
	erin, _, _ := e.state("sub-erin")
	restarted := NewPreapprover(PreapproverDeps{Store: e.st, Ledger: e.led, Clock: e.clk})
	if ok, err := restarted.ApplyPreapproval(ctx, erin.ID, "erin@example.test"); ok || err != nil {
		t.Fatalf("approved again after the file entry went: %v %v", ok, err)
	}
	e.wantActive("sub-erin", store.RoleAdmin, store.RoleDeveloper)
}

// PRE-4: one user.preapprove entry with no actor, the shape of user.roles and
// the matched sources; user.approved, no user.pending; an unused database
// entry is marked with its first use. A second identity with the same email
// is approved too, with its own entry, and the first use stays.
func TestPRE4_AuditEventsAndFirstUse(t *testing.T) {
	e := newPreEnv(t)
	e.add(`["developer"]`, "gina@example.test")
	e.ev.reset()
	e.mustSignIn(claimsFor("sub-gina", "gina@example.test"))
	gina := e.wantActive("sub-gina", store.RoleDeveloper)
	if got := e.ev.types(); strings.Join(got, ",") != hourglass.EventUserApproved {
		t.Fatalf("events %v, want only user.approved", got)
	}
	if ev := e.ev.evs[0]; ev.User == nil || ev.User.ID != gina.ID || strings.Join(ev.User.Roles, ",") != store.RoleDeveloper ||
		ev.User.Email != "gina@example.test" {
		t.Fatalf("user.approved payload %+v", ev.User)
	}
	as := e.preAudits()
	if len(as) != 1 {
		t.Fatalf("%d user.preapprove entries, want 1", len(as))
	}
	a := as[0]
	if a.ActorID != nil || a.ActorService != "" || a.Target != "user:"+itoa(gina.ID) {
		t.Fatalf("audit actor %v %q target %q", a.ActorID, a.ActorService, a.Target)
	}
	if a.BeforeJSON != `{"roles":[],"status":"pending"}` ||
		a.AfterJSON != `{"roles":["developer"],"sources":["database"],"status":"active"}` {
		t.Fatalf("audit before %s after %s", a.BeforeJSON, a.AfterJSON)
	}
	// The audit log shows the system as the actor.
	rec := e.serve("GET", "/api/v1/admin/audit?action=user.preapprove", &e.adm, false, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"actor":{"id":0,"name":"system"}`) {
		t.Fatalf("audit list: %d %s", rec.Code, rec.Body)
	}
	p, err := e.st.GetPreapprovalByEmail(context.Background(), "gina@example.test")
	if err != nil || p.UsedAt == nil || p.UsedBy == nil || *p.UsedBy != gina.ID || !p.UsedAt.Equal(e.clk.Now().Truncate(time.Millisecond)) {
		t.Fatalf("entry %+v %v, want used by %d", p, err, gina.ID)
	}

	// A second SSO identity with the same email: approved too, audited, and
	// the entry keeps its first use.
	e.clk.Advance(time.Hour)
	e.ev.reset()
	e.mustSignIn(claimsFor("sub-gina-2", "gina@example.test"))
	gina2 := e.wantActive("sub-gina-2", store.RoleDeveloper)
	if got := e.ev.types(); strings.Join(got, ",") != hourglass.EventUserApproved {
		t.Fatalf("second identity events %v", got)
	}
	if as := e.preAudits(); len(as) != 2 || as[1].Target != "user:"+itoa(gina2.ID) {
		t.Fatalf("audit entries %+v", as)
	}
	p2, _ := e.st.GetPreapprovalByEmail(context.Background(), "gina@example.test")
	if *p2.UsedBy != gina.ID || !p2.UsedAt.Equal(*p.UsedAt) {
		t.Fatalf("first use overwritten: %+v", p2)
	}
	// Signing in again changes nothing and writes nothing.
	e.ev.reset()
	e.mustSignIn(claimsFor("sub-gina", "gina@example.test"))
	if len(e.ev.types()) != 0 || len(e.preAudits()) != 2 {
		t.Fatalf("an active user's sign-in emitted %v / audited", e.ev.types())
	}
}

// PRE-4 and Compatibility ("Sign-in"): with no entries and no file, a new SSO
// user is pending and gets user.pending, as before; no audit entry is written.
func TestPRE4_NoEntriesNoFileSignInAsBefore(t *testing.T) {
	e := newPreEnv(t)
	e.ev.reset()
	s := e.mustSignIn(claimsFor("sub-henry", "henry@example.test"))
	e.wantPending("sub-henry")
	if got := e.ev.types(); strings.Join(got, ",") != hourglass.EventUserPending {
		t.Fatalf("events %v, want only user.pending", got)
	}
	if len(e.preAudits()) != 0 {
		t.Fatal("audit entry without a pre-approval")
	}
	if v := sessionView(t, e.backend, &s); v["status"] != store.StatusPending || len(v["roles"].([]any)) != 0 {
		t.Fatalf("session %v", v)
	}
}

// PRE-7: the roles of an email in both sources are the union, and the audit
// entry names both sources; a file entry alone applies with source config;
// a pre-approval can grant admin.
func TestPRE7_UnionOfBothSources(t *testing.T) {
	e := newPreEnv(t,
		PreapprovalEntry{Email: "ivy@example.test", Roles: []string{store.RoleAdmin}},
		PreapprovalEntry{Email: "jack@example.test", Roles: []string{store.RoleDeveloper}})
	e.add(`["developer"]`, "ivy@example.test")
	s := e.mustSignIn(claimsFor("sub-ivy", "ivy@example.test"))
	e.wantActive("sub-ivy", store.RoleAdmin, store.RoleDeveloper)
	if as := e.preAudits(); len(as) != 1 || !strings.Contains(as[0].AfterJSON, `"sources":["database","config"]`) ||
		!strings.Contains(as[0].AfterJSON, `"roles":["admin","developer"]`) {
		t.Fatalf("audit %+v", as)
	}
	// The granted admin role works.
	if rec := e.serve("GET", "/api/v1/admin/users", &s, false, ""); rec.Code != 200 {
		t.Fatalf("pre-approved admin: %d %s", rec.Code, rec.Body)
	}

	e.mustSignIn(claimsFor("sub-jack", "jack@example.test"))
	e.wantActive("sub-jack", store.RoleDeveloper)
	if as := e.preAudits(); len(as) != 2 || !strings.Contains(as[1].AfterJSON, `"sources":["config"]`) {
		t.Fatalf("audit %+v", as)
	}
}

// PRE-8: when the approval transaction fails part-way, the sign-in succeeds,
// the user stays pending with no role, no allowance and an unused entry, the
// error is logged, and the new user gets user.pending.
func TestPRE8_FailedApprovalTransactionLeavesTheUserPending(t *testing.T) {
	e := newPreEnv(t)
	e.add(`["developer"]`, "kim@example.test")
	raw, err := sql.Open("sqlite", "file:"+backendDBPath(t)+"?_pragma=busy_timeout(10000)")
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	// The audit insert is the last write of the step: roles, status, the
	// allowance and the first-use mark are already written when it fails.
	if _, err := raw.Exec(`CREATE TRIGGER test_fail_preapprove BEFORE INSERT ON audit_log
		WHEN NEW.action = 'user.preapprove' BEGIN SELECT RAISE(ABORT, 'forced failure of the approval'); END`); err != nil {
		t.Fatal(err)
	}
	e.ev.reset()
	s := e.mustSignIn(claimsFor("sub-kim", "kim@example.test"))
	e.wantPending("sub-kim")
	if v := sessionView(t, e.backend, &s); v["status"] != store.StatusPending || len(v["roles"].([]any)) != 0 {
		t.Fatalf("session %v", v)
	}
	if p, _ := e.st.GetPreapprovalByEmail(context.Background(), "kim@example.test"); p.UsedAt != nil || p.UsedBy != nil {
		t.Fatalf("entry marked used by a failed approval: %+v", p)
	}
	if got := e.ev.types(); strings.Join(got, ",") != hourglass.EventUserPending {
		t.Fatalf("events %v, want only user.pending", got)
	}
	if logs := e.logs.String(); !strings.Contains(logs, "applying a pre-approval failed") || !strings.Contains(logs, "forced failure of the approval") {
		t.Fatalf("failure not logged: %s", logs)
	}
	// Once the fault is gone, the next sign-in approves.
	if _, err := raw.Exec(`DROP TRIGGER test_fail_preapprove`); err != nil {
		t.Fatal(err)
	}
	e.mustSignIn(claimsFor("sub-kim", "kim@example.test"))
	e.wantActive("sub-kim", store.RoleDeveloper)
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
