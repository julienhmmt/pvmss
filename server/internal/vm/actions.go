package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"regexp"
	"strings"
	"time"
)

// auditWrapFmt is the error wrapper used when an audit record write fails.
// Defined once to avoid repeating the literal in every write path (go:S1192).
const auditWrapFmt = "record audit: %w"

// forceStopPoll is the interval between delete retries while waiting for a
// force-stop to take effect. maxForceStopWait bounds the total wait — a var so
// tests can shorten it. Mirrors the pool cascade's poll-then-timeout shape.
const forceStopPoll = 100 * time.Millisecond

var maxForceStopWait = 15 * time.Second

// AuditRecorder is the store dependency for recording a write. Only the method
// T05 needs is on the interface, so the handler test can use the real store
// and production can use *store.Store.
type AuditRecorder interface {
	RecordAction(ctx context.Context, actor, clusterName string, vmid int, action string) error
}

// IndexRefresher rebuilds the Index after a write so the next read reflects it
// (FR-010). *inventory.Worker satisfies this.
type IndexRefresher interface {
	Refresh(ctx context.Context) (time.Time, error)
}

// validActions is the exhaustive set of accepted power actions (FR-006).
var validActions = map[string]bool{
	"start":    true,
	"stop":     true,
	"shutdown": true,
	"reboot":   true,
	"reset":    true,
	"pause":    true,
	"resume":   true,
}

// hostnameRe validates a VM name as a hostname: alphanumeric and hyphen, no
// leading/trailing hyphen, ≤ 63 chars (legacy validation.go rule, reused per
// spec FR-008). Lowercase only — a Proxmox VM name becomes a DNS label when
// cloud-init sets the hostname.
var hostnameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

const maxDescriptionLength = 512

// MaxDescriptionLength is exported so the handler can reference it in error
// messages without duplicating the constant.
const MaxDescriptionLength = maxDescriptionLength

// IsValidAction reports whether action is one of the five accepted power
// transitions (FR-006). Exported so the handler can reject malformed input
// before calling Resolve (constitution XIII: malformed input rejected first).
func IsValidAction(action string) bool {
	return validActions[action]
}

// ErrInvalidName is returned when a PATCH name fails the hostname validation.
var ErrInvalidName = errors.New("invalid name")

// ErrActionRejected is returned when an action string is not one of the five
// accepted values. The handler validates before calling Resolve, so this is a
// defence-in-depth guard at the domain layer too.
var ErrActionRejected = errors.New("action rejected")

// ErrEmptyPatch is returned when a PATCH body has neither name nor description.
var ErrEmptyPatch = errors.New("empty patch")

// ErrDescriptionTooLong is returned when a description exceeds the max length.
var ErrDescriptionTooLong = errors.New("description too long")

// ValidateName checks a VM name against the hostname rule (FR-008). Exported
// so the handler can validate before calling Resolve (rejecting malformed input
// before any authorization check — constitution XIII: malformed input is
// rejected first).
func ValidateName(name string) error {
	if !hostnameRe.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidName, name)
	}

	return nil
}

// WriteDeps groups the shared dependencies and resolution context for a
// single-VM write (Delete/Patch). It collapses the seven positional parameters
// these functions used to take (SonarQube go:S107).
type WriteDeps struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Writer      cluster.Writer
	Audit       AuditRecorder
	Refresher   IndexRefresher
	// Force authorizes Delete to stop a running VM before destroying it. When
	// false (the default), Delete returns cluster.ErrVMRunning if the VM is
	// running, matching what real Proxmox rejects natively. The HTTP handler
	// only sets this after the user has confirmed the force-stop in the UI.
	Force bool
}

// Action performs a power transition on a VM. It is the only path from an
// HTTP action request to the cluster writer (FR-006). The node is always
// Resolve()'s server-resolved value — the caller cannot supply one (S01 root
// cause, structurally closed). After the write, it records the audit entry
// and refreshes the Index (FR-009, FR-010).
func Action(ctx context.Context, deps BulkDeps, index *inventory.Index, clusterName string, vmid int, action string) error {
	if !validActions[action] {
		return fmt.Errorf("%w: %q", ErrActionRejected, action)
	}

	entity, err := Resolve(index, deps.Actor, clusterName, vmid)
	if err != nil {
		return err
	}

	if err := deps.Writer.Action(ctx, entity.Node, entity.VMID, action); err != nil {
		return fmt.Errorf("cluster action: %w", err)
	}

	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, clusterName, vmid, action); err != nil {
		return fmt.Errorf(auditWrapFmt, err)
	}

	_, _ = deps.Refresher.Refresh(ctx)

	return nil
}

// Delete permanently removes a VM and its disks (V14: no soft-delete, no undo).
// Same Resolve() gate as Action — not a parallel ownership check (FR-007).
//
// A running VM is rejected by the cluster writer with cluster.ErrVMRunning
// (real Proxmox returns HTTP 500 "VM X is running - destroy failed"; the fake
// mirrors it). When deps.Force is set, Delete force-stops the VM first and
// retries the destroy until the stop takes effect (bounded by maxForceStopWait);
// the stop is recorded as its own "stop" audit entry so the action trail shows
// the force-stop that preceded the destroy. The HTTP handler only sets Force
// after the user has confirmed the force-stop in the UI.
func Delete(ctx context.Context, deps WriteDeps) error {
	entity, err := Resolve(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
	if err != nil {
		return err
	}

	if err := deps.Writer.Delete(ctx, entity.Node, entity.VMID); err != nil {
		// Any non-"running" error, or a "running" error without Force, propagates.
		if !errors.Is(err, cluster.ErrVMRunning) || !deps.Force {
			return fmt.Errorf("cluster delete: %w", err)
		}

		if err := forceStop(ctx, deps, entity); err != nil {
			return err
		}

		if err := deleteWithRetry(ctx, deps, entity); err != nil {
			return fmt.Errorf("cluster delete: %w", err)
		}
	}

	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, "delete"); err != nil {
		return fmt.Errorf(auditWrapFmt, err)
	}

	_, _ = deps.Refresher.Refresh(ctx)

	return nil
}

// forceStop stops a running VM so Delete can proceed, and records the stop as a
// separate audit entry. The node/vmid come from the already-resolved entity —
// the caller cannot supply them (S01 root cause, structurally closed).
func forceStop(ctx context.Context, deps WriteDeps, entity Entity) error {
	if err := deps.Writer.Action(ctx, entity.Node, entity.VMID, "stop"); err != nil {
		return fmt.Errorf("cluster stop: %w", err)
	}

	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, "stop"); err != nil {
		return fmt.Errorf(auditWrapFmt, err)
	}

	return nil
}

// deleteWithRetry retries the destroy while the cluster still reports the VM as
// running, polling until the force-stop takes effect or maxForceStopWait
// elapses. A non-"running" error is returned immediately; only ErrVMRunning is
// retried. The Refresher is ticked between attempts so the projection catches
// up for subsequent reads.
func deleteWithRetry(ctx context.Context, deps WriteDeps, entity Entity) error {
	err := deps.Writer.Delete(ctx, entity.Node, entity.VMID)
	if err == nil || !errors.Is(err, cluster.ErrVMRunning) {
		return err
	}

	deadline := time.NewTimer(maxForceStopWait)
	defer deadline.Stop()

	ticker := time.NewTicker(forceStopPoll)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, _ = deps.Refresher.Refresh(ctx)

			err = deps.Writer.Delete(ctx, entity.Node, entity.VMID)
			if err == nil || !errors.Is(err, cluster.ErrVMRunning) {
				return err
			}
		case <-deadline.C:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Patch updates a VM's name and/or description. At least one field must be
// non-empty; name is validated as a hostname (FR-008). The audit action is
// "rename" when name changes, "edit_description" when only description changes.
// The handler re-resolves from the refreshed projection to return the updated
// Entity — Patch itself does not return it, keeping the domain layer free of
// projection references.
func Patch(ctx context.Context, deps WriteDeps, name, description string) error {
	if name == "" && strings.TrimSpace(description) == "" {
		return ErrEmptyPatch
	}

	if name != "" {
		if err := ValidateName(name); err != nil {
			return err
		}
	}

	if len(description) > maxDescriptionLength {
		return ErrDescriptionTooLong
	}

	entity, err := Resolve(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
	if err != nil {
		return err
	}

	if err := deps.Writer.Patch(ctx, entity.Node, entity.VMID, name, description); err != nil {
		return fmt.Errorf("cluster patch: %w", err)
	}

	auditAction := "edit_description"
	if name != "" {
		auditAction = "rename"
	}

	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, auditAction); err != nil {
		return fmt.Errorf(auditWrapFmt, err)
	}

	_, _ = deps.Refresher.Refresh(ctx)

	return nil
}
