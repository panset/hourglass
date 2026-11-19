// SPDX-License-Identifier: MIT

package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

const setupPath = "/api/v1/admin/setups"

func setupBody(name, details string, enabled bool) string {
	return fmt.Sprintf(`{"name":%q,"labels":{"tier":"a","site":"x"},"details":%s,"enabled":%t,"resource_name_override":null}`, name, details, enabled)
}

func TestSET1_CreateSetupShapeAndAudit(t *testing.T) {
	e := newAdEnv(t)
	if err := e.st.ReplaceRules(uBg, "rate", []store.Rule{{Selector: "tier=a", Value: 2}}, nil, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	rec := e.send("POST", setupPath, setupBody("lab-01", `{"note":"opaque"}`, true))
	s := uWantOK(t, rec, 201)
	uWantKeys(t, "setup", s, "id", "name", "labels", "weight", "state", "holder", "held_since", "deployed", "metering", "cost_per_hour_tokens", "details", "admin")
	if s["name"] != "lab-01" || uNum(t, s, "weight") != 2 || s["state"] != "free" || s["holder"] != nil || s["held_since"] != nil ||
		s["deployed"] != false || s["metering"] != nil {
		t.Errorf("setup = %v", s)
	}
	if l := adObjOf(t, s["labels"]); l["tier"] != "a" || l["site"] != "x" {
		t.Errorf("labels = %v", l)
	}
	cost := adObjOf(t, s["cost_per_hour_tokens"])
	uWantKeys(t, "cost", cost, "now", "at_peak")
	if uNum(t, cost, "now") != 0 || uNum(t, cost, "at_peak") != 6 { // no pressure sample yet: base band 0x; peak 3x of weight 2
		t.Errorf("cost = %v", cost)
	}
	ad := adObjOf(t, s["admin"])
	uWantKeys(t, "admin", ad, "enabled", "resource_name", "resource_name_override", "last_seen_at", "matched_rule_selector")
	if ad["enabled"] != true || ad["resource_name"] != "lab-01" || ad["resource_name_override"] != nil || ad["last_seen_at"] != nil || ad["matched_rule_selector"] != "tier=a" {
		t.Errorf("admin = %v", ad)
	}
	if d := adObjOf(t, s["details"]); d["note"] != "opaque" {
		t.Errorf("details = %v", d)
	}
	rows := e.audits("setup.create")
	if len(rows) != 1 || rows[0].BeforeJSON != "" || !strings.Contains(rows[0].AfterJSON, `"name":"lab-01"`) || rows[0].Target != "setup:"+uItoa(int64(s["id"].(float64))) {
		t.Errorf("audit = %+v", rows)
	}
}

// TestSET2_DetailsStoredAndReturnedByteForByte covers whitespace, key order,
// HTML-special characters and unicode escapes in the opaque details.
func TestSET2_DetailsStoredAndReturnedByteForByte(t *testing.T) {
	e := newAdEnv(t)
	details := "{ \"z\" : 1,\n  \"a\" : \"<b>&amp;\\u00e9\\n</b>\" ,\"n\":1.50e+2, \"arr\":[ 1 ,2 ] }"
	rec := e.send("POST", setupPath, setupBody("lab-01", details, true))
	if rec.Code != 201 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"details":`+details) {
		t.Errorf("create response does not carry details verbatim: %s", rec.Body)
	}
	id := int64(uObj(t, rec)["id"].(float64))
	x, err := e.st.GetSetup(uBg, id)
	if err != nil || x.DetailsJSON != details {
		t.Fatalf("stored details = %q (%v)", x.DetailsJSON, err)
	}
	// The same on update, and the audit row records only a size and a hash.
	details2 := "[\n  \"<>\",   null ]"
	rec = e.send("PUT", setupPath+"/"+uItoa(id), setupBody("lab-01", details2, true))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"details":`+details2) {
		t.Errorf("update response: %d %s", rec.Code, rec.Body)
	}
	for _, a := range append(e.audits("setup.create"), e.audits("setup.update")...) {
		if strings.Contains(a.BeforeJSON+a.AfterJSON, "amp") || !strings.Contains(a.AfterJSON, "details_sha256") {
			t.Errorf("audit row holds details or lacks its hash: %+v", a)
		}
	}
	// A scalar and null are well-formed JSON too.
	for _, d := range []string{`"text"`, `42`, `null`, `true`} {
		rec = e.send("PUT", setupPath+"/"+uItoa(id), setupBody("lab-01", d, true))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"details":`+d+`,`) && !strings.Contains(rec.Body.String(), `"details":`+d+`}`) {
			t.Errorf("details %s: %d %s", d, rec.Code, rec.Body)
		}
	}
}

func TestSET2_DetailsValidation(t *testing.T) {
	e := newAdEnv(t)
	big := `"` + strings.Repeat("x", maxDetailsBytes) + `"` // 256 KB + 2
	if f, _ := adErr(t, e.send("POST", setupPath, setupBody("a", big, true)), 400, "validation"); f != "details" {
		t.Errorf("oversized field = %q", f)
	}
	exact := `"` + strings.Repeat("x", maxDetailsBytes-2) + `"`
	uWantOK(t, e.send("POST", setupPath, setupBody("b", exact, true)), 201)
	if f, _ := adErr(t, e.send("POST", setupPath, `{"name":"c","labels":{},"enabled":true}`), 400, "validation"); f != "details" {
		t.Errorf("missing details field = %q", f)
	}
	adErr(t, e.send("POST", setupPath, `{"name":"c","labels":{},"details":{bad},"enabled":true}`), 400, "validation")
}

func TestSET3_LabelAndFieldValidation(t *testing.T) {
	e := newAdEnv(t)
	cases := []struct{ name, body, field string }{
		{"bad key", `{"name":"a","labels":{"Bad Key":"v"},"details":{},"enabled":true}`, "labels"},
		{"long value", `{"name":"a","labels":{"k":"` + strings.Repeat("v", 64) + `"},"details":{},"enabled":true}`, "labels"},
		{"non-string value", `{"name":"a","labels":{"k":1},"details":{},"enabled":true}`, "labels.k"},
		{"no labels", `{"name":"a","details":{},"enabled":true}`, "labels"},
		{"no name", `{"labels":{},"details":{},"enabled":true}`, "name"},
		{"blank name", `{"name":"  ","labels":{},"details":{},"enabled":true}`, "name"},
		{"no enabled", `{"name":"a","labels":{},"details":{}}`, "enabled"},
		{"unknown field", `{"name":"a","labels":{},"details":{},"enabled":true,"kind":"x"}`, "kind"},
	}
	many := map[string]string{}
	for i := 0; i < 33; i++ {
		many[fmt.Sprintf("k%d", i)] = "v"
	}
	mb, _ := json.Marshal(many)
	cases = append(cases, struct{ name, body, field string }{"33 labels", `{"name":"a","labels":` + string(mb) + `,"details":{},"enabled":true}`, "labels"})
	for _, c := range cases {
		if f, _ := adErr(t, e.send("POST", setupPath, c.body), 400, "validation"); f != c.field {
			t.Errorf("%s: field = %q, want %q", c.name, f, c.field)
		}
	}
	if n := len(e.audits("setup.create")); n != 0 {
		t.Errorf("rejected input wrote %d audit rows", n)
	}
}

func TestSET4_ResourceNameFromTemplateAndOverride(t *testing.T) {
	e := newAdEnv(t)
	if err := e.st.PutSetting(uBg, "resource_name_template", `"{{.SetupName}}_RP"`, nil, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	s := uWantOK(t, e.send("POST", setupPath, setupBody("lab-01", `{}`, true)), 201)
	if adObjOf(t, s["admin"])["resource_name"] != "lab-01_RP" {
		t.Errorf("admin = %v", s["admin"])
	}
	body := `{"name":"lab-02","labels":{},"details":{},"enabled":true,"resource_name_override":"  pool-7 "}`
	s = uWantOK(t, e.send("POST", setupPath, body), 201)
	ad := adObjOf(t, s["admin"])
	if ad["resource_name"] != "pool-7" || ad["resource_name_override"] != "pool-7" {
		t.Errorf("admin = %v", ad)
	}
	// A blank override falls back to the template.
	s = uWantOK(t, e.send("POST", setupPath, `{"name":"lab-03","labels":{},"details":{},"enabled":true,"resource_name_override":""}`), 201)
	if ad := adObjOf(t, s["admin"]); ad["resource_name"] != "lab-03_RP" || ad["resource_name_override"] != nil {
		t.Errorf("admin = %v", ad)
	}
	// A template that renders empty for this setup is refused.
	if err := e.st.PutSetting(uBg, "resource_name_template", `"{{index .Labels \"missing\"}}"`, nil, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	adErr(t, e.send("POST", setupPath, setupBody("lab-04", `{}`, true)), 400, "validation")
}

func TestSET1_DuplicateNameIsConflict(t *testing.T) {
	e := newAdEnv(t)
	uWantOK(t, e.send("POST", setupPath, setupBody("lab-01", `{}`, true)), 201)
	_, msg := adErr(t, e.send("POST", setupPath, setupBody("lab-01", `{}`, true)), 409, "conflict")
	if !strings.Contains(msg, "name") {
		t.Errorf("message = %q", msg)
	}
	other := uWantOK(t, e.send("POST", setupPath, setupBody("lab-02", `{}`, true)), 201)
	adErr(t, e.send("PUT", setupPath+"/"+uItoa(int64(other["id"].(float64))), setupBody("lab-01", `{}`, true)), 409, "conflict")
	if n := len(e.audits("setup.create")); n != 2 {
		t.Errorf("audit rows = %d", n)
	}
	if n := len(e.audits("setup.update")); n != 0 {
		t.Errorf("failed update wrote an audit row")
	}
}

func TestSET1_UpdateSetup(t *testing.T) {
	e := newAdEnv(t)
	id := e.setup("lab-01", true, map[string]string{"old": "1"})
	path := setupPath + "/" + uItoa(id)
	body := `{"name":"renamed","labels":{"new":"2"},"details":{"x":[1]},"enabled":false,"resource_name_override":"pool"}`
	s := uWantOK(t, e.send("PUT", path, body), 200)
	if s["name"] != "renamed" || s["state"] != "disabled" || adObjOf(t, s["labels"])["new"] != "2" || len(adObjOf(t, s["labels"])) != 1 {
		t.Errorf("setup = %v", s)
	}
	if ad := adObjOf(t, s["admin"]); ad["enabled"] != false || ad["resource_name"] != "pool" {
		t.Errorf("admin = %v", ad)
	}
	rows := e.audits("setup.update")
	if len(rows) != 1 || !strings.Contains(rows[0].BeforeJSON, `"name":"lab-01"`) || !strings.Contains(rows[0].AfterJSON, `"name":"renamed"`) {
		t.Errorf("audit = %+v", rows)
	}
	adErr(t, e.send("PUT", setupPath+"/9999", body), 404, "not_found")
	adErr(t, e.send("PUT", setupPath+"/0", body), 400, "validation")
}

func TestSET5_HeldSetupCannotBeDisabledOrDeleted(t *testing.T) {
	e := newAdEnv(t)
	d := e.dev("dev")
	id := e.setup("lab-01", true, nil)
	h := e.hold(id, d.user.ID)
	path := setupPath + "/" + uItoa(id)

	adErr(t, e.send("PUT", path, setupBody("lab-01", `{}`, false)), 409, "has_active_hold")
	adErr(t, e.send("DELETE", path, ""), 409, "has_active_hold")
	// Editing a held setup without disabling it is allowed, and the response shows the hold.
	s := uWantOK(t, e.send("PUT", path, setupBody("lab-01", `{"edited":true}`, true)), 200)
	if s["state"] != "held" {
		t.Errorf("state = %v", s["state"])
	}
	holder := adObjOf(t, s["holder"])
	uWantKeys(t, "holder", holder, "user_id", "hold_id", "name", "balance_tokens", "overdrawn", "is_you")
	if uNum(t, holder, "hold_id") != float64(h.ID) || holder["name"] != "User dev" || uNum(t, holder, "balance_tokens") != 300 || holder["is_you"] != false || s["held_since"] == nil {
		t.Errorf("holder = %v", holder)
	}
	if m := adObjOf(t, s["metering"]); m["active"] != false || m["expected_start_at"] != nil || m["reason"] != nil {
		t.Errorf("metering = %v", m) // no poll has seen the hold: no reason
	}
	// Once a poll has seen it, the reason is the policy's own words.
	e.policy.SetDefault(hourglass.Decision{Meter: false, Reason: "waiting for the policy's own condition"})
	if err := e.col.RunOnce(uBg); err != nil {
		t.Fatal(err)
	}
	s = uWantOK(t, e.send("PUT", path, setupBody("lab-01", `{"edited":true}`, true)), 200)
	if m := adObjOf(t, s["metering"]); m["active"] != false || m["reason"] != "waiting for the policy's own condition" {
		t.Errorf("metering after a poll = %v", m)
	}
	// After the hold ends the setup may be disabled, but not deleted: it has history.
	if err := e.st.ReleaseHold(uBg, h.ID, &d.user.ID, "", e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	uWantOK(t, e.send("PUT", path, setupBody("lab-01", `{}`, false)), 200)
	_, msg := adErr(t, e.send("DELETE", path, ""), 409, "conflict")
	if !strings.Contains(msg, "disable") {
		t.Errorf("message does not point to disabling: %q", msg)
	}
	if _, err := e.st.GetSetup(uBg, id); err != nil {
		t.Errorf("setup vanished: %v", err)
	}
	if n := len(e.audits("setup.delete")); n != 0 {
		t.Errorf("refused deletes wrote %d audit rows", n)
	}
}

func TestSET1_DeleteSetup(t *testing.T) {
	e := newAdEnv(t)
	id := e.setup("lab-01", true, map[string]string{"k": "v"})
	rec := e.send("DELETE", setupPath+"/"+uItoa(id), "")
	if rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("delete: %d %q", rec.Code, rec.Body)
	}
	if _, err := e.st.GetSetup(uBg, id); err == nil {
		t.Error("setup still exists")
	}
	rows := e.audits("setup.delete")
	if len(rows) != 1 || rows[0].AfterJSON != "" || !strings.Contains(rows[0].BeforeJSON, "lab-01") {
		t.Errorf("audit = %+v", rows)
	}
	adErr(t, e.send("DELETE", setupPath+"/"+uItoa(id), ""), 404, "not_found")
}

func TestSET1_AdminViewShowsDeployedAndLastSeen(t *testing.T) {
	e := newAdEnv(t)
	id := e.setup("lab-01", true, nil)
	_ = id
	e.prov.SetAllocations(hourglassAlloc("lab-01"))
	if err := e.col.RunOnce(uBg); err != nil {
		t.Fatal(err)
	}
	s := uWantOK(t, e.send("PUT", setupPath+"/"+uItoa(id), setupBody("lab-01", `{}`, true)), 200)
	if s["deployed"] != true || adObjOf(t, s["admin"])["last_seen_at"] == nil {
		t.Errorf("setup = %v", s)
	}
	if uNum(t, adObjOf(t, s["cost_per_hour_tokens"]), "now") != 2 { // 0.8 pressure = 2x, weight 1
		t.Errorf("cost = %v", s["cost_per_hour_tokens"])
	}
}
