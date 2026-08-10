package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"regexp"
)

// Snapshot is a live snapshot entry owned by the cluster, not PVMSS.
type Snapshot = cluster.VMSnapshot

var (
	// ErrInvalidSnapshotName reports an invalid or reserved snapshot name.
	ErrInvalidSnapshotName = errors.New("invalid snapshot name")
	// ErrDuplicateSnapshotName reports a name already used by the VM.
	ErrDuplicateSnapshotName = errors.New("duplicate snapshot name")
	// ErrMaxSnapshotsReached reports a VM at its configured snapshot gabarit.
	ErrMaxSnapshotsReached = errors.New("maximum snapshots reached")
	// ErrVMStateRequiresRunning reports RAM-state capture on a stopped VM.
	ErrVMStateRequiresRunning = errors.New("vmstate requires a running VM")
	// ErrVMStateUnsupportedStorage reports a disk on incompatible storage.
	ErrVMStateUnsupportedStorage = errors.New("vmstate storage is unsupported")
	// ErrSnapshotNotFound reports a missing snapshot on an otherwise resolved VM.
	ErrSnapshotNotFound = errors.New("snapshot not found")
)

const maxSnapshotNameLength = 40

var snapshotNamePattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)

// SnapshotDependencies contains the resolved read, write, policy, and audit dependencies.
type SnapshotDependencies struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Reader      cluster.SnapshotReader
	Writer      cluster.SnapshotWriter
	Limits      config.VMLimits
	Audit       AuditRecorder
}

// ValidateSnapshotName validates the single accepted snapshot-name policy.
func ValidateSnapshotName(name string) error {
	if name == "current" || len(name) > maxSnapshotNameLength || !snapshotNamePattern.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidSnapshotName, name)
	}

	return nil
}

// ListSnapshots returns live snapshots and the configured per-VM snapshot gabarit.
func ListSnapshots(ctx context.Context, deps SnapshotDependencies) ([]Snapshot, int, error) {
	entity, err := resolveSnapshotTarget(deps)
	if err != nil {
		return nil, 0, err
	}

	snapshots, err := deps.Reader.ListSnapshots(ctx, entity.Node, entity.VMID)
	if err != nil {
		return nil, 0, fmt.Errorf("list snapshots: %w", err)
	}

	return visibleSnapshots(snapshots), effectiveMaxSnapshots(deps.Limits), nil
}

// CreateSnapshot validates and dispatches an asynchronous snapshot creation.
//
//nolint:wsl_v5 // snapshot guards remain ordered within each domain operation
func CreateSnapshot(ctx context.Context, deps SnapshotDependencies, name, description string, vmstate bool) (string, error) {
	entity, err := resolveSnapshotTarget(deps)
	if err != nil {
		return "", err
	}
	if err := ValidateSnapshotName(name); err != nil {
		return "", err
	}

	snapshots, err := deps.Reader.ListSnapshots(ctx, entity.Node, entity.VMID)
	if err != nil {
		return "", fmt.Errorf("list snapshots before create: %w", err)
	}
	snapshots = visibleSnapshots(snapshots)
	if snapshotNameExists(snapshots, name) {
		return "", fmt.Errorf("%w: a snapshot named %q already exists", ErrDuplicateSnapshotName, name)
	}
	maxSnapshots := effectiveMaxSnapshots(deps.Limits)
	if countRealSnapshots(snapshots) >= maxSnapshots {
		return "", fmt.Errorf("%w: this VM already holds the maximum of %d snapshots", ErrMaxSnapshotsReached, maxSnapshots)
	}
	if err := validateVMState(entity, deps.Index, vmstate); err != nil {
		return "", err
	}

	upid, err := deps.Writer.CreateSnapshot(ctx, entity.Node, entity.VMID, name, description, vmstate)
	if err != nil {
		return "", fmt.Errorf("create snapshot: %w", err)
	}
	if err := recordSnapshotAction(ctx, deps, "vm_snapshot_create"); err != nil {
		return "", err
	}

	return upid, nil
}

// RollbackSnapshot validates existence and dispatches an asynchronous rollback.
//
//nolint:wsl_v5 // snapshot guards remain ordered within each domain operation
func RollbackSnapshot(ctx context.Context, deps SnapshotDependencies, name string) (string, error) {
	entity, err := findSnapshotTarget(ctx, deps, name)
	if err != nil {
		return "", err
	}

	upid, err := deps.Writer.RollbackSnapshot(ctx, entity.Node, entity.VMID, name)
	if err != nil {
		return "", fmt.Errorf("rollback snapshot: %w", err)
	}
	if err := recordSnapshotAction(ctx, deps, "vm_snapshot_rollback"); err != nil {
		return "", err
	}

	return upid, nil
}

// DeleteSnapshot validates existence and dispatches an asynchronous deletion.
//
//nolint:wsl_v5 // snapshot guards remain ordered within each domain operation
func DeleteSnapshot(ctx context.Context, deps SnapshotDependencies, name string) (string, error) {
	entity, err := findSnapshotTarget(ctx, deps, name)
	if err != nil {
		return "", err
	}

	upid, err := deps.Writer.DeleteSnapshot(ctx, entity.Node, entity.VMID, name)
	if err != nil {
		return "", fmt.Errorf("delete snapshot: %w", err)
	}
	if err := recordSnapshotAction(ctx, deps, "vm_snapshot_delete"); err != nil {
		return "", err
	}

	return upid, nil
}

func resolveSnapshotTarget(deps SnapshotDependencies) (Entity, error) {
	if deps.Index == nil {
		return Entity{}, ErrNotFound
	}

	return Resolve(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
}

//nolint:wsl_v5 // snapshot guards remain ordered within each domain operation
func findSnapshotTarget(ctx context.Context, deps SnapshotDependencies, name string) (Entity, error) {
	entity, err := resolveSnapshotTarget(deps)
	if err != nil {
		return Entity{}, err
	}

	snapshots, err := deps.Reader.ListSnapshots(ctx, entity.Node, entity.VMID)
	if err != nil {
		return Entity{}, fmt.Errorf("list snapshots before action: %w", err)
	}
	if !snapshotNameExists(visibleSnapshots(snapshots), name) {
		return Entity{}, fmt.Errorf("%w: %q", ErrSnapshotNotFound, name)
	}

	return entity, nil
}

//nolint:wsl_v5 // snapshot guards remain ordered within each domain operation
func validateVMState(entity Entity, index *inventory.Index, vmstate bool) error {
	if !vmstate {
		return nil
	}
	if entity.Status != cluster.VMRunning {
		return ErrVMStateRequiresRunning
	}
	for _, disk := range entity.Disks {
		if !storageSupportsVMState(index, entity.Node, disk.Storage) {
			return fmt.Errorf("%w: disk %q is on a storage that does not support RAM state", ErrVMStateUnsupportedStorage, disk.Key)
		}
	}

	return nil
}

//nolint:wsl_v5 // snapshot guards remain ordered within each domain operation
func storageSupportsVMState(index *inventory.Index, node, name string) bool {
	if index == nil {
		return false
	}
	for _, storage := range index.StoragesByNode[node] {
		if storage.Name == name {
			return storage.SupportsVMState
		}
	}

	return false
}

func effectiveMaxSnapshots(limits config.VMLimits) int {
	if limits.MaxSnapshots > 0 {
		return limits.MaxSnapshots
	}

	return config.DefaultVMLimits().MaxSnapshots
}

func visibleSnapshots(snapshots []Snapshot) []Snapshot {
	visible := make([]Snapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.Name != "current" {
			visible = append(visible, snapshot)
		}
	}

	return visible
}

//nolint:wsl_v5 // snapshot guards remain ordered within each domain operation
func countRealSnapshots(snapshots []Snapshot) int {
	count := 0
	for _, snapshot := range snapshots {
		if snapshot.Name != "current" {
			count++
		}
	}

	return count
}

func snapshotNameExists(snapshots []Snapshot, name string) bool {
	for _, snapshot := range snapshots {
		if snapshot.Name == name {
			return true
		}
	}

	return false
}

func recordSnapshotAction(ctx context.Context, deps SnapshotDependencies, action string) error {
	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, action); err != nil {
		return fmt.Errorf("record snapshot audit: %w", err)
	}

	return nil
}
