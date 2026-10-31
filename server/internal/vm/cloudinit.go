package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"time"
)

var (
	// ErrNoCloudInitUser reports a password request on a VM whose patch and
	// live config define no ciuser. The password is refused, never applied to
	// a guessed account: a cloud image's root is locked, so a fallback to
	// root would silently write the password where nobody can log in.
	ErrNoCloudInitUser = errors.New("no cloud-init user defined")
	// ErrGuestAgentDisabled reports agent= absent from the VM config - the
	// QEMU guest agent cannot answer, so the password cannot be applied
	// (immediate, actionable pre-flight refusal).
	ErrGuestAgentDisabled = errors.New("guest agent not enabled")
	// ErrVMNotRunning reports a password attempt on a VM that is not running.
	ErrVMNotRunning = errors.New("vm not running")
	// ErrGuestAgentUnreachable reports that the guest agent never answered
	// within the bounded wait, so the password was not applied.
	ErrGuestAgentUnreachable = errors.New("guest agent unreachable")
)

// GetCloudInitConfig reads live structured state after the shared ownership gate.
func GetCloudInitConfig(ctx context.Context, index *inventory.Index, actor auth.Identity, clusterName string, vmid int, reader cluster.CloudInitReader) (cluster.CloudInitConfig, error) {
	entity, err := resolveCloudInitTarget(index, actor, clusterName, vmid)
	if err != nil {
		return cluster.CloudInitConfig{}, err
	}

	config, err := reader.GetCloudInitConfig(ctx, entity.Node, entity.VMID)
	if err != nil {
		return cluster.CloudInitConfig{}, err
	}

	return config, nil
}

// CloudInitConfigDeps groups the shared dependencies for the cloud-init
// password flow used by SetConsolePassword.
type CloudInitConfigDeps struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Reader      cluster.CloudInitReader
	Writer      cluster.Writer
	Audit       AuditRecorder
	Refresher   IndexRefresher
	// StatusReader reads the VM's live power state for the password
	// pre-flight. Optional: when nil the running check is
	// skipped and the bounded agent ping is the only readiness gate.
	StatusReader cluster.VMStatusReader
}

// applyCloudInitPasswordFlow runs the whole password step: the pre-flight
// refusals, the ciuser resolution (patch value first, then the live config -
// never a fallback to a locked root), and the bounded agent wait.
func applyCloudInitPasswordFlow(ctx context.Context, deps CloudInitConfigDeps, writer cluster.Writer, entity Entity, current, effective cluster.CloudInitConfig, password string) error {
	if err := preflightGuestAgent(ctx, deps, writer, entity, current); err != nil {
		return err
	}

	if effective.User == "" {
		return ErrNoCloudInitUser
	}

	if err := applyCloudInitPassword(ctx, writer, entity, effective.User, password); err != nil {
		return fmt.Errorf("apply cloud-init password via guest agent: %w", err)
	}

	return nil
}

// agentPingPoll is the interval between guest-agent pings while waiting for
// the guest to become reachable; maxAgentPingWait bounds the total wait for
// both the ping phase and the missing-account retries. Vars so tests can
// shorten them, mirroring agentExecPoll in the cluster package.
var (
	agentPingPoll    = 2 * time.Second
	maxAgentPingWait = 30 * time.Second
)

// agentPasswordBudget caps the whole guest-agent password step (ping plus
// account retries) inside one synchronous request. It must stay below the
// server WriteTimeout (10 s, cmd/pvmss/main.go): past that the connection is
// killed and the 504 never reaches the client.
var agentPasswordBudget = 7 * time.Second

// preflightGuestAgent refuses a password request that cannot succeed before
// anything is written: the guest agent must be enabled in the VM config, and
// the VM must be running (read live, not from the up-to-30s-stale projection -
// ADR 0001). Both refusals are immediate and actionable where the raw agent
// error today is opaque.
func preflightGuestAgent(ctx context.Context, deps CloudInitConfigDeps, writer cluster.Writer, entity Entity, current cluster.CloudInitConfig) error {
	if !current.Agent {
		return ErrGuestAgentDisabled
	}

	if deps.StatusReader == nil {
		return nil
	}

	status, err := deps.StatusReader.VMStatus(ctx, entity.Node, entity.VMID)
	if err != nil {
		// The live read is a courtesy check; an unreachable reader must not
		// block the operation - the bounded ping below is the real gate.
		return nil //nolint:nilerr // deliberate fall-through: the ping is the authoritative gate
	}

	if status.Status != cluster.VMRunning {
		return ErrVMNotRunning
	}

	// Past the boot grace an agent that does not answer is not coming: one
	// ping decides, instead of a 30 s wait ending in a 504.
	if status.Uptime > agentBootGrace {
		if err := writer.PingGuestAgent(ctx, entity.Node, entity.VMID); err != nil {
			return fmt.Errorf("%w (is qemu-guest-agent installed and running?): %w", ErrGuestAgentUnreachable, err)
		}
	}

	return nil
}

// agentBootGrace is how long after boot the guest agent may still be coming
// up; within it the bounded wait in applyCloudInitPassword applies.
const agentBootGrace = 3 * time.Minute

// applyCloudInitPassword waits for the QEMU guest agent within a bounded
// window, then applies the password to user, retrying while cloud-init has
// not yet created the account (cc_users_groups runs tens of seconds after
// Proxmox reports the VM running). The password is never stored, so the whole
// operation must succeed inside one synchronous request - hence the bounded,
// in-request wait instead of ProxMate's persisted-retry subsystem.
func applyCloudInitPassword(ctx context.Context, writer cluster.Writer, entity Entity, user, password string) error {
	ctx, cancel := context.WithTimeout(ctx, agentPasswordBudget)
	defer cancel()

	deadline := time.NewTimer(maxAgentPingWait)
	defer deadline.Stop()

	ticker := time.NewTicker(agentPingPoll)
	defer ticker.Stop()

	agentReady := false
	for {
		if !agentReady {
			if err := writer.PingGuestAgent(ctx, entity.Node, entity.VMID); err != nil {
				if !waitNextProbe(ctx, deadline.C, ticker.C) {
					return fmt.Errorf("%w: no answer in time (is qemu-guest-agent installed and running?): %w", ErrGuestAgentUnreachable, err)
				}

				continue
			}

			agentReady = true
		}

		err := writer.SetCloudInitPassword(ctx, entity.Node, entity.VMID, user, password)
		if err == nil || !errors.Is(err, cluster.ErrGuestUserUnknown) {
			return err
		}

		if !waitNextProbe(ctx, deadline.C, ticker.C) {
			return fmt.Errorf("guest account %q did not appear within %s: %w", user, maxAgentPingWait, err)
		}
	}
}

// waitNextProbe blocks until the next probe tick, the deadline, or context
// cancellation. ok is false when the caller must stop (deadline or ctx).
func waitNextProbe(ctx context.Context, deadline, tick <-chan time.Time) (ok bool) {
	select {
	case <-ctx.Done():
		return false
	case <-deadline:
		return false
	case <-tick:
		return true
	}
}

func resolveCloudInitTarget(index *inventory.Index, actor auth.Identity, clusterName string, vmid int) (Entity, error) {
	if index == nil {
		return Entity{}, ErrNotFound
	}

	return Resolve(index, actor, clusterName, vmid)
}
