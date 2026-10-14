package httpapi

import (
	"context"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/vm"
	"time"
)

// ActionTaskWait bounds how long a power action waits for its Proxmox task to
// settle before answering 202 accepted. Kept well under the server's 10s
// WriteTimeout. A var so tests can shorten it.
var ActionTaskWait = 3 * time.Second

// awaitActionTask waits up to ActionTaskWait for the power action's task to
// settle. settled is false when the outcome is still unknown (deadline, no task
// reader, or a failed read): the caller must then report the action as
// accepted, never as failed.
func (h *VMDetail) awaitActionTask(ctx context.Context, clusterName, upid string) (status cluster.TaskStatus, settled bool) {
	if upid == "" {
		return cluster.TaskStatus{}, false
	}

	creator, err := resolveCapability(h.clients, h.creator, clusterName, "Creator")
	if err != nil || creator == nil {
		return cluster.TaskStatus{}, false
	}

	waitCtx, cancel := context.WithTimeout(ctx, ActionTaskWait)
	defer cancel()

	status, settled, readErr := vm.AwaitTask(waitCtx, creator, upid)
	if readErr != nil {
		logctx.FromOr(ctx, h.log).WarnContext(ctx, "power action task status read failed", "component", "httpapi", "cluster", clusterName, "error", readErr)
	}

	return status, settled
}

// writeActionOutcome maps the observed task outcome onto the response:
// a failed task is 409 task_failed carrying Proxmox's exit message, a
// finished task is 200 ok, and anything still unknown is 202 accepted with
// the UPID so the client can keep polling GET /api/v1/tasks/{upid}. With no
// UPID (no cluster task was started) it keeps the plain 200 accepted contract.
func (h *VMDetail) writeActionOutcome(w http.ResponseWriter, upid string, status cluster.TaskStatus, settled bool) {
	switch {
	case upid == "":
		h.writeJSONStatus(w, http.StatusOK, actionResponse{Status: "accepted"})
	case settled && status.State == cluster.TaskError:
		h.writeDetailError(w, http.StatusConflict, "task_failed", status.ExitMessage)
	case settled:
		h.writeJSONStatus(w, http.StatusOK, actionResponse{Status: "ok", Upid: upid})
	default:
		h.writeJSONStatus(w, http.StatusAccepted, actionResponse{Status: "accepted", Upid: upid})
	}
}
