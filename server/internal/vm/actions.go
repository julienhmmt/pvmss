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

// Shutdown escalation constants (ticket 05).
const (
	shutdownEscalationWait = 60 * time.Second
	postEscalationStopWait = 30 * time.Second
)

var (
	// ShutdownPollInterval is the interval between live-status polls during
	// shutdown escalation. A var so tests can shorten it.
	ShutdownPollInterval = 2 * time.Second
	// MaxShutdownEscalationWait bounds the normal shutdown poll before
	// escalating to stop. A var so tests can shorten it.
	MaxShutdownEscalationWait = shutdownEscalationWait
	// MaxPostEscalationWait bounds the post-stop poll. A var so tests can
	// shorten it.
	MaxPostEscalationWait = postEscalationStopWait
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
// cause, structurally closed). After the write, it records the audit entry.
//
// Action does NOT refresh the Index — the caller owns refresh (ticket 09).
// handleAction refreshes once for a single-VM action; BulkAction refreshes
// once per distinct affected cluster. This avoids N redundant cluster
// snapshots during a bulk action.
//
// For shutdown, when deps.StatusReader is non-nil, Action implements bounded
// escalation (ticket 05): without Force, it sends shutdown, polls live status
// for up to 60s, and auto-escalates to stop if the guest hasn't stopped; with
// Force, it skips shutdown and sends stop directly. Both shutdown and stop are
// recorded as separate audit entries when escalation occurs (same pattern as
// Delete's force-stop).
func Action(ctx context.Context, deps BulkDeps, index *inventory.Index, clusterName string, vmid int, action string) error {
	if !validActions[action] {
		return fmt.Errorf("%w: %q", ErrActionRejected, action)
	}

	entity, err := Resolve(index, deps.Actor, clusterName, vmid)
	if err != nil {
		return err
	}

	// Shutdown with escalation (ticket 05). Only when a StatusReader is
	// available — legacy callers without one get the old immediate behavior.
	if action == "shutdown" && deps.StatusReader != nil {
		return shutdownWithEscalation(ctx, deps, entity, clusterName, vmid)
	}

	// Idempotence (ticket 08): start on a running VM and stop on a stopped
	// VM are successes, not errors. The user asked for the target state and
	// it already holds. Only applies to start/stop (target states), not to
	// transitions like reboot/reset/shutdown/pause/resume.
	if deps.StatusReader != nil && isIdempotentNoop(action) {
		live, readErr := deps.StatusReader.VMStatus(ctx, entity.Node, entity.VMID)
		if readErr == nil && isAlreadyInTargetState(action, live.Status) {
			// Record the audit entry — the intention is real even though
			// no Proxmox call was made.
			if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, clusterName, vmid, action); err != nil {
				return fmt.Errorf(auditWrapFmt, err)
			}

			return nil
		}
	}

	// Retry-on-lock (ticket 08): a VM locked by backup/migrate/snapshot/etc.
	// rejects actions with "VM is locked (lockname)". Retry with backoff
	// until the lock clears or the budget expires.
	if err := actionWithLockRetry(ctx, deps, entity, action); err != nil {
		return fmt.Errorf("cluster action: %w", err)
	}

	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, clusterName, vmid, action); err != nil {
		return fmt.Errorf(auditWrapFmt, err)
	}

	return nil
}

// isIdempotentNoop reports whether action is a target-state transition
// (start/stop) that can be a no-op when the target state already holds.
// reboot/reset/shutdown/pause/resume are transitions, not target states —
// they must always be sent.
func isIdempotentNoop(action string) bool {
	return action == "start" || action == "stop"
}

// isAlreadyInTargetState reports whether the live status already matches the
// target state of action. Only called for start/stop (isIdempotentNoop).
func isAlreadyInTargetState(action string, live cluster.VMStatus) bool {
	if action == "start" {
		return live == cluster.VMRunning
	}

	if action == "stop" {
		return live == cluster.VMStopped
	}

	return false
}

// Lock retry constants (ticket 08). Vars so tests can shorten them.
const lockRetryBudget = 30 * time.Second

var (
	// LockRetryPollInterval is the interval between retries when a VM is
	// locked. A var so tests can shorten it.
	LockRetryPollInterval = 2 * time.Second
	// MaxLockRetryWait bounds the total retry-on-lock wait. A var so tests
	// can shorten it.
	MaxLockRetryWait = lockRetryBudget
)

// actionWithLockRetry calls deps.Writer.Action and retries when the error
// indicates a VM lock (e.g. "VM is locked (backup)"). Returns a named-lock
// error when the budget expires.
func actionWithLockRetry(ctx context.Context, deps BulkDeps, entity Entity, action string) error {
	err := deps.Writer.Action(ctx, entity.Node, entity.VMID, action)
	if err == nil {
		return nil
	}

	lockName, locked := extractLockName(err)
	if !locked {
		return err
	}

	// Retry with backoff until the lock clears or the budget expires.
	deadline := time.NewTimer(MaxLockRetryWait)
	defer deadline.Stop()

	ticker := time.NewTicker(LockRetryPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err = deps.Writer.Action(ctx, entity.Node, entity.VMID, action)
			if err == nil {
				return nil
			}

			if _, stillLocked := extractLockName(err); !stillLocked {
				return err
			}
		case <-deadline.C:
			return fmt.Errorf("VM %d is locked by a %s; retry once it completes", entity.VMID, lockName)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// extractLockName checks whether err is a Proxmox "VM is locked" error and
// returns the lock name. Proxmox's message format is:
//
//	"VM is locked (backup)"
//	"VM is locked (snapshot-delete)"
//
// The lock name is returned without parentheses.
func extractLockName(err error) (string, bool) {
	if err == nil {
		return "", false
	}

	msg := err.Error()

	idx := strings.Index(msg, "VM is locked (")
	if idx < 0 {
		return "", false
	}

	start := idx + len("VM is locked (")

	end := strings.IndexByte(msg[start:], ')')
	if end < 0 {
		return "", false
	}

	return msg[start : start+end], true
}

// shutdownWithEscalation implements the bounded shutdown→stop escalation
// (ticket 05). Without Force: send shutdown, poll live status for up to
// maxShutdownEscalationWait, escalate to stop if still running, then poll
// for up to maxPostEscalationWait. With Force: skip shutdown, send stop
// directly. Both transitions are audited separately when escalation occurs.
func shutdownWithEscalation(ctx context.Context, deps BulkDeps, entity Entity, clusterName string, vmid int) error {
	if deps.Force {
		// Force: skip shutdown, go directly to stop.
		if err := deps.Writer.Action(ctx, entity.Node, entity.VMID, "stop"); err != nil {
			return fmt.Errorf("cluster stop: %w", err)
		}

		if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, clusterName, vmid, "stop"); err != nil {
			return fmt.Errorf(auditWrapFmt, err)
		}

		return nil
	}

	// Normal: send shutdown with timeout, then poll for stopped.
	if err := deps.Writer.Action(ctx, entity.Node, entity.VMID, "shutdown"); err != nil {
		return fmt.Errorf("cluster shutdown: %w", err)
	}

	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, clusterName, vmid, "shutdown"); err != nil {
		return fmt.Errorf(auditWrapFmt, err)
	}

	// Poll for stopped up to the escalation budget.
	stopped, err := pollForStopped(ctx, deps.StatusReader, entity.Node, vmid, MaxShutdownEscalationWait)
	if err != nil {
		// Status read failure during polling — the shutdown was already sent.
		// Don't fail the whole action (best-effort): returning nil here despite
		// a non-nil err is intentional, not a missed error wrap.
		return nil //nolint:nilerr // best-effort: shutdown already dispatched
	}

	if stopped {
		return nil
	}

	// Escalation: guest didn't stop within the budget. Send stop.
	if err := deps.Writer.Action(ctx, entity.Node, entity.VMID, "stop"); err != nil {
		return fmt.Errorf("cluster stop (shutdown escalation): %w", err)
	}

	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, clusterName, vmid, "stop"); err != nil {
		return fmt.Errorf(auditWrapFmt, err)
	}

	// Poll for stopped up to the post-escalation budget.
	_, _ = pollForStopped(ctx, deps.StatusReader, entity.Node, vmid, MaxPostEscalationWait)

	return nil
}

// pollForStopped polls the live status reader until the VM reports stopped or
// the budget expires. Returns true if the VM reached stopped, false if the
// budget expired. A context cancellation returns the context error.
func pollForStopped(ctx context.Context, reader cluster.VMStatusReader, node string, vmid int, budget time.Duration) (bool, error) {
	deadline := time.NewTimer(budget)
	defer deadline.Stop()

	ticker := time.NewTicker(ShutdownPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			live, err := reader.VMStatus(ctx, node, vmid)
			if err != nil {
				// Transient read error: keep polling.
				continue
			}

			if live.Status == cluster.VMStopped {
				return true, nil
			}
		case <-deadline.C:
			return false, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
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
