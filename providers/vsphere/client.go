// SPDX-License-Identifier: MIT

package vsphere

import "context"

// client is the one small interface all provider and pressure logic is written
// against (SPEC "Testing without a vCenter", layer 2). The govmomi adapter in
// govmomi.go is the only implementation that talks to a vCenter.
//
// Implementations must be read-only: nothing here creates, changes or destroys
// inventory.
type client interface {
	// ResourcePools returns every resource pool of every cluster in scope,
	// including each cluster's hidden root pool (flagged Root).
	ResourcePools(ctx context.Context) ([]pool, error)
	// Clusters returns the capacity and allocation figures of every cluster in scope.
	Clusters(ctx context.Context) ([]clusterFigures, error)
	// Close ends the session. It is safe to call more than once.
	Close(ctx context.Context) error
}

// pool is one resource pool.
type pool struct {
	Name       string // the pool's own name
	Path       string // inventory path, e.g. /DC0/host/DC0_C0/Resources/lab-01_RP
	Parent     string // name of the parent pool ("" for a root pool)
	Cluster    string
	Datacenter string
	Root       bool // the cluster's hidden root pool ("Resources")
}

// resource is a used/maximum pair of figures (MHz for CPU, bytes for memory).
type resource struct {
	ReservationUsed int64
	MaxUsage        int64
}

// datastore is one datastore of a cluster.
type datastore struct {
	Name       string
	Capacity   int64 // bytes
	FreeSpace  int64 // bytes
	Accessible bool
}

// clusterFigures holds what the pressure formula (D6) needs for one cluster.
type clusterFigures struct {
	Name       string
	Datacenter string
	CPU        resource // root resource pool runtime.cpu
	Memory     resource // root resource pool runtime.memory
	Datastores []datastore
}
