package vm

import (
	"context"
	"pvmss/server/internal/cluster"
	"time"
)

// taskPollInterval is the gap between TaskStatus reads while waiting.
const taskPollInterval = 300 * time.Millisecond

// TaskReader reads a Proxmox task's live state. Writers and Creators both
// satisfy it.
type TaskReader interface {
	TaskStatus(ctx context.Context, upid string) (cluster.TaskStatus, error)
}

// AwaitTask polls a task until it leaves the running state or ctx ends. The
// caller bounds the wait with ctx (a shared deadline when several tasks wait
// together). settled is false when the outcome is unknown: ctx ended, or the
// read failed (readErr then carries the cause). The caller must report an
// unsettled task as accepted, never as failed.
func AwaitTask(ctx context.Context, reader TaskReader, upid string) (status cluster.TaskStatus, settled bool, readErr error) {
	ticker := time.NewTicker(taskPollInterval)
	defer ticker.Stop()

	for {
		st, err := reader.TaskStatus(ctx, upid)
		if err != nil {
			return cluster.TaskStatus{}, false, err
		}

		if st.State != cluster.TaskRunning {
			return st, true, nil
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return cluster.TaskStatus{}, false, nil
		}
	}
}
