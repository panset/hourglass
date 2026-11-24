// SPDX-License-Identifier: MIT

package api

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/panset/hourglass/internal/settings"
)

// WGT-17: the reported_weights setting of docs/design/reported-weights.md D1,
// over HTTP.

const rwDefault = `{"enabled":false,"min":0,"max":null,"max_age":"10m"}`

// withRW is fullSettings plus a reported_weights value (raw JSON).
func withRW(rw string) string {
	s := fullSettings("")
	return s[:len(s)-1] + `,"reported_weights":` + rw + `}`
}

func rwOf(t *testing.T, b ujobj) ujobj {
	t.Helper()
	return adObjOf(t, adObjOf(t, b["settings"])["reported_weights"])
}

func jsonObj(t *testing.T, s string) ujobj {
	t.Helper()
	var m ujobj
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("%s: %v", s, err)
	}
	return m
}

func TestWGT17_GetAlwaysReturnsTheKeyWithDefaults(t *testing.T) {
	e := newAdEnv(t)
	if got := rwOf(t, uWantOK(t, e.get(settingsPath), 200)); !reflect.DeepEqual(got, jsonObj(t, rwDefault)) {
		t.Errorf("fresh GET: %v", got)
	}
	// After a PUT without the key, GET and the PUT response still carry it.
	put := uWantOK(t, e.send("PUT", settingsPath, fullSettings("")), 200)
	if got := rwOf(t, put); !reflect.DeepEqual(got, jsonObj(t, rwDefault)) {
		t.Errorf("PUT response: %v", got)
	}
	if got := rwOf(t, uWantOK(t, e.get(settingsPath), 200)); !reflect.DeepEqual(got, jsonObj(t, rwDefault)) {
		t.Errorf("GET after PUT: %v", got)
	}
}

// Every row of the D1 table: each bound accepted, each rejection with its
// field, and enabled true without max refused.
func TestWGT17_EveryRuleOfTheD1Table(t *testing.T) {
	accept := []struct{ name, in, out string }{
		{"the defaults", rwDefault, rwDefault},
		{"on with a maximum", `{"enabled":true,"min":0,"max":20,"max_age":"10m"}`, ""},
		{"min equal to max", `{"enabled":true,"min":20,"max":20,"max_age":"10m"}`, ""},
		{"max at 1e12", `{"enabled":true,"min":0,"max":1e12,"max_age":"10m"}`, `{"enabled":true,"min":0,"max":1000000000000,"max_age":"10m"}`},
		{"max just above 0", `{"enabled":true,"min":0,"max":1e-9,"max_age":"10m"}`, ""},
		{"min without a maximum while off", `{"enabled":false,"min":5,"max":null,"max_age":"10m"}`, ""},
		{"min of 0 written as -0", `{"enabled":false,"min":-0,"max":null,"max_age":"10m"}`, rwDefault},
		{"max_age at 1m", `{"enabled":false,"min":0,"max":null,"max_age":"1m"}`, ""},
		{"max_age at 24h", `{"enabled":false,"min":0,"max":null,"max_age":"24h"}`, ""},
		{"max_age as 60s", `{"enabled":false,"min":0,"max":null,"max_age":"60s"}`, `{"enabled":false,"min":0,"max":null,"max_age":"1m"}`},
		{"max_age as 1440m", `{"enabled":false,"min":0,"max":null,"max_age":"1440m"}`, `{"enabled":false,"min":0,"max":null,"max_age":"24h"}`},
		{"off with limits kept", `{"enabled":false,"min":1,"max":20,"max_age":"30m"}`, ""},
	}
	for _, c := range accept {
		t.Run("accept "+c.name, func(t *testing.T) {
			e := newAdEnv(t)
			want := c.out
			if want == "" {
				want = c.in
			}
			put := uWantOK(t, e.send("PUT", settingsPath, withRW(c.in)), 200)
			if got := rwOf(t, put); !reflect.DeepEqual(got, jsonObj(t, want)) {
				t.Errorf("PUT response %v, want %s", got, want)
			}
			if got := rwOf(t, uWantOK(t, e.get(settingsPath), 200)); !reflect.DeepEqual(got, jsonObj(t, want)) {
				t.Errorf("GET %v, want %s", got, want)
			}
		})
	}

	reject := []struct{ name, in, field string }{
		{"on without a maximum", `{"enabled":true,"min":0,"max":null,"max_age":"10m"}`, "reported_weights.enabled"},
		{"min below 0", `{"enabled":false,"min":-0.5,"max":null,"max_age":"10m"}`, "reported_weights.min"},
		{"min above max", `{"enabled":true,"min":21,"max":20,"max_age":"10m"}`, "reported_weights.min"},
		{"min too large for a float", `{"enabled":false,"min":1e400,"max":null,"max_age":"10m"}`, "reported_weights.min"},
		{"min null", `{"enabled":false,"min":null,"max":null,"max_age":"10m"}`, "reported_weights.min"},
		{"min a string", `{"enabled":false,"min":"0","max":null,"max_age":"10m"}`, "reported_weights.min"},
		{"max of 0", `{"enabled":false,"min":0,"max":0,"max_age":"10m"}`, "reported_weights.max"},
		{"max below 0", `{"enabled":false,"min":0,"max":-1,"max_age":"10m"}`, "reported_weights.max"},
		{"max above 1e12", `{"enabled":true,"min":0,"max":1.0000000001e12,"max_age":"10m"}`, "reported_weights.max"},
		{"max too large for a float", `{"enabled":true,"min":0,"max":1e400,"max_age":"10m"}`, "reported_weights.max"},
		{"max a string", `{"enabled":true,"min":0,"max":"20","max_age":"10m"}`, "reported_weights.max"},
		{"max_age below 1m", `{"enabled":false,"min":0,"max":null,"max_age":"59s"}`, "reported_weights.max_age"},
		{"max_age above 24h", `{"enabled":false,"min":0,"max":null,"max_age":"24h0m1s"}`, "reported_weights.max_age"},
		{"max_age not a duration", `{"enabled":false,"min":0,"max":null,"max_age":"soon"}`, "reported_weights.max_age"},
		{"max_age a number", `{"enabled":false,"min":0,"max":null,"max_age":600}`, "reported_weights.max_age"},
		{"max_age null", `{"enabled":false,"min":0,"max":null,"max_age":null}`, "reported_weights.max_age"},
		{"enabled a string", `{"enabled":"yes","min":0,"max":20,"max_age":"10m"}`, "reported_weights.enabled"},
		{"enabled null", `{"enabled":null,"min":0,"max":20,"max_age":"10m"}`, "reported_weights.enabled"},
		{"an unknown field", `{"enabled":false,"min":0,"max":null,"max_age":"10m","floor":1}`, "reported_weights.floor"},
		{"null", `null`, "reported_weights"},
		{"not an object", `[1]`, "reported_weights"},
	}
	for _, c := range reject {
		t.Run("reject "+c.name, func(t *testing.T) {
			e := newAdEnv(t)
			// A stored non-default value that a refused PUT must not touch.
			uWantOK(t, e.send("PUT", settingsPath, withRW(`{"enabled":true,"min":1,"max":5,"max_age":"15m"}`)), 200)
			f, msg := adErr(t, e.send("PUT", settingsPath, withRW(c.in)), 400, "validation")
			if f != c.field || msg == "" {
				t.Errorf("field %q message %q, want field %q", f, msg, c.field)
			}
			if got := rwOf(t, uWantOK(t, e.get(settingsPath), 200)); !reflect.DeepEqual(got, jsonObj(t, `{"enabled":true,"min":1,"max":5,"max_age":"15m"}`)) {
				t.Errorf("a refused PUT changed the setting: %v", got)
			}
			if n := len(e.audits("settings.update")); n != 1 {
				t.Errorf("audit rows = %d, want 1 (the setup PUT only)", n)
			}
		})
	}
}

// The message for switching on without a maximum is the one the Settings page
// shows (C3): it says what is missing.
func TestWGT17_OnWithoutMaximumMessage(t *testing.T) {
	e := newAdEnv(t)
	_, msg := adErr(t, e.send("PUT", settingsPath, withRW(`{"enabled":true,"min":0,"max":null,"max_age":"10m"}`)), 400, "validation")
	if !strings.Contains(msg, "max") || !strings.Contains(msg, "enabled") {
		t.Errorf("message %q", msg)
	}
}

// When the key is present, all four fields are required.
func TestWGT17_AllFourFieldsRequiredWhenPresent(t *testing.T) {
	full := map[string]string{"enabled": "false", "min": "0", "max": "null", "max_age": `"10m"`}
	for _, missing := range []string{"enabled", "min", "max", "max_age"} {
		var parts []string
		for _, f := range []string{"enabled", "min", "max", "max_age"} {
			if f != missing {
				parts = append(parts, `"`+f+`":`+full[f])
			}
		}
		e := newAdEnv(t)
		f, msg := adErr(t, e.send("PUT", settingsPath, withRW("{"+strings.Join(parts, ",")+"}")), 400, "validation")
		if f != "reported_weights."+missing || !strings.Contains(msg, "required") {
			t.Errorf("without %s: field %q message %q", missing, f, msg)
		}
	}
	e := newAdEnv(t)
	if f, _ := adErr(t, e.send("PUT", settingsPath, withRW(`{}`)), 400, "validation"); !strings.HasPrefix(f, "reported_weights.") {
		t.Errorf("empty object: field %q", f)
	}
}

// A change is audited like the other keys, with the whole object as the before
// and the after value; an unchanged object is not in the audit entry.
func TestWGT17_ChangeAuditedWithWholeObjects(t *testing.T) {
	e := newAdEnv(t)
	on := `{"enabled":true,"min":0.5,"max":20,"max_age":"15m"}`
	uWantOK(t, e.send("PUT", settingsPath, withRW(on)), 200)
	rows := e.audits("settings.update")
	if len(rows) != 1 {
		t.Fatalf("audit rows = %d", len(rows))
	}
	before, after := jsonObj(t, rows[0].BeforeJSON), jsonObj(t, rows[0].AfterJSON)
	if !reflect.DeepEqual(before[settings.KeyReportedWeights], jsonObj(t, rwDefault)) ||
		!reflect.DeepEqual(after[settings.KeyReportedWeights], jsonObj(t, on)) {
		t.Errorf("audit before %s after %s", rows[0].BeforeJSON, rows[0].AfterJSON)
	}
	if rows[0].ActorID == nil || *rows[0].ActorID != e.admin.ID || rows[0].Target != "settings" {
		t.Errorf("audit row %+v", rows[0])
	}

	// Switching off: before and after are again the whole objects.
	off := `{"enabled":false,"min":0.5,"max":20,"max_age":"15m"}`
	uWantOK(t, e.send("PUT", settingsPath, withRW(off)), 200)
	rows = e.audits("settings.update")
	if len(rows) != 2 {
		t.Fatalf("audit rows = %d", len(rows))
	}
	before, after = jsonObj(t, rows[0].BeforeJSON), jsonObj(t, rows[0].AfterJSON)
	if !reflect.DeepEqual(before, ujobj{"reported_weights": jsonObj(t, on)}) || !reflect.DeepEqual(after, ujobj{"reported_weights": jsonObj(t, off)}) {
		t.Errorf("second audit before %s after %s", rows[0].BeforeJSON, rows[0].AfterJSON)
	}

	// The same object again, with another key changed: only that key is audited.
	uWantOK(t, e.send("PUT", settingsPath, strings.Replace(withRW(off), `"max_holds_per_user":3`, `"max_holds_per_user":4`, 1)), 200)
	rows = e.audits("settings.update")
	if len(rows) != 3 || strings.Contains(rows[0].AfterJSON, "reported_weights") || strings.Contains(rows[0].BeforeJSON, "reported_weights") {
		t.Errorf("unchanged object audited: %+v", rows[0])
	}
}

// A PUT with only the eight keys of v1.1.0 leaves a stored non-default value
// unchanged, and answers as v1.1.0 did: 200, the eight keys as sent, the
// runtime block, plus the added key.
func TestWGT17_PutWithoutTheKeyLeavesItUnchanged(t *testing.T) {
	e := newAdEnv(t)
	stored := `{"enabled":true,"min":0.25,"max":20,"max_age":"30m"}`
	uWantOK(t, e.send("PUT", settingsPath, withRW(stored)), 200)
	audits := len(e.audits("settings.update"))

	// The same eight values: nothing changes, nothing is audited.
	put := uWantOK(t, e.send("PUT", settingsPath, fullSettings("")), 200)
	if got := rwOf(t, put); !reflect.DeepEqual(got, jsonObj(t, stored)) {
		t.Fatalf("PUT without the key reset it: %v", got)
	}
	if n := len(e.audits("settings.update")); n != audits {
		t.Errorf("a no-op PUT wrote %d audit rows", n-audits)
	}

	// Other values changed: they are saved and audited, the key is not.
	body := fullSettings(`max_holds_per_user=5|enforcement=false`)
	put = uWantOK(t, e.send("PUT", settingsPath, body), 200)
	uWantKeys(t, "response", put, "settings", "runtime")
	got := adObjOf(t, put["settings"])
	if !reflect.DeepEqual(got["reported_weights"], jsonObj(t, stored)) {
		t.Errorf("PUT response reported_weights %v", got["reported_weights"])
	}
	sent := jsonObj(t, body)
	delete(got, "reported_weights")
	if !reflect.DeepEqual(got, sent) {
		t.Errorf("the eight keys answered %v, sent %v", got, sent)
	}
	if adObjOf(t, put["runtime"])["provider"] != "fake-provider" {
		t.Errorf("runtime = %v", put["runtime"])
	}
	if g := rwOf(t, uWantOK(t, e.get(settingsPath), 200)); !reflect.DeepEqual(g, jsonObj(t, stored)) {
		t.Errorf("GET after PUT: %v", g)
	}
	rows := e.audits("settings.update")
	if len(rows) != audits+1 || strings.Contains(rows[0].AfterJSON, "reported_weights") ||
		!strings.Contains(rows[0].AfterJSON, "max_holds_per_user") {
		t.Errorf("audit after the eight-key PUT: %+v", rows[0])
	}
	// Each of the eight keys is still required.
	if f, _ := adErr(t, e.send("PUT", settingsPath, `{"reported_weights":`+stored+`}`), 400, "validation"); f != "default_weight" {
		t.Errorf("a body with only reported_weights: field %q", f)
	}
}

// The setting is read back by the store-level loader the later milestones use.
func TestWGT17_StoredValueLoads(t *testing.T) {
	e := newAdEnv(t)
	uWantOK(t, e.send("PUT", settingsPath, withRW(`{"enabled":true,"min":0,"max":20,"max_age":"15m"}`)), 200)
	cfg, err := settings.Load(uBg, e.st)
	if err != nil {
		t.Fatal(err)
	}
	rw := cfg.ReportedWeights
	if !rw.Enabled || rw.Min != 0 || rw.Max == nil || *rw.Max != 20 || rw.MaxAge.String() != "15m0s" {
		t.Errorf("loaded %+v", rw)
	}
}

// A min written as -0 is stored and returned as 0, not as the text -0
// (decoded floats compare equal, so the raw text is what is checked).
func TestWGT17_NegativeZeroMinIsStoredAndReturnedAsZero(t *testing.T) {
	e := newAdEnv(t)
	put := e.send("PUT", settingsPath, withRW(`{"enabled":false,"min":-0,"max":null,"max_age":"10m"}`))
	if put.Code != 200 {
		t.Fatalf("PUT %d: %s", put.Code, put.Body)
	}
	get := e.get(settingsPath)
	for name, body := range map[string]string{"PUT": put.Body.String(), "GET": get.Body.String()} {
		if strings.Contains(body, `"min":-0`) || !strings.Contains(body, `"min":0,`) {
			t.Errorf("%s body does not carry min 0: %s", name, body)
		}
	}
}
