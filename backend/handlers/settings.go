package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
	"pvmss/state"
)

// SettingsHandler handles settings-related routes
type SettingsHandler struct {
	stateManager state.StateManager
}

// NewSettingsHandler creates a new instance of SettingsHandler
func NewSettingsHandler(sm state.StateManager) *SettingsHandler {
	return &SettingsHandler{stateManager: sm}
}

// GetSettingsHandler returns the current application settings
func (h *SettingsHandler) GetSettingsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	settings := h.stateManager.GetSettings()
	if settings == nil {
		logger.Get().Error().Msg("Settings not available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Settings not available",
		}); err != nil {
			logger.Get().Error().Err(err).Msg("Failed to encode JSON error response")
		}
		return
	}

	// Do not return the admin password
	settingsResponse := map[string]interface{}{
		"tags":   settings.Tags,
		"isos":   settings.ISOs,
		"vmbrs":  settings.VMBRs,
		"limits": settings.Limits,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(settingsResponse); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to encode JSON response")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// GetAllVMBRsHandler retrieves all available network bridges
func (h *SettingsHandler) GetAllVMBRsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Use shared helper to collect VMBRs
	vmbrs, err := collectAllVMBRs(h.stateManager)
	if err != nil {
		logger.Get().Warn().Err(err).Msg("collectAllVMBRs returned an error")
	}

	// Format for API response
	formatted := make([]map[string]interface{}, 0, len(vmbrs))
	for _, v := range vmbrs {
		formatted = append(formatted, map[string]interface{}{
			"name":        v["iface"],
			"description": v["description"],
			"node":        v["node"],
			"type":        v["type"],
			"method":      v["method"],
			"address":     v["address"],
			"netmask":     v["netmask"],
			"gateway":     v["gateway"],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"vmbrs":  formatted,
	}); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to encode JSON response")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// GetAllSettingsHandler returns all application settings
func (h *SettingsHandler) GetAllSettingsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	settings := h.stateManager.GetSettings()
	if settings == nil {
		logger.Get().Error().Msg("Settings not available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Settings not available",
		}); err != nil {
			logger.Get().Error().Err(err).Msg("Failed to encode JSON error response")
		}
		return
	}

	// Do not return the admin password
	settingsResponse := map[string]interface{}{
		"tags":   settings.Tags,
		"isos":   settings.ISOs,
		"vmbrs":  settings.VMBRs,
		"limits": settings.Limits,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(settingsResponse); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to encode JSON response")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// RegisterRoutes registers routes for settings-related endpoints
func (h *SettingsHandler) RegisterRoutes(router *httprouter.Router) {
	// API routes protected by authentication
	router.GET("/api/settings", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.GetSettingsHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	router.GET("/api/vmbr/all", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.GetAllVMBRsHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// API endpoint for fetching all settings
	router.GET("/api/settings/all", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.GetAllSettingsHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
}
