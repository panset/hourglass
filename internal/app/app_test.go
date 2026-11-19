// SPDX-License-Identifier: MIT

package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/panset/hourglass/internal/app/runhook"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

const (
	testPressure = "app-test-pressure"
	testPolicy   = "app-test-policy"
	testHook     = "app-test-hook"
	adminPW      = "a-long-initial-admin-password"
)

var registerOnce sync.Once

func registerFakes() {
	registerOnce.Do(func() {
		hourglass.RegisterPressureSource(testPressure, func(json.RawMessage) (hourglass.PressureSource, error) {
			p := hourglasstest.NewFakePressureSource()
			p.SetName(testPressure)
			p.SetValue(0.4)
			return p, nil
		})
		hourglass.RegisterMeteringPolicy(testPolicy, func(json.RawMessage) (hourglass.MeteringPolicy, error) {
			p := hourglasstest.NewFakePolicy()
			p.SetName(testPolicy)
			return p, nil
		})
		hourglass.RegisterHook(testHook, func(json.RawMessage) (hourglass.Hook, error) {
			h := hourglasstest.NewFakeHook()
			h.SetName(testHook)
			return h, nil
		})
	})
}

// syncBuf is a writer a test can read while Run still logs to it.
type syncBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// logLine returns the first log record with the given msg, waiting for it.
func (s *syncBuf) logLine(t *testing.T, msg string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		sc := bufio.NewScanner(strings.NewReader(s.String()))
		for sc.Scan() {
			var m map[string]any
			if json.Unmarshal([]byte(sc.Text()), &m) == nil && m["msg"] == msg {
				return m
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("log line %q not seen; log:\n%s", msg, s.String())
	return nil
}

type running struct {
	t      *testing.T
	out    *syncBuf
	cancel context.CancelFunc
	done   chan error
	base   string
	dbPath string
}

// writeConfig writes a config file with defaults for tests plus extra YAML.
func writeConfig(t *testing.T, extra string) (cfgPath, dbPath string) {
	t.Helper()
	dir := t.TempDir()
	dbPath = filepath.Join(dir, "h.db")
	body := fmt.Sprintf("listen: 127.0.0.1:0\ndatabase: %s\nsession_key: %s\nadmin_password: %s\n%s",
		dbPath, strings.Repeat("s", 40), adminPW, extra)
	cfgPath = filepath.Join(dir, "hourglass.yaml")
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfgPath, dbPath
}

func start(t *testing.T, cfgPath, dbPath string) *running {
	t.Helper()
	return startWithClock(t, cfgPath, dbPath, nil)
}

func startWithClock(t *testing.T, cfgPath, dbPath string, clock interface{ Now() time.Time }) *running {
	t.Helper()
	registerFakes()
	ctx, cancel := context.WithCancel(context.Background())
	r := &running{t: t, out: &syncBuf{}, cancel: cancel, done: make(chan error, 1), dbPath: dbPath}
	go func() {
		err := Run(ctx, runhook.Options{
			ConfigPath: cfgPath, DefaultPressureSource: testPressure, DefaultMeteringPolicy: testPolicy,
			Stdout: r.out, Stderr: io.Discard, Clock: clock,
		})
		if err != nil {
			fmt.Fprintf(r.out, "run returned an error: %v\n", err)
		}
		r.done <- err
	}()
	t.Cleanup(func() { cancel(); <-r.done })
	line := r.out.logLine(t, "http listening")
	r.base = "http://" + line["addr"].(string)
	return r
}

func (r *running) stop() error {
	r.cancel()
	select {
	case err := <-r.done:
		r.done <- err // keep Cleanup from blocking
		return err
	case <-time.After(ShutdownTimeout + 5*time.Second):
		r.t.Fatal("Run did not return after cancel")
		return nil
	}
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestEXT4_RunStartsWithoutProviderOrOIDCAndShutsDownCleanly(t *testing.T) {
	cfg, db := writeConfig(t, "")
	r := start(t, cfg, db)

	code, body := get(t, r.base+"/healthz")
	if code != 200 || !strings.Contains(body, `"database":"ok"`) {
		t.Fatalf("healthz: %d %s", code, body)
	}
	if code, body := get(t, r.base+"/api/v1/auth/methods"); code != 200 || strings.TrimSpace(body) != `{"oidc":false,"local":true}` {
		t.Fatalf("methods: %d %s", code, body)
	}
	// The collector loop runs without a provider: a poll succeeds and healthz reports its age.
	deadline := time.Now().Add(10 * time.Second)
	for {
		_, body := get(t, r.base+"/healthz")
		if !strings.Contains(body, `"last_poll_age_seconds":null`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no successful poll within 10s: %s", body)
		}
		time.Sleep(20 * time.Millisecond)
	}
	// The local admin can sign in with the configured initial password.
	resp, err := http.Post(r.base+"/auth/local", "application/json",
		strings.NewReader(`{"username":"admin","password":"`+adminPW+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("admin sign-in: %d", resp.StatusCode)
	}
	if code, body := get(t, r.base+"/api/v1/me"); code != 401 {
		t.Fatalf("unauthenticated /me: %d %s", code, body)
	}

	if err := r.stop(); err != nil {
		t.Fatalf("Run returned %v after cancel", err)
	}
	// Every line the process wrote is one JSON object; the shutdown is logged in order.
	var msgs []string
	sc := bufio.NewScanner(strings.NewReader(r.out.String()))
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("log line is not JSON: %q", sc.Text())
		}
		msgs = append(msgs, fmt.Sprint(m["msg"]))
	}
	order := strings.Join(msgs, "|")
	if !strings.Contains(order, "shutting down|") || !strings.HasSuffix(order, "|stopped") {
		t.Errorf("log messages: %s", order)
	}
	// The database holds the admin, with last_login_at set by the sign-in.
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	admin, err := st.GetUserByUsername(context.Background(), "admin")
	if err != nil || admin.AuthSource != store.AuthLocal || admin.LastLoginAt == nil {
		t.Fatalf("admin %+v %v", admin, err)
	}
	// Secrets never reach the log.
	for _, s := range []string{adminPW, strings.Repeat("s", 40)} {
		if strings.Contains(r.out.String(), s) {
			t.Errorf("log contains a secret")
		}
	}
}

func TestEXT2_UnknownExtensionNameFailsStartupListingRegisteredNames(t *testing.T) {
	registerFakes()
	cases := map[string]struct {
		extra string
		want  []string
	}{
		"pressure": {"pressure: {type: nope}\n", []string{`unknown pressure source "nope"`, testPressure}},
		"policy":   {"metering: {type: nope}\n", []string{`unknown metering policy "nope"`, testPolicy}},
		"provider": {"provider: {type: nope}\n", []string{`unknown provider "nope"`, "registered: none"}},
		"hook":     {"hooks: [{type: " + testHook + "}, {type: nope}]\n", []string{`unknown hook "nope"`, testHook}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, db := writeConfig(t, c.extra)
			err := Run(context.Background(), runhook.Options{
				ConfigPath: cfg, DefaultPressureSource: testPressure, DefaultMeteringPolicy: testPolicy,
				Stdout: io.Discard, Stderr: io.Discard,
			})
			if err == nil {
				t.Fatal("Run succeeded")
			}
			for _, w := range c.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q lacks %q", err, w)
				}
			}
			if _, statErr := os.Stat(db); !errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("a database file was created before the extensions were validated")
			}
		})
	}
	// A default the caller supplies that is not registered fails the same way.
	cfg, _ := writeConfig(t, "")
	err := Run(context.Background(), runhook.Options{ConfigPath: cfg, DefaultPressureSource: "missing-default",
		DefaultMeteringPolicy: testPolicy, Stdout: io.Discard, Stderr: io.Discard})
	if err == nil || !strings.Contains(err.Error(), `unknown pressure source "missing-default"`) {
		t.Errorf("err = %v", err)
	}
}

func TestSET7_RunImportsSeedOnFirstStartOnly(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.yaml")
	if err := os.WriteFile(seed, []byte("setups: [{name: s1, labels: {a: b}}, {name: s2}]\nrate_rules: [{selector: 'a=b', weight: 2}]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, db := writeConfig(t, "seed: "+seed+"\n")
	r := start(t, cfg, db)
	r.out.logLine(t, "seed file imported")
	if err := r.stop(); err != nil {
		t.Fatal(err)
	}
	// Second start: the seed is not applied again.
	r2 := start(t, cfg, db)
	time.Sleep(50 * time.Millisecond)
	if strings.Contains(r2.out.String(), "seed file imported") {
		t.Error("seed applied twice")
	}
	if err := r2.stop(); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if s, _ := st.ListSetups(context.Background(), false); len(s) != 2 {
		t.Errorf("setups = %d", len(s))
	}
}

func TestSET7_RunFailsOnInvalidSeedAndImportsNothing(t *testing.T) {
	registerFakes()
	seed := filepath.Join(t.TempDir(), "seed.yaml")
	if err := os.WriteFile(seed, []byte("setups: [{name: s1}]\nrate_rules: [{selector: 'a in (', weight: 1}]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, db := writeConfig(t, "seed: "+seed+"\n")
	err := Run(context.Background(), runhook.Options{ConfigPath: cfg, DefaultPressureSource: testPressure,
		DefaultMeteringPolicy: testPolicy, Stdout: io.Discard, Stderr: io.Discard})
	var se *SeedError
	if !errors.As(err, &se) {
		t.Fatalf("err = %v", err)
	}
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if s, _ := st.ListSetups(context.Background(), false); len(s) != 0 {
		t.Errorf("partial seed: %d setups", len(s))
	}
}

func TestNFR_MetricsOnSeparateListenerOnly(t *testing.T) {
	// Off by default: nothing listens, and the public port has no /metrics.
	cfg, db := writeConfig(t, "")
	r := start(t, cfg, db)
	if code, _ := get(t, r.base+"/metrics"); code != 404 {
		t.Errorf("public /metrics = %d, want 404", code)
	}
	if strings.Contains(r.out.String(), "metrics listening") {
		t.Error("metrics listener started by default")
	}
	_ = r.stop()

	cfg, db = writeConfig(t, "metrics: {listen: 'localhost:0'}\n")
	r = start(t, cfg, db)
	maddr := r.out.logLine(t, "metrics listening")["addr"].(string)
	// Wait for the first poll so the gauges exist.
	deadline := time.Now().Add(10 * time.Second)
	var body string
	for {
		var code int
		code, body = get(t, "http://"+maddr+"/metrics")
		if code != 200 {
			t.Fatalf("metrics: %d", code)
		}
		if strings.Contains(body, `hourglass_lab_pressure{kind="smoothed"}`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no pressure metric:\n%s", body)
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, name := range []string{"hourglass_poll_duration_seconds", "hourglass_poll_failures_total", "hourglass_lab_pressure", "hourglass_tokens_debited_total"} {
		if !strings.Contains(body, "# TYPE "+name+" ") {
			t.Errorf("metric %s missing", name)
		}
	}
	if code, _ := get(t, r.base+"/metrics"); code != 404 {
		t.Errorf("public /metrics = %d, want 404 with metrics enabled", code)
	}
	if code, _ := get(t, "http://"+maddr+"/healthz"); code != 404 {
		t.Errorf("metrics port serves other paths: %d", code)
	}
	_ = r.stop()
}

func TestConfigPathPrecedence(t *testing.T) {
	t.Setenv("HOURGLASS_CONFIG", "env.yaml")
	for _, c := range []struct {
		name string
		opts runhook.Options
		want string
	}{
		{"flag wins", runhook.Options{ConfigPath: "opt.yaml", Args: []string{"-config", "flag.yaml"}}, "flag.yaml"},
		{"option next", runhook.Options{ConfigPath: "opt.yaml"}, "opt.yaml"},
		{"env last", runhook.Options{}, "env.yaml"},
	} {
		got, err := configPath(c.opts, io.Discard)
		if err != nil || got != c.want {
			t.Errorf("%s: %q %v, want %q", c.name, got, err, c.want)
		}
	}
	t.Setenv("HOURGLASS_CONFIG", "")
	if got, _ := configPath(runhook.Options{}, io.Discard); got != "" {
		t.Errorf("no path: %q", got)
	}
	if _, err := configPath(runhook.Options{Args: []string{"-bogus"}}, io.Discard); err == nil {
		t.Error("unknown flag accepted")
	}
	if _, err := configPath(runhook.Options{Args: []string{"stray"}}, io.Discard); err == nil {
		t.Error("stray argument accepted")
	}
}

func TestPublicRunDelegatesThroughRunhook(t *testing.T) {
	// internal/app is linked into this test binary, so hourglass.Run reaches Run.
	cfg, _ := writeConfig(t, "pressure: {type: nope}\n")
	err := hourglass.Run(context.Background(), hourglass.Options{ConfigPath: cfg,
		DefaultPressureSource: testPressure, DefaultMeteringPolicy: testPolicy, Stdout: io.Discard, Stderr: io.Discard})
	if err == nil || !strings.Contains(err.Error(), `unknown pressure source "nope"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestRunhookWithoutImplFails(t *testing.T) {
	saved := runhook.Impl
	runhook.Impl = nil
	defer func() { runhook.Impl = saved }()
	if err := hourglass.Run(context.Background(), hourglass.Options{}); !errors.Is(err, runhook.ErrNoImpl) {
		t.Fatalf("err = %v", err)
	}
}

func TestUI3_UIFallbackModes(t *testing.T) {
	ui := fstest.MapFS{"index.html": {Data: []byte("<html>")}}
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	if uiFallback(ui, true, log) == nil {
		t.Error("bundled mode with a UI should serve it")
	}
	if uiFallback(ui, false, log) != nil {
		t.Error("ui.serve=false should serve nothing")
	}
	if uiFallback(nil, true, log) != nil {
		t.Error("no embedded UI should serve nothing")
	}
	if n := strings.Count(buf.String(), "not serving the UI"); n != 2 {
		t.Errorf("want one log line per no-UI case, got %d: %s", n, buf.String())
	}
}

// The settings page's runtime block gets the OIDC issuer (not a secret) from
// the config; empty means OIDC is not configured.
func TestADM2_RuntimeCarriesOIDCIssuerAndNames(t *testing.T) {
	x := extensions{pressure: hourglasstest.NewFakePressureSource(), policy: hourglasstest.NewFakePolicy(),
		hooks: []hourglass.Hook{hourglasstest.NewFakeHook()}}
	rt := x.runtime("https://issuer.example")
	if rt.OIDCIssuer != "https://issuer.example" || rt.PressureSource != x.pressure.Name() || rt.MeteringPolicy != x.policy.Name() ||
		len(rt.Hooks) != 1 || rt.Provider != "" {
		t.Errorf("runtime %+v", rt)
	}
	if rt := x.runtime(""); rt.OIDCIssuer != "" {
		t.Errorf("no OIDC: issuer %q", rt.OIDCIssuer)
	}
}

// A clock supplied in the options is the one the collector and the ledger
// service read: a poll run is stamped with it and the period rollover (which
// reads the ledger's clock) opens the period it names.
func TestOptionsClock_CollectorAndLedgerUseTheSuppliedClock(t *testing.T) {
	cfg, db := writeConfig(t, "")
	ctx := context.Background()
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	u, _, err := st.UpsertOIDCLogin(ctx, "sub-1", "a@example.test", "A", time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetUserRoles(ctx, u.ID, []string{store.RoleDeveloper}); err != nil {
		t.Fatal(err)
	}
	// An allowance in the previous period makes a rollover due at the fixed time.
	if _, err := st.AppendLedger(ctx, store.LedgerEntry{UserID: u.ID, Period: "2031-02", Type: "allowance", AmountMT: 1000},
		"2031-02", time.Date(2031, 2, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	fixed := time.Date(2031, 3, 5, 12, 0, 0, 0, time.UTC)
	r := startWithClock(t, cfg, db, hourglasstest.NewFakeClock(fixed))
	deadline := time.Now().Add(10 * time.Second)
	for {
		_, body := get(t, r.base+"/healthz")
		if !strings.Contains(body, `"last_poll_age_seconds":null`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no successful poll within 10s: %s", body)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := r.stop(); err != nil {
		t.Fatal(err)
	}

	st, err = store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	run, err := st.LatestRun(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !run.StartedAt.Equal(fixed) {
		t.Errorf("collector run started at %v, want the supplied clock %v", run.StartedAt, fixed)
	}
	entries, err := st.ListLedger(ctx, u.ID, "2031-03")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no ledger entry in the supplied clock's period 2031-03")
	}
	for _, e := range entries {
		if !e.CreatedAt.Equal(fixed) {
			t.Errorf("ledger entry %s at %v, want %v", e.Type, e.CreatedAt, fixed)
		}
	}
}
