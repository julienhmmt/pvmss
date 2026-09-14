package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
)

// Sentinel errors for the SeaBIOS retrofit.
var (
	// ErrRetrofitRefused is returned when the VM's boot depends on UEFI
	// (TPM state or Secure Boot present) and cannot be safely switched.
	ErrRetrofitRefused = errors.New("retrofit refused: VM boot depends on UEFI")
	// ErrRetrofitRequiresConfirmation is returned when the VM is running
	// and the caller did not confirm the stop/start cycle.
	ErrRetrofitRequiresConfirmation = errors.New("retrofit requires confirmation: VM is running")
	// ErrRetrofitRestartFailed is returned when the VM was switched to
	// SeaBIOS but failed to come back up — the operator must intervene.
	ErrRetrofitRestartFailed = errors.New("retrofit succeeded but the VM failed to restart")
)

// RetrofitDependencies contains the resolved VM write dependencies for the
// SeaBIOS retrofit. Mirrors EnableSerialDependencies.
type RetrofitDependencies struct {
	Index        *inventory.Index
	Actor        auth.Identity
	ClusterName  string
	VMID         int
	Writer       cluster.Writer
	StatusReader cluster.VMStatusReader
	Audit        AuditRecorder
	Refresher    IndexRefresher
	// Confirm, when true, authorizes the stop/start cycle for a running VM.
	// A running VM without Confirm returns ErrRetrofitRequiresConfirmation.
	Confirm bool
}

// RetrofitToSeaBIOS switches an existing UEFI VM to SeaBIOS so its graphical
// console becomes readable. The flow:
//
// 1. Resolve the VM (ownership gate).
// 2. Read the live firmware config; refuse TPM state or Secure Boot.
// 3. If running, require Confirm; stop the VM.
// 4. Delete bios/machine/efidisk0/tpmstate0.
// 5. Start the VM again. A failed restart returns ErrRetrofitRestartFailed
// so the operator gets a clear, actionable error rather than a silent
// partial state.
// 6. Audit + refresh.
//
// cloud-init's packages and runcmd are once-per-instance, so the baseline
// cannot be retrofitted — only the firmware. This is the only retrofittable
// half of the effort.
func RetrofitToSeaBIOS(ctx context.Context, deps RetrofitDependencies) error {
	entity, err := Resolve(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
	if err != nil {
		return err
	}

	if deps.Writer == nil {
		return ErrNotFound
	}

	// Read the live firmware config before changing anything. The projection
	// does not hydrate BIOS/EFIDisk/TPMState/SecureBoot from Proxmox, so the
	// retrofit reads them live.
	fw, err := deps.Writer.ReadFirmwareConfig(ctx, entity.Node, entity.VMID)
	if err != nil {
		return fmt.Errorf("read firmware config: %w", err)
	}

	// Refuse VMs whose boot depends on UEFI before changing anything.
	if fw.HasTPM {
		return fmt.Errorf("%w: TPM state present", ErrRetrofitRefused)
	}

	if fw.SecureBoot {
		return fmt.Errorf("%w: Secure Boot enabled", ErrRetrofitRefused)
	}

	// If the VM is already on SeaBIOS, there is nothing to do.
	if fw.BIOS != "ovmf" && !fw.HasEFIDisk {
		return nil
	}

	// A running VM must be stopped before the config change. Require explicit
	// confirmation so the operator knows a stop/start cycle is coming.
	wasRunning := entity.Status == cluster.VMRunning
	if wasRunning && !deps.Confirm {
		return ErrRetrofitRequiresConfirmation
	}

	if err := applyRetrofit(ctx, deps, entity, wasRunning); err != nil {
		return err
	}

	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, "retrofit_seabios"); err != nil {
		return fmt.Errorf("record retrofit audit: %w", err)
	}

	if _, err := deps.Refresher.Refresh(ctx); err != nil {
		return fmt.Errorf("refresh inventory after retrofit: %w", err)
	}

	return nil
}

// applyRetrofit performs the stop → firmware switch → start sequence. The
// stop/start pair only runs for a VM that was running; a stopped VM gets the
// firmware change alone.
func applyRetrofit(ctx context.Context, deps RetrofitDependencies, entity Entity, wasRunning bool) error {
	if wasRunning {
		if err := deps.Writer.Action(ctx, entity.Node, entity.VMID, "stop"); err != nil {
			return fmt.Errorf("stop VM before retrofit: %w", err)
		}
	}

	if err := deps.Writer.RetrofitToSeaBIOS(ctx, entity.Node, entity.VMID); err != nil {
		return fmt.Errorf("retrofit firmware: %w", err)
	}

	if wasRunning {
		if err := deps.Writer.Action(ctx, entity.Node, entity.VMID, "start"); err != nil {
			return fmt.Errorf("%w: %w", ErrRetrofitRestartFailed, err)
		}
	}

	return nil
}
