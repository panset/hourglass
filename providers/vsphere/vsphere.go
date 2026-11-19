// SPDX-License-Identifier: MIT

// Package vsphere is the bundled vCenter extension (EXT-3): a ResourceProvider
// that lists resource pools and a PressureSource that reports how full the
// configured clusters are. Both are read-only. Register them by importing the
// package:
//
//	import _ "github.com/panset/hourglass/providers/vsphere"
//
// Both extensions take the same config block so a deployment can give both
// the same settings:
//
//	{
//	  "url": "https://vcenter.example/sdk",
//	  "username": "svc-hourglass@vsphere.local",
//	  "password": "",                  // empty: env HOURGLASS_VSPHERE_PASSWORD
//	  "insecure_skip_verify": false,
//	  "datacenter": "",                // optional
//	  "clusters": [],                  // empty: every cluster in scope
//	  "request_timeout": "20s"
//	}
//
// Only govmomi.go imports govmomi; all logic is written against the small
// client interface in client.go.
package vsphere

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"sync"

	"github.com/panset/hourglass/pkg/hourglass"
)

// Name is the registered name of both the provider and the pressure source.
const Name = "vsphere"

func init() {
	hourglass.RegisterProvider(Name, NewProvider)
	hourglass.RegisterPressureSource(Name, NewPressureSource)
}

// newClient builds the real client for a config; tests replace it.
var newClient = func(c config) client { return newGovmomiClient(c) }

var (
	sharedMu sync.Mutex
	shared   = map[connKey]client{}
)

// sharedClient returns the one client (and so one vCenter session) for equal
// connection settings. The client connects lazily on first use, so building
// an extension never touches the network. Shared sessions live for the
// process lifetime.
func sharedClient(c config) client {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	k := c.key()
	cl, ok := shared[k]
	if !ok {
		cl = newClient(c)
		shared[k] = cl
	}
	return cl
}

// NewProvider is the factory registered as the "vsphere" provider.
func NewProvider(raw json.RawMessage) (hourglass.ResourceProvider, error) {
	c, err := parseConfig("vsphere provider", raw)
	if err != nil {
		return nil, err
	}
	return &provider{cfg: c, cl: sharedClient(c)}, nil
}

// NewPressureSource is the factory registered as the "vsphere" pressure source.
func NewPressureSource(raw json.RawMessage) (hourglass.PressureSource, error) {
	c, err := parseConfig("vsphere pressure source", raw)
	if err != nil {
		return nil, err
	}
	return &pressureSource{cfg: c, cl: sharedClient(c)}, nil
}

type provider struct {
	cfg config
	cl  client
}

func (p *provider) Name() string { return Name }

// List returns one Allocation per resource pool in scope, except each
// cluster's hidden root pool. ResourceName is the pool's own name; labels
// carry cluster, datacenter, parent and path.
func (p *provider) List(ctx context.Context) ([]hourglass.Allocation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pools, err := p.cl.ResourcePools(ctx)
	if err != nil {
		return nil, fmt.Errorf("vsphere: list resource pools: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	found := map[string]bool{}
	for _, rp := range pools {
		found[rp.Cluster] = true
	}
	for _, n := range p.cfg.Clusters {
		if !found[n] {
			return nil, fmt.Errorf("vsphere: cluster %q not found", n)
		}
	}
	sort.SliceStable(pools, func(i, j int) bool { return pools[i].Path < pools[j].Path })
	out := make([]hourglass.Allocation, 0, len(pools))
	for _, rp := range pools {
		if rp.Root || !p.cfg.inScope(rp.Cluster) {
			continue
		}
		out = append(out, hourglass.Allocation{
			ResourceName: rp.Name,
			Labels: map[string]string{
				"cluster":    rp.Cluster,
				"datacenter": rp.Datacenter,
				"parent":     rp.Parent,
				"path":       rp.Path,
			},
		})
	}
	return out, nil
}

type pressureSource struct {
	cfg config
	cl  client
}

func (p *pressureSource) Name() string { return Name }

// Pressure implements MET-1's vsphere source per D6: per cluster the highest
// of the CPU, memory and storage ratios, then the highest across the
// configured clusters. The Snapshot is ignored.
func (p *pressureSource) Pressure(ctx context.Context, _ hourglass.Snapshot) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	all, err := p.cl.Clusters(ctx)
	if err != nil {
		return 0, fmt.Errorf("vsphere: read cluster figures: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	found := map[string]bool{}
	var in []clusterFigures
	for _, c := range all {
		if p.cfg.inScope(c.Name) {
			in = append(in, c)
			found[c.Name] = true
		}
	}
	for _, n := range p.cfg.Clusters {
		if !found[n] {
			return 0, fmt.Errorf("vsphere: cluster %q not found", n)
		}
	}
	if len(in) == 0 {
		return 0, fmt.Errorf("vsphere: no clusters found")
	}
	highest := 0.0
	for _, c := range in {
		v, err := clusterPressure(c)
		if err != nil {
			return 0, err
		}
		highest = math.Max(highest, v)
	}
	return highest, nil
}

// clusterPressure is max(cpu, memory, storage) for one cluster (D6).
func clusterPressure(c clusterFigures) (float64, error) {
	cpu, err := reservationRatio(c.Name, "cpu", c.CPU)
	if err != nil {
		return 0, err
	}
	mem, err := reservationRatio(c.Name, "memory", c.Memory)
	if err != nil {
		return 0, err
	}
	v := math.Max(cpu, mem)
	if s, ok := storageRatio(c.Datastores); ok {
		v = math.Max(v, s)
	}
	return v, nil
}

// reservationRatio is reservationUsed / maxUsage clamped to [0,1]. A zero (or
// negative) capacity is an error.
func reservationRatio(cluster, what string, r resource) (float64, error) {
	if r.MaxUsage <= 0 {
		return 0, fmt.Errorf("vsphere: cluster %q reports zero %s capacity", cluster, what)
	}
	return clamp(float64(r.ReservationUsed) / float64(r.MaxUsage)), nil
}

// storageRatio is 1 - sum(free)/sum(capacity) over accessible datastores,
// clamped to [0,1]. ok is false when no accessible datastore has capacity, so
// the cluster contributes no storage ratio.
func storageRatio(ds []datastore) (float64, bool) {
	var capacity, free int64
	for _, d := range ds {
		if !d.Accessible || d.Capacity <= 0 {
			continue
		}
		capacity += d.Capacity
		free += d.FreeSpace
	}
	if capacity == 0 {
		return 0, false
	}
	return clamp(1 - float64(free)/float64(capacity)), true
}

// String methods keep the config (and its password) out of any print of the
// extensions: fmt cannot call secret.String on a field it reaches through an
// unexported one.
//
// The receivers are values so that printing the struct itself (%v, %+v of *p)
// is covered as well as printing the pointer.
func (p provider) String() string   { return "vsphere provider{" + p.cfg.String() + "}" }
func (p provider) GoString() string { return p.String() }
func (p provider) LogValue() slog.Value {
	return slog.StringValue(p.String())
}

func (p pressureSource) String() string   { return "vsphere pressure source{" + p.cfg.String() + "}" }
func (p pressureSource) GoString() string { return p.String() }
func (p pressureSource) LogValue() slog.Value {
	return slog.StringValue(p.String())
}

func clamp(v float64) float64 {
	switch {
	case math.IsNaN(v), v < 0:
		return 0
	case v > 1:
		return 1
	}
	return v
}
