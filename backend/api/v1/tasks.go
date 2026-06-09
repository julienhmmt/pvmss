package apiv1

import (
	"context"
	"net/http"
	"time"

	"pvmss/proxmox"
	"pvmss/state"
)

// TaskHandler handles task status and log API endpoints.
type TaskHandler struct {
	state state.StateManager
}

// MakeTaskHandler creates a new TaskHandler.
func MakeTaskHandler(s state.StateManager) *TaskHandler {
	return &TaskHandler{state: s}
}

// TaskStatusResponse is the response for GET /api/v1/tasks/status
type TaskStatusResponse struct {
	UPID       string `json:"upid"`
	Node       string `json:"node"`
	Status     string `json:"status"`
	ExitStatus string `json:"exitstatus"`
}

// TaskLogEntryResponse is a single log line for GET /api/v1/tasks/log
type TaskLogEntryResponse struct {
	N int    `json:"n"`
	T string `json:"t"`
}

// GetTaskStatus handles GET /api/v1/tasks/status?node=X&upid=Y
func (h *TaskHandler) GetTaskStatus(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errOffline(w)
		return
	}

	node := r.URL.Query().Get("node")
	upid := r.URL.Query().Get("upid")
	if node == "" || upid == "" {
		errBadRequest(w, "node and upid query parameters are required")
		return
	}

	cfg := h.state.GetEnvConfig()
	client, err := proxmox.MakeRestyClientFromEnvConfig(cfg, 30*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	status, err := proxmox.GetTaskStatusResty(ctx, client, node, upid)
	if err != nil {
		writeError(w, http.StatusBadGateway, "task_status_failed", "Failed to get task status from Proxmox")
		return
	}

	writeJSON(w, TaskStatusResponse{
		UPID:       upid,
		Node:       node,
		Status:     status.Status,
		ExitStatus: status.ExitStatus,
	})
}

// GetTaskLog handles GET /api/v1/tasks/log?node=X&upid=Y
func (h *TaskHandler) GetTaskLog(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errOffline(w)
		return
	}

	node := r.URL.Query().Get("node")
	upid := r.URL.Query().Get("upid")
	if node == "" || upid == "" {
		errBadRequest(w, "node and upid query parameters are required")
		return
	}

	cfg := h.state.GetEnvConfig()
	client, err := proxmox.MakeRestyClientFromEnvConfig(cfg, 30*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	logs, err := proxmox.GetTaskLogResty(ctx, client, node, upid)
	if err != nil {
		writeError(w, http.StatusBadGateway, "task_log_failed", "Failed to get task log from Proxmox")
		return
	}

	entries := make([]TaskLogEntryResponse, len(logs))
	for i, l := range logs {
		entries[i] = TaskLogEntryResponse{N: l.N, T: l.T}
	}

	writeJSON(w, entries)
}
