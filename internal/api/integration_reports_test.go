// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/api/apierr"
	"github.com/panset/hourglass/internal/reported"
	"github.com/panset/hourglass/internal/store"
)

// Tests of INT-1 to INT-4 and INT-8 (docs/design/integration-api.md) over HTTP.

const (
	pressurePath    = "/api/v1/integration/pressure"
	deploymentsPath = "/api/v1/integration/deployments"
	statePath       = "/api/v1/integration/state"
)

// reportEnv is a backend that has the reported provider and pressure source
// configured, a service account holding every scope, and two setups.
type reportEnv struct {
	*backend
	svc svcAccount
}

func newReportEnv(t *testing.T) *reportEnv {
	t.Helper()
	b := newBackend(t, backendOpts{start: time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC),
		runtime: Runtime{Provider: "reported", PressureSource: "reported",
			ReportedProviderMaxAge: 10 * time.Minute, ReportedPressureMaxAge: 15 * time.Minute}})
	e := &reportEnv{backend: b, svc: b.mintServiceAccount("reporter", allScopes()...)}
	for _, name := range []string{"lab-01", "lab-02"} {
		if _, err := b.st.CreateSetup(context.Background(), store.Setup{Name: name, Enabled: true,
			Labels: map[string]string{"size": "large"}, DetailsJSON: `{"note":"sample"}`}); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func (e *reportEnv) put(path, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.serveService("PUT", path, e.svc.token, body)
}

func (e *reportEnv) wantError(rec *httptest.ResponseRecorder, status int, code string) map[string]any {
	e.t.Helper()
	if rec.Code != status {
		e.t.Fatalf("status %d, want %d: %s", rec.Code, status, rec.Body)
	}
	m := decodeMap(e.t, rec)
	errObj, _ := m["error"].(map[string]any)
	if errObj["code"] != code {
		e.t.Fatalf("error code %v, want %s: %s", errObj["code"], code, rec.Body)
	}
	return errObj
}

func (e *reportEnv) noPressureStored() {
	e.t.Helper()
	if _, err := e.st.GetReportedPressure(context.Background()); err == nil {
		e.t.Fatal("a pressure report was stored")
	}
}

func (e *reportEnv) noDeploymentsStored() {
	e.t.Helper()
	if _, _, err := e.st.GetReportedDeployments(context.Background()); err == nil {
		e.t.Fatal("a deployment report was stored")
	}
}

func TestINT1_PressureValueValidated(t *testing.T) {
	e := newReportEnv(t)
	for _, body := range []string{
		`{}`, `{"value":null}`, `{"value":-0.01}`, `{"value":1.01}`, `{"value":2}`, `{"value":"0.5"}`,
		`{"value":1e999}`, `{"value":0.5,"observed_at":"yesterday"}`, `not json`,
	} {
		e.wantError(e.put(pressurePath, body), http.StatusBadRequest, apierr.Validation)
		e.noPressureStored()
	}
	// The bounds themselves are valid, and the response carries expires_at.
	for _, v := range []string{"0", "1"} {
		rec := e.put(pressurePath, `{"value":`+v+`,"observed_at":"2026-11-20T09:59:00Z"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("value %s: %d %s", v, rec.Code, rec.Body)
		}
		m := decodeMap(t, rec)
		if m["observed_at"] != "2026-11-20T09:59:00.000Z" || m["expires_at"] != "2026-11-20T10:14:00.000Z" || m["received_at"] != "2026-11-20T10:00:00.000Z" {
			t.Errorf("result %v: expires_at must be observed_at plus the configured max_age (15m)", m)
		}
	}
	got, err := e.st.GetReportedPressure(context.Background())
	if err != nil || got.Value != 1 || got.ServiceAccount != "reporter" {
		t.Errorf("stored %+v %v: want value 1 from the calling account", got, err)
	}
	// observed_at defaults to the time of receipt.
	rec := e.put(pressurePath, `{"value":0.3}`)
	if m := decodeMap(t, rec); rec.Code != 200 || m["observed_at"] != "2026-11-20T10:00:00.000Z" {
		t.Errorf("default observed_at: %d %s", rec.Code, rec.Body)
	}
}

func TestINT2_DeploymentsReplaceWholeSet(t *testing.T) {
	e := newReportEnv(t)
	ctx := context.Background()
	rec := e.put(deploymentsPath, `{"observed_at":"2026-11-20T09:50:00Z","deployments":[{"setup":"lab-01","labels":{"cluster":"c1"}},{"setup":"typo-9"}]}`)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	m := decodeMap(t, rec)
	if m["accepted"] != float64(1) || fmt.Sprint(m["unknown"]) != "[typo-9]" || m["expires_at"] != "2026-11-20T10:00:00.000Z" {
		t.Errorf("result %v: want accepted 1, unknown [typo-9], expires_at observed_at+10m", m)
	}
	// The second report replaces the first: lab-01 is gone, the empty unknown list is [] not null.
	rec = e.put(deploymentsPath, `{"observed_at":"2026-11-20T09:51:00Z","deployments":[{"setup":"lab-02"}]}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"unknown":[]`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	hdr, set, err := e.st.GetReportedDeployments(ctx)
	if err != nil || len(set) != 1 || set[0].SetupName != "lab-02" || hdr.ServiceAccount != "reporter" {
		t.Fatalf("stored %+v %+v %v: want only lab-02, from the calling account", hdr, set, err)
	}
	// An empty list means nothing is deployed.
	if rec = e.put(deploymentsPath, `{"observed_at":"2026-11-20T09:52:00Z","deployments":[]}`); rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if _, set, _ = e.st.GetReportedDeployments(ctx); len(set) != 0 {
		t.Errorf("set %+v, want empty", set)
	}
	// A missing list is not an empty list.
	e.wantError(e.put(deploymentsPath, `{"observed_at":"2026-11-20T09:53:00Z"}`), http.StatusBadRequest, apierr.Validation)

	// Refused reports change nothing.
	e.put(deploymentsPath, `{"observed_at":"2026-11-20T09:54:00Z","deployments":[{"setup":"lab-01"}]}`)
	many := make([]string, 10001)
	for i := range many {
		many[i] = fmt.Sprintf(`{"setup":"n%d"}`, i)
	}
	labels := make([]string, 33)
	for i := range labels {
		labels[i] = fmt.Sprintf(`"k%d":"v"`, i)
	}
	for name, body := range map[string]string{
		"duplicate name":   `{"observed_at":"2026-11-20T09:55:00Z","deployments":[{"setup":"lab-02"},{"setup":"lab-02"}]}`,
		"empty name":       `{"observed_at":"2026-11-20T09:55:00Z","deployments":[{"setup":""}]}`,
		"10001 entries":    `{"observed_at":"2026-11-20T09:55:00Z","deployments":[` + strings.Join(many, ",") + `]}`,
		"33 labels":        `{"observed_at":"2026-11-20T09:55:00Z","deployments":[{"setup":"lab-02","labels":{` + strings.Join(labels, ",") + `}}]}`,
		"over 1 MB":        `{"observed_at":"2026-11-20T09:55:00Z","deployments":[{"setup":"lab-02","labels":{"k":"` + strings.Repeat("x", MaxBodyBytes) + `"}}]}`,
		"unknown field":    `{"deployments":[],"extra":1}`,
		"bad observed_at":  `{"observed_at":"x","deployments":[]}`,
		"unnamed non-list": `{"deployments":{}}`,
	} {
		e.wantError(e.put(deploymentsPath, body), http.StatusBadRequest, apierr.Validation)
		if _, set, _ = e.st.GetReportedDeployments(ctx); len(set) != 1 || set[0].SetupName != "lab-01" {
			t.Fatalf("%s: stored set %+v changed by a refused report", name, set)
		}
	}
	// Exactly 10,000 entries and 32 labels are within the limits.
	ok := make([]string, 10000)
	for i := range ok {
		ok[i] = fmt.Sprintf(`{"setup":"n%d"}`, i)
	}
	if rec = e.put(deploymentsPath, `{"observed_at":"2026-11-20T09:56:00Z","deployments":[`+strings.Join(ok, ",")+`]}`); rec.Code != 200 {
		t.Errorf("10,000 entries: %d %.200s", rec.Code, rec.Body)
	}
	if rec = e.put(deploymentsPath, `{"observed_at":"2026-11-20T09:57:00Z","deployments":[{"setup":"lab-01","labels":{`+strings.Join(labels[:32], ",")+`}}]}`); rec.Code != 200 {
		t.Errorf("32 labels: %d %s", rec.Code, rec.Body)
	}
}

// A reader never sees a mix of two reports: with two reports that each deploy
// one setup and not the other, every state read shows exactly one deployed.
func TestINT2_ReadersNeverSeeAMixOfTwoReports(t *testing.T) {
	e := newReportEnv(t)
	start := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	if rec := e.put(deploymentsPath, `{"observed_at":"2026-11-20T09:00:00Z","deployments":[{"setup":"lab-01"}]}`); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	bad := make(chan string, 1)
	for r := 0; r < 3; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				rec := e.serveService("GET", statePath, e.svc.token, "")
				m := decodeMap(t, rec)
				n := 0
				for _, s := range m["setups"].([]any) {
					if s.(map[string]any)["reported_deployed"] == true {
						n++
					}
				}
				if n != 1 {
					select {
					case bad <- rec.Body.String():
					default:
					}
					return
				}
			}
		}()
	}
	names := []string{"lab-02", "lab-01"}
	for i := 1; i <= 40; i++ {
		at := start.Add(time.Duration(i) * time.Second).Format(time.RFC3339)
		if rec := e.put(deploymentsPath, fmt.Sprintf(`{"observed_at":%q,"deployments":[{"setup":%q}]}`, at, names[i%2])); rec.Code != 200 {
			t.Fatal(rec.Body)
		}
	}
	close(stop)
	wg.Wait()
	select {
	case body := <-bad:
		t.Fatalf("a state read saw a mix of two reports: %s", body)
	default:
	}
}

func TestINT3_OlderReportRefusedEqualReplaces(t *testing.T) {
	e := newReportEnv(t)
	ctx := context.Background()
	e.put(pressurePath, `{"value":0.2,"observed_at":"2026-11-20T09:50:00Z"}`)
	e.put(deploymentsPath, `{"observed_at":"2026-11-20T09:50:00Z","deployments":[{"setup":"lab-01"}]}`)

	// Older: 409 stale_report, nothing changed.
	e.wantError(e.put(pressurePath, `{"value":0.9,"observed_at":"2026-11-20T09:49:59.999Z"}`), http.StatusConflict, apierr.StaleReport)
	e.wantError(e.put(deploymentsPath, `{"observed_at":"2026-11-20T09:49:00Z","deployments":[{"setup":"lab-02"}]}`), http.StatusConflict, apierr.StaleReport)
	if p, _ := e.st.GetReportedPressure(ctx); p.Value != 0.2 {
		t.Errorf("a stale pressure report changed the stored value to %v", p.Value)
	}
	if _, set, _ := e.st.GetReportedDeployments(ctx); len(set) != 1 || set[0].SetupName != "lab-01" {
		t.Errorf("a stale deployment report changed the set to %+v", set)
	}
	// Equal: replaces, and sending the same report twice is harmless.
	for i := 0; i < 2; i++ {
		if rec := e.put(pressurePath, `{"value":0.6,"observed_at":"2026-11-20T09:50:00Z"}`); rec.Code != 200 {
			t.Fatalf("equal pressure report: %d %s", rec.Code, rec.Body)
		}
		if rec := e.put(deploymentsPath, `{"observed_at":"2026-11-20T09:50:00Z","deployments":[{"setup":"lab-02"}]}`); rec.Code != 200 {
			t.Fatalf("equal deployment report: %d %s", rec.Code, rec.Body)
		}
	}
	if p, _ := e.st.GetReportedPressure(ctx); p.Value != 0.6 {
		t.Errorf("an equal report did not replace the value: %v", p.Value)
	}
	if _, set, _ := e.st.GetReportedDeployments(ctx); len(set) != 1 || set[0].SetupName != "lab-02" {
		t.Errorf("an equal report did not replace the set: %+v", set)
	}
	// Newer replaces.
	if rec := e.put(pressurePath, `{"value":0.7,"observed_at":"2026-11-20T09:51:00Z"}`); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	// Clock skew: two minutes ahead is accepted, more is a validation error.
	if rec := e.put(pressurePath, `{"value":0.7,"observed_at":"2026-11-20T10:02:00Z"}`); rec.Code != 200 {
		t.Errorf("2 minutes ahead: %d %s", rec.Code, rec.Body)
	}
	e.wantError(e.put(pressurePath, `{"value":0.7,"observed_at":"2026-11-20T10:02:01Z"}`), http.StatusBadRequest, apierr.Validation)
	e.wantError(e.put(deploymentsPath, `{"observed_at":"2026-11-20T10:03:00Z","deployments":[]}`), http.StatusBadRequest, apierr.Validation)
}

// A report takes effect at the next poll: it never triggers a poll, a debit or
// an event by itself.
func TestINT3_ReportDoesNotPollDebitOrEmit(t *testing.T) {
	e := newReportEnv(t)
	ctx := context.Background()
	dev := e.newUser("holder", store.StatusActive, store.RoleDeveloper)
	setups, _ := e.st.ListSetups(ctx, false)
	h, err := e.st.CreateHold(ctx, setups[0].ID, dev.user.ID, e.clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	before, _ := e.st.GetUser(ctx, dev.user.ID)
	ledgerBefore, _ := e.st.ListLedger(ctx, dev.user.ID, "2026-11")
	e.put(deploymentsPath, `{"deployments":[{"setup":"`+setups[0].Name+`"}]}`)
	e.put(pressurePath, `{"value":0.95}`)
	runs, _ := e.st.ListCollectorRuns(ctx, 10)
	after, _ := e.st.GetUser(ctx, dev.user.ID)
	got, _ := e.st.GetHold(ctx, h.ID)
	deliveries, _ := e.st.ListEventDeliveries(ctx, 10)
	ledger, _ := e.st.ListLedger(ctx, dev.user.ID, "2026-11")
	if len(runs) != 0 || e.col.LastPoll().OK || after.BalanceMT != before.BalanceMT ||
		got.DeployedAt != nil || got.MeteringStartedAt != nil || len(deliveries) != 0 || len(ledger) != len(ledgerBefore) {
		t.Errorf("a report had an effect: runs=%d poll=%v balance %d->%d hold=%+v deliveries=%d",
			len(runs), e.col.LastPoll().OK, before.BalanceMT, after.BalanceMT, got, len(deliveries))
	}
	if len(e.hook.Events()) != 0 {
		t.Errorf("a report fired %d events", len(e.hook.Events()))
	}
}

func TestINT4_NotConfiguredRefusedAndNothingStored(t *testing.T) {
	cases := []struct {
		name    string
		runtime Runtime
		pOK     bool // pressure route configured
		dOK     bool
	}{
		{"neither", Runtime{Provider: "fake", PressureSource: "fake"}, false, false},
		{"only the provider", Runtime{Provider: "reported", ReportedProviderMaxAge: 10 * time.Minute}, false, true},
		{"only the pressure source", Runtime{PressureSource: "reported", ReportedPressureMaxAge: 10 * time.Minute}, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := newBackend(t, backendOpts{start: time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC), runtime: c.runtime})
			e := &reportEnv{backend: b, svc: b.mintServiceAccount("reporter", allScopes()...)}
			rec := e.put(pressurePath, `{"value":0.5}`)
			if c.pOK {
				if rec.Code != 200 {
					t.Fatalf("pressure: %d %s", rec.Code, rec.Body)
				}
			} else {
				msg, _ := e.wantError(rec, http.StatusConflict, apierr.NotConfigured)["message"].(string)
				if !strings.Contains(msg, "pressure.type") {
					t.Errorf("message %q does not name pressure.type", msg)
				}
				e.noPressureStored()
			}
			rec = e.put(deploymentsPath, `{"deployments":[]}`)
			if c.dOK {
				if rec.Code != 200 {
					t.Fatalf("deployments: %d %s", rec.Code, rec.Body)
				}
			} else {
				msg, _ := e.wantError(rec, http.StatusConflict, apierr.NotConfigured)["message"].(string)
				if !strings.Contains(msg, "provider.type") {
					t.Errorf("message %q does not name provider.type", msg)
				}
				e.noDeploymentsStored()
			}
		})
	}
}

// stateKeys is the allow-list of every key INT-8 lets the state response carry,
// by path. "labels" and "details" are free-form maps and not walked.
var stateKeys = map[string][]string{
	"":                   {"server_time", "poll", "pressure", "deployments", "setups"},
	"poll":               {"interval_seconds", "last_ok_at"},
	"pressure":           {"source", "reported", "smoothed", "multiplier"},
	"pressure.reported":  {"value", "observed_at", "expires_at"},
	"deployments":        {"provider", "observed_at", "expires_at"},
	"setups[]":           {"name", "labels", "details", "enabled", "reported_deployed", "hold"},
	"setups[].hold":      {"id", "user", "acquired_at", "metering_started_at", "deployed_at", "release_requested_at"},
	"setups[].hold.user": {"id", "username", "email", "name"},
}

func walkStateKeys(t *testing.T, path string, v any) {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		allowed, ok := stateKeys[path]
		if !ok {
			t.Errorf("object at %q is not in the allow-list", path)
			return
		}
		var keys []string
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		want := append([]string(nil), allowed...)
		sort.Strings(want)
		if fmt.Sprint(keys) != fmt.Sprint(want) {
			t.Errorf("keys at %q are %v, want exactly %v", path, keys, want)
		}
		for _, k := range keys {
			if k == "labels" || k == "details" {
				continue
			}
			child := k
			if path != "" {
				child = path + "." + k
			}
			walkStateKeys(t, child, x[k])
		}
	case []any:
		for _, el := range x {
			walkStateKeys(t, path+"[]", el)
		}
	}
}

func TestINT8_StateHasEverySetupAndHoldAndNothingElse(t *testing.T) {
	e := newReportEnv(t)
	ctx := context.Background()
	// Before any report: reported and the observed_at fields are null.
	rec := e.serveService("GET", statePath, e.svc.token, "")
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	m := decodeMap(t, rec)
	pr := m["pressure"].(map[string]any)
	dp := m["deployments"].(map[string]any)
	if pr["reported"] != nil || dp["observed_at"] != nil || dp["expires_at"] != nil || m["poll"].(map[string]any)["last_ok_at"] != nil {
		t.Errorf("before the first report: %v", m)
	}
	if pr["source"] != "reported" || dp["provider"] != "reported" || m["poll"].(map[string]any)["interval_seconds"] != float64(300) {
		t.Errorf("runtime fields: %v", m)
	}

	// A disabled setup, a held setup (with a user who has a balance and a ledger) and a free one.
	if _, err := e.st.CreateSetup(ctx, store.Setup{Name: "lab-off", Enabled: false, Labels: map[string]string{}, DetailsJSON: `{}`}); err != nil {
		t.Fatal(err)
	}
	alice := e.newUser("alice", store.StatusActive, store.RoleDeveloper)
	bob := e.newUser("bob", store.StatusActive, store.RoleDeveloper)
	setups, _ := e.st.ListSetups(ctx, false)
	byName := map[string]store.Setup{}
	for _, s := range setups {
		byName[s.Name] = s
	}
	if _, err := e.st.CreateHold(ctx, byName["lab-01"].ID, alice.user.ID, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := e.st.CreateHold(ctx, byName["lab-off"].ID, bob.user.ID, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	e.put(pressurePath, `{"value":0.72,"observed_at":"2026-11-20T09:58:00Z"}`)
	e.put(deploymentsPath, `{"observed_at":"2026-11-20T09:59:00Z","deployments":[{"setup":"lab-02"}]}`)

	rec = e.serveService("GET", statePath, e.svc.token, "")
	m = decodeMap(t, rec)
	walkStateKeys(t, "", m)
	body := strings.ToLower(rec.Body.String())
	for _, banned := range []string{"balance", "ledger", "settings", "audit", "password", "token_hash", "allowance", "roles"} {
		if strings.Contains(body, banned) {
			t.Errorf("state response contains %q: %s", banned, rec.Body)
		}
	}

	rows := m["setups"].([]any)
	if len(rows) != 3 {
		t.Fatalf("%d setups, want all 3 (disabled included): %s", len(rows), rec.Body)
	}
	byRow := map[string]map[string]any{}
	for _, r := range rows {
		row := r.(map[string]any)
		byRow[row["name"].(string)] = row
	}
	l1, l2, off := byRow["lab-01"], byRow["lab-02"], byRow["lab-off"]
	if l1["hold"] == nil || l2["hold"] != nil || off["hold"] == nil || off["enabled"] != false {
		t.Errorf("holds or enabled wrong: %s", rec.Body)
	}
	user := l1["hold"].(map[string]any)["user"].(map[string]any)
	if user["username"] != "" || user["email"] != "alice@example.test" || user["name"] != "User alice" || user["id"] != float64(alice.user.ID) {
		t.Errorf("hold.user = %v", user)
	}
	if l1["hold"].(map[string]any)["release_requested_at"] != nil || l1["hold"].(map[string]any)["deployed_at"] != nil {
		t.Errorf("hold timestamps: %v", l1["hold"])
	}
	// reported_deployed comes from the stored report, not the last poll (no poll has run).
	if l1["reported_deployed"] != false || l2["reported_deployed"] != true {
		t.Errorf("reported_deployed: lab-01 %v lab-02 %v", l1["reported_deployed"], l2["reported_deployed"])
	}
	if l1["details"].(map[string]any)["note"] != "sample" || l1["labels"].(map[string]any)["size"] != "large" {
		t.Errorf("labels or details: %v", l1)
	}
	pr = m["pressure"].(map[string]any)["reported"].(map[string]any)
	if pr["value"] != 0.72 || pr["expires_at"] != "2026-11-20T10:13:00.000Z" {
		t.Errorf("pressure.reported = %v (max_age 15m)", pr)
	}
	dp = m["deployments"].(map[string]any)
	if dp["observed_at"] != "2026-11-20T09:59:00.000Z" || dp["expires_at"] != "2026-11-20T10:09:00.000Z" {
		t.Errorf("deployments = %v (max_age 10m)", dp)
	}
	// A released hold is not shown.
	hs, _ := e.st.ActiveHolds(ctx)
	if err := e.st.ReleaseHold(ctx, hs[0].ID, &alice.user.ID, "", e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	m = decodeMap(t, e.serveService("GET", statePath, e.svc.token, ""))
	for _, r := range m["setups"].([]any) {
		if row := r.(map[string]any); row["name"] == "lab-01" && row["hold"] != nil {
			t.Errorf("a released hold is still shown: %v", row)
		}
	}
}

// When the extensions are not the reported ones, pressure.reported and the
// deployments report times are null and nothing is reported as deployed.
func TestINT8_StateWithoutReportedExtensions(t *testing.T) {
	b := newBackend(t, backendOpts{runtime: Runtime{Provider: "other", PressureSource: "other"}})
	svc := b.mintServiceAccount("reader", "state:read")
	// A stored report of an earlier configuration must not show.
	if _, err := reported.New(b.st, b.clk).SubmitDeployments(context.Background(), "old", nil, []reported.Deployment{{Setup: "lab-01"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.st.CreateSetup(context.Background(), store.Setup{Name: "lab-01", Enabled: true, Labels: map[string]string{}, DetailsJSON: `{}`}); err != nil {
		t.Fatal(err)
	}
	rec := b.serveService("GET", statePath, svc.token, "")
	m := decodeMap(t, rec)
	walkStateKeys(t, "", m)
	pr, dp := m["pressure"].(map[string]any), m["deployments"].(map[string]any)
	row := m["setups"].([]any)[0].(map[string]any)
	if pr["reported"] != nil || dp["observed_at"] != nil || dp["expires_at"] != nil || row["reported_deployed"] != false {
		t.Errorf("state = %s", rec.Body)
	}
}

// D62: the admin collector answer carries the stored reports' times and the
// sending account; both are null before the first report and when the
// extension is not `reported`.
func TestINT7_AdminCollectorShowsReportTimes(t *testing.T) {
	e := newReportEnv(t)
	admin := e.newUser("boss", store.StatusActive, store.RoleAdmin)
	get := func(b *backend) map[string]any {
		t.Helper()
		rec := b.serve("GET", "/api/v1/admin/collector", &admin, false, "")
		if rec.Code != 200 {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		return decodeMap(t, rec)
	}
	m := get(e.backend)
	if v, ok := m["last_pressure_report"]; !ok || v != nil {
		t.Errorf("before a report: last_pressure_report = %v", v)
	}
	if v, ok := m["last_deployments_report"]; !ok || v != nil {
		t.Errorf("before a report: last_deployments_report = %v", v)
	}
	if rec := e.put(pressurePath, `{"value":0.5,"observed_at":"2026-11-20T09:59:00Z"}`); rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec := e.put(deploymentsPath, `{"deployments":[]}`); rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	m = get(e.backend)
	pr, _ := m["last_pressure_report"].(map[string]any)
	dr, _ := m["last_deployments_report"].(map[string]any)
	if pr["observed_at"] != "2026-11-20T09:59:00.000Z" || pr["received_at"] != "2026-11-20T10:00:00.000Z" || pr["service_account"] != "reporter" || len(pr) != 3 {
		t.Errorf("last_pressure_report = %v", pr)
	}
	if dr["service_account"] != "reporter" || dr["received_at"] != "2026-11-20T10:00:00.000Z" || len(dr) != 3 {
		t.Errorf("last_deployments_report = %v", dr)
	}

	// A stored report of an earlier configuration does not show when the extensions are not `reported`.
	o := newBackend(t, backendOpts{runtime: Runtime{Provider: "other", PressureSource: "other"}})
	if _, err := reported.New(o.st, o.clk).SubmitDeployments(context.Background(), "old", nil, nil); err != nil {
		t.Fatal(err)
	}
	admin = o.newUser("boss", store.StatusActive, store.RoleAdmin)
	m = get(o)
	if m["last_pressure_report"] != nil || m["last_deployments_report"] != nil {
		t.Errorf("without the reported extensions: %v", m)
	}
}
