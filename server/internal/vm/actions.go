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

// Action performs a power transition on a VM. It is the only path from an
// HTTP action request to the cluster writer (FR-006). The node is always
// Resolve()'s server-resolved value — the caller cannot supply one (S01 root
// cause, structurally closed). After the write, it records the audit entry
// and refreshes the Index (FR-009, FR-010).
func Action(ctx context.Context, index *inventory.Index, actor auth.Identity, clusterName string, vmid int, action string, writer cluster.Writer, audit AuditRecorder, refresher IndexRefresher) error {
	if !validActions[action] {
		return fmt.Errorf("%w: %q", ErrActionRejected, action)
	}
	entity, err := Resolve(index, actor, clusterName, vmid)
	if err != nil {
		return err
	}
	if err := writer.Action(ctx, entity.Node, entity.VMID, action); err != nil {
		return fmt.Errorf("cluster action: %w", err)
	}
	if err := audit.RecordAction(ctx, actor.Username, clusterName, vmid, action); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	_, _ = refresher.Refresh(ctx)
	return nil
}

// Delete permanently removes a VM and its disks (V14: no soft-delete, no undo).
// Same Resolve() gate as Action — not a parallel ownership check (FR-007).
func Delete(ctx context.Context, index *inventory.Index, actor auth.Identity, clusterName string, vmid int, writer cluster.Writer, audit AuditRecorder, refresher IndexRefresher) error {
	entity, err := Resolve(index, actor, clusterName, vmid)
	if err != nil {
		return err
	}
	if err := writer.Delete(ctx, entity.Node, entity.VMID); err != nil {
		return fmt.Errorf("cluster delete: %w", err)
	}
	if err := audit.RecordAction(ctx, actor.Username, clusterName, vmid, "delete"); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	_, _ = refresher.Refresh(ctx)
	return nil
}

// Patch updates a VM's name and/or description. At least one field must be
// non-empty; name is validated as a hostname (FR-008). The audit action is
// "rename" when name changes, "edit_description" when only description changes.
// The handler re-resolves from the refreshed projection to return the updated
// Entity — Patch itself does not return it, keeping the domain layer free of
// projection references.
func Patch(ctx context.Context, index *inventory.Index, actor auth.Identity, clusterName string, vmid int, name, description string, writer cluster.Writer, audit AuditRecorder, refresher IndexRefresher) error {
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
	entity, err := Resolve(index, actor, clusterName, vmid)
	if err != nil {
		return err
	}
	if err := writer.Patch(ctx, entity.Node, entity.VMID, name, description); err != nil {
		return fmt.Errorf("cluster patch: %w", err)
	}
	auditAction := "edit_description"
	if name != "" {
		auditAction = "rename"
	}
	if err := audit.RecordAction(ctx, actor.Username, clusterName, vmid, auditAction); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	_, _ = refresher.Refresh(ctx)
	return nil
}
