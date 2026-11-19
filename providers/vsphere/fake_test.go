// SPDX-License-Identifier: MIT

package vsphere

import (
	"context"
	"sync"
)

// fakeClient is the hand-written fake of the client interface (layer 2).
type fakeClient struct {
	mu       sync.Mutex
	pools    []pool
	clusters []clusterFigures
	err      error
	block    chan struct{} // when set, calls wait for it or for ctx
	calls    int
	closed   int
}

func (f *fakeClient) wait(ctx context.Context) error {
	f.mu.Lock()
	f.calls++
	b, err := f.block, f.err
	f.mu.Unlock()
	if b != nil {
		select {
		case <-b:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

func (f *fakeClient) ResourcePools(ctx context.Context) ([]pool, error) {
	if err := f.wait(ctx); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]pool(nil), f.pools...), nil
}

func (f *fakeClient) Clusters(ctx context.Context) ([]clusterFigures, error) {
	if err := f.wait(ctx); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]clusterFigures(nil), f.clusters...), nil
}

func (f *fakeClient) Close(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed++
	return nil
}
