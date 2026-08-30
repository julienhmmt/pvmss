package vm

import (
	"context"
	"fmt"
	"pvmss/server/internal/cluster"
	"strings"
	"time"
)

// CreateTaskPoll is the interval between task-status reads while waiting for a
// create or clone task; MaxCreateTaskWait bounds the total wait. Exported as
// vars so tests can shorten them, mirroring maxForceStopWait in actions.go.
var CreateTaskPoll = 2 * time.Second

// MaxCreateTaskWait bounds the total wait for a create or clone task.
var MaxCreateTaskWait = 10 * time.Minute

// WaitCreateTask polls upid until the cluster reports it finished. It mirrors
// the poll-then-timeout shape of deleteWithRetry in actions.go (lifecycle-04).
//
// Semantics:
//   - TaskOK → nil
//   - TaskError → error carrying ExitMessage and the last log lines (a bare
//     "create task failed" without the log is inexplicable in support)
//   - TaskRunning → retry at the next tick
//   - a transient read error → do not abort, retry (fail-soft, matching
//     proxmoxTaskLog's best-effort log fetch)
//   - MaxCreateTaskWait exceeded → explicit "create task timed out" error
//   - ctx.Done() → ctx.Err()
func WaitCreateTask(ctx context.Context, creator cluster.Creator, upid string) error {
	deadline := time.Now().Add(MaxCreateTaskWait)

	for {
		status, err := creator.TaskStatus(ctx, upid)
		if err != nil {
			// A transient read error (network blip, 5xx) must not abort the
			// wait — the task is still running on the cluster. Only ctx
			// cancellation propagates.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}

			time.Sleep(CreateTaskPoll)

			continue
		}

		switch status.State {
		case cluster.TaskOK:
			return nil
		case cluster.TaskError:
			return fmt.Errorf("create task failed: %s%s", status.ExitMessage, tailLog(status.Log))
		case cluster.TaskRunning:
			// Still running — fall through to the deadline check below.
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("create task timed out after %s (upid %s)", MaxCreateTaskWait, upid)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(CreateTaskPoll):
		}
	}
}

// waitCreateTask is the internal alias (lifecycle-04).
func waitCreateTask(ctx context.Context, creator cluster.Creator, upid string) error {
	return WaitCreateTask(ctx, creator, upid)
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
