package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
)

var (
	// ErrHardwareExceedsLimit reports a hardware bound violation.
	ErrHardwareExceedsLimit = errors.New("hardware exceeds limit")
	// ErrEmptyHardwarePatch reports a patch with no fields.
	ErrEmptyHardwarePatch = errors.New("empty hardware patch")
)

// HardwarePatch contains the optional hardware and tag fields accepted by a VM patch.
type HardwarePatch struct {
	Sockets  *int
	Cores    *int
	MemoryMB *int
	Tags     *[]string
}

// HardwareDependencies contains the resolved VM write dependencies for hardware updates.
type HardwareDependencies struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Writer      cluster.Writer
	Policy      *policy.Policy
	Gabarit     policy.Gabarit
	Audit       AuditRecorder
	Refresher   IndexRefresher
}

// UpdateHardware applies a hardware patch, restarting a running VM only when
// sockets, cores, or memory changes.
func UpdateHardware(ctx context.Context, deps HardwareDependencies, patch HardwarePatch) error {
	entity, err := resolveHardwareTarget(deps)
	if err != nil {
		return err
	}

	if patch.Sockets == nil && patch.Cores == nil && patch.MemoryMB == nil && patch.Tags == nil {
		return ErrEmptyHardwarePatch
	}

	gabarit := deps.Gabarit
	if deps.Policy != nil {
		gabarit, err = deps.Policy.Gabarit(ctx, deps.ClusterName)
		if err != nil {
			return fmt.Errorf("read gabarit: %w", err)
		}
	}

	if deps.Policy == nil && gabarit.MaxSockets == 0 {
		return policy.ErrUnavailable
	}

	sockets, cores, memoryMB, tags, err := effectiveHardware(entity, patch, gabarit)
	if err != nil {
		return err
	}

	if deps.Policy != nil {
		if err := deps.Policy.CheckNodeCapacity(ctx, deps.ClusterName, entity.Node, sockets, cores, memoryMB, entity.VMID); err != nil {
			return err
		}
	}

	needsRestart := entity.Status == cluster.VMRunning && hardwareChanged(entity, sockets, cores, memoryMB)
	if err := applyHardware(ctx, deps, entity, sockets, cores, memoryMB, tags, needsRestart); err != nil {
		return err
	}

	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, "hardware_update"); err != nil {
		return fmt.Errorf("record hardware audit: %w", err)
	}

	if _, err := deps.Refresher.Refresh(ctx); err != nil {
		return fmt.Errorf("refresh inventory after hardware write: %w", err)
	}

	return nil
}

func applyHardware(ctx context.Context, deps HardwareDependencies, entity Entity, sockets, cores, memoryMB int, tags []string, needsRestart bool) error {
	if needsRestart {
		if err := deps.Writer.Action(ctx, entity.Node, entity.VMID, "stop"); err != nil {
			return fmt.Errorf("stop vm for hardware update: %w", err)
		}
	}

	if err := deps.Writer.UpdateHardware(ctx, entity.Node, entity.VMID, sockets, cores, memoryMB, tags); err != nil {
		return fmt.Errorf("update hardware: %w", err)
	}

	if !needsRestart {
		return nil
	}

	if err := deps.Writer.Action(ctx, entity.Node, entity.VMID, "start"); err != nil {
		return fmt.Errorf("restart vm after hardware update: %w", err)
	}

	return nil
}

func resolveHardwareTarget(deps HardwareDependencies) (Entity, error) {
	if deps.Index == nil {
		return Entity{}, ErrNotFound
	}

	return Resolve(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
}

func effectiveHardware(entity Entity, patch HardwarePatch, gabarit policy.Gabarit) (int, int, int, []string, error) {
	sockets, cores, memoryMB := entity.Sockets, entity.Cores, int(entity.MemoryTotal/(1024*1024))
	if patch.Sockets != nil {
		sockets = *patch.Sockets
	}

	if patch.Cores != nil {
		cores = *patch.Cores
	}

	if patch.MemoryMB != nil {
		memoryMB = *patch.MemoryMB
	}

	if sockets < 1 || sockets > gabarit.MaxSockets {
		return 0, 0, 0, nil, fmt.Errorf("%w: sockets exceeds maxSockets", ErrHardwareExceedsLimit)
	}

	if cores < 1 || cores > gabarit.MaxCores {
		return 0, 0, 0, nil, fmt.Errorf("%w: cores exceeds maxCores", ErrHardwareExceedsLimit)
	}

	if memoryMB < 1 || memoryMB > gabarit.MaxMemoryMB {
		return 0, 0, 0, nil, fmt.Errorf("%w: memory exceeds maxMemoryMB", ErrHardwareExceedsLimit)
	}

	tags := append([]string(nil), entity.Tags...)
	if patch.Tags != nil {
		tags = append([]string(nil), (*patch.Tags)...)
	}

	return sockets, cores, memoryMB, tags, nil
}

func hardwareChanged(entity Entity, sockets, cores, memoryMB int) bool {
	return entity.Sockets != sockets || entity.Cores != cores || entity.MemoryTotal != int64(memoryMB)*1024*1024
}
