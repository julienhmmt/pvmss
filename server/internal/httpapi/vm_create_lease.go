package httpapi

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
)

// bindLeaseAfterCreate waits for the create task and then records the new
// VM's creation identity on its lease, so the sweeper can later tell it apart
// from a VM that reuses the same VMID. Detached from the request and bounded
// like the inventory refresh; a failure is logged and leaves the lease
// unbound, which the sweeper treats as "never delete".
func (h *VMCreate) bindLeaseAfterCreate(ctx context.Context, target createTarget, result vm.CreateResult) {
	if _, err := h.store.GetVMLease(ctx, result.Cluster, result.VMID); errors.Is(err, store.ErrLeaseNotFound) {
		return
	}

	reader, ok := target.writer.(cluster.SnapshotConfigReader)
	if !ok {
		h.log.WarnContext(ctx, "vm lease identity not bound: config reader unavailable", "component", "httpapi", "cluster", result.Cluster, "vmid", result.VMID)

		return
	}

	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), vm.MaxCreateTaskWait+createRefreshSlack)

	go func() {
		defer cancel()

		if err := vm.WaitCreateTask(bounded, target.creator, result.UPID); err != nil {
			h.log.WarnContext(bounded, "vm lease identity not bound: create task did not complete", "component", "httpapi", "cluster", result.Cluster, "vmid", result.VMID, "error", err)

			return
		}

		if err := vm.BindLeaseIdentity(bounded, h.store, reader, result.Cluster, result.Node, result.VMID); err != nil {
			h.log.WarnContext(bounded, "vm lease identity not bound", "component", "httpapi", "cluster", result.Cluster, "vmid", result.VMID, "error", err)
		}
	}()
}
