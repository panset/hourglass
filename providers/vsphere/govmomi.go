// SPDX-License-Identifier: MIT

package vsphere

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

// govmomiClient is the thin adapter between the client interface and a real
// vCenter (or the govmomi simulator). It only reads: session login/logout,
// inventory lookups and property retrieval. It connects lazily, logs in again
// once when a call fails with NotAuthenticated, and logs out on Close.
//
// Privileges the account needs are listed in the task RESULT: System.Read /
// System.View on the in-scope datacenters, clusters, resource pools and
// datastores (the built-in Read-only role).
type govmomiClient struct {
	cfg config

	mu sync.Mutex // one call at a time; a poll runs every few minutes
	vc *vim25.Client
	sm *session.Manager
}

func newGovmomiClient(c config) *govmomiClient { return &govmomiClient{cfg: c} }

// String keeps the password out of any print of the adapter.
func (g *govmomiClient) String() string   { return "vsphere client{" + g.cfg.String() + "}" }
func (g *govmomiClient) GoString() string { return g.String() }

func (g *govmomiClient) loginURL() (*url.URL, error) {
	u, err := url.Parse(g.cfg.URL)
	if err != nil {
		return nil, errors.New("invalid url")
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/sdk"
	}
	u.User = url.UserPassword(g.cfg.Username, g.cfg.Password.reveal())
	return u, nil
}

// connect opens a fresh SOAP client and logs in. Caller holds g.mu.
func (g *govmomiClient) connect(ctx context.Context) error {
	u, err := g.loginURL()
	if err != nil {
		return err
	}
	sc := soap.NewClient(u, g.cfg.InsecureSkipVerify)
	vc, err := vim25.NewClient(ctx, sc)
	if err != nil {
		return g.scrub(err)
	}
	sm := session.NewManager(vc)
	if err := sm.Login(ctx, u.User); err != nil {
		return g.scrub(err)
	}
	g.vc, g.sm = vc, sm
	return nil
}

// scrub removes the password from an error text, as a second line of defence
// (govmomi errors do not normally carry it).
func (g *govmomiClient) scrub(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if pw := g.cfg.Password.reveal(); pw != "" && strings.Contains(msg, pw) {
		return errors.New(strings.ReplaceAll(msg, pw, "[redacted]"))
	}
	return err
}

func isNotAuthenticated(err error) bool {
	if err == nil {
		return false
	}
	if soap.IsSoapFault(err) {
		if _, ok := soap.ToSoapFault(err).VimFault().(types.NotAuthenticated); ok {
			return true
		}
	}
	if soap.IsVimFault(err) {
		if _, ok := soap.ToVimFault(err).(*types.NotAuthenticated); ok {
			return true
		}
	}
	return false
}

// do runs fn with a connected client under the request timeout, logging in
// again once when the session is no longer valid.
func (g *govmomiClient) do(ctx context.Context, fn func(context.Context, *vim25.Client) error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, g.cfg.RequestTimeout)
	defer cancel()

	if g.vc == nil {
		if err := g.connect(ctx); err != nil {
			return err
		}
	}
	err := fn(ctx, g.vc)
	if isNotAuthenticated(err) {
		g.vc, g.sm = nil, nil
		if err = g.connect(ctx); err != nil {
			return err
		}
		err = fn(ctx, g.vc)
	}
	return g.scrub(err)
}

// Close logs out. It is safe to call more than once and with no session.
func (g *govmomiClient) Close(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.sm == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, g.cfg.RequestTimeout)
	defer cancel()
	sm := g.sm
	g.vc, g.sm = nil, nil
	if err := sm.Logout(ctx); err != nil && !isNotAuthenticated(err) {
		return g.scrub(err)
	}
	return nil
}

// clusterRef is a cluster found in scope.
type clusterRef struct {
	name, datacenter, path string
	obj                    mo.ClusterComputeResource
}

// clusters finds every cluster in the configured datacenter (or all of them).
func (g *govmomiClient) clusters(ctx context.Context, vc *vim25.Client) ([]clusterRef, error) {
	f := find.NewFinder(vc)
	var dcs []*object.Datacenter
	var err error
	if g.cfg.Datacenter != "" {
		var dc *object.Datacenter
		if dc, err = f.Datacenter(ctx, g.cfg.Datacenter); err == nil {
			dcs = []*object.Datacenter{dc}
		}
	} else {
		dcs, err = f.DatacenterList(ctx, "*")
	}
	if err != nil {
		var nf *find.NotFoundError
		if errors.As(err, &nf) {
			return nil, nil
		}
		return nil, err
	}
	pc := property.DefaultCollector(vc)
	var out []clusterRef
	for _, dc := range dcs {
		f.SetDatacenter(dc)
		list, err := f.ClusterComputeResourceList(ctx, "*")
		if err != nil {
			var nf *find.NotFoundError
			if errors.As(err, &nf) {
				continue
			}
			return nil, err
		}
		for _, c := range list {
			var o mo.ClusterComputeResource
			if err := pc.RetrieveOne(ctx, c.Reference(), []string{"name", "resourcePool", "datastore"}, &o); err != nil {
				return nil, err
			}
			out = append(out, clusterRef{name: o.Name, datacenter: path.Base(dc.InventoryPath), path: c.InventoryPath, obj: o})
		}
	}
	return out, nil
}

// ResourcePools lists every pool of every in-scope cluster by walking each
// cluster's root pool down through resourcePool (child) references.
func (g *govmomiClient) ResourcePools(ctx context.Context) ([]pool, error) {
	var out []pool
	err := g.do(ctx, func(ctx context.Context, vc *vim25.Client) error {
		out = nil
		cls, err := g.clusters(ctx, vc)
		if err != nil {
			return err
		}
		pc := property.DefaultCollector(vc)
		for _, cl := range cls {
			if cl.obj.ResourcePool == nil {
				continue
			}
			type node struct {
				ref    types.ManagedObjectReference
				parent string
				path   string
				root   bool
			}
			level := []node{{ref: *cl.obj.ResourcePool, root: true}}
			for len(level) > 0 {
				refs := make([]types.ManagedObjectReference, len(level))
				for i, n := range level {
					refs[i] = n.ref
				}
				var rps []mo.ResourcePool
				if err := pc.Retrieve(ctx, refs, []string{"name", "resourcePool"}, &rps); err != nil {
					return err
				}
				byRef := map[types.ManagedObjectReference]mo.ResourcePool{}
				for _, rp := range rps {
					byRef[rp.Reference()] = rp
				}
				var next []node
				for _, n := range level {
					rp, ok := byRef[n.ref]
					if !ok {
						continue
					}
					p := path.Join(cl.path, rp.Name)
					if !n.root {
						p = path.Join(n.path, rp.Name)
					}
					out = append(out, pool{Name: rp.Name, Path: p, Parent: n.parent, Cluster: cl.name, Datacenter: cl.datacenter, Root: n.root})
					for _, c := range rp.ResourcePool {
						next = append(next, node{ref: c, parent: rp.Name, path: p})
					}
				}
				level = next
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Clusters reads each in-scope cluster's root pool runtime figures and its
// datastores' capacity, free space and accessibility.
func (g *govmomiClient) Clusters(ctx context.Context) ([]clusterFigures, error) {
	var out []clusterFigures
	err := g.do(ctx, func(ctx context.Context, vc *vim25.Client) error {
		out = nil
		cls, err := g.clusters(ctx, vc)
		if err != nil {
			return err
		}
		pc := property.DefaultCollector(vc)
		for _, cl := range cls {
			if cl.obj.ResourcePool == nil {
				return fmt.Errorf("cluster %q has no root resource pool", cl.name)
			}
			var root mo.ResourcePool
			if err := pc.RetrieveOne(ctx, *cl.obj.ResourcePool, []string{"runtime"}, &root); err != nil {
				return err
			}
			cf := clusterFigures{
				Name:       cl.name,
				Datacenter: cl.datacenter,
				CPU:        resource{ReservationUsed: root.Runtime.Cpu.ReservationUsed, MaxUsage: root.Runtime.Cpu.MaxUsage},
				Memory:     resource{ReservationUsed: root.Runtime.Memory.ReservationUsed, MaxUsage: root.Runtime.Memory.MaxUsage},
			}
			if len(cl.obj.Datastore) > 0 {
				var dss []mo.Datastore
				if err := pc.Retrieve(ctx, cl.obj.Datastore, []string{"name", "summary"}, &dss); err != nil {
					return err
				}
				for _, d := range dss {
					acc := d.Summary.Accessible
					cf.Datastores = append(cf.Datastores, datastore{
						Name: d.Name, Capacity: d.Summary.Capacity, FreeSpace: d.Summary.FreeSpace, Accessible: acc,
					})
				}
			}
			out = append(out, cf)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
