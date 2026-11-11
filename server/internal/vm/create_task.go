package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"strings"
	"time"
)

// createRollbackTimeout bounds the purge of a half-made VM after a failed clone.
const createRollbackTimeout = 30 * time.Second

// ErrCreateTaskTimeout means a create or clone task was still running at
// MaxCreateTaskWait. The VM may still be mid-clone, so it must not be deleted.
var ErrCreateTaskTimeout = errors.New("create task timed out")

// CreateTaskPoll is the interval between task-status reads while waiting for a
// create or clone task; MaxCreateTaskWait bounds the total wait. Exported as
// vars so tests can shorten them, mirroring maxForceStopWait in actions.go.
var CreateTaskPoll = 2 * time.Second

// MaxCreateTaskWait bounds the total wait for a create or clone task.
var MaxCreateTaskWait = 10 * time.Minute

// WaitCreateTask polls upid until the cluster reports it finished. It mirrors
// the poll-then-timeout shape of deleteWithRetry in actions.go.
//
// Semantics:
// - TaskOK → nil
// - TaskError → error carrying ExitMessage and the last log lines (a bare
// "create task failed" without the log is inexplicable in support)
// - TaskRunning → retry at the next tick
// - a transient read error → do not abort, retry (fail-soft, matching
// proxmoxTaskLog's best-effort log fetch)
// - MaxCreateTaskWait exceeded → explicit "create task timed out" error
// - ctx.Done() → ctx.Err().
func WaitCreateTask(ctx context.Context, creator cluster.Creator, upid string) error {
	deadline := time.Now().Add(MaxCreateTaskWait)

	for {
		status, err := creator.TaskStatus(ctx, upid)
		// A read error (network blip, 5xx) falls through to the deadline check
		// and the poll: the task is still running on the cluster.
		if err == nil {
			switch status.State {
			case cluster.TaskOK:
				return nil
			case cluster.TaskError:
				return fmt.Errorf("create task failed: %s%s", status.ExitMessage, tailLog(status.Log))
			case cluster.TaskRunning:
				// Still running - fall through to the deadline check below.
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("%w after %s (upid %s)", ErrCreateTaskTimeout, MaxCreateTaskWait, upid)
		}

		if pollErr := sleepPoll(ctx); pollErr != nil {
			return pollErr
		}
	}
}

// sleepPoll waits one CreateTaskPoll interval, or returns early when ctx ends.
func sleepPoll(ctx context.Context) error {
	timer := time.NewTimer(CreateTaskPoll)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// awaitCloneTask waits for a clone task on a context a dropped request cannot
// cancel: once the VM exists in Proxmox, abandoning the wait orphans it.
func awaitCloneTask(ctx context.Context, creator cluster.Creator, upid string) error {
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), MaxCreateTaskWait)
	defer cancel()

	return WaitCreateTask(bounded, creator, upid)
}

// cleanupFailedClone handles a clone whose task wait failed. A wait that ran
// out of time leaves the VM possibly mid-clone, and Proxmox refuses to delete a
// locked VM, so it is only logged for manual cleanup. Any other failure purges
// the half-made VM on a context the cancelled request cannot reach.
func cleanupFailedClone(ctx context.Context, deps CreateDeps, actor auth.Identity, clusterName string, vmid int, node string, waitErr error) {
	if errors.Is(waitErr, ErrCreateTaskTimeout) || errors.Is(waitErr, context.DeadlineExceeded) {
		deps.Log.ErrorContext(ctx, "clone task still running, vm needs manual cleanup", "component", "vm", "cluster", clusterName, "vmid", vmid, "node", node, "error", waitErr)

		return
	}

	deps.Log.ErrorContext(ctx, "clone task wait failed", "component", "vm", "cluster", clusterName, "vmid", vmid, "error", waitErr)

	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), createRollbackTimeout)
	defer cancel()

	rollbackFailedCreate(bounded, deps, actor, clusterName, vmid, node, "clone task failed")
}

// tailLog appends up to 3 trailing log lines to an error message so support
// can see the actual Proxmox failure (e.g. "storage X does not support Y")
// without a separate task-log lookup. Mirrors pegaprox's 3-line tail.
func tailLog(log []string) string {
	if len(log) == 0 {
		return ""
	}

	tail := log
	if len(tail) > 3 {
		tail = tail[len(tail)-3:]
	}

	return "\n" + strings.Join(tail, "\n")
}
