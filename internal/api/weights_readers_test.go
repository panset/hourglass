// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/metering"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// Tests of every other reader on the weight resolver, the ledger's weight
// fields, the weight override and the collector view over HTTP
// (docs/design/reported-weights.md A3, C1, C2, D2; WGT-14 to WGT-16, WGT-18,
// WGT-22, WGT-23).

// rEnv is a wEnv (rule tier=gold -> 2; lab-01 and lab-02 gold, lab-03 no
// labels; reported weights on, min 0, max 20, max_age 10m) whose metering
// policy meters every hold without setting a weight, with room for five
// holds per user.
type rEnv struct{ *wEnv }

func newREnv(t *testing.T, start time.Time, loc *time.Location) *rEnv {
	t.Helper()
	b := newBackend(t, backendOpts{start: start, loc: loc,
		runtime: Runtime{Provider: "reported", ReportedProviderMaxAge: 10 * time.Minute}})
	e := &rEnv{wEnvOn(t, b, true)}
	e.policy.SetDefault(hourglass.Decision{Meter: true, Reason: "metered"})
	if err := e.st.PutSetting(context.Background(), "max_holds_per_user", "5", nil, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	return e
}

// addSetup creates a setup in the store.
func (e *rEnv) addSetup(name string, labels map[string]string) int64 {
	e.t.Helper()
	id, err := e.st.CreateSetup(context.Background(), store.Setup{Name: name, Enabled: true, Labels: labels, DetailsJSON: `{}`})
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *rEnv) getOK(path string, s *sess) map[string]any {
	e.t.Helper()
	rec := e.serve("GET", path, s, false, "")
	if rec.Code != http.StatusOK {
		e.t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body)
	}
	return decodeMap(e.t, rec)
}

// putSetup sends the admin PUT of a setup with its stored name, labels,
// details and enabled flag, plus extra (for example `,"weight_override":3`).
func (e *rEnv) putSetup(name, extra string) *httptest.ResponseRecorder {
	e.t.Helper()
	id := e.setupID(name)
	s, err := e.st.GetSetup(context.Background(), id)
	if err != nil {
		e.t.Fatal(err)
	}
	labels, _ := json.Marshal(s.Labels)
	if s.Labels == nil {
		labels = []byte("{}")
	}
	body := fmt.Sprintf(`{"name":%q,"labels":%s,"details":%s,"enabled":%t%s}`, s.Name, labels, s.DetailsJSON, s.Enabled, extra)
	return e.serve("PUT", fmt.Sprintf("/api/v1/admin/setups/%d", id), &e.adm, true, body)
}

func (e *rEnv) putSetupOK(name, extra string) map[string]any {
	e.t.Helper()
	rec := e.putSetup(name, extra)
	if rec.Code != http.StatusOK {
		e.t.Fatalf("PUT setup %s%s: %d %s", name, extra, rec.Code, rec.Body)
	}
	return decodeMap(e.t, rec)
}

// pollOK runs one poll at the fake time and requires it to succeed.
func (e *rEnv) pollOK() {
	e.t.Helper()
	if err := e.col.RunOnce(context.Background()); err != nil {
		e.t.Fatalf("poll: %v", err)
	}
	if !e.col.LastPoll().OK {
		e.t.Fatal("the poll failed")
	}
}

func (e *rEnv) lastSample(holdID int64) store.UsageSample {
	e.t.Helper()
	ss, err := e.st.ListUsageSamples(context.Background(), holdID)
	if err != nil || len(ss) == 0 {
		e.t.Fatalf("samples of hold %d: %v %v", holdID, ss, err)
	}
	return ss[len(ss)-1]
}

// acquire acquires a setup over HTTP and returns the hold id.
func (e *rEnv) acquire(s *sess, name string) int64 {
	e.t.Helper()
	rec := e.serve("POST", fmt.Sprintf("/api/v1/setups/%d/acquire", e.setupID(name)), s, true, "")
	if rec.Code != http.StatusCreated {
		e.t.Fatalf("acquire %s: %d %s", name, rec.Code, rec.Body)
	}
	return int64(decodeMap(e.t, rec)["id"].(float64))
}

// raw opens a second connection to the backend's database, for writing what
// no route can (a sample from before migration 0008, a pruned sample).
func (e *rEnv) raw() *sql.DB {
	e.t.Helper()
	db, err := sql.Open("sqlite", "file:"+e.dbPath+"?_pragma=busy_timeout(10000)")
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { db.Close() })
	return db
}

func (e *rEnv) exec(q string, args ...any) {
	e.t.Helper()
	res, err := e.raw().Exec(q, args...)
	if err != nil {
		e.t.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		e.t.Fatalf("%s %v: %d rows", q, args, n)
	}
}

// byName finds the object named name (key "name" or "setup_name") in a list.
func byName(t *testing.T, list any, key, name string) map[string]any {
	t.Helper()
	for _, x := range list.([]any) {
		if m := x.(map[string]any); m[key] == name {
			return m
		}
	}
	t.Fatalf("no %s %q in %v", key, name, list)
	return nil
}

func names(list any) []string {
	out := []string{}
	for _, x := range list.([]any) {
		out = append(out, x.(map[string]any)["name"].(string))
	}
	return out
}

// shown is a weight and source as one reader shows them.
type shown struct {
	weight float64
	source any
}

func (s shown) String() string { return fmt.Sprintf("%v (%v)", s.weight, s.source) }

func shownOf(m map[string]any) shown {
	return shown{m["weight"].(float64), m["weight_source"]}
}

// ---- WGT-14 ----

// For one database state and one time, every reader shows the same weight
// and source as the state route, for setups priced by each of override,
// reported, rule and default, and a poll at that time records them on its
// sample. A hold whose latest sample has the source policy shows that
// sample's weight and policy in the hold and burn figures, while the setup
// views show the resolver's.
func TestWGT14_EveryReaderShowsTheWeightThePollCharges(t *testing.T) {
	e := newREnv(t, wStart, nil)
	e.addSetup("lab-04", map[string]string{"tier": "gold"})
	e.addSetup("lab-05", map[string]string{"tier": "gold"})
	e.putSetupOK("lab-01", `,"weight_override":3`)
	want := map[string]shown{
		"lab-01": {3, "override"}, "lab-02": {1.15, "reported"}, "lab-03": {1, "default"}, "lab-04": {2, "rule"},
		"lab-05": {2, "rule"},
	}
	all := []string{"lab-01", "lab-02", "lab-03", "lab-04", "lab-05"}
	devs := map[string]sess{}
	holdOf := map[string]int64{}
	var allocs []hourglass.Allocation
	for _, n := range all {
		d := e.newUser("dev-"+n, store.StatusActive, store.RoleDeveloper)
		devs[n] = d
		holdOf[n] = e.acquire(&d, n)
		allocs = append(allocs, hourglassAlloc(n))
	}
	e.prov.SetAllocations(allocs...)
	policyHold := holdOf["lab-05"]
	e.policy.SetFunc(func(h hourglass.HoldState, _ hourglass.Snapshot) hourglass.Decision {
		if h.Hold.ID == policyHold {
			return hourglass.Decision{Meter: true, Weight: 4, Reason: "policy weight"}
		}
		return hourglass.Decision{Meter: true, Reason: "metered"}
	})
	report := `{"weights":[{"setup":"lab-02","weight":1.15,"attributes":{"memory-gb":"512"}},{"setup":"lab-01","weight":9}]}`
	e.ok(report)
	e.pollOK() // metering starts
	e.clk.Advance(5 * time.Minute)
	e.ok(report)
	e.pollOK() // the first debits; lab-05's sample has the source policy
	if s := e.lastSample(policyHold); s.WeightSource != "policy" || s.Weight != 4 {
		t.Fatalf("policy sample %+v", s)
	}
	e.clk.Advance(5 * time.Minute) // the time T of every reading and of the next poll

	st, _ := e.state()
	anyDev := devs["lab-01"]
	devList := e.getOK("/api/v1/setups", &anyDev)
	admList := e.getOK("/api/v1/setups", &e.adm)
	admUsers := e.getOK("/api/v1/admin/users", &e.adm)
	mult := e.col.LastPoll().Multiplier
	readings := map[string]map[string]shown{}
	for _, n := range all {
		d := devs[n]
		w := weightOf(t, st, n)
		r := map[string]shown{"state": {w["applied"].(float64), w["source"]}}
		r["setups (developer)"] = shownOf(byName(t, devList["setups"], "name", n))
		r["setups (admin)"] = shownOf(byName(t, admList["setups"], "name", n))
		r["admin setup view"] = shownOf(e.putSetupOK(n, "")) // the admin view of a setup, unchanged
		hold := e.getOK("/api/v1/me/holds?state=active", &d)["holds"].([]any)[0].(map[string]any)
		r["hold"] = shownOf(hold)
		me := e.getOK("/api/v1/me", &d)
		burning := me["burning"].([]any)[0].(map[string]any)
		r["my usage burn"] = shownOf(burning)
		// Admin Users shows only the burn per hour: it must be that weight's.
		var u map[string]any
		for _, x := range admUsers["users"].([]any) {
			if m := x.(map[string]any); m["id"] == float64(d.user.ID) {
				u = m
			}
		}
		if got, wantBurn := u["burn_per_hour_tokens"], tokens(metering.HourlyCostMT(r["hold"].weight, mult)); got != wantBurn {
			t.Errorf("%s: Admin Users burn %v, want %v (weight %v)", n, got, wantBurn, r["hold"].weight)
		}
		if me["burn_per_hour_tokens"] != u["burn_per_hour_tokens"] {
			t.Errorf("%s: My usage burn %v, Admin Users burn %v", n, me["burn_per_hour_tokens"], u["burn_per_hour_tokens"])
		}
		readings[n] = r
	}
	e.pollOK() // at T
	for _, n := range all {
		s := e.lastSample(holdOf[n])
		if !s.PolledAt.Equal(e.clk.Now()) {
			t.Fatalf("%s: no sample at T: %+v", n, s)
		}
		charged := shown{s.Weight, s.WeightSource}
		for reader, got := range readings[n] {
			wantHere := want[n]
			hold := reader == "hold" || reader == "my usage burn"
			if n == "lab-05" && hold {
				wantHere = shown{4, "policy"}
			}
			if got != wantHere {
				t.Errorf("%s: %s shows %v, want %v", n, reader, got, wantHere)
			}
			if (n != "lab-05" || hold) && got != charged {
				t.Errorf("%s: %s shows %v, the poll at that time charged %v", n, reader, got, charged)
			}
		}
	}
	// A rule edit moves an active hold's weight and the burn at once, with
	// the Setups list (A3), and the next poll charges it.
	if err := e.st.ReplaceRules(context.Background(), "rate", []store.Rule{{Selector: "tier=gold", Value: 6}}, nil, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	d4 := devs["lab-04"]
	hold4 := e.getOK("/api/v1/me/holds?state=active", &d4)["holds"].([]any)[0].(map[string]any)
	burn4 := e.getOK("/api/v1/me", &d4)["burning"].([]any)[0].(map[string]any)
	row4 := byName(t, e.getOK("/api/v1/setups", &d4)["setups"], "name", "lab-04")
	for what, got := range map[string]shown{"hold": shownOf(hold4), "my usage burn": shownOf(burn4), "setups": shownOf(row4)} {
		if got != (shown{6, "rule"}) {
			t.Errorf("lab-04 after a rule edit: %s shows %v, want 6 (rule)", what, got)
		}
	}
	e.clk.Advance(5 * time.Minute)
	e.pollOK()
	if s := e.lastSample(holdOf["lab-04"]); s.Weight != 6 || s.WeightSource != "rule" {
		t.Errorf("lab-04 charged %v (%v) after the rule edit", s.Weight, s.WeightSource)
	}
	// The cost per hour of the hold and of the Setups list agree with the burn.
	for _, n := range []string{"lab-01", "lab-02", "lab-03", "lab-04"} {
		d := devs[n]
		hold := e.getOK("/api/v1/me/holds?state=active", &d)["holds"].([]any)[0].(map[string]any)
		row := byName(t, e.getOK("/api/v1/setups", &d)["setups"], "name", n)
		me := e.getOK("/api/v1/me", &d)
		hc := hold["cost_per_hour_tokens"].(map[string]any)["now"]
		if hc != row["cost_per_hour_tokens"].(map[string]any)["now"] || hc != me["burn_per_hour_tokens"] {
			t.Errorf("%s: hold cost %v, setups cost %v, burn %v", n, hc, row["cost_per_hour_tokens"], me["burn_per_hour_tokens"])
		}
	}
}

// ---- WGT-15 ----

// lw is the ledger world of WGT-15, in Europe/Berlin (UTC+1 in November):
//   - hold A on lab-02 (rule 2): on local Nov 2, polls charged at reported
//     1.15, 1.15, reported 1.4, the rule's 2 after the report empties, and a
//     policy weight 3; on local Nov 3, two polls at the rule's 2 of which the
//     first sample is then deleted (pruned);
//   - hold B on lab-03 (default 1): every poll at 1; its first sample of local
//     Nov 3 is then given a null source (written before migration 0008).
type lw struct {
	*rEnv
	dev  sess
	a, b int64
	nov3 time.Time // the first poll of local Nov 3 (UTC 23:05 on Nov 2)
}

func newLedgerWeightsWorld(t *testing.T) *lw {
	t.Helper()
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	e := newREnv(t, time.Date(2026, 11, 2, 20, 0, 0, 0, time.UTC), berlin)
	w := &lw{rEnv: e, dev: e.newUser("ledger-dev", store.StatusActive, store.RoleDeveloper)}
	w.a, w.b = e.acquire(&w.dev, "lab-02"), e.acquire(&w.dev, "lab-03")
	e.prov.SetAllocations(hourglassAlloc("lab-02"), hourglassAlloc("lab-03"))
	step := func() { e.clk.Advance(5 * time.Minute); e.pollOK() }
	e.ok(`{"weights":[{"setup":"lab-02","weight":1.15}]}`)
	e.pollOK() // 20:00 metering starts
	step()     // 20:05 1.15
	step()     // 20:10 1.15
	e.clk.Advance(2 * time.Minute)
	e.ok(`{"weights":[{"setup":"lab-02","weight":1.4}]}`)
	e.clk.Advance(3 * time.Minute)
	e.pollOK() // 20:15 1.4
	e.clk.Advance(2 * time.Minute)
	e.ok(`{"weights":[]}`)
	e.clk.Advance(3 * time.Minute)
	e.pollOK() // 20:20 rule 2
	e.policy.SetFunc(func(h hourglass.HoldState, _ hourglass.Snapshot) hourglass.Decision {
		if h.Hold.ID == w.a {
			return hourglass.Decision{Meter: true, Weight: 3, Reason: "policy"}
		}
		return hourglass.Decision{Meter: true, Reason: "metered"}
	})
	step() // 20:25 policy 3
	e.policy.SetFunc(nil)
	for e.clk.Now().Before(time.Date(2026, 11, 2, 23, 5, 0, 0, time.UTC)) {
		step() // to 23:05Z: 00:05 local on Nov 3
	}
	w.nov3 = e.clk.Now()
	step() // 23:10Z
	e.exec(`DELETE FROM usage_samples WHERE hold_id = ? AND polled_at = ?`, w.a, store.FormatTime(w.nov3))
	e.exec(`UPDATE usage_samples SET weight_source = NULL WHERE hold_id = ? AND polled_at = ?`, w.b, store.FormatTime(w.nov3))
	return w
}

func (w *lw) ledger(query string) []map[string]any {
	w.t.Helper()
	path := "/api/v1/me/ledger"
	if query != "" {
		path += "?" + query
	}
	var out []map[string]any
	for _, x := range w.getOK(path, &w.dev)["entries"].([]any) {
		out = append(out, x.(map[string]any))
	}
	return out
}

// ledgerWeights is an entry's weight fields as {min, max, sources}.
func ledgerWeights(t *testing.T, e map[string]any) [3]any {
	t.Helper()
	for _, k := range ledgerWeightFields {
		if _, ok := e[k]; !ok {
			t.Fatalf("entry %v lacks %s", e["id"], k)
		}
	}
	return [3]any{e["weight_min"], e["weight_max"], e["weight_sources"]}
}

func srcs(s ...string) []any {
	out := make([]any, len(s))
	for i, x := range s {
		out[i] = x
	}
	return out
}

// dayOf is the local day of an entry's created_at.
func (w *lw) dayOf(e map[string]any) string {
	at, err := time.Parse(time.RFC3339, e["created_at"].(string))
	if err != nil {
		w.t.Fatal(err)
	}
	return at.In(w.deps.Location).Format("2006-01-02")
}

func holdOfEntry(e map[string]any) int64 {
	if h, ok := e["hold_id"].(float64); ok {
		return int64(h)
	}
	return 0
}

// The expected weights of the day entries of each hold: Nov 2 of A has every
// sample (a range and its sources, policy first); Nov 3 of A lacks a sample
// (unknown); Nov 3 of B has a sample without a source (sources unknown).
func (w *lw) wantDays(t *testing.T, view string, entries []map[string]any) {
	t.Helper()
	want := map[string][3]any{
		fmt.Sprintf("%d 2026-11-02", w.a): {1.15, float64(3), srcs("policy", "reported", "rule")},
		fmt.Sprintf("%d 2026-11-03", w.a): {nil, nil, nil},
		fmt.Sprintf("%d 2026-11-02", w.b): {float64(1), float64(1), srcs("default")},
		fmt.Sprintf("%d 2026-11-03", w.b): {float64(1), float64(1), nil},
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e["type"] != "debit" {
			if got := ledgerWeights(t, e); got != [3]any{nil, nil, nil} {
				t.Errorf("%s: %v entry %v has weights %v", view, e["type"], e["id"], got)
			}
			continue
		}
		k := fmt.Sprintf("%d %s", holdOfEntry(e), w.dayOf(e))
		wk, ok := want[k]
		if !ok {
			t.Fatalf("%s: unexpected debit entry %v", view, e)
		}
		seen[k] = true
		if got := ledgerWeights(t, e); !reflect.DeepEqual(got, wk) {
			t.Errorf("%s: day entry %s (entry_count %v) has weights %v, want %v", view, k, e["entry_count"], got, wk)
		}
	}
	if len(seen) != len(want) {
		t.Errorf("%s: day entries %v, want %v", view, seen, want)
	}
}

// A debit's ledger entry shows the lowest and highest weight and the sources
// of the polls it stands for, in the every-poll view, the day view and after
// storage compaction; null unless a sample is found for every poll, sources
// null when any of those samples has none, null for non-debits, and absent on
// the grant and reset responses.
func TestWGT15_LedgerEntriesShowTheWeightsOfTheirPolls(t *testing.T) {
	w := newLedgerWeightsWorld(t)
	t.Run("every-poll view", func(t *testing.T) {
		byTime := map[string][3]any{}
		for _, e := range w.ledger("") {
			if e["type"] != "debit" {
				if got := ledgerWeights(t, e); got != [3]any{nil, nil, nil} {
					t.Errorf("%v entry has weights %v", e["type"], got)
				}
				continue
			}
			if e["entry_count"] != float64(1) {
				t.Fatalf("entry %v stands for %v polls before compaction", e["id"], e["entry_count"])
			}
			if holdOfEntry(e) == w.a {
				byTime[e["created_at"].(string)] = ledgerWeights(t, e)
			}
		}
		for at, want := range map[string][3]any{
			"2026-11-02T20:05:00.000Z": {1.15, 1.15, srcs("reported")},
			"2026-11-02T20:10:00.000Z": {1.15, 1.15, srcs("reported")},
			"2026-11-02T20:15:00.000Z": {1.4, 1.4, srcs("reported")},
			"2026-11-02T20:20:00.000Z": {float64(2), float64(2), srcs("rule")},
			"2026-11-02T20:25:00.000Z": {float64(3), float64(3), srcs("policy")},
			"2026-11-02T23:05:00.000Z": {nil, nil, nil}, // its sample is gone
			"2026-11-02T23:10:00.000Z": {float64(2), float64(2), srcs("rule")},
		} {
			if got, ok := byTime[at]; !ok || !reflect.DeepEqual(got, want) {
				t.Errorf("raw debit at %s: weights %v (present %v), want %v", at, got, ok, want)
			}
		}
		// B's sample without a source: the weight is known, the sources not.
		for _, e := range w.ledger("") {
			if holdOfEntry(e) == w.b && e["created_at"] == store.FormatTime(w.nov3) {
				if got := ledgerWeights(t, e); !reflect.DeepEqual(got, [3]any{float64(1), float64(1), nil}) {
					t.Errorf("debit with a pre-migration sample: %v", got)
				}
			}
		}
	})
	t.Run("day view", func(t *testing.T) { w.wantDays(t, "rollup=day", w.ledger("rollup=day")) })
	t.Run("after compaction", func(t *testing.T) {
		w.clk.Advance(9 * 24 * time.Hour)
		replaced := 0
		for i := 0; i < 100; i++ {
			s, err := w.led.CompactDue(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			replaced += s.Replaced
			if s.Replaced == 0 && !s.More {
				break
			}
		}
		if replaced == 0 {
			t.Fatal("nothing was compacted")
		}
		every := w.ledger("")
		rolled := 0
		for _, e := range every {
			if e["entry_count"].(float64) > 1 {
				rolled++
			}
		}
		if rolled != 4 {
			t.Fatalf("%d rolled-up entries, want 4 (two holds, two days)", rolled)
		}
		w.wantDays(t, "every-poll view after compaction", every)
		w.wantDays(t, "day view after compaction", w.ledger("rollup=day"))
	})
	t.Run("absent on the grant and reset responses", func(t *testing.T) {
		base := fmt.Sprintf("/api/v1/admin/users/%d/", w.dev.user.ID)
		for _, c := range []struct{ path, body string }{
			{base + "grants", `{"amount_tokens":5,"reason":"x"}`}, {base + "reset", `{"reason":"x"}`},
		} {
			rec := w.serve("POST", c.path, &w.adm, true, c.body)
			if rec.Code != http.StatusCreated {
				t.Fatalf("%s: %d %s", c.path, rec.Code, rec.Body)
			}
			entry := decodeMap(t, rec)["entry"].(map[string]any)
			for _, k := range ledgerWeightFields {
				if _, has := entry[k]; has {
					t.Errorf("%s: entry has %s", c.path, k)
				}
			}
		}
	})
}

// ---- WGT-16 ----

// Attributes are stored and returned unchanged, are shown only for a usable
// entry (not when expired, out of bounds or off; still under an override),
// and no selector on the Setups list, rate rule or label index matches them.
func TestWGT16_AttributesAreForDisplayOnly(t *testing.T) {
	e := newREnv(t, wStart, nil)
	dev := e.newUser("attr-dev", store.StatusActive, store.RoleDeveloper)
	e.acquire(&dev, "lab-02")
	attrs := map[string]any{"memory-gb": "512", "storage-gb": "4000", "tier": "gold", "note": "a b, (c)=d!"}
	ab, _ := json.Marshal(attrs)
	e.ok(`{"weights":[{"setup":"lab-02","weight":1.15,"attributes":` + string(ab) + `},` +
		`{"setup":"lab-03","weight":0.5,"attributes":{"tier":"platinum","memory-gb":"64"}}]}`)

	shownAttrs := func() map[string]map[string]any {
		t.Helper()
		st, _ := e.state()
		list := e.getOK("/api/v1/setups", &dev)
		hold := e.getOK("/api/v1/me/holds", &dev)["holds"].([]any)[0].(map[string]any)
		return map[string]map[string]any{
			"state":          weightOf(t, st, "lab-02")["attributes"].(map[string]any),
			"setups":         byName(t, list["setups"], "name", "lab-02")["weight_attributes"].(map[string]any),
			"admin view":     e.putSetupOK("lab-02", "")["weight_attributes"].(map[string]any),
			"hold":           hold["weight_attributes"].(map[string]any),
			"setups lab-03":  byName(t, list["setups"], "name", "lab-03")["weight_attributes"].(map[string]any),
			"admin (lab-03)": byName(t, e.getOK("/api/v1/setups", &e.adm)["setups"], "name", "lab-03")["weight_attributes"].(map[string]any),
		}
	}
	lab03 := map[string]any{"tier": "platinum", "memory-gb": "64"}
	check := func(what string, want02, want03 map[string]any) {
		t.Helper()
		for reader, got := range shownAttrs() {
			want := want02
			if reader == "setups lab-03" || reader == "admin (lab-03)" {
				want = want03
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: %s attributes %v, want %v", what, reader, got, want)
			}
		}
	}
	empty := map[string]any{}
	check("usable", attrs, lab03)

	// No selector, rate rule or label index matches an attribute.
	list := e.getOK("/api/v1/setups?selector=tier%3Dplatinum", &dev)
	if n := names(list["setups"]); len(n) != 0 {
		t.Errorf("selector tier=platinum matched %v", n)
	}
	if n := names(e.getOK("/api/v1/setups?selector=memory-gb%3D512", &dev)["setups"]); len(n) != 0 {
		t.Errorf("selector memory-gb=512 matched %v", n)
	}
	if n := names(e.getOK("/api/v1/setups?selector=tier%3Dgold", &dev)["setups"]); !reflect.DeepEqual(n, []string{"lab-01", "lab-02"}) {
		t.Errorf("selector tier=gold matched %v", n)
	}
	idx := e.getOK("/api/v1/setups", &dev)["label_index"].([]any)
	for _, k := range idx {
		ko := k.(map[string]any)
		if ko["key"] != "tier" {
			t.Errorf("label index has key %v", ko["key"])
			continue
		}
		if vs := ko["values"].([]any); len(vs) != 1 || vs[0].(map[string]any)["value"] != "gold" {
			t.Errorf("label index values of tier: %v", vs)
		}
	}
	rules := e.getOK("/api/v1/admin/rules", &e.adm)
	if mc := rules["rules"].([]any)[0].(map[string]any)["match_count"]; mc != float64(2) {
		t.Errorf("tier=gold match_count %v, want 2 (labels only)", mc)
	}
	adm03 := byName(t, e.getOK("/api/v1/setups", &e.adm)["setups"], "name", "lab-03")["admin"].(map[string]any)
	if adm03["matched_rule_selector"] != nil {
		t.Errorf("lab-03 matched %v through its attributes", adm03["matched_rule_selector"])
	}

	// Still shown under an override.
	e.putSetupOK("lab-02", `,"weight_override":3`)
	check("override", attrs, lab03)
	e.putSetupOK("lab-02", `,"weight_override":null`)

	// Out of bounds: not shown; and lab-03 falls back to the default, not the
	// rule its attribute tier=platinum might suggest.
	e.setting(`{"enabled":true,"min":0.6,"max":20,"max_age":"10m"}`)
	check("lab-03 out of bounds", attrs, empty)
	if w := byName(t, e.getOK("/api/v1/setups", &dev)["setups"], "name", "lab-03"); w["weight"] != float64(1) || w["weight_source"] != "default" {
		t.Errorf("lab-03 fallback: %v %v", w["weight"], w["weight_source"])
	}
	e.setting(`{"enabled":true,"min":0,"max":1,"max_age":"10m"}`)
	check("lab-02 out of bounds", empty, lab03)
	e.setting(`{"enabled":true,"min":0,"max":20,"max_age":"10m"}`)
	check("back within the limits", attrs, lab03)

	// Off: not shown.
	e.setting(`{"enabled":false,"min":0,"max":20,"max_age":"10m"}`)
	check("off", empty, empty)
	e.setting(`{"enabled":true,"min":0,"max":20,"max_age":"10m"}`)

	// Expired: not shown.
	e.clk.Advance(11 * time.Minute)
	check("expired", empty, empty)
}

// ---- WGT-18 ----

// A setup's weight override pins its weight above a reported weight and the
// rate rules, with the feature on or off, and is not bound by the reported
// limits; 0 is allowed and invalid values are refused; setting and clearing
// are audited; on update an absent field leaves it and null clears it;
// requests without the field answer as before; the rules' match counts do
// not change.
func TestWGT18_WeightOverride(t *testing.T) {
	e := newREnv(t, wStart, nil)
	ctx := context.Background()
	e.ok(`{"weights":[{"setup":"lab-02","weight":1.15}]}`)
	rulesBefore := e.getOK("/api/v1/admin/rules", &e.adm)
	stored := func(name string) *float64 {
		t.Helper()
		s, err := e.st.GetSetup(ctx, e.setupID(name))
		if err != nil {
			t.Fatal(err)
		}
		return s.WeightOverride
	}
	view := func(name string) map[string]any {
		t.Helper()
		return byName(t, e.getOK("/api/v1/setups", &e.adm)["setups"], "name", name)
	}
	wantView := func(what string, v map[string]any, weight float64, source string, override, reported, notApplied any) {
		t.Helper()
		a := v["admin"].(map[string]any)
		if v["weight"] != weight || v["weight_source"] != source || a["weight_override"] != override ||
			a["reported_weight"] != reported || a["reported_weight_not_applied"] != notApplied {
			t.Errorf("%s: weight %v source %v override %v reported %v not_applied %v; want %v %v %v %v %v", what,
				v["weight"], v["weight_source"], a["weight_override"], a["reported_weight"], a["reported_weight_not_applied"],
				weight, source, override, reported, notApplied)
		}
	}
	audits := func() int {
		t.Helper()
		as, err := e.st.ListAudit(ctx, store.AuditFilter{})
		if err != nil {
			t.Fatal(err)
		}
		return len(as)
	}

	// Requests without the field answer as before: no override, the
	// reported weight and the rule apply.
	wantView("before", view("lab-02"), 1.15, "reported", nil, 1.15, nil)
	wantView("before", view("lab-01"), 2, "rule", nil, nil, nil)
	if v := e.putSetupOK("lab-01", ""); v["weight"] != float64(2) || v["weight_source"] != "rule" ||
		v["admin"].(map[string]any)["weight_override"] != nil || stored("lab-01") != nil {
		t.Errorf("PUT without the field: %v", v)
	}

	// Set: beats the reported weight and the rule, above the reported maximum.
	v := e.putSetupOK("lab-02", `,"weight_override":50`)
	wantView("PUT response", v, 50, "override", float64(50), 1.15, "override")
	wantView("set above the maximum", view("lab-02"), 50, "override", float64(50), 1.15, "override")
	e.putSetupOK("lab-01", `,"weight_override":0`)
	wantView("set to 0", view("lab-01"), 0, "override", float64(0), nil, nil)
	if c := view("lab-01")["cost_per_hour_tokens"].(map[string]any)["now"]; c != float64(0) {
		t.Errorf("cost with override 0: %v", c)
	}

	// Feature off: the override still applies; no reported weight is shown.
	e.setting(`{"enabled":false,"min":0,"max":20,"max_age":"10m"}`)
	wantView("off", view("lab-02"), 50, "override", float64(50), nil, nil)
	e.setting(`{"enabled":true,"min":0,"max":20,"max_age":"10m"}`)

	// A poll charges it.
	dev := e.newUser("ovr-dev", store.StatusActive, store.RoleDeveloper)
	h := e.acquire(&dev, "lab-02")
	e.prov.SetAllocations(hourglassAlloc("lab-02"))
	e.pollOK()
	e.clk.Advance(5 * time.Minute)
	e.pollOK()
	if s := e.lastSample(h); s.Weight != 50 || s.WeightSource != "override" {
		t.Errorf("sample %+v", s)
	}

	// Absent leaves it unchanged (also through a change of labels).
	e.putSetupOK("lab-02", "")
	if o := stored("lab-02"); o == nil || *o != 50 {
		t.Errorf("absent field changed the override to %v", o)
	}
	wantView("absent", view("lab-02"), 50, "override", float64(50), 1.15, "override")

	// Invalid values are refused, change nothing and audit nothing.
	n := audits()
	for _, bad := range []string{`-1`, `-0.5`, `1000000000001`, `1e400`, `"3"`, `true`, `{}`, `[1]`} {
		rec := e.putSetup("lab-02", `,"weight_override":`+bad)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("weight_override %s: %d %s", bad, rec.Code, rec.Body)
			continue
		}
		if _, field := e.refused(rec, http.StatusBadRequest, "validation"); field != "weight_override" {
			t.Errorf("weight_override %s: field %q", bad, field)
		}
	}
	rec := e.serve("POST", "/api/v1/admin/setups", &e.adm, true, `{"name":"lab-x","labels":{},"details":{},"enabled":true,"weight_override":-1}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST with -1: %d", rec.Code)
	}
	if o := stored("lab-02"); o == nil || *o != 50 || audits() != n {
		t.Errorf("a refused value changed something: override %v, audits %d -> %d", o, n, audits())
	}
	// The upper bound is the rule weight's, inclusive.
	e.putSetupOK("lab-03", `,"weight_override":1e12`)
	e.putSetupOK("lab-03", `,"weight_override":-0`)
	if o := stored("lab-03"); o == nil || *o != 0 {
		t.Errorf("-0 stored as %v", o)
	}

	// Audited: set and clear show in the setup's snapshot.
	e.putSetupOK("lab-02", `,"weight_override":null`)
	if stored("lab-02") != nil {
		t.Error("null did not clear the override")
	}
	wantView("cleared", view("lab-02"), 1.15, "reported", nil, 1.15, nil)
	ups, err := e.st.ListAudit(ctx, store.AuditFilter{Action: "setup.update"})
	if err != nil {
		t.Fatal(err)
	}
	var set, cleared bool
	for _, a := range ups {
		if a.Target != fmt.Sprintf("setup:%d", e.setupID("lab-02")) {
			continue
		}
		var before, after map[string]any
		_ = json.Unmarshal([]byte(a.BeforeJSON), &before)
		_ = json.Unmarshal([]byte(a.AfterJSON), &after)
		_, hadB := before["weight_override"]
		if !hadB && after["weight_override"] == float64(50) {
			set = true
		}
		if before["weight_override"] == float64(50) {
			if _, hasA := after["weight_override"]; !hasA {
				cleared = true
			}
		}
	}
	if !set || !cleared {
		t.Errorf("audit: set %v, cleared %v: %+v", set, cleared, ups)
	}

	// POST: absent and null mean none; a number sets it, audited.
	for _, c := range []struct {
		name, extra string
		want        *float64
	}{{"lab-p1", "", nil}, {"lab-p2", `,"weight_override":null`, nil}, {"lab-p3", `,"weight_override":7.5`, ptrF(7.5)}} {
		rec := e.serve("POST", "/api/v1/admin/setups", &e.adm, true,
			`{"name":"`+c.name+`","labels":{"tier":"gold"},"details":{},"enabled":true`+c.extra+`}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("POST %s: %d %s", c.name, rec.Code, rec.Body)
		}
		got := stored(c.name)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("POST %s: override %v", c.name, got)
		}
		v := decodeMap(t, rec)
		if c.want == nil && (v["weight"] != float64(2) || v["weight_source"] != "rule") {
			t.Errorf("POST %s without an override: %v %v", c.name, v["weight"], v["weight_source"])
		}
	}
	creates, _ := e.st.ListAudit(ctx, store.AuditFilter{Action: "setup.create"})
	found := false
	for _, a := range creates {
		if a.Target == fmt.Sprintf("setup:%d", e.setupID("lab-p3")) {
			var after map[string]any
			_ = json.Unmarshal([]byte(a.AfterJSON), &after)
			found = after["weight_override"] == 7.5
		}
	}
	if !found {
		t.Error("the create audit lacks the override")
	}

	// GET /api/v1/admin/rules keeps counting by selector alone.
	e.putSetupOK("lab-01", `,"weight_override":9`)
	rulesAfter := e.getOK("/api/v1/admin/rules", &e.adm)
	// lab-01, lab-02 and lab-p1 to lab-p3 are the enabled gold setups; three
	// of them have an override, and every one still counts.
	mc := func(m map[string]any) any { return m["rules"].([]any)[0].(map[string]any)["match_count"] }
	if mc(rulesBefore) != float64(2) || mc(rulesAfter) != float64(5) {
		t.Errorf("match_count before %v (want 2), after adding three gold setups and overrides %v (want 5)", mc(rulesBefore), mc(rulesAfter))
	}
}

func ptrF(f float64) *float64 { return &f }

// ---- WGT-22 ----

// The admin collector view shows the last accepted weights report with its
// entry count and how many stored weights the limits exclude, and the last
// refused report; both are null with the feature off.
func TestWGT22_CollectorViewShowsTheWeightsReportAndTheLastRefusal(t *testing.T) {
	e := newREnv(t, wStart, nil)
	col := func() map[string]any { return e.getOK("/api/v1/admin/collector", &e.adm) }
	if c := col(); c["last_weights_report"] != nil || c["last_weights_refusal"] != nil {
		t.Errorf("before any report: %v %v", c["last_weights_report"], c["last_weights_refusal"])
	}
	e.clk.Advance(time.Second)
	e.ok(`{"observed_at":"2026-11-23T10:00:00.500Z","weights":[{"setup":"lab-01","weight":1},{"setup":"lab-02","weight":20},{"setup":"gone","weight":0}]}`)
	c := col()
	want := map[string]any{"observed_at": "2026-11-23T10:00:00.500Z", "received_at": "2026-11-23T10:00:01.000Z",
		"service_account": "weigher", "entries": float64(3), "out_of_bounds": float64(0)}
	if !reflect.DeepEqual(c["last_weights_report"], want) || c["last_weights_refusal"] != nil {
		t.Errorf("after a report: %v %v", c["last_weights_report"], c["last_weights_refusal"])
	}
	e.clk.Advance(time.Second)
	msg, _ := e.refused(e.put(`{"weights":[{"setup":"lab-01","weight":21}]}`), http.StatusBadRequest, "validation")
	c = col()
	if !reflect.DeepEqual(c["last_weights_refusal"], map[string]any{"at": "2026-11-23T10:00:02.000Z", "service_account": "weigher", "message": msg}) {
		t.Errorf("refusal: %v", c["last_weights_refusal"])
	}
	if !reflect.DeepEqual(c["last_weights_report"], want) {
		t.Errorf("a refusal changed the report: %v", c["last_weights_report"])
	}
	// The limits are inclusive: max 20 keeps 20; min 1 excludes 0 only.
	e.setting(`{"enabled":true,"min":1,"max":20,"max_age":"10m"}`)
	if ob := col()["last_weights_report"].(map[string]any)["out_of_bounds"]; ob != float64(1) {
		t.Errorf("min 1: out_of_bounds %v, want 1", ob)
	}
	e.setting(`{"enabled":true,"min":1,"max":19.5,"max_age":"10m"}`)
	if ob := col()["last_weights_report"].(map[string]any)["out_of_bounds"]; ob != float64(2) {
		t.Errorf("min 1 max 19.5: out_of_bounds %v, want 2", ob)
	}
	// Off: both null; nothing is deleted, so on again shows both.
	e.setting(`{"enabled":false,"min":0,"max":20,"max_age":"10m"}`)
	if c := col(); c["last_weights_report"] != nil || c["last_weights_refusal"] != nil {
		t.Errorf("off: %v %v", c["last_weights_report"], c["last_weights_refusal"])
	}
	e.setting(`{"enabled":true,"min":0,"max":20,"max_age":"10m"}`)
	if c := col(); c["last_weights_report"] == nil || c["last_weights_refusal"] == nil {
		t.Errorf("on again: %v %v", c["last_weights_report"], c["last_weights_refusal"])
	}
}

// The design's Tests item end to end, with the admin's own routes:
// tightening the maximum excludes a stored weight at once (the count shows),
// the next report is refused whole (the refusal shows), and polls keep
// succeeding at the fallback weight.
func TestWGT22_TighteningTheMaximumShowsToTheAdmin(t *testing.T) {
	e := newREnv(t, wStart, nil)
	dev := e.newUser("tight-dev", store.StatusActive, store.RoleDeveloper)
	h := e.acquire(&dev, "lab-01")
	e.prov.SetAllocations(hourglassAlloc("lab-01"))
	e.ok(`{"weights":[{"setup":"lab-01","weight":15},{"setup":"lab-02","weight":5}]}`)
	e.pollOK()
	e.clk.Advance(5 * time.Minute)
	e.pollOK()
	if s := e.lastSample(h); s.Weight != 15 || s.WeightSource != "reported" {
		t.Fatalf("before: %+v", s)
	}
	// The admin lowers the maximum through the settings route.
	got := e.getOK("/api/v1/admin/settings", &e.adm)["settings"].(map[string]any)
	got["reported_weights"] = map[string]any{"enabled": true, "min": 0, "max": 10, "max_age": "10m"}
	body, _ := json.Marshal(got)
	if rec := e.serve("PUT", "/api/v1/admin/settings", &e.adm, true, string(body)); rec.Code != http.StatusOK {
		t.Fatalf("settings: %d %s", rec.Code, rec.Body)
	}
	c := e.getOK("/api/v1/admin/collector", &e.adm)
	if r := c["last_weights_report"].(map[string]any); r["entries"] != float64(2) || r["out_of_bounds"] != float64(1) {
		t.Errorf("after tightening: %v", r)
	}
	row := byName(t, e.getOK("/api/v1/setups", &e.adm)["setups"], "name", "lab-01")
	if row["weight"] != float64(2) || row["weight_source"] != "rule" ||
		row["admin"].(map[string]any)["reported_weight_not_applied"] != "out_of_bounds" {
		t.Errorf("setup view after tightening: %v", row)
	}
	e.clk.Advance(time.Minute)
	msg, _ := e.refused(e.put(`{"weights":[{"setup":"lab-01","weight":15},{"setup":"lab-02","weight":5}]}`), http.StatusBadRequest, "validation")
	c = e.getOK("/api/v1/admin/collector", &e.adm)
	if f := c["last_weights_refusal"].(map[string]any); f["message"] != msg || f["service_account"] != "weigher" ||
		f["at"] != store.FormatTime(e.clk.Now()) {
		t.Errorf("refusal: %v (message %q)", f, msg)
	}
	e.clk.Advance(4 * time.Minute)
	e.pollOK()
	if s := e.lastSample(h); s.Weight != 2 || s.WeightSource != "rule" {
		t.Errorf("after tightening: %+v", s)
	}
	e.clk.Advance(10 * time.Minute)
	e.pollOK()
	if s := e.lastSample(h); s.Weight != 2 || s.WeightSource != "rule" {
		t.Errorf("after max_age: %+v", s)
	}
}

// ---- WGT-23 ----

// A released hold shows the weight and source of its last usage sample and
// does not change when its setup's weight changes afterwards (a report, a
// rule edit, an override). One with no sample shows what its setup resolves
// to now; one whose last sample has no source shows a null source.
func TestWGT23_ReleasedHoldShowsItsLastCharge(t *testing.T) {
	e := newREnv(t, wStart, nil)
	dev := e.newUser("rel-dev", store.StatusActive, store.RoleDeveloper)
	h := e.acquire(&dev, "lab-02")
	e.prov.SetAllocations(hourglassAlloc("lab-02"))
	e.ok(`{"weights":[{"setup":"lab-02","weight":1.15,"attributes":{"memory-gb":"512"}}]}`)
	e.pollOK()
	e.clk.Advance(5 * time.Minute)
	e.pollOK()
	if rec := e.serve("POST", fmt.Sprintf("/api/v1/holds/%d/release", h), &dev, true, ""); rec.Code != http.StatusAccepted {
		t.Fatalf("release: %d %s", rec.Code, rec.Body)
	}
	e.prov.SetAllocations()
	e.clk.Advance(5 * time.Minute)
	e.ok(`{"weights":[{"setup":"lab-02","weight":1.15,"attributes":{"memory-gb":"512"}}]}`)
	e.pollOK() // completes the release, charging its last interval at 1.15
	if s := e.lastSample(h); s.Weight != 1.15 || s.WeightSource != "reported" {
		t.Fatalf("last sample %+v", s)
	}
	released := func(id int64) map[string]any {
		t.Helper()
		for _, x := range e.getOK("/api/v1/me/holds?state=released", &dev)["holds"].([]any) {
			if m := x.(map[string]any); int64(m["id"].(float64)) == id {
				return m
			}
		}
		t.Fatalf("hold %d is not released", id)
		return nil
	}
	want := func(what string, id int64, weight float64, source any) {
		t.Helper()
		m := released(id)
		if m["weight"] != weight || m["weight_source"] != source || !reflect.DeepEqual(m["weight_attributes"], map[string]any{}) ||
			m["active"] != false {
			t.Errorf("%s: released hold shows %v %v %v, want %v %v {}", what, m["weight"], m["weight_source"], m["weight_attributes"], weight, source)
		}
	}
	want("at release", h, 1.15, "reported")
	e.ok(`{"weights":[{"setup":"lab-02","weight":5}]}`)
	want("after a new report", h, 1.15, "reported")
	if err := e.st.ReplaceRules(context.Background(), "rate", []store.Rule{{Selector: "tier=gold", Value: 7}}, nil, e.clk.Now()); err != nil {
		t.Fatal(err)
	}
	e.ok(`{"weights":[]}`)
	want("after a rule edit", h, 1.15, "reported")
	e.putSetupOK("lab-02", `,"weight_override":9`)
	want("after an override", h, 1.15, "reported")
	if w := byName(t, e.getOK("/api/v1/setups", &dev)["setups"], "name", "lab-02"); w["weight"] != float64(9) {
		t.Errorf("the setup itself moved on: %v", w["weight"])
	}

	// A sample written before migration 0008 has no source.
	e.exec(`UPDATE usage_samples SET weight_source = NULL WHERE hold_id = ? AND polled_at = ?`, h, store.FormatTime(e.lastSample(h).PolledAt))
	want("pre-migration sample", h, 1.15, nil)

	// No sample: what the setup resolves to now.
	h3 := e.acquire(&dev, "lab-03")
	if rec := e.serve("POST", fmt.Sprintf("/api/v1/holds/%d/release", h3), &dev, true, ""); rec.Code != http.StatusOK {
		t.Fatalf("release without deployment: %d %s", rec.Code, rec.Body)
	}
	if m := released(h3); m["weight"] != float64(1) || m["weight_source"] != "default" {
		t.Errorf("released hold without a sample: %v %v", m["weight"], m["weight_source"])
	}
	e.putSetupOK("lab-03", `,"weight_override":4`)
	if m := released(h3); m["weight"] != float64(4) || m["weight_source"] != "override" {
		t.Errorf("released hold without a sample after an override: %v %v", m["weight"], m["weight_source"])
	}
}
