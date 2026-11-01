package main

import (
	"context"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"time"
)

// leaseSweepInterval is how often the expiry sweep runs after the boot run.
const leaseSweepInterval = 24 * time.Hour

// runLeaseSweep stops and deletes expired leased VMs, once at startup then
// every leaseSweepInterval. A failed cycle for one cluster is logged inside
// the sweep and retried on the next tick.
func runLeaseSweep(ctx context.Context, st *store.Store, clusters *cluster.Registry, inventories *inventory.Registry, log *slog.Logger) {
	deps := vm.LeaseSweepDeps{
		Store:    st,
		Log:      log,
		Clusters: clusters.List,
		Index:    inventories.Index,
		Writer: func(name string) (cluster.Writer, error) {
			client, err := clusters.Client(name)
			if err != nil {
				return nil, err
			}

			writer, ok := client.(cluster.Writer)
			if !ok {
				return nil, cluster.ErrClusterNotFound
			}

			return writer, nil
		},
		Refresh: inventories.Refresh,
	}

	sweep := func() {
		if n := vm.SweepExpiredLeases(ctx, deps, time.Now()); n > 0 {
			log.InfoContext(ctx, "lease sweep completed", "component", "vm", "event", "lease_sweep", "deleted", n)
		}
	}

	log.DebugContext(ctx, "lease sweep loop started", "component", "vm", "intervalMs", leaseSweepInterval.Milliseconds())
	defer log.DebugContext(ctx, "lease sweep loop stopped", "component", "vm")

	sweep()

	ticker := time.NewTicker(leaseSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweep()
		}
	}
}
