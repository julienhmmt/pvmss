package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"pvmss/logger"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// HealthHandler handles health and API endpoints
type HealthHandler struct {
	stateManager state.StateManager
}

// Helper function to send JSON responses
func sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if statusCode > 0 {
		w.WriteHeader(statusCode)
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// MakeHealthHandler creates a new instance of HealthHandler
func MakeHealthHandler(stateManager state.StateManager) *HealthHandler {
	return &HealthHandler{
		stateManager: stateManager,
	}
}

// HealthCheckHandler handles health check requests
func (h *HealthHandler) HealthCheckHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	proxmoxStatus := "ok"
	if !proxmoxConnected {
		proxmoxStatus = "unavailable"
	}

	response := map[string]interface{}{
		"status": "ok",
		"services": map[string]string{
			"proxmox": proxmoxStatus,
		},
	}

	sendJSONResponse(w, 0, response)
}

// ProxmoxStatusHandler handles requests to check Proxmox connection status
func (h *HealthHandler) ProxmoxStatusHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	connected, errorMsg := h.stateManager.GetProxmoxStatus()

	response := map[string]interface{}{
		"connected": connected,
	}
	if !connected && errorMsg != "" {
		response["error"] = errorMsg
	}

	sendJSONResponse(w, 0, response)
}

// NotFoundHandler handles routes that are not found
func (h *HealthHandler) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		sendJSONResponse(w, http.StatusNotFound, map[string]string{
			"error":   "Not Found",
			"message": "The requested resource was not found",
		})
	} else {
		// Redirect to home page for non-API routes (for client-side routing)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// MethodNotAllowedHandler handles unauthorized HTTP methods
func (h *HealthHandler) MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		sendJSONResponse(w, http.StatusMethodNotAllowed, map[string]string{
			"error":   "Method Not Allowed",
			"message": "The requested method is not allowed for this resource",
		})
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(JSONErrorResponse{
			Code:    "METHOD_NOT_ALLOWED",
			Message: "Method not allowed",
		})
	}
}

// RegisterRoutes registers health and API routes
func (h *HealthHandler) RegisterRoutes(router *httprouter.Router) {
	// Keep /health for ops tooling (Kubernetes liveness probes etc.)
	// /api/v1/health and /api/v1/health/proxmox are the canonical endpoints for the SPA.
	router.GET("/health", h.HealthCheckHandler)

	// Legacy API routes for backward compatibility (deprecated)
	router.GET("/api/health", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		logger.Get().Warn().Msg("Legacy /api/health endpoint is deprecated. Use /health or /api/v1/health instead.")
		h.HealthCheckHandler(w, r, p)
	})
	router.GET("/api/health/proxmox", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		logger.Get().Warn().Msg("Legacy /api/health/proxmox endpoint is deprecated. Use /api/v1/health/proxmox instead.")
		h.ProxmoxStatusHandler(w, r, p)
	})
}
