package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"pvmss/state"
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

// NewHealthHandler creates a new instance of HealthHandler
func NewHealthHandler(stateManager state.StateManager) *HealthHandler {
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
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// RegisterRoutes registers health and API routes
func (h *HealthHandler) RegisterRoutes(router *httprouter.Router) {
	// Health endpoints
	router.GET("/health", h.HealthCheckHandler)
	router.GET("/api/health", h.HealthCheckHandler)
	router.GET("/api/health/proxmox", h.ProxmoxStatusHandler)

	// Error handlers
	router.NotFound = http.HandlerFunc(h.NotFoundHandler)
	router.MethodNotAllowed = http.HandlerFunc(h.MethodNotAllowedHandler)
}
