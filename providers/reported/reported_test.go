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
	"github.com/panset/hourglass/internal/holds"
	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/reported"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
	"github.com/panset/hourglass/pkg/storedreports"
	_ "github.com/panset/hourglass/policies/default"
	_ "github.com/panset/hourglass/pressure/inventory"
	_ "github.com/panset/hourglass/providers/reported"
	_ "github.com/panset/hourglass/providers/static"
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
	return newEnvOf(t, "reported", provCfg, "reported", pressCfg)
}

// newEnvOf is newEnv with chosen extensions. An empty provType means no
// provider is configured, which the collector accepts.
func newEnvOf(t *testing.T, provType, provCfg, pressType, pressCfg string) *env {
	t.Helper()
	e := &env{t: t, path: filepath.Join(t.TempDir(), "h.db"), clk: hourglasstest.NewFakeClock(time.Time{}), rec: &recorder{}}
	e.open()
	var prov hourglass.ResourceProvider
	if provType != "" {
		var err error
		if prov, err = hourglass.NewProvider(provType, json.RawMessage(provCfg)); err != nil {
			t.Fatal(err)
		}
	}
	press, err := hourglass.NewPressureSource(pressType, json.RawMessage(pressCfg))
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

// INT-5: the pressure source checks its own report's age. Deployments are
// fresh and their provider would answer; only the pressure report is old, and
// the poll must fail on it, write no sample and no debit, and emit poll.failed.
func TestINT5_StalePressureAloneFailsThePoll(t *testing.T) {
	e := newEnv(t, `{"max_age":"1h"}`, `{"max_age":"2m"}`)
	user, h := e.heldSetup("lab-01")
	e.deployments("lab-01")
	e.pressure(0.6)
	if err := e.c.RunOnce(bg); err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(time.Minute)
	e.deployments("lab-01")
	e.pressure(0.6)
	if err := e.c.RunOnce(bg); err != nil {
		t.Fatal(err)
	}
	samples, debits := e.samples(h), e.debits(user)
	if samples == 0 || debits == 0 {
		t.Fatalf("setup: samples=%d debits=%d, want some of each from two good polls", samples, debits)
	}

	// Pressure is 3m old (max_age 2m); deployments are fresh.
	e.clk.Advance(3 * time.Minute)
	e.deployments("lab-01")
	err := e.c.RunOnce(bg)
	if err == nil || !strings.Contains(err.Error(), "pressure report is 3m0s old") || !strings.Contains(err.Error(), "max_age 2m0s") {
		t.Fatalf("stale pressure: %v", err)
	}
	if e.rec.count(hourglass.EventPollFailed) != 1 || e.samples(h) != samples || e.debits(user) != debits {
		t.Fatalf("after stale pressure: failed=%d samples=%d (was %d) debits=%d (was %d)",
			e.rec.count(hourglass.EventPollFailed), e.samples(h), samples, e.debits(user), debits)
	}

	// A fresh pressure report makes the next poll succeed and charge.
	e.pressure(0.6)
	if err := e.c.RunOnce(bg); err != nil {
		t.Fatalf("fresh pressure: %v", err)
	}
	if e.debits(user) <= debits {
		t.Fatalf("debits after a fresh report = %d, want more than %d", e.debits(user), debits)
	}
}

// INT-5: with fresh deployments and no pressure report at all, the poll fails
// and charges nobody.
func TestINT5_NoPressureReportYetFailsThePoll(t *testing.T) {
	e := newEnv(t, `{}`, `{}`)
	user, h := e.heldSetup("lab-01")
	e.deployments("lab-01")
	err := e.c.RunOnce(bg)
	if err == nil || !strings.Contains(err.Error(), "no pressure report received yet") {
		t.Fatalf("got %v", err)
	}
	if e.rec.count(hourglass.EventPollFailed) != 1 || e.samples(h) != 0 || e.debits(user) != 0 {
		t.Fatalf("failed=%d samples=%d debits=%d", e.rec.count(hourglass.EventPollFailed), e.samples(h), e.debits(user))
	}
}

// INT-5, design "the two extensions work independently": the provider reads
// reports while the pressure comes from another source. No pressure report is
// ever sent and the poll must not ask for one.
func TestINT5_ReportedProviderWithAnotherPressureSource(t *testing.T) {
	e := newEnvOf(t, "reported", `{}`, "inventory", `{}`)
	user, h := e.heldSetup("lab-01")
	e.deployments("lab-01")
	if err := e.c.RunOnce(bg); err != nil {
		t.Fatalf("poll: %v", err)
	}
	got, _ := e.st.GetHold(bg, h.ID)
	if got.DeployedAt == nil || e.rec.count(hourglass.EventHoldDeployed) != 1 {
		t.Fatalf("hold not deployed from the report: %+v", got)
	}
	e.clk.Advance(time.Minute)
	e.deployments("lab-01")
	if err := e.c.RunOnce(bg); err != nil {
		t.Fatalf("second poll: %v", err)
	}
	if e.debits(user) == 0 || e.rec.count(hourglass.EventPollFailed) != 0 {
		t.Fatalf("debits=%d failed=%d", e.debits(user), e.rec.count(hourglass.EventPollFailed))
	}
	// The provider alone still goes stale.
	e.clk.Advance(11 * time.Minute)
	if err := e.c.RunOnce(bg); err == nil || !strings.Contains(err.Error(), "deployment report") {
		t.Fatalf("stale deployments: %v", err)
	}
}

// INT-5, same design sentence the other way round: the pressure is reported
// and there is no provider, or another one. No deployment report is needed.
func TestINT5_ReportedPressureWithNoProviderOrAnotherProvider(t *testing.T) {
	alloc := filepath.Join(t.TempDir(), "alloc.json")
	if err := os.WriteFile(alloc, []byte(`[{"resource_name":"lab-01"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, provType, provCfg string
		wantDeployed            bool
	}{
		{"no provider", "", ``, false},
		{"static provider", "static", `{"path":"` + alloc + `"}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnvOf(t, c.provType, c.provCfg, "reported", `{}`)
			user, h := e.heldSetup("lab-01")
			e.pressure(0.7) // no deployment report is ever sent
			if err := e.c.RunOnce(bg); err != nil {
				t.Fatalf("poll: %v", err)
			}
			if v := e.c.LastPoll(); !v.OK || v.RawPressure != 0.7 {
				t.Fatalf("last poll ok=%v raw pressure=%v, want the reported 0.7", v.OK, v.RawPressure)
			}
			got, _ := e.st.GetHold(bg, h.ID)
			if (got.DeployedAt != nil) != c.wantDeployed {
				t.Fatalf("deployed_at = %v, want deployed=%v", got.DeployedAt, c.wantDeployed)
			}
			e.clk.Advance(time.Minute)
			e.pressure(0.7)
			if err := e.c.RunOnce(bg); err != nil {
				t.Fatalf("second poll: %v", err)
			}
			if c.wantDeployed && e.debits(user) == 0 {
				t.Fatal("a deployed hold was not charged")
			}
			// The pressure alone goes stale.
			e.clk.Advance(11 * time.Minute)
			if err := e.c.RunOnce(bg); err == nil || !strings.Contains(err.Error(), "pressure report") {
				t.Fatalf("stale pressure: %v", err)
			}
		})
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

type noVeto struct{ *recorder }

func (noVeto) Veto(context.Context, hourglass.Event) error { return nil }

// D61 (REVIEW), part a. Whether a user's release is pending is decided from the
// last successful poll's view of the setup, not from the stored report. A
// report that lists the setup but has not been polled yet does not make the
// release pending; a report that no longer lists it does not make a release
// immediate until a poll has read it.
func TestHOLD6_ReleaseIsDecidedFromTheLastPollNotTheStoredReport(t *testing.T) {
	e := newEnv(t, `{}`, `{}`)
	svc := holds.NewService(holds.Deps{Store: e.st, Events: noVeto{e.rec}, ProviderConfigured: true, Clock: e.clk,
		Poll: func() holds.PollView { return e.c.LastPoll().ForHolds() }})
	user, h1 := e.heldSetup("lab-01")
	sid2, err := e.st.CreateSetup(bg, store.Setup{Name: "lab-02", Enabled: true, DetailsJSON: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	h2, err := e.st.CreateHold(bg, sid2, user, e.clk.Now())
	if err != nil {
		t.Fatal(err)
	}

	// The report lists lab-01, but no poll has read it: the release is immediate.
	e.pressure(0.6)
	e.deployments("lab-01")
	out, err := svc.Release(bg, user, h1.ID, holds.ReleaseOpts{})
	if err != nil || holds.Pending(out) || out.ReleasedAt == nil {
		t.Fatalf("release before any poll, report lists the setup: %+v, %v; want immediate", out, err)
	}

	// A poll reads a report that lists lab-02. The next report drops it, but no
	// poll has read that one: the release is still pending.
	e.deployments("lab-02")
	if err := e.c.RunOnce(bg); err != nil {
		t.Fatal(err)
	}
	e.deployments()
	out, err = svc.Release(bg, user, h2.ID, holds.ReleaseOpts{})
	if err != nil || !holds.Pending(out) || out.ReleasedAt != nil {
		t.Fatalf("release after the poll, report no longer lists the setup: %+v, %v; want pending", out, err)
	}
}
