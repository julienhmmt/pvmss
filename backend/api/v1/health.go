package apiv1

import (
	"net/http"

	"pvmss/constants"
	"pvmss/state"
)

// HealthHandler handles public health check endpoints.
type HealthHandler struct {
	state state.StateManager
}

// MakeHealthHandler creates a new HealthHandler.
func MakeHealthHandler(s state.StateManager) *HealthHandler {
	return &HealthHandler{state: s}
}

// Health handles GET /api/v1/health — public, no auth required.
// Returns application status and version.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{
		"status":  "ok",
		"version": constants.AppVersion,
	})
}

// HealthProxmox handles GET /api/v1/health/proxmox — public, no auth required.
// Returns Proxmox connection status.
func (h *HealthHandler) HealthProxmox(w http.ResponseWriter, r *http.Request) {
	connected, _ := h.state.GetProxmoxStatus()
	writeJSON(w, map[string]any{
		"connected": connected,
	})
}
