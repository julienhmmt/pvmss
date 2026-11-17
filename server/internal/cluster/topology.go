package cluster

import (
	"context"
	"sync"
)

// TopologyReader is an optional Client capability: a Snapshot whose VMs are
// the unhydrated /cluster/resources rows (no config, disks, NICs) and with no
// ProxmoxVersion, for callers that only need nodes, storages or VM counts. On Proxmox it skips the per-VM config/status hydration, which
// dominates Snapshot's latency on clusters with many VMs.
type TopologyReader interface {
	Topology(ctx context.Context) (Snapshot, error)
}

// IncrementalSnapshotter is an optional Client capability: a Snapshot that
// reuses prev's per-VM detail for VMs unchanged since prev. The inventory
// worker uses it for its periodic cycle only; writes still take a full
// Snapshot.
type IncrementalSnapshotter interface {
	SnapshotFrom(ctx context.Context, prev Snapshot) (Snapshot, error)
}

// ReadTopology returns the cluster's nodes and storages, using Topology when
// the source offers it and a full Snapshot otherwise.
func ReadTopology(ctx context.Context, source interface {
	Snapshot(ctx context.Context) (Snapshot, error)
},
) (Snapshot, error) {
	if reader, ok := source.(TopologyReader); ok {
		return reader.Topology(ctx)
	}

	return source.Snapshot(ctx)
}

// fanOut calls fn for every item with at most hydrateConcurrency calls in
// flight and returns the results in input order. The first error cancels the
// remaining calls and is returned.
func fanOut[In, Out any](ctx context.Context, items []In, fn func(context.Context, In) (Out, error)) ([]Out, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
		out      = make([]Out, len(items))
		sem      = make(chan struct{}, hydrateConcurrency)
	)

	for i, item := range items {
		sem <- struct{}{}

		wg.Go(func() {
			defer func() { <-sem }()

			value, err := fn(ctx, item)
			if err != nil {
				once.Do(func() { firstErr = err; cancel() })

				return
			}

			out[i] = value
		})
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return out, nil
}
