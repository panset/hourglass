// SPDX-License-Identifier: MIT

package vsphere

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/hourglasstest"
)

const (
	gib = int64(1) << 30
	pw  = "s3cret-pw-do-not-print"
)

func testCfg(clusters ...string) config {
	return config{URL: "https://vc.test/sdk", Username: "u", Password: secret(pw), Clusters: clusters, RequestTimeout: time.Second}
}

func newTestProvider(f *fakeClient, clusters ...string) hourglass.ResourceProvider {
	return &provider{cfg: testCfg(clusters...), cl: f}
}

func newTestPressure(f *fakeClient, clusters ...string) hourglass.PressureSource {
	return &pressureSource{cfg: testCfg(clusters...), cl: f}
}

// cluster builds figures with CPU/memory max 1000 and one 100 GiB datastore.
func cluster(name string, cpuUsed, memUsed int64, freeFrac float64) clusterFigures {
	return clusterFigures{
		Name: name, Datacenter: "DC0",
		CPU:        resource{ReservationUsed: cpuUsed, MaxUsage: 1000},
		Memory:     resource{ReservationUsed: memUsed, MaxUsage: 1000},
		Datastores: []datastore{{Name: "ds", Capacity: 100 * gib, FreeSpace: int64(freeFrac * float64(100*gib)), Accessible: true}},
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// ---- provider -------------------------------------------------------------

func seededPools() []pool {
	return []pool{
		{Name: "Resources", Path: "/DC0/host/C1/Resources", Cluster: "C1", Datacenter: "DC0", Root: true},
		{Name: "lab-01_RP", Path: "/DC0/host/C1/Resources/lab-01_RP", Parent: "Resources", Cluster: "C1", Datacenter: "DC0"},
		{Name: "child_RP", Path: "/DC0/host/C1/Resources/lab-01_RP/child_RP", Parent: "lab-01_RP", Cluster: "C1", Datacenter: "DC0"},
		{Name: "Resources", Path: "/DC0/host/C2/Resources", Cluster: "C2", Datacenter: "DC0", Root: true},
		{Name: "lab-01_RP", Path: "/DC0/host/C2/Resources/lab-01_RP", Parent: "Resources", Cluster: "C2", Datacenter: "DC0"},
	}
}

func TestEXT3_ProviderListsPoolsExcludingRoot(t *testing.T) {
	f := &fakeClient{pools: seededPools()}
	got, err := newTestProvider(f).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range got {
		if a.Labels["parent"] == "" {
			t.Errorf("root pool leaked: %+v", a)
		}
	}
	if len(got) != 3 {
		t.Fatalf("want 3 allocations (5 pools minus 2 roots), got %d: %+v", len(got), got)
	}
}

func TestEXT3_ProviderNestedPoolsListedByOwnName(t *testing.T) {
	f := &fakeClient{pools: seededPools()}
	got, _ := newTestProvider(f, "C1").List(context.Background())
	want := []hourglass.Allocation{
		{ResourceName: "lab-01_RP", Labels: map[string]string{"cluster": "C1", "datacenter": "DC0", "parent": "Resources", "path": "/DC0/host/C1/Resources/lab-01_RP"}},
		{ResourceName: "child_RP", Labels: map[string]string{"cluster": "C1", "datacenter": "DC0", "parent": "lab-01_RP", "path": "/DC0/host/C1/Resources/lab-01_RP/child_RP"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestEXT3_ProviderDuplicateNamesInDifferentClusters(t *testing.T) {
	f := &fakeClient{pools: seededPools()}
	got, _ := newTestProvider(f).List(context.Background())
	var clusters []string
	for _, a := range got {
		if a.ResourceName == "lab-01_RP" {
			clusters = append(clusters, a.Labels["cluster"])
		}
	}
	if !reflect.DeepEqual(clusters, []string{"C1", "C2"}) {
		t.Fatalf("both same-named pools must be listed, with their cluster: %v", clusters)
	}
}

func TestEXT3_ProviderScopeByCluster(t *testing.T) {
	f := &fakeClient{pools: seededPools()}
	got, _ := newTestProvider(f, "C2").List(context.Background())
	if len(got) != 1 || got[0].Labels["cluster"] != "C2" {
		t.Fatalf("got %+v", got)
	}
	got, _ = newTestProvider(f, "nope").List(context.Background())
	if len(got) != 0 {
		t.Fatalf("unknown cluster must list nothing, got %+v", got)
	}
}

func TestEXT3_ProviderPoolDisappears(t *testing.T) {
	f := &fakeClient{pools: seededPools()}
	p := newTestProvider(f)
	f.mu.Lock()
	f.pools = f.pools[:1]
	f.mu.Unlock()
	got, _ := p.List(context.Background())
	if len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestEXT3_ProviderErrorsPropagate(t *testing.T) {
	boom := errors.New("boom")
	f := &fakeClient{err: boom}
	if _, err := newTestProvider(f).List(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
}

func TestEXT3_ProviderContextCancellation(t *testing.T) {
	f := &fakeClient{pools: seededPools(), block: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newTestProvider(f).List(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel2()
	if _, err := newTestProvider(f).List(ctx2); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked call must end with the context: %v", err)
	}
}

func TestEXT5_ProviderConformance(t *testing.T) {
	f := &fakeClient{pools: seededPools()}
	hourglasstest.TestProvider(t, func() hourglass.ResourceProvider { return newTestProvider(f) })
}

func TestEXT5_PressureConformance(t *testing.T) {
	f := &fakeClient{clusters: []clusterFigures{cluster("C1", 500, 100, 0.5)}}
	hourglasstest.TestPressureSource(t, func() hourglass.PressureSource { return newTestPressure(f) })
}

// ---- pressure (D6, D7) ----------------------------------------------------

func pressureOf(t *testing.T, f *fakeClient, clusters ...string) (float64, error) {
	t.Helper()
	return newTestPressure(f, clusters...).Pressure(context.Background(), hourglass.Snapshot{})
}

func TestMET1_PressureNonZeroCPUAndMemory(t *testing.T) {
	// D7: non-zero CPU and memory ratios are asserted here, not in the simulator.
	f := &fakeClient{clusters: []clusterFigures{{
		Name: "C1", CPU: resource{250, 1000}, Memory: resource{300, 1000},
	}}}
	v, err := pressureOf(t, f)
	if err != nil || !near(v, 0.3) {
		t.Fatalf("got %v, %v", v, err)
	}
	f.clusters[0].Memory = resource{100, 1000}
	if v, _ := pressureOf(t, f); !near(v, 0.25) {
		t.Fatalf("cpu wins: got %v", v)
	}
}

func TestMET1_PressureMaxOfThreeEachWins(t *testing.T) {
	cases := []struct {
		name string
		c    clusterFigures
		want float64
	}{
		{"cpu wins", cluster("C1", 800, 100, 0.9), 0.8},
		{"memory wins", cluster("C1", 100, 700, 0.9), 0.7},
		{"storage wins", cluster("C1", 100, 200, 0.4), 0.6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := pressureOf(t, &fakeClient{clusters: []clusterFigures{tc.c}})
			if err != nil || !near(v, tc.want) {
				t.Fatalf("got %v, %v want %v", v, err, tc.want)
			}
		})
	}
}

func TestMET1_PressureMultiClusterMax(t *testing.T) {
	f := &fakeClient{clusters: []clusterFigures{cluster("C1", 100, 100, 0.9), cluster("C2", 900, 100, 0.9), cluster("C3", 400, 100, 0.9)}}
	if v, _ := pressureOf(t, f); !near(v, 0.9) {
		t.Fatalf("all clusters: %v", v)
	}
	if v, _ := pressureOf(t, f, "C1", "C3"); !near(v, 0.4) {
		t.Fatalf("scoped to C1,C3: %v", v)
	}
}

func TestMET1_PressureCrossesBands(t *testing.T) {
	// Values on both sides of the default band edges 0.5 / 0.75 / 0.9.
	for _, tc := range []struct {
		used int64
		want float64
	}{{499, 0.499}, {500, 0.5}, {501, 0.501}, {749, 0.749}, {751, 0.751}, {899, 0.899}, {901, 0.901}} {
		f := &fakeClient{clusters: []clusterFigures{cluster("C1", tc.used, 0, 1)}}
		v, err := pressureOf(t, f)
		if err != nil || !near(v, tc.want) {
			t.Fatalf("used %d: got %v, %v", tc.used, v, err)
		}
	}
}

func TestMET1_PressureZeroCapacityIsError(t *testing.T) {
	for name, mut := range map[string]func(*clusterFigures){
		"cpu":    func(c *clusterFigures) { c.CPU.MaxUsage = 0 },
		"memory": func(c *clusterFigures) { c.Memory.MaxUsage = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			c := cluster("C1", 10, 10, 0.5)
			mut(&c)
			_, err := pressureOf(t, &fakeClient{clusters: []clusterFigures{c}})
			if err == nil || !strings.Contains(err.Error(), "zero "+name+" capacity") {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestMET1_PressureClamp(t *testing.T) {
	over := cluster("C1", 5000, 0, 1) // 500%
	if v, err := pressureOf(t, &fakeClient{clusters: []clusterFigures{over}}); err != nil || v != 1 {
		t.Fatalf("got %v, %v", v, err)
	}
	neg := cluster("C1", -5, -5, 1.5) // negative reservation, free > capacity
	if v, err := pressureOf(t, &fakeClient{clusters: []clusterFigures{neg}}); err != nil || v != 0 {
		t.Fatalf("got %v, %v", v, err)
	}
	full := cluster("C1", 0, 0, -0.5) // free < 0
	if v, _ := pressureOf(t, &fakeClient{clusters: []clusterFigures{full}}); v != 1 {
		t.Fatalf("got %v", v)
	}
}

func TestMET1_PressureInaccessibleDatastores(t *testing.T) {
	c := cluster("C1", 100, 100, 0)
	c.Datastores = []datastore{
		{Name: "full-but-offline", Capacity: 100 * gib, FreeSpace: 0, Accessible: false},
		{Name: "ok", Capacity: 100 * gib, FreeSpace: 75 * gib, Accessible: true},
		{Name: "zero-cap", Capacity: 0, FreeSpace: 0, Accessible: true},
	}
	if v, err := pressureOf(t, &fakeClient{clusters: []clusterFigures{c}}); err != nil || !near(v, 0.25) {
		t.Fatalf("offline datastore must not count: %v, %v", v, err)
	}
	// Summed over accessible ones.
	c.Datastores = []datastore{
		{Capacity: 100 * gib, FreeSpace: 50 * gib, Accessible: true},
		{Capacity: 300 * gib, FreeSpace: 250 * gib, Accessible: true},
	}
	if v, _ := pressureOf(t, &fakeClient{clusters: []clusterFigures{c}}); !near(v, 0.25) {
		t.Fatalf("sum: %v", v)
	}
	// No accessible datastore: no storage ratio, CPU/memory still count.
	c.Datastores = []datastore{{Capacity: 100 * gib, FreeSpace: 0, Accessible: false}}
	c.CPU = resource{300, 1000}
	if v, err := pressureOf(t, &fakeClient{clusters: []clusterFigures{c}}); err != nil || !near(v, 0.3) {
		t.Fatalf("got %v, %v", v, err)
	}
	c.Datastores = nil
	if v, err := pressureOf(t, &fakeClient{clusters: []clusterFigures{c}}); err != nil || !near(v, 0.3) {
		t.Fatalf("got %v, %v", v, err)
	}
}

func TestMET1_PressureNoClustersIsError(t *testing.T) {
	if _, err := pressureOf(t, &fakeClient{}); err == nil {
		t.Fatal("want an error when no clusters are found")
	}
	f := &fakeClient{clusters: []clusterFigures{cluster("C1", 1, 1, 1)}}
	if _, err := pressureOf(t, f, "C1", "missing"); err == nil || !strings.Contains(err.Error(), `"missing"`) {
		t.Fatalf("a named cluster that does not exist is an error: %v", err)
	}
}

func TestMET1_PressureIgnoresSnapshot(t *testing.T) {
	f := &fakeClient{clusters: []clusterFigures{cluster("C1", 600, 0, 1)}}
	snap := hourglasstest.NewSnapshot().Setup("a").Build()
	a, _ := newTestPressure(f).Pressure(context.Background(), snap)
	b, _ := newTestPressure(f).Pressure(context.Background(), hourglass.Snapshot{})
	if a != b {
		t.Fatalf("%v != %v", a, b)
	}
}

func TestMET1_PressureErrorsPropagateAndCancel(t *testing.T) {
	boom := errors.New("boom")
	if _, err := pressureOf(t, &fakeClient{err: boom}); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	f := &fakeClient{clusters: []clusterFigures{cluster("C1", 1, 1, 1)}, block: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := newTestPressure(f).Pressure(ctx, hourglass.Snapshot{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if _, err := newTestPressure(f).Pressure(ctx2, hourglass.Snapshot{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

// ---- config ---------------------------------------------------------------

func rawCfg(m map[string]any) json.RawMessage {
	b, _ := json.Marshal(m)
	return b
}

func TestEXT3_ConfigParsing(t *testing.T) {
	t.Setenv(PasswordEnv, "")
	good := map[string]any{"url": "https://vc.test/sdk", "username": "u", "password": "p"}
	c, err := parseConfig("x", rawCfg(good))
	if err != nil {
		t.Fatal(err)
	}
	if c.RequestTimeout != DefaultRequestTimeout || c.InsecureSkipVerify || c.Datacenter != "" || len(c.Clusters) != 0 {
		t.Fatalf("defaults: %v", c)
	}
	with := func(k string, v any) map[string]any {
		m := map[string]any{}
		for kk, vv := range good {
			m[kk] = vv
		}
		m[k] = v
		return m
	}
	full, err := parseConfig("x", rawCfg(map[string]any{"url": "https://vc.test", "username": "u", "password": "p",
		"insecure_skip_verify": true, "datacenter": "DC0", "clusters": []string{"a", "b", "a"}, "request_timeout": "5s"}))
	if err != nil || !full.InsecureSkipVerify || full.Datacenter != "DC0" || !reflect.DeepEqual(full.Clusters, []string{"a", "b"}) || full.RequestTimeout != 5*time.Second {
		t.Fatalf("full: %v, %v", full, err)
	}
	bad := map[string]json.RawMessage{
		"empty":          nil,
		"null":           json.RawMessage("null"),
		"unknown field":  rawCfg(with("extra", 1)),
		"no url":         rawCfg(with("url", "")),
		"ftp url":        rawCfg(with("url", "ftp://x")),
		"url creds":      rawCfg(with("url", "https://a:b@x/sdk")),
		"no user":        rawCfg(with("username", "")),
		"no password":    rawCfg(with("password", "")),
		"bad timeout":    rawCfg(with("request_timeout", "soon")),
		"zero timeout":   rawCfg(with("request_timeout", "0s")),
		"neg timeout":    rawCfg(with("request_timeout", "-1s")),
		"empty cluster":  rawCfg(with("clusters", []string{""})),
		"trailing data":  json.RawMessage(string(rawCfg(good)) + `{}`),
		"not an object":  json.RawMessage(`[1]`),
		"timeout number": rawCfg(with("request_timeout", 5)),
	}
	for name, raw := range bad {
		if _, err := parseConfig("x", raw); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestEXT3_PasswordEnvFallback(t *testing.T) {
	t.Setenv(PasswordEnv, "from-env")
	c, err := parseConfig("x", rawCfg(map[string]any{"url": "https://vc.test", "username": "u"}))
	if err != nil || c.Password.reveal() != "from-env" {
		t.Fatalf("got %v, %v", c, err)
	}
	c, _ = parseConfig("x", rawCfg(map[string]any{"url": "https://vc.test", "username": "u", "password": "explicit"}))
	if c.Password.reveal() != "explicit" {
		t.Fatalf("config value must win over env")
	}
	t.Setenv(PasswordEnv, "")
	_, err = parseConfig("x", rawCfg(map[string]any{"url": "https://vc.test", "username": "u"}))
	if err == nil || !strings.Contains(err.Error(), PasswordEnv) {
		t.Fatalf("got %v", err)
	}
}

func TestEXT3_PasswordNeverPrinted(t *testing.T) {
	c, err := parseConfig("x", rawCfg(map[string]any{"url": "https://vc.test", "username": "u", "password": pw}))
	if err != nil {
		t.Fatal(err)
	}
	j, _ := json.Marshal(c)
	outs := []string{
		c.String(), fmt.Sprint(c), fmt.Sprintf("%v %+v %#v %s", c, c, c, c), fmt.Sprintf("%v %+v %#v", c.Password, &c, c.key()),
		fmt.Sprint(c.key()), string(j), fmt.Sprintf("%v", &provider{cfg: c}), fmt.Sprintf("%+v", &pressureSource{cfg: c}),
	}
	for _, o := range outs {
		if strings.Contains(o, pw) {
			t.Errorf("password leaked: %q", o)
		}
	}
	// Errors from the extensions.
	f := &fakeClient{err: errors.New("down")}
	_, e1 := (&provider{cfg: c, cl: f}).List(context.Background())
	_, e2 := (&pressureSource{cfg: c, cl: f}).Pressure(context.Background(), hourglass.Snapshot{})
	for _, e := range []error{e1, e2} {
		if e == nil || strings.Contains(e.Error(), pw) {
			t.Errorf("error %v", e)
		}
	}
}

func TestEXT3_FactoryErrorNeverContainsPassword(t *testing.T) {
	_, err := NewProvider(rawCfg(map[string]any{"url": "https://u:" + pw + "@vc.test", "username": "u", "password": pw}))
	if err == nil || strings.Contains(err.Error(), pw) {
		t.Fatalf("got %v", err)
	}
	_, err = NewPressureSource(rawCfg(map[string]any{"url": "https://vc.test", "username": "u", "password": pw, "bogus": pw}))
	if err == nil || strings.Contains(err.Error(), pw) {
		t.Fatalf("got %v", err)
	}
}

func TestEXT3_RegisteredAndShareOneClient(t *testing.T) {
	if !contains(hourglass.Providers(), Name) || !contains(hourglass.PressureSources(), Name) {
		t.Fatal("vsphere not registered as provider and pressure source")
	}
	saved := newClient
	defer func() { newClient = saved }()
	made := 0
	newClient = func(config) client { made++; return &fakeClient{} }
	sharedMu.Lock()
	shared = map[connKey]client{}
	sharedMu.Unlock()

	raw := rawCfg(map[string]any{"url": "https://share.test", "username": "u", "password": "p"})
	p, err := hourglass.NewProvider(Name, raw)
	if err != nil {
		t.Fatal(err)
	}
	s, err := hourglass.NewPressureSource(Name, raw)
	if err != nil {
		t.Fatal(err)
	}
	if made != 1 || p.(*provider).cl != s.(*pressureSource).cl {
		t.Fatalf("equal settings must share one client, made %d", made)
	}
	// Scope does not change the connection; credentials do.
	scoped := rawCfg(map[string]any{"url": "https://share.test", "username": "u", "password": "p", "clusters": []string{"a"}})
	if _, err := hourglass.NewProvider(Name, scoped); err != nil || made != 1 {
		t.Fatalf("scope must not open a second session: %v made=%d", err, made)
	}
	other := rawCfg(map[string]any{"url": "https://share.test", "username": "u2", "password": "p"})
	if _, err := hourglass.NewProvider(Name, other); err != nil || made != 2 {
		t.Fatalf("different user, new client: %v made=%d", err, made)
	}
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// The pressure loop starts from 0, so the lower clamp is only observable on
// the per-cluster helpers; test them directly.
func TestMET1_LowerClampOnRatios(t *testing.T) {
	if v, err := reservationRatio("C1", "cpu", resource{ReservationUsed: -5, MaxUsage: 1000}); err != nil || v != 0 {
		t.Fatalf("negative reservation: got %v, %v", v, err)
	}
	if v, ok := storageRatio([]datastore{{Capacity: 100, FreeSpace: 150, Accessible: true}}); !ok || v != 0 {
		t.Fatalf("free above capacity: got %v, %v", v, ok)
	}
	if v := clamp(math.NaN()); v != 0 {
		t.Fatalf("NaN: got %v", v)
	}
	if v, err := clusterPressure(cluster("C1", -5, -5, 1.5)); err != nil || v != 0 {
		t.Fatalf("cluster: got %v, %v", v, err)
	}
}

func TestEXT3_ProviderUnknownClusterIsError(t *testing.T) {
	f := &fakeClient{pools: seededPools()}
	if _, err := newTestProvider(f, "C1", "ghost").List(context.Background()); err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("want a cluster-not-found error, got %v", err)
	}
	if _, err := newTestProvider(f, "ghost").List(context.Background()); err == nil {
		t.Fatal("only unknown cluster must error")
	}
	// An existing cluster with no pools besides its root is a valid empty result.
	only := &fakeClient{pools: []pool{{Name: "Resources", Path: "/DC0/host/E/Resources", Cluster: "E", Root: true}}}
	got, err := newTestProvider(only, "E").List(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("empty cluster: %v, %v", got, err)
	}
}

func TestEXT3_PasswordNeverPrintedByValueOrLog(t *testing.T) {
	c := testCfg("C1")
	p := provider{cfg: c, cl: &fakeClient{}}
	s := pressureSource{cfg: c, cl: &fakeClient{}}
	var buf bytes.Buffer
	lg := slog.New(slog.NewTextHandler(&buf, nil))
	lg.Info("x", "cfg", c, "p", p, "s", s, "pp", &p, "ps", &s)
	jb := bytes.Buffer{}
	slog.New(slog.NewJSONHandler(&jb, nil)).Info("x", "cfg", c, "p", p, "s", s)
	outs := []string{buf.String(), jb.String()}
	for _, v := range []any{c, p, s, &p, &s} {
		outs = append(outs, fmt.Sprintf("%v|%+v|%#v|%s", v, v, v, v))
	}
	for _, o := range outs {
		if strings.Contains(o, pw) {
			t.Errorf("password leaked: %q", o)
		}
	}
}
