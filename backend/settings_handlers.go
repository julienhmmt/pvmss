package main

import (
	"encoding/json"
	"net/http"

	"pvmss/logger"
	"pvmss/state"
)

// settingsHandler handles GET requests to retrieve the complete current application settings.
// It reads the settings from the state package and returns them as a JSON response.
func settingsHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()
	log.Info().
		Str("handler", "settingsHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Settings handler invoked")

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	settings := state.GetAppSettings()
	if settings == nil {
		log.Error().Msg("Settings not initialized")
		http.Error(w, "Settings not initialized", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(settings); err != nil {
		log.Error().Err(err).Msg("Failed to encode settings response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Debug().Msg("Settings retrieved successfully")
}

// updateIsoSettingsHandler handles POST requests for updating the list of available ISO images.
// It expects a JSON payload containing the new list of ISOs and persists the changes.
func updateIsoSettingsHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()
	log.Info().
		Str("handler", "updateIsoSettingsHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Update ISO settings handler invoked")

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var requestData struct {
		ISOs []string `json:"isos"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate ISOs list
	if requestData.ISOs == nil {
		requestData.ISOs = []string{}
	}

	// Get current settings from state
	settings := state.GetAppSettings()
	if settings == nil {
		log.Error().Msg("Settings not initialized")
		http.Error(w, "Settings not initialized", http.StatusInternalServerError)
		return
	}

	// Create a copy to avoid modifying the cached version
	updatedSettings := *settings
	updatedSettings.ISOs = requestData.ISOs

	// Persist the updated settings
	if err := writeSettings(&updatedSettings); err != nil {
		log.Error().Err(err).Msg("Failed to save updated settings")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	// Update the state with the new settings
	state.SetAppSettings(&updatedSettings)

	log.Info().
		Int("iso_count", len(updatedSettings.ISOs)).
		Msg("ISO settings updated successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "ISO settings updated successfully",
		"isos":    updatedSettings.ISOs,
	})
}

// updateVmbrSettingsHandler handles POST requests for updating the list of available network bridges (VMBRs).
// It expects a JSON payload containing the new list of VMBRs and persists the changes.
func updateVmbrSettingsHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()
	log.Info().
		Str("handler", "updateVmbrSettingsHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Update VMBR settings handler invoked")

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var requestData struct {
		VMBRs []string `json:"vmbrs"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate VMBRs list
	if requestData.VMBRs == nil {
		requestData.VMBRs = []string{}
	}

	// Get current settings from state
	settings := state.GetAppSettings()
	if settings == nil {
		log.Error().Msg("Settings not initialized")
		http.Error(w, "Settings not initialized", http.StatusInternalServerError)
		return
	}

	// Create a copy to avoid modifying the cached version
	updatedSettings := *settings
	updatedSettings.VMBRs = requestData.VMBRs

	// Persist the updated settings
	if err := writeSettings(&updatedSettings); err != nil {
		log.Error().Err(err).Msg("Failed to save updated settings")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	// Update the state with the new settings
	state.SetAppSettings(&updatedSettings)

	log.Info().
		Int("vmbr_count", len(updatedSettings.VMBRs)).
		Msg("VMBR settings updated successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "VMBR settings updated successfully",
		"vmbrs":   updatedSettings.VMBRs,
	})
}
