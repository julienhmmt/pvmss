package apiv1

import (
	"context"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// allocateNextVMID picks the next VMID for a new VM. It prefers highest+1 from
// the cached cluster snapshot (cheap, no API call) and falls back to Proxmox's
// atomic /cluster/nextid when no snapshot is available. Shared by VM create and
// VM clone so the two paths cannot drift.
func allocateNextVMID(ctx context.Context, sm state.StateManager, client *proxmox.RestyClient) (int, error) {
	if snapshot := sm.GetProxmoxSnapshot(); snapshot != nil && len(snapshot.VMs) > 0 {
		highest := 0
		for _, svm := range snapshot.VMs {
			if svm.VMID > highest {
				highest = svm.VMID
			}
		}
		if highest > 0 {
			return highest + 1, nil
		}
	}
	nextID, err := proxmox.GetClusterNextIDResty(ctx, client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("api/v1: failed to get next VMID")
		return 0, err
	}
	return nextID, nil
}
