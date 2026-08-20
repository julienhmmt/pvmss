package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
)

var (
	// ErrVMNotStopped reports an operation that requires a stopped VM.
	ErrVMNotStopped = errors.New("vm must be stopped")
	// ErrDiskStorageNotApproved reports a storage absent from the catalog.
	ErrDiskStorageNotApproved = errors.New("storage is not approved")
	// ErrDiskSizeExceedsLimit reports a disk larger than the configured bound.
	ErrDiskSizeExceedsLimit = errors.New("disk size exceeds limit")
	// ErrBusFull reports that the requested disk bus has no free slot.
	ErrBusFull = errors.New("disk bus is full")
	// ErrInvalidDiskSize reports a non-positive disk size.
	ErrInvalidDiskSize = errors.New("invalid disk size")
	// ErrDiskNotFound reports a disk absent from the resolved VM.
	ErrDiskNotFound = errors.New("disk not found")
	// ErrDiskSizeNotGreater reports an unsupported disk shrink operation.
	ErrDiskSizeNotGreater = errors.New("disk size must be greater")
	// ErrBootDiskProtected reports an attempt to delete the boot disk.
	ErrBootDiskProtected = errors.New("boot disk cannot be deleted")
)

// maxDisksForBus bounds how many disks AddDisk allows per bus. IDE is 2, not
// the hardware's 4 slots: ide2 is reserved for the CD-ROM feature and ide3
// for the cloud-init drive (cluster/proxmox_config.go's cdromDiskKey and
// cloudInitDiskKey) — the real client never offers either as a regular disk
// slot, so this count must match.
var maxDisksForBus = map[cluster.DiskBus]int{
	cluster.DiskBusVirtio: 16,
	cluster.DiskBusSCSI:   31,
	cluster.DiskBusSATA:   6,
	cluster.DiskBusIDE:    2,
}

// DiskDependencies contains the resolved VM write dependencies shared by disk operations.
type DiskDependencies struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Writer      cluster.Writer
	Resources   catalog.Resources
	Policy      *policy.Policy
	Gabarit     policy.Gabarit
	Audit       AuditRecorder
	Refresher   IndexRefresher
}

// AddDisk attaches a new disk after resolving ownership and validating every guard.
func AddDisk(ctx context.Context, deps DiskDependencies, bus cluster.DiskBus, storage string, sizeGB int) (cluster.Disk, error) {
	entity, err := resolveDiskTarget(deps)
	if err != nil {
		return cluster.Disk{}, err
	}

	if entity.Status != cluster.VMStopped {
		return cluster.Disk{}, ErrVMNotStopped
	}

	if !deps.Resources.HasStorage(storage, entity.Node) {
		return cluster.Disk{}, fmt.Errorf("%w: %s", ErrDiskStorageNotApproved, storage)
	}

	if sizeGB <= 0 {
		return cluster.Disk{}, ErrInvalidDiskSize
	}

	gabarit, err := resolveGabarit(ctx, deps.Policy, deps.Gabarit, deps.ClusterName, func(g policy.Gabarit) bool { return g.MaxDiskPerVMGB > 0 })
	if err != nil {
		return cluster.Disk{}, err
	}

	if sizeGB > gabarit.MaxDiskPerVMGB {
		return cluster.Disk{}, ErrDiskSizeExceedsLimit
	}

	maxSlots, ok := maxDisksForBus[bus]
	if !ok {
		return cluster.Disk{}, fmt.Errorf("%w: unknown bus %q", ErrBusFull, bus)
	}

	if countDisks(entity.Disks, bus) >= maxSlots {
		return cluster.Disk{}, ErrBusFull
	}

	key, err := deps.Writer.AddDisk(ctx, entity.Node, entity.VMID, string(bus), storage, sizeGB)
	if err != nil {
		return cluster.Disk{}, fmt.Errorf("add disk: %w", err)
	}

	if err := recordDiskAction(ctx, deps, "add_disk"); err != nil {
		return cluster.Disk{}, err
	}

	if err := refreshDiskIndex(ctx, deps); err != nil {
		return cluster.Disk{}, err
	}

	return cluster.Disk{Key: key, Bus: bus, BusIndex: nextDiskIndex(entity.Disks, bus), Storage: storage, SizeGB: sizeGB}, nil
}

// ResizeDisk grows an existing disk; online growth is allowed while the VM runs.
func ResizeDisk(ctx context.Context, deps DiskDependencies, diskKey string, sizeGB int) error {
	entity, disk, err := findDiskTarget(deps, diskKey)
	if err != nil {
		return err
	}

	if sizeGB <= disk.SizeGB {
		return ErrDiskSizeNotGreater
	}

	gabarit := deps.Gabarit
	if deps.Policy != nil {
		gabarit, err = deps.Policy.Gabarit(ctx, deps.ClusterName)
		if err != nil {
			return fmt.Errorf("read gabarit: %w", err)
		}
	}

	if deps.Policy == nil && gabarit.MaxDiskPerVMGB == 0 {
		return policy.ErrUnavailable
	}

	if sizeGB > gabarit.MaxDiskPerVMGB {
		return ErrDiskSizeExceedsLimit
	}

	if err := deps.Writer.ResizeDisk(ctx, entity.Node, entity.VMID, diskKey, sizeGB); err != nil {
		return fmt.Errorf("resize disk: %w", err)
	}

	if err := recordDiskAction(ctx, deps, "resize_disk"); err != nil {
		return err
	}

	return refreshDiskIndex(ctx, deps)
}

// DeleteDisk removes a non-boot disk from a stopped VM.
func DeleteDisk(ctx context.Context, deps DiskDependencies, diskKey string) error {
	entity, disk, err := findDiskTarget(deps, diskKey)
	if err != nil {
		return err
	}

	if entity.Status != cluster.VMStopped {
		return ErrVMNotStopped
	}

	if disk.IsBoot {
		return ErrBootDiskProtected
	}

	if err := deps.Writer.DeleteDisk(ctx, entity.Node, entity.VMID, diskKey); err != nil {
		return fmt.Errorf("delete disk: %w", err)
	}

	if err := recordDiskAction(ctx, deps, "delete_disk"); err != nil {
		return err
	}

	return refreshDiskIndex(ctx, deps)
}

func resolveDiskTarget(deps DiskDependencies) (Entity, error) {
	if deps.Index == nil {
		return Entity{}, ErrNotFound
	}

	return Resolve(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
}

func findDiskTarget(deps DiskDependencies, diskKey string) (Entity, cluster.Disk, error) {
	entity, err := resolveDiskTarget(deps)
	if err != nil {
		return Entity{}, cluster.Disk{}, err
	}

	for _, disk := range entity.Disks {
		if disk.Key == diskKey {
			return entity, disk, nil
		}
	}

	return Entity{}, cluster.Disk{}, fmt.Errorf("%w: %s", ErrDiskNotFound, diskKey)
}

func recordDiskAction(ctx context.Context, deps DiskDependencies, action string) error {
	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, action); err != nil {
		return fmt.Errorf("record disk audit: %w", err)
	}

	return nil
}

func refreshDiskIndex(ctx context.Context, deps DiskDependencies) error {
	if _, err := deps.Refresher.Refresh(ctx); err != nil {
		return fmt.Errorf("refresh inventory after disk write: %w", err)
	}

	return nil
}

func countDisks(disks []cluster.Disk, bus cluster.DiskBus) int {
	count := 0

	for _, disk := range disks {
		if disk.Bus == bus {
			count++
		}
	}

	return count
}

func nextDiskIndex(disks []cluster.Disk, bus cluster.DiskBus) int {
	index := 0
	for _, disk := range disks {
		if disk.Bus == bus && disk.BusIndex >= index {
			index = disk.BusIndex + 1
		}
	}

	return index
}
