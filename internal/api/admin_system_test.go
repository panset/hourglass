// SPDX-License-Identifier: MIT

package api

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

const rulesPath = "/api/v1/admin/rules"

func TestRULE3_GetRulesCountsAndFallThrough(t *testing.T) {
	e := newAdEnv(t)
	e.setup("a1", true, map[string]string{"tier": "a"})
	e.setup("a2", true, map[string]string{"tier": "a", "site": "x"})
	e.setup("b1", true, map[string]string{"tier": "b"})
	e.setup("c1", true, map[string]string{"tier": "c"})
	e.setup("off", false, map[string]string{"tier": "a"}) // disabled setups are not counted
	if err := e.st.ReplaceRules(uBg, "rate", []store.Rule{
		{Selector: "site=x", Value: 4}, {Selector: "tier=a", Value: 2}, {Selector: "tier=zzz", Value: 9}}, nil, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	if err := e.st.PutSetting(uBg, "default_weight", "1.5", nil, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	b := uWantOK(t, e.get(rulesPath), 200)
	uWantKeys(t, "response", b, "rules", "default_weight", "unmatched_setups")
	if uNum(t, b, "default_weight") != 1.5 {
		t.Errorf("default_weight = %v", b["default_weight"])
	}
	rs := adArr(t, b, "rules")
	if len(rs) != 3 {
		t.Fatalf("rules = %v", rs)
	}
	wantSel, wantCount, wantW := []string{"site=x", "tier=a", "tier=zzz"}, []float64{1, 1, 0}, []float64{4, 2, 9}
	for i, x := range rs {
		r := adObjOf(t, x)
		uWantKeys(t, "rule", r, "id", "position", "selector", "weight", "match_count")
		if r["selector"] != wantSel[i] || uNum(t, r, "match_count") != wantCount[i] || uNum(t, r, "position") != float64(i+1) || uNum(t, r, "weight") != wantW[i] {
			t.Errorf("rule %d = %v", i, r)
		}
	}
	un := adArr(t, b, "unmatched_setups")
	if len(un) != 2 {
		t.Fatalf("unmatched = %v", un)
	}
	first := adObjOf(t, un[0])
	uWantKeys(t, "setup ref", first, "id", "name", "labels")
	if first["name"] != "b1" || adObjOf(t, un[1])["name"] != "c1" {
		t.Errorf("unmatched = %v", un)
	}
}

func TestRULE3_PutRulesReplacesAndAudits(t *testing.T) {
	e := newAdEnv(t)
	e.setup("a1", true, map[string]string{"tier": "a"})
	e.setup("b1", true, map[string]string{"tier": "b"})
	if err := e.st.ReplaceRules(uBg, "rate", []store.Rule{{Selector: "tier=old", Value: 7}}, nil, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	b := uWantOK(t, e.send("PUT", rulesPath, `{"rules":[{"selector":"tier=b","weight":3},{"selector":"tier in (a,b)","weight":0.5},{"selector":"","weight":1}]}`), 200)
	rs := adArr(t, b, "rules")
	if len(rs) != 3 || adObjOf(t, rs[0])["selector"] != "tier=b" || uNum(t, adObjOf(t, rs[0]), "match_count") != 1 || uNum(t, adObjOf(t, rs[1]), "match_count") != 1 ||
		uNum(t, adObjOf(t, rs[1]), "weight") != 0.5 || uNum(t, adObjOf(t, rs[2]), "match_count") != 0 {
		t.Errorf("rules = %v", rs)
	}
	if len(adArr(t, b, "unmatched_setups")) != 0 {
		t.Errorf("unmatched = %v", b["unmatched_setups"])
	}
	// The stored list is exactly the new one, in order, attributed to the admin (RULE-4).
	rows, _ := e.st.ListRules(uBg, "rate")
	if len(rows) != 3 || rows[0].Selector != "tier=b" || rows[2].Position != 2 || rows[0].UpdatedBy == nil || *rows[0].UpdatedBy != e.admin.ID {
		t.Errorf("stored rules = %+v", rows)
	}
	a := e.audits("rules.replace")
	if len(a) != 1 || !strings.Contains(a[0].BeforeJSON, "tier=old") || !strings.Contains(a[0].AfterJSON, "tier in (a,b)") {
		t.Errorf("audit = %+v", a)
	}
	// An empty list clears the rules; everything falls through.
	b = uWantOK(t, e.send("PUT", rulesPath, `{"rules":[]}`), 200)
	if len(adArr(t, b, "rules")) != 0 || len(adArr(t, b, "unmatched_setups")) != 2 {
		t.Errorf("after clearing: %v", b)
	}
}

func TestRULE3_InvalidSelectorRejectsWholeRequest(t *testing.T) {
	e := newAdEnv(t)
	if err := e.st.ReplaceRules(uBg, "rate", []store.Rule{{Selector: "tier=keep", Value: 1}}, nil, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	f, msg := adErr(t, e.send("PUT", rulesPath, `{"rules":[{"selector":"tier=a","weight":1},{"selector":"tier in (a","weight":2}]}`), 400, "invalid_selector")
	if f != "rules[1].selector" || !strings.Contains(msg, "position") {
		t.Errorf("field %q message %q", f, msg)
	}
	rows, _ := e.st.ListRules(uBg, "rate")
	if len(rows) != 1 || rows[0].Selector != "tier=keep" {
		t.Errorf("rules changed by a rejected request: %+v", rows)
	}
	if n := len(e.audits("rules.replace")); n != 0 {
		t.Errorf("rejected request wrote %d audit rows", n)
	}
	for _, c := range []struct{ body, field, code string }{
		{`{"rules":[{"selector":"a=b","weight":-1}]}`, "rules[0].weight", "validation"},
		{`{"rules":[{"selector":"a=b"}]}`, "rules[0].weight", "validation"},
		{`{"rules":[{"weight":1}]}`, "rules[0].selector", "validation"},
		{`{}`, "rules", "validation"},
		{`{"rules":[{"selector":"a=b","weight":1,"x":1}]}`, "x", "validation"},
	} {
		if f, _ := adErr(t, e.send("PUT", rulesPath, c.body), 400, c.code); f != c.field {
			t.Errorf("%s: field %q, want %q", c.body, f, c.field)
		}
	}
}

const settingsPath = "/api/v1/admin/settings"

func fullSettings(mod string) string {
	s := `{"default_weight":1,"display_label_keys":["tier"],` +
		`"bands":{"base":0,"steps":[{"from":0.5,"multiplier":1},{"from":0.8,"multiplier":2}]},"default_allowance":250.5,` +
		`"max_holds_per_user":3,"release_requires_teardown":true,"poll_interval":"10m","enforcement":true}`
	for _, kv := range strings.Split(mod, "|") {
		if kv == "" {
			continue
		}
		k, v, _ := strings.Cut(kv, "=")
		start := strings.Index(s, `"`+k+`":`)
		if start < 0 {
			panic("no key " + k)
		}
		end := start + len(k) + 3
		depth, i := 0, end
		for ; i < len(s); i++ {
			c := s[i]
			if c == '{' || c == '[' {
				depth++
			} else if c == '}' || c == ']' {
				if depth == 0 {
					break
				}
				depth--
			} else if c == ',' && depth == 0 {
				break
			}
		}
		s = s[:end] + v + s[i:]
	}
	return s
}

func TestADM2_GetSettingsShapeAndRuntime(t *testing.T) {
	e := newAdEnv(t)
	b := uWantOK(t, e.get(settingsPath), 200)
	uWantKeys(t, "response", b, "settings", "runtime")
	uWantKeys(t, "settings", adObjOf(t, b["settings"]), "default_weight", "display_label_keys", "bands", "default_allowance",
		"max_holds_per_user", "release_requires_teardown", "poll_interval", "enforcement", "reported_weights")
	st := adObjOf(t, b["settings"])
	if uNum(t, st, "default_allowance") != 300 || st["poll_interval"] != "5m" || st["enforcement"] != false {
		t.Errorf("settings = %v", st)
	}
	rt := adObjOf(t, b["runtime"])
	uWantKeys(t, "runtime", rt, "provider", "pressure_source", "metering_policy", "hooks", "oidc_issuer", "timezone")
	if rt["provider"] != "fake-provider" || rt["pressure_source"] != "fake-pressure" || rt["timezone"] != "UTC" || rt["oidc_issuer"] != nil {
		t.Errorf("runtime = %v", rt)
	}
	if mp := adObjOf(t, rt["metering_policy"]); mp["name"] != "fake-policy" || mp["description"] != nil {
		t.Errorf("metering_policy = %v", mp)
	}
	if h := adArr(t, rt, "hooks"); len(h) != 1 || h[0] != "fake-hook" {
		t.Errorf("hooks = %v", h)
	}
}

func TestADM2_RuntimeShowsOIDCIssuerAndTimezone(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no tzdata")
	}
	b := newBackend(t, backendOpts{loc: loc, runtime: Runtime{OIDCIssuer: "https://issuer.example"}})
	adm := b.session(b.admin)
	rt := adObjOf(t, uWantOK(t, b.serve("GET", "/api/v1/admin/settings", &adm, false, ""), 200)["runtime"])
	if rt["oidc_issuer"] != "https://issuer.example" || rt["timezone"] != "Europe/Berlin" || rt["provider"] != nil {
		t.Errorf("runtime = %v", rt)
	}
}

func TestADM2_PutSettingsSavesAndAuditsChangedKeys(t *testing.T) {
	e := newAdEnv(t)
	b := uWantOK(t, e.send("PUT", settingsPath, fullSettings("")), 200)
	st := adObjOf(t, b["settings"])
	if uNum(t, st, "default_allowance") != 250.5 || uNum(t, st, "max_holds_per_user") != 3 || st["poll_interval"] != "10m" || st["enforcement"] != true {
		t.Errorf("returned settings = %v", st)
	}
	if adObjOf(t, b["runtime"])["provider"] != "fake-provider" {
		t.Errorf("runtime missing from PUT response")
	}
	// GET returns the same body.
	if g := e.get(settingsPath); g.Body.String() != e.get(settingsPath).Body.String() || !strings.Contains(g.Body.String(), `"max_holds_per_user":3`) {
		t.Errorf("GET after PUT: %s", g.Body)
	}
	rows := e.audits("settings.update")
	if len(rows) != 1 {
		t.Fatalf("audit rows = %d", len(rows))
	}
	for _, key := range []string{"display_label_keys", "bands", "default_allowance", "max_holds_per_user", "poll_interval", "enforcement"} {
		if !strings.Contains(rows[0].AfterJSON, `"`+key+`"`) || !strings.Contains(rows[0].BeforeJSON, `"`+key+`"`) {
			t.Errorf("audit lacks changed key %s: %+v", key, rows[0])
		}
	}
	for _, key := range []string{"default_weight", "release_requires_teardown"} {
		if strings.Contains(rows[0].AfterJSON, `"`+key+`"`) {
			t.Errorf("audit holds the unchanged key %s", key)
		}
	}
	// Saving the same values again changes nothing and writes no audit row.
	uWantOK(t, e.send("PUT", settingsPath, fullSettings("")), 200)
	if n := len(e.audits("settings.update")); n != 1 {
		t.Errorf("no-op save wrote an audit row: %d", n)
	}
	// Changes are not retroactive: the allowance setting does not touch ledgers.
	d := e.dev("dev2")
	if n, _ := e.st.SumLedger(uBg, d.user.ID, "2026-11", ""); n != 250_500 {
		t.Errorf("new user's allowance = %d mt", n)
	}
}

func TestADM2_PutSettingsValidation(t *testing.T) {
	e := newAdEnv(t)
	cases := []struct{ name, body, field string }{
		{"poll interval too short", fullSettings(`poll_interval="30s"`), "poll_interval"},
		{"poll interval unparsable", fullSettings(`poll_interval="soon"`), "poll_interval"},
		{"negative weight", fullSettings(`default_weight=-1`), "default_weight"},
		{"bad label key", fullSettings(`display_label_keys=["Bad Key"]`), "display_label_keys"},
		{"duplicate label key", fullSettings(`display_label_keys=["a","a"]`), "display_label_keys"},
		{"bad bands", fullSettings(`bands={"base":0,"steps":[{"from":0.9,"multiplier":1},{"from":0.5,"multiplier":2}]}`), "bands"},
		{"negative allowance", fullSettings(`default_allowance=-5`), "default_allowance"},
		{"negative holds", fullSettings(`max_holds_per_user=-1`), "max_holds_per_user"},
		{"wrong type", fullSettings(`enforcement="yes"`), "enforcement"},
		{"unknown field", strings.Replace(fullSettings(""), `{"default`, `{"bogus":1,"default`, 1), "bogus"},
		{"missing field", `{"enforcement":true}`, "default_weight"},
	}
	for _, c := range cases {
		if f, _ := adErr(t, e.send("PUT", settingsPath, c.body), 400, "validation"); f != c.field {
			t.Errorf("%s: field = %q, want %q", c.name, f, c.field)
		}
	}
	adErr(t, e.send("PUT", settingsPath, `[1]`), 400, "validation")
	if n := len(e.audits("settings.update")); n != 0 {
		t.Errorf("rejected requests wrote %d audit rows", n)
	}
	if b := adObjOf(t, uObj(t, e.get(settingsPath))["settings"]); uNum(t, b, "max_holds_per_user") != 2 {
		t.Errorf("a rejected PUT changed the settings: %v", b)
	}
}

func TestADM4_CollectorHealth(t *testing.T) {
	e := newAdEnv(t)
	d := e.dev("dev")
	held := e.setup("lab-held", true, nil)
	e.setup("lab-free", true, nil)
	e.hold(held, d.user.ID)
	// The provider reports the held setup, the free setup (an orphan, HOLD-4) and an unknown resource.
	e.prov.SetAllocations(hourglassAlloc("lab-held"), hourglassAlloc("lab-free", "site", "x"), hourglassAlloc("unrelated"))
	if err := e.col.RunOnce(uBg); err != nil {
		t.Fatal(err)
	}
	// A failed run, an outage this month and a failed delivery.
	e.clk.Advance(5 * time.Minute)
	e.pressure.SetError(errors.New("pressure down"))
	if err := e.col.RunOnce(uBg); err == nil {
		t.Fatal("poll should have failed")
	}
	e.pressure.SetError(nil)
	if _, err := e.st.InsertOutage(uBg, store.Outage{StartedAt: e.clk.Now().Add(-3 * time.Hour), EndedAt: e.clk.Now().Add(-2 * time.Hour), Reason: "gap"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.st.InsertOutage(uBg, store.Outage{StartedAt: e.clk.Now().AddDate(0, -1, 0), EndedAt: e.clk.Now().AddDate(0, -1, 0).Add(time.Hour), Reason: "old"}); err != nil {
		t.Fatal(err)
	}
	id, err := e.st.InsertEventDelivery(uBg, "hold.acquired", "fake-hook", `{"secret":"payload"}`, e.clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := e.st.FinishEventDelivery(uBg, id, e.clk.Now(), false, "hook exploded"); err != nil {
		t.Fatal(err)
	}

	b := uWantOK(t, e.get("/api/v1/admin/collector"), 200)
	uWantKeys(t, "collector", b, "extensions", "last_pressure_report", "last_deployments_report", "last_successful_poll_at", "last_error", "matched_setups", "orphan_count", "outages_this_month",
		"recent_runs", "outages", "orphans", "failed_deliveries", "queue_length")
	ext := adObjOf(t, b["extensions"])
	uWantKeys(t, "extensions", ext, "provider", "pressure_source", "metering_policy", "hooks")
	if ext["provider"] != "fake-provider" || ext["metering_policy"] != "fake-policy" {
		t.Errorf("extensions = %v", ext)
	}
	if b["last_successful_poll_at"] != "2026-11-19T10:00:00.000Z" {
		t.Errorf("last_successful_poll_at = %v", b["last_successful_poll_at"])
	}
	le := adObjOf(t, b["last_error"])
	uWantKeys(t, "last_error", le, "message", "at")
	if !strings.Contains(le["message"].(string), "pressure down") {
		t.Errorf("last_error = %v", le)
	}
	if uNum(t, b, "matched_setups") != 2 || uNum(t, b, "orphan_count") != 1 {
		t.Errorf("matched %v orphans %v", b["matched_setups"], b["orphan_count"])
	}
	om := adObjOf(t, b["outages_this_month"])
	if uNum(t, om, "count") != 1 || uNum(t, om, "minutes_not_charged") != 60 {
		t.Errorf("outages_this_month = %v", om)
	}
	outs := adArr(t, b, "outages")
	if len(outs) != 2 || uNum(t, adObjOf(t, outs[0]), "duration_minutes") != 60 {
		t.Errorf("outages = %v", outs)
	}
	uWantKeys(t, "outage", adObjOf(t, outs[0]), "started_at", "ended_at", "duration_minutes")
	orphans := adArr(t, b, "orphans")
	o := adObjOf(t, orphans[0])
	uWantKeys(t, "orphan", o, "resource_name", "first_seen_at", "last_seen_at", "setup", "labels")
	if o["resource_name"] != "lab-free" || adObjOf(t, o["setup"])["name"] != "lab-free" || adObjOf(t, o["labels"])["site"] != "x" {
		t.Errorf("orphan = %v", o)
	}
	runs := adArr(t, b, "recent_runs")
	if len(runs) != 2 {
		t.Fatalf("runs = %v", runs)
	}
	uWantKeys(t, "run", adObjOf(t, runs[0]), "id", "started_at", "finished_at", "ok", "error")
	if adObjOf(t, runs[0])["ok"] != false || adObjOf(t, runs[1])["ok"] != true || adObjOf(t, runs[1])["error"] != nil {
		t.Errorf("runs = %v", runs)
	}
	fd := adArr(t, b, "failed_deliveries")
	if len(fd) != 1 {
		t.Fatalf("failed deliveries = %v", fd)
	}
	uWantKeys(t, "delivery", adObjOf(t, fd[0]), "id", "event_type", "hook", "error", "started_at", "finished_at")
	if adObjOf(t, fd[0])["error"] != "hook exploded" || strings.Contains(string(adBytes(b)), "payload") {
		t.Errorf("failed delivery = %v", fd[0])
	}
	if uNum(t, b, "queue_length") < 0 {
		t.Errorf("queue_length = %v", b["queue_length"])
	}
}

func TestADM4_CollectorBeforeFirstPoll(t *testing.T) {
	e := newAdEnv(t)
	b := uWantOK(t, e.get("/api/v1/admin/collector"), 200)
	if b["last_successful_poll_at"] != nil || b["last_error"] != nil || uNum(t, b, "matched_setups") != 0 ||
		len(adArr(t, b, "recent_runs")) != 0 || len(adArr(t, b, "orphans")) != 0 || len(adArr(t, b, "outages")) != 0 || len(adArr(t, b, "failed_deliveries")) != 0 {
		t.Errorf("collector = %v", b)
	}
}

func TestADM3_AuditListFiltersAndPaging(t *testing.T) {
	e := newAdEnv(t)
	other := e.user("boss2", store.StatusActive, store.RoleAdmin)
	d := e.dev("dev")
	grant := "/api/v1/admin/users/" + uItoa(d.user.ID) + "/grants"
	uWantOK(t, e.send("POST", grant, `{"amount_tokens":1,"reason":"one"}`), 201)
	uWantOK(t, e.send("POST", grant, `{"amount_tokens":2,"reason":"two"}`), 201)
	uWantOK(t, e.send("PUT", rulesPath, `{"rules":[{"selector":"a=b","weight":1}]}`), 200)
	uWantOK(t, e.do("PUT", rulesPath, &other, true, `{"rules":[]}`), 200)

	b := uWantOK(t, e.get("/api/v1/admin/audit"), 200)
	uWantKeys(t, "response", b, "entries", "total", "limit", "offset")
	if uNum(t, b, "total") != 4 || uNum(t, b, "limit") != 50 || uNum(t, b, "offset") != 0 {
		t.Errorf("response = %v", b)
	}
	es := adArr(t, b, "entries")
	first := adObjOf(t, es[0])
	uWantKeys(t, "entry", first, "id", "actor", "action", "target", "before", "after", "created_at")
	uWantKeys(t, "actor", adObjOf(t, first["actor"]), "id", "name")
	if first["action"] != "rules.replace" || adObjOf(t, first["actor"])["name"] != "User boss2" || uNum(t, adObjOf(t, first["actor"]), "id") != float64(other.user.ID) || first["target"] != "rules:rate" {
		t.Errorf("newest entry = %v", first)
	}
	if adObjOf(t, adObjOf(t, es[2])["after"]) == nil || adObjOf(t, es[3])["action"] != "ledger.grant" {
		t.Errorf("entries = %v", es)
	}
	// Newest first by id.
	prev := 1e18
	for _, x := range es {
		id := uNum(t, adObjOf(t, x), "id")
		if id >= prev {
			t.Errorf("not newest first: %v", es)
		}
		prev = id
	}
	g := uWantOK(t, e.get("/api/v1/admin/audit?action=ledger.grant"), 200)
	if uNum(t, g, "total") != 2 || len(adArr(t, g, "entries")) != 2 {
		t.Errorf("action filter: %v", g)
	}
	a := uWantOK(t, e.get("/api/v1/admin/audit?actor="+uItoa(other.user.ID)), 200)
	if uNum(t, a, "total") != 1 {
		t.Errorf("actor filter: %v", a)
	}
	p := uWantOK(t, e.get("/api/v1/admin/audit?limit=1&offset=1"), 200)
	pe := adArr(t, p, "entries")
	if uNum(t, p, "total") != 4 || len(pe) != 1 || adObjOf(t, pe[0])["action"] != "rules.replace" || uNum(t, p, "limit") != 1 || uNum(t, p, "offset") != 1 {
		t.Errorf("paging: %v", p)
	}
	empty := uWantOK(t, e.get("/api/v1/admin/audit?actor=99999"), 200)
	if uNum(t, empty, "total") != 0 || len(adArr(t, empty, "entries")) != 0 {
		t.Errorf("empty: %v", empty)
	}
	for _, q := range []string{"limit=0", "limit=201", "limit=x", "offset=-1", "actor=abc"} {
		adErr(t, e.get("/api/v1/admin/audit?"+q), 400, "validation")
	}
}

func TestADM3_SecretsNeverEnterAuditRows(t *testing.T) {
	e := newAdEnv(t)
	uWantOK(t, e.send("POST", setupPath, setupBody("lab-01", `{"password":"hunter2-secret","token":"abc123"}`, true)), 201)
	uWantOK(t, e.send("PUT", settingsPath, fullSettings("")), 200)
	rows, err := e.st.ListAudit(uBg, store.AuditFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if strings.Contains(r.BeforeJSON+r.AfterJSON, "hunter2") || strings.Contains(r.BeforeJSON+r.AfterJSON, "abc123") {
			t.Errorf("audit row holds a secret: %+v", r)
		}
	}
}

func TestEXT7_UserApprovedHookReceivesUser(t *testing.T) {
	e := newAdEnv(t)
	p := e.user("p", store.StatusPending)
	uWantOK(t, e.send("PUT", "/api/v1/admin/users/"+uItoa(p.user.ID)+"/roles", `{"roles":["admin"]}`), 200)
	e.flushEvents()
	evs := e.hook.EventsOfType(hourglass.EventUserApproved)
	if len(evs) != 1 || evs[0].User == nil || len(evs[0].User.Roles) != 1 || evs[0].User.Roles[0] != "admin" {
		t.Errorf("events = %+v", evs)
	}
}

// ADM-3 / RULE-4: a rules change shows the whole before and after list through
// the audit API (the verifier's finding: both came back null).
func TestADM3_RulesAuditShowsBeforeAndAfterLists(t *testing.T) {
	e := newAdEnv(t)
	uWantOK(t, e.send("PUT", rulesPath, `{"rules":[{"selector":"tier=a","weight":2}]}`), 200)
	uWantOK(t, e.send("PUT", rulesPath, `{"rules":[{"selector":"tier=b","weight":3},{"selector":"","weight":1}]}`), 200)
	es := adArr(t, uWantOK(t, e.get("/api/v1/admin/audit?action=rules.replace"), 200), "entries")
	if len(es) != 2 {
		t.Fatalf("entries = %v", es)
	}
	newest := adObjOf(t, es[0])
	before, after := adArr(t, adObjOf(t, newest["before"]), "rules"), adArr(t, adObjOf(t, newest["after"]), "rules")
	if len(before) != 1 || adObjOf(t, before[0])["selector"] != "tier=a" || uNum(t, adObjOf(t, before[0]), "weight") != 2 ||
		len(after) != 2 || adObjOf(t, after[1])["selector"] != "" || adObjOf(t, after[0])["selector"] != "tier=b" {
		t.Errorf("before %v after %v", before, after)
	}
	first := adObjOf(t, es[1])
	if len(adArr(t, adObjOf(t, first["before"]), "rules")) != 0 || len(adArr(t, adObjOf(t, first["after"]), "rules")) != 1 {
		t.Errorf("first change: %v", first)
	}
}

// ADM-3: no audit action reads back with both sides null; each carries what
// changed, as an object, through the API.
func TestADM3_EveryAuditActionCarriesBeforeAndAfter(t *testing.T) {
	e := newAdEnv(t)
	d := e.dev("dev")
	p := e.user("pend", store.StatusPending)
	id := e.setup("s1", true, map[string]string{"tier": "a"})
	sp := setupPath + "/" + uItoa(id)
	uWantOK(t, e.send("PUT", sp, `{"name":"s1","labels":{"tier":"b"},"details":{},"enabled":true}`), 200)
	uWantOK(t, e.send("PUT", settingsPath, fullSettings("default_allowance=1")), 200)
	uWantOK(t, e.send("POST", "/api/v1/admin/users/"+uItoa(d.user.ID)+"/grants", `{"amount_tokens":1,"reason":"one"}`), 201)
	uWantOK(t, e.send("POST", "/api/v1/admin/users/"+uItoa(p.user.ID)+"/reject", ""), 200)
	uWantOK(t, e.send("PUT", "/api/v1/admin/users/"+uItoa(d.user.ID)+"/roles", `{"roles":["developer","admin"]}`), 200)
	uWantOK(t, e.send("PUT", rulesPath, `{"rules":[{"selector":"tier=b","weight":3}]}`), 200)
	if rec := e.send("DELETE", sp, ""); rec.Code != 204 {
		t.Fatalf("delete setup: %d %s", rec.Code, rec.Body)
	}
	es := adArr(t, uWantOK(t, e.get("/api/v1/admin/audit?limit=200"), 200), "entries")
	seen := map[string]bool{}
	for _, x := range es {
		en := adObjOf(t, x)
		act, _ := en["action"].(string)
		seen[act] = true
		b, a := en["before"], en["after"]
		if b == nil && a == nil {
			t.Errorf("%s: before and after are both null", act)
		}
		switch act {
		case "setup.update", "setup.delete", "user.roles", "user.reject", "rules.replace", "settings.update", "ledger.grant":
			if b == nil {
				t.Errorf("%s: before is null", act)
			}
		}
		switch act {
		case "setup.create", "setup.update", "user.roles", "user.reject", "rules.replace", "settings.update", "ledger.grant":
			if a == nil {
				t.Errorf("%s: after is null", act)
			}
		}
	}
	for _, act := range []string{"setup.update", "setup.delete", "user.roles", "user.reject", "rules.replace", "settings.update", "ledger.grant"} {
		if !seen[act] {
			t.Errorf("no %s audit row seen", act)
		}
	}
}
