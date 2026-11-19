// SPDX-License-Identifier: MIT

package api

import (
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// adRoute is one admin route with a body that would be valid.
type adRoute struct{ method, path, body string }

var adminRoutes = []adRoute{
	{"GET", "/api/v1/admin/users", ""},
	{"PUT", "/api/v1/admin/users/1/roles", `{"roles":["developer"]}`},
	{"POST", "/api/v1/admin/users/1/reject", ""},
	{"POST", "/api/v1/admin/users/1/grants", `{"amount_tokens":1,"reason":"x"}`},
	{"POST", "/api/v1/admin/users/1/reset", `{"reason":"x"}`},
	{"POST", "/api/v1/admin/setups", `{"name":"x","labels":{},"details":{},"enabled":true,"resource_name_override":null}`},
	{"PUT", "/api/v1/admin/setups/1", `{"name":"x","labels":{},"details":{},"enabled":true,"resource_name_override":null}`},
	{"DELETE", "/api/v1/admin/setups/1", ""},
	{"GET", "/api/v1/admin/rules", ""},
	{"PUT", "/api/v1/admin/rules", `{"rules":[]}`},
	{"GET", "/api/v1/admin/settings", ""},
	{"PUT", "/api/v1/admin/settings", `{}`},
	{"GET", "/api/v1/admin/collector", ""},
	{"GET", "/api/v1/admin/audit", ""},
}

// TestAUTH4_AdminRoutesNeedAdmin covers the permissions table for every admin
// route: anonymous 401, unassigned or rejected 403 no_role, developer 403
// forbidden, a missing CSRF token 403 csrf, and no data in any refusal.
func TestAUTH4_AdminRoutesNeedAdmin(t *testing.T) {
	e := newAdEnv(t)
	pending := e.user("pend", store.StatusPending)
	rejected := e.user("rej", store.StatusRejected)
	unassigned := e.user("unas", store.StatusActive)
	dev := e.dev("dev")
	for _, rt := range adminRoutes {
		name := rt.method + " " + rt.path
		check := func(who string, s *adSess, csrf bool, status int, code string) {
			t.Helper()
			rec := e.do(rt.method, rt.path, s, csrf, rt.body)
			if f, _ := adErr(t, rec, status, code); f != "" {
				t.Errorf("%s as %s: refusal names a field %q", name, who, f)
			}
			for _, leak := range []string{"example.test", "balance", "setups", "entries"} {
				if strings.Contains(rec.Body.String(), leak) {
					t.Errorf("%s as %s: refusal leaks %q: %s", name, who, leak, rec.Body)
				}
			}
		}
		check("anonymous", nil, false, 401, "unauthenticated")
		check("pending", &pending, true, 403, "no_role")
		check("rejected", &rejected, true, 403, "no_role")
		check("unassigned", &unassigned, true, 403, "no_role")
		check("developer", &dev, true, 403, "forbidden")
		if rt.method != "GET" {
			check("admin without csrf", &e.adminS, false, 403, "csrf")
		}
	}
}

func TestADM1_ListUsersShapeOrderAndSummary(t *testing.T) {
	e := newAdEnv(t)
	zed := e.dev("zed")
	amy := e.dev("amy")
	bob := e.dev("bob") // will be overdrawn
	e.user("boss", store.StatusActive, store.RoleAdmin)
	// bob: -400 => balance -100 (300 allowance).
	if _, err := e.led.Grant(uBg, e.admin.ID, bob.user.ID, -400_000, "correction"); err != nil {
		t.Fatal(err)
	}
	// amy burns: one debit entry.
	if _, err := e.st.AppendLedger(uBg, store.LedgerEntry{UserID: amy.user.ID, Period: "2026-11", Type: "debit", AmountMT: -12_500}, "2026-11", e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	// zed holds a metering setup of weight 2 at 2x => 4 tokens/hour.
	sid := e.setup("lab-01", true, map[string]string{"tier": "big"})
	if err := e.st.ReplaceRules(uBg, "rate", []store.Rule{{Selector: "tier=big", Value: 2}}, nil, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	if err := e.st.PutSetting(uBg, "default_weight", "1", nil, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	h := e.hold(sid, zed.user.ID)
	if _, err := e.st.SetMeteringStarted(uBg, h.ID, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	if err := e.st.InsertPressureSample(uBg, store.PressureSample{PolledAt: e.clk.Now(), Source: "x", Raw: 0.8, Smoothed: 0.8, Multiplier: 2}); err != nil {
		t.Fatal(err)
	}
	pend1 := e.user("p1", store.StatusPending)
	e.clk.Advance(time.Hour)
	pend2 := e.user("p2", store.StatusPending)
	_ = e.user("rej", store.StatusRejected)

	body := uWantOK(t, e.get("/api/v1/admin/users"), 200)
	uWantKeys(t, "response", body, "users", "pending_count", "summary", "period", "period_ends_at", "default_allowance_tokens")
	if body["period"] != "2026-11" || body["period_ends_at"] != "2026-12-01T00:00:00.000Z" || uNum(t, body, "default_allowance_tokens") != 300 || uNum(t, body, "pending_count") != 2 {
		t.Errorf("top-level fields: %v", body)
	}
	sum := adObjOf(t, body["summary"])
	uWantKeys(t, "summary", sum, "user_count", "admin_count", "overdrawn_count", "debited_this_month_tokens", "granted_this_month_tokens", "grant_count_this_month")
	// active users: local admin, zed, amy, bob, boss, rej is rejected, 2 pending => 5 active.
	if uNum(t, sum, "user_count") != 5 || uNum(t, sum, "admin_count") != 2 || uNum(t, sum, "overdrawn_count") != 1 ||
		uNum(t, sum, "debited_this_month_tokens") != 12.5 || uNum(t, sum, "granted_this_month_tokens") != -400 || uNum(t, sum, "grant_count_this_month") != 1 {
		t.Errorf("summary = %v", sum)
	}
	users := adArr(t, body, "users")
	var names []string
	for _, u := range users {
		names = append(names, adObjOf(t, u)["name"].(string))
	}
	// Overdrawn first, then by name (case-insensitive).
	want := []string{"User bob", "admin", "User amy", "User boss", "User p1", "User p2", "User rej", "User zed"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("order = %v, want %v", names, want)
	}
	byName := map[string]ujobj{}
	for _, u := range users {
		m := adObjOf(t, u)
		uWantKeys(t, "user", m, "id", "username", "email", "name", "auth_source", "status", "roles", "is_local_admin", "balance_tokens",
			"overdrawn", "month_to_date_burn_tokens", "burn_per_hour_tokens", "active_holds", "first_sign_in_at", "last_login_at")
		byName[m["name"].(string)] = m
	}
	if b := byName["User bob"]; b["overdrawn"] != true || uNum(t, b, "balance_tokens") != -100 {
		t.Errorf("bob = %v", b)
	}
	if a := byName["User amy"]; uNum(t, a, "balance_tokens") != 287.5 || uNum(t, a, "month_to_date_burn_tokens") != 12.5 || a["overdrawn"] != false {
		t.Errorf("amy = %v", a)
	}
	z := byName["User zed"]
	if uNum(t, z, "burn_per_hour_tokens") != 4 {
		t.Errorf("zed burn = %v", z["burn_per_hour_tokens"])
	}
	ah := adArr(t, z, "active_holds")
	if len(ah) != 1 || adObjOf(t, ah[0])["setup_name"] != "lab-01" || uNum(t, adObjOf(t, ah[0]), "hold_id") != float64(h.ID) {
		t.Errorf("zed holds = %v", ah)
	}
	loc := byName["admin"]
	if loc["is_local_admin"] != true || loc["balance_tokens"] != nil || loc["overdrawn"] != false {
		t.Errorf("local admin = %v", loc)
	}
	if p := byName["User p1"]; p["balance_tokens"] != nil || p["status"] != "pending" || len(adArr(t, p, "roles")) != 0 {
		t.Errorf("pending = %v", p)
	}

	// status=pending: only pending users, oldest first; the totals ignore the filter.
	pb := uWantOK(t, e.get("/api/v1/admin/users?status=pending"), 200)
	pu := adArr(t, pb, "users")
	if len(pu) != 2 || uNum(t, adObjOf(t, pu[0]), "id") != float64(pend1.user.ID) || uNum(t, adObjOf(t, pu[1]), "id") != float64(pend2.user.ID) {
		t.Errorf("pending list = %v", pu)
	}
	if uNum(t, adObjOf(t, pb["summary"]), "user_count") != 5 || uNum(t, pb, "pending_count") != 2 {
		t.Errorf("filtered response changed the totals: %v", pb)
	}
	rb := uWantOK(t, e.get("/api/v1/admin/users?status=rejected"), 200)
	if len(adArr(t, rb, "users")) != 1 {
		t.Errorf("rejected list = %v", rb["users"])
	}
	if f, _ := adErr(t, e.get("/api/v1/admin/users?status=bogus"), 400, "validation"); f != "status" {
		t.Errorf("field = %q", f)
	}
}

func TestADM5_ListUsersPendingCountDrops(t *testing.T) {
	e := newAdEnv(t)
	p := e.user("p", store.StatusPending)
	if n := uNum(t, uWantOK(t, e.get("/api/v1/admin/users"), 200), "pending_count"); n != 1 {
		t.Fatalf("pending_count = %v", n)
	}
	uWantOK(t, e.send("POST", "/api/v1/admin/users/"+uItoa(p.user.ID)+"/reject", ""), 200)
	if n := uNum(t, uWantOK(t, e.get("/api/v1/admin/users"), 200), "pending_count"); n != 0 {
		t.Fatalf("pending_count after reject = %v", n)
	}
}

func TestADM5_SetRolesApprovesCreditsOnceAndEmitsOnce(t *testing.T) {
	e := newAdEnv(t)
	p := e.user("p", store.StatusPending)
	path := "/api/v1/admin/users/" + uItoa(p.user.ID) + "/roles"
	u := uWantOK(t, e.send("PUT", path, `{"roles":["developer"]}`), 200)
	if u["status"] != "active" || len(adArr(t, u, "roles")) != 1 || uNum(t, u, "balance_tokens") != 300 {
		t.Fatalf("approved user = %v", u)
	}
	e.flushEvents()
	if n := len(e.hook.EventsOfType(hourglass.EventUserApproved)); n != 1 {
		t.Fatalf("user.approved emitted %d times", n)
	}
	ev := e.hook.EventsOfType(hourglass.EventUserApproved)[0]
	if ev.User == nil || ev.User.ID != p.user.ID {
		t.Errorf("event user = %+v", ev.User)
	}
	// A second role does not credit again or re-emit.
	u = uWantOK(t, e.send("PUT", path, `{"roles":["developer","admin"]}`), 200)
	if uNum(t, u, "balance_tokens") != 300 {
		t.Errorf("balance after second role = %v", u["balance_tokens"])
	}
	// Emptying the set returns the user to unassigned: still active, not pending.
	u = uWantOK(t, e.send("PUT", path, `{"roles":[]}`), 200)
	if u["status"] != "active" || len(adArr(t, u, "roles")) != 0 {
		t.Errorf("unassigned user = %v", u)
	}
	if n := uNum(t, uWantOK(t, e.get("/api/v1/admin/users?status=pending"), 200), "pending_count"); n != 0 {
		t.Errorf("unassigned user is pending again")
	}
	// Re-assigning in the same period neither credits again nor emits again.
	u = uWantOK(t, e.send("PUT", path, `{"roles":["developer"]}`), 200)
	if uNum(t, u, "balance_tokens") != 300 {
		t.Errorf("balance after re-assign = %v", u["balance_tokens"])
	}
	e.flushEvents()
	if n := len(e.hook.EventsOfType(hourglass.EventUserApproved)); n != 1 {
		t.Errorf("user.approved emitted %d times in total", n)
	}
	n, err := e.st.CountLedger(uBg, p.user.ID, "2026-11", "allowance")
	if err != nil || n != 1 {
		t.Errorf("allowance entries = %d (%v)", n, err)
	}
	// Audit: four user.roles rows, newest first, with before and after.
	rows := e.audits("user.roles")
	if len(rows) != 4 {
		t.Fatalf("audit rows = %d", len(rows))
	}
	last := rows[3] // oldest = the approval
	if !strings.Contains(last.BeforeJSON, `"status":"pending"`) || !strings.Contains(last.AfterJSON, `"status":"active"`) ||
		!strings.Contains(last.AfterJSON, `"developer"`) || last.Target != "user:"+uItoa(p.user.ID) || *last.ActorID != e.admin.ID {
		t.Errorf("approval audit = %+v", last)
	}
}

func TestADM5_RejectedUserCanBeApprovedLater(t *testing.T) {
	e := newAdEnv(t)
	p := e.user("p", store.StatusPending)
	id := uItoa(p.user.ID)
	r := uWantOK(t, e.send("POST", "/api/v1/admin/users/"+id+"/reject", ""), 200)
	if r["status"] != "rejected" {
		t.Fatalf("rejected user = %v", r)
	}
	// Rejecting again, or rejecting an active user, is a conflict.
	adErr(t, e.send("POST", "/api/v1/admin/users/"+id+"/reject", ""), 409, "conflict")
	if n := len(adArr(t, uWantOK(t, e.get("/api/v1/admin/users?status=pending"), 200), "users")); n != 0 {
		t.Errorf("rejected user still pending")
	}
	u := uWantOK(t, e.send("PUT", "/api/v1/admin/users/"+id+"/roles", `{"roles":["developer"]}`), 200)
	if u["status"] != "active" || uNum(t, u, "balance_tokens") != 300 {
		t.Errorf("approved-after-reject = %v", u)
	}
	e.flushEvents()
	if n := len(e.hook.EventsOfType(hourglass.EventUserApproved)); n != 1 {
		t.Errorf("user.approved emitted %d times", n)
	}
	adErr(t, e.send("POST", "/api/v1/admin/users/"+id+"/reject", ""), 409, "conflict")
	if rows := e.audits("user.reject"); len(rows) != 1 || !strings.Contains(rows[0].AfterJSON, "rejected") {
		t.Errorf("reject audit = %+v", rows)
	}
}

func TestADM1_SetRolesRefusals(t *testing.T) {
	e := newAdEnv(t)
	holder := e.dev("holder")
	other := e.user("boss2", store.StatusActive, store.RoleAdmin)
	sid := e.setup("lab-01", true, nil)
	e.hold(sid, holder.user.ID)
	roles := func(id int64) string { return "/api/v1/admin/users/" + uItoa(id) + "/roles" }

	adErr(t, e.send("PUT", roles(holder.user.ID), `{"roles":[]}`), 409, "has_active_hold")
	adErr(t, e.send("PUT", roles(e.admin.ID), `{"roles":["developer"]}`), 409, "local_admin_protected")
	adErr(t, e.send("PUT", roles(9999), `{"roles":["developer"]}`), 404, "not_found")
	if f, _ := adErr(t, e.send("PUT", roles(holder.user.ID), `{"roles":["owner"]}`), 400, "validation"); f != "roles" {
		t.Errorf("field = %q", f)
	}
	adErr(t, e.send("PUT", roles(holder.user.ID), `{"roles":["developer","developer"]}`), 400, "validation")
	if f, _ := adErr(t, e.send("PUT", roles(holder.user.ID), `{}`), 400, "validation"); f != "roles" {
		t.Errorf("field = %q", f)
	}
	adErr(t, e.send("PUT", roles(holder.user.ID), `{"roles":["developer"],"extra":1}`), 400, "validation")
	adErr(t, e.send("PUT", "/api/v1/admin/users/abc/roles", `{"roles":[]}`), 400, "validation")

	// Self-demotion: a second admin cannot drop their own admin role.
	adErr(t, e.do("PUT", roles(other.user.ID), &other, true, `{"roles":["developer"]}`), 409, "self_demote")
	// The local admin, though, may demote another admin.
	u := uWantOK(t, e.send("PUT", roles(other.user.ID), `{"roles":["developer"]}`), 200)
	if len(adArr(t, u, "roles")) != 1 {
		t.Errorf("demoted = %v", u)
	}
	// Nothing was written by the refusals.
	if rows := e.audits("user.roles"); len(rows) != 1 {
		t.Errorf("audit rows after refusals = %d, want 1", len(rows))
	}
	if ids, _ := e.st.GetUserRoles(uBg, holder.user.ID); len(ids) != 1 {
		t.Errorf("holder roles changed: %v", ids)
	}
}

func TestADM1_RejectLocalAdminAndUnknown(t *testing.T) {
	e := newAdEnv(t)
	adErr(t, e.send("POST", "/api/v1/admin/users/"+uItoa(e.admin.ID)+"/reject", ""), 409, "local_admin_protected")
	adErr(t, e.send("POST", "/api/v1/admin/users/9999/reject", ""), 404, "not_found")
}

func TestLED4_GrantShapeAndRefusals(t *testing.T) {
	e := newAdEnv(t)
	d := e.dev("dev")
	nobody := e.user("nobody", store.StatusActive)
	path := func(id int64) string { return "/api/v1/admin/users/" + uItoa(id) + "/grants" }

	rec := e.send("POST", path(d.user.ID), `{"amount_tokens":100.5,"reason":"soak test week"}`)
	b := uWantOK(t, rec, 201)
	uWantKeys(t, "response", b, "entry", "user")
	entry := adObjOf(t, b["entry"])
	uWantKeys(t, "entry", entry, "id", "created_at", "period", "type", "amount_tokens", "balance_after_tokens", "multiplier", "setup_name", "hold_id", "actor_name", "reason")
	if entry["type"] != "grant" || uNum(t, entry, "amount_tokens") != 100.5 || uNum(t, entry, "balance_after_tokens") != 400.5 ||
		entry["period"] != "2026-11" || entry["reason"] != "soak test week" || entry["actor_name"] != "admin" || entry["multiplier"] != nil || entry["hold_id"] != nil {
		t.Errorf("entry = %v", entry)
	}
	if u := adObjOf(t, b["user"]); uNum(t, u, "balance_tokens") != 400.5 {
		t.Errorf("user = %v", u)
	}
	// Negative grants correct mistakes.
	b = uWantOK(t, e.send("POST", path(d.user.ID), `{"amount_tokens":-500,"reason":"fix"}`), 201)
	if u := adObjOf(t, b["user"]); uNum(t, u, "balance_tokens") != -99.5 || u["overdrawn"] != true {
		t.Errorf("user after negative grant = %v", u)
	}
	rows := e.audits("ledger.grant")
	if len(rows) != 2 || rows[0].Target != "user:"+uItoa(d.user.ID) || rows[0].BeforeJSON == "" || rows[0].AfterJSON == "" {
		t.Errorf("grant audit = %+v", rows)
	}

	if f, _ := adErr(t, e.send("POST", path(d.user.ID), `{"amount_tokens":0,"reason":"x"}`), 400, "validation"); f != "amount_tokens" {
		t.Errorf("zero amount field = %q", f)
	}
	if f, _ := adErr(t, e.send("POST", path(d.user.ID), `{"amount_tokens":5}`), 400, "validation"); f != "reason" {
		t.Errorf("missing reason field = %q", f)
	}
	if f, _ := adErr(t, e.send("POST", path(d.user.ID), `{"amount_tokens":5,"reason":"   "}`), 400, "validation"); f != "reason" {
		t.Errorf("blank reason field = %q", f)
	}
	if f, _ := adErr(t, e.send("POST", path(d.user.ID), `{"reason":"x"}`), 400, "validation"); f != "amount_tokens" {
		t.Errorf("missing amount field = %q", f)
	}
	adErr(t, e.send("POST", path(d.user.ID), `{"amount_tokens":1e15,"reason":"x"}`), 400, "validation")
	adErr(t, e.send("POST", path(nobody.user.ID), `{"amount_tokens":5,"reason":"x"}`), 409, "conflict")
	adErr(t, e.send("POST", path(e.admin.ID), `{"amount_tokens":5,"reason":"x"}`), 409, "local_admin_protected")
	adErr(t, e.send("POST", path(9999), `{"amount_tokens":5,"reason":"x"}`), 404, "not_found")
	if n := len(e.audits("ledger.grant")); n != 2 {
		t.Errorf("refusals wrote audit rows: %d", n)
	}
}

func TestLED5_ResetShapeAndRefusals(t *testing.T) {
	e := newAdEnv(t)
	d := e.dev("dev")
	if _, err := e.led.Grant(uBg, e.admin.ID, d.user.ID, -450_000, "debt"); err != nil {
		t.Fatal(err)
	}
	path := func(id int64) string { return "/api/v1/admin/users/" + uItoa(id) + "/reset" }
	b := uWantOK(t, e.send("POST", path(d.user.ID), `{"reason":"fresh start"}`), 201)
	entry := adObjOf(t, b["entry"])
	if entry["type"] != "reset" || uNum(t, entry, "amount_tokens") != 450 || uNum(t, entry, "balance_after_tokens") != 300 || entry["reason"] != "fresh start" {
		t.Errorf("entry = %v", entry)
	}
	if u := adObjOf(t, b["user"]); uNum(t, u, "balance_tokens") != 300 || u["overdrawn"] != false {
		t.Errorf("user = %v", u)
	}
	if rows := e.audits("ledger.reset"); len(rows) != 1 {
		t.Errorf("reset audit rows = %d", len(rows))
	}
	if f, _ := adErr(t, e.send("POST", path(d.user.ID), `{}`), 400, "validation"); f != "reason" {
		t.Errorf("field = %q", f)
	}
	adErr(t, e.send("POST", path(e.admin.ID), `{"reason":"x"}`), 409, "local_admin_protected")
	adErr(t, e.send("POST", path(9999), `{"reason":"x"}`), 404, "not_found")
	nobody := e.user("nobody", store.StatusActive)
	adErr(t, e.send("POST", path(nobody.user.ID), `{"reason":"x"}`), 409, "conflict")
}
