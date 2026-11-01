package vm

import (
	"context"
	"fmt"
	"maps"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"slices"
)

// auditVMClone is the audit action recorded for a successful own-VM clone.
const auditVMClone = "vm_clone"

// bytesPerMiB and bytesPerGiB convert the inventory's byte counts into the
// MB and GB units the gabarit and capacity checks speak.
const (
	bytesPerMiB = 1 << 20
	bytesPerGiB = 1 << 30
)

// CloneOwn duplicates a VM the actor owns as a full clone with a new name.
// It rides the create path: the same name/quota reservation, VMID allocation,
// retry and task wait. The source's config is held to the same rules a new
// VM is (planCreate's catalog, gabarit, isolation VLAN and disk-space gates)
// before a VMID is spent. Snapshots are not copied (Proxmox clones the
// current state only).
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

	policyService := deps.Policy

	gabarit, err := policyService.Gabarit(ctx, clusterName)
	if err != nil {
		return CreateResult{}, fmt.Errorf("read vmid range: %w", err)
	}

	deps.VMIDMin, deps.VMIDMax = gabarit.VMIDMin, gabarit.VMIDMax

	if err := checkCloneCatalog(ctx, deps.Store, clusterName, source); err != nil {
		return CreateResult{}, err
	}

	if err := checkCloneVLAN(gabarit.IsolationVLANTag, source); err != nil {
		return CreateResult{}, err
	}

	if err := checkCloneGabarit(ctx, policyService, clusterName, source); err != nil {
		return CreateResult{}, err
	}

	if err := checkCloneCapacity(ctx, policyService, clusterName, source); err != nil {
		return CreateResult{}, err
	}

	if err := checkCloneDiskSpace(ctx, deps.FreeSpace, source.Node, source.Disks); err != nil {
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

	// A clone is a new VM: it gets its own lease clock, not the source's.
	LeaseAfterDispatch(ctx, deps.Store, deps.Log, clusterName, result.VMID, gabarit.LeaseDays, actor.Pool)

	return result, nil
}

// checkCloneCatalog applies planCreate's catalog rules to the source: its node,
// every disk storage, and each NIC's bridge and model. The primary storage goes
// through validateCatalog; the rest of the disk storages are checked alike.
func checkCloneCatalog(ctx context.Context, st *store.Store, clusterName string, source Entity) error {
	resources, err := catalog.ApprovedResources(ctx, st, clusterName)
	if err != nil {
		return fmt.Errorf("read catalog: %w", err)
	}

	nics := make([]nicPlan, 0, len(source.NetworkInterfaces))
	for _, nic := range source.NetworkInterfaces {
		nics = append(nics, nicPlan{bridge: nic.Bridge, model: nic.Model})
	}

	storages := diskStorages(source.Disks)
	if len(storages) == 0 {
		return fmt.Errorf("%w: source vm has no disk to copy", ErrInvalidRequest)
	}

	if err := validateCatalog(CreateRequest{}, resources, source.Node, storages[0], nics); err != nil {
		return err
	}

	for _, storage := range storages[1:] {
		if !resources.HasStorage(storage, source.Node) {
			return fmt.Errorf("%w: storage %q on node %q", ErrNotApproved, storage, source.Node)
		}
	}

	return nil
}

// checkCloneVLAN refuses a source whose NIC VLAN tags differ from the cluster's
// isolation tag. planCreate stamps that tag on every NIC; a clone must not
// carry any other tag onto the network.
func checkCloneVLAN(isolationTag int, source Entity) error {
	for _, nic := range source.NetworkInterfaces {
		vlan := 0
		if nic.VLAN != nil {
			vlan = *nic.VLAN
		}

		if vlan != isolationTag {
			return fmt.Errorf("%w: nic %d VLAN %d does not match the isolation tag %d", ErrNotApproved, nic.Index, vlan, isolationTag)
		}
	}

	return nil
}

// checkCloneGabarit runs the gabarit ceiling on the source's current hardware,
// the same ceiling planCreate applies to a requested VM.
func checkCloneGabarit(ctx context.Context, policyService *policy.Policy, clusterName string, source Entity) error {
	return policyService.CheckGabarit(ctx, clusterName, source.Sockets, source.Cores,
		int(source.MemoryTotal/bytesPerMiB), int(source.DiskTotal/bytesPerGiB), len(source.NetworkInterfaces))
}

// checkCloneDiskSpace checks each target storage has room for the full copy of
// the disks on it, reusing checkLiveDiskSpace (the same refusal as create).
// Storages are visited in sorted order so the first refusal is deterministic.
func checkCloneDiskSpace(ctx context.Context, freeSpace FreeSpaceChecker, node string, disks []cluster.Disk) error {
	needed := make(map[string]int)
	for _, disk := range disks {
		needed[disk.Storage] += disk.SizeGB
	}

	for _, storage := range slices.Sorted(maps.Keys(needed)) {
		if err := checkLiveDiskSpace(ctx, freeSpace, node, storage, needed[storage]); err != nil {
			return err
		}
	}

	return nil
}

// diskStorages returns the distinct storages of disks, in disk order.
func diskStorages(disks []cluster.Disk) []string {
	var storages []string

	for _, disk := range disks {
		if !slices.Contains(storages, disk.Storage) {
			storages = append(storages, disk.Storage)
		}
	}

	return storages
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

// dispatchClone allocates a VMID, dispatches the full clone, and waits for the
// task. The wait and the rollback of a failed clone match the create path: any
// wait error rolls the half-made VM back and the caller releases the name and
// quota reservation. A cancelled wait is treated the same way (create does too).
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
