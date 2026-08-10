package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
)

var (
	// ErrInvalidCDROMAction reports an unsupported CD-ROM operation.
	ErrInvalidCDROMAction = errors.New("invalid cdrom action")
	// ErrISOVolumeNotApproved reports an ISO absent from the catalog.
	ErrISOVolumeNotApproved = errors.New("iso volume is not approved")
)

// CDROMDependencies contains the resolved VM write dependencies for CD-ROM operations.
type CDROMDependencies struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Writer      cluster.Writer
	Resources   catalog.Resources
	Audit       AuditRecorder
	Refresher   IndexRefresher
}

// SetCDROM mounts, disconnects, or removes the VM's fixed CD-ROM drive.
func SetCDROM(ctx context.Context, deps CDROMDependencies, action, isoVolID string) (cluster.CDROMState, error) {
	entity, err := resolveCDROMTarget(deps)
	if err != nil {
		return cluster.CDROMState{}, err
	}

	state, auditAction, err := cdromState(deps.Resources, action, isoVolID)
	if err != nil {
		return cluster.CDROMState{}, err
	}

	if err := deps.Writer.SetCDROM(ctx, entity.Node, entity.VMID, state); err != nil {
		return cluster.CDROMState{}, fmt.Errorf("set cdrom: %w", err)
	}

	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, auditAction); err != nil {
		return cluster.CDROMState{}, fmt.Errorf("record cdrom audit: %w", err)
	}

	if _, err := deps.Refresher.Refresh(ctx); err != nil {
		return cluster.CDROMState{}, fmt.Errorf("refresh inventory after cdrom write: %w", err)
	}

	return state, nil
}

func resolveCDROMTarget(deps CDROMDependencies) (Entity, error) {
	if deps.Index == nil {
		return Entity{}, ErrNotFound
	}

	return Resolve(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
}

func cdromState(resources catalog.Resources, action, isoVolID string) (cluster.CDROMState, string, error) {
	switch action {
	case "mount":
		if !approvedISOVolume(resources.ISOs, isoVolID) {
			return cluster.CDROMState{}, "", fmt.Errorf("%w: %s", ErrISOVolumeNotApproved, isoVolID)
		}

		return cluster.CDROMState{State: cluster.CDROMMounted, ISOVolID: isoVolID}, "cdrom_mount", nil
	case "disconnect":
		return cluster.CDROMState{State: cluster.CDROMEmpty}, "cdrom_disconnect", nil
	case "remove":
		return cluster.CDROMState{State: cluster.CDROMAbsent}, "cdrom_remove", nil
	default:
		return cluster.CDROMState{}, "", fmt.Errorf("%w: %s", ErrInvalidCDROMAction, action)
	}
}

func approvedISOVolume(isos []catalog.ISO, volumeID string) bool {
	for _, iso := range isos {
		if isoVolumeID(iso) == volumeID {
			return true
		}
	}

	return false
}

func isoVolumeID(iso catalog.ISO) string {
	return fmt.Sprintf("%s:iso/%s", iso.Storage, iso.File)
}
