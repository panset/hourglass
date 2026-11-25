// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/panset/hourglass/internal/auth"
	"github.com/panset/hourglass/pkg/hourglass"
)

// Compatibility with the release v1.1.0 (WGT-20, and the application level of
// WGT-19 in internal/app and the collector level in internal/collector). The
// fixtures in testdata/upgrade-v1.1.0 were written by that release's code; see
// the README there for how. These tests replay what the recorder asked and
// compare what today's code answers with what v1.1.0 answered.

const v110Dir = "testdata/upgrade-v1.1.0"

// v110Recorded is one recorded request and its answer.
type v110Recorded struct {
	Name      string `json:"name"`
	Principal string `json:"principal"`
	Request   struct {
		Method string `json:"method"`
		Path   string `json:"path"`
		Body   any    `json:"body"`
	} `json:"request"`
	Status   int `json:"status"`
	Response any `json:"response"`
}

type v110Doc struct {
	Now       time.Time      `json:"now"`
	Start     time.Time      `json:"start"`
	Timezone  string         `json:"timezone"`
	Facts     map[string]any `json:"facts"`
	Responses []v110Recorded `json:"responses"`
	After     []v110Recorded `json:"responses_after_polls"`
	Polls     []struct {
		At                  time.Time `json:"at"`
		FakePressureValue   float64   `json:"fake_pressure_value"`
		ProviderAllocations []string  `json:"provider_allocations"`
	} `json:"polls"`
}

// readV110 reads one fixture file. A missing file fails the test (never skips).
func readV110(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(findUp(t, v110Dir+"/"+name))
	if err != nil {
		t.Fatalf("the v1.1.0 fixture %s is missing: %v", name, err)
	}
	return b
}

func readV110Doc(t *testing.T, name string) v110Doc {
	t.Helper()
	var d v110Doc
	if err := json.Unmarshal(readV110(t, name), &d); err != nil {
		t.Fatal(err)
	}
	return d
}

// v110Secret is the secret of a fixture service token (the recorder's recSecret).
func v110Secret(label string) string {
	sum := sha256.Sum256([]byte("upgrade-v1.1.0-fixture/" + label))
	return hex.EncodeToString(sum[:])
}

// v110World is the new code on a copy of the release's database, with the
// release's runtime (names and policy) and its clock and zone.
type v110World struct {
	*uenv
	doc      v110Doc
	sessions map[string]*sess
	tokens   map[string]string
}

// newV110World starts at the recorded time of responses.json (the time of the
// last poll of the release's recording).
func newV110World(t *testing.T) *v110World {
	t.Helper()
	doc := readV110Doc(t, "responses.json")
	start := doc.Now
	loc, err := time.LoadLocation(doc.Timezone)
	if err != nil {
		t.Fatal(err)
	}
	b := newBackend(t, backendOpts{start: start, loc: loc, dbCopy: findUp(t, v110Dir+"/hourglass.sqlite3"),
		runtime: Runtime{Provider: "reported", PressureSource: "reported", MeteringPolicy: "fake-policy",
			ReportedProviderMaxAge: 10 * time.Minute, ReportedPressureMaxAge: 15 * time.Minute}})
	b.prov.SetName("reported")
	b.pressure.SetName("reported")
	b.policy.SetFunc(v110Policy)
	w := &v110World{uenv: &uenv{b}, doc: doc, sessions: map[string]*sess{}, tokens: map[string]string{}}
	ids := doc.Facts["user_ids"].(map[string]any)
	user := func(name string) *sess {
		u, err := w.st.GetUser(context.Background(), int64(ids[name].(float64)))
		if err != nil {
			t.Fatal(err)
		}
		s := w.session(u)
		return &s
	}
	w.sessions["developer"] = user("dev")
	w.sessions["developer (holder)"] = user("dev2")
	w.sessions["developer (no activity)"] = user("other")
	w.sessions["user with no role"] = user("norole")
	w.sessions["admin"] = user("admin")
	w.sessions["developer and admin"] = user("ops")
	w.sessions["developer session"] = w.sessions["developer"]
	// The fixture's own service tokens: the database holds their hashes, the
	// secrets derive from fixed labels.
	w.tokens["service account, every scope"] = auth.FormatServiceToken(1, v110Secret("reporter"))
	w.tokens["service account, state:read only"] = auth.FormatServiceToken(2, v110Secret("viewer"))
	return w
}

// v110Policy is the recorder's policy: it meters every hold, and the hold of
// lab-08 gets a weight of its own.
func v110Policy(h hourglass.HoldState, _ hourglass.Snapshot) hourglass.Decision {
	if h.Setup.Name == "lab-08" {
		return hourglass.Decision{Meter: true, Weight: 3.25, Reason: "policy weight"}
	}
	return hourglass.Decision{Meter: true, Reason: "test reason"}
}

// sendBody sends one recorded request with the given body as its recorded
// principal and returns the status and the decoded answer.
func (w *v110World) sendBody(t *testing.T, r v110Recorded, body string) (int, any) {
	t.Helper()
	var rec *httptest.ResponseRecorder
	switch {
	case w.tokens[r.Principal] != "":
		rec = w.serveService(r.Request.Method, r.Request.Path, w.tokens[r.Principal], body)
	case r.Principal == "no credential":
		rec = w.serve(r.Request.Method, r.Request.Path, nil, false, body)
	default:
		s := w.sessions[r.Principal]
		if s == nil {
			t.Fatalf("%s: no principal %q", r.Name, r.Principal)
		}
		rec = w.serve(r.Request.Method, r.Request.Path, s, r.Request.Method != "GET", body)
	}
	var got any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		got = rec.Body.String()
	}
	return rec.Code, got
}

// A difference between a recorded answer and today's.
type v110Diff struct {
	path     string // with an index in each array: a[0].b
	kind     string // "added", "removed" or "changed"
	was, now any
}

func (d v110Diff) String() string {
	switch d.kind {
	case "added":
		return fmt.Sprintf("added   %s = %s", d.path, jsonText(d.now))
	case "removed":
		return fmt.Sprintf("removed %s (was %s)", d.path, jsonText(d.was))
	}
	return fmt.Sprintf("changed %s: %s -> %s", d.path, jsonText(d.was), jsonText(d.now))
}

func jsonText(v any) string {
	b, _ := json.Marshal(v)
	if len(b) > 160 {
		return string(b[:160]) + "..."
	}
	return string(b)
}

// diffJSON lists the differences between two decoded JSON values.
func v110DiffJSON(path string, was, now any, out *[]v110Diff) {
	switch w := was.(type) {
	case map[string]any:
		n, ok := now.(map[string]any)
		if !ok {
			*out = append(*out, v110Diff{path, "changed", was, now})
			return
		}
		keys := map[string]bool{}
		for k := range w {
			keys[k] = true
		}
		for k := range n {
			keys[k] = true
		}
		var sorted []string
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			p := k
			if path != "" {
				p = path + "." + k
			}
			wv, inWas := w[k]
			nv, inNow := n[k]
			switch {
			case !inWas:
				*out = append(*out, v110Diff{p, "added", nil, nv})
			case !inNow:
				*out = append(*out, v110Diff{p, "removed", wv, nil})
			default:
				v110DiffJSON(p, wv, nv, out)
			}
		}
	case []any:
		n, ok := now.([]any)
		if !ok {
			*out = append(*out, v110Diff{path, "changed", was, now})
			return
		}
		for i := 0; i < len(w) || i < len(n); i++ {
			p := fmt.Sprintf("%s[%d]", path, i)
			switch {
			case i >= len(w):
				*out = append(*out, v110Diff{p, "added", nil, n[i]})
			case i >= len(n):
				*out = append(*out, v110Diff{p, "removed", w[i], nil})
			default:
				v110DiffJSON(p, w[i], n[i], out)
			}
		}
	default:
		if !reflect.DeepEqual(was, now) {
			*out = append(*out, v110Diff{path, "changed", was, now})
		}
	}
}

// generic replaces each array index in a path by [].
func generic(path string) string {
	var b strings.Builder
	for i := 0; i < len(path); i++ {
		if path[i] == '[' {
			j := strings.IndexByte(path[i:], ']')
			b.WriteString("[]")
			i += j
			continue
		}
		b.WriteByte(path[i])
	}
	return b.String()
}

// rawBodies are requests whose bytes matter: the fixture stores the decoded
// body, which loses the order of an object's keys, and the audit entry of a
// setup holds a hash of the details exactly as sent. These are the texts the
// recorder sent (recorder_upg_v110_record_test.go.txt); the test checks that
// each decodes to the recorded body.
var v110RawBodies = map[string]string{
	"admin_setup_post": `{"name":"lab-09","labels":{"tier":"gold","zone":"west"},"details":{"note":"opaque","n":[1,2]},"enabled":true}`,
}

// v110Allowed is what one route's answer may differ by from v1.1.0.
type v110Allowed struct {
	// added are the paths (array indexes written as []) of the fields today's
	// answer has and v1.1.0's did not; each must appear.
	added []string
	// changed are the named differences of a value.
	changed []v110Change
	// ignore are paths that differ on every run (a CSRF token is random).
	ignore []string
}

// v110Change is one named, deliberate change of a value: where, what v1.1.0
// said, what today's code says, and how many times.
type v110Change struct {
	name     string
	path     string
	was, now any
	times    int
}

var (
	// design C2: the fields of the Setups list; setups-declutter D94(b): pricing.
	addedSetupsList = []string{"reported_weights_enabled", "setups[].weight_source", "setups[].weight_attributes", "pricing"}
	// and the admin block of a setup (design C2).
	addedSetupsAdmin = append(append([]string{}, addedSetupsList...),
		"setups[].admin.reported_weight", "setups[].admin.reported_weight_not_applied", "setups[].admin.weight_override")
	// the admin setup answers of POST and PUT.
	addedAdminSetup = []string{"weight_source", "weight_attributes",
		"admin.reported_weight", "admin.reported_weight_not_applied", "admin.weight_override"}
	// design C2: a hold.
	addedHolds = []string{"holds[].weight_source", "holds[].weight_attributes"}
	// design C1: a ledger entry.
	addedLedger = []string{"entries[].weight_min", "entries[].weight_max", "entries[].weight_sources"}
	// design D1: the settings object.
	addedSettings = []string{"settings.reported_weights"}
	// design B4: the state route.
	addedState = []string{"weights", "setups[].weight"}
)

// The first deliberate change of design A3: a hold's weight and the burn
// figures follow the resolver now, not the hold's last usage sample. In the
// fixture the hold of lab-08 was priced by the policy (3.25), and its
// samples, written before the migration, have no source. A sample with no
// source counts as not `policy`, so until the first poll after the upgrade
// the hold shows the resolver's weight, the default 1.25, where v1.1.0 showed
// the sample's 3.25. The next poll writes a `policy` sample.
const nameA3PreUpgradePolicyHold = "A3 change 1: the hold of lab-08 was priced by the policy; its samples written before the migration have no source, so it shows the resolver's weight until the first poll"

var v110Routes = map[string]v110Allowed{
	"setups_dev":          {added: addedSetupsList},
	"setups_dev_selector": {added: addedSetupsList},
	"setups_dev2":         {added: addedSetupsList},
	"setups_admin":        {added: addedSetupsAdmin},
	"setups_norole":       {},
	"session_dev":         {ignore: []string{"csrf_token"}},
	"me_dev":              {added: []string{"burning[].weight_source"}},
	"me_dev2": {added: []string{"burning[].weight_source"}, changed: []v110Change{
		{nameA3PreUpgradePolicyHold, "burn_per_hour_tokens", 14.5, 10.5, 1},
		{nameA3PreUpgradePolicyHold, "burning[].tokens_per_hour", 6.5, 2.5, 1},
		{nameA3PreUpgradePolicyHold, "burning[].weight", 3.25, 1.25, 1},
	}},
	"me_other":     {},
	"me_admin":     {},
	"me_holds_dev": {added: addedHolds},
	"me_holds_dev2": {added: addedHolds, changed: []v110Change{
		{nameA3PreUpgradePolicyHold, "holds[].weight", 3.25, 1.25, 1},
		{nameA3PreUpgradePolicyHold, "holds[].cost_per_hour_tokens.now", 6.5, 2.5, 1},
		{nameA3PreUpgradePolicyHold, "holds[].cost_per_hour_tokens.at_peak", 9.75, 3.75, 1},
	}},
	"me_holds_dev2_active": {added: addedHolds, changed: []v110Change{
		{nameA3PreUpgradePolicyHold, "holds[].weight", 3.25, 1.25, 1},
		{nameA3PreUpgradePolicyHold, "holds[].cost_per_hour_tokens.now", 6.5, 2.5, 1},
		{nameA3PreUpgradePolicyHold, "holds[].cost_per_hour_tokens.at_peak", 9.75, 3.75, 1},
	}},
	"me_ledger_dev":               {added: addedLedger},
	"me_ledger_dev_day":           {added: addedLedger},
	"me_ledger_dev2":              {added: addedLedger},
	"me_ledger_dev2_day":          {added: addedLedger},
	"me_usage_dev":                {},
	"me_usage_dev2":               {},
	"pressure_dev":                {},
	"me_norole":                   {},
	"admin_users":                 {},
	"admin_users_ops":             {},
	"admin_users_dev":             {},
	"admin_rules":                 {},
	"admin_settings":              {added: addedSettings},
	"admin_collector":             {added: []string{"last_weights_report", "last_weights_refusal"}},
	"admin_service_accounts":      {},
	"integration_state_reporter":  {added: addedState},
	"integration_state_viewer":    {added: addedState},
	"integration_state_dev":       {},
	"integration_state_anonymous": {},
	"admin_settings_put":          {added: addedSettings},
	"admin_settings_put_dev":      {},
	"admin_setup_post":            {added: addedAdminSetup},
	"admin_setup_post_no_rule":    {added: addedAdminSetup},
	"admin_setup_put":             {added: addedAdminSetup},
	"admin_setup_put_held":        {added: addedAdminSetup},
	"admin_setup_post_dev":        {},
	"setups_admin_after_writes":   {added: addedSetupsAdmin},
	"admin_users_after_writes":    {},
	"admin_audit_after_writes":    {},
}

// The same routes after the three recorded polls, answered in the same
// process (poll.json responses_after_polls). By now the polls have written
// `policy` samples for the hold of lab-08, so its weight is the sample's again
// (no change from v1.1.0) and the source reads `policy`; Admin Users burn
// honours the policy's weight (the second deliberate change of A3).
const nameA3AdminUsersBurn = "A3 change 2: Admin Users burn honours a policy's weight (v1.1.0 used the rate rules)"

var v110RoutesAfterPolls = map[string]v110Allowed{
	"setups_dev_after_polls":     {added: addedSetupsList},
	"setups_admin_after_polls":   {added: addedSetupsAdmin},
	"me_holds_dev2_after_polls":  {added: addedHolds},
	"me_usage_dev_after_polls":   {},
	"me_ledger_dev2_after_polls": {added: addedLedger},
	"admin_users_after_polls": {changed: []v110Change{
		{nameA3AdminUsersBurn, "users[].burn_per_hour_tokens", 2.5, 6.5, 1},
	}},
	"admin_collector_after_polls": {added: []string{"last_weights_report", "last_weights_refusal"}},
}

// checkAgainstRelease compares today's answer with the recorded one and
// reports every difference that the route's list does not allow. A listed
// difference that did not happen is reported too, so a list cannot go stale.
func checkAgainstRelease(t *testing.T, r v110Recorded, status int, got any, allowed v110Allowed) {
	t.Helper()
	if status != r.Status {
		t.Errorf("%s: status %d, v1.1.0 answered %d (%s)", r.Name, status, r.Status, jsonText(got))
		return
	}
	var diffs []v110Diff
	v110DiffJSON("", r.Response, got, &diffs)
	addedSeen := map[string]bool{}
	changedSeen := make([]int, len(allowed.changed))
nextDiff:
	for _, d := range diffs {
		g := generic(d.path)
		for _, ig := range allowed.ignore {
			if g == ig {
				continue nextDiff
			}
		}
		switch d.kind {
		case "added":
			for _, a := range allowed.added {
				if g == a {
					addedSeen[a] = true
					continue nextDiff
				}
			}
		case "changed":
			for i, c := range allowed.changed {
				if g == c.path && reflect.DeepEqual(d.was, c.was) && reflect.DeepEqual(d.now, c.now) {
					changedSeen[i]++
					continue nextDiff
				}
			}
		}
		t.Errorf("%s: a difference from v1.1.0 that the list for this route does not allow: %s", r.Name, d)
	}
	for _, a := range allowed.added {
		if !addedSeen[a] {
			t.Errorf("%s: the allowed addition %s did not appear", r.Name, a)
		}
	}
	for i, c := range allowed.changed {
		if changedSeen[i] != c.times {
			t.Errorf("%s: the named change %q (%s: %v -> %v) happened %d times, want %d", r.Name, c.name, c.path, c.was, c.now, changedSeen[i], c.times)
		}
	}
}

// sendRecorded replays one recorded request, with the exact bytes of the
// raw ones.
func (w *v110World) sendRecorded(t *testing.T, r v110Recorded) (int, any) {
	t.Helper()
	if raw, ok := v110RawBodies[r.Name]; ok {
		var decoded any
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil || !reflect.DeepEqual(decoded, r.Request.Body) {
			t.Fatalf("%s: the raw body does not decode to the recorded one", r.Name)
		}
		return w.sendBody(t, r, raw)
	}
	b, err := json.Marshal(r.Request.Body)
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if r.Request.Body == nil {
		body = ""
	}
	return w.sendBody(t, r, body)
}

// WGT-20 (answers): with reported weights off and no override, every route of
// the recorded set answers as v1.1.0 did, replayed in the recorded order on a
// copy of its database, except for the fields each route's list names: the
// fields the design adds, and the first deliberate change of design A3. An
// unexpected new field, a changed value or a removed field fails.
func TestWGT20_ResponsesDifferFromV110OnlyByTheNamedAdditionsAndChanges(t *testing.T) {
	doc := readV110Doc(t, "responses.json")
	if len(doc.Responses) != len(v110Routes) {
		t.Fatalf("%d recorded requests, %d routes in the list of allowed differences", len(doc.Responses), len(v110Routes))
	}
	w := newV110World(t)
	for _, r := range doc.Responses {
		allowed, ok := v110Routes[r.Name]
		if !ok {
			t.Fatalf("no list of allowed differences for the recorded request %s", r.Name)
		}
		status, got := w.sendRecorded(t, r)
		checkAgainstRelease(t, r, status, got, allowed)
	}
	// The existing requests worked without the new fields: the settings PUT
	// with its eight keys and the setup writes without weight_override all
	// answered as recorded (status and body, above), and left the new things
	// at their defaults.
	set := w.getObj(t, "admin", "/api/v1/admin/settings")["settings"].(map[string]any)
	if !reflect.DeepEqual(set["reported_weights"], map[string]any{"enabled": false, "min": 0.0, "max": nil, "max_age": "10m"}) {
		t.Errorf("after the settings PUT without reported_weights the setting is %v, want the default", set["reported_weights"])
	}
	for _, s := range w.getObj(t, "admin", "/api/v1/setups")["setups"].([]any) {
		m := s.(map[string]any)
		if m["admin"].(map[string]any)["weight_override"] != nil {
			t.Errorf("setup %v has a weight override after writes that did not send one", m["name"])
		}
	}
}

// WGT-20 (named change): the deliberate change of A3 for a hold priced by a
// policy before the upgrade, and the way back. Right after the upgrade the
// hold of lab-08 shows the resolver's weight (1.25) and source `default`,
// where v1.1.0 showed its last sample's 3.25. After one poll on the new code
// the hold shows the policy's weight 3.25 and source `policy` again, in My
// holds, My usage burn and the Admin Users burn.
func TestWGT20_PolicyPricedHoldShowsResolverWeightUntilTheFirstPollThenThePolicyWeight(t *testing.T) {
	doc := readV110Doc(t, "responses.json")
	poll := readV110Doc(t, "poll.json")
	hold8 := int64(doc.Facts["hold_ids"].(map[string]any)["dev2/lab-08"].(float64))
	lab8 := func(w *v110World) map[string]any {
		t.Helper()
		var found map[string]any
		for _, h := range w.getObj(t, "developer (holder)", "/api/v1/me/holds")["holds"].([]any) {
			m := h.(map[string]any)
			if int64(m["id"].(float64)) == hold8 {
				found = m
			}
		}
		if found == nil {
			t.Fatal("the hold of lab-08 is not listed")
		}
		return found
	}
	burn := func(w *v110World) (me, admin float64) {
		t.Helper()
		me = w.getObj(t, "developer (holder)", "/api/v1/me")["burn_per_hour_tokens"].(float64)
		for _, u := range w.getObj(t, "admin", "/api/v1/admin/users")["users"].([]any) {
			m := u.(map[string]any)
			if m["username"] == "dev2" || m["email"] == "dev2@example.test" {
				admin = m["burn_per_hour_tokens"].(float64)
			}
		}
		return me, admin
	}

	w := newV110World(t)
	h := lab8(w)
	if h["weight"] != 1.25 || h["weight_source"] != "default" {
		t.Errorf("right after the upgrade the hold of lab-08 shows weight %v, source %v; want the resolver's 1.25 and default (v1.1.0 showed 3.25)", h["weight"], h["weight_source"])
	}
	if me, adm := burn(w); me != 10.5 || adm != 10.5 {
		t.Errorf("right after the upgrade the burn is %v (My usage) and %v (Admin Users), want 10.5 and 10.5 (v1.1.0: 14.5 and 10.5)", me, adm)
	}

	// One poll on the new code, the first of the recorded polls.
	p := poll.Polls[0]
	w.clk.Advance(5 * time.Minute)
	if !w.clk.Now().Equal(p.At) {
		t.Fatalf("clock %s, recorded %s", w.clk.Now(), p.At)
	}
	w.pressure.SetValue(p.FakePressureValue)
	var allocs []hourglass.Allocation
	for _, n := range p.ProviderAllocations {
		allocs = append(allocs, hourglass.Allocation{ResourceName: n})
	}
	w.prov.SetAllocations(allocs...)
	w.poll()
	h = lab8(w)
	if h["weight"] != 3.25 || h["weight_source"] != "policy" {
		t.Errorf("after one poll the hold of lab-08 shows weight %v, source %v; want the policy's 3.25 and policy", h["weight"], h["weight_source"])
	}
	if me, adm := burn(w); me != 14.5 || adm != 14.5 {
		t.Errorf("after one poll the burn is %v (My usage) and %v (Admin Users), want 14.5 and 14.5", me, adm)
	}
	// The setup itself is still priced by the resolver, not by the policy.
	for _, s := range w.getObj(t, "admin", "/api/v1/setups")["setups"].([]any) {
		m := s.(map[string]any)
		if m["name"] == "lab-08" && (m["weight"] != 1.25 || m["weight_source"] != "default") {
			t.Errorf("the setup lab-08 shows weight %v, source %v, want 1.25 and default", m["weight"], m["weight_source"])
		}
	}
}

// WGT-20 (answers after polls): after the three polls of poll.json replayed on
// today's code, the routes answered in the same process differ from what
// v1.1.0 answered only by the named additions and the second deliberate
// change of A3.
func TestWGT20_ResponsesAfterThePollsDifferFromV110OnlyByTheNamedAdditionsAndChanges(t *testing.T) {
	doc := readV110Doc(t, "poll.json")
	if len(doc.After) != len(v110RoutesAfterPolls) || len(doc.Polls) != 3 {
		t.Fatalf("%d recorded requests after the polls and %d polls; the list has %d routes", len(doc.After), len(doc.Polls), len(v110RoutesAfterPolls))
	}
	w := newV110World(t)
	for i, p := range doc.Polls {
		w.clk.Advance(5 * time.Minute)
		if !w.clk.Now().Equal(p.At) {
			t.Fatalf("poll %d: clock %s, recorded %s", i+1, w.clk.Now(), p.At)
		}
		w.pressure.SetValue(p.FakePressureValue)
		var allocs []hourglass.Allocation
		for _, n := range p.ProviderAllocations {
			allocs = append(allocs, hourglass.Allocation{ResourceName: n})
		}
		w.prov.SetAllocations(allocs...)
		w.poll()
	}
	for _, r := range doc.After {
		allowed, ok := v110RoutesAfterPolls[r.Name]
		if !ok {
			t.Fatalf("no list of allowed differences for the recorded request %s", r.Name)
		}
		status, got := w.sendRecorded(t, r)
		checkAgainstRelease(t, r, status, got, allowed)
	}
}

// getObj is a GET as one of the recorded principals.
func (w *v110World) getObj(t *testing.T, principal, path string) map[string]any {
	t.Helper()
	rec := w.serve("GET", path, w.sessions[principal], false, "")
	if rec.Code != 200 {
		t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body)
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// contractRequired lists, as strings, every request field a client must send
// (required in a request body schema) and every required parameter of an
// OpenAPI document, following $ref, allOf, oneOf and anyOf.
func contractRequired(t *testing.T, path string) map[string]bool {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	out := map[string]bool{}
	var walk func(prefix string, ref *openapi3.SchemaRef, seen map[*openapi3.Schema]bool)
	walk = func(prefix string, ref *openapi3.SchemaRef, seen map[*openapi3.Schema]bool) {
		if ref == nil || ref.Value == nil || seen[ref.Value] {
			return
		}
		s := ref.Value
		seen = copySeen(seen)
		seen[s] = true
		for _, name := range s.Required {
			out[prefix+"."+name] = true
		}
		for name, p := range s.Properties {
			walk(prefix+"."+name, p, seen)
		}
		if s.Items != nil {
			walk(prefix+"[]", s.Items, seen)
		}
		if s.AdditionalProperties.Schema != nil {
			walk(prefix+"{}", s.AdditionalProperties.Schema, seen)
		}
		for _, list := range []openapi3.SchemaRefs{s.AllOf, s.OneOf, s.AnyOf} {
			for _, b := range list {
				walk(prefix, b, seen)
			}
		}
	}
	for p, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			key := method + " " + p
			for _, pr := range append(append(openapi3.Parameters{}, item.Parameters...), op.Parameters...) {
				if pr.Value.Required {
					out["param "+key+" "+pr.Value.In+":"+pr.Value.Name] = true
				}
			}
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				if op.RequestBody.Value.Required {
					out["request body "+key] = true
				}
				for mt, c := range op.RequestBody.Value.Content {
					walk("request "+key+" "+mt, c.Schema, map[*openapi3.Schema]bool{})
				}
			}
		}
	}
	return out
}

// WGT-20 (contract): no route, parameter, request field, response code or
// response field of v1.1.0's api/openapi.yaml is removed or renamed in today's,
// and no request gained a required field, a required parameter or a required
// body. (TestUI2_ContractRoutesMatch and TestUI2_ContractResponsesMatchSchemas
// check that the file and the handlers agree.)
func TestWGT20_NoRouteParameterOrFieldOfV110IsRemovedAndNoRequestGainedARequiredField(t *testing.T) {
	oldPath := findUp(t, v110Dir+"/openapi.yaml")
	old := contractShape(t, oldPath)
	cur := contractShape(t, findUp(t, "api/openapi.yaml"))
	if len(old) < 300 {
		t.Fatalf("only %d items found in the release's contract; the walk is not reading it", len(old))
	}
	var missing []string
	for k := range old {
		if !cur[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d items of v1.1.0's api/openapi.yaml are not in today's file:\n%s", len(missing), strings.Join(missing, "\n"))
	}
	added := 0
	for k := range cur {
		if !old[k] {
			added++
		}
	}
	if added == 0 {
		t.Error("today's contract has nothing the release's lacks; the comparison reads the same file twice")
	}

	oldReq := contractRequired(t, oldPath)
	curReq := contractRequired(t, findUp(t, "api/openapi.yaml"))
	if len(oldReq) < 20 {
		t.Fatalf("only %d required items found in the release's contract; the walk is not reading it", len(oldReq))
	}
	var gained []string
	for k := range curReq {
		if !oldReq[k] && v110GainedByAnExistingRequest(old, k) {
			gained = append(gained, k)
		}
	}
	sort.Strings(gained)
	if len(gained) > 0 {
		t.Errorf("%d request items are required in today's file and were not in v1.1.0's:\n%s", len(gained), strings.Join(gained, "\n"))
	}
}

// v110GainedByAnExistingRequest says whether a newly required item (see
// contractRequired) can break a request that worked against v1.1.0. It cannot
// when the route is new, or when the item sits in an optional object that is
// new itself (the `reported_weights` setting: its four fields are required
// only when the key is sent, and the key is not required).
func v110GainedByAnExistingRequest(old map[string]bool, item string) bool {
	fields := strings.Fields(item)
	// "param METHOD /path in:name", "request body METHOD /path", "request METHOD /path mt.path".
	var method, path string
	switch {
	case fields[0] == "param":
		method, path = fields[1], fields[2]
	case fields[0] == "request" && fields[1] == "body":
		method, path = fields[2], fields[3]
	default:
		method, path = fields[1], fields[2]
	}
	if !old["route "+method+" "+path] {
		return false
	}
	if fields[0] != "request" || fields[1] == "body" {
		return true
	}
	rest := strings.Join(fields[3:], " ") // "application/json.a.b[].c"
	i := strings.LastIndex(rest, ".")
	container := strings.TrimSuffix(strings.TrimSuffix(rest[:i], "[]"), "{}")
	if !strings.Contains(container, ".") {
		return true // a field of the body itself
	}
	return old[strings.Join(fields[:3], " ")+" "+container]
}
