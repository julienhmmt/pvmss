package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"regexp"
	"time"
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
	ErrVMStateRequiresRunning = errors.New("vmstate requires a running vm")
	// ErrVMStateUnsupportedStorage reports a disk on incompatible storage.
	ErrVMStateUnsupportedStorage = errors.New("vmstate storage is unsupported")
	// ErrSnapshotUnsupportedStorage reports a disk on storage that cannot
	// hold snapshots at all (plain lvm, iscsi, raw on file storage — ticket
	// 07). Proxmox rejects such a create outright, so PVMSS refuses before
	// dispatching.
	ErrSnapshotUnsupportedStorage = errors.New("snapshot storage is unsupported")
	// ErrSnapshotNotFound reports a missing snapshot on an otherwise resolved VM.
	ErrSnapshotNotFound = errors.New("snapshot not found")
	// ErrVMLocked reports a Proxmox lock that did not clear within the retry
	// budget (ticket 06). The error message names the lock; a
	// lock=snapshot-delete left behind by a failed delete carries the
	// operator command `qm unlock <vmid>`.
	ErrVMLocked = errors.New("vm is locked")
)

const maxSnapshotNameLength = 40

// currentSnapshotName is Proxmox's pseudo-entry for the live state — filtered
// from lists, never a real snapshot, but resolvable for config reads (ticket
// 08).
const currentSnapshotName = "current"

// snapshotNamePattern mirrors Proxmox's own pve-configid format
// (^[a-z][a-z0-9_-]+$ case-insensitive): a leading letter, at least two
// characters, no dots. Anything looser is accepted here and then rejected by
// Proxmox, which the user only sees as a failed request.
var snapshotNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]+$`)

// SnapshotDependencies contains the resolved read, write, policy, and audit dependencies.
type SnapshotDependencies struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Reader      cluster.SnapshotReader
	Writer      cluster.SnapshotWriter
	Policy      *policy.Policy
	Gabarit     policy.Gabarit
	Audit       AuditRecorder
}

// ValidateSnapshotName validates the single accepted snapshot-name policy.
func ValidateSnapshotName(name string) error {
	if name == currentSnapshotName || len(name) > maxSnapshotNameLength || !snapshotNamePattern.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidSnapshotName, name)
	}

	return nil
}

// ListSnapshots returns live snapshots, the configured per-VM snapshot
// gabarit, and the VM's current snapshot capability (ticket 07).
func ListSnapshots(ctx context.Context, deps SnapshotDependencies) ([]Snapshot, int, SnapshotCapability, error) {
	entity, err := resolveSnapshotTarget(deps)
	if err != nil {
		return nil, 0, SnapshotCapability{}, err
	}

	snapshots, err := deps.Reader.ListSnapshots(ctx, entity.Node, entity.VMID)
	if err != nil {
		return nil, 0, SnapshotCapability{}, fmt.Errorf("list snapshots: %w", err)
	}

	gabarit, err := deps.readGabarit(ctx)
	if err != nil {
		return nil, 0, SnapshotCapability{}, err
	}

	return visibleSnapshots(snapshots), gabarit.MaxSnapshots, ComputeSnapshotCapability(entity, deps.Index), nil
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
	gabarit, err := deps.readGabarit(ctx)
	if err != nil {
		return "", err
	}
	maxSnapshots := gabarit.MaxSnapshots
	if countRealSnapshots(snapshots) >= maxSnapshots {
		return "", fmt.Errorf("%w: this VM already holds the maximum of %d snapshots", ErrMaxSnapshotsReached, maxSnapshots)
	}
	if err := validateVMState(entity, deps.Index, vmstate); err != nil {
		return "", err
	}

	// Ticket 07: refuse before dispatching when any disk sits on storage that
	// cannot hold snapshots (Proxmox rejects outright) — the UI already
	// greys the create button via the capability field of the list response.
	capability := ComputeSnapshotCapability(entity, deps.Index)
	if !capability.CanSnapshot {
		return "", fmt.Errorf("%w: %s", ErrSnapshotUnsupportedStorage, capability.Warnings[0])
	}

	upid, err := snapshotWithLockRetry(ctx, deps, func() (string, error) {
		return deps.Writer.CreateSnapshot(ctx, entity.Node, entity.VMID, name, description, vmstate)
	})
	if err != nil {
		return "", err
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

	upid, err := snapshotWithLockRetry(ctx, deps, func() (string, error) {
		return deps.Writer.RollbackSnapshot(ctx, entity.Node, entity.VMID, name)
	})
	if err != nil {
		return "", err
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

	upid, err := snapshotWithLockRetry(ctx, deps, func() (string, error) {
		return deps.Writer.DeleteSnapshot(ctx, entity.Node, entity.VMID, name)
	})
	if err != nil {
		return "", err
	}
	if err := recordSnapshotAction(ctx, deps, "vm_snapshot_delete"); err != nil {
		return "", err
	}

	return upid, nil
}

// snapshotWithLockRetry dispatches a snapshot write, retrying when Proxmox
// rejects it with "VM is locked (lockname)" — the same bounded retry-on-lock
// as the power actions (ticket 08), reusing extractLockName and the
// LockRetryPollInterval / MaxLockRetryWait budgets. A VM stuck at
// lock=snapshot-delete (NFS ESTALE, pegaprox incident #422) cannot clear
// itself by waiting: the expiry error then tells the operator to run
// `qm unlock <vmid>` on the node.
func snapshotWithLockRetry(ctx context.Context, deps SnapshotDependencies, dispatch func() (string, error)) (string, error) {
	upid, err := dispatch()
	if err == nil {
		return upid, nil
	}

	lockName, locked := extractLockName(err)
	if !locked {
		return "", err
	}

	deadline := time.NewTimer(MaxLockRetryWait)
	defer deadline.Stop()

	ticker := time.NewTicker(LockRetryPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			upid, err = dispatch()
			if err == nil {
				return upid, nil
			}

			if _, stillLocked := extractLockName(err); !stillLocked {
				return "", err
			}
		case <-deadline.C:
			if lockName == "snapshot-delete" {
				return "", fmt.Errorf("%w: VM %d is locked by a %s left behind by a failed snapshot delete — run `qm unlock %d` on the node", ErrVMLocked, deps.VMID, lockName, deps.VMID)
			}

			return "", fmt.Errorf("%w: VM %d is locked by a %s; retry once it completes", ErrVMLocked, deps.VMID, lockName)
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// SnapshotConfig returns one snapshot's stored config as a flat key→value
// map (ticket 08) — the pre-rollback diff. "current" (the pseudo-entry,
// filtered from lists) maps to the live config. A named snapshot must exist
// (404 snapshot_not_found); "current" always resolves.
func SnapshotConfig(ctx context.Context, deps SnapshotDependencies, name string) (map[string]string, error) {
	var (
		entity Entity
		err    error
	)
	if name == currentSnapshotName {
		entity, err = resolveSnapshotTarget(deps)
	} else {
		entity, err = findSnapshotTarget(ctx, deps, name)
	}

	if err != nil {
		return nil, err
	}

	reader, ok := deps.Reader.(cluster.SnapshotConfigReader)
	if !ok {
		return nil, cluster.ErrNotImplemented
	}

	config, err := reader.SnapshotConfig(ctx, entity.Node, entity.VMID, name)
	if err != nil {
		return nil, fmt.Errorf("read snapshot config: %w", err)
	}

	return config, nil
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
		if _, canVMState := diskStorageCapability(index, entity.Node, disk); !canVMState {
			return fmt.Errorf("%w: disk %q is on a storage that does not support RAM state", ErrVMStateUnsupportedStorage, disk.Key)
		}
	}

	return nil
}

// SnapshotCapability describes whether this VM can take snapshots and
// RAM-state snapshots right now, with human-readable reasons (ticket 07).
// Computed from the projection only — no cluster call.
type SnapshotCapability struct {
	CanSnapshot bool
	CanVMState  bool
	Warnings    []string
}

// ComputeSnapshotCapability derives the snapshot capability from the VM
// entity and the storage index. canVMState requires the VM to be running and
// every disk on storage that can hold RAM state; canSnapshot requires every
// disk on snapshot-capable storage. Warnings carry the reasons, in display
// order.
func ComputeSnapshotCapability(entity Entity, index *inventory.Index) SnapshotCapability {
	capability := SnapshotCapability{CanSnapshot: true, CanVMState: entity.Status == cluster.VMRunning}

	if !capability.CanVMState {
		capability.Warnings = append(capability.Warnings, "the VM must be running to capture RAM state")
	}

	for _, disk := range entity.Disks {
		canSnapshot, canVMState := diskStorageCapability(index, entity.Node, disk)

		if !canSnapshot {
			capability.CanSnapshot = false
			capability.Warnings = append(capability.Warnings, fmt.Sprintf("disk %s is on storage %s which does not support snapshots", disk.Key, disk.Storage))
		}

		if !canVMState {
			capability.CanVMState = false
			capability.Warnings = append(capability.Warnings, fmt.Sprintf("disk %s is on storage %s which cannot hold RAM state", disk.Key, disk.Storage))
		}
	}

	return capability
}

func diskStorageCapability(index *inventory.Index, node string, disk cluster.Disk) (canSnapshot, canVMState bool) {
	if index == nil {
		return false, false
	}

	for _, storage := range index.StoragesByNode[node] {
		if storage.Name == disk.Storage {
			return cluster.StorageSnapshotCapability(storage.PluginType, disk.Format)
		}
	}

	return false, false
}

func (deps SnapshotDependencies) readGabarit(ctx context.Context) (policy.Gabarit, error) {
	if deps.Policy != nil {
		return deps.Policy.Gabarit(ctx, deps.ClusterName)
	}

	if deps.Gabarit.MaxSnapshots == 0 {
		return policy.Gabarit{}, policy.ErrUnavailable
	}

	return deps.Gabarit, nil
}

func visibleSnapshots(snapshots []Snapshot) []Snapshot {
	visible := make([]Snapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.Name != currentSnapshotName {
			visible = append(visible, snapshot)
		}
	}

	return visible
}

//nolint:wsl_v5 // snapshot guards remain ordered within each domain operation
func countRealSnapshots(snapshots []Snapshot) int {
	count := 0
	for _, snapshot := range snapshots {
		if snapshot.Name != currentSnapshotName {
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
