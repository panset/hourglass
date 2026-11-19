// SPDX-License-Identifier: MIT

package vsphere

// Layer 3 (SPEC "Testing without a vCenter", item 3; D7): the real adapter
// against the govmomi simulator, in-process, on localhost only. The simulator
// does not move allocation figures (D5), so these tests assert wiring and
// range; layer 2 carries the formula.

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/panset/hourglass/pkg/hourglass"
)

type sim struct {
	url   string // no credentials
	admin *govmomi.Client
	root  *object.ResourcePool // DC0_C0 root pool
}

func startSim(t *testing.T, withTLS bool) *sim {
	t.Helper()
	model := simulator.VPX()
	model.Cluster = 1
	if err := model.Create(); err != nil {
		t.Fatal(err)
	}
	if withTLS {
		model.Service.TLS = new(tls.Config)
	}
	s := model.Service.NewServer()
	t.Cleanup(func() { s.Close(); model.Remove() })

	u, _ := url.Parse(s.URL.String())
	u.User = nil
	ctx := context.Background()
	admin, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Logout(ctx) })
	f := find.NewFinder(admin.Client, true)
	dc, err := f.DefaultDatacenter(ctx)
	if err != nil {
		t.Fatal(err)
	}
	f.SetDatacenter(dc)
	cl, err := f.ClusterComputeResource(ctx, "*")
	if err != nil {
		t.Fatal(err)
	}
	var ccr mo.ClusterComputeResource
	if err := cl.Properties(ctx, cl.Reference(), []string{"resourcePool"}, &ccr); err != nil {
		t.Fatal(err)
	}
	return &sim{url: u.String(), admin: admin, root: object.NewResourcePool(admin.Client, *ccr.ResourcePool)}
}

// createPool is test-only: it changes the simulator's inventory.
func (s *sim) createPool(t *testing.T, parent *object.ResourcePool, name string) *object.ResourcePool {
	t.Helper()
	n := int64(0)
	yes := false
	a := types.ResourceAllocationInfo{Reservation: &n, ExpandableReservation: &yes, Limit: func() *int64 { v := int64(-1); return &v }(),
		Shares: &types.SharesInfo{Level: types.SharesLevelNormal}}
	rp, err := parent.Create(context.Background(), name, types.ResourceConfigSpec{CpuAllocation: a, MemoryAllocation: a})
	if err != nil {
		t.Fatal(err)
	}
	return rp
}

func (s *sim) cfgJSON(extra map[string]any) json.RawMessage {
	m := map[string]any{"url": s.url, "username": "user", "password": "pass", "insecure_skip_verify": true, "request_timeout": "10s"}
	for k, v := range extra {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return b
}

func listNames(t *testing.T, p hourglass.ResourceProvider) map[string]hourglass.Allocation {
	t.Helper()
	got, err := p.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]hourglass.Allocation{}
	for _, a := range got {
		m[a.ResourceName] = a
	}
	return m
}

func TestEXT3_SimulatorProviderListsPools(t *testing.T) {
	s := startSim(t, false)
	p, err := NewProvider(s.cfgJSON(nil))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.(*provider).cl.Close(context.Background()) })

	if got := listNames(t, p); len(got) != 0 {
		t.Fatalf("a fresh simulator has no pools below the root, got %+v", got)
	}
	lab := s.createPool(t, s.root, "lab-01_RP")
	child := s.createPool(t, lab, "child_RP")

	got := listNames(t, p)
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	wantLab := map[string]string{"cluster": "DC0_C0", "datacenter": "DC0", "parent": "Resources", "path": "/DC0/host/DC0_C0/Resources/lab-01_RP"}
	wantChild := map[string]string{"cluster": "DC0_C0", "datacenter": "DC0", "parent": "lab-01_RP", "path": "/DC0/host/DC0_C0/Resources/lab-01_RP/child_RP"}
	if !reflect.DeepEqual(got["lab-01_RP"].Labels, wantLab) {
		t.Errorf("lab labels %v", got["lab-01_RP"].Labels)
	}
	if !reflect.DeepEqual(got["child_RP"].Labels, wantChild) {
		t.Errorf("child labels %v", got["child_RP"].Labels)
	}
	if _, ok := got["Resources"]; ok {
		t.Error("root pool must be excluded")
	}

	// Destroying a pool removes it on the next call.
	task, err := child.Destroy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := task.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	got = listNames(t, p)
	if _, ok := got["child_RP"]; ok || len(got) != 1 {
		t.Fatalf("after destroy: %+v", got)
	}
}

func TestEXT3_SimulatorScope(t *testing.T) {
	s := startSim(t, false)
	s.createPool(t, s.root, "lab-01_RP")
	for _, tc := range []struct {
		extra map[string]any
		want  int
	}{
		{map[string]any{"datacenter": "DC0"}, 1},
		{map[string]any{"datacenter": "DC0", "clusters": []string{"DC0_C0"}}, 1},
	} {
		raw := s.cfgJSON(tc.extra)
		p, err := NewProvider(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := listNames(t, p); len(got) != tc.want {
			t.Errorf("%v: got %+v want %d pools", tc.extra, got, tc.want)
		}
	}
}

// A configured datacenter or cluster that does not exist fails the poll for
// both extensions (the provider no longer returns a silent empty list).
func TestEXT3_SimulatorUnknownScopeIsError(t *testing.T) {
	s := startSim(t, false)
	for _, extra := range []map[string]any{
		{"datacenter": "nope"},
		{"clusters": []string{"other"}},
		{"clusters": []string{"DC0_C0", "other"}},
	} {
		p, err := NewProvider(s.cfgJSON(extra))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.List(context.Background()); err == nil {
			t.Errorf("provider %v: want an error", extra)
		}
		src, err := NewPressureSource(s.cfgJSON(extra))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := src.Pressure(context.Background(), hourglass.Snapshot{}); err == nil {
			t.Errorf("pressure source %v: want an error", extra)
		}
	}
	// An existing cluster with no pools besides its root is a valid empty result.
	p, err := NewProvider(s.cfgJSON(map[string]any{"datacenter": "DC0", "clusters": []string{"DC0_C0"}}))
	if err != nil {
		t.Fatal(err)
	}
	if got := listNames(t, p); len(got) != 0 {
		t.Errorf("empty cluster: %+v", got)
	}
}

func TestMET1_SimulatorPressureEqualsStorageRatio(t *testing.T) {
	s := startSim(t, false)
	src, err := NewPressureSource(s.cfgJSON(nil))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.(*pressureSource).cl.Close(context.Background()) })
	got, err := src.Pressure(context.Background(), hourglass.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}

	// Expected value from the simulator's own datastore figures (D6 storage ratio).
	ctx := context.Background()
	var dss []mo.Datastore
	f := find.NewFinder(s.admin.Client, true)
	dc, err := f.DefaultDatacenter(ctx)
	if err != nil {
		t.Fatal(err)
	}
	f.SetDatacenter(dc)
	list, err := f.DatastoreList(ctx, "*")
	if err != nil {
		t.Fatal(err)
	}
	var refs []types.ManagedObjectReference
	for _, d := range list {
		refs = append(refs, d.Reference())
	}
	if err := s.admin.Retrieve(ctx, refs, []string{"summary"}, &dss); err != nil {
		t.Fatal(err)
	}
	var capacity, free int64
	for _, d := range dss {
		capacity += d.Summary.Capacity
		free += d.Summary.FreeSpace
	}
	if capacity == 0 {
		t.Fatal("simulator datastore has no capacity")
	}
	want := 1 - float64(free)/float64(capacity)
	if want <= 0 || got < 0 || got > 1 || !near(got, want) {
		t.Fatalf("pressure %v, storage ratio %v", got, want)
	}

	// CPU and memory ratios are 0 in the simulator (D5): reading them directly.
	cl, err := s.adapterFor(t).Clusters(ctx)
	if err != nil || len(cl) != 1 {
		t.Fatalf("%v %v", cl, err)
	}
	if cl[0].CPU.ReservationUsed != 0 || cl[0].Memory.ReservationUsed != 0 || cl[0].CPU.MaxUsage <= 0 || cl[0].Memory.MaxUsage <= 0 {
		t.Fatalf("figures: %+v", cl[0])
	}
}

func (s *sim) adapterFor(t *testing.T) *govmomiClient {
	t.Helper()
	c, err := parseConfig("t", s.cfgJSON(nil))
	if err != nil {
		t.Fatal(err)
	}
	g := newGovmomiClient(c)
	t.Cleanup(func() { _ = g.Close(context.Background()) })
	return g
}

func TestEXT3_SimulatorTLS(t *testing.T) {
	s := startSim(t, true)
	if !strings.HasPrefix(s.url, "https://") {
		t.Fatalf("simulator must serve TLS: %s", s.url)
	}
	strict, err := NewProvider(s.cfgJSON(map[string]any{"insecure_skip_verify": false}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = strict.List(context.Background())
	if err == nil {
		t.Fatal("self-signed endpoint must be rejected without insecure_skip_verify")
	}
	if !strings.Contains(err.Error(), "certificate") && !strings.Contains(err.Error(), "x509") {
		t.Fatalf("want a certificate error, got %v", err)
	}
	// startSim's config uses username "user" and password "pass".
	if strings.Contains(err.Error(), "pass") || strings.Contains(err.Error(), "user:") {
		t.Fatalf("credentials in error: %v", err)
	}
	pressure, err := NewPressureSource(s.cfgJSON(map[string]any{"insecure_skip_verify": false}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pressure.Pressure(context.Background(), hourglass.Snapshot{}); err == nil {
		t.Fatal("pressure source must also reject the certificate")
	}

	lax, err := NewProvider(s.cfgJSON(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lax.List(context.Background()); err != nil {
		t.Fatalf("insecure_skip_verify must connect: %v", err)
	}
}

func TestEXT3_SimulatorBadCredentialsDoNotLeakPassword(t *testing.T) {
	model := simulator.VPX()
	if err := model.Create(); err != nil {
		t.Fatal(err)
	}
	model.Service.Listen = &url.URL{User: url.UserPassword("admin", "right")}
	s := model.Service.NewServer()
	t.Cleanup(func() { s.Close(); model.Remove() })
	u := *s.URL
	u.User = nil
	raw, _ := json.Marshal(map[string]any{"url": u.String(), "username": "admin", "password": "wrong-pw-value", "insecure_skip_verify": true})
	p, err := NewProvider(raw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.List(context.Background())
	if err == nil {
		t.Fatal("login with a wrong password must fail")
	}
	if strings.Contains(err.Error(), "wrong-pw-value") {
		t.Fatalf("password in error: %v", err)
	}
}

func TestEXT3_SimulatorReloginAfterSessionTerminated(t *testing.T) {
	s := startSim(t, false)
	s.createPool(t, s.root, "lab-01_RP")
	g := s.adapterFor(t)
	ctx := context.Background()
	if pools, err := g.ResourcePools(ctx); err != nil || len(pools) != 2 { // root + lab-01_RP
		t.Fatalf("first call: %v %v", pools, err)
	}
	// End the session behind the adapter's back.
	g.mu.Lock()
	old := g.sm
	g.mu.Unlock()
	if err := old.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	pools, err := g.ResourcePools(ctx)
	if err != nil || len(pools) != 2 {
		t.Fatalf("call after the session ended must log in again once: %v %v", pools, err)
	}
	if g.sm == old {
		t.Fatal("expected a new session")
	}
	// Same for the figures path.
	_ = g.sm.Logout(ctx)
	if cl, err := g.Clusters(ctx); err != nil || len(cl) != 1 {
		t.Fatalf("clusters after logout: %v %v", cl, err)
	}
	// Close is idempotent and ends the session.
	if err := g.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := g.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
