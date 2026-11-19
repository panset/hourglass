// SPDX-License-Identifier: MIT

package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

var userRoutes = []struct{ method, path string }{
	{"GET", "/api/v1/me"},
	{"GET", "/api/v1/me/holds"},
	{"GET", "/api/v1/me/ledger"},
	{"GET", "/api/v1/me/usage"},
	{"GET", "/api/v1/pressure"},
	{"GET", "/api/v1/setups"},
	{"POST", "/api/v1/setups/1/acquire"},
	{"POST", "/api/v1/holds/1/release"},
}

func TestAUTH4_UserRoutesNeedSignInAndARole(t *testing.T) {
	e := defaultEnv(t)
	e.setup("lab-01", true, map[string]string{"tier": "a"})
	unassigned := e.pending("nobody")
	for _, r := range userRoutes {
		rec := e.do(r.method, r.path, nil, false, "")
		uWantErr(t, rec, http.StatusUnauthorized, "unauthenticated")
		rec = e.do(r.method, r.path, &unassigned, true, "")
		uWantErr(t, rec, http.StatusForbidden, "no_role")
		if strings.Contains(rec.Body.String(), "lab-01") {
			t.Errorf("%s %s leaked data to a user with no role: %s", r.method, r.path, rec.Body)
		}
	}
}

func TestUI6_NoUserRouteIsNotImplemented(t *testing.T) {
	e := defaultEnv(t)
	dev := e.dev("dev", store.RoleDeveloper)
	for _, r := range userRoutes {
		if rec := e.do(r.method, r.path, &dev, true, ""); rec.Code == http.StatusNotImplemented {
			t.Errorf("%s %s answers 501", r.method, r.path)
		}
	}
}

// meteringScenario: dev holds a heavy setup (weight 600 at 2x = 1200 tokens/hour)
// that the provider reports, with three polls so two debits of 100 tokens exist.
func meteringScenario(t *testing.T) (*uenv, usess, int64, int64) {
	e := defaultEnv(t)
	e.rateRule("tier=a", 600)
	sid := e.setup("lab-01", true, map[string]string{"tier": "a"})
	dev := e.dev("dev", store.RoleDeveloper)
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-01"})
	hid := e.acquire(&dev, sid)
	e.poll()
	e.step(5 * time.Minute)
	e.step(5 * time.Minute)
	return e, dev, sid, hid
}

func TestLED2_MeShape(t *testing.T) {
	e, dev, _, hid := meteringScenario(t)
	m := uWantOK(t, e.get("/api/v1/me", &dev), 200)
	uWantKeys(t, "me", m, "period", "period_ends_at", "balance_tokens", "allowance_tokens", "granted_tokens", "carryover_tokens",
		"debited_tokens", "burn_per_hour_tokens", "burning", "projection", "projected_runout_at", "hold_count", "hold_limit", "enforcement", "overdrawn", "can_acquire", "cannot_acquire_reason")
	if m["period"] != "2026-11" || m["period_ends_at"] != "2026-12-01T00:00:00.000Z" {
		t.Errorf("period %v ends %v", m["period"], m["period_ends_at"])
	}
	// 300 allowance, two debits of 600 x 2 x 5/60 = 100 tokens each.
	if !uNear(uNum(t, m, "balance_tokens"), 100) || !uNear(uNum(t, m, "debited_tokens"), 200) {
		t.Errorf("balance %v debited %v", m["balance_tokens"], m["debited_tokens"])
	}
	if uNum(t, m, "allowance_tokens") != 300 || uNum(t, m, "granted_tokens") != 0 || uNum(t, m, "carryover_tokens") != 0 {
		t.Errorf("allowance/granted/carryover %v %v %v", m["allowance_tokens"], m["granted_tokens"], m["carryover_tokens"])
	}
	if !uNear(uNum(t, m, "burn_per_hour_tokens"), 1200) {
		t.Errorf("burn %v", m["burn_per_hour_tokens"])
	}
	b := m["burning"].([]any)
	if len(b) != 1 {
		t.Fatalf("burning %v", b)
	}
	bm := b[0].(ujobj)
	uWantKeys(t, "burning", bm, "hold_id", "setup_id", "setup_name", "weight", "multiplier", "tokens_per_hour")
	if bm["setup_name"] != "lab-01" || uNum(t, bm, "weight") != 600 || uNum(t, bm, "multiplier") != 2 || int64(uNum(t, bm, "hold_id")) != hid {
		t.Errorf("burning %v", bm)
	}
	if m["hold_count"].(float64) != 1 || m["hold_limit"].(float64) != 2 || m["enforcement"] != false || m["overdrawn"] != false {
		t.Errorf("counts %v %v %v %v", m["hold_count"], m["hold_limit"], m["enforcement"], m["overdrawn"])
	}
	// 100 tokens left, 200 tokens burnt in 24 h: 12 hours from now (10:10).
	if m["projection"] != "date" || m["projected_runout_at"] != "2026-11-19T22:10:00.000Z" {
		t.Errorf("projection %v at %v", m["projection"], m["projected_runout_at"])
	}
}

func TestLED2_MeProjectionNotThisMonthAndNotBurning(t *testing.T) {
	e := defaultEnv(t)
	sid := e.setup("lab-01", true, nil)
	dev, idle := e.dev("dev", store.RoleDeveloper), e.dev("idle", store.RoleDeveloper)
	e.acquire(&dev, sid)
	e.poll()
	e.step(5 * time.Minute)
	e.step(5 * time.Minute)
	m := uWantOK(t, e.get("/api/v1/me", &dev), 200)
	// ~0.33 tokens in 24 h against ~300: years away.
	if m["projection"] != "not_this_month" || m["projected_runout_at"] != nil {
		t.Errorf("projection %v at %v", m["projection"], m["projected_runout_at"])
	}
	m = uWantOK(t, e.get("/api/v1/me", &idle), 200)
	if m["projection"] != "not_burning" || m["projected_runout_at"] != nil || m["burning"] == nil ||
		len(m["burning"].([]any)) != 0 || uNum(t, m, "burn_per_hour_tokens") != 0 {
		t.Errorf("idle user: %v", m)
	}
}

func TestLED1_MeLocalAdminHasNoBalance(t *testing.T) {
	e := defaultEnv(t)
	admin := e.adminSess()
	m := uWantOK(t, e.get("/api/v1/me", &admin), 200)
	if m["balance_tokens"] != nil || m["hold_limit"].(float64) != 0 || m["hold_count"].(float64) != 0 || m["overdrawn"] != false ||
		m["projection"] != "not_burning" || m["projected_runout_at"] != nil {
		t.Errorf("local admin me: %v", m)
	}
}

func TestMET6_MeOverdrawn(t *testing.T) {
	e := defaultEnv(t)
	dev := e.dev("dev", store.RoleDeveloper)
	e.setting("enforcement", "true")
	if _, err := e.st.AppendLedger(uBg, store.LedgerEntry{UserID: dev.user.ID, Period: "2026-11", Type: "grant", AmountMT: -300000, Reason: "x"}, "2026-11", e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	m := uWantOK(t, e.get("/api/v1/me", &dev), 200)
	if m["overdrawn"] != true || m["enforcement"] != true || uNum(t, m, "balance_tokens") != 0 || uNum(t, m, "granted_tokens") != -300 {
		t.Errorf("me %v", m)
	}
}

func TestHOLD3_MyHoldsShapeAndStateFilter(t *testing.T) {
	e, dev, sid, hid := meteringScenario(t)
	other := e.dev("other", store.RoleDeveloper)
	s2 := e.setup("lab-02", true, map[string]string{"tier": "b"})
	e.acquire(&other, s2) // someone else's hold must not appear

	hs := uWantOK(t, e.get("/api/v1/me/holds", &dev), 200)["holds"].([]any)
	if len(hs) != 1 {
		t.Fatalf("holds %v", hs)
	}
	h := hs[0].(ujobj)
	uWantKeys(t, "hold", h, "id", "setup_id", "setup_name", "setup_labels", "user_id", "active", "weight", "acquired_at",
		"metering_started_at", "deployed_at", "released_at", "deployed", "metering", "cost_per_hour_tokens", "charged_tokens",
		"released_by", "release_reason", "forced")
	if int64(uNum(t, h, "id")) != hid || int64(uNum(t, h, "setup_id")) != sid || h["setup_name"] != "lab-01" ||
		h["setup_labels"].(ujobj)["tier"] != "a" || h["active"] != true || uNum(t, h, "weight") != 600 {
		t.Errorf("hold %v", h)
	}
	if h["acquired_at"] != "2026-11-19T10:00:00.000Z" || h["released_at"] != nil || h["released_by"] != nil || h["release_reason"] != nil || h["forced"] != false {
		t.Errorf("hold times/release %v", h)
	}
	if h["deployed"] != true || h["deployed_at"] == nil || h["metering_started_at"] == nil {
		t.Errorf("deployed %v / %v / %v", h["deployed"], h["deployed_at"], h["metering_started_at"])
	}
	met := h["metering"].(ujobj)
	uWantKeys(t, "metering", met, "active", "started_at", "expected_start_at", "reason")
	if met["active"] != true || met["expected_start_at"] != nil || met["reason"] != "test reason" || met["started_at"] == nil {
		t.Errorf("metering %v", met)
	}
	cost := h["cost_per_hour_tokens"].(ujobj)
	uWantKeys(t, "cost", cost, "now", "at_peak")
	if !uNear(uNum(t, cost, "now"), 1200) || !uNear(uNum(t, cost, "at_peak"), 1800) {
		t.Errorf("cost %v", cost)
	}
	if !uNear(uNum(t, h, "charged_tokens"), 200) {
		t.Errorf("charged %v", h["charged_tokens"])
	}

	// Release it, then it is past: no cost, released_by set.
	rel := e.post("/api/v1/holds/"+uItoa(hid)+"/release", &dev, "")
	uWantErr(t, rel, 409, "still_deployed")
	e.prov.SetAllocations()
	e.step(5 * time.Minute)
	rh := uWantOK(t, e.post("/api/v1/holds/"+uItoa(hid)+"/release", &dev, `{"force":false}`), 200)
	if rh["active"] != false || rh["cost_per_hour_tokens"] != nil || rh["released_at"] == nil || rh["forced"] != false ||
		rh["released_by"].(ujobj)["name"] != "User dev" || rh["deployed"] != false {
		t.Errorf("released hold %v", rh)
	}
	for state, want := range map[string]int{"": 1, "active": 0, "released": 1} {
		q := ""
		if state != "" {
			q = "?state=" + state
		}
		got := uWantOK(t, e.get("/api/v1/me/holds"+q, &dev), 200)["holds"].([]any)
		if len(got) != want {
			t.Errorf("state=%q: %d holds, want %d", state, len(got), want)
		}
	}
	uWantErr(t, e.get("/api/v1/me/holds?state=bogus", &dev), 400, "validation")
}

func TestLED2_MyLedger(t *testing.T) {
	e, dev, _, hid := meteringScenario(t)
	// An October entry for the Month select.
	if _, err := e.st.AppendLedger(uBg, store.LedgerEntry{UserID: dev.user.ID, Period: "2026-10", Type: "allowance", AmountMT: 300000},
		"2026-11", e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	l := uWantOK(t, e.get("/api/v1/me/ledger", &dev), 200)
	uWantKeys(t, "ledger", l, "period", "available_periods", "entries")
	if l["period"] != "2026-11" {
		t.Errorf("period %v", l["period"])
	}
	ap := l["available_periods"].([]any)
	if len(ap) != 2 || ap[0] != "2026-11" || ap[1] != "2026-10" {
		t.Errorf("available periods %v", ap)
	}
	es := l["entries"].([]any)
	if len(es) != 3 {
		t.Fatalf("entries %v", es)
	}
	// Newest first: debit, debit, allowance. Running balance 100, 200, 300.
	first, last := es[0].(ujobj), es[2].(ujobj)
	uWantKeys(t, "entry", first, "id", "created_at", "period", "type", "amount_tokens", "balance_after_tokens", "multiplier",
		"setup_name", "hold_id", "actor_name", "reason")
	if first["type"] != "debit" || !uNear(uNum(t, first, "amount_tokens"), -100) || !uNear(uNum(t, first, "balance_after_tokens"), 100) ||
		uNum(t, first, "multiplier") != 2 || first["setup_name"] != "lab-01" || int64(uNum(t, first, "hold_id")) != hid ||
		first["actor_name"] != nil || first["created_at"] != "2026-11-19T10:10:00.000Z" {
		t.Errorf("newest entry %v", first)
	}
	if last["type"] != "allowance" || uNum(t, last, "balance_after_tokens") != 300 || last["multiplier"] != nil || last["hold_id"] != nil || last["setup_name"] != nil {
		t.Errorf("oldest entry %v", last)
	}
	// Past period, and an admin grant names its actor.
	past := uWantOK(t, e.get("/api/v1/me/ledger?period=2026-10", &dev), 200)
	if pe := past["entries"].([]any); past["period"] != "2026-10" || len(pe) != 1 {
		t.Errorf("past %v", past)
	}
	if _, err := e.led.Grant(uBg, e.admin.ID, dev.user.ID, 50000, "bonus"); err != nil {
		t.Fatal(err)
	}
	g := uWantOK(t, e.get("/api/v1/me/ledger", &dev), 200)["entries"].([]any)[0].(ujobj)
	if g["type"] != "grant" || g["actor_name"] != "admin" || g["reason"] != "bonus" || uNum(t, g, "amount_tokens") != 50 {
		t.Errorf("grant entry %v", g)
	}
	for _, bad := range []string{"2026-13", "2026-1", "abcd-ef", "2026-11-01", "%20"} {
		rec := e.get("/api/v1/me/ledger?period="+bad, &dev)
		uWantErr(t, rec, 400, "validation")
		if _, f := uErrOf(t, rec); f != "period" {
			t.Errorf("period %q: field %q", bad, f)
		}
	}
}

func TestLED2_MyLedgerOnlyShowsOwnEntries(t *testing.T) {
	e, dev, _, _ := meteringScenario(t)
	other := e.dev("other", store.RoleDeveloper)
	es := uWantOK(t, e.get("/api/v1/me/ledger", &other), 200)["entries"].([]any)
	if len(es) != 1 || es[0].(ujobj)["type"] != "allowance" {
		t.Errorf("other user's ledger %v", es)
	}
	_ = dev
}

func TestUI1_MyUsageZeroFilledPerDay(t *testing.T) {
	e, dev, _, _ := meteringScenario(t)
	u := uWantOK(t, e.get("/api/v1/me/usage", &dev), 200)
	uWantKeys(t, "usage", u, "period", "available_periods", "total_debited_tokens", "days")
	days := u["days"].([]any)
	if len(days) != 19 { // 1st to today, the 19th
		t.Fatalf("%d days", len(days))
	}
	if d := days[0].(ujobj); d["date"] != "2026-11-01" || uNum(t, d, "debited_tokens") != 0 {
		t.Errorf("first day %v", d)
	}
	uWantKeys(t, "day", days[18].(ujobj), "date", "debited_tokens")
	if d := days[18].(ujobj); d["date"] != "2026-11-19" || !uNear(uNum(t, d, "debited_tokens"), 200) {
		t.Errorf("today %v", d)
	}
	if !uNear(uNum(t, u, "total_debited_tokens"), 200) {
		t.Errorf("total %v", u["total_debited_tokens"])
	}
	// A past month lists every day; a future one none.
	past := uWantOK(t, e.get("/api/v1/me/usage?period=2026-10", &dev), 200)
	if pd := past["days"].([]any); len(pd) != 31 || uNum(t, past, "total_debited_tokens") != 0 {
		t.Errorf("past usage: %d days, total %v", len(pd), past["total_debited_tokens"])
	}
	if fd := uWantOK(t, e.get("/api/v1/me/usage?period=2027-01", &dev), 200)["days"].([]any); len(fd) != 0 {
		t.Errorf("future usage: %v", fd)
	}
	uWantErr(t, e.get("/api/v1/me/usage?period=11-2026", &dev), 400, "validation")
}

func TestUI1_MyUsageDaysFollowConfiguredTimezone(t *testing.T) {
	// 03:00 UTC on the 19th is 19:00 on the 18th at UTC-8.
	loc := time.FixedZone("UTC-8", -8*3600)
	e := newUserEnv(t, time.Date(2026, 11, 19, 3, 0, 0, 0, time.UTC), loc)
	e.rateRule("tier=a", 600)
	sid := e.setup("lab-01", true, map[string]string{"tier": "a"})
	dev := e.dev("dev", store.RoleDeveloper)
	e.acquire(&dev, sid)
	e.poll()
	e.step(5 * time.Minute)
	u := uWantOK(t, e.get("/api/v1/me/usage", &dev), 200)
	days := u["days"].([]any)
	if len(days) != 18 || days[17].(ujobj)["date"] != "2026-11-18" || !uNear(uNum(t, days[17].(ujobj), "debited_tokens"), 100) {
		t.Errorf("days %v", days)
	}
}

func TestMET2_PressureBeforeAndAfterPolls(t *testing.T) {
	e := defaultEnv(t)
	dev := e.dev("dev", store.RoleDeveloper)
	e.setup("lab-01", true, nil)
	e.setup("lab-02", true, nil)
	e.setup("lab-off", false, nil)
	p := uWantOK(t, e.get("/api/v1/pressure", &dev), 200)
	uWantKeys(t, "pressure", p, "raw", "smoothed", "multiplier", "bands", "source", "updated_at", "setups_enabled", "setups_deployed", "history")
	if p["updated_at"] != nil || p["raw"].(float64) != 0 || uNum(t, p, "multiplier") != 0 || len(p["history"].([]any)) != 0 || p["source"] != "fake-pressure" {
		t.Errorf("before the first poll: %v", p)
	}
	b := p["bands"].(ujobj)
	uWantKeys(t, "bands", b, "base", "steps")
	if steps := b["steps"].([]any); len(steps) != 3 || steps[1].(ujobj)["from"].(float64) != 0.75 || steps[1].(ujobj)["multiplier"].(float64) != 2 {
		t.Errorf("bands %v", b)
	}

	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-01"}, hourglass.Allocation{ResourceName: "lab-off"})
	e.pressure.SetValue(0.6)
	e.poll()
	e.pressure.SetValue(0.8)
	e.step(5 * time.Minute)
	p = uWantOK(t, e.get("/api/v1/pressure", &dev), 200)
	// Smoothed (0.6 + 0.8) / 2 = 0.7 is in the 1x band although raw is in 2x.
	if p["raw"].(float64) != 0.8 || uNum(t, p, "multiplier") != 1 || p["updated_at"] != "2026-11-19T10:05:00.000Z" {
		t.Errorf("after polls: %v", p)
	}
	if s := p["smoothed"].(float64); s <= 0.6 || s >= 0.8 {
		t.Errorf("smoothed %v not between the raw values", s)
	}
	if p["setups_enabled"].(float64) != 2 || p["setups_deployed"].(float64) != 1 {
		t.Errorf("counts %v %v", p["setups_enabled"], p["setups_deployed"])
	}
	h := p["history"].([]any)
	if len(h) != 2 || h[0].(ujobj)["at"] != "2026-11-19T10:00:00.000Z" || h[1].(ujobj)["multiplier"].(float64) != 1 {
		t.Errorf("history %v", h)
	}
	uWantKeys(t, "history point", h[0].(ujobj), "at", "raw", "smoothed", "multiplier")

	// Samples older than 7 days are not reported.
	e.clk.Advance(8 * 24 * time.Hour)
	if h := uWantOK(t, e.get("/api/v1/pressure", &dev), 200)["history"].([]any); len(h) != 0 {
		t.Errorf("history older than 7 days: %v", h)
	}
}

func TestSET6_SetupsListForDeveloper(t *testing.T) {
	e := defaultEnv(t)
	e.rateRule("tier=a", 2)
	e.setting("display_label_keys", `["tier"]`)
	a := e.setup("lab-a", true, map[string]string{"tier": "a", "site": "x"})
	e.setup("lab-b", true, map[string]string{"tier": "b"})
	e.setup("lab-off", false, map[string]string{"tier": "a"})
	dev, peer := e.dev("dev", store.RoleDeveloper), e.dev("peer", store.RoleDeveloper)
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-a"})
	e.acquire(&peer, a)
	e.poll()

	l := uWantOK(t, e.get("/api/v1/setups", &dev), 200)
	uWantKeys(t, "setups response", l, "setups", "display_label_keys", "summary", "metering_description")
	if k := l["display_label_keys"].([]any); len(k) != 1 || k[0] != "tier" {
		t.Errorf("display_label_keys %v", k)
	}
	if s := l["summary"].(ujobj); s["enabled"].(float64) != 2 || s["deployed"].(float64) != 1 || s["free"].(float64) != 1 {
		t.Errorf("summary %v", s)
	}
	if l["metering_description"] != nil { // the policy interface has no describe method
		t.Errorf("metering_description %v", l["metering_description"])
	}
	ss := l["setups"].([]any)
	if len(ss) != 2 { // the disabled one is hidden from developers
		t.Fatalf("setups %v", ss)
	}
	sa := ss[0].(ujobj)
	uWantKeys(t, "setup", sa, "id", "name", "labels", "weight", "state", "holder", "held_since", "deployed", "metering", "cost_per_hour_tokens", "details", "admin")
	if sa["name"] != "lab-a" || sa["state"] != "held" || uNum(t, sa, "weight") != 2 || sa["deployed"] != true || sa["admin"] != nil ||
		sa["held_since"] != "2026-11-19T10:00:00.000Z" || sa["labels"].(ujobj)["site"] != "x" {
		t.Errorf("held setup %v", sa)
	}
	h := sa["holder"].(ujobj)
	uWantKeys(t, "holder", h, "user_id", "hold_id", "name", "balance_tokens", "overdrawn", "is_you")
	if h["name"] != "User peer" || uNum(t, h, "balance_tokens") != 300 || h["overdrawn"] != false || h["is_you"] != false {
		t.Errorf("holder %v (balance of other users is visible by design)", h)
	}
	if m := sa["metering"].(ujobj); m["active"] != true && m["active"] != false {
		t.Errorf("metering %v", m)
	}
	if c := sa["cost_per_hour_tokens"].(ujobj); !uNear(uNum(t, c, "now"), 4) || !uNear(uNum(t, c, "at_peak"), 6) {
		t.Errorf("cost %v", c) // weight 2 at 2x (poll multiplier) and at the 3x peak
	}
	if d := sa["details"].(ujobj); d["note"] != "opaque" {
		t.Errorf("details %v", d)
	}
	sb := ss[1].(ujobj)
	if sb["state"] != "free" || sb["holder"] != nil || sb["held_since"] != nil || sb["metering"] != nil || uNum(t, sb, "weight") != 1 || sb["deployed"] != false {
		t.Errorf("free setup %v", sb)
	}
	// Holder is_you for the holder's own view.
	mine := uWantOK(t, e.get("/api/v1/setups", &peer), 200)["setups"].([]any)[0].(ujobj)
	if mine["holder"].(ujobj)["is_you"] != true {
		t.Errorf("is_you %v", mine["holder"])
	}
}

func TestSET6_SetupsListForAdminHasAdminFieldsAndDisabled(t *testing.T) {
	e := defaultEnv(t)
	e.rateRule("tier=a", 2)
	e.setting("resource_name_template", `"{{.SetupName}}_RP"`)
	e.setup("lab-a", true, map[string]string{"tier": "a"})
	if _, err := e.st.CreateSetup(uBg, store.Setup{Name: "lab-off", Enabled: false, ResourceNameOverride: "custom-name", Labels: map[string]string{"tier": "z"}, DetailsJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	adm := e.dev("boss", store.RoleAdmin)
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-a_RP"})
	e.poll()
	ss := uWantOK(t, e.get("/api/v1/setups", &adm), 200)["setups"].([]any)
	if len(ss) != 2 {
		t.Fatalf("setups %v", ss)
	}
	a := ss[0].(ujobj)["admin"].(ujobj)
	uWantKeys(t, "admin", a, "enabled", "resource_name", "resource_name_override", "last_seen_at", "matched_rule_selector")
	if a["enabled"] != true || a["resource_name"] != "lab-a_RP" || a["resource_name_override"] != nil ||
		a["last_seen_at"] != "2026-11-19T10:00:00.000Z" || a["matched_rule_selector"] != "tier=a" {
		t.Errorf("admin fields %v", a)
	}
	off := ss[1].(ujobj)
	oa := off["admin"].(ujobj)
	if off["state"] != "disabled" || oa["enabled"] != false || oa["resource_name"] != "custom-name" || oa["resource_name_override"] != "custom-name" ||
		oa["last_seen_at"] != nil || oa["matched_rule_selector"] != nil {
		t.Errorf("disabled setup %v", off)
	}
	// Summary counts enabled setups only.
	if s := uWantOK(t, e.get("/api/v1/setups", &adm), 200)["summary"].(ujobj); s["enabled"].(float64) != 1 {
		t.Errorf("summary %v", s)
	}
}

func TestRULE2_SetupsSelectorFilter(t *testing.T) {
	e := defaultEnv(t)
	e.setup("lab-a", true, map[string]string{"tier": "a", "site": "x"})
	e.setup("lab-b", true, map[string]string{"tier": "b", "site": "y"})
	e.setup("lab-c", true, map[string]string{"tier": "b", "site": "x"})
	dev := e.dev("dev", store.RoleDeveloper)
	names := func(sel string) []string {
		rec := e.get("/api/v1/setups?selector="+url.QueryEscape(sel), &dev)
		var out []string
		for _, s := range uWantOK(t, rec, 200)["setups"].([]any) {
			out = append(out, s.(ujobj)["name"].(string))
		}
		return out
	}
	for sel, want := range map[string]string{
		"": "lab-a,lab-b,lab-c", "tier=b": "lab-b,lab-c", "tier=b,site=x": "lab-c", "site in (x,y)": "lab-a,lab-b,lab-c",
		"tier notin (b)": "lab-a", "nothing=here": "",
	} {
		if got := strings.Join(names(sel), ","); got != want {
			t.Errorf("selector %q: %s, want %s", sel, got, want)
		}
	}
	// The summary ignores the selector.
	rec := e.get("/api/v1/setups?selector=tier%3Da", &dev)
	if s := uWantOK(t, rec, 200)["summary"].(ujobj); s["enabled"].(float64) != 3 {
		t.Errorf("summary %v", s)
	}
	for _, bad := range []string{"=a", "(", "a b c", "tier in x"} {
		rec := e.get("/api/v1/setups?selector="+url.QueryEscape(bad), &dev)
		uWantErr(t, rec, 400, "invalid_selector")
		var b struct{ Error struct{ Message string } }
		_ = jsonUnmarshal(rec.Body.Bytes(), &b)
		if b.Error.Message == "" {
			t.Errorf("selector %q: no parse error in the body", bad)
		}
	}
}

func TestHOLD1_AcquireSuccess(t *testing.T) {
	e := defaultEnv(t)
	sid := e.setup("lab-a", true, map[string]string{"tier": "a"})
	dev := e.dev("dev", store.RoleDeveloper)
	rec := e.post("/api/v1/setups/"+uItoa(sid)+"/acquire", &dev, "")
	h := uWantOK(t, rec, http.StatusCreated)
	if h["setup_name"] != "lab-a" || h["active"] != true || int64(uNum(t, h, "user_id")) != dev.user.ID || h["acquired_at"] != "2026-11-19T10:00:00.000Z" ||
		h["metering_started_at"] != nil || h["deployed"] != false || uNum(t, h, "charged_tokens") != 0 {
		t.Errorf("hold %v", h)
	}
	if m := h["metering"].(ujobj); m["active"] != false || m["reason"] != nil {
		t.Errorf("metering %v", m)
	}
	// It now shows as held in the setup list with the user as holder.
	s := uWantOK(t, e.get("/api/v1/setups", &dev), 200)["setups"].([]any)[0].(ujobj)
	if s["state"] != "held" || !s["holder"].(ujobj)["is_you"].(bool) {
		t.Errorf("setup after acquire %v", s)
	}
}

func TestHOLD1_AcquireRefusals(t *testing.T) {
	e := defaultEnv(t)
	a, b, c := e.setup("a", true, nil), e.setup("b", true, nil), e.setup("c", true, nil)
	off := e.setup("off", false, nil)
	dev, peer := e.dev("dev", store.RoleDeveloper), e.dev("peer", store.RoleDeveloper)
	acq := func(s *usess, id int64) *httptest.ResponseRecorder {
		return e.post("/api/v1/setups/"+uItoa(id)+"/acquire", s, "")
	}
	uWantErr(t, acq(&dev, 9999), 404, "not_found")
	uWantErr(t, acq(&dev, off), 409, "setup_disabled")
	e.acquire(&dev, a)
	uWantErr(t, acq(&peer, a), 409, "setup_held")
	e.acquire(&dev, b)
	uWantErr(t, acq(&dev, c), 409, "hold_limit") // default limit 2
	uWantErr(t, e.post("/api/v1/setups/zero/acquire", &dev, ""), 400, "validation")

	// No tokens: refused while enforcement is on.
	e.setting("enforcement", "true")
	if _, err := e.st.AppendLedger(uBg, store.LedgerEntry{UserID: peer.user.ID, Period: "2026-11", Type: "grant", AmountMT: -300000, Reason: "x"}, "2026-11", e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	uWantErr(t, acq(&peer, c), 409, "insufficient_tokens")

	// A hook veto carries the hook's own message.
	e.setting("enforcement", "false")
	e.hook.SetError(hourglass.EventHoldAcquiring, errors.New("lab is closed on Fridays"))
	rec := acq(&peer, c)
	uWantErr(t, rec, 409, "hook_veto")
	var body struct{ Error struct{ Message string } }
	_ = jsonUnmarshal(rec.Body.Bytes(), &body)
	if body.Error.Message != "lab is closed on Fridays" {
		t.Errorf("veto message %q", body.Error.Message)
	}

	// The local admin cannot acquire.
	adm := e.adminSess()
	uWantErr(t, acq(&adm, c), 403, "forbidden")
	// CSRF is enforced on the state-changing route.
	uWantErr(t, e.do("POST", "/api/v1/setups/"+uItoa(c)+"/acquire", &dev, false, ""), 403, "csrf")
}

func TestHOLD2_ReleaseRules(t *testing.T) {
	e := defaultEnv(t)
	sid := e.setup("lab-a", true, nil)
	dev, peer, adm := e.dev("dev", store.RoleDeveloper), e.dev("peer", store.RoleDeveloper), e.dev("boss", store.RoleAdmin)
	hid := e.acquire(&dev, sid)
	path := "/api/v1/holds/" + uItoa(hid) + "/release"

	uWantErr(t, e.post("/api/v1/holds/9999/release", &dev, ""), 404, "not_found")
	uWantErr(t, e.post("/api/v1/holds/abc/release", &dev, ""), 400, "validation")
	uWantErr(t, e.post(path, &peer, ""), 403, "forbidden")                            // not the owner
	uWantErr(t, e.post(path, &peer, `{"force":true,"reason":"x"}`), 403, "forbidden") // force needs admin
	uWantErr(t, e.post(path, &dev, `{"force":true,"reason":"x"}`), 403, "forbidden")
	uWantErr(t, e.post(path, &adm, ""), 403, "forbidden") // admin must say force for another's hold
	rec := e.post(path, &adm, `{"force":true}`)
	uWantErr(t, rec, 400, "validation")
	if _, f := uErrOf(t, rec); f != "reason" {
		t.Errorf("field %q, want reason", f)
	}
	uWantErr(t, e.post(path, &adm, `{"force":true,"reason":"  "}`), 400, "validation")
	uWantErr(t, e.post(path, &dev, `{"unknown":1}`), 400, "validation")
	uWantErr(t, e.post(path, &dev, `{not json`), 400, "validation")

	// Still deployed: refused for the owner, allowed as a forced release.
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-a"})
	e.poll()
	rec = e.post(path, &dev, `{"force":false}`)
	uWantErr(t, rec, 409, "still_deployed")
	var body struct{ Error struct{ Message string } }
	_ = jsonUnmarshal(rec.Body.Bytes(), &body)
	if body.Error.Message != "This setup is still deployed. Tear it down, then release." {
		t.Errorf("message %q", body.Error.Message)
	}
	h := uWantOK(t, e.post(path, &adm, `{"force":true,"reason":"lab maintenance"}`), 200)
	if h["active"] != false || h["forced"] != true || h["release_reason"] != "lab maintenance" || h["cost_per_hour_tokens"] != nil ||
		h["released_by"].(ujobj)["name"] != "User boss" || int64(uNum(t, h["released_by"].(ujobj), "id")) != adm.user.ID {
		t.Errorf("forced release %v", h)
	}
	// Releasing again: it is gone.
	uWantErr(t, e.post(path, &dev, ""), 409, "conflict") // HOLD-2: already released
	// And the audit trail has it (ADM-3).
	if as, err := e.st.ListAudit(uBg, store.AuditFilter{Action: "hold.force_release"}); err != nil || len(as) != 1 {
		t.Errorf("audit %v %v", as, err)
	}
	// The setup is free again.
	if s := uWantOK(t, e.get("/api/v1/setups", &dev), 200)["setups"].([]any)[0].(ujobj); s["state"] != "free" {
		t.Errorf("setup after release %v", s)
	}
}

func TestHOLD2_OwnerReleaseWithoutProviderOrBody(t *testing.T) {
	e := defaultEnv(t)
	sid := e.setup("lab-a", true, nil)
	dev := e.dev("dev", store.RoleDeveloper)
	hid := e.acquire(&dev, sid)
	h := uWantOK(t, e.post("/api/v1/holds/"+uItoa(hid)+"/release", &dev, ""), 200)
	if h["active"] != false || h["forced"] != false || h["release_reason"] != nil {
		t.Errorf("release %v", h)
	}
}

// The metering reason is the policy's decision at the latest poll that saw
// the hold; null before such a poll, and for a released hold.
func TestMET_ReasonComesFromTheLastPollDecision(t *testing.T) {
	e := defaultEnv(t)
	e.policy.SetDefault(hourglass.Decision{Meter: false, Reason: "waiting for its own condition"})
	sid := e.setup("lab-01", true, nil)
	dev := e.dev("dev", store.RoleDeveloper)
	hid := e.acquire(&dev, sid)
	reasonOf := func() any {
		t.Helper()
		hs := uWantOK(t, e.get("/api/v1/me/holds", &dev), 200)["holds"].([]any)
		return hs[0].(ujobj)["metering"].(ujobj)["reason"]
	}
	listReason := func() any {
		t.Helper()
		for _, s := range uWantOK(t, e.get("/api/v1/setups", &dev), 200)["setups"].([]any) {
			if m, ok := s.(ujobj)["metering"].(ujobj); ok {
				return m["reason"]
			}
		}
		t.Fatal("no held setup in the list")
		return nil
	}
	if r := reasonOf(); r != nil {
		t.Errorf("before a poll: holds reason %v", r)
	}
	if r := listReason(); r != nil {
		t.Errorf("before a poll: setups reason %v", r)
	}
	e.poll()
	if r := reasonOf(); r != "waiting for its own condition" {
		t.Errorf("after a poll: holds reason %v", r)
	}
	if r := listReason(); r != "waiting for its own condition" {
		t.Errorf("after a poll: setups reason %v", r)
	}
	e.policy.SetDefault(hourglass.Decision{Meter: true, Reason: "now charging"})
	e.step(time.Minute)
	if r := reasonOf(); r != "now charging" {
		t.Errorf("next poll: holds reason %v", r)
	}
	h := uWantOK(t, e.post("/api/v1/holds/"+uItoa(hid)+"/release", &dev, ""), 200)
	if m := h["metering"].(ujobj); m["reason"] != nil {
		t.Errorf("released hold metering %v", m)
	}
}

// TestHOLD1_MeCanAcquireFollowsTheAcquireRule: GET /api/v1/me says what
// acquire would do, from the same rule (holds.AcquireBlock), so the UI derives
// nothing (UI-1).
func TestHOLD1_MeCanAcquireFollowsTheAcquireRule(t *testing.T) {
	e := defaultEnv(t)
	a, b, c := e.setup("a", true, nil), e.setup("b", true, nil), e.setup("c", true, nil)
	dev, broke := e.dev("dev", store.RoleDeveloper), e.dev("broke", store.RoleDeveloper)
	check := func(label string, s *usess, can bool, reason any) {
		t.Helper()
		m := uWantOK(t, e.get("/api/v1/me", s), 200)
		if m["can_acquire"] != can || m["cannot_acquire_reason"] != reason {
			t.Errorf("%s: can_acquire=%v reason=%v, want %v %v", label, m["can_acquire"], m["cannot_acquire_reason"], can, reason)
		}
		// The rule and the route agree: a refusal here is the code acquire returns.
		if !can {
			rec := e.post("/api/v1/setups/"+uItoa(c)+"/acquire", s, "")
			uWantErr(t, rec, rec.Code, reason.(string))
		}
	}
	check("fresh developer", &dev, true, nil)
	e.acquire(&dev, a)
	check("one of two holds", &dev, true, nil)
	e.acquire(&dev, b)
	check("at the hold limit", &dev, false, "hold_limit")

	// Out of tokens: refused with enforcement on, allowed in shadow mode (MET-6).
	if _, err := e.st.AppendLedger(uBg, store.LedgerEntry{UserID: broke.user.ID, Period: "2026-11", Type: "grant", AmountMT: -300000, Reason: "x"}, "2026-11", e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	check("no tokens, shadow mode", &broke, true, nil)
	e.setting("enforcement", "true")
	check("no tokens, enforcement", &broke, false, "insufficient_tokens")

	adm := e.adminSess()
	m := uWantOK(t, e.get("/api/v1/me", &adm), 200)
	if m["can_acquire"] != false || m["cannot_acquire_reason"] != "local_admin" {
		t.Errorf("local admin: %v %v", m["can_acquire"], m["cannot_acquire_reason"])
	}
}
