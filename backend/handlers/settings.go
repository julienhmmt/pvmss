package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
	"pvmss/state"
)

// sendSettingsJSONResponse sends settings as JSON response
func sendSettingsJSONResponse(w http.ResponseWriter, settings *state.AppSettings) {
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

	// Provide safe defaults for nil pointers
	tags := settings.Tags
	if tags == nil {
		tags = []string{}
	}

	// Convert LimitsConfig to map[string]interface{} for JSON response compatibility
	limits := make(map[string]interface{})
	vmLimits := make(map[string]interface{})
	vmLimits["sockets"] = map[string]int{"min": settings.Limits.VM.Sockets.Min, "max": settings.Limits.VM.Sockets.Max}
	vmLimits["cores"] = map[string]int{"min": settings.Limits.VM.Cores.Min, "max": settings.Limits.VM.Cores.Max}
	vmLimits["ram"] = map[string]int{"min": settings.Limits.VM.RAM.Min, "max": settings.Limits.VM.RAM.Max}
	vmLimits["disk"] = map[string]int{"min": settings.Limits.VM.Disk.Min, "max": settings.Limits.VM.Disk.Max}
	limits["vm"] = vmLimits

	if len(settings.Limits.Nodes) > 0 {
		nodesLimits := make(map[string]interface{})
		for nodeName, nodeLimits := range settings.Limits.Nodes {
			nodeMap := map[string]interface{}{
				"sockets": map[string]int{"min": nodeLimits.Sockets.Min, "max": nodeLimits.Sockets.Max},
				"cores":   map[string]int{"min": nodeLimits.Cores.Min, "max": nodeLimits.Cores.Max},
				"ram":     map[string]int{"min": nodeLimits.RAM.Min, "max": nodeLimits.RAM.Max},
				"disk":    map[string]int{"min": nodeLimits.Disk.Min, "max": nodeLimits.Disk.Max},
				"max_vms": nodeLimits.MaxVMs,
			}
			nodesLimits[nodeName] = nodeMap
		}
		limits["nodes"] = nodesLimits
	}

	settingsResponse := map[string]interface{}{
		"tags":   tags,
		"isos":   settings.ISOs,
		"vmbrs":  settings.VMBRs,
		"limits": limits,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(settingsResponse); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to encode JSON response")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// SettingsHandler handles settings-related routes
type SettingsHandler struct {
	stateManager state.StateManager
}

// MakeSettingsHandler creates a new instance of SettingsHandler
func MakeSettingsHandler(sm state.StateManager) *SettingsHandler {
	return &SettingsHandler{stateManager: sm}
}

// GetSettingsHandler returns the current application settings
func (h *SettingsHandler) GetSettingsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	sendSettingsJSONResponse(w, h.stateManager.GetSettings())
}

// GetAllVMBRsHandler retrieves all available network bridges
func (h *SettingsHandler) GetAllVMBRsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Use shared helper to collect VMBRs
	vmbrs, err := collectAllVMBRs(r.Context(), h.stateManager)
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

// GetAllSettingsHandler returns all application settings (alias for GetSettingsHandler)
func (h *SettingsHandler) GetAllSettingsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	sendSettingsJSONResponse(w, h.stateManager.GetSettings())
}

// RegisterRoutes registers routes for settings-related endpoints.
// The SPA uses /api/v1/admin/settings instead — these legacy routes are kept for backward compatibility.
func (h *SettingsHandler) RegisterRoutes(router *httprouter.Router) {
	// Legacy API routes for backward compatibility (deprecated)
	router.GET("/api/settings", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		logger.Get().Warn().Msg("Legacy /api/settings endpoint is deprecated. Use /api/v1/admin/settings instead.")
		HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
			h.GetSettingsHandler(w, r, httprouter.ParamsFromContext(r.Context()))
		}))(w, r, p)
	})

	router.GET("/api/vmbr/all", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		logger.Get().Warn().Msg("Legacy /api/vmbr/all endpoint is deprecated. Use /api/v1/admin/vmbr instead.")
		HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
			h.GetAllVMBRsHandler(w, r, httprouter.ParamsFromContext(r.Context()))
		}))(w, r, p)
	})

	router.GET("/api/settings/all", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		logger.Get().Warn().Msg("Legacy /api/settings/all endpoint is deprecated. Use /api/v1/admin/settings instead.")
		HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
			h.GetAllSettingsHandler(w, r, httprouter.ParamsFromContext(r.Context()))
		}))(w, r, p)
	})
}
