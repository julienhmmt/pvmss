package vm

import (
	"context"
	"fmt"

	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
)

// EnableSerialDependencies contains the resolved VM write dependencies for the
// serial-console retrofit. Mirrors HardwareDependencies.
type EnableSerialDependencies struct {
	Index     *inventory.Index
	Actor     auth.Identity
	ClusterName string
	VMID      int
	Writer    cluster.Writer
	Audit     AuditRecorder
	Refresher IndexRefresher
}

// EnableSerialConsole provisions a socket-backed serial port (serial0) on an
// existing VM so the PVMSS Text/serial console becomes reachable for VMs
// created before serial0 was added at create time (commit 2d085e6c). It
// reuses Resolve() — the same ownership gate every write goes through (FR-001)
// — then writes through the cluster.Writer and refreshes the inventory so the
// VM's HasSerial flips without a poll cycle.
func EnableSerialConsole(ctx context.Context, deps EnableSerialDependencies) error {
	entity, err := Resolve(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
	if err != nil {
		return err
	}

	if deps.Writer == nil {
		return ErrNotFound
	}

	if err := deps.Writer.EnableSerial(ctx, entity.Node, entity.VMID); err != nil {
		return fmt.Errorf("enable serial: %w", err)
	}

	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, "serial_enable"); err != nil {
		return fmt.Errorf("record serial enable audit: %w", err)
	}

	if _, err := deps.Refresher.Refresh(ctx); err != nil {
		return fmt.Errorf("refresh inventory after serial enable: %w", err)
	}

	return nil
}
