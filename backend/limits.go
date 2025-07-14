package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"pvmss/backend/proxmox"
)

// limitsHandler routes the requests to the appropriate handler based on the HTTP method.
func limitsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getLimitsHandler(w, r)
	case http.MethodPost:
		updateLimitsHandler(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getLimitsHandler returns the current resource limits for all nodes.
func getLimitsHandler(w http.ResponseWriter, r *http.Request) {
	settings, err := readSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read settings")
		http.Error(w, "Failed to read settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"limits": settings.Limits,
	})
}

// updateLimitsHandler handles updating the resource limits for a specific node.
func updateLimitsHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Node    string `json:"node"`
		Sockets MinMax `json:"sockets"`
		Cores   MinMax `json:"cores"`
		RAM     MinMax `json:"ram"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get node information from Proxmox API
	apiURL := os.Getenv("PROXMOX_URL")
	apiTokenID := os.Getenv("PROXMOX_API_TOKEN_NAME")
	apiTokenSecret := os.Getenv("PROXMOX_API_TOKEN_VALUE")
	insecure := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

	// Default values in case we can't get node details
	maxCPU := 16     // Default max CPU cores
	maxMemoryGB := 64 // Default max memory in GB

	// Try to get actual node details if possible
	if apiURL != "" && apiTokenID != "" && apiTokenSecret != "" {
		client, err := proxmox.NewClientWithOptions(apiURL, apiTokenID, apiTokenSecret, insecure)
		if err == nil {
			// Create context with timeout for API request
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			nodeDetails, err := proxmox.GetNodeDetailsWithContext(ctx, client, payload.Node)
			if err == nil {
				// Successfully got node details
				maxCPU = nodeDetails.MaxCPU
				maxMemoryGB = int(nodeDetails.MaxMemory / (1024 * 1024 * 1024))
				log.Info().Str("node", payload.Node).Int("maxCPU", maxCPU).Int("maxMemoryGB", maxMemoryGB).Msg("Retrieved node details for validation")
			} else {
				log.Warn().Err(err).Str("node", payload.Node).Msg("Could not get node details, using default limits")
			}
		} else {
			log.Warn().Err(err).Msg("Could not create Proxmox client, using default limits")
		}
	} else {
		log.Warn().Msg("Proxmox API credentials not set, using default limits")
	}

	// Validate limits and collect validation messages
	validationMessages := make([]string, 0)

	// Check max values against node capacities
	if payload.Sockets.Max > maxCPU {
		log.Info().Str("node", payload.Node).Int("requested", payload.Sockets.Max).Int("max", maxCPU).Msg("Limiting max sockets to node capacity")
		validationMessages = append(validationMessages, fmt.Sprintf("Max sockets limited to %d (node capacity)", maxCPU))
		payload.Sockets.Max = maxCPU
	}
	if payload.Cores.Max > maxCPU {
		log.Info().Str("node", payload.Node).Int("requested", payload.Cores.Max).Int("max", maxCPU).Msg("Limiting max cores to node capacity")
		validationMessages = append(validationMessages, fmt.Sprintf("Max cores limited to %d (node capacity)", maxCPU))
		payload.Cores.Max = maxCPU
	}
	if payload.RAM.Max > maxMemoryGB {
		log.Info().Str("node", payload.Node).Int("requested", payload.RAM.Max).Int("max", maxMemoryGB).Msg("Limiting max RAM to node capacity")
		validationMessages = append(validationMessages, fmt.Sprintf("Max RAM limited to %d GB (node capacity)", maxMemoryGB))
		payload.RAM.Max = maxMemoryGB
	}

	// Ensure min values are not negative
	if payload.Sockets.Min < 0 {
		validationMessages = append(validationMessages, "Min sockets cannot be negative, set to 0")
		payload.Sockets.Min = 0
	}
	if payload.Cores.Min < 0 {
		validationMessages = append(validationMessages, "Min cores cannot be negative, set to 0")
		payload.Cores.Min = 0
	}
	if payload.RAM.Min < 0 {
		validationMessages = append(validationMessages, "Min RAM cannot be negative, set to 0")
		payload.RAM.Min = 0
	}

	// Ensure min values don't exceed max values
	if payload.Sockets.Min > payload.Sockets.Max {
		validationMessages = append(validationMessages, "Min sockets cannot exceed max, adjusted accordingly")
		payload.Sockets.Min = payload.Sockets.Max
	}
	if payload.Cores.Min > payload.Cores.Max {
		validationMessages = append(validationMessages, "Min cores cannot exceed max, adjusted accordingly")
		payload.Cores.Min = payload.Cores.Max
	}
	if payload.RAM.Min > payload.RAM.Max {
		validationMessages = append(validationMessages, "Min RAM cannot exceed max, adjusted accordingly")
		payload.RAM.Min = payload.RAM.Max
	}

	// Read settings
	settings, err := readSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read settings for update")
		http.Error(w, "Failed to read settings for update", http.StatusInternalServerError)
		return
	}

	if settings.Limits == nil {
		settings.Limits = make(map[string]NodeLimits)
	}

	// Update limits
	limits := NodeLimits{
		Sockets: payload.Sockets,
		Cores:   payload.Cores,
		RAM:     payload.RAM,
	}

	settings.Limits[payload.Node] = limits

	if err := writeSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to write updated settings")
		http.Error(w, "Failed to write updated settings", http.StatusInternalServerError)
		return
	}

	// Return success with any validation messages
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	response := map[string]interface{}{
		"success": true,
		"message": "Limits updated successfully",
		"limits":  limits,
	}
	
	// Include validation messages if there were any adjustments
	if len(validationMessages) > 0 {
		response["validationMessages"] = validationMessages
	}
	
	json.NewEncoder(w).Encode(response)
}
