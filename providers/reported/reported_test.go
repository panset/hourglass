// SPDX-License-Identifier: MIT

package reported_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/app"
	"github.com/panset/hourglass/internal/collector"
	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/reported"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
	"github.com/panset/hourglass/pkg/storedreports"
	_ "github.com/panset/hourglass/policies/default"
	_ "github.com/panset/hourglass/providers/reported"
)

var bg = context.Background()

type recorder struct {
	mu sync.Mutex
	ev []hourglass.Event
}

func (r *recorder) Emit(e hourglass.Event) { r.mu.Lock(); r.ev = append(r.ev, e); r.mu.Unlock() }
func (r *recorder) count(typ string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.ev {
		if e.Type == typ {
			n++
		}
	}
	return n
}

type env struct {
	t    *testing.T
	path string
	st   *store.Store
	clk  *hourglasstest.FakeClock
	rec  *recorder
	rep  *reported.Service
	led  *ledger.Service
	c    *collector.Collector
}

// newEnv wires the real collector to the real "reported" extensions, the
// default policy and a real store, the way the server does.
func newEnv(t *testing.T, provCfg, pressCfg string) *env {
	t.Helper()
	e := &env{t: t, path: filepath.Join(t.TempDir(), "h.db"), clk: hourglasstest.NewFakeClock(time.Time{}), rec: &recorder{}}
	e.open()
	prov, err := hourglass.NewProvider("reported", json.RawMessage(provCfg))
	if err != nil {
		t.Fatal(err)
	}
	press, err := hourglass.NewPressureSource("reported", json.RawMessage(pressCfg))
	if err != nil {
		t.Fatal(err)
	}
	pol, err := hourglass.NewMeteringPolicy("default", nil)
	if err != nil {
		t.Fatal(err)
	}
	e.led = ledger.NewService(e.st, e.rec, e.clk, time.UTC)
	e.c = collector.New(collector.Deps{Store: e.st, Provider: prov, Pressure: press, Policy: pol,
		Events: e.rec, Ledger: e.led, Clock: e.clk, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	return e
}

// open opens the database and publishes the stored-report window.
func (e *env) open() {
	e.t.Helper()
	st, err := store.Open(e.path)
	if err != nil {
		e.t.Fatal(err)
	}
	e.st = st
	e.rep = reported.New(st, e.clk)
	restore := storedreports.Publish(app.NewStoredReader(st, e.clk))
	e.t.Cleanup(func() { restore(); st.Close() })
}

func (e *env) deployments(names ...string) {
	e.t.Helper()
	var d []reported.Deployment
	for _, n := range names {
		d = append(d, reported.Deployment{Setup: n, Labels: map[string]string{"k": n}})
	}
	if _, err := e.rep.SubmitDeployments(bg, "svc", nil, d); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) pressure(v float64) {
	e.t.Helper()
	if _, err := e.rep.SubmitPressure(bg, "svc", v, nil); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) heldSetup(name string) (int64, store.Hold) {
	e.t.Helper()
	u, err := e.st.CreateOIDCUser(bg, "dev", "dev@example.test", "dev", e.clk.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.st.SetUserRoles(bg, u.ID, []string{store.RoleDeveloper}); err != nil {
		e.t.Fatal(err)
	}
	sid, err := e.st.CreateSetup(bg, store.Setup{Name: name, Enabled: true, DetailsJSON: "{}"})
	if err != nil {
		e.t.Fatal(err)
	}
	h, err := e.st.CreateHold(bg, sid, u.ID, e.clk.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	return u.ID, h
}

func (e *env) samples(h store.Hold) int {
	e.t.Helper()
	s, err := e.st.ListUsageSamples(bg, h.ID)
	if err != nil {
		e.t.Fatal(err)
	}
	return len(s)
}

func (e *env) debits(user int64) int {
	e.t.Helper()
	l, err := e.st.ListLedger(bg, user, e.led.CurrentPeriod())
	if err != nil {
		e.t.Fatal(err)
	}
	n := 0
	for _, x := range l {
		if x.Type == "debit" {
			n++
		}
	}
	return n
}

func TestINT5_NoReportThenStaleThenFresh(t *testing.T) {
	e := newEnv(t, `{}`, `{}`)
	user, h := e.heldSetup("lab-01")

	// No report at all.
	if err := e.c.RunOnce(bg); err == nil || !strings.Contains(err.Error(), "no deployment report received yet") {
		t.Fatalf("no report: %v", err)
	}
	if e.rec.count(hourglass.EventPollFailed) != 1 || e.samples(h) != 0 || e.debits(user) != 0 {
		t.Fatalf("after no report: failed=%d samples=%d debits=%d", e.rec.count(hourglass.EventPollFailed), e.samples(h), e.debits(user))
	}

	// A fresh deployment report but no pressure report: the pressure source fails.
	e.deployments("lab-01")
	if err := e.c.RunOnce(bg); err == nil || !strings.Contains(err.Error(), "no pressure report received yet") {
		t.Fatalf("no pressure report: %v", err)
	}

	// Both reported, then they go stale.
	e.pressure(0.6)
	e.clk.Advance(11 * time.Minute)
	err := e.c.RunOnce(bg)
	if err == nil || !strings.Contains(err.Error(), "11m0s old") || !strings.Contains(err.Error(), "max_age 10m0s") {
		t.Fatalf("stale: %v", err)
	}
	if e.rec.count(hourglass.EventPollFailed) != 3 || e.samples(h) != 0 || e.debits(user) != 0 {
		t.Fatalf("after stale: failed=%d samples=%d debits=%d", e.rec.count(hourglass.EventPollFailed), e.samples(h), e.debits(user))
	}

	// A fresh report succeeds.
	e.deployments("lab-01")
	e.pressure(0.6)
	if err := e.c.RunOnce(bg); err != nil {
		t.Fatalf("fresh: %v", err)
	}
	if e.samples(h) != 1 {
		t.Fatalf("samples after fresh poll = %d, want 1", e.samples(h))
	}
}

func TestINT5_StaleDeploymentsAloneFailTheProviderOnly(t *testing.T) {
	e := newEnv(t, `{"max_age":"2m"}`, `{"max_age":"1h"}`)
	e.heldSetup("lab-01")
	e.deployments("lab-01")
	e.pressure(0.6)
	e.clk.Advance(3 * time.Minute)
	if err := e.c.RunOnce(bg); err == nil || !strings.Contains(err.Error(), "deployment report is 3m0s old") {
		t.Fatalf("got %v", err)
	}
	e.deployments("lab-01")
	if err := e.c.RunOnce(bg); err != nil {
		t.Fatalf("pressure within its own max_age: %v", err)
	}
}

func TestINT6_ReportsSurviveReopen(t *testing.T) {
	e := newEnv(t, `{}`, `{}`)
	e.deployments("lab-01", "lab-02")
	e.pressure(0.72)
	if err := e.st.Close(); err != nil {
		t.Fatal(err)
	}
	e.open() // reopens the same file and republishes
	e.clk.Advance(5 * time.Minute)

	prov, _ := hourglass.NewProvider("reported", nil)
	press, _ := hourglass.NewPressureSource("reported", nil)
	got, err := prov.List(bg)
	if err != nil || len(got) != 2 {
		t.Fatalf("List after reopen: %v %v", got, err)
	}
	v, err := press.Pressure(bg, hourglass.Snapshot{})
	if err != nil || v != 0.72 {
		t.Fatalf("Pressure after reopen: %v %v", v, err)
	}
	// Still subject to max_age.
	e.clk.Advance(6 * time.Minute)
	if _, err := prov.List(bg); err == nil {
		t.Fatal("stale deployments after reopen returned")
	}
	if _, err := press.Pressure(bg, hourglass.Snapshot{}); err == nil {
		t.Fatal("stale pressure after reopen returned")
	}
}

func TestINT7_ReportedSetupWithHoldGetsDeployedAndMeteringStarts(t *testing.T) {
	e := newEnv(t, `{}`, `{}`)
	_, h := e.heldSetup("lab-01")
	e.pressure(0.6)

	e.deployments() // nothing deployed yet
	if err := e.c.RunOnce(bg); err != nil {
		t.Fatal(err)
	}
	got, _ := e.st.GetHold(bg, h.ID)
	if got.DeployedAt != nil || got.MeteringStartedAt != nil || e.rec.count(hourglass.EventHoldDeployed) != 0 {
		t.Fatalf("before report: %+v", got)
	}

	e.clk.Advance(5 * time.Minute)
	e.deployments("lab-01")
	e.pressure(0.6)
	if err := e.c.RunOnce(bg); err != nil {
		t.Fatal(err)
	}
	at := e.clk.Now().UTC().Truncate(time.Millisecond)
	got, _ = e.st.GetHold(bg, h.ID)
	if got.DeployedAt == nil || !got.DeployedAt.Equal(at) {
		t.Fatalf("deployed_at = %v, want %v", got.DeployedAt, at)
	}
	if got.MeteringStartedAt == nil || !got.MeteringStartedAt.Equal(at) {
		t.Fatalf("metering_started_at = %v, want %v", got.MeteringStartedAt, at)
	}
	if e.rec.count(hourglass.EventHoldDeployed) != 1 {
		t.Fatalf("hold.deployed fired %d times", e.rec.count(hourglass.EventHoldDeployed))
	}
}

func TestINT7_ReportedSetupWithNoHoldIsAnOrphan(t *testing.T) {
	e := newEnv(t, `{}`, `{}`)
	if _, err := e.st.CreateSetup(bg, store.Setup{Name: "lab-free", Enabled: true, DetailsJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	e.pressure(0.6)
	e.deployments("lab-free")
	if err := e.c.RunOnce(bg); err != nil {
		t.Fatal(err)
	}
	orphans, err := e.st.ListOrphans(bg)
	if err != nil || len(orphans) != 1 || orphans[0].ResourceName != "lab-free" {
		t.Fatalf("orphans = %+v, %v", orphans, err)
	}
}

func TestINT5_ProviderMapsEachDeploymentWithLabels(t *testing.T) {
	e := newEnv(t, `{}`, `{}`)
	e.deployments("a", "b")
	prov, _ := hourglass.NewProvider("reported", nil)
	got, err := prov.List(bg)
	if err != nil || len(got) != 2 || got[0].ResourceName != "a" || got[0].Labels["k"] != "a" || got[1].ResourceName != "b" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestINT5_Config(t *testing.T) {
	cases := []struct {
		cfg string
		ok  bool
	}{
		{``, true}, {`null`, true}, {`{}`, true}, {`{"max_age":"1m"}`, true}, {`{"max_age":"2h"}`, true},
		{`{"max_age":"59s"}`, false}, {`{"max_age":"soon"}`, false}, {`{"max_age":10}`, false},
		{`{"other":1}`, false}, {`{`, false},
	}
	for _, c := range cases {
		if _, err := hourglass.NewProvider("reported", json.RawMessage(c.cfg)); (err == nil) != c.ok {
			t.Errorf("provider %q: err=%v", c.cfg, err)
		}
		if _, err := hourglass.NewPressureSource("reported", json.RawMessage(c.cfg)); (err == nil) != c.ok {
			t.Errorf("pressure %q: err=%v", c.cfg, err)
		}
	}
}

func TestEXT5_Conformance(t *testing.T) {
	e := newEnv(t, `{}`, `{}`)
	e.deployments("a", "b")
	e.pressure(0.4)
	hourglasstest.TestProvider(t, func() hourglass.ResourceProvider {
		p, err := hourglass.NewProvider("reported", nil)
		if err != nil {
			t.Fatal(err)
		}
		return p
	})
	hourglasstest.TestPressureSource(t, func() hourglass.PressureSource {
		p, err := hourglass.NewPressureSource("reported", nil)
		if err != nil {
			t.Fatal(err)
		}
		return p
	})
}

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

func TestCheckProviderPrintsReportAges(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "h.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	rep := reported.New(st, wallClock{})
	obs := time.Now().Add(-3 * time.Minute)
	if _, err := rep.SubmitDeployments(bg, "svc", &obs, []reported.Deployment{{Setup: "lab-01"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := rep.SubmitPressure(bg, "svc", 0.5, &obs); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "config.yaml")
	body := "database: " + dbPath + "\nprovider: {type: reported}\npressure: {type: reported}\n"
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := app.CheckProvider(bg, app.CheckOptions{Args: []string{"-config", cfg}, Stdout: &out}); err != nil {
		t.Fatalf("check-provider: %v\n%s", err, out.String())
	}
	for _, want := range []string{"deployment report: age 3m", "pressure report: age 3m", "raw pressure: 0.5000"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}
