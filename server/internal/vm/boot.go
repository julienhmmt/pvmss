package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"slices"
	"time"
)

// ErrCDROMNotMounted is returned when booting from a CD-ROM that has no
// ISO mounted.
var ErrCDROMNotMounted = errors.New("no iso mounted in cdrom drive")

// Boot poll constants. Vars so tests can shorten them.
var (
	// BootPollInterval is the interval between live-status polls while waiting
	// for the guest to start. A var so tests can shorten it.
	BootPollInterval = 500 * time.Millisecond
	// MaxBootStartWait bounds the wait for the guest to reach running after
	// the start call. A var so tests can shorten it.
	MaxBootStartWait = 60 * time.Second
)

// cdromBootKey is the boot-order key of the VM's fixed CD-ROM drive
// (cluster.cdromDiskKey — duplicated here because that constant is unexported).
const cdromBootKey = "ide2"

// BootDependencies contains the resolved VM write dependencies for the
// boot-from-CDROM flow.
type BootDependencies struct {
	Index        *inventory.Index
	Actor        auth.Identity
	ClusterName  string
	VMID         int
	Writer       cluster.Writer
	Audit        AuditRecorder
	Refresher    IndexRefresher
	StatusReader cluster.VMStatusReader
}

// BootFromCDROM performs a one-time boot of a stopped VM from its mounted
// CD-ROM: set the boot order CD-first, start the guest, then restore the
// original boot order once the guest is running.
//
// Proxmox has no one-time-boot API — boot order is a persistent config key
// that qemu reads at start. Restoring the original order only after the guest
// reports running is therefore race-free: the in-flight boot already consumed
// the CD-first order, and the next reboot boots from disk again. The restore
// is best-effort: a failed restore leaves CD-first order, which the next
// boot-cdrom call or an admin can fix.
func BootFromCDROM(ctx context.Context, deps BootDependencies) error {
	entity, err := Resolve(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
	if err != nil {
		return err
	}

	if entity.CDROM.State != cluster.CDROMMounted || entity.CDROM.ISOVolID == "" {
		return fmt.Errorf("%w", ErrCDROMNotMounted)
	}

	if entity.Status != cluster.VMStopped {
		return fmt.Errorf("%w", cluster.ErrVMRunning)
	}

	original := entity.BootOrder

	if err := deps.Writer.SetBootOrder(ctx, entity.Node, entity.VMID, BootOrderCDFirst(entity)); err != nil {
		return fmt.Errorf("set boot order: %w", err)
	}

	// Restore the original boot order on every path: once the guest is up the
	// boot config was already consumed, and on start failure the CD-first
	// order must not linger.
	defer func() { _ = deps.Writer.SetBootOrder(ctx, entity.Node, entity.VMID, original) }()

	if err := deps.Writer.Action(ctx, entity.Node, entity.VMID, "start"); err != nil {
		return fmt.Errorf("cluster start: %w", err)
	}

	// Wait until the guest is up so the deferred restore cannot race the boot
	// config read. Best-effort: a slow guest still boots from the CD, the
	// restore just lands later than ideal.
	if deps.StatusReader != nil {
		pollForStatus(ctx, deps.StatusReader, entity.Node, entity.VMID, cluster.VMRunning, MaxBootStartWait)
	}

	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, "boot_cdrom"); err != nil {
		return fmt.Errorf(auditWrapFmt, err)
	}

	if _, err := deps.Refresher.Refresh(ctx); err != nil {
		return fmt.Errorf("refresh inventory after boot_cdrom: %w", err)
	}

	return nil
}

// BootOrderCDFirst builds the boot order with the CD-ROM (ide2) first, keeping
// the VM's remaining boot devices in their existing relative order. A VM with
// no recorded boot order falls back to its first disk.
func BootOrderCDFirst(entity Entity) []string {
	order := []string{cdromBootKey}
	for _, key := range entity.BootOrder {
		if key != cdromBootKey && !slices.Contains(order, key) {
			order = append(order, key)
		}
	}

	if len(order) == 1 {
		for _, disk := range entity.Disks {
			if !slices.Contains(order, disk.Key) {
				order = append(order, disk.Key)

				break
			}
		}
	}

	return order
}

// pollForStatus polls the live status reader until the VM reports target or
// the budget expires. Returns true if the target state was reached.
func pollForStatus(ctx context.Context, reader cluster.VMStatusReader, node string, vmid int, target cluster.VMStatus, budget time.Duration) bool {
	deadline := time.NewTimer(budget)
	defer deadline.Stop()

	ticker := time.NewTicker(BootPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			live, err := reader.VMStatus(ctx, node, vmid)
			if err != nil {
				// Transient read error: keep polling.
				continue
			}

			if live.Status == target {
				return true
			}
		case <-deadline.C:
			return false
		case <-ctx.Done():
			return false
		}
	}
}
