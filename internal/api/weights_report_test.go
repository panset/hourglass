// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/internal/reported"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// Tests of the weights report and the state's weight fields over HTTP
// (docs/design/reported-weights.md, B1 to B4, D3; WGT-7 to WGT-11, WGT-13,
// WGT-21).

const weightsPath = "/api/v1/integration/weights"

// wEnv is a backend with a service account holding every scope, an admin,
// a rate rule tier=gold -> 2, and three setups: lab-01 and lab-02 match the
// rule, lab-03 has no labels (default weight 1). Reported weights are on with
// min 0, max 20 and max_age 10m.
type wEnv struct {
	*backend
	svc svcAccount
	adm sess
}

var wStart = time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)

func newWEnv(t *testing.T) *wEnv {
	t.Helper()
	return wEnvOn(t, newBackend(t, backendOpts{start: wStart,
		runtime: Runtime{Provider: "reported", ReportedProviderMaxAge: 10 * time.Minute}}), true)
}

func wEnvOn(t *testing.T, b *backend, seed bool) *wEnv {
	t.Helper()
	e := &wEnv{backend: b, adm: b.newUser(fmt.Sprintf("boss-%d", time.Now().UnixNano()), store.StatusActive, store.RoleAdmin)}
	if !seed {
		return e
	}
	e.svc = b.mintServiceAccount("weigher", allScopes()...)
	ctx := context.Background()
	if err := b.st.ReplaceRules(ctx, "rate", []store.Rule{{Selector: "tier=gold", Value: 2}}, nil, b.clk.Now()); err != nil {
		t.Fatal(err)
	}
	for _, s := range []struct {
		name   string
		labels map[string]string
	}{{"lab-01", map[string]string{"tier": "gold"}}, {"lab-02", map[string]string{"tier": "gold"}}, {"lab-03", map[string]string{}}} {
		if _, err := b.st.CreateSetup(ctx, store.Setup{Name: s.name, Enabled: true, Labels: s.labels, DetailsJSON: `{}`}); err != nil {
			t.Fatal(err)
		}
	}
	e.setting(`{"enabled":true,"min":0,"max":20,"max_age":"10m"}`)
	return e
}

// setting stores the reported_weights setting as an admin's PUT would.
func (e *wEnv) setting(v string) {
	e.t.Helper()
	if err := e.st.PutSetting(context.Background(), "reported_weights", v, nil, e.clk.Now()); err != nil {
		e.t.Fatal(err)
	}
}

func (e *wEnv) put(body string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.serveService("PUT", weightsPath, e.svc.token, body)
}

func (e *wEnv) ok(body string) map[string]any {
	e.t.Helper()
	rec := e.put(body)
	if rec.Code != http.StatusOK {
		e.t.Fatalf("PUT weights %s: %d %s", body, rec.Code, rec.Body)
	}
	return decodeMap(e.t, rec)
}

// refused asserts an error answer and returns its message and field.
func (e *wEnv) refused(rec *httptest.ResponseRecorder, status int, code string) (msg, field string) {
	e.t.Helper()
	if rec.Code != status {
		e.t.Fatalf("status %d, want %d: %s", rec.Code, status, rec.Body)
	}
	eo, _ := decodeMap(e.t, rec)["error"].(map[string]any)
	if eo["code"] != code {
		e.t.Fatalf("code %v, want %s: %s", eo["code"], code, rec.Body)
	}
	msg, _ = eo["message"].(string)
	field, _ = eo["field"].(string)
	return msg, field
}

type storedWeights struct {
	hdr store.ReportedHeader
	set []store.ReportedWeight
	err error
}

func (e *wEnv) stored() storedWeights {
	e.t.Helper()
	h, s, err := e.st.GetReportedWeights(context.Background())
	return storedWeights{h, s, err}
}

func (e *wEnv) state() (map[string]any, string) {
	e.t.Helper()
	rec := e.serveService("GET", statePath, e.svc.token, "")
	if rec.Code != http.StatusOK {
		e.t.Fatalf("state: %d %s", rec.Code, rec.Body)
	}
	return decodeMap(e.t, rec), rec.Body.String()
}

// weightOf is the weight object of a setup in a state response.
func weightOf(t *testing.T, m map[string]any, name string) map[string]any {
	t.Helper()
	for _, r := range m["setups"].([]any) {
		if row := r.(map[string]any); row["name"] == name {
			return row["weight"].(map[string]any)
		}
	}
	t.Fatalf("no setup %q in the state", name)
	return nil
}

// wantWeight checks a setup's weight object: applied, source, reported (nil
// for null), not_applied ("" for null) and attributes.
func wantWeight(t *testing.T, m map[string]any, name string, applied float64, source string, rep any, notApplied string, attrs map[string]any) {
	t.Helper()
	w := weightOf(t, m, name)
	var na any
	if notApplied != "" {
		na = notApplied
	}
	if attrs == nil {
		attrs = map[string]any{}
	}
	if w["applied"] != applied || w["source"] != source || w["reported"] != rep || w["not_applied"] != na ||
		!reflect.DeepEqual(w["attributes"], attrs) {
		t.Errorf("%s: weight = %v, want applied %v source %s reported %v not_applied %v attributes %v",
			name, w, applied, source, rep, na, attrs)
	}
}

func (e *wEnv) setupID(name string) int64 {
	e.t.Helper()
	ss, err := e.st.ListSetups(context.Background(), false)
	if err != nil {
		e.t.Fatal(err)
	}
	for _, s := range ss {
		if s.Name == name {
			return s.ID
		}
	}
	e.t.Fatalf("no setup %q", name)
	return 0
}

// rename renames a setup through the admin route, keeping its labels.
func (e *wEnv) rename(from, to string, labels string) {
	e.t.Helper()
	rec := e.serve("PUT", fmt.Sprintf("/api/v1/admin/setups/%d", e.setupID(from)), &e.adm, true,
		`{"name":"`+to+`","labels":`+labels+`,"details":{},"enabled":true}`)
	if rec.Code != http.StatusOK {
		e.t.Fatalf("rename %s -> %s: %d %s", from, to, rec.Code, rec.Body)
	}
}

// ---- WGT-7 ----

// The whole set is replaced; observed_at is optional and defaults to the
// receipt time; the response is {observed_at, received_at, expires_at,
// accepted, unknown}.
func TestWGT7_ReportReplacesTheWholeSet(t *testing.T) {
	e := newWEnv(t)
	m := e.ok(`{"weights":[{"setup":"lab-01","weight":1.5},{"setup":"lab-02","weight":2.5,"attributes":{"memory-gb":"512"}}]}`)
	want := map[string]any{"observed_at": "2026-11-23T10:00:00.000Z", "received_at": "2026-11-23T10:00:00.000Z",
		"expires_at": "2026-11-23T10:10:00.000Z", "accepted": float64(2), "unknown": []any{}}
	if !reflect.DeepEqual(m, want) {
		t.Errorf("response = %v, want %v", m, want)
	}
	s := e.stored()
	if s.err != nil || len(s.set) != 2 || s.hdr.ServiceAccount != "weigher" || !s.hdr.ObservedAt.Equal(wStart) {
		t.Fatalf("stored = %+v", s)
	}
	st, _ := e.state()
	wantWeight(t, st, "lab-01", 1.5, "reported", 1.5, "", nil)
	wantWeight(t, st, "lab-02", 2.5, "reported", 2.5, "", map[string]any{"memory-gb": "512"})

	// The next report is the complete set: lab-01 is no longer reported.
	e.clk.Advance(time.Minute)
	e.ok(`{"observed_at":"2026-11-23T10:00:30Z","weights":[{"setup":"lab-02","weight":3}]}`)
	s = e.stored()
	if len(s.set) != 1 || s.set[0].SetupName != "lab-02" || s.set[0].Weight != 3 || len(s.set[0].Attributes) != 0 {
		t.Errorf("stored after replace = %+v", s.set)
	}
	st, _ = e.state()
	wantWeight(t, st, "lab-01", 2, "rule", nil, "", nil)
	wantWeight(t, st, "lab-02", 3, "reported", float64(3), "", nil)

	// An empty list is valid: no setup has a reported weight.
	m = e.ok(`{"weights":[]}`)
	if m["accepted"] != float64(0) || len(m["unknown"].([]any)) != 0 {
		t.Errorf("empty report response = %v", m)
	}
	if s = e.stored(); s.err != nil || len(s.set) != 0 {
		t.Errorf("stored after an empty report = %+v", s)
	}
	st, _ = e.state()
	wantWeight(t, st, "lab-02", 2, "rule", nil, "", nil)
}

// Readers never see part of one report and part of another.
func TestWGT7_ReadersNeverSeeAMixOfTwoReports(t *testing.T) {
	e := newWEnv(t)
	var a, b strings.Builder
	a.WriteString(`{"weights":[`)
	b.WriteString(`{"weights":[`)
	for i, n := range []string{"lab-01", "lab-02", "lab-03"} {
		if i > 0 {
			a.WriteString(",")
			b.WriteString(",")
		}
		fmt.Fprintf(&a, `{"setup":%q,"weight":4}`, n)
		fmt.Fprintf(&b, `{"setup":%q,"weight":7}`, n)
	}
	a.WriteString("]}")
	b.WriteString("]}")
	e.ok(a.String())
	var wg sync.WaitGroup
	errs := make(chan string, 100)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 40; i++ {
			body := a.String()
			if i%2 == 1 {
				body = b.String()
			}
			if rec := e.serveService("PUT", weightsPath, e.svc.token, body); rec.Code != 200 {
				errs <- fmt.Sprintf("put: %d %s", rec.Code, rec.Body)
			}
		}
	}()
	for i := 0; i < 40; i++ {
		rec := e.serveService("GET", statePath, e.svc.token, "")
		m := decodeMap(t, rec)
		seen := map[any]bool{}
		for _, r := range m["setups"].([]any) {
			seen[r.(map[string]any)["weight"].(map[string]any)["reported"]] = true
		}
		if len(seen) != 1 {
			errs <- fmt.Sprintf("a read saw reported weights %v", seen)
		}
	}
	wg.Wait()
	close(errs)
	for s := range errs {
		t.Error(s)
	}
}

// Ordering and clock skew are those of the other reports: older is 409
// stale_report and changes nothing, equal replaces, exactly 2 minutes ahead
// is accepted and more is 400 validation.
func TestWGT7_OrderingAndClockSkew(t *testing.T) {
	e := newWEnv(t)
	e.ok(`{"observed_at":"2026-11-23T09:59:00Z","weights":[{"setup":"lab-01","weight":1}]}`)
	before := e.stored()
	if msg, _ := e.refused(e.put(`{"observed_at":"2026-11-23T09:58:59Z","weights":[{"setup":"lab-01","weight":9}]}`),
		http.StatusConflict, apierr.StaleReport); msg == "" {
		t.Error("stale_report has no message")
	}
	if after := e.stored(); !reflect.DeepEqual(after, before) {
		t.Errorf("a stale report changed the stored one: %+v", after)
	}
	e.ok(`{"observed_at":"2026-11-23T09:59:00Z","weights":[{"setup":"lab-01","weight":2.5}]}`)
	if s := e.stored(); s.set[0].Weight != 2.5 {
		t.Errorf("an equal observed_at did not replace: %+v", s.set)
	}
	before = e.stored()
	e.refused(e.put(`{"observed_at":"2026-11-23T10:02:00.001Z","weights":[{"setup":"lab-01","weight":3}]}`),
		http.StatusBadRequest, apierr.Validation)
	if after := e.stored(); !reflect.DeepEqual(after, before) {
		t.Errorf("a report from the future changed the stored one: %+v", after)
	}
	m := e.ok(`{"observed_at":"2026-11-23T10:02:00Z","weights":[{"setup":"lab-01","weight":3}]}`)
	if m["observed_at"] != "2026-11-23T10:02:00.000Z" || m["expires_at"] != "2026-11-23T10:12:00.000Z" {
		t.Errorf("response = %v", m)
	}
	e.refused(e.put(`{"observed_at":"yesterday","weights":[]}`), http.StatusBadRequest, apierr.Validation)
}

// The weights report's ordering is independent of the deployments report's.
func TestWGT7_OrderingIndependentOfTheDeploymentsReport(t *testing.T) {
	e := newWEnv(t)
	if rec := e.serveService("PUT", deploymentsPath, e.svc.token, `{"observed_at":"2026-11-23T10:00:00Z","deployments":[]}`); rec.Code != 200 {
		t.Fatalf("deployments: %d %s", rec.Code, rec.Body)
	}
	// Older than the deployments report: accepted.
	e.ok(`{"observed_at":"2026-11-23T09:55:00Z","weights":[{"setup":"lab-01","weight":1}]}`)
	// The deployments report is still ordered against its own.
	if rec := e.serveService("PUT", deploymentsPath, e.svc.token, `{"observed_at":"2026-11-23T09:57:00Z","deployments":[]}`); rec.Code != http.StatusConflict {
		t.Errorf("an older deployments report: %d %s", rec.Code, rec.Body)
	}
	// Same observed_at as the deployments report: accepted.
	e.ok(`{"observed_at":"2026-11-23T10:00:00Z","weights":[{"setup":"lab-01","weight":2}]}`)
	e.refused(e.put(`{"observed_at":"2026-11-23T09:59:00Z","weights":[]}`), http.StatusConflict, apierr.StaleReport)
	st, _ := e.state()
	if d := st["deployments"].(map[string]any); d["observed_at"] != "2026-11-23T10:00:00.000Z" {
		t.Errorf("deployments = %v", d)
	}
	if w := st["weights"].(map[string]any); w["observed_at"] != "2026-11-23T10:00:00.000Z" {
		t.Errorf("weights = %v", w)
	}
}

// The stored report survives a restart: a backend opened on the same
// database file serves it.
func TestWGT7_ReportSurvivesARestart(t *testing.T) {
	e := newWEnv(t)
	e.ok(`{"observed_at":"2026-11-23T09:59:00Z","weights":[{"setup":"lab-01","weight":1.15,"attributes":{"tier":"gold"}}]}`)
	path := e.dbPath
	if err := e.st.Close(); err != nil {
		t.Fatal(err)
	}
	b2 := newBackend(t, backendOpts{start: wStart, dbPath: path})
	e2 := &wEnv{backend: b2, svc: e.svc}
	st, _ := e2.state()
	wantWeight(t, st, "lab-01", 1.15, "reported", 1.15, "", map[string]any{"tier": "gold"})
	if w := st["weights"].(map[string]any); w["observed_at"] != "2026-11-23T09:59:00.000Z" || w["expires_at"] != "2026-11-23T10:09:00.000Z" {
		t.Errorf("weights = %v", w)
	}
	// Ordering continues against the stored report.
	e2.refused(e2.put(`{"observed_at":"2026-11-23T09:58:00Z","weights":[]}`), http.StatusConflict, apierr.StaleReport)
}

// B3: a report never triggers a poll, a debit or an event.
func TestWGT7_ReportNeverPollsDebitsOrEmits(t *testing.T) {
	e := newWEnv(t)
	ctx := context.Background()
	dev := e.newUser("holder", store.StatusActive, store.RoleDeveloper)
	h, err := e.st.CreateHold(ctx, e.setupID("lab-01"), dev.user.ID, e.clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	before, _ := e.st.GetUser(ctx, dev.user.ID)
	ledgerBefore, _ := e.st.ListLedger(ctx, dev.user.ID, "2026-11")
	e.ok(`{"weights":[{"setup":"lab-01","weight":5}]}`)
	e.put(`{"weights":[{"setup":"lab-01","weight":50}]}`) // refused
	runs, _ := e.st.ListCollectorRuns(ctx, 10)
	after, _ := e.st.GetUser(ctx, dev.user.ID)
	samples, _ := e.st.ListUsageSamples(ctx, h.ID)
	deliveries, _ := e.st.ListEventDeliveries(ctx, 10)
	ledger, _ := e.st.ListLedger(ctx, dev.user.ID, "2026-11")
	if len(runs) != 0 || e.col.LastPoll().OK || after.BalanceMT != before.BalanceMT || len(samples) != 0 ||
		len(deliveries) != 0 || len(e.hook.Events()) != 0 || len(ledger) != len(ledgerBefore) {
		t.Errorf("a report had an effect: runs=%d samples=%d balance %d->%d deliveries=%d events=%d ledger=%d",
			len(runs), len(samples), before.BalanceMT, after.BalanceMT, len(deliveries), len(e.hook.Events()), len(ledger))
	}
}

// ---- WGT-8 ----

// Each clause of WGT-8 refuses the whole report with 400 validation, field
// weights, a message with the position and the name when there is one;
// nothing is stored and the previous report stays in force.
func TestWGT8_EachInvalidEntryRefusesTheWholeReport(t *testing.T) {
	long := strings.Repeat("v", 64)
	var many []string
	for i := 0; i < 17; i++ {
		many = append(many, fmt.Sprintf(`"k%02d":"v"`, i))
	}
	cases := []struct {
		name   string
		entry  string // the entry at position 1 (position 0 is a valid lab-02)
		setup  string // the name the message must give, "" for none
		reason string // a part of the reason
	}{
		{"weight missing", `{"setup":"lab-03"}`, "lab-03", "weight is required"},
		{"weight a string", `{"setup":"lab-03","weight":"1.15"}`, "lab-03", "weight must be a JSON number, not a string"},
		{"weight null", `{"setup":"lab-03","weight":null}`, "lab-03", "weight must be a JSON number, not null"},
		{"weight a boolean", `{"setup":"lab-03","weight":true}`, "lab-03", "weight must be a JSON number, not a boolean"},
		{"weight an object", `{"setup":"lab-03","weight":{"v":1}}`, "lab-03", "weight must be a JSON number, not an object"},
		{"weight too large for a float64", `{"setup":"lab-03","weight":1e400}`, "lab-03", "weight 1e400 is not a finite 64-bit number"},
		{"weight too small for a float64", `{"setup":"lab-03","weight":-1e400}`, "lab-03", "weight -1e400 is not a finite 64-bit number"},
		{"weight below the minimum", `{"setup":"lab-03","weight":0.24}`, "lab-03", "weight 0.24 is below the minimum 0.25"},
		{"weight above the maximum", `{"setup":"lab-03","weight":20.001}`, "lab-03", "weight 20.001 is above the maximum 20"},
		{"name missing", `{"weight":1}`, "", "setup must be a non-empty string"},
		{"name not a string", `{"setup":7,"weight":1}`, "", "setup must be a non-empty string"},
		{"name null", `{"setup":null,"weight":1}`, "", "setup must be a non-empty string"},
		{"name empty", `{"setup":"","weight":1}`, "", "setup must be a non-empty string"},
		{"name repeated", `{"setup":"lab-02","weight":1}`, "lab-02", "setup is repeated (first at weights[0])"},
		{"17 attributes", `{"setup":"lab-03","weight":1,"attributes":{` + strings.Join(many, ",") + `}}`, "lab-03", "attributes has 17 entries (at most 16)"},
		{"bad attribute key", `{"setup":"lab-03","weight":1,"attributes":{"Memory GB":"512"}}`, "lab-03", `invalid label key "Memory GB"`},
		{"attribute value of 64 characters", `{"setup":"lab-03","weight":1,"attributes":{"tier":"` + long + `"}}`, "lab-03", "value is 64 characters (at most 63)"},
		{"attribute value not a string", `{"setup":"lab-03","weight":1,"attributes":{"memory-gb":512}}`, "lab-03", `attributes "memory-gb" must be a string`},
		{"attributes not an object", `{"setup":"lab-03","weight":1,"attributes":["a"]}`, "lab-03", "attributes must be an object of strings"},
		{"unknown field", `{"setup":"lab-03","weight":1,"colour":"red"}`, "lab-03", `unknown field "colour"`},
		{"unknown field without a name", `{"weight":1,"colour":"red"}`, "", `unknown field "colour"`},
		{"entry not an object", `"lab-03"`, "", "must be an object"},
	}
	for _, c := range cases {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			e := newWEnv(t)
			e.setting(`{"enabled":true,"min":0.25,"max":20,"max_age":"10m"}`)
			e.ok(`{"observed_at":"2026-11-23T09:59:00Z","weights":[{"setup":"lab-01","weight":1.5}]}`)
			before := e.stored()
			msg, field := e.refused(e.put(`{"weights":[{"setup":"lab-02","weight":1},`+c.entry+`]}`), http.StatusBadRequest, apierr.Validation)
			prefix := "weights[1]: "
			if c.setup != "" {
				prefix = `weights[1] (setup "` + c.setup + `"): `
			}
			if field != "weights" || !strings.HasPrefix(msg, prefix) || !strings.Contains(msg, c.reason) {
				t.Errorf("field %q message %q, want field weights and %q ... %q", field, msg, prefix, c.reason)
			}
			if after := e.stored(); !reflect.DeepEqual(after, before) {
				t.Errorf("the refused report changed the stored one: %+v", after)
			}
			st, _ := e.state()
			wantWeight(t, st, "lab-01", 1.5, "reported", 1.5, "", nil)
			wantWeight(t, st, "lab-02", 2, "rule", nil, "", nil)
		})
	}
}

// The first invalid entry is the one reported.
func TestWGT8_FirstInvalidEntryReported(t *testing.T) {
	e := newWEnv(t)
	msg, _ := e.refused(e.put(`{"weights":[{"setup":"lab-01","weight":1},{"setup":"lab-02","weight":"2"},{"setup":"lab-03","weight":99},{"bad":1}]}`),
		http.StatusBadRequest, apierr.Validation)
	if want := `weights[1] (setup "lab-02"): weight must be a JSON number, not a string`; msg != want {
		t.Errorf("message %q, want %q", msg, want)
	}
	msg, _ = e.refused(e.put(`{"weights":[{"setup":"lab-01","weight":21},{"setup":"lab-02","weight":"2"}]}`),
		http.StatusBadRequest, apierr.Validation)
	if want := `weights[0] (setup "lab-01"): weight 21 is above the maximum 20`; msg != want {
		t.Errorf("message %q, want %q", msg, want)
	}
	if s := e.stored(); !errors.Is(s.err, store.ErrNotFound) {
		t.Errorf("stored = %+v", s)
	}
}

// Both limits are inclusive, and negative zero counts as 0.
func TestWGT8_LimitsInclusiveAndNegativeZero(t *testing.T) {
	e := newWEnv(t)
	e.setting(`{"enabled":true,"min":0.25,"max":20,"max_age":"10m"}`)
	e.ok(`{"weights":[{"setup":"lab-01","weight":0.25},{"setup":"lab-02","weight":20}]}`)
	st, _ := e.state()
	wantWeight(t, st, "lab-01", 0.25, "reported", 0.25, "", nil)
	wantWeight(t, st, "lab-02", 20, "reported", float64(20), "", nil)

	e.setting(`{"enabled":true,"min":0,"max":20,"max_age":"10m"}`)
	e.ok(`{"weights":[{"setup":"lab-01","weight":-0},{"setup":"lab-02","weight":-0.0}]}`)
	st, body := e.state()
	wantWeight(t, st, "lab-01", 0, "reported", float64(0), "", nil)
	wantWeight(t, st, "lab-02", 0, "reported", float64(0), "", nil)
	if strings.Contains(body, ":-0") {
		t.Errorf("negative zero in the state: %s", body)
	}
	if s := e.stored(); s.set[0].Weight != 0 || s.set[1].Weight != 0 {
		t.Errorf("stored = %+v", s.set)
	}
}

// A body that is not valid JSON (bare NaN or Infinity included) gets the
// generic answer and names no entry; a missing, null or non-list weights is
// 400 with field weights; nothing is stored.
func TestWGT8_BodyRefusals(t *testing.T) {
	e := newWEnv(t)
	for _, body := range []string{
		`{"weights":[{"setup":"lab-01","weight":NaN}]}`,
		`{"weights":[{"setup":"lab-01","weight":Infinity}]}`,
		`{"weights":[{"setup":"lab-01","weight":1}`,
	} {
		msg, field := e.refused(e.put(body), http.StatusBadRequest, apierr.Validation)
		if msg != "request body is not valid JSON" || field != "" {
			t.Errorf("%s: message %q field %q", body, msg, field)
		}
	}
	for _, body := range []string{`{}`, `{"weights":null}`, `{"observed_at":"2026-11-23T10:00:00Z"}`} {
		msg, field := e.refused(e.put(body), http.StatusBadRequest, apierr.Validation)
		if field != "weights" || !strings.Contains(msg, "weights is required") {
			t.Errorf("%s: message %q field %q", body, msg, field)
		}
	}
	for _, body := range []string{`{"weights":{"lab-01":1}}`, `{"weights":"lab-01"}`} {
		if _, field := e.refused(e.put(body), http.StatusBadRequest, apierr.Validation); field != "weights" {
			t.Errorf("%s: field %q", body, field)
		}
	}
	if _, field := e.refused(e.put(`{"weights":[],"extra":1}`), http.StatusBadRequest, apierr.Validation); field != "extra" {
		t.Errorf("unknown top-level field: field %q", field)
	}
	if s := e.stored(); !errors.Is(s.err, store.ErrNotFound) {
		t.Errorf("stored = %+v", s)
	}
}

// Limits: 10,000 entries are accepted, 10,001 are refused, and a body over
// 1 MB is refused; nothing is stored on a refusal.
func TestWGT8_EntryAndBodyLimits(t *testing.T) {
	e := newWEnv(t)
	report := func(n int, attrs string) string {
		var b strings.Builder
		b.WriteString(`{"weights":[`)
		for i := 0; i < n; i++ {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, `{"setup":"s%05d","weight":1%s}`, i, attrs)
		}
		b.WriteString("]}")
		return b.String()
	}
	msg, field := e.refused(e.put(report(10001, "")), http.StatusBadRequest, apierr.Validation)
	if field != "weights" || msg != "weights has 10001 entries (at most 10000)" {
		t.Errorf("10,001 entries: %q %q", field, msg)
	}
	var attrs []string
	for i := 0; i < 16; i++ {
		attrs = append(attrs, fmt.Sprintf(`"k%02d":"%s"`, i, strings.Repeat("v", 63)))
	}
	big := report(1000, `,"attributes":{`+strings.Join(attrs, ",")+`}`)
	if len(big) <= MaxBodyBytes {
		t.Fatalf("test body is only %d bytes", len(big))
	}
	if msg, _ := e.refused(e.put(big), http.StatusBadRequest, apierr.Validation); !strings.Contains(msg, "exceeds") {
		t.Errorf("1 MB: %q", msg)
	}
	if s := e.stored(); !errors.Is(s.err, store.ErrNotFound) {
		t.Errorf("stored = %+v", s)
	}
	m := e.ok(report(10000, ""))
	if m["accepted"] != float64(0) || len(m["unknown"].([]any)) != 10000 {
		t.Errorf("10,000 entries: accepted %v unknown %d", m["accepted"], len(m["unknown"].([]any)))
	}
}

// B2: the last report refused with 400 is kept for the admins: its time, the
// account's name and the message. A 409, a 200 and an off route keep it.
func TestWGT8_LastRefusalKeptForAdmins(t *testing.T) {
	e := newWEnv(t)
	a := &API{d: e.deps, reported: reported.New(e.st, e.clk)}
	h := a.guard(routeOf(t, "reportWeights"), http.HandlerFunc(a.reportWeights))
	put := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PUT", weightsPath, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+e.svc.token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if _, ok := a.reported.LastWeightsRefusal(); ok {
		t.Fatal("a refusal before any report")
	}
	e.clk.Advance(30 * time.Second)
	msg, _ := e.refused(put(`{"weights":[{"setup":"lab-03","weight":25}]}`), http.StatusBadRequest, apierr.Validation)
	r, ok := a.reported.LastWeightsRefusal()
	want := reported.Refusal{At: wStart.Add(30 * time.Second), ServiceAccount: "weigher", Message: msg}
	if !ok || !r.At.Equal(want.At) || r.ServiceAccount != want.ServiceAccount || r.Message != want.Message {
		t.Fatalf("refusal = %+v, want %+v", r, want)
	}
	if msg != `weights[0] (setup "lab-03"): weight 25 is above the maximum 20` {
		t.Errorf("message %q", msg)
	}
	// A 200, a 409 stale_report and a 409 not_configured do not replace it.
	if rec := put(`{"observed_at":"2026-11-23T10:00:00Z","weights":[]}`); rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec := put(`{"observed_at":"2026-11-23T09:00:00Z","weights":[]}`); rec.Code != http.StatusConflict {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	e.setting(`{"enabled":false,"min":0,"max":20,"max_age":"10m"}`)
	if rec := put(`{"weights":"bad"}`); rec.Code != http.StatusConflict {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if r2, _ := a.reported.LastWeightsRefusal(); r2 != r {
		t.Errorf("refusal replaced by a non-400 answer: %+v", r2)
	}
	// A body that is not JSON is a 400 too, and replaces it.
	e.setting(`{"enabled":true,"min":0,"max":20,"max_age":"10m"}`)
	e.clk.Advance(time.Minute)
	put(`{"weights":[{"setup":"lab-01","weight":NaN}]}`)
	if r2, _ := a.reported.LastWeightsRefusal(); r2.Message != "request body is not valid JSON" || !r2.At.Equal(wStart.Add(90*time.Second)) {
		t.Errorf("refusal = %+v", r2)
	}
}

// ---- WGT-9 ----

func TestWGT9_UnknownNameIgnoredAndListed(t *testing.T) {
	e := newWEnv(t)
	before, _ := e.state()
	m := e.ok(`{"weights":[{"setup":"ghost-9","weight":5},{"setup":"lab-01","weight":3},{"setup":"ghost-1","weight":7}]}`)
	if m["accepted"] != float64(1) || !reflect.DeepEqual(m["unknown"], []any{"ghost-9", "ghost-1"}) {
		t.Errorf("response = %v", m)
	}
	if s := e.stored(); len(s.set) != 3 {
		t.Errorf("unknown names are stored too: %+v", s.set)
	}
	st, body := e.state()
	wantWeight(t, st, "lab-01", 3, "reported", float64(3), "", nil)
	for _, name := range []string{"lab-02", "lab-03"} {
		if !reflect.DeepEqual(weightOf(t, st, name), weightOf(t, before, name)) {
			t.Errorf("%s changed: %v", name, weightOf(t, st, name))
		}
	}
	if strings.Contains(body, "ghost") || len(st["setups"].([]any)) != 3 {
		t.Errorf("an unknown name shows in the state: %s", body)
	}
}

// ---- WGT-10 ----

func TestWGT10_OffAnswersNotConfiguredWhateverTheBody(t *testing.T) {
	e := newWEnv(t)
	e.setting(`{"enabled":false,"min":0,"max":null,"max_age":"10m"}`)
	for _, body := range []string{
		`{"weights":[{"setup":"lab-01","weight":1}]}`,   // valid
		`{"weights":[{"setup":"lab-01","weight":"1"}]}`, // invalid entry
		`{"weights":[{"setup":"lab-01","weight":NaN}]}`, // not JSON
		`{}`, // no weights
		``,   // no body
	} {
		msg, _ := e.refused(e.put(body), http.StatusConflict, apierr.NotConfigured)
		if !strings.Contains(msg, "reported_weights") || !strings.Contains(msg, "admin") {
			t.Errorf("%q: message %q does not name the setting and the admin", body, msg)
		}
		if s := e.stored(); !errors.Is(s.err, store.ErrNotFound) {
			t.Errorf("%q: stored = %+v", body, s)
		}
	}
	// With a report stored while it was on, an off route stores nothing new.
	e.setting(`{"enabled":true,"min":0,"max":20,"max_age":"10m"}`)
	e.ok(`{"weights":[{"setup":"lab-01","weight":1}]}`)
	before := e.stored()
	e.setting(`{"enabled":false,"min":0,"max":20,"max_age":"10m"}`)
	e.refused(e.put(`{"weights":[{"setup":"lab-01","weight":2}]}`), http.StatusConflict, apierr.NotConfigured)
	if after := e.stored(); !reflect.DeepEqual(after, before) {
		t.Errorf("stored = %+v", after)
	}
}

// ---- WGT-11 ----

// The route answers only a service token with weights:write: every user kind
// and no credentials get 401, an account with every other scope gets 403
// forbidden. An account with only weights:write is refused on every other
// route that needs any authorization.
func TestWGT11_RouteNeedsWeightsWriteAndTheScopeOpensNothingElse(t *testing.T) {
	f := newPermFix(t)
	for _, a := range f.actors {
		rec := f.send(a, f.probe("reportWeights", a, modeDefault))
		switch a.name {
		case actSvcScoped:
			if rec.Code != http.StatusOK {
				t.Errorf("%s: %d %s", a.name, rec.Code, rec.Body)
			}
		case actSvcNoScope:
			permWantDenied(t, rec, permDenial{http.StatusForbidden, apierr.Forbidden})
		default:
			permWantDenied(t, rec, permDenial{http.StatusUnauthorized, apierr.Unauthenticated})
		}
	}
	if !hasString(scopesWithout(auth.ScopeWeightsWrite), auth.ScopeHoldsRelease) || len(scopesWithout(auth.ScopeWeightsWrite)) != len(auth.Scopes())-1 {
		t.Fatalf("scopesWithout(weights:write) = %v", scopesWithout(auth.ScopeWeightsWrite))
	}

	only := f.mintServiceAccount("wgt11-only-weights", auth.ScopeWeightsWrite)
	anon := f.actor(actAnonymous)
	for _, r := range Routes() {
		if r.OperationID == "reportWeights" || r.Access == AccessPublic {
			continue
		}
		p := f.probe(r.OperationID, anon, modeDefault)
		rec := f.serve(p.method, p.path, nil, false, p.body, bearer(only.token)...)
		want := permDenial{http.StatusUnauthorized, apierr.Unauthenticated}
		if r.Access == AccessService {
			want = permDenial{http.StatusForbidden, apierr.Forbidden}
		}
		if rec.Code != want.status || permErrCode(rec) != want.code {
			t.Errorf("%s %s with only weights:write: %d %s, want %d %s", r.Method, r.Pattern, rec.Code, rec.Body, want.status, want.code)
		}
	}
}

// D3: the scope exists, comes after holds:release, and no existing account
// gains it.
func TestWGT11_ScopeAddedLastAndNotGrantedToExistingAccounts(t *testing.T) {
	want := []string{"state:read", "pressure:write", "deployments:write", "holds:release", "weights:write"}
	if !reflect.DeepEqual(auth.Scopes(), want) {
		t.Errorf("Scopes() = %v, want %v", auth.Scopes(), want)
	}
	b := newBackend(t, backendOpts{start: wStart})
	acct := b.mintServiceAccount("older", "state:read", "pressure:write", "deployments:write", "holds:release")
	ctx := context.Background()
	got, err := b.st.GetServiceAccount(ctx, acct.acct.ID)
	if err != nil {
		t.Fatal(err)
	}
	if hasString(got.Scopes, auth.ScopeWeightsWrite) {
		t.Errorf("an existing account has weights:write: %v", got.Scopes)
	}
	if err := b.st.PutSetting(ctx, "reported_weights", `{"enabled":true,"min":0,"max":20,"max_age":"10m"}`, nil, b.clk.Now()); err != nil {
		t.Fatal(err)
	}
	if rec := b.serveService("PUT", weightsPath, acct.token, `{"weights":[]}`); rec.Code != http.StatusForbidden {
		t.Errorf("an account with the four older scopes: %d %s", rec.Code, rec.Body)
	}
}

// ---- WGT-13 ----

// Every field of B4 for setups priced by override, reported, rule and
// default; each not_applied value and their precedence; the feature off; no
// rate rule selector anywhere.
func TestWGT13_StateWeightFields(t *testing.T) {
	e := newWEnv(t)
	ctx := context.Background()
	for _, s := range []string{"lab-ovr", "lab-ovr-out", "lab-out"} {
		if _, err := e.st.CreateSetup(ctx, store.Setup{Name: s, Enabled: s != "lab-out", Labels: map[string]string{"tier": "gold"}, DetailsJSON: `{}`}); err != nil {
			t.Fatal(err)
		}
	}
	three, four := 3.0, 4.0
	if err := e.st.SetSetupWeightOverride(ctx, e.setupID("lab-ovr"), &three); err != nil {
		t.Fatal(err)
	}
	if err := e.st.SetSetupWeightOverride(ctx, e.setupID("lab-ovr-out"), &four); err != nil {
		t.Fatal(err)
	}
	// Before the first report: the times are null, the switch and limits shown.
	st, _ := e.state()
	if w := st["weights"].(map[string]any); !reflect.DeepEqual(w, map[string]any{"enabled": true, "min": float64(0), "max": float64(20), "observed_at": nil, "expires_at": nil}) {
		t.Errorf("weights before a report = %v", w)
	}
	wantWeight(t, st, "lab-ovr", 3, "override", nil, "", nil)
	wantWeight(t, st, "lab-01", 2, "rule", nil, "", nil)
	wantWeight(t, st, "lab-03", 1, "default", nil, "", nil)

	gold := map[string]any{"tier": "gold", "memory-gb": "512"}
	e.ok(`{"observed_at":"2026-11-23T09:59:30Z","weights":[` +
		`{"setup":"lab-01","weight":1.15,"attributes":{"tier":"gold","memory-gb":"512"}},` +
		`{"setup":"lab-ovr","weight":5,"attributes":{"tier":"gold","memory-gb":"512"}},` +
		`{"setup":"lab-ovr-out","weight":15,"attributes":{"tier":"gold"}},` +
		`{"setup":"lab-out","weight":15,"attributes":{"tier":"gold"}}]}`)
	e.setting(`{"enabled":true,"min":0.5,"max":10,"max_age":"10m"}`)
	st, body := e.state()
	if w := st["weights"].(map[string]any); !reflect.DeepEqual(w, map[string]any{"enabled": true, "min": 0.5, "max": float64(10),
		"observed_at": "2026-11-23T09:59:30.000Z", "expires_at": "2026-11-23T10:09:30.000Z"}) {
		t.Errorf("weights = %v", w)
	}
	wantWeight(t, st, "lab-01", 1.15, "reported", 1.15, "", gold)                      // reported
	wantWeight(t, st, "lab-02", 2, "rule", nil, "", nil)                               // rule
	wantWeight(t, st, "lab-03", 1, "default", nil, "", nil)                            // default
	wantWeight(t, st, "lab-ovr", 3, "override", float64(5), "override", gold)          // override; usable entry keeps its attributes
	wantWeight(t, st, "lab-ovr-out", 4, "override", float64(15), "out_of_bounds", nil) // out_of_bounds before override
	wantWeight(t, st, "lab-out", 2, "rule", float64(15), "out_of_bounds", nil)         // a disabled setup is listed too
	wantNoSelector(t, body)

	// Exactly max_age old: still usable. One millisecond more: expired, first of all reasons.
	e.clk.Set(time.Date(2026, 11, 23, 10, 9, 30, 0, time.UTC))
	st, _ = e.state()
	wantWeight(t, st, "lab-01", 1.15, "reported", 1.15, "", gold)
	e.clk.Advance(time.Millisecond)
	st, body = e.state()
	wantWeight(t, st, "lab-01", 2, "rule", 1.15, "expired", nil)
	wantWeight(t, st, "lab-ovr", 3, "override", float64(5), "expired", nil)
	wantWeight(t, st, "lab-ovr-out", 4, "override", float64(15), "expired", nil)
	wantWeight(t, st, "lab-out", 2, "rule", float64(15), "expired", nil)
	if w := st["weights"].(map[string]any); w["observed_at"] != "2026-11-23T09:59:30.000Z" {
		t.Errorf("an expired report's times are still shown: %v", w)
	}
	wantNoSelector(t, body)

	// Off: reported and not_applied null for every setup, report times null,
	// the override still applies.
	e.clk.Set(wStart)
	e.setting(`{"enabled":false,"min":0,"max":null,"max_age":"10m"}`)
	st, body = e.state()
	if w := st["weights"].(map[string]any); !reflect.DeepEqual(w, map[string]any{"enabled": false, "min": float64(0), "max": nil, "observed_at": nil, "expires_at": nil}) {
		t.Errorf("weights off = %v", w)
	}
	for _, r := range st["setups"].([]any) {
		w := r.(map[string]any)["weight"].(map[string]any)
		if w["reported"] != nil || w["not_applied"] != nil || len(w["attributes"].(map[string]any)) != 0 || w["source"] == "reported" {
			t.Errorf("%v with the feature off: %v", r.(map[string]any)["name"], w)
		}
	}
	wantWeight(t, st, "lab-ovr", 3, "override", nil, "", nil)
	wantWeight(t, st, "lab-01", 2, "rule", nil, "", nil)
	wantNoSelector(t, body)
}

func wantNoSelector(t *testing.T, body string) {
	t.Helper()
	for _, banned := range []string{"tier=gold", "selector", "rule_selector", "matched_rule"} {
		if strings.Contains(body, banned) {
			t.Errorf("the state carries %q: %s", banned, body)
		}
	}
}

// ---- WGT-21 ----

// The stored report follows the name and the switch; an override follows the
// setup through a rename.
func TestWGT21_StoredWeightFollowsTheNameAndTheSwitch(t *testing.T) {
	e := newWEnv(t)
	ctx := context.Background()
	e.ok(`{"observed_at":"2026-11-23T10:00:00Z","weights":[{"setup":"lab-01","weight":1.5},{"setup":"lab-later","weight":2.5},` +
		`{"setup":"lab-target","weight":3.5},{"setup":"lab-pinned-new","weight":9}]}`)

	// A setup created later with a stored unknown name takes that weight.
	if _, err := e.st.CreateSetup(ctx, store.Setup{Name: "lab-later", Enabled: true, Labels: map[string]string{}, DetailsJSON: `{}`}); err != nil {
		t.Fatal(err)
	}
	st, _ := e.state()
	wantWeight(t, st, "lab-later", 2.5, "reported", 2.5, "", nil)

	// A free setup renamed to a stored name takes it at once.
	e.rename("lab-02", "lab-target", `{"tier":"gold"}`)
	// The old name's weight no longer applies to a setup renamed away from it.
	e.rename("lab-01", "lab-01-renamed", `{"tier":"gold"}`)
	st, _ = e.state()
	wantWeight(t, st, "lab-target", 3.5, "reported", 3.5, "", nil)
	wantWeight(t, st, "lab-01-renamed", 2, "rule", nil, "", nil)

	// An override stays with its setup through a rename, even onto a reported name.
	seven := 7.0
	if err := e.st.SetSetupWeightOverride(ctx, e.setupID("lab-03"), &seven); err != nil {
		t.Fatal(err)
	}
	e.rename("lab-03", "lab-pinned-new", `{}`)
	st, _ = e.state()
	wantWeight(t, st, "lab-pinned-new", 7, "override", float64(9), "override", nil)

	// Off then on again within max_age: the report applies again.
	e.setting(`{"enabled":false,"min":0,"max":20,"max_age":"10m"}`)
	st, _ = e.state()
	wantWeight(t, st, "lab-target", 2, "rule", nil, "", nil)
	if s := e.stored(); len(s.set) != 4 {
		t.Errorf("turning the switch off deleted the report: %+v", s)
	}
	e.clk.Advance(5 * time.Minute)
	e.setting(`{"enabled":true,"min":0,"max":20,"max_age":"10m"}`)
	st, _ = e.state()
	wantWeight(t, st, "lab-target", 3.5, "reported", 3.5, "", nil)

	// After max_age the report has expired; raising max_age revives it.
	e.clk.Advance(6 * time.Minute)
	st, _ = e.state()
	wantWeight(t, st, "lab-target", 2, "rule", 3.5, "expired", nil)
	e.setting(`{"enabled":true,"min":0,"max":20,"max_age":"30m"}`)
	st, _ = e.state()
	wantWeight(t, st, "lab-target", 3.5, "reported", 3.5, "", nil)
	wantWeight(t, st, "lab-later", 2.5, "reported", 2.5, "", nil)
	if w := st["weights"].(map[string]any); w["expires_at"] != "2026-11-23T10:30:00.000Z" {
		t.Errorf("expires_at after raising max_age = %v", w["expires_at"])
	}
}

// ---- Tightening the limits (design, Tests; D1) ----

// Tightening the maximum excludes a stored weight at once, refuses the next
// report whole, and polls keep succeeding. (What the admin sees is
// milestone 5: the collector view, WGT-22.)
func TestWGT12_TighteningTheMaximumOverHTTP(t *testing.T) {
	e := newWEnv(t)
	ctx := context.Background()
	e.policy.SetDefault(hourglass.Decision{Meter: true, Reason: "metered"})
	dev := e.newUser("holder", store.StatusActive, store.RoleDeveloper)
	h, err := e.st.CreateHold(ctx, e.setupID("lab-01"), dev.user.ID, e.clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	e.prov.SetAllocations(hourglassAlloc("lab-01"))
	e.ok(`{"weights":[{"setup":"lab-01","weight":15}]}`)
	poll := func() store.UsageSample {
		t.Helper()
		if err := e.col.RunOnce(ctx); err != nil {
			t.Fatalf("poll: %v", err)
		}
		if !e.col.LastPoll().OK {
			t.Fatal("the poll failed")
		}
		ss, err := e.st.ListUsageSamples(ctx, h.ID)
		if err != nil || len(ss) == 0 {
			t.Fatalf("samples: %v %v", ss, err)
		}
		return ss[len(ss)-1]
	}
	poll() // metering starts
	e.clk.Advance(5 * time.Minute)
	if s := poll(); s.Weight != 15 || s.WeightSource != "reported" {
		t.Errorf("before tightening: sample %+v", s)
	}
	e.setting(`{"enabled":true,"min":0,"max":10,"max_age":"10m"}`)
	st, _ := e.state()
	wantWeight(t, st, "lab-01", 2, "rule", float64(15), "out_of_bounds", nil)
	before := e.stored()
	msg, _ := e.refused(e.put(`{"weights":[{"setup":"lab-01","weight":15},{"setup":"lab-02","weight":1}]}`), http.StatusBadRequest, apierr.Validation)
	if msg != `weights[0] (setup "lab-01"): weight 15 is above the maximum 10` {
		t.Errorf("message %q", msg)
	}
	if after := e.stored(); !reflect.DeepEqual(after, before) {
		t.Errorf("the refused report changed the stored one")
	}
	e.clk.Advance(5 * time.Minute)
	if s := poll(); s.Weight != 2 || s.WeightSource != "rule" {
		t.Errorf("after tightening: sample %+v", s)
	}
	e.clk.Advance(10 * time.Minute) // past max_age: still the fallback, polls succeed
	if s := poll(); s.Weight != 2 || s.WeightSource != "rule" {
		t.Errorf("after max_age: sample %+v", s)
	}
}
