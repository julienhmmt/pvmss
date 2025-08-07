package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"pvmss/state"
)

// HealthHandler gère les points de terminaison de santé et d'API
type HealthHandler struct {
	stateManager state.StateManager
}

// ProxmoxStatusResponse represents the Proxmox connection status response
type ProxmoxStatusResponse struct {
	Connected bool   `json:"connected"`
	Error     string `json:"error,omitempty"`
}

// NewHealthHandler crée une nouvelle instance de HealthHandler
func NewHealthHandler(stateManager state.StateManager) *HealthHandler {
	return &HealthHandler{
		stateManager: stateManager,
	}
}

// HealthCheckHandler gère les requêtes de vérification de santé
func (h *HealthHandler) HealthCheckHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Check Proxmox connection status
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	proxmoxStatus := "ok"
	if !proxmoxConnected {
		proxmoxStatus = "unavailable"
	}

	// Session status (always ok for now)
	sessionStatus := "ok"

	// Prepare response
	response := map[string]interface{}{
		"status":  "ok",
		"version": "1.0.0",
		"services": map[string]string{
			"proxmox": proxmoxStatus,
			"session": sessionStatus,
		},
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ProxmoxStatusHandler handles requests to check Proxmox connection status
func (h *HealthHandler) ProxmoxStatusHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Check Proxmox connection status
	connected, errorMsg := h.stateManager.GetProxmoxStatus()

	// Prepare response
	response := ProxmoxStatusResponse{
		Connected: connected,
	}

	if !connected {
		response.Error = errorMsg
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// NotFoundHandler gère les routes non trouvées
func (h *HealthHandler) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		// JSON response for API routes
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": "The requested resource was not found",
		})
	} else {
		// Redirect to home page for non-API routes (for client-side routing)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// MethodNotAllowedHandler gère les méthodes HTTP non autorisées
func (h *HealthHandler) MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		// JSON response for API routes
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Method Not Allowed",
			"message": "The requested method is not allowed for this resource",
		})
	} else {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// RegisterRoutes enregistre les routes de santé et d'API
func (h *HealthHandler) RegisterRoutes(router *httprouter.Router) {
	// Health endpoints
	router.GET("/health", h.HealthCheckHandler)
	router.GET("/api/health", h.HealthCheckHandler)
	router.GET("/api/health/proxmox", h.ProxmoxStatusHandler)

	// Error handlers
	router.NotFound = http.HandlerFunc(h.NotFoundHandler)
	router.MethodNotAllowed = http.HandlerFunc(h.MethodNotAllowedHandler)
}
