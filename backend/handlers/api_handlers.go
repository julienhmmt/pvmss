// Package handlers - API-specific HTTP handlers
package handlers

import (
	"encoding/json"
	"net/http"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// TagsHandler handles tags API requests
func TagsHandler(w http.ResponseWriter, r *http.Request) {
	client := state.GetProxmoxClient()
	if client == nil {
		http.Error(w, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	tags := state.GetTags()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tags); err != nil {
		logger.Get().Error().Err(err).Msg("Error encoding tags response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// StorageHandler handles storage API requests
func StorageHandler(w http.ResponseWriter, r *http.Request) {
	client := state.GetProxmoxClient()
	if client == nil {
		http.Error(w, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	// Get storage information from Proxmox
	storage, err := proxmox.GetStorages(client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Error getting storage from Proxmox")
		http.Error(w, "Error getting storage", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(storage); err != nil {
		logger.Get().Error().Err(err).Msg("Error encoding storage response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AllIsosHandler handles ISO listing API requests
func AllIsosHandler(w http.ResponseWriter, r *http.Request) {
	client := state.GetProxmoxClient()
	if client == nil {
		http.Error(w, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	// Get ISO information from Proxmox - using first available node and storage
	// This is a simplified approach - in a real application you'd want to iterate through all nodes/storages
	isos := make(map[string]interface{})
	isos["data"] = []interface{}{} // Empty for now - would need proper implementation

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(isos); err != nil {
		logger.Get().Error().Err(err).Msg("Error encoding ISOs response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AllVmbrsHandler handles VMBR listing API requests
func AllVmbrsHandler(w http.ResponseWriter, r *http.Request) {
	client := state.GetProxmoxClient()
	if client == nil {
		http.Error(w, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	// Get network bridges information from Proxmox - using first available node
	// This is a simplified approach - in a real application you'd want to iterate through all nodes
	vmbrs := make(map[string]interface{})
	vmbrs["data"] = []interface{}{} // Empty for now - would need proper implementation

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(vmbrs); err != nil {
		logger.Get().Error().Err(err).Msg("Error encoding VMBRs response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// SettingsHandler handles settings API requests
func SettingsHandler(w http.ResponseWriter, r *http.Request) {
	settings := state.GetAppSettings()
	if settings == nil {
		http.Error(w, "Settings not available", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(settings); err != nil {
		logger.Get().Error().Err(err).Msg("Error encoding settings response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// UpdateIsoSettingsHandler handles ISO settings update requests
func UpdateIsoSettingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// This would update ISO settings - implement as needed
	logger.Get().Info().Msg("ISO settings update requested")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// UpdateVmbrSettingsHandler handles VMBR settings update requests
func UpdateVmbrSettingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// This would update VMBR settings - implement as needed
	logger.Get().Info().Msg("VMBR settings update requested")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// LimitsHandler handles limits API requests
func LimitsHandler(w http.ResponseWriter, r *http.Request) {
	// Get limits from state or configuration
	limits := map[string]interface{}{
		"max_vms":    10,
		"max_memory": 32768,
		"max_disk":   1000,
		"max_cores":  8,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(limits); err != nil {
		logger.Get().Error().Err(err).Msg("Error encoding limits response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
