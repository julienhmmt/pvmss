package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"slices"
	"time"
)

var (
	// ErrHardwareExceedsLimit reports a hardware bound violation.
	ErrHardwareExceedsLimit = errors.New("hardware exceeds limit")
	// ErrEmptyHardwarePatch reports a patch with no fields.
	ErrEmptyHardwarePatch = errors.New("empty hardware patch")
	// ErrShutdownTimeout means the guest did not shut down in time for a
	// hardware change; nothing was changed and the VM keeps running.
	ErrShutdownTimeout = errors.New("vm did not shut down in time")
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
	// AllowedTags is the admin-curated tag allowlist for this cluster.
	// A tag patch referencing a name outside it is rejected.
	AllowedTags []string
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

	// Users may only assign admin-curated tags. The mandatory pvmss
	// tag is re-added below, so it never needs to be in the patch.
	if err := validatePatchTags(patch, deps.AllowedTags); err != nil {
		return err
	}

	gabarit, err := resolveGabarit(ctx, deps.Policy, deps.Gabarit, deps.ClusterName, func(g policy.Gabarit) bool { return g.MaxSockets > 0 })
	if err != nil {
		return err
	}

	sockets, cores, memoryMB, tags, err := effectiveHardware(entity, patch, gabarit)
	if err != nil {
		return err
	}

	if deps.Policy != nil {
		if err := deps.Policy.CheckNodeCapacity(ctx, deps.ClusterName, entity.Node, policy.CapacityDelta{Sockets: sockets, Cores: cores, MemoryMB: memoryMB, ExcludeVMID: entity.VMID}); err != nil {
			return err
		}
	}

	needsRestart := entity.Status == cluster.VMRunning && hardwareChanged(entity, sockets, cores, memoryMB)
	if err := applyHardware(ctx, deps, entity, hardwareApply{
		Sockets: sockets, Cores: cores, MemoryMB: memoryMB, Tags: tags, NeedsRestart: needsRestart,
	}); err != nil {
		return err
	}

	return finalizeHardwareWrite(ctx, deps)
}

// finalizeHardwareWrite records the audit entry and refreshes the inventory
// projection after a successful hardware mutation.
func finalizeHardwareWrite(ctx context.Context, deps HardwareDependencies) error {
	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, "hardware_update"); err != nil {
		return fmt.Errorf("record hardware audit: %w", err)
	}

	if _, err := deps.Refresher.Refresh(ctx); err != nil {
		return fmt.Errorf("refresh inventory after hardware write: %w", err)
	}

	return nil
}

// hardwareApply groups the concrete hardware values and restart flag passed to
// applyHardware. It collapses the five positional parameters that helper used
// to take (SonarQube go:S107).
type hardwareApply struct {
	Sockets      int
	Cores        int
	MemoryMB     int
	Tags         []string
	NeedsRestart bool
}

// Shutdown wait for a hardware change. Proxmox gives the guest 60 s
// (cluster.shutdownTimeout); the margin covers the task's own overhead.
// Vars so tests can shorten them.
var (
	stopPollInterval = time.Second
	stopWaitTimeout  = 75 * time.Second
)

func applyHardware(ctx context.Context, deps HardwareDependencies, entity Entity, apply hardwareApply) error {
	if apply.NeedsRestart {
		if err := shutdownAndWait(ctx, deps.Writer, entity); err != nil {
			return err
		}
	}

	if err := deps.Writer.UpdateHardware(ctx, entity.Node, entity.VMID, apply.Sockets, apply.Cores, apply.MemoryMB, apply.Tags); err != nil {
		return fmt.Errorf("update hardware: %w", err)
	}

	if !apply.NeedsRestart {
		return nil
	}

	if err := deps.Writer.Action(ctx, entity.Node, entity.VMID, "start"); err != nil {
		return fmt.Errorf("restart vm after hardware update: %w", err)
	}

	return nil
}

// shutdownAndWait asks the guest to shut down (never a power cut) and waits
// until Proxmox reports it stopped. Config and start must not race the stop.
func shutdownAndWait(ctx context.Context, writer cluster.Writer, entity Entity) error {
	reader, ok := writer.(cluster.VMStatusReader)
	if !ok {
		return errors.New("cluster client cannot read vm status")
	}

	if err := writer.Action(ctx, entity.Node, entity.VMID, "shutdown"); err != nil {
		return fmt.Errorf("shut down vm for hardware update: %w", err)
	}

	deadline := time.Now().Add(stopWaitTimeout)
	for time.Now().Before(deadline) {
		status, err := reader.VMStatus(ctx, entity.Node, entity.VMID)
		if err == nil && status.Status == cluster.VMStopped {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(stopPollInterval):
		}
	}

	return ErrShutdownTimeout
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

	if err := checkHardwareLimits(sockets, cores, memoryMB, gabarit); err != nil {
		return 0, 0, 0, nil, err
	}

	tags := patchedTags(entity.Tags, patch.Tags)

	return sockets, cores, memoryMB, tags, nil
}

// checkHardwareLimits rejects a hardware shape outside the gabarit.
func checkHardwareLimits(sockets, cores, memoryMB int, gabarit policy.Gabarit) error {
	// A zero gabarit field means no cap (same rule as CheckGabarit and the
	// node capacités); a negative one stays a deny-all ceiling.
	if sockets < 1 || (gabarit.MaxSockets != 0 && sockets > gabarit.MaxSockets) {
		return fmt.Errorf("%w: sockets exceeds maxSockets", ErrHardwareExceedsLimit)
	}

	if cores < 1 || (gabarit.MaxCores != 0 && cores > gabarit.MaxCores) {
		return fmt.Errorf("%w: cores exceeds maxCores", ErrHardwareExceedsLimit)
	}

	if memoryMB < 1 || (gabarit.MaxMemoryMB != 0 && memoryMB > gabarit.MaxMemoryMB) {
		return fmt.Errorf("%w: memory exceeds maxMemoryMB", ErrHardwareExceedsLimit)
	}

	return nil
}

// patchedTags returns the entity tags with the patch applied. pvmss is
// mandatory: a VM without it is invisible to every PVMSS endpoint, so a user
// tag patch can never strip it.
func patchedTags(current []string, patch *[]string) []string {
	tags := append([]string(nil), current...)
	if patch == nil {
		return tags
	}

	tags = append([]string(nil), (*patch)...)
	if !slices.Contains(tags, pvmssTag) {
		tags = append(tags, pvmssTag)
	}

	return tags
}

// validatePatchTags rejects tag patches referencing names outside the
// admin-curated catalog. A patch without tags is always valid.
func validatePatchTags(patch HardwarePatch, allowed []string) error {
	if patch.Tags == nil {
		return nil
	}

	return validateTags(*patch.Tags, allowed)
}

// validateTags rejects tags outside the admin-curated catalog.
func validateTags(tags, allowed []string) error {
	for _, tag := range tags {
		if !slices.Contains(allowed, tag) {
			return fmt.Errorf("%w: tag %q", ErrNotApproved, tag)
		}
	}

	return nil
}

func hardwareChanged(entity Entity, sockets, cores, memoryMB int) bool {
	return entity.Sockets != sockets || entity.Cores != cores || entity.MemoryTotal != int64(memoryMB)*1024*1024
}
