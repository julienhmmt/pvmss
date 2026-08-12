package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"time"
)

// TaskInvalidator rebuilds the inventory projection when a creation task
// completes (FR-018). *inventory.Worker satisfies it; the unguarded
// Refresh is used deliberately — the manual-refresh minimum interval does
// not apply to write-triggered invalidation.
type TaskInvalidator interface {
	Refresh(ctx context.Context) (time.Time, error)
}

// Tasks serves GET /api/v1/tasks/{upid} — a live read of an asynchronous
// cluster task (FR-014). No PVMSS-side task table exists; the state is asked
// of the cluster client on every poll (plan.md research decisions).
type Tasks struct {
	auth        *Auth
	creator     cluster.Creator
	invalidator TaskInvalidator
	log         *slog.Logger
}

// NewTasks creates the handler.
func NewTasks(authHandler *Auth, creator cluster.Creator, invalidator TaskInvalidator, log *slog.Logger) *Tasks {
	return &Tasks{auth: authHandler, creator: creator, invalidator: invalidator, log: log}
}

type taskStatusDTO struct {
	UPID        string   `json:"upid"`
	State       string   `json:"state"`
	Log         []string `json:"log"`
	ExitMessage string   `json:"exitMessage,omitempty"`
}

// ServeHTTP polls one task. When the task is observed in its ok state, the
// inventory index is invalidated so the next VM-list load shows the created
// VM without a manual refresh (FR-018) — not at POST /vms time, when the VM
// does not exist yet.
//
// The endpoint authenticates the caller but does not verify task ownership:
// any authenticated user can poll any UPID. This is accepted for T06 because
// UPIDs are opaque, reveal only creation progress (not VM data), and the
// tray is tab-local. T11+ may tie UPIDs to the actor's pool if needed.
func (h *Tasks) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, err := h.auth.Principal(r); err != nil {
		h.writeTaskError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	upid := r.PathValue("upid")
	if upid == "" {
		h.writeTaskError(w, http.StatusBadRequest, "invalid_request", "missing task upid")
		return
	}

	status, err := h.creator.TaskStatus(r.Context(), upid)
	if errors.Is(err, cluster.ErrNotFound) {
		h.writeTaskError(w, http.StatusNotFound, "not_found", "unknown task")
		return
	}

	if err != nil {
		h.log.Error("task status read failed", "component", "httpapi", "error", err)
		h.writeTaskError(w, http.StatusBadGateway, "cluster_error", "cluster rejected the request")

		return
	}

	if status.State == cluster.TaskOK {
		if _, err := h.invalidator.Refresh(r.Context()); err != nil {
			// The task genuinely succeeded; a failed invalidation only delays
			// list visibility until the next automatic cycle — do not fail
			// the poll for it.
			h.log.Error("post-task inventory invalidation failed", "component", "httpapi", "error", err)
		}
	}

	h.writeTaskJSON(w, http.StatusOK, taskStatusDTO{
		UPID:        status.UPID,
		State:       string(status.State),
		Log:         status.Log,
		ExitMessage: status.ExitMessage,
	})
}

func (h *Tasks) writeTaskJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.log.Error("failed to marshal response", "component", "httpapi", "error", err)
		h.writeTaskError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write response", "component", "httpapi", "error", err)
	}
}

func (h *Tasks) writeTaskError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write error response", "component", "httpapi", "code", code, "error", err)
	}
}
