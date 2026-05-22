package handlers

import (
	"encoding/json"
	"net/http"

	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// HealthHandler serves the simple /health liveness probe used by Kubernetes.
// /api/v1/health is the canonical endpoint for the SPA and is registered separately.
type HealthHandler struct {
	stateManager state.StateManager
}

type healthServicesResponse struct {
	Proxmox string `json:"proxmox"`
}

type healthResponse struct {
	Status   string                 `json:"status"`
	Services healthServicesResponse `json:"services"`
}

// MakeHealthHandler creates a new instance of HealthHandler.
func MakeHealthHandler(stateManager state.StateManager) *HealthHandler {
	return &HealthHandler{stateManager: stateManager}
}

// HealthCheckHandler handles GET /health.
func (h *HealthHandler) HealthCheckHandler(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	proxmoxStatus := "ok"
	if !proxmoxConnected {
		proxmoxStatus = "unavailable"
	}

	response := healthResponse{
		Status: "ok",
		Services: healthServicesResponse{
			Proxmox: proxmoxStatus,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// RegisterRoutes registers the /health endpoint.
func (h *HealthHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/health", h.HealthCheckHandler)
}
