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
	UPID             string `json:"upid"`
	Node             string `json:"node"`
	Status           string `json:"status"`
	ExitStatus       string `json:"exitstatus"`
	CloudInitWarning string `json:"cloud_init_warning,omitempty"`
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

	// Resolve any cloud-init warning associated with this UPID. When the
	// sentinel is still pending, the creation task has already reported
	// "stopped" but cloud-init is being applied asynchronously by
	// finalizeAfterTask. Wait briefly for the real result so the frontend can
	// surface snippet-upload failures instead of silently reporting success.
	cloudInitWarning := h.state.GetCloudInitWarning(upid)

	// Refresh while pending only when the creation task itself succeeded;
	// otherwise cloud-init is never applied and finalizeAfterTask clears the
	// sentinel with an empty string.
	if cloudInitWarning == state.CloudInitPending && status.Status == "stopped" && status.ExitStatus == "OK" {
		deadline := time.Now().Add(35 * time.Second)
		ticker := time.NewTicker(500 * time.Millisecond)
		for {
			if time.Now().After(deadline) {
				ticker.Stop()
				cloudInitWarning = "cloud-init-result-unknown"
				break
			}
			cur := h.state.GetCloudInitWarning(upid)
			if cur != state.CloudInitPending {
				cloudInitWarning = cur
				ticker.Stop()
				break
			}
			<-ticker.C
		}
	}

	// Hide the internal sentinel from clients.
	if cloudInitWarning == state.CloudInitPending {
		cloudInitWarning = ""
	}

	writeJSON(w, TaskStatusResponse{
		UPID:             upid,
		Node:             node,
		Status:           status.Status,
		ExitStatus:       status.ExitStatus,
		CloudInitWarning: cloudInitWarning,
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
