package vm

import (
	"context"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
)

// auditVMClone is the audit action recorded for a successful own-VM clone.
const auditVMClone = "vm_clone"

// bytesPerMiB and bytesPerGiB convert the inventory's byte counts into the
// MB and GB units the node capacity check speaks.
const (
	bytesPerMiB = 1 << 20
	bytesPerGiB = 1 << 30
)

// CloneOwn duplicates a VM the actor owns as a full clone with a new name.
// It rides the create path: the same name/quota reservation, VMID allocation,
// retry and task wait. Everything else is refused up front:
//
//  1. administrators are refused (a clone lands in the actor's pool, and admins have none)
//  2. the source must resolve for the actor (pool scoping, PVMSS scope)
//  3. every NIC bridge must still be approved on the target cluster
//  4. the source node must still have capacity for the copy
//
// The clone is always full and stays on the source node. Snapshots are not
// copied (Proxmox clones the current state only).
func CloneOwn(ctx context.Context, actor auth.Identity, index inventory.LookupSource, clusterName string, vmid int, name string, deps CreateDeps) (CreateResult, error) {
	if actor.IsAdmin {
		return CreateResult{}, ErrAdminCannotCreate
	}

	if actor.Pool == "" {
		return CreateResult{}, ErrNoPool
	}

	source, err := Resolve(index, actor, clusterName, vmid, deps.Store)
	if err != nil {
		return CreateResult{}, err
	}

	policyService := selectPolicyService(deps.Store, deps.Services)

	gabarit, err := policyService.Gabarit(ctx, clusterName)
	if err != nil {
		return CreateResult{}, fmt.Errorf("read vmid range: %w", err)
	}

	deps.VMIDMin, deps.VMIDMax = gabarit.VMIDMin, gabarit.VMIDMax

	if err := checkCloneBridges(ctx, deps.Store, clusterName, source); err != nil {
		return CreateResult{}, err
	}

	if err := checkCloneCapacity(ctx, policyService, clusterName, source); err != nil {
		return CreateResult{}, err
	}

	release, err := reserveCreate(ctx, policyService, clusterName, actor, name)
	if err != nil {
		return CreateResult{}, err
	}

	result, err := dispatchClone(ctx, deps, clusterName, actor, source, name)
	if err != nil {
		release()

		return CreateResult{}, err
	}

	return result, nil
}

// checkCloneBridges refuses the clone when a NIC sits on a bridge the target
// cluster no longer approves for the source node. The error names the bridge,
// so an admin knows which approval to restore (same failure path as create).
func checkCloneBridges(ctx context.Context, st *store.Store, clusterName string, source Entity) error {
	resources, err := catalog.ApprovedResources(ctx, st, clusterName)
	if err != nil {
		return fmt.Errorf("read catalog: %w", err)
	}

	for _, nic := range source.NetworkInterfaces {
		if nic.Bridge != "" && !resources.HasBridge(nic.Bridge, source.Node) {
			return fmt.Errorf("%w: bridge %q is not approved on node %s", ErrNotApproved, nic.Bridge, source.Node)
		}
	}

	return nil
}

// checkCloneCapacity runs the node-capacity gate on the source node for the
// copy. The clone is a new VM, so nothing is excluded from the usage.
func checkCloneCapacity(ctx context.Context, policyService *policy.Policy, clusterName string, source Entity) error {
	return policyService.CheckNodeCapacity(ctx, clusterName, source.Node, policy.CapacityDelta{
		Sockets:  source.Sockets,
		Cores:    source.Cores,
		MemoryMB: int(source.MemoryTotal / bytesPerMiB),
		DiskGB:   int(source.DiskTotal / bytesPerGiB),
	})
}

// dispatchClone allocates a VMID, dispatches the full clone, waits for the
// task (the name reservation must outlive the dispatch), and rolls back a
// failed clone the same way a failed create is rolled back.
func dispatchClone(ctx context.Context, deps CreateDeps, clusterName string, actor auth.Identity, source Entity, name string) (CreateResult, error) {
	newVMID, err := allocateVMID(ctx, deps)
	if err != nil {
		return CreateResult{}, allocateVMIDError(err)
	}

	spec := cluster.CloneSpec{
		SourceVMID: source.VMID,
		SourceNode: source.Node,
		NewVMID:    newVMID,
		Name:       name,
		Full:       true,
		Pool:       actor.Pool,
	}

	finalVMID, upid, err := dispatchCloneWithRetry(ctx, deps, spec)
	if err != nil {
		return CreateResult{}, err
	}

	if waitErr := waitCreateTask(ctx, deps.Creator, upid); waitErr != nil {
		deps.Log.ErrorContext(ctx, "clone task wait failed", "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", waitErr)
		rollbackFailedCreate(ctx, deps, actor, clusterName, finalVMID, source.Node, "clone task failed")

		return CreateResult{}, fmt.Errorf("%w: clone task: %w", ErrClusterCreate, waitErr)
	}

	detail := fmt.Sprintf("cloned from vmid %d", source.VMID)
	if err := deps.Store.RecordVMActionDetail(ctx, actor.Username, clusterName, finalVMID, auditVMClone, detail); err != nil {
		deps.Log.ErrorContext(ctx, auditLogMsg, "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", err)
	}

	return CreateResult{Cluster: clusterName, VMID: finalVMID, Name: name, Node: source.Node, UPID: upid}, nil
}
