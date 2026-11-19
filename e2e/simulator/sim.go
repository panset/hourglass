// SPDX-License-Identifier: MIT

// Command hourglass-vcsim is TEST TOOLING for the end-to-end stacks (D8): the
// govmomi simulator running in-process plus a small control HTTP endpoint that
// creates and destroys resource pools and sets the figures the vsphere
// pressure source reads. Nothing outside e2e/ may import it.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/url"
	"sort"
	"sync"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// Config sizes and addresses the simulated vCenter.
type Config struct {
	SOAPAddr string // host:port; port 0 picks a free one
	Username string
	Password string
	Hosts    int // hosts in the single cluster
	Log      *slog.Logger
}

// Sim is a running simulator plus the handles the control API needs.
type Sim struct {
	cfg   Config
	model *simulator.Model
	srv   *simulator.Server
	ctx   *simulator.Context
	log   *slog.Logger
	mu    sync.Mutex // serialises control mutations
}

// ErrNotFound reports a missing pool.
var ErrNotFound = errors.New("not found")

// NewSim builds the model (1 datacenter, 1 cluster, cfg.Hosts hosts) and starts
// the SOAP server with TLS and the simulator's self-signed certificate.
func NewSim(cfg Config) (*Sim, error) {
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	if cfg.Hosts < 1 {
		cfg.Hosts = 3
	}
	m := simulator.VPX()
	m.Datacenter = 1
	m.Cluster = 1
	m.ClusterHost = cfg.Hosts
	m.Host = 0
	if err := m.Create(); err != nil {
		return nil, fmt.Errorf("create model: %w", err)
	}
	m.Service.TLS = new(tls.Config)
	m.Service.Listen = &url.URL{Host: cfg.SOAPAddr, User: url.UserPassword(cfg.Username, cfg.Password)}
	srv := m.Service.NewServer()
	return &Sim{cfg: cfg, model: m, srv: srv, ctx: m.Service.Context, log: cfg.Log}, nil
}

// Close stops the SOAP server and removes the model's temporary files.
func (s *Sim) Close() {
	s.srv.Close()
	s.model.Remove()
}

// SOAPURL is the URL (with credentials) a client on this host can use.
func (s *Sim) SOAPURL() *url.URL {
	port := s.srv.Listener.Addr().(*net.TCPAddr).Port
	return &url.URL{Scheme: "https", Host: net.JoinHostPort("127.0.0.1", fmt.Sprint(port)),
		Path: "/sdk", User: url.UserPassword(s.cfg.Username, s.cfg.Password)}
}

// view is a logged-in client plus the cluster it works on.
type view struct {
	c       *govmomi.Client
	rootRef types.ManagedObjectReference
	dsRefs  []types.ManagedObjectReference
}

// connect logs in over the real SOAP endpoint, so every read and write the
// control API makes goes through the same API surface the adapter uses.
func (s *Sim) connect(ctx context.Context) (*view, func(), error) {
	c, err := govmomi.NewClient(ctx, s.SOAPURL(), true)
	if err != nil {
		return nil, nil, err
	}
	done := func() { _ = c.Logout(context.Background()) }
	f := find.NewFinder(c.Client, true)
	dc, err := f.DefaultDatacenter(ctx)
	if err != nil {
		done()
		return nil, nil, err
	}
	f.SetDatacenter(dc)
	cl, err := f.ClusterComputeResource(ctx, "*")
	if err != nil {
		done()
		return nil, nil, err
	}
	var ccr mo.ClusterComputeResource
	if err := cl.Properties(ctx, cl.Reference(), []string{"resourcePool", "datastore"}, &ccr); err != nil {
		done()
		return nil, nil, err
	}
	return &view{c: c, rootRef: *ccr.ResourcePool, dsRefs: ccr.Datastore}, done, nil
}

func (v *view) pools(ctx context.Context) (map[string]types.ManagedObjectReference, error) {
	var root mo.ResourcePool
	if err := property.DefaultCollector(v.c.Client).RetrieveOne(ctx, v.rootRef, []string{"resourcePool"}, &root); err != nil {
		return nil, err
	}
	out := map[string]types.ManagedObjectReference{}
	if len(root.ResourcePool) == 0 {
		return out, nil
	}
	var rps []mo.ResourcePool
	if err := property.DefaultCollector(v.c.Client).Retrieve(ctx, root.ResourcePool, []string{"name"}, &rps); err != nil {
		return nil, err
	}
	for _, rp := range rps {
		out[rp.Name] = rp.Reference()
	}
	return out, nil
}

// ListPools returns the names of the pools directly under the cluster root pool.
func (s *Sim) ListPools(ctx context.Context) ([]string, error) {
	v, done, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	m, err := v.pools(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

// CreatePool creates name under the root pool; created is false when it
// already existed.
func (s *Sim) CreatePool(ctx context.Context, name string) (created bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, done, err := s.connect(ctx)
	if err != nil {
		return false, err
	}
	defer done()
	m, err := v.pools(ctx)
	if err != nil {
		return false, err
	}
	if _, ok := m[name]; ok {
		return false, nil
	}
	zero, no, unlimited := int64(0), false, int64(-1)
	a := types.ResourceAllocationInfo{Reservation: &zero, ExpandableReservation: &no, Limit: &unlimited,
		Shares: &types.SharesInfo{Level: types.SharesLevelNormal}}
	_, err = object.NewResourcePool(v.c.Client, v.rootRef).Create(ctx, name,
		types.ResourceConfigSpec{CpuAllocation: a, MemoryAllocation: a})
	if err != nil {
		return false, err
	}
	s.log.Info("pool created", "pool", name)
	return true, nil
}

// DeletePool destroys the named pool; ErrNotFound when it does not exist.
func (s *Sim) DeletePool(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, done, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer done()
	m, err := v.pools(ctx)
	if err != nil {
		return err
	}
	ref, ok := m[name]
	if !ok {
		return ErrNotFound
	}
	task, err := object.NewResourcePool(v.c.Client, ref).Destroy(ctx)
	if err != nil {
		return err
	}
	if err := task.Wait(ctx); err != nil {
		return err
	}
	s.log.Info("pool deleted", "pool", name)
	return nil
}

// Pressure is the three fractions, each in [0,1]. Pointers mean "omitted".
type Pressure struct {
	CPU     *float64 `json:"cpu,omitempty"`
	Memory  *float64 `json:"memory,omitempty"`
	Storage *float64 `json:"storage,omitempty"`
}

// ReadPressure reports the fractions as the vsphere pressure source computes
// them, read through the property collector.
func (s *Sim) ReadPressure(ctx context.Context) (cpu, mem, storage float64, err error) {
	v, done, err := s.connect(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	defer done()
	return v.read(ctx)
}

func (v *view) read(ctx context.Context) (cpu, mem, storage float64, err error) {
	var root mo.ResourcePool
	if err = property.DefaultCollector(v.c.Client).RetrieveOne(ctx, v.rootRef, []string{"runtime"}, &root); err != nil {
		return
	}
	cpu = ratio(root.Runtime.Cpu.ReservationUsed, root.Runtime.Cpu.MaxUsage)
	mem = ratio(root.Runtime.Memory.ReservationUsed, root.Runtime.Memory.MaxUsage)
	var dss []mo.Datastore
	if len(v.dsRefs) > 0 {
		if err = property.DefaultCollector(v.c.Client).Retrieve(ctx, v.dsRefs, []string{"summary"}, &dss); err != nil {
			return
		}
	}
	var capSum, freeSum int64
	for _, d := range dss {
		capSum += d.Summary.Capacity
		freeSum += d.Summary.FreeSpace
	}
	if capSum > 0 {
		storage = 1 - float64(freeSum)/float64(capSum)
	}
	return
}

func ratio(used, max int64) float64 {
	if max <= 0 {
		return 0
	}
	return float64(used) / float64(max)
}

// SetPressure applies the given fractions (omitted ones unchanged): the root
// pool's CPU and memory reservationUsed become fraction*maxUsage, and every
// datastore's free space becomes capacity*(1-storage).
func (s *Sim) SetPressure(ctx context.Context, p Pressure) error {
	for _, f := range []*float64{p.CPU, p.Memory, p.Storage} {
		if f != nil && (math.IsNaN(*f) || *f < 0 || *f > 1) {
			return fmt.Errorf("fractions must be between 0 and 1")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, done, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer done()

	var root mo.ResourcePool
	if err := property.DefaultCollector(v.c.Client).RetrieveOne(ctx, v.rootRef, []string{"runtime"}, &root); err != nil {
		return err
	}
	var changes []types.PropertyChange
	if p.CPU != nil {
		changes = append(changes, types.PropertyChange{Name: "runtime.cpu.reservationUsed",
			Val: int64(math.Round(*p.CPU * float64(root.Runtime.Cpu.MaxUsage)))})
	}
	if p.Memory != nil {
		changes = append(changes, types.PropertyChange{Name: "runtime.memory.reservationUsed",
			Val: int64(math.Round(*p.Memory * float64(root.Runtime.Memory.MaxUsage)))})
	}
	if len(changes) > 0 {
		obj := s.ctx.Map.Get(v.rootRef)
		if obj == nil {
			return fmt.Errorf("root pool %s not in simulator", v.rootRef)
		}
		s.ctx.Map.AtomicUpdate(s.ctx, obj, changes)
	}
	if p.Storage != nil && len(v.dsRefs) > 0 {
		var dss []mo.Datastore
		if err := property.DefaultCollector(v.c.Client).Retrieve(ctx, v.dsRefs, []string{"summary"}, &dss); err != nil {
			return err
		}
		for _, d := range dss {
			obj := s.ctx.Map.Get(d.Reference())
			if obj == nil {
				continue
			}
			free := int64(math.Round(float64(d.Summary.Capacity) * (1 - *p.Storage)))
			s.ctx.WithLock(obj, func() {
				s.ctx.Update(obj, []types.PropertyChange{{Name: "summary.freeSpace", Val: free}})
				if ds, ok := obj.(*simulator.Datastore); ok && ds.Info != nil {
					ds.Info.GetDatastoreInfo().FreeSpace = free
				}
			})
		}
	}
	s.log.Info("pressure set", "cpu", p.CPU, "memory", p.Memory, "storage", p.Storage)
	return nil
}
